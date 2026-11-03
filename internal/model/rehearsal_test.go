/*
 * rehearsal_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The rehearsal sequence.
 *
 * A mark is kept as a place in the sequence and the letter is worked out
 * from where it falls. The whole reason is editing: a letter written down
 * is right until somebody puts a mark in before it, and then the page
 * reads A, B, D, C with nothing to say which is wrong. Everything below is
 * some version of that.
 */

package model

import "testing"

// plain is a score of one staff and n bars.
func plain(bars int) *Score {
	s := NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < bars; i++ {
		p.Measures = append(p.Measures, NewMeasure())
	}
	return s
}

// named is what the marks come out as, by bar.
func named(s *Score) map[int]string { return s.RehearsalMarks(nil) }

func TestTheSequenceRunsInOrder(t *testing.T) {
	s := plain(10)
	for _, at := range []int{1, 4, 7} {
		s.SetMarker(at, "", true)
	}
	got := named(s)
	for at, want := range map[int]string{1: "A", 4: "B", 7: "C"} {
		if got[at] != want {
			t.Errorf("bar %d is %q, want %q", at+1, got[at], want)
		}
	}
	if len(got) != 3 {
		t.Errorf("%d marks, want 3", len(got))
	}
}

// The case the whole design is for.
func TestAMarkPutInBeforeTheOthersRenamesThem(t *testing.T) {
	s := plain(10)
	s.SetMarker(2, "", true)
	s.SetMarker(6, "", true)
	if got := named(s); got[2] != "A" || got[6] != "B" {
		t.Fatalf("started as %v", got)
	}

	// Now one before them both.
	s.SetMarker(0, "", true)
	got := named(s)
	for at, want := range map[int]string{0: "A", 2: "B", 6: "C"} {
		if got[at] != want {
			t.Errorf("bar %d is %q, want %q", at+1, got[at], want)
		}
	}
}

func TestTakingOneAwayClosesTheGap(t *testing.T) {
	s := plain(10)
	for _, at := range []int{0, 3, 6} {
		s.SetMarker(at, "", true)
	}
	s.SetMarker(3, "", false) // empty text and not in the sequence: gone

	got := named(s)
	if got[0] != "A" || got[6] != "B" {
		t.Errorf("after taking the middle one away: %v", got)
	}
	if _, still := got[3]; still {
		t.Error("the mark that was taken away is still there")
	}
}

/*
 * What they are called.
 */

func TestLettersSkipI(t *testing.T) {
	// A..H is eight, so the ninth is J.
	if got := RehearsalLetter(8); got != "J" {
		t.Errorf("the ninth letter is %q, want J — I reads as a one", got)
	}
	for i := 0; i < 25; i++ {
		if RehearsalLetter(i) == "I" {
			t.Errorf("letter %d is I", i)
		}
	}
}

// Past Z the letters double, because a conductor shouting "A B" across a
// hall is not heard as one thing.
func TestPastZTheLettersDouble(t *testing.T) {
	if got := RehearsalLetter(25); got != "AA" {
		t.Errorf("the 26th mark is %q, want AA", got)
	}
	if got := RehearsalLetter(26); got != "BB" {
		t.Errorf("the 27th mark is %q, want BB", got)
	}
	if got := RehearsalLetter(50); got != "AAA" {
		t.Errorf("the 51st mark is %q, want AAA", got)
	}
}

func TestTheSequenceCanBeNumbers(t *testing.T) {
	s := plain(10)
	s.Markers = MarkerNumbers
	for _, at := range []int{1, 4} {
		s.SetMarker(at, "", true)
	}
	got := named(s)
	if got[1] != "1" || got[4] != "2" {
		t.Errorf("numbered marks came out %v", got)
	}
}

// Film and television scores call the places by the bar they stand on.
func TestTheSequenceCanBeBarNumbers(t *testing.T) {
	s := plain(10)
	s.Markers = MarkerBarNumbers
	s.SetMarker(4, "", true)

	// With the bars numbered, the mark is named after its bar.
	bars := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := s.RehearsalMarks(bars)[4]; got != "5" {
		t.Errorf("the mark is %q, want 5", got)
	}
	// An upbeat shifts the numbering, and the mark follows it.
	bars = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	if got := s.RehearsalMarks(bars)[4]; got != "4" {
		t.Errorf("after an upbeat the mark is %q, want 4", got)
	}
}

/*
 * Marks written out as themselves.
 */

// An insert called "1a" and a word over a chorus are not places the
// sequence counts through, and must not be renamed by it.
func TestAWrittenMarkIsLeftAlone(t *testing.T) {
	s := plain(10)
	s.SetMarker(0, "", true)
	s.SetMarker(3, "Chorus", false)
	s.SetMarker(6, "", true)

	got := named(s)
	if got[3] != "Chorus" {
		t.Errorf("the chorus came out as %q", got[3])
	}
	// And it does not advance the sequence: the mark after it is B.
	if got[0] != "A" || got[6] != "B" {
		t.Errorf("the sequence around it reads %v", got)
	}
}

/*
 * Movements.
 */

// A symphony's second movement begins at letter A again. The alternative
// is a slow movement that starts at letter P.
func TestTheSequenceRestartsAtAMovement(t *testing.T) {
	s := plain(10)
	s.SetMarker(1, "", true)
	s.SetMarker(3, "", true)
	for _, p := range s.Parts {
		p.Measures[4].EndsSection = true
	}
	s.SetMarker(6, "", true)
	s.SetMarker(8, "", true)

	got := named(s)
	for at, want := range map[int]string{1: "A", 3: "B", 6: "A", 8: "B"} {
		if got[at] != want {
			t.Errorf("bar %d is %q, want %q", at+1, got[at], want)
		}
	}
}

/*
 * What the editor shows before anything is written.
 */

func TestTheNextMarkIsOfferedBeforeItIsWritten(t *testing.T) {
	s := plain(10)
	s.SetMarker(1, "", true)
	s.SetMarker(4, "", true)

	// After them both, the next is C.
	if got := s.NextMarkerIsLetter(7, nil); got != "C" {
		t.Errorf("after two marks the next is %q, want C", got)
	}
	// Between them, it is B — which is also what that mark will be
	// called once it is written, and what the one after it becomes.
	if got := s.NextMarkerIsLetter(2, nil); got != "B" {
		t.Errorf("between the two it offers %q, want B", got)
	}
}

/*
 * A mark belongs to the bar and not to a staff.
 */

func TestAMarkGoesOnEveryStaff(t *testing.T) {
	s := plain(4)
	s.Parts = append(s.Parts, &Part{
		Clef: Treble, Measures: []*Measure{
			NewMeasure(), NewMeasure(), NewMeasure(), NewMeasure(),
		},
	})
	s.SetMarker(2, "", true)

	for i, p := range s.Parts {
		if !p.Measures[2].HasMarker() {
			t.Errorf("staff %d has no mark at bar 3", i+1)
		}
	}
}

func TestHasMarkerSeesBothKinds(t *testing.T) {
	var none *Measure
	if none.HasMarker() {
		t.Error("nothing has a mark")
	}
	if (&Measure{}).HasMarker() {
		t.Error("an empty bar has a mark")
	}
	if !(&Measure{Marker: "A"}).HasMarker() {
		t.Error("a written mark is not seen")
	}
	if !(&Measure{MarkerAuto: true}).HasMarker() {
		t.Error("a sequenced mark is not seen")
	}
}

func TestMarkerStyleNamesRoundTrip(t *testing.T) {
	for _, st := range []MarkerStyle{MarkerLetters, MarkerNumbers, MarkerBarNumbers} {
		got, ok := MarkerStyleFromName(st.String())
		if !ok || got != st {
			t.Errorf("%v came back as %v (%v)", st, got, ok)
		}
	}
	if _, ok := MarkerStyleFromName("sideways"); ok {
		t.Error("a style nobody has heard of was accepted")
	}
}
