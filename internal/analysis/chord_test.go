/*
 * chord_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What the chord is called, and what it is doing.
 *
 * Two questions, and the second needs the first. A, C♯ and E with E at the
 * bottom is A major in second inversion; in E that is IV in second
 * inversion, and in A it is the tonic. The name is a fact about the notes;
 * the numeral is a fact about the notes and the key together, and it is the
 * one that says what the chord is for.
 */

package analysis

import "testing"

// Middle C is 60, so these are all in the octave above it.
const (
	c  = 60
	cs = 61
	d  = 62
	ds = 63
	e  = 64
	f  = 65
	fs = 66
	g  = 67
	gs = 68
	a  = 69
	as = 70
	b  = 71
)

func TestTheTriadsAreNamed(t *testing.T) {
	for _, tc := range []struct {
		notes []int
		want  string
	}{
		{[]int{c, e, g}, "C major"},
		{[]int{c, ds, g}, "C minor"},
		{[]int{c, ds, fs}, "C diminished"},
		{[]int{c, e, gs}, "C augmented"},
		{[]int{c, f, g}, "C suspended fourth"},
		{[]int{g, b, d + 12, f + 12}, "G dominant seventh"},
		{[]int{c, e, g, b}, "C major seventh"},
		{[]int{d, f, a, c + 12}, "D minor seventh"},
		{[]int{b, d + 12, f + 12, a + 12}, "B half-diminished seventh"},
		{[]int{b, d + 12, f + 12, gs + 12}, "B diminished seventh"},
	} {
		if got := NameChord(tc.notes).Name; got != tc.want {
			t.Errorf("%v is called %q, want %q", tc.notes, got, tc.want)
		}
	}
}

// The bass decides the inversion, and the name says which note is down
// there: a chord in first inversion is a different thing to play.
func TestTheBassDecidesTheInversion(t *testing.T) {
	root := NameChord([]int{c, e, g})
	if root.Inversion != 0 {
		t.Errorf("C E G is inversion %d", root.Inversion)
	}
	first := NameChord([]int{e, g, c + 12})
	if first.Inversion != 1 || first.Root != 0 {
		t.Errorf("E G C came out as %+v, want C major with the third in the bass", first)
	}
	if first.Name != "C major, E in the bass" {
		t.Errorf("it is called %q", first.Name)
	}
	second := NameChord([]int{g, c + 12, e + 12})
	if second.Inversion != 2 {
		t.Errorf("G C E is inversion %d", second.Inversion)
	}
}

// A doubled note is the same chord: an orchestra plays a triad on forty
// instruments and it is still a triad.
func TestDoublingDoesNotChangeTheChord(t *testing.T) {
	one := NameChord([]int{c, e, g})
	many := NameChord([]int{c - 24, g - 12, c, e, g, c + 12, e + 12, g + 24})
	if one.Root != many.Root || one.Quality != many.Quality {
		t.Errorf("doubled, the chord became %+v", many)
	}
}

// Nothing here guesses. A set of notes that is not a chord gscore names
// comes back unnamed, because a wrong name is read, believed and acted on.
func TestWhatIsNotAChordIsNotNamed(t *testing.T) {
	for _, notes := range [][]int{
		{c, cs, d},
		{c, cs, d, ds, e},
		{c, d, f, gs, as},
	} {
		if got := NameChord(notes); got.Named {
			t.Errorf("%v was called %q", notes, got.Name)
		}
	}
}

func TestNothingSoundingIsNotAChord(t *testing.T) {
	if NameChord(nil).Named {
		t.Error("silence was named")
	}
}

// The numeral is about the chord and the key together.
func TestTheNumeralSaysWhatTheChordIsDoing(t *testing.T) {
	cmaj := Key{Tonic: 0}
	amin := Key{Tonic: 9, Minor: true}
	for _, tc := range []struct {
		notes []int
		key   Key
		want  string
	}{
		{[]int{c, e, g}, cmaj, "I"},
		{[]int{d, f, a}, cmaj, "ii"},
		{[]int{g, b, d + 12}, cmaj, "V"},
		{[]int{g, b, d + 12, f + 12}, cmaj, "V7"},
		{[]int{b, d + 12, f + 12}, cmaj, "vii°"},
		{[]int{e, g, c + 12}, cmaj, "I6"},
		{[]int{g, c + 12, e + 12}, cmaj, "I6⁄4"},
		{[]int{b, d + 12, f + 12, g + 12}, cmaj, "V6⁄5"},
		// The same notes mean something else in another key.
		{[]int{a, c + 12, e + 12}, amin, "i"},
		{[]int{e, gs, b}, amin, "V"},
		{[]int{d, f, a}, amin, "iv"},
	} {
		got := Numeral(NameChord(tc.notes), tc.key)
		if got != tc.want {
			t.Errorf("%v in %v is %q, want %q", tc.notes, tc.key, got, tc.want)
		}
	}
}

// A chord built on a note that is not in the key is written with an
// accidental in front of the numeral, the way it is written by hand.
func TestAChordFromOutsideTheKeyKeepsItsAccidental(t *testing.T) {
	// D major in C: the V of V.
	got := Numeral(NameChord([]int{d, fs, a}), Key{Tonic: 0})
	if got != "II" {
		t.Errorf("D major in C is %q, want II", got)
	}
	// E flat major in C.
	got = Numeral(NameChord([]int{ds, g, as}), Key{Tonic: 0})
	if got != "♯II" && got != "♭III" {
		t.Errorf("E flat major in C is %q", got)
	}
}

// Nothing to name, nothing to say about it.
func TestAnUnnamedChordHasNoNumeral(t *testing.T) {
	if n := Numeral(NameChord([]int{c, cs, d}), Key{Tonic: 0}); n != "" {
		t.Errorf("a cluster was given the numeral %q", n)
	}
}

func TestTheFunctionIsTheWordAListenerWouldUse(t *testing.T) {
	cmaj := Key{Tonic: 0}
	if got := Function(NameChord([]int{c, e, g}), cmaj); got != "tonic" {
		t.Errorf("C in C is the %q", got)
	}
	if got := Function(NameChord([]int{g, b, d + 12, f + 12}), cmaj); got != "dominant" {
		t.Errorf("G7 in C is the %q", got)
	}
	if got := Function(NameChord([]int{f, a, c + 12}), cmaj); got != "subdominant" {
		t.Errorf("F in C is the %q", got)
	}
}
