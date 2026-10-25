/*
 * engrave.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	"sort"
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

// Beams returns the beams of a drawing.
//
// Every quad in a layout is a beam; the type is named for its shape and this
// is named for what it is, so that anything asking "where are the beams" does
// not have to know they are quads.
func (l *Layout) Beams() []Quad { return l.Quads }

// Left and Right are the beam's ends: its near edge, the one the stems stop
// at, from the first stem to the last.
func (q Quad) Left() (x, y float64)  { return q.X[0], q.Y[0] }
func (q Quad) Right() (x, y float64) { return q.X[1], q.Y[1] }

// Thickness is how deep the beam is, measured at its ends, which are vertical.
func (q Quad) Thickness() float64 {
	t := q.Y[3] - q.Y[0]
	if t < 0 {
		return -t
	}
	return t
}

// Quad is a filled four-sided shape, its corners given in order round it.
//
// It exists for beams. A beam is a parallelogram whose ends are vertical, so
// that they finish flush with the stems they join; drawn as a thick stroked
// line it gets caps square to its own direction instead, which on anything
// but a level beam shows as a point sticking out past the outer stems.
type Quad struct {
	X, Y [4]float64
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
	// Ref is the element this belongs to, where it is one: a fret number on
	// a tab staff is the note, the way a notehead is on a staff of pitches,
	// and the editor and the player have to be able to find it.
	Ref int `json:"ref,omitempty"`
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
	// Quads are filled four-sided shapes. Beams are what need them: a beam's
	// ends are cut vertically, flush with its stems, and a stroked line
	// cannot do that — its caps are square to the line, so a sloping beam
	// came out pointed at both ends and overhanging the first and last stem.
	Quads []Quad
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

	// Dynamics are where the dynamics ended up, so one can be clicked.
	//
	// A dynamic is not where its note is. It is written under the staff, as
	// far down as that stretch of music needs it to be, and the engraver is
	// the only thing that knows how far that turned out to be. Without this
	// the editor could write an ff and then had no way to point at it: a
	// click under the staff found nothing, because everything it knew how to
	// find was on the staff.
	Dynamics []DynamicPos `json:"dynamics"`
}

// DynamicPos is one dynamic's box on the page, in staff spaces.
type DynamicPos struct {
	Part    int `json:"part"`
	Measure int `json:"measure"`
	// Ref is the element the dynamic is written on, the same number the
	// editor uses to reach that element for anything else.
	Ref int `json:"ref"`
	// The box the letters occupy: x and y are its top left corner.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	// Name is the dynamic as the editor spells it, "ff" and the like.
	Name string `json:"name"`
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
	// StaffHeight is from that line to the bottom one. A staff of pitches
	// is four spaces; a tablature is as many as its strings need, set
	// further apart. Anything drawing a box round a staff has to be told
	// rather than assuming.
	StaffHeight float64 `json:"staffHeight"`
	Clef        string  `json:"clef"`
	// KeyChange and TimeChange say this bar carries a change of key or meter.
	// The editor offers to take one away only where there is one, rather
	// than offering always and failing when there is not.
	KeyChange  bool `json:"keyChange,omitempty"`
	TimeChange bool `json:"timeChange,omitempty"`
	// Fifths is the signature in force on this staff at this bar, counting
	// sharps positive and flats negative. It is this staff's written
	// signature and not the concert key, which on a transposing part is a
	// different number — and the written one is what the player in front of
	// you is reading, so it is what a picker should open showing.
	Fifths int `json:"fifths"`
	// Full says the bar already holds as much time as its meter allows.
	//
	// A click in the empty part of a full bar is not a click at the end of
	// the music — it is a click over something that is already sounding
	// there, because the drawn width of a whole-bar rest says nothing about
	// the four beats it lasts. Writing there appended, found the bar full
	// and sent the note into the next bar, which is how clicking one bar
	// wrote in the one beside it.
	Full bool `json:"full,omitempty"`
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
	// Staff is the top line of this system's first staff: where the music
	// begins, as against Top, which is where the room reserved for what is
	// written over it begins. The difference between them is what the next
	// pass measures.
	Staff float64 `json:"staff,omitempty"`
	First int     `json:"first"`
	Last  int     `json:"last"`
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
	// Chord is the chord symbol written over this element, where there is
	// one, and ChordFrets its fingering. The editor needs both to offer
	// what is already there when one is being changed — otherwise changing
	// C7 to C7♯9 means typing the whole thing again.
	Chord      string `json:"chord,omitempty"`
	ChordFrets string `json:"chordFrets,omitempty"`
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
	// HideKeySignature leaves the signature off the head of the system,
	// for a picture of a key *change*: the naturals that cancel the old key
	// and the accidentals of the new one, without the old one printed again
	// beside them. The key picker is what wants it — it is showing what will
	// be written at that bar, not what the bar before it looks like.
	HideKeySignature bool

	// HideTimeSignature leaves the meter off, for a picture that is only
	// meant to show a clef and a key signature.
	HideTimeSignature bool

	// HideClef leaves the clef off as well, for a picture of something that
	// is not on the staff at all — a chord symbol being typed, where a
	// treble clef is most of the width and says nothing.
	HideClef bool
	// ShowPartNames writes each staff's name at the head of the system: the
	// full name on the first, an abbreviation on the rest, which is how a
	// conductor's score is laid out. On a single-staff piece it is noise, so
	// it is off unless asked for.
	ShowPartNames bool
	// ShowMeasureNumbers writes the bar number over the first measure of each
	// system. Rehearsing without them means counting from the beginning, so
	// they are on by default and the View menu takes them away. The editor
	// keeps its own idea of this in app.js, which has to agree with what
	// DefaultOptions sets here or the tick in the menu starts out lying.
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
		Spacings:           DefaultSpacings(),
		Defaults:           smufl.DefaultEngraving(),
		Mode:               Paginated,
		LineWidth:          120,
		ShowTitle:          true,
		ShowTempoMark:      true,
		ShowMeasureNumbers: true,
		WriteAhead:         true,
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

/*
 * Staves that are not five lines of pitches
 */

// tabLineGap is how far apart the lines of a tablature staff are: a space,
// the same as any other staff's. A tab is an ordinary staff with a line for
// each string and numbers where the noteheads would be.
const tabLineGap = 1

// partLines is how many lines a part's staff has: five, or one per string
// where the part is written as tablature.
func partLines(p *model.Part) int {
	if n := p.Tab.Lines(); n > 1 {
		return n
	}
	return staffLines
}

// partGap is the distance between those lines.
func partGap(p *model.Part) float64 {
	if p.Tab.Lines() > 1 {
		return tabLineGap
	}
	return 1
}

// partH is how tall a part's staff is, from its top line to its bottom one.
//
// A tablature staff is taller than a staff of pitches — six lines a space
// and a half apart rather than five a space apart — so the height is a
// property of the part and not a constant. Everything that places one staff
// under another, or measures how far a mark is outside one, asks for it.
func partH(p *model.Part) float64 {
	return float64(partLines(p)-1) * partGap(p)
}

// staffH is the height of the staff being drawn now.
func (e *engraver) staffH() float64 {
	if e.lines < 2 {
		return staffHeight
	}
	return float64(e.lines-1) * e.lineGap
}

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
	lay := engraveWith(s, opt, nil)
	// Again, knowing where the ink went. The room over each system was
	// reserved from a count of what might be written there; this is what
	// was. See room.go — it is worth a page or two on a song.
	if room := roomAbove(lay); len(room) > 0 {
		lay = engraveWith(s, opt, room)
	}
	return lay
}

func engraveWith(s *model.Score, opt Options, room map[int]need) *Layout {
	e := &engraver{opt: opt, score: s, room: room,
		spans: map[[2]int]*openSpans{},
		lay:   &Layout{Refs: map[int]RefPos{}, defaults: opt.Defaults}}
	e.run()
	return e.lay
}

type engraver struct {
	opt   Options
	score *model.Score
	lay   *Layout
	// titleFrom and titleTo bracket the title block's own entries in
	// lay.Texts, so they can be put back on the canvas once its width is
	// known. See centreTitleBlock.
	titleFrom, titleTo int
	// room is how much space each system's ink actually took above its
	// staff, measured on a previous pass and keyed by the system's first
	// bar. Empty on the first pass, when there is nothing to go on but the
	// count.
	room map[int]need
	y    float64 // top line of the staff being drawn
	clef model.Clef
	// lines and lineGap describe the staff being drawn: five lines a space
	// apart for ordinary music, one per string and wider for tablature.
	lines   int
	lineGap float64
	// tab is the tuning of the part being drawn, where it is written as
	// tablature, and nil where it is not.
	tab *model.Tablature
	// tabGaps are the pieces of this staff's lines that are not drawn,
	// because a fret number is sitting on them. They are collected as the
	// bars are drawn and used when the lines are ruled, which happens after.
	tabGaps []tabGap
	// nextRef numbers elements in reading order. It starts at 1 so that zero
	// can mean "belongs to no element".
	nextRef int
	// partIndex is whose staff is being drawn, for the hit regions.
	partIndex int

	// origin maps a drawn staff back to its number in the score, which
	// differs from its position as soon as a staff is hidden.
	origin []int

	// chordRow is this staff's chord symbols, waiting for it to be finished
	// so that they can be written above whatever turned out to be on it.
	chordRow []pendingChord

	// stretch is how far the system being drawn is pulled out to reach the
	// margin. It belongs to the system rather than to any staff on it, which
	// is exactly why the barlines are placed from it.
	stretch float64

	// staffMark and staffTop are where the staff being drawn began, so that
	// its outline can be read at any point while it is being drawn — which
	// is what a slur needs: by the time one closes, everything it arcs over
	// has been drawn, and it has to clear all of it.
	staffMark mark
	staffTop  float64
	staffX0   float64
	staffX1   float64

	// waiting are the marks that go outside the staff and cannot be placed
	// where they are met.
	//
	// A dynamic, a hairpin, an octave line: each has to clear whatever else
	// is under or over that stretch of music, and what that is depends on
	// notes that may not be drawn yet — a hairpin is not even drawn until
	// the bar it closes in, which can be four bars later. So they are
	// collected as the staff is drawn and placed once it is finished, from
	// the staff outwards.
	waiting []waitingMark

	// spans holds the hairpins and octave lines that are open on each staff.
	// A spanner reaches from one note to another, which may be in a later
	// measure or on a later line, so it cannot be drawn where it starts: what
	// is open is carried along and drawn when it closes, or broken at the end
	// of the system and resumed on the next.
	// Keyed by staff and voice. A staff can carry two lines of music, and
	// each keeps its own slurs: a slur opened in the upper voice must not
	// be closed by the lower one reaching the end of its own phrase.
	spans map[[2]int]*openSpans
}

// openSpans are the spanners in progress on one staff.
type openSpans struct {
	hairpin *openSpan
	octave  *openSpan
	slur    *openSpan
	trill   *openSpan
	// ties are kept per notehead, keyed by the staff step they left. A
	// chord is tied notehead by notehead — every voice in it carries on
	// sounding — so one curve for the chord would be one curve short for
	// every note but the first.
	ties map[int]*openSpan
}

// openSpan is a spanner that has started and not yet finished.
type openSpan struct {
	x      float64 // where it began on this system
	kind   model.HairpinKind
	amount int    // octaves, for an octave line
	label  string // what is written over an octave line

	// y and above are where a slur or a tie left the note it started on,
	// and which side of the music it arcs over.
	y     float64
	above bool

	// up and down count the stems of the notes a slur covers, including the
	// ones it starts and ends on.
	//
	// A slur goes on the side away from the stems, and which side that is
	// is a fact about the whole phrase and not about its first note. Taking
	// it from the first note put a slur on the stem side of every note but
	// one wherever a phrase began in a group beamed the other way, and then
	// it had to dive under the beam to clear it.
	up, down int

	// lowest and highest are the outermost noteheads the slur covers, so
	// that once the side is settled it can leave from the right edge of the
	// notes at each end.
	lowest, highest int

	// rel is that same height measured from the top of its staff rather than
	// from the top of the whole drawing. A curve cut at a line break is
	// picked up on the next system, whose staff is somewhere else entirely;
	// without this the continuation was drawn from the height the curve left
	// the previous system at, which is one stroke down the length of the page.
	rel float64
}

// slot is one element's horizontal allocation within a measure.
// voiced is one element and which of the staff's voices it belongs to.
type voiced struct {
	el    model.Element
	voice int
	index int // the element's Ref
}

// slot is one column of a measure: everything that starts at the same moment.
//
// A staff can carry more than one line of music — a held bass note under a
// moving part, two singers on one stave — and they are written across each
// other, not after each other. So a measure is laid out as a row of columns
// at the moments something starts, and every voice puts whatever it has at
// that moment into the same column. With one voice there is one element per
// column and this is the sequence it always was.
type slot struct {
	at    model.Duration // when in the bar, counted from its start
	elems []voiced
	lead  float64 // room before the noteheads, for an accidental
	width float64 // how wide the column is
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
	e.centreTitleBlock()
}

// centreTitleBlock puts the title and the credit on the canvas that was
// actually drawn.
//
// The block is laid out first, before a note is placed, so it has nothing to
// go on but Options.LineWidth. In Strip mode that is not the width of
// anything: the music runs as far as it runs and the canvas is trimmed to
// it. A short piece came out with its title off to the right of centre and
// its composer hanging past the edge, cut off mid-name.
//
// In a paginated score the canvas is LineWidth plus the margins, so every
// sum below gives back exactly where the text already was.
func (e *engraver) centreTitleBlock() {
	sp := e.opt.Spacings
	for i := e.titleFrom; i < e.titleTo && i < len(e.lay.Texts); i++ {
		switch e.lay.Texts[i].Anchor {
		case "middle":
			e.lay.Texts[i].X = e.lay.Width / 2
		case "end":
			e.lay.Texts[i].X = e.lay.Width - sp.Margin
		}
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
	for _, q := range e.lay.Quads {
		for _, y := range q.Y {
			at(y)
		}
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

// systemMarkRoom is how much room to leave over a system for everything
// written there.
//
// It adds the rows up rather than taking the largest of them, because they
// are stacked: a bar number over a letter over an octave line over a note on
// three ledger lines needs all four, and reserving the largest reserved one
// row for four things. The marks then spilled into the gap above, which is
// where the system before it is.
//
// The skyline places each one exactly, within the room reserved here; this
// only has to be enough. It is counted from the music rather than measured
// from it, because nothing has been drawn yet when the system's height is
// decided.
func (e *engraver) systemMarkRoom(parts []*model.Part, start, end int) float64 {
	var hasVolta, hasMark, hasOctave, hasText bool
	for _, p := range parts {
		for mi := start; mi < end && mi < len(p.Measures); mi++ {
			m := p.Measures[mi]
			hasVolta = hasVolta || m.Ending != nil
			hasMark = hasMark || m.Marker != "" || m.Sign != model.NoSign ||
				m.Jump != nil || m.Tempo != nil
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil {
						continue
					}
					if mk.Octave != nil && mk.Octave.Amount > 0 {
						hasOctave = true
					}
					hasText = hasText || len(mk.Texts) > 0
				}
			}
		}
	}

	// What the music itself reaches above the top staff, which everything
	// else is stacked on.
	need := e.musicAbove(parts, start, end)
	if hasOctave {
		need += octaveRise - staffHeight + 2.0
	}
	if hasText {
		need += 2.4
	}
	if hasVolta {
		need += voltaHook + 1.2
	}
	if e.opt.ShowMeasureNumbers && (start > 0 || e.opt.Mode == Strip) {
		need += measureNumberSize + 0.8
	}
	if hasMark {
		need += markRowHeight + 0.8
	}
	// Enough for the chord symbols and the rest even where none of the above
	// is present, which is what the fixed allowance used to give everybody.
	if need < 2.2 {
		need = 2.2
	}
	return need
}

// musicAbove is how far the notes of the top staff reach above it, in staff
// spaces: the ledger lines, and the stems and beams over them.
func (e *engraver) musicAbove(parts []*model.Part, start, end int) float64 {
	if len(parts) == 0 {
		return 0
	}
	p := parts[0]
	highest := staffHeight * 2 // the top line, in staff steps
	for mi := start; mi < end && mi < len(p.Measures); mi++ {
		for _, v := range p.Measures[mi].Voices {
			for _, el := range v.Elements {
				switch n := el.(type) {
				case *model.Note:
					if st := StaffStep(p.Clef, n.Pitch); st > highest {
						highest = st
					}
				case *model.Chord:
					for _, cn := range n.Notes {
						if st := StaffStep(p.Clef, cn.Pitch); st > highest {
							highest = st
						}
					}
				}
			}
		}
	}
	// Half a space per step above the top line, and room over the highest
	// note for a stem and whatever is written on its end.
	return float64(highest-staffHeight*2)/2 + 2.6
}

// What the marks under a staff need, each the height of the mark and the air
// over it. They are the figures from the places that draw them.
const (
	dynamicBand = 1.4 + 0.7
	hairpinBand = hairpinMouth + 0.6
	octaveBelow = octaveBand + 0.5
)

// marksBelow is the room a staff needs under it for the marks that go there.
//
// Counted from the model rather than measured, because a system's height is
// settled before anything in it is drawn. Over-reserving a little costs a
// little air; under-reserving puts a dynamic through the staff below.
//
// Only the kinds that are written under a staff: the dynamics, the hairpins
// that run between them, and an octave line below. The trill's wave, the
// volta and the rehearsal letters go above and are counted there.
//
// Bar by bar, and the worst bar wins. The three bands stack: a hairpin runs
// under the dynamics line and an octave line under that, so where all three
// are written at once the staff needs room for all three. Where they are not,
// it does not — and asking the question once over the whole stretch answered
// "a dynamic somewhere, a hairpin somewhere, an octave line somewhere", which
// over a long stretch is always yes to everything. The continuous strip is
// one system however long the piece is, so a single hairpin in bar 90 pushed
// the staves apart for all 131 bars of the Altes and left a hand's width of
// nothing under the opening.
func marksBelow(p *model.Part, start, end int) float64 {
	worst := 0.0
	for mi := start; mi < end && mi < len(p.Measures); mi++ {
		var dyn, hair, oct bool
		for _, v := range p.Measures[mi].Voices {
			for _, el := range v.Elements {
				m := marksOf(el)
				if m == nil {
					continue
				}
				if m.Dynamic != model.DynamicNone {
					dyn = true
				}
				if m.Hairpin != nil {
					hair = true
				}
				if m.Octave != nil && m.Octave.Amount < 0 {
					oct = true
				}
			}
		}
		room := 0.0
		if dyn {
			room += dynamicBand
		}
		if hair {
			room += hairpinBand
		}
		if oct {
			room += octaveBelow
		}
		if room > worst {
			worst = room
		}
	}
	return worst
}

// staffClearance is the air left between what one staff's music reaches down
// to and what the next one's reaches up to.
//
// Two spaces, which is a notehead and a little: enough that the two runs
// read as two, and that a stem or a slur on either has somewhere to go.
const staffClearance = 2.0

// staffReach is how far one staff's own notes climb above its top line and
// hang below its bottom one, in staff spaces.
//
// Ledger lines only: the stem of a note high above a staff points down,
// towards the staff it belongs to, and the stem of one below it points up,
// so neither reaches further into the gap than the notehead does.
func (e *engraver) staffReach(p *model.Part, start, end int) (above, below float64) {
	const topLine = staffHeight * 2 // the top line, in staff steps
	highest, lowest := topLine, 0
	// Where a staff carries more than one line of music the stems are
	// forced apart — the upper voice up, the lower down — so the outermost
	// notes carry their stems away from the staff rather than back into it.
	// With one voice a low note takes its stem upward and nothing hangs
	// below the notehead, which is why measuring noteheads alone was enough
	// until now and is not any more: a fugue's lower voice reached down to
	// the staff beneath it with a stem nothing had made room for.
	stemOut := 0.0
	for mi := start; mi < end && mi < len(p.Measures); mi++ {
		if len(p.Measures[mi].Voices) > 1 {
			stemOut = stemLength
		}
		for _, v := range p.Measures[mi].Voices {
			for _, el := range v.Elements {
				var steps []int
				switch n := el.(type) {
				case *model.Note:
					steps = []int{StaffStep(p.Clef, n.Pitch)}
				case *model.Chord:
					for _, cn := range n.Notes {
						steps = append(steps, StaffStep(p.Clef, cn.Pitch))
					}
				}
				for _, st := range steps {
					if st > highest {
						highest = st
					}
					if st < lowest {
						lowest = st
					}
				}
			}
		}
	}
	return float64(highest-topLine)/2 + stemOut, float64(-lowest)/2 + stemOut
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
			b := e.measureBox(p, m, mi)
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
			b := e.measureBox(p, &model.Measure{Voices: []*model.Voice{{}}}, maxMeasures)
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
		e.drawSystem(parts, boxes, widths, 0, 0, prefix, 0, true)
		// There is no next system, so the gap that separates them is just
		// empty space under the staff.
		e.y -= e.opt.Spacings.SystemGap
		return
	}
	for start := 0; start < maxMeasures; {
		prefix = prefixAt(start)
		// A tie or a slur carried over from the system above has to come in
		// from somewhere, and the only room at the head of a system is room
		// that was asked for.
		carried := 0.0
		if e.carriedCurves() {
			carried = resumeRoom
		}
		n := e.breakAt(widths[start:], prefix+carried)
		end := start + n
		last := end >= maxMeasures
		e.drawSystem(parts, boxes, widths, start, end, prefix+carried, carried, last)
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
	e.titleFrom = len(e.lay.Texts)
	defer func() { e.titleTo = len(e.lay.Texts) }()
	// A title runs to several lines as often as a credit does — a work, a
	// movement and a catalogue number, which is how a file written by hand
	// in another program arrives. They were drawn as one string and the
	// cursor moved by one line, so every line after the first was drawn on
	// top of the line beneath it and the subtitle landed in the middle of
	// the title.
	for _, line := range strings.Split(e.score.Identity.Title, "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		e.y += 3
		e.lay.Texts = append(e.lay.Texts, Text{
			S: line, X: centre, Y: e.y,
			Size: 2.6, Anchor: "middle", Bold: true,
		})
		e.y += 1.6
	}
	for _, line := range strings.Split(e.score.Identity.Subtitle, "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: line, X: centre, Y: e.y,
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
	if strings.TrimSpace(e.score.Tempo.Text) == "" && e.score.Tempo.BPM <= 0 {
		return
	}
	e.y += 1.4
	e.drawTempoAt(e.score.Tempo, x, e.y)
	e.y += 1.2
}

// drawTempoAt writes one tempo marking at a place on the page and says how
// wide it came out.
//
// Separated from the mark at the head of the score because the same marking
// is written twice over: once above the first bar, where it is part of the
// title block, and again over whichever bar changes speed. Drawing it in two
// places from one piece of code is what keeps "a tempo" at the last chorus
// looking like the mark at the top rather than like a different program.
func (e *engraver) drawTempoAt(t model.Tempo, x, y float64) float64 {
	text := strings.TrimSpace(t.Text)
	bpm := t.BPM
	if text == "" && bpm <= 0 {
		return 0
	}

	at := x
	if text != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: text, X: at, Y: y, Size: 2.0, Anchor: "start", Bold: true,
		})
		at += textWidth(text, 2.0) + 1.2
	}
	if bpm > 0 {
		// The note is the real glyph, not the letter it looks like: a
		// metronome mark written as "J = 140" is a program that gave up.
		glyph, dots := metronomeGlyph(t.Unit)
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: glyph, X: at, Y: y, Size: 4,
		})
		at += smufl.Advance(glyph)
		for i := 0; i < dots; i++ {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: "metAugmentationDot", X: at, Y: y, Size: 4,
			})
			at += smufl.Advance("metAugmentationDot") + 0.1
		}
		label := "= " + itoa(bpm)
		e.lay.Texts = append(e.lay.Texts, Text{
			S: label, X: at + 0.45, Y: y, Size: 2.0, Anchor: "start",
		})
		at += 0.45 + textWidth(label, 2.0)
	}
	return at - x
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
	w := 0.0
	switch {
	case e.opt.HideClef:
		// Nothing at the head but what the key and the meter add below.
		w += e.keySignatureWidth(p.KeyAt(at))
	case p.Tab != nil:
		// The word TAB, and no key signature to follow it.
		w = tabClefWidth(p, sp)
	default:
		clef, _ := smufl.ClefGlyph(p.Clef)
		w = sp.ClefBefore + smufl.Advance(clef) + sp.ClefAfter
		w += e.keySignatureWidth(p.KeyAt(at))
	}
	if e.opt.HideTimeSignature || !meterAt(p, at) {
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

// meterAt says whether a system beginning at this bar writes the meter at
// its head.
//
// The clef and the key signature are repeated at the head of every system,
// because a player landing anywhere in the piece has to know them. The meter
// is not: it is written where the piece begins and wherever it changes, and
// nowhere else. Repeating it says "the meter changes here" to anybody
// reading, and it costs three staff spaces of every line it is printed on —
// which in a part is a bar every few systems.
func meterAt(p *model.Part, at int) bool {
	if at <= 0 {
		return true
	}
	return p.TimeAt(at) != p.TimeAt(at-1)
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
// voiceShift is the room a column needs for a notehead that has to step
// aside, and zero where none does.
//
// The rule is separateColumn's, asked at the one moment when the answer can
// still be acted on: a third apart or more and two voices clear each other,
// closer than that and one of them moves.
func (e *engraver) voiceShift(clef model.Clef, col slot) float64 {
	type reach struct{ lo, hi int }
	byVoice := map[int]*reach{}
	var voices []int
	widest := 0.0
	for _, v := range col.elems {
		pl := e.placedOf(clef, v.el, col.at)
		if pl.rest != nil {
			continue
		}
		if w := noteheadWidth(pl.value); w > widest {
			widest = w
		}
		r, ok := byVoice[v.voice]
		if !ok {
			byVoice[v.voice] = &reach{lo: pl.lo, hi: pl.hi}
			voices = append(voices, v.voice)
			continue
		}
		if pl.lo < r.lo {
			r.lo = pl.lo
		}
		if pl.hi > r.hi {
			r.hi = pl.hi
		}
	}
	if len(voices) < 2 {
		return 0
	}
	sort.Ints(voices)
	for i := 1; i < len(voices); i++ {
		if byVoice[voices[i-1]].lo-byVoice[voices[i]].hi <= 1 {
			return widest
		}
	}
	return 0
}

// a Ref for each of its elements.
func (e *engraver) measureBox(p *model.Part, m *model.Measure, index int) measureBox {
	sp := e.opt.Spacings
	box := measureBox{m: m}

	// Everything every voice holds, with the moment it starts at.
	type entry struct {
		at   model.Duration
		v    voiced
		lead float64
		room float64 // what a flag needs, if this one has one
	}
	var entries []entry
	empty := true
	for vi, voice := range m.Voices {
		if voice == nil || len(voice.Elements) == 0 {
			continue
		}
		empty = false
		flags := e.flagRoom(p, voice.Elements, index)
		at := model.Duration{Num: 0, Den: 1}
		for i, el := range voice.Elements {
			entries = append(entries, entry{
				at:   at,
				v:    voiced{el: el, voice: vi},
				lead: e.accidentalLead(el) + graceLead(el),
				room: flags[i],
			})
			at = at.Add(el.Duration())
		}
	}
	// Into columns, earliest first. Stable, so that within a column the
	// voices stay in the order the score lists them — voice one above
	// voice two, which is what decides their stems.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].at.Cmp(entries[j].at) < 0
	})
	for i := 0; i < len(entries); {
		j := i
		col := slot{at: entries[i].at}
		for ; j < len(entries) && entries[j].at.Cmp(col.at) == 0; j++ {
			en := entries[j]
			en.v.index = e.nextRef
			e.nextRef++
			col.elems = append(col.elems, en.v)
			col.lead = math.Max(col.lead, en.lead)
			col.width = math.Max(col.width, en.room)
		}
		// Two lines of music close enough to clash need room for the one
		// that steps aside. Booked here, before the column is placed,
		// because the step itself is taken when the noteheads are drawn and
		// by then there is nowhere to take it from: the lower voice stepped
		// left into whatever stood there, and at the start of a bar what
		// stands there is the barline. The Altes prints its bar 10 that way
		// — a chord of half notes written on top of the barline.
		col.lead += e.voiceShift(p.Clef, col)
		box.slots = append(box.slots, col)
		i = j
	}
	// How wide each column is: the time from it to the next one, which for
	// one voice is simply the note's own length and lays out exactly as it
	// always did. A half note over two quarters spans two columns and is
	// given the room of both.
	for i := range box.slots {
		col := &box.slots[i]
		span := model.Duration{Num: 0, Den: 1}
		if i+1 < len(box.slots) {
			span = box.slots[i+1].at.Sub(col.at)
		} else {
			// Nothing follows, so the longest thing starting here decides.
			for _, v := range col.elems {
				if d := v.el.Duration(); d.Cmp(span) > 0 {
					span = d
				}
			}
		}
		least := sp.MinNote
		if col.width > 0 { // a flag already claimed more than the minimum
			least = math.Max(least, col.width)
		}
		col.width = math.Max(e.spanWidth(span, col.elems), col.width)
		box.ideal += col.lead + col.width
		box.minimum += col.lead + least
	}
	if empty {
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
		if len(naturals) > 0 && len(accidentals) > 0 {
			w += sp.AfterKeyCancel
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

// afterFlag is the air left between a flag and whatever is written next.
//
// Little, because a flag is a thin curl and not a block: it has to be seen to
// clear the next note, not to stand apart from it.
const afterFlag = 0.2

// flagRoom is how wide each element of a measure has to be for its flag,
// indexed as the elements are. Zero for anything that carries no flag, which
// is most of a bar.
//
// A flag hangs off the right of the stem, and on an up-stem note the stem
// stands at the right edge of the head — so the note's ink reaches further
// right than its duration says, and a rule that only knows about durations
// sets the next notehead underneath the curl. Which notes those are is not
// known from one element either: an eighth in a run is beamed and has no
// flag at all, and a low one in a group of high ones has its stem turned
// down, where the flag hangs over the head it belongs to and asks for
// nothing. So the question is asked of the bar, the way the drawing asks it.
func (e *engraver) flagRoom(p *model.Part, elems []model.Element, index int) map[int]float64 {
	if len(elems) == 0 {
		return nil
	}
	items := make([]*placed, 0, len(elems))
	at := model.Duration{Num: 0, Den: 1}
	for _, el := range elems {
		items = append(items, e.placedOf(p.Clef, el, at))
		at = at.Add(el.Duration())
	}
	time := p.TimeAt(index)
	beamed := make(map[*placed]bool, len(items))
	for _, g := range beamGroups(items, time) {
		for _, q := range g {
			beamed[q] = true
		}
	}
	e.setStemDirections(items, time)

	var out map[int]float64
	for i, it := range items {
		if beamed[it] || it.stemDir < 0 {
			continue
		}
		g := smufl.FlagGlyph(it.value, true)
		if g == "" {
			continue
		}
		if out == nil {
			out = make(map[int]float64, 4)
		}
		// Measured from where drawSingleStem puts the flag, so that the
		// room reserved and the ink drawn are the same measurement.
		out[i] = noteheadWidth(it.value) - e.opt.Defaults.StemThickness/2 +
			smufl.Advance(g) + afterFlag
	}
	return out
}

// elementWidth is the proportional spacing rule that replaces the fixed
// per-duration table of the C program.
// spanWidth is how wide a column is: a power of the time it covers, plus
// the room the dots in it actually take.
//
// The time, not the note. With one voice they are the same thing — the gap
// to the next note is the length of this one — which is why a single-voice
// score lays out to the staff space exactly as it did before columns. With
// two, a half note spans the columns of the two quarters played against it
// and is given the room of both.
func (e *engraver) spanWidth(span model.Duration, els []voiced) float64 {
	sp := e.opt.Spacings
	if span.IsZero() {
		return sp.MinNote
	}
	quarter := model.Duration{Num: 1, Den: 4}
	w := sp.NoteBase * math.Pow(span.Float()/quarter.Float(), sp.NoteExponent)
	dots := 0
	for _, v := range els {
		if _, d, ok := v.el.Duration().Written(); ok && d > dots {
			dots = d
		}
	}
	if dots > 0 {
		// The room the dots actually take: each one is as wide as the glyph,
		// not as wide as the gap after it. Budgeting the gap alone left the
		// dots crowding whatever came next.
		w += sp.DotToNote + float64(dots)*dotStep(sp)
	}
	over := 0.0
	for _, v := range els {
		if o := chordOverhang(v.el); o > over {
			over = o
		}
	}
	w += over
	return math.Max(w, sp.MinNote)
}

// chordOverhang is how far past its own column a chord reaches.
//
// Two noteheads a second apart cannot be drawn through each other, so one of
// each pair is pushed across the stem — always to the right, which is where
// whatever comes next in the bar is standing. A column budgeted at one
// notehead wide has no room for that, and the next thing is printed on top
// of the pushed-out head: in the opening of the Altes, an eighth rest over
// the C of an A flat-C-D chord.
//
// Which heads move depends on the stem, and the stem is not decided until
// the music is drawn. Whether any move at all does not: it happens exactly
// when two of the notes are a diatonic second apart, and that is the same
// question whichever way the stem ends up pointing. So the room can be
// booked here, before any of that is settled.
func chordOverhang(el model.Element) float64 {
	ch, ok := el.(*model.Chord)
	if !ok || len(ch.Notes) < 2 {
		return 0
	}
	steps := make([]int, len(ch.Notes))
	for i, n := range ch.Notes {
		steps[i] = n.Pitch.Octave*7 + int(n.Pitch.Step)
	}
	sort.Ints(steps)
	for i := 1; i < len(steps); i++ {
		if steps[i]-steps[i-1] != 1 {
			continue
		}
		v, _, ok := ch.Duration().Written()
		if !ok {
			v = model.Quarter
		}
		return noteheadWidth(v)
	}
	return 0
}

func (e *engraver) elementWidth(el model.Element) float64 {
	sp := e.opt.Spacings
	d := el.Duration()
	if d.IsZero() {
		return sp.MinNote
	}
	quarter := model.Duration{Num: 1, Den: 4}
	w := sp.NoteBase * math.Pow(d.Float()/quarter.Float(), sp.NoteExponent)
	if _, dots, ok := d.Written(); ok && dots > 0 {
		// The room the dots actually take: each one is as wide as the glyph,
		// not as wide as the gap after it. Budgeting the gap alone left the
		// dots crowding whatever came next.
		w += sp.DotToNote + float64(dots)*dotStep(sp)
	}
	return math.Max(w, sp.MinNote)
}

// drawSystem draws one system: one staff per part, with measure start-x shared
// between them so the barlines line up into columns.
//
// carried is how much of prefix is blank held back for the ties and slurs
// coming in from the system above: it sits after the meter, before the music.
func (e *engraver) drawSystem(parts []*model.Part, boxes [][]measureBox,
	widths []float64, start, end int, prefix, carried float64, last bool) {

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
	// Kept on the engraver so that a measure can place its barline against
	// the column it closes, which is a decision about the system and not
	// about the staff drawing it.
	e.stretch = stretch

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
	reserved := chords + marks
	// The measurement wins outright, in both directions. It is usually less
	// — the count reserves a chord row for a system with one chord in it —
	// but not always: a slur arching over the first note of a line reaches
	// a little above what "there are notes here" allows for, and a system
	// whose recorded top cuts through its own slur is one the page break
	// slices.
	if measured, ok := e.room[start]; ok {
		reserved = measured.above
	}
	e.y += reserved
	top := e.y

	// Everything written over the system stacks above the chord symbols,
	// which are themselves above the staff. Drawing them all at the same
	// height is what put a rehearsal mark through a tempo direction.
	above := top - chords

	// The volta brackets belong to the system, not to a staff: they are drawn
	// once above the top one before the staves move the cursor down.
	// What is written over a system — the brackets of a repeat, the bar
	// numbers, the letters and the directions — belongs to the top staff and
	// has to clear its music, so it is held back with everything else that
	// goes outside a staff and placed when that staff is finished. They are
	// queued here, before any of it is drawn, because the order they are
	// placed in is their rank and not the order they were met in.
	e.drawMeasureNumbers(columns, start, end)
	e.drawVoltas(parts, columns, start, end)
	e.drawMarkers(parts, columns, start, end)
	_ = above

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
	// Words written under a staff need the staff below to move out of the way.
	// How much room each staff needs for what is written around it.
	//
	// Per staff, not one figure for the whole system. A chart has the chords
	// over the piano, the words under the voice and nothing around the drums;
	// widening every gap by the worst case pushes a pianist's two staves half
	// a page apart for the sake of a guitar diagram three staves away.
	gapAfter := func(i int) float64 {
		gap := e.staffGap(len(parts)) + roomBelow(parts[i])
		if i+1 < len(parts) {
			if above := e.partChordRoom(parts[i+1], start, end); above > 0 {
				gap += above
			}
			// And the music itself, with whatever is written under it. The
			// gap is a constant until something goes into it: a run on
			// ledger lines over the lower staff and a dynamic under the
			// upper one meet in the middle, and then nothing on the page
			// says which staff either belongs to.
			//
			// Bar by bar, and the worst bar sets the gap. Two staves are
			// straight lines, so they have to be far enough apart for the
			// worst place along them — but the worst place is one place.
			// Asking for the whole stretch at once took the deepest thing
			// under the upper staff, the highest thing over the lower one
			// and the room for the marks between them, each found wherever
			// in the piece it happened to be, and added them up as though
			// all three were at the same moment. In the continuous strip,
			// where the whole piece is one system, that is a hundred and
			// thirty bars of worst cases summed: the Altes opened with the
			// pianist's two staves a hand's width apart and nothing at all
			// in between. The deepest note under the right hand is at bar
			// 62 and the highest under the left at bar 83.
			for mi := start; mi < end && mi < len(parts[i].Measures); mi++ {
				_, below := e.staffReach(parts[i], mi, mi+1)
				below += marksBelow(parts[i], mi, mi+1)
				above, _ := e.staffReach(parts[i+1], mi, mi+1)
				if need := below + above + staffClearance; gap < need {
					gap = need
				}
			}
		}
		return gap
	}

	// Where each staff of this system was put, for the things drawn across
	// all of them: the brace, the bracket and the names.
	staffTops := make([]float64, 0, len(parts))

	// The lowest point anything in this system reached, filled in as each
	// staff is finished.
	lowest := 0.0

	// How much of the prefix belongs to the names and the bracket rather
	// than to the clefs.
	lead := e.labelWidth(parts, start == 0)
	if len(e.score.Groups) > 0 {
		lead += groupGap + bracketThickness
	}

	for pi, p := range parts {
		e.clef = p.Clef
		// How this staff is ruled: five lines of pitches, or one line per
		// string with the frets written on them.
		e.lines, e.lineGap = partLines(p), partGap(p)
		e.tab = p.Tab
		e.tabGaps = e.tabGaps[:0]
		// A hairpin or an octave line cut off at the end of the last system
		// starts again here, at this system's left edge.
		e.resumeSpans(e.staffNumber(pi), sp.Margin+prefix-carried)

		staffTop := e.y
		staffMark := e.marked()
		e.staffMark, e.staffTop = staffMark, staffTop
		e.staffX0, e.staffX1 = sp.Margin, right
		staffTops = append(staffTops, e.y)
		// Each staff's clef starts far enough right that its own key and
		// meter end where everybody else's do, so a staff in C major and one
		// in five sharps begin their music in the same column.
		//
		// prefix already counts the names and the bracket. Adding lead here
		// as well pushed every staff's clef, key and meter one name-width too
		// far right — past the column the music starts in, so the first note
		// was printed over them.
		px := sp.Margin + prefix - carried - e.prefixWidth(p, start) + sp.ClefBefore
		// Each of the three is recorded as it is drawn, so that clicking one
		// changes the thing it is: the clef, the key or the meter.
		header := func(kind string, x0, x1 float64) {
			if x1 <= x0 {
				return
			}
			e.lay.Headers = append(e.lay.Headers, HeaderPos{
				Part: pi, Measure: start, Kind: kind,
				X0: x0, X1: x1, Top: e.y - 1, Bottom: e.y + e.staffH() + 1,
			})
		}
		was := px
		if e.opt.HideClef {
			// Nothing drawn at the head: the picture is of what is above
			// the staff, not of the staff.
		} else if p.Tab != nil {
			// A tab staff has no clef and no key signature: a fret number
			// says where to put a finger and needs neither. What stands at
			// the head of it is the word TAB.
			px = e.tabClef(p, px)
			header("clef", was-sp.ClefBefore/2, px-sp.ClefAfter/2)
		} else {
			px = e.drawClef(p, px)
			header("clef", was-sp.ClefBefore/2, px-sp.ClefAfter/2)
			was = px
			px = e.drawKeySignature(p, p.KeyAt(start), px)
			header("key", was, px-sp.AfterKeySignature/2)
		}
		if meterAt(p, start) {
			was = px
			px = e.drawTimeSignature(p, p.TimeAt(start), px)
			header("time", was, px-sp.AfterTimeSignature/2)
		}

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
			e.drawMeasure(p, b, columns[mi-start], columns[mi-start+1], s)
		}

		// The staff lines are drawn last, reaching exactly as far as the
		// music. Nothing can hang off the end of them. They start after the
		// names and the bracket: a staff ruled through its own name is how
		// a name looks crossed out.
		e.staffLines(sp.Margin+lead, right-sp.Margin-lead)
		// To the end of the staff, which is where right is. staffLines takes
		// a width and this takes a place, and passing it the width left every
		// hairpin, octave line and tie stopping four spaces short of the end
		// of the line it was supposed to run to.
		e.breakSpans(e.staffNumber(pi), right)

		// The staff is finished, so the marks that go outside it now know
		// what they have to clear. They are placed outwards from the staff
		// in the order notation puts them in, each clearing the last.
		sky := e.outline(staffMark, sp.Margin, right, staffTop)
		// The chord symbols first, because they are a row above the music
		// and everything else is a row above them.
		e.placeChords(sky, sp.Margin, right)
		e.placeWaiting(sky)
		// How far down this staff's music and marks actually went. The room
		// reserved for them was counted from the model before anything was
		// drawn, and a count is an estimate: a dynamic under a slur that
		// dips low ends up lower than any estimate of "there is a dynamic
		// here" can know. This is the measurement, and the system's own
		// bottom is taken from it.
		if low := sky.reach(sp.Margin, right, true); low > lowest {
			lowest = low
		}

		if pi < len(parts)-1 {
			e.y += partH(p) + gapAfter(pi)
		}
	}

	// The names and the brace reach across every staff of the system, so they
	// are drawn once the staves' positions are all known.
	e.drawGroups(parts, sp.Margin+lead, staffTops, start == 0)
	// Down the gaps between the staves, at exactly the x the staves put
	// their own barlines at. A column is where the next bar begins; the
	// barline stands a little before it, by the room its style needs. The
	// joining line was drawn on the column itself, so every grand staff in
	// every score had a step in its barlines — the line between the staves
	// standing to the right of the lines on them.
	for i := range columns {
		x := columns[i]
		if i == 0 {
			// The first column is where the music starts, which is after
			// the clef and the meter — there is no barline there, and a
			// line joining the staves in the middle of a bar's opening is
			// simply wrong. What belongs at the head of a system is the
			// line down its left edge, beside the brace, which is where
			// the staves actually begin.
			x = sp.Margin + lead
		} else {
			if mi := start + i - 1; mi >= 0 && mi < len(parts[0].Measures) {
				_, right := e.barlineRoom(parts[0].Measures[mi].RightBarline)
				x = e.barlineAt(columns[i], right)
			}
		}
		e.drawGroupBarlines(parts, x, staffTops, i == 0)
	}

	// Back to the top of the system, then down past all of it.
	e.y = top
	for i := 0; i < len(parts)-1; i++ {
		e.y += partH(parts[i]) + gapAfter(i)
	}
	// Past the last staff, and past whatever is written under it. The marks
	// below a staff are placed against what is already there, so they end up
	// as far down as they have to go — and if nothing has reserved that room
	// they go on down into the system underneath, where a crescendo that
	// began four bars ago arrives over somebody else's first bar.
	foot := parts[len(parts)-1]
	_, footLow := e.staffReach(foot, start, end)
	e.y += partH(foot) + math.Max(roomBelow(foot),
		footLow+marksBelow(foot, start, end))
	// And no higher than what was drawn, whatever the estimate said.
	if lowest > e.y {
		e.y = lowest + 0.3
	}
	// Nor above where the ink turned out to go. The outline a mark reserves
	// is the box it asked for, and a slur drawn through that box dips below
	// it: measured, the system ends where the last stroke of it does.
	if measured, ok := e.room[start]; ok {
		if want := top + measured.below + 0.3; want > e.y {
			e.y = want
		}
	}
	e.lay.Systems = append(e.lay.Systems, SystemPos{
		// The top reaches above the staff for everything written there: the
		// chord symbols and their diagrams, and above those the brackets of
		// a repeat, the octave signs and the rehearsal marks. A system whose
		// recorded top stopped at the staff had its chord diagrams drawn on
		// the page before it.
		Top:    top - reserved,
		Staff:  top,
		Bottom: e.y, First: start, Last: end - 1,
	})
	e.y += sp.SystemGap

	if r := right + sp.Margin; r > e.lay.Width {
		e.lay.Width = r
	}
}

func (e *engraver) staffLines(x, width float64) {
	t := e.opt.Defaults.StaffLineThickness
	n, gap := e.lines, e.lineGap
	if n < 2 {
		n, gap = staffLines, 1
	}
	for i := 0; i < n; i++ {
		y := e.y + float64(i)*gap
		// A line with fret numbers on it is ruled in pieces, around them: a
		// line through a figure is a line through it, and tab is read as
		// numbers before it is read as lines.
		at := x
		for _, g := range e.tabGaps {
			if math.Abs(g.y-y) > 0.01 || g.x1 <= at || g.x0 >= x+width {
				continue
			}
			if g.x0 > at {
				e.lay.Lines = append(e.lay.Lines, Line{
					X1: at, Y1: y, X2: g.x0, Y2: y, Thickness: t,
				})
			}
			if g.x1 > at {
				at = g.x1
			}
		}
		if at < x+width {
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: at, Y1: y, X2: x + width, Y2: y, Thickness: t,
			})
		}
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
	if k.Fifths == 0 || e.opt.HideKeySignature {
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
	top, bottom := e.y, e.y+e.staffH()

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
