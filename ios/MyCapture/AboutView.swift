// AboutView.swift: the About sheet.
//
// Shows the version from the app bundle (CFBundleShortVersionString, set by
// MARKETING_VERSION in project.yml), and links to the project's GitHub page
// and issue tracker, matching what the macOS and Windows About box offers
// minus the "Copy details" button, since there is no ffmpeg version or
// desktop OS version relevant on iPadOS.

import SwiftUI

/// Shows the app version and links to the project, in a sheet with a Done
/// button.
struct AboutView: View {
    /// Closes the sheet. Supplied by the sheet presentation.
    @Environment(\.dismiss) private var dismiss

    /// The project's GitHub page.
    private let repoURL = URL(string: "https://github.com/steveohara/my-capture")!

    /// The project's issue tracker, for bug reports.
    private let issuesURL = URL(string: "https://github.com/steveohara/my-capture/issues")!

    /// The marketing version string from the app's own bundle, such as "0.1.0".
    private var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "unknown"
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                Image(systemName: "video.fill")
                    .font(.system(size: 56))
                    .foregroundStyle(.tint)
                Text("My Capture").font(.title.bold())
                Text("Version \(version)").foregroundStyle(.secondary)

                Text("""
                Shows the picture and plays the sound of a USB video capture \
                device. Nothing is recorded.
                """)
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
                .padding(.horizontal)

                VStack(spacing: 12) {
                    Link("My Capture on GitHub", destination: repoURL)
                    Link("Report an Issue", destination: issuesURL)
                }
                .padding(.top)

                Spacer()
            }
            .padding()
            .navigationTitle("About My Capture")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
    }
}
