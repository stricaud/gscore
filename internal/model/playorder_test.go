package model

import (
	"fmt"
	"strings"
	"testing"
)

// bars builds a score of n empty bars on one staff, so the structure can be
// described by its barlines alone.
func bars(n int) *Score {
	s := NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < n; i++ {
		p.Measures = append(p.Measures, &Measure{Voices: []*Voice{{}}})
	}
	return s
}

func (s *Score) bar(i int) *Measure { return s.Parts[0].Measures[i] }

func order(s *Score) string {
	var b strings.Builder
	for i, n := range s.PlayOrder() {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d", n+1) // bar numbers, as a player counts them
	}
	return b.String()
}

func TestPlayOrderStraightThrough(t *testing.T) {
	s := bars(4)
	if got, want := order(s), "1 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// |: 1 2 3 4 :|  — round twice, then out.
func TestPlayOrderSimpleRepeat(t *testing.T) {
	s := bars(5)
	s.bar(3).RightBarline = BarRepeatEnd
	if got, want := order(s), "1 2 3 4 1 2 3 4 5"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 1 |: 2 3 :| 4 — the repeat opens part way in.
func TestPlayOrderRepeatFromAMark(t *testing.T) {
	s := bars(4)
	s.bar(0).RightBarline = BarRepeatStart
	s.bar(2).RightBarline = BarRepeatEnd
	if got, want := order(s), "1 2 3 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// |: 1 2 [1. 3 :][2. 4 ] 5 — first and second time bars.
func TestPlayOrderWithEndings(t *testing.T) {
	s := bars(5)
	s.bar(2).Ending = &Ending{Numbers: []int{1}, Start: true, Stop: true}
	s.bar(2).RightBarline = BarRepeatEnd
	s.bar(3).Ending = &Ending{Numbers: []int{2}, Start: true, Stop: true, Open: true}
	if got, want := order(s), "1 2 3 1 2 4 5"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A repeat that sends you back to a bar that sends you back must still stop.
func TestPlayOrderCannotRunForever(t *testing.T) {
	s := bars(3)
	s.bar(0).RightBarline = BarRepeatStart
	s.bar(1).RightBarline = BarRepeatEnd
	s.bar(2).RightBarline = BarRepeatEnd
	got := s.PlayOrder()
	if len(got) == 0 {
		t.Fatal("nothing was played")
	}
	if len(got) >= maxPlayedBars {
		t.Errorf("it ran to the limit (%d bars), so it was not going to stop", len(got))
	}
	t.Logf("played %d bars: %s", len(got), order(s))
}

// Every bar of a score with no repeats is played exactly once.
func TestPlayOrderCoversEveryBar(t *testing.T) {
	s := bars(12)
	got := s.PlayOrder()
	if len(got) != 12 {
		t.Fatalf("played %d of 12 bars", len(got))
	}
	for i, n := range got {
		if n != i {
			t.Errorf("position %d is bar %d", i, n)
		}
	}
}
