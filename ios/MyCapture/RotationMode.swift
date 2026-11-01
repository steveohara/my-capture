// RotationMode.swift: the user's manual choice of picture rotation.
//
// WHY MANUAL AND NOT AUTOMATIC?
// Neither a capture card's picture nor a built-in camera's has any fixed
// relationship to how the iPad happens to be held, or to the shape of the
// app's own window on iPadOS versions that let an app run in a window
// smaller than the screen. Rather than guess, the user picks the rotation
// that makes the picture upright, the same way the aspect mode is a manual
// choice rather than a guess from the source's own shape.

import CoreGraphics
enum RotationMode: Int, CaseIterable, Identifiable {
    case none = 0
    case quarter = 90
    case half = 180
    case threeQuarter = 270

    var id: Int { rawValue }

    /// The text shown in the rotate menu.
    var title: String {
        switch self {
        case .none: return "No rotation"
        case .quarter: return "Rotate 90°"
        case .half: return "Rotate 180°"
        case .threeQuarter: return "Rotate 270°"
        }
    }

    /// The angle in degrees passed to AVCaptureConnection.videoRotationAngle.
    var angle: CGFloat { CGFloat(rawValue) }
}
