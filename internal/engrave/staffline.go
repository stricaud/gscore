/*
 * staffline.go
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
	"sort"
)

// lineEnd is one end of a line to another staff, where it was drawn.
type lineEnd struct {
	start, stop bool
	measure     int
	at          float64 // when in the bar, in whole notes
	staff       int
	// x0 and x1 are the notehead's (or the rest's) left and right, top and
	// bottom its highest and lowest point.
	x0, x1      float64
	top, bottom float64
	rest        bool
	// tipX and tipY are the far end of its stem, past the beam where it
	// has one, and tip says it has a stem at all. A line that leaves on
	// the side the stem points leaves from there.
	tipX, tipY float64
	tip        bool
	// ref is the element's number, which the editor names it by.
	ref int
}

// restInkAbove is how far a rest's ink reaches above the point it is
// placed by.
const restInkAbove = 0.6

// lineGap is how far a line to another staff stands off what it joins.
const lineGap = 0.5

// drawStaffLines joins each start of a line to another staff to the next end
// after it, and forgets them: a line that has not arrived by the end of the
// system is not drawn, and the next system begins with none.
//
// From the right of the one to the left of the other, leaving each from the
// side that faces the other: the bottom of a note in the upper staff, the
// top of a rest in the lower. Drawn that way it reads as the line of music
// going on, which is all it is there to say.
func (e *engraver) drawStaffLines() {
	ends := e.lineEnds
	e.lineEnds = nil
	sort.SliceStable(ends, func(i, j int) bool {
		if ends[i].measure != ends[j].measure {
			return ends[i].measure < ends[j].measure
		}
		return ends[i].at < ends[j].at
	})
	used := make([]bool, len(ends))
	for i, a := range ends {
		if !a.start {
			continue
		}
		for j := i + 1; j < len(ends); j++ {
			b := ends[j]
			if used[j] || !b.stop || b.staff == a.staff {
				continue
			}
			used[j] = true
			e.joinStaves(a, b)
			break
		}
	}
}

// joinStaves draws the line from a to b.
//
// From the side of each that faces the other. Where that side is the
// stem's, the line leaves from beyond the stem's end and the beam on it:
// drawn from the notehead it ran down through the stem and the beam, and a
// line through a beam reads as one more stem, not as the music moving to the
// other hand. Where it is not, it leaves from the notehead, or from the rest.
func (e *engraver) joinStaves(a, b lineEnd) {
	down := b.top > a.bottom
	from := func(p lineEnd, toward bool) (float64, float64) {
		// toward: this end faces downwards (the other end is below it).
		if p.tip && (p.tipY > p.bottom) == toward {
			dy := lineGap
			if !toward {
				dy = -dy
			}
			return p.tipX, p.tipY + dy
		}
		// A rest is placed by a point its ink reaches above — the dot
		// of an eighth rest is up there — so it is given that much more.
		top := p.top
		if p.rest {
			top -= restInkAbove
		}
		if toward {
			return (p.x0 + p.x1) / 2, p.bottom + lineGap
		}
		return (p.x0 + p.x1) / 2, top - lineGap
	}
	x1, y1 := from(a, down)
	x2, y2 := from(b, !down)
	// The line leans into the music, never back against it: it starts a
	// little after the first and arrives a little before the second.
	x1 += lineGap
	x2 -= lineGap * 0.5
	if x2 < x1 {
		x2 = x1
	}
	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x1, Y1: y1, X2: x2, Y2: y2,
		Thickness: e.opt.Defaults.StemThickness * 1.2,
	})
	e.lay.StaffLines = append(e.lay.StaffLines, StaffLinePos{
		From: a.ref, To: b.ref, X1: x1, Y1: y1, X2: x2, Y2: y2,
	})
}

// stemReach is where p's stem ends, past the beam on the end of it, for the
// note whose heads run from headTop to headBot: the stem's x, the far end's
// y, and whether it has a stem.
func (e *engraver) stemReach(p *placed, headTop, headBot float64) (float64, float64, bool) {
	const touch = 0.6
	if e.staffStrokes > len(e.lay.Lines) || e.staffQuads > len(e.lay.Quads) {
		return 0, 0, false
	}
	x0, x1 := p.x-0.2, p.x+noteheadWidth(p.value)+0.2
	for _, l := range e.lay.Lines[e.staffStrokes:] {
		if math.Abs(l.X1-l.X2) > 0.01 || l.X1 < x0 || l.X1 > x1 ||
			l.Thickness > e.opt.Defaults.StemThickness+1e-9 {
			continue
		}
		lo, hi := math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2)
		var tip float64
		switch {
		case hi > headBot-touch && hi < headBot+touch && lo < headTop:
			tip = lo // up from the lowest head
		case lo > headTop-touch && lo < headTop+touch && hi > headBot:
			tip = hi // down from the highest
		default:
			continue
		}
		// The beam on the end of it.
		for _, q := range e.lay.Quads[e.staffQuads:] {
			qx0, qx1, qlo, qhi := q.X[0], q.X[0], q.Y[0], q.Y[0]
			for i := range q.X {
				qx0, qx1 = math.Min(qx0, q.X[i]), math.Max(qx1, q.X[i])
				qlo, qhi = math.Min(qlo, q.Y[i]), math.Max(qhi, q.Y[i])
			}
			if l.X1 < qx0-0.2 || l.X1 > qx1+0.2 || tip < qlo-touch || tip > qhi+touch {
				continue
			}
			if tip == lo {
				tip = math.Min(tip, qlo)
			} else {
				tip = math.Max(tip, qhi)
			}
		}
		return l.X1, tip, true
	}
	return 0, 0, false
}
