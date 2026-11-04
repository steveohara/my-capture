// AudioController.swift: plays the sound of the chosen audio input, delayed so
// that it lines up with the picture.
//
// THE AUDIO PATH
// AVAudioEngine joins four nodes: the input (the capture device's audio, or a
// microphone), an AVAudioUnitDelay, a fixed gain boost (an AVAudioMixerNode
// used only for its outputVolume), and the main mixer, which plays on
// whatever output the iPad is currently using. The delay unit does the job of
// the ring buffer in the Go versions: audio normally arrives sooner than the
// picture, so it is held back by the number of milliseconds the user sets.
//
// CHOOSING THE DEVICES
// iPadOS lets an app choose its input (setPreferredInput) but not its output.
// The output is chosen by the user with the system route picker (see
// RoutePicker.swift), which lists the speakers, Bluetooth headphones, AirPlay
// and USB outputs.
//
// DESIGN NOTES
//   - The session mode is measurement, which turns off the automatic gain,
//     noise suppression and echo cancellation that would otherwise change the
//     sound of the capture device and add latency.
//   - The I/O buffer is asked to be 5 ms. iPadOS may choose a larger one.
//   - There is no user-facing volume control, because the iPad's own volume
//     buttons do it. A fixed gain boost (see boostGain) is applied in the
//     node graph itself, because capture devices tested on iPad came through
//     far quieter than the same hardware on macOS and Windows, even at full
//     hardware volume; 175x (about +44.9 dB) was needed to match a normal
//     playback volume on the device and card tested, found by live
//     listening with a temporary debug gain slider (since removed).
//
// THREADING
// Most of the controller runs on the main thread, where SwiftUI calls it and
// where AVAudioEngine's own node graph setup belongs. The two places that
// call AVAudioSession.setActive(_:), in configure() and resume(), run that
// one blocking call on a background queue first, because setActive can take
// long enough to be felt as a UI freeze if called on the main thread (Xcode's
// Thread Performance Checker flags it for exactly this reason); everything
// after it that touches a @Published property or the engine hops back to the
// main thread before running. AVAudioEngine does its real time audio work on
// its own threads regardless of which thread started it.

import AVFoundation
import Combine
import Foundation

/// Owns the audio session, the audio engine and the list of audio inputs.
final class AudioController: ObservableObject {
    /// The smallest and largest delay offered, in milliseconds. They match the
    /// range of the other versions.
    static let delayRange: ClosedRange<Double> = 20...1000

    /// The delay used the first time the app runs, in milliseconds.
    static let defaultDelayMs: Double = 60

    /// The audio inputs available now, such as a USB capture device or the
    /// built-in microphone.
    @Published private(set) var inputs: [AVAudioSessionPortDescription] = []

    /// The uniqueID of the selected input, or nil if none is chosen.
    @Published private(set) var selectedInputUID: String?

    /// The name of the current output, as the system reports it.
    @Published private(set) var outputName = ""

    /// The current delay in milliseconds.
    @Published private(set) var delayMs: Double

    /// A problem to show the user, or nil.
    @Published private(set) var message: String?

    /// The engine that runs the input to output path.
    private let engine = AVAudioEngine()

    /// The delay node. It is attached once and kept for the life of the app.
    private let delay = AVAudioUnitDelay()

    /// A fixed gain stage after the delay, attached once and kept for the
    /// life of the app. AVAudioMixerNode's outputVolume accepts values above
    /// 1.0, so this node is used purely for its gain, not for mixing.
    private let boost = AVAudioMixerNode()

    /// The fixed gain applied by boost, as a linear multiplier (175.0 is
    /// about +44.9 dB). Capture devices tested on iPad came through far
    /// quieter than the same hardware on macOS and Windows at full hardware
    /// volume, even with a large boost; 175 was found by live listening on a
    /// real device and capture card to match a normal playback volume,
    /// using a temporary debug gain slider that has since been removed.
    private static let boostGain: Float = 175.0

    /// Notification observers, kept so they are not released.
    private var observers: [NSObjectProtocol] = []

    /// True once permission was granted and the session is configured.
    private var started = false

    /// UserDefaults key holding the last chosen input's name.
    private let inputNameKey = "audioInputName"

    /// True once the user has explicitly chosen "(none)" with stopInput(),
    /// so refreshInputs() leaves the engine stopped instead of restarting it
    /// with whatever input it would otherwise prefer. Reset by
    /// selectInput(uid:) as soon as the user picks a real input again.
    private var explicitlyStopped = false

    /// UserDefaults key holding the delay in milliseconds.
    private let delayKey = "audioDelayMs"

    /// Creates the controller with the remembered delay.
    init() {
        let saved = UserDefaults.standard.double(forKey: delayKey)
        delayMs = saved == 0 ? AudioController.defaultDelayMs
                             : min(max(saved, AudioController.delayRange.lowerBound),
                                   AudioController.delayRange.upperBound)
        engine.attach(delay)
        engine.attach(boost)
        boost.outputVolume = AudioController.boostGain
    }

    // -----------------------------------------------------------------------
    // Starting
    // -----------------------------------------------------------------------

    /// Asks for microphone permission if needed, then starts the audio path.
    /// Safe to call more than once.
    func start() {
        AVAudioApplication.requestRecordPermission { granted in
            DispatchQueue.main.async {
                if granted {
                    self.configure()
                } else {
                    self.message = "Microphone access is off. Turn it on for My Capture in Settings, Privacy & Security, Microphone."
                }
            }
        }
    }

    /// Restarts the audio path after the app returns to the foreground.
    /// Does nothing if the user had explicitly chosen "(none)" with
    /// stopInput(): coming back to the foreground should not turn the
    /// microphone back on by itself. setActive is a blocking call, so it
    /// runs off the main thread; the engine and message are only touched
    /// back on the main thread, since message is @Published and engine is
    /// otherwise only used there (see the file header's threading note).
    func resume() {
        guard started, !explicitlyStopped else { return }
        DispatchQueue.global(qos: .userInitiated).async {
            try? AVAudioSession.sharedInstance().setActive(true)
            DispatchQueue.main.async {
                if !self.engine.isRunning { self.rebuildAndStart() }
            }
        }
    }

    /// Sets up the audio session and the observers, then starts the engine.
    /// The AVAudioSession calls, in particular setActive, are blocking, so
    /// they run off the main thread; everything from started = true onward
    /// touches @Published properties or the engine and so runs back on the
    /// main thread (see the file header's threading note).
    private func configure() {
        guard !started else { return }
        DispatchQueue.global(qos: .userInitiated).async {
            let session = AVAudioSession.sharedInstance()
            var failure: String?
            do {
                try session.setCategory(.playAndRecord, mode: .measurement,
                                        options: [.allowBluetoothA2DP, .defaultToSpeaker])
                try session.setPreferredSampleRate(48000)
                try session.setPreferredIOBufferDuration(0.005)
                try session.setActive(true)
            } catch {
                failure = "Could not start audio: \(error.localizedDescription)"
            }
            DispatchQueue.main.async {
                if let failure {
                    self.message = failure
                    return
                }
                self.finishConfiguring()
            }
        }
    }

    /// The part of configure() that touches @Published properties, the
    /// notification center and the engine, and so must run on the main
    /// thread. Split out from configure() only so the AVAudioSession setup
    /// above it can run on a background queue first.
    private func finishConfiguring() {
        started = true

        let center = NotificationCenter.default
        observers.append(center.addObserver(forName: AVAudioSession.routeChangeNotification,
                                            object: nil, queue: .main) { [weak self] _ in
            self?.routeChanged()
        })
        observers.append(center.addObserver(forName: .AVAudioEngineConfigurationChange,
                                            object: engine, queue: .main) { [weak self] _ in
            // The hardware format changed, so the connections must be redone.
            self?.rebuildAndStart()
        })
        observers.append(center.addObserver(forName: AVAudioSession.interruptionNotification,
                                            object: nil, queue: .main) { [weak self] note in
            let raw = note.userInfo?[AVAudioSessionInterruptionTypeKey] as? UInt
            if raw == AVAudioSession.InterruptionType.ended.rawValue { self?.resume() }
        })

        refreshInputs()
        rebuildAndStart()
    }

    // -----------------------------------------------------------------------
    // Inputs and outputs
    // -----------------------------------------------------------------------

    /// Reacts to a device being plugged in or removed, or the output changing.
    /// Does not restart the engine if the user had explicitly chosen
    /// "(none)" with stopInput(): a route change (for example unplugging the
    /// capture card) should not turn the microphone back on by itself.
    private func routeChanged() {
        refreshInputs()
        if !engine.isRunning, !explicitlyStopped { rebuildAndStart() }
    }

    /// Reads the list of inputs and keeps the selection valid. A USB audio
    /// input is preferred when nothing was chosen yet. If the user had
    /// explicitly chosen "(none)" (selectedInputUID is nil but the engine is
    /// not running; see stopInput()), refreshing the list does not restart
    /// it: only a genuinely new launch or a route change while an input was
    /// already selected does.
    func refreshInputs() {
        let session = AVAudioSession.sharedInstance()
        inputs = session.availableInputs ?? []
        outputName = session.currentRoute.outputs.first?.portName ?? ""

        let current = inputs.first(where: { $0.uid == selectedInputUID })
        if current != nil { return }
        if explicitlyStopped { return }

        let saved = UserDefaults.standard.string(forKey: inputNameKey)
        let choice = inputs.first(where: { $0.portName == saved })
            ?? inputs.first(where: { $0.portType == .usbAudio })
            ?? session.currentRoute.inputs.first
            ?? inputs.first
        select(input: choice)
    }

    /// Selects an input by uniqueID.
    func selectInput(uid: String) {
        explicitlyStopped = false
        select(input: inputs.first(where: { $0.uid == uid }))
    }

    /// Stops listening to any audio input and silences the output, the
    /// microphone equivalent of VideoController's selectDevice(id: nil).
    /// Kept selected (rather than snapping back to an input on the next
    /// refreshInputs()) until the user picks one again.
    func stopInput() {
        explicitlyStopped = true
        selectedInputUID = nil
        message = nil
        engine.stop()
    }

    /// Makes port the input of the session.
    private func select(input port: AVAudioSessionPortDescription?) {
        guard let port else {
            selectedInputUID = nil
            return
        }
        do {
            try AVAudioSession.sharedInstance().setPreferredInput(port)
            selectedInputUID = port.uid
            UserDefaults.standard.set(port.portName, forKey: inputNameKey)
            message = nil
            if !engine.isRunning { rebuildAndStart() }
        } catch {
            message = "Could not use \"\(port.portName)\": \(error.localizedDescription)"
        }
    }

    // -----------------------------------------------------------------------
    // Delay
    // -----------------------------------------------------------------------

    /// Changes the audio delay. The sound may click once as the delay moves,
    /// so the slider calls this when the user lets go, not while dragging.
    func setDelay(_ ms: Double) {
        let clamped = min(max(ms, AudioController.delayRange.lowerBound),
                          AudioController.delayRange.upperBound)
        delayMs = clamped
        UserDefaults.standard.set(clamped, forKey: delayKey)
        delay.delayTime = clamped / 1000
    }

    // -----------------------------------------------------------------------
    // Engine
    // -----------------------------------------------------------------------

    /// Connects input, delay, the fixed gain boost and output with the
    /// hardware's current format, and starts the engine. Called at start and
    /// whenever the route changes.
    private func rebuildAndStart() {
        engine.stop()
        engine.disconnectNodeOutput(engine.inputNode)
        engine.disconnectNodeOutput(delay)
        engine.disconnectNodeOutput(boost)

        // A route with no input reports a zero sample rate, and connecting
        // nodes with that format would crash.
        let format = engine.inputNode.outputFormat(forBus: 0)
        guard format.sampleRate > 0, format.channelCount > 0 else {
            message = "No audio input is available."
            return
        }

        engine.connect(engine.inputNode, to: delay, format: format)
        engine.connect(delay, to: boost, format: format)
        engine.connect(boost, to: engine.mainMixerNode, format: format)

        // Delay unit as a pure delay: all wet, no feedback echoes, and a
        // low pass filter at the Nyquist frequency so no treble is lost (its
        // default of 15 kHz would dull the sound).
        delay.wetDryMix = 100
        delay.feedback = 0
        delay.lowPassCutoff = Float(format.sampleRate / 2)
        delay.delayTime = delayMs / 1000

        do {
            try engine.start()
            message = nil
        } catch {
            message = "Could not start audio: \(error.localizedDescription)"
        }
        outputName = AVAudioSession.sharedInstance().currentRoute.outputs.first?.portName ?? ""
    }
}
