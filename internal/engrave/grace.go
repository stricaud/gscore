/*
 * grace.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	// graceScale is how big a grace note is against a full-size one. Three
	// fifths is the figure the engraving books give for cue size, and it is
	// small enough to read as small beside a full note without becoming a
	// speck of its own.
	graceScale = 0.6
	// graceGap is the space between two grace notes of one group.
	graceGap = 0.34
	// graceAccGap is the space between an accidental and the small note it
	// belongs to.
	graceAccGap = 0.16
	// graceToNote is the space between the group and the note it leans on.
	graceToNote = 0.55
	// graceStem is how long a grace note's stem is: a full stem at cue size,
	// so that it stays in proportion with the head if either is changed.
	graceStem = stemLength * graceScale
	// graceStemMin is the shortest a stem may be cut back to when the beam
	// above it slopes away. Below this the note stops reading as a stemmed
	// note at all.
	graceStemMin = 1.5
	// graceBeamSlope is the steepest a beam over a group may run, in staff
	// spaces per staff space of width. Engravers cap the slope rather than
	// following the pitches exactly, because a steep beam reads as an error.
	graceBeamSlope = 0.3
	// graceSlashReach is how far the stroke through the stem or beam extends
	// on each side of it.
	graceSlashReach = 0.6
	// graceSize is the type size a grace glyph is drawn at, in staff spaces.
	//
	// A SMuFL glyph's type size is four staff spaces — that is the convention
	// the whole standard rests on — so a cue-size glyph is four spaces times
	// the cue scale. Passing the scale on its own asks for a glyph a sixth of
	// the size intended, which is what the PDF drew while the SVG, through a
	// bug of its own, drew them full size: two renderers, two wrong answers,
	// neither of them the one asked for.
	graceSize = 4 * graceScale
)

// gracePlacement is where every piece of a grace group goes.
//
// It is worked out once and used both to budget the room and to draw into it.
// Two separate calculations of the same thing is how the group came to be
// drawn a whole accidental to the right of the space reserved for it.
type gracePlacement struct {
	// Each is measured from the left edge of the group.
	heads []float64
	// accs holds the accidental's x where there is one, and a negative
	// number where there is not.
	accs  []float64
	width float64
}

// graceLayout measures a group.
func graceLayout(g *model.GraceGroup) gracePlacement {
	out := gracePlacement{}
	if g == nil || len(g.Notes) == 0 {
		return out
	}
	head := smufl.Advance("noteheadBlack") * graceScale
	x := 0.0
	for _, n := range g.Notes {
		acc := -1.0
		if a := smufl.AccidentalGlyph(n.Accidental); a != "" {
			acc = x
			x += smufl.Advance(a)*graceScale + graceAccGap
		}
		out.accs = append(out.accs, acc)
		out.heads = append(out.heads, x)
		x += head + graceGap
	}
	x -= graceGap

	// A single grace note carries a flag, which hangs off the right of its
	// stem and has to be inside the budget or it prints on the note the
	// group belongs to. A group of two or more is beamed instead, and a beam
	// stops at the last stem.
	if len(g.Notes) == 1 {
		x += smufl.Advance("flag8thUp") * graceScale
	}
	out.width = x + graceToNote
	return out
}

// graceWidth is the room a grace group needs in front of its note.
func graceWidth(g *model.GraceGroup) float64 {
	return graceLayout(g).width
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
// They are laid out from the left edge of the room reserved for them, which
// starts before the main note's own accidental: a sharp in front of the note
// and a grace group in front of that are two things in the same place
// otherwise, and the group was drawn over the sharp.
func (e *engraver) drawGrace(part *model.Part, p *placed) {
	m := p.marks()
	if m == nil || m.Grace == nil || len(m.Grace.Notes) == 0 {
		return
	}
	g := m.Grace
	lay := graceLayout(g)

	var el model.Element
	switch {
	case p.note != nil:
		el = p.note
	case p.chord != nil:
		el = p.chord
	}
	left := p.x - e.accidentalLead(el) - lay.width

	head := smufl.Advance("noteheadBlack") * graceScale
	t := e.opt.Defaults.StemThickness * graceScale

	// The heads first, and the stem positions they imply. The stem always
	// points up, whatever the note beside it does: that is the convention,
	// and it is why a grace note is recognisable at a glance without
	// measuring its size against its neighbours.
	xs := make([]float64, len(g.Notes))
	ys := make([]float64, len(g.Notes))
	for i, n := range g.Notes {
		step := StaffStep(part.Clef, n.Pitch)
		x := left + lay.heads[i]
		y := e.y + stepY(float64(step))
		xs[i], ys[i] = x+head-t/2, y

		if lay.accs[i] >= 0 {
			a := smufl.AccidentalGlyph(n.Accidental)
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: a, X: left + lay.accs[i], Y: y, Size: graceSize,
			})
		}
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "noteheadBlack", X: x, Y: y, Size: graceSize,
		})
		e.graceLedgerLines(x, head, step)
	}

	if len(g.Notes) == 1 {
		top := ys[0] - graceStem
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: xs[0], Y1: ys[0], X2: xs[0], Y2: top, Thickness: t})
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "flag8thUp", X: xs[0], Y: top, Size: graceSize,
		})
		if g.Slashed {
			e.graceSlash(xs[0], (ys[0]+top)/2, t)
		}
		return
	}

	// Two or more are beamed. They were flagged, one flag each, and a flag
	// hangs off the right of its stem far enough to print on the next small
	// notehead — which is what a group of grace notes looked like. Beaming
	// them is both what fixes that and what an engraver does.
	y0, y1 := e.graceBeam(xs, ys)
	beamAt := func(x float64) float64 {
		if xs[len(xs)-1] == xs[0] {
			return y0
		}
		f := (x - xs[0]) / (xs[len(xs)-1] - xs[0])
		return y0 + (y1-y0)*f
	}
	for i := range xs {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: xs[i], Y1: ys[i], X2: xs[i], Y2: beamAt(xs[i]), Thickness: t,
		})
	}
	bt := e.opt.Defaults.BeamThickness * graceScale
	e.beamQuad(xs[0]-t/2, y0, xs[len(xs)-1]+t/2, y1, bt, 1)

	if g.Slashed {
		// Through the beam, in front of the first stem, which is where the
		// stroke goes on a beamed group: it says how the whole group is
		// played, and saying it once is how it is written.
		e.graceSlash(xs[0]-head*0.3, (y0+ys[0])/2, t)
	}
}

// graceBeam decides where the beam over a group runs.
//
// It follows the pitches, because a beam that ignores them reads as wrong,
// but its slope is capped and it is lifted until the shortest stem under it
// is still long enough to read as a stem.
func (e *engraver) graceBeam(xs, ys []float64) (y0, y1 float64) {
	// y grows downward, so the highest note has the smallest y.
	span := xs[len(xs)-1] - xs[0]
	y0, y1 = ys[0]-graceStem, ys[len(ys)-1]-graceStem
	if span > 0 {
		if max := span * graceBeamSlope; y1-y0 > max {
			y1 = y0 + max
		} else if y1-y0 < -max {
			y1 = y0 - max
		}
	}
	// Lift the whole beam until no stem is shorter than the minimum.
	lift := 0.0
	for i := range xs {
		at := y0
		if span > 0 {
			at = y0 + (y1-y0)*(xs[i]-xs[0])/span
		}
		if short := graceStemMin - (ys[i] - at); short > lift {
			lift = short
		}
	}
	return y0 - lift, y1 - lift
}

// graceSlash draws the stroke that makes a group an acciaccatura.
//
// The stroke is the only thing that tells a player to crush the notes in
// before the beat rather than take their written value out of the note they
// precede, so it is drawn rather than implied.
func (e *engraver) graceSlash(x, y, t float64) {
	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x - graceSlashReach, Y1: y + graceSlashReach*0.8,
		X2: x + graceSlashReach, Y2: y - graceSlashReach*0.8,
		Thickness: t * 1.5,
	})
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
