/*
 * hiddenrest_test.go
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

// Anything picked up can be taken off the page and put back. It keeps its
// room and its time either way; only the printing changes. See
// model.Marks.Hidden.
func TestWhatIsPickedUpCanBeMadeInvisibleAndBack(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&dur=quarter&kind=rest")
	h.note(0, 0, 4, "quarter")

	rest := func() *model.Rest {
		r, ok := h.elements(0, 0)[0].(*model.Rest)
		if !ok {
			t.Fatalf("the first element is a %T, want the rest", h.elements(0, 0)[0])
		}
		return r
	}
	note := func() *model.Note {
		n, ok := h.elements(0, 0)[1].(*model.Note)
		if !ok {
			t.Fatalf("the second element is a %T, want the note", h.elements(0, 0)[1])
		}
		return n
	}

	// The one command turns the thing over.
	d := h.post("/api/hidden?part=0&measure=0&at=1")
	if d["hidden"] != true || !rest().Hidden {
		t.Errorf("the rest is still printed: %v", d)
	}
	if want := (model.Duration{Num: 1, Den: 4}); rest().Dur.Cmp(want) != 0 {
		t.Errorf("hiding the rest changed its length to %s", rest().Dur)
	}
	d = h.post("/api/hidden?part=0&measure=0&at=1")
	if d["hidden"] != false || rest().Hidden {
		t.Errorf("the rest did not come back: %v", d)
	}

	// A note as much as a rest.
	d = h.post("/api/hidden?part=0&measure=0&at=2")
	if d["hidden"] != true || !note().Marks.Hidden {
		t.Errorf("the note is still printed: %v", d)
	}

	// A selection with some of it hidden is hidden as a whole, and only a
	// selection hidden throughout comes back.
	d = h.post("/api/hidden?part=0&measure=0&at=1&sel=0:0:0,0:0:1")
	if d["hidden"] != true || !rest().Hidden || !note().Marks.Hidden {
		t.Errorf("a half-hidden selection was not hidden as a whole: %v", d)
	}
	d = h.post("/api/hidden?part=0&measure=0&at=1&sel=0:0:0,0:0:1")
	if d["hidden"] != false || rest().Hidden || note().Marks.Hidden {
		t.Errorf("a hidden selection did not come back as a whole: %v", d)
	}

	// Said outright, it is what was said.
	h.post("/api/hidden?part=0&measure=0&at=2&hidden=1")
	h.post("/api/hidden?part=0&measure=0&at=2&hidden=1")
	if !note().Marks.Hidden {
		t.Error("hidden=1 twice over showed the note")
	}
	h.post("/api/hidden?part=0&measure=0&at=2&hidden=0")
	if note().Marks.Hidden {
		t.Error("hidden=0 left the note hidden")
	}

	// With nothing there, nothing to hide.
	if msg := h.postFails("/api/hidden?part=0&measure=3&at=1"); msg == "" {
		t.Error("hiding an element of a bar that is not there was taken")
	}
}
