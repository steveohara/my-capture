// Package video captures live video from a USB capture device by running
// ffmpeg as a child process and reading raw RGBA frames from its output.
//
// WHY FFMPEG?
// OBS Studio reaches capture devices through AVFoundation on macOS and
// DirectShow on Windows. ffmpeg exposes exactly those two operating system
// frameworks as its "avfoundation" and "dshow" input devices, so running it
// as a child process gives the same device coverage without writing and
// maintaining cgo bindings for either framework, and without a dependency
// on OpenCV.
//
// This file contains the pure text parsers that turn ffmpeg's diagnostic
// output into Go values. They have no side effects, which keeps the
// platform specific scraping separate from the process handling in
// capture.go and device.go.
//
// ffmpeg writes device lists, supported modes and stream details to its
// error stream as free text rather than as a documented machine readable
// format, so every parser here is deliberately forgiving: a line that does
// not match is skipped, never treated as an error.
package video

import (
	"regexp"
	"strconv"
	"strings"
)

// -----------------------------------------------------------------------
// Regular expressions
// -----------------------------------------------------------------------

var (
	// avfDeviceRe matches a device line of "-f avfoundation -list_devices",
	// for example "[AVFoundation indev @ 0x7f8] [0] FaceTime HD Camera".
	// Group 1 is the device index and group 2 is the device name.
	avfDeviceRe = regexp.MustCompile(`^\[[^\]]*\]\s+\[(\d+)\]\s+(.+?)\s*$`)

	// avfModeRe matches a supported mode line printed by the avfoundation
	// input device, for example "  1920x1080@[30.000000 60.000000]fps".
	// Groups are width, height, minimum fps and maximum fps.
	avfModeRe = regexp.MustCompile(`(\d+)x(\d+)@\[([\d.]+)\s+([\d.]+)\]fps`)

	// dshowDeviceRe matches a device line of "-f dshow -list_devices", for
	// example `[dshow @ 0000] "USB Video" (video)`. Group 1 is the name and
	// group 2 the kind list, such as "video", "audio" or "video, audio".
	dshowDeviceRe = regexp.MustCompile(`"([^"]+)"\s+\(([a-z, ]+)\)`)

	// dshowLegacyDeviceRe matches the device line format used before
	// ffmpeg 5, where the kind is given by the preceding section heading
	// and the line holds only the quoted name.
	dshowLegacyDeviceRe = regexp.MustCompile(`\]\s+"([^"]+)"\s*$`)

	// dshowAltNameRe matches the unique "Alternative name" line printed
	// below each DirectShow device. Group 1 is the alternative name.
	dshowAltNameRe = regexp.MustCompile(`Alternative name\s+"([^"]+)"`)

	// dshowModeRe matches a line of "-list_options true -f dshow", for
	// example "pixel_format=yuyv422  min s=640x480 fps=5 max s=640x480
	// fps=30". Groups are the kind (pixel_format or vcodec), its value, and
	// the minimum and maximum size and frame rate.
	dshowModeRe = regexp.MustCompile(
		`(pixel_format|vcodec)=(\S+)\s+min s=(\d+)x(\d+) fps=([\d.]+)\s+max s=(\d+)x(\d+) fps=([\d.]+)`)

	// parenRe matches a parenthesised group. Stream description lines are
	// stripped of these before the frame size is searched for, because they
	// contain hexadecimal tags such as "0x41424752" that look like sizes.
	parenRe = regexp.MustCompile(`\([^)]*\)`)

	// streamSizeRe finds the frame size inside a stream description line.
	streamSizeRe = regexp.MustCompile(`Video:.*?\b(\d{2,5})x(\d{2,5})\b`)
)

// -----------------------------------------------------------------------
// Device lists
// -----------------------------------------------------------------------

// parseAVFoundationDevices extracts the video devices from the output of
// "ffmpeg -f avfoundation -list_devices true -i ”". Screen capture pseudo
// devices are omitted because this application is for capture hardware.
// The returned Device.ID is the avfoundation device index.
func parseAVFoundationDevices(output string) []Device {
	var devices []Device
	inVideo := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.Contains(line, "video devices"):
			inVideo = true
			continue
		case strings.Contains(line, "audio devices"):
			inVideo = false
			continue
		}
		if !inVideo {
			continue
		}
		m := avfDeviceRe.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(m[2], "Capture screen") {
			continue
		}
		devices = append(devices, Device{ID: m[1], Name: m[2]})
	}
	return devices
}

// parseDShowDevices extracts the video devices from the output of
// "ffmpeg -list_devices true -f dshow -i dummy". When a device reports an
// alternative name it becomes Device.ID, because it stays unique when two
// identical capture cards are plugged in; otherwise the display name is used.
func parseDShowDevices(output string) []Device {
	var devices []Device
	section := "" // "video" or "audio"; only used by the pre-5.0 format
	last := -1    // index in devices of the device the next alt name belongs to

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")

		if strings.Contains(line, "DirectShow video devices") {
			section = "video"
			continue
		}
		if strings.Contains(line, "DirectShow audio devices") {
			section = "audio"
			continue
		}

		if m := dshowAltNameRe.FindStringSubmatch(line); m != nil {
			if last >= 0 {
				devices[last].ID = m[1]
			}
			continue
		}

		last = -1
		if m := dshowDeviceRe.FindStringSubmatch(line); m != nil {
			if strings.Contains(m[2], "video") {
				devices = append(devices, Device{ID: m[1], Name: m[1]})
				last = len(devices) - 1
			}
			continue
		}
		if m := dshowLegacyDeviceRe.FindStringSubmatch(line); m != nil && section == "video" {
			devices = append(devices, Device{ID: m[1], Name: m[1]})
			last = len(devices) - 1
		}
	}
	return devices
}

// -----------------------------------------------------------------------
// Capture modes
// -----------------------------------------------------------------------

// parseAVFoundationModes extracts the supported modes from the error that
// the avfoundation input device prints when it is asked for a frame size it
// cannot provide. The highest frame rate of each range is used.
func parseAVFoundationModes(output string) []Mode {
	var modes []Mode
	for _, m := range avfModeRe.FindAllStringSubmatch(output, -1) {
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		fps, _ := strconv.ParseFloat(m[4], 64)
		modes = appendMode(modes, Mode{Width: w, Height: h, FPS: fps})
	}
	return modes
}

// parseDShowModes extracts the supported modes from the output of
// "ffmpeg -list_options true -f dshow -i video=NAME". The maximum size and
// frame rate of each reported range is used.
func parseDShowModes(output string) []Mode {
	var modes []Mode
	for _, m := range dshowModeRe.FindAllStringSubmatch(output, -1) {
		w, _ := strconv.Atoi(m[6])
		h, _ := strconv.Atoi(m[7])
		fps, _ := strconv.ParseFloat(m[8], 64)
		mode := Mode{Width: w, Height: h, FPS: fps}
		if m[1] == "vcodec" {
			mode.Codec = m[2]
		} else {
			mode.PixelFormat = m[2]
		}
		modes = appendMode(modes, mode)
	}
	return modes
}

// appendMode appends m to modes unless an identical mode is already present
// or m has no usable size. DirectShow reports the same mode once per
// capture pin, so duplicates are common.
func appendMode(modes []Mode, m Mode) []Mode {
	if m.Width <= 0 || m.Height <= 0 {
		return modes
	}
	for _, existing := range modes {
		if existing == m {
			return modes
		}
	}
	return append(modes, m)
}

// -----------------------------------------------------------------------
// Output stream size
// -----------------------------------------------------------------------

// sizeScanner finds the frame size of ffmpeg's output stream in its
// diagnostic output. The size is needed to know how many bytes make up one
// raw frame on the pipe.
//
// The input stream is described by a similar "Stream #0:0: Video:" line
// earlier in the output, so lines are ignored until the "Output #0" heading
// has been seen.
type sizeScanner struct {
	// inOutput becomes true once the "Output #" heading has been fed.
	inOutput bool
}

// Feed examines one line of ffmpeg diagnostic output. It returns the frame
// size and true when the line describes the output video stream.
func (s *sizeScanner) Feed(line string) (width, height int, ok bool) {
	if strings.HasPrefix(strings.TrimSpace(line), "Output #") {
		s.inOutput = true
		return 0, 0, false
	}
	if !s.inOutput || !strings.Contains(line, "Stream #") {
		return 0, 0, false
	}
	m := streamSizeRe.FindStringSubmatch(parenRe.ReplaceAllString(line, ""))
	if m == nil {
		return 0, 0, false
	}
	width, _ = strconv.Atoi(m[1])
	height, _ = strconv.Atoi(m[2])
	return width, height, width > 0 && height > 0
}
