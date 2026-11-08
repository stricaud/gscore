/*
 * lastpage_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The foot of the last page, and the size the music is set at to reach it.
 *
 * Two things sent a part over onto a second sheet it did not need. The
 * drawing carries a margin of its own under the last system, and the page
 * already sets one aside — counted twice, a part whose music stopped two
 * spaces inside the paper was turned over for white. And the sizes tried
 * when shrinking stepped past the smallest the rule allows without ever
 * landing on it, so an alto part that fits one sheet at exactly that size
 * was printed across two beside a flute part of the same music on one.
 */

package pdf

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// drawing is a layout of n systems, each a fixed height with a gap between,
// and a margin of its own under the last of them.
func drawing(n int, height, gap, margin float64) *engrave.Layout {
	lay := &engrave.Layout{Width: 100}
	y := margin
	for i := 0; i < n; i++ {
		lay.Systems = append(lay.Systems, engrave.SystemPos{
			Top: y, Bottom: y + height, First: i, Last: i,
		})
		y += height + gap
	}
	lay.Content = y - gap
	lay.Height = lay.Content + margin
	return lay
}

func TestTheMarginUnderTheMusicDoesNotCostASheet(t *testing.T) {
	// Three systems whose music stops inside a page that holds 50, with
	// the drawing's own margin under them taking the canvas past it.
	lay := drawing(3, 13, 2, 4)
	if lay.Content > 50 || lay.Height <= 50 {
		t.Fatalf("the fixture is wrong: content %.1f, canvas %.1f",
			lay.Content, lay.Height)
	}
	if n := len(Bands(lay, 50)); n != 1 {
		t.Errorf("the music ends at %.1f on a page holding 50 and came to "+
			"%d sheets", lay.Content, n)
	}
	// And music that really is too tall still turns the page.
	if n := len(Bands(drawing(4, 13, 2, 4), 50)); n != 2 {
		t.Error("a system past the foot of the page was printed on it anyway")
	}
}

// A layout from before this was recorded says nothing about where its music
// stops. It is read as it always was — by the whole canvas — rather than as
// a drawing with no content at all.
func TestADrawingThatDoesNotSayWhereItsMusicStops(t *testing.T) {
	// A canvas that fits is one sheet, as it always was.
	lay := drawing(3, 12, 2, 4)
	lay.Content = 0
	if lay.Height > 50 {
		t.Fatalf("the fixture's canvas is %.1f, which does not fit", lay.Height)
	}
	if n := len(Bands(lay, 50)); n != 1 {
		t.Errorf("it came to %d sheets, want one", n)
	}
	lay = drawing(5, 13, 2, 4)
	lay.Content = 0
	if n := len(Bands(lay, 50)); n < 2 {
		t.Error("five systems were printed on one page")
	}
}

// The smallest size the rule allows is one of the sizes tried.
func TestTheSmallestTypeAPartIsSetInIsTried(t *testing.T) {
	sizes := tightenSizes(6.2)
	if len(sizes) == 0 {
		t.Fatal("nothing to try")
	}
	last := sizes[len(sizes)-1]
	if last != PartStaffSpace {
		t.Errorf("the sizes tried stop at %.2f, a step above the %.2f the "+
			"rule allows", last, PartStaffSpace)
	}
	for i, s := range sizes {
		if s < PartStaffSpace {
			t.Errorf("size %d is %.2f, smaller than a printed part", i, s)
		}
		if s >= 6.2 {
			t.Errorf("size %d is %.2f, which is not smaller than what was "+
				"asked for", i, s)
		}
		if i > 0 && s >= sizes[i-1] {
			t.Errorf("the sizes are not tried largest first: %.2f after %.2f",
				s, sizes[i-1])
		}
	}
	// Already at the floor, or under it: there is nothing smaller to try.
	if n := len(tightenSizes(PartStaffSpace)); n != 0 {
		t.Errorf("at the floor it still offers %d sizes", n)
	}
}
