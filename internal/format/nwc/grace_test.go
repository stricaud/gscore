/*
 * grace_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The bit that says a note is a grace note.
 *
 * It is 0x20 of the note's slur byte, and it was read inside the test for
 * the versions before 1.75 — so from 1.75 on no NoteWorthy file ever
 * arrived with a grace note in it. They came through as notes of their own,
 * which is music that is not there: played on the beat, taking time out of
 * the bar, and pushing everything after them late.
 *
 * The cellos lean into bar 37 of the William Tell overture with two of
 * them. The bar is three four; read as a pair of full thirty-seconds they
 * put it a sixteenth over, and the bar after that much later again.
 */

package nwc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// leaningNote is a note record, with the slur byte given: 0x20 in it is the
// grace bit, which the two small notes of the Rossini carry as 0xA1 — a
// slur onward and the grace.
func leaningNote(slur byte) []byte {
	return []byte{
		0x08, 0x00, 0x05, // type and its flags
		0x05,       // a thirty-second
		0x00,       // pad
		0x11,       // beam and stem
		0x00,       // pad
		0x00,       // articulations
		slur,       // the slur byte, and the grace bit in it
		0x03, 0x05, // position and accidental
	}
}

// The main note: a dotted half, which fills a three-four bar on its own.
var leantOn = []byte{
	0x08, 0x00, 0x00,
	0x01,       // a half
	0x00,       // pad
	0x10,       // beam and stem
	0x00,       // pad
	0x04,       // dotted
	0x00,       // no slur, no grace
	0x03, 0x95, // position and accidental
}

func graceBar(t *testing.T, version int, slur byte) *model.Part {
	t.Helper()
	b := append(append([]byte{}, leaningNote(slur)...), leaningNote(slur)...)
	b = append(b, leantOn...)
	w := &walker{b: b, version: version}
	st := &staffState{part: &model.Part{Clef: model.Bass,
		Time: model.TimeSignature{Beats: 3, BeatValue: 4}}}
	for i := 0; i < 3; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("%d: object %d: %v", version, i+1, err)
		}
	}
	st.closeMeasure(model.BarFinal)
	return st.part
}

func TestAGraceNoteIsOneFromSeventeenFiveOn(t *testing.T) {
	// 1.75 on, which is where it was never read. A 1.70 record has a
	// layout of its own and already had its grace notes.
	for _, version := range []int{175, 200, 205} {
		p := graceBar(t, version, 0xA1)
		if len(p.Measures) == 0 {
			t.Fatalf("%d: no bar", version)
		}
		els := p.Measures[0].Voices[0].Elements
		if len(els) != 1 {
			t.Errorf("%d: the bar holds %d elements, want the one note the "+
				"small ones lean into", version, len(els))
			continue
		}
		n, ok := els[0].(*model.Note)
		if !ok {
			t.Fatalf("%d: the bar holds a %T", version, els[0])
		}
		if n.Grace == nil || len(n.Grace.Notes) != 2 {
			t.Errorf("%d: the note carries %v, want two grace notes",
				version, n.Grace)
			continue
		}
		// And the bar is its meter: the small notes take no time in it.
		total := model.Duration{Num: 0, Den: 1}
		for _, e := range els {
			total = total.Add(e.Duration())
		}
		if want := (model.Duration{Num: 3, Den: 4}); total != want {
			t.Errorf("%d: the bar lasts %v, want %v", version, total, want)
		}
	}
}

// And a note without the bit is still a note of its own.
func TestANoteWithoutTheBitIsNotAGraceNote(t *testing.T) {
	p := graceBar(t, 205, 0x81) // a slur onward, and no grace
	els := p.Measures[0].Voices[0].Elements
	if len(els) != 3 {
		t.Fatalf("the bar holds %d elements, want 3", len(els))
	}
	for i, e := range els {
		if n, ok := e.(*model.Note); ok && n.Grace != nil {
			t.Errorf("element %d came out carrying grace notes", i)
		}
	}
}
