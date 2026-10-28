// Package assets holds the files that are compiled into the my-capture
// program, so it needs nothing next to it on disk to show its icon.
//
// The icon files in assets/icon are produced by tools/genicon (run "make
// icons" to regenerate them). This package embeds the two PNG files the
// running program shows in its windows, and the two files it needs to build
// a temporary macOS app bundle for itself when it is started outside one (see
// internal/ui/platform_darwin.go): the bundle's Info.plist and icon.icns.
// "make dist-macos" uses the very same two files for the real bundle. The
// Windows icon (icon.ico) is embedded in the executable through
// rsrc_windows_amd64.syso at the repository root, not through this package.
package assets

import _ "embed"

// IconPNG is the application icon for windows and task bars on Windows: a
// 256 pixel PNG in which the icon tile fills the image.
//
//go:embed icon/icon.png
var IconPNG []byte

// IconMacPNG is the application icon in macOS style, a 512 pixel PNG with the
// transparent margin macOS icons have. It is used for the Dock icon when the
// program runs outside an application bundle, such as from a terminal or IDE.
//
//go:embed icon/icon-mac.png
var IconMacPNG []byte

// InfoPlist is the Info.plist template for the macOS application bundle, from
// assets/macos/Info.plist. Its @VERSION@ placeholders are replaced by the
// build version.
//
//go:embed macos/Info.plist
var InfoPlist []byte

// IconICNS is the macOS application icon, from assets/icon/icon.icns.
//
//go:embed icon/icon.icns
var IconICNS []byte
