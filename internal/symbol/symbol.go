/*
 * symbol.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package symbol holds the drawable marks a score can use, from whichever of
// three sources: a glyph in the music font, a vector outline, or a grid of
// pixels drawn by hand.
//
// The third source is the point of the package. Notation has a long tail of
// symbols that no standard font carries — a composer's own articulation, an
// early-music neume, a percussion pictogram, something invented for one piece
// — and a program that can only draw what its font knows cannot set them at
// all. gscore could always do this, in its own way: all 287 of its symbols in
// legacy/pixmaps are XPM files, which are pixel grids in C source form that
// can be edited by hand in any text editor. That capability is kept.
package symbol

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"
)

// Kind says where a symbol's artwork comes from.
type Kind int

const (
	// FromFont is a glyph in the active SMuFL music font, named by its
	// codepoint. This is the normal case and the only one that benefits from
	// the font's own metrics.
	FromFont Kind = iota

	// FromVector is an outline, which scales to any size and prints at the
	// output device's full resolution.
	FromVector

	// FromPixels is a hand-drawn grid. It is the most direct thing to author
	// and the only one with a fixed resolution, so it is the right choice for
	// a symbol that exists nowhere else and the wrong one for a notehead.
	FromPixels
)

var kindNames = [...]string{"font", "vector", "pixels"}

func (k Kind) String() string {
	if k < FromFont || int(k) >= len(kindNames) {
		return "unknown"
	}
	return kindNames[k]
}

// Point is a position in staff spaces, relative to a symbol's origin.
type Point struct {
	X, Y float64
}

// Symbol is one drawable mark.
//
// Every measurement is in staff spaces, never pixels, so that the same symbol
// serves a screen at any zoom and a printed page at any size.
type Symbol struct {
	// Name identifies the symbol. For font glyphs this is the SMuFL canonical
	// name, such as "noteheadBlack"; for user symbols it is whatever the user
	// called it.
	Name string

	Kind Kind

	// Codepoint is the glyph to set, for FromFont. SMuFL assigns the standard
	// glyphs from U+E000 and reserves U+F400 to U+F8FF for a font's own
	// additions, which is where a custom glyph belongs if it is ever built
	// into a font.
	Codepoint rune

	// Path is an outline in SVG path syntax, for FromVector, expressed in
	// staff spaces with y increasing downward.
	Path string

	// Pixels is the grid, for FromPixels.
	Pixels *Bitmap

	// Anchors are the attachment points the engraver needs: where a stem meets
	// a notehead, where a flag joins a stem, and so on. SMuFL publishes these
	// for font glyphs in the font's metadata; a user symbol has to declare any
	// it needs, and most need none.
	Anchors map[string]Point

	// Advance is how much horizontal room the symbol occupies, in staff
	// spaces. Zero means to measure it from the artwork.
	Advance float64
}

// Bitmap is a symbol drawn on a pixel grid.
//
// A grid alone cannot be placed on a staff: it has no size and no idea which
// of its pixels should land on the note's position. XPM records neither, which
// is why the C engine hardcoded both — see legacy/src/realize_area.c, where
// each pixmap is drawn at an offset worked out by hand. Both are stored
// explicitly here instead.
type Bitmap struct {
	W, H int

	// Pix is row-major and holds W*H entries. A zero alpha is transparent,
	// which is what XPM's "None" becomes.
	Pix []color.NRGBA

	// PixelSize is how much of a staff space one pixel covers. It is what
	// gives the grid a musical size: at the default, a 9-by-8 notehead is
	// exactly one staff space tall, whatever the zoom.
	PixelSize float64

	// OriginX and OriginY are the registration point within the grid, in
	// pixels: the point placed at the symbol's position on the staff.
	OriginX, OriginY float64
}

// DefaultPixelSize is one eighth of a staff space per pixel.
//
// It comes from the C program, which created every staff with eight pixels
// between its lines — score.c:207 calls create_staff(score, 5, 8, ...) — so
// all 287 pixmaps in legacy/pixmaps were drawn to that scale. Importing them
// at this size reproduces their intended proportions exactly.
const DefaultPixelSize = 1.0 / 8.0

// NewBitmap returns a transparent grid at the default scale, with its origin
// at the left edge, vertically centred: the registration a notehead or any
// other staff-positioned symbol wants.
func NewBitmap(w, h int) *Bitmap {
	return &Bitmap{
		W:         w,
		H:         h,
		Pix:       make([]color.NRGBA, w*h),
		PixelSize: DefaultPixelSize,
		OriginY:   float64(h) / 2,
	}
}

// At returns the pixel at x, y. Out-of-range coordinates read as transparent,
// so callers can scan freely.
func (b *Bitmap) At(x, y int) color.NRGBA {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return color.NRGBA{}
	}
	return b.Pix[y*b.W+x]
}

// Set writes the pixel at x, y, ignoring coordinates outside the grid.
func (b *Bitmap) Set(x, y int, c color.NRGBA) {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return
	}
	b.Pix[y*b.W+x] = c
}

// WidthInSpaces is the grid's width in staff spaces.
func (b *Bitmap) WidthInSpaces() float64 { return float64(b.W) * b.PixelSize }

// HeightInSpaces is the grid's height in staff spaces.
func (b *Bitmap) HeightInSpaces() float64 { return float64(b.H) * b.PixelSize }

// Image returns the grid as a standard image, for encoding or rendering.
func (b *Bitmap) Image() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, b.W, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			img.SetNRGBA(x, y, b.At(x, y))
		}
	}
	return img
}

// Opaque reports how many pixels are not fully transparent, which is a cheap
// way to tell an empty grid from a drawn one.
func (b *Bitmap) Opaque() int {
	n := 0
	for _, c := range b.Pix {
		if c.A != 0 {
			n++
		}
	}
	return n
}

// Registry maps names to symbols.
//
// It is what lets a custom symbol be referred to from the model by name
// alone, so that the music stays free of artwork and a symbol can be
// redrawn, or swapped from a bitmap to an outline, without touching the score.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]*Symbol
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Symbol{}}
}

// Add registers a symbol, replacing any symbol of the same name. A later
// definition winning is deliberate: it is how a user overrides a font glyph
// with their own drawing.
func (r *Registry) Add(s *Symbol) error {
	if s == nil {
		return fmt.Errorf("symbol: cannot add a nil symbol")
	}
	if s.Name == "" {
		return fmt.Errorf("symbol: cannot add a symbol with no name")
	}
	if s.Kind == FromPixels && s.Pixels == nil {
		return fmt.Errorf("symbol %q: declared as pixels but carries no grid", s.Name)
	}
	if s.Kind == FromVector && s.Path == "" {
		return fmt.Errorf("symbol %q: declared as vector but carries no path", s.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[s.Name] = s
	return nil
}

// Lookup finds a symbol by name.
func (r *Registry) Lookup(name string) (*Symbol, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

// Names lists every registered symbol, in order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byName))
	for n := range r.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Len reports how many symbols are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byName)
}
