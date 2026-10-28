/*
 * reach_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How far a part reaches.
 *
 * The two answers are different kinds of answer and the tests keep them
 * apart: a note the instrument does not have is a fact, and a stretch it
 * would rather not hold is a judgement. Running them together would make the
 * facts arguable and the judgements binding, and both of those are worse
 * than saying nothing.
 */

package analysis

import "testing"

// A horn in F, as the table has it: B1 to F5, comfortable G2 to C5.
var horn = Instrument{
	Name: "french horn", Known: true,
	Low: 35, High: 77, EasyLow: 43, EasyHigh: 72,
}

// names a pitch the way the tests read: the number, so a failure says which.
func number(midi int) string { return itoa(midi) }

// line builds a part of one note per bar, from the first bar given.
func line(part, fromBar int, pitches ...int) []Sounded {
	var out []Sounded
	for i, p := range pitches {
		out = append(out, Sounded{
			Part: part, MIDI: p, Bar: fromBar + i, Ref: 100 + i,
			Held: 1.0 / 4,
		})
	}
	return out
}

func only(t *testing.T, got []PartReach) PartReach {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("%d staves reported, want 1", len(got))
	}
	return got[0]
}

func TestThePartsTwoEndsAreFoundAndPlaced(t *testing.T) {
	r := only(t, Reaches(line(0, 3, 60, 72, 48, 65), []Instrument{horn}, number))

	if r.Lowest.MIDI != 48 || r.Lowest.Bar != 5 {
		t.Errorf("the lowest note is %d in bar %d, want 48 in bar 5",
			r.Lowest.MIDI, r.Lowest.Bar)
	}
	if r.Highest.MIDI != 72 || r.Highest.Bar != 4 {
		t.Errorf("the highest note is %d in bar %d, want 72 in bar 4",
			r.Highest.MIDI, r.Highest.Bar)
	}
	if r.Lowest.Ref == 0 || r.Highest.Ref == 0 {
		t.Error("neither end can be pointed at on the page")
	}
	if r.Notes != 4 {
		t.Errorf("%d notes counted, want 4", r.Notes)
	}
}

// Where a part lives is about time, not about the notes it touches. One top
// note held for an instant does not make a part a high part.
func TestTheTessituraIgnoresWhatIsOnlyTouched(t *testing.T) {
	var notes []Sounded
	for i := 0; i < 40; i++ {
		notes = append(notes, Sounded{Part: 0, MIDI: 60, Bar: i, Held: 1})
	}
	notes = append(notes, Sounded{Part: 0, MIDI: 77, Bar: 40, Held: 1.0 / 64})

	r := only(t, Reaches(notes, []Instrument{horn}, number))
	if r.Highest.MIDI != 77 {
		t.Errorf("the top note is %d: it is still the top note", r.Highest.MIDI)
	}
	if r.Tessitura.High != 60 {
		t.Errorf("the part sits up to %d, and it spends all its time on 60",
			r.Tessitura.High)
	}
}

// A note the instrument does not have is a fact, and says where it is.
func TestANoteAboveTheInstrumentIsReportedAsImpossible(t *testing.T) {
	r := only(t, Reaches(line(0, 10, 60, 62, 84, 64), []Instrument{horn}, number))

	if len(r.Findings) != 1 {
		t.Fatalf("%d findings, want the one about the note that cannot be played: %+v",
			len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Severity != "impossible" {
		t.Errorf("the finding is %q, want it stated as a fact", f.Severity)
	}
	if f.Worst.MIDI != 84 || f.Worst.Bar != 12 {
		t.Errorf("it points at %d in bar %d, want 84 in bar 12", f.Worst.MIDI, f.Worst.Bar)
	}
	if f.Notes != 1 {
		t.Errorf("it is about %d notes, want 1", f.Notes)
	}
}

func TestANoteBelowTheInstrumentIsReportedToo(t *testing.T) {
	r := only(t, Reaches(line(0, 0, 60, 30), []Instrument{horn}, number))
	if len(r.Findings) != 1 || r.Findings[0].Worst.MIDI != 30 {
		t.Fatalf("the note under the bottom of the horn was not reported: %+v", r.Findings)
	}
}

// One bar at the top is a climax, and saying so would be noise.
func TestOneBarHighIsNotAComplaint(t *testing.T) {
	r := only(t, Reaches(line(0, 0, 60, 60, 76, 60, 60), []Instrument{horn}, number))
	for _, f := range r.Findings {
		t.Errorf("a single high bar was reported: %q", f.Text)
	}
}

// Four in a row is a tessitura, which is the thing that tires a player out.
func TestFourBarsHighIsWorthSaying(t *testing.T) {
	r := only(t, Reaches(line(0, 7, 60, 74, 75, 76, 74, 60), []Instrument{horn}, number))

	if len(r.Findings) != 1 {
		t.Fatalf("%d findings, want the one about the four high bars: %+v",
			len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Severity != "strain" {
		t.Errorf("the finding is %q, want it offered rather than ruled", f.Severity)
	}
	if f.FromBar != 8 || f.ToBar != 11 {
		t.Errorf("it covers bars %d to %d, want 8 to 11", f.FromBar, f.ToBar)
	}
	if f.Worst.MIDI != 76 {
		t.Errorf("it points at %d, want the highest of them at 76", f.Worst.MIDI)
	}
}

// A bar that goes up and comes back down is a phrase, not a tessitura.
func TestABarThatComesBackDownBreaksTheRun(t *testing.T) {
	notes := line(0, 0, 74, 75, 76, 74)
	// A second, low note in the middle bar: that bar is no longer all high.
	notes = append(notes, Sounded{Part: 0, MIDI: 55, Bar: 2, Ref: 9, Held: 1.0 / 4})

	r := only(t, Reaches(notes, []Instrument{horn}, number))
	for _, f := range r.Findings {
		t.Errorf("a run broken by a bar that came down was still reported: %q", f.Text)
	}
}

// A part that rests for a bar has not come down, and counting the rest as a
// break would split the one thing worth reporting into two stretches too
// short to report.
func TestARestInTheMiddleDoesNotBreakTheRun(t *testing.T) {
	notes := append(line(0, 0, 74, 75), line(0, 3, 76, 74)...)
	r := only(t, Reaches(notes, []Instrument{horn}, number))
	if len(r.Findings) != 1 {
		t.Fatalf("%d findings over four high bars with a rest among them: %+v",
			len(r.Findings), r.Findings)
	}
	if f := r.Findings[0]; f.FromBar != 0 || f.ToBar != 4 {
		t.Errorf("the stretch is bars %d to %d, want 0 to 4", f.FromBar, f.ToBar)
	}
}

// An instrument gscore does not know gets the range the part covers and no
// opinion about it. A guessed range is worse than a blank: a composer who
// cannot tell which is which cannot use either.
func TestAnUnknownInstrumentIsNotJudged(t *testing.T) {
	r := only(t, Reaches(line(0, 0, 20, 120), []Instrument{{Name: "theremin"}}, number))
	if r.Playable != nil || r.Easy != nil {
		t.Error("a range was offered for an instrument gscore does not know")
	}
	if len(r.Findings) != 0 {
		t.Errorf("it was judged anyway: %+v", r.Findings)
	}
	if r.Lowest.MIDI != 20 || r.Highest.MIDI != 120 {
		t.Error("the notes it does cover were not reported either")
	}
}

// A staff with nothing on it is a fact about the score, and leaving it out
// looks like a staff that was missed.
func TestAnEmptyStaffIsStillListed(t *testing.T) {
	got := Reaches(line(1, 0, 60), []Instrument{horn, horn}, number)
	if len(got) != 2 {
		t.Fatalf("%d staves reported, want both", len(got))
	}
	if got[0].Part != 0 || got[0].Notes != 0 {
		t.Errorf("the empty staff came out as %+v", got[0])
	}
	if len(got[0].Findings) != 0 {
		t.Error("an empty staff was found fault with")
	}
}

func TestStavesComeBackInOrder(t *testing.T) {
	notes := append(line(2, 0, 60), line(0, 0, 60)...)
	got := Reaches(notes, []Instrument{horn, horn, horn}, number)
	for i, r := range got {
		if r.Part != i {
			t.Errorf("staff %d is reported in place %d", r.Part, i)
		}
	}
}
