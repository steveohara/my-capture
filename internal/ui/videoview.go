// Package ui implements the my-capture window: the device pickers and the
// video picture.
//
// WHY FYNE?
// Fyne is a pure Go toolkit that draws with OpenGL on macOS and Windows. It
// provides native windows with resizing, full screen and movement between
// monitors, and ready-made drop-down lists, so the program needs no
// platform specific window code.
//
// This file contains VideoView, the widget that shows captured frames. It
// receives frames from the capture goroutine and hands the newest one to the
// Fyne main thread, dropping frames when the screen cannot keep up so the
// picture never lags behind the source.
package ui

import (
	"image"
	"image/color"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// -----------------------------------------------------------------------
// Aspect modes
// -----------------------------------------------------------------------

// AspectMode says how the picture is fitted into the video area.
//
// AspectMode is a plain value and safe for concurrent use.
type AspectMode struct {
	// Name is the text shown in the aspect picker. It is also how the choice
	// is remembered between runs, so renaming a mode forgets it.
	Name string

	// Fill is how Fyne scales the picture: contain keeps the source shape
	// and leaves black bars, cover keeps the shape and crops the edges, and
	// stretch ignores the shape. It is ignored when Ratio is set.
	Fill canvas.ImageFill

	// Ratio, when not zero, is the width divided by height the picture is
	// forced to, whatever shape the source has. The picture is stretched to
	// that shape and then fitted into the area with black bars.
	Ratio float64
}

// AspectModes lists the modes offered to the user. The first is the default.
var AspectModes = []AspectMode{
	{Name: "Fit (keep source shape)", Fill: canvas.ImageFillContain},
	{Name: "Stretch to window", Fill: canvas.ImageFillStretch},
	{Name: "Fill window (crop edges)", Fill: canvas.ImageFillCover},
	{Name: "16:9", Ratio: 16.0 / 9},
	{Name: "16:10", Ratio: 16.0 / 10},
	{Name: "4:3", Ratio: 4.0 / 3},
	{Name: "21:9", Ratio: 21.0 / 9},
	{Name: "1:1", Ratio: 1},
}

// -----------------------------------------------------------------------
// VideoView
// -----------------------------------------------------------------------

const (
	// minViewWidth and minViewHeight are the smallest size, in Fyne units,
	// that the video area asks for. They keep the picture from collapsing
	// when the window is made small.
	minViewWidth  = 320
	minViewHeight = 180
)

// VideoView is a widget that shows the most recent video frame, scaled to
// fit while keeping its aspect ratio, on a black background. Double clicking
// it calls the function given to NewVideoView, which the window uses to
// toggle full screen.
//
// SetFrame and Clear may be called from any goroutine. All other methods
// follow the usual Fyne rule of being called from the main thread.
type VideoView struct {
	widget.BaseWidget

	// img displays the frame. Its Image field is only touched on the Fyne
	// main thread.
	img *canvas.Image

	// onDoubleTap is called when the view is double clicked. May be nil.
	onDoubleTap func()

	// aspect is how the picture is fitted into the area. Main thread only.
	aspect AspectMode

	// latest is the newest frame handed to SetFrame and not yet shown.
	latest atomic.Pointer[image.RGBA]

	// pending is true while a redraw is queued for the main thread, so that
	// at most one is ever outstanding.
	pending atomic.Bool

	// clearing is set by Clear and consumed by the next queued redraw, which
	// then shows no picture.
	clearing atomic.Bool
}

// NewVideoView creates an empty (black) video view. onDoubleTap may be nil.
func NewVideoView(onDoubleTap func()) *VideoView {
	v := &VideoView{onDoubleTap: onDoubleTap, aspect: AspectModes[0]}
	v.img = canvas.NewImageFromImage(nil)
	v.img.FillMode = canvas.ImageFillContain

	// ImageScaleFastest leaves scaling to the graphics card. The default,
	// ImageScaleSmooth, resamples every frame on the CPU, which is far too
	// slow for video.
	v.img.ScaleMode = canvas.ImageScaleFastest
	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer implements fyne.Widget. The picture is stacked on a black
// rectangle so the bars around a picture with a different aspect ratio are
// black rather than the theme background.
func (v *VideoView) CreateRenderer() fyne.WidgetRenderer {
	return &videoRenderer{v: v, bg: canvas.NewRectangle(color.Black)}
}

// SetAspect changes how the picture is fitted into the area. It must be
// called on the Fyne main thread.
func (v *VideoView) SetAspect(m AspectMode) {
	v.aspect = m
	v.Refresh() // runs videoRenderer.Refresh, which lays the picture out again
}

// videoRenderer lays out the black background and the picture. Fyne's own
// layouts cannot be used, because forcing an aspect ratio needs the picture
// sized to a computed rectangle in the middle of the area.
type videoRenderer struct {
	// v is the widget being drawn.
	v *VideoView

	// bg is the black background that fills the whole area.
	bg *canvas.Rectangle
}

// Layout implements fyne.WidgetRenderer. It places the picture according to
// the widget's aspect mode.
func (r *videoRenderer) Layout(size fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)

	img, m := r.v.img, r.v.aspect
	if m.Ratio <= 0 {
		img.FillMode = m.Fill
		img.Move(fyne.NewPos(0, 0))
		img.Resize(size)
		return
	}

	// Largest rectangle of the wanted shape that fits, centred. Stretch fills
	// that rectangle exactly, which is what bends the source to the shape.
	w, h := size.Width, size.Width/float32(m.Ratio)
	if h > size.Height {
		w, h = size.Height*float32(m.Ratio), size.Height
	}
	img.FillMode = canvas.ImageFillStretch
	img.Move(fyne.NewPos((size.Width-w)/2, (size.Height-h)/2))
	img.Resize(fyne.NewSize(w, h))
}

// MinSize implements fyne.WidgetRenderer.
func (r *videoRenderer) MinSize() fyne.Size {
	return fyne.NewSize(minViewWidth, minViewHeight)
}

// Refresh implements fyne.WidgetRenderer. It is called by SetAspect, so it
// lays out again and redraws the picture with its new fill mode.
func (r *videoRenderer) Refresh() {
	r.Layout(r.v.Size())
	r.bg.Refresh()
	r.v.img.Refresh()
}

// Objects implements fyne.WidgetRenderer.
func (r *videoRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.v.img}
}

// Destroy implements fyne.WidgetRenderer. There is nothing to release.
func (r *videoRenderer) Destroy() {}

// DoubleTapped implements fyne.DoubleTappable.
func (v *VideoView) DoubleTapped(_ *fyne.PointEvent) {
	if v.onDoubleTap != nil {
		v.onDoubleTap()
	}
}

// SetFrame shows frame f. If the main thread has not yet drawn the previous
// frame, that frame is skipped instead and only the newest is drawn.
//
// f must stay unchanged until a few more frames have been delivered, which
// the video package guarantees by cycling through several buffers.
func (v *VideoView) SetFrame(f *image.RGBA) {
	v.clearing.Store(false)
	v.latest.Store(f)
	v.schedule()
}

// Clear removes the picture, leaving the black background.
func (v *VideoView) Clear() {
	v.clearing.Store(true)
	v.latest.Store(nil)
	v.schedule()
}

// schedule queues a redraw on the main thread unless one is already queued.
func (v *VideoView) schedule() {
	if !v.pending.CompareAndSwap(false, true) {
		return
	}
	fyne.Do(func() {
		// Reset before reading latest, so a frame stored after the read
		// queues another redraw instead of being lost.
		v.pending.Store(false)
		f := v.latest.Load()
		if f == nil {
			if !v.clearing.Load() {
				return
			}
			v.img.Image = nil
		} else {
			v.img.Image = f
		}
		v.img.Refresh()
	})
}
