/*
 * alteredunison_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Building an altered unison in the editor.
 *
 * The engraver sets an F and an F sharp in one chord side by side, but the
 * editor would not let one be written: a second note on a step that was
 * taken was refused, whatever its accidental. Reported as "why do I have
 * this for rule h", with the refusal on the screen.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func chordNotes(t *testing.T, h *harness) []*model.Note {
	t.Helper()
	el := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0]
	ch, ok := el.(*model.Chord)
	if !ok {
		t.Fatalf("the element is a %T, want a chord", el)
	}
	return ch.Notes
}

func TestANoteWithADifferentAccidentalJoinsTheSameStep(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=f4&dur=quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=f%234")

	notes := chordNotes(t, h)
	if len(notes) != 2 {
		t.Fatalf("the chord has %d notes, want the F and the F sharp", len(notes))
	}
	// Lower pitch first, and both signs written: the F gets its natural
	// so that the two heads on one line say which is which.
	if notes[0].Pitch.Alter != 0 || notes[1].Pitch.Alter != 1 {
		t.Errorf("the notes are altered %d and %d, want 0 then 1", notes[0].Pitch.Alter, notes[1].Pitch.Alter)
	}
	if notes[0].Accidental != model.Natural {
		t.Errorf("the F carries %v, want a natural written on it", notes[0].Accidental)
	}
	if notes[1].Accidental != model.Sharp {
		t.Errorf("the F sharp carries %v, want a sharp", notes[1].Accidental)
	}
}

func TestTheSamePitchTwiceIsStillRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=f4&dur=quarter")
	msg := h.postFails("/api/addpitch?part=0&measure=0&index=0&pitch=f4")
	if !strings.Contains(msg, "already holds") {
		t.Errorf("adding the same pitch again was answered %q", msg)
	}
	// And once the altered unison is there, neither of its notes again.
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=f%234")
	if msg := h.postFails("/api/addpitch?part=0&measure=0&index=0&pitch=f%234"); !strings.Contains(msg, "already holds") {
		t.Errorf("adding the F sharp again was answered %q", msg)
	}
	if n := len(chordNotes(t, h)); n != 2 {
		t.Errorf("the chord has %d notes, want 2", n)
	}
}

// By the staff step and an accidental, which is how the editor's own chord
// command sends it: the cursor on the line the F is on, a sharp picked.
func TestAnAlteredUnisonCanBeWrittenFromTheCursor(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=f4&dur=quarter")
	// F4 on the treble staff is step 1, the first space.
	h.post("/api/addpitch?part=0&measure=0&index=0&step=1&accidental=sharp")
	notes := chordNotes(t, h)
	if len(notes) != 2 || notes[1].Pitch.Step != model.StepF || notes[1].Pitch.Alter != 1 {
		t.Fatalf("the chord came out as %v", notes)
	}
}
