//go:build windows

// Package video: proc_windows.go stops ffmpeg from opening a console window.
//
// my-capture is a GUI program, and a console program started by a GUI
// program on Windows would otherwise flash a black console window every time
// ffmpeg is launched.
package video

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the CREATE_NO_WINDOW process creation flag from the
// Windows API.
const createNoWindow = 0x08000000

// hideWindow configures cmd so that Windows does not show a console window
// for it.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
