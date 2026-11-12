/*
 * export.go
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
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/version"
)

const docType = `<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">`

// Export writes a score as MusicXML.
func Export(w io.Writer, s *model.Score) error {
	doc := build(s)

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	if _, err := io.WriteString(w, docType+"\n"); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("musicxml: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func build(s *model.Score) *scorePartwise {
	divisions := chooseDivisions(s)

	doc := &scorePartwise{Version: "4.0"}
	if s.Identity.Title != "" {
		doc.Work = &work{Title: s.Identity.Title}
	}
	doc.Ident = &identity{
		Encoding: &encoding{Software: "gscore " + version.Version},
	}
	if s.Identity.Composer != "" {
		doc.Ident.Creators = append(doc.Ident.Creators,
			creator{Type: "composer", Name: s.Identity.Composer})
	}
	// The arranger is a creator of its own kind, which is the whole reason
	// for keeping it apart from the composer: written into the composer's
	// name it comes back as part of it, and a catalogue looking for the
	// arranger's work finds nothing.
	if s.Identity.Arranger != "" {
		doc.Ident.Creators = append(doc.Ident.Creators,
			creator{Type: "arranger", Name: s.Identity.Arranger})
	}
	// The subtitle, where the format has somewhere for it.
	//
	// Not as a creator: a subtitle is not a person, "subtitle" is not one
	// of the kinds of creator the specification lists, and nothing else
	// reading the file would have found it there — gscore included, which
	// is how a file gscore wrote came back without the subtitle it went
	// out with. It belongs in the credits, which are what is printed on
	// the first page, and in <work-number>, which is where a catalogue
	// number goes and is what a subtitle most often holds.
	if s.Identity.Subtitle != "" {
		doc.Credits = append(doc.Credits, creditElem{
			Type:  []string{"subtitle"},
			Words: []string{s.Identity.Subtitle},
		})
		if doc.Work == nil {
			doc.Work = &work{}
		}
		doc.Work.Number = s.Identity.Subtitle
	}
	if s.Identity.Composer != "" {
		doc.Credits = append(doc.Credits, creditElem{
			Type:  []string{"composer"},
			Words: strings.Split(s.Identity.Composer, "\n"),
		})
	}
	if s.Identity.Arranger != "" {
		doc.Credits = append(doc.Credits, creditElem{
			Type:  []string{"arranger"},
			Words: strings.Split(s.Identity.Arranger, "\n"),
		})
	}
	doc.Ident.Rights = s.Identity.Copyright

	for pi, p := range s.Parts {
		id := partID(pi)
		name := p.Name
		if name == "" {
			name = "Part " + strconv.Itoa(pi+1)
		}
		sp := scorePart{ID: id, Name: name}
		// MusicXML counts programs from one.
		sp.MidiInst = &midiInstrument{
			ID: id + "-I1", Program: int(p.Instrument) + 1, Channel: pi + 1,
		}
		doc.PartList.ScoreParts = append(doc.PartList.ScoreParts, sp)
		doc.Parts = append(doc.Parts, buildPart(s, p, id, pi, divisions))
	}
	return doc
}

func partID(i int) string { return "P" + strconv.Itoa(i+1) }

func buildPart(s *model.Score, p *model.Part, id string, index, divisions int) xmlPart {
	out := xmlPart{ID: id}

	// A repeat that opens is written on the left of the bar it opens, so the
	// style of the bar before decides it. Carried forward rather than looked
	// back at, because it has to be the first thing in the measure.
	var opening *barline

	// The rehearsal marks, with the sequence worked out: a mark is kept as
	// a place in the sequence and the letter is derived, so the letter is
	// what has to be written into the file.
	//
	// Counted by position, with no bar numbers handed in. That is what the
	// rest of this file does too — every <measure number> below is its
	// place — so a score whose marks are named after their bars comes out
	// agreeing with its own numbering. The engraver knows better, about
	// upbeats and about a numbering a file stated for itself, and it is
	// the engraver's business: it is about the page and this is about the
	// file.
	marks := s.RehearsalMarks(nil)

	for mi, m := range p.Measures {
		xm := xmlMeasure{Number: strconv.Itoa(mi + 1)}

		if opening != nil {
			xm.add(measureItem{Barline: opening})
			opening = nil
		}

		// The clef, key and meter: stated in full on the first measure,
		// and again wherever the music changes one of them.
		//
		// They used to be written once and never again, which was the
		// reading of them said back: a piece that changes key or meter
		// halfway through came out of gscore having changed neither, and
		// a file of three movements was written as one.
		switch {
		case mi == 0:
			c := clefSigns[p.Clef]
			xm.add(measureItem{Attributes: &attributes{
				Divisions: divisions,
				Keys:      []key{{Fifths: p.Key.Fifths, Mode: modeNames[p.Key.Mode]}},
				Times:     []time{*buildTime(p.Time)},
				Clefs:     []clef{c},
			}})
			if index == 0 && s.Tempo.BPM > 0 {
				d := buildTempo(s.Tempo)
				xm.add(measureItem{Direction: &d})
			}
		case m.Key != nil || m.Time != nil || m.Clef != nil:
			a := &attributes{}
			if m.Key != nil {
				a.Keys = []key{{
					Fifths: m.Key.Fifths, Mode: modeNames[m.Key.Mode],
				}}
			}
			if m.Time != nil {
				a.Times = []time{*buildTime(*m.Time)}
			}
			if m.Clef != nil {
				a.Clefs = []clef{clefSigns[*m.Clef]}
			}
			xm.add(measureItem{Attributes: a})
		}

		// And a change of speed, on the bar it is written over. Like the
		// mark at the head of the score it belongs to the piece, so it is
		// written once, on the first part.
		if mi > 0 && index == 0 {
			if t := s.TempoChangeAt(mi); t != nil {
				d := buildTempo(*t)
				xm.add(measureItem{Direction: &d})
			}
		}

		// The rehearsal mark, written once on the top part.
		//
		// gscore keeps it on every staff of the bar, because that is what
		// it means; the file writes it once, because that is what every
		// program reading one expects and because writing it on all
		// twenty staves of an orchestral score would have every reader
		// print twenty letters. It is first in the bar, before the notes,
		// since it names the bar rather than anything in it.
		if text := marks[mi]; index == 0 && text != "" {
			d := direction{
				Placement: "above",
				Types:     []directionType{{Rehearsal: text}},
			}
			xm.add(measureItem{Direction: &d})
		}

		if len(m.Voices) > 0 {
			for _, el := range m.Voices[0].Elements {
				// The words written on a note go before it, and before its
				// dynamic: "più p" is a word in front of the letters, and
				// the order the directions are written in is the order a
				// reader prints them in.
				for _, t := range textsOf(el) {
					if dir, ok := buildWords(t); ok {
						xm.add(measureItem{Direction: &dir})
					}
				}
				// A dynamic is a direction, written immediately before the
				// note it applies to. The order is the meaning.
				if d := dynamicOf(el); d != model.DynamicNone {
					dir := buildDynamic(d)
					xm.add(measureItem{Direction: &dir})
				}
				for _, n := range buildElement(el, divisions) {
					note := n
					xm.add(measureItem{Note: &note})
				}
			}
		}
		if b := buildBarline(m.RightBarline); b != nil {
			xm.add(measureItem{Barline: b})
		}
		// And what it opens goes at the head of the next one.
		opening = openingBarline(m.RightBarline)
		out.Measures = append(out.Measures, xm)
	}
	return out
}

// dynamicOf reports the dynamic written on an element, if any.
func dynamicOf(el model.Element) model.Dynamic {
	switch e := el.(type) {
	case *model.Note:
		return e.Dynamic
	case *model.Chord:
		return e.Dynamic
	case *model.Rest:
		return e.Dynamic
	}
	return model.DynamicNone
}

// textsOf reports the words written on an element, if any.
func textsOf(el model.Element) []model.TextMark {
	switch e := el.(type) {
	case *model.Note:
		return e.Texts
	case *model.Chord:
		return e.Texts
	case *model.Rest:
		return e.Texts
	}
	return nil
}

// buildWords writes a word over or under a note: "dolce", "pizz.", the
// "più" in front of a p.
//
// These were read and never written. A file opened in gscore and saved back
// out as MusicXML came out with every word on every note missing — which on
// a Dorico score is most of what is printed over the staves — and nothing
// said so.
//
// MusicXML has no names for the kinds of writing gscore keeps, so what goes
// across is what the words look like: which side of the staff they sit on,
// and whether they are slanted or heavy. That is also what comes back:
// convertWords reads the side off the direction and the face off the font,
// so a word keeps its place and its appearance through a round trip even
// where the kind it was has no way to be said.
func buildWords(t model.TextMark) (direction, bool) {
	if strings.TrimSpace(t.Text) == "" {
		return direction{}, false
	}
	look := t.Style.Look()
	// Where the mark itself asks for, and otherwise where its kind puts it.
	place := t.Placement
	if place == model.PlacementAuto {
		place = look.Place
	}
	side := "above"
	if place == model.PlacementBelow {
		side = "below"
	}
	w := wordsElem{Text: t.Text}
	// Written down rather than left to the kind, because the kind is not
	// going across: a word that was italic because it was an expression
	// has to say it is italic or it comes back upright.
	if t.Italic || look.Italic {
		w.Style = "italic"
	}
	if t.Bold || look.Bold {
		w.Weight = "bold"
	}
	return direction{
		Placement: side,
		Types:     []directionType{{Words: []wordsElem{w}}},
	}, true
}

func buildDynamic(d model.Dynamic) direction {
	dyn := &dynamics{}
	dyn.set(d)
	return direction{
		Placement: "below",
		Types:     []directionType{{Dynamics: dyn}},
		Sound:     &sound{Dynamics: float64(d.Velocity()) * 100 / 90},
	}
}

func buildTime(t model.TimeSignature) *time {
	out := &time{Beats: strconv.Itoa(t.Beats), Type: t.BeatValue}
	switch t.Symbol {
	case model.TimeCommon:
		out.Symbol = "common"
	case model.TimeCut:
		out.Symbol = "cut"
	}
	return out
}

func buildTempo(t model.Tempo) direction {
	unit, dots, ok := t.Unit.Written()
	name := "quarter"
	if ok {
		if n, found := typeNames[unit]; found {
			name = n
		}
	}
	m := &metronome{BeatUnit: name, PerMin: strconv.Itoa(t.BPM)}
	if dots > 0 {
		m.Dot = &empty{}
	}
	// A sound element carries the tempo in quarter notes per minute, which is
	// what a player reads; the metronome mark is what gets printed.
	qpm := float64(t.BPM) * t.Unit.Float() * 4
	dt := directionType{Metronome: m, Words: words(t.Text)}
	// A speed that is played and not printed: the word stands over the
	// bar and the number is only in the sound. Writing the metronome mark
	// anyway would put a crotchet and a number on a page that had none,
	// and the next read would believe it. See model.Tempo.Silent.
	if t.Silent {
		dt.Metronome = nil
	}
	return direction{
		Placement: "above",
		Types:     []directionType{dt},
		Sound:     &sound{Tempo: qpm},
	}
}

func buildElement(el model.Element, divisions int) []xmlNote {
	switch e := el.(type) {
	case *model.Note:
		return []xmlNote{buildNote(e, divisions, false)}
	case *model.Rest:
		return []xmlNote{buildRest(e, divisions)}
	case *model.Chord:
		out := make([]xmlNote, 0, len(e.Notes))
		for i, n := range e.Notes {
			// Every note of a chord after the first carries <chord/>, which is
			// how MusicXML says "sounds with the previous note".
			note := buildNote(n, divisions, i > 0)
			note.Duration = durationIn(e.Dur, divisions)
			applyMarks(&note, &e.Marks)
			out = append(out, note)
		}
		return out
	}
	return nil
}

func buildNote(n *model.Note, divisions int, inChord bool) xmlNote {
	out := xmlNote{
		Duration: durationIn(n.Dur, divisions),
		Voice:    "1",
		Pitch: &pitch{
			Step:   n.Pitch.Step.String(),
			Octave: n.Pitch.Octave,
		},
	}
	if n.Pitch.Alter != 0 {
		a := float64(n.Pitch.Alter)
		out.Pitch.Alter = &a
	}
	if inChord {
		out.Chord = &empty{}
	}
	if v, dots, ok := n.Dur.Written(); ok {
		out.Type = typeNames[v]
		out.Dots = make([]empty, dots)
	}
	if name, ok := accidentalNames[n.Accidental]; ok {
		out.Accidental = name
	}
	switch n.Stem {
	case model.StemUp:
		out.Stem = "up"
	case model.StemDown:
		out.Stem = "down"
	}
	applyMarks(&out, &n.Marks)
	return out
}

func buildRest(r *model.Rest, divisions int) xmlNote {
	out := xmlNote{
		Rest:     &restElem{},
		Duration: durationIn(r.Dur, divisions),
		Voice:    "1",
	}
	if v, dots, ok := r.Dur.Written(); ok {
		out.Type = typeNames[v]
		out.Dots = make([]empty, dots)
	}
	return out
}

func applyMarks(out *xmlNote, m *model.Marks) {
	// A tie is both a sounding instruction and a printed curve, and MusicXML
	// keeps those separate: <tie> for the sound, <tied> for the notation.
	var notations notationsTag
	used := false

	if m.TieStop {
		out.Tie = append(out.Tie, tieElem{Type: "stop"})
		notations.Tied = append(notations.Tied, tieElem{Type: "stop"})
		used = true
	}
	if m.TieStart {
		out.Tie = append(out.Tie, tieElem{Type: "start"})
		notations.Tied = append(notations.Tied, tieElem{Type: "start"})
		used = true
	}
	if m.SlurStop > 0 {
		notations.Slurs = append(notations.Slurs, slurElem{Type: "stop", Number: 1})
		used = true
	}
	if m.SlurStart > 0 {
		notations.Slurs = append(notations.Slurs, slurElem{Type: "start", Number: 1})
		used = true
	}

	var arts articulations
	anyArt := false
	for _, a := range m.Articulations {
		switch a {
		case model.Staccato:
			arts.Staccato = &empty{}
			anyArt = true
		case model.Staccatissimo:
			arts.Staccatissimo = &empty{}
			anyArt = true
		case model.Accent:
			arts.Accent = &empty{}
			anyArt = true
		case model.Tenuto:
			arts.Tenuto = &empty{}
			anyArt = true
		case model.Marcato, model.MarcatoDown:
			arts.Marcato = &empty{}
			anyArt = true
		case model.UpBow:
			arts.UpBow = &empty{}
			anyArt = true
		case model.DownBow:
			arts.DownBow = &empty{}
			anyArt = true
		case model.Fermata:
			notations.Fermata = &empty{}
			used = true
		}
	}
	if anyArt {
		notations.Articulations = &arts
		used = true
	}
	if used {
		out.Notations = &notations
	}
}

// buildBarline is what closes the measure it is written on.
//
// A barline in MusicXML belongs to a measure and to one side of it, and the
// repeats are the awkward case: the one closing a passage is written on the
// right of the bar it ends, and the one opening a passage on the *left of the
// bar it opens*, which is the next measure along. So a style that opens a
// repeat produces nothing here and comes back from openingBarline instead, to
// be written at the head of the measure after this one.
func buildBarline(style model.BarlineStyle) *barline {
	switch style {
	case model.BarDouble:
		return &barline{Location: "right", BarStyle: "light-light"}
	case model.BarFinal:
		return &barline{Location: "right", BarStyle: "light-heavy"}
	case model.BarNone:
		// An invisible barline is something a score says on purpose, and
		// MusicXML has a word for it. Left out, it came back as an ordinary
		// line and the music had a barline nobody had written.
		return &barline{Location: "right", BarStyle: "none"}
	case model.BarRepeatEnd, model.BarRepeatBoth:
		return &barline{Location: "right", BarStyle: "light-heavy",
			Repeat: &repeatTag{Direction: "backward"}}
	}
	return nil
}

// openingBarline is what a style puts at the head of the measure after it.
//
// Both styles that open a repeat belong here, and neither was exported
// properly. repeat-start was written as a left-hand barline on the measure it
// closes — the bar before the one it opens, and appended after that bar's
// notes, where a left-hand barline cannot be. And repeat-both was not written
// at all: a bar where one repeat ends and the next begins exported as the
// ending alone, so the Badinerie's turn at bar 16 came out as nothing.
func openingBarline(style model.BarlineStyle) *barline {
	switch style {
	case model.BarRepeatStart, model.BarRepeatBoth:
		return &barline{Location: "left", BarStyle: "heavy-light",
			Repeat: &repeatTag{Direction: "forward"}}
	}
	return nil
}

/*
 * The compressed form
 */

// ExportMXL writes a score as .mxl: MusicXML in the zip container the
// specification defines and every other program writes.
//
// The same XML, in a container that makes it a tenth the size — the Altès
// solo is 667 KB of MusicXML and 27 KB as an .mxl. That matters for a file
// somebody is sent, for a file kept in a repository, and for the one place
// size is the whole question: a score carried inside a link.
//
// The layout is fixed by the specification. mimetype comes first and is
// stored rather than deflated, the way ODF and EPUB do it, so that a reader
// can identify the file from its first bytes without unpacking it.
func ExportMXL(w io.Writer, s *model.Score) error {
	const scoreName = "score.musicxml"

	zw := zip.NewWriter(w)
	mt, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(mt, "application/vnd.recordare.musicxml"); err != nil {
		return err
	}

	container, err := zw.Create("META-INF/container.xml")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(container,
		`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
			`<container><rootfiles><rootfile full-path="%s" `+
			`media-type="application/vnd.recordare.musicxml+xml"/>`+
			`</rootfiles></container>`+"\n", scoreName); err != nil {
		return err
	}

	score, err := zw.Create(scoreName)
	if err != nil {
		return err
	}
	if err := Export(score, s); err != nil {
		return err
	}
	return zw.Close()
}
