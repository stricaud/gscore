/*
 * repeatmenu_test.go
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

	"github.com/stricaud/gscore/internal/model"
)

// A repeat asked for from the Repeat menu lands on a line that may already
// carry the other half of one. Closing a repeat on the line that opens the
// next is dots on both sides of it, not the loss of whichever came first.
func TestARepeatJoinsTheOneAlreadyOnTheLine(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 12; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}
	bar := func(i int) model.BarlineStyle {
		h.app.mu.RLock()
		defer h.app.mu.RUnlock()
		return h.app.score.Parts[0].Measures[i].RightBarline
	}

	// Bars one and two repeated, then bars three on repeated as well: the
	// line after bar two closes one repeat and opens the next.
	h.post("/api/barline?part=0&measure=1&style=repeat-end&join=1")
	if got := bar(1); got != model.BarRepeatEnd {
		t.Fatalf("the line after bar 2 is %v, want a close repeat", got)
	}
	h.post("/api/barline?part=0&measure=1&style=repeat-start&join=1")
	if got := bar(1); got != model.BarRepeatBoth {
		t.Errorf("the line after bar 2 is %v, want the sign that closes and opens", got)
	}
	// Asked the other way round, the same sign.
	h.post("/api/barline?part=0&measure=2&style=repeat-start&join=1")
	h.post("/api/barline?part=0&measure=2&style=repeat-end&join=1")
	if got := bar(2); got != model.BarRepeatBoth {
		t.Errorf("the line after bar 3 is %v, want the sign that closes and opens", got)
	}

	// Without asking, a kind chosen is the kind drawn: clicking the line and
	// picking "Close repeat" means that and nothing more.
	h.post("/api/barline?part=0&measure=1&style=repeat-end")
	if got := bar(1); got != model.BarRepeatEnd {
		t.Errorf("the line after bar 2 is %v, want the close repeat that was chosen", got)
	}
}
