/*
 * nwc1_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package nwc

import "testing"

// The header of a binary NoteWorthy file, and the two places it moves.
//
// The layout between the title block and the music changed at 1.75 and again
// at 2.0, and the reader used to count its way across with a constant. A file
// from the other version landed in the middle of a font name and reported
// "expected the score marker 0xFF but found 0x54" — the T of Times New Roman.
// So the fonts are found by their shape instead.

// fontBlock is n typefaces with four bytes of style after each, which is
// what a font block looks like in every version — n and all.
func fontBlock(face string, n int) []byte {
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, []byte(face)...)
		out = append(out, 0)
		out = append(out, 1, 2, 3, 4)
	}
	return out
}

func TestTheFontsAreFoundWhateverPrecedesThem(t *testing.T) {
	// The same fonts, with a different amount of print settings in front —
	// which is exactly how 1.75 and 2.0 differ.
	for _, pad := range []int{0, 38, 39, 57, 200} {
		w := &walker{b: append(append(make([]byte, pad), fontBlock("Times New Roman", 12)...), 0xFF)}
		if err := w.findFonts(); err != nil {
			t.Errorf("%d bytes of settings in front: %v", pad, err)
			continue
		}
		if b, err := w.peek(); err != nil || b != 0xFF {
			t.Errorf("%d bytes in front: landed on 0x%02X, not the score marker", pad, b)
		}
	}
}

// It must not find a font block that is not there: a header that has drifted
// should say so rather than carry on into the music.
func TestTheFontsAreNotFoundWhereThereAreNone(t *testing.T) {
	w := &walker{b: make([]byte, 400)} // four hundred zero bytes
	if err := w.findFonts(); err == nil {
		t.Error("twelve empty strings were taken for a font block")
	}
}

// A typeface is a short printable name, which is what tells the font block
// from a run of padding.
func TestWhatLooksLikeATypeface(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"Times New Roman", true},
		{"Arial", true},
		{"", false},
		{"\x01\x02", false},
		{"Times\x00Roman", false},
	} {
		if got := looksLikeTypeface(tc.in); got != tc.want {
			t.Errorf("%q: %v", tc.in, got)
		}
	}
}

// The version is a pair, and comparing the minor number alone put 2.0 before
// 1.75 — so a 2.0 file was read with the layout of a version from before
// 1995 and lost its way in the first staff.
func TestTwoPointZeroIsNewerThanOneSevenFive(t *testing.T) {
	type v struct{ major, minor int }
	older := v{1, 75}
	newer := v{2, 0}

	if newer.minor >= older.minor {
		t.Fatal("this test is pointless if 2.0's minor number is not the smaller one")
	}
	if n, o := newer.major*100+newer.minor, older.major*100+older.minor; n <= o {
		t.Errorf("2.0 counts as %d and 1.75 as %d", n, o)
	}
}

// How many fonts a score carries is not the same in every version: 1.70
// keeps ten and 1.75 and 2.0 keep twelve. Counted as twelve, a 1.70 file
// ran off the end of its own font list and the header was lost — "the
// twelve fonts are not where any version puts them", on a file where they
// were exactly where 1.70 puts them.
func TestTheFontsAreFoundHoweverManyThereAre(t *testing.T) {
	for _, n := range []int{10, 12, 16} {
		w := &walker{b: append(append(make([]byte, 40), fontBlock("Times New Roman", n)...), 0xFF)}
		if err := w.findFonts(); err != nil {
			t.Errorf("%d fonts: %v", n, err)
			continue
		}
		if b, err := w.peek(); err != nil || b != 0xFF {
			t.Errorf("%d fonts: landed on 0x%02X", n, b)
		}
	}
	// But not so few that any pair of strings would do.
	w := &walker{b: append(fontBlock("Arial", 3), 0xFF)}
	if err := w.findFonts(); err == nil {
		t.Error("three strings were taken for a font block")
	}
}

// A staff begins with four printable names. Knowing that is what lets the
// walk find the next staff when a count has run it aground in the middle of
// the one before.
func TestRecognisingAStaffHeader(t *testing.T) {
	header := func(names ...string) []byte {
		var out []byte
		for _, n := range names {
			out = append(out, []byte(n)...)
			out = append(out, 0)
		}
		return append(out, make([]byte, 64)...)
	}

	for _, tc := range []struct {
		name string
		in   []byte
		want bool
	}{
		{"a staff", header("Vocal", "Vocal", "Clarinet", "Standard"), true},
		{"with empty fields", header("Right Hand", "Piano", "", "Standard"), true},
		{"no name", header("", "Piano", "", "Standard"), false},
		{"binary in it", header("Vo\x01cal", "x", "y", "z"), false},
		{"nothing after it", []byte("a\x00b\x00c\x00d\x00"), false},
	} {
		w := &walker{b: tc.in}
		if got := w.atStaffHeader(); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}

func TestFindingTheNextStaff(t *testing.T) {
	// A staff's worth of rubbish, then a real header.
	rubbish := make([]byte, 120)
	for i := range rubbish {
		rubbish[i] = byte(i%7) + 1
	}
	var b []byte
	b = append(b, rubbish...)
	b = append(b, []byte("Left Hand\x00Piano\x00\x00Standard\x00")...)
	b = append(b, make([]byte, 64)...)

	w := &walker{b: b}
	if !w.findStaffHeader() {
		t.Fatal("the staff after the rubbish was not found")
	}
	if got, _ := w.cstring(); got != "Left Hand" {
		t.Errorf("it found %q", got)
	}
}

// What was skipped is named. A staff dropped without a word is the one thing
// this reader must not do, and going round a broken staff made it newly
// possible to drop one and carry on as if nothing had happened.
func TestEveryLostStaffIsNamed(t *testing.T) {
	e := &PartialError{Staves: 5, Total: 9, Lost: []string{
		"staff 6: object 15 of 524: unknown object type 115",
		"staff 7: the file ends before it",
	}}
	if len(e.Details()) != 2 {
		t.Errorf("Details() gives %v", e.Details())
	}
	if got := e.Error(); got != "nwc: 2 staves could not be read" {
		t.Errorf("it says %q", got)
	}

	// With a cause as well, both are told.
	e.Cause = errBadStaff
	if n := len(e.Details()); n != 3 {
		t.Errorf("with a cause as well Details() gives %d lines, want 3", n)
	}
}

var errBadStaff = fmtError("staff 8: the file stops making sense here")

type fmtError string

func (e fmtError) Error() string { return string(e) }
