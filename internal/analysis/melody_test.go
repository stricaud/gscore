/*
 * melody_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a line is made of.
 *
 * A line that is nearly all steps is a tune somebody can hum after one
 * hearing; a line of leaps is a fanfare or an arpeggio. Neither is better,
 * and a composer writing a vocal line and getting the second is usually not
 * doing it on purpose. All of it is a description and not a mark —
 * Stravinsky's lines leap and are not improvable.
 */

package analysis

import "testing"

// walk turns a run of pitches into the moves between them. letters says how
// many letter names apart each move is written, which only matters for
// telling an augmented second from a minor third.
func walk(pitches []int, letters ...int) []Step {
	var out []Step
	for i := 0; i+1 < len(pitches); i++ {
		l := 0
		if i < len(letters) {
			l = letters[i]
		}
		out = append(out, Step{
			Line: 0, Bar: i / 4, From: pitches[i], To: pitches[i+1],
			Ref: i + 1, Letters: l,
		})
	}
	return out
}

func shapeOf(pitches []int) Shape {
	return Shapes(walk(pitches), []string{"soprano"})[0]
}

func TestALineOfStepsIsMostlySteps(t *testing.T) {
	s := shapeOf([]int{60, 62, 64, 65, 67, 65, 64, 62, 60})
	if s.Leaps != 0 {
		t.Errorf("%d leaps in a scale", s.Leaps)
	}
	if s.Steps != 8 {
		t.Errorf("%d steps, want 8", s.Steps)
	}
	if !s.Stepwise() {
		t.Error("a scale is not counted as singable")
	}
}

func TestALineOfLeapsIsNot(t *testing.T) {
	s := shapeOf([]int{60, 67, 60, 72, 60, 76})
	if s.Steps != 0 {
		t.Errorf("%d steps in a line of fifths and octaves", s.Steps)
	}
	if s.Stepwise() {
		t.Error("a fanfare is counted as singable")
	}
}

func TestARepeatedNoteIsNeitherAStepNorALeap(t *testing.T) {
	s := shapeOf([]int{60, 60, 60, 62})
	if s.Same != 2 {
		t.Errorf("%d repeats, want 2", s.Same)
	}
	if s.Steps != 1 || s.Leaps != 0 {
		t.Errorf("the one move came out as %d steps and %d leaps", s.Steps, s.Leaps)
	}
	// A line of repeated notes is as easy to sing as a line of steps.
	if !s.Stepwise() {
		t.Error("a line of repeated notes is not counted as singable")
	}
}

func TestTheWidestMoveIsFoundAndPlaced(t *testing.T) {
	s := shapeOf([]int{60, 62, 64, 60, 72, 71})
	if s.Widest != 12 {
		t.Errorf("the widest move is %d semitones, want the octave at 12", s.Widest)
	}
	if s.WidestBar != 0 {
		t.Errorf("it is said to be in bar %d", s.WidestBar)
	}
}

func TestWhichWayTheLineGoesIsCounted(t *testing.T) {
	s := shapeOf([]int{60, 62, 64, 65, 64})
	if s.Up != 3 || s.Down != 1 {
		t.Errorf("the line goes up %d times and down %d, want 3 and 1", s.Up, s.Down)
	}
}

func TestTheCommonestIntervalIsReported(t *testing.T) {
	// Mostly tones, with one third.
	s := shapeOf([]int{60, 62, 64, 66, 68, 71})
	if s.Commonest != 2 {
		t.Errorf("the commonest move is %d semitones, want the tone at 2", s.Commonest)
	}
}

func TestALineWithNoMovesIsNotCalledSingable(t *testing.T) {
	s := Shapes(nil, []string{"alto"})[0]
	if s.Stepwise() {
		t.Error("a line with nothing in it was called singable")
	}
	if s.Notes != 0 {
		t.Errorf("it has %d notes", s.Notes)
	}
}

// A leap the line does not turn back from leaves the ear a long way from
// where it started with nothing to hold on to.
func TestALeapThatKeepsGoingIsReported(t *testing.T) {
	got := Awkwardness(walk([]int{60, 69, 71}), []string{"soprano"}, pitchNumber)
	if len(got) != 1 || got[0].Kind != "leap not turned" {
		t.Fatalf("a sixth that carried on was not reported: %+v", got)
	}
	if got[0].Ref == 0 || got[0].Name != "soprano" {
		t.Errorf("it does not say where or in which line: %+v", got[0])
	}
}

func TestALeapThatTurnsBackIsLeftAlone(t *testing.T) {
	got := Awkwardness(walk([]int{60, 69, 67}), []string{"soprano"}, pitchNumber)
	if len(got) != 0 {
		t.Errorf("a sixth that turned back was reported: %+v", got)
	}
}

// A leap at the very end has nowhere to turn, and complaining about it
// would be complaining that the piece stopped.
func TestALeapAtTheEndHasNowhereToTurn(t *testing.T) {
	got := Awkwardness(walk([]int{60, 69}), []string{"soprano"}, pitchNumber)
	if len(got) != 0 {
		t.Errorf("the last leap in a line was reported: %+v", got)
	}
}

// A fifth is not a big leap. The line has to be left room to move.
func TestAFifthIsNotABigLeap(t *testing.T) {
	got := Awkwardness(walk([]int{60, 67, 69}), []string{"soprano"}, pitchNumber)
	if len(got) != 0 {
		t.Errorf("a fifth was called a big leap: %+v", got)
	}
}

// An augmented second and a minor third are the same sound, and only the
// spelling tells them apart: A to C is a third anybody sings, and A to B
// sharp is the step singers have complained about for four hundred years.
func TestAnAugmentedSecondIsToldFromAMinorThird(t *testing.T) {
	// Three semitones written as a second.
	got := Awkwardness(walk([]int{65, 68}, 1), []string{"alto"}, pitchNumber)
	if len(got) != 1 || got[0].Kind != "augmented second" {
		t.Fatalf("an augmented second was not seen: %+v", got)
	}
	// The same sound written as a third.
	got = Awkwardness(walk([]int{65, 68}, 2), []string{"alto"}, pitchNumber)
	if len(got) != 0 {
		t.Errorf("a minor third was called an augmented second: %+v", got)
	}
}

func TestEveryFindingSaysSomething(t *testing.T) {
	got := Awkwardness(walk([]int{60, 69, 71}), []string{"soprano"}, pitchNumber)
	for i, a := range got {
		if a.Text == "" {
			t.Errorf("finding %d says nothing", i)
		}
	}
}
