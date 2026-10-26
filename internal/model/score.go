/*
 * score.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package model holds the musical content of a score and nothing else.
//
// Nothing here knows about pixels, widgets, fonts or files. The 0.0.9 Score_t
// carried width, height, staff_extremity_end_x, staff_startx and a window
// title alongside the music; all of that was display state, and its absence
// here is the point of the package. Engraving reads this model and produces
// geometry separately, which is what makes both halves testable.
package model

import (
	"fmt"
	"strings"
)

// Clef fixes which pitch each staff line carries.
type Clef int

// The clefs 0.0.9's staff code supported.
const (
	Treble Clef = iota
	Bass
	Alto
	Tenor
	Percussion
)

var clefNames = [...]string{"treble", "bass", "alto", "tenor", "percussion"}

func (c Clef) String() string {
	if c < Treble || int(c) >= len(clefNames) {
		return "unknown"
	}
	return clefNames[c]
}

// Mode is the scale a key signature is understood in. Two keys can share a
// signature and differ in mode, so the mode is kept rather than inferred.
type Mode int

// The church modes, with Major and Minor as the usual names for two of them.
const (
	Major Mode = iota
	Minor
	Dorian
	Phrygian
	Lydian
	Mixolydian
	Locrian
)

var modeNames = [...]string{
	"major", "minor", "dorian", "phrygian", "lydian", "mixolydian", "locrian",
}

func (m Mode) String() string {
	if m < Major || int(m) >= len(modeNames) {
		return "unknown"
	}
	return modeNames[m]
}

// KeySignature is held as a position on the circle of fifths: 0 is C, positive
// counts sharps and negative counts flats, so the range is -7 to 7.
type KeySignature struct {
	Fifths int
	Mode   Mode
}

// sharpOrder and flatOrder give the steps a signature alters, in the order the
// accidentals are written on the staff.
var (
	sharpOrder = [7]Step{StepF, StepC, StepG, StepD, StepA, StepE, StepB}
	flatOrder  = [7]Step{StepB, StepE, StepA, StepD, StepG, StepC, StepF}
)

// AlterFor returns the alteration the signature applies to a step, in
// semitones. This is what turns a written note into a sounding pitch.
func (k KeySignature) AlterFor(s Step) int {
	switch {
	case k.Fifths > 0:
		for i := 0; i < k.Fifths && i < len(sharpOrder); i++ {
			if sharpOrder[i] == s {
				return 1
			}
		}
	case k.Fifths < 0:
		for i := 0; i < -k.Fifths && i < len(flatOrder); i++ {
			if flatOrder[i] == s {
				return -1
			}
		}
	}
	return 0
}

// TimeSymbol distinguishes a time signature printed as digits from the two
// that have their own glyph.
type TimeSymbol int

// How a time signature is drawn.
const (
	TimeNumeric TimeSymbol = iota
	TimeCommon             // the C glyph, for 4/4
	TimeCut                // the slashed C, for 2/2
)

// TimeSignature is the meter. Beats over BeatValue, as in 6 over 8.
//
// The 0.0.9 Staff_t declared guint8 time_signature[2] and then wrote to index
// 2 in staff.c, one past the end; it never crashed only because struct padding
// happened to absorb the byte. Named fields make that class of bug
// unrepresentable.
type TimeSignature struct {
	Beats     int
	BeatValue int
	Symbol    TimeSymbol
}

// BarDuration is how much time a full measure of this meter holds.
func (t TimeSignature) BarDuration() Duration {
	if t.BeatValue == 0 {
		return Duration{0, 1}
	}
	return Duration{t.Beats, t.BeatValue}.Reduced()
}

func (t TimeSignature) String() string { return fmt.Sprintf("%d/%d", t.Beats, t.BeatValue) }

// BarlineStyle is the kind of barline closing a measure.
type BarlineStyle int

// The barline styles the ABC samples use, plus repeats.
const (
	BarSingle BarlineStyle = iota
	BarDouble
	BarFinal
	BarRepeatStart
	BarRepeatEnd
	BarRepeatBoth
	BarNone
)

var barlineNames = [...]string{
	"single", "double", "final", "repeat-start", "repeat-end", "repeat-both", "none",
}

func (b BarlineStyle) String() string {
	if b < BarSingle || int(b) >= len(barlineNames) {
		return "unknown"
	}
	return barlineNames[b]
}

// Tempo is a metronome mark: Unit notes per minute, optionally with a text
// indication such as "Allegro" beside it.
type Tempo struct {
	BPM  int
	Unit Duration
	Text string

	// Parens puts the metronome mark in brackets, which is how a score
	// writes one that is a suggestion rather than an instruction — and how
	// nearly every engraved score writes one that follows a word. Altes
	// prints "All° Moderato (♩ = 116)", and so did everybody else in 1888.
	Parens bool
}

// Identity is the bibliographic header of a score, from the 0.0.9 Identity_t.
type Identity struct {
	Title     string
	Subtitle  string
	Composer  string
	Copyright string
}

// Voice is one monophonic line of music within a measure.
//
// A measure holds a slice of these so that two independent lines can share a
// staff. The 0.0.9 model had no concept of a voice, which is why its own ABC
// sample, carrying V:0 and V:1, could not round-trip.
type Voice struct {
	Elements []Element
}

// Duration totals the voice's elements.
func (v *Voice) Duration() Duration {
	total := Duration{0, 1}
	for _, e := range v.Elements {
		total = total.Add(e.Duration())
	}
	return total
}

// Measure is one bar of one part.
type Measure struct {
	Voices []*Voice

	// RightBarline closes the measure. The opening barline of a measure is the
	// closing barline of the one before it, so only one is stored.
	RightBarline BarlineStyle

	// Key and Time, when set, change the signature from this measure on. Music
	// changes key and meter in the middle of a piece, and a signature that
	// lived only on the part could not say so. Both are pointers because the
	// absence of a change is not the same as a change to C major.
	Key  *KeySignature
	Time *TimeSignature

	// Clef, when set, changes the clef from this measure on.
	//
	// A staff is not one clef. A pianist's right hand drops into the bass
	// for a phrase and comes back; a cello part climbs into the tenor clef
	// and down again. The sign is drawn where it changes and everything
	// after it is read against it, which is why this cannot live on the
	// part alone — and why a score that keeps one clef per staff draws the
	// notes of those passages an octave and a half away from where they
	// belong, if it draws them at all.
	Clef *Clef

	// Tempo, when set, is a new speed written over this bar and held until
	// the next one says otherwise.
	//
	// A piece is not one speed. "Slower" at the bridge, a tempo at the last
	// chorus, a metronome mark at every section of an exercise: all of it is
	// ordinary writing, and a score that keeps a single number for the whole
	// piece cannot say any of it — nor play it, which is what the number is
	// mostly for.
	//
	// Like a repeat it belongs to the piece rather than to a staff;
	// Score.TempoAt reads it from whichever part carries it, and the mark is
	// drawn once above the system. The score's own Tempo is the one it
	// starts in, which is the same thing written at bar one.
	Tempo *Tempo

	// Ending is a volta bracket: the first-time and second-time bars of a
	// repeat. It is stored per measure of each part, as MusicXML stores it,
	// and drawn once above the system.
	Ending *Ending

	// Repeat marks the bar as "play the one before again", drawn as the
	// slash with two dots that every chart uses. It is 1 for a one-bar
	// repeat and 2 for the sign that stands for the two bars before it.
	//
	// It belongs to one staff, not to the score: a drummer repeating a bar
	// while the horns play something new is the ordinary case, and in fact
	// most of what the sign is for.
	Repeat int

	// Marker is a rehearsal mark — the boxed A, B, C that a conductor calls
	// out. It belongs to the bar across the whole score, because "from B"
	// has to mean the same bar to everybody in the room.
	Marker string

	// Sign is the segno, the coda, Fine or To Coda written at this bar: the
	// places the directions below refer to.
	Sign Sign

	// Jump is the direction written at the end of this bar — D.C., D.S. and
	// their al Fine and al Coda forms — which sends the player back.
	//
	// Like a repeat, it belongs to the piece and not to a staff: everybody
	// goes back at the same bar. The editor writes it on every part, and
	// playback reads it from whichever has it.
	Jump *Jump
}

// Ending is one bracket of a repeated passage — the "1." and "2." written over
// the bars that differ between times through.
type Ending struct {
	// Numbers are the times through that this bracket is played, so a bracket
	// marked "1., 2." holds both.
	Numbers []int
	// Start opens the bracket at this measure and Stop closes it at the end of
	// this one. A bracket one bar long has both.
	Start, Stop bool
	// Open leaves the bracket without its closing hook, which is how a last
	// ending is written: there is nothing to go back for.
	Open bool
}

// Label spells the bracket the way it is printed: "1." or "1., 2.".
func (e *Ending) Label() string {
	if e == nil || len(e.Numbers) == 0 {
		return ""
	}
	var b strings.Builder
	for i, n := range e.Numbers {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d.", n)
	}
	return b.String()
}

// Part is one instrumental line, drawn on its own staff.
type Part struct {
	ID   string
	Name string
	// Instrument is what this staff sounds as, carried over from
	// Staff_t.midi_instrument.
	Instrument Instrument
	Clef       Clef
	Key        KeySignature
	Time       TimeSignature
	// Transpose is the interval between this staff's written pitches and what
	// is heard. Everything stored in Measures is written pitch.
	Transpose Transposition

	// Hidden leaves the staff out of the drawing without taking it out of
	// the score. A conductor working on the strings does not want the rest of
	// the orchestra in the way, and a part still has to sound while it is out
	// of sight — so this is a decision about the page and not about the
	// music, and playback ignores it.
	Hidden bool

	// Tab writes this part as tablature instead of on a staff of pitches:
	// a line for each string, and the fret to stop it at.
	//
	// It is a way of writing the same music, not different music. The notes
	// keep their pitches — that is what sounds, what transposes and what a
	// player of any other instrument reads — and the tab is how they are
	// shown. A part written as tab and the same part written on a staff are
	// the same Measures.
	Tab *Tablature

	Measures []*Measure
}

// Tablature is how a fretted instrument's part is written: one line per
// string, and on it the fret to stop that string at.
type Tablature struct {
	// Strings are the pitches of the open strings, from the top line of the
	// tab downwards — for a guitar, E4 B3 G3 D3 A2 E2.
	//
	// Downwards, because that is the order they are drawn in and the order
	// the numbering follows: string 1 is the line at the top. Files list
	// them the other way about, lowest first, and the reader turns them
	// round once rather than everything after it counting backwards.
	Strings []Pitch

	// Frets is how many the instrument has, which is what says whether a
	// note can be played at all.
	Frets int
}

// Lines is how many lines the tab staff has: one per string.
func (t *Tablature) Lines() int {
	if t == nil {
		return 0
	}
	return len(t.Strings)
}

// At finds where a pitch is played: the string nearest the top that can
// reach it, and the fret that does.
//
// The highest string that will reach gives the lowest fret, which is where a
// player's hand goes unless the music says otherwise. A note the instrument
// cannot reach at all answers false, and is written as the pitch it is
// rather than as a fret that does not exist.
func (t *Tablature) At(p Pitch) (str, fret int, ok bool) {
	if t == nil || len(t.Strings) == 0 {
		return 0, 0, false
	}
	want := p.MIDI()
	for i, open := range t.Strings {
		f := want - open.MIDI()
		if f < 0 {
			continue // this string is already higher than the note
		}
		if t.Frets > 0 && f > t.Frets {
			continue // off the end of the neck
		}
		return i + 1, f, true
	}
	return 0, 0, false
}

// KeyAt is the key signature in force at a measure, following any change
// written in an earlier one.
func (p *Part) KeyAt(index int) KeySignature {
	k := p.Key
	for i := 0; i <= index && i < len(p.Measures); i++ {
		if p.Measures[i].Key != nil {
			k = *p.Measures[i].Key
		}
	}
	return k
}

// ClefAt is the clef in force at a bar, following any change written in an
// earlier one.
func (p *Part) ClefAt(index int) Clef {
	c := p.Clef
	for i := 0; i <= index && i < len(p.Measures); i++ {
		if p.Measures[i].Clef != nil {
			c = *p.Measures[i].Clef
		}
	}
	return c
}

// TempoAt is the speed in force at a bar, following any change written in an
// earlier one.
//
// It is asked of the score rather than of a part because a tempo change is
// written once, over the top staff, and means the same to everybody in the
// room. Whichever part carries it, it is the piece's.
func (s *Score) TempoAt(index int) Tempo {
	t := s.Tempo
	for i := 0; i <= index; i++ {
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].Tempo != nil {
				t = *p.Measures[i].Tempo
				break
			}
		}
	}
	return t
}

// TempoChangeAt is the tempo written at exactly this bar, if any, and nothing
// when the bar only carries on at the speed it was given earlier.
func (s *Score) TempoChangeAt(index int) *Tempo {
	for _, p := range s.Parts {
		if index < len(p.Measures) && p.Measures[index].Tempo != nil {
			return p.Measures[index].Tempo
		}
	}
	return nil
}

// TimeAt is the meter in force at a measure, following any change written in
// an earlier one.
func (p *Part) TimeAt(index int) TimeSignature {
	t := p.Time
	for i := 0; i <= index && i < len(p.Measures); i++ {
		if p.Measures[i].Time != nil {
			t = *p.Measures[i].Time
		}
	}
	return t
}

// Cursor is where the writing cursor was left.
//
// Not music, and said so: it is where somebody was working, the way a text
// editor remembers the line you were on. It is kept with the score because
// losing it is the small, daily annoyance of opening a piece and having to
// find your place again.
type Cursor struct {
	// Part and Measure are which staff and which bar.
	Part, Measure int
	// At is the place between elements the cursor sits in: 0 is before the
	// first, len is after the last.
	At int
	// Step is the staff position the next note would be written at, counted
	// in half spaces from the bottom line.
	Step int
}

// Score is a complete piece of music.
type Score struct {
	Identity Identity
	Tempo    Tempo
	Parts    []*Part

	// Cursor is where the writing cursor was left, when the file said. A
	// score read from a format that does not record one has none, and the
	// editor puts the cursor at the end of the first staff instead.
	Cursor *Cursor

	// Groups join staves that belong together: the two hands of a piano, the
	// four staves of a string quartet. Without them a score is a stack of
	// unrelated lines, and the eye has nothing to tell it where one
	// instrument ends and the next begins.
	Groups []StaffGroup
}

// GroupSymbol is what is drawn down the left of a group of staves.
type GroupSymbol int

// A brace joins the staves of one instrument — a piano, a harp. A bracket
// joins different instruments of a family — the strings, the woodwind. A plain
// line joins without saying which.
const (
	GroupBracket GroupSymbol = iota
	GroupBrace
	GroupLine
)

func (g GroupSymbol) String() string {
	switch g {
	case GroupBrace:
		return "brace"
	case GroupLine:
		return "line"
	}
	return "bracket"
}

// StaffGroup is a run of staves drawn as belonging together.
type StaffGroup struct {
	// First and Last are indices into Score.Parts, inclusive.
	First, Last int
	Symbol      GroupSymbol
	// Barlines draws the barlines through the gap between the staves, which
	// is what says "play these together" rather than "these are next to each
	// other". A piano has it; a choir usually does not.
	Barlines bool
	// Name labels the group rather than each staff — "Piano", "Violin".
	Name string
}

// NewScore returns the score a new document starts from: one treble part in C
// major and common time, holding a single empty measure.
//
// The empty measure is deliberate and follows the C program, where
// create_staff set both measure_number and total_measures to 1. A score with
// no measures at all has nowhere to put the first note, so there would be
// nothing to draw and nothing to click.
func NewScore() *Score {
	return &Score{
		Tempo: Tempo{BPM: 120, Unit: Duration{1, 4}},
		Parts: []*Part{{
			Clef:     Treble,
			Key:      KeySignature{Fifths: 0, Mode: Major},
			Time:     TimeSignature{Beats: 4, BeatValue: 4, Symbol: TimeNumeric},
			Measures: []*Measure{NewMeasure()},
		}},
	}
}

// NewMeasure returns an empty measure holding one empty voice.
func NewMeasure() *Measure {
	return &Measure{
		Voices:       []*Voice{{}},
		RightBarline: BarSingle,
	}
}

// AppendMeasure adds an empty measure to the part and returns it.
func (p *Part) AppendMeasure() *Measure {
	m := NewMeasure()
	p.Measures = append(p.Measures, m)
	return m
}

/*
 * Transposing instruments
 */

// Transposition is the interval between what a player reads and what sounds.
//
// An alto saxophone reading a written C sounds the E-flat a major sixth below
// it, which is Diatonic -5 and Chromatic -9. The two numbers are both needed
// and for the same reason Pitch.Transpose needs both: the semitones say what
// is heard and the steps say how it is spelled, and an interval is not fully
// described by either alone.
//
// The direction is written-to-sounding, as MusicXML's <transpose> is, so a
// score read from one program and written back to another keeps its meaning.
// gscore stores the written pitches — what is actually printed on that staff —
// so clicking the staff puts a note exactly where it was clicked, whatever
// instrument the staff is for. Playback is where the transposition is applied.
type Transposition struct {
	Diatonic     int
	Chromatic    int
	OctaveChange int
}

// IsZero reports whether the part sounds as written, which most do.
func (t Transposition) IsZero() bool {
	return t.Diatonic == 0 && t.Chromatic == 0 && t.OctaveChange == 0
}

// Steps and Semitones are the whole interval, octave change included.
func (t Transposition) Steps() int     { return t.Diatonic + 7*t.OctaveChange }
func (t Transposition) Semitones() int { return t.Chromatic + 12*t.OctaveChange }

// Apply turns a written pitch into the one that sounds.
func (t Transposition) Apply(p Pitch) Pitch {
	if t.IsZero() {
		return p
	}
	return p.Transpose(t.Steps(), t.Semitones())
}

// Fifths is how far around the circle the written key moves.
//
// A key signature is not transposed by moving its accidentals: it is a point
// on the circle of fifths, and an interval moves it by a fixed number of
// places. Seven semitones — a fifth — is one place sharpwards, and every other
// interval follows from that, which is why this is arithmetic rather than a
// table.
func (t Transposition) Fifths() int {
	// Each semitone of the interval is 7 places around the circle, modulo 12.
	f := (t.Semitones() * 7) % 12
	if f > 6 {
		f -= 12
	}
	if f < -5 {
		f += 12
	}
	return f
}

// Written returns the key signature as that instrument's player reads it.
//
// A concert-pitch score in C gives an alto saxophone three sharps, because the
// part is written a major sixth above what it sounds.
func (t Transposition) Written(k KeySignature) KeySignature {
	if t.IsZero() {
		return k
	}
	out := k
	out.Fifths = k.Fifths - t.Fifths()
	for out.Fifths > 7 {
		out.Fifths -= 12
	}
	for out.Fifths < -7 {
		out.Fifths += 12
	}
	return out
}

// TranspositionFor gives the usual transposition of an instrument, so that
// choosing "alto sax" sets up a part that behaves like one without the player
// having to know the interval by heart.
//
// Only the instruments that are not in C are listed; everything else sounds as
// written and gets the zero value.
func TranspositionFor(i Instrument) Transposition {
	switch i {
	case SopranoSax, Clarinet, Trumpet:
		// In B-flat: sounds a major second below what is written.
		return Transposition{Diatonic: -1, Chromatic: -2}
	case AltoSax:
		// In E-flat: a major sixth below.
		return Transposition{Diatonic: -5, Chromatic: -9}
	case TenorSax:
		// In B-flat, a major ninth below.
		return Transposition{Diatonic: -1, Chromatic: -2, OctaveChange: -1}
	case BaritoneSax:
		// In E-flat, an octave and a major sixth below.
		return Transposition{Diatonic: -5, Chromatic: -9, OctaveChange: -1}
	case FrenchHorn, EnglishHorn:
		// In F: a perfect fifth below.
		return Transposition{Diatonic: -4, Chromatic: -7}
	case Piccolo:
		return Transposition{OctaveChange: 1}
	case Contrabass, AcousticGuitar, ElectricGuitar, AcousticBass:
		// Written an octave above what they sound.
		return Transposition{OctaveChange: -1}
	}
	return Transposition{}
}

// Normalise tidies a score after it has been read.
//
// Two things go wrong when a file is read and nothing else notices.
//
// A part can arrive with fewer measures than the rest, because the reader
// stopped early or because the file was written that way. The engraver draws
// every staff to the same width whatever is on it, so a short staff is ruled
// right across the system and then simply stops having music in it — which
// looks exactly like a bar of notes printed after the final barline. Padding
// them to the longest keeps the barlines in a column, which is the whole point
// of a score.
//
// A part can also arrive with no measures at all, which happens when a reader
// fails on a staff and keeps what it already had. That is not an empty staff
// somebody wanted, it is wreckage, and drawing five ruled lines with nothing
// on them says the file had an empty staff in it. It did not.
func (s *Score) Normalise() {
	if s == nil {
		return
	}

	kept := s.Parts[:0]
	dropped := 0
	for i, p := range s.Parts {
		if len(p.Measures) == 0 && i == len(s.Parts)-1 && len(s.Parts) > 1 {
			// Only a trailing one: a staff in the middle with no measures
			// was meant to be there, and padding will fill it.
			dropped++
			continue
		}
		kept = append(kept, p)
	}
	s.Parts = kept
	if dropped > 0 {
		var groups []StaffGroup
		for _, g := range s.Groups {
			if g.Last < len(s.Parts) && g.Last > g.First {
				groups = append(groups, g)
			}
		}
		s.Groups = groups
	}

	longest := 0
	for _, p := range s.Parts {
		if n := len(p.Measures); n > longest {
			longest = n
		}
	}
	for _, p := range s.Parts {
		// The barlines of the part that reaches furthest, so a repeat or a
		// final bar still runs down the whole system.
		for len(p.Measures) < longest {
			p.Measures = append(p.Measures, &Measure{
				Voices: []*Voice{{}}, RightBarline: BarSingle,
			})
		}
		for _, m := range p.Measures {
			if len(m.Voices) == 0 {
				m.Voices = []*Voice{{}}
			}
		}
	}
	if longest > 0 && len(s.Parts) > 0 {
		src := s.Parts[0]
		for _, p := range s.Parts {
			for i := range p.Measures {
				if len(p.Measures[i].Voices[0].Elements) == 0 && i < len(src.Measures) {
					p.Measures[i].RightBarline = src.Measures[i].RightBarline
				}
			}
		}
	}
	s.agreeOnRepeats(longest)
}

// agreeOnRepeats gives every staff the repeats and the final barline the
// piece has, at the bars it has them.
//
// A repeat belongs to the piece and not to a staff: everybody goes back at
// the same bar, and everybody stops at the same one. Drawn down one staff of
// a system and not the rest, the sign is not a repeat — it is a mistake, and
// a player reading any other part never sees it at all.
//
// It has to be done here rather than left to each reader, because the files
// do not say it. MuseScore writes the repeat once, on the first staff, and
// means it for the score; so does everything else that keeps a repeat as a
// property of the bar. A reader that copied it onto every staff would be
// guessing in one format and right in the next, and the fact is about music
// either way.
//
// Only repeats and the final barline. A double barline is a section's end and
// may well be written on one staff and not another — this is not a rule that
// every barline down a column has to match.
func (s *Score) agreeOnRepeats(bars int) {
	for i := 0; i < bars; i++ {
		start, end, final := false, false, false
		for _, p := range s.Parts {
			if i >= len(p.Measures) {
				continue
			}
			switch p.Measures[i].RightBarline {
			case BarRepeatStart:
				start = true
			case BarRepeatEnd:
				end = true
			case BarRepeatBoth:
				start, end = true, true
			case BarFinal:
				final = true
			}
		}
		if !start && !end && !final {
			continue
		}
		// A repeat outranks the final barline where a bar carries both: the
		// repeat sign is drawn heavy already, and losing the dots would lose
		// the instruction.
		style := BarFinal
		switch {
		case start && end:
			style = BarRepeatBoth
		case start:
			style = BarRepeatStart
		case end:
			style = BarRepeatEnd
		}
		for _, p := range s.Parts {
			if i < len(p.Measures) {
				p.Measures[i].RightBarline = style
			}
		}
	}
}

// How long a bar lasts.
//
// This is one question with one answer, and it is asked in two places: the
// engraver needs it to say where in its bar a note falls, and playback needs
// it to decide when each bar begins. Answering it twice is how the cursor and
// the note it points at came to disagree — the engraver measured against the
// meter while playback measured the music actually in the bar, and in a score
// where those differ the two drift apart within the bar.

// RepeatSource is the voice a repeat sign stands for.
//
// One sign means the bar before, a two-bar sign the two before it; walking
// forwards that is "the bar n back" either way. A sign pointing at another
// sign is followed through, but not forever: a file that points one at itself
// comes back empty rather than hanging.
func (p *Part) RepeatSource(at, bars int) *Voice {
	if bars < 1 {
		bars = 1
	}
	for hops := 0; hops < 64; hops++ {
		at -= bars
		if at < 0 || at >= len(p.Measures) {
			return nil
		}
		m := p.Measures[at]
		if len(m.Voices) == 0 {
			return nil
		}
		if m.Repeat == 0 {
			return m.Voices[0]
		}
		bars = m.Repeat
	}
	return nil
}

// MeasureContent is the music in one bar of one staff, following a repeat
// sign to the bar it stands for.
func (p *Part) MeasureContent(i int) *Voice {
	if i < 0 || i >= len(p.Measures) {
		return nil
	}
	m := p.Measures[i]
	if len(m.Voices) == 0 {
		return nil
	}
	if m.Repeat > 0 {
		if src := p.RepeatSource(i, m.Repeat); src != nil {
			return src
		}
	}
	return m.Voices[0]
}

// MeasureDuration is how long a bar lasts, across the whole score.
//
// As long as the staff that fills it most: a staff resting through it must not
// shorten it, and neither must a staff that stops early and is padded with an
// empty bar so the barlines stay in a column. A bar nothing sounds in takes
// its meter's worth, or a cursor would skip straight over it.
func (s *Score) MeasureDuration(i int) Duration {
	longest := Duration{Num: 0, Den: 1}
	for _, p := range s.Parts {
		v := p.MeasureContent(i)
		if v == nil {
			continue
		}
		if d := v.Duration(); d.Cmp(longest) > 0 {
			longest = d
		}
	}
	if !longest.IsZero() {
		return longest
	}
	for _, p := range s.Parts {
		if i < len(p.Measures) {
			if bar := p.TimeAt(i).BarDuration(); !bar.IsZero() {
				return bar
			}
		}
	}
	return Duration{Num: 0, Den: 1}
}

// MeasureCount is how many bars the score has, which is as many as its
// longest staff.
func (s *Score) MeasureCount() int {
	n := 0
	for _, p := range s.Parts {
		if len(p.Measures) > n {
			n = len(p.Measures)
		}
	}
	return n
}
