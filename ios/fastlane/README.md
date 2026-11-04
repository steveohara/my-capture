fastlane documentation
----

# Installation

Make sure you have the latest version of the Xcode command line tools installed:

```sh
xcode-select --install
```

For _fastlane_ installation instructions, see [Installing _fastlane_](https://docs.fastlane.tools/#installing-fastlane)

# Available Actions

## iOS

### ios screenshots

```sh
[bundle exec] fastlane ios screenshots
```

Upload screenshots to App Store Connect

### ios metadata

```sh
[bundle exec] fastlane ios metadata
```

Upload text metadata (description, keywords, and so on) to App Store Connect

### ios upload_assets

```sh
[bundle exec] fastlane ios upload_assets
```

Upload both metadata and screenshots to App Store Connect

### ios release

```sh
[bundle exec] fastlane ios release
```

Build, archive and upload the app binary to App Store Connect

----

This README.md is auto-generated and will be re-generated every time [_fastlane_](https://fastlane.tools) is run.

More information about _fastlane_ can be found on [fastlane.tools](https://fastlane.tools).

The documentation of _fastlane_ can be found on [docs.fastlane.tools](https://docs.fastlane.tools).
