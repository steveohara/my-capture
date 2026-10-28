// Package video: capture.go runs one ffmpeg capture process and delivers
// its frames to a callback.
//
// HOW IT WORKS
// ffmpeg opens the device with AVFoundation (macOS) or DirectShow (Windows),
// converts every frame to 8 bit RGBA and writes the raw bytes to its standard
// output. Raw frames have no framing, so the reader needs the frame size to
// know where one frame ends. ffmpeg announces the size of its output stream
// on the error stream shortly after starting, and that line is parsed by
// sizeScanner in parse.go.
//
// Three goroutines cooperate, all of which end when the process ends:
//   - the stderr goroutine reads ffmpeg's diagnostics, finds the frame size
//     and keeps the last few lines for error messages;
//   - the frame goroutine waits for the size and then reads frames;
//   - the supervisor goroutine waits for the process to exit and reports an
//     unexpected exit.
//
// Audio is not captured here. It is captured and played by the audio package
// so the user can pick its input and output devices independently, which is
// also how OBS Studio treats audio.
package video

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// -----------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------

const (
	// bytesPerPixel is the size of one RGBA pixel.
	bytesPerPixel = 4

	// frameBuffers is the number of frame buffers cycled through. The frame
	// callback receives a buffer that the capture will not overwrite until
	// frameBuffers-1 further frames have been read, which gives the user
	// interface time to finish uploading the picture to the graphics card.
	frameBuffers = 4

	// sizeTimeout is how long to wait for ffmpeg to announce the frame size.
	// Capture hardware can take several seconds to wake up, and on macOS the
	// first run waits for the user to answer the camera permission prompt.
	sizeTimeout = 30 * time.Second

	// stderrTail is the number of ffmpeg error lines kept for error messages.
	stderrTail = 12
)

// -----------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------

// Options describes the capture to start.
type Options struct {
	// FFmpegPath is the path of the ffmpeg executable, from FindFFmpeg.
	FFmpegPath string

	// Device is the capture device to open.
	Device Device

	// Mode is the resolution and frame rate to request. The zero Mode lets
	// the device choose.
	Mode Mode

	// Debug copies ffmpeg's error stream to os.Stderr.
	Debug bool
}

// Capture is a running ffmpeg capture process.
//
// A Capture is created by Start and ended by Stop. Stop may be called more
// than once and from any goroutine.
type Capture struct {
	// cancel kills the ffmpeg process by cancelling its context.
	cancel context.CancelFunc

	// stopped is set by Stop so that the supervisor can tell a deliberate
	// shutdown from a crash and stay quiet about the former.
	stopped atomic.Bool

	// done is closed after the process has exited and every goroutine has
	// finished.
	done chan struct{}
}

// -----------------------------------------------------------------------
// Starting and stopping
// -----------------------------------------------------------------------

// Start launches ffmpeg and returns immediately.
//
// onFrame is called from a capture goroutine for every frame. The *image.RGBA
// it receives is reused after a few more frames, so the callback must not
// keep it, and it must return quickly: ffmpeg is stalled while it runs.
// onFrame is never called after Stop returns.
//
// onExit is called at most once, and only if ffmpeg stops without Stop having
// been called, for instance when the device is unplugged or cannot be opened.
// Its argument describes the failure. It runs on a capture goroutine, so it
// must not call Stop, which would wait for itself.
func Start(parent context.Context, opts Options, onFrame func(*image.RGBA), onExit func(error)) (*Capture, error) {
	args, err := captureArgs(opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, opts.FFmpegPath, args...)
	cmd.WaitDelay = 2 * time.Second // do not hang on a pipe held open by a grandchild
	hideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	c := &Capture{cancel: cancel, done: make(chan struct{})}
	tail := &lineTail{max: stderrTail}
	sizeCh := make(chan image.Point, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	// Lifetime: ends when ffmpeg closes its error stream, i.e. on exit.
	// Closing sizeCh on the way out wakes the frame goroutine if ffmpeg
	// died before it announced a size.
	go func() {
		defer wg.Done()
		defer close(sizeCh)
		readDiagnostics(stderr, opts.Debug, tail, sizeCh)
	}()

	// Lifetime: ends when the size never arrives (timeout, cancel or early
	// exit), or when stdout reaches EOF or errors, i.e. when ffmpeg exits.
	go func() {
		defer wg.Done()
		select {
		case size, ok := <-sizeCh:
			if ok {
				readFrames(stdout, size, onFrame)
			}
		case <-time.After(sizeTimeout):
			cancel() // ffmpeg never produced a picture; kill it
		case <-ctx.Done():
		}
	}()

	// Lifetime: ends when ffmpeg has exited and been reaped.
	go func() {
		// The readers must finish before Wait, which closes the pipes.
		wg.Wait()
		waitErr := cmd.Wait()
		cancel()
		if !c.stopped.Load() && onExit != nil {
			onExit(exitError(waitErr, tail.String()))
		}
		close(c.done)
	}()

	return c, nil
}

// Stop kills the ffmpeg process and waits for the capture goroutines to
// finish. After Stop returns, onFrame is not called again.
func (c *Capture) Stop() {
	c.stopped.Store(true)
	c.cancel()
	<-c.done
}

// -----------------------------------------------------------------------
// Command line
// -----------------------------------------------------------------------

// captureArgs builds the ffmpeg command line for opts.
func captureArgs(opts Options) ([]string, error) {
	args := []string{"-hide_banner", "-nostats", "-loglevel", "info", "-fflags", "nobuffer"}

	switch runtime.GOOS {
	case "darwin":
		args = append(args, "-f", "avfoundation")
		args = append(args, modeArgs(opts.Mode)...)
		// "none" after the colon means no audio: audio is handled elsewhere.
		args = append(args, "-i", opts.Device.ID+":none")
	case "windows":
		args = append(args, "-f", "dshow", "-rtbufsize", "64M")
		args = append(args, modeArgs(opts.Mode)...)
		if opts.Mode.Codec != "" {
			args = append(args, "-vcodec", opts.Mode.Codec)
		} else if opts.Mode.PixelFormat != "" {
			args = append(args, "-pixel_format", opts.Mode.PixelFormat)
		}
		args = append(args, "-i", "video="+opts.Device.ID)
	default:
		return nil, ErrUnsupportedPlatform
	}

	// -fps_mode passthrough: write each frame exactly once, as it arrives.
	// Without it ffmpeg pads the output to a constant frame rate by repeating
	// frames, and when the first camera timestamps are odd it repeats the
	// same frame hundreds of times a second, which shows as a frozen picture.
	// Needs ffmpeg 5.1 or later.
	//
	// -pix_fmt rgba: the layout of image.RGBA, so frames need no conversion
	// in Go. -f rawvideo to pipe:1 writes bare frames to standard output.
	args = append(args, "-an", "-fps_mode", "passthrough", "-pix_fmt", "rgba", "-f", "rawvideo", "pipe:1")
	return args, nil
}

// modeArgs returns the input options that request m, or nothing for the
// zero Mode.
func modeArgs(m Mode) []string {
	if m.Width == 0 || m.Height == 0 {
		return nil
	}
	args := []string{"-video_size", fmt.Sprintf("%dx%d", m.Width, m.Height)}
	if m.FPS > 0 {
		args = append(args, "-framerate", strconv.FormatFloat(m.FPS, 'f', -1, 64))
	}
	return args
}

// -----------------------------------------------------------------------
// Reading ffmpeg output
// -----------------------------------------------------------------------

// readDiagnostics reads ffmpeg's error stream until it closes. It sends the
// output frame size to sizeCh the first time it sees it and records the most
// recent lines in tail. The stream must always be drained, or ffmpeg blocks
// when the pipe fills.
func readDiagnostics(r io.Reader, debug bool, tail *lineTail, sizeCh chan<- image.Point) {
	var scanner sizeScanner
	sent := false

	s := bufio.NewScanner(r)
	s.Split(scanLinesCR)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		line := s.Text()
		if debug {
			fmt.Fprintln(os.Stderr, "ffmpeg: "+line)
		}
		tail.Add(line)
		if sent {
			continue
		}
		if w, h, ok := scanner.Feed(line); ok {
			sent = true
			sizeCh <- image.Pt(w, h) // buffered with room for exactly one value
		}
	}
}

// readFrames reads raw RGBA frames of the given size from r and passes each
// to onFrame, until r fails or reaches EOF.
func readFrames(r io.Reader, size image.Point, onFrame func(*image.RGBA)) {
	rect := image.Rect(0, 0, size.X, size.Y)
	var bufs [frameBuffers]*image.RGBA
	for i := range bufs {
		bufs[i] = &image.RGBA{
			Pix:    make([]byte, size.X*size.Y*bytesPerPixel),
			Stride: size.X * bytesPerPixel,
			Rect:   rect,
		}
	}
	for n := 0; ; n++ {
		img := bufs[n%frameBuffers]
		if _, err := io.ReadFull(r, img.Pix); err != nil {
			return // EOF or closed pipe: the process is ending
		}
		onFrame(img)
	}
}

// scanLinesCR is a bufio.SplitFunc that splits on both "\n" and "\r".
// ffmpeg ends its progress lines with a bare carriage return, which the
// standard line splitter would not treat as a line break.
func scanLinesCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// -----------------------------------------------------------------------
// Error reporting
// -----------------------------------------------------------------------

// lineTail remembers the last few lines it was given. It is safe for
// concurrent use.
type lineTail struct {
	// max is the number of lines to keep.
	max int

	// mu guards lines.
	mu sync.Mutex

	// lines holds the most recent non-empty lines, oldest first.
	lines []string
}

// Add records a line, discarding the oldest when the tail is full. Blank
// lines are ignored.
func (t *lineTail) Add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

// String returns the last few recorded lines joined by newlines.
func (t *lineTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := min(len(t.lines), 4)
	return strings.Join(t.lines[len(t.lines)-n:], "\n")
}

// exitError builds the error reported to onExit from the process exit
// status and the last lines of ffmpeg's diagnostics, which usually state the
// actual cause such as "Selected framerate is not supported".
func exitError(waitErr error, tail string) error {
	if waitErr == nil {
		waitErr = errors.New("ffmpeg stopped unexpectedly")
	}
	if tail == "" {
		return waitErr
	}
	return fmt.Errorf("%w\n%s", waitErr, tail)
}
