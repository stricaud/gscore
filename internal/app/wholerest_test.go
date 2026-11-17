/*
 * wholerest_test.go
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

// A whole rest written into an empty bar is the bar's rest, however long
// the bar is: a semibreve rest alone in a bar is a silent bar in any meter.
//
// Taken literally in 12/8 it was two thirds of the bar, drawn at the left
// with a third of a bar of nothing after it. Reported as "I added a whole
// rest, why is it not centred?"
func TestAWholeRestInAnEmptyBarIsTheBarsRest(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&time=12/8")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=whole&kind=rest")

	els := h.elements(0, 0)
	if len(els) != 1 {
		t.Fatalf("the bar holds %d elements, want the one rest", len(els))
	}
	r, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("the bar holds a %T, want a rest", els[0])
	}
	if want := (model.Duration{Num: 12, Den: 8}); r.Dur.Reduced().Cmp(want) != 0 {
		t.Errorf("the rest lasts %s, want the whole bar, %s", r.Dur, want)
	}

	// Which is what makes it a silent bar on the page: full, so centred.
	h.post("/api/insert?part=0&measure=1&at=-1&step=4&dur=quarter&kind=note")
	if n := len(h.elements(0, 1)); n != 1 {
		t.Fatalf("the second bar holds %d elements, want the note: the rest "+
			"left the first bar short and the note went into it", n)
	}
}

// A rest that is not a semibreve means its own length, and a semibreve
// among other things means its own length too.
func TestOtherRestsMeanWhatTheySay(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&time=12/8")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=half&kind=rest")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=whole&kind=rest")
	els := h.elements(0, 0)
	if len(els) != 2 {
		t.Fatalf("the bar holds %d elements, want 2", len(els))
	}
	if want := (model.Duration{Num: 1, Den: 2}); els[0].Duration().Cmp(want) != 0 {
		t.Errorf("the half rest lasts %s", els[0].Duration())
	}
	if want := (model.Duration{Num: 1, Den: 1}); els[1].Duration().Cmp(want) != 0 {
		t.Errorf("the whole rest after it lasts %s, want %s", els[1].Duration(), want)
	}
}
