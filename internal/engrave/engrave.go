/*
 * engrave.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package engrave turns music into geometry.
//
// Engrave is a pure function: a score and a spacing table go in, a Layout of
// glyphs and lines comes out, and nothing is stored back on the model. That
// separation is the point of the rewrite. In gscore 0.0.9 the two were fused —
// every Object_t carried six screen coordinates, with a comment on them
// reading "BTW: and YES, I don't like that!" — which meant the layout could
// not be tested, reused for printing, or recomputed at a different size.
//
// Every coordinate here is in staff spaces, with y increasing downward and the
// top line of a staff at its own y. A renderer multiplies by whatever size it
// wants.
package engrave

import (
	"math"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Glyph is one character of the music font, placed.
type Glyph struct {
	Name string
	X, Y float64
	// Ref identifies the model element this came from, numbered in the order
	// the score is read. Playback numbers its events the same way, which is
	// what lets the editor highlight the note that is sounding.
	Ref int
}

// Line is a straight stroke: staff lines, stems, ledger lines, barlines, beams.
type Line struct {
	X1, Y1, X2, Y2 float64
	Thickness      float64
	// Dash is the length of the dashes, in staff spaces, for the broken lines
	// that octave signs are drawn with. Zero is a solid line.
	Dash float64
}

// Text is a word on the page, sized in staff spaces.
type Text struct {
	S      string
	X, Y   float64
	Size   float64
	Anchor string // "start", "middle" or "end"
	Italic bool
	Bold   bool
}

// Curve is a slur or a tie, drawn as a quadratic arc.
type Curve struct {
	X1, Y1, CX, CY, X2, Y2 float64
	Thickness              float64
}

// Layout is a complete drawing, in staff spaces.
type Layout struct {
	Width, Height float64
	Glyphs        []Glyph
	Lines         []Line
	Texts         []Text
	Curves        []Curve
	// Refs maps an element's Ref to where its notehead landed, so the editor
	// can put a playback cursor on it without re-deriving the layout.
	Refs map[int]RefPos

	// defaults is what the layout was drawn with.
	defaults smufl.EngravingDefaults

	// Barlines are where each measure ends, so one can be clicked and
	// changed. A barline is drawn between two bars and belongs to neither as
	// far as the eye is concerned, which is why it needs saying where it is
	// rather than being worked out from a measure's extent.
	Barlines []BarlinePos `json:"barlines"`

	// Hits are the clickable regions, one per drawn measure. The editor needs
	// to turn a point back into a place in the music, and the engraver is the
	// only thing that knows where anything went.
	Hits []Hit
}

// Hit is one measure's region on the page, in staff spaces.
type Hit struct {
	Part    int     `json:"part"`
	Measure int     `json:"measure"`
	X0, X1  float64 `json:"-"`
	XMin    float64 `json:"x0"`
	XMax    float64 `json:"x1"`
	// StaffTop is the y of the staff's top line; a click's distance below it
	// gives the staff step, and the clef turns that into a pitch.
	StaffTop float64 `json:"staffTop"`
	Clef     string  `json:"clef"`
	// Elements is what the measure already holds, with where each one landed.
	// A click needs this to tell "select that note" from "put a note here",
	// which is the difference between reading a score and writing one.
	Elements []ElementPos `json:"elements"`
}

// ElementPos is one element's place on the page.
// BarlinePos is one drawn barline: which measure it closes, and where.
type BarlinePos struct {
	Part    int     `json:"part"`
	Measure int     `json:"measure"`
	X       float64 `json:"x"`
	Top     float64 `json:"top"`
	Bottom  float64 `json:"bottom"`
	Style   string  `json:"style"`
}

type ElementPos struct {
	// Ref is the element's number, shared with the drawn glyph and with
	// playback, so selecting, highlighting and sounding all agree.
	Ref int `json:"ref"`
	// Index is the element's position within its measure's first voice.
	Index int     `json:"index"`
	X     float64 `json:"x"`
	// Y is the topmost notehead and Y2 the lowest; they differ only for a
	// chord, which a click may land anywhere within.
	Y  float64 `json:"y"`
	Y2 float64 `json:"y2"`
	// Width is how much room the element occupies, for hit testing.
	Width float64 `json:"width"`
	// Steps are the staff steps of the element's noteheads, lowest first. A
	// click at a step already here means that notehead; a click at any other
	// step in the same column means a note is being added to the chord, and
	// the editor cannot tell the two apart without knowing which are taken.
	Steps []int `json:"steps,omitempty"`
	// Kind is "note", "chord", "rest" or "symbol".
	Kind string `json:"kind"`
}

// Defaults returns the engraving measurements a layout was drawn with, so a
// reader can tell a stem from a beam from a staff line by thickness.
func (l *Layout) Defaults() smufl.EngravingDefaults { return l.defaults }

// RefPos is where an element ended up.
type RefPos struct {
	X, Y  float64
	Staff float64 // y of the staff's top line
}

// Mode selects how the music is broken into lines.
type Mode int

const (
	// Paginated breaks the music into systems that fit a width and stretches
	// each to reach the margin. This is what printing needs.
	Paginated Mode = iota
	// Strip is the behaviour of gscore 0.0.9: one unbroken horizontal run of
	// music, never wrapped. It remains the better mode for editing, which is
	// why the program called itself non-page-oriented.
	Strip
)

// Options control one engraving run.
type Options struct {
	Spacings Spacings
	Defaults smufl.EngravingDefaults
	Mode     Mode
	// LineWidth is how much room a system has, in staff spaces, between the
	// margins. Ignored in Strip mode.
	LineWidth float64
	// ShowTitle draws the score's title block above the first system.
	ShowTitle bool
	// HideTimeSignature leaves the meter off, for a picture that is only
	// meant to show a clef and a key signature.
	HideTimeSignature bool
}

// DefaultOptions returns a sensible engraving setup.
func DefaultOptions() Options {
	return Options{
		Spacings:  DefaultSpacings(),
		Defaults:  smufl.DefaultEngraving(),
		Mode:      Paginated,
		LineWidth: 120,
		ShowTitle: true,
	}
}

const (
	// staffLines is the usual five.
	staffLines = 5
	// staffHeight is the distance from the top line to the bottom, in spaces.
	staffHeight = staffLines - 1
	// middleStep is the staff step of the middle line, the pivot for stem
	// direction, from realize_area.c:605.
	middleStep = 4
	// stemLength is the standard stem, in staff spaces.
	stemLength = 3.5
	// dynamicDrop is how far below the bottom staff line a dynamic sits.
	dynamicDrop = 3.2
)

// clefPitchOffset converts a pitch's diatonic index into a staff step, where
// step 0 is the bottom line and each step is half a staff space.
//
// For the treble clef, E4 sits on the bottom line: its diatonic index is 2, so
// the offset is -2. The others follow from where their reference line falls.
var clefPitchOffset = map[model.Clef]int{
	model.Treble:     -2,
	model.Bass:       10,
	model.Alto:       4,
	model.Tenor:      6,
	model.Percussion: 4,
}

// clefRefStep is the staff step the clef glyph's own origin sits on.
var clefRefStep = map[model.Clef]float64{
	model.Treble:     2, // the G line
	model.Bass:       6, // the F line
	model.Alto:       4,
	model.Tenor:      6,
	model.Percussion: 4,
}

// StaffStep returns which line or space a pitch occupies: 0 is the bottom
// line, 8 the top. An accidental never moves it, which is why this reads
// DiatonicIndex and not MIDI.
func StaffStep(c model.Clef, p model.Pitch) int {
	return p.DiatonicIndex() + clefPitchOffset[c]
}

// stepY converts a staff step into a y offset from the staff's top line.
func stepY(step float64) float64 { return staffHeight - step/2 }

// Engrave lays out a score.
func Engrave(s *model.Score, opt Options) *Layout {
	e := &engraver{opt: opt, score: s,
		spans: map[int]*openSpans{},
		lay:   &Layout{Refs: map[int]RefPos{}, defaults: opt.Defaults}}
	e.run()
	return e.lay
}

type engraver struct {
	opt   Options
	score *model.Score
	lay   *Layout
	y     float64 // top line of the staff being drawn
	clef  model.Clef
	// nextRef numbers elements in reading order. It starts at 1 so that zero
	// can mean "belongs to no element".
	nextRef int
	// partIndex is whose staff is being drawn, for the hit regions.
	partIndex int

	// spans holds the hairpins and octave lines that are open on each staff.
	// A spanner reaches from one note to another, which may be in a later
	// measure or on a later line, so it cannot be drawn where it starts: what
	// is open is carried along and drawn when it closes, or broken at the end
	// of the system and resumed on the next.
	spans map[int]*openSpans
}

// openSpans are the spanners in progress on one staff.
type openSpans struct {
	hairpin *openSpan
	octave  *openSpan
}

// openSpan is a spanner that has started and not yet finished.
type openSpan struct {
	x      float64 // where it began on this system
	kind   model.HairpinKind
	amount int    // octaves, for an octave line
	label  string // what is written over an octave line
}

// slot is one element's horizontal allocation within a measure.
type slot struct {
	elem  model.Element
	lead  float64 // room before the notehead, for an accidental
	width float64 // the element's own ideal width
	index int     // the element's Ref
}

// measureBox is a measure with its computed widths.
type measureBox struct {
	m       *model.Measure
	index   int
	slots   []slot
	ideal   float64
	minimum float64

	// change is the key or meter change written at the head of this measure,
	// and prefix how much room it takes. Music changes key in the middle of a
	// piece far more often than it starts in one, so this is not an edge
	// case to bolt on afterwards.
	change signatureChange
	prefix float64
}

// signatureChange is a new key or meter taking effect at a measure.
type signatureChange struct {
	key     *model.KeySignature
	from    model.KeySignature
	time    *model.TimeSignature
	hasKey  bool
	hasTime bool
}

func (e *engraver) run() {
	sp := e.opt.Spacings
	e.y = sp.marginY()
	e.nextRef = 1

	if e.opt.ShowTitle {
		e.titleBlock()
		// Leave room for whatever reaches above the first staff. A title set
		// at a fixed distance lands on the ledger lines of a high passage,
		// and these scales climb well above the staff.
		e.y += e.overhang()
	}
	// Brackets and octave signs are drawn above the top staff, which on the
	// first system means above the top of the page unless room is made.
	e.y += e.headroom()
	e.layout()

	e.lay.Height = e.y + sp.marginY()
	if e.lay.Width == 0 {
		e.lay.Width = e.opt.LineWidth + 2*sp.Margin
	}
}

// layout lays every part out together, so that one system holds one staff per
// part and the same measure begins at the same x on all of them.
//
// This is what makes a score readable as lanes, the way NoteWorthy Composer
// shows it: the eye runs down a column and finds the same moment in every
// part. Engraving each part on its own, as the first version did, left the
// barlines at different places on every staff and the columns meaningless.
// headroom is how much space the marks written above the staff need.
//
// It is measured rather than reserved always: a score with no repeats and no
// octave signs should not be pushed down the page for marks it does not have.
func (e *engraver) headroom() float64 {
	need := 0.0
	for _, p := range e.score.Parts {
		for _, m := range p.Measures {
			if m.Ending != nil && need < voltaRise+1 {
				need = voltaRise + 1
			}
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil || mk.Octave == nil || mk.Octave.Amount <= 0 {
						continue
					}
					if need < octaveRise+1 {
						need = octaveRise + 1
					}
				}
			}
		}
	}
	return need
}

// marksOf gets an element's marks, or nil for one that carries none.
func marksOf(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

func (e *engraver) layout() {
	parts := e.score.Parts
	if len(parts) == 0 {
		return
	}

	// Refs are claimed here, walking parts then measures, which is the order
	// playback walks in too.
	boxes := make([][]measureBox, len(parts))
	maxMeasures := 0
	for pi, p := range parts {
		for mi, m := range p.Measures {
			e.clef = p.Clef
			ch, w := e.signatureAt(p, mi)
			b := e.measureBox(p, m)
			b.index = mi
			b.change, b.prefix = ch, w
			b.ideal += w
			b.minimum += w
			boxes[pi] = append(boxes[pi], b)
		}
		if n := len(boxes[pi]); n > maxMeasures {
			maxMeasures = n
		}
	}

	// A measure is as wide as the part that needs the most room for it.
	widths := make([]float64, maxMeasures)
	for mi := 0; mi < maxMeasures; mi++ {
		for pi := range parts {
			if mi < len(boxes[pi]) && boxes[pi][mi].ideal > widths[mi] {
				widths[mi] = boxes[pi][mi].ideal
			}
		}
	}

	// One prefix width for all parts, so the music starts in the same column
	// however wide each part's own clef and key signature happen to be. It is
	// measured where the system begins, because a key change earlier in the
	// piece is restated at the head of every system after it.
	prefixAt := func(at int) float64 {
		w := 0.0
		for _, p := range parts {
			if v := e.prefixWidth(p, at); v > w {
				w = v
			}
		}
		return w
	}
	prefix := prefixAt(0)

	if maxMeasures == 0 {
		// A part with no measures still has a clef and a key signature, and
		// drawing just those is what the key signature picker asks for.
		e.drawSystem(parts, boxes, widths, 0, 0, prefix, true)
		// There is no next system, so the gap that separates them is just
		// empty space under the staff.
		e.y -= e.opt.Spacings.SystemGap
		return
	}
	for start := 0; start < maxMeasures; {
		prefix = prefixAt(start)
		n := e.breakAt(widths[start:], prefix)
		end := start + n
		last := end >= maxMeasures
		e.drawSystem(parts, boxes, widths, start, end, prefix, last)
		if last {
			// The gap belongs between systems, not after the final one.
			// Leaving it there pads every score with empty space, which is
			// glaring when several are shown one under another.
			e.y -= e.opt.Spacings.SystemGap
		}
		start = end
	}
}

// breakAt returns how many measures fit on one system.
func (e *engraver) breakAt(widths []float64, prefix float64) int {
	if e.opt.Mode == Strip {
		return len(widths)
	}
	avail := e.opt.LineWidth - prefix
	used, n := 0.0, 0
	for _, w := range widths {
		if n > 0 && used+w > avail {
			break
		}
		used += w
		n++
	}
	if n == 0 {
		n = 1 // a measure too wide for any line still has to go somewhere
	}
	return n
}

// overhang is how far the music rises above the top staff line, in staff
// spaces, counting ledger lines and anything stemmed or marked upward.
func (e *engraver) overhang() float64 {
	highest := staffLines * 2 // the top line, in staff steps
	stemUp := false

	for _, p := range e.score.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					step, up, ok := topOf(p.Clef, el)
					if !ok {
						continue
					}
					if step > highest {
						highest = step
						stemUp = up
					} else if step == highest && up {
						stemUp = true
					}
				}
			}
		}
	}

	// Steps are half a space each, measured from the bottom line.
	above := float64(highest-staffHeight*2) / 2
	if above <= 0 {
		return 0
	}
	// A ledger line and an articulation need room of their own, and a stem
	// pointing up needs its whole length.
	clearance := 1.0
	if stemUp {
		clearance += stemLength
	}
	return above + clearance
}

// topOf returns the highest staff step an element reaches and whether its stem
// points that way.
func topOf(c model.Clef, el model.Element) (step int, stemUp bool, ok bool) {
	switch n := el.(type) {
	case *model.Note:
		s := StaffStep(c, n.Pitch)
		return s, n.Stem == model.StemUp || (n.Stem == model.StemAuto && s < middleStep), true
	case *model.Chord:
		if len(n.Notes) == 0 {
			return 0, false, false
		}
		hi, lo := StaffStep(c, n.Notes[0].Pitch), StaffStep(c, n.Notes[0].Pitch)
		for _, x := range n.Notes {
			v := StaffStep(c, x.Pitch)
			if v > hi {
				hi = v
			}
			if v < lo {
				lo = v
			}
		}
		up := n.Stem == model.StemUp ||
			(n.Stem == model.StemAuto && (middleStep-lo) >= (hi-middleStep))
		return hi, up, true
	}
	return 0, false, false
}

func (e *engraver) titleBlock() {
	sp := e.opt.Spacings
	centre := sp.Margin + e.opt.LineWidth/2
	if e.score.Identity.Title != "" {
		e.y += 3
		e.lay.Texts = append(e.lay.Texts, Text{
			S: e.score.Identity.Title, X: centre, Y: e.y,
			Size: 2.6, Anchor: "middle", Bold: true,
		})
		e.y += 1.6
	}
	if e.score.Identity.Subtitle != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: e.score.Identity.Subtitle, X: centre, Y: e.y,
			Size: 1.5, Anchor: "middle", Italic: true,
		})
		e.y += 1.6
	}
	if e.score.Identity.Composer != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: e.score.Identity.Composer, X: sp.Margin + e.opt.LineWidth, Y: e.y,
			Size: 1.5, Anchor: "end", Italic: true,
		})
		e.y += 2
	}
	e.y += 2
}

func (e *engraver) prefixWidth(p *model.Part, at int) float64 {
	sp := e.opt.Spacings
	clef, _ := smufl.ClefGlyph(p.Clef)
	w := sp.ClefBefore + smufl.Advance(clef) + sp.ClefAfter
	w += e.keySignatureWidth(p.KeyAt(at))
	if e.opt.HideTimeSignature {
		return w
	}
	num, den, sym := smufl.TimeSignatureGlyphs(p.TimeAt(at))
	if sym != "" {
		w += smufl.Advance(sym)
	} else {
		w += math.Max(glyphRunWidth(num), glyphRunWidth(den))
	}
	return w + sp.AfterTimeSignature
}

func glyphRunWidth(names []string) float64 {
	w := 0.0
	for _, n := range names {
		w += smufl.Advance(n)
	}
	return w
}

func (e *engraver) keySignatureWidth(k model.KeySignature) float64 {
	sp := e.opt.Spacings
	n := k.Fifths
	if n < 0 {
		n = -n
	}
	if n == 0 {
		return 0
	}
	glyph := "accidentalSharp"
	if k.Fifths < 0 {
		glyph = "accidentalFlat"
	}
	return float64(n)*(smufl.Advance(glyph)+sp.BetweenKeyAccidentals) + sp.AfterKeySignature
}

// measureBox computes an ideal and a minimum width for a measure, and claims
// a Ref for each of its elements.
func (e *engraver) measureBox(p *model.Part, m *model.Measure) measureBox {
	sp := e.opt.Spacings
	box := measureBox{m: m}

	var elems []model.Element
	if len(m.Voices) > 0 {
		elems = m.Voices[0].Elements
	}
	for _, el := range elems {
		s := slot{elem: el, index: e.nextRef}
		e.nextRef++
		s.width = e.elementWidth(el)
		s.lead = e.accidentalLead(el)
		box.slots = append(box.slots, s)
		box.ideal += s.lead + s.width
		box.minimum += s.lead + sp.MinNote
	}
	if len(elems) == 0 {
		// An empty measure still needs room to be clicked into.
		box.ideal = sp.NoteBase * 2
		box.minimum = sp.NoteBase
	}
	// The gap and the barline are part of what a measure occupies, so they
	// belong in its width. Leaving them out was the other half of the
	// overflow bug.
	box.ideal += sp.BetweenMeasures + sp.AfterBarline
	box.minimum += sp.BetweenMeasures + sp.AfterBarline
	return box
}

// signatureAt works out what, if anything, is written at the head of a
// measure, and how wide it is.
//
// A key change is not only the new signature: the accidentals the old one had
// and the new one does not are cancelled with naturals first, which is why
// what came before has to be known here.
func (e *engraver) signatureAt(p *model.Part, index int) (signatureChange, float64) {
	var ch signatureChange
	if index < 0 || index >= len(p.Measures) {
		return ch, 0
	}
	m := p.Measures[index]
	// The first measure's signature is drawn in the system prefix, not here.
	if index == 0 {
		return ch, 0
	}
	sp := e.opt.Spacings
	w := 0.0

	if m.Key != nil {
		ch.key, ch.hasKey = m.Key, true
		ch.from = p.KeyAt(index - 1)
		naturals, accidentals := keyChangeSteps(p.Clef, ch.from, *m.Key)
		for range naturals {
			w += smufl.Advance("accidentalNatural") + sp.BetweenKeyAccidentals
		}
		glyph := "accidentalSharp"
		if m.Key.Fifths < 0 {
			glyph = "accidentalFlat"
		}
		for range accidentals {
			w += smufl.Advance(glyph) + sp.BetweenKeyAccidentals
		}
		if w > 0 {
			w += sp.AfterKeySignature
		}
	}
	if m.Time != nil && !e.opt.HideTimeSignature {
		ch.time, ch.hasTime = m.Time, true
		num, den, sym := smufl.TimeSignatureGlyphs(*m.Time)
		if sym != "" {
			w += smufl.Advance(sym) + sp.AfterTimeSignature
		} else {
			w += math.Max(glyphRunWidth(num), glyphRunWidth(den)) + sp.AfterTimeSignature
		}
	}
	if w > 0 {
		// A signature change is announced, not squeezed in: a little air in
		// front of it separates it from the bar it follows.
		w += sp.ClefBefore
	}
	return ch, w
}

// keyChangeSteps gives the staff steps of the naturals that cancel the old key
// and of the accidentals of the new one.
//
// Cancelling is not a matter of taste: going from three sharps to one flat,
// the two sharps that go away are written out as naturals, or a player reads
// them as still in force. Going from one sharp to three, nothing is cancelled
// because nothing was taken away.
func keyChangeSteps(c model.Clef, from, to model.KeySignature) (naturals, accidentals []int) {
	accidentals = keyAccidentalSteps(c, to.Fifths)

	// Same side of the circle and growing: nothing is lost, nothing cancels.
	old := keyAccidentalSteps(c, from.Fifths)
	sameSide := (from.Fifths >= 0) == (to.Fifths >= 0)
	keep := 0
	if sameSide {
		keep = len(accidentals)
	}
	if keep > len(old) {
		keep = len(old)
	}
	for _, step := range old[keep:] {
		naturals = append(naturals, step)
	}
	return naturals, accidentals
}

// accidentalLead is how much room an element needs to the left of its
// noteheads for the accidentals written on them.
//
// A chord needs this as much as a single note does, and for longer: three
// noteheads a second apart carry their accidentals in a column, not one on top
// of another. Taking the widest is enough for them not to collide with the
// music in front.
func (e *engraver) accidentalLead(el model.Element) float64 {
	sp := e.opt.Spacings
	widest := 0.0
	consider := func(a model.Accidental) {
		if a == model.NoAccidental {
			return
		}
		if w := smufl.Advance(smufl.AccidentalGlyph(a)); w > widest {
			widest = w
		}
	}
	switch n := el.(type) {
	case *model.Note:
		consider(n.Accidental)
	case *model.Chord:
		for _, cn := range n.Notes {
			consider(cn.Accidental)
		}
	}
	if widest == 0 {
		return 0
	}
	return sp.BeforeAccidental + widest + sp.AccidentalToNote
}

// elementWidth is the proportional spacing rule that replaces the fixed
// per-duration table of the C program.
func (e *engraver) elementWidth(el model.Element) float64 {
	sp := e.opt.Spacings
	d := el.Duration()
	if d.IsZero() {
		return sp.MinNote
	}
	quarter := model.Duration{Num: 1, Den: 4}
	w := sp.NoteBase * math.Pow(d.Float()/quarter.Float(), sp.NoteExponent)
	if _, dots, ok := d.Written(); ok && dots > 0 {
		w += sp.DotToNote + float64(dots)*sp.BetweenDots
	}
	return math.Max(w, sp.MinNote)
}

// drawSystem draws one system: one staff per part, with measure start-x shared
// between them so the barlines line up into columns.
func (e *engraver) drawSystem(parts []*model.Part, boxes [][]measureBox,
	widths []float64, start, end int, prefix float64, last bool) {

	sp := e.opt.Spacings

	ideal := 0.0
	for mi := start; mi < end; mi++ {
		ideal += widths[mi]
	}

	// Stretch to the margin, except on the last system, which would look wrong
	// pulled out to full width.
	stretch := 1.0
	if e.opt.Mode == Paginated && !last && ideal > 0 {
		if avail := e.opt.LineWidth - prefix; avail > ideal {
			stretch = avail / ideal
		}
	}

	// The column each measure starts in, shared by every staff.
	columns := make([]float64, end-start+1)
	x := sp.Margin + prefix
	for mi := start; mi < end; mi++ {
		columns[mi-start] = x
		x += widths[mi] * stretch
	}
	columns[end-start] = x
	right := x

	top := e.y
	// The volta brackets belong to the system, not to a staff: they are drawn
	// once above the top one before the staves move the cursor down.
	e.drawVoltas(parts, columns, start, end, top)

	for pi, p := range parts {
		e.clef = p.Clef
		// A hairpin or an octave line cut off at the end of the last system
		// starts again here, at this system's left edge.
		e.resumeSpans(pi, sp.Margin+prefix)

		px := sp.Margin + sp.ClefBefore
		px = e.drawClef(p, px)
		px = e.drawKeySignature(p, p.KeyAt(start), px)
		e.drawTimeSignature(p, p.TimeAt(start), px)

		for mi := start; mi < end; mi++ {
			if mi >= len(boxes[pi]) {
				continue // this part has run out of music before the others
			}
			b := boxes[pi][mi]
			e.partIndex = pi
			// A change in the measure that opens a system is already written
			// in the system's head: a new key is announced once, not twice.
			ideal := b.ideal
			if mi == start && b.prefix > 0 {
				ideal -= b.prefix
				b.prefix = 0
			}
			slotWidth := widths[mi] * stretch
			// Each part fills its column at whatever rate its own content
			// needs, so the columns stay aligned even when one staff is busier.
			s := 1.0
			if ideal > 0 {
				s = slotWidth / ideal
			}
			e.drawMeasure(p, b, columns[mi-start], s)
		}

		// The staff lines are drawn last, reaching exactly as far as the
		// music. Nothing can hang off the end of them.
		e.staffLines(sp.Margin, right-sp.Margin)
		e.breakSpans(pi, right-sp.Margin)

		if pi < len(parts)-1 {
			e.y += staffHeight + sp.StaffGap
		}
	}

	e.y = top + float64(len(parts)-1)*(staffHeight+sp.StaffGap)
	e.y += staffHeight + sp.SystemGap

	if r := right + sp.Margin; r > e.lay.Width {
		e.lay.Width = r
	}
}

func (e *engraver) staffLines(x, width float64) {
	t := e.opt.Defaults.StaffLineThickness
	for i := 0; i < staffLines; i++ {
		y := e.y + float64(i)
		e.lay.Lines = append(e.lay.Lines, Line{X1: x, Y1: y, X2: x + width, Y2: y, Thickness: t})
	}
}

func (e *engraver) drawClef(p *model.Part, x float64) float64 {
	name, _ := smufl.ClefGlyph(p.Clef)
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: x, Y: e.y + stepY(clefRefStep[p.Clef]),
	})
	return x + smufl.Advance(name) + e.opt.Spacings.ClefAfter
}

// keyAccidentalSteps returns where each accidental of a key signature sits.
//
// The accidentals do not simply climb: each one is written a fourth below the
// one before it, and when that would drop off the bottom of the usual band it
// goes up a fifth instead. That zigzag is what produces the familiar shapes,
// and following the rule rather than hardcoding the treble pattern makes the
// other clefs come out right too.
func keyAccidentalSteps(c model.Clef, fifths int) []int {
	n := fifths
	sharps := fifths > 0
	if !sharps {
		n = -fifths
	}
	if n == 0 {
		return nil
	}
	if n > 7 {
		n = 7
	}

	start := keySignatureStart(c, sharps)
	out := make([]int, 0, n)
	cur := start
	for i := 0; i < n; i++ {
		out = append(out, cur)
		if sharps {
			// Down a fourth, or up a fifth when that leaves the band.
			if cur-3 < start-5 {
				cur += 4
			} else {
				cur -= 3
			}
		} else {
			// Flats mirror it: up a fourth, or down a fifth.
			if cur+3 > start+3 {
				cur -= 4
			} else {
				cur += 3
			}
		}
	}
	return out
}

// keySignatureStart is where the first accidental goes: F sharp or B flat,
// placed for the clef in hand. The treble positions are the known ones, and
// every other clef is that shifted by how far its pitches sit.
func keySignatureStart(c model.Clef, sharps bool) int {
	const trebleSharp, trebleFlat = 8, 4
	base := trebleFlat
	lo, hi := 1, 7
	if sharps {
		base = trebleSharp
		lo, hi = 3, 9
	}
	step := base + (clefPitchOffset[c] - clefPitchOffset[model.Treble])
	for step > hi {
		step -= 7
	}
	for step < lo {
		step += 7
	}
	return step
}

func (e *engraver) drawKeySignature(p *model.Part, k model.KeySignature, x float64) float64 {
	if k.Fifths == 0 {
		return x
	}
	sp := e.opt.Spacings
	glyph := "accidentalSharp"
	if k.Fifths < 0 {
		glyph = "accidentalFlat"
	}
	for _, step := range keyAccidentalSteps(p.Clef, k.Fifths) {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: glyph, X: x, Y: e.y + stepY(float64(step)),
		})
		x += smufl.Advance(glyph) + sp.BetweenKeyAccidentals
	}
	return x + sp.AfterKeySignature
}

func (e *engraver) drawTimeSignature(p *model.Part, t model.TimeSignature, x float64) float64 {
	if e.opt.HideTimeSignature {
		return x
	}
	return e.placeTimeSignature(t, x)
}

// placeTimeSignature draws one meter at x, wherever it is called for: at the
// head of a system or in the middle of a line where the meter changes.
func (e *engraver) placeTimeSignature(t model.TimeSignature, x float64) float64 {
	sp := e.opt.Spacings
	num, den, sym := smufl.TimeSignatureGlyphs(t)
	if sym != "" {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: sym, X: x, Y: e.y + stepY(4)})
		return x + smufl.Advance(sym) + sp.AfterTimeSignature
	}

	nw, dw := glyphRunWidth(num), glyphRunWidth(den)
	w := math.Max(nw, dw)
	// The two halves are centred on each other, over and under the middle line.
	place := func(names []string, runWidth, step float64) {
		gx := x + (w-runWidth)/2
		for _, n := range names {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: n, X: gx, Y: e.y + stepY(step)})
			gx += smufl.Advance(n)
		}
	}
	place(num, nw, 6)
	place(den, dw, 2)
	return x + w + sp.AfterTimeSignature
}

func (e *engraver) drawBarline(style model.BarlineStyle, x float64) {
	if style == model.BarNone {
		return
	}
	d := e.opt.Defaults
	top, bottom := e.y, e.y+staffHeight

	thin := func(at float64) {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: at, Y1: top, X2: at, Y2: bottom, Thickness: d.ThinBarlineThickness,
		})
	}
	thick := func(at float64) {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: at, Y1: top, X2: at, Y2: bottom, Thickness: d.ThickBarlineThickness,
		})
	}

	switch style {
	case model.BarDouble:
		thin(x)
		thin(x + d.BarlineSeparation)
	case model.BarFinal:
		thin(x)
		thick(x + d.BarlineSeparation + d.ThickBarlineThickness/2)
	case model.BarRepeatEnd:
		e.repeatDots(x - 0.7)
		thin(x)
		thick(x + d.BarlineSeparation + d.ThickBarlineThickness/2)
	case model.BarRepeatStart:
		thick(x)
		thin(x + d.BarlineSeparation + d.ThickBarlineThickness/2)
		e.repeatDots(x + 1.1)
	default:
		thin(x)
	}
}

func (e *engraver) repeatDots(x float64) {
	for _, step := range []float64{3, 5} {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "augmentationDot", X: x, Y: e.y + stepY(step),
		})
	}
}
