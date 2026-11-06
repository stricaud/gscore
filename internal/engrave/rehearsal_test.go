/*
 * rehearsal_test.go
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

	"github.com/stricaud/gscore/internal/textmetric"
)

// The letter sits in the middle of its box.
//
// It used to sit over the right-hand edge. The box is the width of the
// letter plus four tenths of a space either side, and the width came from a
// reckoning that gave every character 0.52 of the type size whatever it was
// — so a bold C, which is 0.722, came out 0.485 spaces wider than the box
// had been built for, and spent its last 0.085 outside it.
//
// An M would have crossed the edge by a whole staff space and a W by more.
// Asked of those as well, because a rehearsal sequence that runs past L gets
// to them, and because the error is the width of the letter: the narrow ones
// nearly fit and hide it.
func TestARehearsalLetterIsCentredInItsBox(t *testing.T) {
	const pad = 0.4
	for _, text := range []string{"A", "B", "C", "E", "I", "M", "W", "A1", "12"} {
		w := boldWidth(text, jumpTextSize) + pad*2
		ink := textmetric.Width(text, jumpTextSize, textmetric.Style{Bold: true})

		// Where drawRehearsalMark puts them, for a mark starting at x.
		const x = 10.0
		left, right := x-pad, x-pad+w
		centre := x - pad + w/2
		inkLeft, inkRight := centre-ink/2, centre+ink/2

		if got := inkLeft - left; math.Abs(got-pad) > 0.001 {
			t.Errorf("%q: %.3f of air on the left, want %.3f", text, got, pad)
		}
		if got := right - inkRight; math.Abs(got-pad) > 0.001 {
			t.Errorf("%q: %.3f of air on the right, want %.3f", text, got, pad)
		}
	}
}

// And the drawing agrees with that arithmetic.
//
// The box is four lines and the letter is a Text with an anchor, so the two
// are written in different places and could disagree. They did: the box was
// built around a width the letter was never measured at.
func TestTheRehearsalBoxHoldsItsLetter(t *testing.T) {
	for _, text := range []string{"A", "C", "M", "W"} {
		e := &engraver{opt: DefaultOptions(), lay: &Layout{}}
		const x, row = 10.0, 5.0
		e.drawRehearsalMark(text, x, row)

		if len(e.lay.Texts) != 1 {
			t.Fatalf("%q: %d texts drawn, want 1", text, len(e.lay.Texts))
		}
		got := e.lay.Texts[0]
		if got.Anchor != "middle" {
			t.Errorf("%q: the letter is anchored %q, so it is not centred on "+
				"anything", text, got.Anchor)
		}
		if !got.Bold {
			t.Errorf("%q: the letter is not bold, and it is measured as "+
				"though it were", text)
		}

		// The box: four lines, whose leftmost and rightmost are its edges.
		if len(e.lay.Lines) != 4 {
			t.Fatalf("%q: %d lines drawn, want the 4 of a box", text, len(e.lay.Lines))
		}
		left, right := math.Inf(1), math.Inf(-1)
		for _, l := range e.lay.Lines {
			left = math.Min(left, math.Min(l.X1, l.X2))
			right = math.Max(right, math.Max(l.X1, l.X2))
		}

		// The letter, centred on its anchor.
		ink := textmetric.Width(text, got.Size, textmetric.Style{Bold: true})
		inkLeft, inkRight := got.X-ink/2, got.X+ink/2

		if inkLeft < left {
			t.Errorf("%q: the letter starts %.3f before the left edge of its box",
				text, left-inkLeft)
		}
		if inkRight > right {
			t.Errorf("%q: the letter runs %.3f past the right edge of its box",
				text, inkRight-right)
		}
		// And squarely in the middle of it.
		if off := (inkLeft - left) - (right - inkRight); math.Abs(off) > 0.001 {
			t.Errorf("%q: %.3f more air on one side than the other", text, off)
		}
	}
}
