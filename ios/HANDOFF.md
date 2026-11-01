# iPad port: handoff

This note lets a new agent session, on a Mac with Xcode, continue the iPad
version of My Capture. The port itself (goal below) is finished and has been
tested on real hardware. The note now also tracks the App Store submission
that followed it, in "App Store submission" near the end. Delete it once the
app has actually been submitted and there is nothing left open here.

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

Items 1 to 6 are done, including the Mac and device testing in item 5 below,
which has now happened over a full session with real hardware (an iPad Pro
11-inch and a Guermok USB3 Video capture card). What is left is mostly App
Store submission, which needs a paid Apple Developer Program account the user
has enrolled in and is waiting on Apple to approve; see "App Store
submission" below for what is ready now and what is blocked on that.

1. ~~**PreviewView rotation fix.**~~ Superseded: the original fixed,
   always-unrotated connection setting this item described was replaced in
   this session by a user-facing manual Rotate control (`RotationMode.swift`,
   `PreviewUIView.rotationAngle`), because an automatic, orientation-tracking
   rotation (tried first, then reverted) was wrong for a capture card: the
   card's picture has no relationship to how the iPad is held, so letting the
   user turn it manually in 90 degree steps is the correct design, not a fix
   to the original approach.
2. ~~**`ContentView.swift`**~~ Done, with the layout described below, plus
   this session's additions: a two-row fallback for the control panel at
   portrait width (`ViewThatFits`, see `oneRowControls`/`twoRowControls`), a
   Rotate menu, and `(none)` for the audio input to match the video device
   picker.
3. ~~**`HelpView.swift` and `AboutView.swift`**~~ Done, adapted from
   `internal/ui/help.md` with shortcuts and environment variables left out,
   and updated this session to mention Rotate and audio `(none)`.
4. ~~**README.md**~~ Done and kept current this session alongside
   `docs/architecture.md` and this file.
5. ~~**Build and test on the Mac**~~ Done. `xcodegen generate` and an
   unsigned simulator build both still succeed with no warnings. On the real
   iPad with the Guermok card: the mode list contains 1920x1080 at 60 fps and
   it is chosen first; the picture appears and, with the manual Rotate
   control, can be turned to stay upright; the card's audio input is chosen
   automatically; the delay slider works; unplugging and replugging the card
   behaves as expected. The built-in back camera was also tried (useful for
   testing without the card, per the user) and surfaced real bugs, now fixed:
   see "Bugs found and fixed this session" below.
6. ~~**App icon**~~ Done.
7. **Possible latency tuning if the user reports delay on iPad**: unchanged
   from the original note, the audio I/O buffer is requested at 5 ms but
   iPadOS may choose more; not yet reported as an issue.

## Bugs found and fixed this session

For whoever picks this up next, these were real, user-reported bugs found
through live testing on the device, not just read from the code:

- **Developer App Certificate not trusted** on first install with a free
  Apple ID: expected, not a bug; documented in `ios/DEBUGGING.md`.
- **Signing team silently lost** after `xcodegen generate` regenerated
  `MyCapture.xcodeproj`, because the team was only ever chosen in Xcode's UI,
  not in `project.yml`. Fixed by setting `DEVELOPMENT_TEAM` in `project.yml`
  itself; see "Why the team belongs in project.yml" in `ios/DEBUGGING.md`.
- **Control panel labels stacked one letter per line** at portrait width: a
  single `HStack` with five menus had no room to lay out horizontally.
  Fixed with `ViewThatFits` falling back to two rows.
- **Black picture with the back camera selected**: the format and session
  opened without error, but the preview connection never produced a visible
  frame in the configuration tried at the time; resolved itself after
  further runs, suspected to be a transient AVFoundation session state
  rather than a code bug, since the same code path now works reliably.
- **Picture went black and restarted on some aspect changes**: `ContentView.
  picture` branched between two different view trees (`if let ratio = ...
  { preview.aspectRatio(...) } else { preview }`), which gave `PreviewView`
  two different identities and made SwiftUI tear down and recreate the
  underlying `AVCaptureVideoPreviewLayer`'s connection whenever aspectMode
  crossed between a fixed-ratio mode and Fit, Stretch or Fill. Fixed by using
  `aspectRatio`'s own optional parameter instead of branching, so the view
  has one identity across every aspectMode.
- **Fit, Stretch and Fill looked wrong once rotated** (filling only one
  dimension): `videoGravity`'s own aspect math, and the outer SwiftUI
  `aspectRatio` modifier for the fixed-ratio modes, both work out their fit
  against the picture's un-rotated width and height, not against how it
  looks once `videoRotationAngle` turns it, which swaps wide and tall at 90
  and 270 degrees. Fixed in two places: `AspectMode.ratio(rotatedBy:)` swaps
  the requested ratio for the fixed-shape modes, and
  `PreviewUIView.layoutSubviews` swaps the preview layer's own `bounds` for
  Fit and Fill, with `clipsToBounds = true` so the swapped (often larger)
  bounds never spill past the space actually given to the view.
- **No way to stop the microphone from the app**: the video device picker
  already had `(none)`, audio did not. Added `AudioController.stopInput()`
  and a matching `(none)` entry in the audio input menu, with
  `explicitlyStopped` so a route change or returning from the background does
  not silently turn the microphone back on after the user turned it off.
- **AVAudioSession Hang Risk** (Xcode's Thread Performance Checker):
  `setActive(true)` in `configure()` and `resume()` ran on the main thread.
  Both now run that call on a background queue, then hop back to main for
  everything that touches a `@Published` property or the engine.
- A **persistent ⌘⇧ keyboard-shortcut overlay** turned out to be unrelated to
  the app: no physical keyboard was attached, it appeared system-wide in
  every app, and console logs pointed at the `PointerUI` subsystem and a
  "tracking window controller" failing to resolve a keyboard scene,
  consistent with a connected pointer accessory (mouse, trackpad, or a hub
  presenting a stray HID interface) rather than Sticky Keys or anything in
  `MyCapture`. Not something this app's code can cause or fix.

## App Store submission

The user wants to publish this to the App Store and asked for the submission
material (description, keywords, privacy policy) and the development
documentation to both be accurate and complete. Enrollment in the paid Apple
Developer Program (individual, $99/year) is underway but not yet approved;
several things are ready now and some are blocked on that approval.

Done already:
- `project.yml` sets `INFOPLIST_KEY_ITSAppUsesNonExemptEncryption: NO`, since
  the app makes no network calls of its own (the only network use is opening
  ordinary HTTPS links from About in the system browser, which is exempt).
  This avoids the manual export compliance question on every future upload.
- Draft App Store Connect metadata (name, subtitle, promotional text,
  description, keywords) has been written and given to the user in chat, not
  yet committed anywhere in the repository since App Store Connect, not a
  file here, is where it is actually used.

Still blocked on Apple's enrollment approval:
- The free "Apple Development" signing in `project.yml`
  (`DEVELOPMENT_TEAM: DV8Q73MK77`, `CODE_SIGN_STYLE: Automatic`) cannot submit
  to the Store. Once the paid account is approved, it has its own, different
  team ID; `project.yml` will need that new team ID, or a second signing
  configuration, before an App Store build can be archived.
- App Store Connect app record, screenshots (device-specific sizes, taken on
  the iPad or iPad simulator, not reused from the macOS screenshot in
  `docs/images`), age rating questionnaire, and the privacy practices
  declaration, all live in App Store Connect itself, not in this repository.
- **Privacy policy URL**: still needs the user's decision on where to host
  it (a GitHub-rendered Markdown page in this repository is the simplest
  option, needing no extra infrastructure); draft text has not been written
  yet pending that decision.
