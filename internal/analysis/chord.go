/*
 * chord.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// What the chord is called, and what it is doing.
//
// Two questions, and the second needs the first. "A, C♯, E with E in the
// bass" is A major in second inversion; in the key of E that is IV in second
// inversion, and in the key of A it is the tonic. The name is a fact about
// the notes; the numeral is a fact about the notes and the key together, and
// it is the one that says what the chord is for.
//
// Nothing here guesses. A set of notes that is not one of the chords gscore
// names comes back unnamed, because a wrong name is worse than a blank: it
// is read, believed and acted on.

package analysis

import "sort"

// Chord is what a set of sounding notes is called.
type Chord struct {
	// Root is the pitch class the chord is built on, Quality what is built
	// on it: "major", "minor seventh", and so on.
	Root    int    `json:"root"`
	Quality string `json:"quality"`
	// Bass is the pitch class at the bottom, and Inversion which chord
	// tone that is: 0 for the root, 1 for the third, and so on.
	Bass      int `json:"bass"`
	Inversion int `json:"inversion"`
	// Name is the whole of it written out: "A major", "G7/B".
	Name string `json:"name"`
	// Named says the notes are a chord gscore has a name for. Where it is
	// false nothing else is filled in.
	Named bool `json:"named"`
}

// chordShape is one kind of chord: the intervals above its root, and what it
// is called. Ordered so that the first match is the simplest reading — a
// major triad is a major triad and not an incomplete thirteenth.
var chordShapes = []struct {
	steps []int
	name  string
	// symbol is the shorthand, for the chord written over the staff.
	symbol string
}{
	{[]int{0, 4, 7}, "major", ""},
	{[]int{0, 3, 7}, "minor", "m"},
	{[]int{0, 3, 6}, "diminished", "dim"},
	{[]int{0, 4, 8}, "augmented", "aug"},
	{[]int{0, 5, 7}, "suspended fourth", "sus4"},
	{[]int{0, 2, 7}, "suspended second", "sus2"},
	{[]int{0, 4, 7, 10}, "dominant seventh", "7"},
	{[]int{0, 4, 7, 11}, "major seventh", "maj7"},
	{[]int{0, 3, 7, 10}, "minor seventh", "m7"},
	{[]int{0, 3, 6, 10}, "half-diminished seventh", "ø7"},
	{[]int{0, 3, 6, 9}, "diminished seventh", "dim7"},
	{[]int{0, 3, 7, 11}, "minor-major seventh", "mMaj7"},
	{[]int{0, 4, 7, 9}, "major sixth", "6"},
	{[]int{0, 3, 7, 9}, "minor sixth", "m6"},
	{[]int{0, 4, 8, 10}, "augmented seventh", "aug7"},
	// Two notes are not a chord, but naming the interval is better than
	// saying nothing about the commonest thing on a page after a triad.
	{[]int{0, 7}, "open fifth", "5"},
	{[]int{0, 4}, "major third", ""},
	{[]int{0, 3}, "minor third", "m"},
}

// NameChord works out what a set of sounding pitches is called.
//
// midis are as they sound, in any order and with repeats; the lowest of them
// is the bass, which is what decides the inversion. A chord with no third is
// a chord with no third and is named as one rather than as the triad
// somebody probably meant.
func NameChord(midis []int) Chord {
	if len(midis) == 0 {
		return Chord{}
	}
	bass := midis[0]
	seen := map[int]bool{}
	for _, m := range midis {
		if m < bass {
			bass = m
		}
		seen[((m%12)+12)%12] = true
	}
	var pcs []int
	for pc := range seen {
		pcs = append(pcs, pc)
	}
	sort.Ints(pcs)
	bassPC := ((bass % 12) + 12) % 12

	// The bass is tried as the root first. Most chords have one root and it
	// makes no difference; the diminished seventh and the augmented triad
	// are the same set of notes whichever of them you build on, and for
	// those the note at the bottom is the one it is named after. B D F A♭
	// came out as D diminished seventh with B in the bass, which is a
	// reading of the same chord and not the one anybody writes.
	roots := append([]int{bassPC}, pcs...)

	for _, shape := range chordShapes {
		if len(shape.steps) != len(pcs) {
			continue
		}
		for _, root := range roots {
			if !fits(pcs, root, shape.steps) {
				continue
			}
			c := Chord{
				Root: root, Quality: shape.name, Bass: bassPC,
				Inversion: inversionOf(shape.steps, root, bassPC), Named: true,
			}
			c.Name = chordName(c, shape.symbol)
			return c
		}
	}
	return Chord{Bass: bassPC}
}

// fits says whether the pitch classes are exactly the shape built on a root.
func fits(pcs []int, root int, steps []int) bool {
	want := map[int]bool{}
	for _, s := range steps {
		want[(root+s)%12] = true
	}
	if len(want) != len(pcs) {
		return false
	}
	for _, pc := range pcs {
		if !want[pc] {
			return false
		}
	}
	return true
}

func inversionOf(steps []int, root, bass int) int {
	for i, s := range steps {
		if (root+s)%12 == bass {
			return i
		}
	}
	return 0
}

func chordName(c Chord, symbol string) string {
	name := sharpNames[c.Root] + " " + c.Quality
	if c.Inversion != 0 {
		name += ", " + sharpNames[c.Bass] + " in the bass"
	}
	_ = symbol
	return name
}

// Numerals are the seven degrees, written the way a numeral is written.
var numerals = [7]string{"I", "II", "III", "IV", "V", "VI", "VII"}

// Semitones above the tonic for each degree of a major scale, which is what
// a numeral counts in. A chord on a root that is not a degree of the scale
// is named with an accidental in front of the numeral, the way it is written
// by hand.
var majorDegrees = [7]int{0, 2, 4, 5, 7, 9, 11}

// Numeral says what the chord is doing in a key: the degree it is built on,
// its quality in the case the numeral is written in, and the figures for its
// inversion.
//
// Empty where the chord has no name, because a numeral is a statement about
// a chord and there is nothing to make it about.
func Numeral(c Chord, k Key) string {
	if !c.Named {
		return ""
	}
	// Where the root sits in the key, in semitones above the tonic.
	above := ((c.Root-k.Tonic)%12 + 12) % 12
	// The minor keys count their degrees from the natural minor, which is
	// the major scale starting a minor third higher.
	degrees := majorDegrees
	if k.Minor {
		degrees = [7]int{0, 2, 3, 5, 7, 8, 10}
	}
	degree, alter := -1, 0
	for i, d := range degrees {
		if d == above {
			degree = i
			break
		}
	}
	if degree < 0 {
		// Not a degree of the scale: the nearest one below, with a sharp.
		for i := len(degrees) - 1; i >= 0; i-- {
			if degrees[i] < above {
				degree, alter = i, above-degrees[i]
				break
			}
		}
		if degree < 0 {
			degree, alter = 0, above
		}
	}

	n := numerals[degree]
	lower := false
	switch c.Quality {
	case "minor", "minor third", "minor seventh", "minor sixth",
		"minor-major seventh":
		lower = true
	case "diminished", "diminished seventh", "half-diminished seventh":
		lower = true
	case "open fifth", "suspended fourth", "suspended second":
		// No third, so the chord does not say whether it is major or
		// minor and the key has to. A bare fifth on the tonic of a minor
		// key is written i, and calling it I says the piece is in the
		// major — which is the one thing a bare fifth is written to
		// avoid saying.
		lower = minorDegree(degree, k.Minor)
	}
	if lower {
		n = lowerCase(n)
	}
	switch alter {
	case 1:
		n = "♯" + n
	case -1, 11:
		n = "♭" + n
	}
	switch c.Quality {
	case "diminished", "diminished seventh":
		n += "°"
	case "half-diminished seventh":
		n += "ø"
	case "augmented", "augmented seventh":
		n += "+"
	}
	return n + figures(c)
}

// figures are the numbers written under a numeral to say which note is in
// the bass: the intervals above it that the chord makes, as they have been
// written since figured bass.
func figures(c Chord) string {
	seventh := false
	switch c.Quality {
	case "dominant seventh", "major seventh", "minor seventh",
		"half-diminished seventh", "diminished seventh",
		"minor-major seventh", "augmented seventh":
		seventh = true
	}
	if seventh {
		switch c.Inversion {
		case 0:
			return "7"
		case 1:
			return "6⁄5"
		case 2:
			return "4⁄3"
		case 3:
			return "4⁄2"
		}
		return "7"
	}
	switch c.Inversion {
	case 1:
		return "6"
	case 2:
		return "6⁄4"
	}
	return ""
}

func lowerCase(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + ('a' - 'A')
		}
	}
	return string(out)
}

// Function is what the chord does in the key, in the word a listener would
// use. Empty where the chord is not one of the degrees that has a job.
func Function(c Chord, k Key) string {
	if !c.Named {
		return ""
	}
	switch ((c.Root-k.Tonic)%12 + 12) % 12 {
	case 0:
		return "tonic"
	case 5:
		return "subdominant"
	case 7:
		if c.Quality == "dominant seventh" || c.Quality == "major" {
			return "dominant"
		}
		return "on the fifth"
	case 2:
		return "supertonic"
	case 9, 8:
		return "submediant"
	case 4, 3:
		return "mediant"
	case 11, 10:
		return "leading tone"
	}
	return ""
}

// minorDegree says whether the triad built on a degree of the scale is minor
// or diminished, which is what decides the case of its numeral where the
// chord itself has no third to say.
func minorDegree(degree int, minorKey bool) bool {
	if minorKey {
		// i ii° III iv v VI VII
		switch degree {
		case 0, 1, 3, 4:
			return true
		}
		return false
	}
	// I ii iii IV V vi vii°
	switch degree {
	case 1, 2, 5, 6:
		return true
	}
	return false
}
