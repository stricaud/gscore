/*
 * grace.go
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

import (
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Grace notes: the small notes crushed in before the note they belong to.
//
// They are drawn to the left of that note, at two thirds size, with a stem
// that always points up however the note beside them is stemmed — which is
// the convention, and the reason a grace note is recognisable at a glance
// without reading its size against its neighbours.
//
// A stroke through the stem makes it an acciaccatura: played as fast as
// possible before the beat, rather than taking its written value out of the
// note it precedes. The stroke is the only thing that says so, so it is drawn
// rather than implied.

const (
	// graceScale is how big a grace note is against a full-size one.
	graceScale = 0.66
	// graceGap is the space between two grace notes of one group.
	graceGap = 0.3
	// graceToNote is the space between the last grace note and the note it
	// leans on.
	graceToNote = 0.5
	// graceStem is how long a grace note's stem is, in staff spaces. Shorter
	// than a full stem, in proportion with the head.
	graceStem = 2.4
	// graceSlashReach is how far the stroke through the stem extends on each
	// side of it.
	graceSlashReach = 0.55
)

// graceWidth is the room a grace group needs in front of its note.
func graceWidth(g *model.GraceGroup) float64 {
	if g == nil || len(g.Notes) == 0 {
		return 0
	}
	one := smufl.Advance("noteheadBlack")*graceScale + graceGap
	w := one*float64(len(g.Notes)) + graceToNote - graceGap
	// An accidental on the first of them sits further left again, and has to
	// be in the budget or it lands on whatever came before.
	if a := smufl.AccidentalGlyph(g.Notes[0].Accidental); a != "" {
		w += smufl.Advance(a)*graceScale + graceGap
	}
	return w
}

// graceLead is the room an element's grace notes need before its notehead.
func graceLead(el model.Element) float64 {
	if m := marksOf(el); m != nil {
		return graceWidth(m.Grace)
	}
	return 0
}

// drawGrace draws the small notes before a note.
//
// They run right up to the notehead, so they are laid out backwards from it:
// the last grace note is nearest the note it leans on, which is where the eye
// expects it and where the room was budgeted.
func (e *engraver) drawGrace(part *model.Part, p *placed) {
	m := p.marks()
	if m == nil || m.Grace == nil || len(m.Grace.Notes) == 0 {
		return
	}
	g := m.Grace
	head := smufl.Advance("noteheadBlack") * graceScale
	t := e.opt.Defaults.StemThickness * graceScale

	// Backwards from the notehead, less the accidental room graceWidth put at
	// the front of the budget: the notes themselves start after it.
	first := true
	x := p.x - graceWidth(g)
	if a := smufl.AccidentalGlyph(g.Notes[0].Accidental); a != "" {
		x += smufl.Advance(a)*graceScale + graceGap
	}
	for _, n := range g.Notes {
		step := StaffStep(part.Clef, n.Pitch)
		y := e.y + stepY(float64(step))

		if a := smufl.AccidentalGlyph(n.Accidental); a != "" {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: a, Y: y, Size: graceScale,
				X: x - smufl.Advance(a)*graceScale - graceGap,
			})
		}
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "noteheadBlack", X: x, Y: y, Size: graceScale,
		})
		e.graceLedgerLines(x, head, step)

		// The stem points up whatever the note beside it does, and carries an
		// eighth-note flag: a grace note is written as an eighth or shorter
		// whatever its sounding length, which is a convention and not an
		// approximation.
		sx := x + head - t/2
		top := y - graceStem
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: sx, Y1: y, X2: sx, Y2: top, Thickness: t})
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "flag8thUp", X: sx, Y: top, Size: graceScale,
		})

		if g.Slashed && first {
			// Through the stem, halfway up it. On a group of several notes
			// the stroke goes through the first one only — it says how the
			// whole group is played, and saying it once is how it is written.
			my := (y + top) / 2
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: sx - graceSlashReach, Y1: my + graceSlashReach*0.8,
				X2: sx + graceSlashReach, Y2: my - graceSlashReach*0.8,
				Thickness: t * 1.4,
			})
		}
		first = false
		x += head + graceGap
	}
}

// graceLedgerLines draws the short lines a grace note needs off the staff.
//
// They are as short as the head they carry: a full-width ledger line under a
// two-thirds-size notehead sticks out either side and reads as a mistake.
func (e *engraver) graceLedgerLines(x, headWidth float64, step int) {
	d := e.opt.Defaults
	ext := d.LegerLineExtension * graceScale
	draw := func(st int) {
		y := e.y + stepY(float64(st))
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x - ext, Y1: y, X2: x + headWidth + ext, Y2: y,
			Thickness: d.LegerLineThickness,
		})
	}
	for st := 10; st <= step; st += 2 {
		draw(st)
	}
	for st := -2; st >= step; st -= 2 {
		draw(st)
	}
}
