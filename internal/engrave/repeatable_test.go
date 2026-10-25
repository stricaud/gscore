/*
 * repeatable_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

/*
 * The same score engraves to the same drawing every time.
 *
 * It did not. Spanners are kept in maps — what is open on each staff, and the
 * tie open on each notehead of a chord — and the curves left at a line break
 * were drawn while walking them. Go walks a map in a different order every
 * run on purpose, so a score with two ties carried over a break came out with
 * its marks in one order on one run and another on the next. Every mark was
 * in the right place; only the order they were listed in moved.
 *
 * Which sounds harmless and is not. It is the thing that makes a drawing
 * comparable with itself: the golden layouts hash what was drawn, so the
 * record agreed with the engraver on some runs and not others, and the build
 * failed on a machine that had changed nothing.
 */
func TestEngravingIsRepeatable(t *testing.T) {
	for name, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		first := describe(engrave.Engrave(s, engrave.DefaultOptions()))
		for i := 0; i < 3; i++ {
			if again := describe(engrave.Engrave(s, engrave.DefaultOptions())); again != first {
				t.Errorf("%s: engraving it again gave a different drawing", name)
				break
			}
		}
	}
}
