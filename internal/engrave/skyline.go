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
	"strings"

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
// newInkline is an outline of the ink alone, with no staff under it.
//
// The one above is seeded with the staff's own box, because what is placed
// against it goes outside the staff: a dynamic or a bar number is measured
// from the staff, not from the nearest notehead. A slur is the other case.
// It belongs among the notes and is drawn inside the staff whenever they
// are, so measuring it against a box that includes the staff told it there
// was something in the way everywhere — and a phrase low in the staff was
// arched clear over the top line to miss a line that is not in its way.
func newInkline(x0, x1 float64) *skyline {
	n := int((x1-x0)/skylineStep) + 2
	if n < 1 {
		n = 1
	}
	s := &skyline{x0: x0, up: make([]float64, n), down: make([]float64, n)}
	for i := range s.up {
		s.up[i] = math.Inf(1)
		s.down[i] = math.Inf(-1)
	}
	return s
}

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
		e.staffInk = newInkline(e.staffX0, e.staffX1)
		e.staffMusic = newInkline(e.staffX0, e.staffX1)
		e.staffSkyAt = e.staffMark
	}
	// All from the same mark, and the mark moved once: the outlines hold
	// the same ink and differ only in what they start from — and in
	// whether the slurs are in them.
	e.addSince(e.staffSky, e.staffSkyAt)
	e.addSince(e.staffInk, e.staffSkyAt)
	e.addSinceExcept(e.staffMusic, e.staffSkyAt, e.slurCurve)
	e.staffSkyAt = e.marked()
	return e.staffSky
}

// staffInkline is the same outline without the staff under it, for the
// marks that live among the notes rather than outside them.
func (e *engraver) staffInkline() *skyline {
	e.staffOutline()
	return e.staffInk
}

// slurInkline is that outline again with the slurs taken out of it, which
// is what a slur is placed against. See the staffMusic field.
func (e *engraver) slurInkline() *skyline {
	e.staffOutline()
	return e.staffMusic
}

// noteSlur records that a curve is a slur, so the outline slurs are placed
// against can leave it out.
func (e *engraver) noteSlur(at int) {
	if e.slurCurve == nil {
		e.slurCurve = map[int]bool{}
	}
	e.slurCurve[at] = true
}

// curveSteps is how many pieces a slur is measured in.
//
// Sixteen is enough that the error at the top of an arc is under a tenth
// of a space at any width a slur is ever drawn, and few enough that a
// score of some thousands of slurs costs nothing noticeable.
const curveSteps = 16

// addCurve puts a slur or a tie into an outline as the shape it is.
//
// It used to go in as the box round its ends and its control point, which
// is a safe outline and the wrong one: an arc is only at its highest in
// the middle, and the box is that high across the whole width. A slur
// over three bars then stood seven spaces tall over the bar where it has
// already come back down to the notes, and everything placed above it —
// the bar numbers, and the volta brackets above those — was lifted clear
// of a shape that is not there. The voltas of the Tico-Tico floated
// fifteen spaces over the staff for want of this.
//
// Measured in pieces instead, each piece the box of its own span, which
// is the curve to within a tenth of a space.
func addCurve(s *skyline, c Curve) {
	px, py := c.X1, c.Y1
	for i := 1; i <= curveSteps; i++ {
		t := float64(i) / curveSteps
		u := 1 - t
		x := u*u*c.X1 + 2*u*t*c.CX + t*t*c.X2
		y := u*u*c.Y1 + 2*u*t*c.CY + t*t*c.Y2
		s.add(math.Min(px, x), math.Max(px, x),
			math.Min(py, y), math.Max(py, y))
		px, py = x, y
	}
}

// addSince registers everything drawn since a mark on an outline.
func (e *engraver) addSince(s *skyline, from mark) {
	e.addSinceExcept(s, from, nil)
}

// addSinceExcept is the same, leaving out the curves named in skip.
func (e *engraver) addSinceExcept(s *skyline, from mark, skip map[int]bool) {
	for _, g := range e.lay.Glyphs[from.glyphs:] {
		up, down := glyphHeight(g)
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
	for i, c := range e.lay.Curves[from.curves:] {
		if skip != nil && skip[from.curves+i] {
			continue // a slur, which other slurs are not placed against
		}
		addCurve(s, c)
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

// glyphHeight is how far a glyph reaches above and below the point it is
// drawn at, in staff spaces, at the size it is drawn.
//
// The size matters and used to be left out. A cue note, a grace note and an
// ossia staff are drawn from the same glyphs at a fraction of the size, and
// a full-size reach booked for a two-thirds-size note is a third of a space
// of nothing above every one of them — the same mistake glyphWidth beside
// this was already careful not to make.
func glyphHeight(g Glyph) (up, down float64) {
	if isGroupFurniture(g.Name) {
		// A brace or a bracket is not in the way of anything.
		//
		// It is drawn down the left of a system, outside the music, and it
		// is stretched to span the staves it joins — so how tall it is is
		// decided by where those staves were put. Feeding that back in as
		// room the system needs is circular, and it is why a piano score
		// came out with every system pushed apart: a brace spanning twelve
		// spaces claimed twelve spaces of room, for a shape nothing is
		// ever placed against.
		return 0, 0
	}
	up, down = glyphReach(g.Name)
	if g.Size > 0 {
		// Four staff spaces is the size the font quotes its boxes at.
		up *= g.Size / 4
		down *= g.Size / 4
	}
	return up, down
}

// glyphReach is how far a glyph reaches above and below the point it is
// drawn at, in staff spaces, at full size.
//
// The rules below come first, and where none of them answers, the font does:
// a SMuFL font publishes a bounding box for every glyph it has, so a glyph
// gscore has no rule for is measured rather than guessed. That replaced a
// flat one space either way — the answer for 2,389 of Bravura's 3,434
// glyphs, which is every ornament, every pedal mark and every piece of
// furniture the rules below do not name.
//
// The rules come first on purpose, and it is worth writing down why, because
// the obvious thing to do is delete them and read every glyph from the font.
// That was tried. It makes the pages 23% taller and reads no better.
//
// A box is where a glyph's ink is, and that is not the same question as how
// much room a glyph needs. Three cases came apart:
//
//   - A brace is drawn stretched to span the staves it joins, so its box,
//     scaled, is three spaces taller than the rules said — and it sits in
//     the margin, where no mark is ever placed against it. Counted as room
//     above the system it pushes every system apart for ink nothing reads.
//   - A flag's box reaches back along the stem it hangs from, which the stem
//     has already booked. True, and already counted.
//   - A treble clef really does reach 2.4 spaces above the top line, and an
//     engraver lets it, rather than opening the gap between systems for it.
//
// That is the warning in the decision this came from, read the other way
// round: finer geometry lets a program reserve more room than an engraver
// would, and the restraint has to come from somewhere else. In Ooloi's
// design that somewhere is its clearances and gap classes; gscore has no
// such thing yet, so until it does, the rules a reader tuned by looking at
// pages beat the font's own account of its outlines.
func glyphReach(name string) (up, down float64) {
	if up, down, ok := tunedReach(name); ok {
		return up, down
	}
	if b, ok := smufl.BBox(name); ok {
		// The font measures up as positive and the page measures down as
		// positive, which is why the bottom of the box is the reach below.
		return b.Top, -b.Bottom
	}
	// Neither a rule nor a box. Generous is the safe direction here: a mark
	// placed further out than it had to be is untidy, one placed through a
	// notehead is wrong.
	return 1.0, 1.0
}

// tunedReach is the reach for the glyphs gscore has looked at, settled by
// looking at pages rather than at outlines. ok is false where there is no
// rule for a glyph, which is most of them.
func tunedReach(name string) (up, down float64, ok bool) {
	switch {
	case has(name, "notehead"), has(name, "augmentationDot"):
		return 0.55, 0.55, true
	case has(name, "accidental"):
		return 1.4, 1.4, true
	case has(name, "rest"):
		return 1.2, 1.2, true
	case has(name, "flag"):
		// A flag is drawn at the end of its stem and hangs back along it,
		// so it reaches one way only and the name says which: an Up flag
		// is on a stem pointing up and hangs down from the tip of it.
		//
		// A space and a fifth both ways was the old answer, and it put a
		// space and a fifth of nothing above every flagged note. A slur
		// ending on one started that far over the stem it belongs to.
		if strings.HasSuffix(name, "Up") {
			return 0.1, 1.2, true
		}
		return 1.2, 0.1, true
	case has(name, "dynamic"):
		return 0.9, 0.5, true
	case has(name, "tremolo"), has(name, "buzzRoll"):
		return 1.0, 1.0, true
	case has(name, "artic"), has(name, "fermata"), has(name, "strings"):
		return 0.8, 0.8, true
	case has(name, "met"):
		return 1.6, 0.6, true
	case has(name, "gClef"), has(name, "fClef"), has(name, "cClef"):
		return 2.6, 2.6, true
	case has(name, "timeSig"):
		return 1.1, 1.1, true
	case has(name, "segno"), has(name, "coda"):
		return 1.6, 1.6, true
	case has(name, "repeat"):
		return 2.1, 2.1, true
	}
	return 0, 0, false
}

// isGroupFurniture names the glyphs drawn down the left of a system to say
// which staves belong together, rather than to be read.
func isGroupFurniture(name string) bool {
	return has(name, "brace") || has(name, "reversedBrace") ||
		has(name, "bracket")
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
	// join puts marks on one line: everything with the same join is placed
	// together, at the depth the deepest of them needs. Zero is a mark of
	// its own.
	//
	// It is for the marks that are read as one thing and drawn as several
	// — a diminuendo running into a crescendo, which is two wedges meeting
	// at a point and one marking to anybody reading it. Placed one at a
	// time they come out at two heights whenever the music under one of
	// them hangs lower than under the other, which is a down stem away.
	join int
	draw func(y float64)
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
	rankTrill    = 50 // TrillSpanner: nearer the staff than anything else
	rankOrnament = 60 // Script: a mordent or a turn, just outside the trill
	// rankBeatMark: the row of beat marks, just inside the bar numbers.
	//
	// Nearest the music of anything that belongs to the whole system,
	// because it is read against the music: an arrow four spaces above the
	// note it is pointing at is pointing at nothing in particular. It is
	// booked across the system's whole width, so the bar numbers and
	// everything beyond them ride above the row rather than through it.
	rankBeatMark  = 80
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
	for i := 0; i < len(marks); {
		// The run that belongs on one line with this one. Contiguous
		// because the sort above is by rank and then by x, and a joined
		// run is marks of one rank following one another across the page.
		j := i + 1
		if marks[i].join != 0 {
			for j < len(marks) && marks[j].join == marks[i].join {
				j++
			}
		}
		if j == i+1 {
			marks[i].draw(sky.place(marks[i].x0, marks[i].x1,
				marks[i].height, marks[i].gap, marks[i].below))
			i = j
			continue
		}
		// All of them against the whole run's reach, so the line is clear
		// of everything under any of them, and all of them are on it.
		x0, x1 := marks[i].x0, marks[i].x1
		height, gap := marks[i].height, marks[i].gap
		for _, m := range marks[i+1 : j] {
			if m.x0 < x0 {
				x0 = m.x0
			}
			if m.x1 > x1 {
				x1 = m.x1
			}
			if m.height > height {
				height = m.height
			}
			if m.gap > gap {
				gap = m.gap
			}
		}
		at := sky.place(x0, x1, height, gap, marks[i].below)
		for _, m := range marks[i:j] {
			m.draw(at)
		}
		i = j
	}
}
