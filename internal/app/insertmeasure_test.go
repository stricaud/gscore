/*
 * insertmeasure_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Putting bars in after the one picked.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// barsOn counts a staff's bars.
func barsOn(h *harness, part int) int { return len(h.app.score.Parts[part].Measures) }

func lengthen(h *harness, to int) {
	for h.app.score.MeasureCount() < to {
		h.post("/api/addmeasure")
	}
}

func TestBarsGoInWhereAsked(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 6)
	h.post("/api/insertmeasure?measure=1&count=3")

	if got := h.app.score.MeasureCount(); got != 9 {
		t.Errorf("the score is %d bars, want 9", got)
	}
}

func TestOneBarIsTheDefault(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 4)
	h.post("/api/insertmeasure?measure=0")
	if got := h.app.score.MeasureCount(); got != 5 {
		t.Errorf("the score is %d bars, want 5", got)
	}
}

// Every staff by default, so the barlines stay in a column.
func TestEveryStaffGetsThemByDefault(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=strings.cello")
	lengthen(h, 5)
	h.post("/api/insertmeasure?measure=2&count=2")

	for i := range h.app.score.Parts {
		if got := barsOn(h, i); got != 7 {
			t.Errorf("staff %d has %d bars, want 7", i+1, got)
		}
	}
}

// One staff only: its music moves along against everybody else's, and the
// others are made up at the end so the score is still square.
func TestOneStaffOnlyKeepsTheScoreSquare(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=strings.cello")
	lengthen(h, 5)
	h.post("/api/insertmeasure?measure=1&count=2&where=staff&part=1")

	for i := range h.app.score.Parts {
		if got := barsOn(h, i); got != 7 {
			t.Errorf("staff %d has %d bars, want 7", i+1, got)
		}
	}
	// The bars went into the middle of staff 1 and onto the end of the
	// others, which is the whole difference between the two answers.
	if h.app.score.Parts[1].Measures[2].Voices[0].Elements == nil {
		// A bar put in is a bar of rest, so it is not empty.
		t.Error("the bar put in holds nothing at all")
	}
}

// A bar put in is a bar of rest until something is written in it.
func TestABarPutInIsABarOfRest(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 3)
	h.post("/api/insertmeasure?measure=0&count=1")

	m := h.app.score.Parts[0].Measures[1]
	if len(m.Voices) == 0 || len(m.Voices[0].Elements) == 0 {
		t.Fatal("the bar is empty rather than a bar of rest")
	}
	if _, ok := m.Voices[0].Elements[0].(*model.Rest); !ok {
		t.Errorf("the bar holds a %T", m.Voices[0].Elements[0])
	}
}

// A repeat closing the bar picked, with bars pushed in after it, is a
// repeat of that many bars more.
func TestTheBarlineFollowsTheRun(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 5)
	h.post("/api/barline?part=0&measure=1&style=repeat-end")
	h.post("/api/insertmeasure?measure=1&count=2")

	p := h.app.score.Parts[0]
	if got := p.Measures[1].RightBarline; got != model.BarSingle {
		t.Errorf("the bar picked still closes with %v", got)
	}
	if got := p.Measures[3].RightBarline; got != model.BarRepeatEnd {
		t.Errorf("the last bar put in closes with %v", got)
	}
}

func TestNonsenseIsRefusedAndNothingChanges(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 4)
	before := h.app.score.MeasureCount()

	for _, q := range []string{
		"measure=99", "measure=-1", "measure=0&count=0",
		"measure=0&count=1000", "measure=0&count=x",
		"measure=0&where=staff", "measure=0&where=staff&part=9",
	} {
		if got := h.postFails("/api/insertmeasure?" + q); strings.Contains(got, "meant to be refused") {
			t.Errorf("%q was allowed", q)
		}
	}
	if got := h.app.score.MeasureCount(); got != before {
		t.Errorf("the score went from %d bars to %d", before, got)
	}
}

// It can be taken back, like any other edit.
func TestItCanBeUndone(t *testing.T) {
	h := newHarness(t)
	lengthen(h, 4)
	before := h.app.score.MeasureCount()
	h.post("/api/insertmeasure?measure=1&count=3")
	h.post("/api/undo")
	if got := h.app.score.MeasureCount(); got != before {
		t.Errorf("after undo the score is %d bars, want %d", got, before)
	}
}
