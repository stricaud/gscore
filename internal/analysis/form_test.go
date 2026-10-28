/*
 * form_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What repeats what.
 *
 * Music is made of things coming back, and a list of where they come back is
 * the skeleton somebody listening is trying to hear. It is also the thing a
 * composer most often cannot see in their own work, because they know what
 * they meant by it.
 */

package analysis

import "testing"

// bar builds a bar of quarter notes on the given pitches, one staff.
func bar(pitches ...int) BarShape {
	var notes []BarNote
	for i, p := range pitches {
		notes = append(notes, BarNote{
			Part: 0, MIDI: p, At: float64(i) * 0.25, Held: 0.25,
		})
	}
	return Shaped(notes)
}

func shapes(bars ...BarShape) []BarShape { return bars }

// Four phrases with nothing in common. They have to be built carefully: a
// shape is counted from the bar's own lowest note, so C D E F and G A B C
// are the same shape a fifth apart — which is the whole point of the thing
// and makes it very easy to write a test fixture that is secretly a
// sequence. The first draft of this one was, and the code was right about it.
var (
	phraseA = bar(60, 62, 64, 65) // 0 2 4 5
	phraseB = bar(55, 58, 62)     // 0 3 7
	phraseC = bar(72, 73)         // 0 1
	phraseD = bar(48, 52, 55, 60) // 0 4 7 12
)

func TestThePhraseAndTheSamePhraseAgainHaveOneShape(t *testing.T) {
	a := bar(60, 62, 64, 65)
	b := bar(60, 62, 64, 65)
	if a.Shape != b.Shape {
		t.Error("the same bar written twice came out as two shapes")
	}
	if a.Base != b.Base {
		t.Error("and at two pitches")
	}
}

// The same phrase a fifth higher is the same phrase, which is how a sequence
// is found as well as a literal return.
func TestThePhraseMovedKeepsItsShape(t *testing.T) {
	low := bar(60, 62, 64, 65)
	high := bar(67, 69, 71, 72)
	if low.Shape != high.Shape {
		t.Error("the same phrase a fifth up came out as a different shape")
	}
	if high.Base-low.Base != 7 {
		t.Errorf("they are %d apart, want a fifth", high.Base-low.Base)
	}
}

// A different phrase is a different shape, however alike it looks.
func TestADifferentPhraseIsADifferentShape(t *testing.T) {
	if bar(60, 62, 64, 65).Shape == bar(60, 62, 64, 67).Shape {
		t.Error("two different bars came out as one shape")
	}
	// The same pitches in another rhythm is other music.
	one := Shaped([]BarNote{{MIDI: 60, At: 0, Held: 0.5}, {MIDI: 62, At: 0.5, Held: 0.5}})
	two := Shaped([]BarNote{{MIDI: 60, At: 0, Held: 0.25}, {MIDI: 62, At: 0.25, Held: 0.75}})
	if one.Shape == two.Shape {
		t.Error("the same notes in another rhythm came out as the same bar")
	}
}

// Which staff plays what is part of the music: the same tune in the cellos
// is not the same bar as the tune in the flutes.
func TestWhoPlaysItIsPartOfTheShape(t *testing.T) {
	flute := Shaped([]BarNote{{Part: 0, MIDI: 72, At: 0, Held: 0.25}})
	cello := Shaped([]BarNote{{Part: 4, MIDI: 72, At: 0, Held: 0.25}})
	if flute.Shape == cello.Shape {
		t.Error("the same note on two staves came out as one bar")
	}
}

func TestAnEmptyBarHasNoShape(t *testing.T) {
	if s := Shaped(nil); s.Notes != 0 || s.Shape != "" {
		t.Errorf("an empty bar came out as %+v", s)
	}
}

// Four bars that come back four bars later are a return, and it is reported
// once rather than bar by bar.
func TestAReturnIsFoundAndReportedOnce(t *testing.T) {
	a, b, c, d := phraseA, phraseB, phraseC, phraseD
	got := Echoes(shapes(a, b, c, d, a, b, c, d), LeastEcho)
	if len(got) != 1 {
		t.Fatalf("%d returns for one repeat of four bars: %+v", len(got), got)
	}
	e := got[0]
	if e.FromBar != 4 || e.ToBar != 7 || e.LikeBar != 0 || e.Bars != 4 {
		t.Errorf("the return came out as %+v, want bars 4-7 repeating 0-3", e)
	}
	if e.Transposed != 0 {
		t.Errorf("it is said to be %d semitones away, and it is the same notes",
			e.Transposed)
	}
}

// A sequence is the same music somewhere else, and saying how far is the
// whole point of telling somebody about it.
func TestASequenceSaysHowFarItHasMoved(t *testing.T) {
	a, b := bar(60, 62), bar(64, 65)
	up := []BarShape{bar(62, 64), bar(66, 67)}
	got := Echoes(shapes(a, b, up[0], up[1]), LeastEcho)
	if len(got) != 1 {
		t.Fatalf("%d returns: %+v", len(got), got)
	}
	if got[0].Transposed != 2 {
		t.Errorf("the sequence is said to be %d semitones up, want a whole tone",
			got[0].Transposed)
	}
}

// One bar coming back is an accompaniment figure, and a list of every bar of
// an Alberti bass matching the one before it is a list of nothing.
func TestOneBarComingBackIsNotAReturn(t *testing.T) {
	a, b, c := phraseA, phraseB, phraseC
	got := Echoes(shapes(a, b, c, a, c, b), LeastEcho)
	for _, e := range got {
		if e.Bars < LeastEcho {
			t.Errorf("a single bar was reported as a return: %+v", e)
		}
	}
}

// A bar belongs to the longest return it is part of, and that return is
// measured against the earliest place the music was heard: the
// recapitulation of a sonata is a return of the exposition, not of the
// development's own quotation of it.
func TestTheLongestReturnWinsAndPointsAtTheFirstTime(t *testing.T) {
	a, b, c, d := phraseA, phraseB, phraseC, phraseD
	// A B C D   A B   A B C D
	got := Echoes(shapes(a, b, c, d, a, b, a, b, c, d), LeastEcho)
	if len(got) != 2 {
		t.Fatalf("%d returns: %+v", len(got), got)
	}
	if got[1].Bars != 4 || got[1].LikeBar != 0 {
		t.Errorf("the second return is %+v, want four bars pointing at bar 0", got[1])
	}
}

// Rests are not a return. A piece where half the staves are silent for
// eighty bars would otherwise be reported as repeating itself continuously.
func TestSilenceIsNotARepeat(t *testing.T) {
	quiet := BarShape{}
	got := Echoes(shapes(quiet, quiet, quiet, quiet, quiet, quiet), LeastEcho)
	if len(got) != 0 {
		t.Errorf("silence was reported as repeating: %+v", got)
	}
}

// A return cannot reach into the stretch it is a return of, or two bars
// repeating each other would be read as one return a thousand bars long.
func TestAReturnDoesNotRunIntoItself(t *testing.T) {
	a := phraseA
	got := Echoes(shapes(a, a, a, a, a, a), LeastEcho)
	for _, e := range got {
		if e.LikeBar+e.Bars > e.FromBar {
			t.Errorf("a return overlaps the music it returns to: %+v", e)
		}
	}
}

func TestTheBusiestStaffSetsTheScale(t *testing.T) {
	all := []Busy{
		{Part: 0, Bars: []int{1, 4, 2}},
		{Part: 1, Bars: []int{0, 0, 9}},
	}
	if got := Busiest(all); got != 9 {
		t.Errorf("the busiest bar has %d notes, want 9", got)
	}
	if got := Busiest(nil); got != 0 {
		t.Errorf("an empty score came out at %d", got)
	}
}
