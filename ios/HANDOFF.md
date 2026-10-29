# iPad port: handoff

This note lets a new Claude Code session, on a Mac with Xcode, continue the
iPad version of My Capture. Delete it when the port is finished.

## Goal

A native Swift/SwiftUI iPad app (iPadOS 17 or later) that shows the picture and
plays the sound of a USB-C video capture device, like the macOS and Windows Go
versions. It is a separate program: iPadOS does not allow starting ffmpeg, so
the Go capture code cannot be reused. The design is reused: device and mode
pickers, aspect modes, audio delay (20 to 1000 ms, default 60), Help and About.

Decisions already made with the user:
- Native Swift/SwiftUI, not Go with Fyne.
- Free Apple ID signing, so the app is installed on the user's own iPad.
- First version: video from the capture card, audio in/out with delay, Help
  and About. Hotplug refresh is handled by the controllers but was not listed
  as required.

## Repository rules (see AGENTS.md and .kiro/steering)

- Never commit, amend, push or create PRs unless the user asks.
- Documentation is mandatory: a header comment on every file, and a comment on
  every type and non-trivial member. Never use the em-dash character.
- Output style: return only final code, do not narrate or summarise changes.
- README.md must stay consistent with the code.

## Background: why the Windows app lagged

Unrelated to the iPad work, but unresolved. On Windows the 1080p60 yuyv422
stream from the Guermok USB3 Video card stuttered even in plain ffplay, and
MJPEG was worse. ffmpeg and the Go capture code kept up at 60 fps in
measurements. The user's Mac was fine. Suspect USB bandwidth or the driver; the
user was going to check the USB link speed. No Go code was changed (an earlier
experiment was reverted).

## State of ios/

Written, never compiled:
- `project.yml`: XcodeGen description (iPad only, iOS 17, camera and
  microphone usage strings, bundle id `io.github.steveohara.mycapture`).
- `MyCapture/MyCaptureApp.swift`: entry point, creates the two controllers.
- `MyCapture/AspectMode.swift`: the eight aspect modes.
- `MyCapture/VideoController.swift`: AVCaptureDevice discovery (external and
  built-in cameras), mode list, AVCaptureSession on a serial queue,
  connect/disconnect notifications, remembered device and mode.
- `MyCapture/AudioController.swift`: AVAudioSession (playAndRecord,
  measurement), input selection, AVAudioEngine input -> AVAudioUnitDelay ->
  main mixer, route and interruption handling.
- `MyCapture/PreviewView.swift`: `PreviewView` (AVCaptureVideoPreviewLayer) and
  `RoutePicker` (AVRoutePickerView, the only way to choose the output on iPadOS).
- `MyCapture/Assets.xcassets`: AppIcon slot is empty (the repo icon is only
  256x256; a 1024x1024 image without transparency is needed).
- `.gitignore` already ignores `ios/*.xcodeproj` and `xcuserdata/`.

## Still to do

1. **PreviewView rotation fix (rejected edit, not applied).** The preview layer
   has no connection until a device is attached, so `updateUIView` cannot set
   `videoRotationAngle = 0`, `automaticallyAdjustsVideoMirroring = false` and
   `isVideoMirrored = false` reliably. Add to `PreviewUIView` a `watch(session:)`
   that sets `previewLayer.session` and observes
   `AVCaptureSession.didStartRunningNotification`, plus a
   `connectionSettings()` method that applies those three settings; call
   `watch` from `makeUIView` and `connectionSettings()` from `updateUIView`.
2. **`ContentView.swift`** (missing; the app does not build without it):
   - Black background; `PreviewView` filling the window, with
     `.aspectRatio(mode.ratio, contentMode: .fit)` when `AspectMode.ratio` is
     set, and `mode.gravity` otherwise. Aspect choice kept with `@AppStorage`.
   - A top control panel (material background): Menu pickers for video device
     and mode (`VideoController`), audio input (`AudioController.inputs`),
     `RoutePicker` plus `outputName` for the output, a delay slider
     (`AudioController.delayRange`) that calls `setDelay` only when editing
     ends, an aspect menu, and Help and About buttons.
   - Tap the picture to hide or show the panel; hide status bar and home
     indicator when hidden (`.statusBarHidden`, `.persistentSystemOverlays`).
   - Show `video.message` and `audio.message` in a banner.
   - `.onAppear`: `video.start()`, `audio.start()`,
     `UIApplication.shared.isIdleTimerDisabled = true`; on `scenePhase`
     `.active`: `video.resume()`, `audio.resume()`.
3. **`HelpView.swift` and `AboutView.swift`**: sheets. Help text adapted from
   `internal/ui/help.md` (leave out keyboard shortcuts and environment
   variables, which do not exist on iPad; mention the system route picker and
   the iPad volume buttons). About shows version from
   `CFBundleShortVersionString`, the repo link `https://github.com/steveohara/my-capture`,
   and the issues link.
4. **README.md**: add a "My Capture for iPad" section (requirements: Mac,
   Xcode, `brew install xcodegen`, `xcodegen` in `ios/`, set the signing team,
   run on the iPad; iPadOS 17 or later; USB-C iPad; the card's video appears as
   an external camera) and update the intro line that says it targets macOS and
   Windows. Optionally note it in `docs/architecture.md`.
5. **Build and test on the Mac**: `cd ios && xcodegen`, open
   `MyCapture.xcodeproj`, set the team, run on the iPad with the Guermok card.
   Expected checks:
   - the mode list contains 1920x1080 @ 60 fps and it is chosen first;
   - the picture appears and does not rotate with the iPad;
   - the card's audio input is chosen automatically (USB audio port);
   - the delay slider changes the audio delay without crashing;
   - unplugging the card shows "The video device was unplugged." and plugging
     it back restarts it.
   Likely compile issues to watch for, since nothing has been built:
   `CMFormatDescriptionGetMediaSubType` switch patterns in `codecName`,
   `AVAudioApplication.requestRecordPermission` availability, and Swift 5 mode
   concurrency warnings from touching `session` inside `sessionQueue` blocks.
6. **App icon**: render a 1024x1024 opaque PNG (for example with
   `tools/genicon`) into `Assets.xcassets/AppIcon.appiconset` and name it in
   `Contents.json`.
7. **Possible latency tuning if the user reports delay on iPad**: the audio
   I/O buffer is requested at 5 ms but iPadOS may choose more; the delay
   slider compensates for video latency, which is low because the preview layer
   draws frames directly.
