/*
 * multirest_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * "Z" is a bar's rest, and "Z4" is four of them.
 *
 * It was stepped over as a letter nobody knew. That is the one gap in the
 * reader that lost a whole piece rather than a mark: a tune beginning with
 * a rest had no elements at all by the end of the file, and "no music found
 * in any tune" threw it away. Tune books rarely use Z, which is why reading
 * forty-six books of them showed nothing; parts are full of it.
 */

func TestZIsABarOfRest(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nZ|CDEF|\n")
	if len(s.Parts) != 1 {
		t.Fatalf("%d parts, want 1", len(s.Parts))
	}
	ms := s.Parts[0].Measures
	if len(ms) != 2 {
		t.Fatalf("%d bars, want a bar of rest and a bar of notes", len(ms))
	}
	r, ok := ms[0].Voices[0].Elements[0].(*model.Rest)
	if !ok {
		t.Fatalf("the first bar holds %T, want a rest", ms[0].Voices[0].Elements[0])
	}
	if want := (model.Duration{Num: 1, Den: 1}); r.Dur != want {
		t.Errorf("the rest lasts %v, want %v — a whole bar of 4/4", r.Dur, want)
	}
}

func TestZCountsItsBars(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nZ4|CDEF|\n")
	if n := len(s.Parts[0].Measures); n != 5 {
		t.Fatalf("Z4 gave %d bars, want 4 of rest and 1 of notes", n)
	}
	for i := 0; i < 4; i++ {
		els := s.Parts[0].Measures[i].Voices[0].Elements
		if len(els) != 1 {
			t.Errorf("bar %d holds %d elements, want one rest", i+1, len(els))
			continue
		}
		if _, ok := els[0].(*model.Rest); !ok {
			t.Errorf("bar %d holds %T, want a rest", i+1, els[0])
		}
	}
	if n := len(notesOf(s)); n != 4 {
		t.Errorf("%d notes after the rest, want 4", n)
	}
}

func TestZInThreeFourLastsThreeBeats(t *testing.T) {
	// The bar's length is the meter's, not a whole note's.
	s := importOne(t, "X:1\nT:t\nM:3/4\nL:1/4\nK:C\nZ2|CDE|\n")
	r := s.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Rest)
	if want := (model.Duration{Num: 3, Den: 4}); r.Dur != want {
		t.Errorf("the rest lasts %v, want %v", r.Dur, want)
	}
}

func TestZKeepsTheBarlineWrittenAfterIt(t *testing.T) {
	// The last bar of the run is left open so the written barline closes
	// it: emitting a plain one here would both lose the final barline and
	// leave an empty bar behind it.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|Z2|]\n")
	ms := s.Parts[0].Measures
	if len(ms) != 3 {
		t.Fatalf("%d bars, want notes and two of rest", len(ms))
	}
	if got := ms[2].RightBarline; got != model.BarFinal {
		t.Errorf("the run ends on %v, want the final barline that was written", got)
	}
}
