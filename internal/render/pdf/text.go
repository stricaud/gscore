/*
 * text.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package pdf

import (
	"github.com/stricaud/gscore/internal/pdf"
	"github.com/stricaud/gscore/internal/smufl"
)

// Music symbols inside ordinary words.
//
// The reading faces a PDF carries without embedding them are the fourteen
// standard ones, and they are encoded in WinAnsi: a byte per character, and
// no flat sign anywhere in it. Anything outside it was printed as a question
// mark, so a band score's staff names came out as "Clarinet in B? 1" — and
// the name of a transposing instrument is the one place a flat sign is
// certain to appear.
//
// They are set from Bravura instead, which is embedded already because the
// music needs it. A line is cut into runs — words in the text face, symbols
// in the music face — and each run is measured and drawn in its own.
//
// Only the three signs that turn up in words are mapped. This is not a
// general fallback for every character the standard faces lack: a Cyrillic
// title is a different problem and wants a different font, not Bravura.

// musicSign maps a character to the SMuFL glyph that stands for it.
func musicSign(r rune) (string, bool) {
	switch r {
	case '♭':
		return "accidentalFlat", true
	case '♯':
		return "accidentalSharp", true
	case '♮':
		return "accidentalNatural", true
	}
	return "", false
}

// How a music sign is set beside letters.
//
// Bravura's accidentals are drawn for a staff, where the glyph's origin is
// the line the note sits on and the sign reaches further above it than below.
// Set from a text baseline at the text's own size the result is small and
// hangs under the line, so it is drawn a little larger and lifted.
const (
	signScale = 1.15
	signRise  = 0.22 // of the text size
)

// run is one stretch of a line set in one font.
type run struct {
	f    *pdf.Font
	s    string
	size pdf.Point
	dy   pdf.Point
}

// split cuts a line into the runs it is set in.
//
// A line with no music in it comes back as one run, which is the common case
// and costs a scan of the string.
func (r *renderer) split(line string, f *pdf.Font, size pdf.Point) []run {
	var out []run
	var plain []rune
	flush := func() {
		if len(plain) > 0 {
			out = append(out, run{f: f, s: string(plain), size: size})
			plain = plain[:0]
		}
	}
	for _, c := range line {
		name, ok := musicSign(c)
		if !ok {
			plain = append(plain, c)
			continue
		}
		g, have := smufl.Glyph(name)
		if !have {
			plain = append(plain, c)
			continue
		}
		flush()
		out = append(out, run{
			f: r.music, s: string(g), size: size * signScale,
			dy: size * signRise,
		})
	}
	flush()
	return out
}

// width is how wide a line is once the symbols in it are counted at their
// own size and in their own font.
func (r *renderer) width(rs []run) pdf.Point {
	var w pdf.Point
	for _, x := range rs {
		w += r.page.TextWidth(x.f, x.size, x.s)
	}
	return w
}

// drawRuns prints a line from a left baseline, each run after the last.
func (r *renderer) drawRuns(rs []run, x, y pdf.Point) {
	for _, s := range rs {
		r.page.Text(s.f, s.size, x, y+s.dy, s.s)
		x += r.page.TextWidth(s.f, s.size, s.s)
	}
}
