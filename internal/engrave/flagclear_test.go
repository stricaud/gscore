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
