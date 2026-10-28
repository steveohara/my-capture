// Package main implements my-capture, a small desktop program that shows the
// picture and plays the sound of a USB video capture device.
//
// WHAT IT DOES
// The user picks a video capture device, an audio input and an audio output
// device in a window. The video appears in the window, which can be resized,
// moved between monitors and made full screen. The sound is played on any
// output device the computer has, including Bluetooth headphones. Nothing is
// recorded, and there is no volume control, because the external device and
// the operating system already provide those.
//
// HOW IT FITS TOGETHER
//   - internal/video runs ffmpeg (AVFoundation on macOS, DirectShow on
//     Windows, as OBS Studio does) and reads raw RGBA frames from it.
//   - internal/audio captures and plays sound with miniaudio, with the
//     input and output chosen independently.
//   - internal/ui is the Fyne window that joins the two.
//   - internal/config reads the environment variables listed below.
//
// ENVIRONMENT VARIABLES
//   - MYCAPTURE_FFMPEG: path of the ffmpeg executable (default: searched for).
//   - MYCAPTURE_AUDIO_DELAY_MS: initial audio delay in milliseconds,
//     20 to 1000 (default 60).
//   - MYCAPTURE_DEBUG: 1, true, yes or on shows ffmpeg's messages on stderr.
//   - MYCAPTURE_NO_RELAUNCH: macOS only. 1, true, yes or on stops the program
//     restarting itself inside a temporary app bundle.
//
// Run "my-capture -help" for the full description of each.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"my-capture/internal/audio"
	"my-capture/internal/config"
	"my-capture/internal/ui"
	"my-capture/internal/video"
)

// version is the release this program was built from, such as "v0.4.0". The
// Makefile sets it at link time (-ldflags "-X main.version=..."); a plain
// "go build" leaves it as "dev".
var version = "dev"

// main parses the command line and either prints the devices or opens the
// window.
func main() {
	listDevices := flag.Bool("list-devices", false,
		"print the video devices, audio inputs and audio outputs, then exit")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("my-capture %s\n", version)
		return
	}

	cfg := config.Load()

	if *listDevices {
		if err := printDevices(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ui.Run(cfg, version)
}

// usage prints the help text for -help, including every environment
// variable.
func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, `my-capture shows the picture and plays the sound of a USB video capture device.

Usage:
  my-capture [flags]

Pick the video device, audio input and audio output in the window. Double
click the picture, or press F or F11, for full screen; press Escape to leave
it. Press A to change how the picture fills the window (fit, stretch, crop,
or a fixed shape such as 16:9), and C to hide or show the controls. Press F1,
or use the Help menu, for the help built into the window; About shows the
version. Nothing is recorded. Use the external device and the computer's own
volume control to adjust the sound.

The device lists update by themselves every few seconds when devices are
plugged in or removed.

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(out, "\n%s", config.Usage())
}

// printDevices lists every device the window would offer, for diagnosing
// problems without opening the window.
func printDevices(cfg config.Config) error {
	ffmpeg, err := video.FindFFmpeg(cfg.FFmpegPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: video devices unavailable: %v\n", err)
	} else {
		devs, err := video.ListDevices(context.Background(), ffmpeg)
		if err != nil {
			return fmt.Errorf("list video devices: %w", err)
		}
		fmt.Println("Video devices:")
		for _, d := range devs {
			fmt.Printf("  %s\n", d.Name)
		}
	}

	engine, err := audio.NewEngine(cfg.AudioDelayMs, nil)
	if err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	defer engine.Close()

	inputs, err := engine.Inputs()
	if err != nil {
		return fmt.Errorf("list audio inputs: %w", err)
	}
	fmt.Println("Audio inputs:")
	for _, d := range inputs {
		fmt.Printf("  %s\n", d.Name)
	}

	outputs, err := engine.Outputs()
	if err != nil {
		return fmt.Errorf("list audio outputs: %w", err)
	}
	fmt.Println("Audio outputs:")
	for _, d := range outputs {
		fmt.Printf("  %s\n", d.Name)
	}
	return nil
}
