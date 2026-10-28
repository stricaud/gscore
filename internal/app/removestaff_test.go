/*
 * removestaff_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Taking a staff away, and getting it back.
 *
 * Removing one used to be guarded by making somebody type the staff's name,
 * which is a lock rather than a question: it is slow when it works, and on a
 * staff with no name of its own it asked for the words "that staff", which
 * nobody would guess. The guard that is worth having is being able to undo
 * it, and that is what these check — the music comes back, not just the
 * staff.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// notesOn counts what is written on a staff, so that a staff coming back
// can be told from an empty one of the right shape.
func notesOn(p *model.Part) int {
	n := 0
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			n += len(v.Elements)
		}
	}
	return n
}

func TestAStaffComesBackWhenTheRemovalIsUndone(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff")
	h.post("/api/addstaff")
	if n := len(h.app.score.Parts); n != 3 {
		t.Fatalf("%d staves to start with, want 3", n)
	}
	// Something on the middle one, so that what comes back can be told from
	// an empty staff of the right shape.
	h.note(1, 0, 4, "quarter")
	h.note(1, 0, 6, "quarter")
	before := notesOn(h.app.score.Parts[1])
	if before != 2 {
		t.Fatalf("%d notes written on the middle staff, want 2", before)
	}

	h.post("/api/removestaff?part=1")
	if n := len(h.app.score.Parts); n != 2 {
		t.Fatalf("%d staves after removing one, want 2", n)
	}

	h.post("/api/undo")
	if n := len(h.app.score.Parts); n != 3 {
		t.Fatalf("%d staves after undoing the removal, want the 3 there were", n)
	}
	if got := notesOn(h.app.score.Parts[1]); got != before {
		t.Errorf("the staff came back with %d notes on it, want the %d it had",
			got, before)
	}
}

func TestTheLastStaffCannotBeRemoved(t *testing.T) {
	h := newHarness(t)
	if n := len(h.app.score.Parts); n != 1 {
		t.Fatalf("a new score has %d staves", n)
	}
	if msg := h.postFails("/api/removestaff?part=0"); msg == "" {
		t.Error("the only staff was removed, leaving a score with none")
	}
	if n := len(h.app.score.Parts); n != 1 {
		t.Errorf("%d staves left", n)
	}
}

func TestRemovingAStaffThatIsNotThereIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff")
	for _, at := range []string{"-1", "2", "99"} {
		if msg := h.postFails("/api/removestaff?part=" + at); msg == "" {
			t.Errorf("staff %s was removed, and there is no staff %s", at, at)
		}
	}
	if n := len(h.app.score.Parts); n != 2 {
		t.Errorf("%d staves left, want the 2 there were", n)
	}
}
