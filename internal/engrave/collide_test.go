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

/*
 * Nothing is written outside its own bar.
 *
 * Where two voices on one staff sound a note at the same moment and close
 * enough to clash, one of them steps sideways so that the reader can see
 * there are two. The step was taken when the noteheads were drawn, into
 * whatever space happened to be to the left — and at the start of a bar what
 * is to the left is the barline. The Altes printed its bar 10 that way: a
 * chord of half notes written on top of the line that begins the bar.
 *
 * The room is booked by the spacing now, before the column is placed, so
 * this asks the question the page asks: is everything inside the bar it
 * belongs to.
 */
func TestNothingIsWrittenOutsideItsBar(t *testing.T) {
	for name, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		lay := engrave.Engrave(s, engrave.DefaultOptions())
		for _, h := range lay.Hits {
			for _, el := range h.Elements {
				// A hair, for the rounding in the stretch.
				if el.X < h.XMin-0.05 {
					t.Errorf("%s: staff %d bar %d has something at x=%.2f, "+
						"left of the bar's own start at %.2f",
						name, h.Part, h.Measure+1, el.X, h.XMin)
				}
			}
		}
	}
}

/*
 * Nothing written under one staff is written over the next one.
 *
 * The gap between two staves is reserved before either is drawn, from a
 * count of what the model says goes into it — a dynamic here, a hairpin
 * there. A count knows the marks it has names for and nothing else, and the
 * marks that go outside a staff are placed against what is already drawn
 * around it, so they end up as far out as they have to go whether or not
 * anything reserved the room.
 *
 * So a hairpin that had to clear a row of accents and a slur, neither of
 * them counted, went on down past the gap and came to rest across the first
 * note of the staff below. Bar 55 of the Rossini is where it was found, and
 * it is a diminuendo lying over a saxophone's top line.
 *
 * Asked of the hairpins and the dynamics because those are the marks the
 * layout records a box for, and they are the ones that travel furthest from
 * their own staff.
 */
func TestNothingUnderAStaffLandsOnTheNextOne(t *testing.T) {
	for name, path := range corpus(t) {
		s, _ := load(path)
		if s == nil {
			continue
		}
		lay := engrave.Engrave(s, engrave.DefaultOptions())

		// Which system a bar is on, and so which staff is the one below
		// this one. By system rather than by bar: a hairpin cut by a line
		// break is recorded at the bar it opened in, which is on the system
		// before the one it was finally drawn on, and looking its neighbour
		// up by that bar measures it against a staff on another page.
		sysOf := func(y float64) int {
			for i, sys := range lay.Systems {
				if y >= sys.Top && y <= sys.Bottom {
					return i
				}
			}
			return -1
		}
		// A hair of overlap is the thickness of a line, not a collision.
		const hair = 0.1
		under := func(what string, part, measure int, x0, x1, y, bottom float64) {
			si := sysOf(y)
			if si < 0 {
				return
			}
			sys := lay.Systems[si]
			for _, h := range lay.Hits {
				if h.Part != part+1 || h.Measure < sys.First || h.Measure > sys.Last {
					continue
				}
				// The staff's own lines.
				if bottom > h.StaffTop+hair {
					t.Errorf("%s: the %s under staff %d at bar %d reaches "+
						"to %.2f, %.2f into the staff below it",
						name, what, part+1, measure+1, bottom, bottom-h.StaffTop)
					return
				}
				// And its notes, which climb above them. This is the one
				// that was wrong: the hairpin stopped just short of the
				// line below and lay across the notehead over it.
				for _, el := range h.Elements {
					if el.X > x1 || el.X+el.Width < x0 {
						continue
					}
					if head := el.Y - 0.5; bottom > head+hair {
						t.Errorf("%s: the %s under staff %d at bar %d "+
							"reaches to %.2f, %.2f into a notehead of the "+
							"staff below it",
							name, what, part+1, measure+1, bottom, bottom-head)
						return
					}
				}
			}
		}
		for _, h := range lay.Hairpins {
			under("hairpin", h.Part, h.Measure, h.X, h.X+h.Width, h.Y, h.Y+h.Height)
		}
		for _, d := range lay.Dynamics {
			under("dynamic", d.Part, d.Measure, d.X, d.X+d.Width, d.Y, d.Y+d.Height)
		}
	}
}
