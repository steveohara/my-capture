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

    /// How the picture is rotated. Kept between launches, the same way as
    /// aspectMode: neither the capture card nor a built-in camera has a
    /// fixed relationship to how the iPad is held (see RotationMode).
    @AppStorage("rotationMode") private var rotationMode = RotationMode.none

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

    /// The video picture, fitted or filled according to aspectMode, rotated
    /// according to rotationMode.
    ///
    /// WHY ONE VIEW AND NOT AN IF/ELSE BRANCH?
    /// aspectRatio(_:contentMode:) takes an optional ratio, so the same
    /// PreviewView value can be used for every aspectMode by passing nil for
    /// the fixed-shape modes' absence of one. Branching between "preview with
    /// aspectRatio applied" and "preview alone" in an if/else, as this used
    /// to, gives the two branches different view tree identities; SwiftUI
    /// then tears down and recreates the PreviewUIView, including its
    /// AVCaptureVideoPreviewLayer's connection, whenever aspectMode moved
    /// between a fixed-shape mode and Fit, Stretch or Fill, which is what
    /// made the picture flash black and restart on those particular changes.
    /// A single expression has one identity throughout, so only
    /// updateUIView runs, and the existing connection's frames keep showing.
    private var picture: some View {
        PreviewView(session: video.session, gravity: aspectMode.gravity,
                    rotationAngle: rotationMode.angle)
            .aspectRatio(aspectMode.ratio(rotatedBy: rotationMode.angle), contentMode: .fit)
    }

    /// The top panel: device, mode, audio and aspect pickers, plus Help and
    /// About. Sits on a material background so it reads over any picture.
    ///
    /// WIDTH HANDLING
    /// All five controls plus the two sheet buttons do not fit on one row at
    /// portrait width on an iPad (around 390 pt), where SwiftUI would
    /// otherwise squeeze each Label's text into a single narrow column one
    /// letter per line rather than truncate it sensibly. ViewThatFits tries
    /// the single row first, exactly as before at landscape width, and falls
    /// back to two rows, each a plain HStack with its own Spacer, when the
    /// single row does not fit.
    private var controlPanel: some View {
        VStack(alignment: .leading, spacing: 10) {
            ViewThatFits(in: .horizontal) {
                oneRowControls
                twoRowControls
            }
            delayRow
        }
        .padding()
        .background(.ultraThinMaterial)
    }

    /// All controls and the two sheet buttons in a single row. Fits at
    /// landscape width; see controlPanel for why a fallback exists.
    private var oneRowControls: some View {
        HStack(spacing: 16) {
            videoMenu
            audioInputMenu
            outputControl
            aspectMenu
            rotateMenu
            Spacer()
            helpButton
            aboutButton
        }
    }

    /// The same controls split across two rows, for widths where
    /// oneRowControls would otherwise compress its labels unreadably (see
    /// controlPanel). The device and mode pickers stay together on the first
    /// row since choosing a device usually means choosing its mode next.
    private var twoRowControls: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 16) {
                videoMenu
                Spacer()
                helpButton
                aboutButton
            }
            HStack(spacing: 16) {
                audioInputMenu
                outputControl
                aspectMenu
                rotateMenu
                Spacer()
            }
        }
    }

    /// Opens the Help sheet.
    private var helpButton: some View {
        Button {
            showHelp = true
        } label: {
            Image(systemName: "questionmark.circle")
        }
    }

    /// Opens the About sheet.
    private var aboutButton: some View {
        Button {
            showAbout = true
        } label: {
            Image(systemName: "info.circle")
        }
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

    /// Chooses the audio input. "(none)" stops listening to any input,
    /// matching the "(none)" choice in the video device menu.
    private var audioInputMenu: some View {
        Menu {
            Button("(none)") { audio.stopInput() }
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

    /// Chooses how the picture is rotated. The current choice is kept
    /// between launches (see rotationMode in ContentView).
    private var rotateMenu: some View {
        Menu {
            ForEach(RotationMode.allCases) { mode in
                Button(mode.title) { rotationMode = mode }
            }
        } label: {
            Label(rotationMode.title, systemImage: "rotate.right")
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
