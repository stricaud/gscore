/*
 * duration.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"math"
	"sort"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

// valueOf decides how long a notehead lasts.
//
// The head itself only says whole, half or black, and a black head is
// anything from a quarter down. What settles it is the number of beams or
// flags, and those are counted rather than recognised:
//
//   - a flag is a glyph and the font names it, so it is simply read;
//   - a beam is a filled slanted bar, and the reader records the outline of
//     a filled shape, so each beam leaves a pair of short vertical edges
//     where it meets the stem. Counting the short verticals along a stem
//     counts the beams, which is the figure wanted — two beams is a
//     sixteenth whether the note is beamed to its neighbour or flagged
//     alone.
//
// Nothing here tries to follow a beam from note to note. It does not need
// to: the beams crossing one stem are all that stem's note is worth.
func valueOf(c *pdf.Content, st staff, marks []mark, i int) model.NoteValue {
	note := marks[i]
	base := note.s.value
	if base != model.Quarter {
		return base // a whole or half head carries no beams
	}
	if v, ok := flagNear(st, marks, i); ok {
		return v
	}
	if n := beamsOnStem(c, st, note.x, note.y); n > 0 {
		v := model.NoteValue(int(model.Quarter) + n)
		if v > model.HundredTwentyEighth {
			v = model.HundredTwentyEighth
		}
		return v
	}
	return model.Quarter
}

// flagNear finds a flag drawn on this note's stem.
//
// A flag sits at the far end of the stem, so it is well away from the head
// vertically but almost exactly above or below it. The horizontal test is
// therefore tight and the vertical one loose, which is the opposite of how
// the accidental is found and for the same reason: that is where engraving
// puts it.
func flagNear(st staff, marks []mark, i int) (model.NoteValue, bool) {
	note := marks[i]
	for _, m := range marks {
		if m.s.kind != kFlag {
			continue
		}
		if math.Abs(m.x-note.x) > 1.6*st.inter {
			continue
		}
		if math.Abs(m.y-note.y) > 6*st.inter {
			continue
		}
		return m.s.value, true
	}
	return 0, false
}

// beamsOnStem counts the beams crossing the stem of the note at (x, y).
func beamsOnStem(c *pdf.Content, st staff, x, y float64) int {
	type bar struct{ x, lo, hi float64 }
	var near []bar
	seen := map[[3]float64]bool{}
	for _, s := range c.Segments {
		if !s.Vertical() {
			continue
		}
		b := bar{(s.X1 + s.X2) / 2, math.Min(s.Y1, s.Y2), math.Max(s.Y1, s.Y2)}
		// the stem stands at the side of the notehead, not through its
		// middle, and the head's own x is its left edge — so the stem may be
		// a whole head's width away, and the window has to allow for that
		if math.Abs(b.x-x) > 1.8*st.inter {
			continue
		}
		// a filled rectangle hands over each of its edges more than once
		key := [3]float64{math.Round(b.x*4) / 4,
			math.Round(b.lo*4) / 4, math.Round(b.hi*4) / 4}
		if seen[key] {
			continue
		}
		seen[key] = true
		near = append(near, b)
	}
	// The stem is the tall one, and of the tall ones the nearest: a wide
	// window finds the neighbour's stem as well, and counting that one's
	// beams would make an isolated quarter into a sixteenth.
	stem := bar{}
	for _, s := range near {
		if s.hi-s.lo < 1.2*st.inter {
			continue
		}
		// A stem begins at its own notehead, so the head's height lies
		// within its span. Without this the nearest tall line wins, and
		// where a down-stem note stands beside an up-stem one the nearest
		// is the neighbour's — which lends this note the neighbour's beams
		// and turns an isolated quarter into a sixteenth.
		if y < s.lo-0.7*st.inter || y > s.hi+0.7*st.inter {
			continue
		}
		if stem.hi == stem.lo || math.Abs(s.x-x) < math.Abs(stem.x-x) {
			stem = s
		}
	}
	if stem.hi-stem.lo < 1.2*st.inter {
		return 0
	}
	// the beams are at the end of the stem away from the notehead
	far := stem.hi
	if math.Abs(stem.lo-y) > math.Abs(stem.hi-y) {
		far = stem.lo
	}

	// Count the beams by what crosses the stem, not by what ends on it.
	//
	// A beam is one filled bar spanning the whole group, so its short end
	// edges exist only at the first and last stem; the stems in between are
	// crossed by the bar but touched by none of its ends. Counting ends gets
	// the outer notes of every group right and the inner ones wrong, which
	// is a worse failure than getting none of them right, because it looks
	// like it nearly works.
	//
	// What does hold for every stem is that the bar's two long edges pass
	// over it. So we count edges crossing the stem and halve: two edges to a
	// beam, four to a pair of them.
	full := 0.9 * (st.x1 - st.x0)
	crossings := 0
	seenY := map[float64]bool{}
	for _, s := range c.Segments {
		dx, dy := s.X2-s.X1, s.Y2-s.Y1
		if math.Abs(dx) < 1.2*st.inter || math.Abs(dx) > full {
			continue // too short for a beam, or a staff line
		}
		if math.Abs(dy) > 0.6*math.Abs(dx) {
			continue // too steep to be a beam's edge
		}
		lo, hi := math.Min(s.X1, s.X2), math.Max(s.X1, s.X2)
		if stem.x < lo || stem.x > hi {
			continue
		}
		// where the edge passes over the stem
		yAt := s.Y1 + dy*(stem.x-s.X1)/dx
		if math.Abs(yAt-far) > 2.2*st.inter {
			continue // not at the business end of the stem
		}
		if math.Abs(yAt-y) < 1.2*st.inter {
			continue // a ledger line through the head, not a beam
		}
		key := math.Round(yAt * 4)
		if seenY[key] {
			continue
		}
		seenY[key] = true
		crossings++
	}
	if crossings >= 2 {
		return (crossings + 1) / 2
	}
	// a group of one — the beam has ends but nothing between them — still
	// shows its end edges on this very stem
	var mids []float64
	for _, s := range near {
		if math.Abs(s.x-stem.x) > 0.4*st.inter {
			continue
		}
		h := s.hi - s.lo
		if h < 0.2*st.inter || h > 0.9*st.inter {
			continue
		}
		mid := (s.lo + s.hi) / 2
		if math.Abs(mid-far) > 3*st.inter {
			continue
		}
		mids = append(mids, mid)
	}
	if len(mids) == 0 {
		return 0
	}
	sort.Float64s(mids)
	n := 1
	for k := 1; k < len(mids); k++ {
		if mids[k]-mids[k-1] > 0.25*st.inter {
			n++
		}
	}
	return n
}

// dotsOn counts the augmentation dots written after a notehead.
func dotsOn(st staff, marks []mark, i int) int {
	note := marks[i]
	n := 0
	for _, m := range marks {
		if m.s.kind != kDot {
			continue
		}
		if m.x <= note.x || m.x-note.x > 2.5*st.inter {
			continue
		}
		// a dot sits in the space beside its head, which is the head's own
		// height or half a step above it when the head is on a line
		if math.Abs(m.y-note.y) > 0.7*st.inter {
			continue
		}
		n++
	}
	return n
}

// dotted lengthens a duration by its dots: one dot adds a half, two add
// three quarters, and so on.
func dotted(d model.Duration, dots int) model.Duration {
	for k := 0; k < dots && k < 4; k++ {
		d = d.Add(d.Scale(1, 1<<(k+1)))
	}
	return d
}
