/*
 * octaveclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The treble clef with an 8 under it.
 *
 * MusicXML writes the notes under it where they sound: a tenor's middle C
 * is <octave>3</octave>, with the clef saying it is read an octave up.
 * gscore keeps what is written, with the octave in Transpose, so the same
 * note is a C4 on a Treble8vb staff that sounds an octave down.
 *
 * The men's part of "La Mer" came in at sounding pitch and was drawn an
 * octave low: every note hanging off the bottom of the staff on ledger
 * lines, down among the words.
 */

const tenorBar = `
  <measure number="1">
    <attributes>
      <divisions>1</divisions>
      <clef><sign>G</sign><line>2</line><clef-octave-change>-1</clef-octave-change></clef>
    </attributes>
    <note><pitch><step>D</step><octave>3</octave></pitch>
      <duration>2</duration><type>half</type></note>
    <note><chord/><pitch><step>B</step><alter>-1</alter><octave>2</octave></pitch>
      <duration>2</duration><type>half</type></note>
    <note><grace/><pitch><step>E</step><octave>3</octave></pitch><type>eighth</type></note>
    <note><pitch><step>C</step><octave>3</octave></pitch>
      <duration>2</duration><type>half</type></note>
  </measure>`

func TestNotesUnderTheOctaveClefAreWrittenAnOctaveUp(t *testing.T) {
	s := score(t, tenorBar)
	p := s.Parts[0]
	if p.Clef != model.Treble8vb {
		t.Fatalf("the staff is in clef %v; the file says treble with an 8 under it", p.Clef)
	}
	if p.Transpose.OctaveChange != -1 {
		t.Errorf("the staff sounds %d octaves from where it is written; "+
			"the clef says one down", p.Transpose.OctaveChange)
	}
	els := p.Measures[0].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("%d elements read, want a chord and a note", len(els))
	}
	ch, ok := els[0].(*model.Chord)
	if !ok {
		t.Fatalf("the first element is %T, want a chord", els[0])
	}
	for i, want := range []model.Pitch{{Step: model.StepD, Octave: 4}, {Step: model.StepB, Octave: 3, Alter: -1}} {
		if got := ch.Notes[i].Pitch; got != want {
			t.Errorf("chord note %d is %v, want %v: the D below middle C is "+
				"read as the D above it under this clef", i, got, want)
		}
	}
	n, ok := els[1].(*model.Note)
	if !ok {
		t.Fatalf("the second element is %T, want a note", els[1])
	}
	if want := (model.Pitch{Step: model.StepC, Octave: 4}); n.Pitch != want {
		t.Errorf("the note is %v, want %v", n.Pitch, want)
	}
	if n.Marks.Grace == nil || len(n.Marks.Grace.Notes) != 1 {
		t.Fatal("the grace note was lost")
	}
	if g := n.Marks.Grace.Notes[0].Pitch; g.Octave != 4 {
		t.Errorf("the grace note is written in octave %d; it is under the same clef", g.Octave)
	}
	// And the sound is what the file had.
	if got := p.Transpose.Apply(n.Pitch); got.Octave != 3 {
		t.Errorf("the note sounds in octave %d, want 3", got.Octave)
	}
}

func TestNotesUnderTheOctaveClefGoBackOutWhereTheySound(t *testing.T) {
	s := score(t, tenorBar)
	var out bytes.Buffer
	if err := Export(&out, s); err != nil {
		t.Fatal(err)
	}
	x := out.String()
	if !strings.Contains(x, "<clef-octave-change>-1</clef-octave-change>") {
		t.Error("the octave clef was not written back")
	}
	if strings.Contains(x, "<octave>4</octave>") {
		t.Error("a note under the octave clef went out written an octave up:\n" + x)
	}
	// Read again, it is the same music.
	back, err := importer{}.Import(strings.NewReader(x))
	if err != nil {
		t.Fatal(err)
	}
	n := back.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	if want := (model.Pitch{Step: model.StepC, Octave: 4}); n.Pitch != want {
		t.Errorf("after a round trip the note is %v, want %v", n.Pitch, want)
	}
}

func TestAStaffThatLeavesTheOctaveClefIsSaidSo(t *testing.T) {
	bars := tenorBar + `
  <measure number="2">
    <attributes><clef><sign>G</sign><line>2</line></clef></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>4</duration><type>whole</type></note>
  </measure>`
	doc := `<?xml version="1.0"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>Tenor</part-name></score-part></part-list>
  <part id="P1">` + bars + `</part>
</score-partwise>`
	s, err := importer{}.Import(strings.NewReader(doc))
	pe, ok := err.(*PartialError)
	if err != nil && !ok {
		t.Fatal(err)
	}
	// The whole staff moves, the second bar with it.
	n := s.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Note)
	if n.Pitch.Octave != 5 {
		t.Errorf("the bar after the clef change is in octave %d; the whole "+
			"staff is written an octave up", n.Pitch.Octave)
	}
	found := false
	if pe != nil {
		for _, s := range pe.Skipped {
			if strings.Contains(s, "changes out of the treble clef with an 8") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("nothing was said about the staff leaving the clef: %v", err)
	}
}
