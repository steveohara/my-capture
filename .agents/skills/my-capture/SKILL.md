---
name: my-capture
description: Build, run and troubleshoot my-capture, a Go desktop program that shows the video and plays the audio of a USB video capture device on macOS and Windows. Use when working on or diagnosing capture devices, ffmpeg, audio output selection, or the full screen window.
---

# my-capture

Shows the picture and plays the sound of a USB video capture device. Video is
captured by ffmpeg (AVFoundation on macOS, DirectShow on Windows). Audio is
captured and played by miniaudio with the input and output chosen
independently. Nothing is recorded and there is no volume control.

## Build, test and package

Use the Makefile; `make help` lists everything.

```sh
make build         # bin/my-capture for this computer
make run           # build and start
make test          # unit tests; make test-race, make test-e2e also exist
make check         # fmt-check, vet and test
make dist-macos    # dist/my-capture-VERSION-macos-universal.zip (Mac only)
make dist-windows  # dist/my-capture-VERSION-windows-amd64.zip (needs MinGW-w64)
make dist          # both
make ffmpeg-macos  # build only the bundled ffmpeg (also ffmpeg-windows, clean-ffmpeg)
make icons         # redraw the icon and rebuild icns and the Windows resource (Mac only)
make clean
```

Variables: `VERSION` (artefact name and bundle version, default git describe),
`BUNDLE_FFMPEG` (default 1: bundle a small LGPL ffmpeg built by
`packaging/ffmpeg/build.sh`), `FFMPEG_BIN` and `FFMPEG_EXE` (use this ffmpeg instead), `MACOS_ARCHS`
(default `arm64 amd64`), `MACOS_MIN` (default `12.0`), `SIGN_IDENTITY` (default
`-`, ad-hoc), `MINGW_CC` (default `x86_64-w64-mingw32-gcc`), `GO`.

Needs Go 1.26+, a C compiler (cgo), make, and nasm for the bundled ffmpeg
(`brew install nasm`). The downloads include their own ffmpeg, so users install
nothing; only running from source needs ffmpeg 5.1+ on the PATH (`brew install
ffmpeg`, `winget install Gyan.FFmpeg`).
`make dist-windows` on a Mac also needs `brew install mingw-w64`. Without make:
`go build -o bin/my-capture .`, adding `-ldflags "-H=windowsgui"` on Windows.

## Releasing

GitHub Releases are published by the Release workflow
(`.github/workflows/release.yml`). It runs automatically on every push to
`master` (always a minor bump, published at once; `[skip ci]` in the commit
message skips it) and can be started by hand from Actions, Release, Run
workflow, or `gh workflow run release.yml -f bump=patch`. Hand-run inputs:
`bump` (`minor` by default, or `patch` or `major`) and `draft`. It finds the highest
`vMAJOR.MINOR.PATCH` tag and increments it (minor: v0.3.1 to v0.4.0; the first
release is v0.1.0), runs `make check`, builds the macOS zip (macOS runner) and
Windows zip (Windows runner, MSYS2), then creates the tag and release with
generated notes (categories in `.github/release.yml`, by pull request label),
both zips, `SHA256SUMS.txt` and build provenance attestations. The version is
built in: `my-capture -version`. Verify a download with
`gh attestation verify FILE --repo steveohara/my-capture`. Never create tags or
releases by hand or from a script; use the workflow. Pushing to master therefore
releases: do not push work in progress to master.

## Usage

```
my-capture [flags]

Flags:
  -list-devices
        print the video devices, audio inputs and audio outputs, then exit
  -version
        print the version and exit
```

Use `my-capture -list-devices` first when diagnosing: it shows what the program
can see without opening the window. Double click the picture, or press `F`, `F11` or
`Cmd/Ctrl+F`, for full screen; `Escape` leaves it. `C` (or View, Hide Controls / Show Controls, `Cmd/Ctrl+Shift+C`) hides or shows the controls strip in
windowed mode. `F1` or the Help menu opens the
built-in help, and About (application menu on macOS, Help menu on Windows) shows
the version and a Copy details button for bug reports. `Cmd/Ctrl+R` refreshes
the device lists. The Aspect picker (or the `A` key,
which also works in full screen) sets how the picture fills the window: fit,
stretch to window, fill with cropping, or a forced 16:9, 16:10, 4:3, 21:9 or
1:1.

## Environment variables

| Variable | Meaning | Default |
|---|---|---|
| `MYCAPTURE_FFMPEG` | Absolute path of the ffmpeg executable used to capture video. When unset, my-capture uses the one next to its own executable (the downloads include one), then one on the `PATH`, then `/opt/homebrew/bin` and `/usr/local/bin`. | unset |
| `MYCAPTURE_AUDIO_DELAY_MS` | Initial audio delay in milliseconds, 20 to 1000. The audio is buffered for this long before it is played. Raise it if the sound arrives before the picture, or if the audio crackles. It can also be changed with the "Audio delay" slider, but slider changes are not remembered; set this variable to keep a value. | `60` |
| `MYCAPTURE_DEBUG` | Set to `1`, `true`, `yes` or `on` to print everything ffmpeg reports on its error stream. Use it to find out why a capture device will not open. | unset |
| `MYCAPTURE_NO_RELAUNCH` | macOS only. A program started from a terminal or an IDE has no application bundle, so macOS would label it with its file name in the Dock. To avoid that, my-capture restarts itself once inside a temporary `My Capture.app` in the user cache folder. Set to `1`, `true`, `yes` or `on` to turn that off. It is never done when the program is already in an app bundle, or under a debugger. | unset (restart inside a bundle) |

## Troubleshooting

- "ffmpeg was not found": install ffmpeg or set `MYCAPTURE_FFMPEG`.
- Video does not start: run with `MYCAPTURE_DEBUG=1`, then try another mode or
  `Device default`.
- No devices on macOS: check Privacy & Security, Camera and Microphone.
- Sound ahead of or behind the picture: move the Audio delay slider.
- Bluetooth headphones missing: connect them; the lists update every few
  seconds, or press Refresh. A device that was in use and is reconnected is
  chosen again automatically.

## Code map

- `main.go`: flags, help text, `-list-devices`.
- `internal/config`: environment variables and their help text.
- `internal/video`: ffmpeg discovery, device and mode parsing, frame capture.
- `internal/audio`: miniaudio engine and the ring buffer between the devices.
- `internal/ui`: the Fyne window, `VideoView`, the menu bar and About (`menu.go`),
  and the built-in help text (`help.md`, embedded in the program).
- `assets`, `tools/genicon`, `rsrc_windows_amd64.syso`: the icon (drawn in code;
  change it, then `make icons`) and its macOS and Windows forms.
  `internal/ui/platform_darwin.go` restarts an unbundled macOS run inside a
  temporary app bundle (skipped under a debugger or with `MYCAPTURE_NO_RELAUNCH`)
  so the menu bar, Dock tooltip and icon are right.
- `packaging/ffmpeg/build.sh`: builds the small LGPL ffmpeg that goes in the
  downloads (version and checksum at the top; change the components there).
- `Makefile`, `assets/macos`, `packaging/macos`: build, test and release targets,
  the macOS Info.plist template and the signing entitlements.

When a flag, command, control, shortcut or environment variable changes, update
the `-help` text (`main.go` and `internal/config`), the in-app help
(`internal/ui/help.md`), `README.md`, `docs/architecture.md` and this file
together. The same applies to the Makefile: change its targets or variables
and `make help`, `README.md` and this file together.
