// ContentView.swift: the single window of My Capture for iPad.
//
// LAYOUT
// The picture fills the window on a black background. A control panel sits
// over the top, with a translucent material background so the picture shows
// through at its edges. Tapping the picture hides or shows the panel and, with
// it, the status bar and home indicator, so the picture can have the whole
// screen. A banner below the panel shows any problem reported by the two
// controllers.
//
// WHY ONE VIEW AND NOT A NAVIGATION STACK?
// There is nothing to navigate to: Help and About are sheets over this one
// screen, matching the single window the macOS and Windows versions have.

import AVFoundation
import SwiftUI

/// The main and only screen of the app.
struct ContentView: View {
    /// The video device, mode and capture session.
    @EnvironmentObject private var video: VideoController

    /// The audio input, output and delay.
    @EnvironmentObject private var audio: AudioController

    /// Whether the app is in the foreground, backgrounded or inactive.
    @Environment(\.scenePhase) private var scenePhase

    /// How the picture is fitted into its area. Kept between launches.
    @AppStorage("aspectMode") private var aspectMode = AspectMode.fit

    /// Whether the control panel is shown. Starts shown so the controls are
    /// discoverable the first time the app runs.
    @State private var controlsVisible = true

    /// Whether the Help sheet is shown.
    @State private var showHelp = false

    /// Whether the About sheet is shown.
    @State private var showAbout = false

    /// The delay slider's own value while the user is dragging it. Kept
    /// separate from audio.delayMs so the slider can move freely without
    /// sending a change to AudioController on every tick.
    @State private var delayDraft: Double = AudioController.defaultDelayMs

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            picture
                .onTapGesture { controlsVisible.toggle() }

            VStack(spacing: 0) {
                if controlsVisible {
                    controlPanel
                    banner
                }
                Spacer()
            }
        }
        .statusBarHidden(!controlsVisible)
        .persistentSystemOverlays(controlsVisible ? .automatic : .hidden)
        .animation(.default, value: controlsVisible)
        .sheet(isPresented: $showHelp) { HelpView() }
        .sheet(isPresented: $showAbout) { AboutView() }
        .onAppear {
            video.start()
            audio.start()
            UIApplication.shared.isIdleTimerDisabled = true
            delayDraft = audio.delayMs
        }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active {
                video.resume()
                audio.resume()
            }
        }
    }

    /// The video picture, fitted or filled according to aspectMode.
    @ViewBuilder
    private var picture: some View {
        let preview = PreviewView(session: video.session, gravity: aspectMode.gravity)
        if let ratio = aspectMode.ratio {
            preview.aspectRatio(ratio, contentMode: .fit)
        } else {
            preview
        }
    }

    /// The top panel: device, mode, audio and aspect pickers, plus Help and
    /// About. Sits on a material background so it reads over any picture.
    private var controlPanel: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 16) {
                videoMenu
                audioInputMenu
                outputControl
                aspectMenu
                Spacer()
                Button {
                    showHelp = true
                } label: {
                    Image(systemName: "questionmark.circle")
                }
                Button {
                    showAbout = true
                } label: {
                    Image(systemName: "info.circle")
                }
            }
            delayRow
        }
        .padding()
        .background(.ultraThinMaterial)
    }

    /// Chooses the video device and its mode.
    private var videoMenu: some View {
        HStack(spacing: 16) {
            Menu {
                Button("(none)") { video.selectDevice(id: nil) }
                ForEach(video.devices, id: \.uniqueID) { device in
                    Button(device.localizedName) { video.selectDevice(id: device.uniqueID) }
                }
            } label: {
                Label(video.devices.first { $0.uniqueID == video.selectedDeviceID }?.localizedName
                        ?? "Video device", systemImage: "video")
            }

            Menu {
                ForEach(video.modes) { mode in
                    Button(mode.title) { video.selectMode(id: mode.id) }
                }
            } label: {
                Label(video.modes.first { $0.id == video.selectedModeID }?.title ?? "Mode",
                      systemImage: "rectangle.on.rectangle")
            }
            .disabled(video.modes.isEmpty)
        }
    }

    /// Chooses the audio input.
    private var audioInputMenu: some View {
        Menu {
            ForEach(audio.inputs, id: \.uid) { port in
                Button(port.portName) { audio.selectInput(uid: port.uid) }
            }
        } label: {
            Label(audio.inputs.first { $0.uid == audio.selectedInputUID }?.portName ?? "Audio input",
                  systemImage: "mic")
        }
        .disabled(audio.inputs.isEmpty)
    }

    /// The system route picker, labelled with the output it chose.
    private var outputControl: some View {
        HStack(spacing: 6) {
            RoutePicker()
                .frame(width: 28, height: 28)
            Text(audio.outputName.isEmpty ? "Audio output" : audio.outputName)
                .lineLimit(1)
        }
    }

    /// Chooses how the picture is fitted into its area.
    private var aspectMenu: some View {
        Menu {
            ForEach(AspectMode.allCases) { mode in
                Button(mode.title) { aspectMode = mode }
            }
        } label: {
            Label(aspectMode.title, systemImage: "aspectratio")
        }
    }

    /// The audio delay slider. The value is only sent to AudioController when
    /// the user lets go, because applying it while dragging would click.
    private var delayRow: some View {
        HStack {
            Text("Audio delay")
            Slider(value: Binding(
                get: { audio.delayMs },
                set: { audio.setDelay($0) }
            ), in: AudioController.delayRange, step: 10, onEditingChanged: { editing in
                if !editing { audio.setDelay(audio.delayMs) }
            })
            Text("\(Int(audio.delayMs)) ms")
                .monospacedDigit()
                .frame(width: 60, alignment: .trailing)
        }
    }

    /// Shows the most recent problem from either controller, if any.
    @ViewBuilder
    private var banner: some View {
        if let message = video.message ?? audio.message {
            Text(message)
                .font(.footnote)
                .padding(8)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(.ultraThinMaterial)
        }
    }
}
