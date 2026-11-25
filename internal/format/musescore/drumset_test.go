/*
 * drumset_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A drum staff drawn with the file's own kit.
 *
 * MuseScore writes a drum note as the MIDI key of the drum and nothing else,
 * and keeps where each drum goes in the <Drum> entries of the instrument. Read
 * as pitches, the hi-hat and bass drum of a C Jam Blues chart came out two
 * ledger lines under the staff.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

const drumScore = `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.02">
  <Score>
    <Part>
      <Staff id="1"><StaffType group="percussion"><name>perc5Line</name></StaffType></Staff>
      <trackName>Drumset</trackName>
      <Instrument id="drumset">
        <useDrumset>1</useDrumset>
        <Drum pitch="35"><head>normal</head><line>7</line><voice>1</voice><stem>2</stem></Drum>
        <Drum pitch="42"><head>cross</head><line>-1</line><voice>0</voice><stem>1</stem></Drum>
      </Instrument>
    </Part>
    <Staff id="1"><Measure><voice>
      <Chord><durationType>half</durationType>
        <Note><pitch>42</pitch><tpc>20</tpc><head>cross</head></Note></Chord>
      <Chord><durationType>quarter</durationType>
        <Note><pitch>35</pitch><tpc>19</tpc></Note></Chord>
      <Chord><durationType>quarter</durationType>
        <Note><pitch>38</pitch><tpc>16</tpc></Note></Chord>
    </voice></Measure></Staff>
  </Score>
</museScore>`

func TestDrumsAreWrittenWhereTheFilesKitPutsThem(t *testing.T) {
	s := read(t, drumScore)
	p := s.Parts[0]
	if p.Clef != model.Percussion {
		t.Fatalf("clef %v, want percussion", p.Clef)
	}
	var notes []*model.Note
	for _, el := range p.Measures[0].Voices[0].Elements {
		if c, ok := el.(*model.Chord); ok {
			notes = append(notes, c.Notes...)
		} else if n, ok := el.(*model.Note); ok {
			notes = append(notes, n)
		}
	}
	if len(notes) != 3 {
		t.Fatalf("%d notes, want 3", len(notes))
	}
	// MuseScore's line counts half spaces down from the top line; the
	// middle line of a percussion staff is C4.
	want := []struct {
		pitch model.Pitch
		head  model.NoteheadKind
		key   int
	}{
		{model.PitchFromDiatonicIndex(5), model.HeadX, 42},       // line -1: the space above
		{model.PitchFromDiatonicIndex(-3), model.HeadNormal, 35}, // line 7: the bottom space
		// No <Drum> for the snare: gscore's own kit places it.
		{model.PitchFromDiatonicIndex(1), model.HeadNormal, 38},
	}
	for i, w := range want {
		n := notes[i]
		if n.Pitch.Step != w.pitch.Step || n.Pitch.Octave != w.pitch.Octave {
			t.Errorf("note %d at %v, want %v", i, n.Pitch, w.pitch)
		}
		if n.Head != w.head {
			t.Errorf("note %d head %v, want %v", i, n.Head, w.head)
		}
		if n.Percussion != w.key {
			t.Errorf("note %d plays key %d, want %d", i, n.Percussion, w.key)
		}
	}
}
