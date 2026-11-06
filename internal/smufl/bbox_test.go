/*
 * bbox_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package smufl

import (
	"math"
	"testing"
)

// The boxes and the font are a pair, and the pair is what has to hold.
//
// A box is a measurement of a shape. Read for one version of a font and
// applied to another it is a measurement of a different shape, and nothing
// about the resulting page says so. Both files are embedded from the same
// directory of this repository, so this is the check that they came from the
// same place.
func TestTheBoxesBelongToTheFontThatIsShipped(t *testing.T) {
	name, version := BoxFont()
	if name != FontFamily {
		t.Errorf("the boxes are %s's, and the font shipped is %s", name, FontFamily)
	}
	if version <= 0 {
		t.Errorf("the boxes name no font version (%v)", version)
	}
}

func TestEveryGlyphTheFontHasIsMeasured(t *testing.T) {
	// Bravura has rather more glyph names than glyphs, since the names
	// include every optional and alternate form, so this is a floor rather
	// than an equality.
	if n := BoxCount(); n < 3000 {
		t.Errorf("only %d glyphs have a box; the metadata is not what it was", n)
	}
}

// The glyphs the engraver places most all have one, because those are the
// ones a wrong answer shows up on.
func TestTheGlyphsPlacedMostAreMeasured(t *testing.T) {
	for name := range advanceWidths {
		if _, ok := BBox(name); !ok {
			t.Errorf("%s has an advance width and no box", name)
		}
	}
}

// A box has to be the right way up, or every mark placed against it goes the
// wrong way and the page is nonsense rather than slightly wrong.
func TestABoxIsTheRightWayUp(t *testing.T) {
	for name, b := range AllBoxNames() {
		if b.Top < b.Bottom {
			t.Errorf("%s: top %v is below bottom %v", name, b.Top, b.Bottom)
		}
		if b.Right < b.Left {
			t.Errorf("%s: right %v is left of left %v", name, b.Right, b.Left)
		}
	}
}

// Spot checks against the font's own published numbers, so that a regenerated
// data file that lost its scale or its sign says so here.
//
// A notehead is a space tall and sits astride the line it is on. A treble
// clef reaches four and a bit above the line it names, which the engraver's
// old guess was short of by 1.79 spaces. And a whole rest hangs below its
// line rather than sitting on it, reaching 0.036 above and 0.54 below —
// where the guess gave it 1.2 both ways, and so left a space and a sixth of
// nothing above every bar of one.
func TestTheBoxesAreTheNumbersTheFontPublishes(t *testing.T) {
	for _, c := range []struct {
		name                     string
		left, right, bottom, top float64
	}{
		{"noteheadBlack", 0, 1.18, -0.5, 0.5},
		{"accidentalSharp", 0, 0.996, -1.392, 1.4},
		{"accidentalFlat", 0, 0.904, -0.7, 1.756},
		{"gClef", 0, 2.684, -2.632, 4.392},
		{"restWhole", 0, 1.128, -0.54, 0.036},
		{"restHalf", 0, 1.128, -0.008, 0.568},
	} {
		b, ok := BBox(c.name)
		if !ok {
			t.Errorf("%s has no box", c.name)
			continue
		}
		for _, f := range []struct {
			what      string
			got, want float64
		}{
			{"left", b.Left, c.left},
			{"right", b.Right, c.right},
			{"bottom", b.Bottom, c.bottom},
			{"top", b.Top, c.top},
		} {
			if math.Abs(f.got-f.want) > 0.001 {
				t.Errorf("%s %s is %v, want %v", c.name, f.what, f.got, f.want)
			}
		}
	}
}

func TestAnUnknownGlyphHasNoBox(t *testing.T) {
	if _, ok := BBox("noSuchGlyphAnywhere"); ok {
		t.Error("an invented name was given a box")
	}
}
