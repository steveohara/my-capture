// PreviewView.swift: SwiftUI wrappers for the two UIKit views the app needs.
//
// PreviewView shows the capture session. RoutePicker shows the system button
// for choosing the audio output.

import AVFoundation
import AVKit
import SwiftUI
import UIKit

// -----------------------------------------------------------------------
// Video preview
// -----------------------------------------------------------------------

/// A UIView whose backing layer is an AVCaptureVideoPreviewLayer, so the
/// session's frames are drawn straight onto the screen by the system.
///
/// The layer has no connection until a device is attached to the session, so
/// the rotation and mirroring settings cannot be applied just once in
/// makeUIView: they must be reapplied whenever the session (re)starts and
/// gets a fresh connection. watch(session:) observes that moment.
final class PreviewUIView: UIView {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }

    /// The backing layer, typed.
    var previewLayer: AVCaptureVideoPreviewLayer {
        layer as! AVCaptureVideoPreviewLayer
    }

    /// The didStartRunning observer, kept so it is not released, and removed
    /// when a new session is watched.
    private var startObserver: NSObjectProtocol?

    /// Points the layer at session and arranges for connectionSettings() to
    /// run every time that session starts running, which is when its
    /// connection to the preview layer is (re)created. Safe to call again
    /// with the same or a different session.
    func watch(session: AVCaptureSession) {
        if let startObserver {
            NotificationCenter.default.removeObserver(startObserver)
        }
        previewLayer.session = session
        connectionSettings() // in case a connection already exists
        startObserver = NotificationCenter.default.addObserver(
            forName: AVCaptureSession.didStartRunningNotification,
            object: session, queue: .main) { [weak self] _ in
            self?.connectionSettings()
        }
    }

    /// Applies the settings that make a capture card's picture appear the
    /// right way up and unmirrored, regardless of how the iPad is held. Does
    /// nothing if the layer has no connection yet.
    func connectionSettings() {
        guard let connection = previewLayer.connection else { return }
        if connection.isVideoRotationAngleSupported(0) {
            connection.videoRotationAngle = 0
        }
        connection.automaticallyAdjustsVideoMirroring = false
        connection.isVideoMirrored = false
    }
}

/// Shows the picture of an AVCaptureSession, scaled by gravity.
struct PreviewView: UIViewRepresentable {
    /// The session to show.
    let session: AVCaptureSession

    /// How the picture is scaled inside the view.
    let gravity: AVLayerVideoGravity

    func makeUIView(context: Context) -> PreviewUIView {
        let view = PreviewUIView()
        view.backgroundColor = .black
        view.watch(session: session)
        return view
    }

    func updateUIView(_ view: PreviewUIView, context: Context) {
        view.previewLayer.videoGravity = gravity
        view.connectionSettings()
    }
}

// -----------------------------------------------------------------------
// Audio output picker
// -----------------------------------------------------------------------

/// The system's audio route button. Tapping it lists the speakers, Bluetooth
/// headphones, AirPlay devices and USB outputs. iPadOS does not let an app
/// choose the output itself, so this is the way to do it.
struct RoutePicker: UIViewRepresentable {
    func makeUIView(context: Context) -> AVRoutePickerView {
        let view = AVRoutePickerView()
        view.tintColor = .white
        view.activeTintColor = .systemBlue
        view.prioritizesVideoDevices = false
        return view
    }

    func updateUIView(_ view: AVRoutePickerView, context: Context) {}
}
