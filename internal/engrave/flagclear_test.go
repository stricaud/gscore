/*
 * flagclear_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A flag whose tail comes down to the height of the next notehead wants
// more air than one that passes above it. See flagClear.
//
// Reported from a bar of 12/8: an eighth on the bottom line of the treble
// staff, beamed to nothing, followed by the note a step above it. The flag
// cleared the head by the usual sliver, and at the height of the tail a
// sliver reads as touching.
func flaggedBeforeAHeadAtItsHeight(next model.Step, octave int) *model.Score {
	e8 := model.Duration{Num: 1, Den: 8}
	first := &model.Note{Pitch: pitch(model.StepE, 4), Dur: e8}
	second := &model.Note{Pitch: pitch(next, octave), Dur: e8}
	third := &model.Note{Pitch: pitch(model.StepA, 4), Dur: e8}
	// Beamed by hand the way the bar was written: the second and third
	// together, the first on its own with a flag.
	second.Marks.Beam = model.BeamStart
	third.Marks.Beam = model.BeamStop
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 12, BeatValue: 8},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Rest{Dur: e8}, first, second, third,
					&model.Note{Pitch: pitch(model.StepF, 4), Dur: model.Duration{Num: 1, Den: 4}},
					&model.Rest{Dur: model.Duration{Num: 3, Den: 4}},
				}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// flagGap is the air between the first flag drawn and the notehead after it.
func flagGap(t *testing.T, lay *Layout) float64 {
	t.Helper()
	glyphs := append([]Glyph(nil), lay.Glyphs...)
	sort.Slice(glyphs, func(i, j int) bool { return glyphs[i].X < glyphs[j].X })
	for i, g := range glyphs {
		if !has(g.Name, "flag") {
			continue
		}
		for _, next := range glyphs[i+1:] {
			if has(next.Name, "notehead") && next.X > g.X {
				return next.X - (g.X + glyphWidth(g))
			}
		}
	}
	t.Fatal("no flag followed by a notehead was drawn")
	return 0
}

func TestAFlagStandsClearOfAHeadAtItsOwnHeight(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip

	// The head a step above the flagged note sits where the tail of the
	// flag comes down.
	near := flagGap(t, Engrave(flaggedBeforeAHeadAtItsHeight(model.StepF, 4), opt))
	if near < afterFlag+flagClear-0.01 {
		t.Errorf("the flag clears a head at its own height by %.2f, want %.2f",
			near, afterFlag+flagClear)
	}
	// A head well above the flag's reach gets the ordinary sliver.
	far := flagGap(t, Engrave(flaggedBeforeAHeadAtItsHeight(model.StepG, 5), opt))
	if far > afterFlag+0.01 {
		t.Errorf("a head the flag passes under was given %.2f of air, want %.2f",
			far, afterFlag)
	}
}

// flagGapOnStaff is the air after every flag on the staff it is drawn on,
// in a layout that may hold several systems: a head on the system below is
// not the next note.
func flagGapsOnStaff(lay *Layout) []float64 {
	glyphs := append([]Glyph(nil), lay.Glyphs...)
	sort.Slice(glyphs, func(i, j int) bool { return glyphs[i].X < glyphs[j].X })
	var out []float64
	for i, g := range glyphs {
		if !has(g.Name, "flag") {
			continue
		}
		for _, next := range glyphs[i+1:] {
			dy := next.Y - g.Y
			if has(next.Name, "notehead") && next.X > g.X && dy > -2 && dy < 10 {
				out = append(out, next.X-(g.X+glyphWidth(g)))
				break
			}
		}
	}
	return out
}

// A line pressed below the width its music asked for used to press every
// column by the same factor, the room booked for a flag with the rest — so
// the collision came back on any full line. The floor is ink, not air, and
// a pressed line keeps it. Reported as "the F as eighth note still has its
// flag colliding, this is back".
func TestAPressedLineKeepsTheAirAfterAFlag(t *testing.T) {
	// Three bars of the same figure on one staff, so that lines of one, two
	// and three bars are all tried, some pulled out and some pressed in.
	s := flaggedBeforeAHeadAtItsHeight(model.StepF, 4)
	for i := 0; i < 2; i++ {
		more := flaggedBeforeAHeadAtItsHeight(model.StepF, 4)
		s.Parts[0].Measures = append(s.Parts[0].Measures, more.Parts[0].Measures[0])
	}
	want := afterFlag + flagClear
	for lw := 30.0; lw <= 90; lw += 4 {
		opt := DefaultOptions()
		opt.Mode = Paginated
		opt.LineWidth = lw
		lay := Engrave(s, opt)
		gaps := flagGapsOnStaff(lay)
		if len(gaps) != 3 {
			t.Fatalf("line width %.0f: %d flags measured, want 3", lw, len(gaps))
		}
		for _, g := range gaps {
			if g < want-0.01 {
				t.Errorf("line width %.0f: a flag clears the next head by %.2f, want %.2f",
					lw, g, want)
				break
			}
		}
	}
}
