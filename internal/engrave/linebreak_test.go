/*
 * linebreak_test.go
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
	"testing"
)

// How many bars go on a line.
//
// A system is filled to the margin whatever goes on it, so the question is
// not what fits but what leaves the line closest to the width it has. Taking
// bars while they fit and stopping answers the other question and produces
// lines three-quarters full pulled out by a third, with the bar that would
// have filled them at the head of the next one.

// breaker is an engraver that knows nothing but how long a line is, which
// is all breakAt asks it.
func breaker(width float64) *engraver {
	o := DefaultOptions()
	o.Mode = Paginated
	o.LineWidth = width
	return &engraver{opt: o}
}

// same is a run of bars all of one width, with room to be pressed in.
func same(n int, ideal, least float64) (widths, floors []float64) {
	for i := 0; i < n; i++ {
		widths = append(widths, ideal)
		floors = append(floors, least)
	}
	return widths, floors
}

// A bar that would overflow the line by a hair goes on it, because the line
// is going to be stretched to the margin either way and stretching it by a
// third to avoid a four per cent squeeze is the worse page.
func TestALineTakesTheBarThatAlmostFits(t *testing.T) {
	// Four bars come to 104 on a line of 100; three come to 78.
	widths, floors := same(8, 26, 20)
	if got := breaker(100).breakAt(widths, floors, 0); got != 4 {
		t.Errorf("the line took %d bars, want 4: three of them would be "+
			"stretched by a quarter to save a squeeze of four per cent", got)
	}
}

// But not where the bar would have to go under its own minimum. Past that
// the noteheads are closer together than they are wide, and the line stops
// being readable rather than becoming tighter.
func TestNoBarIsPressedPastItsMinimum(t *testing.T) {
	// The same four bars, but they cannot give anything up: 104 of floor
	// on a line of 100.
	widths, floors := same(8, 26, 26)
	if got := breaker(100).breakAt(widths, floors, 0); got != 3 {
		t.Errorf("the line took %d bars, want 3: the fourth has nothing to "+
			"give and would be drawn over the margin", got)
	}
}

// A line that fits outright is not being squeezed, so the minimum has
// nothing to say about it. Asking too early threw away lines that fitted:
// a bar thick with accidentals has a minimum as wide as its ideal, and
// every such line came back one bar short.
func TestAFittingLineIsNotRefusedForItsMinimum(t *testing.T) {
	// Four bars come to 100 exactly, and none of them can be pressed at
	// all. They still fit.
	widths, floors := same(8, 25, 25)
	if got := breaker(100).breakAt(widths, floors, 0); got != 4 {
		t.Errorf("the line took %d bars, want 4 — they fit as they are", got)
	}
}

// A bar too wide for any line still has to be drawn somewhere.
func TestABarWiderThanTheLineStillGoesOnIt(t *testing.T) {
	widths, floors := same(3, 500, 400)
	if got := breaker(100).breakAt(widths, floors, 0); got != 1 {
		t.Errorf("the line took %d bars, want 1", got)
	}
}

// The head of the system — the names, the bracket, the clef and the key —
// is room the music does not get.
func TestThePrefixComesOffTheLine(t *testing.T) {
	widths, floors := same(8, 25, 20)
	if got := breaker(100).breakAt(widths, floors, 50); got != 2 {
		t.Errorf("the line took %d bars, want 2 of the 50 left to it", got)
	}
}

// Stretching and squeezing are counted the same way, which the ratio on its
// own does not do: pulled out by a third is 1.33 and pressed in by a third
// is 0.75, and those two numbers do not compare. Their logarithms do.
func TestStretchingAndSqueezingWeighTheSame(t *testing.T) {
	out := lineBadness(100, 75) // pressed in to three quarters
	in := lineBadness(100, 133) // pulled out by a third
	if math.Abs(out-in) > 0.01 {
		t.Errorf("a quarter in is %.3f and a third out is %.3f, want them "+
			"as bad as each other", out, in)
	}
	if lineBadness(100, 100) != 0 {
		t.Error("a line that needs no scaling at all is not perfect")
	}
}
