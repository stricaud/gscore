/*
 * typedchord_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing a chord by naming its notes.
 *
 * Typing "[ceg]" is how somebody who types music writes a chord, and the
 * editor builds one the way the mouse does: the bottom notehead is written
 * as a note, and the others are added to it. What the typed line could not
 * say before is which pitch each of them is, since a click says it by staff
 * position and a letter does not.
 *
 * The one rule a list of letters needs is which way it is read. A chord is
 * read and written from the bottom up, so each notehead stands above the one
 * before it however far that is: "[ca]" is a sixth, and the nearest A to a C
 * is the one below it.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// typedChord writes a chord the way the typed line writes one: the bottom
// notehead as a note, then the rest of them added to it.
func typedChord(t *testing.T, h *harness, dur string, pitches ...string) {
	t.Helper()
	out := h.post("/api/insert?part=0&measure=0&at=-1&kind=note&dur=" + dur +
		"&pitch=" + pitches[0])
	// The answer gives the place after the note, so the note is one before.
	at, ok := out["at"].(float64)
	if !ok {
		t.Fatalf("the note was written and the answer did not say where: %v", out)
	}
	for _, pitch := range pitches[1:] {
		h.post("/api/addpitch?part=0&measure=0&above=1" +
			"&index=" + itoa(int(at)-1) + "&pitch=" + pitch)
	}
}

func TestATypedChordsNoteheadsStackUpwards(t *testing.T) {
	h := newHarness(t)
	typedChord(t, h, "quarter", "c", "e", "g")

	ch := chordAt(t, h, 0, 0)
	if len(ch.Notes) != 3 {
		t.Fatalf("the chord has %d noteheads, want three", len(ch.Notes))
	}
	want := []model.Step{model.StepC, model.StepE, model.StepG}
	for i, n := range ch.Notes {
		if n.Pitch.Step != want[i] {
			t.Errorf("notehead %d is %v, want %v", i, n.Pitch.Step, want[i])
		}
		if i > 0 && n.Pitch.DiatonicIndex() <= ch.Notes[i-1].Pitch.DiatonicIndex() {
			t.Errorf("notehead %d (%v) is not above the one below it (%v)",
				i, n.Pitch, ch.Notes[i-1].Pitch)
		}
	}
}

// The rule, in the one place it shows: a sixth and not an A under the C.
func TestATypedChordReadsUpwardsHoweverFarThatIs(t *testing.T) {
	h := newHarness(t)
	typedChord(t, h, "quarter", "c", "a")

	ch := chordAt(t, h, 0, 0)
	if len(ch.Notes) != 2 {
		t.Fatalf("the chord has %d noteheads, want two", len(ch.Notes))
	}
	low, high := ch.Notes[0].Pitch, ch.Notes[1].Pitch
	if low.Step != model.StepC || high.Step != model.StepA {
		t.Fatalf("[ca] came out as %v over %v", high, low)
	}
	if got := high.DiatonicIndex() - low.DiatonicIndex(); got != 5 {
		t.Errorf("the A is %d steps from the C, want the five of a sixth "+
			"(%v over %v)", got, high, low)
	}
}

// An octave written out is taken as it is written, which is the only way to
// say a chord wider than an octave.
func TestATypedChordKeepsTheOctavesItWasGiven(t *testing.T) {
	h := newHarness(t)
	typedChord(t, h, "quarter", "c4", "g4", "e5")

	ch := chordAt(t, h, 0, 0)
	want := []model.Pitch{
		{Step: model.StepC, Octave: 4},
		{Step: model.StepG, Octave: 4},
		{Step: model.StepE, Octave: 5},
	}
	if len(ch.Notes) != len(want) {
		t.Fatalf("the chord has %d noteheads, want %d", len(ch.Notes), len(want))
	}
	for i, n := range ch.Notes {
		if n.Pitch.Step != want[i].Step || n.Pitch.Octave != want[i].Octave {
			t.Errorf("notehead %d is %v, want %v", i, n.Pitch, want[i])
		}
	}
}

// A chord is one sound, and a sound has one length: the length typed in
// front of it is the whole chord's.
func TestATypedChordIsOneSoundOfOneLength(t *testing.T) {
	h := newHarness(t)
	typedChord(t, h, "half", "c", "e", "g")

	ch := chordAt(t, h, 0, 0)
	half := model.Duration{Num: 1, Den: 2}
	if ch.Dur.Cmp(half) != 0 {
		t.Errorf("the chord is %v long, want a half note", ch.Dur)
	}
	for i, n := range ch.Notes {
		if n.Dur.Cmp(half) != 0 {
			t.Errorf("notehead %d is %v long, want a half note", i, n.Dur)
		}
	}
	// One sound: the bar holds the chord and nothing beside it.
	if els := h.elements(0, 0); len(els) != 1 {
		t.Errorf("the bar holds %d things, want the one chord", len(els))
	}
}

// A click still asks for the pitch nearest the position it landed on, which
// is what it has always asked for and is not what a typed letter means.
func TestAClickedNoteheadIsStillPlacedByItsPosition(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 0, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=2")

	ch := chordAt(t, h, 0, 0)
	if len(ch.Notes) != 2 {
		t.Fatalf("the chord has %d noteheads, want two", len(ch.Notes))
	}
	if got := ch.Notes[1].Pitch.DiatonicIndex() - ch.Notes[0].Pitch.DiatonicIndex(); got != 2 {
		t.Errorf("the two steps between the positions came out as %d", got)
	}
}
