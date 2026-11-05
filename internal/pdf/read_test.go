/*
 * read_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
)

// The writer and the reader in one package make the strongest test available:
// draw something at coordinates we chose, read the file back, and insist on
// getting those coordinates. No fixture can go stale and nothing has to be
// believed on trust.
func TestRoundTripLines(t *testing.T) {
	d := New()
	p := d.NewPage(400, 300)
	p.SetLineWidth(2)
	// five lines eight points apart, as a staff
	for i := 0; i < 5; i++ {
		y := Point(100 + 8*i)
		p.Line(50, y, 350, y)
	}
	p.Line(50, 100, 50, 132) // a barline
	p.Close()

	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	c, err := doc.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Width != 400 || c.Height != 300 {
		t.Errorf("page %g×%g, want 400×300", c.Width, c.Height)
	}

	var horiz, vert []Segment
	for _, s := range c.Segments {
		switch {
		case s.Horizontal():
			horiz = append(horiz, s)
		case s.Vertical():
			vert = append(vert, s)
		}
	}
	if len(horiz) != 5 {
		t.Fatalf("got %d horizontal segments, want 5", len(horiz))
	}
	if len(vert) != 1 {
		t.Errorf("got %d vertical segments, want 1", len(vert))
	}
	for i, s := range horiz {
		want := float64(100 + 8*i)
		if math.Abs(s.Y1-want) > 0.01 {
			t.Errorf("line %d at y=%.3f, want %.0f", i, s.Y1, want)
		}
		if math.Abs(s.X1-50) > 0.01 || math.Abs(s.X2-350) > 0.01 {
			t.Errorf("line %d runs %.3f..%.3f, want 50..350", i, s.X1, s.X2)
		}
		if math.Abs(s.Width-2) > 0.01 {
			t.Errorf("line %d is %.3f thick, want 2", i, s.Width)
		}
	}
}

// Glyphs must come back where they were put, because that position is the
// pitch: everything this package is for rests on it being exact.
func TestRoundTripGlyphPositions(t *testing.T) {
	d := New()
	f := d.Standard(Helvetica)
	p := d.NewPage(400, 300)
	want := []struct {
		s    string
		x, y Point
	}{
		{"A", 72, 200},
		{"B", 150.5, 123.25},
		{"C", 300, 60},
	}
	for _, w := range want {
		p.Text(f, 12, w.x, w.y, w.s)
	}
	p.Close()

	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	c, err := doc.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Glyphs) != len(want) {
		t.Fatalf("got %d glyphs, want %d", len(c.Glyphs), len(want))
	}
	for i, w := range want {
		g := c.Glyphs[i]
		if g.Code != uint32(w.s[0]) {
			t.Errorf("glyph %d is %q, want %q", i, rune(g.Code), rune(w.s[0]))
		}
		if math.Abs(g.X-float64(w.x)) > 0.01 || math.Abs(g.Y-float64(w.y)) > 0.01 {
			t.Errorf("glyph %d at (%.3f, %.3f), want (%g, %g)",
				i, g.X, g.Y, w.x, w.y)
		}
		if math.Abs(g.Size-12) > 0.01 {
			t.Errorf("glyph %d is %.3f pt, want 12", i, g.Size)
		}
	}
}

// A transform must be honoured, or everything read from a file that uses one
// — which is every file LilyPond writes, since it works in tenths — lands in
// the wrong place.
func TestRoundTripUnderTransform(t *testing.T) {
	d := New()
	p := d.NewPage(400, 300)
	p.Save()
	p.buf.WriteString("0.5 0 0 0.5 100 50 cm\n") // half size, shifted
	p.SetLineWidth(4)
	p.Line(0, 0, 200, 0)
	p.Restore()
	p.Close()

	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	doc, _ := Parse(buf.Bytes())
	c, err := doc.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(c.Segments))
	}
	s := c.Segments[0]
	// (0,0)..(200,0) halved and shifted is (100,50)..(200,50)
	if math.Abs(s.X1-100) > 0.01 || math.Abs(s.Y1-50) > 0.01 ||
		math.Abs(s.X2-200) > 0.01 || math.Abs(s.Y2-50) > 0.01 {
		t.Errorf("segment (%.2f,%.2f)..(%.2f,%.2f), want (100,50)..(200,50)",
			s.X1, s.Y1, s.X2, s.Y2)
	}
	if math.Abs(s.Width-2) > 0.01 {
		t.Errorf("width %.3f, want 2 (4 halved by the transform)", s.Width)
	}
}

// TestReadEngraved reads a real engraved PDF and reports what it found.
//
// Skipped unless GSCORE_PDF names one, for the same reason the scan test is:
// the files are somebody else's and do not belong in the repository. Point it
// at anything LilyPond, MuseScore or Finale produced.
//
//	GSCORE_PDF=score.pdf go test ./internal/pdf/ -run Engraved -v
func TestReadEngraved(t *testing.T) {
	path := os.Getenv("GSCORE_PDF")
	if path == "" {
		t.Skip("set GSCORE_PDF to an engraved PDF")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	c, err := doc.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: page 1 is %.0f×%.0f pt, %d glyphs, %d segments",
		path, c.Width, c.Height, len(c.Glyphs), len(c.Segments))

	byName := map[string]int{}
	for _, g := range c.Glyphs {
		n := g.Name
		if n == "" && g.Rune != 0 {
			n = fmt.Sprintf("U+%04X", g.Rune)
		}
		if n == "" {
			n = "(unnamed)"
		}
		byName[n]++
	}
	var names []string
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		t.Logf("   %-24s %d", n, byName[n])
	}
	var horiz int
	for _, s := range c.Segments {
		if s.Horizontal() {
			horiz++
		}
	}
	t.Logf("   %d horizontal segments (staff lines, beams, ledgers)", horiz)
}
