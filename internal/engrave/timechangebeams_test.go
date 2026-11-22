/*
 * timechangebeams_test.go
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

// A bar after a change of meter is beamed by the meter it is in.
//
// Reported from "I put a spell on you", bar 24: a voice that opens in 4/4 and
// turns to 12/8 at bar 5. Drawn by the part's opening 4/4, the eighth that
// ends each dotted-quarter beat came off its beam and got a flag; spaced by
// the bar's own 12/8, it had been booked no room for one. The flag was drawn
// into the B after it — "the 8th note flag and the quarter note colliding".
func afterAChangeOfMeter() *model.Score {
	d := func(n, den int) model.Duration { return model.Duration{Num: n, Den: den} }
	note := func(s model.Step, oct int, dur model.Duration) *model.Note {
		return &model.Note{Pitch: pitch(s, oct), Dur: dur}
	}
	twelveEight := model.TimeSignature{Beats: 12, BeatValue: 8}
	return &model.Score{
		Tempo: model.Tempo{BPM: 60, Unit: d(1, 4)},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Key:  model.KeySignature{Fifths: 3, Mode: model.Major},
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{
				{
					Voices:       []*model.Voice{{Elements: []model.Element{&model.Rest{Dur: d(1, 1)}}}},
					RightBarline: model.BarSingle,
				},
				{
					Time: &twelveEight,
					Voices: []*model.Voice{{Elements: []model.Element{
						&model.Rest{Dur: d(1, 8)},
						note(model.StepF, 4, d(1, 16)),
						note(model.StepA, 4, d(1, 16)),
						note(model.StepA, 4, d(1, 8)),
						note(model.StepB, 4, d(1, 4)),
						&model.Rest{Dur: d(1, 16)},
						note(model.StepF, 4, d(1, 16)),
						note(model.StepA, 4, d(1, 16)),
						note(model.StepA, 4, d(3, 16)),
						note(model.StepA, 4, d(1, 8)),
						note(model.StepB, 4, d(1, 4)),
						note(model.StepF, 4, d(1, 8)),
					}}},
					RightBarline: model.BarFinal,
				},
			},
		}},
	}
}

func TestABarAfterAChangeOfMeterIsBeamedByIt(t *testing.T) {
	for _, mode := range []Mode{Paginated, Strip} {
		opt := DefaultOptions()
		opt.Mode = mode
		lay := Engrave(afterAChangeOfMeter(), opt)

		glyphs := append([]Glyph(nil), lay.Glyphs...)
		sort.Slice(glyphs, func(i, j int) bool { return glyphs[i].X < glyphs[j].X })
		flags := 0
		for i, g := range glyphs {
			if !has(g.Name, "flag") {
				continue
			}
			flags++
			// Whatever flag is drawn has the room the spacing booked for it.
			for _, next := range glyphs[i+1:] {
				if has(next.Name, "notehead") && next.X > g.X {
					if gap := next.X - (g.X + glyphWidth(g)); gap < afterFlag-0.01 {
						t.Errorf("mode %v: %s is %.2f from the notehead after it, want at least %.2f",
							mode, g.Name, gap, afterFlag)
					}
					break
				}
			}
		}
		// In 12/8 each eighth is on the beam of its dotted-quarter beat; the
		// only flags are the sixteenth after its rest and the last eighth,
		// which has a beat to itself. By the opening 4/4 there were four.
		if flags != 2 {
			t.Errorf("mode %v: %d flags drawn, want 2: the bar was beamed by a meter it is not in", mode, flags)
		}
	}
}
