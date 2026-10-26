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

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

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
	if got := e.Error(); got != "nwc: 2 staves could not be read in full" {
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

// A staff that goes wrong partway keeps the music it read.
//
// Throwing away four hundred objects of music to report a clean failure is
// the opposite of what this reader is for: the file is somebody's piece, and
// most of it is right there. Beethoven's Albumblatt — Für Elise — gave up
// nothing at all before this and gives up thirty-six bars now, with a line
// in the log saying where it stopped and why.
func TestAPartlyReadStaffIsKept(t *testing.T) {
	e := &PartialError{Staves: 1, Total: 2, Lost: []string{
		"staff 1: object 472 of 1020: unknown object type 21, keeping the 36 bars read before that",
	}}
	if got := e.Error(); got != "nwc: one staff could not be read in full" {
		t.Errorf("it says %q", got)
	}
	// The detail has to say both halves: what went wrong, and what was kept.
	d := e.Details()[0]
	for _, want := range []string{"object 472 of 1020", "keeping the 36 bars"} {
		if !contains(d, want) {
			t.Errorf("%q does not mention %q", d, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A 1.70 note is thirteen bytes and a 1.70 rest is eleven.
//
// Which is the kind of thing only the file can tell you. Reading notes as
// eight bytes of value left two bytes of every note behind and the walk was
// two further out of step with each one — Für Elise came out as F, G sharp
// where it is E, D sharp. Reading rests as ten instead ate the two bytes
// after every rest, and the time signature arrived as 8/0 instead of 3/8.
func TestNoteAndRestSizesInSeventy(t *testing.T) {
	// The opening of Beethoven's Albumblatt: an E and a D sharp, each a
	// sixteenth, as the file stores them.
	note := func(pos byte, acc byte) []byte {
		return []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 1, pos, acc, 0, 0}
	}
	b := append(note(0xfd, 5), note(0xfe, 0)...)
	// A barline after them, which is what must be found if the sizes are
	// right.
	b = append(b, 2, 0, 0, 0, 1)

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 3; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if w.i != len(b) {
		t.Errorf("the three objects came to %d bytes, want %d", w.i, len(b))
	}

	var notes []*model.Note
	for _, m := range st.part.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					notes = append(notes, n)
				}
			}
		}
	}
	if len(notes) != 2 {
		t.Fatalf("%d notes read", len(notes))
	}
	// E, then D sharp: the position byte counts downwards in 1.70.
	if notes[0].Pitch.Step != model.StepE {
		t.Errorf("the first note is %v, want E", notes[0].Pitch.Step)
	}
	if notes[1].Pitch.Step != model.StepD || notes[1].Accidental != model.Sharp {
		t.Errorf("the second note is %v%v, want D sharp",
			notes[1].Pitch.Step, notes[1].Accidental)
	}
	if notes[0].Pitch.Octave != notes[1].Pitch.Octave {
		t.Errorf("they came out in different octaves: %d and %d",
			notes[0].Pitch.Octave, notes[1].Pitch.Octave)
	}
}

// And a 1.75 note is still eight bytes of value, which is what the Requiem
// files are read with.
func TestASeventyFiveNoteIsUnchanged(t *testing.T) {
	b := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 1, 0xfd, 5}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 175}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	if w.i != len(b) {
		t.Errorf("a 1.75 note came to %d bytes, want %d", w.i, len(b))
	}
}

// The clef a staff opens with is the staff's clef.
//
// A staff that changes clef partway — the left hand of a piano piece going
// up into the treble for a while — writes another clef object, and taking
// the last one made the whole staff treble from its first bar. Für Elise's
// left hand came out in the treble clef, an octave and a half from where it
// is written.
func TestTheOpeningClefIsTheStaffsClef(t *testing.T) {
	clef := func(n byte) []byte { return []byte{0, 0, 0, n, 0, 0, 0} }
	b := append(clef(1), clef(0)...) // bass, then treble partway through

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 2; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if st.part.Clef != model.Bass {
		t.Errorf("the staff came out in the %v clef, want the bass it opened with",
			st.part.Clef)
	}
}

// Position 0 is the middle line of the staff, which is what says the 1.70
// reading is right rather than merely fitted.
//
// The letters came out right with the wrong constant and the octaves did
// not: Für Elise opened on the correct E, an octave below where it is
// written. The middle line is the one position whose pitch is a fact about
// the clef rather than about the piece.
func TestTheMiddleLineInSeventy(t *testing.T) {
	for _, tc := range []struct {
		clef   model.Clef
		step   model.Step
		octave int
	}{
		{model.Treble, model.StepB, 4}, // B above middle C
		{model.Bass, model.StepD, 3},   // D below it
	} {
		// Position 0, as a 1.70 note record carries it.
		b := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 1, 0x00, 5, 0, 0}
		st := &staffState{part: &model.Part{Clef: tc.clef}}
		w := &walker{b: b, version: 170}
		if err := w.object(st); err != nil {
			t.Fatalf("%v: %v", tc.clef, err)
		}
		st.closeMeasure(model.BarSingle)
		var got *model.Note
		for _, m := range st.part.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok {
						got = n
					}
				}
			}
		}
		if got == nil {
			t.Fatalf("%v: no note read", tc.clef)
		}
		if got.Pitch.Step != tc.step || got.Pitch.Octave != tc.octave {
			t.Errorf("%v: position 0 is %v%d, want the middle line %v%d",
				tc.clef, got.Pitch.Step, got.Pitch.Octave, tc.step, tc.octave)
		}
	}
}

// The speed the file names, rather than the default.
//
// The value was read and thrown away, so every score opened at a hundred
// and twenty whatever it said. Für Elise is marked Poco Moto, sixty-nine to
// the quarter — which NoteWorthy shows as 138 to the eighth, the same speed
// said the other way round.
func TestTheTempoIsRead(t *testing.T) {
	// A tempo object of 69, as the file carries it.
	b := []byte{6, 0, 0, 0xf1, 0x08, 69, 0, 0, 0, 'P', 'o', 'c', 'o', 0}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	if w.tempo != 69 {
		t.Errorf("the tempo came out as %d, want 69", w.tempo)
	}

	// And a value that cannot be a speed is not taken for one.
	w2 := &walker{b: []byte{6, 0, 0, 0, 0, 0, 0, 0, 0, 0}, version: 170}
	if err := w2.object(&staffState{part: &model.Part{}}); err != nil {
		t.Fatalf("%v", err)
	}
	if w2.tempo != 0 {
		t.Errorf("a tempo of zero was taken as %d", w2.tempo)
	}
}

// A record marked 5 is four bytes long, whatever its type byte says.
//
// 1.70 writes these between the music — as a 255, as a 254, and as a 1 that
// is not a key signature. Reading that last one as the thirteen-byte key
// signature its type claims took the walk through a barline and into the
// middle of a fingering mark, and Für Elise stopped at bar seventy-one.
func TestAMarkedRecordIsFourBytes(t *testing.T) {
	for _, objType := range []byte{1, 254, 255} {
		// The record, then a barline that must still be found after it.
		b := []byte{objType, 5, 0, 0, 2, 0, 0, 0, 1}
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		w := &walker{b: b, version: 170}
		for i := 0; i < 2; i++ {
			if err := w.object(st); err != nil {
				t.Fatalf("type %d, object %d: %v", objType, i+1, err)
			}
		}
		if w.i != len(b) {
			t.Errorf("type %d: the two records came to %d bytes, want %d",
				objType, w.i, len(b))
		}
	}

	// And in 1.75 the same bytes are read as the type says, since nothing
	// there writes them.
	b := []byte{1, 5, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 0}
	w := &walker{b: b, version: 175}
	if err := w.object(&staffState{part: &model.Part{}}); err != nil {
		t.Fatalf("%v", err)
	}
	if w.i != 13 {
		t.Errorf("a 1.75 key signature came to %d bytes, want 13", w.i)
	}
}

// A 1.70 chord is a note-sized record with four bytes after it.
//
// Thirteen for the record and four for whatever counts the noteheads above
// it: left unread, the walk came out two bytes short at the next chord and
// the left hand of Für Elise was lost a third of the way through.
func TestAChordInSeventyIsSeventeenBytes(t *testing.T) {
	chord := []byte{0x0a, 0, 0, 0x04, 0, 0x21, 0, 0x02, 0, 0xfb, 0x1d, 0, 0, 0, 0, 2, 0}
	b := append(chord, 2, 0, 0, 0, 1) // a barline after it

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 2; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if w.i != len(b) {
		t.Errorf("the chord and the barline came to %d bytes, want %d", w.i, len(b))
	}
	if w.flatChords != 1 {
		t.Errorf("%d chords were counted as read flat, want one", w.flatChords)
	}
	// The note it carries is on the page, rather than the chord being
	// dropped for having no noteheads the reader could find.
	st.closeMeasure(model.BarSingle)
	notes := 0
	for _, m := range st.part.Measures {
		for _, v := range m.Voices {
			notes += len(v.Elements)
		}
	}
	if notes == 0 {
		t.Error("the chord put nothing on the page")
	}
}

// A slur is a span written on every note it covers.
//
// Not a mark on its two ends: bit one of the byte says the note is joined
// to the one after it and bit two to the one before, so the opening of Für
// Elise reads 01, 03, 03, 03, 03, 03, 03, 03, 02 — a slur beginning,
// carrying over seven notes and ending on the ninth, which is exactly the
// phrase mark printed over it. Nothing read this byte before, so no score
// opened in gscore had a slur in it at all.
func TestASlurIsASpan(t *testing.T) {
	note := func(slur byte) []byte {
		return []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, slur, 0xfd, 5, 0, 0}
	}
	// Begins, carries, ends.
	var b []byte
	for _, s := range []byte{0x01, 0x03, 0x02} {
		b = append(b, note(s)...)
	}

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 3; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	st.closeMeasure(model.BarSingle)

	var notes []*model.Note
	for _, m := range st.part.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					notes = append(notes, n)
				}
			}
		}
	}
	if len(notes) != 3 {
		t.Fatalf("%d notes read", len(notes))
	}
	if !notes[0].SlurStart || notes[0].SlurStop {
		t.Error("the first note does not begin the slur")
	}
	if notes[1].SlurStart || notes[1].SlurStop {
		t.Error("the note in the middle was taken for an end of it")
	}
	if !notes[2].SlurStop || notes[2].SlurStart {
		t.Error("the last note does not end the slur")
	}
}

// And only in 1.70, where that is what the byte means.
//
// A 1.75 score leaves it zero on all but a handful of notes and a 2.0 one
// writes 01, 80 and 81 in it. Read as a span, a 2.0 score came out with
// nearly six hundred slurs that begin and never end — drawn to the margin
// of every system after them.
func TestTheSlurByteIsNotReadInLaterVersions(t *testing.T) {
	b := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 0x01, 0xfd, 5}
	for _, version := range []int{175, 200} {
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		w := &walker{b: b, version: version}
		if err := w.object(st); err != nil {
			t.Fatalf("version %d: %v", version, err)
		}
		st.closeMeasure(model.BarSingle)
		for _, m := range st.part.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok && (n.SlurStart || n.SlurStop) {
						t.Errorf("version %d: a slur was read out of a byte that does not mean one", version)
					}
				}
			}
		}
	}
}
