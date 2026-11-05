/*
 * omr.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package omr reads music off a picture of a page.
//
// It exists so that gscore can open a scan. internal/pdf writes PDFs and does
// not read them; this package starts one step further back, from whatever
// raster a PDF page or a photograph yields, and works towards a model.Score.
//
// # What it is for, and what it is not
//
// This is not a general optical music recognition system, and it is not meant
// to become one. Audiveris is that, it is good at it, and it needs a JVM.
// What is here is deliberately narrower: the engraving conventions of the
// nineteenth and twentieth centuries, read by geometry rather than by a
// trained classifier. Five parallel lines, a round head, a straight stem, a
// vertical bar. Those four shapes carry most of the information on most
// pages, they can be found by measuring rather than by guessing, and a method
// that measures can say honestly how sure it is.
//
// The price is that anything ornamental — slurs, dynamics, text, ornaments —
// is ignored rather than misread. That is the right trade for an editor: a
// score that opens with the right notes and no articulation is a usable
// starting point, while one with invented articulation is a trap.
//
// # Resolution is the thing that matters
//
// Everything here is expressed in units of the staff interline, never in
// pixels, so the same code works at any scale. But scale still decides
// whether it works at all. Below roughly ten pixels of interline the ink that
// separates a notehead from the line it sits on is simply not in the image,
// and no amount of care recovers it; around twenty, hollow and filled heads
// separate cleanly. A 300 dpi scan of a normal page gives about thirty.
//
// The package therefore reports the interline it measured, and callers are
// expected to look at it before trusting anything else.
//
// # Honesty
//
// Every note carries how it was found and how far it sat from the degree it
// was rounded to. A reader that cannot say which of its answers are shaky is
// worse than useless in an editor, because the person correcting it has to
// re-check all of them.
package omr

import (
	"image"
	"math"
)

// Binary is a page reduced to ink and paper.
//
// The bit set means ink. Pages are stored row-major in a flat slice because
// every pass over them is either a row scan or a column scan, and the bounds
// checks that a [][]bool would add show up in both.
type Binary struct {
	W, H int
	bits []bool
}

// NewBinary returns an all-paper page of the given size.
func NewBinary(w, h int) *Binary {
	return &Binary{W: w, H: h, bits: make([]bool, w*h)}
}

// At reports whether the pixel is ink. Points outside the page are paper, so
// that neighbourhood loops need no edge cases.
func (b *Binary) At(x, y int) bool {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return false
	}
	return b.bits[y*b.W+x]
}

// Set marks a pixel as ink or paper.
func (b *Binary) Set(x, y int, ink bool) {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return
	}
	b.bits[y*b.W+x] = ink
}

// Grey is a greyscale page, 0 black and 255 white.
type Grey struct {
	W, H int
	Pix  []uint8
}

// FromImage converts any image to greyscale.
//
// Scans arrive as RGB even when they carry no colour — the Gallica scans this
// was built against are three identical channels — so the luminance formula
// costs nothing and handles the occasional genuinely coloured scan.
func FromImage(src image.Image) *Grey {
	r := src.Bounds()
	g := &Grey{W: r.Dx(), H: r.Dy(), Pix: make([]uint8, r.Dx()*r.Dy())}
	i := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := src.At(x, y).RGBA()
			// the RGBA() values are 16-bit; 299/587/114 are the usual weights
			lum := (299*uint32(cr>>8) + 587*uint32(cg>>8) + 114*uint32(cb>>8)) / 1000
			g.Pix[i] = uint8(lum)
			i++
		}
	}
	return g
}

// Binarise separates ink from paper by comparing each pixel with the local
// average rather than with a fixed threshold.
//
// A global threshold cannot work on these pages. The paper is tinted, the
// tint drifts across the sheet, and the gutter shadow at the spine is darker
// than some of the ink at the outer margin. Comparing against a window a few
// interlines wide follows the drift and leaves the ink standing.
//
// The window must be wider than anything we want to keep, or that thing
// becomes its own background and vanishes: at thirteen interlines it is much
// wider than a notehead and still narrower than the lighting variation.
func (g *Grey) Binarise(window int, margin uint8) *Binary {
	if window < 3 {
		window = 3
	}
	if window%2 == 0 {
		window++
	}
	mean := g.boxMean(window)
	b := NewBinary(g.W, g.H)
	for i, p := range g.Pix {
		b.bits[i] = float64(p)+float64(margin) < mean[i]
	}
	return b
}

// boxMean is the mean over a window×window square, by summed-area table so
// that the cost does not grow with the window.
func (g *Grey) boxMean(window int) []float64 {
	w, h := g.W, g.H
	// the table is (w+1)×(h+1) so that every rectangle is four lookups with
	// no special case at the top or left edge
	sat := make([]float64, (w+1)*(h+1))
	for y := 0; y < h; y++ {
		var row float64
		for x := 0; x < w; x++ {
			row += float64(g.Pix[y*w+x])
			sat[(y+1)*(w+1)+x+1] = sat[y*(w+1)+x+1] + row
		}
	}
	r := window / 2
	out := make([]float64, w*h)
	for y := 0; y < h; y++ {
		y0, y1 := clamp(y-r, 0, h), clamp(y+r+1, 0, h)
		for x := 0; x < w; x++ {
			x0, x1 := clamp(x-r, 0, w), clamp(x+r+1, 0, w)
			sum := sat[y1*(w+1)+x1] - sat[y0*(w+1)+x1] -
				sat[y1*(w+1)+x0] + sat[y0*(w+1)+x0]
			out[y*w+x] = sum / float64((y1-y0)*(x1-x0))
		}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Line is a staff line, as y = A·x + B.
//
// A straight line is enough and a curve would be worse. The pages are tilted
// by the binding, by a degree or two, which is linear; they are not warped,
// because the plates were printed flat. Fitting a curve to a straight line
// with noise on it buys nothing and can bend towards a notehead.
type Line struct{ A, B float64 }

// Y gives the line's height at x.
func (l Line) Y(x float64) float64 { return l.A*x + l.B }

func fitLine(xs, ys []float64) Line {
	n := float64(len(xs))
	if n == 0 {
		return Line{}
	}
	var sx, sy, sxx, sxy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
	}
	den := n*sxx - sx*sx
	if math.Abs(den) < 1e-9 {
		return Line{A: 0, B: sy / n}
	}
	a := (n*sxy - sx*sy) / den
	return Line{A: a, B: (sy - a*sx) / n}
}
