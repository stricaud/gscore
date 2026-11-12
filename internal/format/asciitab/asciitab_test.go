/*
 * asciitab_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The tablature guitarists type into text files.
 *
 * The format has no definition, so these are not tests against a standard:
 * they are tests against the tabs people actually write, as the Wikipedia
 * article and twenty years of forum posts have them — string names at the
 * left, pipes for bars, letters between the frets for the hand, and nothing
 * at all about rhythm.
 */

package asciitab

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// read parses a tab, failing the test if nothing at all came of it.
func read(t *testing.T, src string) (*model.Score, error) {
	t.Helper()
	s, err := Import(strings.NewReader(src))
	if s == nil {
		t.Fatalf("no score: %v", err)
	}
	return s, err
}

// bar is the elements of one bar of the one staff.
func bar(s *model.Score, i int) []model.Element {
	return s.Parts[0].Measures[i].Voices[0].Elements
}

// A tab as it is typed: a title, chord names over the bars, two lines of
// the page, and the things a hand does between the frets.
const song = `Greensleeves (trad.)

      Am          C
e|-----0-----3-----|-5h7-------|
B|-----1-----0-----|-----5p3---|
G|-----0-----0-----|-----------|
D|-----2-----0-----|-----------|
A|-----3-----2-----|-----------|
E|-----------3-----|-----------|

e|-12/14~--x---(5)--|
B|-------------------|
G|-------------------|
D|-------------------|
A|-------------------|
E|---------3b5-------|
`

func TestAChordIsTheFretsStruckInOneColumn(t *testing.T) {
	s, _ := read(t, song)
	first := bar(s, 0)
	if len(first) != 2 {
		t.Fatalf("the first bar holds %d things, want the two chords", len(first))
	}
	// The first thing in the bar is the A minor chord: five strings
	// struck together, read as one chord. The hyphens before it are the
	// tab's spacing, not a rest.
	ch, ok := first[0].(*model.Chord)
	if !ok {
		t.Fatalf("the first chord came out as a %T", first[0])
	}
	if len(ch.Notes) != 5 {
		t.Errorf("the A minor has %d notes, want five", len(ch.Notes))
	}
	// Top string open is E4; the A string at the third fret is C3.
	want := map[int]string{1: "E4", 5: "C3"}
	for _, n := range ch.Notes {
		if w, ok := want[n.String]; ok && n.Pitch.String() != w {
			t.Errorf("string %d fret %d sounds %s, want %s", n.String, n.Fret, n.Pitch, w)
		}
	}
}

func TestTheHandsMarksAreRead(t *testing.T) {
	s, _ := read(t, song)
	// Bar two: 5h7 on the top string, 5p3 on the second.
	second := bar(s, 1)
	var techs []model.Technique
	for _, el := range second {
		if n, ok := el.(*model.Note); ok {
			techs = append(techs, n.Techniques...)
		}
	}
	has := func(want model.Technique) bool {
		for _, tc := range techs {
			if tc == want {
				return true
			}
		}
		return false
	}
	if !has(model.HammerOn) {
		t.Errorf("the hammer-on in 5h7 was not read: %v", techs)
	}
	if !has(model.PullOff) {
		t.Errorf("the pull-off in 5p3 was not read: %v", techs)
	}
	// Bar three: a slide up into a two-figure fret, a dead note, a ghost
	// note, a vibrato and a bend.
	third := bar(s, 2)
	var frets []int
	var slid, dead, ghost, vib, bent bool
	// The x and the 3 stand in one column, so they are one chord: every
	// note of the bar, in a chord or on its own.
	var notes []*model.Note
	for _, el := range third {
		switch e := el.(type) {
		case *model.Note:
			notes = append(notes, e)
		case *model.Chord:
			notes = append(notes, e.Notes...)
		}
	}
	for _, n := range notes {
		frets = append(frets, n.Fret)
		for _, tc := range n.Techniques {
			switch tc {
			case model.SlideUp:
				slid = true
			case model.Vibrato:
				vib = true
			case model.BendFull:
				bent = true
			}
		}
		if n.Head == model.HeadX {
			dead = true
		}
		if n.Small {
			ghost = true
		}
	}
	for _, want := range []int{12, 14, 5, 3} {
		found := false
		for _, f := range frets {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("fret %d is missing from %v", want, frets)
		}
	}
	if !slid || !dead || !ghost || !vib || !bent {
		t.Errorf("slide %v dead %v ghost %v vibrato %v bend %v — want all of them",
			slid, dead, ghost, vib, bent)
	}
}

func TestTheTitleAndTheWordsAroundTheTab(t *testing.T) {
	s, err := read(t, song)
	if s.Identity.Title != "Greensleeves (trad.)" {
		t.Errorf("the title is %q", s.Identity.Title)
	}
	// The chord names over the bars are not read as music, and the file
	// says so, by line.
	p, ok := err.(*Partial)
	if !ok {
		t.Fatalf("the chord names went by in silence: %v", err)
	}
	joined := strings.Join(p.Details(), "\n")
	if !strings.Contains(joined, "line 3:") || !strings.Contains(joined, "Am") {
		t.Errorf("the chord line was not reported: %v", p.Details())
	}
}

func TestTheRhythmFollowsTheSpacing(t *testing.T) {
	// Four frets evenly spaced in a bar are four quarters; eight are
	// eighths. The tab says nothing else about time, so this is the
	// whole of what can be read.
	s, _ := read(t, `
e|----------------|----------------|
B|-1---1---1---1--|-1-1-1-1-1-1-1-1|
G|----------------|----------------|
`)
	for i, el := range bar(s, 0) {
		if d := el.Duration(); d != (model.Duration{Num: 1, Den: 4}) {
			t.Errorf("note %d of the first bar is %v, want a quarter", i+1, d)
		}
	}
	for i, el := range bar(s, 1) {
		if d := el.Duration(); d != (model.Duration{Num: 1, Den: 8}) {
			t.Errorf("note %d of the second bar is %v, want an eighth", i+1, d)
		}
	}
}

func TestAnEmptyBarIsABarOfRest(t *testing.T) {
	s, _ := read(t, `
e|-----|--3--|
B|-----|-----|
G|-----|-----|
`)
	first := bar(s, 0)
	if len(first) != 1 {
		t.Fatalf("the empty bar holds %d things", len(first))
	}
	if _, ok := first[0].(*model.Rest); !ok {
		t.Errorf("the empty bar holds a %T, want a rest", first[0])
	}
}

func TestFourStringsAreABass(t *testing.T) {
	s, _ := read(t, `
G|---------|
D|---------|
A|-----5---|
E|-3-------|
`)
	p := s.Parts[0]
	if p.Name != "Bass" || !strings.HasPrefix(p.Sound, "pluck.bass") {
		t.Errorf("four strings came out as %q playing %q", p.Name, p.Sound)
	}
	// A bass's bottom string is E1, so its third fret is G1.
	n := bar(s, 0)[0].(*model.Note)
	if n.Pitch.String() != "G1" {
		t.Errorf("the third fret of the bottom string sounds %s, want G1", n.Pitch)
	}
}

func TestTheTuningIsReadOffTheStringNames(t *testing.T) {
	// Drop D: the bottom string a tone down, and nothing else changed.
	s, _ := read(t, `
e|-----|
B|-----|
G|-----|
D|-----|
A|-----|
D|-0---|
`)
	tab := s.Parts[0].Tab
	if tab == nil || len(tab.Strings) != 6 {
		t.Fatalf("the staff is not a six-string tab: %v", tab)
	}
	if got := tab.Strings[5].String(); got != "D2" {
		t.Errorf("the bottom string is %s, want D2", got)
	}
	if got := tab.Strings[0].String(); got != "E4" {
		t.Errorf("the top string is %s, want E4", got)
	}
}

func TestATabWithNoBarlinesIsOneBar(t *testing.T) {
	s, _ := read(t, `
e|--0--2--3--
B|-----------
G|-----------
`)
	if n := len(s.Parts[0].Measures); n != 1 {
		t.Errorf("%d bars, want one", n)
	}
	if n := len(bar(s, 0)); n != 3 {
		t.Errorf("the bar holds %d things, want three notes", n)
	}
}

func TestTwoLinesOfTheTabAreOnePart(t *testing.T) {
	s, _ := read(t, song)
	if n := len(s.Parts); n != 1 {
		t.Errorf("%d parts, want one", n)
	}
	if n := len(s.Parts[0].Measures); n != 3 {
		t.Errorf("%d bars, want three across the two lines", n)
	}
	if last := s.Parts[0].Measures[2].RightBarline; last != model.BarFinal {
		t.Errorf("the piece ends with %v, want the final barline", last)
	}
}

func TestAFileWithNoTabInItIsRefused(t *testing.T) {
	_, err := Import(strings.NewReader("Just some words\nand a second line\n"))
	if err == nil || !strings.Contains(err.Error(), "no tablature") {
		t.Errorf("a text file was read as a tab: %v", err)
	}
}
