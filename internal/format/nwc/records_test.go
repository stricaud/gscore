/*
 * records_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two records of a 2.5 staff, sized by the William Tell overture.
 *
 * Both were wrong, and both had hidden behind a file that could not tell
 * the right answer from the wrong one. The type 22 at the head of a staff
 * was read a byte long, which moved the clef after it and turned the key
 * signature behind that into two more clefs — and fifteen bytes of key
 * signature read as two seven-byte clefs is one byte short, so the two
 * mistakes cancelled and the walk came back into step. Every staff with a
 * key signature survived it; the horns in G, the clarinets in A, the
 * trumpets in E and the three percussion staves, which are written with no
 * signature at all, had nothing to cancel it and were lost from the fourth
 * record to the end.
 *
 * The tempo was read as a speed and two bytes of padding, on the evidence
 * of a file whose tempos carry no words — and a tempo with no words is nine
 * bytes whether the last byte is padding or an empty string, so that file
 * could not say. The piccolo here is marked "Andante": read as padding, the
 * A was eaten and the walk went on from the n.
 */

package nwc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// run walks n objects and says where the walk ended.
func run(t *testing.T, b []byte, n int) (*walker, *staffState) {
	t.Helper()
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 3, BeatValue: 4}}}
	for i := 0; i < n; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d of %d: %v", i+1, n, err)
		}
	}
	return w, st
}

// The head of the Piatti staff, which is the one place in the file where
// the length of this record is not a matter of opinion: it opens with two
// of them back to back, and that only reads as two records if each is four
// bytes long.
func TestTheRecordAtTheHeadOfAStaffIsFourBytes(t *testing.T) {
	b := []byte{
		0x16, 0x00, 0x00, 0x02, // one
		0x16, 0x00, 0x00, 0x02, // and another
		0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, // the clef
	}
	// The clef code is Piatti's own: a cymbal staff, which this reader has
	// no clef for and reads as the treble it was given. The length is what
	// is being checked.
	w, _ := run(t, b, 3)
	if w.i != len(b) {
		t.Errorf("the two records and the clef came to %d bytes, want %d",
			w.i, len(b))
	}
}

// And the clef and the key signature behind it land where they should.
//
// The bassoons: a type 22, a clef of 1, and a key signature of one sharp.
// Read a byte late the clef came out 0 — a bassoon in the treble — and the
// key was read as two more clefs, so the staff had no key at all.
func TestTheClefAndKeyBehindItAreReadRight(t *testing.T) {
	b := []byte{
		0x16, 0x00, 0x00, 0x02, // the record at the head
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, // clef 1: bass
		0x01, 0x00, 0x00, // a key signature
		0x00,                      // flats, as a bitmap
		0x00,                      // and the byte after it
		0x20,                      // sharps: one bit, one sharp
		0, 0, 0, 0, 0, 0, 0, 0, 0, // what 2.5 writes behind it
	}
	w, st := run(t, b, 3)
	if w.i != len(b) {
		t.Errorf("the three records came to %d bytes, want %d", w.i, len(b))
	}
	if st.part.Clef != model.Bass {
		t.Errorf("the staff came out in the %v clef, want bass", st.part.Clef)
	}
	if st.part.Key.Fifths != 1 {
		t.Errorf("the staff came out in %d sharps, want 1", st.part.Key.Fifths)
	}
}

func TestATempoCarriesItsWordsInEveryVersion(t *testing.T) {
	// The piccolo's: fifty-four to the quarter, marked Andante.
	marked := []byte{
		0x06, 0x00, 0x02, // type and its flags
		0xF7, 0x00, // position and placement
		0x36, 0x00, // the speed: 54
		0x02, // the beat it is counted in
		'A', 'n', 'd', 'a', 'n', 't', 'e', 0x00,
	}
	// A barline after it, which is what must be found if the size is right.
	b := append(append([]byte{}, marked...), 0x02, 0x00, 0x00, 0x00, 0x01)
	w, st := run(t, b, 2)
	if w.i != len(b) {
		t.Errorf("the tempo and the barline came to %d bytes, want %d",
			w.i, len(b))
	}
	if w.tempo != 54 {
		t.Errorf("the speed came out %d, want 54", w.tempo)
	}
	if len(st.pendingText) != 1 || st.pendingText[0] != "Andante" {
		t.Errorf("the words came out %q, want [Andante]", st.pendingText)
	}

	// And a tempo with no words is the nine bytes it always was — which is
	// why the file this was first read on could not settle it.
	none := []byte{0x06, 0x00, 0x02, 0xF7, 0x00, 0x36, 0x00, 0x02, 0x00}
	if len(none) != 9 {
		t.Fatal("a wordless tempo is nine bytes")
	}
	w, st = run(t, append(append([]byte{}, none...),
		0x02, 0x00, 0x00, 0x00, 0x01), 2)
	if w.i != len(none)+5 {
		t.Errorf("a wordless tempo and the barline came to %d bytes, want %d",
			w.i, len(none)+5)
	}
	if len(st.pendingText) != 0 {
		t.Errorf("a tempo with no words left %q behind", st.pendingText)
	}
}
