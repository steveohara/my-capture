# Architecture

my-capture follows how OBS Studio captures from hardware: the video comes from
the operating system's capture framework, the audio is a separate source with
its own input and output devices.

```
USB capture device
  |-- video --> ffmpeg (avfoundation | dshow) --> raw RGBA on a pipe --> internal/video --> VideoView (Fyne)
  `-- audio --> miniaudio capture device --> ring buffer --> miniaudio playback device --> speakers / BLE headphones
```

## Video

`internal/video` runs ffmpeg as a child process.

- Devices: `ffmpeg -f avfoundation -list_devices true -i ""` on macOS and
  `ffmpeg -list_devices true -f dshow -i dummy` on Windows. The text is parsed
  by `parse.go`.
- Modes: on macOS ffmpeg is asked for an impossible 1x1 frame, and its error
  lists the supported modes. On Windows `-list_options true` is used.
- Capture: ffmpeg converts each frame to RGBA and writes it to standard output.
  The frame size is read from ffmpeg's description of its output stream on
  standard error. Four frame buffers are cycled so the user interface can draw
  one while the next is read.
- Video only. ffmpeg is told `-an`, because the audio path is separate.
- `-fps_mode passthrough` (ffmpeg 5.1+) writes each frame once. Without it ffmpeg
  pads to a constant rate and can repeat one frame hundreds of times a second
  when the camera's first timestamps are odd, which looks like a frozen picture.

`VideoView` keeps only the newest frame and has at most one redraw queued for
the Fyne main thread, so a slow screen drops frames instead of falling behind.
It uses `ImageScaleFastest`, which leaves scaling to the GPU; the default
smooth mode resamples on the CPU and is too slow for video.

### Aspect ratio

`VideoView` has a custom renderer, because forcing a shape needs the picture
placed in a computed rectangle. Fit, stretch and fill map directly onto
Fyne's `ImageFillContain`, `ImageFillStretch` and `ImageFillCover`. A fixed
ratio such as 16:9 is done by stretching the picture into the largest
rectangle of that ratio that fits, centred on the black background. All of
this is scaling on the GPU, so changing the mode costs nothing per frame. The
choice is remembered by name.

## Audio

`internal/audio` opens two miniaudio devices, a capture device and a playback
device, both as 48 kHz stereo 16 bit. miniaudio converts to the hardware
format. They are joined by a ring buffer because their clocks differ.

The ring buffer holds back the configured delay before playback starts. If the
writer runs more than 100 ms ahead of the target, the oldest audio is dropped.
If the reader runs dry, silence is played while the buffer refills. The delay is
the lip sync control: the video path through ffmpeg is usually slower than the
audio path, so the sound is delayed to match.

Changing the output device restarts only the playback device, so capture is not
interrupted.

## Window

`internal/ui` builds the Fyne window. All state lives on the Fyne main thread.
Slow work runs in goroutines that return with `fyne.Do`. A generation counter
discards stale results. Audio changes go through one worker goroutine so they
are applied in order.

Device lists are polled, not pushed, because neither Fyne nor ffmpeg reports
hot plug events. A ticker goroutine (`watchDevices`) asks the main thread to
start a check every 2 seconds for audio and 4 for video. The check runs in a
background goroutine, at most one at a time, and a failed check leaves the
lists unchanged. A picker is only redrawn when its list really changed, so an
open drop-down is not closed under the user. If the chosen device vanishes the
picker falls back to its first entry and remembers the name; when the name
appears again it is chosen again.

The menu bar is a `fyne.MainMenu`, which Fyne turns into the native menu: the
screen menu bar on macOS, a bar inside the window on Windows. Two Fyne
conventions matter. On macOS an item labelled exactly "About" is moved into the
application menu, so that label is used there and "About My Capture" elsewhere;
and Fyne adds Quit itself. Menu shortcuts only fire with a modifier held, so
`Cmd/Ctrl+F` and `Cmd/Ctrl+R` are menu shortcuts while the plain keys (`F`,
`F11`, `A`, `F1`, `Escape`) are handled by the key handler. Giving a plain key
to a menu item would also make the macOS menu treat it as `Cmd+key` and fire it
twice. The help window renders `internal/ui/help.md`, embedded with `go:embed`,
followed by the environment variable text from `internal/config`, so those
descriptions have one source. About shows the version, set at build time, and
asks ffmpeg for its own version.

The window is created unpadded and the controls carry their own padding, because
Fyne's `SetPadded` moves the content without resizing it: switching padding off
for full screen left a strip of window background, two paddings wide, down the
right and bottom edges.

The controls strip is shown only when the window is neither full screen nor
has had its controls hidden by the user (`fullscreen` and `controlsHidden` in
`applyControls`), so leaving full screen restores what the user had. Fyne
sends key presses to the focused widget, and only to the window-level handler
when nothing has focus. A dropdown or button that keeps the focus after being
used would therefore swallow `Escape`, `C`, `A` and `F`, so the focus is cleared
after each choice and whenever the controls are hidden.

Full screen uses Fyne's `SetFullScreen`, which uses the monitor the window is
currently on. Moving between monitors is done with the window's own title bar.

## Environment variables

All configuration is read in `internal/config`. The same text is printed by
`my-capture -help` and repeated in `README.md` and
`.agents/skills/my-capture/SKILL.md`; change all of them together.

| Variable | Meaning | Default |
|---|---|---|
| `MYCAPTURE_FFMPEG` | Absolute path of the ffmpeg executable. When unset it is searched for next to the executable, on the `PATH`, and in `/opt/homebrew/bin` and `/usr/local/bin`. | unset |
| `MYCAPTURE_AUDIO_DELAY_MS` | Initial audio delay in milliseconds, 20 to 1000. | `60` |
| `MYCAPTURE_DEBUG` | `1`, `true`, `yes` or `on` prints everything ffmpeg reports on its error stream. | unset |
| `MYCAPTURE_NO_RELAUNCH` | macOS only. `1`, `true`, `yes` or `on` stops the program restarting itself inside a temporary app bundle (see Name and icon). | unset |

## Name and icon

An application's name and icon normally come from its package: the `.app`
bundle on macOS (`Info.plist`, `icon.icns`) and the resources embedded in the
`.exe` on Windows (`rsrc_windows_amd64.syso`, picked up by Go because of its
`_windows_amd64` suffix). The icon is drawn by `tools/genicon` from distance
functions for rounded rectangles and circles, with the macOS version given the
transparent margin Apple's icons have.

A program run without a bundle gets neither, and macOS names it after its
executable (an IDE may build it as `___1App`). That name shows in the menu bar
and in the Dock tooltip. Two parts of this are worth knowing:

- The menu bar name is read by GLFW and AppKit from the bundle information, so
  `platform_darwin.go` writes the real name into that dictionary, and the
  program name, before Fyne creates the menu bar. It also sets the Dock icon,
  queued on the main thread because creating the application object early would
  stop GLFW installing its own.
- The Dock tooltip can not be fixed that way. The Dock takes the label when it
  first hears of the program and ignores later changes, including a changed
  LaunchServices display name (tried, and verified by hovering over the icon).
  The only reliable fix is to be inside a bundle from the start, so
  `relaunchInBundle` builds `My Capture.app` in the user cache folder from the
  embedded `Info.plist` and `icon.icns`, with a hard link (or copy) of the
  executable, and `exec`s into it. `exec` keeps the process ID, so an IDE's Stop
  button works. It is skipped when already in a bundle, when
  `MYCAPTURE_NO_RELAUNCH` is set, and when the P_TRACED flag shows a debugger,
  because a debugger loses a process that replaces itself. The environment
  variable `MYCAPTURE_RELAUNCHED` is set in the restarted process as a guard
  against restart loops.

Windows needs nothing at run time beyond Fyne's `SetIcon` for the window.

## Build and release

The `Makefile` holds the build, test and packaging targets, documented by
`make help` and in `README.md`. Because the program uses cgo, each artefact
needs a C compiler for its platform: the macOS bundle is built on a Mac (one
binary per architecture, joined with `lipo` into a universal binary, signed
and zipped), and the Windows build cross-compiles with MinGW-w64. The macOS
`Info.plist` in `packaging/macos` has the camera and microphone usage
descriptions without which macOS gives a bundled application no capture access.

### Releases

`.github/workflows/release.yml` is started by hand (`workflow_dispatch`). Job
`version` runs `.github/scripts/next-version.sh`, which raises the minor number
of the highest `vMAJOR.MINOR.PATCH` tag (or the patch or major number if
asked). Jobs `build-macos` and `build-windows` run `make check` and the
`dist-*` targets natively, because the program needs cgo and a C compiler for
its own platform; Windows uses MSYS2 for make, zip and MinGW-w64 gcc. Job
`release` downloads both zips, writes `SHA256SUMS.txt`, records build
provenance attestations, and runs `gh release create`, which makes the tag and
the release in one step with `--generate-notes`. A `concurrency` group stops
two runs from choosing the same version. The version reaches the program
through `-ldflags "-X main.version=..."`.

## Design decisions

- **ffmpeg as a child process, not cgo bindings.** It gives the same device
  coverage as OBS (AVFoundation and DirectShow) without maintaining bindings or
  depending on OpenCV. The cost is that ffmpeg must be installed or shipped.
- **Raw RGBA over a pipe.** Simple and fast to convert (swscale), but a 1080p60
  stream is about 500 MB/s. The mode list therefore prefers the widest mode up to 1920
  pixels, so 1920x1080 comes before odd shapes such as a 1552x1552 square crop.
- **Two audio devices, not one duplex device.** Duplex across two physical
  devices is not available on every miniaudio backend.
- **No volume control and no recording**, by design.

## Platform notes

- macOS is the platform this was developed and run on. The Windows (DirectShow)
  device and mode parsing is covered by sample output only and has not been run
  against real hardware.
- Linux is not a target. `internal/video` returns `ErrUnsupportedPlatform`.
