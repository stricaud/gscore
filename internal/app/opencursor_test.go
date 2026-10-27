/*
 * opencursor_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where a score opens.
 *
 * It used to open at the last bar of the first staff, on the reasoning that
 * a piece somebody opens to add to is a piece they want to carry on
 * writing. That is true of a twelve-bar tune and wrong about everything
 * else: open Beethoven's ninth and the page is at bar two thousand six
 * hundred and seventeen, which is nowhere anybody asked to be and a long
 * way back from where they wanted to start reading.
 *
 * A score opens at its beginning, like any other document. Carrying on from
 * the end is something somebody asks for, and the staff sprints in the
 * editor are how they ask.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func barsOf(n int) []*model.Measure {
	out := make([]*model.Measure, n)
	for i := range out {
		out[i] = &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{Dur: model.Duration{Num: 1, Den: 4}},
			&model.Note{Dur: model.Duration{Num: 1, Den: 4}},
		}}}}
	}
	return out
}

func TestAScoreOpensAtItsBeginning(t *testing.T) {
	a := &App{score: &model.Score{Parts: []*model.Part{{Measures: barsOf(2617)}}}}

	got := a.cursorInfo()
	if got.Measure != 0 {
		t.Errorf("the score opened at bar %d, want the first", got.Measure+1)
	}
	if got.At != 0 {
		t.Errorf("the cursor is %d elements into the bar, want the start of it", got.At)
	}
	if got.Step != middleStaffStep {
		t.Errorf("the cursor is at step %d, want the middle line", got.Step)
	}
	if got.Part != 0 {
		t.Errorf("the cursor is on staff %d, want the first", got.Part)
	}
}

func TestAFileThatRemembersWhereItWasOpensThere(t *testing.T) {
	// A .gscore records it, and that is somebody's own place in their own
	// score. The beginning is only the answer when nothing has been said.
	s := &model.Score{Parts: []*model.Part{{Measures: barsOf(40)}}}
	s.Cursor = &model.Cursor{Part: 0, Measure: 31, At: 1, Step: 6}
	a := &App{score: s}

	got := a.cursorInfo()
	if got.Measure != 31 || got.At != 1 || got.Step != 6 {
		t.Errorf("opened at bar %d element %d step %d, want where the file said: 32, 1, 6",
			got.Measure+1, got.At, got.Step)
	}
}

func TestAnEmptyScoreHasNowhereToPutTheCursor(t *testing.T) {
	// A staff with no bars in it, and a score with no staves: both have to
	// answer something, because there is a cursor on the page from the
	// moment it opens.
	for _, tc := range []struct {
		name  string
		score *model.Score
	}{
		{"no staves", &model.Score{}},
		{"a staff with no bars", &model.Score{Parts: []*model.Part{{}}}},
	} {
		a := &App{score: tc.score}
		if got := a.cursorInfo(); got.Measure != 0 || got.At != 0 {
			t.Errorf("%s: %+v, want the origin", tc.name, got)
		}
	}
}
