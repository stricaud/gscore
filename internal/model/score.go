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
	// Treble8vb is the treble clef with an 8 under it: the same staff
	// positions, sounding an octave lower. It is what tenors read in every
	// choral score there is, and what a guitar is written in.
	//
	// The notes on the page are the notes of a treble staff — that is the
	// whole point of the clef — so the octave is in the sound and not in
	// the writing, which is where Transpose keeps it.
	Treble8vb
)

var clefNames = [...]string{"treble", "bass", "alto", "tenor", "percussion",
	"treble8vb"}

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
	// TimeHidden is a meter that is in force and is not drawn.
	//
	// The bars are counted in it and nothing says so on the page. It is
	// how a score with staves in different meters is written down — the
	// bars have to be some length, and the length is not what any one
	// staff is printed in — and it is what Dorico writes for the 3/4 that
	// makes Bach's Jesu, joy align against the 9/8 of the first violin.
	// Drawn, that 3/4 appears at the second bar of the piece and reads as
	// a change of meter nobody wrote.
	TimeHidden
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

// BeatUnit is the note the meter is counted in: what a conductor's hand
// marks and what a metronome ticks.
//
// Not the lower number of the signature. A compound meter is written in
// eighths and counted in threes — 6/8 is two beats of a dotted quarter, not
// six of an eighth — which is why its notes are beamed in threes and why a
// metronome mark for it is written over a dotted crotchet. Counting the
// bottom number gives three times as many ticks as anybody beats, and at a
// jig's speed that is a rattle rather than a pulse.
//
// 3/8 is the exception the rule needs: it is one bar of three quavers at
// speed and is beaten in three, so the threes only start once there are more
// of them than one bar's worth.
//
// The one place this is decided. The beaming asks it, the row of beat marks
// over the staff asks it, and the metronome that ticks under the playback
// asks it — and a beat that is one thing to the eye and another to the ear
// is worse than no beat at all.
func (t TimeSignature) BeatUnit() Duration {
	if t.BeatValue == 0 {
		return Duration{Num: 1, Den: 4}
	}
	if t.Beats%3 == 0 && t.Beats > 3 {
		return Duration{Num: 3, Den: t.BeatValue}.Reduced()
	}
	return Duration{Num: 1, Den: t.BeatValue}
}

// BeatsPerBar is how many of those beats a full bar of this meter holds.
func (t TimeSignature) BeatsPerBar() int {
	u := t.BeatUnit()
	if u.IsZero() {
		return 0
	}
	bar := t.BarDuration()
	if bar.IsZero() {
		return 0
	}
	// Rounded rather than truncated: the division is exact in every meter
	// that divides evenly, and the float is there only for the ones that do
	// not — 7/8, which is seven eighths counted in sevens.
	n := int(bar.Float()/u.Float() + 0.5)
	if n < 1 {
		return 1
	}
	return n
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
	// BarHeavyLight is the final barline the other way round: a heavy
	// stroke and then a thin one. It opens rather than closes — the first
	// bar of a piece after its introduction, the head of a new section —
	// and is the mirror of the sign at the end, which is why the books
	// call the pair of them opening and closing double bars. MusicXML
	// names it heavy-light, and NoteWorthy and ABC both have it: a score
	// read from either had its openings drawn as ordinary double bars.
	BarHeavyLight
)

var barlineNames = [...]string{
	"single", "double", "final", "repeat-start", "repeat-end", "repeat-both", "none",
	"heavy-light",
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

	// ToBPM and OverBars are a speed on its way somewhere: a ritardando or
	// an accelerando, which is not a new tempo but a journey to one.
	//
	// The words over the staff — "rit.", "accel.", "rall." — are an
	// instruction to a player and a slowing to anything that plays the
	// score, and a reader who is given only the words hears the music run
	// on at the old speed straight through the end of a phrase. Beethoven's
	// fifth has five of them.
	//
	// BPM is where the change starts and ToBPM where it arrives, over
	// OverBars bars: the speed at the start of each bar in between is a
	// step along the way, which is how a sequencer plays one and close
	// enough to how a player does. ToBPM of nought is an ordinary tempo
	// that stays where it is.
	ToBPM    int `json:",omitempty"`
	OverBars int `json:",omitempty"`

	// Silent keeps the number off the page while still playing at it.
	//
	// "Allegro Moderato" over a bar is an instruction to a player and a
	// speed to a sequencer, and most scores print only the words. A file
	// says so by giving a speed with no metronome mark to print: MusicXML
	// writes <sound tempo="114"/> under the word and nothing else, which
	// is what every editor does for a tempo the user did not ask to see.
	// Drawing a crotchet and a number there puts a mark on the page that
	// nobody wrote.
	Silent bool
}

// Identity is the bibliographic header of a score, from the 0.0.9 Identity_t.
type Identity struct {
	Title    string
	Subtitle string
	Composer string
	// Arranger is its own field and not a second line of the composer.
	//
	// They are two people and a score credits them in two places — the
	// composer above, the arranger under them, often in different type —
	// and every format worth reading says which is which: MusicXML has
	// <creator type="arranger">, Finale and MuseScore both keep their own
	// field for it. Folded into the composer it comes back as part of
	// somebody's name, and a search for the arranger's work finds nothing.
	Arranger string
	// Lyricist is who wrote the words, and is its own field for the reason
	// the arranger is: they are a different person and a score credits
	// them in a different place.
	//
	// A song prints the two of them level with each other, the words on
	// the left and the music on the right — "Words by OTTO HARBACH and
	// OSCAR HAMMERSTEIN IInd" against "Music by RUDOLF FRIML" — which is a
	// layout and not a sentence, and cannot be had by writing both names
	// into one field. Every format that matters keeps them apart as well:
	// MusicXML has <creator type="lyricist">, and so a lyricist folded
	// into the composer comes back from a round trip as part of the
	// composer's name.
	Lyricist  string
	Copyright string

	// Runs is the formatting of the words above, where somebody has given
	// them any: the same text broken into the runs it is set in.
	//
	// The words stay in the plain fields, which is what every importer
	// writes and every exporter reads — formatting is for the page and
	// nothing else should have to know about it. A field with no runs is
	// plain, which is nearly every field of nearly every score.
	TitleRuns    []Span
	SubtitleRuns []Span
	ComposerRuns []Span
	ArrangerRuns []Span
	LyricistRuns []Span
}

// Artwork is a picture kept in the score: the file as it was given, and
// where on the page it goes.
//
// The bytes travel in the score rather than a path to them, for the reason
// the scripts and the comments do: a file that is copied, renamed or sent to
// somebody takes everything it needs with it, and a path does not — it is a
// picture that is there on the machine it was made on and missing
// everywhere else.
//
// The file is kept as it came. Converting it to something of ours would be
// a second decoding to get wrong and would throw away whatever the original
// knew: an SVG stays a drawing that prints at any size, and a photograph
// stays a photograph.
type Artwork struct {
	// Data is the file itself, and Format what kind it is: "png", "jpeg"
	// or "svg".
	Data   []byte
	Format string
	// Align is "left", "center" or "right", for a picture narrower than
	// the page. Empty means centred.
	Align string
}

// Clone returns a copy sharing no bytes with the one it came from.
func (a *Artwork) Clone() *Artwork {
	if a == nil {
		return nil
	}
	out := *a
	out.Data = append([]byte(nil), a.Data...)
	return &out
}

// Span is a run of header text in one style.
//
// A credit is not one thing in one face as often as it is: "Music by" plain
// with the name bold after it, an arranger's line in grey, a title with the
// opus number smaller behind it. A span is one such piece of a line.
type Span struct {
	Text   string `json:"text"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
	// Colour is a CSS colour, empty being the ink the page is printed in.
	Colour string `json:"colour,omitempty"`
	// Size is a multiple of the size the field is set at, so that a run
	// stays in proportion when the title size changes. Zero means one.
	Size float64 `json:"size,omitempty"`
}

// Scale is Size with its zero read as the one it means.
func (s Span) Scale() float64 {
	if s.Size <= 0 {
		return 1
	}
	return s.Size
}

// Spans is one header field as it is drawn: its runs, where they are the
// words it holds, and the words in one plain run where they are not.
//
// The check is the point. The words and their formatting are stored apart,
// so a file written by hand, an older file, or an importer that knows about
// one and not the other can hand over runs that say something other than the
// field does — and words on the page that the file does not say are worse
// than words with no formatting. So they are only used when they add up.
func Spans(text string, runs []Span) []Span {
	if text == "" {
		return nil
	}
	if PlainSpans(runs) == text {
		return runs
	}
	return []Span{{Text: text}}
}

// Clone is an Identity sharing no run list with the one it came from. The
// words are strings and come with the copy; the formatting is slices and
// does not.
func (i Identity) Clone() Identity {
	out := i
	for _, f := range []struct{ from, to *[]Span }{
		{&i.TitleRuns, &out.TitleRuns},
		{&i.SubtitleRuns, &out.SubtitleRuns},
		{&i.ComposerRuns, &out.ComposerRuns},
		{&i.ArrangerRuns, &out.ArrangerRuns},
		{&i.LyricistRuns, &out.LyricistRuns},
	} {
		if *f.from != nil {
			*f.to = append([]Span(nil), *f.from...)
		}
	}
	return out
}

// PlainSpans is the words of a run list with the formatting dropped.
func PlainSpans(runs []Span) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
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

	// Marker is a rehearsal mark written out as itself — the boxed A, B, C
	// that a conductor calls out. It belongs to the bar across the whole
	// score, because "from B" has to mean the same bar to everybody in the
	// room.
	//
	// Used for the marks that are not part of the sequence: "1a" at an
	// insert, a word at the top of a chorus, and everything read out of a
	// file, which arrives already written out. For a mark that should
	// count along with the others, see MarkerAuto.
	Marker string

	// MarkerAuto says this bar is in the rehearsal sequence, and leaves
	// what it is called to be worked out from where it falls.
	//
	// The difference matters the moment anybody edits. A letter stored as
	// the letter is right until a mark is put in before it, and then the
	// page reads A, B, D, C with nothing to say which is wrong; delete one
	// and the sequence has a hole. Stored as a place in the sequence, all
	// of that comes out right without anybody being asked, which is the
	// whole reason to keep it this way round.
	//
	// What it is called comes from Score.Markers — letters, numbers, or
	// the bar's own number. See Score.RehearsalMarks.
	MarkerAuto bool `json:",omitempty"`

	// SectionTitle names the movement that begins at this bar, and is
	// written over the staff where it begins.
	//
	// Empty on every bar but the first of a movement — and on that one
	// only where the file says what the movement is called. A sonata's
	// third movement is headed "Movement III" or "Presto agitato" and
	// nothing else is headed at all.
	SectionTitle string

	// EndsSection says the music starts afresh after this bar: one movement
	// ends here and another begins.
	//
	// It matters to playing rather than to drawing, because everything that
	// sends a player somewhere else is written as though its own movement
	// were the whole piece. "D.C. al Fine" at the end of a minuet's trio
	// means the beginning of the minuet and the Fine in the middle of it,
	// not the first bar of the sonata and not the end of the evening.
	//
	// Read as one stretch, the Moonlight sonata played its first movement,
	// its second, went back to bar one for the D.C., played as far as the
	// Fine and stopped — with the whole of the third movement unplayed.
	EndsSection bool

	// Sign is the segno, the coda, Fine or To Coda written at this bar: the
	// places the directions below refer to.
	Sign Sign

	// SystemBreak starts a new line of music at this bar.
	//
	// The engraver fits as many bars to a line as will go, which is right
	// until somebody wants otherwise: four bars to a line through a study
	// so each one is read the same way, a page turn moved off a passage
	// nobody can spare a hand for, a system evened out because the last
	// line of a piece holding one bar looks like a mistake. None of that
	// can be worked out from the notes; it is a decision, and this is
	// where it is written down.
	//
	// It belongs to the bar across the whole score, like a rehearsal
	// mark: a line of music is a line on every staff at once.
	SystemBreak bool

	// NumberFrom restarts the bar numbering here, at the number it holds.
	//
	// A movement already restarts the count on its own, and that covers
	// most of what anybody needs. This is the rest: a piece printed in
	// sections numbered separately, a part whose publisher began again at
	// 1 after a long introduction, an exercise book where every study
	// starts afresh. Nothing can infer those — somebody has to say so.
	//
	// A pointer, because restarting at bar one is a thing you can mean and
	// zero cannot otherwise be told from "not set".
	NumberFrom *int

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
	// Sound is what the staff is written for, named the way MusicXML's
	// standard instrument sounds name it: "wind.reed.contrabassoon",
	// "voice.soprano". It says what a program number cannot — General MIDI
	// has one bassoon and no contrabassoon, one choir and no soprano — and
	// it is what the range is asked of. Empty where the file did not say.
	Sound string `json:",omitempty"`
	Clef  Clef
	Key   KeySignature
	Time  TimeSignature
	// Transpose is the interval between this staff's written pitches and what
	// is heard. Everything stored in Measures is written pitch.
	Transpose Transposition

	// Hidden takes the staff off the page and out of what is played. A
	// conductor working on the strings does not want the rest of the
	// orchestra in the way, on the page or in the ear, so a staff that has
	// been put away is silent until it comes back.
	//
	// It is still in the score: the bars keep their length, whatever is
	// written on a hidden staff is still there to come back to, and
	// nothing about the music is changed by looking away from it.
	Hidden bool

	// StaffLines is how many lines the staff is ruled with, where it is not
	// the usual five. Zero means five.
	//
	// One line is the ordinary way to write a drum that has no pitch — a
	// snare, a bass drum, a tam-tam — and there is nothing unusual about
	// it: Ravel's Boléro gives each of its percussion a line of its own,
	// which is five one-line staves down every page of the score. Two,
	// three and four lines are written for a small kit, where the lines
	// stand for the instruments rather than for pitches.
	//
	// What is drawn changes; where the notes go does not. The staff keeps
	// the height and the step arithmetic of a five-line staff and only the
	// middle line is ruled, so a note, a stem, a beam and a rest all land
	// where they always did and everything that measures from the staff
	// goes on working.
	StaffLines int `json:",omitempty"`

	// Tab writes this part as tablature instead of on a staff of pitches:
	// a line for each string, and the fret to stop it at.
	//
	// It is a way of writing the same music, not different music. The notes
	// keep their pitches — that is what sounds, what transposes and what a
	// player of any other instrument reads — and the tab is how they are
	// shown. A part written as tab and the same part written on a staff are
	// the same Measures.
	Tab *Tablature

	// MirrorOf says this staff shows another staff's music, written its
	// own way, and holds none of its own.
	//
	// A guitar part printed as notation above and tablature below is one
	// player reading one line written down twice. Two staves each holding
	// their own copy of it would be two copies to keep in step, and they
	// would not stay in step: a note fixed in one and not the other is
	// the mistake nobody finds until somebody plays from the wrong half.
	//
	// So the lower staff holds nothing. Its Measures are empty and the
	// music is read from the staff it names, exactly as a part is a view
	// of the score rather than a copy of it. Writing aimed at it is sent
	// to the staff that holds the music — see Score.MusicOf.
	//
	// By index into Score.Parts, and a pointer because zero is a staff.
	MirrorOf *int `json:",omitempty"`

	Measures []*Measure
}

// MusicOf is the part whose measures a staff shows: itself, or the staff
// it mirrors.
//
// Everything that reads or writes the music of a staff goes through this,
// so that a tab drawn from the staff above it is drawn from the same notes
// and an edit aimed at it lands on them.
func (s *Score) MusicOf(i int) *Part {
	if s == nil || i < 0 || i >= len(s.Parts) {
		return nil
	}
	p := s.Parts[i]
	if p.MirrorOf == nil {
		return p
	}
	at := *p.MirrorOf
	// A mirror of a mirror, or of nothing, is a mirror of itself: a
	// staff that cannot find its music draws what it has, which is
	// nothing, rather than sending the reader round a loop.
	if at < 0 || at >= len(s.Parts) || at == i || s.Parts[at].MirrorOf != nil {
		return p
	}
	return s.Parts[at]
}

// Mirrors says a staff shows another's music and holds none of its own,
// so that everything which pads or appends bars across the score leaves
// it alone: bars added to it would be bars nobody reads.
func (p *Part) Mirrors() bool { return p != nil && p.MirrorOf != nil }

// MusicIndex is the staff that holds the music for staff i.
func (s *Score) MusicIndex(i int) int {
	if s == nil || i < 0 || i >= len(s.Parts) {
		return i
	}
	if m := s.Parts[i].MirrorOf; m != nil {
		if at := *m; at >= 0 && at < len(s.Parts) && at != i &&
			s.Parts[at].MirrorOf == nil {
			return at
		}
	}
	return i
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
// Reach is how far this staff's instrument plays.
//
// The sound the part names is asked first and the General MIDI program only
// after it: the program is what the staff is played with and the sound is
// what it is. Beethoven's Controfagotto has a bassoon's program, because
// General MIDI has nothing closer, and asking the program gave the bottom of
// a bassoon for an instrument that plays an octave below one.
func (p *Part) Reach() (Reach, bool) {
	if p.Sound != "" {
		if r, ok := SoundReach(p.Sound); ok {
			return r, true
		}
		// A sound gscore knows and has no range for — a section patch, a
		// drum — is an answer, not a gap to fall through.
		if _, known := SoundName(p.Sound); known {
			return Reach{}, false
		}
	}
	return p.Instrument.Reach()
}

// SoundName is what this staff is played on, in as many words as gscore can
// manage: the sound where the file named one, and the program otherwise.
func (p *Part) SoundName() string {
	if p.Sound != "" {
		if n, ok := SoundName(p.Sound); ok {
			return n
		}
	}
	return p.Instrument.String()
}

// Unpitched says the staff is read by position on a kit rather than by
// pitch, either because it is drawn in a percussion clef or because the file
// named a drum.
func (p *Part) Unpitched() bool {
	return p.Clef == Percussion || SoundIsPercussion(p.Sound)
}

func (p *Part) KeyAt(index int) KeySignature {
	k := p.Key
	for i := 0; i <= index && i < len(p.Measures); i++ {
		m := p.Measures[i]
		if m.Key != nil {
			k = *m.Key
		}
		// A change written inside this bar takes effect after it, so it
		// belongs to the bars that follow rather than to this one — the
		// same rule the clef follows, and for the same reason.
		if i == index {
			break
		}
		if last, ok := lastKeyIn(m); ok {
			k = last
		}
	}
	return k
}

// lastKeyIn is the final change of key written inside a bar, if any.
//
// By when it happens rather than by which voice holds it, as the clef's is:
// two lines on one staff can each turn the key, and the one that counts at
// the barline is whichever turned it last.
func lastKeyIn(m *Measure) (KeySignature, bool) {
	var out KeySignature
	var at Duration
	found := false
	for _, v := range m.Voices {
		where := Duration{0, 1}
		for _, el := range v.Elements {
			if mk := marksOf(el); mk != nil && mk.Key != nil {
				if !found || where.Cmp(at) >= 0 {
					out, at, found = *mk.Key, where, true
				}
			}
			where = where.Add(el.Duration())
		}
	}
	return out, found
}

// ClefAt is the clef in force at a bar, following any change written in an
// earlier one — at the head of a bar or inside it.
//
// Inside counts. A hand that drops into the bass partway through a bar is
// still in the bass at the next barline, and a clef read only from the heads
// of bars forgot that at every one: the music after such a change was placed
// against the clef the staff last opened a bar with, which in Danse Macabre
// put the left hand's chords nine to sixteen steps above a staff that is
// eight steps tall, on towers of ledger lines.
func (p *Part) ClefAt(index int) Clef {
	c := p.Clef
	for i := 0; i <= index && i < len(p.Measures); i++ {
		m := p.Measures[i]
		if m.Clef != nil {
			c = *m.Clef
		}
		// A change written inside this bar takes effect after it, so it
		// belongs to the bars that follow rather than to this one.
		if i == index {
			break
		}
		if last, ok := lastClefIn(m); ok {
			c = last
		}
	}
	return c
}

// lastClefIn is the final change of clef written inside a bar, if any.
//
// By when it happens rather than by which voice holds it: two lines on one
// staff can each move the clef, and the one that counts at the barline is
// whichever moved it last.
// Clefs is the clef in force at the head of every bar of a part, worked out
// in one pass down it.
//
// ClefAt answers that for a single bar and walks from the beginning of the
// part to do it, which is the right shape for a question asked once and the
// wrong one for a question asked about every bar in turn. Laying out
// Beethoven's ninth — fifty-three staves of two and a half thousand bars —
// took fifty-five seconds, and more than half of it was spent in ClefAt
// re-reading bars it had already read.
//
// The two agree bar for bar, and a test says so.
func (p *Part) Clefs() []Clef {
	out := make([]Clef, len(p.Measures))
	c := p.Clef
	for i, m := range p.Measures {
		if m.Clef != nil {
			c = *m.Clef
		}
		// A change written inside this bar takes effect after it, so the
		// bar itself keeps the clef it began with.
		out[i] = c
		if last, ok := lastClefIn(m); ok {
			c = last
		}
	}
	return out
}

func lastClefIn(m *Measure) (Clef, bool) {
	var found Clef
	var at Duration
	ok := false
	for _, v := range m.Voices {
		now := Duration{0, 1}
		for _, e := range v.Elements {
			if mk := marksOf(e); mk != nil && mk.Clef != nil {
				if !ok || now.Cmp(at) >= 0 {
					found, at, ok = *mk.Clef, now, true
				}
			}
			now = now.Add(e.Duration())
		}
	}
	return found, ok
}

// TempoAt is the speed in force at a bar, following any change written in an
// earlier one.
//
// It is asked of the score rather than of a part because a tempo change is
// written once, over the top staff, and means the same to everybody in the
// room. Whichever part carries it, it is the piece's.
// Tempos is the tempo in force in every bar of the score, worked out in one
// pass through it.
//
// TempoAt answers for a single bar and walks from the first one across every
// staff to do it, which is the right shape asked once and the wrong one
// asked about every note in turn. Building the performance of Beethoven's
// ninth took thirty-one seconds and twenty-six of them were here, so Play
// looked like a button that did nothing.
//
// The two agree bar for bar, and a test says so.
func (s *Score) Tempos() []Tempo {
	n := 0
	for _, p := range s.Parts {
		if len(p.Measures) > n {
			n = len(p.Measures)
		}
	}
	out := make([]Tempo, n)
	t := s.Tempo
	// Where a gradual change is in force: the bar it started at, and what
	// it is on its way to.
	ramp, rampAt := Tempo{}, -1
	for i := range out {
		// A tempo written at this bar governs this bar, so it is taken
		// before the answer is recorded rather than after.
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].Tempo != nil {
				t = t.changedBy(*p.Measures[i].Tempo)
				if t.ToBPM > 0 && t.OverBars > 0 {
					ramp, rampAt = t, i
				} else {
					rampAt = -1
				}
				break
			}
		}
		out[i] = t
		// A bar inside a ritardando is played at a step along the way.
		if rampAt >= 0 && i >= rampAt {
			out[i] = ramp.steppedTo(i - rampAt)
			if i-rampAt >= ramp.OverBars {
				t = out[i]
				rampAt = -1
			}
		}
	}
	return out
}

// changedBy is this tempo after a mark is written over a bar.
//
// A mark that carries no metronome mark changes what is written and not how
// fast to play: "Andantino mosso" over a bar is a word about the character
// of what follows, and a piece that took it for a speed of nought stopped
// dead there. Other programs write a great many of them — a tempo library
// tends to hold a twin of every word with no playback effect — and so does
// anybody who types a word over a bar.
func (t Tempo) changedBy(c Tempo) Tempo {
	t.Text, t.Parens = c.Text, c.Parens
	if c.BPM > 0 {
		t.BPM, t.Unit = c.BPM, c.Unit
	}
	// A gradual change belongs to the mark that opened it and not to the
	// speed it started from, so it is taken from the new mark whatever it
	// says — including the nought that says this one is not one.
	t.ToBPM, t.OverBars = c.ToBPM, c.OverBars
	return t
}

// steppedTo is this gradual change so many bars in.
//
// Straight-line between the two speeds, a step a bar: at the bar it starts
// on it is still the old speed, and at the bar it arrives on it is the new
// one. The arrival is a bar of its own — a ritardando over two bars is
// slower in the second than in the first and has arrived by the third —
// which is what a player does with it.
func (t Tempo) steppedTo(bars int) Tempo {
	if t.ToBPM <= 0 || t.OverBars <= 0 || bars <= 0 {
		return t
	}
	if bars >= t.OverBars {
		t.BPM, t.ToBPM, t.OverBars = t.ToBPM, 0, 0
		return t
	}
	from, to := float64(t.BPM), float64(t.ToBPM)
	t.BPM = int(from + (to-from)*float64(bars)/float64(t.OverBars) + 0.5)
	return t
}

func (s *Score) TempoAt(index int) Tempo {
	t := s.Tempo
	for i := 0; i <= index; i++ {
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].Tempo != nil {
				t = t.changedBy(*p.Measures[i].Tempo)
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

// ViewState is how a score was last being looked at.
//
// Not music either, and kept for the same reason the cursor is: it is work
// somebody did on this score. Turning the beat marks on to pick apart a
// syncopated entry, or colouring the noteheads for a pupil, is a decision
// about *this* piece, and having to make it again every time the file is
// opened is the kind of small daily friction that makes a program tiring.
//
// Every field is explicit rather than defaulted, because the struct is only
// ever read when Score.View is non-nil — that is, when a file actually said
// something. A file that says nothing leaves the person's own settings
// alone. That is also why MeasureNumbers can be false here and still be
// true by default everywhere else: "off" written down is a choice, and the
// absence of the whole struct is the absence of one.
//
// What is deliberately *not* here: the zoom. How large the music is drawn is
// a fact about the screen in front of you, not about the piece — the same
// score wants one size on a laptop and another on a monitor — so it stays
// with the person, as the interface scale does.
type ViewState struct {
	// Paginated draws pages rather than one continuous strip.
	Paginated bool `json:"paginated,omitempty"`

	// The drawing options from the View menu.
	StaffNames     bool `json:"staffNames,omitempty"`
	MeasureNumbers bool `json:"measureNumbers,omitempty"`
	MultiRests     bool `json:"multiRests,omitempty"`
	TempoMark      bool `json:"tempoMark,omitempty"`
	BeatMarks      bool `json:"beatMarks,omitempty"`

	// ConcertPitch shows the score as it sounds rather than as each player
	// reads it. A view and not a change: what is stored stays as written.
	ConcertPitch bool `json:"concertPitch,omitempty"`

	// ColourNotes draws each notehead in the colour of the note it is, and
	// NoteColours is the palette, by the note's letter: {"C": "#d0312d"}.
	//
	// The palette travels with the score as well as living in the person's
	// own settings, so that a part sent to a pupil arrives in the colours it
	// was prepared in. Where a file names colours, those are the ones drawn;
	// where it does not, the reader's own are.
	ColourNotes bool              `json:"colourNotes,omitempty"`
	NoteColours map[string]string `json:"noteColours,omitempty"`
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

	// Scripts are the programs somebody wrote to make this music, kept
	// with it.
	//
	// A piece built from a process — a row, an isorhythm, a phase — is not
	// fully written down by its notes. The notes are the output; the
	// process is the thought, and a composer who comes back to the piece a
	// year later wants the thought. So the scripts travel in the file.
	//
	// Nothing here ever runs by itself. A score that could execute code on
	// being opened is a document that can attack the person who opens it,
	// which is the mistake Word made and spent a decade undoing. What is
	// kept is text: the console shows it, and a person reads it and
	// decides.
	Scripts []Script

	// Comments are what the people working on the score said to each other
	// about it.
	//
	// Not music and not meant for the page: "Clarinets change to C for this
	// last movement, I've kept them as B flat", written against the bar it
	// is about. An editor's reasoning is the first thing lost when a score
	// moves between programs and the thing nobody can reconstruct — the
	// notes are on the page, and why they are those notes is not.
	Comments []Comment

	// Engraving is what this score wants done differently from the house
	// style: how wide a quarter note's slot is, how heavy a beam is, how
	// far apart the staves sit.
	//
	// Keyed by name and holding only what was changed, rather than a struct
	// of every setting. Three reasons. A file then says what somebody chose
	// and not what the program happened to default to the day it was
	// written, so a later change to a default reaches old scores instead of
	// being frozen into them. A name this version does not know is ignored
	// rather than refused, so a file from a newer gscore still opens. And
	// the model stays out of it: these are the engraver's numbers, named by
	// the engraver — see engrave.Knobs, which is the one list of them and
	// is what the dialog, the endpoint and the layout all read.
	//
	// Not coordinates. Nothing here says where anything goes; they are the
	// measurements the engraver works the places out from, which is why
	// they can live on a score without the model knowing what a page is.
	// Description is a few words about the piece, for whoever is looking
	// at a shelf of them: what it is, who it is for, why it is here.
	//
	// Not the subtitle, which is printed on the page under the title and
	// is part of the music. This is not printed at all — it is what the
	// score says about itself to somebody deciding whether to open it.
	Description string

	// Tags are what this piece is, in the words whoever keeps it would
	// look for it by: "advent", "flute duet", "grade 3", "to learn".
	//
	// In the file rather than in a folder name or a list kept beside it,
	// because a score that is copied, renamed or sent to somebody takes
	// its own description with it and a list does not. A shelf of scores
	// can then be sorted by anything without being told anything: the
	// tags are already in the files.
	//
	// Free words and not a fixed set. Nobody can write down in advance
	// the useful ways to divide up somebody else's music.
	Tags []string

	Engraving map[string]float64

	// Page is the paper this score is printed on: its size, which way up,
	// and its margins. See Page, which has the reasoning.
	//
	// A decision about the document rather than about the music, and it
	// belongs to the score for the same reason the engraving measurements
	// do: asked again at every print it would be answered differently
	// every time, and the file would not remember what the piece looks
	// like.
	Page Page `json:",omitempty"`

	// Cover is a picture printed as the first page of a PDF, and Stamp one
	// printed at the foot of the last page. Both are nil in nearly every
	// score.
	//
	// They belong to the document and not to the music, which is why they
	// sit beside Page rather than in the Identity: a cover is the sheet a
	// publisher wraps the music in, and the printer's block at the foot of
	// the last page — "POPULAR COMMUNITY SONGS, PRICE 25¢ A COPY" on the
	// Ellington — is an advertisement, not a note. Neither is drawn while
	// the music is being written, where they would be in the way.
	Cover *Artwork `json:",omitempty"`
	Stamp *Artwork `json:",omitempty"`

	// Markers is what the rehearsal marks in the sequence are called:
	// letters, numbers, or the number of the bar they stand on.
	//
	// A decision about the whole score and not about any one mark. A
	// conductor calls the places one way through a piece, and half of
	// them lettered and half numbered would be two systems of names for
	// one set of places. The marks written out as themselves are the
	// exception and are left alone — see RehearsalMarks.
	Markers MarkerStyle `json:",omitempty"`

	// Numbering is how often the bar numbers are printed and whether they
	// are boxed.
	//
	// Separate from Engraving, which holds measurements. These are not
	// numbers to be tuned but a decision about what the page says, and
	// they are not floats: a conductor rehearsing a long score wants a
	// number on every bar or every five, and wants them boxed so they are
	// not lost among the music. A song wants one at the head of each line
	// and nothing more.
	Numbering MeasureNumbering

	// View is how this score was last being looked at: which of the drawing
	// options were on, and the colours the noteheads were read in.
	//
	// Not music, and said so — the same way Cursor is not music. It is the
	// state of the desk rather than of the piece, and it is kept for the
	// same reason: somebody who turned the beat marks on to work out a
	// syncopated entry, or coloured the notes for a pupil, has done a piece
	// of work on this score, and closing the file should not throw it away.
	//
	// What it is not is a decision about the printed page. Page, Numbering
	// and Engraving are those, and they live on the score in their own
	// right. This is "how I was looking at it", which is why a score that
	// has never said anything carries nothing here and is drawn however the
	// person's own settings say.
	//
	// A pointer, so that "this file says nothing about the view" and "this
	// file says every option was off" are different answers. They have to
	// be: every score written before this existed says nothing, and reading
	// those as "everything off" would turn the bar numbers off on every
	// score anybody already has.
	View *ViewState `json:",omitempty"`

	// Sheets are the parts: the separate documents this score is printed
	// as, one for each player's stand.
	//
	// Empty in most scores, and that is not a score without parts — it is
	// a score whose part is the score, which is what a piano piece, a song
	// and a lead sheet are. Sheets are made when somebody asks for them.
	Sheets []Sheet
}

// Sheet is one printed document made out of the score: a part.
//
// What a part is, and what it is not. It is not a copy of the music. The
// one thing that goes wrong with parts in every program that has them is
// that the score and the part come to disagree — a note fixed in one and
// not the other, and nobody finds out until the rehearsal. So a sheet
// holds no music at all. It says which staves it prints and how it wants
// them laid out, and the notes it shows are the score's own: there is
// nothing to keep in step because there is only one copy of the music.
//
// What it does hold is everything about a part that is genuinely not the
// score's business. A part is engraved larger, because it is read at a
// stand and not on a conductor's desk. Its page turns fall in different
// places, because it has a quarter of the staves. It counts its bars of
// rest instead of printing them, which a score must never do. None of
// those is a fact about the music, and all of them are facts about the
// page — which is why they are here and not on the measures.
type Sheet struct {
	// Name is what the part is called on its own title page and in the
	// list of them: "Flute", "Violin I", "Piano".
	Name string

	// Staves are the staves it prints, by their index in Score.Parts, in
	// the order they appear on its page.
	//
	// Usually one. Two for a piano or a divisi, and occasionally more for
	// a part a single player reads — a percussionist with four lines.
	Staves []int

	// Engraving is what this part wants differently from the score, by the
	// same names and read the same way: see Score.Engraving, which has the
	// reasoning. Only what was changed, so a part that asks for a wider
	// note slot and nothing else says exactly that.
	Engraving map[string]float64

	// StaffSpace is how big the staff is drawn, in the units the renderer
	// measures in, or zero for the score's own size.
	//
	// The one setting that is wrong by default for every part ever
	// printed. A score is read across a desk at a foot and a half; a part
	// is read across a stand at arm's length, by somebody whose eyes are
	// also on a conductor. Parts are engraved larger for that reason and
	// for no other, and it is not an engraving measurement — nothing about
	// the layout changes, only how large it comes out.
	StaffSpace float64

	// Numbering is how this part's bars are numbered. A part wants numbers
	// more often than a score does: a player who has just counted
	// thirty-two bars of rest needs to be told where they are.
	Numbering MeasureNumbering

	// MultiRests counts runs of empty bars instead of printing them, which
	// is most of what makes a part readable. True for a part made by
	// gscore; it is here rather than assumed because a part of continuous
	// music has nothing to count and somebody may not want it.
	MultiRests bool

	// Breaks are the bars this part begins a system at, by index.
	//
	// Its own, and not the score's. Where the page turns is the whole
	// difference between a part somebody can play and one they cannot: a
	// turn has to fall where there are bars of rest to turn in, and the
	// score's turns fall wherever its own fuller systems ran out of room.
	Breaks []int

	// Cues are the bars of somebody else's line printed small in this part
	// so the player can hear their entry coming.
	Cues []Cue

	// Voicing is which of a shared staff's notes this part prints.
	//
	// Two players on one staff is how wind and brass are written: "Flute
	// 1, 2" is one line of the score, and the notes on it are a chord
	// where they differ and one note where they do not. The score is
	// right as it stands; it is the parts that have to be told apart,
	// because the second flute reads the lower note and nothing else.
	//
	// Empty for nearly every part, and that is the common case: a part
	// with one player on one staff prints the staff.
	Voicing []Voicing `json:",omitempty"`
}

// VoicingPick is which notes of a chord a part takes.
type VoicingPick int

const (
	// VoicingAll prints the staff as it is written.
	VoicingAll VoicingPick = iota
	// VoicingTop and VoicingBottom take one note of each chord: the first
	// player reads the upper line and the second the lower.
	VoicingTop
	VoicingBottom
	// VoicingNth takes named notes counted through the chord, for a staff
	// carrying three or four players.
	VoicingNth
)

func (v VoicingPick) String() string {
	switch v {
	case VoicingTop:
		return "top"
	case VoicingBottom:
		return "bottom"
	case VoicingNth:
		return "nth"
	}
	return "all"
}

// Voicing says which notes of one staff a part prints.
//
// It holds no music, like everything else about a part: it is a rule read
// when the part is drawn, against the score's own notes. A part that had
// been given a copy of the second flute's line would go stale the first
// time somebody edited the staff it came from.
//
// Written against chords and against layers both, because both are how the
// thing is written. Two players sharing a staff in unison and in thirds are
// a single line of chords; two players who are rhythmically apart are two
// layers, with their stems opposed. One score often has both, bar by bar,
// which is why there is a rule for each and not a choice between them.
type Voicing struct {
	// Staff is which staff this rule is about, by index in Score.Parts.
	// A part printing two staves can voice them differently.
	Staff int

	// Pick is which notes of a chord to take, in bars written as one
	// layer.
	Pick VoicingPick `json:",omitempty"`

	// Nth are the notes to take when Pick is VoicingNth, counted from one
	// through the chord: {1, 3} is the first and third.
	Nth []int `json:",omitempty"`

	// FromBottom counts Nth up from the lowest note instead of down from
	// the highest. Scores are written both ways and the player's number
	// is what decides: horns count down, and a string divisi counts up.
	FromBottom bool `json:",omitempty"`

	// Single prints a note that is alone in its bar even where Pick would
	// not have chosen it.
	//
	// This is the a2 rule, and it is the difference between a part that
	// can be played and one that cannot. Where both players play the same
	// note the score writes it once, so a rule asking for the third note
	// of a chord finds no third note and the bar comes out empty — the
	// player sits through the loudest passage in the piece looking at a
	// rest. With this, a lone note belongs to everybody reading the
	// staff.
	Single bool `json:",omitempty"`

	// Layer is which voice to print in bars written with more than one,
	// counted from one. Zero prints them all, which is what a part that
	// is the whole staff wants.
	Layer int `json:",omitempty"`
}

// IsPlain reports a rule that changes nothing, which is the same as having
// no rule at all.
func (v Voicing) IsPlain() bool {
	return v.Pick == VoicingAll && v.Layer == 0
}

// VoicingFor is the rule a sheet has for a staff, and whether it has one.
func (s *Sheet) VoicingFor(staff int) (Voicing, bool) {
	for _, v := range s.Voicing {
		if v.Staff == staff {
			return v, !v.IsPlain()
		}
	}
	return Voicing{Staff: staff}, false
}

// Cue is a few bars of another staff's music, printed small in a part.
//
// What a player does after forty bars of rest is count, and counting goes
// wrong. A cue is the fix every printed part uses: the last few bars before
// the entry carry the line of whoever is playing then, small, with their
// name over it, so there is something to listen for instead of a number to
// hold on to.
//
// It belongs to the part and not to the score, because it is not music: the
// score already has that line on its own staff at full size, and printing
// it twice there would be nonsense. And it holds no notes of its own — only
// which bars of which staff to show — for the same reason a part holds no
// music at all. A cue whose notes were copied would go stale the first time
// somebody edited the line it was copied from, and would be wrong in the
// one place nobody thinks to check.
type Cue struct {
	// At is the bar it begins at and Bars how many of them it covers.
	At, Bars int
	// From is the staff it is taken from, by index in Score.Parts.
	From int
	// Label is what is written over it. Empty means the staff's own name,
	// which is what it almost always should be.
	Label string
}

// MeasureNumbering is how a score's bars are numbered.
type MeasureNumbering struct {
	// Every is how often a number is printed: every nth bar.
	//
	// Zero is the default and does not mean "never". It means "wherever
	// they are wanted", which is the first bar of each system on a page
	// and every bar in the editor's strip — the two places the question
	// "which bar is this" gets asked, and the answers differ. A score
	// that wants something else puts a number here and gets it in both.
	Every int

	// Boxed draws a rectangle round each number, the way a rehearsal mark
	// is boxed.
	Boxed bool
}

// Comment is a note about the music, written by somebody working on it.
//
// It belongs to a place rather than to a note: a bar, or a run of bars, and
// a staff where it is about one player. It is never drawn on the page — a
// comment is a conversation about the score and not a marking in it.
type Comment struct {
	// Text is what was said.
	Text string
	// Author is who said it, as the file gives the name.
	Author string
	// At is the bar it is written against, counted from nought, and Bars
	// how many it covers. A comment about one bar covers one.
	At   int
	Bars int
	// Part is the staff it is about, or -1 where it is about the score.
	Part int
	// When it was written, as a Unix time. Zero where the file does not
	// say.
	When int64
}

// Script is one saved program, with the name its author gave it.
type Script struct {
	Name string
	Body string
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

// Unapply turns a sounding pitch into the one that is written.
//
// The inverse of Apply, and needed wherever a pitch arrives as what is
// heard rather than as what is printed. A tablature's tuning is one such
// place: an open low E on a guitar sounds E2 and is written E3, because a
// guitar sounds an octave below what it reads, and the model keeps what
// is printed.
func (t Transposition) Unapply(p Pitch) Pitch {
	if t.IsZero() {
		return p
	}
	return p.Transpose(-t.Steps(), -t.Semitones())
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

// Sounding returns the concert key a written signature stands for.
//
// The inverse of Written, and needed wherever a part's own signature is all
// there is to go on: an alto saxophone part in three sharps is a piece in C,
// and a staff joining that piece has to be given the key of the music rather
// than a copy of what the saxophonist happens to read.
func (t Transposition) Sounding(k KeySignature) KeySignature {
	if t.IsZero() {
		return k
	}
	out := k
	out.Fifths = k.Fifths + t.Fifths()
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
