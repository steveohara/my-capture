# Live debugging on a real iPad

My Capture for iPad needs its only USB-C port for the video capture card, so
the usual Xcode debugging cable often cannot be plugged in at the same time.
This note covers the two ways around that: debugging over Wi-Fi (no cable at
all once set up), and debugging over a USB-C hub (when a wired connection is
required or Wi-Fi is not available).

Wireless debugging is the one to use day to day. The hub is a fallback.

If Xcode has never been opened for this project before, or the Run button is
missing or greyed out, start with "First time setup" below before either
option.

## First time setup

This section is for anyone new to Xcode. It covers generating the project,
opening it correctly, and getting the Run button to work, before pairing or
debugging comes into it at all.

### Generate the Xcode project

`MyCapture.xcodeproj` is not kept in git; `ios/project.yml` is the source of
truth that [XcodeGen](https://github.com/yonaskolb/XcodeGen) reads to create
it. Run this every time `project.yml` changes, and the first time there is no
`.xcodeproj` yet:

```sh
cd ios
xcodegen generate
```

This writes `ios/MyCapture.xcodeproj`.

### Open the project, not a folder

Open `ios/MyCapture.xcodeproj` itself with **File, Open** (`Cmd+O`) in Xcode,
or `open MyCapture.xcodeproj` from the `ios` folder in a terminal. Do not open
the `ios` folder or the repository root as a folder: Xcode will show the
Swift files but without a proper iOS target, so the destination picker has
nothing to build against and the Run button stays unusable.

### Set a signing team

A free Apple ID was used during development; the project now signs with a
paid Apple Developer Program account for App Store submission. Either way
the team is set in `ios/project.yml` (`DEVELOPMENT_TEAM`), not chosen by hand
in Xcode, because of how the project is regenerated; see "Why the team
belongs in project.yml" below. Signing & Capabilities should already show a
team filled in as soon as the project is opened.

1. In the left sidebar, click the blue **MyCapture** project icon at the top.
2. Under **TARGETS**, select **MyCapture**.
3. Open the **Signing & Capabilities** tab and confirm a **Team** is shown.
   If it reads "None" the first time, see "Why the team belongs in
   project.yml" below rather than choosing one directly in this tab.

### Why the team belongs in project.yml, not in Xcode's UI

Choosing a team directly in Signing & Capabilities seems to work at first,
but `ios/MyCapture.xcodeproj` is a generated file: `xcodegen generate`
rebuilds it from `ios/project.yml` alone, discarding anything set only in the
`.xcodeproj` itself. This project's own tooling runs that command whenever a
new Swift file is added or `project.yml` changes, so a team chosen only in
Xcode's UI is silently lost the next time that happens, which then blocks
Run with a signing error until the team is chosen again. `project.yml`
already sets `DEVELOPMENT_TEAM: VGC7Z7SNU7` (the paid Apple Developer
Program account `sohara@pivotal-solutions.co.uk`), so this should not come
up normally.

To sign with a different Apple ID:

1. Sign into that Apple ID in Xcode, Settings, Accounts.
2. Find its team ID: run `security find-identity -v -p codesigning` in a
   terminal and read the ID in parentheses after the account's email, or in
   Xcode, Settings, Accounts, select the team and look at its details.
3. Change `DEVELOPMENT_TEAM` in `ios/project.yml` to that ID.
4. Run `xcodegen generate` again, and reopen the project if it was open.

### Pick the iPad as the run destination

Recent Xcode versions replaced the old **Window, Devices and Simulators**
panel with a separate **Device Hub** app, and moved the run destination into
a single pill-shaped control in the main window's toolbar (it reads "My Mac"
until a device or simulator is chosen instead).

1. Connect the iPad with a USB-C cable (capture card unplugged for now).
2. On the iPad, tap **Trust This Computer** and enter the passcode.
3. Wait for Xcode to finish preparing the device. Progress shows either in
   the destination pill's dropdown or in Device Hub (open it from **Xcode,
   Open Developer Tool, Device Hub**, or launch it on its own from Spotlight;
   it does not need Xcode running).
4. Click the destination pill in the toolbar and choose the iPad from the
   list.
5. If the pill instead reads something like "iPad Pro 11-inch (M4) (mismatched
   platform)", the project that is open does not target iOS for the currently
   selected scheme. Check that `ios/MyCapture.xcodeproj` was opened directly
   (see above) and that the scheme dropdown next to the destination pill says
   **MyCapture**. `project.yml` sets `platform: iOS` and
   `TARGETED_DEVICE_FAMILY: "2"` (iPad) for this target, so once the right
   project and scheme are selected the mismatch clears.

### Build and run

Press **Cmd+R**. If the Run button itself is not visible in a narrow or
partially off-screen window, `Cmd+R` still works, and **Product, Run** in the
menu bar is the same action.

The first run on a given iPad prompts for **Developer Mode** (Settings,
Privacy & Security, Developer Mode, then restart when asked). iOS and iPadOS
only offer that switch after Xcode has tried to run something on the device
at least once, which is why it will not appear in Settings beforehand no
matter how carefully you look.

Once the app runs on the device over the cable at least once, continue with
wireless pairing below so future sessions do not need the capture card's only
port taken by the debugging cable.

### Trusting the Developer App certificate

The first install of a development-signed build on a given iPad shows an
Xcode alert: "The application could not be launched because the Developer
App Certificate is not trusted." The build and install both succeeded; the
iPad just has not been told to trust that certificate yet. This applies to
both a free Apple ID and a paid Apple Developer Program account.

1. On the iPad, open **Settings**.
2. Go to **General, VPN & Device Management** (some iPadOS versions show
   **General, Device Management**, or **Profiles & Device Management**).
3. Under **Developer App**, tap the entry for your Apple ID.
4. Tap **Trust "[your Apple ID]"**, then confirm **Trust**.
5. Launch **MyCapture** from the iPad's home screen directly; a re-run from
   Xcode is not needed.

With a free Apple ID, provisioning profiles expire about every 7 days, so
this same alert, and the same fix, can reappear periodically; that is
expected, not a new fault. A paid Apple Developer Program account's
provisioning profiles last up to a year, so this should be rare now that
`project.yml` signs with one.

## Option A: wireless debugging (recommended)

Xcode can install, run, debug and stream logs from an iPad over Wi-Fi once the
Mac and the iPad have paired. Pairing itself needs one cable connection, but
that is done with the capture card unplugged and is not needed again
afterwards unless the iPad is unpaired, erased, or has a major iPadOS update.

### One-time pairing

1. Unplug the capture card from the iPad, if it is plugged in.
2. Connect the iPad to the Mac with a USB-C cable.
3. On the iPad, if asked, tap **Trust This Computer** and enter the passcode.
4. On the iPad, turn on Developer Mode if it is not already on: Settings,
   Privacy & Security, Developer Mode, then restart the iPad when asked. iOS
   and iPadOS only offer this switch after Xcode has tried to run something on
   the device at least once, so the first run (see "First time setup" above)
   must happen over the cable regardless.
5. Open **Device Hub**: **Xcode, Open Developer Tool, Device Hub**, or launch
   it directly without Xcode running. Select the iPad in the sidebar.
   (Older Xcode versions call this the **Window, Devices and Simulators**
   panel, Shift+Cmd+2, instead; the setting below is in the same place either
   way.)
6. Turn on **Connect via Network** (sometimes shown as a Wi-Fi or network
   toggle in the device's detail pane). A network icon appears next to the
   device name once pairing has finished, which can take a minute. If the
   control is greyed out, wait for Xcode to finish "preparing" the device
   first.
7. The iPad may ask to allow My Capture, or Xcode, to find devices on the
   local network: allow it. This is the iOS **Local Network** permission, and
   it is also listed under Settings, Privacy & Security, Local Network if it
   needs changing later.
8. Unplug the USB-C cable and plug the capture card back in.

### Every session after that

1. Make sure the Mac and the iPad are on the same Wi-Fi network. Wireless
   debugging does not cross a guest network with client isolation turned on,
   and does not work over cellular.
2. Open `MyCapture.xcodeproj` (generate it first with `xcodegen` in `ios/` if
   it does not exist) and choose the iPad from the run destination list. A
   paired device shows there with a network icon even without a cable
   plugged in.
3. Run as normal (Cmd+R). Breakpoints, the LLDB console, the variable viewer
   and the usual Xcode debugging all work over the network connection.
4. Installing the app is slower over Wi-Fi than over a cable, noticeably so
   for a large rebuild. A small SwiftUI app like this one is normally fine.

### Logs without a debug session

Once paired, Device Hub (or **Window, Devices and Simulators** on older
Xcode versions) also has a console pane for the device, and the Mac's own
Console.app lists the iPad under Devices in its sidebar and can stream and
filter its log, including from a build that was not started from Xcode (for
example one left running from a previous session).

### Command line alternative

`xcrun devicectl` can list, install to, and launch on a paired wireless
device without opening Xcode, which is useful for scripting a build-install
cycle:

```sh
xcrun devicectl list devices                                  # find the device's identifier
xcrun devicectl device install app --device <identifier> MyCapture.app
xcrun devicectl device process launch --device <identifier> io.github.steveohara.mycapture
```

`devicectl` does not attach a debugger (no breakpoints or LLDB); for that,
use Xcode itself as above.

## Option B: a USB-C hub (fallback)

Use this only when Wi-Fi debugging is unavailable (no shared network, a
locked-down network, or a first-time pairing problem) or when the slower
Wi-Fi install speed is a real obstacle, for example rebuilding often while
chasing a crash.

### What the hub needs

- **At least two more data ports** (USB-A or USB-C), so the capture card and
  the debugging cable to the Mac are both plugged into the hub rather than
  sharing the iPad's single port directly.
- **Independent USB 3.x data lanes to the downstream ports**, not a cheap
  hub that multiplexes every port through one USB 2.0 controller. The capture
  card is a USB Video Class device that needs real bandwidth (1080p60 is
  already bandwidth-sensitive: the same Guermok USB3 Video card stuttered
  over this at 1080p60 on Windows even in plain ffplay, though it has been
  fine on macOS and the iPad), and a hub that shares bandwidth unevenly
  between the card and the debug cable can reintroduce that same kind of
  stutter on iPadOS. If the hub's listing does not say how many independent
  USB 3.x controllers
  it has, assume it has one and treat the combination as a risk, not a
  certainty.
- **USB Power Delivery passthrough**, if the plan is to debug for long
  sessions. My Capture disables the iPad's auto-lock while running
  (`UIApplication.shared.isIdleTimerDisabled`), so the screen stays on and
  the battery drains faster than normal; a hub that passes through charging
  power avoids having to stop to charge.
- A **Lightning or USB-C cable** from the hub to the Mac, matching whichever
  port the hub exposes for it.

### Using it

1. Plug the hub into the iPad's USB-C port.
2. Plug the capture card into one hub port and the Mac debugging cable into
   another.
3. Trust the Mac on the iPad if asked, the same as a direct cable connection.
4. Run from Xcode as normal. The connection behaves like a direct cable: fast
   installs, full debugging, no network dependency.
5. Watch the picture for any new stutter or dropped frames compared with the
   card plugged in alone. If it appears, the hub is the first thing to
   suspect, and wireless debugging (Option A) avoids the question entirely
   by leaving the capture card as the only thing in the USB-C port.

### If only one spare port is available

A single-port adapter (not a hub) cannot carry both the capture card and a
debug cable at once. In that case, pair for wireless debugging once (Option
A) using that single port, then rely on Wi-Fi from then on, and treat the
port as dedicated to the capture card the rest of the time.
