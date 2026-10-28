//go:build !windows

// Package video: proc_other.go is the non-Windows counterpart of
// proc_windows.go.
//
// Only Windows needs help to keep a child process from opening a console
// window, so on every other platform the function does nothing.
package video

import "os/exec"

// hideWindow is a no-op outside Windows.
func hideWindow(_ *exec.Cmd) {}
