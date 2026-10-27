/*
 * skyline.go
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
	"sort"

	"github.com/stricaud/gscore/internal/smufl"
)

// Where the marks outside a staff go.
//
// Everything drawn off the staff used to be placed at a fixed distance from
// it: a hairpin four and a half spaces below, a dynamic three, a rehearsal
// mark about four above. That is right until two of them want the same
// place, and then they are simply drawn through each other — a hairpin
// through a slur, a boxed letter around a high note, an 8va sign on top of a
// segno. Reserving the largest of what they each need does not help, because
// it is one allowance shared by all of them rather than one each.
//
// A skyline is the outline of what is already on the page: for every point
// along a staff, how far the music reaches above it and how far below. A
// mark is then placed against that outline rather than against the staff,
// and once placed it becomes part of it, so the next one clears it too.
// Working outwards in a fixed order is what gives a page its layers — the
// dynamics nearest the staff, the hairpins beyond them, the brackets and the
// letters above everything.
//
// It is how every notation program does this, and it is cheap here because
// the engraver already produces a layout of plain shapes: the outline can be
// read off what has been drawn rather than predicted before drawing it.

// skylineStep is how finely the outline is sampled, in staff spaces.
//
// A quarter of a space is finer than any gap that matters — a notehead is
// more than a space wide — and keeps the arrays small enough that building
// one per staff per system costs nothing.
const skylineStep = 0.25

// skyline is the outline of everything drawn on one staff of one system.
//
// y grows downwards, so up holds the smallest y reached at each point and
// down the largest. Both start at the staff's own lines, which every mark
// has to clear whether or not there is a note under it.
type skyline struct {
	x0   float64
	up   []float64
	down []float64
}

// newSkyline starts an outline over a range, with nothing in it but the
// staff itself, from its top line to its bottom one.
func newSkyline(x0, x1, top, height float64) *skyline {
	n := int((x1-x0)/skylineStep) + 2
	if n < 1 {
		n = 1
	}
	s := &skyline{x0: x0, up: make([]float64, n), down: make([]float64, n)}
	for i := range s.up {
		s.up[i] = top
		s.down[i] = top + height
	}
	return s
}

// span turns an x range into the slice of samples it covers.
//
// Half open: a sample belongs to the range when the range reaches into it,
// not when the range stops exactly where it starts. Rounding both ends
// outwards made two things drawn side by side share a sample and so appear
// to overlap — which is why a row of mordents over consecutive notes came
// out as a staircase, each one climbing to clear a neighbour it never
// touched.
func (s *skyline) span(x0, x1 float64) (lo, hi int) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	lo = int(math.Floor((x0 - s.x0) / skylineStep))
	hi = int(math.Ceil((x1-s.x0)/skylineStep)) - 1
	if hi < lo {
		hi = lo
	}
	if lo < 0 {
		lo = 0
	}
	if hi >= len(s.up) {
		hi = len(s.up) - 1
	}
	return lo, hi
}

// add registers something occupying a box, so that whatever is placed next
// has to clear it.
func (s *skyline) add(x0, x1, top, bottom float64) {
	if bottom < top {
		top, bottom = bottom, top
	}
	lo, hi := s.span(x0, x1)
	for i := lo; i <= hi; i++ {
		if top < s.up[i] {
			s.up[i] = top
		}
		if bottom > s.down[i] {
			s.down[i] = bottom
		}
	}
}

// reach is how far the outline comes out over a range: the smallest y above,
// or the largest below.
func (s *skyline) reach(x0, x1 float64, below bool) float64 {
	lo, hi := s.span(x0, x1)
	if lo > hi {
		// A range off the end of the outline clears everything.
		if below {
			return s.down[len(s.down)-1]
		}
		return s.up[0]
	}
	out := s.up[lo]
	if below {
		out = s.down[lo]
	}
	for i := lo + 1; i <= hi; i++ {
		if below {
			if s.down[i] > out {
				out = s.down[i]
			}
		} else if s.up[i] < out {
			out = s.up[i]
		}
	}
	return out
}

// place finds where something of a given height goes, clear of what is
// already there by gap, and registers it.
//
// It returns the near edge: the top of a thing placed below the staff, and
// the bottom of a thing placed above it, because that is the edge a caller
// measures its own drawing from.
func (s *skyline) place(x0, x1, height, gap float64, below bool) float64 {
	at := s.reach(x0, x1, below)
	if below {
		at += gap
		s.add(x0, x1, at, at+height)
		return at
	}
	at -= gap
	s.add(x0, x1, at-height, at)
	return at
}

/*
 * Reading the outline off what has been drawn
 */

// mark is how much of the layout existed before a staff's music was drawn,
// so that the outline can be read from what was added after it.
type mark struct {
	glyphs, lines, curves, quads, texts int
}

func (e *engraver) marked() mark {
	return mark{
		glyphs: len(e.lay.Glyphs), lines: len(e.lay.Lines),
		curves: len(e.lay.Curves), quads: len(e.lay.Quads),
		texts: len(e.lay.Texts),
	}
}

// outline builds the skyline of everything drawn since a mark.
//
// The shapes are exact where it matters most: a stem, a beam, a ledger line
// and a slur are lines and curves whose extents the engraver computed, so
// those go in as they are. Only the glyphs are estimated, because the font
// gives their widths but not their heights — and a notehead, which is nearly
// all of them, is one space tall and centred on where it sits.
func (e *engraver) outline(from mark, x0, x1, top float64) *skyline {
	s := newSkyline(x0, x1, top, e.staffH())
	e.addSince(s, from)
	return s
}

// staffOutline is the outline of the staff being drawn, kept and extended
// rather than built again each time it is wanted.
//
// Every slur asks for it, and building one walks everything on the staff and
// lays out a sample every quarter of a space across the staff's whole width.
// In the editor's strip a staff is the whole piece: Beethoven's ninth built
// a two-million-sample outline for each of its thousands of slurs, and spent
// eleven of its twelve seconds of layout doing it. Kept, each slur adds only
// what has been drawn since the last one asked.
func (e *engraver) staffOutline() *skyline {
	if e.staffSky == nil {
		e.staffSky = newSkyline(e.staffX0, e.staffX1, e.staffTop, e.staffH())
		e.staffSkyAt = e.staffMark
	}
	e.addSince(e.staffSky, e.staffSkyAt)
	e.staffSkyAt = e.marked()
	return e.staffSky
}

// addSince registers everything drawn since a mark on an outline.
func (e *engraver) addSince(s *skyline, from mark) {
	for _, g := range e.lay.Glyphs[from.glyphs:] {
		up, down := glyphReach(g.Name)
		w := glyphWidth(g)
		s.add(g.X, g.X+w, g.Y-up, g.Y+down)
	}
	for _, l := range e.lay.Lines[from.lines:] {
		s.add(l.X1, l.X2, math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2))
	}
	for _, q := range e.lay.Quads[from.quads:] {
		if len(q.X) == 0 {
			continue
		}
		lo, hi := q.Y[0], q.Y[0]
		xa, xb := q.X[0], q.X[0]
		for i := range q.X {
			lo, hi = math.Min(lo, q.Y[i]), math.Max(hi, q.Y[i])
			xa, xb = math.Min(xa, q.X[i]), math.Max(xb, q.X[i])
		}
		s.add(xa, xb, lo, hi)
	}
	for _, c := range e.lay.Curves[from.curves:] {
		// A quadratic never leaves the box of its ends and its control
		// point, so the box is a safe outline and a cheap one.
		lo := math.Min(c.Y1, math.Min(c.Y2, c.CY))
		hi := math.Max(c.Y1, math.Max(c.Y2, c.CY))
		s.add(math.Min(c.X1, c.X2), math.Max(c.X1, c.X2), lo, hi)
	}
	for _, t := range e.lay.Texts[from.texts:] {
		w := textWidth(t.S, t.Size)
		x := t.X
		switch t.Anchor {
		case "middle":
			x -= w / 2
		case "end":
			x -= w
		}
		// Text is drawn from its baseline, so it reaches up by about its
		// size and down by a descender.
		s.add(x, x+w, t.Y-t.Size*0.8, t.Y+t.Size*0.25)
	}
}

// glyphWidth is how wide a glyph is where the layout does not say.
func glyphWidth(g Glyph) float64 {
	w := smufl.Advance(g.Name)
	if g.Size > 0 {
		// A glyph drawn at another size scales with it; four staff spaces is
		// the size the advances are quoted at.
		w *= g.Size / 4
	}
	return w
}

// glyphReach estimates how far a glyph reaches above and below the point it
// is drawn at, in staff spaces.
//
// Estimated, because the font carries advance widths and not bounding boxes:
// reading those out of an OpenType font with CFF outlines means walking
// charstrings, which is a lot of machinery for a number that only has to be
// about right. Generous is the safe direction — a mark placed a little
// further out than it had to be is tidy, one placed through a notehead is
// wrong — so where a glyph is not known the answer is a whole space either
// way.
func glyphReach(name string) (up, down float64) {
	switch {
	case has(name, "notehead"), has(name, "augmentationDot"):
		return 0.55, 0.55
	case has(name, "accidental"):
		return 1.4, 1.4
	case has(name, "rest"):
		return 1.2, 1.2
	case has(name, "flag"):
		// A flag hangs from the end of a stem, on one side only, but which
		// side is in its name and the stem has already claimed that reach.
		return 1.2, 1.2
	case has(name, "dynamic"):
		return 0.9, 0.5
	case has(name, "tremolo"), has(name, "buzzRoll"):
		return 1.0, 1.0
	case has(name, "artic"), has(name, "fermata"), has(name, "strings"):
		return 0.8, 0.8
	case has(name, "met"):
		return 1.6, 0.6
	case has(name, "gClef"), has(name, "fClef"), has(name, "cClef"):
		return 2.6, 2.6
	case has(name, "timeSig"):
		return 1.1, 1.1
	case has(name, "segno"), has(name, "coda"):
		return 1.6, 1.6
	case has(name, "repeat"):
		return 2.1, 2.1
	}
	return 1.0, 1.0
}

func has(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

/*
 * Marks that wait for the staff to be finished
 */

// waitingMark is one mark that goes outside the staff, held until the staff
// it belongs to has been drawn and its outline is known.
//
// rank is how near the staff it belongs, smallest first: the dynamics sit
// closest, then the hairpins, then the lines and brackets that reach over
// whole phrases. That order is what gives a page its layers, and it is a
// decision about notation rather than about the order the engraver happened
// to meet them in.
type waitingMark struct {
	x0, x1 float64
	height float64
	gap    float64
	below  bool
	rank   int
	draw   func(y float64)
}

// How near the staff each kind of mark sits, smallest first.
//
// The numbers are LilyPond's, from the outside-staff-priority property its
// grobs carry in scm/define-grobs.scm. They are kept as they are rather than
// renumbered 1, 2, 3, so that the order can be checked against the program
// that settled it and so there is room to put something between two of them.
//
// The ordering is a real piece of engraving knowledge and not an arbitrary
// choice: the bar number sits closest to the staff, the dynamics and their
// hairpins next — on one line, which is why they share a number — then the
// octave lines, the directions, the volta brackets, and furthest out the
// metronome mark, the signs and the rehearsal letters, which are the things
// somebody has to find from across a room.
const (
	rankTrill     = 50 // TrillSpanner: nearer the staff than anything else
	rankOrnament  = 60 // Script: a mordent or a turn, just outside the trill
	rankBarNumber = 100
	rankDynamic   = 250 // DynamicLineSpanner: a dynamic and a hairpin share it
	rankHairpin   = 250
	rankOctave    = 400  // OttavaBracket
	rankText      = 450  // TextScript
	rankVolta     = 600  // VoltaBracketSpanner
	rankMarks     = 1500 // RehearsalMark, and the signs and tempo with it
)

// wait holds a mark back until the staff is finished.
func (e *engraver) wait(m waitingMark) { e.waiting = append(e.waiting, m) }

// placeWaiting draws everything that was held back, outwards from the staff.
//
// Each is placed clear of what is already there and then becomes part of it,
// so the next one clears it in turn. Within a rank they are placed left to
// right, which keeps a row of dynamics on one line rather than stepping down
// the page as each clears the last.
func (e *engraver) placeWaiting(sky *skyline) {
	marks := e.waiting
	e.waiting = nil
	if len(marks) == 0 {
		return
	}
	sort.SliceStable(marks, func(i, j int) bool {
		if marks[i].rank != marks[j].rank {
			return marks[i].rank < marks[j].rank
		}
		return marks[i].x0 < marks[j].x0
	})
	for _, m := range marks {
		m.draw(sky.place(m.x0, m.x1, m.height, m.gap, m.below))
	}
}
