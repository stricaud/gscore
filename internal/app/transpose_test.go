/*
 * transpose_test.go
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

/*
 * Moving music to another key.
 *
 * The part worth testing is the spelling. Anybody can add semitones; what
 * makes a transposition right or wrong is what the notes are called when it
 * lands — whether the third of the chord is an E sharp or an F, whether a
 * note that needed no sign now needs one, and whether the key signature
 * went with the music or stayed behind.
 */

// tuneIn writes a few notes in a key, so what comes back can be read.
func tuneIn(h *harness, fifths int, pitches ...string) {
	p := h.app.score.Parts[0]
	p.Key = model.KeySignature{Fifths: fifths}
	p.Measures = nil
	var els []model.Element
	for _, name := range pitches {
		got, _, ok := model.ParsePitch(name)
		if !ok {
			h.t.Fatalf("%s is not a pitch", name)
		}
		n := &model.Note{Pitch: got, Dur: model.Duration{Num: 1, Den: 4}}
		// Written as the key spells it, which is what the editor stores
		// when the note is typed.
		n.Accidental = signFor(got, p.Key.AlterFor(got.Step))
		els = append(els, n)
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: els}},
		RightBarline: model.BarSingle,
	})
}

// An interval is two numbers: four semitones and two letters is a major
// third, four semitones and three letters is a diminished fourth, and the
// difference is what the player reads.
func TestAnIntervalIsItsLettersAsWellAsItsSemitones(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4")
	h.post("/api/transpose?steps=2&semitones=4&key=0")
	if got := pitches(h); got != "E4" {
		t.Errorf("a major third above C came out as %s, want E4", got)
	}

	h = newHarness(t)
	tuneIn(h, 0, "c4")
	h.post("/api/transpose?steps=3&semitones=4&key=0")
	if got := pitches(h); got != "Fb4" {
		t.Errorf("a diminished fourth above C came out as %s, want Fb4", got)
	}
}

// The whole piece moving is a piece in another key, so the signature goes
// with it and the notes are spelled against the new one.
func TestMovingTheWholePieceTakesTheKeyWithIt(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "d4", "e4", "f4") // C major
	out := h.post("/api/transpose?steps=1&semitones=2&key=1")

	if got, want := pitches(h), "D4 E4 F#4 G4"; got != want {
		t.Fatalf("a tone up came out as %q, want %q", got, want)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != 2 {
		t.Errorf("the key is %d sharps, want the two of D major", got)
	}
	if got := out["fifths"]; got != float64(2) {
		t.Errorf("it answered with %v sharps", got)
	}
	// In D major the F sharp is in the signature, so nothing is written in
	// front of it: the point of moving the key is that the page stays
	// clean.
	if got := signs(t, h); got != "" {
		t.Errorf("the page carries %q, and the new signature already says it", got)
	}
}

// A phrase lifted inside a piece is still in the key it was in, so what
// moved has to say so on the page.
func TestMovingPartOfAPieceLeavesTheKeyAloneAndWritesTheSigns(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "d4", "e4", "f4")
	h.post("/api/transpose?sel=0:0:0,0:0:1&steps=1&semitones=2")

	if got, want := pitches(h), "D4 E4 E4 F4"; got != want {
		t.Fatalf("the bar came out as %q, want %q — the first two moved and no more", got, want)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != 0 {
		t.Errorf("the key moved to %d, and only part of the piece did", got)
	}
}

// The sign a note needs depends on what has been said before it in the bar.
func TestANoteIsWrittenAgainstWhatTheBarHasAlreadySaid(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "c4", "c4")
	// Up a semitone, in C major and without moving the key: the first C
	// sharp needs a sharp written, and the two after it are already
	// sharpened by it.
	h.post("/api/transpose?steps=0&semitones=1&key=0")

	if got, want := pitches(h), "C#4 C#4 C#4"; got != want {
		t.Fatalf("they came out as %q, want %q", got, want)
	}
	// One sharp on the page: the first says it and the bar holds it.
	if got := signs(t, h); got != "#" {
		t.Errorf("the page carries %q, want one sharp on the first of them", got)
	}
}

// Moving by letters alone stays in the key: a third up in C minor is an E
// flat, because that is the third degree of the key and not because
// anybody counted semitones.
func TestADiatonicMoveIsSpelledByTheKey(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, -3, "c4", "d4", "e4") // C minor: three flats
	h.post("/api/transpose?steps=2&mode=diatonic")

	if got, want := pitches(h), "Eb4 F4 G4"; got != want {
		t.Errorf("a third up in three flats came out as %q, want %q", got, want)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != -3 {
		t.Errorf("the key moved to %d: moving by letters stays in the key", got)
	}
}

// A chord goes as one, and keeps its order.
func TestAChordMovesWhole(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4")
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=e&above=1")
	h.post("/api/addpitch?part=0&measure=0&index=0&pitch=g&above=1")
	h.post("/api/transpose?steps=1&semitones=2&key=1")

	ch := chordAt(t, h, 0, 0)
	want := []string{"D4", "F#4", "A4"}
	if len(ch.Notes) != len(want) {
		t.Fatalf("the chord has %d noteheads", len(ch.Notes))
	}
	for i, n := range ch.Notes {
		if n.Pitch.String() != want[i] {
			t.Errorf("notehead %d is %s, want %s", i, n.Pitch, want[i])
		}
	}
}

// A drum staff's lines name instruments, not pitches: moving a snare up a
// tone moves it to a tom, which is nobody's idea of transposing.
func TestADrumStaffIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "d4")
	h.app.score.Parts[0].Clef = model.Percussion

	before := pitches(h)
	if why := h.postFails("/api/transpose?steps=1&semitones=2&key=1"); why == "" {
		t.Fatal("it moved a drum staff")
	}
	if got := pitches(h); got != before {
		t.Errorf("the drums were moved: %q, were %q", got, before)
	}
}

// Nowhere to go is a mistake, and so is four octaves and a bit.
func TestWhatItRefuses(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4")
	for _, q := range []string{
		"/api/transpose?steps=0&semitones=0",
		"/api/transpose?steps=40&semitones=60",
	} {
		if why := h.postFails(q); why == "" {
			t.Errorf("%s was accepted", q)
		}
	}
}

// Everything it does can be taken back, which is the whole reason the
// route is in the undoable list.
func TestTransposingCanBeUndone(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "e4", "g4")
	before := pitches(h)

	h.post("/api/transpose?steps=4&semitones=7&key=1")
	if got := pitches(h); got == before {
		t.Fatal("nothing moved")
	}
	h.post("/api/undo")
	if got := pitches(h); got != before {
		t.Errorf("after undo the notes are %q, were %q", got, before)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != 0 {
		t.Errorf("the key stayed at %d sharps after undo", got)
	}
}
