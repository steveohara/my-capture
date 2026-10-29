// AspectMode.swift: how the picture is fitted into the video area.
//
// These are the same choices as the macOS and Windows versions. The choice is
// remembered between runs by its raw value, so renaming a case forgets it.

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
    /// the area is the whole window.
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
}
