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
	"math"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Import reads a MusicXML document, compressed or not.
// log collects what a file asked for that gscore could not give it exactly.
//
// A MusicXML file is a conversation between two programs that do not have the
// same ideas, and the gaps matter: a microtone silently rounded to the nearest
// semitone changes the music, and the person who wrote the file is the only
// one who can decide whether that is acceptable. So it is written down and
// handed back with the score rather than either discarded or treated as a
// reason to refuse the whole file.
type log struct {
	seen    map[string]bool
	skipped []string
}

func (l *log) say(format string, args ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if l.seen[msg] {
		return
	}
	l.seen[msg] = true
	l.skipped = append(l.skipped, msg)
}

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
	lg := &log{}
	s, err := convert(&doc, lg)
	if err != nil {
		return nil, err
	}
	if len(lg.skipped) > 0 {
		return s, &PartialError{Skipped: lg.skipped}
	}
	return s, nil
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

func convert(doc *scorePartwise, lg *log) (*model.Score, error) {
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
					p.Time = convertTime(a.Time, lg)
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
			convertMeasure(xm, divisions, m, lg)
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	return s, nil
}

// countBeats reads what stands above the line in a time signature.
//
// Usually a number. Sometimes a sum — "3+2" is a bar of five, counted three
// and two, and writing it that way is the whole point of a composite meter.
// gscore has one number there, so the sum is added up; the grouping is lost
// but the bar is the right length, which is the part the music depends on.
func countBeats(s string, lg *log) int {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	total, any := 0, false
	for _, part := range strings.Split(s, "+") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			total += n
			any = true
		}
	}
	if !any {
		lg.say("a time signature of %q could not be read; four beats assumed", s)
		return 4
	}
	lg.say("the composite time signature %q is counted as %d beats in a bar",
		s, total)
	return total
}

func convertTime(t *time, lg *log) model.TimeSignature {
	out := model.TimeSignature{Beats: countBeats(t.Beats, lg), BeatValue: t.Type}
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
func convertNotes(notes []xmlNote, divisions int, v *model.Voice, lg *log) {
	var pending *model.Chord

	flush := func() {
		if pending != nil {
			v.Elements = append(v.Elements, pending)
			pending = nil
		}
	}

	// Grace notes waiting for the note they lean on. They take no time out
	// of the bar — that is what makes them grace notes — so they cannot go
	// into the voice on their own account and have to be carried until the
	// note they belong to turns up.
	var grace *model.GraceGroup
	takeGrace := func(m *model.Marks) {
		if grace != nil {
			m.Grace = grace
			grace = nil
		}
	}

	for i := range notes {
		xn := &notes[i]
		dur := model.Duration{Num: xn.Duration, Den: 4 * divisions}.Reduced()

		if xn.Grace != nil {
			if xn.Pitch == nil {
				continue // a grace rest is not a thing anybody writes
			}
			if grace == nil {
				grace = &model.GraceGroup{}
			}
			if strings.EqualFold(xn.Grace.Slash, "yes") {
				grace.Slashed = true
			}
			g := convertNote(xn, writtenValue(xn.Type), lg)
			grace.Notes = append(grace.Notes, g)
			continue
		}

		if xn.Duration == 0 {
			// No duration and not marked as a grace note. There is nothing
			// to be done with it: giving it time it does not claim would
			// push the rest of the bar along.
			lg.say("a note with no duration was left out")
			continue
		}

		if xn.Rest != nil {
			flush()
			r := &model.Rest{Dur: dur}
			takeGrace(&r.Marks)
			v.Elements = append(v.Elements, r)
			continue
		}
		if xn.Pitch == nil && xn.Unpitched == nil {
			lg.say("a note with neither a pitch nor a drum was left out")
			continue
		}

		n := convertNote(xn, dur, lg)
		takeGrace(&n.Marks)

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
	if grace != nil {
		// Grace notes at the very end of a bar, or after the note they lean
		// on, which MusicXML allows and gscore has nowhere to put: a grace
		// group hangs off the note in front of it, and there is no note in
		// front of it here.
		lg.say("%d grace note(s) at the end of a bar were left out",
			len(grace.Notes))
	}
}

// convertMeasure walks a measure in document order, so that each dynamic
// lands on the note that follows it rather than all of them on the first.
func convertMeasure(xm xmlMeasure, divisions int, m *model.Measure, lg *log) {
	pending := model.DynamicNone
	var notes []xmlNote
	v := m.Voices[0]

	flush := func() {
		if len(notes) == 0 {
			return
		}
		before := len(v.Elements)
		convertNotes(notes, divisions, v, lg)
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
		case it.Shift != nil:
			flush()
			v = shiftVoice(m, v, *it.Shift, divisions, lg)
		}
	}
	flush()
}

// filled is how far through the bar a voice has been written.
func filled(v *model.Voice) model.Duration {
	total := model.Duration{Num: 0, Den: 1}
	for _, el := range v.Elements {
		total = total.Add(el.Duration())
	}
	return total.Reduced()
}

// shiftVoice honours <backup> and <forward>, and returns the voice the notes
// after it belong to.
//
// Winding back is how MusicXML says "another line of music starts here", so
// it opens a voice. Winding forward is a gap in the voice being written, and
// a gap is a rest — one that is not printed, because the voice underneath is
// already holding that beat and a second rest drawn on top of it would only
// be something else to read.
func shiftVoice(m *model.Measure, v *model.Voice, s shift,
	divisions int, lg *log) *model.Voice {

	by := model.Duration{Num: s.Duration, Den: 4 * divisions}.Reduced()
	if by.IsZero() {
		return v
	}

	if !s.Back {
		v.Elements = append(v.Elements, &model.Rest{Dur: by, Hidden: true})
		return v
	}

	at := filled(v).Sub(by).Reduced()
	if at.Cmp(model.Duration{Num: 0, Den: 1}) < 0 {
		lg.say("a backup reached past the start of its bar; it was read as the start")
		at = model.Duration{Num: 0, Den: 1}
	}
	next := &model.Voice{}
	if !at.IsZero() {
		next.Elements = append(next.Elements, &model.Rest{Dur: at, Hidden: true})
	}
	m.Voices = append(m.Voices, next)
	return next
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

func convertNote(xn *xmlNote, dur model.Duration, lg *log) *model.Note {
	// A drum is written where it is played, not at a pitch: the file says
	// which line of the staff the notehead sits on and nothing about what
	// note it is. That line is a position, so it is kept as one.
	if xn.Pitch == nil && xn.Unpitched != nil {
		step, _ := model.StepFromLetter(firstByte(xn.Unpitched.Step))
		return &model.Note{
			Pitch: model.Pitch{Step: step, Octave: xn.Unpitched.Octave},
			Dur:   dur,
		}
	}
	step, _ := model.StepFromLetter(firstByte(xn.Pitch.Step))
	alter := 0
	if xn.Pitch.Alter != nil {
		// A decimal here is a microtone — -1.5 is a sesquiflat — and gscore
		// writes pitches in whole semitones. Rounding is the only thing it
		// can do, and changing somebody's music without telling them is the
		// one thing it must not do.
		alter = int(math.Round(*xn.Pitch.Alter))
		if *xn.Pitch.Alter != float64(alter) {
			lg.say("a microtone (alter %g) was rounded to %d semitones",
				*xn.Pitch.Alter, alter)
		}
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

// writtenValue is how long a grace note is drawn as.
//
// A grace note carries no <duration>, because it takes no time out of the
// bar, but it is still written as something — an eighth with one flag, a
// sixteenth with two — and that is what decides how it looks.
func writtenValue(kind string) model.Duration {
	if v, ok := typeValues[kind]; ok {
		return v.Duration()
	}
	return model.Eighth.Duration()
}
