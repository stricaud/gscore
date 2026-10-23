/*
 * spanners.go
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

import "github.com/stricaud/gscore/internal/model"

// The marks that reach from one note to another: hairpins, octave lines and
// the volta brackets over the bars of a repeat.
//
// None of these can be drawn where they begin, because where they end is not
// known yet and may be on a later line. So each staff carries what is open,
// and the mark is drawn when it closes — or cut off at the right margin and
// started again on the next system, which is how a crescendo spanning a page
// turn is written on paper.

const (
	// hairpinDrop is how far below the bottom staff line a hairpin sits. It
	// is under the dynamics, since a hairpin usually runs from one to
	// another and crossing them would be unreadable.
	hairpinDrop = 4.6
	// hairpinMouth is how far the wedge opens, top to bottom.
	hairpinMouth = 1.1

	// octaveRise is how far above the top staff line an 8va line runs, and
	// octaveFall how far below the bottom line an 8vb does.
	octaveRise = 3.4
	octaveFall = 4.6

	// voltaRise is how far above the top staff of the system the brackets of
	// a repeat are drawn, and voltaHook how far their ends turn down.
	voltaRise = 3.0
	voltaHook = 1.1
)

// staffSpans returns what is open on a staff, making the record if this is the
// first spanner it has seen.
func (e *engraver) staffSpans(part int) *openSpans {
	s, ok := e.spans[part]
	if !ok {
		s = &openSpans{}
		e.spans[part] = s
	}
	return s
}

// spanAt handles the spanners that open or close at one element.
func (e *engraver) spanAt(m *model.Marks, p *placed) {
	if m == nil {
		return
	}
	open := e.staffSpans(e.partIndex)
	// The middle of the notehead, so a hairpin between two notes reaches from
	// under one to under the other rather than from their left edges.
	x := p.x + noteheadWidth(p.value)/2

	if h := m.Hairpin; h != nil {
		if h.Start && open.hairpin == nil {
			open.hairpin = &openSpan{x: x, kind: h.Kind}
		}
		if h.Stop && open.hairpin != nil {
			e.drawHairpin(open.hairpin, x, true)
			open.hairpin = nil
		}
	}

	if o := m.Octave; o != nil {
		if o.Start && open.octave == nil {
			open.octave = &openSpan{x: x - 0.5, amount: o.Amount, label: o.Label()}
		}
		if o.Stop && open.octave != nil {
			e.drawOctave(open.octave, x+noteheadWidth(p.value)/2, true)
			open.octave = nil
		}
	}
}

// breakSpans ends every open spanner at the right margin of a system and
// leaves it open, so the next system starts it again at its own left edge.
func (e *engraver) breakSpans(part int, right float64) {
	open, ok := e.spans[part]
	if !ok {
		return
	}
	if open.hairpin != nil {
		e.drawHairpin(open.hairpin, right, false)
		open.hairpin.x = -1 // resumed at the start of the next system
	}
	if open.octave != nil {
		e.drawOctave(open.octave, right, false)
		open.octave.x = -1
	}
}

// resumeSpans picks a broken spanner up again at the left of a new system.
func (e *engraver) resumeSpans(part int, left float64) {
	open, ok := e.spans[part]
	if !ok {
		return
	}
	if open.hairpin != nil && open.hairpin.x < 0 {
		open.hairpin.x = left
	}
	if open.octave != nil && open.octave.x < 0 {
		open.octave.x = left
		// The "8va" is written once, where the line begins. A continuation
		// carries the line alone, as it is written on paper.
		open.octave.label = ""
	}
}

// drawHairpin draws the wedge. A crescendo opens to the right from a point; a
// diminuendo starts open and closes to a point.
func (e *engraver) drawHairpin(h *openSpan, to float64, complete bool) {
	if to <= h.x {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.2
	y := e.y + staffHeight + hairpinDrop
	half := hairpinMouth / 2

	// An unfinished wedge is cut at the margin rather than brought to a
	// point, which is what says "this goes on".
	openAt, pointAt := to, h.x
	if h.kind == model.Diminuendo {
		openAt, pointAt = h.x, to
	}
	spread := half
	if !complete && openAt == to {
		// Opening but cut short: it has not reached its full mouth yet.
		spread = half * 0.8
	}
	if !complete && pointAt == to {
		// Closing but cut short: it never reaches the point.
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: openAt, Y1: y - half, X2: to, Y2: y - half*0.3, Thickness: t},
			Line{X1: openAt, Y1: y + half, X2: to, Y2: y + half*0.3, Thickness: t})
		return
	}
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: pointAt, Y1: y, X2: openAt, Y2: y - spread, Thickness: t},
		Line{X1: pointAt, Y1: y, X2: openAt, Y2: y + spread, Thickness: t})
}

// drawOctave draws the broken line with its name at the left, and the hook at
// the end that says where the shift stops.
func (e *engraver) drawOctave(o *openSpan, to float64, complete bool) {
	if to <= o.x {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.1
	above := o.amount > 0
	y := e.y - octaveRise
	if !above {
		y = e.y + staffHeight + octaveFall
	}

	x := o.x
	if o.label != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: o.label, X: x, Y: y + 0.4, Size: 1.9, Anchor: "start", Italic: true, Bold: true,
		})
		x += float64(len(o.label)) * 0.95
	}
	if to <= x {
		return
	}
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: x, Y1: y, X2: to, Y2: y, Thickness: t, Dash: 0.5})
	if complete {
		// The hook turns towards the staff: down for a line above it, up for
		// one below.
		dy := 0.9
		if !above {
			dy = -0.9
		}
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: to, Y1: y, X2: to, Y2: y + dy, Thickness: t})
	}
}

// drawVoltas draws the first-time and second-time brackets over a system.
//
// They are drawn once, above the top staff, because a repeat belongs to the
// piece and not to one instrument in it: every player goes back at the same
// bar. The measures are taken from the first part that has them, which is
// where MusicXML keeps them too.
func (e *engraver) drawVoltas(parts []*model.Part, columns []float64, start, end int, top float64) {
	if len(parts) == 0 || end <= start {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.4
	y := top - voltaRise

	var open *model.Ending
	openX := 0.0

	closeAt := func(x float64, hook bool) {
		if open == nil {
			return
		}
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: openX, Y1: y, X2: x, Y2: y, Thickness: t},
			Line{X1: openX, Y1: y, X2: openX, Y2: y + voltaHook, Thickness: t})
		if hook && !open.Open {
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: x, Y1: y, X2: x, Y2: y + voltaHook, Thickness: t})
		}
		if s := open.Label(); s != "" {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: s, X: openX + 0.5, Y: y + voltaHook - 0.15, Size: 1.9, Anchor: "start",
			})
		}
		open = nil
	}

	for mi := start; mi < end; mi++ {
		var end *model.Ending
		for _, p := range parts {
			if mi < len(p.Measures) && p.Measures[mi].Ending != nil {
				end = p.Measures[mi].Ending
				break
			}
		}
		if end == nil {
			continue
		}
		if end.Start {
			closeAt(columns[mi-start], false)
			open, openX = end, columns[mi-start]
		}
		if end.Stop {
			closeAt(columns[mi-start+1], true)
		}
	}
	// A bracket still open at the end of the line is cut at the margin and
	// has no closing hook; the next system opens one again.
	closeAt(columns[end-start], false)
}
