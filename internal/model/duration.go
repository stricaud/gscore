/*
 * duration.go
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

// Duration is an exact musical duration, held as a fraction of a whole note.
// A quarter note is {1, 4} and a dotted half is {3, 4}.
//
// It is kept as a fraction rather than a note value plus a dot count because
// tuplets, ABC's unit note length and any arithmetic over a measure all need
// durations that no notehead can spell. Written reverses the derivation when a
// notehead is finally needed.
type Duration struct {
	Num, Den int
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// Reduced returns the duration in lowest terms with a positive denominator.
func (d Duration) Reduced() Duration {
	if d.Den < 0 {
		d.Num, d.Den = -d.Num, -d.Den
	}
	g := gcd(d.Num, d.Den)
	return Duration{d.Num / g, d.Den / g}
}

// IsZero reports whether the duration is zero length.
func (d Duration) IsZero() bool { return d.Num == 0 }

// Add returns d + e.
func (d Duration) Add(e Duration) Duration {
	return Duration{d.Num*e.Den + e.Num*d.Den, d.Den * e.Den}.Reduced()
}

// Sub returns d - e.
func (d Duration) Sub(e Duration) Duration {
	return Duration{d.Num*e.Den - e.Num*d.Den, d.Den * e.Den}.Reduced()
}

// Scale returns d multiplied by num/den.
func (d Duration) Scale(num, den int) Duration {
	return Duration{d.Num * num, d.Den * den}.Reduced()
}

// Cmp returns -1, 0 or 1 as d sorts before, with, or after e.
func (d Duration) Cmp(e Duration) int {
	l, r := d.Num*e.Den, e.Num*d.Den
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	}
	return 0
}

// Float returns the duration in whole notes, for layout arithmetic only.
func (d Duration) Float() float64 { return float64(d.Num) / float64(d.Den) }

func (d Duration) String() string {
	r := d.Reduced()
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

// NoteValue is a written note value: what the notehead, stem and flags show,
// before any augmentation dots are counted.
type NoteValue int

// The writable note values, from the breve down.
const (
	DoubleWhole NoteValue = iota
	Whole
	Half
	Quarter
	Eighth
	Sixteenth
	ThirtySecond
	SixtyFourth
	HundredTwentyEighth
)

var noteValueNames = [...]string{
	"double whole", "whole", "half", "quarter", "eighth",
	"sixteenth", "thirty-second", "sixty-fourth", "hundred-twenty-eighth",
}

func (v NoteValue) String() string {
	if v < DoubleWhole || int(v) >= len(noteValueNames) {
		return fmt.Sprintf("NoteValue(%d)", int(v))
	}
	return noteValueNames[v]
}

// Duration returns the undotted duration of the note value.
func (v NoteValue) Duration() Duration {
	if v <= DoubleWhole {
		return Duration{2, 1}
	}
	return Duration{1, 1 << (v - 1)}
}

// Flags returns how many flags or beams the value carries, 0 for a quarter and
// anything longer.
func (v NoteValue) Flags() int {
	if v <= Quarter {
		return 0
	}
	return int(v - Quarter)
}

// maxDots bounds the search in Written. Three is as many as notation uses in
// practice, and past it the fraction is better spelled as tied notes.
const maxDots = 3

// Written decomposes a duration into a note value and a number of augmentation
// dots. It reports ok false when no value and dot count can spell it, which is
// the normal answer for a tuplet member or a duration that needs tied notes;
// callers must handle that rather than rounding.
func (d Duration) Written() (v NoteValue, dots int, ok bool) {
	r := d.Reduced()
	if r.Num <= 0 {
		return 0, 0, false
	}
	for value := DoubleWhole; value <= HundredTwentyEighth; value++ {
		base := value.Duration()
		for n := 0; n <= maxDots; n++ {
			// n dots multiply the base by (2^(n+1) - 1) / 2^n.
			cand := base.Scale(1<<(n+1)-1, 1<<n)
			if cand.Cmp(r) == 0 {
				return value, n, true
			}
		}
	}
	return 0, 0, false
}
