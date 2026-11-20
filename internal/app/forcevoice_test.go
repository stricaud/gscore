/*
 * forcevoice_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A sign can be insisted on in the second voice too.
 *
 * Asking for the sharp a note already has, by the key or by an earlier
 * sign in the bar, prints it anyway: that is the one way to put a sign on
 * the page that the rules would leave off. It worked in the first voice
 * and not in the second, where the note was asked about as though it
 * stood in the first — so a D sharp after a D sharp was told no sharp was
 * in force, the sign was written as an ordinary one, and the rules took
 * it off again. Reported as "I want to have a sharp sign in this note that
 * is visible because I want to. Even if I know it was already set by a
 * previous note in the same bar."
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestASignCanBeInsistedOnInTheSecondVoice(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=c4&dur=whole")
	m := h.app.score.Parts[0].Measures[0]
	q := model.Duration{Num: 1, Den: 4}
	dSharp := func() *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: model.StepD, Octave: 5, Alter: 1}, Dur: q}
	}
	m.Voices = append(m.Voices, &model.Voice{Elements: []model.Element{
		dSharp(), dSharp(), &model.Rest{Dur: model.Duration{Num: 1, Den: 2}},
	}})
	h.app.score.SpellAccidentals()
	second := m.Voices[1].Elements[1].(*model.Note)
	if second.Accidental != model.NoAccidental {
		t.Fatalf("the second D sharp already carries %v", second.Accidental)
	}

	out := h.post("/api/accidental?accidental=sharp&sel=0:0:1/1")
	if out["forced"] != float64(1) {
		t.Errorf("the editor was told %v notes were marked, want 1", out["forced"])
	}
	h.app.score.SpellAccidentals()
	if !second.Forced || second.Accidental != model.Sharp {
		t.Errorf("the note carries %v, forced %v; want a sharp, insisted on", second.Accidental, second.Forced)
	}
	if second.Pitch.Alter != 1 {
		t.Errorf("the note is altered by %d, want 1 still", second.Pitch.Alter)
	}

	// And asking again takes it off.
	out = h.post("/api/accidental?accidental=sharp&sel=0:0:1/1")
	if out["unforced"] != float64(1) {
		t.Errorf("the editor was told %v notes were cleared, want 1", out["unforced"])
	}
	h.app.score.SpellAccidentals()
	if second.Forced || second.Accidental != model.NoAccidental || second.Pitch.Alter != 1 {
		t.Errorf("after asking again the note carries %v, forced %v, altered %d", second.Accidental, second.Forced, second.Pitch.Alter)
	}
}
