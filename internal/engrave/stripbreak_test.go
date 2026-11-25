/*
 * stripbreak_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A line break in the strip.
 *
 * A break the writer asked for ends a line on the page. The strip has no
 * lines, and obeying it there cut Take the A Train into a system of one bar,
 * then one of two, each stacked under the last with the rest of the strip
 * empty.
 */

package engrave

import "testing"

func TestTheStripRunsOnPastALineBreak(t *testing.T) {
	s := movements(8)
	s.Parts[0].Measures[1].SystemBreak = true
	s.Parts[0].Measures[3].SystemBreak = true

	opt := DefaultOptions()
	opt.Mode = Strip
	if lay := Engrave(s, opt); len(lay.Systems) != 1 {
		t.Errorf("%d systems in the strip, want the one", len(lay.Systems))
	}
	// On the page the breaks still hold.
	lay := Engrave(s, DefaultOptions())
	for _, sys := range lay.Systems {
		for _, at := range []int{1, 3} {
			if sys.First < at && sys.Last >= at {
				t.Errorf("a page system runs from bar %d to %d, across the break before bar %d",
					sys.First+1, sys.Last+1, at+1)
			}
		}
	}
}
