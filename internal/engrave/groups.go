/*
 * groups.go
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

// What is drawn down the left of a system: the staff names, and the braces and
// brackets that say which staves belong together.
//
// A stack of five staves with nothing joining them is five pieces of music on
// one page. The brace that joins a pianist's two hands and the bracket that
// gathers the strings are not decoration: they are what says where one
// instrument ends and the next begins, and a reader uses them before reading a
// single note.

const (
	// nameSize is the type the staff names are set in, in staff spaces.
	nameSize = 1.9
	// nameGap is the air between the name and what follows it.
	nameGap = 1.2
	// groupGap is the air between a bracket and the clefs behind it.
	groupGap = 0.9
	// bracketThickness is how heavy the vertical of a bracket is.
	bracketThickness = 0.5
	// measureNumberSize is the type a bar number is set in.
	measureNumberSize = 1.5
	// stripHeight is how tall the band over a bar is: enough to hit without
	// aiming, and no taller than the bar numbers that sit in it.
	stripHeight = 2.4
	// markerRoom is how much room a boxed rehearsal mark takes, so that
	// anything else written over the same bar goes above it.
	markerRoom = 3.0
)

// partLabel is what a staff is called at the head of a system.
//
// After the first system it is abbreviated, because the full name is already
// known by then and the music is what wants the room. "Violoncello" becomes
// "Vc." the way it does in every printed score.
func partLabel(p *model.Part, first bool) string {
	name := p.Name
	if name == "" {
		name = p.Instrument.String()
	}
	if name == "" || first {
		return name
	}
	return abbreviate(name)
}

// abbreviate shortens a name the way a score does: the first few letters of
// each word, with a full stop.
func abbreviate(name string) string {
	runes := []rune(name)
	if len(runes) <= 4 {
		return name
	}
	// Two words become their initials — "Alto Sax" is "A. Sx." in nobody's
	// score, so only the first word is shortened and the rest dropped.
	cut := 3
	for i, r := range runes {
		if r == ' ' {
			cut = i
			break
		}
	}
	if cut > 3 {
		cut = 3
	}
	return string(runes[:cut]) + "."
}

// labelWidth is how much room the names need, so the music starts in the same
// column on every staff.
//
// It is measured rather than guessed at because a name that does not fit is
// drawn over the clef, which is worse than not drawing it at all.
func (e *engraver) labelWidth(parts []*model.Part, first bool) float64 {
	if !e.opt.ShowPartNames {
		return 0
	}
	widest := 0.0
	for _, p := range parts {
		if w := textWidth(partLabel(p, first), nameSize); w > widest {
			widest = w
		}
	}
	for _, g := range e.score.Groups {
		if g.Name == "" {
			continue
		}
		label := g.Name
		if !first {
			label = abbreviate(label)
		}
		if w := textWidth(label, nameSize); w > widest {
			widest = w
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + nameGap
}

// textWidth guesses how wide a word is.
//
// The font's own metrics are not in hand here — the names are set in the
// reader's serif face, not in Bravura — so this is an average character width,
// chosen generously. Leaving too much room pushes the music right; leaving too
// little puts a name through a clef, and only one of those is a mistake a
// reader notices.
func textWidth(s string, size float64) float64 {
	if s == "" {
		return 0
	}
	return float64(len([]rune(s))) * size * 0.52
}

// drawGroups draws the braces and brackets at the left of a system, and the
// names beside them.
func (e *engraver) drawGroups(parts []*model.Part, left float64, tops []float64, first bool) {
	if len(tops) == 0 {
		return
	}

	for _, g := range e.score.Groups {
		// A group names staves by their number in the score, and the ones
		// drawn are only those not hidden. Hiding the lower half of a piano
		// should not leave its brace reaching into the staff below.
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 || li <= fi || li >= len(tops) {
			continue
		}
		top := tops[fi]
		bottom := tops[li] + staffHeight
		x := left - groupGap

		switch g.Symbol {
		case model.GroupBrace:
			e.drawBrace(x-0.4, top, bottom)
		case model.GroupLine:
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: top, X2: x, Y2: bottom, Thickness: bracketThickness / 2,
			})
		default:
			// A bracket: a heavy vertical with a serif turning right at each
			// end, which is what distinguishes it from a barline.
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: x, Y1: top - 0.2, X2: x, Y2: bottom + 0.2, Thickness: bracketThickness},
				Line{X1: x, Y1: top - 0.2, X2: x + 0.7, Y2: top - 0.5,
					Thickness: bracketThickness * 0.7},
				Line{X1: x, Y1: bottom + 0.2, X2: x + 0.7, Y2: bottom + 0.5,
					Thickness: bracketThickness * 0.7})
		}

		if g.Name != "" && e.opt.ShowPartNames {
			label := g.Name
			if !first {
				label = abbreviate(label)
			}
			e.lay.Texts = append(e.lay.Texts, Text{
				S: label, X: x - groupGap, Y: (top+bottom)/2 + nameSize*0.35,
				Size: nameSize, Anchor: "end",
			})
		}
	}

	if !e.opt.ShowPartNames {
		return
	}
	// A staff inside a named group is already labelled by the group.
	named := map[int]bool{}
	for _, g := range e.score.Groups {
		if g.Name == "" {
			continue
		}
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 {
			continue
		}
		for i := fi; i <= li && i < len(tops); i++ {
			named[i] = true
		}
	}
	for i, p := range parts {
		if i >= len(tops) || named[i] {
			continue
		}
		label := partLabel(p, first)
		if label == "" {
			continue
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			// Centred on the staff's middle line, which is where a name sits
			// in every printed score.
			S: label, X: left - groupGap - 0.4,
			Y: tops[i] + staffHeight/2 + nameSize*0.35,
			Size: nameSize, Anchor: "end",
		})
	}
}

// drawGroupBarlines runs the barline down through the gap between the staves
// of a group, so that a pianist's two hands are one instrument on the page.
func (e *engraver) drawGroupBarlines(x float64, tops []float64) {
	d := e.opt.Defaults
	for _, g := range e.score.Groups {
		if !g.Barlines {
			continue
		}
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 || li <= fi || li >= len(tops) {
			continue
		}
		for i := fi; i < li; i++ {
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: tops[i] + staffHeight, X2: x, Y2: tops[i+1],
				Thickness: d.ThinBarlineThickness,
			})
		}
	}
}

// Sizes for the row of directions over a system.
const (
	// jumpTextSize is the type "D.C. al Fine" and "Fine" are set in. The
	// same size as a rehearsal mark, because they are read in the same
	// glance and one larger than the other would say it mattered more.
	jumpTextSize = 2.4
	// signSize is the type size of the segno and coda glyphs, in staff
	// spaces. They are drawn a little larger than the words beside them,
	// which is how every printed chart sets them.
	signSize = 3.2
	// afterMark is the gap left after something in the row before the next
	// thing may start.
	afterMark = 0.8
)

// drawMarkers writes the rehearsal marks, signs and jump directions over a
// system.
//
// All three go in one row above the top staff, and all three are taken from
// whichever part has them: "from letter B", the segno and "D.S. al Coda" each
// have to mean the same bar to every player in the room, so a direction that
// belonged to one staff would be worse than none.
//
// They share a row because they share a bar more often than not — a chart
// puts the segno at letter A as a matter of course. Each is placed after the
// last thing written over that bar rather than at the bar line, which is what
// keeps the sign from being printed through the letter.
func (e *engraver) drawMarkers(parts []*model.Part, columns []float64, start, end int, top float64) {
	at := func(mi int) *model.Measure {
		for _, p := range parts {
			if mi < len(p.Measures) {
				m := p.Measures[mi]
				if m.Marker != "" || m.Sign != model.NoSign || m.Jump != nil {
					return m
				}
			}
		}
		return nil
	}

	for mi := start; mi < end; mi++ {
		m := at(mi)
		if m == nil {
			continue
		}
		x := columns[mi-start]

		if m.Marker != "" {
			x = e.drawRehearsalMark(m.Marker, x, top) + afterMark
		}
		if g := m.Sign.Glyph(); g != "" {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: g, X: x, Y: top - 1.6, Size: signSize,
			})
			x += smufl.Advance(g)*signSize/4 + afterMark
		}
		if label := m.Sign.Label(); label != "" {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: label, X: x, Y: top - 1.6, Size: jumpTextSize,
				Anchor: "start", Bold: true, Italic: true,
			})
		}

		// The direction is written at the end of the bar, because that is
		// where the player reaches it: a D.C. printed at the head of its bar
		// says to go back before playing the bar it is in.
		if m.Jump != nil {
			if mi-start+1 < len(columns) {
				e.lay.Texts = append(e.lay.Texts, Text{
					S: m.Jump.Label(), X: columns[mi-start+1] - 0.4, Y: top - 1.6,
					Size: jumpTextSize, Anchor: "end", Bold: true, Italic: true,
				})
			}
		}
	}
}

// drawRehearsalMark writes one boxed letter and returns where it ends.
//
// The box is what makes a rehearsal mark findable when somebody says "from B"
// and everybody in the room is looking for a letter.
func (e *engraver) drawRehearsalMark(text string, x, top float64) float64 {
	e.lay.Texts = append(e.lay.Texts, Text{
		S: text, X: x, Y: top - 2.0, Size: jumpTextSize, Anchor: "start", Bold: true,
	})
	w := textWidth(text, jumpTextSize) + 0.8
	t := e.opt.Defaults.StaffLineThickness * 1.3
	y0, y1 := top-4.0, top-1.3
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: x - 0.4, Y1: y0, X2: x - 0.4 + w, Y2: y0, Thickness: t},
		Line{X1: x - 0.4, Y1: y1, X2: x - 0.4 + w, Y2: y1, Thickness: t},
		Line{X1: x - 0.4, Y1: y0, X2: x - 0.4, Y2: y1, Thickness: t},
		Line{X1: x - 0.4 + w, Y1: y0, X2: x - 0.4 + w, Y2: y1, Thickness: t})
	return x - 0.4 + w
}

// drawMeasureNumbers writes the bar numbers over a system.
//
// Where they go depends on how the music is being looked at, because the
// question they answer is different in each case.
//
// On a page, the number belongs over the first bar of each system: that is
// where a player looks when someone says "from bar forty-one", and a number
// over every bar is clutter between the staves.
//
// A continuous strip has only ever one system, so numbering its first bar
// prints a single "1" and nothing else — which is what made the menu entry
// look like it did nothing at all. Editing is also when you most want to find
// a bar by its number, so there every bar is numbered.
func (e *engraver) drawMeasureNumbers(columns []float64, start, end int, top float64) {
	if !e.opt.ShowMeasureNumbers {
		return
	}
	write := func(n int, x float64) {
		if n <= 1 {
			return // bar one needs no telling
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: itoa(n), X: x, Y: top - 1.4, Size: measureNumberSize, Anchor: "start",
		})
	}
	if e.opt.Mode != Strip {
		write(start+1, columns[0])
		return
	}
	for mi := start; mi < end && mi-start < len(columns); mi++ {
		write(mi+1, columns[mi-start])
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// unused keeps the smufl import honest if the brace glyph is ever dropped.
var _ = smufl.Advance

// drawnAt turns a staff's number in the score into its position among the
// ones being drawn, or says it is not being drawn at all.
func (e *engraver) drawnAt(staff int) (int, bool) {
	for i, n := range e.origin {
		if n == staff {
			return i, true
		}
	}
	return 0, false
}

// drawBrace draws the curly brace that joins the staves of one instrument.
//
// Drawn rather than set as a glyph. A brace has to stretch to however many
// staves it joins, and a glyph can only be scaled in both directions at once:
// stretched to a grand staff it comes out four times too wide, and sized to
// look right it does not reach. Fonts solve this by shipping several braces of
// different sizes, which still only fits the sizes they ship.
//
// A brace is two mirrored S-curves: a tip at each end curling towards the
// staves, and a cusp in the middle pointing away from them. Four quadratics
// draw it at any height.
func (e *engraver) drawBrace(x, top, bottom float64) {
	h := bottom - top
	if h <= 0 {
		return
	}
	mid := (top + bottom) / 2
	half := h / 2
	// How far the cusp stands out from the tips. A brace that joins more
	// staves is drawn a little wider, within reason, the way a larger brace
	// in a font is not simply a scaled-up small one.
	w := 0.9 + h*0.02
	if w > 1.6 {
		w = 1.6
	}
	t := e.opt.Defaults.StaffLineThickness * 2.6

	// Each half sweeps from the tip to the cusp in two curves, so the stroke
	// leaves the tip almost vertically and arrives at the cusp almost
	// horizontally — which is what makes it read as a brace rather than as a
	// pair of brackets.
	arm := func(tip float64, dir float64) {
		q1 := tip + dir*half*0.5
		e.lay.Curves = append(e.lay.Curves,
			Curve{
				X1: x, Y1: tip,
				CX: x - w*0.2, CY: tip + dir*half*0.3,
				X2: x - w*0.5, Y2: q1,
				Thickness: t,
			},
			Curve{
				X1: x - w*0.5, Y1: q1,
				CX: x - w*0.85, CY: tip + dir*half*0.72,
				X2: x - w, Y2: mid,
				Thickness: t,
			})
	}
	arm(top, 1)
	arm(bottom, -1)
}
