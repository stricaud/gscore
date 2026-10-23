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
		if g.First < 0 || g.Last >= len(tops) || g.Last <= g.First {
			continue // a group of one staff needs no bracket
		}
		top := tops[g.First]
		bottom := tops[g.Last] + staffHeight
		x := left - groupGap

		switch g.Symbol {
		case model.GroupBrace:
			// The brace is a glyph and has to stretch to the staves it joins,
			// so it is drawn at whatever size makes it that tall. Bravura's
			// brace is four staff spaces from tip to tip at its own size.
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: "brace", X: x - 0.6, Y: (top + bottom) / 2,
				Size: (bottom - top) / 4 * 1.0,
			})
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
		for i := g.First; i <= g.Last && i < len(tops); i++ {
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
		if !g.Barlines || g.First < 0 || g.Last >= len(tops) || g.Last <= g.First {
			continue
		}
		for i := g.First; i < g.Last; i++ {
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: tops[i] + staffHeight, X2: x, Y2: tops[i+1],
				Thickness: d.ThinBarlineThickness,
			})
		}
	}
}

// drawMeasureNumber writes the bar number over the start of a system.
func (e *engraver) drawMeasureNumber(n int, x, top float64) {
	if !e.opt.ShowMeasureNumbers || n <= 1 {
		return
	}
	e.lay.Texts = append(e.lay.Texts, Text{
		S: itoa(n), X: x, Y: top - 1.4, Size: measureNumberSize, Anchor: "start",
	})
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
