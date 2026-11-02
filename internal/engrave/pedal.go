/*
 * pedal.go
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
)

/*
 * The sustaining pedal, and sliding from one note to another.
 *
 * Two spanners, drawn here because they are both lines between two notes and
 * neither is an arc. What they have in common is where they go: a pedal mark
 * is read under the staff it belongs to, always, and a glissando is read
 * between the two noteheads it joins, always — neither of them has the
 * "whichever side the stems are not" question that every other mark has.
 */

const (
	// pedalBelow is how far under the staff the line runs. Far enough to
	// clear the lowest note a hand plays down there, which on a piano's
	// lower staff is a long way: the mark is read as a row under the music
	// rather than as part of it, so it is better low than crowded.
	pedalBelow = 4.0
	// pedalUprightDepth is how far the hooks at each end turn up towards
	// the staff, and pedalNotchDepth how deep the V is where the pedal is
	// changed.
	pedalUprightDepth = 0.9
	pedalNotchDepth   = 0.9
	// pedalNotchHalfWidth is half the width of that V.
	pedalNotchHalfWidth = 0.45
	// pedalThickness is the weight of the line, which is a hairpin's: both
	// are read under the staff and must not compete with the music.
	pedalThickness = 0.14
	// pedalTextGap is the air between "Ped." and the note it is under.
	pedalTextGap = 0.4
	// pedalHalfSize is the type "1/2" is set in beside a half pedal.
	pedalHalfSize = 1.4

	// glissGap is the air left at each end of a slide, so the line starts
	// clear of the notehead it leaves rather than out of it.
	glissGap = 0.5
	// glissTextSize is the type "gliss." is set in, over the line.
	glissTextSize = 1.5
	// glissWavePeriod is the length of one wave of a wavy slide, measured
	// along the line, and glissWaveDepth how far it swings either side.
	// Small: at reading size a wave of much more than this reads as a
	// drawing rather than as a line with a tremble in it.
	glissWavePeriod = 0.7
	glissWaveDepth  = 0.22
	// glissLabelClear is how far off the line the word over it is set,
	// measured away from the line and not straight up.
	glissLabelClear = 1.5
)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// drawPedal draws the sustaining pedal held across a stretch of music.
//
// The modern form: a line under the staff with a hook turned up at each end,
// and a V wherever the player is told to let the pedal up and press it again
// without lifting the phrase. The old form — "Ped." under the first note and
// a star under the last — is drawn instead where the score asks for it,
// because a great deal of printed music has it and a program that cannot
// show it cannot show that music.
//
// notches are where the pedal is changed along the way.
func (e *engraver) drawPedal(sp *openSpan, to float64, notches []float64, complete bool) {
	y := e.y + e.staffH() + pedalBelow
	// Under whatever is already below the staff here — a low run, a
	// dynamic — rather than through it.
	if edge := e.staffInkline().reach(sp.x, to, true); !math.IsInf(edge, 0) {
		if down := edge + 1.2; down > y {
			y = down
		}
	}

	if sp.above { // the old form: Ped. … *
		e.lay.Glyphs = append(e.lay.Glyphs,
			Glyph{Name: "keyboardPedalPed", X: sp.x, Y: y},
			Glyph{Name: "keyboardPedalUp", X: to, Y: y})
		if sp.amount == 1 {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: "1/2", X: sp.x, Y: y + pedalHalfSize,
				Size: pedalHalfSize, Anchor: "middle",
			})
		}
		e.recordPedal(sp, to, y)
		return
	}

	// The line, with a hook turned up at each end.
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: sp.x, Y1: y, X2: to, Y2: y, Thickness: pedalThickness},
		Line{X1: sp.x, Y1: y, X2: sp.x, Y2: y - pedalUprightDepth,
			Thickness: pedalThickness},
		Line{X1: to, Y1: y, X2: to, Y2: y - pedalUprightDepth,
			Thickness: pedalThickness})

	// And a V at each change: up to the line's own level and back down,
	// which is how a pianist is told to change the pedal on a beat.
	for _, at := range notches {
		if at <= sp.x || at >= to {
			continue
		}
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: at - pedalNotchHalfWidth, Y1: y, X2: at, Y2: y - pedalNotchDepth,
				Thickness: pedalThickness},
			Line{X1: at, Y1: y - pedalNotchDepth, X2: at + pedalNotchHalfWidth, Y2: y,
				Thickness: pedalThickness})
	}
	if sp.amount == 1 {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: "1/2", X: sp.x, Y: y + pedalHalfSize + pedalTextGap,
			Size: pedalHalfSize, Anchor: "start",
		})
	}
	e.recordPedal(sp, to, y)
}

// recordPedal says where the mark ended up, so it can be pointed at and
// taken away: a spanner belongs to no single note on the page, which is why
// nothing in the music could be clicked to reach one.
func (e *engraver) recordPedal(sp *openSpan, to, y float64) {
	e.lay.Spans = append(e.lay.Spans, SpanPos{
		Kind: "pedal", Part: sp.part, Measure: sp.measure,
		From: sp.ref, To: sp.toRef,
		X1: sp.x, Y1: y - pedalUprightDepth, X2: to, Y2: y,
	})
}

// wavyLine draws a line with a tremble in it, from one point to another at
// whatever angle they put it at.
//
// A zigzag of short strokes rather than a true wave: at the size this is
// read, a fine zigzag and a wave are the same mark, and a zigzag follows any
// slope without the font having to be turned on its side.
func (e *engraver) wavyLine(x1, y1, x2, y2 float64) {
	span := math.Hypot(x2-x1, y2-y1)
	if span <= 0 {
		return
	}
	ux, uy := (x2-x1)/span, (y2-y1)/span
	// Perpendicular to the line, which is which way the wave swings.
	px, py := -uy, ux

	n := int(math.Max(2, math.Round(span/(glissWavePeriod/2))))
	at := func(i int) (float64, float64) {
		t := span * float64(i) / float64(n)
		// Alternating either side, and flat at both ends so the line meets
		// the notes it joins rather than starting mid-swing.
		d := glissWaveDepth
		if i%2 == 0 {
			d = -d
		}
		if i == 0 || i == n {
			d = 0
		}
		return x1 + ux*t + px*d, y1 + uy*t + py*d
	}
	for i := 0; i < n; i++ {
		ax, ay := at(i)
		bx, by := at(i + 1)
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: ax, Y1: ay, X2: bx, Y2: by, Thickness: pedalThickness,
		})
	}
}

// drawGlissando draws a slide from one note to another.
//
// Between the two noteheads, clear of both: a line that touched either of
// them would read as a tie gone wrong. A straight line is a portamento and a
// wavy one a glissando in most houses, which is what Wavy on the mark says,
// and "gliss." is written over it where the score asks.
func (e *engraver) drawGlissando(sp *openSpan, to, toY float64, p *placed) {
	x1, y1 := sp.x, sp.y
	x2, y2 := to, toY
	// In from each end, so the line starts and finishes clear of the
	// noteheads it joins.
	dx, dy := x2-x1, y2-y1
	span := math.Hypot(dx, dy)
	if span <= glissGap*2 {
		return
	}
	ux, uy := dx/span, dy/span
	// Perpendicular to the slide, for setting the word beside it.
	px, py := -uy, ux
	x1 += ux * glissGap
	y1 += uy * glissGap
	x2 -= ux * glissGap
	y2 -= uy * glissGap

	if sp.above { // wavy: a glissando that sounds the notes between
		// Drawn rather than set from the font. The font's wiggle pieces lie
		// flat, to be turned on their side by whatever uses them, and a
		// slide runs at whatever angle its two notes put it at: set upright
		// along a steep slide they come out as a staircase instead of a
		// wave.
		e.wavyLine(x1, y1, x2, y2)
	} else {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x1, Y1: y1, X2: x2, Y2: y2, Thickness: pedalThickness,
		})
	}

	if sp.label != "" {
		// Over the middle of the line and clear of it, measured away from
		// the line rather than straight up: on a steep slide, straight up
		// is along the line and the word is printed on it.
		// Always on the upper side. The perpendicular is (-uy, ux) and a
		// slide always runs left to right, so ux is always positive and
		// the upper side is always the negative one — the first version of
		// this chose a sign from the slope and so put the word under
		// every ascending slide.
		off := -glissLabelClear
		e.lay.Texts = append(e.lay.Texts, Text{
			S:    sp.label,
			X:    (x1+x2)/2 + px*off,
			Y:    (y1+y2)/2 + py*off,
			Size: glissTextSize, Anchor: "middle", Italic: true,
		})
	}

	e.lay.Spans = append(e.lay.Spans, SpanPos{
		Kind: "glissando", Part: sp.part, Measure: sp.measure,
		From: sp.ref, To: sp.toRef,
		X1: x1, Y1: math.Min(y1, y2), X2: x2, Y2: math.Max(y1, y2),
	})
}
