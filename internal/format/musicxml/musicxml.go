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
	"io"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
	format.RegisterExporter(exporter{})
}

type importer struct{}

func (importer) Name() string         { return "MusicXML" }
func (importer) Extensions() []string { return []string{".musicxml", ".xml", ".mxl"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

type exporter struct{}

func (exporter) Name() string         { return "MusicXML" }
func (exporter) Extensions() []string { return []string{".musicxml", ".xml"} }

func (exporter) Export(w io.Writer, s *model.Score) error { return Export(w, s) }

/*
 * The document, as far as gscore models it.
 */

type scorePartwise struct {
	XMLName  xml.Name  `xml:"score-partwise"`
	Version  string    `xml:"version,attr"`
	Work     *work     `xml:"work"`
	Ident    *identity `xml:"identification"`
	PartList partList  `xml:"part-list"`
	Parts    []xmlPart `xml:"part"`
}

type work struct {
	Title string `xml:"work-title,omitempty"`
}

type identity struct {
	Creators []creator `xml:"creator"`
	Rights   string    `xml:"rights,omitempty"`
	Encoding *encoding `xml:"encoding,omitempty"`
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
	Divisions int   `xml:"divisions,omitempty"`
	Key       *key  `xml:"key,omitempty"`
	Time      *time `xml:"time,omitempty"`
	Clef      *clef `xml:"clef,omitempty"`
}

type key struct {
	Fifths int    `xml:"fifths"`
	Mode   string `xml:"mode,omitempty"`
}

type time struct {
	Symbol string `xml:"symbol,attr,omitempty"`
	Beats  int    `xml:"beats"`
	Type   int    `xml:"beat-type"`
}

type clef struct {
	Sign string `xml:"sign"`
	Line int    `xml:"line"`
}

type direction struct {
	Placement string        `xml:"placement,attr,omitempty"`
	Type      directionType `xml:"direction-type"`
	Sound     *sound        `xml:"sound,omitempty"`
}

type directionType struct {
	Metronome *metronome `xml:"metronome,omitempty"`
	Words     string     `xml:"words,omitempty"`
	Dynamics  *dynamics  `xml:"dynamics,omitempty"`
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
	PerMin   int    `xml:"per-minute"`
}

type sound struct {
	Tempo    float64 `xml:"tempo,attr,omitempty"`
	Dynamics float64 `xml:"dynamics,attr,omitempty"`
}

type empty struct{}

type xmlNote struct {
	Chord      *empty        `xml:"chord,omitempty"`
	Rest       *restElem     `xml:"rest,omitempty"`
	Pitch      *pitch        `xml:"pitch,omitempty"`
	Duration   int           `xml:"duration"`
	Tie        []tieElem     `xml:"tie,omitempty"`
	Voice      int           `xml:"voice,omitempty"`
	Type       string        `xml:"type,omitempty"`
	Dots       []empty       `xml:"dot,omitempty"`
	Accidental string        `xml:"accidental,omitempty"`
	Stem       string        `xml:"stem,omitempty"`
	Beams      []beamElem    `xml:"beam,omitempty"`
	Notations  *notationsTag `xml:"notations,omitempty"`
}

type restElem struct{}

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
	UpBow         *empty `xml:"up-bow,omitempty"`
	DownBow       *empty `xml:"down-bow,omitempty"`
}

type pitch struct {
	Step   string `xml:"step"`
	Alter  *int   `xml:"alter,omitempty"`
	Octave int    `xml:"octave"`
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
	return m
}()
