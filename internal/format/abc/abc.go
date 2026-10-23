/*
 * abc.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package abc reads scores written in the ABC notation language.
//
// It replaces legacy/plugins/abc, which reached ABC through the external
// libabcp library from abcp.sf.net. That library was the single biggest
// obstacle to anyone building gscore — the 0.0.9 announcement had to tell
// people "some people have trouble getting abcp compiled on their system" and
// point Debian users at third-party packages — so the parser lives here
// instead, with no dependencies.
package abc

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string         { return "ABC" }
func (importer) Extensions() []string { return []string{".abc"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Import reads the first tune in an ABC file.
func Import(r io.Reader) (*model.Score, error) {
	scores, err := ImportAll(r)
	if err != nil {
		return nil, err
	}
	return scores[0], nil
}

// ImportAll implements format.MultiImporter.
func (importer) ImportAll(r io.Reader) ([]*model.Score, error) { return ImportAll(r) }

// ImportAll reads every tune in an ABC file.
//
// An ABC file is a tune book, not a single piece: each X: field starts a new
// tune, and one file in gscore's own corpus holds 48 of them. Import returns
// only the first, so anything able to show more than one score calls this.
func ImportAll(r io.Reader) ([]*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("abc: %w", err)
	}
	header, tunes := splitTunes(decodeText(data))
	if len(tunes) == 0 {
		return nil, errors.New("abc: no tune found; a tune starts with an X: field")
	}

	out := make([]*model.Score, 0, len(tunes))
	for _, t := range tunes {
		p := newParser()
		// Anything before the first X: is a file header and applies to every
		// tune in the book.
		if err := p.run(header); err != nil {
			return nil, err
		}
		p.lineOffset = t.line - 1
		if err := p.run(t.text); err != nil {
			return nil, err
		}
		s, err := p.finish()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

type tuneText struct {
	text string
	line int // 1-based line in the original file, for error messages
}

// splitTunes cuts a tune book at each X: field.
func splitTunes(src string) (header string, tunes []tuneText) {
	lines := strings.Split(src, "\n")
	var headerLines []string
	start := -1

	flush := func(end int) {
		if start >= 0 {
			tunes = append(tunes, tuneText{
				text: strings.Join(lines[start:end], "\n"),
				line: start + 1,
			})
		}
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "X:") {
			flush(i)
			start = i
			continue
		}
		if start < 0 {
			headerLines = append(headerLines, line)
		}
	}
	flush(len(lines))

	// A bare fragment with no X: field at all is still a tune. The field is
	// required by the spec, but refusing to read music that is plainly there
	// helps nobody.
	if len(tunes) == 0 && strings.TrimSpace(src) != "" {
		return "", []tuneText{{text: src, line: 1}}
	}
	return strings.Join(headerLines, "\n"), tunes
}

// decodeText returns the input as UTF-8.
//
// ABC files written before the language settled on UTF-8 are Latin-1, and
// gscore's own samples are among them: boite-musique.abc, exported by gscore
// 0.0.8 in 2004, holds "La Boîte à musique" as Latin-1 bytes. Since no
// well-formed UTF-8 text decodes as valid Latin-1 by accident in the other
// direction, testing for valid UTF-8 first is a safe way to tell the two apart.
func decodeText(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		b.WriteRune(rune(c))
	}
	return b.String()
}

// accKey identifies a note for the purpose of remembering an accidental. An
// accidental holds for the rest of the measure, and only in the octave it was
// written in, so the octave belongs in the key.
type accKey struct {
	Step   model.Step
	Octave int
}

type parser struct {
	titles    []string
	composer  string
	copyright string

	tempoBPM  int
	tempoUnit model.Duration
	tempoText string
	tempoSet  bool

	// State that fields change as the file is read, applying from that point
	// on, which is how ABC's inline fields work.
	unitLen     model.Duration
	unitLenSeen bool
	key         model.KeySignature
	time        model.TimeSignature
	clef        model.Clef

	parts []*model.Part
	byID  map[string]*model.Part
	cur   *model.Part

	meas  *model.Measure
	voice *model.Voice

	accidentals map[accKey]int

	// pending holds marks written before the note they attach to, which is
	// where ABC puts decorations and slur openings.
	pending model.Marks
	// last points at the marks of the element just parsed, so that a tie or a
	// closing slur written after it can reach back.
	last           *model.Marks
	pendingTieStop bool

	// lastElem is the element those marks belong to. Broken rhythm has to
	// reach back and change its duration, which marks alone cannot do.
	lastElem model.Element

	// pendingBroken counts the > or < signs waiting to be resolved against the
	// next element: positive for >, negative for <.
	pendingBroken int

	// stemDir is the direction [I:stemdir ...] has put in force.
	stemDir model.Stem

	// An open (n:... tuplet: how many elements are still inside it.
	tupletLeft           int
	tupletNum, tupletDen int
	tupletFirst          bool

	lineNo int
	// lineOffset keeps error messages pointing at the line in the whole file
	// rather than the line within one tune of a book.
	lineOffset int
}

func newParser() *parser {
	t := model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric}
	return &parser{
		time:        t,
		unitLen:     defaultUnitLen(t),
		key:         model.KeySignature{Fifths: 0, Mode: model.Major},
		clef:        model.Treble,
		byID:        map[string]*model.Part{},
		accidentals: map[accKey]int{},
		tempoUnit:   model.Duration{Num: 1, Den: 4},
	}
}

// defaultUnitLen is ABC's rule for L: when the file does not say: 1/16 for
// meters below 3/4, and 1/8 otherwise.
func defaultUnitLen(t model.TimeSignature) model.Duration {
	if t.BeatValue == 0 {
		return model.Duration{Num: 1, Den: 8}
	}
	if (model.Duration{Num: t.Beats, Den: t.BeatValue}).Cmp(model.Duration{Num: 3, Den: 4}) < 0 {
		return model.Duration{Num: 1, Den: 16}
	}
	return model.Duration{Num: 1, Den: 8}
}

func (p *parser) errf(f string, args ...any) error {
	return fmt.Errorf("abc: line %d: %s", p.lineNo+p.lineOffset, fmt.Sprintf(f, args...))
}

func (p *parser) run(src string) error {
	inText := false
	for i, raw := range strings.Split(src, "\n") {
		p.lineNo = i + 1
		line := strings.TrimRight(raw, "\r \t")
		if line == "" {
			continue
		}
		// A %%begintext block holds prose whose lines are not themselves
		// comment-marked, so it has to be tracked rather than filtered.
		if strings.HasPrefix(line, "%%begintext") {
			inText = true
			continue
		}
		if strings.HasPrefix(line, "%%endtext") {
			inText = false
			continue
		}
		if strings.HasPrefix(line, "%%MIDI") {
			p.midiDirective(line)
			continue
		}
		if inText || strings.HasPrefix(line, "%") {
			continue
		}
		if name, value, ok := fieldLine(line); ok {
			if err := p.field(name, value); err != nil {
				return err
			}
			continue
		}
		if err := p.music(line); err != nil {
			return err
		}
	}
	return nil
}

// fieldLine recognises ABC's "X:value" information fields, which are only
// fields when they begin the line.
// midiDirective reads the %%MIDI lines that ABC files use to say what a part
// sounds as. There is no field for it in the language itself.
func (p *parser) midiDirective(line string) {
	f := strings.Fields(line)
	if len(f) < 3 || !strings.EqualFold(f[1], "program") {
		return
	}
	// The form is "%%MIDI program [channel] program".
	v, err := strconv.Atoi(f[len(f)-1])
	if err != nil || v < 0 || v > 127 {
		return
	}
	p.ensurePart()
	p.cur.Instrument = model.Instrument(v)
}

func fieldLine(line string) (byte, string, bool) {
	if len(line) < 2 || line[1] != ':' {
		return 0, "", false
	}
	c := line[0]
	if !isLetter(c) {
		return 0, "", false
	}
	return c, strings.TrimSpace(line[2:]), true
}

func isLetter(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

func isNoteLetter(c byte) bool {
	return c >= 'A' && c <= 'G' || c >= 'a' && c <= 'g'
}

/*
 * Fields
 */

func (p *parser) field(name byte, value string) error {
	switch name {
	case 'X':
		// Reference number. Nothing in the model corresponds to it.
	case 'T':
		if value != "" {
			p.titles = append(p.titles, value)
		}
	case 'C':
		p.composer = value
	case 'Q':
		return p.fieldTempo(value)
	case 'M':
		return p.fieldMeter(value)
	case 'L':
		return p.fieldUnitLength(value)
	case 'K':
		return p.fieldKey(value)
	case 'V':
		return p.fieldVoice(value)
	case 'I':
		// An instruction field; the only one that changes the notes is the
		// stem direction, which applies from that point on.
		v := strings.ToLower(strings.TrimSpace(value))
		if rest, ok := strings.CutPrefix(v, "stemdir"); ok {
			switch strings.TrimSpace(rest) {
			case "up":
				p.stemDir = model.StemUp
			case "down":
				p.stemDir = model.StemDown
			default:
				p.stemDir = model.StemAuto
			}
		}
	case 'A', 'B', 'D', 'F', 'G', 'H', 'N', 'O', 'P', 'R', 'S', 'U', 'W', 'Z', 'w':
		// Informational and layout fields that do not bear on the notes.
	default:
		return p.errf("unknown field %q", string(name)+":")
	}
	return nil
}

func parseFraction(s string) (model.Duration, error) {
	s = strings.TrimSpace(s)
	num, den := s, "1"
	if i := strings.IndexByte(s, '/'); i >= 0 {
		num, den = s[:i], s[i+1:]
		if strings.TrimSpace(den) == "" {
			den = "2"
		}
	}
	n, err := strconv.Atoi(strings.TrimSpace(num))
	if err != nil {
		return model.Duration{}, fmt.Errorf("bad numerator %q", num)
	}
	d, err := strconv.Atoi(strings.TrimSpace(den))
	if err != nil {
		return model.Duration{}, fmt.Errorf("bad denominator %q", den)
	}
	if d == 0 {
		return model.Duration{}, errors.New("zero denominator")
	}
	// Deliberately not reduced: a time signature is not a fraction. Reducing
	// here turned M:4/4 into 1/1 and would equally turn 6/8 into 3/4, which is
	// a different meter with different beaming.
	return model.Duration{Num: n, Den: d}, nil
}

func (p *parser) fieldUnitLength(value string) error {
	d, err := parseFraction(value)
	if err != nil {
		return p.errf("L: %v", err)
	}
	p.unitLen = d
	p.unitLenSeen = true
	return nil
}

func (p *parser) fieldMeter(value string) error {
	v := strings.TrimSpace(value)
	var t model.TimeSignature
	switch {
	case strings.EqualFold(v, "C"):
		t = model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon}
	case strings.EqualFold(v, "C|"):
		t = model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut}
	case strings.EqualFold(v, "none"):
		t = model.TimeSignature{Beats: 0, BeatValue: 0, Symbol: model.TimeNumeric}
	default:
		d, err := parseFraction(v)
		if err != nil {
			return p.errf("M: %v", err)
		}
		t = model.TimeSignature{Beats: d.Num, BeatValue: d.Den, Symbol: model.TimeNumeric}
	}
	p.time = t
	if !p.unitLenSeen {
		p.unitLen = defaultUnitLen(t)
	}
	if p.cur != nil {
		p.cur.Time = t
	}
	return nil
}

// fieldTempo reads Q:. Both the modern "1/4=120" and the deprecated bare
// "120" are accepted; a bare number counts unit note lengths per minute, which
// is what the spec says it means and what gscore 0.0.8 itself wrote out.
func (p *parser) fieldTempo(value string) error {
	v := strings.TrimSpace(value)
	if i := strings.IndexByte(v, '"'); i >= 0 {
		if j := strings.LastIndexByte(v, '"'); j > i {
			p.tempoText = v[i+1 : j]
			v = strings.TrimSpace(v[:i] + v[j+1:])
		}
	}
	if v == "" {
		p.tempoSet = p.tempoText != ""
		return nil
	}
	unit := p.unitLen
	if i := strings.IndexByte(v, '='); i >= 0 {
		d, err := parseFraction(v[:i])
		if err != nil {
			return p.errf("Q: %v", err)
		}
		unit = d
		v = strings.TrimSpace(v[i+1:])
	}
	bpm, err := strconv.Atoi(v)
	if err != nil {
		return p.errf("Q: bad tempo %q", v)
	}
	p.tempoBPM, p.tempoUnit, p.tempoSet = bpm, unit, true
	return nil
}

// naturalFifths places the seven natural notes on the circle of fifths.
// Raising a tonic by a semitone moves it seven places clockwise, so the whole
// circle follows from these and the tonic's accidental: G sharp major is G's
// one sharp plus seven, which is eight, and G sharp minor is five.
var naturalFifths = map[model.Step]int{
	model.StepF: -1, model.StepC: 0, model.StepG: 1, model.StepD: 2,
	model.StepA: 3, model.StepE: 4, model.StepB: 5,
}

type modeInfo struct {
	offset int
	mode   model.Mode
}

// modeOffsets gives each mode's distance from the major key on the same tonic,
// in fifths: a dorian scale shares its signature with the major a tone below,
// hence -2.
var modeOffsets = map[string]modeInfo{
	"":    {0, model.Major},
	"maj": {0, model.Major},
	"ion": {0, model.Major},
	"m":   {-3, model.Minor},
	"min": {-3, model.Minor},
	"aeo": {-3, model.Minor},
	"dor": {-2, model.Dorian},
	"phr": {-4, model.Phrygian},
	"lyd": {1, model.Lydian},
	"mix": {-1, model.Mixolydian},
	"loc": {-5, model.Locrian},
}

var clefNames = map[string]model.Clef{
	"treble": model.Treble, "bass": model.Bass, "alto": model.Alto,
	"tenor": model.Tenor, "perc": model.Percussion, "percussion": model.Percussion,
}

// splitAttrs pulls "key=value" pairs off the end of a field, returning the
// leading bare token and the pairs. Quoted values are kept whole.
func splitAttrs(value string) (string, map[string]string) {
	attrs := map[string]string{}
	var head []string
	for _, tok := range splitQuoted(value) {
		if i := strings.IndexByte(tok, '='); i > 0 {
			k := strings.ToLower(tok[:i])
			v := strings.Trim(tok[i+1:], `"`)
			attrs[k] = v
			continue
		}
		head = append(head, tok)
	}
	return strings.Join(head, " "), attrs
}

// splitQuoted splits on whitespace but keeps double-quoted runs together.
func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case (c == ' ' || c == '\t') && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}

func (p *parser) fieldKey(value string) error {
	spec, attrs := splitAttrs(value)
	spec = strings.TrimSpace(spec)

	if spec != "" && !strings.EqualFold(spec, "none") {
		k, err := parseKeySpec(spec)
		if err != nil {
			return p.errf("K: %v", err)
		}
		p.key = k
		if p.cur != nil {
			p.cur.Key = k
		}
	}
	if name, ok := attrs["clef"]; ok {
		c, ok := clefNames[strings.ToLower(name)]
		if !ok {
			return p.errf("K: unknown clef %q", name)
		}
		p.clef = c
		if p.cur != nil {
			p.cur.Clef = c
		}
	}
	return nil
}

func parseKeySpec(spec string) (model.KeySignature, error) {
	step, ok := model.StepFromLetter(spec[0])
	if !ok {
		return model.KeySignature{}, fmt.Errorf("bad tonic %q", spec)
	}
	i := 1
	alter := 0
	for i < len(spec) {
		if spec[i] == '#' {
			alter++
		} else if spec[i] == 'b' {
			alter--
		} else {
			break
		}
		i++
	}
	fifths := naturalFifths[step] + 7*alter

	modeStr := strings.ToLower(strings.TrimSpace(spec[i:]))
	info, ok := modeOffsets[modeStr]
	if !ok && len(modeStr) >= 3 {
		info, ok = modeOffsets[modeStr[:3]]
	}
	if !ok {
		return model.KeySignature{}, fmt.Errorf("unknown mode %q", modeStr)
	}
	return model.KeySignature{Fifths: fifths + info.offset, Mode: info.mode}, nil
}

// fieldVoice selects or creates a part. ABC voices become parts, each on its
// own staff, which is how the 0.0.9 sample meant them: its V:0 is captioned
// "1st flute part".
func (p *parser) fieldVoice(value string) error {
	head, attrs := splitAttrs(value)
	id := strings.TrimSpace(head)
	if id == "" {
		return p.errf("V: needs a voice identifier")
	}

	p.closeMeasure(model.BarNone)

	if pt, ok := p.byID[id]; ok {
		p.cur = pt
		return nil
	}
	pt := &model.Part{ID: id, Clef: p.clef, Key: p.key, Time: p.time}
	if name, ok := attrs["name"]; ok {
		pt.Name = name
	} else if name, ok := attrs["nm"]; ok {
		pt.Name = name
	}
	if name, ok := attrs["clef"]; ok {
		c, ok := clefNames[strings.ToLower(name)]
		if !ok {
			return p.errf("V: unknown clef %q", name)
		}
		pt.Clef = c
	}
	p.parts = append(p.parts, pt)
	p.byID[id] = pt
	p.cur = pt
	return nil
}

/*
 * Structure
 */

func (p *parser) ensurePart() {
	if p.cur != nil {
		return
	}
	pt := &model.Part{Clef: p.clef, Key: p.key, Time: p.time}
	p.parts = append(p.parts, pt)
	p.cur = pt
}

func (p *parser) ensureMeasure() {
	p.ensurePart()
	if p.meas == nil {
		p.voice = &model.Voice{}
		p.meas = &model.Measure{Voices: []*model.Voice{p.voice}}
	}
}

// closeMeasure ends the measure in progress. A barline with nothing before it
// closes nothing, which is what lets a leading or doubled barline be harmless.
func (p *parser) closeMeasure(style model.BarlineStyle) {
	if p.meas == nil {
		return
	}
	p.meas.RightBarline = style
	p.cur.Measures = append(p.cur.Measures, p.meas)
	p.meas, p.voice = nil, nil
	p.accidentals = map[accKey]int{}
	// Broken rhythm pairs two adjacent notes and does not reach across a
	// barline, so an unresolved sign is dropped rather than carried over.
	p.pendingBroken = 0
}

func (p *parser) add(e model.Element) {
	p.ensureMeasure()
	p.applyBroken(e)
	p.voice.Elements = append(p.voice.Elements, e)
	p.lastElem = e
}

// elementDuration and setElementDuration reach the Dur field that Note, Rest
// and Chord each carry separately. Broken rhythm is the one thing that has to
// rewrite a duration after the fact.
func elementDuration(e model.Element) model.Duration {
	switch el := e.(type) {
	case *model.Note:
		return el.Dur
	case *model.Rest:
		return el.Dur
	case *model.Chord:
		return el.Dur
	}
	return model.Duration{Num: 0, Den: 1}
}

func setElementDuration(e model.Element, d model.Duration) {
	switch el := e.(type) {
	case *model.Note:
		el.Dur = d
	case *model.Rest:
		el.Dur = d
	case *model.Chord:
		el.Dur = d
		for _, n := range el.Notes {
			n.Dur = d
		}
	}
}

// applyBroken resolves a pending > or < between the previous element and this
// one. A single > dots the first and halves the second; each extra sign halves
// again, so a>>b is seven quarters against one.
func (p *parser) applyBroken(next model.Element) {
	n := p.pendingBroken
	p.pendingBroken = 0
	if n == 0 || p.lastElem == nil {
		return
	}
	k := n
	if k < 0 {
		k = -k
	}
	long, short := 1<<(k+1)-1, 1<<k

	first, second := p.lastElem, next
	if n < 0 {
		first, second = second, first
	}
	setElementDuration(first, elementDuration(first).Scale(long, short))
	setElementDuration(second, elementDuration(second).Scale(1, short))
}

// applyPending moves the marks waiting for this element onto it.
func (p *parser) applyPending(m *model.Marks) {
	for _, a := range p.pending.Articulations {
		m.AddArticulation(a)
	}
	m.Symbols = append(m.Symbols, p.pending.Symbols...)
	if p.pending.Dynamic != model.DynamicNone {
		m.Dynamic = p.pending.Dynamic
	}
	if p.pending.Stem != model.StemAuto {
		m.Stem = p.pending.Stem
	}
	if p.pending.SlurStart {
		m.SlurStart = true
	}
	if p.pendingTieStop {
		m.TieStop = true
		p.pendingTieStop = false
	}
	// A stem direction in force applies to everything after it until changed.
	if m.Stem == model.StemAuto && p.stemDir != model.StemAuto {
		m.Stem = p.stemDir
	}
	p.pending = model.Marks{}
	p.last = m
}

/*
 * Music
 */

type scanner struct {
	s string
	i int
}

func (sc *scanner) eof() bool    { return sc.i >= len(sc.s) }
func (sc *scanner) rest() string { return sc.s[sc.i:] }

func (sc *scanner) peek() byte {
	if sc.eof() {
		return 0
	}
	return sc.s[sc.i]
}

func (sc *scanner) peekAt(n int) byte {
	if sc.i+n >= len(sc.s) {
		return 0
	}
	return sc.s[sc.i+n]
}

func (sc *scanner) next() byte {
	c := sc.peek()
	sc.i++
	return c
}

func (sc *scanner) advance(n int) { sc.i += n }

func (sc *scanner) digits() (int, bool) {
	start := sc.i
	for !sc.eof() && isDigit(sc.peek()) {
		sc.i++
	}
	if sc.i == start {
		return 0, false
	}
	n, err := strconv.Atoi(sc.s[start:sc.i])
	if err != nil {
		return 0, false
	}
	return n, true
}

func (p *parser) music(line string) error {
	sc := &scanner{s: line}
	for !sc.eof() {
		c := sc.peek()
		switch {
		case c == ' ' || c == '\t':
			sc.next()
		case c == '%':
			return nil
		case c == '\\':
			// Line continuation; the following newline is already implicit.
			sc.next()
		case c == '|' || c == ':':
			if err := p.barline(sc); err != nil {
				return err
			}
		case c == '[':
			if isLetter(sc.peekAt(1)) && sc.peekAt(2) == ':' {
				if err := p.inlineField(sc); err != nil {
					return err
				}
				continue
			}
			if sc.peekAt(1) == '|' {
				if err := p.barline(sc); err != nil {
					return err
				}
				continue
			}
			if err := p.chord(sc); err != nil {
				return err
			}
		case c == '(':
			if isDigit(sc.peekAt(1)) {
				if err := p.tuplet(sc); err != nil {
					return err
				}
				continue
			}
			sc.next()
			p.pending.SlurStart = true
		case c == ')':
			sc.next()
			if p.last != nil {
				p.last.SlurStop = true
			}
		case c == '-':
			sc.next()
			if p.last != nil {
				p.last.TieStart = true
				p.pendingTieStop = true
			}
		case c == '.':
			sc.next()
			p.pending.AddArticulation(model.Staccato)
		case c == 'u':
			sc.next()
			p.pending.AddArticulation(model.UpBow)
		case c == 'v':
			sc.next()
			p.pending.AddArticulation(model.DownBow)
		case c == 'H':
			sc.next()
			p.pending.AddArticulation(model.Fermata)
		case c == 'L':
			sc.next()
			p.pending.AddArticulation(model.Accent)
		case c == 'M':
			sc.next()
			p.pending.AddArticulation(model.Marcato)
		case c == '"':
			// A chord symbol or annotation. Nothing in the model holds text
			// attached to a note yet, so it is skipped rather than guessed at.
			sc.next()
			for !sc.eof() && sc.peek() != '"' {
				sc.next()
			}
			if !sc.eof() {
				sc.next()
			}
		case c == 'z' || c == 'x':
			hidden := c == 'x'
			sc.next()
			dur, _ := p.length(sc)
			r := &model.Rest{Dur: dur, Hidden: hidden}
			p.applyTuplet(&r.Marks, &r.Dur)
			p.add(r)
			// A rest carries no marks, so anything pending is dropped rather
			// than leaking onto the next note.
			p.pending = model.Marks{}
			p.last = nil
		case c == '^' || c == '_' || c == '=' || isNoteLetter(c):
			n, err := p.parseNote(sc)
			if err != nil {
				return err
			}
			p.applyPending(&n.Marks)
			p.applyTuplet(&n.Marks, &n.Dur)
			p.add(n)
		case c == '>' || c == '<':
			for !sc.eof() && sc.peek() == c {
				sc.next()
				if c == '>' {
					p.pendingBroken++
				} else {
					p.pendingBroken--
				}
			}
		case c == '{':
			return p.errf("grace notes are not supported yet")
		case c == '!':
			if err := p.decoration(sc); err != nil {
				return err
			}
		case c == '&':
			return p.errf("voice overlay (&) is not supported yet")
		case c == 'Z':
			return p.errf("multi-measure rests are not supported yet")
		default:
			return p.errf("unexpected %q", string(c))
		}
	}
	return nil
}

// decorationArticulations maps ABC's decoration names onto the articulations
// the model knows. The symbolic short forms are the ones that turn up in real
// scores: !>! for an accent is by far the most common.
var decorationArticulations = map[string]model.Articulation{
	">": model.Accent, "accent": model.Accent, "emphasis": model.Accent,
	"^": model.Marcato, "marcato": model.Marcato,
	".": model.Staccato, "staccato": model.Staccato,
	"fermata": model.Fermata, "H": model.Fermata,
	"tenuto":  model.Tenuto,
	"upbow":   model.UpBow,
	"u":       model.UpBow,
	"downbow": model.DownBow,
	"v":       model.DownBow,
}

// decorationDynamics maps ABC's dynamic decorations onto the model's own.
// These used to arrive as loose symbols; they are part of the music now, so
// they export, round-trip and sound.
var decorationDynamics = map[string]model.Dynamic{
	"pppp": model.PPPP, "ppp": model.PPP, "pp": model.PP, "p": model.Piano,
	"mp": model.MP, "mf": model.MF,
	"f": model.Forte, "ff": model.FF, "fff": model.FFF, "ffff": model.FFFF,
	"sfz": model.SForzando, "sf": model.SForzato, "fp": model.FortePiano,
	"rfz": model.Rinforzando, "niente": model.Niente,
}

// decorationStems maps the stem directives written as decorations.
var decorationStems = map[string]model.Stem{
	"stemup": model.StemUp, "stemdown": model.StemDown, "stemauto": model.StemAuto,
}

// decorationSymbols maps decorations that are none of the above onto their
// SMuFL names, so an ornament still survives the import even though the model
// has no type for it yet.
var decorationSymbols = map[string]string{
	"crescendo(": "dynamicCrescendoHairpin", "crescendo)": "dynamicCrescendoHairpin",
	"diminuendo(": "dynamicDiminuendoHairpin", "diminuendo)": "dynamicDiminuendoHairpin",
	"<(": "dynamicCrescendoHairpin", "<)": "dynamicCrescendoHairpin",
	">(": "dynamicDiminuendoHairpin", ">)": "dynamicDiminuendoHairpin",
	"trill": "ornamentTrill", "turn": "ornamentTurn",
	"mordent": "ornamentMordent", "lowermordent": "ornamentMordent",
	"uppermordent": "ornamentShortTrill", "pralltriller": "ornamentShortTrill",
	"segno": "segno", "coda": "coda",
	"D.S.": "dalSegno", "D.C.": "daCapo", "fine": "fine",
	"breath":          "breathMarkComma",
	"arpeggio":        "arpeggiatoUp",
	"invertedfermata": "fermataBelow",
	"shortphrase":     "caesuraShort",
	"longphrase":      "caesura",
	"trem1":           "tremolo1", "trem2": "tremolo2", "trem3": "tremolo3",
}

// decoration reads a !...! decoration and attaches it to the next element.
//
// A name that is neither a known articulation nor a known SMuFL glyph becomes
// a symbol named after itself rather than an error. That keeps the mark in the
// score instead of discarding it, and the user can draw it: see docs/symbols.md.
func (p *parser) decoration(sc *scanner) error {
	sc.next() // '!'
	var b strings.Builder
	for !sc.eof() && sc.peek() != '!' {
		b.WriteByte(sc.next())
	}
	if sc.eof() {
		return p.errf("unterminated !...! decoration")
	}
	sc.next() // '!'

	name := strings.TrimSpace(b.String())
	if name == "" {
		return p.errf("empty !! decoration")
	}
	if a, ok := decorationArticulations[name]; ok {
		p.pending.AddArticulation(a)
		return nil
	}
	if d, ok := decorationDynamics[name]; ok {
		p.pending.Dynamic = d
		return nil
	}
	if st, ok := decorationStems[name]; ok {
		p.pending.Stem = st
		return nil
	}
	symbol := name
	if s, ok := decorationSymbols[name]; ok {
		symbol = s
	}
	p.pending.Symbols = append(p.pending.Symbols, model.SymbolMark{Name: symbol})
	return nil
}

// tuplet reads ABC's (p:q:r form.
//
// (3 alone means three notes in the time of two; the full form gives the ratio
// and how many notes it covers. The defaults for q depend on p and on whether
// the meter is compound, which is the rule the spec lays down.
func (p *parser) tuplet(sc *scanner) error {
	sc.next() // '('
	num, ok := sc.digits()
	if !ok || num < 2 {
		return p.errf("bad tuplet")
	}

	den := defaultTupletDen(num, p.time)
	count := num
	if !sc.eof() && sc.peek() == ':' {
		sc.next()
		if d, ok := sc.digits(); ok {
			den = d
		}
		if !sc.eof() && sc.peek() == ':' {
			sc.next()
			if r, ok := sc.digits(); ok {
				count = r
			}
		}
	}
	if den < 1 {
		return p.errf("bad tuplet ratio")
	}

	p.tupletNum, p.tupletDen = num, den
	p.tupletLeft, p.tupletFirst = count, true
	return nil
}

// defaultTupletDen is ABC's table for the unwritten half of the ratio.
func defaultTupletDen(num int, t model.TimeSignature) int {
	compound := t.Beats%3 == 0 && t.Beats > 3
	switch num {
	case 2, 4, 8:
		return 3
	case 3, 6:
		return 2
	case 5, 7, 9:
		if compound {
			return 3
		}
		return 2
	}
	return 2
}

// applyTuplet scales an element into the open tuplet and marks the group.
func (p *parser) applyTuplet(m *model.Marks, dur *model.Duration) {
	if p.tupletLeft <= 0 {
		return
	}
	// Three in the time of two means each one sounds for two thirds of what
	// it is written as.
	*dur = dur.Scale(p.tupletDen, p.tupletNum)
	t := &model.Tuplet{Actual: p.tupletNum, Normal: p.tupletDen, Start: p.tupletFirst}
	p.tupletFirst = false
	p.tupletLeft--
	if p.tupletLeft == 0 {
		t.Stop = true
	}
	m.Tuplet = t
}

func (p *parser) inlineField(sc *scanner) error {
	sc.next() // '['
	name := sc.next()
	sc.next() // ':'
	var b strings.Builder
	for !sc.eof() && sc.peek() != ']' {
		b.WriteByte(sc.next())
	}
	if sc.eof() {
		return p.errf("unterminated inline field")
	}
	sc.next() // ']'
	return p.field(name, strings.TrimSpace(b.String()))
}

func (p *parser) barline(sc *scanner) error {
	var style model.BarlineStyle
	var width int
	switch r := sc.rest(); {
	case strings.HasPrefix(r, "|]"), strings.HasPrefix(r, "[|"):
		style, width = model.BarFinal, 2
	case strings.HasPrefix(r, "||"):
		style, width = model.BarDouble, 2
	case strings.HasPrefix(r, "|:"):
		style, width = model.BarRepeatStart, 2
	case strings.HasPrefix(r, "::"):
		style, width = model.BarRepeatBoth, 2
	case strings.HasPrefix(r, ":|"):
		style, width = model.BarRepeatEnd, 2
	case strings.HasPrefix(r, "|"):
		style, width = model.BarSingle, 1
	default:
		return p.errf("unexpected %q", string(sc.peek()))
	}
	sc.advance(width)
	p.closeMeasure(style)
	// A repeat ending number such as |1 follows the barline; nothing in the
	// model records it yet.
	sc.digits()
	return nil
}

// length reads ABC's note length suffix and returns the resulting duration.
// The suffix multiplies the unit note length: "2" doubles it, "/" halves it,
// "//" quarters it and "/3" divides by three. written reports whether any
// suffix was actually present, which chords need in order to tell an outer
// length from none.
func (p *parser) length(sc *scanner) (dur model.Duration, written bool) {
	num, den := 1, 1
	if n, ok := sc.digits(); ok {
		num, written = n, true
	}
	slashes := 0
	for !sc.eof() && sc.peek() == '/' {
		slashes++
		sc.next()
	}
	if slashes > 0 {
		written = true
		if d, ok := sc.digits(); ok && d != 0 {
			den = d << (slashes - 1)
		} else {
			den = 1 << slashes
		}
	}
	return p.unitLen.Scale(num, den), written
}

func (p *parser) parseNote(sc *scanner) (*model.Note, error) {
	p.ensurePart()

	acc, hasAcc := model.NoAccidental, false
	switch sc.peek() {
	case '^':
		sc.next()
		if sc.peek() == '^' {
			sc.next()
			acc = model.DoubleSharp
		} else {
			acc = model.Sharp
		}
		hasAcc = true
	case '_':
		sc.next()
		if sc.peek() == '_' {
			sc.next()
			acc = model.DoubleFlat
		} else {
			acc = model.Flat
		}
		hasAcc = true
	case '=':
		sc.next()
		acc = model.Natural
		hasAcc = true
	}

	if sc.eof() {
		return nil, p.errf("accidental with no note after it")
	}
	c := sc.peek()
	step, ok := model.StepFromLetter(c)
	if !ok {
		return nil, p.errf("expected a note letter, got %q", string(c))
	}
	sc.next()

	// Upper case is the octave starting at middle C; lower case is the one
	// above it. A comma drops an octave and an apostrophe raises one.
	octave := 4
	if c >= 'a' && c <= 'z' {
		octave = 5
	}
octaveMarks:
	for !sc.eof() {
		switch sc.peek() {
		case ',':
			octave--
			sc.next()
		case '\'':
			octave++
			sc.next()
		default:
			break octaveMarks
		}
	}

	dur, _ := p.length(sc)

	// What sounds and what is printed are decided separately. An explicit
	// accidental both prints and holds for the rest of the measure in its own
	// octave; otherwise the key signature alone decides the alteration and
	// nothing is printed.
	k := accKey{Step: step, Octave: octave}
	alter := 0
	switch {
	case hasAcc:
		alter = acc.Semitones()
		p.accidentals[k] = alter
	default:
		if a, ok := p.accidentals[k]; ok {
			alter = a
		} else {
			alter = p.cur.Key.AlterFor(step)
		}
	}

	return &model.Note{
		Pitch:      model.Pitch{Step: step, Octave: octave, Alter: alter},
		Dur:        dur,
		Accidental: acc,
	}, nil
}

func (p *parser) chord(sc *scanner) error {
	sc.next() // '['
	ch := &model.Chord{}
	tied := 0
	for !sc.eof() && sc.peek() != ']' {
		if c := sc.peek(); c == ' ' || c == '\t' {
			sc.next()
			continue
		}
		n, err := p.parseNote(sc)
		if err != nil {
			return err
		}
		// A note inside a chord may carry a tie of its own: "[C-E-G]" ties
		// those noteheads to the ones after it. The grammar has it as
		// "[" 2*note "]" with note ::= pitch [note-length] [tie], so this is
		// separate from "[CEG]-", which ties the chord as a whole.
		if !sc.eof() && sc.peek() == '-' {
			sc.next()
			n.TieStart = true
			tied++
		}
		ch.Notes = append(ch.Notes, n)
	}
	if sc.eof() {
		return p.errf("unterminated chord")
	}
	sc.next() // ']'

	if len(ch.Notes) == 0 {
		return p.errf("empty chord")
	}
	if outer, written := p.length(sc); written {
		ch.Dur = outer
	} else {
		ch.Dur = ch.Notes[0].Dur
	}
	// The chord sounds as one duration, whatever its noteheads were written
	// with individually.
	for _, n := range ch.Notes {
		n.Dur = ch.Dur
	}

	p.applyPending(&ch.Marks)
	p.applyTuplet(&ch.Marks, &ch.Dur)
	for _, n := range ch.Notes {
		n.Dur = ch.Dur
	}
	p.add(ch)
	if tied > 0 {
		// Set after applyPending, or the chord would absorb the tie stop it
		// is itself starting. Whatever comes next carries it, exactly as for
		// a tie written after the closing bracket.
		p.pendingTieStop = true
		if tied == len(ch.Notes) {
			ch.TieStart = true
		}
	}
	return nil
}

func (p *parser) finish() (*model.Score, error) {
	p.closeMeasure(model.BarNone)

	if len(p.parts) == 0 {
		return nil, errors.New("abc: no music found")
	}

	s := &model.Score{Parts: p.parts}
	if len(p.titles) > 0 {
		s.Identity.Title = p.titles[0]
	}
	if len(p.titles) > 1 {
		s.Identity.Subtitle = strings.Join(p.titles[1:], " / ")
	}
	s.Identity.Composer = p.composer
	s.Identity.Copyright = p.copyright

	s.Tempo = model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}}
	if p.tempoSet {
		s.Tempo = model.Tempo{BPM: p.tempoBPM, Unit: p.tempoUnit, Text: p.tempoText}
	}
	return s, nil
}
