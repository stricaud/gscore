/*
 * nwc.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package nwc reads NoteWorthy Composer scores.
//
// NoteWorthy has two formats. The .nwctxt one is text, documented, and what
// this package reads. The .nwc one is a compressed binary format that is not
// published; a file in it is detected and reported rather than guessed at,
// because a half-understood binary parser produces plausible wrong music,
// which is worse than refusing.
//
// NoteWorthy Composer itself exports .nwctxt, so anything in the binary form
// can be converted by the program that wrote it.
package nwc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string         { return "NoteWorthy" }
func (importer) Extensions() []string { return []string{".nwctxt", ".nwc"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Import reads a NoteWorthy score, text or binary.
func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("nwc: %w", err)
	}

	// A binary container is unwrapped first. From version 2.75 what comes out
	// is the same NWCTXT the text files hold; before that it is the 1.7x
	// object stream, which has its own reader.
	if isBinary(data) {
		raw, major, minor, uerr := inflate(data)
		if uerr != nil {
			return nil, uerr
		}
		if text, ok := findNWCTXT(raw); ok {
			data = text
		} else {
			return parseNWC1(raw, minor)
		}
		_ = minor
		_ = major
	}

	p := &parser{score: &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		p.line++
		if err := p.read(strings.TrimSpace(sc.Text())); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("nwc: %w", err)
	}
	return p.finish()
}

// isBinary tells a container or an inflated score from plain NWCTXT.
func isBinary(data []byte) bool {
	return bytes.HasPrefix(data, []byte(nwzMagic)) ||
		bytes.HasPrefix(data, []byte(vendorMarker))
}

type parser struct {
	score *model.Score
	cur   *model.Part
	meas  *model.Measure
	line  int

	// Carried from the staff header until the first note arrives.
	pendingClef model.Clef
	pendingKey  model.KeySignature
	pendingTime model.TimeSignature
	tempoSet    bool
}

func (p *parser) errf(f string, args ...any) error {
	return fmt.Errorf("nwc: line %d: %s", p.line, fmt.Sprintf(f, args...))
}

// read handles one line, which is either a marker or a |Item|Key:Value| row.
func (p *parser) read(line string) error {
	if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, ";") {
		return nil
	}
	if !strings.HasPrefix(line, "|") {
		return nil
	}

	fields := splitFields(line)
	if len(fields) == 0 {
		return nil
	}
	item := fields[0]
	attrs := map[string]string{}
	for _, f := range fields[1:] {
		if i := strings.IndexByte(f, ':'); i > 0 {
			attrs[f[:i]] = strings.Trim(f[i+1:], `"`)
		} else if f != "" {
			attrs[f] = ""
		}
	}

	switch item {
	case "SongInfo":
		p.score.Identity.Title = attrs["Title"]
		if a := attrs["Author"]; a != "" {
			p.score.Identity.Composer = a
		}
		p.score.Identity.Copyright = attrs["Copyright1"]
	case "AddStaff":
		p.newStaff(attrs["Name"])
	case "Clef":
		p.setClef(attrs["Type"])
	case "Key":
		p.setKey(attrs["Signature"], attrs["Tonic"])
	case "TimeSig":
		p.setTime(attrs["Signature"])
	case "Tempo":
		p.setTempo(attrs)
	case "Note":
		return p.addNote(attrs)
	case "Rest":
		return p.addRest(attrs)
	case "Chord":
		return p.addChord(attrs)
	case "Bar":
		p.closeMeasure(barStyles[attrs["Style"]])
	}
	return nil
}

// splitFields cuts a line on | while leaving quoted text alone, since a title
// may well contain one.
func splitFields(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for i := 1; i < len(line); i++ { // skip the leading |
		c := line[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == '|' && !inQuote:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

var clefTypes = map[string]model.Clef{
	"Treble": model.Treble, "Bass": model.Bass,
	"Alto": model.Alto, "Tenor": model.Tenor,
	"Percussion": model.Percussion,
}

var barStyles = map[string]model.BarlineStyle{
	"":                  model.BarSingle,
	"Single":            model.BarSingle,
	"Double":            model.BarDouble,
	"SectionOpen":       model.BarDouble,
	"SectionClose":      model.BarFinal,
	"MasterRepeatOpen":  model.BarRepeatStart,
	"MasterRepeatClose": model.BarRepeatEnd,
	"LocalRepeatOpen":   model.BarRepeatStart,
	"LocalRepeatClose":  model.BarRepeatEnd,
}

func (p *parser) newStaff(name string) {
	p.closeMeasure(model.BarNone)
	p.cur = &model.Part{
		Name: name,
		Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
	}
	p.score.Parts = append(p.score.Parts, p.cur)
}

func (p *parser) ensureStaff() {
	if p.cur == nil {
		p.newStaff("")
	}
}

func (p *parser) setClef(t string) {
	p.ensureStaff()
	if c, ok := clefTypes[t]; ok {
		p.cur.Clef = c
	}
}

// setKey reads the signature, which NoteWorthy writes as the altered notes
// themselves rather than a count: "F#,C#" is two sharps.
func (p *parser) setKey(sig, tonic string) {
	p.ensureStaff()
	if sig == "" {
		return
	}
	sharps, flats := 0, 0
	for _, part := range strings.Split(sig, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasSuffix(part, "#"):
			sharps++
		case strings.HasSuffix(part, "b"):
			flats++
		}
	}
	fifths := sharps
	if flats > 0 {
		fifths = -flats
	}
	mode := model.Major
	if strings.HasSuffix(tonic, "m") {
		mode = model.Minor
	}
	p.cur.Key = model.KeySignature{Fifths: fifths, Mode: mode}
}

func (p *parser) setTime(sig string) {
	p.ensureStaff()
	switch sig {
	case "Common":
		p.cur.Time = model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon}
		return
	case "AllaBreve":
		p.cur.Time = model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut}
		return
	}
	n, d, ok := strings.Cut(sig, "/")
	if !ok {
		return
	}
	beats, err1 := strconv.Atoi(strings.TrimSpace(n))
	value, err2 := strconv.Atoi(strings.TrimSpace(d))
	if err1 != nil || err2 != nil || value == 0 {
		return
	}
	p.cur.Time = model.TimeSignature{Beats: beats, BeatValue: value, Symbol: model.TimeNumeric}
}

var tempoBases = map[string]model.Duration{
	"Whole": {Num: 1, Den: 1}, "Half": {Num: 1, Den: 2},
	"Quarter": {Num: 1, Den: 4}, "Eighth": {Num: 1, Den: 8},
	"Sixteenth": {Num: 1, Den: 16},
}

func (p *parser) setTempo(attrs map[string]string) {
	if p.tempoSet {
		return
	}
	bpm, err := strconv.Atoi(attrs["Tempo"])
	if err != nil || bpm <= 0 {
		return
	}
	unit := model.Duration{Num: 1, Den: 4}
	if u, ok := tempoBases[attrs["Base"]]; ok {
		unit = u
	}
	if strings.Contains(attrs["Base"], "Dotted") {
		unit = unit.Scale(3, 2)
	}
	p.score.Tempo = model.Tempo{BPM: bpm, Unit: unit, Text: attrs["Text"]}
	p.tempoSet = true
}

/*
 * Notes
 */

var durNames = map[string]model.Duration{
	"Whole": {Num: 1, Den: 1}, "Half": {Num: 1, Den: 2},
	"4th": {Num: 1, Den: 4}, "8th": {Num: 1, Den: 8},
	"16th": {Num: 1, Den: 16}, "32nd": {Num: 1, Den: 32},
	"64th": {Num: 1, Den: 64},
}

// parseDur reads a Dur field: a value plus comma-separated modifiers.
func parseDur(s string) (dur model.Duration, triplet bool, ok bool) {
	parts := strings.Split(s, ",")
	d, ok := durNames[strings.TrimSpace(parts[0])]
	if !ok {
		return model.Duration{}, false, false
	}
	for _, m := range parts[1:] {
		switch strings.TrimSpace(m) {
		case "Dotted":
			d = d.Scale(3, 2)
		case "DblDotted":
			d = d.Scale(7, 4)
		case "Triplet", "Triplet=First", "Triplet=End":
			// Three in the time of two, so each one sounds two thirds as long.
			d = d.Scale(2, 3)
			triplet = true
		}
	}
	return d, triplet, true
}

// parsePos reads one entry of a Pos field.
//
// NoteWorthy counts staff positions from the middle line, so 0 is the middle
// line and positive is upward. An accidental may be written in front of the
// number, and a tie marked after it.
func parsePos(s string) (step int, acc model.Accidental, tie bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false, fmt.Errorf("empty position")
	}
	switch s[0] {
	case '#':
		acc, s = model.Sharp, s[1:]
	case 'b':
		acc, s = model.Flat, s[1:]
	case 'n':
		acc, s = model.Natural, s[1:]
	case 'x':
		acc, s = model.DoubleSharp, s[1:]
	case 'v':
		acc, s = model.DoubleFlat, s[1:]
	}

	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, false, fmt.Errorf("no position in %q", s)
	}
	n, convErr := strconv.Atoi(strings.TrimPrefix(s[:i], "+"))
	if convErr != nil {
		return 0, 0, false, convErr
	}
	// Anything after the number is a tie marker or a notehead option.
	tie = strings.Contains(s[i:], "^")

	// Middle line is 0 in NoteWorthy and 4 in gscore, where 0 is the bottom.
	return n + 4, acc, tie, nil
}

func clefOffset(c model.Clef) int {
	switch c {
	case model.Bass:
		return 10
	case model.Alto:
		return 4
	case model.Tenor:
		return 6
	case model.Percussion:
		return 4
	}
	return -2
}

func (p *parser) pitchAt(step int, acc model.Accidental) model.Pitch {
	pitch := model.PitchFromDiatonicIndex(step - clefOffset(p.cur.Clef))
	if acc != model.NoAccidental {
		pitch.Alter = acc.Semitones()
	} else {
		pitch.Alter = p.cur.Key.AlterFor(pitch.Step)
	}
	return pitch
}

func (p *parser) ensureMeasure() {
	p.ensureStaff()
	if p.meas == nil {
		p.meas = &model.Measure{Voices: []*model.Voice{{}}}
	}
}

func (p *parser) add(el model.Element) {
	p.ensureMeasure()
	p.meas.Voices[0].Elements = append(p.meas.Voices[0].Elements, el)
}

func (p *parser) closeMeasure(style model.BarlineStyle) {
	if p.meas == nil {
		return
	}
	p.meas.RightBarline = style
	p.cur.Measures = append(p.cur.Measures, p.meas)
	p.meas = nil
}

func tupletMark(triplet bool) *model.Tuplet {
	if !triplet {
		return nil
	}
	return &model.Tuplet{Actual: 3, Normal: 2}
}

func (p *parser) addNote(attrs map[string]string) error {
	p.ensureStaff()
	dur, triplet, ok := parseDur(attrs["Dur"])
	if !ok {
		return p.errf("unknown duration %q", attrs["Dur"])
	}
	step, acc, tie, err := parsePos(attrs["Pos"])
	if err != nil {
		return p.errf("%v", err)
	}
	n := &model.Note{Pitch: p.pitchAt(step, acc), Dur: dur, Accidental: acc}
	n.TieStart = tie
	n.Tuplet = tupletMark(triplet)
	p.add(n)
	return nil
}

func (p *parser) addRest(attrs map[string]string) error {
	p.ensureStaff()
	dur, triplet, ok := parseDur(attrs["Dur"])
	if !ok {
		return p.errf("unknown duration %q", attrs["Dur"])
	}
	r := &model.Rest{Dur: dur}
	r.Tuplet = tupletMark(triplet)
	p.add(r)
	return nil
}

func (p *parser) addChord(attrs map[string]string) error {
	p.ensureStaff()
	dur, triplet, ok := parseDur(attrs["Dur"])
	if !ok {
		return p.errf("unknown duration %q", attrs["Dur"])
	}
	c := &model.Chord{Dur: dur}
	c.Tuplet = tupletMark(triplet)
	for _, entry := range strings.Split(attrs["Pos"], ",") {
		step, acc, tie, err := parsePos(entry)
		if err != nil {
			return p.errf("%v", err)
		}
		n := &model.Note{Pitch: p.pitchAt(step, acc), Dur: dur, Accidental: acc}
		n.TieStart = tie
		c.Notes = append(c.Notes, n)
	}
	if len(c.Notes) == 0 {
		return p.errf("a chord with no notes")
	}
	p.add(c)
	return nil
}

func (p *parser) finish() (*model.Score, error) {
	p.closeMeasure(model.BarFinal)
	if len(p.score.Parts) == 0 {
		return nil, fmt.Errorf("nwc: no staves found")
	}
	// A tie needs its other end marked, which NoteWorthy leaves implicit.
	for _, part := range p.score.Parts {
		linkTies(part)
	}
	return p.score, nil
}

// linkTies walks a part and marks the note each tie lands on.
func linkTies(part *model.Part) {
	open := map[int]bool{}
	for _, m := range part.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *model.Note:
					if open[e.Pitch.MIDI()] {
						e.TieStop = true
						delete(open, e.Pitch.MIDI())
					}
					if e.TieStart {
						open[e.Pitch.MIDI()] = true
					}
				case *model.Chord:
					for _, n := range e.Notes {
						if open[n.Pitch.MIDI()] {
							n.TieStop = true
							delete(open, n.Pitch.MIDI())
						}
						if n.TieStart {
							open[n.Pitch.MIDI()] = true
						}
					}
				}
			}
		}
	}
}
