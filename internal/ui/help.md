# My Capture help

My Capture shows the picture and plays the sound of a USB video capture device. Nothing is recorded. There is no volume control, because the external device and your computer's own volume control already do that.

## Getting started

1. Connect the capture device.
2. Choose it in **Video device**. The picture starts at once.
3. Choose the matching **Audio input**. It is picked for you when its name matches the video device.
4. Choose where the sound plays in **Audio output**. This can be the speakers, wired or Bluetooth headphones, or any other output. **System default** follows the computer's own choice.

Choosing **(none)** for the video device or the audio input stops it.

## The controls

- **Video device**: the capture device to show.
- **Mode**: the resolution and frame rate. The widest mode up to 1920 pixels (normally 1920x1080) is chosen first. **Device default** lets the device decide.
- **Audio input**: the sound source, usually named like the capture device.
- **Audio output**: where the sound is played.
- **Aspect**: how the picture fills the window. **Fit** keeps the shape of the source and leaves black bars. **Stretch to window** fills the whole area and distorts the shape. **Fill window** keeps the shape and crops the edges. **16:9**, **16:10**, **4:3**, **21:9** and **1:1** force that shape and then fit it in the window.
- **Audio delay**: delays the sound so it lines up with the picture. Raise it if the sound arrives before the picture, or if the sound crackles.
- **Refresh**: looks for devices straight away.
- **Hide controls**: hides this strip so the picture fills the whole window. Bring it back with **View, Show Controls** or the **C** key.
- **Full screen**: shows the picture alone on the monitor the window is on.

## Keyboard and mouse

- **Cmd+F** on macOS, **Ctrl+F** on Windows, **F** or **F11**: enter or leave full screen.
- **Double click** the picture: enter or leave full screen.
- **Escape**: leave full screen.
- **C** or **Cmd+Shift+C** on macOS, **Ctrl+Shift+C** on Windows: hide or show the controls. In full screen they are always hidden, and come back as they were when you leave.
- **A**: step through the aspect modes. This works in full screen, where the controls are hidden.
- **Cmd+R** on macOS, **Ctrl+R** on Windows: refresh the device lists.
- **F1**: show this help.

## Windows and monitors

Resize the window by dragging its edges. Drag it by its title bar to another monitor. To use full screen on another monitor, move the window there first and then enter full screen.

## Plugging devices in and out

The device lists are checked every few seconds, so devices you plug in appear and devices you remove disappear without pressing anything. If the device in use is removed, that part stops and the status line says so. If it comes back, it is chosen again automatically. This also applies to a device from your last session that was not connected when the program started, such as Bluetooth headphones.

Your choices of device and aspect are remembered between sessions.

## Troubleshooting

- **"ffmpeg was not found"**: the picture is captured with ffmpeg 5.1 or later. Install it (macOS: brew install ffmpeg. Windows: winget install Gyan.FFmpeg), then press Refresh. You can also set MYCAPTURE_FFMPEG to its full path.
- **The picture does not start**: try another mode, or **Device default**. Start the program with MYCAPTURE_DEBUG=1 from a terminal to see what ffmpeg reports.
- **No devices on macOS**: allow camera and microphone access in System Settings, Privacy and Security.
- **The sound is ahead of or behind the picture**: move the **Audio delay** slider.
- **The sound crackles**: raise the audio delay.
- **A Bluetooth device is missing**: connect it. It appears within a few seconds, or press Refresh.
- **Reporting a problem**: choose **About** and press **Copy details**, then include that text. **Help, Report an Issue** opens the issue page, and **Help, My Capture on GitHub** opens the project's main page.

## Command line

Run my-capture -list-devices from a terminal to print the devices the program can see. Run my-capture -version to print the version, and my-capture -help for the command line help.
