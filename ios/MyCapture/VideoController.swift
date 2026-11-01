// VideoController.swift: finds the video capture devices and runs the capture
// session that feeds the on-screen picture.
//
// WHY AVFOUNDATION AND NOT FFMPEG?
// iPadOS does not let an app start another program, so the ffmpeg process used
// on macOS and Windows is not an option. AVFoundation is the framework that
// ffmpeg's own macOS input uses anyway. Since iPadOS 17 it lists USB video
// class devices (capture cards, webcams) plugged into the USB-C port as
// "external" cameras.
//
// WHY IS IT LOW LATENCY?
// The AVCaptureSession is shown by an AVCaptureVideoPreviewLayer (see
// PreviewView.swift). The frames go from the driver to the display without
// passing through the program, so there is no pipe, no pixel conversion and no
// frame copy, unlike the other versions.
//
// THREADING
// The published properties are only changed on the main thread. The capture
// session is only configured on sessionQueue, because startRunning blocks and
// Apple asks for it to be kept off the main thread. Every public method must
// be called from the main thread, which is where SwiftUI calls them.

import AVFoundation
import Combine
import CoreMedia
import Foundation

// -----------------------------------------------------------------------
// Modes
// -----------------------------------------------------------------------

/// One resolution, frame rate and pixel format offered by a device.
struct VideoMode: Identifiable, Hashable {
    /// Frame width in pixels.
    let width: Int

    /// Frame height in pixels.
    let height: Int

    /// Frame rate in frames per second.
    let fps: Double

    /// Short name of the pixel format or codec, such as "yuyv422" or "mjpeg".
    let codec: String

    /// Index into the device's formats array of the format that gives this mode.
    let formatIndex: Int

    /// The frame duration that gives fps, taken from the device's own frame
    /// rate range so that it is a value the device accepts exactly.
    let frameDuration: CMTime

    /// A stable identifier, also used to remember the choice between runs.
    var id: String { "\(width)x\(height)@\(fps)-\(codec)" }

    /// The text shown in the mode menu, for example "1920x1080 @ 60 fps (yuyv422)".
    var title: String {
        let rate = fps == fps.rounded() ? String(Int(fps)) : String(format: "%.2f", fps)
        return "\(width)x\(height) @ \(rate) fps (\(codec))"
    }

    static func == (lhs: VideoMode, rhs: VideoMode) -> Bool { lhs.id == rhs.id }

    func hash(into hasher: inout Hasher) { hasher.combine(id) }
}

// -----------------------------------------------------------------------
// Controller
// -----------------------------------------------------------------------

/// Owns the capture session, the list of devices and the chosen device and mode.
final class VideoController: ObservableObject {
    /// The session the preview layer draws. It lives for the life of the app.
    let session = AVCaptureSession()

    /// The video devices currently plugged in, external ones first.
    @Published private(set) var devices: [AVCaptureDevice] = []

    /// The modes of the selected device, best first.
    @Published private(set) var modes: [VideoMode] = []

    /// The uniqueID of the selected device, or nil for none.
    @Published private(set) var selectedDeviceID: String?

    /// The id of the selected mode, or nil for none.
    @Published private(set) var selectedModeID: String?

    /// A problem to show the user, or nil. It is cleared when a device is picked.
    @Published private(set) var message: String?

    /// True while the session is delivering frames.
    @Published private(set) var isRunning = false

    /// Serial queue on which the capture session is configured and started.
    private let sessionQueue = DispatchQueue(label: "my-capture.video.session")

    /// Notification observers, kept so they are not released.
    private var observers: [NSObjectProtocol] = []

    /// True once the camera permission has been granted and observers added.
    private var started = false

    /// UserDefaults key holding the last chosen device name. The name is kept
    /// rather than the uniqueID because the ID of an external device can change
    /// when it is replugged, as with the other versions.
    private let deviceNameKey = "videoDeviceName"

    // -----------------------------------------------------------------------
    // Starting
    // -----------------------------------------------------------------------

    /// Asks for camera permission if needed, then lists the devices and starts
    /// the remembered one. Safe to call more than once.
    func start() {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            begin()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { granted in
                DispatchQueue.main.async {
                    if granted { self.begin() } else { self.denied() }
                }
            }
        default:
            denied()
        }
    }

    /// Resumes the session after the app returns to the foreground.
    func resume() {
        guard started, selectedDeviceID != nil else { return }
        sessionQueue.async {
            if !self.session.isRunning && !self.session.inputs.isEmpty {
                self.session.startRunning()
            }
        }
    }

    /// Reports that the camera permission was refused.
    private func denied() {
        message = "Camera access is off. Turn it on for My Capture in Settings, Privacy & Security, Camera."
    }

    /// Adds the observers and picks the first device.
    private func begin() {
        guard !started else { return }
        started = true

        let center = NotificationCenter.default
        let refresh: (Notification) -> Void = { [weak self] _ in
            DispatchQueue.main.async { self?.refreshDevices() }
        }
        observers.append(center.addObserver(forName: AVCaptureDevice.wasConnectedNotification,
                                            object: nil, queue: nil, using: refresh))
        observers.append(center.addObserver(forName: AVCaptureDevice.wasDisconnectedNotification,
                                            object: nil, queue: nil, using: refresh))
        observers.append(center.addObserver(forName: AVCaptureSession.didStartRunningNotification,
                                            object: session, queue: .main) { [weak self] _ in
            self?.isRunning = true
        })
        observers.append(center.addObserver(forName: AVCaptureSession.didStopRunningNotification,
                                            object: session, queue: .main) { [weak self] _ in
            self?.isRunning = false
        })
        observers.append(center.addObserver(forName: AVCaptureSession.runtimeErrorNotification,
                                            object: session, queue: .main) { [weak self] note in
            let error = note.userInfo?[AVCaptureSessionErrorKey] as? NSError
            self?.message = "The capture stopped: \(error?.localizedDescription ?? "unknown error")"
        })

        refreshDevices()
    }

    // -----------------------------------------------------------------------
    // Devices
    // -----------------------------------------------------------------------

    /// Reads the list of devices. The running capture is only touched when its
    /// device has gone, or when nothing is selected yet.
    func refreshDevices() {
        let discovery = AVCaptureDevice.DiscoverySession(
            deviceTypes: [.external, .builtInWideAngleCamera, .builtInUltraWideCamera],
            mediaType: .video,
            position: .unspecified)
        // External devices first: they are what this program is for.
        devices = discovery.devices.sorted { a, b in
            (a.deviceType == .external) && (b.deviceType != .external)
        }

        if let id = selectedDeviceID, !devices.contains(where: { $0.uniqueID == id }) {
            stopCapture()
            selectedDeviceID = nil
            modes = []
            selectedModeID = nil
            message = "The video device was unplugged."
        }

        if selectedDeviceID == nil, let device = preferredDevice() {
            selectDevice(id: device.uniqueID)
        }
    }

    /// Chooses the device to start with: the one used last time if it is
    /// plugged in, otherwise the first external device.
    private func preferredDevice() -> AVCaptureDevice? {
        let saved = UserDefaults.standard.string(forKey: deviceNameKey)
        if let saved, let match = devices.first(where: { $0.localizedName == saved }) {
            return match
        }
        return devices.first(where: { $0.deviceType == .external })
    }

    /// Selects a device by uniqueID, or none when id is nil, and starts it.
    func selectDevice(id: String?) {
        guard let id, let device = devices.first(where: { $0.uniqueID == id }) else {
            stopCapture()
            selectedDeviceID = nil
            modes = []
            selectedModeID = nil
            return
        }
        message = nil
        selectedDeviceID = id
        UserDefaults.standard.set(device.localizedName, forKey: deviceNameKey)

        modes = VideoController.modes(of: device)
        let savedMode = UserDefaults.standard.string(forKey: modeKey(device))
        let mode = modes.first(where: { $0.id == savedMode }) ?? modes.first
        selectedModeID = mode?.id
        if let mode { startCapture(device: device, mode: mode) }
    }

    /// Selects a mode of the selected device and restarts the capture with it.
    func selectMode(id: String) {
        guard let deviceID = selectedDeviceID,
              let device = devices.first(where: { $0.uniqueID == deviceID }),
              let mode = modes.first(where: { $0.id == id }) else { return }
        message = nil
        selectedModeID = id
        UserDefaults.standard.set(id, forKey: modeKey(device))
        startCapture(device: device, mode: mode)
    }

    /// UserDefaults key holding the last mode chosen for device.
    private func modeKey(_ device: AVCaptureDevice) -> String {
        "videoMode.\(device.localizedName)"
    }

    // -----------------------------------------------------------------------
    // Capture session
    // -----------------------------------------------------------------------

    /// Points the session at device in mode and makes sure it is running.
    private func startCapture(device: AVCaptureDevice, mode: VideoMode) {
        sessionQueue.async {
            self.session.beginConfiguration()
            // inputPriority makes the session keep the format set on the
            // device. Any other preset would override it with its own.
            self.session.sessionPreset = .inputPriority
            for input in self.session.inputs { self.session.removeInput(input) }

            var failure: String?
            do {
                let input = try AVCaptureDeviceInput(device: device)
                if self.session.canAddInput(input) {
                    self.session.addInput(input)
                    try device.lockForConfiguration()
                    device.activeFormat = device.formats[mode.formatIndex]
                    device.activeVideoMinFrameDuration = mode.frameDuration
                    device.activeVideoMaxFrameDuration = mode.frameDuration
                    device.unlockForConfiguration()
                } else {
                    failure = "The video device \"\(device.localizedName)\" cannot be used."
                }
            } catch {
                failure = "Could not open \"\(device.localizedName)\": \(error.localizedDescription)"
            }
            self.session.commitConfiguration()

            if let failure {
                DispatchQueue.main.async { self.message = failure }
                return
            }
            if !self.session.isRunning { self.session.startRunning() }
        }
    }

    /// Stops the session and detaches its input.
    private func stopCapture() {
        sessionQueue.async {
            if self.session.isRunning { self.session.stopRunning() }
            self.session.beginConfiguration()
            for input in self.session.inputs { self.session.removeInput(input) }
            self.session.commitConfiguration()
        }
    }

    // -----------------------------------------------------------------------
    // Mode discovery
    // -----------------------------------------------------------------------

    /// Lists the modes of device, best first: modes no wider than 1920 pixels
    /// come before larger ones, then wider before narrower, taller before
    /// shorter, and higher frame rates before lower. The same order as the
    /// macOS and Windows versions, which puts 1920x1080 at 60 fps first.
    static func modes(of device: AVCaptureDevice) -> [VideoMode] {
        var found: [VideoMode] = []
        var seen = Set<String>()
        for (index, format) in device.formats.enumerated() {
            let size = CMVideoFormatDescriptionGetDimensions(format.formatDescription)
            let codec = codecName(CMFormatDescriptionGetMediaSubType(format.formatDescription))
            for range in format.videoSupportedFrameRateRanges {
                let mode = VideoMode(width: Int(size.width), height: Int(size.height),
                                     fps: range.maxFrameRate, codec: codec,
                                     formatIndex: index, frameDuration: range.minFrameDuration)
                if seen.insert(mode.id).inserted { found.append(mode) }
            }
        }
        return found.sorted { a, b in
            if (a.width <= 1920) != (b.width <= 1920) { return a.width <= 1920 }
            if a.width != b.width { return a.width > b.width }
            if a.height != b.height { return a.height > b.height }
            return a.fps > b.fps
        }
    }

    /// Names a pixel format or codec with the ffmpeg style names that the
    /// other versions show, so the same card reads the same on every system.
    private static func codecName(_ type: FourCharCode) -> String {
        switch type {
        case kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange,
             kCVPixelFormatType_420YpCbCr8BiPlanarFullRange:
            return "nv12"
        case kCVPixelFormatType_422YpCbCr8_yuvs: return "yuyv422"
        case kCVPixelFormatType_422YpCbCr8: return "uyvy422"
        case kCVPixelFormatType_32BGRA: return "bgra"
        case kCMVideoCodecType_JPEG, kCMVideoCodecType_JPEG_OpenDML: return "mjpeg"
        case kCMVideoCodecType_H264: return "h264"
        case kCMVideoCodecType_HEVC: return "hevc"
        default:
            let bytes = [24, 16, 8, 0].map { UInt8((type >> UInt32($0)) & 0xFF) }
            return String(bytes: bytes, encoding: .ascii) ?? "unknown"
        }
    }
}
