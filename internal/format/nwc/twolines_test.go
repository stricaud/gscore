/*
 * twolines_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A staff carrying two lines of music at once.
 *
 * NoteWorthy writes both into one stream, in time order, one object at a
 * time, and nothing in the bytes says which line an object belongs to. Read
 * straight through, the two come out end to end: the bar is twice its
 * meter, the second line's opening rest is heard as a gap after the first
 * line's note, and everything after it arrives a bar late.
 *
 * Reported on bar 184 of the William Tell overture — a flute holding a
 * dotted quarter while a run of sixty-fourths plays under it — as "there is
 * in the listening a silence after the dotted quarter when there should not
 * be". That silence is the run's own opening rest, played in sequence.
 *
 * The two clocks separate them, and only where one line cannot hold the
 * bar: a bar that adds up is one line whatever else is in it.
 */

package nwc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// note is a note record of this written length: 0 whole, 1 half, 2 quarter
// and so on down.
func note(dur byte) []byte {
	return []byte{
		0x08, 0x00, 0x00,
		dur,              // the written length
		0x00, 0x10, 0x00, // pad, stem, pad
		0x00,       // no articulations
		0x00,       // no slur, no grace
		0x03, 0x05, // position and accidental
	}
}

// restChord is a rest NoteWorthy kept as a chord, which is the one thing in
// the stream that says a second line is going at the same time.
func restChord(dur byte) []byte {
	return []byte{
		0x12, 0x00, 0x00,
		dur,
		0x00, 0x20, 0x00,
		0x00,
		0x00,
		0x2F, 0xF8, // what a rest carries where a note has its accidental
		0x01, 0x00, // and the two bytes behind it
	}
}

// barOf reads these records into one bar of the given meter.
func barOf(t *testing.T, meter model.TimeSignature, recs ...[]byte) *model.Measure {
	t.Helper()
	var b []byte
	for _, r := range recs {
		b = append(b, r...)
	}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble, Time: meter},
		time: meter}
	for i := range recs {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	st.closeMeasure(model.BarSingle)
	if len(st.part.Measures) != 1 {
		t.Fatalf("%d bars, want 1", len(st.part.Measures))
	}
	return st.part.Measures[0]
}

func lengths(m *model.Measure) []model.Duration {
	out := make([]model.Duration, len(m.Voices))
	for i, v := range m.Voices {
		out[i] = model.Duration{Num: 0, Den: 1}
		for _, e := range v.Elements {
			out[i] = out[i].Add(e.Duration())
		}
	}
	return out
}

func TestABarOfTwoLinesIsReadAsTwo(t *testing.T) {
	half := model.TimeSignature{Beats: 2, BeatValue: 4}
	// A quarter rest under a half note, then a quarter to finish the
	// second line: two bars of two four laid end to end in the file.
	m := barOf(t, half, restChord(2), note(1), note(2))
	if len(m.Voices) != 2 {
		t.Fatalf("the bar came out in %d voices, want 2", len(m.Voices))
	}
	want := model.Duration{Num: 1, Den: 2}
	for i, got := range lengths(m) {
		if got != want {
			t.Errorf("line %d lasts %v, want %v", i, got, want)
		}
	}
	// The long note is the line that does not open with the rest.
	if len(m.Voices[0].Elements) != 1 {
		t.Errorf("the first line holds %d elements, want the one long note",
			len(m.Voices[0].Elements))
	}
	if _, ok := m.Voices[1].Elements[0].(*model.Rest); !ok {
		t.Error("the second line does not open with its rest")
	}
}

// A bar that adds up is one line, whatever is in it.
//
// A rest kept as a chord turns up in perfectly ordinary bars, and splitting
// those would make two voices out of one: the left hand of Danse Macabre at
// bar 254 is one of these and five quavers, and together they are exactly
// the bar.
func TestABarThatAddsUpIsLeftAsOneLine(t *testing.T) {
	half := model.TimeSignature{Beats: 2, BeatValue: 4}
	m := barOf(t, half, restChord(2), note(2))
	if len(m.Voices) != 1 {
		t.Errorf("a bar that fits its meter came out in %d voices",
			len(m.Voices))
	}
	if got := lengths(m)[0]; got != (model.Duration{Num: 1, Den: 2}) {
		t.Errorf("the bar lasts %v, want 1/2", got)
	}
}

// And a bar with no second line in it is never taken apart, however long it
// runs: the two halves have to be a bar each, or what the bar is has not
// been understood and leaving it long is the lesser mistake.
func TestABarThatSplitsIntoNothingSensibleIsLeftAlone(t *testing.T) {
	half := model.TimeSignature{Beats: 2, BeatValue: 4}
	// Three whole notes: six bars of music in one, which no two lines
	// account for.
	m := barOf(t, half, note(0), note(0), note(0))
	if len(m.Voices) != 1 {
		t.Errorf("the bar came out in %d voices, want it left alone",
			len(m.Voices))
	}
}
