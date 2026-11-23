/*
 * tiestem_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the
 * MIT licence; see the LICENSE file at the root of this repository.
 */

package engrave

import "testing"

// A tie under the staff that passes a stem coming down from the other voice
// goes round the end of that stem, and one that passes nothing keeps its
// shape. Bar 24 of Black and Tan Fantasy: the F tied over into the next bar
// had the B natural's stem run straight through it.
func TestATieGoesRoundAStemBetweenItsNotes(t *testing.T) {
	e := &engraver{opt: DefaultOptions(), lay: &Layout{}}
	th := e.opt.Defaults.StemThickness
	const y, cy = 10.0, 11.0 // a tie bowing down from 10 to about 10.5
	if got := e.tieOverStems(0, 6, y, y, cy, 1); got != cy {
		t.Errorf("with nothing between, the arch moved to %.2f", got)
	}
	// A stem down from 7 to 12 at the middle of the tie.
	e.lay.Lines = append(e.lay.Lines, Line{X1: 3, Y1: 7, X2: 3, Y2: 12, Thickness: th})
	got := e.tieOverStems(0, 6, y, y, cy, 1)
	if mid := 0.25*y + 0.5*got + 0.25*y; mid <= 12 {
		t.Errorf("the tie is at %.2f under the stem, which reaches 12", mid)
	}
	// A barline is not a stem: a tie over the bar crosses it.
	e.lay.Lines = []Line{{X1: 3, Y1: 7, X2: 3, Y2: 12, Thickness: e.opt.Defaults.ThinBarlineThickness}}
	if got := e.tieOverStems(0, 6, y, y, cy, 1); got != cy {
		t.Errorf("the tie went round a barline, to %.2f", got)
	}
}
