/*
 * staffcount_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How many staves a binary score has, and what is not one.
 *
 * Both were found by the same file: Rossini's William Tell overture, a
 * NoteWorthy 2.5 score of twenty-three staves that would not open at all.
 *
 * It reported one thousand and twenty-three staves, because the block
 * between the layering byte and the count is a different length in every
 * file and this one put two 0xFF markers in it where the reader expected
 * one. Walked past, the count it then read was three. And reading on past
 * three — the count is a floor, not a ceiling — the object streams threw up
 * twenty-one shapes that pass for staff headers: a letter and four empty
 * strings, or a word written on the staff followed by a tab. Read as staves
 * those stood between the real ones carrying their music, and a score of
 * twenty-three came out as forty-four.
 */

package nwc

import (
	"encoding/binary"
	"testing"
)

// header is a 2.5 staff header: the five names and enough behind them to
// pass for a staff.
func header(names ...string) []byte {
	var b []byte
	for _, n := range names {
		b = append(b, []byte(n)...)
		b = append(b, 0)
	}
	return append(b, make([]byte, 160)...)
}

func TestTheStaffCountIsReadFromInFrontOfTheFirstStaff(t *testing.T) {
	staff := header("Flauto Piccolo", "Flauto Piccolo", "Picc.", "Piccolo", "Legni")

	// The shape the Rossini has: a block of words nobody has described,
	// ending in a count the walk cannot be aimed at, and then the staff.
	var b []byte
	b = append(b, 0xFF, 0xFF, 0x03, 0x00) // what the walk reads: three
	b = append(b, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0xFA, 0xFF)
	b = binary.LittleEndian.AppendUint16(b, 23) // what the staff vouches for
	b = append(b, staff...)

	w := &walker{b: b, version: 205}
	// The walk, as parseNWC1 does it.
	for {
		c, err := w.peek()
		if err != nil || (c != 0x00 && c != 0xFF) {
			break
		}
		_ = w.skip(1)
	}
	walked, err := w.u16()
	if err != nil {
		t.Fatal(err)
	}
	if walked != 3 {
		t.Fatalf("the walk read %d, so this fixture is not the one that "+
			"needed the staff to vouch for the count", walked)
	}
	at, ok := w.nextStaffHeader()
	if !ok {
		t.Fatal("the first staff was not found at all")
	}
	if got := int(binary.LittleEndian.Uint16(b[at-2:])); got != 23 {
		t.Errorf("the two bytes in front of the staff say %d, want 23", got)
	}
}

// And where there is no staff to be found, what the walk read stands: this
// is a second opinion and not a replacement.
func TestWithNoStaffToFindTheWalkedCountStands(t *testing.T) {
	b := append([]byte{0xFF, 0x00, 0x05, 0x00}, make([]byte, 64)...)
	w := &walker{b: b, version: 205}
	for {
		c, err := w.peek()
		if err != nil || (c != 0x00 && c != 0xFF) {
			break
		}
		_ = w.skip(1)
	}
	n, err := w.u16()
	if err != nil || n != 5 {
		t.Fatalf("the walk read %d, want 5", n)
	}
	if _, ok := w.nextStaffHeader(); ok {
		t.Error("a staff was found in a block of zeros")
	}
}

func TestWhatAnObjectStreamMakesThatIsNotAStaff(t *testing.T) {
	for _, c := range []struct {
		what  string
		names []string
		staff bool
	}{
		{"a staff", []string{"Corni in G", "Corni in G", "Cor.", "French Horn", "Ottoni"}, true},
		{"a staff that says only what it is and whose group it is in",
			[]string{"Piano RH1 Sounding", "", "", "Acoustic Grand Piano", "Hidden"}, true},
		// The twenty-one the Rossini's object streams make.
		{"a letter and nothing else", []string{"K", "", "", "", ""}, false},
		{"a word written on the staff, then a tab",
			[]string{"Andante", "\t", "", "", ""}, false},
		{"a name with nothing behind it at all",
			[]string{"Allegro vivace", "", "", "", ""}, false},
	} {
		t.Run(c.what, func(t *testing.T) {
			w := &walker{b: header(c.names...), version: 205}
			if got := w.atStaffHeader(); got != c.staff {
				t.Errorf("read as a staff: %v, want %v", got, c.staff)
			}
		})
	}
}

// A 1.75 staff writes its name and its group and nothing else, and a staff
// filed under nothing is a staff all the same — so the rule above is for the
// versions that have more to say.
func TestAnEarlyStaffWithNoGroupIsStillAStaff(t *testing.T) {
	w := &walker{b: header("Right Hand", ""), version: 175}
	if !w.atStaffHeader() {
		t.Error("a 1.75 staff with no group was not read as a staff")
	}
}
