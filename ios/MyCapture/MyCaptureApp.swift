// MyCaptureApp.swift: the entry point of My Capture for iPad.
//
// WHAT IT IS
// The iPad version of My Capture shows the picture and plays the sound of a
// USB video capture device plugged into the iPad's USB-C port. Nothing is
// recorded, and there is no volume control: use the iPad's own volume buttons.
//
// HOW IT FITS TOGETHER
//   - VideoController finds the capture devices with AVFoundation and runs
//     the AVCaptureSession whose picture the preview layer draws directly on
//     the screen, so no frame is ever copied by the program. This is the
//     lowest latency path iPadOS offers and replaces the ffmpeg process that
//     the macOS and Windows versions use.
//   - AudioController runs an AVAudioEngine from the chosen audio input
//     through a delay unit to the system's current output.
//   - ContentView is the window that joins them.
//
// This version is a separate Swift program, not the Go program: iPadOS does
// not allow an app to start another program such as ffmpeg.

import SwiftUI

/// The application. It creates the two controllers once and shares them with
/// every view through the environment.
@main
struct MyCaptureApp: App {
    /// Owns the capture session and the list of video devices.
    @StateObject private var video = VideoController()

    /// Owns the audio engine and the list of audio inputs.
    @StateObject private var audio = AudioController()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(video)
                .environmentObject(audio)
                .preferredColorScheme(.dark)
        }
    }
}
