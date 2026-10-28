//go:build !darwin

// Package ui: platform_other.go is the non-macOS counterpart of
// platform_darwin.go.
//
// On Windows the name and icon come from the executable itself (the icon is
// embedded by rsrc_windows_amd64.syso) and from the icon set on the Fyne
// application, so there is nothing to do before Fyne starts.
package ui

// platformInit does nothing outside macOS.
func platformInit(_, _ string, _ []byte, _ bool) {}
