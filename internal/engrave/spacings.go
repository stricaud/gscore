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
	//
	// It decides how much music goes on a line, and through that how many
	// pages a score runs to: at 3.2 a part came out a third longer than it
	// needed to be, with lines two thirds full of air. A bar of four
	// quarters is about 28mm at the size a part is printed, which is what a
	// printed part looks like.
	NoteBase float64
	// NoteExponent shapes that scaling. At 1 the spacing is strictly
	// proportional to duration, which engravers find too wide for long notes
	// and too tight for short ones; the traditional range is 0.5 to 0.75.
	NoteExponent float64
	// MinNote is the floor under any element's width.
	MinNote float64

	// BeforeAccidental is the air left in front of an accidental, separating
	// it from whatever was written before. Without it the accidental is
	// pushed up against the previous notehead or its stem and reads as
	// belonging to that note instead of this one.
	BeforeAccidental   float64
	AccidentalToNote   float64 // Accidentals.sbaan
	BetweenAccidentals float64 // Accidentals.sbaon
	DotToNote          float64 // AugmentationDots.sbdan
	BetweenDots        float64 // AugmentationDots.sbd

	ClefBefore float64 // Clefs.sb
	ClefAfter  float64 // Clefs.sa

	BetweenKeyAccidentals float64 // KeySignatures.sbksa
	AfterKeySignature     float64 // KeySignatures.saks

	// AfterKeyCancel is the air between the naturals that cancel the old key
	// and the accidentals of the new one.
	//
	// More than the air inside either group, which is almost none: the
	// accidentals of a key signature are set close enough to read as one
	// block, and two blocks set that close read as one block of mixed signs.
	// A player glancing at a key change has to see where the cancelling
	// stops and the new key starts.
	AfterKeyCancel     float64
	AfterTimeSignature float64 // TimeSignatures.sats

	AfterBarline    float64 // Barlines.sab
	BetweenMeasures float64 // Measures.slbm

	// StaffGap and SystemGap are vertical, measured between the staves of one
	// system and between systems. A system's own extent already holds the
	// room its marks need above and below it, so SystemGap is the air on top
	// of that and does not need to be much. The original had no equivalent: it drew one
	// unbroken strip and never had to lay out a second line.
	StaffGap  float64
	SystemGap float64

	// Margin is the blank border around the music.
	Margin float64

	// MarginY is the border above and below, when it should differ from the
	// one at the sides. Zero means use Margin for both.
	//
	// The two are not the same thing. A clef is drawn well outside the staff
	// it names — a treble clef reaches about two and a half spaces past the
	// lines at each end — so a picture of one needs room vertically that the
	// music never needs horizontally.
	MarginY float64
}

// marginY is the vertical border actually in force.
func (s Spacings) marginY() float64 {
	if s.MarginY > 0 {
		return s.MarginY
	}
	return s.Margin
}

// DefaultSpacings returns the starting table, in staff spaces.
func DefaultSpacings() Spacings {
	return Spacings{
		NoteBase:     2.8,
		NoteExponent: 0.6,
		MinNote:      1.6,

		BeforeAccidental:   0.4,
		AccidentalToNote:   0.25,
		BetweenAccidentals: 0.15,
		DotToNote:          0.35,
		BetweenDots:        0.3,

		ClefBefore: 0.6,
		ClefAfter:  0.9,

		BetweenKeyAccidentals: 0.1,
		AfterKeyCancel:        0.7,
		AfterKeySignature:     0.9,
		AfterTimeSignature:    1.2,

		AfterBarline:    0.8,
		BetweenMeasures: 0.6,

		StaffGap:  10,
		SystemGap: 5,

		Margin: 4,
	}
}

// minStaffGap is as close as two staves of one system are ever set.
//
// It still leaves room for a dynamic under one staff and a slur over the next
// without their meeting, which is what the gap is for.
const minStaffGap = 6

// staffGap is the air between the staves of a system, which gets less as
// there are more of them.
//
// Spacings.StaffGap is the air a small score can afford. A big one cannot:
// the gap is most of a system's height, and a system taller than the page is
// either printed at a type size nobody can read across a pit or — worse —
// cut in half, with the horns on one sheet and the strings on the next.
// Seventeen staves at the roomy gap came to twice the height of A4, so the
// first page of a song printed four staves of it and dropped the rest.
//
// A printed conductor's score is set tight for exactly this reason: the air
// goes, so the notes can stay big enough to read. Half a space goes for each
// staff past the fourth, which leaves a song or a piano piece exactly as it
// was.
func (e *engraver) staffGap(staves int) float64 {
	gap := e.opt.Spacings.StaffGap
	if staves <= 4 {
		return gap
	}
	if gap -= 0.5 * float64(staves-4); gap < minStaffGap {
		gap = minStaffGap
	}
	return gap
}
