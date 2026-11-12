/*
 * guido_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading GUIDO Music Notation.
 *
 * The examples below are the specification's own wherever one exists,
 * because the thing being tested is agreement with somebody else's format
 * and our opinion of what a file should look like is not evidence.
 */

package guido

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// read is a file, read, failing the test if there was no music in it.
func read(t *testing.T, src string) *model.Score {
	t.Helper()
	s, err := Import(strings.NewReader(src))
	if s == nil {
		t.Fatalf("nothing was read: %v", err)
	}
	return s
}

// notesIn lists the notes of one bar of one staff, as pitch names.
func notesIn(t *testing.T, s *model.Score, part, bar int) []*model.Note {
	t.Helper()
	if part >= len(s.Parts) {
		t.Fatalf("there is no part %d; the score has %d", part, len(s.Parts))
	}
	p := s.Parts[part]
	if bar >= len(p.Measures) {
		t.Fatalf("there is no bar %d; the staff has %d", bar+1, len(p.Measures))
	}
	var out []*model.Note
	for _, el := range p.Measures[bar].Voices[0].Elements {
		switch n := el.(type) {
		case *model.Note:
			out = append(out, n)
		case *model.Chord:
			out = append(out, n.Notes...)
		}
	}
	return out
}

// A note's octave is the one thing a reader cannot get nearly right. The
// spec fixes a1 as the 440 Hz A, which is A4 everywhere else, so every
// octave in the file is three below the one gscore means.
func TestTheOctaveNumbersAreThreeApart(t *testing.T) {
	s := read(t, `[ a1/4 c1 c0 c-1 ]`)
	want := []int{4, 4, 3, 2}
	got := notesIn(t, s, 0, 0)
	if len(got) != len(want) {
		t.Fatalf("read %d notes, want %d", len(got), len(want))
	}
	for i, n := range got {
		if n.Pitch.Octave != want[i] {
			t.Errorf("note %d is in octave %d, want %d (written %s)",
				i, n.Pitch.Octave, want[i], n.Pitch)
		}
	}
}

// h is the German B and b the international one, and they are the same
// note. Read as a B flat — which is what b means in German — a whole file
// comes back a semitone low wherever it is written that way.
func TestBIsBNaturalAndNotBFlat(t *testing.T) {
	s := read(t, `[ h1/4 b1 b&1 h&1 ]`)
	want := []int{0, 0, -1, -1}
	for i, n := range notesIn(t, s, 0, 0) {
		if n.Pitch.Step != model.StepB {
			t.Errorf("note %d is %v, want B", i, n.Pitch.Step)
		}
		if n.Pitch.Alter != want[i] {
			t.Errorf("note %d sounds %+d, want %+d", i, n.Pitch.Alter, want[i])
		}
	}
}

// The three naming systems, which a file may mix even though it should not.
func TestTheNoteNamesAreReadInEverySystem(t *testing.T) {
	s := read(t, `[ do1/4 re mi fa sol la si cis dis fis gis ais ]`)
	want := []struct {
		step  model.Step
		alter int
	}{
		{model.StepC, 0}, {model.StepD, 0}, {model.StepE, 0}, {model.StepF, 0},
		{model.StepG, 0}, {model.StepA, 0}, {model.StepB, 0},
		{model.StepC, 1}, {model.StepD, 1}, {model.StepF, 1},
		{model.StepG, 1}, {model.StepA, 1},
	}
	got := notesIn(t, s, 0, 0)
	if len(got) != len(want) {
		t.Fatalf("read %d notes, want %d", len(got), len(want))
	}
	for i, n := range got {
		if n.Pitch.Step != want[i].step || n.Pitch.Alter != want[i].alter {
			t.Errorf("note %d is %v%+d, want %v%+d",
				i, n.Pitch.Step, n.Pitch.Alter, want[i].step, want[i].alter)
		}
	}
}

// An omitted octave or duration is the last one given, which is how a real
// file is written: the scale below says its octave once.
func TestTheOctaveAndLengthCarryFromTheNoteBefore(t *testing.T) {
	s := read(t, `[ \meter<"4/4"> c1/4 d e f g a h c2 ]`)
	notes := notesIn(t, s, 0, 0)
	notes = append(notes, notesIn(t, s, 0, 1)...)
	if len(notes) != 8 {
		t.Fatalf("read %d notes, want 8", len(notes))
	}
	for i, n := range notes {
		want := 4
		if i == 7 {
			want = 5
		}
		if n.Pitch.Octave != want {
			t.Errorf("note %d is in octave %d, want %d", i, n.Pitch.Octave, want)
		}
		if got := (model.Duration{Num: 1, Den: 4}); n.Dur.Cmp(got) != 0 {
			t.Errorf("note %d lasts %s, want %s", i, n.Dur, got)
		}
	}
}

// The duration forms of the spec, including its own examples.
func TestTheLengthsAreReadAsWritten(t *testing.T) {
	s := read(t, `[ c1*3/4 c*2 c/8 c/4. c/4.. c*500ms ]`)
	want := []model.Duration{
		{Num: 3, Den: 4}, {Num: 2, Den: 1}, {Num: 1, Den: 8},
		{Num: 3, Den: 8}, {Num: 7, Den: 16}, {Num: 1, Den: 8},
	}
	got := notesIn(t, s, 0, 0)
	if len(got) != len(want) {
		t.Fatalf("read %d notes, want %d", len(got), len(want))
	}
	for i, n := range got {
		if n.Dur.Cmp(want[i]) != 0 {
			t.Errorf("note %d lasts %s, want %s", i, n.Dur, want[i])
		}
	}
}

// Comments, both kinds. The nested one matters: taking four bars out of a
// piece to try something else is how somebody writes one.
func TestCommentsAreNotMusic(t *testing.T) {
	s := read(t, `
% a whole line of it
[ c1/4 % and the end of this one
  d (* taken out: e f
     (* and this was already out *)
     g *) a ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 3 {
		t.Fatalf("read %d notes, want 3 (c d a)", len(got))
	}
	if got[2].Pitch.Step != model.StepA {
		t.Errorf("the last note is %v, want A — the comment was read as music",
			got[2].Pitch.Step)
	}
}

/*
 * Barring, which is the whole of the difference between the two models
 */

// The meter cuts the bars, because GUIDO's autoBarlines says so and most
// files have no barline in them at all.
func TestTheMeterCutsTheBars(t *testing.T) {
	s := read(t, `[ \meter<"3/4"> c1/4 d e f g a h c2 d ]`)
	if n := len(s.Parts[0].Measures); n != 3 {
		t.Fatalf("nine quarters in 3/4 came out as %d bars, want 3", n)
	}
	for i := 0; i < 3; i++ {
		if n := len(notesIn(t, s, 0, i)); n != 3 {
			t.Errorf("bar %d holds %d notes, want 3", i+1, n)
		}
	}
}

// An explicit barline overrides the meter, which is how an upbeat is
// written — the spec's own example.
func TestAnExplicitBarlineMakesAnUpbeat(t *testing.T) {
	s := read(t, `[ \meter<"3/4"> a0/8 h | c1/2 c/4 | c/2. ]`)
	if n := len(s.Parts[0].Measures); n != 3 {
		t.Fatalf("the upbeat came out as %d bars, want 3", n)
	}
	if n := len(notesIn(t, s, 0, 0)); n != 2 {
		t.Errorf("the upbeat holds %d notes, want 2", n)
	}
}

// A barline that says what the meter already said is still a barline.
//
// It was dropped. The meter cut the bar at the fourth quarter, the \doubleBar
// after it closed a measure that did not exist yet, and the double bar went
// missing — along with every \repeatEnd that fell at the end of a full bar,
// which is where a repeat is always written.
func TestABarlineThatAgreesWithTheMeterIsNotLost(t *testing.T) {
	s := read(t, `[ \meter<"4/4"> c1/2 e&/4 e& | _/4 f a&/2 \doubleBar c/4 d e f ]`)
	ms := s.Parts[0].Measures
	if len(ms) != 3 {
		t.Fatalf("read %d bars, want 3", len(ms))
	}
	if ms[1].RightBarline != model.BarDouble {
		t.Errorf("bar 2 closes with a %v, want a double bar", ms[1].RightBarline)
	}
}

func TestARepeatClosesTheBarItIsWrittenAt(t *testing.T) {
	s := read(t, `[ \meter<"2/4"> \repeatBegin c1/4 d e f \repeatEnd g a ]`)
	ms := s.Parts[0].Measures
	if len(ms) != 3 {
		t.Fatalf("read %d bars, want 3", len(ms))
	}
	if ms[1].RightBarline != model.BarRepeatEnd {
		t.Errorf("bar 2 closes with a %v, want the end of a repeat",
			ms[1].RightBarline)
	}
}

// A bar that does not add up is kept, and said. The music is somebody's
// piece; refusing it or quietly cutting a note in half would both be worse
// than printing a bar that is too long.
func TestAnOverfullBarIsKeptAndReported(t *testing.T) {
	s, err := Import(strings.NewReader(`[ \meter<"2/4"> c1/4 d/2 e/4 ]`))
	if s == nil {
		t.Fatal("the music was thrown away")
	}
	if err == nil {
		t.Error("nothing was said about three quarters crammed into 2/4")
	}
	kept := 0
	for _, m := range s.Parts[0].Measures {
		kept += len(m.Voices[0].Elements)
	}
	if kept != 3 {
		t.Errorf("%d notes survived, want 3", kept)
	}
}

/*
 * Spelling
 */

// GUIDO repeats an accidental on every altered note, because the spec says
// one is valid only where it is written. Printing them all put a sharp on
// every note of an F sharp minor bar.
func TestAnAccidentalIsPrintedOnceAndTheNaturalThatEndsIt(t *testing.T) {
	s := read(t, `[ \meter<"4/4"> c#1/4 c# c d ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 4 {
		t.Fatalf("read %d notes, want 4", len(got))
	}
	want := []model.Accidental{
		model.Sharp, model.NoAccidental, model.Natural, model.NoAccidental,
	}
	for i, n := range got {
		if n.Accidental != want[i] {
			t.Errorf("note %d prints %v, want %v", i, n.Accidental, want[i])
		}
	}
	// And what they sound, which the printing must not have changed.
	alters := []int{1, 1, 0, 0}
	for i, n := range got {
		if n.Pitch.Alter != alters[i] {
			t.Errorf("note %d sounds %+d, want %+d", i, n.Pitch.Alter, alters[i])
		}
	}
}

// What the key already says is not written on the note.
func TestTheKeySignatureIsNotPrintedAgainOnEveryNote(t *testing.T) {
	s := read(t, `[ \key<2> \meter<"4/4"> f#1/4 c# f g ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 4 {
		t.Fatalf("read %d notes, want 4", len(got))
	}
	if got[0].Accidental != model.NoAccidental {
		t.Errorf("the F sharp of D major prints %v, want nothing",
			got[0].Accidental)
	}
	if got[0].Pitch.Alter != 1 {
		t.Errorf("the F sharp sounds %+d, want +1", got[0].Pitch.Alter)
	}
	if got[2].Accidental != model.Natural {
		t.Errorf("the F natural after it prints %v, want a natural",
			got[2].Accidental)
	}
}

func TestAKeyIsReadBothWaysItIsWritten(t *testing.T) {
	for _, tc := range []struct {
		src    string
		fifths int
		mode   model.Mode
	}{
		{`\key<0>`, 0, model.Major},
		{`\key<-3>`, -3, model.Major},
		{`\key<+2>`, 2, model.Major},
		{`\key<"C">`, 0, model.Major},
		{`\key<"e&">`, -6, model.Minor},
		{`\key<"D">`, 2, model.Major},
	} {
		s := read(t, "[ "+tc.src+" c1/4 ]")
		got := s.Parts[0].Key
		if got.Fifths != tc.fifths || got.Mode != tc.mode {
			t.Errorf("%s is %d/%v, want %d/%v",
				tc.src, got.Fifths, got.Mode, tc.fifths, tc.mode)
		}
	}
}

/*
 * Tags
 */

func TestTheClefsAreRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		want model.Clef
	}{
		{"g2", model.Treble}, {"treble", model.Treble}, {"violino", model.Treble},
		{"f", model.Bass}, {"f4", model.Bass}, {"basso", model.Bass},
		{"c3", model.Alto}, {"alto", model.Alto},
		{"c4", model.Tenor}, {"tenor", model.Tenor},
		{"perc", model.Percussion},
		{"g2-8", model.Treble},
	} {
		s := read(t, `[ \clef<"`+tc.name+`"> c1/4 ]`)
		if got := s.Parts[0].Clef; got != tc.want {
			t.Errorf("\\clef<%q> is a %v, want a %v", tc.name, got, tc.want)
		}
	}
}

func TestASlurCoversTheNotesItIsWrittenRound(t *testing.T) {
	s := read(t, `[ \meter<"4/4"> \sl(c1/4 d e) f ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 4 {
		t.Fatalf("read %d notes, want 4", len(got))
	}
	if got[0].SlurStart == 0 || got[0].SlurStop > 0 {
		t.Error("the slur does not open on the first note")
	}
	if got[2].SlurStop == 0 {
		t.Error("the slur does not close on the third note")
	}
	if got[3].SlurStart > 0 || got[3].SlurStop > 0 {
		t.Error("the note after the slur is inside it")
	}
}

// A tie joins each note of its group to the next, which is not the same as
// a mark on the ends: \tie(a b c) is two ties and not one.
func TestATieJoinsEachNoteToTheNext(t *testing.T) {
	s := read(t, `[ \tie(c1/4 c c) ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 3 {
		t.Fatalf("read %d notes, want 3", len(got))
	}
	for i, n := range got {
		wantStart, wantStop := i < 2, i > 0
		if n.TieStart != wantStart || n.TieStop != wantStop {
			t.Errorf("note %d is tied start=%v stop=%v, want %v/%v",
				i, n.TieStart, n.TieStop, wantStart, wantStop)
		}
	}
}

// The ranges that open and close with their own tags, which is the only way
// to write a span that crosses a barline.
func TestASlurCanOpenInOneBarAndCloseInTheNext(t *testing.T) {
	s := read(t, `[ \meter<"2/4"> c1/4 \slurBegin d | e \slurEnd f ]`)
	first := notesIn(t, s, 0, 0)
	second := notesIn(t, s, 0, 1)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("read %d and %d notes, want 2 and 2", len(first), len(second))
	}
	if first[1].SlurStart == 0 {
		t.Error("the slur does not open on the last note of bar 1")
	}
	if second[0].SlurStop == 0 {
		t.Error("the slur does not close on the first note of bar 2")
	}
}

func TestAChordIsOneElement(t *testing.T) {
	s := read(t, `[ {c1/4., e&, g} ]`)
	p := s.Parts[0].Measures[0].Voices[0].Elements
	if len(p) != 1 {
		t.Fatalf("the chord came out as %d elements, want 1", len(p))
	}
	c, ok := p[0].(*model.Chord)
	if !ok {
		t.Fatalf("the chord came out as a %T", p[0])
	}
	if len(c.Notes) != 3 {
		t.Fatalf("the chord has %d notes, want 3", len(c.Notes))
	}
	// Its length is its longest note's, and the notes that said nothing
	// take it.
	want := model.Duration{Num: 3, Den: 8}
	if c.Dur.Cmp(want) != 0 {
		t.Errorf("the chord lasts %s, want %s", c.Dur, want)
	}
	if c.Notes[1].Pitch.Alter != -1 {
		t.Errorf("the E flat sounds %+d, want -1", c.Notes[1].Pitch.Alter)
	}
}

func TestGraceNotesKeepTheirAccidentals(t *testing.T) {
	s := read(t, `[ \grace(c1/16 d&) e/4 ]`)
	notes := notesIn(t, s, 0, 0)
	if len(notes) != 1 {
		t.Fatalf("read %d notes of music, want 1", len(notes))
	}
	g := notes[0].Grace
	if g == nil || len(g.Notes) != 2 {
		t.Fatalf("the grace group has %v", g)
	}
	if g.Notes[1].Pitch.Alter != -1 {
		t.Errorf("the grace D flat sounds %+d, want -1 — the flat was lost "+
			"because grace notes never reach the pass that resolves one",
			g.Notes[1].Pitch.Alter)
	}
	if g.Notes[1].Accidental != model.Flat {
		t.Errorf("the grace D flat prints %v, want a flat", g.Notes[1].Accidental)
	}
}

// Basic GUIDO has no \tuplet tag at all: a triplet is three notes of a
// twelfth and nothing else. gscore cannot draw a twelfth — no notehead
// spells one — so the ratio has to be worked out from the lengths.
func TestATripletWithNoTagIsStillATriplet(t *testing.T) {
	s := read(t, `[ \meter<"4/4"> c1*1/12 d e f/4 g a ]`)
	got := notesIn(t, s, 0, 0)
	if len(got) != 6 {
		t.Fatalf("read %d notes, want 6", len(got))
	}
	for i := 0; i < 3; i++ {
		tp := got[i].Tuplet
		if tp == nil {
			t.Fatalf("note %d is not in a tuplet", i)
		}
		if tp.Actual != 3 || tp.Normal != 2 {
			t.Errorf("note %d is %d in the time of %d, want 3 in the time of 2",
				i, tp.Actual, tp.Normal)
		}
	}
	if !got[0].Tuplet.Start || !got[2].Tuplet.Stop {
		t.Error("the bracket does not cover exactly the three notes")
	}
	if got[3].Tuplet != nil {
		t.Error("the quarter after the triplet was swept into it")
	}
}

func TestTheTupletTagIsBelievedWhereItIsWritten(t *testing.T) {
	s := read(t, `[ \tuplet<"5:4">(c1*1/20 d e f g) ]`)
	for i, n := range notesIn(t, s, 0, 0) {
		if n.Tuplet == nil || n.Tuplet.Actual != 5 || n.Tuplet.Normal != 4 {
			t.Fatalf("note %d is %v, want 5 in the time of 4", i, n.Tuplet)
		}
	}
}

func TestTupletRatiosAreReadEveryWayTheyAreWritten(t *testing.T) {
	for _, tc := range []struct {
		spec           string
		actual, normal int
	}{
		{"3", 3, 2}, {"-3-", 3, 2}, {"5:4", 5, 4}, {"-7:8-", 7, 8},
		{"6", 6, 4}, {"--", 0, 0}, {"", 0, 0},
	} {
		a, n := tupletRatio(tc.spec)
		if a != tc.actual || n != tc.normal {
			t.Errorf("%q is %d:%d, want %d:%d", tc.spec, a, n, tc.actual, tc.normal)
		}
	}
}

/*
 * Voices and staves
 */

// Voices that name the same staff share it, which is the spec's own
// example of two lines on one stave and a third on another.
func TestVoicesOnTheSameStaffShareIt(t *testing.T) {
	s := read(t, `{
	  [ \staff<1> \meter<"2/4"> e1/8. \slur(f/16 e/8) d ],
	  [ \staff<1> \meter<"2/4"> c1/4 d ],
	  [ \staff<2> \meter<"2/4"> a0/4 h ] }`)
	if len(s.Parts) != 2 {
		t.Fatalf("the score has %d staves, want 2", len(s.Parts))
	}
	if n := len(s.Parts[0].Measures[0].Voices); n != 2 {
		t.Errorf("the first staff holds %d voices, want 2", n)
	}
	if n := len(s.Parts[1].Measures[0].Voices); n != 1 {
		t.Errorf("the second staff holds %d voices, want 1", n)
	}
}

// With nothing said, each voice is its own staff.
func TestVoicesWithNoStaffTagGetOneEach(t *testing.T) {
	s := read(t, `{ [ c1/4 ], [ e1/4 ], [ g1/4 ] }`)
	if len(s.Parts) != 3 {
		t.Fatalf("the score has %d staves, want 3", len(s.Parts))
	}
}

func TestTheHeaderBelongsToThePiece(t *testing.T) {
	s := read(t, `{ [ \title<"Aubade"> \composer<"Holger H. Hoos">
	  \tempo<"Moderato","1/4=60"> \instr<"Flauto 1"> e&2/8 _ ] }`)
	if s.Identity.Title != "Aubade" {
		t.Errorf("the title is %q, want Aubade", s.Identity.Title)
	}
	if s.Identity.Composer != "Holger H. Hoos" {
		t.Errorf("the composer is %q", s.Identity.Composer)
	}
	if s.Tempo.BPM != 60 || s.Tempo.Text != "Moderato" {
		t.Errorf("the tempo is %+v, want Moderato at 60", s.Tempo)
	}
	if s.Parts[0].Name != "Flauto 1" {
		t.Errorf("the staff is called %q, want Flauto 1", s.Parts[0].Name)
	}
}

func TestVoltaBracketsAreReadFromTheRepeat(t *testing.T) {
	s := read(t, `[ \meter<"3/4"> \repeatBegin c1/2 g/4
	  \volta<"1.">(e/4 g/2) \repeatEnd \volta<"2.">(e&/4 f e&) ]`)
	ms := s.Parts[0].Measures
	if len(ms) != 3 {
		t.Fatalf("read %d bars, want 3", len(ms))
	}
	if ms[1].Ending == nil || ms[1].Ending.Numbers[0] != 1 {
		t.Errorf("bar 2 carries %v, want a first-time bracket", ms[1].Ending)
	}
	if ms[2].Ending == nil || ms[2].Ending.Numbers[0] != 2 {
		t.Errorf("bar 3 carries %v, want a second-time bracket", ms[2].Ending)
	}
}

/*
 * Never refusing a file
 */

// One character nobody can read is not a reason to lose the piece.
func TestABadCharacterLosesItselfAndNothingElse(t *testing.T) {
	s, err := Import(strings.NewReader(`[ c1/4 d ? e f ]`))
	if s == nil {
		t.Fatalf("the whole file was refused: %v", err)
	}
	if err == nil {
		t.Error("nothing was said about the character that could not be read")
	}
	if n := len(notesIn(t, s, 0, 0)); n != 4 {
		t.Errorf("%d notes survived, want 4", n)
	}
}

func TestAVoiceMissingItsBracketIsStillRead(t *testing.T) {
	s, err := Import(strings.NewReader(`{ c1/4 d e f ] }`))
	if s == nil {
		t.Fatalf("the whole file was refused: %v", err)
	}
	if err == nil {
		t.Error("nothing was said about the missing bracket")
	}
	if n := len(notesIn(t, s, 0, 0)); n != 4 {
		t.Errorf("%d notes survived, want 4", n)
	}
}

// A file with nothing in it is the one case worth refusing, because there
// is nothing to hand back.
func TestAnEmptyFileIsRefused(t *testing.T) {
	if _, err := Import(strings.NewReader("% nothing but a comment\n")); err == nil {
		t.Error("a file with no music in it was accepted")
	}
}

func TestThePartialReadSaysWhatWasSkipped(t *testing.T) {
	_, err := Import(strings.NewReader(`[ c1/4 ? d ]`))
	p, ok := err.(*PartialError)
	if !ok {
		t.Fatalf("the error is a %T, want a PartialError", err)
	}
	if len(p.Details()) == 0 {
		t.Error("the complaint lists nothing")
	}
	if !strings.Contains(p.Details()[0], "line 1") {
		t.Errorf("the complaint does not say where: %q", p.Details()[0])
	}
}
