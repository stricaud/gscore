/*
 * align_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// What sounds together is written in one column on every staff: four
// quarters over eight eighths, and over a bar only half written. The second
// is bar 28 of Black and Tan Fantasy, where a left hand's eighths, spread
// over the whole bar because the rest of it was not written yet, landed on
// the right hand's notes at moments they do not sound at.
func TestTheSameMomentIsTheSameColumnOnEveryStaff(t *testing.T) {
	note := func(oct, den int) model.Element {
		return &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: oct},
			Dur: model.Duration{Num: 1, Den: den}}
	}
	run := func(n, oct, den int) []model.Element {
		var out []model.Element
		for i := 0; i < n; i++ {
			out = append(out, note(oct, den))
		}
		return out
	}
	for _, c := range []struct {
		name         string
		upper, lower []model.Element
	}{
		{"quarters over eighths", run(4, 5, 4), run(8, 3, 8)},
		{"over half a bar", run(4, 5, 4), run(4, 3, 8)},
	} {
		lay := Engrave(pianoWith(c.upper, c.lower), DefaultOptions())
		xs := map[float64][]float64{}
		for _, h := range lay.Hits {
			for _, el := range h.Elements {
				xs[el.At] = append(xs[el.At], el.X)
			}
		}
		shared := 0
		for at, x := range xs {
			if len(x) < 2 {
				continue
			}
			shared++
			if math.Abs(x[0]-x[1]) > 0.01 {
				t.Errorf("%s: at %.3f of the bar the staves are at %.2f and %.2f",
					c.name, at, x[0], x[1])
			}
		}
		if shared == 0 {
			t.Errorf("%s: no moment found on both staves", c.name)
		}
	}
}

// A note has room up to the next moment on any staff, not only up to its
// own staff's next note. Bar 28 again: the left hand's last eighth, at the
// moment before the right hand's eighth, was left under a space by a right
// hand that had been resting until then, and the two were drawn on top of
// each other.
func TestANoteHasRoomUntilTheNextMomentOnAnyStaff(t *testing.T) {
	d := func(n, den int) model.Duration { return model.Duration{Num: n, Den: den} }
	note := func(oct int, dur model.Duration) model.Element {
		return &model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: oct}, Dur: dur}
	}
	upper := []model.Element{&model.Rest{Dur: d(1, 4)}, &model.Rest{Dur: d(1, 4)},
		note(5, d(1, 8)), note(5, d(1, 4)), note(5, d(1, 8))}
	lower := []model.Element{note(3, d(1, 8)), note(3, d(1, 8)), note(3, d(1, 8)), note(3, d(1, 8))}
	lay := Engrave(pianoWith(upper, lower), DefaultOptions())
	var last, next float64
	for _, h := range lay.Hits {
		for _, el := range h.Elements {
			if h.Part == 1 && el.At == 0.375 {
				last = el.X
			}
			if h.Part == 0 && el.At == 0.5 {
				next = el.X
			}
		}
	}
	e := &engraver{opt: DefaultOptions()}
	if gap, want := next-last, e.spanWidth(d(1, 8), nil); gap < want-0.01 {
		t.Errorf("the eighth before the next moment has %.2f spaces; an eighth asks for %.2f", gap, want)
	}
}
