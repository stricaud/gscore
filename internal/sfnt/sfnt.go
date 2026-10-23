/*
 * sfnt.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package sfnt reads just enough of an OpenType font to put it in a PDF.
//
// A PDF that embeds a font has to describe it: how many glyphs it has, how
// wide each one is, which glyph a character maps to, and what the thing is
// called. All of that is in the font file already, in tables with a format
// that has not changed since 1991.
//
// This is not a font library. It reads six tables and ignores the rest —
// no shaping, no kerning, no outlines, no variations. Bravura is a static
// OpenType font with CFF outlines, which a PDF can embed whole, so there is
// nothing to rasterise or convert and nothing here needs to understand what
// a glyph looks like.
package sfnt

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Font is a parsed font, holding the original bytes alongside what was read
// out of them. A PDF embeds the file itself, so the bytes are the point.
type Font struct {
	Data []byte

	// UnitsPerEm is the font's own coordinate grid. Everything a PDF says
	// about a font is in thousandths of the type size, so widths have to be
	// scaled by 1000/UnitsPerEm.
	UnitsPerEm int
	NumGlyphs  int
	// PostScriptName is what the font calls itself, which is what a PDF has
	// to call it too.
	PostScriptName string
	// Bounds is the font's bounding box in font units.
	XMin, YMin, XMax, YMax int
	// Ascent, Descent and CapHeight are needed for the font descriptor; a
	// viewer uses them to lay out text when the font is missing.
	Ascent, Descent, CapHeight int
	// ItalicAngle, in degrees, negative for a face that leans right.
	ItalicAngle float64
	// IsCFF says the outlines are PostScript rather than TrueType, which
	// decides which kind of PDF font object describes it.
	IsCFF bool

	advances []uint16 // per glyph, in font units
	cmap     map[rune]uint16
}

// Parse reads a font file.
func Parse(data []byte) (*Font, error) {
	if len(data) < 12 {
		return nil, errors.New("sfnt: not a font: too short")
	}
	tag := be32(data, 0)
	switch tag {
	case 0x4F54544F: // "OTTO" — CFF outlines
	case 0x00010000, 0x74727565: // TrueType outlines
	case 0x74746366:
		return nil, errors.New("sfnt: font collections are not supported")
	default:
		return nil, fmt.Errorf("sfnt: not a font: tag %08x", tag)
	}

	f := &Font{Data: data, IsCFF: tag == 0x4F54544F}

	n := int(be16(data, 4))
	tables := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		rec := 12 + i*16
		if rec+16 > len(data) {
			return nil, errors.New("sfnt: the table directory runs past the end")
		}
		name := string(data[rec : rec+4])
		off := int(be32(data, rec+8))
		length := int(be32(data, rec+12))
		if off < 0 || length < 0 || off+length > len(data) {
			// A table that does not fit is skipped rather than fatal: DSIG
			// in particular is often recorded with a length past the end.
			continue
		}
		tables[name] = data[off : off+length]
	}
	if _, ok := tables["CFF "]; ok {
		f.IsCFF = true
	}

	if err := f.readHead(tables["head"]); err != nil {
		return nil, err
	}
	if err := f.readMaxp(tables["maxp"]); err != nil {
		return nil, err
	}
	if err := f.readHmtx(tables["hhea"], tables["hmtx"]); err != nil {
		return nil, err
	}
	f.readOS2(tables["OS/2"])
	f.readPost(tables["post"])
	f.PostScriptName = readName(tables["name"])
	if f.PostScriptName == "" {
		f.PostScriptName = "EmbeddedFont"
	}
	if err := f.readCmap(tables["cmap"]); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Font) readHead(t []byte) error {
	if len(t) < 54 {
		return errors.New("sfnt: no usable head table")
	}
	f.UnitsPerEm = int(be16(t, 18))
	if f.UnitsPerEm == 0 {
		f.UnitsPerEm = 1000
	}
	f.XMin = int(int16(be16(t, 36)))
	f.YMin = int(int16(be16(t, 38)))
	f.XMax = int(int16(be16(t, 40)))
	f.YMax = int(int16(be16(t, 42)))
	return nil
}

func (f *Font) readMaxp(t []byte) error {
	if len(t) < 6 {
		return errors.New("sfnt: no usable maxp table")
	}
	f.NumGlyphs = int(be16(t, 4))
	return nil
}

func (f *Font) readHmtx(hhea, hmtx []byte) error {
	if len(hhea) < 36 {
		return errors.New("sfnt: no usable hhea table")
	}
	f.Ascent = int(int16(be16(hhea, 4)))
	f.Descent = int(int16(be16(hhea, 6)))
	count := int(be16(hhea, 34))
	if count == 0 || len(hmtx) < count*4 {
		return errors.New("sfnt: no usable hmtx table")
	}
	f.advances = make([]uint16, f.NumGlyphs)
	last := uint16(0)
	for i := 0; i < f.NumGlyphs; i++ {
		if i < count {
			last = be16(hmtx, i*4)
		}
		// Past the last full metric every glyph keeps the final advance,
		// which is how monospaced tails are stored.
		f.advances[i] = last
	}
	return nil
}

func (f *Font) readOS2(t []byte) {
	if len(t) >= 72 {
		if a := int(int16(be16(t, 68))); a != 0 {
			f.Ascent = a
		}
		if d := int(int16(be16(t, 70))); d != 0 {
			f.Descent = d
		}
	}
	if len(t) >= 90 {
		f.CapHeight = int(int16(be16(t, 88)))
	}
	if f.CapHeight == 0 {
		f.CapHeight = f.Ascent
	}
}

func (f *Font) readPost(t []byte) {
	if len(t) < 12 {
		return
	}
	// A 16.16 fixed-point number.
	f.ItalicAngle = float64(int32(be32(t, 4))) / 65536
}

// readName pulls the PostScript name, which is name ID 6.
func readName(t []byte) string {
	if len(t) < 6 {
		return ""
	}
	count := int(be16(t, 2))
	strOff := int(be16(t, 4))
	best := ""
	for i := 0; i < count; i++ {
		rec := 6 + i*12
		if rec+12 > len(t) {
			break
		}
		platform := be16(t, rec)
		encoding := be16(t, rec+2)
		nameID := be16(t, rec+6)
		length := int(be16(t, rec+8))
		off := strOff + int(be16(t, rec+10))
		if nameID != 6 || off+length > len(t) {
			continue
		}
		raw := t[off : off+length]
		// Platform 1 is Macintosh and its strings are bytes; everything else
		// here is UTF-16BE, which for a PostScript name means every other
		// byte is zero.
		if platform == 1 {
			return string(raw)
		}
		if platform == 3 && (encoding == 0 || encoding == 1 || encoding == 10) {
			out := make([]byte, 0, length/2)
			for j := 0; j+1 < length; j += 2 {
				if raw[j] == 0 {
					out = append(out, raw[j+1])
				}
			}
			best = string(out)
		}
	}
	return best
}

func (f *Font) readCmap(t []byte) error {
	if len(t) < 4 {
		return errors.New("sfnt: no usable cmap table")
	}
	f.cmap = map[rune]uint16{}

	n := int(be16(t, 2))
	// The best subtable wins: a format 12 table covers everything above the
	// basic plane, which a music font very much needs — SMuFL lives at
	// U+E000 and above but its optional ranges reach past U+FFFF.
	bestOff, bestScore := -1, -1
	for i := 0; i < n; i++ {
		rec := 4 + i*8
		if rec+8 > len(t) {
			break
		}
		platform := be16(t, rec)
		encoding := be16(t, rec+2)
		off := int(be32(t, rec+4))
		if off+4 > len(t) {
			continue
		}
		format := int(be16(t, off))
		score := -1
		switch {
		case platform == 3 && encoding == 10 && format == 12:
			score = 4
		case platform == 0 && encoding == 4 && format == 12:
			score = 3
		case platform == 3 && encoding == 1 && format == 4:
			score = 2
		case platform == 0 && format == 4:
			score = 1
		}
		if score > bestScore {
			bestScore, bestOff = score, off
		}
	}
	if bestOff < 0 {
		return errors.New("sfnt: the font has no Unicode cmap")
	}

	switch be16(t, bestOff) {
	case 4:
		return f.readCmap4(t[bestOff:])
	case 12:
		return f.readCmap12(t[bestOff:])
	}
	return errors.New("sfnt: unsupported cmap format")
}

func (f *Font) readCmap4(t []byte) error {
	if len(t) < 14 {
		return errors.New("sfnt: truncated cmap format 4")
	}
	segX2 := int(be16(t, 6))
	seg := segX2 / 2
	endAt := 14
	startAt := endAt + segX2 + 2
	deltaAt := startAt + segX2
	rangeAt := deltaAt + segX2
	if rangeAt+segX2 > len(t) {
		return errors.New("sfnt: truncated cmap format 4")
	}

	for i := 0; i < seg; i++ {
		end := int(be16(t, endAt+i*2))
		start := int(be16(t, startAt+i*2))
		delta := int(int16(be16(t, deltaAt+i*2)))
		rangeOff := int(be16(t, rangeAt+i*2))
		if start > end {
			continue
		}
		for c := start; c <= end && c <= 0xFFFF; c++ {
			var g int
			if rangeOff == 0 {
				g = (c + delta) & 0xFFFF
			} else {
				at := rangeAt + i*2 + rangeOff + (c-start)*2
				if at+2 > len(t) {
					continue
				}
				g = int(be16(t, at))
				if g == 0 {
					continue
				}
				g = (g + delta) & 0xFFFF
			}
			if g != 0 {
				f.cmap[rune(c)] = uint16(g)
			}
		}
	}
	return nil
}

func (f *Font) readCmap12(t []byte) error {
	if len(t) < 16 {
		return errors.New("sfnt: truncated cmap format 12")
	}
	groups := int(be32(t, 12))
	for i := 0; i < groups; i++ {
		at := 16 + i*12
		if at+12 > len(t) {
			break
		}
		start := int(be32(t, at))
		end := int(be32(t, at+4))
		gid := int(be32(t, at+8))
		if end < start || end-start > 0x10FFFF {
			continue
		}
		for c := start; c <= end; c++ {
			g := gid + (c - start)
			if g > 0 && g < 65536 {
				f.cmap[rune(c)] = uint16(g)
			}
		}
	}
	return nil
}

// GlyphIndex is the glyph a character is drawn with, or 0 — the notdef glyph —
// when the font has nothing for it.
func (f *Font) GlyphIndex(r rune) uint16 { return f.cmap[r] }

// Advance is how wide a glyph is, in thousandths of the type size, which is
// the unit PDF measures text in.
func (f *Font) Advance(g uint16) int {
	if int(g) >= len(f.advances) {
		return 0
	}
	return int(f.advances[g]) * 1000 / f.UnitsPerEm
}

// Scale converts a measurement in font units to thousandths of the type size.
func (f *Font) Scale(v int) int { return v * 1000 / f.UnitsPerEm }

func be16(b []byte, i int) uint16 {
	if i+2 > len(b) {
		return 0
	}
	return binary.BigEndian.Uint16(b[i:])
}

func be32(b []byte, i int) uint32 {
	if i+4 > len(b) {
		return 0
	}
	return binary.BigEndian.Uint32(b[i:])
}
