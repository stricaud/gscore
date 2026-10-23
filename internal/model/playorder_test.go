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

/*
 * Jumps: D.C., D.S., Fine and the coda
 */

// 1 2 3 4 D.C. — back to the head and out.
func TestPlayOrderDaCapo(t *testing.T) {
	s := bars(4)
	s.bar(3).Jump = &Jump{FromStart, UntilEnd}
	if got, want := order(s), "1 2 3 4 1 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 1 2 Fine 3 4 D.C. al Fine — the last bar written is not the last played.
func TestPlayOrderDaCapoAlFine(t *testing.T) {
	s := bars(4)
	s.bar(1).Sign = Fine
	s.bar(3).Jump = &Jump{FromStart, UntilFine}
	if got, want := order(s), "1 2 3 4 1 2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Fine does nothing on the way down: it is only the direction that makes it
// mean anything, which is why a piece with a Fine and no D.C. plays through.
func TestFineWithoutADirectionDoesNothing(t *testing.T) {
	s := bars(4)
	s.bar(1).Sign = Fine
	if got, want := order(s), "1 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 1 §2 3 4 D.S. — back to the sign.
func TestPlayOrderDalSegno(t *testing.T) {
	s := bars(4)
	s.bar(1).Sign = Segno
	s.bar(3).Jump = &Jump{FromSegno, UntilEnd}
	if got, want := order(s), "1 2 3 4 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 1 §2 3[To Coda] 4 D.S. al Coda ... 5[Coda] 6
func TestPlayOrderDalSegnoAlCoda(t *testing.T) {
	s := bars(6)
	s.bar(1).Sign = Segno
	s.bar(2).Sign = ToCoda
	s.bar(3).Jump = &Jump{FromSegno, UntilCoda}
	s.bar(4).Sign = Coda
	if got, want := order(s), "1 2 3 4 2 3 5 6"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// To Coda is passed straight over on the way down. A chart that jumped to the
// coda the first time through would never play the bars in between.
func TestToCodaIsIgnoredOnTheWayDown(t *testing.T) {
	s := bars(5)
	s.bar(1).Sign = ToCoda
	s.bar(3).Sign = Coda
	if got, want := order(s), "1 2 3 4 5"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A D.S. is taken once. Taking it every time round is how a chart that ends
// "D.S. al Coda" would play for ever.
func TestAJumpIsTakenOnlyOnce(t *testing.T) {
	s := bars(3)
	s.bar(2).Jump = &Jump{FromStart, UntilEnd}
	if got, want := order(s), "1 2 3 1 2 3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// On the way through again the written repeats are not observed. That is what
// D.C. means to a player in every chart that uses it.
func TestRepeatsAreNotTakenAfterAJump(t *testing.T) {
	s := bars(4)
	s.bar(1).RightBarline = BarRepeatEnd
	s.bar(3).Jump = &Jump{FromStart, UntilEnd}
	if got, want := order(s), "1 2 1 2 3 4 1 2 3 4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A section marked 1. and 2. and reached a third time, which is what a D.C.
// does, goes through the second ending and not the first.
//
//	1 |: 2 3 [1. 4 :][2. 5 ] 6 D.C.
func TestAfterAJumpTheLastEndingIsTaken(t *testing.T) {
	s := bars(6)
	s.bar(0).RightBarline = BarRepeatStart
	s.bar(3).Ending = &Ending{Numbers: []int{1}, Start: true, Stop: true}
	s.bar(3).RightBarline = BarRepeatEnd
	s.bar(4).Ending = &Ending{Numbers: []int{2}, Start: true, Stop: true}
	s.bar(5).Jump = &Jump{FromStart, UntilEnd}

	// Down: round twice, through the first ending and then the second.
	// Back: once through, taking the second ending, because the repeats are
	// not observed again.
	if got, want := order(s), "1 2 3 4 2 3 5 6 1 2 3 5 6"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A D.S. in a score with no sign in it goes back to the beginning. The score
// is wrong either way; playing it from the top is the reading that leaves the
// music audible.
func TestDalSegnoWithNoSignGoesToTheStart(t *testing.T) {
	s := bars(3)
	s.bar(2).Jump = &Jump{FromSegno, UntilEnd}
	if got, want := order(s), "1 2 3 1 2 3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Likewise a direction al Coda with no coda written: it plays to the end
// rather than stopping dead.
func TestAlCodaWithNoCodaPlaysOn(t *testing.T) {
	s := bars(3)
	s.bar(1).Sign = ToCoda
	s.bar(2).Jump = &Jump{FromStart, UntilCoda}
	if got, want := order(s), "1 2 3 1 2 3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The directions belong to the piece, so they are read from whichever staff
// carries them: a conductor's score has them over the top line only.
func TestJumpsAreReadFromAnyStaff(t *testing.T) {
	s := bars(4)
	s.Parts = append(s.Parts, &Part{Measures: []*Measure{
		{Voices: []*Voice{{}}}, {Voices: []*Voice{{}}},
		{Voices: []*Voice{{}}}, {Voices: []*Voice{{}}},
	}})
	s.Parts[1].Measures[1].Sign = Fine
	s.Parts[1].Measures[3].Jump = &Jump{FromStart, UntilFine}
	if got, want := order(s), "1 2 3 4 1 2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJumpNamesRoundTrip(t *testing.T) {
	for _, name := range []string{"dc", "dc-fine", "dc-coda", "ds", "ds-fine", "ds-coda"} {
		j, ok := JumpFromName(name)
		if !ok || j == nil {
			t.Fatalf("%q was not read as a jump", name)
		}
		if got := j.Name(); got != name {
			t.Errorf("%q came back as %q", name, got)
		}
		if j.Label() == "" {
			t.Errorf("%q has nothing to print", name)
		}
	}
	if _, ok := JumpFromName("nonesuch"); ok {
		t.Error("a name that is not a jump was accepted")
	}
	if j, ok := JumpFromName("none"); !ok || j != nil {
		t.Error(`"none" should be no jump at all`)
	}
}

func TestSignNamesRoundTrip(t *testing.T) {
	for _, want := range []Sign{Segno, Coda, Fine, ToCoda} {
		got, ok := SignFromName(want.String())
		if !ok || got != want {
			t.Errorf("%v came back as %v", want, got)
		}
	}
	if _, ok := SignFromName("nonesuch"); ok {
		t.Error("a name that is not a sign was accepted")
	}
}
