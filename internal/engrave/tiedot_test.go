/*
 * tiedot_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tie leaves a dotted note after its dot.
 *
 * The dot is part of the note, and a tie that set out from the notehead ran
 * straight through it: four tied dotted half notes read as four dots strung
 * on their own curves. Reported as "this is a situation where the tie should
 * start after the dot so we can see it cleanly".
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// dottedTied is two bars of 3/4: a dotted half tied into a dotted half.
// A single note, or a chord of two notes a second apart, whose upper head
// steps across the stem and pushes the dots further out.
func dottedTied(chord bool) *model.Score {
	d := model.Duration{Num: 3, Den: 4}
	make := func(start, stop bool) model.Element {
		if !chord {
			n := &model.Note{Pitch: pitch(model.StepF, 4), Dur: d}
			n.Marks.TieStart, n.Marks.TieStop = start, stop
			return n
		}
		c := &model.Chord{Dur: d, Notes: []*model.Note{
			{Pitch: pitch(model.StepF, 4), Dur: d},
			{Pitch: pitch(model.StepG, 4), Dur: d},
		}}
		c.Marks.TieStart, c.Marks.TieStop = start, stop
		return c
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 3, BeatValue: 4},
			Measures: []*model.Measure{
				{Voices: []*model.Voice{{Elements: []model.Element{make(true, false)}}}},
				{Voices: []*model.Voice{{Elements: []model.Element{make(false, true)}}},
					RightBarline: model.BarFinal},
			},
		}},
	}
}

// dotsEndOf is the right edge of the furthest dot drawn left of x — the
// first note's dots, when x is where its tie arrives — and how many there
// were.
func dotsEndOf(lay *Layout, before float64) (float64, int) {
	end, n := 0.0, 0
	for _, g := range lay.Glyphs {
		if g.Name != "augmentationDot" || g.X >= before {
			continue
		}
		n++
		if right := g.X + smufl.Advance(g.Name)*g.Size/4; right > end {
			end = right
		}
	}
	return end, n
}

func TestATieLeavesADottedNoteAfterItsDot(t *testing.T) {
	for _, chord := range []bool{false, true} {
		opt := DefaultOptions()
		opt.Mode = Strip
		lay := Engrave(dottedTied(chord), opt)
		if len(lay.Curves) == 0 {
			t.Fatalf("chord=%v: no tie was drawn", chord)
		}
		// The first note's dots are the ones before the tie's far end.
		end, n := dotsEndOf(lay, lay.Curves[0].X2)
		if n == 0 {
			t.Fatalf("chord=%v: no dots were drawn", chord)
		}
		// Every curve is a tie here, and every one of them sets out from
		// the first chord: each must begin after the dots.
		for _, c := range lay.Curves {
			if c.X1 < end+tieGap-0.01 {
				t.Errorf("chord=%v: a tie leaves at %.2f, through the dots that finish at %.2f; want %.2f or more",
					chord, c.X1, end, end+tieGap)
			}
		}
	}
}
