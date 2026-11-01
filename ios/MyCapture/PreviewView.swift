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
/// ROTATION
/// The layer has no connection until a device is attached to the session, so
/// the rotation setting cannot be applied just once in makeUIView: it must be
/// reapplied whenever the session (re)starts and gets a fresh connection (see
/// watch(session:)). Rotation itself is a user choice, not automatic: neither
/// a capture card's picture nor a built-in camera's has any fixed
/// relationship to how the iPad happens to be held or how large the app's own
/// window is (later iPadOS lets an app run in a window that does not fill the
/// screen), so ContentView offers a manual "Rotate" control and passes the
/// chosen angle in here as rotationAngle.
///
/// WHY THE PREVIEW LAYER'S OWN BOUNDS ARE SWAPPED, NOT JUST ROTATED
/// videoRotationAngle turns the picture inside the layer's existing bounds,
/// but videoGravity's own aspect fit math (used by Fit and Fill) is computed
/// against the connection's un-rotated width and height, not against how the
/// turned picture actually looks on screen. At a 90 or 270 degree rotation,
/// what the viewer sees as wide and tall is swapped from what the source
/// reports, so asking the layer to gravity-fit inside this view's own bounds
/// directly would fit the wrong way and either overflow one edge or leave
/// the other mostly empty, which is the bug this was written to fix.
/// layoutSubviews instead gives the preview layer bounds with width and
/// height already swapped for a 90 or 270 degree rotation, so videoGravity
/// fits or fills against a shape that already matches the rotated picture.
/// masksToBounds on this view's own layer, set in makeUIView, clips the
/// preview layer back to this view's real (unswapped) frame, since the
/// swapped bounds are otherwise a different, usually larger, rectangle than
/// the space actually available on screen.
final class PreviewUIView: UIView {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }

    /// The backing layer, typed.
    var previewLayer: AVCaptureVideoPreviewLayer {
        layer as! AVCaptureVideoPreviewLayer
    }

    /// The rotation angle in degrees the user has chosen, applied to the
    /// preview connection and to this view's own layout. Set by
    /// PreviewView.updateUIView from ContentView's rotation state.
    var rotationAngle: CGFloat = 0 {
        didSet {
            connectionSettings()
            setNeedsLayout()
        }
    }

    /// The didStartRunning observer, kept so it is not released, and removed
    /// when a new session is watched.
    private var startObserver: NSObjectProtocol?

    /// Points the layer at session and arranges for connectionSettings() to
    /// run every time that session starts running, which is when its
    /// connection to the preview layer is (re)created. Safe to call again
    /// with the same or a different session.
    func watch(session: AVCaptureSession) {
        clipsToBounds = true
        if let startObserver {
            NotificationCenter.default.removeObserver(startObserver)
        }
        previewLayer.session = session
        connectionSettings() // in case a connection already exists
        startObserver = NotificationCenter.default.addObserver(
            forName: AVCaptureSession.didStartRunningNotification,
            object: session, queue: .main) { [weak self] _ in
            self?.connectionSettings()
            self?.setNeedsLayout()
        }
    }

    /// Applies rotationAngle and turns mirroring off, regardless of device
    /// or how the iPad is held: mirroring is never wanted for either a
    /// capture card or a camera here, since the app shows exactly what the
    /// device sees. Does nothing if the layer has no connection yet.
    func connectionSettings() {
        guard let connection = previewLayer.connection else { return }
        if connection.isVideoRotationAngleSupported(rotationAngle) {
            connection.videoRotationAngle = rotationAngle
        }
        connection.automaticallyAdjustsVideoMirroring = false
        connection.isVideoMirrored = false
    }

    /// Gives the preview layer bounds whose width and height are already
    /// swapped for a 90 or 270 degree rotation, so videoGravity's own aspect
    /// math (used by Fit and Fill) works out against the shape the picture
    /// actually ends up once videoRotationAngle turns it, rather than
    /// against the connection's un-rotated width and height; see the type's
    /// documentation above. videoRotationAngle, not a transform here, does
    /// the actual visual turn, so only the bounds are touched: the layer
    /// still fills this view's frame in the normal, untransformed sense, it
    /// is simply told the swapped shape to fit the picture into. At 0 or 180
    /// degrees the shape is unchanged, since those angles do not swap which
    /// dimension is wide and which is tall.
    override func layoutSubviews() {
        super.layoutSubviews()
        let swapped = Int(rotationAngle.truncatingRemainder(dividingBy: 360)) % 180 != 0
        previewLayer.bounds = CGRect(origin: .zero,
                                     size: swapped ? CGSize(width: bounds.height, height: bounds.width)
                                                    : bounds.size)
        previewLayer.position = CGPoint(x: bounds.midX, y: bounds.midY)
    }
}

/// Shows the picture of an AVCaptureSession, scaled by gravity.
struct PreviewView: UIViewRepresentable {
    /// The session to show.
    let session: AVCaptureSession

    /// How the picture is scaled inside the view.
    let gravity: AVLayerVideoGravity

    /// The user's chosen rotation, in degrees. See RotationMode.
    let rotationAngle: CGFloat

    func makeUIView(context: Context) -> PreviewUIView {
        let view = PreviewUIView()
        view.backgroundColor = .black
        view.watch(session: session)
        return view
    }

    func updateUIView(_ view: PreviewUIView, context: Context) {
        view.previewLayer.videoGravity = gravity
        view.rotationAngle = rotationAngle
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
