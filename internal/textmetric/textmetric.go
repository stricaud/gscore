/*
 * textmetric.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package textmetric is how wide a line of text is.
//
// The engraver has to know before anything is drawn: a rehearsal letter is
// boxed, a staff name reserves the margin it needs, a tuplet number is
// centred over its bracket. All of those are decided while the page is being
// laid out, and none of them can be asked of a renderer, because the layout
// comes first and is drawn twice — once as SVG and once as PDF.
//
// These are Adobe's own numbers for the fourteen standard PostScript fonts,
// in thousandths of the type size, for the printable ASCII range. They have
// shipped with PostScript since 1984 and they are what a PDF reader uses for
// a font the file does not carry.
//
// They were in internal/pdf, which is where they are needed to write a file
// and not where they are needed to lay a page out — so the engraver guessed
// instead, at a flat 0.52 of the type size for every character whatever it
// was. A bold C is 0.722 and a bold W is 1.0, so a boxed rehearsal letter
// came out with its letter over the right-hand edge of its own box, and an M
// or a W would have crossed it by a whole staff space.
//
// Both renderers set their text in Times — the PDF in the standard face it
// carries without embedding, the SVG in Times New Roman, which is drawn to
// these same widths — so what is measured here is what is drawn.
//
// It was not always so: the SVG asked for Georgia, which is about a fifth
// wider in the lower case. Where the engraver merely reserves room, a width
// read short is a box a little tight. Where it puts the next thing after the
// words — the note of a metronome mark after "Molto moderato" — a width read
// short is one thing drawn over another, and that is what happened.
package textmetric

type face struct {
	ascii [95]int16 // space (32) through tilde (126)
	avg   int16
}

func (f *face) width(r rune) int {
	if f == nil {
		return 500
	}
	if r >= 32 && r <= 126 {
		return int(f.ascii[r-32])
	}
	return int(f.avg)
}

var timesRoman = face{avg: 500, ascii: [95]int16{
	250, 333, 408, 500, 500, 833, 778, 180, 333, 333, 500, 564, 250, 333, 250, 278,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 278, 278, 564, 564, 564, 444,
	921, 722, 667, 667, 722, 611, 556, 722, 722, 333, 389, 722, 611, 889, 722, 722,
	556, 722, 667, 556, 611, 722, 722, 944, 722, 722, 611, 333, 278, 333, 469, 500,
	333, 444, 500, 444, 500, 444, 333, 500, 500, 278, 278, 500, 278, 778, 500, 500,
	500, 500, 333, 389, 278, 500, 500, 722, 500, 500, 444, 480, 200, 480, 541,
}}

var timesBold = face{avg: 500, ascii: [95]int16{
	250, 333, 555, 500, 500, 1000, 833, 278, 333, 333, 500, 570, 250, 333, 250, 278,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 333, 333, 570, 570, 570, 500,
	930, 722, 667, 722, 722, 667, 611, 778, 778, 389, 500, 778, 667, 944, 722, 778,
	611, 778, 722, 556, 667, 722, 722, 1000, 722, 722, 667, 333, 278, 333, 581, 500,
	333, 500, 556, 444, 556, 444, 333, 500, 556, 278, 333, 556, 278, 833, 556, 500,
	556, 556, 444, 389, 333, 556, 500, 722, 500, 500, 444, 394, 220, 394, 520,
}}

var timesItalic = face{avg: 500, ascii: [95]int16{
	250, 333, 420, 500, 500, 833, 778, 214, 333, 333, 500, 675, 250, 333, 250, 278,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 333, 333, 675, 675, 675, 500,
	920, 611, 611, 667, 722, 611, 611, 722, 722, 333, 444, 667, 556, 833, 667, 722,
	611, 722, 611, 500, 556, 722, 611, 833, 611, 556, 556, 389, 278, 389, 422, 500,
	333, 500, 500, 444, 500, 444, 278, 500, 500, 278, 278, 444, 278, 722, 500, 500,
	500, 500, 389, 389, 278, 500, 444, 667, 444, 444, 389, 400, 275, 400, 541,
}}

var timesBoldItalic = face{avg: 500, ascii: [95]int16{
	250, 389, 555, 500, 500, 833, 778, 278, 333, 333, 500, 570, 250, 333, 250, 278,
	500, 500, 500, 500, 500, 500, 500, 500, 500, 500, 333, 333, 570, 570, 570, 500,
	832, 667, 667, 667, 722, 667, 667, 722, 778, 389, 500, 667, 611, 889, 722, 722,
	611, 722, 667, 556, 611, 722, 667, 889, 667, 611, 611, 333, 278, 333, 570, 500,
	333, 500, 500, 444, 500, 444, 333, 500, 556, 278, 278, 500, 278, 778, 556, 500,
	500, 500, 389, 389, 278, 556, 444, 667, 500, 444, 389, 348, 220, 348, 570,
}}

var helvetica = face{avg: 556, ascii: [95]int16{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}}

var helveticaBold = face{avg: 556, ascii: [95]int16{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
}}

// Courier is monospaced, so every character is the same width.
var courier = func() face {
	f := face{avg: 600}
	for i := range f.ascii {
		f.ascii[i] = 600
	}
	return f
}()

// Style picks which of the four Times faces a line is set in. They are
// different widths — a bold C is 0.722 of the type size and a roman one
// 0.667 — so measuring the wrong one is as good as not measuring.
type Style struct {
	Bold, Italic bool
}

func faceFor(s Style) *face {
	switch {
	case s.Bold && s.Italic:
		return &timesBoldItalic
	case s.Bold:
		return &timesBold
	case s.Italic:
		return &timesItalic
	}
	return &timesRoman
}

// Width is how wide a line of text is, in the same units as size.
func Width(s string, size float64, style Style) float64 {
	f := faceFor(style)
	total := 0
	for _, r := range s {
		total += f.width(r)
	}
	return float64(total) / 1000 * size
}

// Sans is the same for Helvetica, which nothing lays out yet and which is
// here so that the tables are not half exported.
func Sans(s string, size float64, bold bool) float64 {
	f := &helvetica
	if bold {
		f = &helveticaBold
	}
	total := 0
	for _, r := range s {
		total += f.width(r)
	}
	return float64(total) / 1000 * size
}

// StdWidth is how wide a character is in one of the fourteen standard fonts,
// in thousandths of the type size, named as PDF names them.
//
// It is here for the PDF writer, which has to put the widths in the file for
// a reader that may not have the font. The engraver uses Width instead.
func StdWidth(name string, r rune) int {
	var f *face
	switch name {
	case "Times-Bold":
		f = &timesBold
	case "Times-Italic":
		f = &timesItalic
	case "Times-BoldItalic":
		f = &timesBoldItalic
	case "Helvetica", "Helvetica-Oblique":
		f = &helvetica
	case "Helvetica-Bold":
		f = &helveticaBold
	case "Courier":
		f = &courier
	default:
		f = &timesRoman
	}
	return f.width(r)
}
