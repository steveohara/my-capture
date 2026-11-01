// AspectMode.swift: how the picture is fitted into the video area.
//
// These are the same choices as the macOS and Windows versions. The choice is
// remembered between runs by its raw value, so renaming a case forgets it.
//
// WHY ratio(rotatedBy:) AND NOT A PLAIN PROPERTY?
// AVCaptureVideoPreviewLayer's videoGravity and an outer SwiftUI
// aspectRatio(_:contentMode:) both compute their fit against the picture's
// width and height as delivered by the capture connection, before
// videoRotationAngle turns it on screen. Rotating 90 or 270 degrees swaps
// what the viewer sees as wide and tall, so a fixed shape such as 16:9 has to
// be asked for sideways (9:16) to end up looking like 16:9 once rotated, or
// the frame ends up the wrong way round and the picture is left filling only
// one dimension inside it. See RotationMode and ContentView.picture for where
// this is used.

import AVFoundation
import CoreGraphics

/// How the picture is fitted into the video area.
enum AspectMode: String, CaseIterable, Identifiable {
    case fit
    case stretch
    case fill
    case ratio16x9
    case ratio16x10
    case ratio4x3
    case ratio21x9
    case ratio1x1

    var id: String { rawValue }

    /// The text shown in the aspect menu.
    var title: String {
        switch self {
        case .fit: return "Fit (keep source shape)"
        case .stretch: return "Stretch to window"
        case .fill: return "Fill window (crop edges)"
        case .ratio16x9: return "16:9"
        case .ratio16x10: return "16:10"
        case .ratio4x3: return "4:3"
        case .ratio21x9: return "21:9"
        case .ratio1x1: return "1:1"
        }
    }

    /// How the preview layer scales the picture inside its own bounds. For the
    /// fixed shapes it stretches, because the bounds are already the wanted
    /// shape and the picture is bent to fill them.
    var gravity: AVLayerVideoGravity {
        switch self {
        case .fit: return .resizeAspect
        case .fill: return .resizeAspectFill
        default: return .resize
        }
    }

    /// Width divided by height that the picture area is forced to, or nil when
    /// the area is the whole window. This is the shape before any rotation;
    /// use ratio(rotatedBy:) to get the shape to ask SwiftUI for once a
    /// rotation is applied.
    var ratio: CGFloat? {
        switch self {
        case .ratio16x9: return 16.0 / 9
        case .ratio16x10: return 16.0 / 10
        case .ratio4x3: return 4.0 / 3
        case .ratio21x9: return 21.0 / 9
        case .ratio1x1: return 1
        default: return nil
        }
    }

    /// ratio, adjusted for rotationAngle. A 90 or 270 degree rotation swaps
    /// what ends up wide and tall on screen, so the frame SwiftUI lays the
    /// picture out in has to be asked for in the swapped shape (height over
    /// width instead of width over height) for the rotated picture to still
    /// end up the requested shape once it is turned. A 0 or 180 degree
    /// rotation does not change the shape, only which edge is which.
    func ratio(rotatedBy angle: CGFloat) -> CGFloat? {
        guard let ratio else { return nil }
        let swapped = Int(angle.truncatingRemainder(dividingBy: 360)) % 180 != 0
        return swapped ? 1 / ratio : ratio
    }
}
