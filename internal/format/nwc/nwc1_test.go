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
	lost := []string{
		"staff 6: object 15 of 524: unknown object type 115",
		"staff 7: the file ends before it",
	}
	e := &PartialError{Staves: 5, Total: 9, Lost: lost, LostStaves: countStaffLines(lost)}
	if len(e.Details()) != 2 {
		t.Errorf("Details() gives %v", e.Details())
	}
	if got := e.Error(); got != "nwc: 2 staves could not be read in full" {
		t.Errorf("it says %q", got)
	}

	// A remark about the file is not a staff that was lost: a score with
	// both its staves on the page must not be reported as having lost two.
	e2 := &PartialError{Staves: 2, Total: 2,
		Lost:       []string{"81 chords were read as the one note they are written on"},
		LostStaves: 0}
	if got := e2.Error(); got != "nwc: some of this file was not read" {
		t.Errorf("a remark was counted as a lost staff: %q", got)
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
	lost := []string{
		"staff 1: object 472 of 1020: unknown object type 21, keeping the 36 bars read before that",
	}
	e := &PartialError{Staves: 1, Total: 2, Lost: lost, LostStaves: countStaffLines(lost)}
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

// A 1.70 chord is a note-sized record with four bytes after it, and its
// noteheads follow it as ordinary notes.
//
// Thirteen bytes for the record and four more, the third of which says how
// many noteheads it has. The count was skipped and the noteheads were read
// as notes of their own, so a chord of three became three notes in a row
// and took three times the time. Für Elise came out with bars holding two
// and three bars of music — the right hand's chords in bar 66 printed as
// four quarter notes — and since the two hands lost different amounts, they
// pulled apart: the left hand raced through its bar while the right was
// still in the bar they share.
func TestAChordInSeventyIsSeventeenBytes(t *testing.T) {
	// A chord of two, and the two notes that are its noteheads.
	chord := []byte{0x0a, 0, 0, 0x04, 0, 0x21, 0, 0x02, 0, 0xfb, 0x1d, 0, 0, 0, 0, 2, 0}
	// 0x40 in the articulation byte is what says a note is a notehead of
	// the chord before it rather than a note of its own.
	note := func(pos byte) []byte {
		return []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0x40, 0, pos, 0x1d, 0, 0}
	}
	b := append(chord, note(0xfb)...)
	b = append(b, note(0xfe)...)
	b = append(b, 2, 0, 0, 0, 1) // a barline after them

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 4; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if w.i != len(b) {
		t.Errorf("the records came to %d bytes, want %d", w.i, len(b))
	}

	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("%d things in the bar, want the one chord", len(els))
	}
	ch, ok := els[0].(*model.Chord)
	if !ok {
		t.Fatalf("the bar holds a %T, want a chord", els[0])
	}
	if len(ch.Notes) != 2 {
		t.Errorf("the chord has %d noteheads, want the two it said it had", len(ch.Notes))
	}
	// Two notes sounding together last as long as one of them, which is the
	// whole point: read as two notes in a row the bar held twice the time.
	if want := (model.Duration{Num: 1, Den: 16}); ch.Duration().Cmp(want) != 0 {
		t.Errorf("the chord lasts %v, want %v", ch.Duration(), want)
	}
	if w.flatChords != 0 {
		t.Errorf("%d chords were given up on, want none", w.flatChords)
	}
}

// A chord whose count is not a count is not followed off the page.
//
// The reader believed the file for years and had nothing to fall back on;
// a count of nought or of two hundred would have swallowed the notes after
// it. The value block the chord carries is one of its own noteheads, so
// there is always one real note to keep.
func TestAChordWithNoUsableCountKeepsItsNote(t *testing.T) {
	chord := []byte{0x0a, 0, 0, 0x04, 0, 0x21, 0, 0x02, 0, 0xfb, 0x1d, 0, 0, 0, 0, 99, 0}
	b := append(chord, 2, 0, 0, 0, 1)

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
		t.Errorf("%d chords were counted as given up on, want one", w.flatChords)
	}
	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("%d things in the bar, want the one note kept", len(els))
	}
	if _, ok := els[0].(*model.Note); !ok {
		t.Errorf("the bar holds a %T, want the note the chord carried", els[0])
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

// Bit one of that byte is the slur from 1.75 too; the rest of it is not.
//
// A 2.0 file writes 0x80 in the same byte, and read as part of a span that
// was nearly six hundred slurs beginning and never ending, drawn to the
// margin of every system after them. So only bit one is taken, and the far
// end is worked out from where the run of them stops.
func TestOnlyBitOneOfThatByteIsASlur(t *testing.T) {
	slurred := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 0x01, 0xfd, 5}
	other := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 0x80, 0xfd, 5}
	opens := func(b []byte, version int) bool {
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		w := &walker{b: b, version: version}
		if err := w.object(st); err != nil {
			t.Fatalf("version %d: %v", version, err)
		}
		st.closeMeasure(model.BarSingle)
		n := st.part.Measures[0].Voices[0].Elements[0].(*model.Note)
		return n.SlurStart
	}
	for _, version := range []int{175, 200, 205} {
		if !opens(slurred, version) {
			t.Errorf("version %d does not read the slur", version)
		}
		if opens(other, version) {
			t.Errorf("version %d read a slur out of the bit that is not one", version)
		}
	}
}

// A note's length is what the file says it is.
//
// Bit 0x20 of the beam-and-stem byte was read as a triplet flag. It is the
// stem direction, and half the notes in every file carry it, so half the
// notes in every file were played at two thirds of their length. In Für
// Elise the six sixteenths of the first bar became six twenty-fourths: the
// bar held a quarter of the time it should and the rest of it was silence,
// which is what "the playback is weird, listen to the first 3 measures"
// sounded like.
func TestTheStemByteIsNotATriplet(t *testing.T) {
	// A sixteenth with 0x21 in byte two — stem one way, first of a beam,
	// which is what Für Elise's opening E is.
	b := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 1, 0xfd, 5, 0, 0}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	n := firstNoteIn(t, st)
	if want := (model.Duration{Num: 1, Den: 16}); n.Dur.Cmp(want) != 0 {
		t.Errorf("the note is %v long, want the %v it is written as", n.Dur, want)
	}
	if n.Tuplet != nil {
		t.Error("it came out in a tuplet")
	}
}

// The augmentation dot is bit 0x04 of the articulation byte.
//
// It was read as 0x01, which is set on no note in any of the seventeen
// published scores this reader was tried against — so no dotted note
// anywhere was ever given its dot, and every bar holding one came out short.
// 0x02 was read as a double dot and is something else again: as a double dot
// it lengthened two hundred and twenty-one plain notes in Für Elise alone.
func TestTheDotIsBitFour(t *testing.T) {
	note := func(artic byte) []byte {
		return []byte{8, 0, 0, 0x03, 0, 0, 0, artic, 0, 0, 5, 0, 0}
	}
	for _, tc := range []struct {
		artic byte
		want  model.Duration
		what  string
	}{
		{0x00, model.Duration{Num: 1, Den: 8}, "a plain eighth"},
		{0x04, model.Duration{Num: 3, Den: 16}, "a dotted eighth"},
		// The two bits that used to be the dot and the double dot mean
		// something this reader has not worked out, and neither of them
		// lengthens a note.
		{0x01, model.Duration{Num: 1, Den: 8}, "0x01, which is not the dot"},
		{0x02, model.Duration{Num: 1, Den: 8}, "0x02, which is not the double dot"},
	} {
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		w := &walker{b: note(tc.artic), version: 170}
		if err := w.object(st); err != nil {
			t.Fatalf("articulation 0x%02X: %v", tc.artic, err)
		}
		n := firstNoteIn(t, st)
		if n.Dur.Cmp(tc.want) != 0 {
			t.Errorf("articulation 0x%02X is %v, want %v (%s)",
				tc.artic, n.Dur, tc.want, tc.what)
		}
	}
}

// A bar of silence lasts one bar.
//
// A staff with nothing to play writes one whole rest, whatever the meter,
// because that is how a bar rest is printed. Read as a whole note, Für
// Elise's left hand made its empty bars eight times too long — the six notes
// of the opening were followed by two and a half seconds of nothing — and
// the staves of a NoteWorthy score share their barlines, so a whole note of
// silence in a three-eight bar is not something the file can mean.
func TestABarOfSilenceLastsOneBar(t *testing.T) {
	rest := func(d model.Duration) *model.Measure {
		return &model.Measure{Voices: []*model.Voice{{
			Elements: []model.Element{&model.Rest{Dur: d}},
		}}}
	}
	whole := model.Duration{Num: 1, Den: 1}
	bar := model.Duration{Num: 3, Den: 8}
	full := &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
		&model.Rest{Dur: whole},
		&model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: whole},
	}}}}
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 3, BeatValue: 8},
		Measures: []*model.Measure{
			rest(whole),                          // the empty bar
			rest(model.Duration{Num: 1, Den: 8}), // a rest that fits, left alone
			full,                                 // not alone in its bar, left alone
		},
	}
	score := &model.Score{Parts: []*model.Part{p}}

	fitBarRests(score)

	if got := p.Measures[0].Voices[0].Elements[0].Duration(); got.Cmp(bar) != 0 {
		t.Errorf("the empty bar is %v long, want the %v its meter says", got, bar)
	}
	if got := p.Measures[1].Voices[0].Elements[0].Duration(); got.Cmp(model.Duration{Num: 1, Den: 8}) != 0 {
		t.Errorf("a rest that fits its bar was changed to %v", got)
	}
	if got := p.Measures[2].Voices[0].Elements[0].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("a rest with a note beside it was changed to %v", got)
	}
}

// firstNoteIn is the first note a staff read. The bar it is in is still open
// — a bar is closed by the barline object that ends it — so it is closed
// here before the measures are looked through.
func firstNoteIn(t *testing.T, st *staffState) *model.Note {
	t.Helper()
	st.closeMeasure(model.BarSingle)
	for _, m := range st.part.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					return n
				}
			}
		}
	}
	t.Fatal("no note was read")
	return nil
}

// A chord takes only the notes that say they are its noteheads.
//
// The count is read from the file and could be read wrongly; without this a
// chord of two that said three would have eaten the note after it, and the
// bar would come out short with a melody note missing from the middle of it
// — invisible on the page until it is played.
func TestAChordDoesNotSwallowTheNoteAfterIt(t *testing.T) {
	// A chord claiming three noteheads, with only two that say so.
	chord := []byte{0x0a, 0, 0, 0x04, 0, 0x21, 0, 0x02, 0, 0xfb, 0x1d, 0, 0, 0, 0, 3, 0}
	head := func(pos byte) []byte {
		return []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0x40, 0, pos, 0x1d, 0, 0}
	}
	plain := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0, 0, 0xfe, 0x1d, 0, 0}
	b := append(chord, head(0xfb)...)
	b = append(b, head(0xf8)...)
	b = append(b, plain...)
	b = append(b, 2, 0, 0, 0, 1)

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 5; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("%d things in the bar, want the chord and the note after it", len(els))
	}
	ch, ok := els[0].(*model.Chord)
	if !ok {
		t.Fatalf("the first is a %T, want the chord", els[0])
	}
	if len(ch.Notes) != 2 {
		t.Errorf("the chord took %d noteheads, want the two that said they were its", len(ch.Notes))
	}
	if _, ok := els[1].(*model.Note); !ok {
		t.Errorf("the second is a %T, want the note the chord did not own", els[1])
	}
}

// The staccato is bit 0x02 of the articulation byte.
//
// It was read as 0x20. NoteWorthy prints a dot under every note of Für
// Elise's left hand through the middle of the piece and every one of them
// carries 0x02; 0x20 is on twenty notes in the whole piece.
func TestTheStaccatoIsBitOne(t *testing.T) {
	note := func(artic byte) []byte {
		return []byte{8, 0, 0, 0x04, 0, 0x21, 0, artic, 0, 0, 5, 0, 0}
	}
	for _, tc := range []struct {
		artic byte
		want  bool
	}{
		{0x02, true},
		{0x00, false},
		{0x20, false},
	} {
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		w := &walker{b: note(tc.artic), version: 170}
		if err := w.object(st); err != nil {
			t.Fatalf("articulation 0x%02X: %v", tc.artic, err)
		}
		n := firstNoteIn(t, st)
		got := false
		for _, a := range n.Articulations {
			if a == model.Staccato {
				got = true
			}
		}
		if got != tc.want {
			t.Errorf("articulation 0x%02X: staccato %v, want %v", tc.artic, got, tc.want)
		}
	}
}

// A small note takes none of the bar's time.
//
// Bit 0x20 of the slur byte says the note is a grace note. Für Elise has
// three — the two it leans into bar 25 with and the one before the run in
// bar 28 — and read as notes of their own they were an eighth of music that
// is not in the bar, played on the beat and pushing everything after them
// late. Those two bars were the last in the piece whose notes did not add up
// to their meter, each by exactly the grace notes in it.
func TestAGraceNoteTakesNoTime(t *testing.T) {
	note := func(dur, slur, pos byte) []byte {
		return []byte{8, 0, 0, dur, 0, 0x21, 0, 0, slur, pos, 5, 0, 0}
	}
	// Two small sixteenths, then the quarter they lean into.
	b := append(note(0x04, 0x20, 0x03), note(0x04, 0x20, 0x01)...)
	b = append(b, note(0x02, 0x00, 0xff)...)
	b = append(b, 2, 0, 0, 0, 1)

	st := &staffState{part: &model.Part{
		Clef: model.Treble, Time: model.TimeSignature{Beats: 1, BeatValue: 4},
	}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 4; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}

	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("%d things in the bar, want the one note the small ones belong to", len(els))
	}
	n, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the bar holds a %T, want a note", els[0])
	}
	if n.Grace == nil || len(n.Grace.Notes) != 2 {
		t.Fatalf("the note carries %v grace notes, want two", n.Grace)
	}
	// The bar is a quarter long, not three sixteenths more than that.
	if got, want := st.part.Measures[0].Voices[0].Duration(), (model.Duration{Num: 1, Den: 4}); got.Cmp(want) != 0 {
		t.Errorf("the bar holds %v, want the %v its meter says", got, want)
	}
}

// The accent is bit 0x20 of the articulation byte.
//
// NoteWorthy prints a ">" over the dotted note in bar 25 of Für Elise and
// over its twin in bar 26, and those are the notes carrying 0x24 — the
// accent together with the dot.
func TestTheAccentIsBitFive(t *testing.T) {
	b := []byte{8, 0, 0, 0x04, 0, 0x21, 0, 0x24, 0, 0, 5, 0, 0}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	n := firstNoteIn(t, st)
	accent := false
	for _, a := range n.Articulations {
		if a == model.Accent {
			accent = true
		}
	}
	if !accent {
		t.Errorf("the note came out with %v, want an accent", n.Articulations)
	}
	// 0x24 is the accent with the dot, so the note is lengthened as well.
	if want := (model.Duration{Num: 3, Den: 32}); n.Dur.Cmp(want) != 0 {
		t.Errorf("the note is %v long, want the dotted %v", n.Dur, want)
	}
}

// The volta brackets are read, one bar at a time.
//
// NoteWorthy writes the ending on every bar a bracket covers rather than
// once at its head, so a bar carrying the same number as the one before it
// goes on with that bracket and every other bar closes its own. Closing them
// at the next structural barline instead ran Für Elise's second bracket from
// bar 24 to the end of the piece.
func TestTheVoltaBracketsAreRead(t *testing.T) {
	ending := func(n byte) []byte { return []byte{3, 0, 0, n, 0} }
	note := []byte{8, 0, 0, 0x02, 0, 0, 0, 0, 0, 0, 5, 0, 0}
	// Bar one is ending one and closes on a repeat; bar two is ending two
	// and the music goes on from it.
	b := append(ending(1), note...)
	b = append(b, 2, 0, 0, 7, 2) // repeat end
	b = append(b, ending(2)...)
	b = append(b, note...)
	b = append(b, 2, 0, 0, 0, 1) // a plain barline

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 170}
	for i := 0; i < 6; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if len(st.part.Measures) != 2 {
		t.Fatalf("%d bars, want two", len(st.part.Measures))
	}
	first, second := st.part.Measures[0].Ending, st.part.Measures[1].Ending
	if first == nil || second == nil {
		t.Fatalf("the brackets are %v and %v, want both", first, second)
	}
	if len(first.Numbers) != 1 || first.Numbers[0] != 1 || !first.Start || !first.Stop {
		t.Errorf("the first bracket is %+v, want a closed 1.", first)
	}
	if first.Open {
		t.Error("the first bracket has no hook, and it ends on the repeat")
	}
	if len(second.Numbers) != 1 || second.Numbers[0] != 2 || !second.Start || !second.Stop {
		t.Errorf("the second bracket is %+v, want a closed 2.", second)
	}
	if !second.Open {
		t.Error("the second bracket has a hook, and there is nothing to go back for")
	}
}

/*
 * NoteWorthy 2.5
 *
 * A version this reader had not met. Its header is 2.0's with things added
 * to it, and each of them moved everything after it: the file would not open
 * at all, reporting one thousand two hundred and eighty staves, which is
 * what a five-staff score looks like when the count is read one byte early.
 */

// The staff count is found rather than counted to.
//
// 1.75 goes straight from the layering byte to the count, 2.0 puts a second
// 0xFF marker in between, and 2.5 adds a zero byte after that. A count's low
// byte is never zero — a score has at least one staff — so a zero there is
// padding whatever version wrote it.
func TestTheStaffCountIsFoundPastWhateverPadsIt(t *testing.T) {
	for _, tc := range []struct {
		what  string
		bytes []byte
	}{
		{"1.75, straight to the count", []byte{0x05, 0x00}},
		{"2.0, a second marker", []byte{0xFF, 0x05, 0x00}},
		{"2.5, a marker and a zero", []byte{0xFF, 0x00, 0x05, 0x00}},
		{"and however many zeros a later one writes", []byte{0xFF, 0x00, 0x00, 0x05, 0x00}},
	} {
		w := &walker{b: append(tc.bytes, make([]byte, 64)...)}
		if b, err := w.peek(); err == nil && b == 0xFF {
			_ = w.skip(1)
		}
		for {
			b, err := w.peek()
			if err != nil || b != 0x00 {
				break
			}
			_ = w.skip(1)
		}
		n, err := w.u16()
		if err != nil || n != 5 {
			t.Errorf("%s: %d staves, want 5", tc.what, n)
		}
	}
}

// A 2.5 staff header carries one string more than a 2.0 one: the layer it
// belongs to.
//
// "Hidden" and "Display" are how one instrument is written twice, a staff
// that sounds and a staff that is printed. Counted as four strings the walk
// ended a string early, and the score came out as staves called "Hidden"
// with no music in them.
func TestATwentyFiveStaffHeaderHasFiveNames(t *testing.T) {
	header := func(names ...string) []byte {
		var out []byte
		for _, n := range names {
			out = append(out, []byte(n)...)
			out = append(out, 0)
		}
		return append(out, make([]byte, 128)...)
	}

	w := &walker{b: header("Piano RH1 Sounding", "", "", "Acoustic Grand Piano", "Hidden"),
		version: 205}
	p, err := w.staff(205)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if p.Name != "Piano RH1 Sounding" {
		t.Errorf("the staff is called %q", p.Name)
	}
	// The layer's name was read as part of the header, so the walk is past
	// all five: a 2.0 reading would have stopped before "Hidden" and left
	// it to be taken for the next staff.
	if got := w.i; got <= len("Piano RH1 Sounding\x00\x00\x00Acoustic Grand Piano\x00Hidden") {
		t.Errorf("the walk stopped at %d, before the end of the five names", got)
	}
}

// A 2.5 instrument patch is nineteen bytes of payload, not eight.
//
// The last eight of them are the ladder of velocities a dynamic plays at.
// Read as eight, the walk came out eleven bytes short at the first patch of
// the piece and read the next object from the middle of this one: "unknown
// object type 56", which is no type at all but the 0x38 inside a patch.
func TestATwentyFivePatchIsLonger(t *testing.T) {
	// Copied from the ottava mark over bar 76 of Danse Macabre: eight
	// bytes, the words the mark draws, one byte, and the ladder of
	// velocities a dynamic plays at.
	patch := []byte{
		4, 0, 1, // type and its flags
		0xF2, 0x01, 0x01, 0, 0, 0, 0x01, 0x00, // eight
		'8', 0x00, // the words
		0x0C,
		0x19, 0x23, 0x2D, 0x37, 0x44, 0x50, 0x5C, 0x6C, // the ladder
	}
	// A barline after it, which is what must be found if the size is right.
	b := append(patch, 2, 0, 0, 0, 1)

	st := &staffState{part: &model.Part{Clef: model.Treble}}
	w := &walker{b: b, version: 205}
	for i := 0; i < 2; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if w.i != len(b) {
		t.Errorf("the patch and the barline came to %d bytes, want %d", w.i, len(b))
	}

	// And a 2.0 patch is still the eight it always was.
	short := []byte{4, 0, 0, 0, 8, 0, 0, 4, 0, 0, 0}
	short = append(short, 2, 0, 0, 0, 1)
	w = &walker{b: short, version: 200}
	st = &staffState{part: &model.Part{Clef: model.Treble}}
	for i := 0; i < 2; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("2.0 object %d: %v", i+1, err)
		}
	}
	if w.i != len(short) {
		t.Errorf("a 2.0 patch and barline came to %d bytes, want %d", w.i, len(short))
	}
}

// A bit in an object's third byte makes its record three bytes longer.
//
// Barlines come in two sizes in a 2.5 file and the bit is what tells them
// apart. Read at the plain size, the walk came off the rails a third of the
// way down the first staff of Danse Macabre and read the music after it as a
// run of clefs.
func TestTheWideBitMakesARecordLonger(t *testing.T) {
	// Two barlines and a clef, the first barline plain and the second wide.
	b := []byte{
		2, 0, 0x08, 0, 0,
		2, 0, 0x20, 0, 0, 1, 0xFE, 5,
		0, 0, 0, 1, 0, 0, 0,
	}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for i := 0; i < 3; i++ {
		if err := w.object(st); err != nil {
			t.Fatalf("object %d: %v", i+1, err)
		}
	}
	if w.i != len(b) {
		t.Errorf("the three records came to %d bytes, want %d", w.i, len(b))
	}

	// And the same bit means nothing to the versions before 2.5, whose
	// barlines are five bytes whatever is in that byte.
	w = &walker{b: b, version: 200}
	st = &staffState{part: &model.Part{Clef: model.Treble}}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	if w.i != 5 {
		t.Errorf("a 2.0 barline came to %d bytes, want 5", w.i)
	}
}

// From 2.0 only the bottom half of the duration byte says how long a note is.
//
// Danse Macabre writes 0x11 for a half note. Read whole, that is no duration
// this reader knows and the staff stopped there — two thirds of the way down
// the first one, with four hundred bars still to come.
func TestTheDurationByteCarriesFlagsOfItsOwn(t *testing.T) {
	// A note: three bytes of header, then the value block.
	note := func(dur byte) []byte {
		return []byte{8, 0, 0, dur, 0, 0x10, 0, 0, 0, 5, 5}
	}
	w := &walker{b: note(0x11), version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	st.closeMeasure(model.BarFinal)
	half := model.Duration{Num: 1, Den: 2}
	got := st.part.Measures
	if len(got) == 0 || len(got[0].Voices) == 0 || len(got[0].Voices[0].Elements) == 0 {
		t.Fatalf("no note came out")
	}
	n, ok := got[0].Voices[0].Elements[0].(*model.Note)
	if !ok {
		t.Fatalf("the element is a %T", got[0].Voices[0].Elements[0])
	}
	if n.Dur.Cmp(half) != 0 {
		t.Errorf("0x11 reads as %v, want the %v its bottom four bits say", n.Dur, half)
	}
}

// A staff is named for somebody to read, so its name starts with something
// printable.
//
// Four printable strings in a row is how a staff header is recognised, and an
// object stream makes that shape by accident here and there. Every accident
// in Danse Macabre is called "\t", and read as staves they added three to an
// eight-staff score — one of them carrying three hundred and ninety-three
// bars of the staff it was found inside.
func TestAStaffCalledNothingIsNotAStaff(t *testing.T) {
	header := func(name string) []byte {
		b := append([]byte(name), 0)
		for i := 0; i < 3; i++ {
			b = append(b, 0)
		}
		return append(b, make([]byte, 160)...)
	}
	if (&walker{b: header("\t")}).atStaffHeader() {
		t.Error("a staff called \\t was taken for a real one")
	}
	if !(&walker{b: header("Piano LH2")}).atStaffHeader() {
		t.Error("a staff with an ordinary name was not recognised")
	}
	// A name that opens with a digit is ordinary.
	if !(&walker{b: header("1st violins")}).atStaffHeader() {
		t.Error("a staff called 1st violins was not recognised")
	}
}

// The search for a staff stops at the first place that makes the header
// shape, and a stray byte or two in front of the name makes that shape too.
//
// Danse Macabre's fifth staff was found two bytes early and came back called
// "0øPiano RH1 Display". The name is only the half of it: everything in a
// staff is measured from where its header begins, so two bytes of rubbish in
// front cost that staff and the one after it as well.
func TestAStaffNameIsNotTakenTwoBytesEarly(t *testing.T) {
	var b []byte
	b = append(b, '0', 0xF8)
	b = append(b, []byte("Piano RH1 Display")...)
	b = append(b, 0, 0, 0, 0)
	b = append(b, make([]byte, 160)...)

	w := &walker{b: b, version: 205}
	if !w.findStaffHeader() {
		t.Fatal("no staff header found at all")
	}
	if w.i != 2 {
		t.Errorf("the staff was found at %d, want 2 — where its name starts", w.i)
	}
	// And a name that is all printable to begin with is not trimmed.
	w = &walker{b: append([]byte("1st violins\x00\x00\x00\x00"), make([]byte, 160)...)}
	if !w.findStaffHeader() || w.i != 0 {
		t.Errorf("1st violins was found at %d, want 0", w.i)
	}
}

// A 2.5 staff says which of the two layers it belongs to, and the hidden one
// is not printed.
//
// NoteWorthy writes an instrument twice over — a staff to be played and a
// staff to be read — and prints only the second. Danse Macabre is eight
// staves that way and two on the page; drawn all together they came out as
// four grand staves of the same music stacked up the page.
func TestTheHiddenLayerIsNotPrinted(t *testing.T) {
	header := func(names ...string) []byte {
		var out []byte
		for _, n := range names {
			out = append(out, []byte(n)...)
			out = append(out, 0)
		}
		return append(out, make([]byte, 128)...)
	}
	for _, c := range []struct {
		layer string
		hide  bool
	}{{"Hidden", true}, {"Display", false}} {
		w := &walker{b: header("Piano RH1", "", "", "", c.layer), version: 205}
		p, err := w.staff(205)
		if err != nil {
			t.Fatalf("%s: %v", c.layer, err)
		}
		if p.Hidden != c.hide {
			t.Errorf("a %s staff comes back hidden=%v, want %v", c.layer, p.Hidden, c.hide)
		}
	}
}

// A staff goes on until the next one begins, whatever object count it gives.
//
// The number is not always the number it holds: three of the four printed
// staves of Danse Macabre stopped five, seven and thirteen bars before the
// end of the piece — the last chords of a Liszt transcription, gone with
// nothing said about them — and Für Elise lost its last nine bars, which is
// the whole of its ending.
func TestAStaffGoesOnPastTheCountItGives(t *testing.T) {
	// Five names, the settings, no lyrics, and then the count.
	var b []byte
	for _, n := range []string{"Right hand", "", "", "", "Display"} {
		b = append(b, []byte(n)...)
		b = append(b, 0)
	}
	b = append(b, make([]byte, 41+2)...) // settings, no lyric lines
	b = append(b, 3, 0)                  // "one object", counted the 1.7x way

	// Three notes and a barline each, where the staff admits to one.
	for i := 0; i < 3; i++ {
		b = append(b, 8, 0, 0, 2, 0, 0x10, 0, 0, 0, byte(i), 5)
		b = append(b, 2, 0, 0, 0, 1)
	}
	// And then the next staff, which is where this one has to stop.
	for _, n := range []string{"Left hand", "", "", "", "Display"} {
		b = append(b, []byte(n)...)
		b = append(b, 0)
	}
	b = append(b, make([]byte, 160)...)

	w := &walker{b: b, version: 205}
	p, err := w.staff(205)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got := len(p.Measures); got != 3 {
		t.Errorf("the staff came back with %d bars, want the 3 that are written", got)
	}
	if !w.atStaffHeader() {
		t.Error("the walk did not stop on the next staff")
	}
}

// The first clef, key and meter are the staff's own; the rest are changes
// written where they happen.
//
// Every one of them used to be written onto the part, so the last of each in
// the piece became the one its first bar was read under. Danse Macabre opens
// in two-four and turns into a waltz partway down, and it opened as a waltz:
// three-four over bars holding a half note each, which is not a bar of music.
func TestASignatureChangeIsWrittenWhereItHappens(t *testing.T) {
	// Two-four, a bar, then three-four.
	b := []byte{
		5, 0, 0, 2, 0, 2, 0, 0, 0, // 2/4
		8, 0, 0, 1, 0, 0x10, 0, 0, 0, 0, 5, // a note, so the bar is a bar
		2, 0, 0, 0, 1, // barline
		5, 0, 0, 3, 0, 2, 0, 0, 0, // 3/4
		8, 0, 0, 1, 0, 0x10, 0, 0, 0, 0, 5,
		2, 0, 0, 0, 1, // barline
	}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	two := model.TimeSignature{Beats: 2, BeatValue: 4, Symbol: model.TimeNumeric}
	three := model.TimeSignature{Beats: 3, BeatValue: 4, Symbol: model.TimeNumeric}
	if st.part.Time != two {
		t.Errorf("the staff opens in %v, want the %v it is written in", st.part.Time, two)
	}
	if len(st.part.Measures) < 2 {
		t.Fatalf("%d bars", len(st.part.Measures))
	}
	if got := st.part.Measures[1].Time; got == nil || *got != three {
		t.Errorf("the second bar changes to %v, want %v", got, three)
	}
}

// A note is read against the clef in force where it is written, not against
// the one its staff opened with.
//
// NoteWorthy stores where on the staff a note sits; what note that is depends
// on the sign at the head of the line. The left hand of Danse Macabre goes up
// into the treble at bar five and stays there for fifty bars, and read in the
// bass throughout it came out an octave and a half below where it belongs —
// and was drawn there, piled under the staff on ledger lines.
func TestANoteIsReadAgainstTheClefInForce(t *testing.T) {
	note := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 0, 5}
	pitchUnder := func(clefCode byte) model.Pitch {
		b := []byte{0, 0, 0, clefCode, 0, 0, 0}
		b = append(b, note...)
		w := &walker{b: b, version: 205}
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		for w.remaining() > 0 {
			if err := w.object(st); err != nil {
				t.Fatalf("%v", err)
			}
		}
		st.closeMeasure(model.BarFinal)
		n := st.part.Measures[0].Voices[0].Elements[0].(*model.Note)
		return n.Pitch
	}
	// The same staff position under the two clefs is two different notes,
	// a sixth apart.
	treble, bass := pitchUnder(0), pitchUnder(1)
	if treble == bass {
		t.Fatalf("the same position reads as %v%d under both clefs", treble.Step, treble.Octave)
	}
	if treble.MIDI() <= bass.MIDI() {
		t.Errorf("under the treble the note is %v%d and under the bass %v%d, which is the wrong way round",
			treble.Step, treble.Octave, bass.Step, bass.Octave)
	}
}

// Two staves marked "layer with next" are one staff on the page.
//
// NoteWorthy writes a pianist's hand as two staves when it has two
// independent lines, and prints them as one. Read one part per staff, Danse
// Macabre came out as four grand staves of the same music stacked up the
// page where NoteWorthy shows two. And a bar one layer writes exactly as the
// other already has is printed once: the sustained note under a run is in
// both layers, and added twice it was two half notes side by side in every
// one of the first four bars.
func TestLayeredStavesBecomeOneStaff(t *testing.T) {
	bar := func(els ...model.Element) *model.Measure {
		return &model.Measure{Voices: []*model.Voice{{Elements: els}}}
	}
	half := model.Duration{Num: 1, Den: 2}
	g5 := model.Pitch{Step: model.StepG, Octave: 5}
	c5 := model.Pitch{Step: model.StepC, Octave: 5}

	upper := &model.Part{Name: "RH1", Measures: []*model.Measure{
		bar(&model.Note{Pitch: g5, Dur: half}),
		bar(&model.Note{Pitch: c5, Dur: half}),
	}}
	lower := &model.Part{Name: "RH2", Measures: []*model.Measure{
		bar(&model.Note{Pitch: g5, Dur: half}), // the same bar, printed once
		bar(&model.Note{Pitch: g5, Dur: half}), // its own line, printed too
	}}
	alone := &model.Part{Name: "LH", Measures: []*model.Measure{bar(), bar()}}

	out, names := mergeLayers([]*model.Part{upper, lower, alone},
		[]bool{true, false, false}, []string{"Piano", "", ""})
	if len(out) != 2 {
		t.Fatalf("%d staves came back, want 2", len(out))
	}
	if out[0].Name != "RH1" || out[1].Name != "LH" {
		t.Errorf("the staves are %q and %q", out[0].Name, out[1].Name)
	}
	if got := len(out[0].Measures[0].Voices); got != 1 {
		t.Errorf("the unison bar has %d voices, want the 1 it is printed with", got)
	}
	if got := len(out[0].Measures[1].Voices); got != 2 {
		t.Errorf("the bar of two lines has %d voices, want 2", got)
	}

	// A staff that shares with the next is not merged across the line
	// between what sounds and what is printed.
	hidden := &model.Part{Name: "hidden", Hidden: true, Measures: []*model.Measure{bar(), bar()}}
	// The group name survives the merge, so the brace still knows what it
	// is round.
	if len(names) != 2 || names[0] != "Piano" {
		t.Errorf("the merged staves are filed under %v, want the Piano the first carried", names)
	}

	out, _ = mergeLayers([]*model.Part{hidden, alone}, []bool{true, false}, []string{"", ""})
	if len(out) != 2 {
		t.Errorf("a hidden staff was merged with a printed one")
	}
}

// 2.5 counts a note's staff position downwards from the middle line, as 1.70
// does and as 1.75 and 2.0 do not.
//
// Measured against NoteWorthy's own page. The opening of Danse Macabre is a
// half note under the treble staff, a right hand alternating A4 and D5 above
// it, and a left hand of F sharp 4 and B4 under a treble clef. Counted
// upwards those notes come out G5, C5 and G4, E5 and B4 — every one of them
// the mirror of what is printed, reflected in the middle line.
func TestTwentyFiveCountsTheOtherWay(t *testing.T) {
	// Position 5: five steps from the middle line, whichever way that is.
	note := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 5, 5}
	read := func(version int) model.Pitch {
		w := &walker{b: note, version: version}
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		if err := w.object(st); err != nil {
			t.Fatalf("%d: %v", version, err)
		}
		st.closeMeasure(model.BarFinal)
		return st.part.Measures[0].Voices[0].Elements[0].(*model.Note).Pitch
	}
	// Five above the middle line of a treble staff is G5; five below is D4.
	if got := read(200); got.Step != model.StepG || got.Octave != 5 {
		t.Errorf("2.0 reads position 5 as %v%d, want G5", got.Step, got.Octave)
	}
	if got := read(205); got.Step != model.StepD || got.Octave != 4 {
		t.Errorf("2.5 reads position 5 as %v%d, want D4", got.Step, got.Octave)
	}
}

// An accidental is written once and holds for the rest of the bar.
//
// A rule of notation rather than of this file format: NoteWorthy draws the
// sharp on the first F of the bar and on none of the others, and means all of
// them. Read note by note, the left hand of Danse Macabre played F sharp, F,
// F, F where the page says F sharp four times over.
func TestAnAccidentalLastsToTheBarline(t *testing.T) {
	// The accidental is the bottom three bits of the last byte: nought is
	// a sharp and five is nothing written at all.
	sharp := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 1, 0}
	plain := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 1, 5}
	var b []byte
	b = append(b, sharp...)
	b = append(b, plain...)
	b = append(b, 2, 0, 0, 0, 1) // the barline
	b = append(b, plain...)

	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)

	alters := []int{}
	for _, m := range st.part.Measures {
		for _, v := range m.Voices {
			for _, e := range v.Elements {
				if n, ok := e.(*model.Note); ok {
					alters = append(alters, n.Pitch.Alter)
				}
			}
		}
	}
	if len(alters) != 3 {
		t.Fatalf("%d notes came out, want 3", len(alters))
	}
	if alters[0] != 1 || alters[1] != 1 {
		t.Errorf("the bar reads %v, want the sharp to hold over the second note", alters[:2])
	}
	if alters[2] != 0 {
		t.Errorf("the note after the barline is altered by %d, want none of it", alters[2])
	}
}

// Staves filed under one name are braced together and labelled with it.
//
// A staff with no name of its own belongs to the group above it, which is how
// NoteWorthy writes a grand staff: "Piano" on the right hand and nothing on
// the left, because the word labels the pair. Read as one group per staff,
// Danse Macabre had no brace at all and its two staves stood apart on the
// page as though they were two instruments.
func TestAGrandStaffIsBracedAndNamed(t *testing.T) {
	parts := []*model.Part{{Name: "RH"}, {Name: "LH"}}
	got := staffGroups(parts, []string{"Piano", ""})
	if len(got) != 1 {
		t.Fatalf("%d groups, want 1: %v", len(got), got)
	}
	g := got[0]
	if g.First != 0 || g.Last != 1 {
		t.Errorf("the group runs %d..%d, want 0..1", g.First, g.Last)
	}
	if g.Symbol != model.GroupBrace || !g.Barlines {
		t.Errorf("two staves of one instrument are drawn with %v, want a brace with its barlines joined", g.Symbol)
	}
	if g.Name != "Piano" {
		t.Errorf("the group is called %q", g.Name)
	}

	// Three or more under one name are a section, so a bracket.
	got = staffGroups([]*model.Part{{}, {}, {}}, []string{"Strings", "", ""})
	if len(got) != 1 || got[0].Symbol != model.GroupBracket {
		t.Errorf("three staves are drawn with %v, want a bracket", got)
	}

	// The name every NoteWorthy file carries when nobody has filed the
	// staff under anything is not a group, and nothing is drawn round it.
	if got := staffGroups([]*model.Part{{}, {}}, []string{"Standard", ""}); len(got) != 0 {
		t.Errorf("the default group was drawn: %v", got)
	}

	// A staff that is not on the page does not hold a group open across
	// the ones that are.
	got = staffGroups([]*model.Part{{Name: "a"}, {Hidden: true}, {Name: "b"}},
		[]string{"Piano", "", ""})
	if len(got) != 0 {
		t.Errorf("a group reached across a hidden staff: %v", got)
	}
}

// A clef change written partway through a bar is drawn partway through the
// bar.
//
// Hoisted to the head of the bar instead, it is read by the notes in front of
// it as well, and they are then drawn against a clef they were never written
// under. Danse Macabre's left hand dives into the bass and back inside a bar
// at 118 and 119, and the note before each sign came out six ledger lines
// below the staff.
func TestAClefChangeStaysWhereItIsWritten(t *testing.T) {
	note := func(pos byte) []byte {
		return []byte{8, 0, 0, 2, 0, 0x10, 0, 0, pos, 0, 5}
	}
	clef := func(c byte) []byte { return []byte{0, 0, 0, c, 0, 0, 0} }

	var b []byte
	b = append(b, clef(0)...) // the staff opens in the treble
	b = append(b, note(0)...)
	b = append(b, clef(1)...) // and drops into the bass mid-bar
	b = append(b, note(0)...)
	b = append(b, 2, 0, 0, 0, 1)

	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)

	m := st.part.Measures[0]
	if m.Clef != nil {
		t.Errorf("the bar changes clef at its head (%v); the change is written inside it", *m.Clef)
	}
	els := m.Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("%d elements in the bar", len(els))
	}
	if mk := marksOf(els[0]); mk == nil || mk.Clef != nil {
		t.Errorf("the first note carries a clef change; the change comes after it")
	}
	if mk := marksOf(els[1]); mk == nil || mk.Clef == nil || *mk.Clef != model.Bass {
		t.Errorf("the second note does not carry the change into the bass")
	}
	// And the notes keep the pitches they were written at: the same
	// position under two clefs is two different notes.
	a := els[0].(*model.Note).Pitch
	c := els[1].(*model.Note).Pitch
	if a.MIDI() == c.MIDI() {
		t.Errorf("both notes came out as %v%d, so the change was not applied", a.Step, a.Octave)
	}
}

// The file says which notes are beamed together, and the bottom two bits of
// the stem byte are where it says it.
//
// NoteWorthy spells its groups out note by note rather than leaving them to
// be worked out from the meter, which is the only way a group like the three
// quavers in bar 119 of Danse Macabre can be written down at all: nothing
// about 2/4 says to beam three of them together. Left to the engraver, the
// six semiquavers of Für Elise's second bar came out as three beamed pairs
// where the file — and every edition — has one group of six.
func TestTheFileSaysWhatIsBeamed(t *testing.T) {
	note := func(beam byte) []byte {
		return []byte{8, 0, 0, 3, 0, 0x20 | beam, 0, 0, 0, 0, 5}
	}
	var b []byte
	for _, beam := range []byte{1, 2, 3, 0} {
		b = append(b, note(beam)...)
	}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)

	want := []model.Beam{model.BeamStart, model.BeamContinue, model.BeamStop, model.BeamNone}
	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != len(want) {
		t.Fatalf("%d notes came out", len(els))
	}
	for i, w := range want {
		if got := marksOf(els[i]).Beam; got != w {
			t.Errorf("note %d is beamed %v, want %v", i+1, got, w)
		}
	}
}

// A fermata is a Tempo Variance, written just before the note it sits over.
//
// NoteWorthy has no fermata of its own: the pause over the last chord of a
// phrase is a Tempo Variance whose style says Fermata. Bar 30 of Danse
// Macabre is a whole rest under one in both hands, and both came out bare.
func TestAFermataIsATempoVariance(t *testing.T) {
	// The style is the third byte of the four that follow the header.
	b := []byte{14, 0, 0, 0xF9, 0x1A, nwcFermata, 0}
	b = append(b, 9, 0, 0, 0, 0, 0x20, 0, 0, 0, 0, 0) // the whole rest
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)
	r, ok := st.part.Measures[0].Voices[0].Elements[0].(*model.Rest)
	if !ok {
		t.Fatalf("the element is a %T", st.part.Measures[0].Voices[0].Elements[0])
	}
	found := false
	for _, a := range r.Articulations {
		if a == model.Fermata {
			found = true
		}
	}
	if !found {
		t.Errorf("the rest came out bare, want the pause over it")
	}
}

// The words written beside the music are kept.
//
// They were read and thrown away, so every direction a composer wrote on the
// page was missing from it: "Mouvement modéré de Valse" over bar 31 of Danse
// Macabre, the "marcato" beside it, "una corda" under the first bar.
//
// Except the ones drawn in NoteWorthy's own symbol font, which are strokes
// rather than words: the 8va over bars 24 to 29 is a run of 0xF2 in it, which
// is a dash, and printing that as text would put a line of rubbish on the
// page where the bracket belongs.
func TestTheWordsBesideTheMusicAreKept(t *testing.T) {
	text := func(font byte, s string) []byte {
		// Three bytes of header, two more, and then the font the words
		// are drawn in.
		b := []byte{17, 0, 0, 0xF0, 0, font}
		b = append(b, []byte(s)...)
		return append(b, 0)
	}
	var b []byte
	b = append(b, text(2, "Mouvement modéré de Valse")...)
	b = append(b, text(nwcMusicFont, "\xF2\xF2\xF2\xF2")...)
	b = append(b, 8, 0, 0, 3, 0, 0x20, 0, 0, 0, 0, 5)

	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)
	got := marksOf(st.part.Measures[0].Voices[0].Elements[0]).Texts
	if len(got) != 1 {
		t.Fatalf("the note carries %v, want the one direction", got)
	}
	if got[0].Text != "Mouvement modéré de Valse" {
		t.Errorf("the note carries %q", got[0].Text)
	}
}

// A rest that another line is playing through is not printed.
//
// Two layers share one staff, and NoteWorthy prints the notes and leaves out
// the rest that sits under them: a layer that holds its place for half a bar
// while the other plays does it with a rest nobody sees. Bar 254 of Danse
// Macabre is one — a minim rest under two crotchets — and printed it sat in
// the middle of the music where the page has nothing.
//
// Hidden rather than thrown away: the rest is what holds the rest of its
// layer in time.
func TestARestUnderAnotherLineIsNotPrinted(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	half := model.Duration{Num: 1, Den: 2}
	note := func(d model.Duration) model.Element {
		return &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: d}
	}

	// One layer plays two crotchets; the other plays two quavers and then
	// holds its place with a minim rest.
	upper := &model.Part{Measures: []*model.Measure{{Voices: []*model.Voice{{
		Elements: []model.Element{note(quarter), note(quarter)},
	}}}}}
	lower := &model.Part{Measures: []*model.Measure{{Voices: []*model.Voice{{
		Elements: []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 3}, Dur: quarter},
			&model.Rest{Dur: half},
		},
	}}}}}
	out, _ := mergeLayers([]*model.Part{upper, lower}, []bool{true, false}, []string{"", ""})
	if len(out) != 1 {
		t.Fatalf("%d staves came back", len(out))
	}
	vs := out[0].Measures[0].Voices
	if len(vs) != 2 {
		t.Fatalf("%d voices in the bar", len(vs))
	}
	r, ok := vs[1].Elements[1].(*model.Rest)
	if !ok {
		t.Fatalf("the second element is a %T", vs[1].Elements[1])
	}
	if !r.Hidden {
		t.Error("the rest under the other line is printed")
	}
	if r.Dur.Cmp(half) != 0 {
		t.Errorf("the rest is %v long, want the %v that holds its line in time", r.Dur, half)
	}

	// A bar where nothing else plays keeps its rest on the page.
	a := &model.Part{Measures: []*model.Measure{{Voices: []*model.Voice{{
		Elements: []model.Element{&model.Rest{Dur: half}},
	}}}}}
	b := &model.Part{Measures: []*model.Measure{{Voices: []*model.Voice{{
		Elements: []model.Element{&model.Rest{Dur: quarter}},
	}}}}}
	out, _ = mergeLayers([]*model.Part{a, b}, []bool{true, false}, []string{"", ""})
	for _, v := range out[0].Measures[0].Voices {
		for _, e := range v.Elements {
			if r, ok := e.(*model.Rest); ok && r.Hidden {
				t.Error("a bar of silence lost its rest")
			}
		}
	}
}

// A rest chord holds time like any other rest.
//
// NoteWorthy keeps a rest as a chord when the layer it is in has notes
// elsewhere in the bar. Stepped over, its bar came out short: the left hand
// of Danse Macabre at bar 254 is one of these and five quavers, and without
// it the bar is a quaver light.
func TestARestChordHoldsItsTime(t *testing.T) {
	// Three bytes of header, the value block, and two more.
	b := []byte{18, 0, 0, 3, 0, 0x10, 2, 0, 0, 0x30, 0xF8, 1, 0}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	if err := w.object(st); err != nil {
		t.Fatalf("%v", err)
	}
	if w.i != len(b) {
		t.Errorf("the record came to %d bytes, want %d", w.i, len(b))
	}
	st.closeMeasure(model.BarFinal)
	els := st.part.Measures[0].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("%d elements came out", len(els))
	}
	r, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("the element is a %T", els[0])
	}
	if want := (model.Duration{Num: 1, Den: 8}); r.Dur.Cmp(want) != 0 {
		t.Errorf("the rest is %v long, want %v", r.Dur, want)
	}
}

// An octave bracket is a word and then a row of dashes, and only the hook on
// the end says how far it reaches.
//
// NoteWorthy has no ottava of its own: a staff author types "8va" over the
// music and draws the bracket after it in a music font. Read as text that is
// a word with nothing after it — which is what the page showed — and the
// dashes are a line of o-graves. Danse Macabre's first bracket runs from bar
// 19 to bar 29, where its hook turns down.
//
// A word with no bracket after it stays a word. Two thirds of them are.
func TestAnOctaveBracketReachesAsFarAsItsHook(t *testing.T) {
	text := func(font byte, s string) []byte {
		b := []byte{17, 0, 0, 0xF0, 0, font}
		b = append(b, []byte(s)...)
		return append(b, 0)
	}
	note := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 0, 5}
	dashes := "\xF2\xF2\xF2\xF2\xF2"

	read := func(b []byte) []model.Element {
		w := &walker{b: b, version: 205}
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		for w.remaining() > 0 {
			if err := w.object(st); err != nil {
				t.Fatalf("%v", err)
			}
		}
		st.closeMeasure(model.BarFinal)
		return st.part.Measures[0].Voices[0].Elements
	}

	// The word, two notes under it, more dashes, and the hook.
	var b []byte
	b = append(b, text(2, "8va")...)
	b = append(b, note...)
	b = append(b, text(nwcMusicFont, dashes)...)
	b = append(b, note...)
	b = append(b, text(nwcMusicFont, dashes+"w")...)
	b = append(b, note...)

	els := read(b)
	if len(els) != 3 {
		t.Fatalf("%d notes came out", len(els))
	}
	start := marksOf(els[0]).Octave
	if start == nil || !start.Start || start.Amount != 1 {
		t.Fatalf("the first note does not open the bracket: %+v", start)
	}
	if !start.SoundsAsWritten {
		t.Error("the bracket moves the music; the notes under it are at pitch already")
	}
	if stop := marksOf(els[1]).Octave; stop == nil || !stop.Stop {
		t.Errorf("the bracket does not end where its hook is")
	}
	// The line draws its own label, so the word is not written as well.
	for _, tx := range marksOf(els[0]).Texts {
		if tx.Text == "8va" {
			t.Error(`the word "8va" is printed as well as the bracket`)
		}
	}

	// And a word with no bracket after it is a word.
	var plain []byte
	plain = append(plain, text(2, "8va")...)
	plain = append(plain, note...)
	els = read(plain)
	if marksOf(els[0]).Octave != nil {
		t.Error("a word with no bracket after it drew one anyway")
	}
	found := false
	for _, tx := range marksOf(els[0]).Texts {
		if tx.Text == "8va" {
			found = true
		}
	}
	if !found {
		t.Error("the word was taken off the page and nothing drawn in its place")
	}
}

// A crescendo or a diminuendo is a dynamic variance, and the third byte of
// it says which way the wedge points.
//
// The record was stepped over, so no NoteWorthy score gscore opened had ever
// had a wedge in it. Which way round the styles are did not have to be
// guessed: a crescendo runs from a softer dynamic to a louder one, and
// across the corpus fifty-three of the sixty-two marked 0 that are followed
// by another dynamic get louder, while every one of the sixteen marked 1
// gets softer. The page agrees — bar 251 of Danse Macabre is marked 0, says
// "cresc.", and its wedge opens to the right.
func TestAWedgePointsTheWayTheStyleSays(t *testing.T) {
	note := []byte{8, 0, 0, 2, 0, 0x10, 0, 0, 0, 0, 5}
	variance := func(style byte) []byte { return []byte{15, 0, 0, 0x0E, 0, style} }

	read := func(style byte) []model.Element {
		var b []byte
		b = append(b, variance(style)...)
		b = append(b, note...)
		b = append(b, note...)
		b = append(b, 2, 0, 0, 0, 1) // the barline ends it
		w := &walker{b: b, version: 205}
		st := &staffState{part: &model.Part{Clef: model.Treble}}
		for w.remaining() > 0 {
			if err := w.object(st); err != nil {
				t.Fatalf("%v", err)
			}
		}
		return st.part.Measures[0].Voices[0].Elements
	}

	for style, want := range map[byte]model.HairpinKind{0: model.Crescendo, 1: model.Diminuendo} {
		els := read(style)
		if len(els) != 2 {
			t.Fatalf("style %d: %d notes", style, len(els))
		}
		h := marksOf(els[0]).Hairpin
		if h == nil || !h.Start {
			t.Fatalf("style %d opens no wedge", style)
		}
		if h.Kind != want {
			t.Errorf("style %d draws a %v, want a %v", style, h.Kind, want)
		}
		// It ends at the barline rather than running on down the page.
		if end := marksOf(els[1]).Hairpin; end == nil || !end.Stop {
			t.Errorf("style %d: the wedge does not close at the barline", style)
		}
	}

	// A dynamic closes it too, which is where NoteWorthy draws it to.
	var b []byte
	b = append(b, variance(0)...)
	b = append(b, note...)
	b = append(b, 7, 0, 0, 0, 0, 5, 0, 0, 0, 0) // forte
	b = append(b, note...)
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)
	els := st.part.Measures[0].Voices[0].Elements
	if h := marksOf(els[0]).Hairpin; h == nil || !h.Stop {
		t.Errorf("the wedge did not end at the dynamic it arrives at")
	}
}

// From 1.75 a slur is one bit, and where it ends has to be worked out.
//
// The file marks every element the phrase mark covers as joined to the next,
// and writes nothing at all about the far end: the slur stops on the first
// element after the run. 1.70 writes both ends and is read as it was.
//
// Measured against the page. Bar 195 of Danse Macabre carries the bit on six
// of its elements and bar 196 on two, and the mark printed over them runs
// from the second element of 195 to the third of 196 — the same eight. Read
// with the 1.70 rule, every file but Für Elise came back with no slurs at
// all; Danse Macabre has two hundred and fifteen.
func TestASlurEndsOnTheNoteAfterTheLastJoinedOne(t *testing.T) {
	// The slur is the sixth byte of the value block, which is the ninth of
	// the record.
	note := func(joined byte) []byte {
		return []byte{8, 0, 0, 3, 0, 0x20, 0, 0, joined, 0, 5}
	}
	var b []byte
	for _, j := range []byte{0, 1, 1, 1, 0, 0} {
		b = append(b, note(j)...)
	}
	w := &walker{b: b, version: 205}
	st := &staffState{part: &model.Part{Clef: model.Treble}}
	for w.remaining() > 0 {
		if err := w.object(st); err != nil {
			t.Fatalf("%v", err)
		}
	}
	st.closeMeasure(model.BarFinal)
	els := st.part.Measures[0].Voices[0].Elements

	var got string
	for _, e := range els {
		mk := marksOf(e)
		switch {
		case mk.SlurStart:
			got += "("
		case mk.SlurStop:
			got += ")"
		default:
			got += "."
		}
	}
	// The run of three opens on the first of them and closes on the one
	// after the last.
	if want := ".(..)."; got != want {
		t.Errorf("the slur reads %q, want %q", got, want)
	}

}
