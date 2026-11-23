/*
 * beamacrossbar_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Two eighths at the end of one bar beamed by hand into the eighth that
// opens the next, as a run swept into the downbeat is written. carry says
// whether the first note of the second bar is marked to finish the beam.
func beamOverTheBar(carry model.Beam) *model.Score {
	d := func(n, den int) model.Duration { return model.Duration{Num: n, Den: den} }
	eighth := func(s model.Step, oct int, b model.Beam) *model.Note {
		n := &model.Note{Pitch: pitch(s, oct), Dur: d(1, 8)}
		n.Beam = b
		return n
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 60, Unit: d(1, 4)},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{
				{
					Voices: []*model.Voice{{Elements: []model.Element{
						&model.Note{Pitch: pitch(model.StepG, 4), Dur: d(3, 4)},
						eighth(model.StepF, 4, model.BeamStart),
						eighth(model.StepB, 4, model.BeamContinue),
					}}},
					RightBarline: model.BarSingle,
				},
				{
					Voices: []*model.Voice{{Elements: []model.Element{
						eighth(model.StepD, 5, carry),
						&model.Rest{Dur: d(1, 8)},
						&model.Rest{Dur: d(3, 4)},
					}}},
					RightBarline: model.BarFinal,
				},
			},
		}},
	}
}

func flagCount(l *Layout) int {
	n := 0
	for _, g := range l.Glyphs {
		if has(g.Name, "flag") {
			n++
		}
	}
	return n
}

func TestABeamCanBeWrittenOverTheBarline(t *testing.T) {
	for _, mode := range []Mode{Paginated, Strip} {
		opt := DefaultOptions()
		opt.Mode = mode
		lay := Engrave(beamOverTheBar(model.BeamStop), opt)

		if n := flagCount(lay); n != 0 {
			t.Errorf("mode %v: %d flags, want none: all three eighths are on one beam", mode, n)
		}
		bar := -1.0
		for _, b := range lay.Barlines {
			if b.Measure == 0 {
				bar = b.X
			}
		}
		if bar < 0 {
			t.Fatalf("mode %v: no barline after the first bar", mode)
		}
		across := false
		for _, q := range lay.Quads {
			lo, hi := q.X[0], q.X[0]
			for _, x := range q.X {
				lo, hi = min(lo, x), max(hi, x)
			}
			if lo < bar && hi > bar {
				across = true
			}
		}
		if !across {
			t.Errorf("mode %v: no beam runs over the barline at %.2f", mode, bar)
		}
	}
}

// Without the note after the barline saying so, the beam stops at the bar:
// the two eighths are beamed to each other and the one after has a flag.
func TestABeamLeftOpenStopsAtTheBarline(t *testing.T) {
	lay := Engrave(beamOverTheBar(model.BeamAuto), DefaultOptions())
	if n := flagCount(lay); n != 1 {
		t.Errorf("%d flags, want one, on the eighth after the barline", n)
	}
}
