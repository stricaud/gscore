/*
 * clefoctave_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The 8 under a clef.
 *
 * A guitar and a tenor voice are written in a treble clef with a little 8
 * below it: the notes are read where they are drawn and sound an octave
 * lower. NoteWorthy writes that as a number beside the clef, and this
 * reader threw it away — so every one of those staves came out sounding an
 * octave high, with nothing anywhere to say why.
 */

package nwc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// clefRecord is a clef object: the type, two flag bytes, the clef and the
// octave it carries.
func clefRecord(clef, shift byte) []byte {
	return []byte{0x00, 0x00, 0x00, clef, 0x00, shift, 0x00}
}

func TestAClefCanCarryAnOctave(t *testing.T) {
	for _, c := range []struct {
		what  string
		shift byte
		want  int
	}{
		{"no 8 at all", 0, 0},
		{"an 8 above", 1, 1},
		{"an 8 below", 2, -1},
	} {
		_, st := run(t, clefRecord(0, c.shift), 1)
		if got := st.part.Transpose.OctaveChange; got != c.want {
			t.Errorf("a treble clef with %s transposes the staff by %d "+
				"octaves, want %d", c.what, got, c.want)
		}
		// And the clef itself is still the clef: the octave is what it
		// sounds at, not what it is.
		if st.part.Clef != model.Treble {
			t.Errorf("a treble clef with %s came out as %v",
				c.what, st.part.Clef)
		}
	}
}

func TestTheOctaveIsNotWrittenIntoThePitches(t *testing.T) {
	// gscore holds what the player reads and sounds it through the part's
	// own interval. Written into the pitches instead, a guitar part would
	// print an octave below where it is written — the right music on the
	// wrong page.
	_, st := run(t, clefRecord(0, 2), 1)
	p := st.pitch(0, model.NoAccidental)
	plain := pitchFor(model.Treble, model.KeySignature{}, 0, model.NoAccidental)
	if p.MIDI() != plain.MIDI() {
		t.Errorf("the bottom line reads %v under an octave-down clef and "+
			"%v under a plain one", p, plain)
	}
}

func TestAnOctaveNobodyKnowsIsLeftAlone(t *testing.T) {
	// Three choices are offered where this is written down and three are
	// read. A fourth is something this reader has not met, and moving a
	// staff by a guess is worse than leaving it where it is.
	for _, shift := range []byte{3, 7, 200} {
		_, st := run(t, clefRecord(0, shift), 1)
		if got := st.part.Transpose.OctaveChange; got != 0 {
			t.Errorf("an octave shift of %d moved the staff by %d octaves",
				shift, got)
		}
	}
}

func TestAnOctaveThatChangesPartwayIsReported(t *testing.T) {
	// The staff's own octave belongs to the part. One that changes with a
	// clef partway through belongs to a stretch of it, which the model
	// cannot say — so it is counted and said rather than halved quietly.
	b := append(clefRecord(0, 0), clefRecord(1, 2)...)
	w, st := run(t, b, 2)
	if st.octaveChanges != 1 {
		t.Errorf("%d octave changes were counted, want 1", st.octaveChanges)
	}
	if st.part.Clef != model.Treble {
		t.Errorf("the staff opens in %v, want treble", st.part.Clef)
	}
	_ = w
}
