// Package main is genicon, the program that draws the My Capture application
// icon and writes it in every size and format the project needs.
//
// WHY A PROGRAM?
// The icon is a simple shape (a screen with a camera lens on a gradient tile),
// so drawing it in code keeps the project free of binary design files that
// nobody can edit, produces crisp results at every size because each size is
// rendered directly instead of scaled down, and lets the colours and shapes
// be changed by editing a few constants. The Makefile target "icons" runs it.
//
// OUTPUT (in the directory given by -out, default assets/icon)
//   - icon.png: 256 pixels, the tile fills the image. Used for the window and
//     task bar icon on Windows.
//   - icon-mac.png: 512 pixels with the transparent margin that macOS icons
//     have. Used for the Dock icon when the program runs outside an app bundle.
//   - icon.ico: the Windows icon, with 16 to 256 pixel images, for the
//     executable (see the "icons" Makefile target, which turns it into a
//     Windows resource).
//   - icon.iconset/: the ten PNG files that macOS iconutil turns into
//     icon.icns for the application bundle.
//
// Environment variables: none.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// -----------------------------------------------------------------------
// Design
// -----------------------------------------------------------------------

// All shapes below are measured in "tile units": the icon tile is a square
// from 0 to 1 in both directions, whatever size it is rendered at.

const (
	// macPad is the transparent margin around the tile, as a fraction of the
	// image, in macOS style (the tile is 824 of 1024 pixels).
	macPad = 0.098

	// winPad is the small margin used for Windows icons, which fill their
	// canvas almost completely.
	winPad = 0.03

	// tileRadius is the corner radius of the tile.
	tileRadius = 0.225
)

var (
	// gradientTop and gradientBottom are the tile colours at its top left
	// and bottom right corners.
	gradientTop    = color.NRGBA{0x43, 0x38, 0xCA, 0xFF}
	gradientBottom = color.NRGBA{0x0E, 0x74, 0x90, 0xFF}

	// bezel is the screen frame, screenFill the dark glass inside it.
	bezel      = color.NRGBA{0xF8, 0xFA, 0xFC, 0xFF}
	screenFill = color.NRGBA{0x0B, 0x12, 0x20, 0xFF}

	// The lens, from the outside in.
	lensRing  = color.NRGBA{0xE2, 0xE8, 0xF0, 0xFF}
	lensDark  = color.NRGBA{0x0F, 0x17, 0x2A, 0xFF}
	lensGlass = color.NRGBA{0x22, 0xD3, 0xEE, 0xFF}
	lensCore  = color.NRGBA{0x08, 0x91, 0xB2, 0xFF}
	lensShine = color.NRGBA{0xFF, 0xFF, 0xFF, 0xE6}
)

// shape is one filled shape: dist returns the signed distance from a point to
// its edge (negative inside), and fill is its colour at a point.
type shape struct {
	dist func(x, y float64) float64
	fill func(x, y float64) color.NRGBA
}

// solid returns a fill function that is the same colour everywhere.
func solid(c color.NRGBA) func(x, y float64) color.NRGBA {
	return func(float64, float64) color.NRGBA { return c }
}

// roundRect returns the distance function of a rounded rectangle with centre
// (cx, cy), half sizes (hw, hh) and corner radius r.
func roundRect(cx, cy, hw, hh, r float64) func(x, y float64) float64 {
	return func(x, y float64) float64 {
		qx := math.Abs(x-cx) - (hw - r)
		qy := math.Abs(y-cy) - (hh - r)
		outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
		inside := math.Min(math.Max(qx, qy), 0)
		return outside + inside - r
	}
}

// circle returns the distance function of a circle.
func circle(cx, cy, r float64) func(x, y float64) float64 {
	return func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - r }
}

// shapes lists the layers of the icon from back to front.
func shapes() []shape {
	const lx, ly = 0.5, 0.45 // lens centre

	gradient := func(x, y float64) color.NRGBA {
		t := (x + y) / 2
		mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
		return color.NRGBA{mix(gradientTop.R, gradientBottom.R),
			mix(gradientTop.G, gradientBottom.G), mix(gradientTop.B, gradientBottom.B), 0xFF}
	}

	return []shape{
		{roundRect(0.5, 0.5, 0.5, 0.5, tileRadius), gradient},
		{roundRect(0.5, 0.45, 0.34, 0.215, 0.06), solid(bezel)},
		{roundRect(0.5, 0.45, 0.31, 0.185, 0.035), solid(screenFill)},
		{circle(lx, ly, 0.135), solid(lensRing)},
		{circle(lx, ly, 0.112), solid(lensDark)},
		{circle(lx, ly, 0.085), solid(lensGlass)},
		{circle(lx, ly, 0.045), solid(lensCore)},
		{circle(lx-0.03, ly-0.03, 0.02), solid(lensShine)},
		{roundRect(0.5, 0.745, 0.045, 0.04, 0.01), solid(bezel)},
		{roundRect(0.5, 0.8, 0.16, 0.025, 0.025), solid(bezel)},
	}
}

// -----------------------------------------------------------------------
// Rendering
// -----------------------------------------------------------------------

// render draws the icon at size by size pixels, with the tile inset from the
// edges by pad (a fraction of size).
//
// Edges are anti-aliased by turning each shape's distance to its edge into a
// coverage between 0 and 1 over one pixel, which is cheap and gives smooth
// curves without drawing at a higher resolution.
func render(size int, pad float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	tile := float64(size) * (1 - 2*pad) // pixels across the tile
	layers := shapes()

	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			// Position in tile units, at the pixel centre.
			x := (float64(px) + 0.5 - float64(size)*pad) / tile
			y := (float64(py) + 0.5 - float64(size)*pad) / tile

			var r, g, b, a float64 // premultiplied accumulator
			for _, l := range layers {
				cov := clamp01(0.5 - l.dist(x, y)*tile)
				if cov == 0 {
					continue
				}
				c := l.fill(x, y)
				sa := cov * float64(c.A) / 255
				r = float64(c.R)/255*sa + r*(1-sa)
				g = float64(c.G)/255*sa + g*(1-sa)
				b = float64(c.B)/255*sa + b*(1-sa)
				a = sa + a*(1-sa)
			}
			if a > 0 {
				img.SetNRGBA(px, py, color.NRGBA{
					uint8(math.Round(r / a * 255)), uint8(math.Round(g / a * 255)),
					uint8(math.Round(b / a * 255)), uint8(math.Round(a * 255)),
				})
			}
		}
	}
	return img
}

// clamp01 limits v to the range 0 to 1.
func clamp01(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

// -----------------------------------------------------------------------
// Output
// -----------------------------------------------------------------------

// encodePNG returns img as PNG data.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writePNG renders the icon and writes it to path.
func writePNG(path string, size int, pad float64) error {
	data, err := encodePNG(render(size, pad))
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0o644)
}

// writeICO writes a Windows icon file holding one PNG image per size.
// Windows has accepted PNG data inside icon files since Vista, for every size.
func writeICO(path string, sizes []int, pad float64) error {
	images := make([][]byte, len(sizes))
	for i, s := range sizes {
		data, err := encodePNG(render(s, pad))
		if err != nil {
			return fmt.Errorf("encode %dx%d: %w", s, s, err)
		}
		images[i] = data
	}

	var buf bytes.Buffer
	put := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) } // a bytes.Buffer never fails
	put(uint16(0))                                                        // reserved
	put(uint16(1))                                                        // type: icon
	put(uint16(len(sizes)))                                               // image count

	offset := 6 + 16*len(sizes) // the images follow the header and the directory
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0 // 0 means 256 in the directory
		}
		put(dim)                    // width
		put(dim)                    // height
		put(uint8(0))               // colours in palette: none
		put(uint8(0))               // reserved
		put(uint16(1))              // colour planes
		put(uint16(32))             // bits per pixel
		put(uint32(len(images[i]))) // size of the image data
		put(uint32(offset))         // where the image data starts
		offset += len(images[i])
	}
	for _, data := range images {
		buf.Write(data)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// iconsetFiles lists the files macOS iconutil expects in an .iconset folder,
// with the pixel size of each.
var iconsetFiles = []struct {
	name string
	size int
}{
	{"icon_16x16.png", 16}, {"icon_16x16@2x.png", 32},
	{"icon_32x32.png", 32}, {"icon_32x32@2x.png", 64},
	{"icon_128x128.png", 128}, {"icon_128x128@2x.png", 256},
	{"icon_256x256.png", 256}, {"icon_256x256@2x.png", 512},
	{"icon_512x512.png", 512}, {"icon_512x512@2x.png", 1024},
}

// main writes all the icon files.
func main() {
	out := flag.String("out", filepath.Join("assets", "icon"), "directory to write the icon files to")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run writes every icon file into dir.
func run(dir string) error {
	set := filepath.Join(dir, "icon.iconset")
	if err := os.MkdirAll(set, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", set, err)
	}

	if err := writePNG(filepath.Join(dir, "icon.png"), 256, winPad); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(dir, "icon-mac.png"), 512, macPad); err != nil {
		return err
	}
	if err := writeICO(filepath.Join(dir, "icon.ico"), []int{16, 24, 32, 48, 64, 128, 256}, winPad); err != nil {
		return err
	}
	for _, f := range iconsetFiles {
		if err := writePNG(filepath.Join(set, f.name), f.size, macPad); err != nil {
			return err
		}
	}
	return nil
}
