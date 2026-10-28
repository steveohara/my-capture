// Package audio captures sound from an input device and plays it on an
// output device chosen independently of it, using miniaudio through the
// malgo bindings.
//
// WHY MINIAUDIO?
// miniaudio talks to WASAPI on Windows and Core Audio on macOS, enumerates
// every playback and capture endpoint (including Bluetooth headphones and
// USB capture devices), and converts sample rate and channel count on the
// fly, so both ends can be opened in one fixed format regardless of what the
// hardware natively uses.
//
// This file holds the ring buffer that joins the two ends. The capture
// device and the playback device each have their own clock, and the two
// clocks never run at exactly the same speed. The ring buffer absorbs the
// short term jitter, and deliberately drops or repeats silence when the
// clocks drift far enough apart, so the delay stays near the configured value
// rather than growing without bound.
package audio

import "sync"

// -----------------------------------------------------------------------
// Audio format
// -----------------------------------------------------------------------

const (
	// sampleRate is the rate in frames per second used by both devices.
	// miniaudio resamples to and from the hardware rate where they differ.
	sampleRate = 48000

	// channels is the channel count used by both devices.
	channels = 2

	// bytesPerFrame is the size of one interleaved stereo frame of signed
	// 16 bit samples.
	bytesPerFrame = channels * 2

	// bytesPerMs is the number of bytes that make up one millisecond.
	bytesPerMs = sampleRate / 1000 * bytesPerFrame

	// ringCapacityMs is the size of the ring buffer in milliseconds. It must
	// comfortably exceed the largest permitted delay plus the drift slack.
	ringCapacityMs = 2500

	// driftSlackMs is how far above the target delay the buffered amount may
	// grow before the oldest audio is discarded.
	driftSlackMs = 100
)

// -----------------------------------------------------------------------
// Ring buffer
// -----------------------------------------------------------------------

// ringBuffer is a byte FIFO between the capture callback (the writer) and
// the playback callback (the reader). Both callbacks run on real time audio
// threads, so every method does bounded work and never allocates.
//
// ringBuffer is safe for concurrent use. Its contents are always a whole
// number of audio frames.
type ringBuffer struct {
	// mu guards every field below.
	mu sync.Mutex

	// buf is the circular storage.
	buf []byte

	// rd is the index of the oldest buffered byte.
	rd int

	// n is the number of buffered bytes.
	n int

	// primed is false while the reader is waiting for the buffer to fill to
	// the target delay, during which silence is played.
	primed bool

	// targetBytes is the delay, in bytes, that must be buffered before
	// playback starts and that is restored after an underrun.
	targetBytes int
}

// newRing returns an empty ring buffer holding the default target delay.
func newRing(delayMs int) *ringBuffer {
	return &ringBuffer{
		buf:         make([]byte, ringCapacityMs*bytesPerMs),
		targetBytes: delayMs * bytesPerMs,
	}
}

// SetDelay changes the target delay. The buffered audio is discarded and the
// reader re-primes, so the new delay takes effect immediately at the price of
// a short gap in the sound.
func (r *ringBuffer) SetDelay(ms int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targetBytes = ms * bytesPerMs
	r.rd, r.n, r.primed = 0, 0, false
}

// Reset discards all buffered audio and makes the reader wait for the
// target delay again.
func (r *ringBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rd, r.n, r.primed = 0, 0, false
}

// Write appends p, which is trimmed to a whole number of frames. When the
// buffer is full the oldest audio is overwritten, because stale sound is
// worth less than fresh sound.
func (r *ringBuffer) Write(p []byte) {
	p = p[:len(p)-len(p)%bytesPerFrame]
	if len(p) > len(r.buf) {
		p = p[len(p)-len(r.buf):]
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if over := r.n + len(p) - len(r.buf); over > 0 {
		r.rd = (r.rd + over) % len(r.buf)
		r.n -= over
	}
	wr := (r.rd + r.n) % len(r.buf)
	first := copy(r.buf[wr:], p)
	copy(r.buf, p[first:])
	r.n += len(p)
}

// Read fills out with the oldest buffered audio. It writes silence instead
// when the buffer is still filling to the target delay, and after an
// underrun it plays what is left, pads with silence and goes back to
// filling. Read always fills out completely.
func (r *ringBuffer) Read(out []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clock drift: if the writer has run ahead, skip to the target so the
	// delay does not grow without bound.
	if limit := r.targetBytes + driftSlackMs*bytesPerMs; r.n > limit {
		drop := (r.n - r.targetBytes) / bytesPerFrame * bytesPerFrame
		r.rd = (r.rd + drop) % len(r.buf)
		r.n -= drop
	}

	if !r.primed {
		if r.n < r.targetBytes {
			clear(out)
			return
		}
		r.primed = true
	}

	take := min(len(out), r.n)
	first := copy(out[:take], r.buf[r.rd:])
	copy(out[first:take], r.buf)
	r.rd = (r.rd + take) % len(r.buf)
	r.n -= take

	if take < len(out) {
		clear(out[take:]) // underrun: the writer has not kept up
		r.primed = false
	}
}
