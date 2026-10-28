/*
 * typed_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing a note by naming it, or by saying how far it is from the last one.
 *
 * Both are how somebody typing a tune in thinks about it. "c4 d e f g" is a
 * scale; "up a third, down a step" is a melody. Neither is a staff position,
 * which is what the editor asked for, and which is the clef's arithmetic
 * done in your head.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func notesOf(t *testing.T, a *App) []*model.Note {
	t.Helper()
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []*model.Note
	for _, m := range a.score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

func TestANoteCanBeWrittenByName(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=c4&dur=quarter&kind=note")

	got := notesOf(t, h.app)
	if len(got) != 1 {
		t.Fatalf("%d notes written", len(got))
	}
	if got[0].Pitch.Step != model.StepC || got[0].Pitch.Octave != 4 {
		t.Errorf("c4 came out as %v", got[0].Pitch)
	}
}

// A run of bare letters goes the shortest way, which is what a scale is.
func TestBareLettersFollowTheNoteBefore(t *testing.T) {
	h := newHarness(t)
	for _, n := range []string{"c4", "d", "e", "f", "g"} {
		h.post("/api/insert?part=0&measure=0&at=-1&pitch=" + n + "&dur=quarter&kind=note")
	}
	got := notesOf(t, h.app)
	if len(got) != 5 {
		t.Fatalf("%d notes written", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Pitch.MIDI() <= got[i-1].Pitch.MIDI() {
			t.Errorf("note %d (%v) is not above the one before (%v): a scale "+
				"came out jumping octaves", i, got[i].Pitch, got[i-1].Pitch)
		}
	}
}

// And down, where down is nearer: c then b is a semitone, not a seventh.
func TestABareLetterGoesDownWhenDownIsNearer(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=c4&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=b&dur=quarter&kind=note")

	got := notesOf(t, h.app)
	if d := got[0].Pitch.MIDI() - got[1].Pitch.MIDI(); d != 1 {
		t.Errorf("c then b came out %d semitones apart, want 1", d)
	}
}

// Up a third, down a step: the way a melody is said out loud.
func TestANoteCanBeWrittenByItsIntervalFromTheLast(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=c4&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&interval=2&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&interval=-1&dur=quarter&kind=note")

	got := notesOf(t, h.app)
	if len(got) != 3 {
		t.Fatalf("%d notes written", len(got))
	}
	if got[1].Pitch.Step != model.StepE || got[1].Pitch.Octave != 4 {
		t.Errorf("a third above c4 came out as %v", got[1].Pitch)
	}
	if got[2].Pitch.Step != model.StepD || got[2].Pitch.Octave != 4 {
		t.Errorf("a step below e4 came out as %v", got[2].Pitch)
	}
}

// The interval is counted from the note before wherever it is, including
// across a barline: a melody does not start again at every bar.
func TestAnIntervalReachesBackAcrossTheBarline(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=g4&dur=whole&kind=note")
	h.post("/api/insert?part=0&measure=1&at=-1&interval=1&dur=whole&kind=note")

	got := notesOf(t, h.app)
	if len(got) != 2 {
		t.Fatalf("%d notes written", len(got))
	}
	if got[1].Pitch.Step != model.StepA || got[1].Pitch.Octave != 4 {
		t.Errorf("a step above g4 in the next bar came out as %v", got[1].Pitch)
	}
}

// A letter typed with no accidental takes the key's; one typed with its own
// means what it says.
func TestTheKeySpellsABareLetterAndNotOneWithAnAccidental(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?part=0&fifths=2") // D major: F sharp and C sharp
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=f4&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=fn4&dur=quarter&kind=note")

	got := notesOf(t, h.app)
	if len(got) != 2 {
		t.Fatalf("%d notes written", len(got))
	}
	if got[0].Pitch.Alter != 1 {
		t.Errorf("a bare f in D major came out as %v, want the key's F sharp",
			got[0].Pitch)
	}
	if got[1].Pitch.Alter != 0 {
		t.Errorf("f natural came out as %v", got[1].Pitch)
	}
}

func TestWhatIsNotAPitchIsRefusedWithAReason(t *testing.T) {
	h := newHarness(t)
	why := h.postFails("/api/insert?part=0&measure=0&at=-1&pitch=zz9&dur=quarter&kind=note")
	if why == "" {
		t.Error("it was refused without a reason")
	}
}
