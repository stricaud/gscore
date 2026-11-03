/*
 * technique.go
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
	"github.com/stricaud/gscore/internal/smufl"
)

// Drawing how a note is played on a fretted instrument.
//
// Three shapes between them cover the lot. A hammer-on, a pull-off and a
// slide join two notes, and are drawn in the gap between them: a slur with
// H or P over it, or the stroke a tab writes a slide with. A bend leaves the
// one note and goes up, so it is an arrow rising to the right with how far
// it goes written at the head. Everything else — the mutes, the harmonics,
// the tap — is a word set over the note, and vibrato is a wave.
//
// None of it is tab-only. The same marks are written over the staff of
// pitches beside the tab, and a part printed without its tab has to carry
// them or it has lost the instruction. So this draws against the staff it
// is given and asks nothing about which kind it is — on a tab the notes sit
// at their strings and on a staff at their pitches, and the arc between two
// of them is the same arc either way.

const (
	// techGap is the air left at each end of a link, so the mark starts
	// clear of the fret number or the notehead it leaves.
	techGap = 0.45

	// techArc is how far a hammer-on or pull-off slur bulges from the
	// straight line between its two notes.
	techArc = 0.85

	// techTextSize is the type the letters are set in, in staff spaces.
	// Smaller than a fret number, since they are read as a hint on top of
	// the notes rather than as part of them.
	techTextSize = 1.0

	// techThickness is the weight of the slurs, strokes and arrows.
	techThickness = 0.12

	// techBand is how tall a word set over the staff is, for the skyline
	// to keep the next thing clear of it.
	techBand = 1.3

	// bendRise is how far a bend arrow climbs, and bendRun how far right
	// it reaches, in staff spaces.
	bendRise = 2.4
	bendRun  = 1.3

	// bendHead is the length of the arrowhead's barbs.
	bendHead = 0.42

	// vibratoWidth is the least a vibrato wave is drawn at. A wave shorter
	// than this reads as a smudge, so a vibrato on a note with nothing
	// after it still gets a wave somebody can see.
	vibratoWidth = 1.6
)

// techLink is the mark that joins this note to the next: which one, and
// where on the page the note it left from is.
type techLink struct {
	kind model.Technique
	x, y float64
}

// linkFrom is the technique on an element that reaches to the next note, or
// nil. Only one: a note is hammered onto or slid from, not both.
func linkFrom(m *model.Marks) (model.Technique, bool) {
	for _, t := range m.Techniques {
		if t.Links() {
			return t, true
		}
	}
	return model.HammerOn, false
}

// drawTechLink draws the mark between the note it left and the note it
// arrived at.
func (e *engraver) drawTechLink(from techLink, x2, y2 float64) {
	x1, y1 := from.x, from.y
	dx, dy := x2-x1, y2-y1
	span := math.Hypot(dx, dy)
	if span <= techGap*2 {
		return
	}
	ux, uy := dx/span, dy/span
	x1 += ux * techGap
	y1 += uy * techGap
	x2 -= ux * techGap
	y2 -= uy * techGap

	switch from.kind {
	case model.SlideUp, model.SlideDown:
		// A slide is the stroke itself and carries no letter. Which way
		// it leans is the instruction, and on a tab the two fret numbers
		// are usually level — 5/7 is written on one string — so the lean
		// is put there rather than taken from where the notes happen to
		// sit. Written between the numbers at whatever angle, it reads
		// the same as the / and \ a plain-text tab uses.
		lean := 0.55
		if from.kind == model.SlideDown {
			lean = -lean
		}
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x1, Y1: y1 + lean, X2: x2, Y2: y2 - lean,
			Thickness: techThickness,
		})
		return
	}

	// A hammer-on or a pull-off: the slur a guitar book draws, with the
	// letter over its middle.
	//
	// Always arcing upwards. A slur between two notes of a phrase goes on
	// the side away from the stems, which is a decision about the phrase;
	// this one is a label and belongs where the label can be read, which
	// on a tab — where there are no stems to be on the far side of — is
	// above, every time.
	mx, my := (x1+x2)/2, (y1+y2)/2
	e.lay.Curves = append(e.lay.Curves, Curve{
		X1: x1, Y1: y1, CX: mx, CY: my - techArc*2, X2: x2, Y2: y2,
		Thickness: techThickness,
	})
	if s := from.kind.Label(); s != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: s, X: mx, Y: my - techArc - techTextSize*0.55,
			Size: techTextSize, Anchor: "middle", Italic: true,
		})
	}
}

// drawTechniques draws the marks on one note that are about that note
// alone: the bends, the mutes, the harmonics and the vibrato.
//
// The links are not here. They need the note after this one and are opened
// and closed by spanAt, the way a slide is.
func (e *engraver) drawTechniques(m *model.Marks, p *placed) {
	if m == nil || len(m.Techniques) == 0 {
		return
	}
	x := p.x + noteheadWidth(p.value)/2
	for _, t := range m.Techniques {
		switch {
		case t.Links():
			// Handled by spanAt, which has the note it reaches.
		case t.Bends():
			e.drawBend(t, x, e.y+stepY(float64(p.hi)))
		case t == model.Vibrato || t == model.WideVibrato:
			e.waitVibrato(t, x)
		default:
			e.waitTechWord(t.Label(), x)
		}
	}
}

// drawBend draws the arrow a bend is written with: up and to the right from
// the note, with how far it goes at the head.
//
// Up from the top of the note rather than from its middle, so that on a
// staff of pitches the arrow leaves the notehead instead of crossing it.
func (e *engraver) drawBend(t model.Technique, x, top float64) {
	y0 := top - 0.6
	x1, y1 := x+bendRun, y0-bendRise

	switch t {
	case model.PreBend:
		// Bent before it is struck, so there is no rise to draw: the
		// arrow goes straight up from the note, which is how the books
		// print it.
		x1 = x
	case model.BendRelease:
		// Up and then back down, which is the whole instruction: the
		// second arrow is what says to let it go again.
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x1, Y1: y1, X2: x1 + bendRun, Y2: y0,
			Thickness: techThickness,
		})
		e.arrowhead(x1+bendRun, y0, 0, 1)
	}

	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x, Y1: y0, X2: x1, Y2: y1, Thickness: techThickness,
	})
	dx, dy := x1-x, y1-y0
	if n := math.Hypot(dx, dy); n > 0 {
		e.arrowhead(x1, y1, dx/n, dy/n)
	}

	if s := t.Label(); s != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: s, X: x1, Y: y1 - techTextSize*0.5,
			Size: techTextSize, Anchor: "middle",
		})
	}
}

// arrowhead draws the two barbs of an arrow pointing along (ux, uy).
func (e *engraver) arrowhead(x, y, ux, uy float64) {
	px, py := -uy, ux
	for _, s := range []float64{1, -1} {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x, Y1: y,
			X2:        x - ux*bendHead + px*s*bendHead*0.6,
			Y2:        y - uy*bendHead + py*s*bendHead*0.6,
			Thickness: techThickness,
		})
	}
}

// waitTechWord sets a word over the staff, clear of whatever is already
// there: "P.M.", "N.H.", "T".
func (e *engraver) waitTechWord(s string, x float64) {
	if s == "" {
		return
	}
	w := textWidth(s, techTextSize)
	e.wait(waitingMark{
		x0: x - w/2, x1: x + w/2, height: techBand, gap: 0.4, rank: rankText,
		draw: func(edge float64) {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: s, X: x, Y: edge - 0.25,
				Size: techTextSize, Anchor: "middle", Italic: true,
			})
		},
	})
}

// waitVibrato draws the wave over the note, also clear of the staff.
//
// Set from the font rather than drawn. Bravura cuts guitarVibratoStroke and
// guitarWideVibratoStroke to be laid end to end into a wave of any length,
// which is what a vibrato held over four beats needs and what a stroked
// line of our own would only approximate — and a wide vibrato is a glyph of
// its own rather than the same wave drawn deeper, so the two are told apart
// the way the font means them to be.
func (e *engraver) waitVibrato(t model.Technique, x float64) {
	name := "guitarVibratoStroke"
	if t == model.WideVibrato {
		name = "guitarWideVibratoStroke"
	}
	w := smufl.Advance(name)
	if w <= 0 {
		return
	}
	// At least one stroke, and enough of them to cover the note.
	n := int(math.Max(1, math.Round(vibratoWidth/w)))
	run := w * float64(n)
	e.wait(waitingMark{
		x0: x - run/2, x1: x + run/2,
		height: techBand, gap: 0.4, rank: rankText,
		draw: func(edge float64) {
			y := edge - 0.4
			for i := 0; i < n; i++ {
				e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
					Name: name, X: x - run/2 + w*float64(i), Y: y,
				})
			}
		},
	})
}
