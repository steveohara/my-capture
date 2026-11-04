# Releasing My Capture for iPad

This note covers what is needed to build and submit a new version of the
iPad app to the App Store. For day to day building and running on a device
during development, see `ios/DEBUGGING.md` instead; this note assumes that
already works.

## What you need

- Everything in `ios/DEBUGGING.md`'s "First time setup": XcodeGen, the
  Xcode project generated and opened, and a signing team resolving in
  Signing & Capabilities.
- A paid Apple Developer Program account. `project.yml`'s
  `DEVELOPMENT_TEAM` (`VGC7Z7SNU7`, `sohara@pivotal-solutions.co.uk`) must
  be enrolled in it; a free Apple ID can build and run on a device but
  cannot archive an App Store build.
- fastlane. Installed via Homebrew (`brew install fastlane`) is enough; no
  Gemfile or Bundler is used here. If `fastlane` is not found on `PATH`
  after installing, run `brew link fastlane`.
- An **app-specific password** for the Apple ID above, set as an
  environment variable before uploading a binary:
  ```sh
  export FASTLANE_APPLE_APPLICATION_SPECIFIC_PASSWORD="xxxx-xxxx-xxxx-xxxx"
  ```
  Generate one at appleid.apple.com, Sign-In and Security, App-Specific
  Passwords. This is required because the binary upload step goes through
  Apple's legacy Application Loader (`altool`), which does not accept an
  ordinary Apple ID password even after fastlane's own API login succeeds;
  without it the `release` lane below fails at the upload step with
  "Authentication error" (ITunesConnectFoundationErrorDomain Code=1235),
  after the archive and export steps have already succeeded.
- The App Store Connect app record for My Capture must already exist (My
  Apps, My Capture). `ios/fastlane/Appfile` reads the bundle id and team
  from `project.yml`, but creating the app record itself, the first time,
  is still a manual step in App Store Connect.

## The fastlane lanes

Run every lane from the `ios` directory:

```sh
cd ios
fastlane metadata        # upload only the text fields (name, description, keywords, URLs)
fastlane screenshots     # upload only the screenshots
fastlane upload_assets   # both of the above together
fastlane release         # build, archive and upload the binary
```

`ios/fastlane/Fastfile` defines all four. None of them submit the app for
review; that is a separate, final step in App Store Connect (see below).

### metadata, screenshots, upload_assets

These wrap `deliver` and read from:
- `ios/fastlane/metadata/en-GB` and `ios/fastlane/metadata/copyright.txt`,
  `primary_category.txt`: the App Store listing text. `en-GB` because that
  is My Capture's primary language in App Store Connect; a locale with no
  files here is simply left untouched on Apple's side.
- `ios/fastlane/screenshots/en-GB`: the screenshots. See its own README.md
  for required sizes and how to take them. `deliver` matches a file to a
  device size by its pixel dimensions, not its name.

Edit a file in either folder, then re-run the matching lane to push the
change. There is no need to run `release` again just because text or
screenshots changed, and no need to run `upload_assets` again just because
a new binary was built.

### release

Archives with `xcodebuild` (via `build_app`/gym: scheme `MyCapture`,
configuration `Release`, `export_method: "app-store"`, automatic signing
against the team in `project.yml`), then uploads the resulting `.ipa` with
`upload_to_app_store` (skipping metadata and screenshots, which the other
lanes already cover).

`export_xcargs: "-allowProvisioningUpdates"` is set because raw
`xcodebuild -exportArchive` does not create or fetch a missing App Store
distribution profile on its own the way Xcode's Organizer does; without it
the export step fails with "No profiles for '...' were found" even though
the preceding archive step succeeds.

After a successful run, the new build appears in App Store Connect (My
Capture, the version page, Build section) once Apple finishes processing
it, usually within half an hour.

## Bumping the version

Before releasing a new version:

1. Change `MARKETING_VERSION` in `project.yml` to the new version string
   (for example `"1.1"`). This must match a version App Store Connect
   already has, or is about to have, for the app; `deliver` looks up the
   version by this exact string, and a mismatch fails with a
   `Spaceship::ConnectAPI::Models No data (RuntimeError)` error that is
   easy to misread as an authentication or app-record problem rather than
   a version string problem.
2. Run `xcodegen generate` so `MyCapture.xcodeproj` picks up the change
   (it is not kept in git; see `project.yml`'s own header comment).
3. In App Store Connect, add the new version on the app's page if it does
   not exist yet (`+ Version` next to the platform), before running
   `fastlane release` or `fastlane upload_assets` against it.
4. `CURRENT_PROJECT_VERSION` (the build number) does not need to change by
   hand between uploads to the same `MARKETING_VERSION`; App Store Connect
   accepts repeated uploads and distinguishes them internally, but raise it
   too if you want the build number itself to be meaningful.

## Submitting for review

Once a build has processed and the metadata, screenshots, age rating and
privacy declaration are all in place:

1. App Store Connect, My Capture, the version page.
2. **Build**: select the build `fastlane release` uploaded.
3. Confirm **Pricing and Availability** is set (not handled by anything in
   this repository or by fastlane here).
4. **Add for Review** / **Submit for Review**. Apple asks a few final
   questions on this screen:
   - Export compliance: already answered by `project.yml`'s
     `INFOPLIST_KEY_ITSAppUsesNonExemptEncryption: NO`, so this should not
     come up again.
   - Advertising identifier (IDFA) use: **No**, the app uses none.
   - Content rights: the app owns all its own content (UI, icon); the only
     third-party material is whatever video and audio the user's own
     capture device provides, which the app neither stores nor transmits.

## Where the fixed answers live

Some App Store Connect fields are filled in by the Fastfile itself, not
re-entered by hand each release:

- **App Review Information** (contact details and reviewer notes): set in
  `review_information` at the bottom of `ios/fastlane/Fastfile`. The notes
  explain that the app needs a USB-C capture card plugged in to show
  anything, so a reviewer testing without one does not mistake a black
  screen and "(none)" pickers for a bug. Update this function if the
  contact details change.
- **Age Rating**: every content category (violence, sexual content,
  profanity, alcohol/tobacco/drugs, mature themes, horror, gambling) is
  None; Unrestricted Web Access is No; Contests is No. Lowest tier (4+).
  Entered directly in App Store Connect (App Information, Age Rating); not
  scriptable through `deliver` in this fastlane version.
- **App Privacy**: "Do you or your third-party partners collect data from
  this app?" is No, matching `docs/privacy.md`: nothing is collected,
  stored off-device, or transmitted. Also entered directly in App Store
  Connect (App Privacy tab); not scriptable through `deliver`.

Both of the above need re-entering by hand only if the answer itself would
ever change, for example if a future version added analytics or a network
feature; the app as it stands today does neither.
