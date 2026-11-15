/*
 * restvoice_test.go
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

/*
 * Where a rest goes when two voices share a staff.
 *
 * A rest belongs in the middle of the staff, and with two voices on it both
 * of them belong there — so they are written in the same place as readily as
 * two notes are, and unlike two notes they cannot step sideways: a rest and
 * a note sounding at the same moment are at the same point along the bar,
 * because that is what sounding together means. The quarter rest of the
 * second voice was printed over the first voice's held whole note. Reported
 * from the page, with a picture of how it should look.
 *
 * So each voice's rests move out of the other's way, on the side its own
 * stems point, and far enough to clear whatever that other voice is
 * actually playing there.
 */

// soloBar is one bar on a staff nothing else shares.
func soloBar(els ...model.Element) *model.Score {
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 2, BeatValue: 2},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func restTestNote(step model.Step, octave int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: octave},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
}

func restTestRest() *model.Rest {
	return &model.Rest{Dur: model.Duration{Num: 1, Den: 4}}
}

// staffMiddleY is the height of the middle line of the first staff.
func staffMiddleY(lay *Layout) float64 {
	return lay.Hits[0].StaffTop + lay.Hits[0].StaffHeight/2
}

func TestEachVoicesRestsGoToItsOwnSide(t *testing.T) {
	// A rest on the first beat of each voice, and notes after it so that
	// neither voice is nothing but silence.
	score := twoVoices(
		[]model.Element{restTestRest(), restTestNote(model.StepG, 5),
			restTestNote(model.StepG, 5), restTestNote(model.StepG, 5)},
		[]model.Element{restTestRest(), restTestNote(model.StepD, 4),
			restTestNote(model.StepD, 4), restTestNote(model.StepD, 4)},
	)
	lay := Engrave(score, DefaultOptions())

	rests := restsDrawn(lay)
	if len(rests) != 2 {
		t.Fatalf("%d rests drawn, want one for each voice", len(rests))
	}
	// y grows downward, so the upper voice's is the smaller number.
	middle := staffMiddleY(lay)
	var above, below bool
	for _, r := range rests {
		above = above || r.Y < middle
		below = below || r.Y > middle
	}
	if !above || !below {
		t.Errorf("the two rests are at %.2f and %.2f with the middle line "+
			"at %.2f: they are not on opposite sides of it",
			rests[0].Y, rests[1].Y, middle)
	}
}

// A single line of music leaves its rests where they belong, in the middle.
func TestARestOnItsOwnStaffStaysInTheMiddle(t *testing.T) {
	lay := Engrave(soloBar(restTestRest(), restTestNote(model.StepG, 4),
		restTestNote(model.StepG, 4), restTestNote(model.StepG, 4)),
		DefaultOptions())

	rests := restsDrawn(lay)
	if len(rests) != 1 {
		t.Fatalf("%d rests drawn, want one", len(rests))
	}
	if got, want := rests[0].Y, staffMiddleY(lay); got != want {
		t.Errorf("the rest is at %.2f and the middle line at %.2f", got, want)
	}
}

// And it goes further where the other voice is playing where it would land.
//
// Two spaces out of the way is enough nearly always and not always: a second
// voice reaching above the first voice's rest put the rest back onto a
// notehead, which is where it started. The Altes had two of them, an eighth
// rest and a quarter rest each printed exactly over a note.
func TestARestClearsWhatTheOtherVoiceIsPlayingThere(t *testing.T) {
	// The second voice's chord reaches up past where the first voice's rest
	// would ordinarily go.
	high := &model.Chord{
		Notes: []*model.Note{
			restTestNote(model.StepD, 4), restTestNote(model.StepG, 5),
		},
		Dur: model.Duration{Num: 1, Den: 4},
	}
	score := twoVoices(
		[]model.Element{restTestRest(), restTestNote(model.StepG, 5),
			restTestNote(model.StepG, 5), restTestNote(model.StepG, 5)},
		[]model.Element{high, restTestNote(model.StepD, 4),
			restTestNote(model.StepD, 4), restTestNote(model.StepD, 4)},
	)
	lay := Engrave(score, DefaultOptions())

	rests := restsDrawn(lay)
	if len(rests) != 1 {
		t.Fatalf("%d rests drawn, want the one", len(rests))
	}
	// Clear of the chord's top note, which stands in the same column.
	top := lay.Hits[0].StaffTop + stepY(float64(StaffStep(model.Treble,
		model.Pitch{Step: model.StepG, Octave: 5})))
	if rests[0].Y >= top {
		t.Errorf("the rest is at %.2f and the note it has to clear at %.2f",
			rests[0].Y, top)
	}
}

// Wherever the writer puts it, which is the arrow keys. See model.Rest.Lift.
func TestARestGoesWhereItHasBeenPut(t *testing.T) {
	at := func(lift int) float64 {
		r := restTestRest()
		r.Lift = lift
		lay := Engrave(soloBar(r, restTestNote(model.StepG, 4),
			restTestNote(model.StepG, 4), restTestNote(model.StepG, 4)),
			DefaultOptions())
		rests := restsDrawn(lay)
		if len(rests) != 1 {
			t.Fatalf("%d rests drawn, want one", len(rests))
		}
		return rests[0].Y
	}

	was, now := at(0), at(3)
	// Three steps up, and y grows downward.
	if want := was + stepY(3) - stepY(0); now != want {
		t.Errorf("a rest lifted three steps is at %.2f, want %.2f (it was "+
			"at %.2f)", now, want, was)
	}
}
