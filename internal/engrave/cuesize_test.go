/*
 * cuesize_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * That a cue is actually drawn smaller.
 *
 * cue.go scales every measurement a note is drawn from, and nothing until
 * now asked whether the drawing that came out was any different. A cue at
 * full size is a cue somebody plays.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// barOfFour draws one bar of four quarter notes, cue-sized where small
// says, on the octave given.
//
// The octave matters for the stems: a stem reaches the middle line
// whatever else it does, so on a note near the staff the clamp decides the
// length and the scale never shows. Far above it, the length is the
// length.
func barOfFour(t *testing.T, small bool, octave int) *Layout {
	t.Helper()
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: octave},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		n.Small = small
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	return Engrave(s, DefaultOptions())
}

// noteheads are the glyphs a notehead is drawn with.
func noteheads(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" || g.Name == "noteheadHalf" ||
			g.Name == "noteheadWhole" {
			out = append(out, g)
		}
	}
	return out
}

func TestACueIsDrawnSmallerThanTheMusic(t *testing.T) {
	full := noteheads(barOfFour(t, false, 4))
	cue := noteheads(barOfFour(t, true, 4))
	if len(full) != 4 || len(cue) != 4 {
		t.Fatalf("%d full-size noteheads and %d cue-sized were drawn, "+
			"want four of each", len(full), len(cue))
	}
	// A played note is drawn at the staff's own size, which the drawing
	// says by naming no size at all; a cue names one.
	if full[0].Size != 0 {
		t.Errorf("a played notehead asks for size %g, and should take the "+
			"staff's", full[0].Size)
	}
	// Around three quarters of it, which is the convention in printed
	// parts.
	want := 4 * cueScale
	if math.Abs(cue[0].Size-want) > 0.01 {
		t.Errorf("a cue notehead is drawn at %g, want %g — %.2f of the "+
			"staff's own size", cue[0].Size, want, cueScale)
	}
}

func TestACueKeepsItsPitch(t *testing.T) {
	// The size changes and the position does not. Scaling where a note sits
	// as well as how big it is would move it to a different note, which is
	// the mistake cue.go was written to avoid.
	full := noteheads(barOfFour(t, false, 4))
	cue := noteheads(barOfFour(t, true, 4))
	for i := range full {
		if math.Abs(full[i].Y-cue[i].Y) > 0.01 {
			t.Errorf("notehead %d sits at %g full size and %g as a cue: "+
				"the cue is a different note", i+1, full[i].Y, cue[i].Y)
		}
	}
}

func TestACueStemIsShorterThanAPlayedOne(t *testing.T) {
	// Everything about the note scales, not only the head: a full-size stem
	// on a small notehead is the thing threading the scale through thirty
	// functions was meant to prevent.
	// The line beside the first notehead, which is its stem. Picked that
	// way because stemsOf keeps the barlines too, and a barline is four
	// spaces tall whatever the notes beside it are doing.
	stemBy := func(lay *Layout) float64 {
		heads := noteheads(lay)
		if len(heads) == 0 {
			t.Fatal("no noteheads were drawn")
		}
		for _, l := range stemsOf(lay) {
			if math.Abs(l.X1-heads[0].X) < 1.5 {
				return math.Abs(l.Y2 - l.Y1)
			}
		}
		return 0
	}
	// Just above the middle line, where the stem's own length decides
	// where it ends. Further out, every stem reaches the middle line and
	// the length never shows — which is correct engraving and says
	// nothing about the size it was drawn at.
	full, cue := stemBy(barOfFour(t, false, 5)), stemBy(barOfFour(t, true, 5))
	if full == 0 || cue == 0 {
		t.Fatalf("stems: %g full size, %g as a cue", full, cue)
	}
	if cue >= full {
		t.Errorf("a cue stem is %g long and a played one %g", cue, full)
	}
}
