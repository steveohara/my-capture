// HelpView.swift: the Help sheet.
//
// The text is adapted from internal/ui/help.md, the help built into the
// macOS and Windows versions, leaving out the keyboard shortcuts and
// environment variables that only exist there, and mentioning the things
// that are specific to the iPad: the system route picker for audio output,
// and the iPad's own volume buttons.

import SwiftUI

/// Shows how to use the app, in a scrollable sheet with a Done button.
struct HelpView: View {
    /// Closes the sheet. Supplied by the sheet presentation.
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    section("Getting started") {
                        Text("""
                        Connect the capture device to the iPad's USB-C port. Choose it \
                        from the video device menu; the picture starts at once. Its \
                        matching audio input is chosen for you when its name matches the \
                        video device, so usually nothing else is needed for sound.
                        """)
                    }

                    section("The controls") {
                        bullet("Video device", "the capture device to show, or (none) to stop the picture.")
                        bullet("Mode", "the resolution and frame rate. The widest mode up to 1920 pixels (normally 1920x1080 at 60 fps) is chosen first.")
                        bullet("Audio input", "the sound source, usually named like the capture device.")
                        bullet("Audio output button", "opens the system's output picker, which lists the speakers, wired or Bluetooth headphones, AirPlay devices and any USB output. iPadOS only allows choosing the output this way, not from within the app.")
                        bullet("Aspect", "how the picture fills its area. Fit keeps the source shape and leaves black bars. Stretch to window fills the area and distorts the shape. Fill window keeps the shape and crops the edges. 16:9, 16:10, 4:3, 21:9 and 1:1 force that shape.")
                        bullet("Audio delay", "delays the sound so it lines up with the picture. Raise it if the sound arrives before the picture, or if it crackles.")
                    }

                    section("Volume") {
                        Text("""
                        There is no volume control in the app. Use the iPad's own volume \
                        buttons, or the control on whatever Bluetooth or AirPlay output is \
                        chosen.
                        """)
                    }

                    section("Showing and hiding the controls") {
                        Text("""
                        Tap the picture to hide or show the control panel. While hidden, \
                        the status bar and home indicator are also hidden, so the picture \
                        has the whole screen.
                        """)
                    }

                    section("Plugging devices in and out") {
                        Text("""
                        If the video device in use is unplugged, the picture stops and a \
                        banner says so. Plugging it back in starts it again automatically. \
                        Your choice of device and aspect mode is remembered between launches.
                        """)
                    }

                    section("Troubleshooting") {
                        bullet("No picture", "check that the capture card is seen as a camera: open Settings, Privacy & Security, Camera, and allow My Capture.")
                        bullet("No sound", "open Settings, Privacy & Security, Microphone, and allow My Capture. Then choose the right audio input in the app.")
                        bullet("The sound is ahead of or behind the picture", "move the Audio delay slider.")
                        bullet("The sound crackles", "raise the audio delay.")
                    }
                }
                .padding()
            }
            .navigationTitle("My Capture Help")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
    }

    /// A titled block of help text.
    @ViewBuilder
    private func section(_ title: String, @ViewBuilder content: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title).font(.headline)
            content()
        }
    }

    /// One bold term followed by its explanation, as a bullet line.
    private func bullet(_ term: String, _ text: String) -> some View {
        (Text("\(term): ").bold() + Text(text))
            .fixedSize(horizontal: false, vertical: true)
    }
}
