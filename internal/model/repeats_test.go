/*
 * repeats_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package model

import "testing"

// A repeat belongs to the piece and not to one staff of it.
//
// Files do not say so. MuseScore writes the repeat once, on the first staff,
// and means it for the score; so does anything else that keeps a repeat as a
// property of a bar. Read literally, a conductor's score came out with the
// sign down the top staff and a plain barline down the fifteen below it,
// which tells every player but the flute to carry on.

// band is a score of n staves and bars, with nothing in it but the barlines,
// which are what these tests are about.
func band(staves, bars int) *Score {
	s := &Score{Tempo: Tempo{BPM: 120, Unit: Duration{Num: 1, Den: 4}}}
	for i := 0; i < staves; i++ {
		p := &Part{Clef: Treble, Time: TimeSignature{Beats: 4, BeatValue: 4}}
		for j := 0; j < bars; j++ {
			p.Measures = append(p.Measures, &Measure{
				// A note in every bar: the rule is about the music, not
				// about which bars happen to be empty.
				Voices: []*Voice{{Elements: []Element{&Note{
					Pitch: Pitch{Step: StepG, Octave: 4},
					Dur:   Duration{Num: 1, Den: 1},
				}}}},
				RightBarline: BarSingle,
			})
		}
		s.Parts = append(s.Parts, p)
	}
	return s
}

func barlines(s *Score, bar int) []BarlineStyle {
	out := make([]BarlineStyle, 0, len(s.Parts))
	for _, p := range s.Parts {
		out = append(out, p.Measures[bar].RightBarline)
	}
	return out
}

func allAre(got []BarlineStyle, want BarlineStyle) bool {
	for _, g := range got {
		if g != want {
			return false
		}
	}
	return true
}

func TestARepeatIsWrittenDownEveryStaff(t *testing.T) {
	s := band(5, 8)
	// Written where a file writes it: on the first staff and nowhere else.
	s.Parts[0].Measures[1].RightBarline = BarRepeatStart
	s.Parts[0].Measures[5].RightBarline = BarRepeatEnd
	s.Normalise()

	if got := barlines(s, 1); !allAre(got, BarRepeatStart) {
		t.Errorf("the repeat opens as %v; every staff should open it", got)
	}
	if got := barlines(s, 5); !allAre(got, BarRepeatEnd) {
		t.Errorf("the repeat closes as %v; every staff should close it", got)
	}
	// And nothing else was touched.
	if got := barlines(s, 3); !allAre(got, BarSingle) {
		t.Errorf("bar 4 came out as %v", got)
	}
}

// The bar that ends the piece ends it for everybody.
func TestTheFinalBarlineIsWrittenDownEveryStaff(t *testing.T) {
	s := band(4, 6)
	s.Parts[2].Measures[5].RightBarline = BarFinal
	s.Normalise()

	if got := barlines(s, 5); !allAre(got, BarFinal) {
		t.Errorf("the piece ends as %v; every staff should end it", got)
	}
}

// Where one staff closes a repeat and another opens one at the same barline,
// the barline is both — which is a single sign, not two.
func TestABarThatBothClosesAndOpensIsDrawnAsOne(t *testing.T) {
	s := band(3, 6)
	s.Parts[0].Measures[2].RightBarline = BarRepeatEnd
	s.Parts[1].Measures[2].RightBarline = BarRepeatStart
	s.Normalise()

	if got := barlines(s, 2); !allAre(got, BarRepeatBoth) {
		t.Errorf("the barline came out as %v, want repeat-both on every staff", got)
	}
}

// A double barline is a section's end, and sections do not have to line up:
// the flutes can finish a phrase where the horns are still in one. So this
// rule is about repeats and the end of the piece and stops there.
func TestADoubleBarlineIsLeftWhereItWasWritten(t *testing.T) {
	s := band(3, 6)
	s.Parts[1].Measures[3].RightBarline = BarDouble
	s.Normalise()

	got := barlines(s, 3)
	want := []BarlineStyle{BarSingle, BarDouble, BarSingle}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("staff %d has %v at bar 4, want %v", i, got[i], want[i])
		}
	}
}

// A staff that stops short is padded to the longest, and the repeat has to
// reach the padding too — those bars are drawn, and a barline missing from
// them is a hole in the column.
func TestAShortStaffStillGetsTheRepeat(t *testing.T) {
	s := band(2, 8)
	s.Parts[1].Measures = s.Parts[1].Measures[:4]
	s.Parts[0].Measures[6].RightBarline = BarRepeatEnd
	s.Normalise()

	if len(s.Parts[1].Measures) != 8 {
		t.Fatalf("the short staff was padded to %d bars", len(s.Parts[1].Measures))
	}
	if got := barlines(s, 6); !allAre(got, BarRepeatEnd) {
		t.Errorf("the repeat closes as %v", got)
	}
}
