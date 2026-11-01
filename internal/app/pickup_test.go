/*
 * pickup_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * The bar before the first bar.
 *
 * Declaring one is making the first bar short; everything else the
 * engraver already works out, which is the point of doing it this way.
 * What has to hold is that the bar really is that long afterwards, on
 * every staff, that the music in it is still there, and that the
 * numbering follows.
 */

// twoBars is a quarter note at the top of the piece and a full second bar,
// on as many staves as asked for.
func twoBars(h *harness, staves int) {
	s := h.app.score
	s.Parts = s.Parts[:1]
	for len(s.Parts) < staves {
		s.Parts = append(s.Parts, &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		})
	}
	for _, p := range s.Parts {
		p.Time = model.TimeSignature{Beats: 4, BeatValue: 4}
		p.Measures = []*model.Measure{
			{Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur: model.Duration{Num: 1, Den: 4}},
				&model.Rest{Dur: model.Duration{Num: 3, Den: 4}},
			}}}},
			{Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 5},
					Dur: model.Duration{Num: 1, Den: 1}},
			}}}},
		}
	}
}

// barTime is how much time a bar holds on a staff.
func barTime(h *harness, part, measure int) model.Duration {
	m := h.app.score.Parts[part].Measures[measure]
	return m.Voices[0].Duration()
}

func TestDeclaringAnUpbeatShortensTheFirstBar(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)
	h.post("/api/pickup?dur=quarter")

	want := model.Duration{Num: 1, Den: 4}
	if got := barTime(h, 0, 0); got.Cmp(want) != 0 {
		t.Errorf("the first bar holds %v, want a quarter", got)
	}
	// The note that was in it is still in it. pitches reads the first bar,
	// which is the one that changed length.
	if got := pitches(h); got != "G4" {
		t.Errorf("the upbeat holds %q, want the G4 that was written there", got)
	}
	// And the second bar is untouched.
	if got := barTime(h, 0, 1); got.Cmp(model.Duration{Num: 1, Den: 1}) != 0 {
		t.Errorf("the second bar holds %v", got)
	}
}

// Bars stand in the same columns on every staff, so a first bar of one
// beat on one staff and four on another is not a score.
func TestEveryStaffIsMadeTheSameLength(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 3)
	h.post("/api/pickup?dur=half&dots=1")

	want := model.Duration{Num: 3, Den: 8}
	want = model.Duration{Num: 3, Den: 4}
	for i := range h.app.score.Parts {
		if got := barTime(h, i, 0); got.Cmp(want) != 0 {
			t.Errorf("staff %d opens with %v, want a dotted half", i, got)
		}
	}
}

// The numbering is the whole reason for declaring it: the first full bar
// is bar one, and the upbeat is not numbered at all.
func TestTheNumberingFollows(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)

	numbered := func() int {
		o := engrave.DefaultOptions()
		o.Mode = engrave.Strip
		o.ShowMeasureNumbers = true
		o.WriteAhead = false
		lay := engrave.Engrave(h.app.score, o)
		// The number written over the second bar.
		n := 0
		for _, tx := range lay.Texts {
			if tx.S == "2" {
				n++
			}
		}
		return n
	}
	before := numbered()
	h.post("/api/pickup?dur=quarter")
	if after := numbered(); after != 0 || before == 0 {
		t.Errorf("the second bar is numbered 2 before (%d) and after (%d) the "+
			"upbeat was declared; after it, it is bar one", before, after)
	}
}

// Taking it away puts the bar back to its full length.
func TestTakingTheUpbeatAwayFillsTheBar(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)
	h.post("/api/pickup?dur=quarter")
	h.post("/api/pickup?dur=none")

	if got := barTime(h, 0, 0); got.Cmp(model.Duration{Num: 1, Den: 1}) != 0 {
		t.Errorf("the first bar holds %v, want a whole bar", got)
	}
	if got := pitches(h); got != "G4" {
		t.Errorf("the first bar holds %q, want its G4 still there", got)
	}
}

// Nothing is thrown away: a bar with more music in it than the upbeat
// would hold is a bar somebody has written in.
func TestItWillNotCutMusicToFit(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)
	// Fill the first bar with four quarters.
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = []model.Element{
		&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4}},
		&model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4}},
		&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: model.Duration{Num: 1, Den: 4}},
		&model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 5}, Dur: model.Duration{Num: 1, Den: 4}},
	}
	if why := h.postFails("/api/pickup?dur=quarter"); why == "" {
		t.Fatal("it shortened a bar with four beats of music in it")
	}
	if got := barTime(h, 0, 0); got.Cmp(model.Duration{Num: 1, Den: 1}) != 0 {
		t.Errorf("the bar is %v after being refused", got)
	}
}

func TestAnUpbeatIsShorterThanABar(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)
	if why := h.postFails("/api/pickup?dur=breve"); why == "" {
		t.Error("an upbeat longer than the bar was accepted")
	}
}

func TestTheUpbeatCanBeTakenBack(t *testing.T) {
	h := newHarness(t)
	twoBars(h, 1)
	h.post("/api/pickup?dur=quarter")
	h.post("/api/undo")
	if got := barTime(h, 0, 0); got.Cmp(model.Duration{Num: 1, Den: 1}) != 0 {
		t.Errorf("after undo the first bar holds %v, want the whole bar", got)
	}
}
