# Live debugging on a real iPad

My Capture for iPad needs its only USB-C port for the video capture card, so
the usual Xcode debugging cable often cannot be plugged in at the same time.
This note covers the two ways around that: debugging over Wi-Fi (no cable at
all once set up), and debugging over a USB-C hub (when a wired connection is
required or Wi-Fi is not available).

Wireless debugging is the one to use day to day. The hub is a fallback.

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
   the device at least once, so the first run (see "Build and run" in
   README.md) must happen over the cable regardless.
5. In Xcode, open **Window, Devices and Simulators** (Shift+Cmd+2) and select
   the iPad in the left-hand list.
6. Tick **Connect via network**. A small network icon appears next to the
   device name once pairing has finished, which can take a minute.
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

Once paired, **Window, Devices and Simulators** also has a console pane for
the device, and the Mac's own Console.app lists the iPad under Devices in its
sidebar and can stream and filter its log, including from a build that was
not started from Xcode (for example one left running from a previous
session).

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
  already bandwidth-sensitive; see the Windows stuttering note in
  `HANDOFF.md`), and a hub that shares bandwidth unevenly between the card
  and the debug cable can reintroduce that same kind of stutter on iPadOS.
  If the hub's listing does not say how many independent USB 3.x controllers
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
