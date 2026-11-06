/*
 * glyphbox_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/smufl"
)

// Nothing gscore draws is guessed at.
//
// How far a glyph reaches above and below the point it is drawn at decides
// where everything that is not a note goes, and it used to be a list of
// rules by name with a flat space either way for anything the list did not
// cover. The font publishes a box for every glyph it has, so the rules are
// now a short list of places where a reader looking at pages beat the
// outlines, and the font answers everywhere else.
//
// There is still a blind guess underneath both, for a glyph that has neither
// — and this is what says it is never reached. A flat guess is how the clefs
// came to be 1.79 spaces out and the whole rests 1.16, for years, with
// nothing failing.
//
// Asked of every score in the tree, because that is the only way to know
// which glyphs the engraver really places: the font has 3,434 names and
// these scores draw a few dozen of them.
func TestNothingDrawnIsGuessedAt(t *testing.T) {
	missing := map[string]int{}
	drawn := map[string]bool{}
	for _, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		for _, g := range engrave.Engrave(s, engrave.DefaultOptions()).Glyphs {
			drawn[g.Name] = true
			if _, ok := smufl.BBox(g.Name); !ok {
				missing[g.Name]++
			}
		}
	}
	if len(drawn) == 0 {
		t.Fatal("no glyphs were drawn at all, so this proves nothing")
	}

	var names []string
	for n := range missing {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		t.Errorf("%s is drawn %d times and the font declares no box for it",
			n, missing[n])
	}
	t.Logf("%d glyph names drawn across the corpus, all measured", len(drawn))
}
