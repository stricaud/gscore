/*
 * musescore4_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a PDF from MuseScore 4 looks like to the reader, and what went
 * wrong with it: staff lines stroked a measure at a time, a frame around
 * the page, and the parts' names as text beside the first system.
 */

package engraved

import (
	"testing"

	"github.com/stricaud/gscore/internal/pdf"
)

// fiveLines is a staff of five lines at y, each stroked in pieces — one
// per measure — as MuseScore 4 writes them.
func fiveLines(y, inter float64, pieces [][2]float64) []pdf.Segment {
	var out []pdf.Segment
	for k := 0; k < 5; k++ {
		for _, p := range pieces {
			out = append(out, pdf.Segment{X1: p[0], Y1: y + float64(k)*inter, X2: p[1], Y2: y + float64(k)*inter, Width: 0.28})
		}
	}
	return out
}

func TestStaffLinesStrokedAMeasureAtATimeAreOneStaff(t *testing.T) {
	// An eighteen-stave page has staves ten points high, and a page
	// frame the full width of the paper, which is longer than any of
	// them. A nine-stave arrangement came back as "a scan rather than an
	// engraving" over exactly this.
	c := &pdf.Content{Width: 595, Height: 842}
	c.Segments = append(c.Segments,
		pdf.Segment{X1: 0, Y1: 842, X2: 595, Y2: 842, Width: 0.2},
		pdf.Segment{X1: 0, Y1: 0.3, X2: 595, Y2: 0.3, Width: 0.2},
	)
	measures := [][2]float64{{83, 183}, {183, 300}, {300, 420}, {420, 550}}
	c.Segments = append(c.Segments, fiveLines(400, 2.55, measures)...)
	c.Segments = append(c.Segments, fiveLines(440, 2.55, measures)...)
	// And a beam, which is short and must not count.
	c.Segments = append(c.Segments, pdf.Segment{X1: 100, Y1: 460, X2: 130, Y2: 460, Width: 1})

	sts := staves(c)
	if len(sts) != 2 {
		t.Fatalf("%d staves found, want 2", len(sts))
	}
	for _, st := range sts {
		if st.x0 != 83 || st.x1 != 550 {
			t.Errorf("a staff runs %v to %v, want 83 to 550", st.x0, st.x1)
		}
		if st.inter < 2.5 || st.inter > 2.6 {
			t.Errorf("the interline is %v", st.inter)
		}
	}
}

func TestThePartNameIsReadBesideTheStaff(t *testing.T) {
	c := &pdf.Content{Width: 595, Height: 842}
	c.Segments = fiveLines(400, 2.55, [][2]float64{{83, 550}})
	st := staves(c)[0]
	// "Flûte 2", one letter at a time, each placed by the file, with the
	// font's own advance for each: the gap before the 2 is wider than the
	// e it follows, the gaps inside the word are not.
	size := 5.2
	type l struct {
		r   rune
		x   float64
		adv float64
	}
	for _, g := range []l{{'F', 60, 2.9}, {'l', 62.9, 1.4}, {'û', 64.3, 2.3}, {'t', 66.6, 1.4}, {'e', 68, 2.3}, {'2', 71.9, 2.6}} {
		c.Glyphs = append(c.Glyphs, pdf.Glyph{Rune: g.r, Base: "Edwin-Roman", Size: size, X: g.x, Y: 404, Advance: g.adv})
	}
	// Music on the staff itself, and a dynamic's letters below it, are
	// neither of them the name.
	c.Glyphs = append(c.Glyphs,
		pdf.Glyph{Rune: 0xE0A4, Base: "Leland", Size: 10.2, X: 120, Y: 405},
		pdf.Glyph{Rune: 'p', Base: "Edwin-Italic", Size: 6, X: 120, Y: 380},
	)
	if got := partName(c, st); got != "Flûte 2" {
		t.Errorf("the name read is %q, want Flûte 2", got)
	}
	// A wide letter — an m — is not a space, even without the advance to
	// say so, and a word gap still is.
	c.Glyphs = nil
	for _, g := range []l{{'T', 60, 0}, {'r', 63.2, 0}, {'o', 65, 0}, {'m', 67.6, 0}, {'p', 71.6, 0}, {'e', 74.3, 0}, {'t', 76.6, 0}, {'t', 78, 0}, {'e', 79.5, 0}} {
		c.Glyphs = append(c.Glyphs, pdf.Glyph{Rune: g.r, Base: "Edwin-Roman", Size: size, X: g.x, Y: 404, Advance: g.adv})
	}
	if got := partName(c, st); got != "Trompette" {
		t.Errorf("without advances, the name read is %q", got)
	}
	if got := partName(c, staff{y: [5]float64{600, 602, 604, 606, 608}, x0: 83, x1: 550, inter: 2}); got != "" {
		t.Errorf("a staff with nothing beside it is named %q", got)
	}
}
