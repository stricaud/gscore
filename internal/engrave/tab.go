/*
 * tab.go
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
	"strconv"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Tablature: a line for each string, and on it the fret to stop it at.
//
// It is the same music as the staff of pitches beside it — the notes keep
// their pitches, and that is what sounds and what transposes — written the
// way a guitarist reads. A part written as tab and the same part written on
// a staff are the same Measures; only the drawing differs.
//
// What is left out is as much of it as what is put in. There are no clefs
// and no key signatures, because a fret number says where to put a finger
// and needs neither. There are no stems, beams or flags: common guitar tab
// leaves the rhythm to the staff above or to the player's ear, and the
// numbers are what the eye is reading. Rests are not drawn for the same
// reason — the file says so too, with showRests off.

const (
	// tabFretSize is the type the numbers are set in, in staff spaces.
	//
	// Smaller than the gap between the lines, so that two frets on
	// neighbouring strings do not touch.
	tabFretSize = 1.15

	// tabNumberPad is the air cut out of a staff line on each side of a
	// number, so the line does not run through the figures.
	tabNumberPad = 0.22
)

// tabStep is where a string sits, as a staff step.
//
// The engraver measures everything on a staff in steps through stepY, and
// giving the strings their own places in the same units is what lets the
// rest of it — the ties, the articulations, the marks placed against an
// outline — go on working without knowing whether it is looking at a staff
// or at a tab.
func tabStep(str int) int {
	return staffHeight*2 - (str-1)*tabLineGap*2
}

// tabPosition is where a note is played: what the file recorded, or the
// first place on the neck that reaches the pitch.
func tabPosition(t *model.Tablature, n *model.Note) (str, fret int, ok bool) {
	if n.String > 0 && n.String <= t.Lines() {
		return n.String, n.Fret, true
	}
	return t.At(n.Pitch)
}

// noteStep is where a note sits on the staff being drawn.
//
// On a staff of pitches that is the clef's answer. On a tablature it is the
// string the note is played on — and saying so here, once, is what lets
// everything else go on as it was: the stems, the beams, the ties and the
// marks all work in staff steps and none of them has to know which kind of
// staff it is looking at.
func (e *engraver) noteStep(c model.Clef, n *model.Note) int {
	if e.tab == nil {
		return StaffStep(c, n.Pitch)
	}
	str, _, ok := tabPosition(e.tab, n)
	if !ok {
		return StaffStep(c, n.Pitch)
	}
	return tabStep(str)
}

// fretNumber writes one note's fret where its notehead would have gone.
//
// It returns false for a note the instrument cannot reach, which is drawn as
// the notehead it is rather than as a fret that does not exist.
func (e *engraver) fretNumber(n *model.Note, x, y float64, index int) bool {
	str, fret, ok := tabPosition(e.tab, n)
	if !ok {
		return false
	}
	_ = str
	s := strconv.Itoa(fret)
	w := textWidth(s, tabFretSize)
	e.lay.Texts = append(e.lay.Texts, Text{
		S: s, X: x, Y: y + tabFretSize*0.36,
		Size: tabFretSize, Anchor: "middle", Ref: index,
	})
	// The line is ruled around the figure rather than through it.
	e.tabGaps = append(e.tabGaps, tabGap{
		y: y, x0: x - w/2 - tabNumberPad, x1: x + w/2 + tabNumberPad,
	})
	return true
}

// tabGap is a piece of a staff line that is not drawn, because a fret
// number is sitting there.
type tabGap struct {
	y      float64
	x0, x1 float64
}

// tabClefName is the word TAB, drawn for as many strings as there are.
func tabClefName(p *model.Part) string {
	if p.Tab.Lines() <= 4 {
		return "4stringTabClef"
	}
	return "6stringTabClef"
}

// tabClefSize is the type size the word is drawn at.
//
// The glyph is cut for a staff whose lines are a space apart — six of them
// make five spaces — and this staff's are further apart than that, so it is
// scaled by as much as the staff is taller. Four spaces is the size the font
// quotes everything at.
func tabClefSize(p *model.Part) float64 {
	return 4 * partH(p) / float64(partLines(p)-1)
}

// tabClefRoom is how wide it is at that size.
func tabClefRoom(p *model.Part) float64 {
	return smufl.Advance(tabClefName(p)) * tabClefSize(p) / 4
}

// tabClef is the "TAB" that stands at the head of the staff in place of a
// clef.
func (e *engraver) tabClef(p *model.Part, x float64) float64 {
	name := tabClefName(p)
	// The glyph is registered on its own middle, so it is placed on the
	// middle of the staff and spans it.
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: x, Y: e.y + partH(p)/2, Size: tabClefSize(p),
	})
	return x + tabClefRoom(p) + e.opt.Spacings.ClefAfter
}

// tabClefWidth is what that takes, for the layout to reserve.
func tabClefWidth(p *model.Part, sp Spacings) float64 {
	return sp.ClefBefore + tabClefRoom(p) + sp.ClefAfter
}
