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
final class PreviewUIView: UIView {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }

    /// The backing layer, typed.
    var previewLayer: AVCaptureVideoPreviewLayer {
        layer as! AVCaptureVideoPreviewLayer
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
        view.previewLayer.session = session
        return view
    }

    func updateUIView(_ view: PreviewUIView, context: Context) {
        view.previewLayer.videoGravity = gravity

        // A capture card's picture is already the right way up, so it must not
        // turn when the iPad does, which is what the layer does by default.
        if let connection = view.previewLayer.connection {
            if connection.isVideoRotationAngleSupported(0) {
                connection.videoRotationAngle = 0
            }
            connection.automaticallyAdjustsVideoMirroring = false
            connection.isVideoMirrored = false
        }
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
