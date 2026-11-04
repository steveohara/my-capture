# Screenshots for fastlane deliver

This folder is empty until real screenshots are added. `fastlane deliver`
(see `ios/fastlane/Fastfile`, lane `screenshots` or `upload_assets`) uploads
whatever PNG or JPEG files are placed in `en-GB/` here; nothing else in this
repository generates them. `en-GB` because that is My Capture's primary
language in App Store Connect; a locale deliver has no files for is left
empty, which is also why there is no `en-US` folder here even though that
is a common default elsewhere.

## Required sizes

My Capture for iPad is iPad only (`project.yml`'s `TARGETED_DEVICE_FAMILY:
"2"`), so only iPad display sizes are needed. As of iOS 17/18, App Store
Connect asks for screenshots from at least one 13-inch class iPad and one
11-inch class iPad simulator or device:

| Device class | Resolution (portrait) | Resolution (landscape) |
|---|---|---|
| 13-inch iPad (e.g. iPad Pro 13-inch M4) | 2064 x 2752 | 2752 x 2064 |
| 11-inch iPad (e.g. iPad Pro 11-inch M4) | 1668 x 2388 | 2388 x 1668 |

Check the current requirement in App Store Connect itself (App Store tab,
the version, Media Manager) before uploading, since Apple periodically
changes which sizes are mandatory for new submissions. My Capture's control
panel is designed for landscape use, so landscape screenshots are the
natural choice, but portrait is also supported by the app (see
`ContentView.swift`'s `twoRowControls` fallback) if portrait shots are
wanted too.

## Naming

`deliver` matches files to a device size by their pixel dimensions, not by
file name, but a descriptive name still helps when there are several:

```
en-GB/iPad Pro 13-inch (M4)-01-picture.png
en-GB/iPad Pro 13-inch (M4)-02-controls.png
en-GB/iPad Pro 11-inch (M4)-01-picture.png
```

## Taking them

### From a real iPad (preferred, since it shows a real capture card)

1. Plug in the capture card and get the app showing a picture.
2. Press the Side Button and Volume Up together to take a screenshot, or use
   Xcode's own "Screenshot" toolbar action while running from Xcode
   (Debug, View Debugging, or the camera icon in the device toolbar).
3. AirDrop or cable-transfer the screenshot PNGs from Photos into this
   folder, renamed to say what they show.

### From the iPad Simulator (no capture card, picture stays black)

Useful for the control panel, Help and About screenshots, where no real
picture is needed:

```sh
xcrun simctl list devices                                      # find the simulator's name or UDID
xcrun simctl boot "iPad Pro 13-inch (M4)"                       # if not already booted
xcrun simctl io "iPad Pro 13-inch (M4)" screenshot \
    "ios/fastlane/screenshots/en-GB/iPad Pro 13-inch (M4)-controls.png"
```

Run the app in that simulator first (`xcodebuild ... -destination
'platform=iOS Simulator,name=iPad Pro 13-inch (M4)'`, or Xcode's own Run
with that simulator chosen) so there is something on screen to capture.

## Uploading

```sh
cd ios
fastlane screenshots       # screenshots only
fastlane upload_assets     # screenshots and metadata together
```
