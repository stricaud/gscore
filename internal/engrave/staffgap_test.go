/*
 * staffgap_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The gap between two staves of a system is a constant until the notes go
// into it.
//
// A run on ledger lines over the lower staff and another hanging under the
// upper one meet in the middle, and then nothing on the page says which
// staff either of them belongs to.

// pair is two staves, each playing one pitch for a bar.
func pair(upper, lower model.Pitch) *model.Score {
	part := func(p model.Pitch) *model.Part {
		var els []model.Element
		for i := 0; i < 4; i++ {
			els = append(els, &model.Note{
				Pitch: p, Dur: model.Duration{Num: 1, Den: 4},
			})
		}
		return &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			}},
		}
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{part(upper), part(lower)},
	}
}

// staffTops is where each staff of the first system was put.
func staffTops(lay *Layout) []float64 {
	seen := map[float64]bool{}
	var out []float64
	for _, h := range lay.Hits {
		if h.Measure != 0 || seen[h.StaffTop] {
			continue
		}
		seen[h.StaffTop] = true
		out = append(out, h.StaffTop)
	}
	return out
}

func gapBetween(t *testing.T, s *model.Score) float64 {
	t.Helper()
	opt := DefaultOptions()
	opt.Mode = Strip
	tops := staffTops(Engrave(s, opt))
	if len(tops) != 2 {
		t.Fatalf("%d staves were drawn", len(tops))
	}
	if tops[0] > tops[1] {
		tops[0], tops[1] = tops[1], tops[0]
	}
	return tops[1] - tops[0]
}

func TestNotesInTheGapPushTheStavesApart(t *testing.T) {
	middle := model.Pitch{Step: model.StepB, Octave: 4}
	plain := gapBetween(t, pair(middle, middle))

	// The upper staff hangs a long way below its own lines and the lower one
	// climbs a long way above its own: left at the plain distance they would
	// be written through each other.
	crowded := gapBetween(t, pair(
		model.Pitch{Step: model.StepC, Octave: 3}, // three ledger lines below
		model.Pitch{Step: model.StepC, Octave: 7}, // and four above
	))
	if crowded <= plain {
		t.Errorf("two staves whose music fills the gap are %.2f apart, the "+
			"same as two that leave it empty (%.2f)", crowded, plain)
	}

	// And far enough apart that the two runs do not meet: the lower staff's
	// top line, less where the upper staff's notes reach down to.
	const step = 0.5                       // a staff step, in spaces
	below := float64(staffHeight*2) * step // C3 is this far under the staff…
	above := float64(staffHeight*2) * step // …and C7 this far over it
	if crowded < below+above {
		t.Errorf("the staves are %.2f apart for music reaching %.2f into the "+
			"gap from either side", crowded, below+above)
	}
}

// Music that stays on its own staff changes nothing: the gap only grows for
// what goes into it, or every ordinary score would be stretched down the
// page for no reason anybody reading it could name.
func TestMusicInsideTheStaffLeavesTheGapAlone(t *testing.T) {
	middle := model.Pitch{Step: model.StepB, Octave: 4}
	plain := gapBetween(t, pair(middle, middle))
	inside := gapBetween(t, pair(
		model.Pitch{Step: model.StepE, Octave: 4}, // the bottom line
		model.Pitch{Step: model.StepF, Octave: 5}, // the top one
	))
	if inside != plain {
		t.Errorf("notes inside the staff moved it: %.2f against %.2f",
			inside, plain)
	}
}
