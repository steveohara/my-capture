// Package video: device.go finds ffmpeg and asks it which capture devices
// exist and which video modes each device supports.
//
// The questions are put to ffmpeg itself, with the same operating system
// framework that the capture will later use (AVFoundation on macOS,
// DirectShow on Windows), so the answers always match what ffmpeg can open.
// The text that comes back is turned into Go values by the parsers in
// parse.go.
package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// -----------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------

// Device is a video capture device reported by the operating system.
//
// Device is a plain value and safe for concurrent use.
type Device struct {
	// Name is the human readable name shown in the device picker.
	Name string

	// ID identifies the device to ffmpeg. On macOS it is the AVFoundation
	// device index, which can change when devices are plugged in, so it is
	// only valid until the next call to ListDevices. On Windows it is the
	// DirectShow alternative name when available, otherwise the device name.
	ID string
}

// Mode is one resolution and frame rate combination offered by a device.
//
// A Mode with all fields zero means "let the device choose its default".
// Mode is comparable, which parse.go relies on to remove duplicates.
type Mode struct {
	// Width is the frame width in pixels.
	Width int

	// Height is the frame height in pixels.
	Height int

	// FPS is the frame rate in frames per second.
	FPS float64

	// PixelFormat is the ffmpeg name of the uncompressed pixel format, such
	// as "yuyv422". It is only set for DirectShow modes.
	PixelFormat string

	// Codec is the compressed format the device delivers, such as "mjpeg".
	// It is only set for DirectShow modes, and exclusive with PixelFormat.
	Codec string
}

// String formats the mode for the mode picker, for example
// "1920x1080 @ 60 fps (mjpeg)".
func (m Mode) String() string {
	if m == (Mode{}) {
		return "Device default"
	}
	s := fmt.Sprintf("%dx%d @ %s fps", m.Width, m.Height,
		strconv.FormatFloat(m.FPS, 'f', -1, 64))
	switch {
	case m.Codec != "":
		s += " (" + m.Codec + ")"
	case m.PixelFormat != "":
		s += " (" + m.PixelFormat + ")"
	}
	return s
}

// -----------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------

const (
	// probeTimeout bounds every short ffmpeg query so that a misbehaving
	// driver cannot hang the user interface forever.
	probeTimeout = 15 * time.Second
)

// ErrFFmpegNotFound is returned by FindFFmpeg when no ffmpeg executable
// could be located. The caller should tell the user to install ffmpeg or to
// set the MYCAPTURE_FFMPEG environment variable.
var ErrFFmpegNotFound = errors.New("ffmpeg executable not found")

// ErrUnsupportedPlatform is returned when video capture is requested on an
// operating system other than macOS or Windows.
var ErrUnsupportedPlatform = errors.New("video capture is only supported on macOS and Windows")

// -----------------------------------------------------------------------
// Locating ffmpeg
// -----------------------------------------------------------------------

// FindFFmpeg returns the path of the ffmpeg executable. An explicit path
// (from MYCAPTURE_FFMPEG) is used as is. Otherwise the directory of the
// running executable is tried first, so a copy of ffmpeg can be shipped
// next to the application, then the PATH, then the package manager
// directories that macOS GUI applications do not have on their PATH.
func FindFFmpeg(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("ffmpeg path %q: %w", explicit, err)
		}
		return explicit, nil
	}

	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}

	var candidates []string
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), exe))
	}
	if p, err := exec.LookPath(exe); err == nil {
		candidates = append(candidates, p)
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg")
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", ErrFFmpegNotFound
}

// -----------------------------------------------------------------------
// Device and mode discovery
// -----------------------------------------------------------------------

// FFmpegVersion returns the first line of "ffmpeg -version", such as
// "ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers". It is
// shown in the About box, so a bug report says which ffmpeg was used.
func FFmpegVersion(ctx context.Context, ffmpeg string) (string, error) {
	out, err := runProbe(ctx, ffmpeg, "-hide_banner", "-version")
	if err != nil {
		return "", fmt.Errorf("ffmpeg version: %w", err)
	}
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(line), nil
}

// ListDevices returns the video capture devices known to the operating
// system. The list is empty, with a nil error, when no device is connected.
func ListDevices(ctx context.Context, ffmpeg string) ([]Device, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := runProbe(ctx, ffmpeg, "-hide_banner", "-f", "avfoundation",
			"-list_devices", "true", "-i", "")
		if err != nil {
			return nil, fmt.Errorf("list avfoundation devices: %w", err)
		}
		return parseAVFoundationDevices(out), nil
	case "windows":
		out, err := runProbe(ctx, ffmpeg, "-hide_banner", "-list_devices", "true",
			"-f", "dshow", "-i", "dummy")
		if err != nil {
			return nil, fmt.Errorf("list dshow devices: %w", err)
		}
		return parseDShowDevices(out), nil
	}
	return nil, ErrUnsupportedPlatform
}

// ListModes returns the modes that dev supports, best first: modes no wider
// than 1920 pixels come before larger ones (raw 4K frames are too heavy to
// copy around in real time), then wider frames before narrower (which puts
// 1920x1080 ahead of odd shapes such as a 1552x1552 square crop), then
// taller before shorter, then higher frame rates before lower. The list can be empty when the device does not
// report its modes, in which case the caller should use the zero Mode.
func ListModes(ctx context.Context, ffmpeg string, dev Device) ([]Mode, error) {
	var modes []Mode
	switch runtime.GOOS {
	case "darwin":
		// Asking for a 1x1 frame can never succeed, and as a courtesy the
		// input device then prints every mode it does support.
		out, err := runProbe(ctx, ffmpeg, "-hide_banner", "-f", "avfoundation",
			"-framerate", "1", "-video_size", "1x1", "-i", dev.ID+":none")
		if err != nil {
			return nil, fmt.Errorf("list avfoundation modes: %w", err)
		}
		modes = parseAVFoundationModes(out)
	case "windows":
		out, err := runProbe(ctx, ffmpeg, "-hide_banner", "-list_options", "true",
			"-f", "dshow", "-i", "video="+dev.ID)
		if err != nil {
			return nil, fmt.Errorf("list dshow modes: %w", err)
		}
		modes = parseDShowModes(out)
	default:
		return nil, ErrUnsupportedPlatform
	}

	sort.SliceStable(modes, func(i, j int) bool {
		a, b := modes[i], modes[j]
		if (a.Width <= 1920) != (b.Width <= 1920) {
			return a.Width <= 1920
		}
		if a.Width != b.Width {
			return a.Width > b.Width
		}
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		return a.FPS > b.FPS
	})
	return modes, nil
}

// runProbe runs ffmpeg with args and returns everything it wrote to its
// error stream. ffmpeg always exits with a non-zero status when asked to
// list devices or options, because there is no media to process, so a
// non-zero exit is not an error here. Only a failure to start ffmpeg is.
func runProbe(ctx context.Context, ffmpeg string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return "", err
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("ffmpeg did not answer within %s", probeTimeout)
	}
	return string(out), nil
}
