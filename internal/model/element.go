/*
 * element.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import (
	"sort"
	"strconv"
)

// Element is anything that takes up time in a voice.
//
// Where the 0.0.9 Object_t was one struct holding every kind of mark together
// with six screen coordinates, elements here are distinct types and carry no
// geometry whatsoever. Positions belong to the engraver's output, are measured
// in staff spaces rather than pixels, and are thrown away and recomputed
// whenever the music changes.
type Element interface {
	// Duration is how much time the element occupies in its voice.
	Duration() Duration
}

// Accidental is an accidental as written on the page.
//
// It is independent of Pitch.Alter, which is what sounds. A note can sound
// sharp because of the key signature with no accidental printed, and a natural
// is printed precisely when nothing is altered.
type Accidental int

// The accidentals that can be written on a note.
const (
	NoAccidental Accidental = iota
	Natural
	Sharp
	Flat
	DoubleSharp
	DoubleFlat
)

var accidentalNames = [...]string{"none", "natural", "sharp", "flat", "double sharp", "double flat"}

func (a Accidental) String() string {
	if a < NoAccidental || int(a) >= len(accidentalNames) {
		return "unknown"
	}
	return accidentalNames[a]
}

// Semitones returns the alteration the accidental implies.
func (a Accidental) Semitones() int {
	switch a {
	case Sharp:
		return 1
	case Flat:
		return -1
	case DoubleSharp:
		return 2
	case DoubleFlat:
		return -2
	}
	return 0
}

// Articulation is a mark attached to a single note or chord.
type Articulation int

// The articulations carried over from 0.0.9's articulation code, plus the bow
// marks the ABC samples use.
const (
	Staccato Articulation = iota
	Staccatissimo
	Accent
	Tenuto
	Marcato
	MarcatoDown
	Fermata
	UpBow
	DownBow

	// The ornaments written on a note rather than over a stretch of music.
	//
	// A mordent says to drop to the note below and come back, a short
	// trill — the upper mordent, the pralltriller — to go up and come
	// back, and a turn to go round the note above, the note itself, the
	// note below and back. They are written where an articulation is
	// written and are placed with them, which is why they are here rather
	// than in a kind of their own: the thing a page has to get right about
	// them is that they stack clear of the staccato dot and the accent.
	//
	// The trill is not among them. It can last — "tr" and a wavy line to
	// where it stops — so it is a span and lives in Marks.Trill.
	Mordent
	ShortTrill
	Turn
	TurnInverted
)

var articulationNames = [...]string{
	"staccato", "staccatissimo", "accent", "tenuto", "marcato",
	"marcato down", "fermata", "up-bow", "down-bow",
	"mordent", "short trill", "turn", "inverted turn",
}

// Ornament says this mark is one of the ornaments: a direction to play
// notes that are not written, rather than a way of playing the one that is.
func (a Articulation) Ornament() bool {
	return a >= Mordent && a <= TurnInverted
}

func (a Articulation) String() string {
	if a < Staccato || int(a) >= len(articulationNames) {
		return "unknown"
	}
	return articulationNames[a]
}

// Beam says how a note joins the beam over it.
type Beam int

// How a note is beamed. Auto leaves it to the engraver, which groups by beat.
const (
	BeamAuto Beam = iota
	// BeamStart opens a beam at this note, BeamContinue carries it on and
	// BeamStop closes it. Together they spell out a group by hand.
	BeamStart
	BeamContinue
	BeamStop
	// BeamNone keeps the note off any beam: it gets a flag of its own.
	BeamNone
)

var beamNames = [...]string{"auto", "start", "continue", "stop", "none"}

func (b Beam) String() string {
	if b < BeamAuto || int(b) >= len(beamNames) {
		return "auto"
	}
	return beamNames[b]
}

// BeamFromName is the inverse, for files and query strings.
func BeamFromName(s string) (Beam, bool) {
	for i, n := range beamNames {
		if n == s {
			return Beam(i), true
		}
	}
	return BeamAuto, false
}

// Placement says which side of the staff something sits on. It is a musical
// decision, not a coordinate: the engraver turns it into a position.
type Placement int

// Where a mark is placed relative to the staff.
const (
	PlacementAuto Placement = iota
	PlacementAbove
	PlacementBelow
)

var placementNames = [...]string{"auto", "above", "below"}

func (p Placement) String() string {
	if p < PlacementAuto || int(p) >= len(placementNames) {
		return "unknown"
	}
	return placementNames[p]
}

// SymbolMark attaches a drawable symbol to an element by name.
//
// The name is resolved against the symbol registry at engraving time, so it
// may refer to a glyph in the music font or to something the user drew
// themselves. Notation has a long tail of marks that no font carries, and a
// score must be able to use one without the model knowing anything about how
// it looks. Nothing about the artwork is stored here — only which symbol, and
// which side of the staff it belongs on.
type SymbolMark struct {
	Name      string
	Placement Placement

	// OffsetX and OffsetY move the symbol from where the engraver put it, in
	// staff spaces.
	//
	// This is the one place geometry is allowed into the model, and it is
	// deliberate: a nudge is not derived layout, it is a decision somebody
	// made and wants kept. Staff spaces rather than pixels, so the nudge
	// survives a change of size, and saved with the score because losing it
	// would lose the decision.
	OffsetX, OffsetY float64
}

// Tuplet records that an element is part of a group played in less (or more)
// time than it is written: Actual notes in the time of Normal ones, so a
// triplet is {3, 2}.
//
// It has to be stored because the duration alone cannot carry it. A triplet
// eighth lasts a twelfth of a whole note, and no notehead spells a twelfth —
// Duration.Written reports exactly that. The written value is recovered by
// scaling the sounding duration back up by Actual/Normal.
type Tuplet struct {
	// Hidden leaves the figure and the bracket off the page, though the
	// notes still take their tuplet time.
	//
	// A tuplet is usually written to be read: three in the time of two,
	// with a 3 over them. Sometimes it is only arithmetic. A measured
	// tremolo — a dotted quarter with strokes through its stem, meaning
	// "repeat this note at sixteenths" — is six sixteenths in the time of
	// four, and the strokes are how that is written down. The tuplet is
	// there in the file because the arithmetic is there; it is not there to
	// be drawn, and MuseScore says so. Drawn anyway, every tremolo in
	// Beethoven's ninth carried a bracketed 6 over it that is in no printed
	// score of the piece.
	Hidden bool

	Actual, Normal int
	// Start and Stop bracket the group, for the number and bracket drawn
	// over it.
	Start, Stop bool
}

// Ratio reports whether the tuplet is set and usable.
func (t *Tuplet) Ratio() (num, den int, ok bool) {
	if t == nil || t.Actual <= 0 || t.Normal <= 0 {
		return 1, 1, false
	}
	return t.Actual, t.Normal, true
}

// Stem says which way a stem points.
//
// gscore 0.0.9 carried this on every object as O_STEMUP, O_STEMDOWN and
// O_AUTOSTEM, and realize_area.c:610 let them override the rule it had just
// worked out. The override matters: the automatic choice is only a default,
// and it is wrong whenever two voices share a staff, or where a phrase reads
// better one way.
type Stem int

// Which way a stem points, or that the engraver should decide.
const (
	StemAuto Stem = iota
	StemUp
	StemDown
)

var stemNames = [...]string{"auto", "up", "down"}

func (s Stem) String() string {
	if s < StemAuto || int(s) >= len(stemNames) {
		return "unknown"
	}
	return stemNames[s]
}

// Dynamic is a volume marking.
//
// The 0.0.9 toolbar had nine of these, from pianissississimo to
// fortississimo, and an Insert menu to match; they were drawn but never
// modelled as anything a player could read. Here they are part of the music,
// so they survive export and can be sounded.
type Dynamic int

// The dynamic markings, quietest first. DynamicNone means none is written.
const (
	DynamicNone Dynamic = iota
	Niente
	PPPP
	PPP
	PP
	Piano
	MP
	MF
	Forte
	FF
	FFF
	FFFF
	SForzando
	SForzato
	FortePiano
	Rinforzando
)

var dynamicNames = [...]string{
	"", "niente", "pppp", "ppp", "pp", "p", "mp", "mf",
	"f", "ff", "fff", "ffff", "sfz", "sf", "fp", "rfz",
}

func (d Dynamic) String() string {
	if d < DynamicNone || int(d) >= len(dynamicNames) {
		return "unknown"
	}
	return dynamicNames[d]
}

// DynamicFromName reads a dynamic written the way musicians write it.
func DynamicFromName(s string) (Dynamic, bool) {
	for i, n := range dynamicNames {
		if n != "" && n == s {
			return Dynamic(i), true
		}
	}
	return DynamicNone, false
}

// Velocity is how loud the dynamic sounds, as a MIDI velocity. The ladder
// follows NoteWorthy's own default table, which is as good a reading of these
// words as any.
func (d Dynamic) Velocity() int {
	switch d {
	case Niente:
		return 1
	case PPPP:
		return 8
	case PPP:
		return 16
	case PP:
		return 33
	case Piano:
		return 49
	case MP:
		return 64
	case MF:
		return 80
	case Forte:
		return 96
	case FF:
		return 112
	case FFF, FFFF:
		return 127
	case SForzando, SForzato, Rinforzando:
		return 112
	case FortePiano:
		return 96
	}
	return 0
}

// Marks are the attachments a note or a chord can carry. They are shared
// because an articulation or a slur applies to a chord as a whole, not to one
// of its noteheads.
type Marks struct {
	Articulations []Articulation
	// ArticulationPlace is where each of those is written, where somebody
	// has said: index for index with Articulations, and PlacementAuto —
	// which is also what a shorter list means — for the ones left to the
	// engraver.
	//
	// Kept beside them rather than in them because an articulation is a
	// number everywhere in this program and in every file it is read from,
	// and a mark that is a number is a mark that can be compared, counted
	// and looked up. Where it goes is a decision about the page and not
	// about the music, which is why it lives apart from it.
	ArticulationPlace []Placement

	// Techniques are how a fretted instrument is played at this note: a
	// hammer-on, a slide, a bend, a palm mute. See Technique.
	Techniques []Technique

	// Cross draws this element on the staff above or below the one it
	// belongs to, without changing what it is or what it sounds. See
	// CrossStaff — it is what piano writing needs for a figure that runs
	// past what one hand can reach.
	Cross CrossStaff `json:",omitempty"`

	// Symbols are user-supplied marks, named in the symbol registry. The
	// built-in Articulations above cover what the C program knew; this is the
	// escape hatch for everything else.
	Symbols []SymbolMark

	// TieStart and TieStop join this element to the one of the same pitch next
	// to it. Ties cross barlines, so they are held here rather than scoped to
	// a measure.
	TieStart, TieStop bool

	// SlurStart and SlurStop bracket a phrase, which unlike a tie may span any
	// pitches.
	SlurStart, SlurStop bool

	// Tuplet is set when the element belongs to a tuplet group.
	Tuplet *Tuplet

	// Tremolo repeats the note for its written length.
	Tremolo *Tremolo

	// Clef, when set, changes the clef from this element on.
	//
	// On the element and not on the measure, because that is where a change
	// of clef actually happens: a pianist's right hand plays a few beats,
	// drops into the bass for a phrase and comes back, all inside one bar,
	// and the sign is printed in the middle of the bar where the hand moves.
	// Altes writes four of them in two bars that way.
	Clef *Clef

	// Stem overrides the direction the engraver would otherwise choose.
	Stem Stem

	// Beam overrides how this note is beamed to the ones around it. The
	// engraver groups by beat, which is right nearly always and wrong
	// whenever the music is phrased against the beat — which is a decision
	// only the person writing it can make.
	Beam Beam

	// Dynamic is a volume marking written at this point in the music.
	Dynamic Dynamic

	// Hairpin is a crescendo or diminuendo opening or closing here. Unlike a
	// Dynamic, which is an instant, this covers the stretch between the note
	// it starts on and the one it stops on.
	Hairpin *Hairpin

	// Octave is an 8va or 8vb line opening or closing here. The notes under
	// it are written where they are drawn and sound where the line says, so
	// the model keeps the written pitch and playback does the shifting.
	Octave *OctaveShift

	// Trill is a trill opening or closing here.
	//
	// It is a spanner and not an articulation because a trill can last: "tr"
	// over one note is a trill on that note, and "tr" with a wavy line
	// running four bars is a trill held for four bars, and the difference is
	// the whole instruction. A trill on a single note carries both ends.
	Trill *Trill

	// Lyrics are the syllables sung on this note, one per verse.
	Lyrics []Lyric

	// Chord is the chord symbol written over this note — "Em6", "B7", "N.C."
	// — with, optionally, the fingering diagram that goes above it.
	//
	// It hangs off a note rather than off a measure because that is where it
	// is read: a bar can change chord twice and the player needs to know
	// where the second one starts, which is a note and not a bar.
	Chord *ChordSymbol

	// Texts are words written near this note: "rit.", "solo", a fingering, a
	// note to the player. They hang off a note rather than off a point on the
	// page, so that they stay where they belong when the music is re-engraved
	// at another size — which is the same reason symbols do.
	Texts []TextMark

	// Grace holds the little notes crushed in before this one.
	//
	// They hang off the note they lean on rather than standing beside it as
	// elements of their own, because they take no time out of the measure: a
	// bar of four quarters with a grace note before the third beat is still a
	// bar of four quarters. Putting them in the voice would make every bar
	// that has one overflow.
	Grace *GraceGroup

	// Arpeggio rolls the chord: a wavy line in front of it, played bottom
	// to top unless the arrow says otherwise.
	Arpeggio *Arpeggio

	// Break is the breath or pause taken after this note: a comma, a tick,
	// or the two strokes of a caesura.
	Break Breath

	// Fingering is which fingers play this note, printed as the figures a
	// player reads: 1 to 5 for a keyboard, 0 to 4 for a string.
	//
	// Written as the text it is rather than as numbers, because a fingering
	// is not always a number: "1-2" is a substitution on one key, "4/5" a
	// choice, and a dash between two of them a slide. Numbers would make
	// the common case tidy and the real cases impossible.
	//
	// One per notehead of a chord, from the bottom up, which is how a
	// pianist reads a chord's fingering.
	Fingering []string

	// Glissando slides from this note to the next, and Pedal is the
	// sustaining pedal held across a stretch of music. Both are spans: see
	// Glissando and PedalMark.
	Glissando *Glissando
	Pedal     *PedalMark

	// Small draws the element at cue size: a note that is printed but is
	// not yours to play.
	//
	// What it is for is the cue in a part — a few bars of somebody else's
	// line, printed small during a long rest so you can hear your entry
	// coming. It says nothing about the music: a cue sounds nothing, lasts
	// nothing a player counts, and is there to be read and not played,
	// which is the whole of what the smaller size means on a page.
	//
	// A mark rather than a kind of note, because that is what it is — the
	// same note, written smaller. It is set on the copy of the score a part
	// is drawn from and never on the score itself, where somebody else's
	// line already stands on its own staff at full size.
	Small bool
}

// Arpeggio is the wavy line in front of a chord that says to roll it.
//
// Upward unless Down says otherwise, which is why the field is named for the
// exception: a plain wavy line means bottom to top, and an arrowhead at its
// foot is what says to come down through the chord instead.
type Arpeggio struct {
	Down bool
}

// Breath is the pause taken after a note.
type Breath int

const (
	NoBreath Breath = iota
	// BreathComma is the ordinary breath mark, a comma over the top line.
	BreathComma
	// BreathTick is the stroke used where a comma would be lost among
	// other marks, and in string music.
	BreathTick
	// Caesura is the two strokes — "railroad tracks" — that stop the music
	// altogether rather than asking for a breath.
	Caesura
	// CaesuraCurved is the same pause drawn with curved strokes, which some
	// houses prefer.
	CaesuraCurved
)

var breathNames = [...]string{"none", "comma", "tick", "caesura", "curved caesura"}

// BreathFromName reads back what String wrote, for a file.
func BreathFromName(s string) (Breath, bool) {
	for i, n := range breathNames {
		if n == s {
			return Breath(i), true
		}
	}
	return NoBreath, false
}

func (b Breath) String() string {
	if int(b) < len(breathNames) {
		return breathNames[b]
	}
	return "none"
}

// Glissando is a slide from one note to another.
//
// Start and Stop mark its two ends, the way a slur's do. Wavy is the
// difference between a glissando and a portamento as most houses draw them —
// a wavy line for a slide that sounds the notes between, a straight one for
// one that does not — and Text is what is written over it, usually "gliss."
// and sometimes nothing at all.
type Glissando struct {
	Start, Stop bool
	Wavy        bool
	Text        string
}

// PedalMark is the sustaining pedal, held across a stretch of music.
//
// Start and Stop are the two ends and Change is a release and re-press at
// one note — the notch in the middle of a pedal line, which is how a pianist
// is told to change the pedal on a beat without lifting the phrase.
//
// Text draws the old form, "Ped." under the first note and a star under the
// last, instead of the line. The books call that method somewhat obsolete
// and so does everybody who has had to read a page of it, but it is what a
// great deal of printed music has and a score that cannot show it cannot
// show that music.
//
// Half says the pedal is only half down, written as "1/2" beside the mark.
type PedalMark struct {
	Start, Stop bool
	Change      bool
	Text        bool
	Half        bool
}

// GraceGroup is the small notes played before the note they belong to.
//
// Slashed is the difference between an acciaccatura and an appoggiatura, and
// it is not decoration: a stroke through the stem means "as quickly as
// possible, stolen from before the beat", and no stroke means the grace notes
// take their written value out of the note they precede. A player does two
// different things with the two.
type GraceGroup struct {
	Notes   []*Note
	Slashed bool
}

// Duration is how long the whole group is written as, which is what decides
// how much room it needs rather than how long it sounds.
func (g *GraceGroup) Duration() Duration {
	if g == nil {
		return Duration{}
	}
	total := Duration{Num: 0, Den: 1}
	for _, n := range g.Notes {
		total = total.Add(n.Dur)
	}
	return total
}

// ChordSymbol is the name of a chord, written above the staff.
//
// It is kept as the pieces a chord name is made of rather than as one string,
// because the pieces are set differently: the root is full size, the quality
// is smaller and raised, and the bass note after a slash is full size again.
// "Cmaj7/G" set as plain text in one size is the mark of a program that did
// not look at a lead sheet before writing its chord symbols.
type ChordSymbol struct {
	// Root is the letter and any accidental: "C", "Eb", "F#".
	Root string
	// Quality is everything after it: "m7", "maj9", "sus4", "13(b9)".
	Quality string
	// Bass is what follows a slash, for an inversion: the G of "C/G".
	Bass string
	// Text replaces all of it when the symbol is not a chord at all — "N.C."
	// for no chord, or a direction somebody wants set like one.
	Text string
	// Frame is the fingering diagram drawn above the symbol, for guitar.
	Frame *ChordFrame
}

// String spells the symbol out, for anything that wants it as one piece.
func (c *ChordSymbol) String() string {
	if c == nil {
		return ""
	}
	if c.Text != "" {
		return c.Text
	}
	s := c.Root + c.Quality
	if c.Bass != "" {
		s += "/" + c.Bass
	}
	return s
}

// ChordFrame is a guitar chord diagram: which fret each string is stopped at.
//
// The strings run low to high, as they are drawn left to right. A fret of 0 is
// an open string, -1 is one that is not played, and anything else is the fret
// counted from FirstFret.
type ChordFrame struct {
	Frets []int
	// Fingers, when given, are the left-hand fingers for each string, 0 for
	// none. A beginner needs these more than they need the dots.
	Fingers []int
	// FirstFret is the fret the diagram starts at, 1 for the nut. A chord up
	// the neck is drawn with the fret number beside it rather than with
	// twelve empty frets above it.
	FirstFret int
	// Barre, when more than zero, is the fret a finger lies across, and
	// BarreFrom and BarreTo the strings it covers.
	Barre              int
	BarreFrom, BarreTo int
}

// Spell writes the fingering out the way it is typed in: one character a
// string, lowest first, a fret number or x for a string that is not played.
//
// Frets past nine need spaces between them, because "10" is two characters
// and a diagram is read a string at a time.
func (f *ChordFrame) Spell() string {
	if f == nil || len(f.Frets) == 0 {
		return ""
	}
	wide := false
	for _, n := range f.Frets {
		if n > 9 {
			wide = true
		}
	}
	out := ""
	for i, n := range f.Frets {
		if wide && i > 0 {
			out += " "
		}
		if n < 0 {
			out += "x"
			continue
		}
		out += strconv.Itoa(n)
	}
	return out
}

// TextMark is a piece of writing attached to a note.
type TextMark struct {
	Text string

	// Style is what kind of writing this is, which decides how it is set
	// and where it goes: see TextStyle. The zero value is an expression
	// marking, which is what the commonest words on a page are.
	Style TextStyle `json:",omitempty"`

	// Placement overrides where the style would put it. PlacementAuto —
	// the zero value — leaves it to the style, which is what nearly every
	// mark wants: words of one kind all sitting on the same side of the
	// staff is most of what makes a page readable.
	Placement Placement

	// Offsets are in staff spaces from where the engraver would put it.
	OffsetX, OffsetY float64

	// Italic and Bold are set ON TOP of the style rather than instead of
	// it. They can add a slant or a weight and cannot take one away:
	// taking one away would need a third state for "not set", and a mark
	// that says plainly what it wants is worth more than one that can
	// countermand its own style in both directions.
	Italic, Bold bool

	// Runs is the formatting of the words, where somebody has set them in
	// anything: the same text broken into the runs it is set in.
	//
	// The words stay in Text, which is what every importer writes and
	// every exporter reads, and this is extra — the same arrangement the
	// title and the credits use, and for the same reason. A mark with no
	// runs is plain, which is nearly every mark of nearly every score.
	Runs []Span
}

// Spans is the words as they are drawn: their runs where those spell them,
// and the words in one plain run where they do not. See model.Spans.
func (t TextMark) Spans() []Span { return Spans(t.Text, t.Runs) }

// Look is how this mark is set: its style, with whatever the mark itself
// insists on.
func (t TextMark) Look() TextLook {
	look := t.Style.Look()
	look.Italic = look.Italic || t.Italic
	look.Bold = look.Bold || t.Bold
	if t.Placement != PlacementAuto {
		look.Place = t.Placement
	}
	return look
}

// Syllabic says where a syllable falls in its word, which is what decides
// whether a hyphen is drawn after it.
type Syllabic int

// A syllable is a whole word, or the start, middle or end of one.
const (
	SyllableSingle Syllabic = iota
	SyllableBegin
	SyllableMiddle
	SyllableEnd
)

// Lyric is one syllable under one note.
//
// Verses are numbered from 1 and stack under the staff, so a hymn with four
// verses puts four lines of words under the same music.
type Lyric struct {
	Text     string
	Verse    int
	Syllabic Syllabic
	// Extend draws the long line that carries a held syllable across the
	// notes it is sung over.
	Extend bool
}

// NeedsHyphen reports whether a hyphen is drawn after this syllable, joining
// it to the next one of the same word.
func (l Lyric) NeedsHyphen() bool {
	return l.Syllabic == SyllableBegin || l.Syllabic == SyllableMiddle
}

// HairpinKind is which way a hairpin opens.
type HairpinKind int

// The two hairpins. A crescendo opens to the right, a diminuendo closes.
const (
	Crescendo HairpinKind = iota
	Diminuendo
)

func (h HairpinKind) String() string {
	if h == Diminuendo {
		return "diminuendo"
	}
	return "crescendo"
}

// Hairpin is a gradual change of volume drawn as a wedge under the staff.
type Hairpin struct {
	Kind HairpinKind
	// Start opens the wedge at this element, Stop closes it at this one.
	Start, Stop bool
	// Alone says the wedge begins and ends on this element, drawn across
	// the note's own width.
	//
	// It is a third thing and not the two others together, because of the
	// one place in notation where they are not the same: a note where one
	// hairpin ends and another begins — a diminuendo running into a
	// crescendo, which is written point to point in every edition. That
	// note stops what is open and starts what is new, so Start and Stop
	// are both set and neither of them is about a wedge on this note
	// alone. Without a third thing to say it, a swell on one long note and
	// the meeting of two hairpins are the same mark, and one of them has
	// to be drawn wrongly.
	Alone bool
}

// Trill is an ornament held over a stretch of music: "tr" at the note it
// begins on, and a wavy line to where it stops.
//
// Start and Stop bracket it, as a hairpin's do. Both on one element is a
// trill on that note alone, which is how the ornament is written when there
// is nothing to extend it over.
type Trill struct {
	Start, Stop bool
}

// Tremolo is a note repeated for its whole written length, written as
// slashes through its stem rather than as the notes it stands for.
//
// A bar of repeated thirty-second notes written out is a bar of beams that
// says nothing but "shake"; the slashes say the same thing and leave the
// pitch and the length of the note readable. The number of them is the
// number of beams the repeated note would have had, which is why it is
// counted rather than named: one slash is eighths, two sixteenths, three
// thirty-seconds, and so on whatever the note they are drawn through.
//
// Two notes alternating is the other kind, and Start and Stop say so. It is
// the same mark counted the same way — the strokes are still the beams the
// repeated note would have had — but it is drawn between the two stems
// rather than through one, and it is heard as the two notes alternating
// rather than as one note repeated.
type Tremolo struct {
	// Strokes is how many slashes, from one to five.
	Strokes int

	// Buzz is the unmeasured roll: as fast as the player can, with no
	// division implied. It is written as a z through the stem, and a snare
	// drum's press roll is the usual reason for it.
	Buzz bool

	// Start and Stop make this one end of a two-note tremolo: the first
	// note of the pair and the second. Neither set is the ordinary tremolo
	// on a single note.
	//
	// The two notes keep their own written lengths and those lengths still
	// add up the way the bar says — two half notes of a two-note tremolo
	// fill four beats, exactly as two half notes do. What changes is only
	// what is heard in that time.
	Start, Stop bool
}

// Paired says this tremolo is one end of a two-note tremolo rather than the
// slashes on a single note.
func (t *Tremolo) Paired() bool {
	return t != nil && (t.Start || t.Stop)
}

// Beams is how many beams the repeated note has, which is what the strokes
// count. A buzz roll has no measure, so it answers with the fastest thing
// anybody writes and lets the player decide.
func (t *Tremolo) Beams() int {
	if t == nil {
		return 0
	}
	if t.Buzz {
		return 4
	}
	if t.Strokes < 1 {
		return 1
	}
	if t.Strokes > 5 {
		return 5
	}
	return t.Strokes
}

// Repeat is the duration of the note the slashes stand for: a whole note
// divided by two for each beam, so one slash is an eighth and three is a
// thirty-second.
func (t *Tremolo) Repeat() Duration {
	return Duration{Num: 1, Den: 1 << (t.Beams() + 2)}
}

// OctaveShift is an 8va, 8vb, 15ma or 15mb line.
//
// Amount is in octaves and signed: 1 for 8va above the staff, -1 for 8vb
// below, 2 and -2 for the fifteenths.
type OctaveShift struct {
	Amount      int
	Start, Stop bool
	// SoundsAsWritten says the notes under the line are already at the
	// pitch they are played at, so the bracket is drawn and nothing is
	// transposed.
	//
	// A written ottava means "play these an octave up". This one means
	// "these are already up there, and the bracket says where they are".
	// NoteWorthy has no ottava of its own, so a staff author draws one by
	// typing "8va" and a row of dashes over notes that are at pitch
	// already — the staff that sounds carries the same pitches as the
	// staff that is printed, which is how that is known. Taken for a
	// written ottava, half of Danse Macabre would play an octave above
	// what anybody wrote.
	SoundsAsWritten bool
}

// Label spells the line the way it is printed over the music.
func (o *OctaveShift) Label() string {
	switch {
	case o == nil:
		return ""
	case o.Amount >= 2:
		return "15ma"
	case o.Amount == 1:
		return "8va"
	case o.Amount == -1:
		return "8vb"
	case o.Amount <= -2:
		return "15mb"
	}
	return ""
}

// WrittenDuration is the duration the notehead shows, which differs from the
// sounding duration inside a tuplet.
func (m *Marks) WrittenDuration(sounding Duration) Duration {
	num, den, ok := m.Tuplet.Ratio()
	if !ok {
		return sounding
	}
	return sounding.Scale(num, den)
}

// AlwaysAbove says this mark goes over the staff whichever way the stem
// points.
//
// A fermata is the case worth naming: it is an instruction to everybody in
// the room to hold, and it is written above the staff in every edition —
// the one under a note means the lower of two parts holds, which is a
// different mark with a glyph of its own. Put on the side away from the
// stem like a staccato dot, a fermata over a whole note came out beneath
// it, where it reads as belonging to music that is not there.
func (a Articulation) AlwaysAbove() bool { return a == Fermata }

// AddArticulation records an articulation, ignoring a repeat of one already
// present.
func (m *Marks) AddArticulation(a Articulation) {
	for _, have := range m.Articulations {
		if have == a {
			return
		}
	}
	m.Articulations = append(m.Articulations, a)
}

// PlaceArticulation says which side of the note one of them is written on.
// PlacementAuto hands the decision back to the engraver.
func (m *Marks) PlaceArticulation(a Articulation, where Placement) bool {
	for i, have := range m.Articulations {
		if have != a {
			continue
		}
		for len(m.ArticulationPlace) <= i {
			m.ArticulationPlace = append(m.ArticulationPlace, PlacementAuto)
		}
		m.ArticulationPlace[i] = where
		return true
	}
	return false
}

// ArticulationPlacement is where one of them is written, or PlacementAuto
// where nobody has said.
func (m *Marks) ArticulationPlacement(i int) Placement {
	if i < 0 || i >= len(m.ArticulationPlace) {
		return PlacementAuto
	}
	return m.ArticulationPlace[i]
}

// RemoveArticulation takes one off, and its placement with it.
func (m *Marks) RemoveArticulation(a Articulation) bool {
	for i, have := range m.Articulations {
		if have != a {
			continue
		}
		m.Articulations = append(m.Articulations[:i:i], m.Articulations[i+1:]...)
		if i < len(m.ArticulationPlace) {
			m.ArticulationPlace = append(
				m.ArticulationPlace[:i:i], m.ArticulationPlace[i+1:]...)
		}
		return true
	}
	return false
}

// HasArticulation reports whether the mark is present.
func (m *Marks) HasArticulation(a Articulation) bool {
	for _, have := range m.Articulations {
		if have == a {
			return true
		}
	}
	return false
}

// Note is one sounding pitch.
type Note struct {
	Pitch Pitch
	Dur   Duration

	// Accidental is what to print, which Pitch.Alter does not determine.
	Accidental Accidental

	// Parens puts the accidental in brackets.
	//
	// A bracketed accidental is a reminder rather than an instruction: the
	// note sounds the same with or without it, and the brackets say that
	// the editor is pointing something out — a courtesy after a tie over a
	// barline, or a reading an editor has supplied. It belongs to the sign
	// and not to the pitch, which is why it sits here and not on Pitch, and
	// it draws nothing at all without one.
	Parens bool

	// Forced keeps the accidental that is written here, whatever the rules
	// of notation would have printed.
	//
	// SpellAccidentals works out which notes need a sign and writes it, and
	// without this there would be no way to tell it it was wrong. Two
	// things need that. One is somebody who wants a sign where none is
	// required, or none where one is — their own reading of a passage, and
	// not a mistake to be corrected on the next edit. The other is a file
	// that says what was printed rather than what sounds: MusicXML's
	// <accidental> is a statement about a page somebody engraved, and
	// throwing it away to recompute it would be gscore overruling the
	// program the music came from.
	//
	// It says nothing about the pitch, which Pitch.Alter holds, and the
	// note still alters everything after it in the bar either way.
	Forced bool

	// Head overrides the shape the notehead is drawn with. On a drum staff
	// this is how a cymbal is told from a drum; elsewhere it is how a
	// harmonic or an improvised bar is written.
	Head NoteheadKind

	// Percussion is the General MIDI drum this note plays, when it is on a
	// percussion staff. A drum staff's positions name instruments rather than
	// pitches, so the pitch alone cannot say what sounds.
	Percussion int

	// String and Fret are where this note is played on a fretted
	// instrument, for a part written as tablature. String is counted from
	// the top line of the tab down, so 1 is the highest string; 0 says
	// nothing was recorded and the engraver works it out from the pitch and
	// the tuning.
	//
	// Recorded rather than always worked out, because which string to play a
	// note on is a decision somebody made: the same pitch is available in
	// several places and they do not sound or finger alike.
	String, Fret int

	Marks
}

// Duration implements Element.
func (n *Note) Duration() Duration { return n.Dur }

// Rest is silence.
type Rest struct {
	Dur Duration

	// Hidden marks a rest that spaces a voice without being printed, which is
	// ABC's x.
	Hidden bool

	Marks
}

// Duration implements Element.
func (r *Rest) Duration() Duration { return r.Dur }

// Chord is several pitches sounded together in one voice.
//
// The 0.0.9 model expressed this by giving each notehead the same group_id in
// one flat list, which left chords.c re-walking the staff to reassemble them
// and broke outright when the note holding the group's id was deleted — a
// FIXME admits as much. Holding the notes together removes both problems.
type Chord struct {
	Notes []*Note
	Dur   Duration

	Marks
}

// Duration implements Element.
func (c *Chord) Duration() Duration { return c.Dur }

// SymbolElement places a symbol in a voice on its own, rather than attached to
// a note: a segno, a coda, a breath mark, or anything the user has drawn. It
// occupies horizontal room but no time.
type SymbolElement struct {
	Symbol SymbolMark
}

// Duration implements Element. A standalone symbol takes no time, so it never
// shifts the notes around it.
func (s *SymbolElement) Duration() Duration { return Duration{Num: 0, Den: 1} }

// Pitches returns the chord's pitches in the order given.
func (c *Chord) Pitches() []Pitch {
	out := make([]Pitch, len(c.Notes))
	for i, n := range c.Notes {
		out[i] = n.Pitch
	}
	return out
}

// Voiced is one element of a measure, with which voice it is in and when it
// starts.
type Voiced struct {
	El    Element
	Voice int      // which line of music on the staff
	Index int      // its place within that voice
	At    Duration // when it starts, from the start of the measure
}

// Ordered is a measure's elements in the one order everything agrees on: by
// the moment they start, and within a moment by voice.
//
// A staff can carry more than one line of music, and the engraver draws them
// as columns — everything starting together in the same place. The playback
// has to walk the same elements in the same order, because the number given
// to each is the only thing tying a sounding note to the notehead drawn for
// it. Two walks that have to agree are two walks that will stop agreeing, so
// there is one here and both use it.
func (m *Measure) Ordered() []Voiced {
	var out []Voiced
	for vi, v := range m.Voices {
		if v == nil {
			continue
		}
		at := Duration{Num: 0, Den: 1}
		for i, el := range v.Elements {
			out = append(out, Voiced{El: el, Voice: vi, Index: i, At: at})
			at = at.Add(el.Duration())
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].At.Cmp(out[j].At) < 0
	})
	return out
}

// marksOf is the marks an element carries, if it is the kind that carries
// any.
func marksOf(el Element) *Marks {
	switch e := el.(type) {
	case *Note:
		return &e.Marks
	case *Chord:
		return &e.Marks
	case *Rest:
		return &e.Marks
	}
	return nil
}

// CrossStaff says this note is drawn on a neighbouring staff.
//
// Piano writing needs it constantly. A figure belongs to one hand's
// rhythm — one beam, one voice — and runs below or above what that hand
// can reach, so the notes that stray are written on the other staff with
// the beam crossing between them. The alternative is to write them in the
// other hand's voice, which puts them in a different rhythm from the
// figure they are part of and is simply a different piece of music.
//
// 1 is the staff below and -1 the staff above; 0, the zero value, is an
// ordinary note. One step only: this is about a pianist's two staves, and
// a note sent two staves away is on nobody's hand.
//
// The pitch does not change and nothing about what sounds changes. The
// note is written somewhere else and played the same, which is the whole
// of what the mark means.
type CrossStaff int

const (
	OnOwnStaff   CrossStaff = 0
	OnStaffBelow CrossStaff = 1
	OnStaffAbove CrossStaff = -1
)

func (c CrossStaff) String() string {
	switch c {
	case OnStaffBelow:
		return "below"
	case OnStaffAbove:
		return "above"
	}
	return "own"
}

// CrossStaffFromName is the inverse, for files and query strings.
func CrossStaffFromName(s string) (CrossStaff, bool) {
	switch s {
	case "below":
		return OnStaffBelow, true
	case "above":
		return OnStaffAbove, true
	case "own", "none", "":
		return OnOwnStaff, true
	}
	return OnOwnStaff, false
}
