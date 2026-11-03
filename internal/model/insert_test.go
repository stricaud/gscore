/*
 * insert_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Putting bars into the middle of a piece.
 *
 * The music moving over is the easy half. The half that goes wrong is
 * everything holding a bar by its number — a part's page turns, the bars a
 * cue is taken from, where the cursor was left — each of which ends up
 * pointing a little earlier in the music than it did.
 */

package model

import "testing"

// counted is a score of staves staves whose bars can be told apart: each
// one holds a rest of a length that says which bar it is.
func counted(staves, bars int) *Score {
	s := NewScore()
	s.Parts = nil
	for i := 0; i < staves; i++ {
		p := &Part{Clef: Treble, Time: TimeSignature{Beats: 4, BeatValue: 4}}
		for b := 0; b < bars; b++ {
			m := NewMeasure()
			m.Voices[0].Elements = []Element{
				&Rest{Dur: Duration{Num: b + 1, Den: 16}},
			}
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	return s
}

// barOrder is which bar each one started as, by the rest in it. Zero for a
// bar that was put in.
func barOrder(p *Part) []int {
	var out []int
	for _, m := range p.Measures {
		n := 0
		if len(m.Voices) > 0 && len(m.Voices[0].Elements) > 0 {
			if r, ok := m.Voices[0].Elements[0].(*Rest); ok {
				n = r.Dur.Num
			}
		}
		out = append(out, n)
	}
	return out
}

func same(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBarsGoInAfterTheOneChosen(t *testing.T) {
	s := counted(1, 4)
	if n := s.InsertMeasures(1, 2, -1); n != 2 {
		t.Fatalf("put in %d bars", n)
	}
	// 1, 2, then two new ones, then 3, 4.
	if got := barOrder(s.Parts[0]); !same(got, []int{1, 2, 0, 0, 3, 4}) {
		t.Errorf("the bars read %v", got)
	}
}

func TestBarsCanGoInAtTheFront(t *testing.T) {
	s := counted(1, 3)
	s.InsertMeasures(-1, 1, -1)
	if got := barOrder(s.Parts[0]); !same(got, []int{0, 1, 2, 3}) {
		t.Errorf("the bars read %v", got)
	}
}

func TestBarsCanGoInAtTheEnd(t *testing.T) {
	s := counted(1, 3)
	s.InsertMeasures(2, 2, -1)
	if got := barOrder(s.Parts[0]); !same(got, []int{1, 2, 3, 0, 0}) {
		t.Errorf("the bars read %v", got)
	}
}

// A bar is a bar across the score: the barlines have to stay in a column.
func TestEveryStaffGrowsTogether(t *testing.T) {
	s := counted(3, 4)
	s.InsertMeasures(1, 2, -1)
	for i, p := range s.Parts {
		if got := barOrder(p); !same(got, []int{1, 2, 0, 0, 3, 4}) {
			t.Errorf("staff %d reads %v", i+1, got)
		}
	}
}

func TestNothingHappensForNonsense(t *testing.T) {
	s := counted(2, 3)
	for _, tc := range []struct{ after, count int }{
		{1, 0}, {1, -1}, {-2, 1}, {3, 1}, {99, 1},
	} {
		if n := s.InsertMeasures(tc.after, tc.count, -1); n != 0 {
			t.Errorf("after=%d count=%d put in %d bars", tc.after, tc.count, n)
		}
	}
	if s.MeasureCount() != 3 {
		t.Errorf("the score is now %d bars", s.MeasureCount())
	}
}

/*
 * One staff only.
 */

// The case this is for: a line typed a bar short against the rest.
func TestOneStaffCanGrowOnItsOwn(t *testing.T) {
	s := counted(3, 4)
	s.InsertMeasures(1, 2, 1)

	if got := barOrder(s.Parts[1]); !same(got, []int{1, 2, 0, 0, 3, 4}) {
		t.Errorf("the staff that grew reads %v", got)
	}
	// The others keep their music where it was and are made up at the
	// end, so every staff is the same length.
	for _, i := range []int{0, 2} {
		if got := barOrder(s.Parts[i]); !same(got, []int{1, 2, 3, 4, 0, 0}) {
			t.Errorf("staff %d reads %v", i+1, got)
		}
	}
	for i, p := range s.Parts {
		if len(p.Measures) != 6 {
			t.Errorf("staff %d has %d bars", i+1, len(p.Measures))
		}
	}
}

/*
 * The barline at the end of the run.
 */

// A repeat closing bar four, with two bars pushed in after it, is a repeat
// of six bars — not a repeat of four with two left outside it.
func TestTheBarlineFollowsTheEndOfTheRun(t *testing.T) {
	s := counted(1, 4)
	s.Parts[0].Measures[1].RightBarline = BarRepeatEnd
	s.InsertMeasures(1, 2, -1)

	if got := s.Parts[0].Measures[1].RightBarline; got != BarSingle {
		t.Errorf("the bar it was on still closes with %v", got)
	}
	if got := s.Parts[0].Measures[3].RightBarline; got != BarRepeatEnd {
		t.Errorf("the last new bar closes with %v, want the repeat", got)
	}
}

// And a plain barline stays plain: nothing is moved that was not a mark.
func TestAPlainBarlineIsLeftAlone(t *testing.T) {
	s := counted(1, 4)
	s.InsertMeasures(1, 1, -1)
	for i, m := range s.Parts[0].Measures {
		if m.RightBarline != BarSingle {
			t.Errorf("bar %d closes with %v", i+1, m.RightBarline)
		}
	}
}

/*
 * Everything that names a bar by its number.
 */

func TestAPartsPageTurnsMoveWithTheMusic(t *testing.T) {
	s := counted(2, 8)
	s.Sheets = []Sheet{{Name: "Flute", Staves: []int{0}, Breaks: []int{1, 4, 6}}}
	s.InsertMeasures(2, 2, -1)

	// The break at bar 1 is before the insert and stays; the others move.
	want := []int{1, 6, 8}
	for i, b := range s.Sheets[0].Breaks {
		if b != want[i] {
			t.Errorf("the breaks are %v, want %v", s.Sheets[0].Breaks, want)
			break
		}
	}
}

func TestACueFollowsTheBarsItIsTakenFrom(t *testing.T) {
	s := counted(2, 8)
	s.Sheets = []Sheet{{Name: "Flute", Staves: []int{0}, Cues: []Cue{
		{At: 1, Bars: 2, From: 1},
		{At: 5, Bars: 2, From: 1},
	}}}
	s.InsertMeasures(2, 3, -1)

	if got := s.Sheets[0].Cues[0].At; got != 1 {
		t.Errorf("the cue before the insert moved to bar %d", got+1)
	}
	if got := s.Sheets[0].Cues[1].At; got != 8 {
		t.Errorf("the cue after it is at bar %d, want 9", got+1)
	}
}

func TestTheCursorStaysWhereTheMusicWent(t *testing.T) {
	s := counted(2, 8)
	s.Cursor = &Cursor{Part: 0, Measure: 5, At: 1}
	s.InsertMeasures(2, 2, -1)
	if s.Cursor.Measure != 7 {
		t.Errorf("the cursor is at bar %d, want 8", s.Cursor.Measure+1)
	}

	// And a cursor before the insert does not move.
	s.Cursor = &Cursor{Part: 0, Measure: 1}
	s.InsertMeasures(4, 2, -1)
	if s.Cursor.Measure != 1 {
		t.Errorf("the cursor moved to bar %d", s.Cursor.Measure+1)
	}
}

// Where only one staff grew, only the things pointing at that staff move.
func TestOnlyWhatFollowsThatStaffMoves(t *testing.T) {
	s := counted(3, 8)
	s.Sheets = []Sheet{
		{Name: "One", Staves: []int{0}, Breaks: []int{4}},
		{Name: "Two", Staves: []int{1}, Breaks: []int{4}},
	}
	s.Cursor = &Cursor{Part: 2, Measure: 5}
	s.InsertMeasures(1, 2, 1)

	if got := s.Sheets[0].Breaks[0]; got != 4 {
		t.Errorf("a part printing another staff had its break moved to %d", got)
	}
	if got := s.Sheets[1].Breaks[0]; got != 6 {
		t.Errorf("the part printing the staff that grew has its break at %d", got)
	}
	// The cursor is on a staff that did not grow.
	if s.Cursor.Measure != 5 {
		t.Errorf("the cursor moved to bar %d", s.Cursor.Measure+1)
	}
}
