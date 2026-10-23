/*
 * json.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package gscore

import (
	"fmt"

	"github.com/stricaud/gscore/internal/model"
)

// The model is written through these types rather than tagged directly.
//
// Element is an interface, so a file has to say which kind each one is, and a
// saved file should not change shape every time a Go field is renamed. Keeping
// the wire format separate is what lets the model be refactored freely while
// old files still open.

type scoreDoc struct {
	Title     string    `json:"title,omitempty"`
	Subtitle  string    `json:"subtitle,omitempty"`
	Composer  string    `json:"composer,omitempty"`
	Copyright string    `json:"copyright,omitempty"`
	Tempo     tempoDoc  `json:"tempo"`
	Parts     []partDoc `json:"parts"`
}

type tempoDoc struct {
	BPM  int    `json:"bpm"`
	Unit string `json:"unit"`
	Text string `json:"text,omitempty"`
}

type partDoc struct {
	ID         string       `json:"id,omitempty"`
	Name       string       `json:"name,omitempty"`
	Instrument string       `json:"instrument,omitempty"`
	Program    int          `json:"program,omitempty"`
	Clef       string       `json:"clef"`
	Fifths     int          `json:"fifths"`
	Mode       string       `json:"mode"`
	Beats      int          `json:"beats"`
	Beat       int          `json:"beatValue"`
	TimeSym    string       `json:"timeSymbol,omitempty"`
	Measures   []measureDoc `json:"measures"`
}

type measureDoc struct {
	Barline string      `json:"barline,omitempty"`
	Voices  [][]elemDoc `json:"voices"`
}

type elemDoc struct {
	Kind string `json:"kind"` // "note", "rest", "chord" or "symbol"
	Dur  string `json:"dur,omitempty"`

	Pitch  *pitchDoc  `json:"pitch,omitempty"`
	Notes  []pitchDoc `json:"notes,omitempty"`
	Acc    string     `json:"accidental,omitempty"`
	Hidden bool       `json:"hidden,omitempty"`
	Marks  *marksDoc  `json:"marks,omitempty"`
	Symbol *symbolRef `json:"symbol,omitempty"`
}

type pitchDoc struct {
	Step   string `json:"step"`
	Octave int    `json:"octave"`
	Alter  int    `json:"alter,omitempty"`
	Acc    string `json:"accidental,omitempty"`
}

type symbolRef struct {
	Name      string  `json:"name"`
	Placement string  `json:"placement,omitempty"`
	OffsetX   float64 `json:"offsetX,omitempty"`
	OffsetY   float64 `json:"offsetY,omitempty"`
}

type marksDoc struct {
	Articulations []string    `json:"articulations,omitempty"`
	Symbols       []symbolRef `json:"symbols,omitempty"`
	TieStart      bool        `json:"tieStart,omitempty"`
	TieStop       bool        `json:"tieStop,omitempty"`
	SlurStart     bool        `json:"slurStart,omitempty"`
	SlurStop      bool        `json:"slurStop,omitempty"`
	Tuplet        *tupletDoc  `json:"tuplet,omitempty"`
	Stem          string      `json:"stem,omitempty"`
	Dynamic       string      `json:"dynamic,omitempty"`
}

type tupletDoc struct {
	Actual int  `json:"actual"`
	Normal int  `json:"normal"`
	Start  bool `json:"start,omitempty"`
	Stop   bool `json:"stop,omitempty"`
}

/*
 * Names. Written out in full so a file reads sensibly and does not depend on
 * the numeric value of a Go constant.
 */

var clefNames = map[model.Clef]string{
	model.Treble: "treble", model.Bass: "bass", model.Alto: "alto",
	model.Tenor: "tenor", model.Percussion: "percussion",
}
var clefValues = invert(clefNames)

var modeNames = map[model.Mode]string{
	model.Major: "major", model.Minor: "minor", model.Dorian: "dorian",
	model.Phrygian: "phrygian", model.Lydian: "lydian",
	model.Mixolydian: "mixolydian", model.Locrian: "locrian",
}
var modeValues = invert(modeNames)

var barNames = map[model.BarlineStyle]string{
	model.BarSingle: "single", model.BarDouble: "double", model.BarFinal: "final",
	model.BarRepeatStart: "repeat-start", model.BarRepeatEnd: "repeat-end",
	model.BarRepeatBoth: "repeat-both", model.BarNone: "none",
}
var barValues = invert(barNames)

var timeSymNames = map[model.TimeSymbol]string{
	model.TimeNumeric: "", model.TimeCommon: "common", model.TimeCut: "cut",
}
var timeSymValues = invert(timeSymNames)

var accNames = map[model.Accidental]string{
	model.NoAccidental: "", model.Natural: "natural", model.Sharp: "sharp",
	model.Flat: "flat", model.DoubleSharp: "double-sharp", model.DoubleFlat: "double-flat",
}
var accValues = invert(accNames)

var articNames = map[model.Articulation]string{
	model.Staccato: "staccato", model.Accent: "accent", model.Tenuto: "tenuto",
	model.Marcato: "marcato", model.Fermata: "fermata",
	model.UpBow: "up-bow", model.DownBow: "down-bow",
}
var articValues = invert(articNames)

var stemNames = map[model.Stem]string{
	model.StemAuto: "", model.StemUp: "up", model.StemDown: "down",
}
var stemValues = invert(stemNames)

var placementNames = map[model.Placement]string{
	model.PlacementAuto: "", model.PlacementAbove: "above", model.PlacementBelow: "below",
}
var placementValues = invert(placementNames)

func invert[K comparable, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func durString(d model.Duration) string {
	r := d.Reduced()
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

func parseDur(s string) (model.Duration, error) {
	var num, den int
	if _, err := fmt.Sscanf(s, "%d/%d", &num, &den); err != nil || den == 0 {
		return model.Duration{}, fmt.Errorf("gscore: bad duration %q", s)
	}
	return model.Duration{Num: num, Den: den}, nil
}

/*
 * Encoding
 */

func encodeScore(s *model.Score) *scoreDoc {
	doc := &scoreDoc{
		Title:     s.Identity.Title,
		Subtitle:  s.Identity.Subtitle,
		Composer:  s.Identity.Composer,
		Copyright: s.Identity.Copyright,
		Tempo: tempoDoc{
			BPM: s.Tempo.BPM, Unit: durString(s.Tempo.Unit), Text: s.Tempo.Text,
		},
	}
	for _, p := range s.Parts {
		pd := partDoc{
			ID: p.ID, Name: p.Name,
			Instrument: p.Instrument.String(),
			Program:    int(p.Instrument),
			Clef:       clefNames[p.Clef],
			Fifths:     p.Key.Fifths,
			Mode:       modeNames[p.Key.Mode],
			Beats:      p.Time.Beats,
			Beat:       p.Time.BeatValue,
			TimeSym:    timeSymNames[p.Time.Symbol],
		}
		for _, m := range p.Measures {
			md := measureDoc{Barline: barNames[m.RightBarline]}
			for _, v := range m.Voices {
				var ed []elemDoc
				for _, el := range v.Elements {
					ed = append(ed, encodeElement(el))
				}
				md.Voices = append(md.Voices, ed)
			}
			pd.Measures = append(pd.Measures, md)
		}
		doc.Parts = append(doc.Parts, pd)
	}
	return doc
}

func encodeElement(el model.Element) elemDoc {
	switch e := el.(type) {
	case *model.Note:
		return elemDoc{
			Kind: "note", Dur: durString(e.Dur),
			Pitch: &pitchDoc{
				Step: e.Pitch.Step.String(), Octave: e.Pitch.Octave, Alter: e.Pitch.Alter,
			},
			Acc:   accNames[e.Accidental],
			Marks: encodeMarks(&e.Marks),
		}
	case *model.Rest:
		return elemDoc{
			Kind: "rest", Dur: durString(e.Dur), Hidden: e.Hidden,
			Marks: encodeMarks(&e.Marks),
		}
	case *model.Chord:
		d := elemDoc{Kind: "chord", Dur: durString(e.Dur), Marks: encodeMarks(&e.Marks)}
		for _, n := range e.Notes {
			d.Notes = append(d.Notes, pitchDoc{
				Step: n.Pitch.Step.String(), Octave: n.Pitch.Octave,
				Alter: n.Pitch.Alter, Acc: accNames[n.Accidental],
			})
		}
		return d
	case *model.SymbolElement:
		return elemDoc{Kind: "symbol", Symbol: &symbolRef{
			Name: e.Symbol.Name, Placement: placementNames[e.Symbol.Placement],
			OffsetX: e.Symbol.OffsetX, OffsetY: e.Symbol.OffsetY,
		}}
	}
	return elemDoc{Kind: "rest", Dur: "0/1"}
}

func encodeMarks(m *model.Marks) *marksDoc {
	if len(m.Articulations) == 0 && len(m.Symbols) == 0 && m.Tuplet == nil &&
		!m.TieStart && !m.TieStop && !m.SlurStart && !m.SlurStop &&
		m.Stem == model.StemAuto && m.Dynamic == model.DynamicNone {
		return nil
	}
	out := &marksDoc{
		TieStart: m.TieStart, TieStop: m.TieStop,
		SlurStart: m.SlurStart, SlurStop: m.SlurStop,
		Stem:    stemNames[m.Stem],
		Dynamic: m.Dynamic.String(),
	}
	for _, a := range m.Articulations {
		out.Articulations = append(out.Articulations, articNames[a])
	}
	for _, s := range m.Symbols {
		out.Symbols = append(out.Symbols, symbolRef{
			Name: s.Name, Placement: placementNames[s.Placement],
			OffsetX: s.OffsetX, OffsetY: s.OffsetY,
		})
	}
	if m.Tuplet != nil {
		out.Tuplet = &tupletDoc{
			Actual: m.Tuplet.Actual, Normal: m.Tuplet.Normal,
			Start: m.Tuplet.Start, Stop: m.Tuplet.Stop,
		}
	}
	return out
}

/*
 * Decoding
 */

func decodeScore(doc *scoreDoc) (*model.Score, error) {
	s := &model.Score{}
	s.Identity = model.Identity{
		Title: doc.Title, Subtitle: doc.Subtitle,
		Composer: doc.Composer, Copyright: doc.Copyright,
	}
	unit := model.Duration{Num: 1, Den: 4}
	if doc.Tempo.Unit != "" {
		if d, err := parseDur(doc.Tempo.Unit); err == nil {
			unit = d
		}
	}
	bpm := doc.Tempo.BPM
	if bpm <= 0 {
		bpm = 120
	}
	s.Tempo = model.Tempo{BPM: bpm, Unit: unit, Text: doc.Tempo.Text}

	for _, pd := range doc.Parts {
		p := &model.Part{
			ID: pd.ID, Name: pd.Name,
			Instrument: model.Instrument(pd.Program),
			Clef:       clefValues[pd.Clef],
			Key:        model.KeySignature{Fifths: pd.Fifths, Mode: modeValues[pd.Mode]},
			Time: model.TimeSignature{
				Beats: pd.Beats, BeatValue: pd.Beat, Symbol: timeSymValues[pd.TimeSym],
			},
		}
		if p.Time.BeatValue == 0 {
			p.Time = model.TimeSignature{Beats: 4, BeatValue: 4}
		}
		for _, md := range pd.Measures {
			m := &model.Measure{RightBarline: barValues[md.Barline]}
			for _, vd := range md.Voices {
				v := &model.Voice{}
				for _, ed := range vd {
					el, err := decodeElement(ed)
					if err != nil {
						return nil, err
					}
					if el != nil {
						v.Elements = append(v.Elements, el)
					}
				}
				m.Voices = append(m.Voices, v)
			}
			if len(m.Voices) == 0 {
				m.Voices = []*model.Voice{{}}
			}
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	if len(s.Parts) == 0 {
		return nil, fmt.Errorf("gscore: the score has no parts")
	}
	return s, nil
}

func decodeElement(ed elemDoc) (model.Element, error) {
	dur := model.Duration{Num: 0, Den: 1}
	if ed.Dur != "" {
		d, err := parseDur(ed.Dur)
		if err != nil {
			return nil, err
		}
		dur = d
	}

	switch ed.Kind {
	case "note":
		if ed.Pitch == nil {
			return nil, fmt.Errorf("gscore: a note with no pitch")
		}
		n := &model.Note{Dur: dur, Pitch: decodePitch(*ed.Pitch), Accidental: accValues[ed.Acc]}
		decodeMarks(ed.Marks, &n.Marks)
		return n, nil
	case "rest":
		r := &model.Rest{Dur: dur, Hidden: ed.Hidden}
		decodeMarks(ed.Marks, &r.Marks)
		return r, nil
	case "chord":
		c := &model.Chord{Dur: dur}
		for _, pd := range ed.Notes {
			c.Notes = append(c.Notes, &model.Note{
				Pitch: decodePitch(pd), Dur: dur, Accidental: accValues[pd.Acc],
			})
		}
		decodeMarks(ed.Marks, &c.Marks)
		return c, nil
	case "symbol":
		if ed.Symbol == nil {
			return nil, nil
		}
		return &model.SymbolElement{Symbol: model.SymbolMark{
			Name: ed.Symbol.Name, Placement: placementValues[ed.Symbol.Placement],
			OffsetX: ed.Symbol.OffsetX, OffsetY: ed.Symbol.OffsetY,
		}}, nil
	}
	return nil, fmt.Errorf("gscore: unknown element kind %q", ed.Kind)
}

func decodePitch(pd pitchDoc) model.Pitch {
	step, _ := model.StepFromLetter(firstByte(pd.Step))
	return model.Pitch{Step: step, Octave: pd.Octave, Alter: pd.Alter}
}

func firstByte(s string) byte {
	if s == "" {
		return 'C'
	}
	return s[0]
}

func decodeMarks(md *marksDoc, m *model.Marks) {
	if md == nil {
		return
	}
	m.TieStart, m.TieStop = md.TieStart, md.TieStop
	m.SlurStart, m.SlurStop = md.SlurStart, md.SlurStop
	m.Stem = stemValues[md.Stem]
	if d, ok := model.DynamicFromName(md.Dynamic); ok {
		m.Dynamic = d
	}
	for _, a := range md.Articulations {
		if v, ok := articValues[a]; ok {
			m.AddArticulation(v)
		}
	}
	for _, s := range md.Symbols {
		m.Symbols = append(m.Symbols, model.SymbolMark{
			Name: s.Name, Placement: placementValues[s.Placement],
			OffsetX: s.OffsetX, OffsetY: s.OffsetY,
		})
	}
	if md.Tuplet != nil && md.Tuplet.Actual > 0 && md.Tuplet.Normal > 0 {
		m.Tuplet = &model.Tuplet{
			Actual: md.Tuplet.Actual, Normal: md.Tuplet.Normal,
			Start: md.Tuplet.Start, Stop: md.Tuplet.Stop,
		}
	}
}
