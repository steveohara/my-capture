// Package audio: engine.go owns the miniaudio context and the two devices
// (capture and playback) that make up the audio path.
//
// The Engine is the only type the rest of the program touches. The user
// interface asks it for the lists of input and output devices, and tells it
// which of each to use. Switching either device restarts only that device,
// so changing the headphones does not interrupt capture.
//
// DESIGN NOTES
//   - The capture and playback devices are separate miniaudio devices joined
//     by a ringBuffer, instead of one duplex device, because miniaudio's
//     duplex mode cannot span two different physical devices on every
//     backend, and mixing a USB input with Bluetooth output is the main use.
//   - Both ends use the same fixed format (48 kHz, stereo, 16 bit). miniaudio
//     converts to whatever the hardware really supports.
//   - Volume is intentionally absent. The user controls it with the host's
//     own volume controls, as the specification requires.
package audio

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
)

// -----------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------

// Device is an audio input or output endpoint.
//
// Device values are produced by Engine.Inputs and Engine.Outputs and are
// valid for the lifetime of the Engine that produced them. They are
// read-only and safe for concurrent use.
type Device struct {
	// Name is the human readable device name shown in the pickers. Names are
	// also how selections are remembered between runs, because the numeric
	// identifiers can change when devices are replugged.
	Name string

	// IsDefault is true for the device the operating system currently uses
	// by default.
	IsDefault bool

	// id is the miniaudio identifier of the device. It is nil for the
	// "System default" output entry, which follows the operating system's
	// choice even when that changes while the program runs.
	id *malgo.DeviceID
}

// SystemDefaultName is the name of the pseudo output device that always
// plays on whatever the operating system currently considers the default.
const SystemDefaultName = "System default"

// Engine runs the audio path from an input device to an output device.
//
// Engine is safe for concurrent use: every method takes an internal lock.
// Create one with NewEngine and release it with Close.
type Engine struct {
	// ctx is the miniaudio context that enumerates and opens devices.
	ctx *malgo.AllocatedContext

	// ring carries audio from the capture callback to the playback callback.
	ring *ringBuffer

	// onStopped is told when a device stops by itself, for example when a
	// Bluetooth headset switches off. It may be nil.
	onStopped func(role string)

	// mu guards the fields below.
	mu sync.Mutex

	// capture is the open input device, or nil.
	capture *malgo.Device

	// playback is the open output device, or nil.
	playback *malgo.Device

	// captureQuiet is set just before the input device is deliberately
	// closed, so that its stop notification is not mistaken for the device
	// failing. A new flag is made for every device that is opened.
	captureQuiet *atomic.Bool

	// playbackQuiet is the same flag for the output device.
	playbackQuiet *atomic.Bool

	// closed is true after Close.
	closed bool
}

// ErrClosed is returned by Engine methods called after Close.
var ErrClosed = errors.New("audio engine is closed")

// -----------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------

// NewEngine initialises the audio backend. delayMs is the initial audio
// delay. onStopped, which may be nil, is called with "input" or "output"
// when a running device stops without having been asked to; it runs on an
// audio thread, so it must not call back into the Engine.
func NewEngine(delayMs int, onStopped func(role string)) (*Engine, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("initialise audio backend: %w", err)
	}
	return &Engine{ctx: ctx, ring: newRing(delayMs), onStopped: onStopped}, nil
}

// Close stops both devices and releases the audio backend. The Engine cannot
// be used afterwards.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	e.stopCaptureLocked()
	e.stopPlaybackLocked()
	_ = e.ctx.Uninit() // nothing useful can be done if this fails at exit
	e.ctx.Free()
}

// -----------------------------------------------------------------------
// Device lists
// -----------------------------------------------------------------------

// Inputs returns the audio capture devices, such as the audio channel of a
// USB capture device or a microphone.
func (e *Engine) Inputs() ([]Device, error) {
	return e.list(malgo.Capture, false)
}

// Outputs returns the audio playback devices. The first entry is always the
// "System default" pseudo device.
func (e *Engine) Outputs() ([]Device, error) {
	return e.list(malgo.Playback, true)
}

// list enumerates devices of one kind. withDefault prepends the "System
// default" pseudo device.
func (e *Engine) list(kind malgo.DeviceType, withDefault bool) ([]Device, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrClosed
	}
	infos, err := e.ctx.Devices(kind)
	if err != nil {
		return nil, fmt.Errorf("list audio devices: %w", err)
	}

	var devices []Device
	if withDefault {
		devices = append(devices, Device{Name: SystemDefaultName})
	}
	for i := range infos {
		id := infos[i].ID // own copy, so the pointer given to miniaudio outlives infos
		devices = append(devices, Device{
			Name:      infos[i].Name(),
			IsDefault: infos[i].IsDefault != 0,
			id:        &id,
		})
	}
	return devices, nil
}

// -----------------------------------------------------------------------
// Selecting devices
// -----------------------------------------------------------------------

// SetInput starts capturing from dev, replacing any previous input. A nil
// dev stops capturing. Buffered audio is discarded so that the new source
// does not start with a tail of the old one.
func (e *Engine) SetInput(dev *Device) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	e.stopCaptureLocked()
	e.ring.Reset()
	if dev == nil {
		return nil
	}

	d, quiet, err := e.open(malgo.Capture, dev, "input", func(_, in []byte, _ uint32) {
		e.ring.Write(in)
	})
	if err != nil {
		return fmt.Errorf("open audio input %q: %w", dev.Name, err)
	}
	e.capture, e.captureQuiet = d, quiet
	return nil
}

// SetOutput starts playing on dev, replacing any previous output. A nil dev
// stops playback.
func (e *Engine) SetOutput(dev *Device) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	e.stopPlaybackLocked()
	if dev == nil {
		return nil
	}

	d, quiet, err := e.open(malgo.Playback, dev, "output", func(out, _ []byte, _ uint32) {
		e.ring.Read(out)
	})
	if err != nil {
		return fmt.Errorf("open audio output %q: %w", dev.Name, err)
	}
	e.playback, e.playbackQuiet = d, quiet
	return nil
}

// SetDelay changes the audio delay in milliseconds. The sound drops out
// briefly while the buffer refills.
func (e *Engine) SetDelay(ms int) {
	e.ring.SetDelay(ms)
}

// -----------------------------------------------------------------------
// Opening and closing devices
// -----------------------------------------------------------------------

// open creates and starts one miniaudio device of the given kind. It also
// returns the flag that must be set before the device is deliberately closed.
// Callers must hold e.mu.
func (e *Engine) open(kind malgo.DeviceType, dev *Device, role string, data malgo.DataProc) (*malgo.Device, *atomic.Bool, error) {
	cfg := malgo.DefaultDeviceConfig(kind)
	cfg.SampleRate = sampleRate
	cfg.PeriodSizeInMilliseconds = 10 // small periods keep the added latency low

	sub := &cfg.Playback
	if kind == malgo.Capture {
		sub = &cfg.Capture
	}
	sub.Format = malgo.FormatS16
	sub.Channels = channels
	if dev.id != nil {
		sub.DeviceID = dev.id.Pointer()
	}

	// A stop that happens without the quiet flag set was not asked for by
	// us, so the device went away on its own.
	quiet := new(atomic.Bool)
	callbacks := malgo.DeviceCallbacks{
		Data: data,
		Stop: func() {
			if !quiet.Load() && e.onStopped != nil {
				e.onStopped(role)
			}
		},
	}

	d, err := malgo.InitDevice(e.ctx.Context, cfg, callbacks)
	if err != nil {
		return nil, nil, err
	}
	if err := d.Start(); err != nil {
		quiet.Store(true)
		d.Uninit()
		return nil, nil, err
	}
	return d, quiet, nil
}

// stopCaptureLocked closes the input device. The caller must hold e.mu.
func (e *Engine) stopCaptureLocked() {
	if e.capture != nil {
		e.captureQuiet.Store(true)
		e.capture.Uninit()
		e.capture = nil
	}
}

// stopPlaybackLocked closes the output device. The caller must hold e.mu.
func (e *Engine) stopPlaybackLocked() {
	if e.playback != nil {
		e.playbackQuiet.Store(true)
		e.playback.Uninit()
		e.playback = nil
	}
}
