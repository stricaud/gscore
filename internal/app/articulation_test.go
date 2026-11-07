/*
 * articulation_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which side of a note a mark is written on, and how one is taken off.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

func articulationsOn(h *harness) []engrave.ArticulationPos {
	return engrave.Engrave(h.app.score, engrave.DefaultOptions()).Articulations
}

// A fermata goes over the staff whichever way the stem points.
//
// Reported from a whole note, which has no stem at all: the mark went
// wherever the stem would have been away from, and came out underneath —
// where it reads as belonging to a lower part that is not there. It is an
// instruction to everybody to hold, and every edition writes it above.
func TestAFermataIsWrittenOverTheNote(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=fermata")

	got := articulationsOn(h)
	if len(got) != 1 {
		t.Fatalf("%d marks were drawn, want 1", len(got))
	}
	if !got[0].Above {
		t.Error("the fermata was written under the note")
	}
	if got[0].Mark != "fermata" {
		t.Errorf("the mark is called %q", got[0].Mark)
	}
}

// And a mark can be asked for on either side.
func TestAMarkCanBeAskedForOnEitherSide(t *testing.T) {
	for _, c := range []struct {
		where string
		above bool
	}{{"above", true}, {"below", false}} {
		h := newHarness(t)
		h.note(0, 0, 2, "quarter") // low enough that the stem points up
		h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato" +
			"&placement=" + c.where)

		got := articulationsOn(h)
		if len(got) != 1 {
			t.Fatalf("%s: %d marks were drawn", c.where, len(got))
		}
		if got[0].Above != c.above {
			t.Errorf("asked for %s, it was written above=%v", c.where, got[0].Above)
		}
	}
}

// Taking one off takes that one and leaves the others.
func TestOneMarkCanBeTakenOffWithoutTheRest(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=accent")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato&remove=1")

	note := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if note.HasArticulation(model.Staccato) {
		t.Error("the staccato is still there")
	}
	if !note.HasArticulation(model.Accent) {
		t.Error("the accent went with it")
	}
	// And asking for one that is not there says so rather than doing
	// nothing quietly.
	if msg := h.postFails(
		"/api/articulation?part=0&measure=0&at=1&articulation=staccato&remove=1"); msg == "" {
		t.Error("removing a mark that is not there was taken")
	}
}

// The placement follows the mark it belongs to when another is taken off.
func TestAPlacementFollowsItsOwnMark(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=accent&placement=above")
	h.post("/api/articulation?part=0&measure=0&at=1&articulation=staccato&remove=1")

	note := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if got := note.ArticulationPlacement(0); got != model.PlacementAbove {
		t.Errorf("the accent is placed %v, want above — the staccato's place "+
			"was left behind when it went", got)
	}
}
