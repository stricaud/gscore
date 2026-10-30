/*
 * pitch.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import (
	"fmt"
	"strings"
)

// Step is a diatonic scale step, carrying no alteration of its own.
type Step int

// The seven diatonic steps, ordered as they are spelled from C.
const (
	StepC Step = iota
	StepD
	StepE
	StepF
	StepG
	StepA
	StepB
)

var stepLetters = [7]byte{'C', 'D', 'E', 'F', 'G', 'A', 'B'}

// semitonesFromC gives each step's distance above C within one octave.
var semitonesFromC = [7]int{0, 2, 4, 5, 7, 9, 11}

func (s Step) String() string {
	if s < StepC || s > StepB {
		return fmt.Sprintf("Step(%d)", int(s))
	}
	return string(stepLetters[s])
}

// StepFromLetter maps a letter in either case to its diatonic step.
func StepFromLetter(c byte) (Step, bool) {
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	for i, l := range stepLetters {
		if l == c {
			return Step(i), true
		}
	}
	return 0, false
}

// Pitch is a sounding pitch, spelled.
//
// Alter is the alteration in semitones that actually sounds, wherever it comes
// from: an accidental written on the note, or the key signature. It is not a
// request to print anything. What gets printed is Note.Accidental, which is
// kept separate precisely because the two differ constantly: in G major a
// sharpened F sounds with Alter 1 and prints nothing at all.
//
// Octave follows scientific pitch notation, so middle C is {StepC, 4, 0}.
type Pitch struct {
	Step   Step
	Octave int
	Alter  int
}

// MIDI returns the MIDI note number, in which middle C is 60.
func (p Pitch) MIDI() int {
	return (p.Octave+1)*12 + semitonesFromC[p.Step] + p.Alter
}

// DiatonicIndex counts diatonic steps from middle C, which is index 0. This is
// what engraving needs: combined with a clef it gives the staff position, and
// it deliberately ignores Alter, since an accidental never moves a notehead.
func (p Pitch) DiatonicIndex() int {
	return p.Octave*7 + int(p.Step) - 4*7
}

// PitchFromDiatonicIndex is the inverse of DiatonicIndex, with no alteration.
func PitchFromDiatonicIndex(i int) Pitch {
	step := i % 7
	oct := i / 7
	if step < 0 {
		step += 7
		oct--
	}
	return Pitch{Step: Step(step), Octave: oct + 4}
}

// Transpose moves the pitch by an interval given as a number of diatonic steps
// together with a number of semitones. Supplying both is what fixes the
// spelling of the result: a major third up is Transpose(2, 4) and gives E from
// C, while a diminished fourth up is Transpose(3, 4) and gives F-flat.
func (p Pitch) Transpose(steps, semitones int) Pitch {
	out := PitchFromDiatonicIndex(p.DiatonicIndex() + steps)
	out.Alter = p.MIDI() + semitones - out.MIDI()
	return out
}

// String spells the pitch the way musicians write it, as in "F#4" or "Ebb3".
func (p Pitch) String() string {
	s := p.Step.String()
	switch {
	case p.Alter > 0:
		for i := 0; i < p.Alter; i++ {
			s += "#"
		}
	case p.Alter < 0:
		for i := 0; i < -p.Alter; i++ {
			s += "b"
		}
	}
	return fmt.Sprintf("%s%d", s, p.Octave)
}

// ParsePitch reads a pitch written the way a musician types one: a letter,
// any accidentals, and an octave. "c4", "F#3", "Bb5", "gx2", "Ebb1".
//
// Sharps are # or s, flats b, and a double sharp may be written ## or x.
// The letter's case does not matter; the octave is scientific, so middle C
// is c4. A name with no octave comes back with ok and an octave of zero,
// which is a caller's problem to resolve: in a run of notes the octave
// nearest the one before is almost always what was meant, and only the
// caller knows what came before.
func ParsePitch(s string) (p Pitch, hasOctave bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Pitch{}, false, false
	}
	letter := s[0]
	if letter >= 'A' && letter <= 'G' {
		letter += 'a' - 'A'
	}
	if letter < 'a' || letter > 'g' {
		return Pitch{}, false, false
	}
	// a b c d e f g, as steps counted from C.
	p.Step = Step([7]int{5, 6, 0, 1, 2, 3, 4}[letter-'a'])

	i := 1
	for i < len(s) {
		switch s[i] {
		case '#', 's', 'S':
			p.Alter++
		case 'b', 'B':
			// A flat, unless it is the start of an octave — which it
			// cannot be, since an octave is a digit.
			p.Alter--
		case 'x', 'X':
			p.Alter += 2
		case 'n', 'N':
			// A natural. It alters nothing, and saying so is the point:
			// in D major a bare f is the key's F sharp and "fn" is the F
			// the writer meant.
			p.Alter += 0
		default:
			goto octave
		}
		i++
	}
octave:
	// Two is as far as notation goes in either direction. A triple sharp
	// is not a pitch anybody means, and reading one would be reading a
	// typing mistake as music.
	if p.Alter < -2 || p.Alter > 2 {
		return Pitch{}, false, false
	}
	if i >= len(s) {
		return p, false, true
	}
	rest := s[i:]
	neg := false
	if rest[0] == '-' {
		neg, rest = true, rest[1:]
	}
	if rest == "" {
		return Pitch{}, false, false
	}
	n := 0
	for k := 0; k < len(rest); k++ {
		if rest[k] < '0' || rest[k] > '9' {
			return Pitch{}, false, false
		}
		n = n*10 + int(rest[k]-'0')
	}
	if neg {
		n = -n
	}
	if n < -1 || n > 9 {
		return Pitch{}, false, false
	}
	p.Octave = n
	return p, true, true
}

// NearestTo is this pitch moved to whichever octave puts it closest to
// another, which is what a run of note names means when they leave the
// octave out: "c d e f g" climbs, and "c b" goes down a semitone rather
// than up a seventh.
func (p Pitch) NearestTo(other Pitch) Pitch {
	best := p
	best.Octave = other.Octave
	for _, oct := range []int{other.Octave - 1, other.Octave, other.Octave + 1} {
		try := p
		try.Octave = oct
		if abs(try.MIDI()-other.MIDI()) < abs(best.MIDI()-other.MIDI()) {
			best = try
		}
	}
	return best
}

// JustAbove is this pitch moved to whichever octave puts it just above
// another, which is what the notes of a chord mean when they leave the
// octave out.
//
// A chord is read and written from the bottom up, so each notehead stands
// above the one before it however far that is: "[ca]" is a sixth, where the
// nearest A to a C is the one below it and NearestTo would put it there.
//
// Above on the staff rather than above in sound, because what is being
// stacked is a notehead: a flat on it does not drop it under the notehead
// below, and C with D flat over it is the cluster it was written as.
func (p Pitch) JustAbove(other Pitch) Pitch {
	up := p
	up.Octave = other.Octave
	if up.DiatonicIndex() <= other.DiatonicIndex() {
		up.Octave++
	}
	return up
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
