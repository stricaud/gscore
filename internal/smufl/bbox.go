/*
 * bbox.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package smufl

import (
	_ "embed"
	"encoding/json"
	"sync"
)

/*
 * Where a glyph's ink is.
 *
 * An advance width says how far to move along before drawing the next thing.
 * It says nothing about how tall a glyph is, or how far it hangs below the
 * line it is drawn on, and those are what decide where everything that is
 * not a note goes: how high a slur arches, how far above the staff a tempo
 * mark sits, how far apart two staves have to be.
 *
 * Before this, those were guessed. The engraver had a list of rules by name
 * — anything called "accidental" reaches 1.4 spaces each way, anything
 * called "rest" 1.2 — and a glyph matching none of them was given a whole
 * space in both directions and hoped for. The comment on that list said
 * generous was the safe direction, and it was not: measured against the
 * font, the guess fell short of the real ink on 36 of the 74 glyphs the
 * engraver places most, and overstated it on 25. A treble clef reaches 4.39
 * spaces above the line it names and was given 2.6. A whole rest reaches
 * 0.04 above its origin and was given 1.2, so everything above a bar of
 * whole rest sat a space and a sixth higher than it needed to.
 *
 * The font knows. SMuFL fonts publish a bounding box for every glyph, in
 * staff spaces, measured from the outlines by whoever drew them — so the
 * numbers here are Bravura's own account of its own shapes rather than
 * anybody's reading of them.
 *
 * It is a box and not the ink itself, which overstates a glyph wherever a
 * corner is empty: a sharp is mostly air, and its box claims all of it. The
 * ink would have to come from the outlines, which means walking CFF
 * charstrings, and the box is the larger part of the improvement for a
 * fraction of the work. Where the difference starts to matter is tucking an
 * accidental under the notehead before it, and nothing here does that yet.
 */

//go:embed data/bravura-bboxes.json
var bravuraBoxesJSON []byte

// Box is where a glyph's ink is, relative to the point the glyph is drawn
// at, in staff spaces. Up is positive, as the font measures it, and not as
// the screen does.
type Box struct {
	Left, Right float64
	Bottom, Top float64
}

// Height is how tall the box is.
func (b Box) Height() float64 { return b.Top - b.Bottom }

// Width is how wide the box is, which is not the advance width: a glyph may
// be narrower than the space it books, and a few are wider.
func (b Box) Width() float64 { return b.Right - b.Left }

var (
	boxesOnce sync.Once
	boxes     map[string]Box
	boxFont   struct {
		Name    string
		Version float64
	}
)

func loadBoxes() {
	var meta struct {
		FontName    string  `json:"fontName"`
		FontVersion float64 `json:"fontVersion"`
		GlyphBBoxes map[string]struct {
			NE [2]float64 `json:"bBoxNE"`
			SW [2]float64 `json:"bBoxSW"`
		} `json:"glyphBBoxes"`
	}
	if err := json.Unmarshal(bravuraBoxesJSON, &meta); err != nil {
		// Embedded at build time from a file in this repository, so this
		// cannot happen to a built binary. An empty table is the right
		// answer if it somehow does: every caller has a fallback, and a
		// page drawn from guesses beats no page at all.
		boxes = map[string]Box{}
		return
	}
	boxFont.Name, boxFont.Version = meta.FontName, meta.FontVersion
	boxes = make(map[string]Box, len(meta.GlyphBBoxes))
	for name, b := range meta.GlyphBBoxes {
		boxes[name] = Box{
			Left: b.SW[0], Right: b.NE[0],
			Bottom: b.SW[1], Top: b.NE[1],
		}
	}
}

// BBox returns the box the font declares for a glyph.
//
// Read from a table and never computed, which is what keeps a page the same
// page: no rasteriser, no renderer and no library version takes part, so
// nothing but a change of font file can move a note. The table is read once.
func BBox(name string) (Box, bool) {
	boxesOnce.Do(loadBoxes)
	b, ok := boxes[name]
	return b, ok
}

// BoxCount reports how many glyphs have a declared box.
func BoxCount() int {
	boxesOnce.Do(loadBoxes)
	return len(boxes)
}

// BoxFont names the font the boxes were taken from, and its version.
//
// The boxes and the font are a pair: a box read for one version of a font
// and applied to another is a measurement of the wrong shape. Nothing checks
// this at run time because both are embedded from the same directory of this
// repository, and a test checks that they still agree.
func BoxFont() (name string, version float64) {
	boxesOnce.Do(loadBoxes)
	return boxFont.Name, boxFont.Version
}

// AllBoxNames is every glyph with a declared box, for tests.
func AllBoxNames() map[string]Box {
	boxesOnce.Do(loadBoxes)
	return boxes
}

/*
 * Regenerating the data file
 *
 * data/bravura-bboxes.json is the glyphBBoxes object of Bravura's own
 * redist/Bravura.json, keys sorted, nothing else kept. To take it again from
 * a new release of the font:
 *
 *	python3 -c '
 *	import json, collections
 *	m = json.load(open("Bravura.json"))
 *	out = collections.OrderedDict(sorted(m["glyphBBoxes"].items()))
 *	json.dump({"fontName": m["fontName"], "fontVersion": m["fontVersion"],
 *	           "glyphBBoxes": out}, open("bravura-bboxes.json", "w"),
 *	          separators=(",", ":"), sort_keys=True)'
 *
 * The font file beside it has to be taken from the same release, and
 * TestTheBoxesBelongToTheFontThatIsShipped is what says so. The numbers are
 * written down rather than measured at run time on purpose: a page should
 * move because somebody changed the font, and for no other reason.
 */
