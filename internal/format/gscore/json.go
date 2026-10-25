/*
 * json.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	Groups    []groupDoc `json:"groups,omitempty"`
	Title     string     `json:"title,omitempty"`
	Subtitle  string     `json:"subtitle,omitempty"`
	Composer  string     `json:"composer,omitempty"`
	Copyright string     `json:"copyright,omitempty"`
	Tempo     tempoDoc   `json:"tempo"`
	Parts     []partDoc  `json:"parts"`
	Cursor    *cursorDoc `json:"cursor,omitempty"`
}

// Where the writing cursor was left. Not music: it is where somebody was
// working, kept so that opening a piece puts you back where you were.
type cursorDoc struct {
	Part    int `json:"part"`
	Measure int `json:"measure"`
	At      int `json:"at"`
	Step    int `json:"step"`
}

type groupDoc struct {
	First    int    `json:"first"`
	Last     int    `json:"last"`
	Symbol   string `json:"symbol,omitempty"`
	Barlines bool   `json:"barlines,omitempty"`
	Name     string `json:"name,omitempty"`
}

type tempoDoc struct {
	BPM  int    `json:"bpm"`
	Unit string `json:"unit"`
	Text string `json:"text,omitempty"`
}

type partDoc struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Instrument string `json:"instrument,omitempty"`
	Program    int    `json:"program,omitempty"`
	Clef       string `json:"clef"`
	Fifths     int    `json:"fifths"`
	Mode       string `json:"mode"`
	Beats      int    `json:"beats"`
	Beat       int    `json:"beatValue"`
	TimeSym    string `json:"timeSymbol,omitempty"`
	// Transpose is how far below what is printed this staff sounds, written
	// the way MusicXML writes it so the two agree.
	Transpose *transposeDoc `json:"transpose,omitempty"`
	Measures  []measureDoc  `json:"measures"`
}

type transposeDoc struct {
	Diatonic     int `json:"diatonic,omitempty"`
	Chromatic    int `json:"chromatic,omitempty"`
	OctaveChange int `json:"octaveChange,omitempty"`
}

type measureDoc struct {
	Barline string      `json:"barline,omitempty"`
	Voices  [][]elemDoc `json:"voices"`

	// A key or meter written here changes it from this bar on; absent means
	// no change, which is not the same as a change to C major.
	Fifths  *int   `json:"fifths,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Beats   int    `json:"beats,omitempty"`
	Beat    int    `json:"beatValue,omitempty"`
	TimeSym string `json:"timeSymbol,omitempty"`

	// Tempo is a new speed written over this bar, held until another says
	// otherwise. Absent means the bar carries on at whatever it was given.
	Tempo *tempoDoc `json:"tempo,omitempty"`

	// Ending is a volta bracket over this bar.
	Ending *endingDoc `json:"ending,omitempty"`
	// Marker is a rehearsal mark, and Repeat the sign that says to play the
	// bar before again.
	Marker string `json:"marker,omitempty"`
	Repeat int    `json:"repeat,omitempty"`

	// Sign is the segno, coda, Fine or To Coda written at this bar, and Jump
	// the direction written at the end of it. Both are stored by name rather
	// than by number, so that a file keeps its meaning if the order of the
	// constants ever changes.
	Sign string `json:"sign,omitempty"`
	Jump string `json:"jump,omitempty"`
}

type endingDoc struct {
	Numbers []int `json:"numbers"`
	Start   bool  `json:"start,omitempty"`
	Stop    bool  `json:"stop,omitempty"`
	Open    bool  `json:"open,omitempty"`
}

type elemDoc struct {
	Kind string `json:"kind"` // "note", "rest", "chord" or "symbol"
	Dur  string `json:"dur,omitempty"`

	Pitch  *pitchDoc  `json:"pitch,omitempty"`
	Notes  []pitchDoc `json:"notes,omitempty"`
	Acc    string     `json:"accidental,omitempty"`
	Hidden bool       `json:"hidden,omitempty"`
	// Head is the notehead's shape and Percussion the drum it plays, both
	// of which a drum staff needs and neither of which the pitch can say.
	Head       string     `json:"notehead,omitempty"`
	Percussion int        `json:"percussion,omitempty"`
	Marks      *marksDoc  `json:"marks,omitempty"`
	Symbol     *symbolRef `json:"symbol,omitempty"`
}

type pitchDoc struct {
	Step       string `json:"step"`
	Octave     int    `json:"octave"`
	Alter      int    `json:"alter,omitempty"`
	Acc        string `json:"accidental,omitempty"`
	Head       string `json:"notehead,omitempty"`
	Percussion int    `json:"percussion,omitempty"`
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
	Hairpin       *hairpinDoc `json:"hairpin,omitempty"`
	Octave        *octaveDoc  `json:"octave,omitempty"`
	Trill         *trillDoc   `json:"trill,omitempty"`
	Tremolo       *tremoloDoc `json:"tremolo,omitempty"`
	Lyrics        []lyricDoc  `json:"lyrics,omitempty"`
	Beam          string      `json:"beam,omitempty"`
	Chord         *chordDoc   `json:"chord,omitempty"`
	Texts         []textDoc   `json:"texts,omitempty"`
}

type chordDoc struct {
	Root    string    `json:"root,omitempty"`
	Quality string    `json:"quality,omitempty"`
	Bass    string    `json:"bass,omitempty"`
	Text    string    `json:"text,omitempty"`
	Frame   *frameDoc `json:"frame,omitempty"`
}

type frameDoc struct {
	Frets     []int `json:"frets"`
	Fingers   []int `json:"fingers,omitempty"`
	FirstFret int   `json:"firstFret,omitempty"`
	Barre     int   `json:"barre,omitempty"`
	BarreFrom int   `json:"barreFrom,omitempty"`
	BarreTo   int   `json:"barreTo,omitempty"`
}

type textDoc struct {
	Text      string  `json:"text"`
	Placement string  `json:"placement,omitempty"`
	OffsetX   float64 `json:"offsetX,omitempty"`
	OffsetY   float64 `json:"offsetY,omitempty"`
	Italic    bool    `json:"italic,omitempty"`
	Bold      bool    `json:"bold,omitempty"`
}

type hairpinDoc struct {
	Kind  string `json:"kind"` // "crescendo" or "diminuendo"
	Start bool   `json:"start,omitempty"`
	Stop  bool   `json:"stop,omitempty"`
}

type octaveDoc struct {
	Amount int  `json:"amount"` // octaves, signed: 1 is 8va, -1 is 8vb
	Start  bool `json:"start,omitempty"`
	Stop   bool `json:"stop,omitempty"`
}

// A trill is a spanner: "tr" where it begins and a wavy line to where it
// stops. Both ends on one note is a trill on that note alone.
// A tremolo is the slashes and whether it is measured at all, which is all
// there is to one: how fast it goes follows from the number of slashes.
type tremoloDoc struct {
	Strokes int  `json:"strokes,omitempty"`
	Buzz    bool `json:"buzz,omitempty"`
}

type trillDoc struct {
	Start bool `json:"start,omitempty"`
	Stop  bool `json:"stop,omitempty"`
}

type lyricDoc struct {
	Text     string `json:"text"`
	Verse    int    `json:"verse,omitempty"`
	Syllabic string `json:"syllabic,omitempty"` // "", "begin", "middle", "end"
	Extend   bool   `json:"extend,omitempty"`
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
		Cursor:    cursorToDoc(s.Cursor),
		Tempo: tempoDoc{
			BPM: s.Tempo.BPM, Unit: durString(s.Tempo.Unit), Text: s.Tempo.Text,
		},
	}
	for _, g := range s.Groups {
		doc.Groups = append(doc.Groups, groupDoc{
			First: g.First, Last: g.Last, Symbol: g.Symbol.String(),
			Barlines: g.Barlines, Name: g.Name,
		})
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
		if !p.Transpose.IsZero() {
			pd.Transpose = &transposeDoc{
				Diatonic: p.Transpose.Diatonic, Chromatic: p.Transpose.Chromatic,
				OctaveChange: p.Transpose.OctaveChange,
			}
		}
		for _, m := range p.Measures {
			md := measureDoc{Barline: barNames[m.RightBarline]}
			if m.Key != nil {
				f := m.Key.Fifths
				md.Fifths, md.Mode = &f, modeNames[m.Key.Mode]
			}
			if m.Time != nil {
				md.Beats, md.Beat = m.Time.Beats, m.Time.BeatValue
				md.TimeSym = timeSymNames[m.Time.Symbol]
			}
			if m.Tempo != nil {
				md.Tempo = &tempoDoc{
					BPM: m.Tempo.BPM, Unit: durString(m.Tempo.Unit),
					Text: m.Tempo.Text,
				}
			}
			if m.Ending != nil {
				md.Ending = &endingDoc{
					Numbers: m.Ending.Numbers, Start: m.Ending.Start,
					Stop: m.Ending.Stop, Open: m.Ending.Open,
				}
			}
			md.Marker, md.Repeat = m.Marker, m.Repeat
			if m.Sign != model.NoSign {
				md.Sign = m.Sign.String()
			}
			if m.Jump != nil {
				md.Jump = m.Jump.Name()
			}
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
			Acc:        accNames[e.Accidental],
			Head:       headNames[e.Head],
			Percussion: e.Percussion,
			Marks:      encodeMarks(&e.Marks),
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
				Head: headNames[n.Head], Percussion: n.Percussion,
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
		m.Stem == model.StemAuto && m.Dynamic == model.DynamicNone &&
		m.Hairpin == nil && m.Octave == nil && len(m.Lyrics) == 0 &&
		m.Tremolo == nil && m.Trill == nil &&
		m.Beam == model.BeamAuto && m.Chord == nil && len(m.Texts) == 0 {
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
	if m.Hairpin != nil {
		out.Hairpin = &hairpinDoc{
			Kind:  m.Hairpin.Kind.String(),
			Start: m.Hairpin.Start, Stop: m.Hairpin.Stop,
		}
	}
	if m.Octave != nil {
		out.Octave = &octaveDoc{
			Amount: m.Octave.Amount, Start: m.Octave.Start, Stop: m.Octave.Stop,
		}
	}
	if m.Trill != nil {
		out.Trill = &trillDoc{Start: m.Trill.Start, Stop: m.Trill.Stop}
	}
	if m.Tremolo != nil {
		out.Tremolo = &tremoloDoc{
			Strokes: m.Tremolo.Strokes, Buzz: m.Tremolo.Buzz,
		}
	}
	for _, ly := range m.Lyrics {
		out.Lyrics = append(out.Lyrics, lyricDoc{
			Text: ly.Text, Verse: ly.Verse,
			Syllabic: syllabicNames[ly.Syllabic], Extend: ly.Extend,
		})
	}
	if m.Beam != model.BeamAuto {
		out.Beam = m.Beam.String()
	}
	if c := m.Chord; c != nil {
		cd := &chordDoc{Root: c.Root, Quality: c.Quality, Bass: c.Bass, Text: c.Text}
		if f := c.Frame; f != nil {
			cd.Frame = &frameDoc{
				Frets: f.Frets, Fingers: f.Fingers, FirstFret: f.FirstFret,
				Barre: f.Barre, BarreFrom: f.BarreFrom, BarreTo: f.BarreTo,
			}
		}
		out.Chord = cd
	}
	for _, t := range m.Texts {
		out.Texts = append(out.Texts, textDoc{
			Text: t.Text, Placement: placementNames[t.Placement],
			OffsetX: t.OffsetX, OffsetY: t.OffsetY,
			Italic: t.Italic, Bold: t.Bold,
		})
	}
	return out
}

var headNames = map[model.NoteheadKind]string{
	model.HeadNormal: "", model.HeadX: "x", model.HeadCircleX: "circle-x",
	model.HeadTriangle: "triangle", model.HeadDiamond: "diamond",
	model.HeadSlash: "slash",
}
var headValues = invert(headNames)

var groupSymbolNames = map[model.GroupSymbol]string{
	model.GroupBracket: "bracket", model.GroupBrace: "brace",
	model.GroupLine: "line",
}
var groupSymbolValues = invert(groupSymbolNames)

var syllabicNames = map[model.Syllabic]string{
	model.SyllableSingle: "", model.SyllableBegin: "begin",
	model.SyllableMiddle: "middle", model.SyllableEnd: "end",
}
var syllabicValues = invert(syllabicNames)

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
	if c := doc.Cursor; c != nil {
		s.Cursor = &model.Cursor{
			Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step,
		}
	}

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
		if t := pd.Transpose; t != nil {
			p.Transpose = model.Transposition{
				Diatonic: t.Diatonic, Chromatic: t.Chromatic,
				OctaveChange: t.OctaveChange,
			}
		}
		for _, md := range pd.Measures {
			m := &model.Measure{RightBarline: barValues[md.Barline]}
			if md.Fifths != nil {
				m.Key = &model.KeySignature{Fifths: *md.Fifths, Mode: modeValues[md.Mode]}
			}
			if md.Beat > 0 {
				m.Time = &model.TimeSignature{
					Beats: md.Beats, BeatValue: md.Beat,
					Symbol: timeSymValues[md.TimeSym],
				}
			}
			if td := md.Tempo; td != nil {
				unit := model.Duration{Num: 1, Den: 4}
				if td.Unit != "" {
					if d, err := parseDur(td.Unit); err == nil {
						unit = d
					}
				}
				m.Tempo = &model.Tempo{BPM: td.BPM, Unit: unit, Text: td.Text}
			}
			if md.Ending != nil {
				m.Ending = &model.Ending{
					Numbers: md.Ending.Numbers, Start: md.Ending.Start,
					Stop: md.Ending.Stop, Open: md.Ending.Open,
				}
			}
			m.Marker, m.Repeat = md.Marker, md.Repeat
			if sign, ok := model.SignFromName(md.Sign); ok {
				m.Sign = sign
			}
			if jump, ok := model.JumpFromName(md.Jump); ok {
				m.Jump = jump
			}
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
	for _, gd := range doc.Groups {
		if gd.First < 0 || gd.Last >= len(s.Parts) || gd.Last <= gd.First {
			continue // a group naming staves that are not there is dropped
		}
		s.Groups = append(s.Groups, model.StaffGroup{
			First: gd.First, Last: gd.Last,
			Symbol:   groupSymbolValues[gd.Symbol],
			Barlines: gd.Barlines, Name: gd.Name,
		})
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
		n := &model.Note{
			Dur: dur, Pitch: decodePitch(*ed.Pitch),
			Accidental: accValues[ed.Acc],
			Head:       headValues[ed.Head], Percussion: ed.Percussion,
		}
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
				Head: headValues[pd.Head], Percussion: pd.Percussion,
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
	if h := md.Hairpin; h != nil {
		kind := model.Crescendo
		if h.Kind == "diminuendo" {
			kind = model.Diminuendo
		}
		m.Hairpin = &model.Hairpin{Kind: kind, Start: h.Start, Stop: h.Stop}
	}
	if o := md.Octave; o != nil && o.Amount != 0 {
		m.Octave = &model.OctaveShift{Amount: o.Amount, Start: o.Start, Stop: o.Stop}
	}
	if tm := md.Tremolo; tm != nil {
		m.Tremolo = &model.Tremolo{Strokes: tm.Strokes, Buzz: tm.Buzz}
	}
	if tr := md.Trill; tr != nil {
		m.Trill = &model.Trill{Start: tr.Start, Stop: tr.Stop}
	}
	if b, ok := model.BeamFromName(md.Beam); ok {
		m.Beam = b
	}
	if cd := md.Chord; cd != nil {
		c := &model.ChordSymbol{
			Root: cd.Root, Quality: cd.Quality, Bass: cd.Bass, Text: cd.Text,
		}
		if f := cd.Frame; f != nil {
			c.Frame = &model.ChordFrame{
				Frets: f.Frets, Fingers: f.Fingers, FirstFret: f.FirstFret,
				Barre: f.Barre, BarreFrom: f.BarreFrom, BarreTo: f.BarreTo,
			}
		}
		m.Chord = c
	}
	for _, t := range md.Texts {
		m.Texts = append(m.Texts, model.TextMark{
			Text: t.Text, Placement: placementValues[t.Placement],
			OffsetX: t.OffsetX, OffsetY: t.OffsetY,
			Italic: t.Italic, Bold: t.Bold,
		})
	}
	for _, ly := range md.Lyrics {
		verse := ly.Verse
		if verse < 1 {
			verse = 1
		}
		m.Lyrics = append(m.Lyrics, model.Lyric{
			Text: ly.Text, Verse: verse,
			Syllabic: syllabicValues[ly.Syllabic], Extend: ly.Extend,
		})
	}
}

func cursorToDoc(c *model.Cursor) *cursorDoc {
	if c == nil {
		return nil
	}
	return &cursorDoc{Part: c.Part, Measure: c.Measure, At: c.At, Step: c.Step}
}
