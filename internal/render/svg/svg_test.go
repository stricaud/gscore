/*
 * svg_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package svg

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

func render(t *testing.T, lay *engrave.Layout) string {
	t.Helper()
	var b strings.Builder
	opt := DefaultOptions()
	opt.EmbedFont = false
	if err := Render(&b, lay, opt); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestASizedGlyphIsActuallyDrawnSmaller is the check that would have caught
// the bug it is named for.
//
// The size went out as a font-size presentation attribute, which is
// author-level styling of the lowest priority — so the stylesheet's own .g
// rule beat it, and every glyph that asked to be drawn smaller came out full
// size. Nothing anywhere said so: the layout was right, the renderer ran
// without complaint, and only the picture was wrong.
func TestASizedGlyphIsActuallyDrawnSmaller(t *testing.T) {
	lay := &engrave.Layout{
		Width: 20, Height: 20,
		Glyphs: []engrave.Glyph{
			{Name: "noteheadBlack", X: 1, Y: 5},            // full size
			{Name: "noteheadBlack", X: 5, Y: 5, Size: 2.4}, // cue size
		},
	}
	out := render(t, lay)

	sizes := regexp.MustCompile(`font-size:([\d.]+)px`).FindAllStringSubmatch(out, -1)
	if len(sizes) == 0 {
		t.Fatal("no glyph carried a size of its own: the small one is drawn full size")
	}
	got, err := strconv.ParseFloat(sizes[len(sizes)-1][1], 64)
	if err != nil {
		t.Fatal(err)
	}

	// The default is four staff spaces, so a glyph asked for at 2.4 has to
	// come out at 60% of it.
	full := regexp.MustCompile(`\.g\{[^}]*font-size:([\d.]+)px`).FindStringSubmatch(out)
	if full == nil {
		t.Fatal("the stylesheet sets no default glyph size")
	}
	base, _ := strconv.ParseFloat(full[1], 64)
	if ratio := got / base; ratio < 0.55 || ratio > 0.65 {
		t.Errorf("the cue glyph is %.0f%% of full size, want 60%%", ratio*100)
	}
}

// TestInlineSizeBeatsTheStylesheet states the rule the fix turns on, so that
// going back to a presentation attribute fails here rather than in a picture
// somebody has to look at.
func TestInlineSizeBeatsTheStylesheet(t *testing.T) {
	lay := &engrave.Layout{
		Width: 20, Height: 20,
		Glyphs: []engrave.Glyph{{Name: "noteheadBlack", X: 1, Y: 5, Size: 2.4}},
	}
	out := render(t, lay)

	if strings.Contains(out, ` font-size="`) {
		t.Error("the size is a presentation attribute, which the .g rule overrides")
	}
	if !strings.Contains(out, `style="font-size:`) {
		t.Error("the size is not an inline style, so the stylesheet wins")
	}
}

// TestBeamsAreFilledQuads checks that beams reach the SVG at all.
//
// They moved out of Lines and into Quads to get their ends cut vertically. A
// renderer that was not told would stop drawing them, silently and only in
// its own output format.
func TestBeamsAreFilledQuads(t *testing.T) {
	lay := &engrave.Layout{
		Width: 20, Height: 20,
		Quads: []engrave.Quad{{
			X: [4]float64{2, 8, 8, 2},
			Y: [4]float64{4, 3, 3.5, 4.5},
		}},
	}
	out := render(t, lay)
	if !strings.Contains(out, "<polygon") {
		t.Fatal("the beam was not drawn")
	}
	if !strings.Contains(out, `fill="#000"`) {
		t.Error("the beam is not filled")
	}
}

// A fragment is the shapes alone, to go inside a drawing already on the page.
//
// It is how a long score is drawn a piece at a time: the editor keeps one
// <svg> the size of the whole score and fills it with these, adding the
// piece ahead and dropping the one behind. Wrapped in its own <svg> a piece
// cannot be put inside anything, and carrying its own stylesheet would be
// the same rules over again for every piece on the page.
func TestAFragmentIsTheShapesAlone(t *testing.T) {
	lay := &engrave.Layout{
		Width: 40, Height: 10,
		Lines:  []engrave.Line{{X1: 1, Y1: 1, X2: 9, Y2: 1, Thickness: 0.1}},
		Glyphs: []engrave.Glyph{{Name: "noteheadBlack", X: 5, Y: 5}},
	}
	opt := Options{StaffSpace: 8}

	var whole strings.Builder
	if err := Render(&whole, lay, opt); err != nil {
		t.Fatal(err)
	}
	opt.Fragment = true
	var part strings.Builder
	if err := Render(&part, lay, opt); err != nil {
		t.Fatal(err)
	}

	for _, s := range []string{"<svg", "</svg>", "<style>"} {
		if strings.Contains(part.String(), s) {
			t.Errorf("a fragment carries %s, so it cannot go inside a drawing", s)
		}
		if !strings.Contains(whole.String(), s) {
			t.Errorf("a whole drawing has no %s", s)
		}
	}
	// The same shapes either way: a fragment is the same picture without
	// the paper round it.
	for _, s := range []string{"<line", "<text"} {
		if !strings.Contains(part.String(), s) {
			t.Errorf("a fragment left out its %s", s)
		}
	}
}
