// AudioController.swift: plays the sound of the chosen audio input, delayed so
// that it lines up with the picture.
//
// THE AUDIO PATH
// AVAudioEngine joins three nodes: the input (the capture device's audio, or a
// microphone), an AVAudioUnitDelay, and the main mixer, which plays on whatever
// output the iPad is currently using. The delay unit does the job of the ring
// buffer in the Go versions: audio normally arrives sooner than the picture,
// so it is held back by the number of milliseconds the user sets.
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
//   - There is no volume control, because the iPad's own volume buttons do it.
//
// THREADING
// Everything runs on the main thread. AVAudioEngine does its real time work on
// its own threads.

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

    /// Notification observers, kept so they are not released.
    private var observers: [NSObjectProtocol] = []

    /// True once permission was granted and the session is configured.
    private var started = false

    /// UserDefaults key holding the last chosen input's name.
    private let inputNameKey = "audioInputName"

    /// UserDefaults key holding the delay in milliseconds.
    private let delayKey = "audioDelayMs"

    /// Creates the controller with the remembered delay.
    init() {
        let saved = UserDefaults.standard.double(forKey: delayKey)
        delayMs = saved == 0 ? AudioController.defaultDelayMs
                             : min(max(saved, AudioController.delayRange.lowerBound),
                                   AudioController.delayRange.upperBound)
        engine.attach(delay)
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
    func resume() {
        guard started else { return }
        try? AVAudioSession.sharedInstance().setActive(true)
        if !engine.isRunning { rebuildAndStart() }
    }

    /// Sets up the audio session and the observers, then starts the engine.
    private func configure() {
        guard !started else { return }
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(.playAndRecord, mode: .measurement,
                                    options: [.allowBluetoothA2DP, .defaultToSpeaker])
            try session.setPreferredSampleRate(48000)
            try session.setPreferredIOBufferDuration(0.005)
            try session.setActive(true)
        } catch {
            message = "Could not start audio: \(error.localizedDescription)"
            return
        }
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
    private func routeChanged() {
        refreshInputs()
        if !engine.isRunning { rebuildAndStart() }
    }

    /// Reads the list of inputs and keeps the selection valid. A USB audio
    /// input is preferred when nothing was chosen, because that is the capture
    /// device's own sound.
    func refreshInputs() {
        let session = AVAudioSession.sharedInstance()
        inputs = session.availableInputs ?? []
        outputName = session.currentRoute.outputs.first?.portName ?? ""

        let current = inputs.first(where: { $0.uid == selectedInputUID })
        if current != nil { return }

        let saved = UserDefaults.standard.string(forKey: inputNameKey)
        let choice = inputs.first(where: { $0.portName == saved })
            ?? inputs.first(where: { $0.portType == .usbAudio })
            ?? session.currentRoute.inputs.first
            ?? inputs.first
        select(input: choice)
    }

    /// Selects an input by uniqueID.
    func selectInput(uid: String) {
        select(input: inputs.first(where: { $0.uid == uid }))
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

    /// Connects input, delay and output with the hardware's current format and
    /// starts the engine. Called at start and whenever the route changes.
    private func rebuildAndStart() {
        engine.stop()
        engine.disconnectNodeOutput(engine.inputNode)
        engine.disconnectNodeOutput(delay)

        // A route with no input reports a zero sample rate, and connecting
        // nodes with that format would crash.
        let format = engine.inputNode.outputFormat(forBus: 0)
        guard format.sampleRate > 0, format.channelCount > 0 else {
            message = "No audio input is available."
            return
        }

        engine.connect(engine.inputNode, to: delay, format: format)
        engine.connect(delay, to: engine.mainMixerNode, format: format)

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
