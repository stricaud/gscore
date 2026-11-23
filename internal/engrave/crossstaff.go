/*
 * crossstaff.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"

	"github.com/stricaud/gscore/internal/model"
)

// Notes written on the staff next door.
//
// Piano writing needs this constantly. A figure belongs to one hand's
// rhythm — one beam, one voice — and runs past what that hand can reach,
// so the notes that stray are written on the other staff and the beam
// crosses between them. Writing them in the other hand's voice instead
// would put them in a different rhythm from the figure they are part of,
// which is a different piece of music.
//
// Nothing about the note changes: the same pitch, the same duration, the
// same bar. Only where it is drawn. So the mark lives on the note, the
// playback never hears about it, and everything that reads the music gets
// the same answer it always did.
//
// Two things have to follow the note across. Its height, because a C
// written on the bass staff is on a different line from the same C on the
// treble staff — so it is read against the other staff's clef. And its
// stem, which still has to reach the beam its group is under, and the
// beam is now between the two staves rather than over one of them.

// crossContext is what a staff knows about its neighbours, for the notes
// that are written on them.
//
// Set per staff by whatever is walking the staves. Empty where there is
// nothing next door, and empty during the passes that only measure widths
// — a width does not depend on which staff a notehead is drawn on, and the
// clef it is read against is all those passes need.
type crossContext struct {
	// above and below are the neighbouring staves' clefs at this point in
	// the music, and ok says whether there is such a staff at all.
	above, below       model.Clef
	hasAbove, hasBelow bool
	// dyAbove and dyBelow are how far that staff's top line is from this
	// one's, in staff spaces: negative upwards.
	//
	// Known before the staff is drawn because the gap between staves is
	// worked out before any of them are — see gapAfter, which answers
	// from what a previous pass measured and what this system holds. It
	// is zero in the measuring passes, where nothing is being drawn.
	dyAbove, dyBelow float64
}

// clefFor is the clef a note written on the neighbouring staff is read
// against, and whether there is one.
func (c crossContext) clefFor(dir model.CrossStaff) (model.Clef, bool) {
	switch dir {
	case model.OnStaffAbove:
		return c.above, c.hasAbove
	case model.OnStaffBelow:
		return c.below, c.hasBelow
	}
	return 0, false
}

// shiftFor is how far down the note is drawn from where its step would put
// it on its own staff.
func (c crossContext) shiftFor(dir model.CrossStaff) float64 {
	switch dir {
	case model.OnStaffAbove:
		return c.dyAbove
	case model.OnStaffBelow:
		return c.dyBelow
	}
	return 0
}

// crossOf is the direction an element is written in, or OnOwnStaff.
//
// A note with nowhere to go is written where it is. A pianist's right hand
// is the top staff of the system and has nothing above it; a mark asking
// for the staff above would otherwise draw the note off the top of the
// page, which is worse than ignoring it.
func (e *engraver) crossOf(el model.Element) model.CrossStaff {
	m := marksOfElement(el)
	if m == nil || m.Cross == model.OnOwnStaff {
		return model.OnOwnStaff
	}
	if _, ok := e.cross.clefFor(m.Cross); !ok {
		return model.OnOwnStaff
	}
	return m.Cross
}

// setCross tells a staff where its neighbours are and what they are
// written in.
//
// dyAbove and dyBelow are passed in rather than worked out, because only
// the caller knows them: the walk that draws the staves is the thing that
// decides how far apart they go.
func (e *engraver) setCross(parts []*model.Part, pi, at int, dyAbove, dyBelow float64) {
	e.cross = crossContext{}
	if pi > 0 {
		e.cross.above = e.clefAt(parts[pi-1], at)
		e.cross.hasAbove = true
		e.cross.dyAbove = dyAbove
	}
	if pi+1 < len(parts) {
		e.cross.below = e.clefAt(parts[pi+1], at)
		e.cross.hasBelow = true
		e.cross.dyBelow = dyBelow
	}
}

// crossBeamY is where the beam of a group that crosses the staves goes.
//
// Between them, which is where a reader expects it and where there is room
// for it: a beam over the upper staff with stems running down past the
// lower one's top line crosses everything written between the two, and a
// beam under the lower staff does the same upwards.
//
// Midway between the staff lines nearest each other — the bottom line of
// the upper staff and the top line of the lower — so the stems coming from
// each side are about as long as each other.
func (e *engraver) crossBeamY(g []*placed) (float64, bool) {
	below, above := false, false
	for _, p := range g {
		switch p.cross {
		case model.OnStaffBelow:
			below = true
		case model.OnStaffAbove:
			above = true
		}
	}
	// Short of a stem's length from no notehead on either side, wherever
	// the middle falls: staves set close together would otherwise give
	// the notes stubs of stems, and a beam hugging the notes it joins
	// reads as a line ruled under them rather than as their beam.
	const shortest = 2.5
	// From the staves' near lines, not from their tops. Measured from the
	// upper staff's top line, the middle was two spaces up — the beam ran
	// along under the treble staff and the left hand's notes written up
	// there hung off it on stems a space long.
	clamp := func(y float64, crossDown bool) float64 {
		// The heads on each side: the ones written across, and the ones at
		// home, with the beam to stay a stem's length from both.
		near, far := math.Inf(-1), math.Inf(1)
		for _, p := range g {
			top := e.y + p.crossDY + stepY(float64(p.hi))
			bot := e.y + p.crossDY + stepY(float64(p.lo))
			if (p.cross != model.OnOwnStaff) == crossDown {
				near = math.Max(near, bot) // above the beam
			} else {
				far = math.Min(far, top) // below the beam
			}
		}
		if !math.IsInf(near, -1) {
			y = math.Max(y, near+shortest)
		}
		if !math.IsInf(far, 1) {
			y = math.Min(y, far-shortest)
		}
		return y
	}
	switch {
	case below && !above:
		// This staff and the one under it: between this staff's bottom
		// line and that staff's top line.
		return clamp(e.y+(e.staffH()+e.cross.dyBelow)/2, false), true
	case above && !below:
		// The staff above and this one: between that staff's bottom line
		// and this one's top.
		return clamp(e.y+(e.cross.dyAbove+e.staffH())/2, true), true
	case above && below:
		// Both at once is a figure spread over three staves, which is not
		// a thing a pianist's hands can do and not a thing this draws.
		// The beam stays where it would have been.
		return 0, false
	}
	return 0, false
}
