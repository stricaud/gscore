/*
 * charts_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * What a score holds besides notes.
 *
 * A chart's chords, the brace that says two staves are one instrument, the
 * slashes a rhythm part is written with and the crosses a drum part is drawn
 * with. None of them is a note, all of them were read as nothing, and the
 * page said so: Jingle Bells came out with a wall of five-note chords where
 * its guitar should have had slashes and a chord symbol, two unnamed staves
 * where the piano should have been, and its cymbal a sixth above the staff.
 */

// A chord symbol's root is a scale degree, so it only means anything once
// the key is known: the same 3 is C in G major and F in C major.
func TestAChordRootIsAScaleDegree(t *testing.T) {
	g := model.KeySignature{Fifths: 1}
	c := model.KeySignature{Fifths: 0}
	for _, tc := range []struct {
		key    model.KeySignature
		degree int
		alter  int
		lower  bool
		want   string
	}{
		{g, 0, 0, false, "G"},  // the tonic
		{g, 3, 0, false, "C"},  // Jingle Bells' first chord
		{g, 4, 0, false, "D"},  // and its third
		{g, 6, 0, false, "F#"}, // the leading note keeps the key's sharp
		{g, 6, -1, false, "F"}, // flattened back by the symbol itself
		{c, 3, 0, false, "F"},
		{c, 1, 0, true, "d"}, // lower case, which is how Finale writes a minor triad
		{c, 1, -1, false, "Db"},
	} {
		if got := degreeName(tc.key, tc.degree, tc.alter, tc.lower); got != tc.want {
			t.Errorf("%d fifths, degree %d alter %d: %q, want %q",
				tc.key.Fifths, tc.degree, tc.alter, got, tc.want)
		}
	}
}

// The quality is a row of characters, each either a number or the code of
// one.
func TestAChordQualityIsSpelledOut(t *testing.T) {
	part := func(v int, number bool) chordSuffixPart {
		p := chordSuffixPart{Value: v}
		if number {
			p.IsNumber = &struct{}{}
		}
		return p
	}
	for _, tc := range []struct {
		parts []chordSuffixPart
		want  string
	}{
		// Jingle Bells' D7: one character, the digit seven.
		{[]chordSuffixPart{part('7', false)}, "7"},
		{[]chordSuffixPart{part('m', false), part('7', false)}, "m7"},
		// A six-nine chord, held as two numbers rather than as characters.
		{[]chordSuffixPart{part(6, true), part(9, true)}, "69"},
		// A character of the chord font that stands for something.
		{[]chordSuffixPart{part(0xE872, false), part('7', false)}, "Δ7"},
		// And one that means nothing here is left out rather than printed
		// as whatever letter shares its number.
		{[]chordSuffixPart{part(0xE999, false), part('9', false)}, "9"},
	} {
		if got := suffixText(tc.parts); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.parts, got, tc.want)
		}
	}
}

// A drum staff's numbers are places on the staff, not pitches.
//
// Read as pitches, with the key's tonic added on top, Jingle Bells' cymbal
// came out a sixth above the staff it belongs on.
func TestADrumStaffIsReadAsPositions(t *testing.T) {
	r := &reader{percussion: true, skipped: map[string]int{}}
	// 11 is where Finale writes the cymbal of this piece, and the top line
	// is where it prints it.
	n := r.note(enigNote{HarmLev: 11}, model.Duration{Num: 1, Den: 4},
		model.KeySignature{Fifths: 1})
	if got, want := spell(n.Pitch), "G4"; got != want {
		t.Errorf("the drum note is at %s, want the %s that is the top line", got, want)
	}
	// The key does not touch it: a drum staff has none.
	flat := r.note(enigNote{HarmLev: 11}, model.Duration{Num: 1, Den: 4},
		model.KeySignature{Fifths: -3})
	if flat.Pitch != n.Pitch {
		t.Errorf("the key moved a drum note from %s to %s", spell(n.Pitch), spell(flat.Pitch))
	}
	if n.Pitch.Alter != 0 {
		t.Errorf("a drum note came out with an accidental: %+v", n.Pitch)
	}
}

// And it is drawn with the notehead its layout says, which the font names.
func TestADrumNoteKeepsItsNotehead(t *testing.T) {
	r := &reader{}
	r.doc.Others.PercNotes = []percussionNote{
		{NoteType: 74, HarmLev: 11, Closed: 0xE0A9}, // a cross, for a cymbal
		{NoteType: 2, HarmLev: 3, Closed: 0xE0A4},   // a plain head, for a drum
	}
	if got := r.headFor(74, 11); got != model.HeadX {
		t.Errorf("the cymbal is drawn with %v, want a cross", got)
	}
	if got := r.headFor(2, 3); got != model.HeadNormal {
		t.Errorf("the drum is drawn with %v, want a plain head", got)
	}
	// An instrument the file does not describe is drawn plainly rather than
	// not at all.
	if got := r.headFor(99, 1); got != model.HeadNormal {
		t.Errorf("an unknown instrument came out as %v", got)
	}
}

// A brace says two staves are one instrument; a bracket says several
// instruments belong together.
func TestWhatJoinsTheStaves(t *testing.T) {
	if got := groupSymbol(3); got != model.GroupBrace {
		t.Errorf("Finale's brace came out as %v", got)
	}
	if got := groupSymbol(6); got != model.GroupBracket {
		t.Errorf("Finale's bracket came out as %v", got)
	}
	if got := groupSymbol(0); got != model.GroupLine {
		t.Errorf("a group with no bracket came out as %v", got)
	}
}
