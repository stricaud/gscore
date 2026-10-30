/*
 * parsepitch_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading a pitch written the way a musician types one.
 *
 * The point of it is typing: somebody entering a tune from paper writes
 * "c4 d e f g" and means a scale, not five middle Cs and four guesses. So
 * the octave may be left out, and what it then means is a question for
 * whatever came before — which only the caller knows.
 */

package model

import "testing"

func TestAPitchIsReadTheWayItIsTyped(t *testing.T) {
	for _, c := range []struct {
		in    string
		step  Step
		alter int
		oct   int
	}{
		{"c4", StepC, 0, 4},
		{"C4", StepC, 0, 4},
		{"f#3", StepF, 1, 3},
		{"fs3", StepF, 1, 3},
		{"bb5", StepB, -1, 5},
		{"ebb1", StepE, -2, 1},
		{"gx2", StepG, 2, 2},
		{"g##2", StepG, 2, 2},
		{"a-1", StepA, 0, -1},
	} {
		p, hasOct, ok := ParsePitch(c.in)
		if !ok {
			t.Errorf("%q was not read at all", c.in)
			continue
		}
		if !hasOct {
			t.Errorf("%q came back without its octave", c.in)
		}
		if p.Step != c.step || p.Alter != c.alter || p.Octave != c.oct {
			t.Errorf("%q came out as %+v, want step %v alter %d octave %d",
				c.in, p, c.step, c.alter, c.oct)
		}
	}
}

// A B flat is written "bb", and the second b is the flat rather than the
// start of an octave: an octave is a digit.
func TestBFlatIsNotBThenB(t *testing.T) {
	p, _, ok := ParsePitch("bb3")
	if !ok || p.Step != StepB || p.Alter != -1 || p.Octave != 3 {
		t.Errorf("bb3 came out as %+v", p)
	}
}

func TestAPitchWithNoOctaveSaysSo(t *testing.T) {
	p, hasOct, ok := ParsePitch("f#")
	if !ok {
		t.Fatal("a pitch with no octave was refused")
	}
	if hasOct {
		t.Error("it claimed an octave it was not given")
	}
	if p.Step != StepF || p.Alter != 1 {
		t.Errorf("it came out as %+v", p)
	}
}

func TestWhatIsNotAPitchIsRefused(t *testing.T) {
	for _, s := range []string{"", "h4", "4", "c#x", "c4x", "cb12", "-", "c-"} {
		if _, _, ok := ParsePitch(s); ok {
			t.Errorf("%q was read as a pitch", s)
		}
	}
}

// A run of names with no octaves climbs and falls by the shortest way,
// which is what "c d e f g" means and what "c b" means.
func TestANamelessOctaveTakesTheShortestWay(t *testing.T) {
	c4 := Pitch{Step: StepC, Octave: 4}
	b, _, _ := ParsePitch("b")
	if got := b.NearestTo(c4); got.Octave != 3 {
		t.Errorf("c then b went to %v, want the B below at octave 3", got)
	}
	d, _, _ := ParsePitch("d")
	if got := d.NearestTo(c4); got.Octave != 4 {
		t.Errorf("c then d went to %v, want the D above", got)
	}
	// A tritone is the same distance either way; up is the tie-break, as
	// a scale climbs more often than it falls.
	f, _, _ := ParsePitch("f#")
	if got := f.NearestTo(c4); got.Octave != 4 {
		t.Errorf("c then f sharp went to %v", got)
	}
}

// The notes of a chord are not a run: they are read from the bottom up, so
// each stands above the one before it however far that is.
//
// "[ca]" is a sixth. The nearest A to a C is the one below it, which is what
// the run's rule would have picked and what a chord never means.
func TestAChordsNotesStackUpwards(t *testing.T) {
	c4 := Pitch{Step: StepC, Octave: 4}
	a, _, _ := ParsePitch("a")
	got := a.JustAbove(c4)
	if got.Octave != 4 || got.Step != StepA {
		t.Errorf("c then a went to %v, want the A a sixth above", got)
	}

	// The same letter is an octave, there being nowhere nearer above.
	same, _, _ := ParsePitch("c")
	if got := same.JustAbove(c4); got.Octave != 5 {
		t.Errorf("c over c went to %v, want the octave", got)
	}

	// Above on the staff, by notehead: a flat on it does not drop it under
	// the notehead below, so C with a D flat over it is the cluster it was
	// written as and not a seventh.
	db, _, _ := ParsePitch("db")
	if got := db.JustAbove(c4); got.Octave != 4 {
		t.Errorf("c then d flat went to %v, want the semitone above", got)
	}

	// And it reaches the octave it has to from wherever it starts: a pitch
	// written low is lifted, not left where it was.
	low := Pitch{Step: StepE, Octave: 1}
	if got := low.JustAbove(c4); got.Octave != 4 {
		t.Errorf("an E from octave 1 over c4 went to %v, want octave 4", got)
	}
}
