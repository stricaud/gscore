/*
 * import.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Import reads a MusicXML document, compressed or not.
func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("musicxml: %w", err)
	}
	if isZip(data) {
		data, err = unwrapMXL(data)
		if err != nil {
			return nil, err
		}
	}

	var doc scorePartwise
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("musicxml: %w", err)
	}
	if len(doc.Parts) == 0 {
		return nil, errors.New("musicxml: no parts; score-timewise is not supported")
	}
	return convert(&doc)
}

func isZip(b []byte) bool {
	return len(b) > 4 && b[0] == 'P' && b[1] == 'K' && (b[2] == 3 || b[2] == 5)
}

// unwrapMXL pulls the score out of a compressed .mxl container.
//
// The container names the real document in META-INF/container.xml; when that
// is missing or unreadable, the first .xml or .musicxml file that is not
// container metadata is used instead.
func unwrapMXL(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("musicxml: bad .mxl container: %w", err)
	}

	wanted := ""
	for _, f := range zr.File {
		if f.Name != "META-INF/container.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		var c struct {
			Rootfiles []struct {
				FullPath string `xml:"full-path,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if xml.Unmarshal(b, &c) == nil && len(c.Rootfiles) > 0 {
			wanted = c.Rootfiles[0].FullPath
		}
		break
	}

	for _, f := range zr.File {
		name := f.Name
		if wanted != "" && name != wanted {
			continue
		}
		if wanted == "" {
			if strings.HasPrefix(name, "META-INF/") {
				continue
			}
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".xml") && !strings.HasSuffix(lower, ".musicxml") {
				continue
			}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("musicxml: %w", err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, errors.New("musicxml: no score found inside the .mxl container")
}

func convert(doc *scorePartwise) (*model.Score, error) {
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}
	if doc.Work != nil {
		s.Identity.Title = doc.Work.Title
	}
	if doc.Ident != nil {
		s.Identity.Copyright = doc.Ident.Rights
		for _, c := range doc.Ident.Creators {
			switch c.Type {
			case "composer":
				s.Identity.Composer = c.Name
			case "subtitle":
				s.Identity.Subtitle = c.Name
			}
		}
	}

	names := map[string]string{}
	programs := map[string]model.Instrument{}
	for _, sp := range doc.PartList.ScoreParts {
		names[sp.ID] = sp.Name
		if sp.MidiInst != nil && sp.MidiInst.Program > 0 {
			// Counted from one in the file, from zero in General MIDI.
			programs[sp.ID] = model.Instrument(sp.MidiInst.Program - 1)
		}
	}

	tempoSet := false
	for _, xp := range doc.Parts {
		p := &model.Part{
			ID:         xp.ID,
			Name:       names[xp.ID],
			Instrument: programs[xp.ID],
			Clef:       model.Treble,
			Key:        model.KeySignature{Fifths: 0, Mode: model.Major},
			Time:       model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
		}

		// Divisions carry over from measure to measure until restated.
		divisions := 1
		for _, xm := range xp.Measures {
			for _, it := range xm.Content {
				a := it.Attributes
				if a == nil {
					continue
				}
				if a.Divisions > 0 {
					divisions = a.Divisions
				}
				if a.Key != nil {
					p.Key = model.KeySignature{
						Fifths: a.Key.Fifths,
						Mode:   modeValues[strings.ToLower(a.Key.Mode)],
					}
				}
				if a.Time != nil && a.Time.Type > 0 {
					p.Time = convertTime(a.Time)
				}
				if a.Clef != nil {
					p.Clef = convertClef(a.Clef)
				}
			}

			var barline *barline
			for _, it := range xm.Content {
				if it.Barline != nil {
					barline = it.Barline
				}
				if it.Direction != nil && !tempoSet {
					if t, ok := convertTempo(*it.Direction); ok {
						s.Tempo = t
						tempoSet = true
					}
				}
			}

			m := &model.Measure{
				Voices:       []*model.Voice{{}},
				RightBarline: convertBarline(barline),
			}
			convertMeasure(xm, divisions, m.Voices[0])
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	return s, nil
}

func convertTime(t *time) model.TimeSignature {
	out := model.TimeSignature{Beats: t.Beats, BeatValue: t.Type}
	switch strings.ToLower(t.Symbol) {
	case "common":
		out.Symbol = model.TimeCommon
	case "cut":
		out.Symbol = model.TimeCut
	default:
		out.Symbol = model.TimeNumeric
	}
	return out
}

func convertClef(c *clef) model.Clef {
	switch strings.ToUpper(c.Sign) {
	case "F":
		return model.Bass
	case "C":
		if c.Line == 4 {
			return model.Tenor
		}
		return model.Alto
	case "PERCUSSION":
		return model.Percussion
	}
	return model.Treble
}

func convertTempo(d direction) (model.Tempo, bool) {
	m := d.Type.Metronome
	if m == nil || m.PerMin <= 0 {
		return model.Tempo{}, false
	}
	v, ok := typeValues[m.BeatUnit]
	if !ok {
		v = model.Quarter
	}
	unit := v.Duration()
	if m.Dot != nil {
		unit = unit.Scale(3, 2)
	}
	return model.Tempo{BPM: m.PerMin, Unit: unit, Text: d.Type.Words}, true
}

func convertBarline(b *barline) model.BarlineStyle {
	if b == nil {
		return model.BarSingle
	}
	if b.Repeat != nil {
		if strings.EqualFold(b.Repeat.Direction, "forward") {
			return model.BarRepeatStart
		}
		return model.BarRepeatEnd
	}
	switch b.BarStyle {
	case "light-light":
		return model.BarDouble
	case "light-heavy":
		return model.BarFinal
	case "none":
		return model.BarNone
	}
	return model.BarSingle
}

// convertNotes turns one measure's notes into elements, gathering the runs
// marked <chord/> back into chords.
func convertNotes(notes []xmlNote, divisions int, v *model.Voice) {
	var pending *model.Chord

	flush := func() {
		if pending != nil {
			v.Elements = append(v.Elements, pending)
			pending = nil
		}
	}

	for i := range notes {
		xn := &notes[i]
		dur := model.Duration{Num: xn.Duration, Den: 4 * divisions}.Reduced()
		if xn.Duration == 0 {
			// A grace note has no duration; nothing models it yet, so it is
			// skipped rather than given time it does not have.
			continue
		}

		if xn.Rest != nil {
			flush()
			v.Elements = append(v.Elements, &model.Rest{Dur: dur})
			continue
		}
		if xn.Pitch == nil {
			continue
		}

		n := convertNote(xn, dur)

		if xn.Chord != nil {
			// This note sounds with the one before it.
			if pending != nil {
				pending.Notes = append(pending.Notes, n)
				continue
			}
			if k := len(v.Elements); k > 0 {
				if prev, ok := v.Elements[k-1].(*model.Note); ok {
					v.Elements = v.Elements[:k-1]
					pending = &model.Chord{
						Notes: []*model.Note{prev, n},
						Dur:   prev.Dur,
						Marks: prev.Marks,
					}
					continue
				}
			}
			// Nothing to attach to; treat it as its own note.
		}

		flush()
		v.Elements = append(v.Elements, n)
	}
	flush()
}

// convertMeasure walks a measure in document order, so that each dynamic
// lands on the note that follows it rather than all of them on the first.
func convertMeasure(xm xmlMeasure, divisions int, v *model.Voice) {
	pending := model.DynamicNone
	var notes []xmlNote

	flush := func() {
		if len(notes) == 0 {
			return
		}
		before := len(v.Elements)
		convertNotes(notes, divisions, v)
		if pending != model.DynamicNone {
			attachDynamic(v.Elements[before:], pending)
			pending = model.DynamicNone
		}
		notes = nil
	}

	for _, it := range xm.Content {
		switch {
		case it.Direction != nil && it.Direction.Type.Dynamics != nil:
			// Everything gathered so far belongs to the previous marking.
			flush()
			if d := it.Direction.Type.Dynamics.get(); d != model.DynamicNone {
				pending = d
			}
		case it.Note != nil:
			notes = append(notes, *it.Note)
		}
	}
	flush()
}

// attachDynamic puts a dynamic on the first sounding element of a run.
func attachDynamic(els []model.Element, d model.Dynamic) {
	for _, el := range els {
		switch e := el.(type) {
		case *model.Note:
			e.Dynamic = d
			return
		case *model.Chord:
			e.Dynamic = d
			return
		}
	}
}

func convertNote(xn *xmlNote, dur model.Duration) *model.Note {
	step, _ := model.StepFromLetter(firstByte(xn.Pitch.Step))
	alter := 0
	if xn.Pitch.Alter != nil {
		alter = *xn.Pitch.Alter
	}
	n := &model.Note{
		Pitch: model.Pitch{Step: step, Octave: xn.Pitch.Octave, Alter: alter},
		Dur:   dur,
	}
	if a, ok := accidentalValues[xn.Accidental]; ok {
		n.Accidental = a
	}
	switch xn.Stem {
	case "up":
		n.Stem = model.StemUp
	case "down":
		n.Stem = model.StemDown
	}

	for _, t := range xn.Tie {
		switch t.Type {
		case "start":
			n.TieStart = true
		case "stop":
			n.TieStop = true
		}
	}
	if nt := xn.Notations; nt != nil {
		for _, t := range nt.Tied {
			switch t.Type {
			case "start":
				n.TieStart = true
			case "stop":
				n.TieStop = true
			}
		}
		for _, sl := range nt.Slurs {
			switch sl.Type {
			case "start":
				n.SlurStart = true
			case "stop":
				n.SlurStop = true
			}
		}
		if nt.Fermata != nil {
			n.AddArticulation(model.Fermata)
		}
		if a := nt.Articulations; a != nil {
			if a.Staccato != nil {
				n.AddArticulation(model.Staccato)
			}
			if a.Staccatissimo != nil {
				n.AddArticulation(model.Staccatissimo)
			}
			if a.Accent != nil {
				n.AddArticulation(model.Accent)
			}
			if a.Tenuto != nil {
				n.AddArticulation(model.Tenuto)
			}
			if a.Marcato != nil {
				n.AddArticulation(model.Marcato)
			}
			if a.UpBow != nil {
				n.AddArticulation(model.UpBow)
			}
			if a.DownBow != nil {
				n.AddArticulation(model.DownBow)
			}
		}
	}
	return n
}

func firstByte(s string) byte {
	if s == "" {
		return 'C'
	}
	return s[0]
}
