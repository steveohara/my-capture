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

Items 1 to 4 and 6 below are done. What remains needs a Mac with Xcode, a free
Apple ID signed in, and the physical iPad with the Guermok capture card; see
item 5.

1. ~~**PreviewView rotation fix.**~~ Done: `PreviewUIView.watch(session:)`
   observes `AVCaptureSession.didStartRunningNotification` and reapplies
   `connectionSettings()` (rotation, mirroring) whenever the session gets a
   fresh connection; `updateUIView` also calls it so a later aspect change
   does not lose the setting.
2. ~~**`ContentView.swift`**~~ Done, with the layout described below.
3. ~~**`HelpView.swift` and `AboutView.swift`**~~ Done, adapted from
   `internal/ui/help.md` with shortcuts and environment variables left out.
4. ~~**README.md**~~ Done: a "My Capture for iPad" section, the project
   layout table, and the intro line now mention iPadOS.
   `docs/architecture.md` has a matching section.
5. **Build and test on the Mac**: `cd ios && xcodegen generate`, open
   `MyCapture.xcodeproj`, set the team, run on the iPad with the Guermok card.
   `xcodegen generate` and an unsigned simulator build
   (`xcodebuild ... -destination 'platform=iOS Simulator,name=iPad (A16)'
   CODE_SIGNING_ALLOWED=NO`) both succeed with no warnings as of this note,
   and the control panel, Help and About sheets, and the microphone
   permission prompt all render correctly in the simulator. None of that
   exercises real hardware. Expected checks still needed on the device:
   - the mode list contains 1920x1080 @ 60 fps and it is chosen first;
   - the picture appears and does not rotate with the iPad;
   - the card's audio input is chosen automatically (USB audio port);
   - the delay slider changes the audio delay without crashing;
   - unplugging the card shows "The video device was unplugged." and plugging
     it back restarts it.
6. ~~**App icon**~~ Done: `Assets.xcassets/AppIcon.appiconset/icon-1024.png`,
   a 1024x1024 opaque PNG rendered with the same shapes as `tools/genicon`
   (full bleed, no macOS transparent margin), named in `Contents.json`.
7. **Possible latency tuning if the user reports delay on iPad**: the audio
   I/O buffer is requested at 5 ms but iPadOS may choose more; the delay
   slider compensates for video latency, which is low because the preview layer
   draws frames directly.
