/*
 * element.go
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

package model

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

// The articulations carried over from legacy/src/articulation.c, plus the bow
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
)

var articulationNames = [...]string{
	"staccato", "staccatissimo", "accent", "tenuto", "marcato",
	"marcato down", "fermata", "up-bow", "down-bow",
}

func (a Articulation) String() string {
	if a < Staccato || int(a) >= len(articulationNames) {
		return "unknown"
	}
	return articulationNames[a]
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

	// Stem overrides the direction the engraver would otherwise choose.
	Stem Stem

	// Dynamic is a volume marking written at this point in the music.
	Dynamic Dynamic
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
