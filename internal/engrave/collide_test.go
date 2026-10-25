/*
 * collide_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/smufl"
)

/*
 * Nothing is printed on top of a notehead.
 *
 * Every other test here asks whether a mark is present and roughly where. The
 * eye asks a different question — is anything sitting on anything else — and
 * that one went unasked, so a rest printed through the C of an A flat-C-D
 * chord passed every check there was. It was found by looking at the page.
 *
 * The cause was that a chord holding a second is a notehead wider than its
 * own column, because one of the two is pushed across the stem, and the
 * spacing did not know. Checked over every score in the tree, from the
 * layout rather than from a picture of it.
 */
func TestMarksClearOfNoteheads(t *testing.T) {
	// Where a rest and a notehead do still share the same ground.
	//
	// Two voices on one staff, a rest in one of them sounding at the same
	// moment as a note in the other. They belong at the same place along the
	// bar — that is what sounding together means — so the rest has to move
	// up or down out of the way rather than along, and nothing here does
	// that yet. Listed rather than quietly tolerated, so that a third one
	// appearing somewhere else says so.
	expected := map[string]int{
		"33c-Spanners-Slurs.musicxml": 1,
	}

	total := 0
	for name, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		n := overlaps(engrave.Engrave(s, engrave.DefaultOptions()))
		if want := expected[filepath.Base(name)]; n != want {
			t.Errorf("%s: %d marks drawn over a notehead, expected %d",
				filepath.Base(name), n, want)
			total += n
		}
	}
	if total > 0 {
		t.Errorf("%d marks were drawn over a notehead", total)
	}
}

// overlaps counts the glyphs that share ground with a notehead: the same
// stretch of the bar, and within a staff space of the same height.
//
// Rests and articulations. Both are placed against a note rather than drawn
// at a fixed height, and both got it wrong in the same way — by measuring
// from the wrong edge of a chord.
func overlaps(lay *engrave.Layout) int {
	type box struct{ x0, x1, y float64 }
	var heads, marks []box
	for _, g := range lay.Glyphs {
		b := box{g.X, g.X + smufl.Advance(g.Name), g.Y}
		switch {
		case strings.HasPrefix(g.Name, "notehead"):
			heads = append(heads, b)
		case strings.HasPrefix(g.Name, "rest"),
			strings.HasPrefix(g.Name, "artic"):
			marks = append(marks, b)
		}
	}
	n := 0
	for _, r := range marks {
		for _, h := range heads {
			// A hair of overlap is the two just touching, which the eye
			// does not read as a collision.
			if h.x0 < r.x1-0.05 && r.x0 < h.x1-0.05 &&
				h.y > r.y-0.9 && h.y < r.y+0.9 {
				n++
				break
			}
		}
	}
	return n
}
