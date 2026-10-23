/*
 * pitch.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package model

import "fmt"

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
