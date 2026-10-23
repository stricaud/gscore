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
	"strings"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Glyph is one character of the music font, placed.
type Glyph struct {
	Name string
	X, Y float64
	// Size overrides the staff-space size the glyph is drawn at. A brace has
	// to stretch to however many staves it joins, which no fixed size can do.
	Size float64
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

// Circle is a round mark: the dot on a chord diagram, the ring over a string
// played open.
//
// It exists because these are the one thing on a score that is neither a
// glyph, a line nor a word. Drawing the ring as the character "○" seemed
// easier and was wrong: it is not in the standard PDF text encodings, so it
// printed as a question mark over every open string.
type Circle struct {
	X, Y   float64
	Radius float64
	// Filled is a dot; otherwise it is a ring of Thickness.
	Filled    bool
	Thickness float64
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
	Circles       []Circle
	// Refs maps an element's Ref to where its notehead landed, so the editor
	// can put a playback cursor on it without re-deriving the layout.
	Refs map[int]RefPos

	// defaults is what the layout was drawn with.
	defaults smufl.EngravingDefaults

	// Systems are where each system begins and ends vertically, in staff
	// spaces. Breaking a drawing into pages has to happen between systems and
	// never through one, and nothing but the engraver knows where they are.
	Systems []SystemPos `json:"systems"`

	// Headers are the clef, key signature and meter at the head of a system,
	// as clickable boxes. They are drawn by the engraver and nothing else
	// knows where they ended up, so the editor cannot turn a click on one
	// into "change the key" without being told.
	Headers []HeaderPos `json:"headers"`

	// Strips are the band above each bar of a system: the place a bar is
	// picked up by, rather than a note in it.
	//
	// Selecting a bar is a thing you need to do — "key change from here" is
	// said about a bar — and until this there was nowhere to click for it. A
	// bar full of notes has no empty space in it, so clicking inside one
	// always found a note instead.
	Strips []StripPos `json:"strips"`

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
	// KeyChange and TimeChange say this bar carries a change of key or meter.
	// The editor offers to take one away only where there is one, rather
	// than offering always and failing when there is not.
	KeyChange  bool `json:"keyChange,omitempty"`
	TimeChange bool `json:"timeChange,omitempty"`
	// Elements is what the measure already holds, with where each one landed.
	// A click needs this to tell "select that note" from "put a note here",
	// which is the difference between reading a score and writing one.
	Elements []ElementPos `json:"elements"`
}

// ElementPos is one element's place on the page.
// StripPos is the clickable band over one bar of one system.
type StripPos struct {
	Measure int     `json:"measure"`
	X0      float64 `json:"x0"`
	X1      float64 `json:"x1"`
	Top     float64 `json:"top"`
	Bottom  float64 `json:"bottom"`
}

// SystemPos is one system's vertical extent and which measures it holds.
type SystemPos struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	First  int     `json:"first"`
	Last   int     `json:"last"`
}

// HeaderPos is a clef, key signature or meter at the head of a system.
type HeaderPos struct {
	Part int `json:"part"`
	// Measure is the bar this system starts at, which is where a change
	// clicked here takes effect.
	Measure int     `json:"measure"`
	Kind    string  `json:"kind"` // "clef", "key" or "time"
	X0      float64 `json:"x0"`
	X1      float64 `json:"x1"`
	Top     float64 `json:"top"`
	Bottom  float64 `json:"bottom"`
}

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
	// At is where in its measure the element begins, as a fraction: 0 at the
	// opening barline and 1 at the closing one.
	//
	// Time, not distance. Notes are spaced proportionally — a whole note is
	// wider than a quarter but nowhere near four times as wide — so a
	// playback cursor that slides across a bar at a constant speed drifts
	// away from the notes it is supposed to be pointing at. Giving each
	// element the moment it sounds lets the cursor be drawn between them
	// instead, which is the only way it lands on the notes.
	At float64 `json:"at"`

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
	// ShowPartNames writes each staff's name at the head of the system: the
	// full name on the first, an abbreviation on the rest, which is how a
	// conductor's score is laid out. On a single-staff piece it is noise, so
	// it is off unless asked for.
	ShowPartNames bool
	// ShowMeasureNumbers writes the bar number over the first measure of each
	// system. Rehearsing without them means counting from the beginning.
	ShowMeasureNumbers bool
	// ShowTempoMark writes the metronome mark and the word that goes with it
	// above the first staff.
	ShowTempoMark bool

	// WriteAhead draws one empty bar past the end of the music, so there is
	// somewhere to click to carry on writing.
	//
	// Without it the only way to add a bar is to fill the last one and let
	// the overflow make a new one, which means the place to click is a place
	// that is not there yet. The bar is drawn, not stored: it becomes real
	// when something is written in it, so a score does not pick up an empty
	// bar at the end just for having been looked at.
	//
	// Not after a final barline. The piece is over.
	WriteAhead bool
}

// DefaultOptions returns a sensible engraving setup.
func DefaultOptions() Options {
	return Options{
		Spacings:  DefaultSpacings(),
		Defaults:  smufl.DefaultEngraving(),
		Mode:          Paginated,
		LineWidth:     120,
		ShowTitle:     true,
		ShowTempoMark: true,
		WriteAhead:    true,
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

	// lyricDrop is how far below the bottom staff line the first verse is
	// written, and lyricLine the distance from one verse to the next.
	lyricDrop = 6.2
	lyricLine = 2.2
	// lyricSize is the type size words are set at, in staff spaces.
	lyricSize = 1.75
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
// staffNumber turns a drawn staff's position into its number in the score.
func (e *engraver) staffNumber(drawn int) int {
	if drawn >= 0 && drawn < len(e.origin) {
		return e.origin[drawn]
	}
	return drawn
}

func stepY(step float64) float64 { return staffHeight - step/2 }

// PitchForStep is the inverse of StaffStep: which pitch is written at a staff
// position under a clef.
//
// It lives here, beside StaffStep, because the two have to agree exactly and
// the surest way to make them agree is for there to be one table. The editor
// used to keep a copy with a comment admitting as much.
func PitchForStep(c model.Clef, step int) model.Pitch {
	return model.PitchFromDiatonicIndex(step - clefPitchOffset[c])
}

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

	// origin maps a drawn staff back to its number in the score, which
	// differs from its position as soon as a staff is hidden.
	origin []int

	// textRise is how far the words written over a staff have to be lifted
	// to clear the chord symbols already there.
	textRise float64

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

	// ahead marks the empty bar drawn past the end of the music: somewhere
	// to click to carry on writing. It has no barline of its own and is not
	// numbered, because it is not a bar of the piece yet.
	ahead bool

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
	e.layout()

	e.lay.Height = e.y + sp.marginY()
	// Whatever hangs below the last staff belongs on the page too. The
	// cursor moves down by staves and gaps, which does not know about a
	// dynamic three spaces under the bottom line or a hairpin under that —
	// so the drawing was taller than the layout said and the last staff's
	// dynamics were cut off by the edge.
	if low := e.lowestInk(); low+sp.marginY() > e.lay.Height {
		e.lay.Height = low + sp.marginY()
	}
	if e.lay.Width == 0 {
		e.lay.Width = e.opt.LineWidth + 2*sp.Margin
	}
}

// lowestInk is how far down the drawing actually reaches.
//
// Measured from what was drawn rather than worked out from what might have
// been: a renderer that has to know in advance about every kind of mark that
// can sit below a staff is a renderer that will be wrong the next time one is
// added.
func (e *engraver) lowestInk() float64 {
	low := 0.0
	at := func(v float64) {
		if v > low {
			low = v
		}
	}
	for _, g := range e.lay.Glyphs {
		// A glyph is drawn from its baseline, so its ink reaches below it.
		at(g.Y + 1)
	}
	for _, t := range e.lay.Texts {
		at(t.Y + t.Size*0.3)
	}
	for _, l := range e.lay.Lines {
		at(l.Y1 + l.Thickness)
		at(l.Y2 + l.Thickness)
	}
	for _, c := range e.lay.Curves {
		at(c.Y1 + c.Thickness)
		at(c.Y2 + c.Thickness)
		at(c.CY)
	}
	for _, c := range e.lay.Circles {
		at(c.Y + c.Radius + c.Thickness)
	}
	return low
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
// verses is how many lines of words are written under the staves, which is
// what the gap between systems has to clear.
func (e *engraver) verses() int {
	most := 0
	for _, p := range e.score.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil {
						continue
					}
					for _, ly := range mk.Lyrics {
						if ly.Verse > most {
							most = ly.Verse
						}
					}
				}
			}
		}
	}
	return most
}

func (e *engraver) systemMarkRoom(parts []*model.Part, start, end int) float64 {
	need := 0.0
	at := func(v float64) {
		if v > need {
			need = v
		}
	}
	for _, p := range parts {
		for mi := start; mi < end && mi < len(p.Measures); mi++ {
			m := p.Measures[mi]
			if m.Ending != nil {
				at(voltaRise + 1)
			}
			if m.Marker != "" {
				at(markerRoom + 1.6)
			}
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil {
						continue
					}
					if mk.Octave != nil && mk.Octave.Amount > 0 {
						at(octaveRise + 1)
					}
					// A direction written on a note stacks above the rest.
					if len(mk.Texts) > 0 {
						at(markerRoom + 2.8)
					}
				}
			}
		}
	}
	// The bar number over the start of a later system needs a little too.
	if e.opt.ShowMeasureNumbers && start > 0 {
		at(2.2)
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

// writesAhead reports whether to draw the extra bar.
//
// Only while editing, which is what the continuous strip is for, and only
// while the music is unfinished: a final barline says the piece is over, and
// offering somewhere to write past it invites a bar nobody wants.
func (e *engraver) writesAhead(parts []*model.Part, count int) bool {
	if !e.opt.WriteAhead || e.opt.Mode != Strip || count == 0 {
		return false
	}
	for _, p := range parts {
		if count-1 < len(p.Measures) {
			switch p.Measures[count-1].RightBarline {
			case model.BarFinal, model.BarRepeatEnd, model.BarRepeatBoth:
				return false
			}
		}
	}
	return true
}

func (e *engraver) layout() {
	all := e.score.Parts
	if len(all) == 0 {
		return
	}

	// A hidden staff is left out of the drawing but not out of the score, so
	// it still claims its element numbers below. Skipping it entirely would
	// renumber everything after it, and playback numbers the same way — the
	// highlight would land on the wrong note on every staff under a hidden
	// one.
	var parts []*model.Part
	// Which staff of the score each drawn one is. The editor addresses a note
	// by the staff it is on, so a drawn staff has to carry its real number
	// and not its position among the ones that happen to be shown.
	var origin []int
	for i, p := range all {
		if !p.Hidden {
			parts = append(parts, p)
			origin = append(origin, i)
		}
	}
	if len(parts) == 0 {
		// Everything hidden is still a page, and showing nothing at all
		// would look like a program that had crashed.
		parts, origin = all[:1], []int{0}
	}
	e.origin = origin

	// Refs are claimed here, walking parts then measures, which is the order
	// playback walks in too.
	// Refs are claimed over every part in score order, hidden ones included,
	// so the numbering matches what playback walks.
	byPart := make(map[*model.Part][]measureBox, len(all))
	for _, p := range all {
		var rows []measureBox
		for mi, m := range p.Measures {
			e.clef = p.Clef
			ch, w := e.signatureAt(p, mi)
			b := e.measureBox(p, m)
			b.index = mi
			b.change, b.prefix = ch, w
			b.ideal += w
			b.minimum += w
			rows = append(rows, b)
		}
		byPart[p] = rows
	}

	boxes := make([][]measureBox, len(parts))
	maxMeasures := 0
	for pi, p := range parts {
		boxes[pi] = byPart[p]
		if n := len(boxes[pi]); n > maxMeasures {
			maxMeasures = n
		}
	}

	// Room to carry on writing, when the piece has not been declared over.
	if e.writesAhead(parts, maxMeasures) {
		for pi, p := range parts {
			b := e.measureBox(p, &model.Measure{Voices: []*model.Voice{{}}})
			b.index = maxMeasures
			b.ahead = true
			boxes[pi] = append(boxes[pi], b)
		}
		maxMeasures++
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
	// leadAt is the room in front of the clefs: the staff names, and the
	// brace or bracket that joins them.
	leadAt := func(at int) float64 {
		w := e.labelWidth(parts, at == 0)
		if len(e.score.Groups) > 0 {
			w += groupGap + bracketThickness
		}
		return w
	}
	// prefixAt is all of it: the names, the bracket, and the widest staff's
	// clef, key and meter — so the music starts in the same column on every
	// staff of the system.
	prefixAt := func(at int) float64 {
		w := 0.0
		for _, p := range parts {
			if v := e.prefixWidth(p, at); v > w {
				w = v
			}
		}
		return w + leadAt(at)
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
		// A credit runs to several lines as often as not — words by one
		// person, music by another, arranged by a third — and they stack at
		// the right rather than running off the edge as one long line.
		for _, line := range strings.Split(e.score.Identity.Composer, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			e.lay.Texts = append(e.lay.Texts, Text{
				S: line, X: sp.Margin + e.opt.LineWidth, Y: e.y,
				Size: 1.5, Anchor: "end", Italic: true,
			})
			e.y += 1.7
		}
		e.y += 0.3
	}
	e.drawTempoMark(sp.Margin)
	e.y += 2
}

// drawTempoMark writes the metronome mark and whatever word goes with it.
//
// "Moderately" and a quarter note equal to 140 are two different things and a
// chart often has both: the word says how it should feel and the number says
// how fast, and a player uses the first and a drummer with a click uses the
// second. They are written together at the left, above the first staff, which
// is where every printed score puts them.
func (e *engraver) drawTempoMark(x float64) {
	if !e.opt.ShowTempoMark {
		return
	}
	text := strings.TrimSpace(e.score.Tempo.Text)
	bpm := e.score.Tempo.BPM
	if text == "" && bpm <= 0 {
		return
	}

	e.y += 1.4
	at := x
	if text != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: text, X: at, Y: e.y, Size: 2.0, Anchor: "start", Bold: true,
		})
		at += textWidth(text, 2.0) + 1.2
	}
	if bpm > 0 {
		// The note is the real glyph, not the letter it looks like: a
		// metronome mark written as "J = 140" is a program that gave up.
		glyph, dots := metronomeGlyph(e.score.Tempo.Unit)
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: glyph, X: at, Y: e.y, Size: 4,
		})
		at += smufl.Advance(glyph)
		for i := 0; i < dots; i++ {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: "metAugmentationDot", X: at, Y: e.y, Size: 4,
			})
			at += smufl.Advance("metAugmentationDot") + 0.1
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: "= " + itoa(bpm), X: at + 0.45, Y: e.y, Size: 2.0, Anchor: "start",
		})
	}
	e.y += 1.2
}

// metronomeGlyph picks the note a metronome mark is counted in.
func metronomeGlyph(d model.Duration) (string, int) {
	v, dots, ok := d.Written()
	if !ok {
		return "metNoteQuarterUp", 0
	}
	switch v {
	case model.Whole:
		return "metNoteWhole", dots
	case model.Half:
		return "metNoteHalfUp", dots
	case model.Eighth:
		return "metNote8thUp", dots
	case model.Sixteenth:
		return "metNote16thUp", dots
	}
	return "metNoteQuarterUp", dots
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
	left, right := e.barlineRoom(m.RightBarline)
	box.ideal += sp.BetweenMeasures + sp.AfterBarline + left + right
	box.minimum += sp.BetweenMeasures + sp.AfterBarline + left + right
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

	// Room is reserved before the system is drawn, not claimed back over the
	// one above it. Recording a system as reaching up into its predecessor
	// made the two overlap, and a page cut between them then sliced the
	// bottom staff off the page above.
	chords := e.systemChordRoom(parts, start, end)
	marks := e.systemMarkRoom(parts, start, end)
	e.y += chords + marks
	top := e.y

	// Everything written over the system stacks above the chord symbols,
	// which are themselves above the staff. Drawing them all at the same
	// height is what put a rehearsal mark through a tempo direction.
	above := top - chords

	// The volta brackets belong to the system, not to a staff: they are drawn
	// once above the top one before the staves move the cursor down.
	e.drawVoltas(parts, columns, start, end, above)
	e.drawMeasureNumbers(columns, start, end, above)
	e.drawMarkers(parts, columns, start, end, above)

	// The band the bar numbers sit in is also where a bar is picked up. One
	// band per bar, reaching to where the next begins, so there is no dead
	// gap over a barline.
	for mi := start; mi < end && mi-start+1 < len(columns); mi++ {
		// Not the empty bar drawn past the end of the music: it is not a bar
		// of the piece yet, so there is nothing there to select.
		real := false
		for _, p := range parts {
			if mi < len(p.Measures) {
				real = true
				break
			}
		}
		if !real {
			continue
		}
		e.lay.Strips = append(e.lay.Strips, StripPos{
			Measure: mi,
			X0:      columns[mi-start],
			X1:      columns[mi-start+1],
			// Stopping at the top staff line, not over it: a band that
			// reached onto the staff would take clicks meant for the notes
			// on its top line.
			Top:    above - stripHeight,
			Bottom: above,
		})
	}
	// A note's own words go above the chord symbols and above the rehearsal
	// marks: three things written over the same bar, in three layers rather
	// than in one place.
	e.textRise = top - above + markerRoom

	// Words written under a staff need the staff below to move out of the way.
	// How much room each staff needs for what is written around it.
	//
	// Per staff, not one figure for the whole system. A chart has the chords
	// over the piano, the words under the voice and nothing around the drums;
	// widening every gap by the worst case pushes a pianist's two staves half
	// a page apart for the sake of a guitar diagram three staves away.
	gapAfter := func(i int) float64 {
		gap := sp.StaffGap + roomBelow(parts[i])
		if i+1 < len(parts) {
			if above := e.partChordRoom(parts[i+1], start, end); above > 0 {
				gap += above
			}
		}
		return gap
	}

	// Where each staff of this system was put, for the things drawn across
	// all of them: the brace, the bracket and the names.
	staffTops := make([]float64, 0, len(parts))

	// How much of the prefix belongs to the names and the bracket rather
	// than to the clefs.
	lead := e.labelWidth(parts, start == 0)
	if len(e.score.Groups) > 0 {
		lead += groupGap + bracketThickness
	}

	for pi, p := range parts {
		e.clef = p.Clef
		// A hairpin or an octave line cut off at the end of the last system
		// starts again here, at this system's left edge.
		e.resumeSpans(e.staffNumber(pi), sp.Margin+prefix)

		staffTops = append(staffTops, e.y)
		// Each staff's clef starts far enough right that its own key and
		// meter end where everybody else's do, so a staff in C major and one
		// in five sharps begin their music in the same column.
		//
		// prefix already counts the names and the bracket. Adding lead here
		// as well pushed every staff's clef, key and meter one name-width too
		// far right — past the column the music starts in, so the first note
		// was printed over them.
		px := sp.Margin + prefix - e.prefixWidth(p, start) + sp.ClefBefore
		// Each of the three is recorded as it is drawn, so that clicking one
		// changes the thing it is: the clef, the key or the meter.
		header := func(kind string, x0, x1 float64) {
			if x1 <= x0 {
				return
			}
			e.lay.Headers = append(e.lay.Headers, HeaderPos{
				Part: pi, Measure: start, Kind: kind,
				X0: x0, X1: x1, Top: e.y - 1, Bottom: e.y + staffHeight + 1,
			})
		}
		was := px
		px = e.drawClef(p, px)
		header("clef", was-sp.ClefBefore/2, px-sp.ClefAfter/2)
		was = px
		px = e.drawKeySignature(p, p.KeyAt(start), px)
		header("key", was, px-sp.AfterKeySignature/2)
		was = px
		px = e.drawTimeSignature(p, p.TimeAt(start), px)
		header("time", was, px-sp.AfterTimeSignature/2)

		for mi := start; mi < end; mi++ {
			if mi >= len(boxes[pi]) {
				continue // this part has run out of music before the others
			}
			b := boxes[pi][mi]
			e.partIndex = e.staffNumber(pi)
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
		// music. Nothing can hang off the end of them. They start after the
		// names and the bracket: a staff ruled through its own name is how
		// a name looks crossed out.
		e.staffLines(sp.Margin+lead, right-sp.Margin-lead)
		e.breakSpans(pi, right-sp.Margin)

		if pi < len(parts)-1 {
			e.y += staffHeight + gapAfter(pi)
		}
	}

	// The names and the brace reach across every staff of the system, so they
	// are drawn once the staves' positions are all known.
	e.drawGroups(parts, sp.Margin+lead, staffTops, start == 0)
	for i := range columns {
		e.drawGroupBarlines(columns[i], staffTops)
	}

	// Back to the top of the system, then down past all of it.
	e.y = top
	for i := 0; i < len(parts)-1; i++ {
		e.y += staffHeight + gapAfter(i)
	}
	e.y += staffHeight + roomBelow(parts[len(parts)-1])
	e.lay.Systems = append(e.lay.Systems, SystemPos{
		// The top reaches above the staff for everything written there: the
		// chord symbols and their diagrams, and above those the brackets of
		// a repeat, the octave signs and the rehearsal marks. A system whose
		// recorded top stopped at the staff had its chord diagrams drawn on
		// the page before it.
		Top:    top - chords - marks,
		Bottom: e.y, First: start, Last: end - 1,
	})
	e.y += sp.SystemGap

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

// barlineRoom is how much space a barline needs on each side of the point it
// is drawn at.
//
// A barline is not always a line. A repeat opens with a thick stroke, a thin
// one and two dots, and the dots sit over a space of their own — so leaving
// the same gap after every barline puts the dots of an opening repeat on top
// of the first note of the bar it opens, which is what it did.
func (e *engraver) barlineRoom(style model.BarlineStyle) (left, right float64) {
	d := e.opt.Defaults
	heavy := d.BarlineSeparation + d.ThickBarlineThickness
	dots := 1.1 + smufl.Advance("augmentationDot")

	switch style {
	case model.BarDouble:
		return 0, d.BarlineSeparation
	case model.BarFinal:
		return 0, heavy
	case model.BarRepeatEnd:
		// The dots come before the line, so the room is on the left.
		return 0.7 + smufl.Advance("augmentationDot"), heavy
	case model.BarRepeatStart:
		return 0, dots
	case model.BarRepeatBoth:
		return 0.7 + smufl.Advance("augmentationDot"), dots
	}
	return 0, 0
}

func (e *engraver) repeatDots(x float64) {
	for _, step := range []float64{3, 5} {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "augmentationDot", X: x, Y: e.y + stepY(step),
		})
	}
}
