/*
 * spacings.go
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

package engrave

// Spacings is the tunable engraving table, carried over from the Spacings_t
// tree in legacy/src/include/spacings.h.
//
// gscore had this in 2004, before SMuFL existed: named, user-editable
// engraving measurements reachable from a settings dialog, rather than
// constants buried in the drawing code. The idea was right and is kept. Two
// things change.
//
// First, the values are staff spaces rather than pixels, which is what lets
// one table serve the screen at any zoom and a printed page at any size.
//
// Second, the per-duration "space after" fields are now minima rather than
// answers. The original gave each note value a fixed width, which meant
// nothing could ever be stretched to fill a line or squeezed to fit one — see
// docs/engraving.md. Width is now proportional to duration, and these values
// only stop things colliding.
type Spacings struct {
	// NoteBase is the ideal width of a quarter note's slot. Other durations
	// scale from it, which is what makes justification possible.
	NoteBase float64
	// NoteExponent shapes that scaling. At 1 the spacing is strictly
	// proportional to duration, which engravers find too wide for long notes
	// and too tight for short ones; the traditional range is 0.5 to 0.75.
	NoteExponent float64
	// MinNote is the floor under any element's width.
	MinNote float64

	AccidentalToNote   float64 // Accidentals.sbaan
	BetweenAccidentals float64 // Accidentals.sbaon
	DotToNote          float64 // AugmentationDots.sbdan
	BetweenDots        float64 // AugmentationDots.sbd

	ClefBefore float64 // Clefs.sb
	ClefAfter  float64 // Clefs.sa

	BetweenKeyAccidentals float64 // KeySignatures.sbksa
	AfterKeySignature     float64 // KeySignatures.saks
	AfterTimeSignature    float64 // TimeSignatures.sats

	AfterBarline    float64 // Barlines.sab
	BetweenMeasures float64 // Measures.slbm

	// StaffGap and SystemGap are vertical, measured between the staves of one
	// system and between systems. The original had no equivalent: it drew one
	// unbroken strip and never had to lay out a second line.
	StaffGap  float64
	SystemGap float64

	// Margin is the blank border around the music.
	Margin float64
}

// DefaultSpacings returns the starting table, in staff spaces.
func DefaultSpacings() Spacings {
	return Spacings{
		NoteBase:     3.2,
		NoteExponent: 0.6,
		MinNote:      1.6,

		AccidentalToNote:   0.25,
		BetweenAccidentals: 0.15,
		DotToNote:          0.35,
		BetweenDots:        0.3,

		ClefBefore: 0.6,
		ClefAfter:  0.9,

		BetweenKeyAccidentals: 0.1,
		AfterKeySignature:     0.9,
		AfterTimeSignature:    1.2,

		AfterBarline:    0.8,
		BetweenMeasures: 0.6,

		StaffGap:  10,
		SystemGap: 9,

		Margin: 4,
	}
}
