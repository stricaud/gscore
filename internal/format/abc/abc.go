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
	// A partial read hands back the music it got along with the account of
	// what it could not read. Dropping the score here would have thrown that
	// away and left the caller with nothing but the complaint.
	if len(scores) == 0 {
		return nil, err
	}
	return scores[0], err
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
	header, tunes := splitTunes(normaliseLines(decodeText(data)))
	if len(tunes) == 0 {
		return nil, errors.New("abc: no tune found; a tune starts with an X: field")
	}

	out := make([]*model.Score, 0, len(tunes))
	var skipped []string
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
		skipped = append(skipped, p.skipped...)
		s, err := p.finish()
		if err != nil {
			// An X: block with nothing in it but fields — a title somebody
			// meant to come back to, or a tune whose every line was skipped.
			// It is not a tune, and it is not a reason to stop reading the
			// book it sits in.
			skipped = append(skipped, fmt.Sprintf("tune at line %d has no music", t.line))
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("abc: no music found in any tune")
	}
	if len(skipped) > 0 {
		return out, &PartialError{Skipped: skipped}
	}
	return out, nil
}

// PartialError reports the lines of a tune book that could not be read.
//
// The music that was read is returned with it. One mistyped character in a
// book of nine hundred tunes used to mean none of them opened, which is the
// wrong trade by any measure.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	n := len(e.Skipped)
	first := e.Skipped[0]
	if n == 1 {
		return "abc: skipped one line: " + first
	}
	return fmt.Sprintf("abc: skipped %d lines, the first being: %s", n, first)
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string { return e.Skipped }

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

// normaliseLines turns every line ending into a newline.
//
// Tune books are not all written on the same machine. Four of the forty-four
// in the Village Music Project's collection end their lines with a bare
// carriage return, the classic Mac ending, and read as one enormous line: the
// parser found the X: field, no music after it, and reported an empty tune.
// Normalising here rather than in each reader lets the rest of the parser go
// on assuming that a line is a line.
func normaliseLines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
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
	// Character by character, not all or nothing. A file with one bad byte in
	// it and UTF-8 everywhere else used to be decoded as if the whole thing
	// were single-byte, which turned every é in it into Ã©: one mistyped
	// character cost the correct reading of all the others.
	var b strings.Builder
	b.Grow(len(data))
	for i := 0; i < len(data); {
		if r, n := utf8.DecodeRune(data[i:]); r != utf8.RuneError || n > 1 {
			b.WriteRune(r)
			i += n
			continue
		}
		b.WriteRune(cp1252(data[i]))
		i++
	}
	return b.String()
}

// cp1252High maps the bytes 0x80 to 0x9F, the only range where Windows-1252
// differs from Latin-1.
//
// Those bytes are where the punctuation a word processor inserts lives — the
// curly quotes, the dash, the ellipsis — so a file typed on Windows and saved
// as plain text has them in it. Read as Latin-1 they are control characters,
// which is why a transcriber's apostrophe used to come out as U+0082.
var cp1252High = [32]rune{
	'\u20AC', '\u0081', '\u201A', '\u0192', '\u201E', '\u2026', '\u2020', '\u2021',
	'\u02C6', '\u2030', '\u0160', '\u2039', '\u0152', '\u008D', '\u017D', '\u008F',
	'\u0090', '\u2018', '\u2019', '\u201C', '\u201D', '\u2022', '\u2013', '\u2014',
	'\u02DC', '\u2122', '\u0161', '\u203A', '\u0153', '\u009D', '\u017E', '\u0178',
}

func cp1252(c byte) rune {
	if c >= 0x80 && c <= 0x9F {
		return cp1252High[c-0x80]
	}
	return rune(c)
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

	// pendingOctave counts the ' and , marks written before a note rather
	// than after it. The language puts them after, and several of the tune
	// books put them before; both mean the same thing to a player.
	pendingOctave int

	// stemDir is the direction [I:stemdir ...] has put in force.
	stemDir model.Stem

	// pendingEnding is a volta number read off a barline. It belongs to the
	// measure after that barline rather than the one it closed, which is why
	// it waits here.
	pendingEnding *model.Ending
	// endingOpen is the volta bracket now open, so the barline that ends it
	// can close it instead of the bracket being guessed at a bar later.
	endingOpen *model.Ending

	// An open (n:... tuplet: how many elements are still inside it.
	tupletLeft           int
	tupletNum, tupletDen int
	tupletFirst          bool

	lineNo int
	// lineOffset keeps error messages pointing at the line in the whole file
	// rather than the line within one tune of a book.
	lineOffset int

	// elements counts everything added to the score, which is how a line
	// that produced no music at all is told from one that produced some.
	elements int

	// skipped holds the lines that could not be read. A tune book is written
	// by hand over years and has typing mistakes in it; the reader keeps
	// going and says what it dropped, rather than refusing the book.
	skipped []string
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
		// Trimmed at both ends: a field indented by a space is still a field,
		// and " F: http://..." read as music is a bar of nonsense. ABC says
		// a field starts in the first column; refusing to read a source note
		// because somebody lined it up is not worth the fidelity.
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
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
			value = stripFieldComment(name, value)
			// Several fields written on one line: "M: 2/4 L: 1/8 K: G" is
			// three of them, which is how Prestons1798.abc writes every
			// tune's header.
			value, more := splitFieldRun(name, value)
			// A key field with music after it on the same line. "K: Em |:"
			// opens a repeat the moment the key is given; it is not what the
			// spec describes, but it is what Thompson1765.abc writes, and the
			// music after it is music.
			trailing := ""
			if name == 'K' {
				if i := strings.IndexByte(value, '|'); i >= 0 {
					trailing, value = value[i:], strings.TrimSpace(value[:i])
				}
			}
			if err := p.field(name, value); err != nil {
				p.skip(line, err)
				continue
			}
			for _, f := range more {
				n2, v2, ok := fieldLine(f)
				if !ok {
					continue
				}
				v2 = stripFieldComment(n2, v2)
				if n2 == 'K' {
					if i := strings.IndexByte(v2, '|'); i >= 0 {
						trailing, v2 = v2[i:], strings.TrimSpace(v2[:i])
					}
				}
				if err := p.field(n2, v2); err != nil {
					p.skip(f, err)
				}
			}
			if trailing != "" {
				if err := p.music(trailing); err != nil {
					p.skip(trailing, err)
				}
			}
			continue
		}
		if err := p.music(line); err != nil {
			p.skip(line, err)
		}
	}
	return nil
}

// skip records a line the reader could not make sense of.
//
// Dropping music silently would be the worst of the options, so every skipped
// line is kept and reported as a partial read: the book opens, and the person
// reading it is told which lines did not come through.
func (p *parser) skip(line string, cause error) {
	if len(line) > 48 {
		line = line[:48] + "…"
	}
	p.note(fmt.Sprintf("%v, in %q", cause, line))
}

// structuralFields are the fields whose value the parser acts on, and the only
// ones a run of fields on a single line is split at. Splitting a title or a
// note would cut it in half at the first colon in it.
var structuralFields = map[byte]bool{
	'I': true, 'K': true, 'L': true, 'M': true, 'P': true, 'Q': true, 'V': true,
}

// splitFieldRun cuts a field value at the next field written on the same line.
//
// It returns the value up to that point and the fields that followed. Only a
// structural field starts a new one: "F: http://..." holds a colon that is not
// a field, and so does "Z: 2014 John Chambers <jc:trillian.mit.edu>".
func splitFieldRun(name byte, value string) (string, []string) {
	if !structuralFields[name] {
		return value, nil
	}
	i := nextFieldStart(value)
	if i < 0 {
		return value, nil
	}
	head, rest := strings.TrimSpace(value[:i]), value[i:]
	var more []string
	for {
		j := nextFieldStart(rest)
		if j < 0 {
			return head, append(more, strings.TrimSpace(rest))
		}
		more = append(more, strings.TrimSpace(rest[:j]))
		rest = rest[j:]
	}
}

// nextFieldStart finds a "X:" that begins a field inside a field's value: a
// single structural letter, preceded by whitespace and followed by a colon.
func nextFieldStart(value string) int {
	for i := 1; i+1 < len(value); i++ {
		if value[i+1] != ':' || !structuralFields[value[i]] {
			continue
		}
		if c := value[i-1]; c != ' ' && c != '\t' {
			continue
		}
		return i
	}
	return -1
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

// textFields are the fields whose value is prose, where a per cent sign is a
// per cent sign rather than the start of a comment.
var textFields = map[byte]bool{
	'A': true, 'B': true, 'C': true, 'D': true, 'F': true, 'G': true,
	'H': true, 'N': true, 'O': true, 'R': true, 'S': true, 'T': true,
	'W': true, 'w': true, 'X': true, 'Z': true,
}

// stripFieldComment cuts a trailing % comment off a field the parser has to
// understand.
//
// "M: C % Actually 2/4" is a real line out of Straights1783.abc, and a meter
// field that refuses to be read because the transcriber explained himself in
// it is a bug in the reader, not in the book.
func stripFieldComment(name byte, value string) string {
	if textFields[name] {
		return value
	}
	if i := strings.IndexByte(value, '%'); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value)
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
	// A field with nothing after the colon says nothing. "M:" on its own,
	// which Malchair.abc writes seventeen times, leaves the meter as it was
	// rather than being a meter the reader cannot parse.
	if value == "" && structuralFields[name] {
		return nil
	}
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
	// An explicit accidental list in place of a tonic. "K:=f=c=g" cancels
	// three accidentals rather than naming a key, which is how a tune book
	// writes a change of mode in the middle of a strain.
	if spec[0] == '=' || spec[0] == '^' || spec[0] == '_' {
		return parseAccidentalKey(spec)
	}
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
		// A mode nobody recognises is a typo or a stray word, not a reason to
		// refuse the three hundred other tunes in the book. The tonic was
		// readable, so the signature is right even when the word after it is
		// not; major is what an unqualified key means.
		info = modeOffsets[""]
	}
	return model.KeySignature{Fifths: fifths + info.offset, Mode: info.mode}, nil
}

// parseAccidentalKey reads a key given as the accidentals themselves.
//
// The model holds a signature as a number of fifths, so the list is reduced to
// the sharps and flats it asks for. A list of naturals, which is what
// Thompson1778.abc writes, is the open key — and that is exactly what it
// means.
func parseAccidentalKey(spec string) (model.KeySignature, error) {
	fifths := 0
	for i := 0; i < len(spec); i++ {
		switch c := spec[i]; {
		case c == '^':
			fifths++
		case c == '_':
			fifths--
		case c == '=' || c == ' ' || c == '\t' || c == ',' || c == '\'':
		case isNoteLetter(c):
		default:
			return model.KeySignature{}, fmt.Errorf("bad accidental list %q", spec)
		}
	}
	if fifths > 7 {
		fifths = 7
	}
	if fifths < -7 {
		fifths = -7
	}
	return model.KeySignature{Fifths: fifths, Mode: model.Major}, nil
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
		if p.pendingEnding != nil {
			p.meas.Ending = p.pendingEnding
			p.endingOpen = p.pendingEnding
			p.pendingEnding = nil
		}
	}
}

// closeMeasure ends the measure in progress. A barline with nothing before it
// closes nothing, which is what lets a leading or doubled barline be harmless.
func (p *parser) closeMeasure(style model.BarlineStyle) {
	if p.meas == nil {
		return
	}
	p.meas.RightBarline = style
	// A volta bracket ends where the repeat it belongs to ends. Closing it
	// here rather than on the next number means a "1." bracket over a bar
	// that is never repeated still gets its hook.
	if p.endingOpen != nil && closesEnding(style) {
		if p.meas.Ending == nil {
			p.meas.Ending = &model.Ending{Numbers: p.endingOpen.Numbers}
		}
		p.meas.Ending.Stop = true
		p.endingOpen = nil
	}
	p.cur.Measures = append(p.cur.Measures, p.meas)
	p.meas, p.voice = nil, nil
	p.accidentals = map[accKey]int{}
	// Broken rhythm pairs two adjacent notes and does not reach across a
	// barline, so an unresolved sign is dropped rather than carried over.
	// Nor does an octave mark waiting for a note that never came.
	p.pendingBroken = 0
	p.pendingOctave = 0
}

func (p *parser) add(e model.Element) {
	p.elements++
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
	m.Texts = append(m.Texts, p.pending.Texts...)
	if p.pending.Chord != nil {
		m.Chord = p.pending.Chord
	}
	if p.pending.Grace != nil {
		m.Grace = p.pending.Grace
	}
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
	// A line that is not music at all — an English sentence continuing the
	// note above it, which several of the books write without the +: the
	// language provides for it — would otherwise be reported one character at
	// a time, forty entries for one line. Watch for a line that yielded
	// nothing but complaints and collapse it into one.
	marked, elements := len(p.skipped), p.elements

	if looksLikeProse(line) {
		p.skip(line, errors.New("not music"))
		return nil
	}

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
				// Not a barline after all, and not worth losing the bar over.
				p.ignore(sc)
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
			if isDigit(sc.peekAt(1)) {
				// "[1" opens a volta bracket in the middle of a line, which
				// is how Prestons1798.abc writes its endings. Read as a chord
				// it asked for a note called 1.
				sc.next()
				if nums := endingNumbers(sc); nums != nil {
					p.pendingEnding = &model.Ending{Numbers: nums, Start: true}
					p.ensureMeasure()
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
			if err := p.quoted(sc); err != nil {
				return err
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
			if err := p.graceGroup(sc); err != nil {
				return err
			}
		case c == '!':
			if err := p.decoration(sc); err != nil {
				return err
			}
		case legacyDecorationNames[c] != "":
			// ABC's one-letter decorations, the shorthand that predates the
			// !name! form: T is a trill, ~ a roll, S a segno. The letters are
			// reserved by the language and none of them is a note letter, so
			// there is nothing to disambiguate.
			sc.next()
			p.applyDecoration(legacyDecorationNames[c])
		case c == 'y':
			// A spacer. It widens the engraving and makes no sound, so
			// reading it as nothing is exactly right.
			sc.next()
			p.length(sc)
		case c == '$':
			// A typesetting-only line-break marker, from "I:linebreak $".
			sc.next()
		case c == ']':
			// A bracket left over from something the reader did not take
			// whole. It carries no music; failing a tune book over it would
			// be absurd.
			sc.next()
		case c == '&':
			// A voice overlay: what follows sounds over the bar just written,
			// as a second voice. gscore has no second voice inside a measure
			// yet, so the overlay is skipped up to the next barline — the one
			// voice that is read is better than the whole book refused.
			sc.next()
			skipped := 0
			for !sc.eof() && sc.peek() != '|' {
				sc.next()
				skipped++
			}
			p.pending = model.Marks{}
			p.note(fmt.Sprintf("dropped an overlaid second voice of %d characters", skipped))
		case c == '\'' || c == ',':
			// An octave mark written before its note instead of after it.
			// "'dd/d/" is how John Moore's second book raises an octave, and
			// "G,,2,G,," is how Rutherford's lowers one.
			sc.next()
			if c == '\'' {
				p.pendingOctave++
			} else {
				p.pendingOctave--
			}
		case c == 't' && sc.peekAt(1) == 'r':
			// "tr" written out in front of the note, which is what a book
			// writes when it does not know about T.
			sc.advance(2)
			p.applyDecoration("trill")
		case isDigit(c):
			// A length that got separated from its note by a space: "BA 2"
			// in SBG_PC_VOL_1.abc means the A is twice as long. Attaching it
			// to the note before is both what was meant and what keeps the
			// bar's arithmetic right.
			n, _ := sc.digits()
			if p.lastElem != nil && n > 0 {
				setElementDuration(p.lastElem, p.unitLen.Scale(n, 1))
			} else {
				p.note(fmt.Sprintf("a length %d with no note before it", n))
			}
		case isLetter(c) && sc.peekAt(1) == ':':
			// A field written after the music on the same line, which is how
			// several of the books add a W: of words or a K: for the next
			// tune. Note letters are matched above, so A to G never land here.
			name := sc.next()
			sc.next() // ':'
			value := stripFieldComment(name, strings.TrimSpace(sc.rest()))
			sc.advance(len(sc.rest()))
			if err := p.field(name, value); err != nil {
				p.note(err.Error())
			}
		case c == 'Z':
			return p.errf("multi-measure rests are not supported yet")
		default:
			p.ignore(sc)
		}
	}
	if p.elements == elements && len(p.skipped)-marked >= 3 {
		p.skipped = p.skipped[:marked]
		p.skip(line, errors.New("not music"))
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
	// One place decides what a decoration means, for both the !name! form
	// and the one-letter shorthand. Two chains of lookups is how !segno!
	// came to be a symbol on a notehead while a bare S was a direction on
	// the bar: the same mark, read two different ways, in the same file.
	p.applyDecoration(name)
	return nil
}

// looksLikeProse reports whether a line is a sentence rather than music.
//
// Several of the tune books continue a note to the player onto the line after
// the N: field it started in, without the +: the language provides for that.
// Read as music, "There should be a quaver rest after the last note" is not
// an error at all: a, e, d, c, b, f and g are note letters, so it quietly
// writes a bar of nonsense into the tune — which is worse than failing, since
// nothing says it happened.
//
// The test is the proportion of letters that cannot be notes. Music is nearly
// all note letters and barlines; a sentence is nearly all the other letters.
// A line with a barline in it is music whatever else it holds, which is what
// keeps a bar full of trills and bowing marks out of this.
func looksLikeProse(line string) bool {
	if strings.ContainsRune(line, '|') {
		return false
	}
	line = withoutNames(line)
	letters, notNotes := 0, 0
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case isNoteLetter(c), c == 'x', c == 'y', c == 'z', c == 'X', c == 'Z':
			letters++
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= 0x80:
			letters++
			notNotes++
		}
	}
	return letters >= 10 && notNotes*100 >= letters*35
}

// withoutNames removes the parts of a music line that are words by design:
// the name inside a !...! decoration, the text inside a "..." chord symbol or
// annotation, and anything after a % comment.
//
// They are spelt out in letters and none of those letters are notes, so
// counting them made "!fermata!c !nosuchmark!e" look more like a sentence
// than like the music it is.
func withoutNames(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		switch c := line[i]; c {
		case '%':
			return b.String()
		case '!', '"':
			if j := strings.IndexByte(line[i+1:], c); j >= 0 {
				i += j + 1
				continue
			}
			// Unterminated: the rest of the line is that name, whatever it
			// was meant to be.
			return b.String()
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ignore steps over a character the reader does not understand and says so.
//
// A tune book has typing mistakes in it — a stray i in Anon2.abc, a ":}" for
// a repeat in Thompson1779.abc. Abandoning the line would throw away the bar
// those mistakes sit in; reading past the character keeps the music and the
// report says what was passed over, so nothing is dropped in silence.
func (p *parser) ignore(sc *scanner) {
	// A whole character, not a byte. The scanner works in bytes because ABC's
	// own syntax is all ASCII, but a curly apostrophe or a degree sign in the
	// middle of a bar is one character to the person who typed it — and
	// stepping over it a byte at a time reported it three times, each time as
	// a piece of mojibake that appears nowhere in their file.
	r, n := utf8.DecodeRuneInString(sc.rest())
	if n <= 0 {
		n = 1
	}
	sc.advance(n)
	p.note(fmt.Sprintf("ignored %q", string(r)))
}

// note records something the reader had to let pass, for the partial read.
//
// The list is capped: a file in the wrong format altogether would otherwise
// produce a note per character, and a report nobody can read is no report.
func (p *parser) note(what string) {
	const most = 40
	if len(p.skipped) >= most {
		return
	}
	p.skipped = append(p.skipped, fmt.Sprintf("line %d: %s", p.lineNo+p.lineOffset, what))
}

// legacyDecorationNames maps ABC's one-letter decorations onto the names the
// !...! form uses, so that both spellings take the same path through the
// parser. H, L, M, u and v have cases of their own further up because they
// were there first.
var legacyDecorationNames = map[byte]string{
	'~': "roll",
	'T': "trill",
	'S': "segno",
	'O': "coda",
	'P': "uppermordent",
	'J': "slide",
	'R': "roll",
}

// jumpDecorations are the decorations that are directions rather than marks
// on a note.
//
// They used to become symbols hung off the next notehead, which drew the
// right picture and did nothing: a tune marked !D.C.! played once and
// stopped. They belong to the bar, because that is what they act on.
var jumpDecorations = map[string]model.Sign{
	"segno": model.Segno,
	"coda":  model.Coda,
	"fine":  model.Fine,
	// ABC has no decoration of its own for "To Coda"; abcm2ps users write it
	// as an annotation, which arrives as text rather than here.
	"tocoda": model.ToCoda,
}

// directionDecorations are the ones that send the player back.
//
// The names are matched with their spaces and capitals taken out, because a
// file writes "D.C. al Fine" and "dacapoalfine" and means the same thing.
var directionDecorations = map[string]string{
	"d.c.": "dc", "dacapo": "dc",
	"d.c.alfine": "dc-fine", "dacapoalfine": "dc-fine",
	"d.c.alcoda": "dc-coda", "dacapoalcoda": "dc-coda", "dacoda": "dc-coda",
	"d.s.": "ds", "dalsegno": "ds",
	"d.s.alfine": "ds-fine", "dalsegnoalfine": "ds-fine",
	"d.s.alcoda": "ds-coda", "dalsegnoalcoda": "ds-coda",
}

// flatten takes the spaces and capitals out of a decoration name, so that the
// half-dozen ways a file spells "D.S. al Coda" all find the same entry.
func flatten(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}

// applyDecoration attaches a decoration, named as the !...! form names it, to
// the next element.
func (p *parser) applyDecoration(name string) {
	// A direction acts on the bar, so it is written there rather than on the
	// note that happens to follow it.
	if sign, ok := jumpDecorations[name]; ok {
		p.ensureMeasure()
		p.meas.Sign = sign
		return
	}
	if dir, ok := directionDecorations[flatten(name)]; ok {
		if j, ok := model.JumpFromName(dir); ok {
			p.ensureMeasure()
			p.meas.Jump = j
			return
		}
	}
	if a, ok := decorationArticulations[name]; ok {
		p.pending.AddArticulation(a)
		return
	}
	if d, ok := decorationDynamics[name]; ok {
		p.pending.Dynamic = d
		return
	}
	if st, ok := decorationStems[name]; ok {
		p.pending.Stem = st
		return
	}
	symbol := name
	if sym, ok := decorationSymbols[name]; ok {
		symbol = sym
	}
	p.pending.Symbols = append(p.pending.Symbols, model.SymbolMark{Name: symbol})
}

// quoted reads a "..." string: a chord symbol over the note, or an annotation
// placed by the character it starts with.
//
// These used to be read and thrown away. A lead sheet is mostly chord
// symbols, so a reader that drops them gets the notes and loses the piece.
func (p *parser) quoted(sc *scanner) error {
	sc.next() // '"'
	var b strings.Builder
	for !sc.eof() && sc.peek() != '"' {
		b.WriteByte(sc.next())
	}
	if sc.eof() {
		return p.errf("unterminated \"...\" text")
	}
	sc.next() // '"'
	s := strings.TrimSpace(b.String())
	if s == "" {
		return nil
	}

	// A leading ^ _ < > @ places an annotation rather than naming a chord:
	// "^No key sig in ms", out of WG1.abc, is a note to the reader and not a
	// chord called No.
	switch s[0] {
	case '^', '_', '<', '>', '@':
		place := model.PlacementAbove
		if s[0] == '_' {
			place = model.PlacementBelow
		}
		p.pending.Texts = append(p.pending.Texts, model.TextMark{
			Text: strings.TrimSpace(s[1:]), Placement: place,
		})
		return nil
	}
	// A number is not a chord. What it is, in every file that has one, is a
	// bar number: somebody wrote the numbering of the score they were
	// copying from into the only syntax ABC has for putting text over a
	// staff. Read as a chord it is set in bold over the music, beside the
	// bar number gscore works out for itself, and the two disagree as often
	// as not — the source counts an anacrusis, or a repeat, differently.
	//
	// So it is dropped and said so. gscore numbers bars itself, and anybody
	// who really wants a number written over a bar has "^7" for it, which
	// is ABC's way of saying "this is an annotation, not a chord".
	if isAllDigits(s) {
		p.note(fmt.Sprintf("%q over bar %d looks like a bar number written as a "+
			"chord symbol, and was left out; gscore numbers the bars itself",
			s, p.barNumber()))
		return nil
	}

	p.pending.Chord = parseChordName(s)
	return nil
}

// barNumber is the bar being read, counting from one, for a message about
// it. Before any music has been seen there is no bar yet and it is the first.
func (p *parser) barNumber() int {
	if p.cur == nil {
		return 1
	}
	return len(p.cur.Measures) + 1
}

// isAllDigits says a string is nothing but digits. Empty is not: an empty
// annotation has already been dropped above.
func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// parseChordName splits a chord symbol into the pieces it is set in: the root
// full size, the quality smaller and raised, the bass note after a slash.
//
// Anything that does not begin like a chord — a word, a direction, "N.C." —
// becomes the symbol's Text instead, which is how the model says "set this
// like a chord symbol but do not pretend it is one".
func parseChordName(s string) *model.ChordSymbol {
	step := s[0]
	if step < 'A' || step > 'G' {
		return &model.ChordSymbol{Text: s}
	}
	i := 1
	for i < len(s) && (s[i] == '#' || s[i] == 'b') {
		i++
	}
	root := s[:i]
	rest := s[i:]

	// A root followed by a letter that cannot start a quality is a word that
	// happens to begin with a note letter: "Chorus", "Bars 1-6", "Ab ovo".
	if rest != "" && isLetter(rest[0]) && !strings.ContainsAny(rest[:1], "mMdaosjAD+-") {
		return &model.ChordSymbol{Text: s}
	}

	c := &model.ChordSymbol{Root: root}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		c.Quality, c.Bass = rest[:i], rest[i+1:]
	} else {
		c.Quality = rest
	}
	return c
}

// graceGroup reads {ge} or {/g} — the little notes crushed in before the next
// one.
//
// They go into the following note's marks rather than into the voice, because
// they take no time out of the bar. The parser used to refuse any file that
// held one, which loses music: the {B} in Thompson1767.abc is an ornament the
// player is meant to play.
func (p *parser) graceGroup(sc *scanner) error {
	sc.next() // '{'
	g := &model.GraceGroup{}
	if sc.peek() == '/' {
		// An acciaccatura: stolen from before the beat rather than taken out
		// of the note it leans on. The stroke through the stem is what says
		// so, and a player does two different things with the two.
		sc.next()
		g.Slashed = true
	}
	for !sc.eof() && sc.peek() != '}' {
		c := sc.peek()
		switch {
		case c == '^' || c == '_' || c == '=' || isNoteLetter(c):
			n, err := p.parseNote(sc)
			if err != nil {
				return err
			}
			g.Notes = append(g.Notes, n)
		default:
			// Slurs, spacing and decorations inside the braces. None of them
			// is worth failing a tune over.
			sc.next()
		}
	}
	if sc.eof() {
		return p.errf("unterminated grace note group")
	}
	sc.next() // '}'
	if len(g.Notes) > 0 {
		p.pending.Grace = g
	}
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
	return p.field(name, stripFieldComment(name, strings.TrimSpace(b.String())))
}

// barlineTokens are the barline spellings, longest first.
//
// Order is the whole point: taking the characters two at a time read ":|]",
// the way a repeated section at the end of a tune is written, as ":|" with a
// stray bracket after it — and the stray bracket failed the file. Matching the
// longest spelling that fits leaves nothing behind.
var barlineTokens = []struct {
	tok   string
	style model.BarlineStyle
}{
	{":||:", model.BarRepeatBoth},
	{":|]", model.BarRepeatEnd},
	{":]", model.BarRepeatEnd},
	{"[|]", model.BarFinal},
	{":|:", model.BarRepeatBoth},
	{"|:|", model.BarRepeatBoth},
	{"||:", model.BarRepeatStart},
	{":||", model.BarRepeatEnd},
	{"::", model.BarRepeatBoth},
	{":|", model.BarRepeatEnd},
	{"|:", model.BarRepeatStart},
	{"|]", model.BarFinal},
	{"[|", model.BarFinal},
	{"||", model.BarDouble},
	{"|", model.BarSingle},
}

// closesEnding reports whether a barline of this kind ends a volta bracket.
func closesEnding(style model.BarlineStyle) bool {
	switch style {
	case model.BarRepeatEnd, model.BarRepeatBoth, model.BarFinal, model.BarDouble:
		return true
	}
	return false
}

func (p *parser) barline(sc *scanner) error {
	r := sc.rest()
	for _, b := range barlineTokens {
		if !strings.HasPrefix(r, b.tok) {
			continue
		}
		sc.advance(len(b.tok))
		p.closeMeasure(b.style)
		// The times-through follow the barline: "|1" and ":|2" are how a
		// tune book written before brackets came into the language spells a
		// first and second ending.
		if nums := endingNumbers(sc); nums != nil {
			p.pendingEnding = &model.Ending{Numbers: nums, Start: true}
		}
		return nil
	}
	return p.errf("unexpected %q", string(sc.peek()))
}

// endingNumbers reads the times-through written after a barline or an opening
// bracket: "1", "1,2", "1-3". It returns nil when there is no number there.
func endingNumbers(sc *scanner) []int {
	if sc.eof() || !isDigit(sc.peek()) {
		return nil
	}
	var out []int
	for {
		n, ok := sc.digits()
		if !ok {
			break
		}
		out = append(out, n)
		if sc.peek() == '-' && isDigit(sc.peekAt(1)) {
			sc.next()
			if to, ok := sc.digits(); ok {
				for v := n + 1; v <= to; v++ {
					out = append(out, v)
				}
			}
		}
		if sc.peek() == ',' && isDigit(sc.peekAt(1)) {
			sc.next()
			continue
		}
		break
	}
	return out
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
	// A ' or , written in front of the note rather than after it means the
	// same thing, so it is added here where both spellings meet.
	octave += p.pendingOctave
	p.pendingOctave = 0

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
		c := sc.peek()
		switch {
		case c == ' ' || c == '\t':
			sc.next()
			continue
		case c == '!':
			// A decoration on one notehead of the chord.
			if err := p.decoration(sc); err != nil {
				return err
			}
			continue
		case c == '.':
			sc.next()
			p.pending.AddArticulation(model.Staccato)
			continue
		case legacyDecorationNames[c] != "":
			// "[F.d]" and "[B.g]" out of Anon2.abc: the mark goes in front of
			// the notehead it belongs to, inside the brackets. Read as a note
			// letter it asked for a note called full stop, and failed the bar.
			sc.next()
			p.applyDecoration(legacyDecorationNames[c])
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
