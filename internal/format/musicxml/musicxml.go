/*
 * musicxml.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package musicxml reads and writes MusicXML, the interchange format for
// notation.
//
// This is the bridge to MuseScore 4, and to Dorico, Finale and Sibelius with
// it. MuseScore's own .mscz is an internal serialisation that changes between
// releases, so writing it would be a treadmill; MusicXML is a published
// standard that all of them read and write well.
//
// Both the uncompressed .musicxml/.xml form and the zipped .mxl container are
// supported for reading; writing produces the uncompressed form.
package musicxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
	format.RegisterExporter(exporter{})
	format.RegisterExporter(compressedExporter{})
}

type importer struct{}

func (importer) Name() string         { return "MusicXML" }
func (importer) Extensions() []string { return []string{".musicxml", ".xml", ".mxl"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

type exporter struct{}

func (exporter) Name() string         { return "MusicXML" }
func (exporter) Extensions() []string { return []string{".musicxml", ".xml"} }

func (exporter) Export(w io.Writer, s *model.Score) error { return Export(w, s) }

// compressedExporter writes the same document in the zip container the
// specification defines. It is a second exporter rather than a flag on the
// first because an exporter is chosen by the name of the file asked for,
// and is handed a writer without being told what that name was.
type compressedExporter struct{}

func (compressedExporter) Name() string         { return "MusicXML (compressed)" }
func (compressedExporter) Extensions() []string { return []string{".mxl"} }

func (compressedExporter) Export(w io.Writer, s *model.Score) error {
	return ExportMXL(w, s)
}

/*
 * The document, as far as gscore models it.
 */

type scorePartwise struct {
	XMLName xml.Name  `xml:"score-partwise"`
	Version string    `xml:"version,attr"`
	Work    *work     `xml:"work"`
	Ident   *identity `xml:"identification"`
	// What is printed on the first page, as opposed to what the piece is
	// called. A file often says the subtitle and the arranger only here —
	// the specification has nowhere else for either — so a reader that
	// skips credits loses them both.
	Credits  []creditElem `xml:"credit"`
	PartList partList     `xml:"part-list"`
	Parts    []xmlPart    `xml:"part"`
}

type work struct {
	Number string `xml:"work-number,omitempty"`
	Title  string `xml:"work-title,omitempty"`
}

// creditElem is one block of text on the title page, with the type the
// file gives it ("title", "subtitle", "composer", "arranger", "rights").
type creditElem struct {
	Type  []string `xml:"credit-type"`
	Words []string `xml:"credit-words"`
}

// lines is the credit as it is set on the page.
//
// Each <credit-words> is its own line, and a line may carry more of them
// inside it: a credit block naming the composer and the arranger is one
// <credit>, written as two <credit-words> or as one with a newline in it,
// and both mean two lines on the page.
func (c *creditElem) lines() []string {
	var out []string
	for _, w := range c.Words {
		for _, line := range strings.Split(w, "\n") {
			if s := strings.TrimSpace(line); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// text is the whole of a credit on one line, for the parts of it that are
// one thing however they were set — a title, a subtitle.
func (c *creditElem) text() string {
	return strings.Join(c.lines(), " ")
}

type identity struct {
	Creators []creator `xml:"creator"`
	Rights   string    `xml:"rights,omitempty"`
	Encoding *encoding `xml:"encoding,omitempty"`
	// Where an encoder puts what the format has no element for. MuseScore
	// writes the subtitle here.
	Misc []miscField `xml:"miscellaneous>miscellaneous-field"`
}

// miscField is one named scrap under <miscellaneous>.
type miscField struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type creator struct {
	Type string `xml:"type,attr"`
	Name string `xml:",chardata"`
}

type encoding struct {
	Software string `xml:"software,omitempty"`
}

type partList struct {
	ScoreParts []scorePart `xml:"score-part"`
}

type scorePart struct {
	ID       string          `xml:"id,attr"`
	Name     string          `xml:"part-name"`
	MidiInst *midiInstrument `xml:"midi-instrument,omitempty"`
}

// midiInstrument is how MusicXML says what a staff sounds as. The program is
// a General MIDI number, counted from one in the file and from zero
// everywhere else, which is a classic way to be a semitone's worth of wrong.
type midiInstrument struct {
	ID      string `xml:"id,attr"`
	Program int    `xml:"midi-program"`
	Channel int    `xml:"midi-channel,omitempty"`
}

type xmlPart struct {
	ID       string       `xml:"id,attr"`
	Measures []xmlMeasure `xml:"measure"`
}

type attributes struct {
	Divisions int `xml:"divisions,omitempty"`
	// How many staves this part is written on. A piano part is one part on
	// two staves, not two parts: both hands are one instrument, and the
	// file says so here.
	Staves int `xml:"staves,omitempty"`
	// Slices, not one of each: a part writes one signature per staff where
	// its staves differ, and reading only the first gave both hands of a
	// two-key exercise whichever key happened to be written first.
	Keys  []key  `xml:"key,omitempty"`
	Times []time `xml:"time,omitempty"`
	// One per staff, each numbered. A part on a single staff writes one
	// clef with no number at all, which reads as staff one.
	Clefs []clef `xml:"clef,omitempty"`
}

type key struct {
	// Which staff of the part it belongs to. A part may carry a different
	// signature on each staff — a harp writing the pedal setting on one
	// hand, an exercise in two keys at once — and the specification says
	// so by numbering them. No number means the whole part.
	Number int    `xml:"number,attr,omitempty"`
	Fifths int    `xml:"fifths"`
	Mode   string `xml:"mode,omitempty"`
}

type time struct {
	// As for a key: which staff, where a part gives its staves different
	// meters. No number means the whole part.
	Number int    `xml:"number,attr,omitempty"`
	Symbol string `xml:"symbol,attr,omitempty"`
	// A string, not a number, because the specification says so and because
	// files use it: a composite meter is written "3+2", three beats and two
	// in a bar of five. Read as an int it ended the import of the whole
	// file at the first one.
	Beats string `xml:"beats"`
	Type  int    `xml:"beat-type"`
}

type clef struct {
	Number int    `xml:"number,attr,omitempty"`
	Sign   string `xml:"sign"`
	Line   int    `xml:"line"`
}

// staffOf is which staff something belongs to, counted from one. Anything
// that does not say belongs to the first, which is what a part on a single
// staff leaves out everywhere.
func staffOf(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

type direction struct {
	Placement string `xml:"placement,attr,omitempty"`
	// A slice: one <direction> may hold several <direction-type> children
	// and often does. "cresc." and the dashed line that carries it across
	// four bars are two of them under one direction, and a reader that
	// took only the first lost the line — or, where the file wrote the
	// line first, lost the word.
	Types []directionType `xml:"direction-type"`
	Sound *sound          `xml:"sound,omitempty"`
	// Which staff of the part it is written against: a piano's left hand
	// has its own dynamics.
	Staff int `xml:"staff,omitempty"`
}

// A direction's children are searched rather than indexed.
//
// One <direction> may hold several <direction-type>, and which one holds
// what is the file's business: "cresc." and the dashed line under it, a
// metronome mark and the word beside it, a dynamic and the wedge it opens.
// Reaching for a particular one by position is how a reader loses the
// others — and reading the element as a single struct, which is what Go's
// decoder does with repeated elements, keeps only the last of them.

// dynamic is the marking written under this direction, if any.
func (d *direction) dynamic() model.Dynamic {
	for i := range d.Types {
		if dy := d.Types[i].Dynamics; dy != nil {
			if v := dy.get(); v != model.DynamicNone {
				return v
			}
		}
	}
	return model.DynamicNone
}

// metronome is the printed speed, if one is written here.
func (d *direction) metronome() *metronome {
	for i := range d.Types {
		if m := d.Types[i].Metronome; m != nil {
			return m
		}
	}
	return nil
}

// rehearsal is the letter this direction names the bar by.
func (d *direction) rehearsal() string {
	for i := range d.Types {
		if s := strings.TrimSpace(d.Types[i].Rehearsal); s != "" {
			return s
		}
	}
	return ""
}

// wedge is the hairpin this direction opens or closes.
func (d *direction) wedge() *wedgeElem {
	for i := range d.Types {
		if w := d.Types[i].Wedge; w != nil {
			return w
		}
	}
	return nil
}

// octave is the 8va this direction opens or closes.
func (d *direction) octave() *octaveShiftElem {
	for i := range d.Types {
		if o := d.Types[i].OctaveShift; o != nil {
			return o
		}
	}
	return nil
}

// pedal is the pedalling written here.
func (d *direction) pedal() *pedalElem {
	for i := range d.Types {
		if p := d.Types[i].Pedal; p != nil {
			return p
		}
	}
	return nil
}

// dashed says whether a word written here is carried on by a dashed line,
// which is how "cresc." is told from a word said once.
func (d *direction) dashed() bool {
	for i := range d.Types {
		if s := d.Types[i].Dashes; s != nil && strings.EqualFold(s.Type, "start") {
			return true
		}
	}
	return false
}

// words is every word written under one direction, joined. A multi-line
// marking is several <words> in a row — the specification's own
// 31f-Direction-Multiline-Compounds does exactly that — and they are one
// thing to a reader.
func (d *direction) words() string {
	var parts []string
	for _, t := range d.Types {
		for _, w := range t.Words {
			if s := strings.TrimSpace(w.Text); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

type directionType struct {
	Metronome *metronome `xml:"metronome,omitempty"`
	// Several, because a multi-line marking is written as several of
	// them, and because the font of each is its own.
	Words    []wordsElem `xml:"words,omitempty"`
	Dynamics *dynamics   `xml:"dynamics,omitempty"`
	// The hairpin: where a crescendo opens and where it closes.
	Wedge *wedgeElem `xml:"wedge,omitempty"`
	// The dashed line that carries a word on — "cresc. - - - -". It is
	// kept so that the word it continues can be marked as lasting rather
	// than said once.
	Dashes *spanElem `xml:"dashes,omitempty"`
	// 8va and 8vb: the sign that moves a passage an octave and the line
	// under or over it saying how far it reaches.
	OctaveShift *octaveShiftElem `xml:"octave-shift,omitempty"`
	// The sustaining pedal.
	Pedal *pedalElem `xml:"pedal,omitempty"`
	// Rehearsal is the letter a conductor calls a bar by.
	//
	// Its own element and not words, which is the distinction the format
	// is making: a rehearsal mark names a place and a word over the music
	// says something about it, and a reader that treated them alike would
	// put "poco rit." in the list of places to start from. The spec gives
	// it a square enclosure unless it says otherwise, which is what
	// gscore draws, so nothing needs to be written for the box.
	Rehearsal string `xml:"rehearsal,omitempty"`
}

// wordsElem is a line of text written over or under the staff, with
// enough of its appearance to put it back the way it was read.
type wordsElem struct {
	Text   string `xml:",chardata"`
	Style  string `xml:"font-style,attr,omitempty"`
	Weight string `xml:"font-weight,attr,omitempty"`
}

// words is one line of text as a direction-type holds it, or nothing at
// all where there is no text: an empty <words/> is a mark with no mark in
// it, and every reader draws it as a blank.
func words(s string) []wordsElem {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []wordsElem{{Text: s}}
}

// wedgeElem is a hairpin. "crescendo" and "diminuendo" open one and "stop"
// closes it; the number pairs the two ends where more than one is open.
type wedgeElem struct {
	Type   string `xml:"type,attr"`
	Number int    `xml:"number,attr,omitempty"`
}

// spanElem is anything whose only content is a start, a stop and a number
// to pair them by: dashes, and the wavy line of a trill.
type spanElem struct {
	Type   string `xml:"type,attr"`
	Number int    `xml:"number,attr,omitempty"`
}

// octaveShiftElem is 8va, 8vb, 15ma and the rest.
//
// The size is in notes, not octaves: 8 is one octave, 15 two, 22 three.
// "down" on the element means the notes are written an octave higher than
// they sound, which is what 8va over a passage asks for — so the sign a
// reader calls "up an octave" is type="down" in the file.
type octaveShiftElem struct {
	Type   string `xml:"type,attr"`
	Size   int    `xml:"size,attr,omitempty"`
	Number int    `xml:"number,attr,omitempty"`
}

// pedalElem is the sustaining pedal: down at "start", up at "stop", and
// "change" for the flick that clears the harmony without letting go.
type pedalElem struct {
	Type string `xml:"type,attr"`
	// "no" means the bracketed line rather than the word "Ped.".
	Line string `xml:"line,attr,omitempty"`
	Sign string `xml:"sign,attr,omitempty"`
}

// dynamics is a choice of empty elements, one per marking, which is how
// MusicXML spells p, mf and the rest.
type dynamics struct {
	P    *empty `xml:"p,omitempty"`
	PP   *empty `xml:"pp,omitempty"`
	PPP  *empty `xml:"ppp,omitempty"`
	PPPP *empty `xml:"pppp,omitempty"`
	MP   *empty `xml:"mp,omitempty"`
	MF   *empty `xml:"mf,omitempty"`
	F    *empty `xml:"f,omitempty"`
	FF   *empty `xml:"ff,omitempty"`
	FFF  *empty `xml:"fff,omitempty"`
	FFFF *empty `xml:"ffff,omitempty"`
	SF   *empty `xml:"sf,omitempty"`
	SFZ  *empty `xml:"sfz,omitempty"`
	FP   *empty `xml:"fp,omitempty"`
	RFZ  *empty `xml:"rfz,omitempty"`
	N    *empty `xml:"n,omitempty"`
}

// set marks the one element a dynamic corresponds to.
func (d *dynamics) set(v model.Dynamic) bool {
	e := &empty{}
	switch v {
	case model.Piano:
		d.P = e
	case model.PP:
		d.PP = e
	case model.PPP:
		d.PPP = e
	case model.PPPP:
		d.PPPP = e
	case model.MP:
		d.MP = e
	case model.MF:
		d.MF = e
	case model.Forte:
		d.F = e
	case model.FF:
		d.FF = e
	case model.FFF:
		d.FFF = e
	case model.FFFF:
		d.FFFF = e
	case model.SForzato:
		d.SF = e
	case model.SForzando:
		d.SFZ = e
	case model.FortePiano:
		d.FP = e
	case model.Rinforzando:
		d.RFZ = e
	case model.Niente:
		d.N = e
	default:
		return false
	}
	return true
}

// get reads back whichever element is present.
func (d *dynamics) get() model.Dynamic {
	switch {
	case d.PPPP != nil:
		return model.PPPP
	case d.PPP != nil:
		return model.PPP
	case d.PP != nil:
		return model.PP
	case d.P != nil:
		return model.Piano
	case d.MP != nil:
		return model.MP
	case d.MF != nil:
		return model.MF
	case d.FFFF != nil:
		return model.FFFF
	case d.FFF != nil:
		return model.FFF
	case d.FF != nil:
		return model.FF
	case d.F != nil:
		return model.Forte
	case d.SFZ != nil:
		return model.SForzando
	case d.SF != nil:
		return model.SForzato
	case d.FP != nil:
		return model.FortePiano
	case d.RFZ != nil:
		return model.Rinforzando
	case d.N != nil:
		return model.Niente
	}
	return model.DynamicNone
}

type metronome struct {
	BeatUnit string `xml:"beat-unit"`
	Dot      *empty `xml:"beat-unit-dot,omitempty"`

	// Text, not a number, because the standard says so: per-minute is a text
	// element exactly so that a score can be marked "ca. 76", "c. 120" or
	// "92-96" the way an engraver would write it. Read as an int it is a
	// parse error and the whole score is refused over a tempo marking that
	// could perfectly well have been rounded — which is how a Satie with
	// "ca. 76" over the first bar became a file gscore would not open.
	PerMin string `xml:"per-minute"`
}

// bpm is the number in a per-minute marking, or 0 if there is not one.
//
// The first run of digits and nothing cleverer. "ca. 76" is 76; a range like
// "92-96" is 92, the slow end, which is the one to practise at. A marking
// with no figure in it at all — "Allegro", or an empty element — is 0, and
// the caller reads that as no metronome mark, which is what it is.
func (m *metronome) bpm() int {
	n := 0
	for i := 0; i < len(m.PerMin); i++ {
		c := m.PerMin[i]
		if c < '0' || c > '9' {
			if n > 0 {
				break
			}
			continue
		}
		n = n*10 + int(c-'0')
		if n > 9999 {
			break
		}
	}
	return n
}

type sound struct {
	Tempo    float64 `xml:"tempo,attr,omitempty"`
	Dynamics float64 `xml:"dynamics,attr,omitempty"`
}

type empty struct{}

type xmlNote struct {
	// A grace note: written small before the note it leans on, and taking
	// no time out of the bar, which is why it has no <duration>.
	Grace *graceElem `xml:"grace,omitempty"`
	Chord *empty     `xml:"chord,omitempty"`
	Rest  *restElem  `xml:"rest,omitempty"`
	Pitch *pitch     `xml:"pitch,omitempty"`
	// A drum, which has no pitch but still has a line to sit on: the
	// display step and octave say where on the staff to draw it.
	Unpitched *unpitched `xml:"unpitched,omitempty"`
	Duration  int        `xml:"duration"`
	// Which staff of the part it is written on.
	Staff int       `xml:"staff,omitempty"`
	Tie   []tieElem `xml:"tie,omitempty"`
	// Text, by the specification — a voice is named, not numbered, and a
	// file in this suite names one of them "X".
	Voice      string        `xml:"voice,omitempty"`
	Type       string        `xml:"type,omitempty"`
	Dots       []empty       `xml:"dot,omitempty"`
	Accidental string        `xml:"accidental,omitempty"`
	Stem       string        `xml:"stem,omitempty"`
	Beams      []beamElem    `xml:"beam,omitempty"`
	Notations  *notationsTag `xml:"notations,omitempty"`
	// How many of this note are played in the time of how many written
	// ones: three in the time of two, eleven in the time of six. The
	// <duration> above is already the sounding length, so this is not
	// needed to get the timing right — it is needed to know that the
	// group is a tuplet at all, and so to write the figure over it.
	TimeMod *timeModElem `xml:"time-modification,omitempty"`
	// The words sung on this note, one entry per verse.
	Lyrics []lyricElem `xml:"lyric,omitempty"`
	// "no" means the note is there and is not drawn: a tie's far end
	// written out for playback, an editor's suggestion left invisible.
	PrintObject string `xml:"print-object,attr,omitempty"`
	// A cue note is drawn small and is not played. gscore keeps the size
	// and plays it anyway, which is the lesser of the two wrongs.
	Cue *empty `xml:"cue,omitempty"`
}

type timeModElem struct {
	Actual int `xml:"actual-notes"`
	Normal int `xml:"normal-notes"`
}

type unpitched struct {
	Step   string `xml:"display-step"`
	Octave int    `xml:"display-octave"`
}

type restElem struct{}

// graceElem is <grace>, whose slash attribute is the stroke through the stem
// that tells a player to crush the note rather than dwell on it.
type graceElem struct {
	Slash string `xml:"slash,attr,omitempty"`
}

type tieElem struct {
	Type string `xml:"type,attr"`
}

type beamElem struct {
	Number int    `xml:"number,attr"`
	Value  string `xml:",chardata"`
}

type notationsTag struct {
	Tied          []tieElem      `xml:"tied,omitempty"`
	Slurs         []slurElem     `xml:"slur,omitempty"`
	Articulations *articulations `xml:"articulations,omitempty"`
	Fermata       *empty         `xml:"fermata,omitempty"`
	// The ornaments: a trill and the wavy line that carries it, a turn, a
	// mordent, and the strokes of a tremolo.
	Ornaments *ornaments `xml:"ornaments,omitempty"`
	// What the hand does: fingering above all, which is half the point of
	// an exercise book.
	Technical *technical `xml:"technical,omitempty"`
	// The wavy stroke up a chord that says to spread it.
	Arpeggiate *arpeggiateElem `xml:"arpeggiate,omitempty"`
	// Also a spread chord, written the other way up.
	NonArpeggiate *empty `xml:"non-arpeggiate,omitempty"`
	// A slide between two notes: <glissando> draws a wavy line and
	// <slide> a straight one, and they are the same thing to a player.
	Glissando []glissElem `xml:"glissando,omitempty"`
	Slide     []glissElem `xml:"slide,omitempty"`
	// Where the figure over a tuplet begins and ends. A file may leave
	// these out and give only the time-modification, which is why the
	// ends are inferred where they are missing — see convertNotes.
	Tuplets []tupletElem `xml:"tuplet,omitempty"`
}

// tupletElem is the bracket and figure over a tuplet, as against the
// arithmetic of it, which is time-modification.
type tupletElem struct {
	Type string `xml:"type,attr"`
	// "no" leaves the whole thing off the page: the notes keep their
	// tuplet time and nothing is written over them. A measured tremolo is
	// the usual case — six sixteenths in the time of four, written as
	// strokes through a stem and not as a 6.
	PrintObject string `xml:"print-object,attr"`
	// "none" leaves the figure off while keeping the bracket. Anything
	// else, including nothing at all, means the figure is written.
	ShowNumber string `xml:"show-number,attr"`
}

type slurElem struct {
	Type   string `xml:"type,attr"`
	Number int    `xml:"number,attr,omitempty"`
}

type articulations struct {
	Staccato      *empty `xml:"staccato,omitempty"`
	Staccatissimo *empty `xml:"staccatissimo,omitempty"`
	Accent        *empty `xml:"accent,omitempty"`
	Tenuto        *empty `xml:"tenuto,omitempty"`
	Marcato       *empty `xml:"strong-accent,omitempty"`
	// The bowing marks live under <technical> in the specification and
	// not here; they are kept for the files that put them here anyway.
	UpBow   *empty `xml:"up-bow,omitempty"`
	DownBow *empty `xml:"down-bow,omitempty"`
	// Where a player breathes, and the two strokes that cut a phrase off
	// altogether.
	Breath  *breathElem `xml:"breath-mark,omitempty"`
	Caesura *empty      `xml:"caesura,omitempty"`
}

// breathElem is <breath-mark>, whose text says which sign to draw: a comma
// by default, sometimes a tick.
type breathElem struct {
	Value string `xml:",chardata"`
}

// ornaments is what is written over a note rather than done to it.
type ornaments struct {
	Trill    *empty     `xml:"trill-mark,omitempty"`
	WavyLine []spanElem `xml:"wavy-line,omitempty"`
	Turn     *empty     `xml:"turn,omitempty"`
	TurnInv  *empty     `xml:"inverted-turn,omitempty"`
	Mordent  *empty     `xml:"mordent,omitempty"`
	MordInv  *empty     `xml:"inverted-mordent,omitempty"`
	Tremolo  []tremElem `xml:"tremolo,omitempty"`
}

// tremElem is the strokes through a stem. Its text is how many; "single"
// shakes one note, and "start"/"stop" shake between two.
type tremElem struct {
	Type    string `xml:"type,attr,omitempty"`
	Strokes string `xml:",chardata"`
}

// technical is what the hand does. Only fingering is read: the rest —
// frets, bends, harmonics — has nowhere to go in gscore's model, and
// leaving them out of the struct would be the same as reading them badly.
type technical struct {
	Fingering []string `xml:"fingering,omitempty"`
	UpBow     *empty   `xml:"up-bow,omitempty"`
	DownBow   *empty   `xml:"down-bow,omitempty"`
}

// arpeggiateElem is the stroke up a chord. "down" draws the arrow the
// other way, for a chord spread from the top.
type arpeggiateElem struct {
	Direction string `xml:"direction,attr,omitempty"`
}

// glissElem is a slide from one note to another, start and stop, with
// whatever word is written along it ("gliss.", "port.").
type glissElem struct {
	Type   string `xml:"type,attr"`
	Number int    `xml:"number,attr,omitempty"`
	Text   string `xml:",chardata"`
}

// lyricElem is a syllable under a note.
type lyricElem struct {
	// Which line of words it belongs to. Verse two of a hymn is written
	// under verse one, not instead of it.
	Number string `xml:"number,attr,omitempty"`
	Name   string `xml:"name,attr,omitempty"`
	// Whether the syllable begins a word, ends one, or stands alone —
	// which is what decides the hyphen drawn after it.
	Syllabic []string `xml:"syllabic,omitempty"`
	Text     []string `xml:"text,omitempty"`
	// The line drawn under a syllable held over several notes.
	Extend *empty `xml:"extend,omitempty"`
}

// harmonyElem is a chord symbol: the letter written over the staff for a
// player reading from chords rather than from notes.
type harmonyElem struct {
	Root *rootElem `xml:"root,omitempty"`
	Kind kindElem  `xml:"kind"`
	Bass *rootElem `xml:"bass,omitempty"`
}

// rootElem is the letter of a chord symbol, and the bass note under a
// slash chord, which the format spells the same way.
type rootElem struct {
	Step  string  `xml:"root-step"`
	Alter float64 `xml:"root-alter,omitempty"`
	// A bass is written with its own element names and otherwise reads
	// identically.
	BassStep  string  `xml:"bass-step"`
	BassAlter float64 `xml:"bass-alter,omitempty"`
}

// name is the letter, with its sharp or flat.
func (r *rootElem) name() string {
	step, alter := r.Step, r.Alter
	if step == "" {
		step, alter = r.BassStep, r.BassAlter
	}
	if step == "" {
		return ""
	}
	switch {
	case alter >= 1:
		return step + strings.Repeat("#", int(alter))
	case alter <= -1:
		return step + strings.Repeat("b", int(-alter))
	}
	return step
}

// kindElem is what sort of chord: "major", "dominant", "half-diminished".
// The text attribute is how the file wants it printed — "maj7" rather than
// "major-seventh" — and is preferred wherever it is given.
type kindElem struct {
	Text  string `xml:"text,attr,omitempty"`
	Value string `xml:",chardata"`
}

type pitch struct {
	Step string `xml:"step"`
	// Semitones, and a decimal one: the specification defines alter as a
	// decimal so that microtones can be written, and -1.5 is a sesquiflat.
	// gscore writes pitches in whole semitones, so a fraction is rounded
	// and said so in the log rather than throwing the file out.
	Alter  *float64 `xml:"alter,omitempty"`
	Octave int      `xml:"octave"`
}

type barline struct {
	Location string     `xml:"location,attr,omitempty"`
	BarStyle string     `xml:"bar-style,omitempty"`
	Repeat   *repeatTag `xml:"repeat,omitempty"`
}

type repeatTag struct {
	Direction string `xml:"direction,attr"`
}

/*
 * Durations
 *
 * MusicXML counts time in divisions per quarter note. gscore holds exact
 * fractions, so a divisions value is chosen that can express every duration in
 * the score without rounding, rather than a fixed one that silently cannot.
 */

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func lcm(a, b int) int {
	if a == 0 || b == 0 {
		return 1
	}
	return a / gcd(a, b) * b
}

// chooseDivisions finds divisions per quarter note that make every duration in
// the score a whole number.
func chooseDivisions(s *model.Score) int {
	div := 1
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					d := el.Duration().Reduced()
					if d.Num == 0 {
						continue
					}
					// A duration of Num/Den whole notes is Num*4/Den quarters,
					// so the denominator has to divide evenly.
					div = lcm(div, d.Den)
				}
			}
		}
	}
	// Four divisions to the quarter at minimum keeps common music readable to
	// anything that opens the file.
	if div < 4 {
		div = 4
	}
	return div
}

func durationIn(d model.Duration, divisions int) int {
	// divisions are per quarter note; a whole note is four quarters.
	return d.Num * 4 * divisions / d.Den
}

var typeNames = map[model.NoteValue]string{
	model.DoubleWhole: "breve", model.Whole: "whole", model.Half: "half",
	model.Quarter: "quarter", model.Eighth: "eighth", model.Sixteenth: "16th",
	model.ThirtySecond: "32nd", model.SixtyFourth: "64th",
	model.HundredTwentyEighth: "128th",
}

var typeValues = func() map[string]model.NoteValue {
	m := map[string]model.NoteValue{}
	for k, v := range typeNames {
		m[v] = k
	}
	m["long"] = model.DoubleWhole
	return m
}()

var accidentalNames = map[model.Accidental]string{
	model.Sharp: "sharp", model.Flat: "flat", model.Natural: "natural",
	model.DoubleSharp: "double-sharp", model.DoubleFlat: "flat-flat",
}

var accidentalValues = func() map[string]model.Accidental {
	m := map[string]model.Accidental{}
	for k, v := range accidentalNames {
		m[v] = k
	}
	m["sharp-sharp"] = model.DoubleSharp
	return m
}()

var clefSigns = map[model.Clef]clef{
	model.Treble:     {Sign: "G", Line: 2},
	model.Bass:       {Sign: "F", Line: 4},
	model.Alto:       {Sign: "C", Line: 3},
	model.Tenor:      {Sign: "C", Line: 4},
	model.Percussion: {Sign: "percussion", Line: 3},
}

var modeNames = map[model.Mode]string{
	model.Major: "major", model.Minor: "minor", model.Dorian: "dorian",
	model.Phrygian: "phrygian", model.Lydian: "lydian",
	model.Mixolydian: "mixolydian", model.Locrian: "locrian",
}

var modeValues = func() map[string]model.Mode {
	m := map[string]model.Mode{}
	for k, v := range modeNames {
		m[v] = k
	}
	// The two modes with two names. Ionian is the major scale and aeolian
	// the minor one, and a file that uses the older names was read as
	// having no mode at all — so a piece in A aeolian came out in A major
	// and the change into it was not drawn, both signatures being two
	// sharps.
	m["ionian"] = model.Major
	m["aeolian"] = model.Minor
	return m
}()

// PartialError says a file was read but not all of it came through.
//
// The same shape the other importers use: what was read is worth keeping, and
// what was not is listed so the person who wrote the file can see exactly
// which of it gscore did not understand. The interface shows it as a warning.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "musicxml: " + e.Skipped[0]
	}
	return fmt.Sprintf("musicxml: %d things could not be read exactly, "+
		"the first being: %s", len(e.Skipped), e.Skipped[0])
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details lists everything that did not come through.
func (e *PartialError) Details() []string { return e.Skipped }
