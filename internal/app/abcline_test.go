/*
 * abcline_test.go
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
 * Writing a line of ABC into a score that already exists.
 *
 * The editor's own way of typing music is the short one — "8abcdef" is six
 * quavers. ABC is the other, and the point of offering it is that it is a
 * notation a great many people already have in their fingers: it has to be
 * the real thing, read by the same parser that opens an .abc file, or it is
 * a second poorer ABC to keep in step with the first.
 */

// what is in a bar, as the model holds it.
func barOf(t *testing.T, a *App, part, measure int) []model.Element {
	t.Helper()
	p := a.score.Parts[part]
	if measure >= len(p.Measures) {
		t.Fatalf("staff %d has %d bars, not %d", part, len(p.Measures), measure+1)
	}
	return p.Measures[measure].Voices[0].Elements
}

func TestALineOfABCIsWrittenAtTheCursor(t *testing.T) {
	h := newHarness(t)
	out := h.postBody("/api/abc?part=0&measure=0&at=0", "cdef gabc'")
	if got := out["wrote"]; got != float64(8) {
		t.Fatalf("wrote %v things, want 8", got)
	}
	if got := out["trouble"]; got != "" {
		t.Errorf("it had trouble with it: %v", got)
	}

	// Eight eighths fill the bar exactly, so the rest that was holding it
	// open is carried into the next one.
	els := barOf(t, h.app, 0, 0)
	if len(els) != 8 {
		t.Fatalf("the bar holds %d things, want eight", len(els))
	}
	// Eight eighths, climbing from middle C's octave up an octave.
	first, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the first is a %T", els[0])
	}
	if first.Pitch.Step != model.StepC || first.Pitch.Octave != 5 {
		t.Errorf("it starts on %v, and ABC's c is the C above middle C", first.Pitch)
	}
	if want := (model.Duration{Num: 1, Den: 8}); first.Dur.Reduced() != want {
		t.Errorf("it lasts %v, and ABC's default in four four is an eighth", first.Dur)
	}
	last := els[7].(*model.Note)
	if last.Pitch.Step != model.StepC || last.Pitch.Octave != 6 {
		t.Errorf("it ends on %v, and c' is the octave above c", last.Pitch)
	}
}

// The key the cursor is standing in, not ABC's own.
//
// ABC spells its accidentals against the key: "c" in A major is C sharp, and
// the letter says nothing about it. A fragment read on its own would come out
// in C major and every note of it would be wrong by as many semitones as the
// signature has sharps — which is the whole reason the fragment is given a
// header rather than read as it stands.
func TestABCIsReadInTheKeyTheCursorIsIn(t *testing.T) {
	h := newHarness(t)
	h.app.score.Parts[0].Key = model.KeySignature{Fifths: 3} // A major
	h.postBody("/api/abc?part=0&measure=0&at=0", "cdef")

	els := barOf(t, h.app, 0, 0)
	if len(els) != 4 {
		t.Fatalf("the bar holds %d things, want four", len(els))
	}
	// C sharp, D, E, F sharp.
	for i, want := range []int{1, 0, 0, 1} {
		n, ok := els[i].(*model.Note)
		if !ok {
			t.Fatalf("element %d is a %T", i, els[i])
		}
		if n.Pitch.Alter != want {
			t.Errorf("%v is bent by %d, want %d", n.Pitch, n.Pitch.Alter, want)
		}
	}
}

// Chords, ties and rests come through, because it is the real parser.
func TestABCWritesChordsAndTies(t *testing.T) {
	h := newHarness(t)
	h.postBody("/api/abc?part=0&measure=0&at=0", "[ceg]2 d2-d2 z2")

	els := barOf(t, h.app, 0, 0)
	if len(els) != 4 {
		t.Fatalf("the bar holds %d things, want four", len(els))
	}
	ch, ok := els[0].(*model.Chord)
	if !ok {
		t.Fatalf("the first is a %T, want a chord", els[0])
	}
	if len(ch.Notes) != 3 {
		t.Errorf("the chord has %d notes, want three", len(ch.Notes))
	}
	if n := els[1].(*model.Note); !n.TieStart {
		t.Error("the tie does not start")
	}
	if n := els[2].(*model.Note); !n.TieStop {
		t.Error("the tie does not land")
	}
	if _, ok := els[3].(*model.Rest); !ok {
		t.Errorf("the last is a %T, want a rest", els[3])
	}
}

// Where the bars fall is the score's business, and a barline written in the
// ABC still says what kind it is.
//
// The piece has a meter; writing music into it fills the bar and carries the
// rest into the next one, which is what happens when the notes are typed one
// at a time. So the fragment's own bar lines do not place anything — but the
// kind of line each one is follows the note it came after, and lands on
// whichever bar that note ended up in.
func TestABCBarlinesFollowTheNoteTheyCameAfter(t *testing.T) {
	h := newHarness(t)
	// Two full bars of four four, the second closing a repeat.
	h.postBody("/api/abc?part=0&measure=0&at=0", "cdef gabc'|cdef gabc':|")

	p := h.app.score.Parts[0]
	if len(barOf(t, h.app, 0, 0)) != 8 || len(barOf(t, h.app, 0, 1)) != 8 {
		t.Fatalf("the bars hold %d and %d things, want eight each",
			len(barOf(t, h.app, 0, 0)), len(barOf(t, h.app, 0, 1)))
	}
	if got := p.Measures[1].RightBarline; got != model.BarRepeatEnd {
		t.Errorf("the second bar closes with %v, want a repeat", got)
	}
	if got := p.Measures[0].RightBarline; got == model.BarRepeatEnd {
		t.Error("the first bar was given the repeat as well")
	}
}

// Written in where the cursor is, not at the front of the bar.
func TestABCWritesWhereTheCursorIs(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 0, "quarter")
	h.postBody("/api/abc?part=0&measure=0&at=1", "d2")

	els := barOf(t, h.app, 0, 0)
	if len(els) != 2 {
		t.Fatalf("the bar holds %d things, want two", len(els))
	}
	if n, ok := els[1].(*model.Note); !ok || n.Pitch.Step != model.StepD {
		t.Errorf("the second thing in the bar is %v", els[1])
	}
}

// A line with nothing readable in it is refused, and says so.
//
// Refused whole: a line that writes half of itself and then stops leaves the
// music in a state nobody asked for and the cursor somewhere in the middle
// of it.
func TestABCSaysWhatItCouldNotRead(t *testing.T) {
	h := newHarness(t)
	// A new score's first bar holds the rest that fills it, which is what
	// "nothing has been written here" looks like.
	before := len(barOf(t, h.app, 0, 0))
	if why := h.postFails("/api/abc?part=0&measure=0&at=0"); why == "" {
		t.Error("an empty line was refused without a reason")
	}
	if got := len(barOf(t, h.app, 0, 0)); got != before {
		t.Errorf("a refused line left %d things in the bar, and there were %d",
			got, before)
	}
}
