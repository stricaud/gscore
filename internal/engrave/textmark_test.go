/*
 * textmark_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the words written on a note ended up.
 *
 * Only the engraver knows: a text is pushed as far above or below the staff
 * as that stretch of music needs, and until the staff is finished there is
 * no saying how far. Without it being said, words can be written and never
 * pointed at again.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func withWords(t model.TextMark) *Layout {
	note := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	note.Marks.Texts = []model.TextMark{t}
	s := &model.Score{Parts: []*model.Part{{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{{
			Elements: []model.Element{note,
				&model.Rest{Dur: model.Duration{Num: 3, Den: 4}}},
		}}}},
	}}}
	return Engrave(s, DefaultOptions())
}

func TestWordsOnANoteSayWhereTheyLanded(t *testing.T) {
	lay := withWords(model.TextMark{Text: "pizz.", Style: model.TextTechnique})
	if len(lay.TextMarks) != 1 {
		t.Fatalf("%d texts were reported, want 1", len(lay.TextMarks))
	}
	got := lay.TextMarks[0]
	if got.Text != "pizz." {
		t.Errorf("it says %q", got.Text)
	}
	if got.Style != "technique" {
		t.Errorf("it is set as %q, want technique", got.Style)
	}
	if got.Ref == 0 && got.Index != 0 {
		t.Errorf("it belongs to element %d, text %d", got.Ref, got.Index)
	}
	if got.Width <= 0 || got.Height <= 0 {
		t.Errorf("its box is %gx%g", got.Width, got.Height)
	}
	// The box has to be where the words are: they are drawn inside it.
	var found bool
	for _, tx := range lay.Texts {
		if tx.S != "pizz." {
			continue
		}
		found = true
		if tx.X < got.X-0.01 || tx.X > got.X+got.Width+0.01 {
			t.Errorf("the words are at x=%g and the box is %g wide from %g",
				tx.X, got.Width, got.X)
		}
		if tx.Y < got.Y-0.01 || tx.Y > got.Y+got.Height+0.01 {
			t.Errorf("the words are at y=%g and the box runs %g to %g",
				tx.Y, got.Y, got.Y+got.Height)
		}
	}
	if !found {
		t.Error("the words are not on the page at all")
	}
}

// And which side of the staff they were asked for, which is what the window
// that edits them opens on.
func TestWordsSayWhichSideTheyWereAskedFor(t *testing.T) {
	cases := []struct {
		place model.Placement
		want  string
	}{
		{model.PlacementAuto, ""},
		{model.PlacementAbove, "above"},
		{model.PlacementBelow, "below"},
	}
	var ys []float64
	for _, c := range cases {
		lay := withWords(model.TextMark{
			Text: "pizz.", Style: model.TextTechnique, Placement: c.place,
		})
		if len(lay.TextMarks) != 1 {
			t.Fatalf("%v: %d texts", c.place, len(lay.TextMarks))
		}
		if got := lay.TextMarks[0].Place; got != c.want {
			t.Errorf("%v came back as %q, want %q", c.place, got, c.want)
		}
		ys = append(ys, lay.TextMarks[0].Y)
	}
	// Under the staff is further down the page than over it, which is the
	// whole of what the choice is for.
	if ys[2] <= ys[1] {
		t.Errorf("below is at y=%g and above at y=%g", ys[2], ys[1])
	}
}
