/*
 * multirest.go
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

/*
 * Bars of rest, counted instead of printed one by one.
 *
 * Eight empty bars printed as eight empty bars is eight bars of nothing to
 * read and a place to lose count. Written as one bar with a thick stroke
 * through it and an 8 over it, it is one thing to see and the number is the
 * answer to the only question the player has.
 *
 * Only where every staff rests. The bars of a score stand in the same
 * columns on every staff, so one staff cannot count bars through a stretch
 * where another is playing: there would be nowhere on the page to print the
 * notes. That is why this is a property of the system and not of a part, and
 * why a run stops the moment any staff has something to say.
 *
 * It stops, too, at anything that has to be printed at a bar — a rehearsal
 * mark, a change of key, a double barline — because a bar that is not drawn
 * cannot carry it. Such a thing at the head of the run is no trouble: that
 * bar is drawn, and whatever is written over or in front of it is written
 * over or in front of the whole run.
 */

// minMultiRest is the shortest run worth collapsing. Two bars is the
// shortest a player can lose count in, and the shortest anybody writes.
const minMultiRest = 2

// How the sign itself is drawn, in staff spaces.
const (
	// multiRestBase is the width of the sign, before the count widens it. A
	// bar of music is wider than a bar of silence, and a multi-measure rest
	// is wider than either: it has to read as a stretch of the piece rather
	// than as one more bar.
	multiRestBase = 10.0
	// multiRestGrowth is how much each bar past the second adds, and
	// multiRestMax where it stops. A hundred bars' rest is not a hundred
	// bars wide — it is one sign with a hundred over it — but a long rest
	// looking longer than a short one is worth a little of the page.
	multiRestGrowth = 0.35
	multiRestMax    = 18.0
	// multiRestMargin is the air between the sign and the barlines either
	// side of it.
	multiRestMargin = 1.4
	// multiRestThickness is how deep the stroke is, and multiRestSerif how
	// far the uprights at its ends reach above and below the middle line.
	multiRestThickness = 1.0
	multiRestSerif     = 1.0
	// multiRestNumberLift is how far over the top line the count is set.
	multiRestNumberLift = 2.0
)

// multiRest is a run of silent bars drawn as one.
type multiRest struct {
	// count is how many bars it stands for, and what is written over it.
	count int
	// close is the barline the last bar of the run ends with, drawn at the
	// end of the sign. A run may finish at a double bar or a repeat — that
	// is where a stretch of rest usually does finish — and the line has to
	// be printed even though the bar that carries it is not.
	close model.BarlineStyle
}

// multiRestWidth is how much of the system one of these asks for.
func multiRestWidth(count int) float64 {
	w := multiRestBase
	if count > minMultiRest {
		w += float64(count-minMultiRest) * multiRestGrowth
	}
	if w > multiRestMax {
		w = multiRestMax
	}
	return w
}

// planMultiRests finds the runs of silent bars in the staves being drawn.
//
// It answers with the head of each run and what it stands for, and with the
// bars that are swallowed by one — the ones to draw nothing for at all.
func planMultiRests(parts []*model.Part, bars int) (map[int]multiRest, map[int]bool) {
	heads := map[int]multiRest{}
	hidden := map[int]bool{}

	for at := 0; at < bars; {
		if !opensMultiRest(parts, at) {
			at++
			continue
		}
		// How far it runs: while every staff goes on resting, and while
		// nothing has to be printed at the bar. A bar that closes with
		// anything but a plain line is the last of the run rather than the
		// first one left out of it, so the line it closes with can be
		// drawn at the end of the sign.
		end := at + 1
		for closedAt(parts, end-1) == model.BarSingle &&
			end < bars && followsMultiRest(parts, end) {
			end++
		}
		if n := end - at; n >= minMultiRest {
			heads[at] = multiRest{count: n, close: closedAt(parts, end-1)}
			for i := at + 1; i < end; i++ {
				hidden[i] = true
			}
		}
		at = end
	}
	return heads, hidden
}

// opensMultiRest reports whether a run may begin at this bar: every staff
// rests through it, and nothing is written at it that the sign would lose.
//
// A rehearsal mark, a new key, a tempo — all of those may stand at the head
// of a run, because the bar that carries them is the bar that is drawn.
func opensMultiRest(parts []*model.Part, at int) bool {
	for _, p := range parts {
		if !barRests(p, at) || !barKeepsItsPlace(p, at) {
			return false
		}
	}
	return true
}

// followsMultiRest reports whether this bar can be swallowed by the run
// before it: it rests, and there is nothing in it to print.
func followsMultiRest(parts []*model.Part, at int) bool {
	if !opensMultiRest(parts, at) {
		return false
	}
	for _, p := range parts {
		if at >= len(p.Measures) {
			continue
		}
		m := p.Measures[at]
		// A sign written at the bar is a sign nobody would see: the bar it
		// belongs to is not drawn.
		if m.Key != nil || m.Time != nil || m.Clef != nil ||
			m.Marker != "" || m.SectionTitle != "" || m.Tempo != nil ||
			m.Sign != model.NoSign {
			return false
		}
	}
	return true
}

// barKeepsItsPlace reports whether a bar can be inside one of these at all.
//
// A volta bracket is drawn over the bars it covers, a D.C. is read at the
// end of the bar it is written at, and a bar that ends a movement is where
// the page starts again. None of the three survives being counted instead
// of drawn, so none of them is ever part of a run.
func barKeepsItsPlace(p *model.Part, at int) bool {
	if at >= len(p.Measures) {
		return true
	}
	m := p.Measures[at]
	return m.Ending == nil && m.Jump == nil && !m.EndsSection
}

// closedAt is the barline a bar ends with, taken from whichever staff writes
// something other than a plain line. A double bar drawn on one staff of a
// system is drawn on all of them.
func closedAt(parts []*model.Part, at int) model.BarlineStyle {
	for _, p := range parts {
		if at >= len(p.Measures) {
			continue
		}
		if s := p.Measures[at].RightBarline; s != model.BarSingle {
			return s
		}
	}
	return model.BarSingle
}

// barRests reports whether this staff has nothing but silence in the bar.
//
// A staff that has run out of music rests: the bars past the end of a short
// part are empty, and a score of staves of different lengths should not be
// stopped from counting by the shortest of them.
func barRests(p *model.Part, at int) bool {
	if at >= len(p.Measures) {
		return true
	}
	m := p.Measures[at]
	// The sign for "play the bar before again" is not silence, whatever is
	// written in the bar underneath it.
	if m.Repeat != 0 {
		return false
	}
	for _, v := range m.Voices {
		for _, el := range v.Elements {
			if _, ok := el.(*model.Rest); !ok {
				return false
			}
			if !bareRest(el) {
				return false
			}
		}
	}
	return true
}

// bareRest reports whether a rest carries nothing that has to be drawn.
//
// A rest can hold a dynamic, a fermata, the end of a hairpin or the end of a
// slur, and each of those is a thing somebody wrote and expects to see. A
// run of bars is only silent in the sense that matters here when its rests
// say nothing at all.
func bareRest(el model.Element) bool {
	m := marksOf(el)
	if m == nil {
		return true
	}
	return len(m.Articulations) == 0 && len(m.Symbols) == 0 &&
		len(m.Texts) == 0 && len(m.Lyrics) == 0 &&
		m.Tuplet == nil && m.Tremolo == nil && m.Clef == nil &&
		m.Trill == nil && m.Hairpin == nil && m.Octave == nil &&
		m.Chord == nil && m.Grace == nil &&
		m.Dynamic == model.DynamicNone &&
		!m.TieStart && !m.TieStop && !m.SlurStart && !m.SlurStop
}

// drawMultiRest draws the sign that stands for a run of silent bars: the
// thick stroke across the bar, the uprights that close it, and the count
// over the staff.
func (e *engraver) drawMultiRest(part *model.Part, b measureBox, left, right float64, run multiRest) {
	e.barIndex = b.index

	// Whatever is written at the head of the bar — a new key, a new meter —
	// is written in front of the sign, exactly as it would be in front of
	// the music it replaces.
	x := left
	if b.prefix > 0 {
		x = e.drawSignatureChange(part, b, x)
	}

	_, room := e.barlineRoom(run.close)
	bx := e.barlineAt(right, room)

	x0, x1 := x+multiRestMargin, bx-multiRestMargin
	if x1-x0 < multiRestBase/2 {
		// A system squeezed past what the sign needs: it is drawn narrow
		// rather than inside out.
		x1 = x0 + multiRestBase/2
	}
	mid := e.y + stepY(4)
	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x0, Y1: mid, X2: x1, Y2: mid, Thickness: multiRestThickness,
	})
	for _, at := range []float64{x0, x1} {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: at, Y1: mid - multiRestSerif, X2: at, Y2: mid + multiRestSerif,
			Thickness: e.opt.Defaults.ThickBarlineThickness,
		})
	}

	// The count, in the figures a time signature is set in, centred over the
	// sign. It is the whole point of the thing: the stroke says "rest" and
	// the number says how long for.
	names := smufl.DigitGlyphs(run.count)
	gx := (x0+x1)/2 - glyphRunWidth(names)/2
	for _, n := range names {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: n, X: gx, Y: e.y - multiRestNumberLift,
		})
		gx += smufl.Advance(n)
	}

	e.drawBarline(run.close, bx)
	e.lay.Barlines = append(e.lay.Barlines, BarlinePos{
		Part: e.partIndex, Measure: b.index + run.count - 1, X: bx,
		Top: e.y, Bottom: e.y + e.staffH(),
		Style: run.close.String(),
	})
}
