/*
 * chordtones_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What notes a named chord is made of.
 *
 * The table is the program's one statement about what a chord is, so what is
 * checked here is that it says the right thing and that nothing reading it
 * can be given a wrong answer quietly.
 */

package analysis

import "testing"

// The chords anybody writes, against the notes they are. Semitones above the
// root, which is the part a reader can check by ear.
func TestTheCommonChordsAreWhatTheyAre(t *testing.T) {
	for _, c := range []struct {
		quality string
		want    []int
	}{
		{"", []int{0, 4, 7}},
		{"m", []int{0, 3, 7}},
		{"7", []int{0, 4, 7, 10}},
		{"maj7", []int{0, 4, 7, 11}},
		{"m7", []int{0, 3, 7, 10}},
		{"m7b5", []int{0, 3, 6, 10}},
		{"dim7", []int{0, 3, 6, 9}},
		{"sus4", []int{0, 5, 7}},
		{"sus2", []int{0, 2, 7}},
		{"6", []int{0, 4, 7, 9}},
		{"9", []int{0, 4, 7, 10, 14}},
		{"7b9", []int{0, 4, 7, 10, 13}},
		{"13", []int{0, 4, 7, 10, 14, 21}},
	} {
		tones, ok := ChordTones(c.quality)
		if !ok {
			t.Errorf("%q is not a chord gscore knows", c.quality)
			continue
		}
		if len(tones) != len(c.want) {
			t.Errorf("%q has %d notes, want %d", c.quality, len(tones), len(c.want))
			continue
		}
		for i, tone := range tones {
			if tone.Semitones != c.want[i] {
				t.Errorf("%q note %d is %d semitones up, want %d",
					c.quality, i+1, tone.Semitones, c.want[i])
			}
		}
	}
}

// Every tone is spelled on a letter, and which letter is the whole reason
// this table has two numbers in it. A degree that does not match the
// distance it is paired with is a chord that will be spelled wrong: a third
// eleven semitones up would come out as a wildly accidental E.
func TestEveryToneIsSpelledOnThePlausibleLetter(t *testing.T) {
	// How far above the root each degree sits in a major scale, which is
	// what a letter sounds at before anything is done to it.
	plain := map[int]int{0: 0, 1: 2, 2: 4, 3: 5, 4: 7, 5: 9, 6: 11,
		8: 14, 10: 17, 12: 21}
	for quality, tones := range ChordQualities() {
		// Whether this chord has a flattened fifth in it, which is what
		// lets its seventh be flattened twice. C diminished seventh is
		// C, E flat, G flat, B double flat: the seventh really is two
		// semitones below where the letter B sounds, because it is a
		// diminished seventh and not a minor one. It is the only doubly
		// diminished interval anybody writes, so it is the only place a
		// second flat is allowed here.
		flatFifth := false
		for _, tone := range tones {
			if tone.Degree == 4 && tone.Semitones == 6 {
				flatFifth = true
			}
		}
		for _, tone := range tones {
			natural, ok := plain[tone.Degree]
			if !ok {
				t.Errorf("%q is spelled on degree %d, which is not a degree "+
					"anybody writes a chord tone on", quality, tone.Degree)
				continue
			}
			// Otherwise within a semitone either way: a chord tone is the
			// degree sharpened or flattened, never more. Two away is a
			// different degree written badly.
			low := -1
			if flatFifth && tone.Degree == 6 {
				low = -2
			}
			if d := tone.Semitones - natural; d < low || d > 1 {
				t.Errorf("%q spells %d semitones up on degree %d, which is "+
					"%d away from where that letter sounds",
					quality, tone.Semitones, tone.Degree, d)
			}
		}
	}
}

// Case matters in exactly one place and it matters a great deal.
func TestMajorAndMinorSeventhsAreToldApart(t *testing.T) {
	big, _ := ChordTones("M7")
	small, _ := ChordTones("m7")
	if len(big) < 4 || len(small) < 4 {
		t.Fatal("one of them is not a seventh chord at all")
	}
	if big[1].Semitones != 4 || big[3].Semitones != 11 {
		t.Errorf("M7 is %v, which is not a major seventh", big)
	}
	if small[1].Semitones != 3 || small[3].Semitones != 10 {
		t.Errorf("m7 is %v, which is not a minor seventh", small)
	}
}

// The spellings people really type, each reaching the chord it means.
func TestTheSpellingsPeopleTypeAreKnown(t *testing.T) {
	same := func(a, b string) {
		t.Helper()
		x, okx := ChordTones(a)
		y, oky := ChordTones(b)
		if !okx || !oky {
			t.Errorf("%q or %q is not known", a, b)
			return
		}
		if len(x) != len(y) {
			t.Errorf("%q and %q are different chords", a, b)
			return
		}
		for i := range x {
			if x[i] != y[i] {
				t.Errorf("%q and %q are different chords", a, b)
				return
			}
		}
	}
	same("m", "min")
	same("m", "-")
	same("m", "MIN")
	same("maj7", "Δ")
	same("maj7", "ma7")
	same("m7b5", "ø")
	same("dim", "°")
	same("aug", "+")
	same("sus", "sus4")
	same("6/9", "69")
}

// Nothing is guessed. A quality gscore does not know comes back unknown, so
// that whatever asked can say so — a chord filled in with the triad somebody
// probably meant is a chord that will be read, believed and played.
func TestAnUnknownQualityIsNotGuessedAt(t *testing.T) {
	for _, q := range []string{"zonk", "m7b5b9b13", "?", "7b13", "maj", "mi"} {
		_, ok := ChordTones(q)
		switch q {
		case "maj", "mi":
			if !ok {
				t.Errorf("%q is a chord anybody would type", q)
			}
		default:
			if ok {
				t.Errorf("%q was given a reading", q)
			}
		}
	}
}

// The table handed out is a copy. It is the program's one statement about
// what a chord is, and something that was handed it should not be able to
// edit it for everybody else.
func TestTheTableHandedOutIsACopy(t *testing.T) {
	got := ChordQualities()
	got["maj7"][0] = Tone{Degree: 9, Semitones: 9}
	got["zonk"] = []Tone{{0, 0}}

	tones, ok := ChordTones("maj7")
	if !ok || tones[0].Semitones != 0 || tones[0].Degree != 0 {
		t.Errorf("maj7 is now %v", tones)
	}
	if _, ok := ChordTones("zonk"); ok {
		t.Error("a chord was added to the program from outside it")
	}
}

// Every chord begins on its root. A chord whose first tone is not the root
// would be spelled from the wrong note by anybody reading the list in order.
func TestEveryChordBeginsOnItsRoot(t *testing.T) {
	for quality, tones := range ChordQualities() {
		if len(tones) < 2 {
			t.Errorf("%q has %d notes in it", quality, len(tones))
			continue
		}
		if tones[0] != (Tone{Degree: 0, Semitones: 0}) {
			t.Errorf("%q begins on %v rather than on its root", quality, tones[0])
		}
		for i := 1; i < len(tones); i++ {
			if tones[i].Semitones <= tones[i-1].Semitones {
				t.Errorf("%q is not written from the bottom up: %v", quality, tones)
				break
			}
		}
	}
}
