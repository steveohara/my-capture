// Package config loads the runtime configuration for my-capture from
// environment variables.
//
// WHY A SEPARATE PACKAGE?
// Every setting that can be changed without recompiling lives here, so the
// documentation for the environment variables (see Usage) has exactly one
// place to be kept in sync with the code. The UI, audio and video packages
// receive plain values and never read the environment themselves.
//
// ENVIRONMENT VARIABLES
//   - MYCAPTURE_FFMPEG: absolute path of the ffmpeg executable. When unset,
//     the ffmpeg next to the my-capture executable is used (the downloads
//     include one), then one on the PATH, then the usual package manager
//     locations.
//   - MYCAPTURE_AUDIO_DELAY_MS: initial audio delay in milliseconds
//     (default 60, range 20 to 1000). The audio is buffered for this long
//     before it is played, which lets the user line the sound up with the
//     picture, because the video path through ffmpeg is usually slower than
//     the audio path.
//   - MYCAPTURE_DEBUG: set to 1, true or yes to copy everything ffmpeg writes
//     to its error stream onto the my-capture error stream.
//   - MYCAPTURE_NO_RELAUNCH: macOS only. Set to 1, true or yes to stop the
//     program restarting itself inside a temporary application bundle when it
//     is started from a terminal or an IDE.
//
// All four variables are also described by Usage, which is what the
// -help flag prints.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// -----------------------------------------------------------------------
// Names and limits
// -----------------------------------------------------------------------

const (
	// EnvFFmpeg is the name of the environment variable holding the path of
	// the ffmpeg executable.
	EnvFFmpeg = "MYCAPTURE_FFMPEG"

	// EnvAudioDelayMs is the name of the environment variable holding the
	// initial audio delay in milliseconds.
	EnvAudioDelayMs = "MYCAPTURE_AUDIO_DELAY_MS"

	// EnvDebug is the name of the environment variable that enables ffmpeg
	// diagnostic output.
	EnvDebug = "MYCAPTURE_DEBUG"

	// EnvNoRelaunch is the name of the environment variable that stops the
	// macOS program restarting itself inside a temporary application bundle.
	EnvNoRelaunch = "MYCAPTURE_NO_RELAUNCH"
)

const (
	// DefaultAudioDelayMs is the audio delay used when EnvAudioDelayMs is
	// unset or invalid. 60 ms is long enough to ride out scheduling jitter
	// on the capture and playback threads without being noticeable.
	DefaultAudioDelayMs = 60

	// MinAudioDelayMs is the smallest accepted audio delay. Anything lower
	// causes audible crackling because the playback callback regularly runs
	// out of data.
	MinAudioDelayMs = 20

	// MaxAudioDelayMs is the largest accepted audio delay, which is also the
	// upper end of the slider in the user interface.
	MaxAudioDelayMs = 1000
)

// Config holds the settings read from the environment.
//
// Config is a plain value that is filled in once at startup and never
// modified afterwards, so it is safe for concurrent use.
type Config struct {
	// FFmpegPath is the explicit ffmpeg executable path from EnvFFmpeg. It is
	// empty when the variable is unset, in which case the video package
	// searches for ffmpeg itself.
	FFmpegPath string

	// AudioDelayMs is the initial audio delay in milliseconds, always within
	// MinAudioDelayMs and MaxAudioDelayMs.
	AudioDelayMs int

	// Debug is true when ffmpeg diagnostic output should be shown on the
	// error stream.
	Debug bool

	// NoRelaunch is true when the program must not restart itself inside a
	// temporary application bundle. Only used on macOS.
	NoRelaunch bool
}

// -----------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------

// Load reads the configuration from the environment. Invalid values are not
// fatal: a warning is written to os.Stderr and the default is used, so a
// typo in a variable never prevents the window from opening.
func Load() Config {
	return Config{
		FFmpegPath:   envOrDefault(EnvFFmpeg, ""),
		AudioDelayMs: loadAudioDelay(),
		Debug:        envFlag(EnvDebug),
		NoRelaunch:   envFlag(EnvNoRelaunch),
	}
}

// envOrDefault returns the value of the named environment variable, or def
// when the variable is unset or empty.
func envOrDefault(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

// loadAudioDelay parses EnvAudioDelayMs and clamps it to the accepted range.
func loadAudioDelay() int {
	raw := envOrDefault(EnvAudioDelayMs, "")
	if raw == "" {
		return DefaultAudioDelayMs
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a whole number, using %d\n",
			EnvAudioDelayMs, raw, DefaultAudioDelayMs)
		return DefaultAudioDelayMs
	}
	if v < MinAudioDelayMs || v > MaxAudioDelayMs {
		clamped := min(max(v, MinAudioDelayMs), MaxAudioDelayMs)
		fmt.Fprintf(os.Stderr, "warning: %s=%d is outside %d-%d, using %d\n",
			EnvAudioDelayMs, v, MinAudioDelayMs, MaxAudioDelayMs, clamped)
		return clamped
	}
	return v
}

// envFlag reports whether the named environment variable is set to a true
// value: 1, true, yes or on.
func envFlag(name string) bool {
	switch strings.ToLower(envOrDefault(name, "")) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// -----------------------------------------------------------------------
// Help text
// -----------------------------------------------------------------------

// Usage returns the help text describing every environment variable. It is
// appended to the flag help printed by -help, and README.md and SKILL.md
// repeat it, so change all three together.
func Usage() string {
	return fmt.Sprintf(`Environment variables:
  %s
        Absolute path of the ffmpeg executable used to capture video.
        Default: unset. my-capture then uses the ffmpeg next to its own
        executable (the downloads include one), then one on the PATH, then
        /opt/homebrew/bin and /usr/local/bin.

  %s
        Initial audio delay in milliseconds, %d to %d. The audio is
        buffered for this long before it is played. Raise it if the sound
        arrives before the picture, or if the audio crackles. It can also be
        changed while running with the "Audio delay" slider, but slider
        changes are not remembered; set this variable to keep a value.
        Default: %d.

  %s
        Set to 1, true, yes or on to print everything ffmpeg reports on its
        error stream. Use it to find out why a capture device will not open.
        Default: unset (ffmpeg output is hidden unless a capture fails).

  %s
        macOS only. A program started from a terminal or an IDE has no
        application bundle, so macOS would label it with its file name in the
        Dock. To avoid that, my-capture restarts itself once inside a
        temporary "My Capture.app" in the user cache folder. Set to 1, true,
        yes or on to turn that off. It is never done when the program is
        already in an app bundle, or under a debugger.
        Default: unset (restart inside a bundle).
`,
		EnvFFmpeg,
		EnvAudioDelayMs, MinAudioDelayMs, MaxAudioDelayMs, DefaultAudioDelayMs,
		EnvDebug,
		EnvNoRelaunch)
}
