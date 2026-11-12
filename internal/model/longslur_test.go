/*
 * longslur_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Slurs that reach further than a slur reaches.
 *
 * Asked for by name, after three of them came out of one file in an
 * afternoon: "we should have a harness for long slurs; it could happen and
 * be correct, but most of the time this is an import mistake."
 */

package model

import "testing"

// phrased is one staff of so many bars of quarter notes, with a slur opened
// at one bar and closed at another — or never, where to is past the end.
func phrased(bars, from, to int) *Score {
	s := NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < bars; i++ {
		var els []Element
		for j := 0; j < 4; j++ {
			n := &Note{
				Pitch: Pitch{Step: StepC, Octave: 5},
				Dur:   Duration{Num: 1, Den: 4},
			}
			if j == 0 && i == from {
				n.SlurStart = 1
			}
			if j == 3 && i == to {
				n.SlurStop = 1
			}
			els = append(els, n)
		}
		p.Measures = append(p.Measures, &Measure{
			Voices: []*Voice{{Elements: els}}, RightBarline: BarSingle,
		})
	}
	return s
}

func TestAPhraseOfAFewBarsIsNotReported(t *testing.T) {
	for _, bars := range []int{1, 2, 3} {
		if got := LongSlurs(phrased(8, 1, bars), 3); len(got) != 0 {
			t.Errorf("a slur over %d bars was reported: %v", bars, got)
		}
	}
}

func TestASlurThatReachesTooFarIsReportedWithItsPlace(t *testing.T) {
	got := LongSlurs(phrased(8, 1, 5), 3)
	if len(got) != 1 {
		t.Fatalf("%d slurs reported, want the one: %v", len(got), got)
	}
	l := got[0]
	if l.Measure != 1 || l.Bars != 5 || l.Open {
		t.Errorf("reported as %+v, want bar 2 (index 1), 5 bars", l)
	}
	if s := l.String(); s != "staff 1 bar 2, 5 bars" {
		t.Errorf("said as %q", s)
	}
}

func TestASlurThatNeverClosesIsReported(t *testing.T) {
	// The commonest shape of the fault: the far end was lost, so the
	// curve runs to the end of the piece.
	got := LongSlurs(phrased(8, 6, 99), 3)
	if len(got) != 1 || !got[0].Open || got[0].Measure != 6 {
		t.Fatalf("an open slur was reported as %v", got)
	}
	if s := got[0].String(); s != "staff 1 bar 7, never closed" {
		t.Errorf("said as %q", s)
	}
}

func TestVoicesAreCountedApart(t *testing.T) {
	// An opening in one voice and a closing in the other is exactly the
	// fault a divided staff produces, and it is reported as what is drawn:
	// the opening never closes.
	s := phrased(4, 0, 99)
	m := s.Parts[0].Measures[3]
	closer := &Note{Pitch: Pitch{Step: StepC, Octave: 5}, Dur: Duration{Num: 1, Den: 1}}
	closer.SlurStop = 1
	m.Voices = append(m.Voices, &Voice{Elements: []Element{closer}})
	got := LongSlurs(s, 3)
	if len(got) != 1 || !got[0].Open || got[0].Voice != 0 {
		t.Errorf("the torn slur was reported as %v, want one open slur in voice 1", got)
	}
}

func TestNothingIsReportedOfNothing(t *testing.T) {
	if got := LongSlurs(nil, 3); got != nil {
		t.Errorf("a nil score reported %v", got)
	}
	if got := LongSlurs(NewScore(), 3); len(got) != 0 {
		t.Errorf("an empty score reported %v", got)
	}
}
