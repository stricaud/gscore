/*
 * tievoice_test.go
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
 * A tie that crosses from one voice into another still ends.
 *
 * A chord in one voice is held into the next bar, where the two notes have
 * parted and belong to different voices. That is ordinary piano writing and
 * the Altes does it at bar 17: an F and an A flat tied together, arriving in
 * voices that have separated.
 *
 * Ties were kept per voice, so the note that changed voice had nothing to end
 * on. A tie with no end is drawn to the right margin and picked up on the
 * next line, for ever: on the page, a line that starts and never ends.
 */
func TestATieIntoAnotherVoiceEnds(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	note := func(step model.Step, oct int, start, stop bool) *model.Note {
		n := &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: half}
		n.Marks.TieStart, n.Marks.TieStop = start, stop
		return n
	}

	// Bar 1: both notes in one voice, as a chord, both tied on.
	held := &model.Chord{Dur: half, Notes: []*model.Note{
		note(model.StepF, 4, true, false),
		note(model.StepA, 4, true, false),
	}}
	// Bar 2: the same two pitches, now one in each voice, each ending a tie.
	s := &model.Score{Parts: []*model.Part{{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 2, BeatValue: 4},
		Measures: []*model.Measure{
			{Voices: []*model.Voice{{Elements: []model.Element{held}}},
				RightBarline: model.BarSingle},
			{Voices: []*model.Voice{
				{Elements: []model.Element{note(model.StepA, 4, false, true)}},
				{Elements: []model.Element{note(model.StepF, 4, false, true)}},
			}, RightBarline: model.BarFinal},
		},
	}}}

	lay := Engrave(s, DefaultOptions())
	if len(lay.Curves) != 2 {
		t.Fatalf("%d curves drawn; the two ties should be two", len(lay.Curves))
	}
	// The two notes of bar 2 sound at the same moment, so the two ties end
	// at the same place. One that never found its note is drawn to the right
	// margin instead, which is further along than the note is.
	a, b := lay.Curves[0], lay.Curves[1]
	if d := a.X2 - b.X2; d > 0.01 || d < -0.01 {
		t.Errorf("the ties end at x=%.2f and x=%.2f; both notes are at the "+
			"same moment, so one of them was drawn to the margin instead of "+
			"to its note", a.X2, b.X2)
	}
}
