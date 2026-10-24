/*
 * room.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

// How much room a system really needs above it.
//
// The room over a staff has to be reserved before the staff is drawn: the
// page is laid out downwards, and where this system starts decides where the
// next one does. So it is counted from the model — is there a chord symbol
// anywhere in these bars, a volta, an octave line, a bar number — and each
// of those adds its height.
//
// A count is an upper bound and not a measurement. It says "there is a chord
// symbol in this system" and reserves a full row for it across the whole
// line, when the symbol may be one bar wide and sit over a staff whose notes
// never leave it. Several of those together, on every system of a song, is
// a page.
//
// What is written here is the other half: engrave once, look at where the
// ink actually went, and engrave again knowing. The second pass cannot
// change what it is measuring — the bars that go on a line are settled by
// the width of the music and not by its height, and everything inside a
// system is placed relative to its own staff — so the two passes draw the
// same systems, the second one closer together.

import "math"

// quadExtent is how far a quadratic curve really reaches, in y.
//
// Not as far as its control point. The outline used while drawing takes the
// box of the ends and the control point, which is safe and cheap and all it
// needs — but this is a measurement, and a slur that is recorded as dipping
// to its control point reserves half as much room again as it uses.
func quadExtent(y0, cy, y1 float64) (lo, hi float64) {
	lo, hi = math.Min(y0, y1), math.Max(y0, y1)
	// The turning point, where the derivative is zero. Off the curve, there
	// is nothing between the ends to find.
	den := y0 - 2*cy + y1
	if den == 0 {
		return lo, hi
	}
	t := (y0 - cy) / den
	if t <= 0 || t >= 1 {
		return lo, hi
	}
	at := (1-t)*(1-t)*y0 + 2*t*(1-t)*cy + t*t*y1
	return math.Min(lo, at), math.Max(hi, at)
}

// need is what one system turned out to want around its staff.
type need struct {
	// above is from the top staff line up to the highest ink.
	above float64
	// below is from the top staff line down to the lowest ink, which is
	// where the system really ends.
	below float64
}

// minRoomAbove is the least a system keeps over its top staff line.
//
// Something is always there, even in the emptiest bar: the top line itself
// has a thickness, a whole-bar rest hangs from the line above the staff, and
// a system set hard against the one over it reads as one staff of ten lines.
const minRoomAbove = 1.2

// roomAbove measures, for each system of a finished drawing, how far its ink
// actually reached above its top staff line.
//
// Keyed by the first bar of the system, which is what the next pass has in
// hand when it comes to lay that system out again.
func roomAbove(lay *Layout) map[int]need {
	if lay == nil || len(lay.Systems) == 0 {
		return nil
	}
	out := make(map[int]need, len(lay.Systems))
	for i, sys := range lay.Systems {
		if sys.Staff <= 0 {
			continue
		}
		// Everything drawn between this system's top and the next one's
		// belongs to this one: that is what the gap between them is for.
		end := math.Inf(1)
		if i+1 < len(lay.Systems) {
			end = lay.Systems[i+1].Top
		}
		highest, lowest := sys.Staff, sys.Staff
		note := func(top, bottom float64) {
			if bottom < sys.Top || top >= end {
				return // something belonging to a neighbour
			}
			if top < highest {
				highest = top
			}
			if bottom > lowest {
				lowest = bottom
			}
		}
		for _, g := range lay.Glyphs {
			up, down := glyphReach(g.Name)
			if g.Size > 0 {
				up, down = up*g.Size/4, down*g.Size/4
			}
			note(g.Y-up, g.Y+down)
		}
		for _, l := range lay.Lines {
			note(math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2))
		}
		for _, c := range lay.Curves {
			note(quadExtent(c.Y1, c.CY, c.Y2))
		}
		for _, q := range lay.Quads {
			for _, y := range q.Y {
				note(y, y)
			}
		}
		for _, t := range lay.Texts {
			// Text is drawn from its baseline, so it reaches up by about
			// its size and down by a descender.
			note(t.Y-t.Size*0.8, t.Y+t.Size*0.25)
		}
		for _, ci := range lay.Circles {
			note(ci.Y-ci.Radius, ci.Y+ci.Radius)
		}

		n := need{above: sys.Staff - highest, below: lowest - sys.Staff}
		if n.above < minRoomAbove {
			n.above = minRoomAbove
		}
		out[sys.First] = n
	}
	return out
}
