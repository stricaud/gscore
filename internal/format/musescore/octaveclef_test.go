/*
 * octaveclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musescore

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The treble clef with an 8 under it, which MuseScore calls G8vb.
 *
 * It was read as a plain treble clef, with the notes left where they sound:
 * a tenor part came out an octave below its staff, under the wrong clef.
 * gscore keeps what is written under this clef, with the octave in
 * Transpose — see model.Part.WriteAnOctaveUp.
 */

func TestTheOctaveClefIsReadAsItself(t *testing.T) {
	// A tenor singing the C below middle C, MIDI 48, which is read as
	// middle C.
	s := read(t, strings.Replace(oldScore(`<Measure>`+aQuarter(48)+`</Measure>`),
		`<defaultClef>F</defaultClef>`, `<defaultClef>G8vb</defaultClef>`, 1))
	p := s.Parts[0]
	if p.Clef != model.Treble8vb {
		t.Fatalf("the staff is in clef %v; the file says G8vb", p.Clef)
	}
	if p.Transpose.OctaveChange != -1 {
		t.Errorf("the staff sounds %d octaves from where it is written; "+
			"the clef says one down", p.Transpose.OctaveChange)
	}
	n := firstNote(t, p)
	if want := (model.Pitch{Step: model.StepC, Octave: 4}); n.Pitch != want {
		t.Errorf("the note is written %v, want %v", n.Pitch, want)
	}
	if got := p.Transpose.Apply(n.Pitch).MIDI(); got != 48 {
		t.Errorf("the note sounds as MIDI %d, want 48: what is played "+
			"must be what the file had", got)
	}
}
