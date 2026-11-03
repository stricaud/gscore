/*
 * tuning_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Tunings, and the two directions between a fret and a pitch.
 *
 * A tab is written by saying which fret, and the pitch is worked out and
 * kept — because the pitch is what sounds, what transposes and what every
 * other part of the program reads. Getting that wrong is the one way a tab
 * can be wrong in a file and look right on the page.
 */

package model

import "testing"

func TestTheStandardGuitarIsStandard(t *testing.T) {
	tab, ok := TuningNamed("guitar")
	if !ok {
		t.Fatal("there is no guitar")
	}
	if tab.Lines() != 6 {
		t.Fatalf("a guitar has %d strings", tab.Lines())
	}
	// E4 B3 G3 D3 A2 E2, from the top line down.
	want := []int{64, 59, 55, 50, 45, 40}
	for i, w := range want {
		if got := tab.Strings[i].MIDI(); got != w {
			t.Errorf("string %d is %d, want %d", i+1, got, w)
		}
	}
}

func TestEveryTuningIsPlayable(t *testing.T) {
	for _, tn := range Tunings() {
		if len(tn.Strings) < 3 {
			t.Errorf("%s has %d strings", tn.Name, len(tn.Strings))
		}
		if tn.Frets < 10 || tn.Frets > 30 {
			t.Errorf("%s has %d frets", tn.Name, tn.Frets)
		}
		if _, ok := TuningNamed(tn.Name); !ok {
			t.Errorf("%s cannot be asked for by name", tn.Name)
		}
	}
}

// Each staff gets its own, or two guitars in one score would retune
// together.
func TestATuningIsFreshEachTime(t *testing.T) {
	a, _ := TuningNamed("guitar")
	b, _ := TuningNamed("guitar")
	a.Strings[5] = p("D2") // drop one to D
	if b.Strings[5].MIDI() != 40 {
		t.Error("retuning one staff retuned another")
	}
}

func TestATuningIsRecognisedAgain(t *testing.T) {
	tab, _ := TuningNamed("guitar — drop D")
	if got := TuningName(tab); got != "guitar — drop D" {
		t.Errorf("it came back as %q", got)
	}
	// Somebody else's tuning is not an error; it just has no name here.
	odd := &Tablature{Strings: []Pitch{p("C4"), p("G3"), p("C3")}, Frets: 12}
	if got := TuningName(odd); got != "custom" {
		t.Errorf("an unknown tuning came back as %q", got)
	}
	if got := TuningName(nil); got != "" {
		t.Errorf("no tablature came back as %q", got)
	}
}

/*
 * Fret to pitch, which is the direction somebody types in.
 */

func TestAFretSoundsThePitchItStops(t *testing.T) {
	tab, _ := TuningNamed("guitar")
	for _, tc := range []struct {
		str, fret int
		want      string
	}{
		{1, 0, "E4"},  // the open top string
		{6, 0, "E2"},  // the open bottom string
		{6, 3, "G2"},  // third fret, bottom string
		{5, 2, "B2"},  // second fret, A string
		{3, 2, "A3"},  // second fret, G string
		{1, 12, "E5"}, // the octave
	} {
		got, ok := tab.PitchAt(tc.str, tc.fret)
		if !ok {
			t.Errorf("string %d fret %d cannot be played", tc.str, tc.fret)
			continue
		}
		if want := p(tc.want); got.MIDI() != want.MIDI() {
			t.Errorf("string %d fret %d is %v, want %v",
				tc.str, tc.fret, got, want)
		}
	}
}

// A guitar tuned down reads in flats, so its frets should be written in
// flats: the player is reading a part in flats and F sharp is not it.
func TestAFlatTuningIsSpelledInFlats(t *testing.T) {
	tab, _ := TuningNamed("guitar — half step down")
	got, ok := tab.PitchAt(6, 2) // E flat string, two frets up
	if !ok {
		t.Fatal("it cannot be played")
	}
	if got.Alter > 0 {
		t.Errorf("the second fret came out as %v, which is sharp", got)
	}
	if want := p("F2"); got.MIDI() != want.MIDI() {
		t.Errorf("it sounds %v, want %v", got, want)
	}
}

func TestAFretOffTheNeckCannotBePlayed(t *testing.T) {
	tab, _ := TuningNamed("ukulele") // fifteen frets
	if _, ok := tab.PitchAt(1, 16); ok {
		t.Error("the sixteenth fret of a fifteen-fret instrument was allowed")
	}
	if _, ok := tab.PitchAt(1, -1); ok {
		t.Error("a negative fret was allowed")
	}
	if _, ok := tab.PitchAt(9, 0); ok {
		t.Error("a ninth string was allowed on a four-string instrument")
	}
}

/*
 * Pitch to fret, which is the direction a note written on a staff goes.
 */

func TestANoteCanBeAskedForOnAChosenString(t *testing.T) {
	tab, _ := TuningNamed("guitar")
	// A3 is the second fret of the G string, and the seventh of the D.
	if got, ok := tab.Fretted(p("A3"), 3); !ok || got != 2 {
		t.Errorf("on string 3 it is fret %d (%v)", got, ok)
	}
	if got, ok := tab.Fretted(p("A3"), 4); !ok || got != 7 {
		t.Errorf("on string 4 it is fret %d (%v)", got, ok)
	}
	// And not at all on a string already higher than it.
	if _, ok := tab.Fretted(p("A3"), 1); ok {
		t.Error("a note below the open string was given a fret")
	}
}

// At is the other question: not "can this string play it" but "where does
// a hand go". The highest string that reaches gives the lowest fret.
func TestAtTakesTheNearestPositionToTheNut(t *testing.T) {
	tab, _ := TuningNamed("guitar")
	str, fret, ok := tab.At(p("A3"))
	if !ok {
		t.Fatal("A3 cannot be played on a guitar")
	}
	if str != 3 || fret != 2 {
		t.Errorf("A3 came out on string %d fret %d, want string 3 fret 2", str, fret)
	}
}

func TestANoteTheInstrumentCannotReach(t *testing.T) {
	tab, _ := TuningNamed("guitar")
	if _, _, ok := tab.At(p("C1")); ok {
		t.Error("a guitar was given a note below its bottom string")
	}
}

// Round trip, on every string of every tuning: a fret turned into a pitch
// and asked for again on the same string is the fret it started as.
func TestFretsAndPitchesAgree(t *testing.T) {
	for _, tn := range Tunings() {
		tab, _ := TuningNamed(tn.Name)
		for str := 1; str <= tab.Lines(); str++ {
			for fret := 0; fret <= tab.Frets; fret++ {
				pitch, ok := tab.PitchAt(str, fret)
				if !ok {
					t.Fatalf("%s: string %d fret %d", tn.Name, str, fret)
				}
				back, ok := tab.Fretted(pitch, str)
				if !ok || back != fret {
					t.Errorf("%s: string %d fret %d came back as %d (%v)",
						tn.Name, str, fret, back, ok)
				}
			}
		}
	}
}

// Retuning moves every string by the same interval and leaves the frets
// alone, which is the whole of what it is for: the shapes are played as
// written and what comes out is somewhere else.
func TestRetunedMovesEveryStringTogether(t *testing.T) {
	guitar, ok := TuningNamed("guitar")
	if !ok {
		t.Fatal("there is no guitar")
	}
	down, ok := Retuned(guitar, -2)
	if !ok {
		t.Fatal("a guitar cannot be taken down a tone")
	}
	if len(down.Strings) != len(guitar.Strings) {
		t.Fatalf("it came back with %d strings", len(down.Strings))
	}
	for i := range guitar.Strings {
		if got, want := down.Strings[i].MIDI(), guitar.Strings[i].MIDI()-2; got != want {
			t.Errorf("string %d is %d, want %d", i+1, got, want)
		}
	}
	if down.Frets != guitar.Frets {
		t.Errorf("the neck grew to %d frets", down.Frets)
	}
	// And the original is untouched, so a move that is only being
	// considered cannot change the instrument.
	if guitar.Strings[0].MIDI() != 64 {
		t.Errorf("the guitar itself moved: top string is %v", guitar.Strings[0])
	}
}

// A fret keeps its number and what it sounds follows the tuning. This is
// the thing being checked, and it is what a capo does.
func TestARetunedFretSoundsSomewhereElse(t *testing.T) {
	guitar, _ := TuningNamed("guitar")
	was, ok := guitar.PitchAt(6, 3)
	if !ok {
		t.Fatal("the third fret of the bottom string is not there")
	}
	up, _ := Retuned(guitar, 3) // a capo at the third fret
	now, ok := up.PitchAt(6, 3)
	if !ok {
		t.Fatal("the third fret went away")
	}
	if got, want := now.MIDI(), was.MIDI()+3; got != want {
		t.Errorf("the same fret sounds %d, want %d", got, want)
	}
}

// Down gives flats and up gives sharps: a guitar is taken down to E flat,
// never up to D sharp.
func TestARetunedStringIsSpelledByTheWayItWent(t *testing.T) {
	guitar, _ := TuningNamed("guitar")
	down, _ := Retuned(guitar, -1)
	if s := down.Strings[0]; s.Step != StepE || s.Alter != -1 {
		t.Errorf("down a semitone the top string is %v, want E flat", s)
	}
	up, _ := Retuned(guitar, 2)
	if s := up.Strings[0]; s.Step != StepF || s.Alter != 1 {
		t.Errorf("up a tone the top string is %v, want F sharp", s)
	}
}

// A move nobody can make changes nothing rather than half of something.
func TestARetuningOffTheEndOfWhatCanBeWrittenIsRefused(t *testing.T) {
	guitar, _ := TuningNamed("guitar")
	if _, ok := Retuned(guitar, -120); ok {
		t.Error("a guitar was tuned below the bottom of MIDI")
	}
	if _, ok := Retuned(nil, 2); ok {
		t.Error("nothing was retuned")
	}
}
