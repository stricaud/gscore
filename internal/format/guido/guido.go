/*
 * guido.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package guido reads and writes GUIDO Music Notation.
//
// GMN is a plain-text score language from the GUIDO project, begun at
// Darmstadt in the 1990s and open source at Grame since 2002. A piece is
// braces around comma-separated voices, each voice brackets around a stream
// of notes, chords and tags:
//
//	{ [ \clef<"g2"> \meter<"4/4"> \key<-1> e1/8. \slur(f/16 e/8) d c h0 a/4 ],
//	  [ c1/4 d e/2 ] }
//
// The format carries none of gscore's own code and nothing of the GUIDO
// engine's: it is implemented from the published specification, which is
// what makes it safe to sit beside an MIT program.
//
// # The one thing that is not like us
//
// GUIDO has no measures. A voice is a flat stream of events, and \meter
// carries an autoBarlines setting that defaults to on — the engine bars the
// stream itself. gscore's model is the other way round: a part is a list of
// measures and the notes live inside them.
//
// So the reader has to bar the stream, and that is where the work in this
// file is. It accumulates duration against the meter in force, cuts where
// the bar fills, and honours an explicit \bar wherever one disagrees — which
// is how an upbeat, a cadenza and a mid-piece meter change are written.
// Where the arithmetic will not come out it keeps the music and says so,
// because a bar that does not add up is somebody's piece and not a reason to
// refuse the file.
//
// # What the spelling costs
//
// GUIDO accidentals do not carry through a bar: the spec says an accidental
// is valid only for the note it is written on, so a file repeats the sharp
// on every sharpened note. Conventional notation prints it once. The reader
// therefore decides what to print rather than copying what was written — it
// tracks the bar the way a player reads it, and sets an accidental only
// where one is needed. Copying them straight through put a sharp on every
// note of an F sharp minor bar.
package guido

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
	format.RegisterExporter(exporter{})
}

type importer struct{}

func (importer) Name() string         { return "GUIDO" }
func (importer) Extensions() []string { return []string{".gmn"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// PartialError reports what could not be read. The music that was read comes
// back with it — see format.Partial.
type PartialError struct {
	Skipped []string
}

func (e *PartialError) Error() string {
	if len(e.Skipped) == 1 {
		return "guido: " + e.Skipped[0]
	}
	return fmt.Sprintf("guido: %d things could not be read, the first being: %s",
		len(e.Skipped), e.Skipped[0])
}

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string { return e.Skipped }

// Import reads a GUIDO score.
func Import(r io.Reader) (*model.Score, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("guido: %w", err)
	}
	rd := &reader{vars: map[string]string{}}
	voices, err := rd.parse(string(data))
	if err != nil {
		return nil, err
	}
	if len(voices) == 0 {
		return nil, fmt.Errorf("guido: there is no music in this file")
	}
	s := rd.build(voices)
	if len(rd.skipped) > 0 {
		return s, &PartialError{Skipped: rd.skipped}
	}
	return s, nil
}

// reader is one read of one file: what the whole score says, and what could
// not be read.
type reader struct {
	skipped []string
	vars    map[string]string

	// What the header tags said. They belong to the piece rather than to the
	// voice they happen to be written in.
	title    string
	composer string
	tempo    *model.Tempo
}

// most is how many complaints are worth keeping. A file whose every line is
// unreadable produces one complaint per line, and nobody reads the
// four-hundredth.
const most = 40

func (rd *reader) complain(line int, format string, a ...any) {
	if len(rd.skipped) >= most {
		return
	}
	rd.skipped = append(rd.skipped,
		fmt.Sprintf("line %d: %s", line, fmt.Sprintf(format, a...)))
}

/*
 * Reading the text
 */

// scanner walks the file. Whitespace and comments are skipped between
// tokens, never inside one: the spec is explicit that no white space may
// appear within a note description, which is what makes "c1/4 d" two notes
// and not one.
type scanner struct {
	src  string
	pos  int
	line int
}

func newScanner(src string) *scanner { return &scanner{src: src, line: 1} }

func (s *scanner) eof() bool { return s.pos >= len(s.src) }

func (s *scanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.src[s.pos]
}

func (s *scanner) at(i int) byte {
	if s.pos+i >= len(s.src) {
		return 0
	}
	return s.src[s.pos+i]
}

func (s *scanner) advance(n int) {
	for i := 0; i < n && !s.eof(); i++ {
		if s.src[s.pos] == '\n' {
			s.line++
		}
		s.pos++
	}
}

// skip eats whitespace and comments.
//
// Two kinds, both from the spec: % runs to the end of its line, and (* … *)
// spans lines and nests. The nesting is not decoration — a commented-out
// passage that itself holds a comment is exactly what somebody does when
// they take four bars out of a piece to try something.
func (s *scanner) skip() {
	for !s.eof() {
		switch c := s.peek(); {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.advance(1)
		case c == '%':
			for !s.eof() && s.peek() != '\n' {
				s.advance(1)
			}
		case c == '(' && s.at(1) == '*':
			depth := 0
			for !s.eof() {
				if s.peek() == '(' && s.at(1) == '*' {
					depth++
					s.advance(2)
					continue
				}
				if s.peek() == '*' && s.at(1) == ')' {
					depth--
					s.advance(2)
					if depth == 0 {
						break
					}
					continue
				}
				s.advance(1)
			}
		default:
			return
		}
	}
}

func (s *scanner) accept(c byte) bool {
	s.skip()
	if s.peek() == c {
		s.advance(1)
		return true
	}
	return false
}

/*
 * What a file says
 */

// symbol is one thing written in a voice: an event or a tag.
type symbol interface{ where() int }

// written is one notehead as the file spelled it.
//
// The alteration is kept as written rather than resolved, because what
// sounds depends on the key signature in force and what prints depends on
// the bar — neither of which is known until the stream has been barred.
type written struct {
	step model.Step
	// alter is the semitones the spelling asks for, whether they came
	// from a chromatic name like "cis" or from the accidentals after it.
	alter   int
	octave  int
	hasOct  bool
	percent bool // the "empty" note: takes time and is not printed
}

// event is a note, a rest or a chord.
type event struct {
	line   int
	rest   bool
	notes  []written
	dur    model.Duration
	hasDur bool
}

func (e *event) where() int { return e.line }

// tag is \name, \name<params>, \name(body) or \name<params>(body).
type tag struct {
	line   int
	name   string
	num    int // the :n that pairs overlapping ranges; 0 is none
	params []param
	body   []symbol
	ranged bool
}

func (t *tag) where() int { return t.line }

// param is one tag argument, named or not.
type param struct {
	key   string
	text  string
	num   float64
	isNum bool
}

// str is the parameter's value as text, whichever way it was written.
func (p param) str() string {
	if p.isNum {
		return strconv.FormatFloat(p.num, 'g', -1, 64)
	}
	return p.text
}

// pick finds a parameter by name, or by position among the unnamed ones.
//
// GUIDO allows both — \tuplet<"3","above"> and \tuplet<format="3",
// position="above"> are the same tag — so every reading of a parameter has
// to try both ways round.
func pick(params []param, name string, at int) (param, bool) {
	for _, p := range params {
		if p.key != "" && strings.EqualFold(p.key, name) {
			return p, true
		}
	}
	n := 0
	for _, p := range params {
		if p.key != "" {
			continue
		}
		if n == at {
			return p, true
		}
		n++
	}
	return param{}, false
}

/*
 * The parser
 */

// parse reads the whole file: the variables, then the score.
func (rd *reader) parse(src string) ([][]symbol, error) {
	s := newScanner(src)
	rd.variables(s)

	s.skip()
	if s.eof() {
		return nil, fmt.Errorf("guido: this file holds no score")
	}

	// A score is braces around voices, or a single bracketed voice on its
	// own. The brace is also a chord inside a voice, which is not ambiguous
	// here: at the top level nothing but a voice list can follow it.
	if s.peek() == '{' {
		s.advance(1)
		var voices [][]symbol
		for {
			s.skip()
			if s.eof() {
				rd.complain(s.line, "the score is not closed")
				break
			}
			if s.peek() == '}' {
				s.advance(1)
				break
			}
			v, err := rd.voice(s)
			if err != nil {
				return nil, err
			}
			voices = append(voices, v)
			if !s.accept(',') {
				s.skip()
				if s.peek() == '}' {
					s.advance(1)
				}
				break
			}
		}
		return voices, nil
	}

	v, err := rd.voice(s)
	if err != nil {
		return nil, err
	}
	return [][]symbol{v}, nil
}

// variables reads the $name = value assignments a file may open with.
func (rd *reader) variables(s *scanner) {
	for {
		s.skip()
		if s.peek() != '$' {
			return
		}
		at := s.pos
		s.advance(1)
		name := s.word()
		s.skip()
		if name == "" || s.peek() != '=' {
			// Not an assignment after all — a variable used where a score
			// was expected. Put it back and let the score parser deal.
			s.pos = at
			return
		}
		s.advance(1)
		s.skip()
		rd.vars[name] = s.value()
	}
}

// word reads a bare name: a letter or underscore, then letters, digits and
// underscores.
func (s *scanner) word() string {
	start := s.pos
	for !s.eof() {
		c := s.peek()
		ok := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(s.pos > start && c >= '0' && c <= '9')
		if !ok {
			break
		}
		s.advance(1)
	}
	return s.src[start:s.pos]
}

// value reads a variable's value: a quoted string or a run of characters.
func (s *scanner) value() string {
	if s.peek() == '"' {
		return s.quoted()
	}
	start := s.pos
	for !s.eof() && s.peek() != '\n' && s.peek() != ';' {
		s.advance(1)
	}
	return strings.TrimSpace(s.src[start:s.pos])
}

// quoted reads a "…" string, with \" for a quote inside one.
func (s *scanner) quoted() string {
	s.advance(1) // the opening quote
	var b strings.Builder
	for !s.eof() {
		c := s.peek()
		if c == '\\' && s.at(1) == '"' {
			b.WriteByte('"')
			s.advance(2)
			continue
		}
		if c == '"' {
			s.advance(1)
			break
		}
		b.WriteByte(c)
		s.advance(1)
	}
	return b.String()
}

// voice reads one [ … ] sequence.
//
// A voice that does not open with its bracket is read anyway, as far as the
// comma or brace that ends it. One missing character is a typing mistake
// and not a reason to hand back nothing: the music after it is still there,
// and refusing the file would be the one outcome that helps nobody.
func (rd *reader) voice(s *scanner) ([]symbol, error) {
	s.skip()
	if s.peek() != '[' {
		rd.complain(s.line, "this voice does not open with '['")
		return rd.symbols(s, 0), nil
	}
	s.advance(1)
	return rd.symbols(s, ']'), nil
}

// symbols reads until the closing character, which it consumes.
func (rd *reader) symbols(s *scanner, until byte) []symbol {
	var out []symbol
	for {
		s.skip()
		if s.eof() {
			if until != 0 {
				rd.complain(s.line, "the music ends before its closing %q",
					string(until))
			}
			return out
		}
		c := s.peek()
		if c == until {
			s.advance(1)
			return out
		}
		switch {
		case c == '\\':
			if t := rd.tag(s); t != nil {
				out = append(out, t)
			}
		case c == '|':
			// The shorthand for \bar, and the commonest character in a
			// real file.
			s.advance(1)
			out = append(out, &tag{line: s.line, name: "bar"})
		case c == '{':
			if e := rd.chord(s); e != nil {
				out = append(out, e)
			}
		case c == '$':
			s.advance(1)
			name := s.word()
			text, ok := rd.vars[name]
			if !ok {
				rd.complain(s.line, "$%s has no value", name)
				continue
			}
			inner := newScanner(text)
			inner.line = s.line
			out = append(out, rd.symbols(inner, 0)...)
		case c == ',' || c == ')' || c == ']' || c == '}':
			// A closing character that is not ours: the caller's business.
			return out
		default:
			e, ok := rd.event(s)
			if !ok {
				// Nothing here reads as music. Skip the character rather
				// than stopping: one stray bracket in a long file is not a
				// reason to lose the rest of the piece.
				rd.complain(s.line, "cannot read %q", string(c))
				s.advance(1)
				continue
			}
			out = append(out, e)
		}
	}
}

// tag reads \name[:n][<params>][(body)].
func (rd *reader) tag(s *scanner) *tag {
	line := s.line
	s.advance(1) // the backslash
	name := s.word()
	if name == "" {
		rd.complain(line, "a tag with no name")
		return nil
	}
	t := &tag{line: line, name: name}
	if s.peek() == ':' && isDigit(s.at(1)) {
		s.advance(1)
		t.num = s.integer()
	}
	if s.peek() == '<' {
		s.advance(1)
		t.params = rd.params(s)
	}
	// The body, if this is a range. Written with no space between the
	// parameters and the bracket, but files are written by people.
	s.skip()
	if s.peek() == '(' {
		s.advance(1)
		t.ranged = true
		t.body = rd.symbols(s, ')')
	}
	return t
}

// params reads a tag's argument list, up to the closing '>'.
func (rd *reader) params(s *scanner) []param {
	var out []param
	for {
		s.skip()
		if s.eof() {
			rd.complain(s.line, "a tag's parameters are not closed")
			return out
		}
		if s.peek() == '>' {
			s.advance(1)
			return out
		}
		if s.peek() == ',' {
			s.advance(1)
			continue
		}

		var p param
		// A name in front of an '=' is the parameter's name, not its value.
		save := s.pos
		if w := s.word(); w != "" {
			s.skip()
			if s.peek() == '=' {
				s.advance(1)
				p.key = w
				s.skip()
			} else {
				s.pos = save
			}
		}
		switch {
		case s.peek() == '"':
			p.text = s.quoted()
		case s.peek() == '$':
			s.advance(1)
			p.text = rd.vars[s.word()]
		case isDigit(s.peek()) || s.peek() == '-' || s.peek() == '+' ||
			s.peek() == '.':
			p.num = s.number()
			p.isNum = true
			// A unit suffix — hs, pt, cm — is a measurement and none of our
			// business; the value is what matters.
			s.word()
		default:
			start := s.pos
			for !s.eof() && s.peek() != ',' && s.peek() != '>' {
				s.advance(1)
			}
			p.text = strings.TrimSpace(s.src[start:s.pos])
		}
		out = append(out, p)
	}
}

// chord reads { note, note, … }.
//
// Tags written inside a chord are applied to the chord as a whole: gscore
// hangs a tie or an articulation off the chord and not off one of its
// noteheads, which is what a player reads anyway.
func (rd *reader) chord(s *scanner) *event {
	line := s.line
	s.advance(1) // the brace
	e := &event{line: line}
	for {
		s.skip()
		if s.eof() {
			rd.complain(line, "a chord is not closed")
			break
		}
		if s.peek() == '}' {
			s.advance(1)
			break
		}
		if s.peek() == ',' {
			s.advance(1)
			continue
		}
		if s.peek() == '\\' {
			// A tag on a chord member. Read it so the text advances; what
			// it says is kept by the walker through the chord itself.
			rd.tag(s)
			continue
		}
		n, ok := rd.event(s)
		if !ok {
			rd.complain(s.line, "cannot read %q in a chord", string(s.peek()))
			s.advance(1)
			continue
		}
		if n.rest {
			continue
		}
		e.notes = append(e.notes, n.notes...)
		// The chord's duration is the longest of its notes, as the spec
		// says, and a note that omits one takes the chord's.
		if n.hasDur && (!e.hasDur || n.dur.Cmp(e.dur) > 0) {
			e.dur, e.hasDur = n.dur, true
		}
	}
	if len(e.notes) == 0 {
		return nil
	}
	return e
}

// noteNames are the spellings a note can be written with, longest first so
// that "sol" is read before "s" would be and "cis" before "c".
//
// Three systems, as the spec has them: diatonic, chromatic and solfège. The
// chromatic names carry their own sharp, which is why they are listed with
// an alteration: "cis" is the same sound as "c#" and the spec is explicit
// that they are not the same spelling.
var noteNames = []struct {
	text  string
	step  model.Step
	alter int
}{
	{"empty", model.StepC, 0}, // a note that takes time and is not printed
	{"sol", model.StepG, 0},
	{"cis", model.StepC, 1},
	{"dis", model.StepD, 1},
	{"fis", model.StepF, 1},
	{"gis", model.StepG, 1},
	{"ais", model.StepA, 1},
	{"do", model.StepC, 0},
	{"re", model.StepD, 0},
	{"mi", model.StepE, 0},
	{"me", model.StepE, 0},
	{"fa", model.StepF, 0},
	{"la", model.StepA, 0},
	{"si", model.StepB, 0},
	{"ti", model.StepB, 0},
	{"c", model.StepC, 0},
	{"d", model.StepD, 0},
	{"e", model.StepE, 0},
	{"f", model.StepF, 0},
	{"g", model.StepG, 0},
	{"a", model.StepA, 0},
	// h is the German B natural and b the international one. They are the
	// same pitch class: a file that means B flat writes "h&" or "b&", and
	// reading b as B flat would put every one of them a semitone low.
	{"h", model.StepB, 0},
	{"b", model.StepB, 0},
}

// event reads one note or rest.
func (rd *reader) event(s *scanner) (*event, bool) {
	line := s.line
	e := &event{line: line}

	if s.peek() == '_' {
		s.advance(1)
		e.rest = true
	} else {
		var w written
		matched := ""
		for _, n := range noteNames {
			if !strings.HasPrefix(s.src[s.pos:], n.text) {
				continue
			}
			// A name is only a name if what follows it cannot continue one:
			// "fa" must not be read as "f" followed by nonsense, and the
			// longest-first order above is what settles that.
			matched, w.step, w.alter = n.text, n.step, n.alter
			break
		}
		if matched == "" {
			return nil, false
		}
		s.advance(len(matched))
		if matched == "empty" {
			w.percent = true
		}
		// A MIDI pitch in angle brackets overrides what the name spells.
		// Read past it: the spelling is what a score is written in.
		if s.peek() == '<' {
			for !s.eof() && s.peek() != '>' {
				s.advance(1)
			}
			s.advance(1)
		}
		for s.peek() == '#' || s.peek() == '&' {
			if s.peek() == '#' {
				w.alter++
			} else {
				w.alter--
			}
			s.advance(1)
		}
		if isDigit(s.peek()) || ((s.peek() == '-' || s.peek() == '+') &&
			isDigit(s.at(1))) {
			w.octave = s.integer()
			w.hasOct = true
		}
		e.notes = []written{w}
	}

	if d, ok := s.duration(); ok {
		e.dur, e.hasDur = d, true
	}
	return e, true
}

// duration reads *n/m, *n or /m, with up to three dots after it.
func (s *scanner) duration() (model.Duration, bool) {
	d := model.Duration{Num: 1, Den: 1}
	got := false
	switch {
	case s.peek() == '*':
		s.advance(1)
		n := s.integer()
		if strings.HasPrefix(s.src[s.pos:], "ms") {
			// The millisecond form, at the spec's sixty quarter notes a
			// minute: a thousand milliseconds is a quarter.
			s.advance(2)
			return model.Duration{Num: n, Den: 4000}.Reduced(), true
		}
		d = model.Duration{Num: n, Den: 1}
		if s.peek() == '/' {
			s.advance(1)
			d.Den = s.integer()
		}
		got = true
	case s.peek() == '/':
		s.advance(1)
		d = model.Duration{Num: 1, Den: s.integer()}
		got = true
	}
	if !got {
		return d, false
	}
	dots := 0
	for s.peek() == '.' {
		// A dot only counts where a digit does not follow it: "1/4." is a
		// dotted quarter and "0.5" is a number.
		if isDigit(s.at(1)) {
			break
		}
		dots++
		s.advance(1)
	}
	switch dots {
	case 1:
		d = model.Duration{Num: d.Num * 3, Den: d.Den * 2}
	case 2:
		d = model.Duration{Num: d.Num * 7, Den: d.Den * 4}
	case 3:
		d = model.Duration{Num: d.Num * 15, Den: d.Den * 8}
	}
	if d.Den == 0 {
		return model.Duration{Num: 1, Den: 4}, true
	}
	return d, true
}

func (s *scanner) integer() int {
	neg := false
	if s.peek() == '-' || s.peek() == '+' {
		neg = s.peek() == '-'
		s.advance(1)
	}
	n := 0
	for isDigit(s.peek()) {
		n = n*10 + int(s.peek()-'0')
		s.advance(1)
	}
	if neg {
		return -n
	}
	return n
}

func (s *scanner) number() float64 {
	start := s.pos
	if s.peek() == '-' || s.peek() == '+' {
		s.advance(1)
	}
	for isDigit(s.peek()) || s.peek() == '.' {
		s.advance(1)
	}
	f, _ := strconv.ParseFloat(strings.TrimPrefix(s.src[start:s.pos], "+"), 64)
	return f
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
