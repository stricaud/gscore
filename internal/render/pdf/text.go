/*
 * text.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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

// plainly turns a character the reading faces cannot set into the nearest
// one they can.
//
// The standard faces are encoded in WinAnsi, and what it has not got it
// prints as a question mark. Most of what a score contains is inside it, but
// the punctuation a word processor puts into words is not: a lyric typed
// anywhere but a plain text editor carries a curly apostrophe, and "ev-'ry
// dream gone" came out of a song sheet as "ev-?ry dream gone".
//
// Only the marks that stand for an ordinary one. A character with no plain
// equivalent — a Cyrillic title, a Japanese one — is a different problem and
// wants a font, not a substitution.
func plainly(c rune) rune {
	switch c {
	case '\u2018', '\u2019', '\u02bc': // the curly quotes and the modifier
		return '\''
	case '\u201c', '\u201d':
		return '"'
	case '\u2013', '\u2014', '\u2212': // en dash, em dash, minus
		return '-'
	case '\u00a0', '\u2009', '\u200a': // the spaces that are not one
		return ' '
	}
	return c
}

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
			plain = append(plain, plainly(c))
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
