/*
 * crossstaff_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing a note on the staff next door.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func aGrandStaff(t *testing.T, h *harness) {
	t.Helper()
	h.post("/api/staff?part=0&clef=treble")
	h.post("/api/addstaff?instrument=piano&clef=bass")
}

func TestANoteCanBeWrittenOnTheStaffBelow(t *testing.T) {
	h := newHarness(t)
	aGrandStaff(t, h)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=eighth")
	h.post("/api/crossstaff?part=0&measure=0&at=1&to=below")

	if got := firstNote(t, h, 0).Cross; got != model.OnStaffBelow {
		t.Errorf("the note is written on %v", got)
	}
}

// Nothing about the music changes, which is why it is a mark and not a
// move: the note stays in the voice and bar it was in, so what sounds is
// untouched and so is the rhythm of the figure it belongs to.
func TestWritingItAcrossMovesNothing(t *testing.T) {
	h := newHarness(t)
	aGrandStaff(t, h)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=eighth")
	before := firstNote(t, h, 0).Pitch
	lower := len(h.app.score.Parts[1].Measures[0].Voices[0].Elements)

	h.post("/api/crossstaff?part=0&measure=0&at=1&to=below")

	if got := firstNote(t, h, 0).Pitch; got != before {
		t.Errorf("the pitch became %v, was %v", got, before)
	}
	if got := len(h.app.score.Parts[1].Measures[0].Voices[0].Elements); got != lower {
		t.Errorf("the lower staff went from %d things to %d", lower, got)
	}
}

func TestItCanBeSentBack(t *testing.T) {
	h := newHarness(t)
	aGrandStaff(t, h)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=eighth")
	h.post("/api/crossstaff?part=0&measure=0&at=1&to=below")
	h.post("/api/crossstaff?part=0&measure=0&at=1&to=own")

	if got := firstNote(t, h, 0).Cross; got != model.OnOwnStaff {
		t.Errorf("it is still written on %v", got)
	}
}

func TestANonsenseDirectionIsRefused(t *testing.T) {
	h := newHarness(t)
	aGrandStaff(t, h)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=eighth")
	got := h.postFails("/api/crossstaff?part=0&measure=0&at=1&to=sideways")
	if !strings.Contains(got, "above") {
		t.Errorf("it said %q", got)
	}
	if firstNote(t, h, 0).Cross != model.OnOwnStaff {
		t.Error("it moved the note anyway")
	}
}

// A whole selection at once, which is how a figure is written across: the
// notes that stray are picked up together and sent together.
func TestAWholeFigureGoesAcrossAtOnce(t *testing.T) {
	h := newHarness(t)
	aGrandStaff(t, h)
	for i := 0; i < 4; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=eighth")
	}
	h.post("/api/crossstaff?sel=0:0:1,0:0:2,0:0:3&to=below")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	crossed := 0
	for _, el := range els {
		if n, ok := el.(*model.Note); ok && n.Cross == model.OnStaffBelow {
			crossed++
		}
	}
	if crossed != 3 {
		t.Errorf("%d of them went across, want 3", crossed)
	}
}
