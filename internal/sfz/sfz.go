/*
 * sfz.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package sfz reads SFZ instruments: the text file that says which
// recording to play for which note.
//
// A soundfont and an SFZ instrument answer the same question and answer it
// in opposite ways. An .sf2 is one file with the recordings inside it, laid
// out as a binary structure nobody reads by hand. An .sfz is a plain text
// file beside a folder of WAVs, and every line of it is legible:
//
//	<region> sample=Flute_A4.wav lokey=a3 hikey=c5 pitch_keycenter=a4
//
// That is why most of the free orchestral libraries of the last fifteen
// years are SFZ and not SF2 — Versilian's VSCO 2, which is public domain,
// the Virtual Playing Orchestra, Sonatina. gscore could play none of them,
// because the synthesiser it plays through reads SF2 and nothing else.
//
// This package is the reading half: text in, regions out. What is done with
// them afterwards — finding the samples, handing them to something that can
// make a sound — is not here, so that the part that is only parsing can be
// tested as only parsing.
package sfz

import (
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Region is one recording and the rules for when to play it.
//
// The opcodes are already resolved: what <global>, <master> and <group> said
// is folded in, with the region's own winning. A region is therefore
// complete on its own, which is what the thing reading it wants — nobody
// downstream should have to remember which header a value came from.
type Region struct {
	// Opcodes is every setting that applies here, by its lowercase name.
	Opcodes map[string]string
	// Line is where in the file the region began, for saying where a
	// mistake is.
	Line int
}

// Instrument is an SFZ file, read.
type Instrument struct {
	// Name is what the file was called, for errors and for naming the
	// instrument downstream.
	Name string
	// Regions are in the order the file gave them, which is the order they
	// are matched in: where two cover the same note, SFZ plays both.
	Regions []Region
	// Control is what <control> said — default_path and the offsets — kept
	// because a reader that wants to resolve sample paths itself needs the
	// same answer this one used.
	Control map[string]string
}

// maxInclude is how deep #include may go.
//
// A file that includes itself is a file that never finishes being read, and
// an instrument built out of a hundred nested fragments is a mistake rather
// than a library. Eight is past anything real.
const maxInclude = 8

// Opener finds a file named relative to the one being read.
//
// Passed in rather than reached for, because this package does not know
// what a filesystem is: on a machine it opens a file beside the .sfz, in
// the browser build there is nothing to open, and in a test it hands back a
// string. It is also the only thing an .sfz can reach outside itself, which
// is where a caller puts whatever limits it wants on that.
type Opener func(name string) (io.ReadCloser, error)

// Parse reads an instrument.
//
// name is the file's own name, used to resolve #include and to say where an
// error is. open may be nil where the file includes nothing.
func Parse(r io.Reader, name string, open Opener) (*Instrument, error) {
	text, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("sfz: %s: %w", name, err)
	}
	p := &parser{open: open, defines: map[string]string{}}
	inst := &Instrument{Name: name, Control: map[string]string{}}
	if err := p.file(inst, string(text), name, 0); err != nil {
		return nil, err
	}
	p.flush(inst)
	if len(inst.Regions) == 0 {
		return nil, fmt.Errorf("sfz: %s: no regions, so nothing to play", name)
	}
	return inst, nil
}

type parser struct {
	open    Opener
	defines map[string]string

	// The headers in force, outermost first. A region takes what they say
	// and then its own.
	global, master, group map[string]string
	// region is the one being filled, nil between regions.
	region map[string]string
	// current is the header being read, so an opcode knows where it goes.
	current string
	line    int
}

// flush finishes the region being read, if there is one.
func (p *parser) flush(inst *Instrument) {
	if p.region == nil {
		return
	}
	out := map[string]string{}
	for _, layer := range []map[string]string{p.global, p.master, p.group, p.region} {
		for k, v := range layer {
			out[k] = v
		}
	}
	inst.Regions = append(inst.Regions, Region{Opcodes: out, Line: p.line})
	p.region = nil
}

func (p *parser) file(inst *Instrument, text, name string, depth int) error {
	if depth > maxInclude {
		return fmt.Errorf("sfz: %s: #include goes more than %d deep",
			name, maxInclude)
	}
	for lineNo, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "#define"), strings.HasPrefix(line, "#DEFINE"):
			p.define(line)
			continue
		case strings.HasPrefix(line, "#include"), strings.HasPrefix(line, "#INCLUDE"):
			if err := p.include(inst, line, name, depth); err != nil {
				return err
			}
			continue
		}
		p.line = lineNo + 1
		p.statements(inst, p.expand(line))
	}
	return nil
}

// define records a #define, which is textual substitution of $NAME.
func (p *parser) define(line string) {
	rest := strings.TrimSpace(line[len("#define"):])
	name, value, ok := strings.Cut(rest, " ")
	if !ok {
		return
	}
	p.defines[strings.TrimSpace(name)] = strings.TrimSpace(value)
}

// expand puts the #defines in. Longest name first, so that $vel does not
// eat the start of $velocity.
func (p *parser) expand(line string) string {
	if len(p.defines) == 0 || !strings.Contains(line, "$") {
		return line
	}
	names := make([]string, 0, len(p.defines))
	for k := range p.defines {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, k := range names {
		line = strings.ReplaceAll(line, k, p.defines[k])
	}
	return line
}

func (p *parser) include(inst *Instrument, line, from string, depth int) error {
	rest := strings.TrimSpace(line[len("#include"):])
	rest = strings.Trim(p.expand(rest), `"`)
	if rest == "" {
		return nil
	}
	if p.open == nil {
		return fmt.Errorf("sfz: %s: #include %q, and there is nothing to "+
			"open it with", from, rest)
	}
	// Relative to the file that includes it, the way every other format
	// resolves an include, and with the separator the file uses: an SFZ
	// written on Windows says \ and is read everywhere.
	name := path.Join(path.Dir(from), strings.ReplaceAll(rest, `\`, "/"))
	f, err := p.open(name)
	if err != nil {
		return fmt.Errorf("sfz: %s: #include %q: %w", from, rest, err)
	}
	defer f.Close()
	text, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("sfz: %s: #include %q: %w", from, rest, err)
	}
	return p.file(inst, string(text), name, depth+1)
}

// statements reads the headers and opcodes on one line. SFZ puts no weight
// on line breaks: a region may be written across six lines or crammed onto
// one, and both are ordinary.
func (p *parser) statements(inst *Instrument, line string) {
	for len(line) > 0 {
		line = strings.TrimLeft(line, " \t\r")
		if line == "" {
			return
		}
		if line[0] == '<' {
			end := strings.IndexByte(line, '>')
			if end < 0 {
				return // an unfinished header: nothing more on this line
			}
			p.header(inst, strings.ToLower(line[1:end]))
			line = line[end+1:]
			continue
		}
		name, value, rest := opcode(line)
		if name == "" {
			return // not an opcode and not a header: nothing to be done with it
		}
		p.set(inst, name, value)
		line = rest
	}
}

// header starts a new section, and clears the ones it contains.
func (p *parser) header(inst *Instrument, name string) {
	p.flush(inst)
	switch name {
	case "global":
		p.global, p.master, p.group = map[string]string{}, nil, nil
	case "master":
		p.master, p.group = map[string]string{}, nil
	case "group":
		p.group = map[string]string{}
	case "region":
		p.region = map[string]string{}
	case "control":
		// Not a layer: what it says is about the file rather than about
		// any region, and it holds for everything after it.
		p.region = nil
	default:
		// <curve>, <effect>, <midi>, and whatever a later version of the
		// format adds. Their opcodes are not a region's, so they are read
		// and dropped rather than folded into the next region by accident.
		p.region = nil
	}
	p.current = name
}

// set files an opcode under the header it was written in.
func (p *parser) set(inst *Instrument, name, value string) {
	switch p.current {
	case "global":
		p.global[name] = value
	case "master":
		p.master[name] = value
	case "group":
		p.group[name] = value
	case "region":
		p.region[name] = value
	case "control":
		inst.Control[name] = value
	default:
		// Before any header at all, or inside one this package does not
		// read. Dropped: an opcode with nothing to belong to belongs to
		// nothing.
	}
}

// stripComment removes a // comment, leaving the rest of the line.
//
// A comment begins a word: at the start of a line, or after a space. Taken
// anywhere at all it would cut a path in half — "sample=C:/Sounds//Flute.wav"
// has a doubled separator in it and is a real shape in a file written by
// hand — and a path is harder to get back than a comment is to leave in.
func stripComment(line string) string {
	for i := 0; i+1 < len(line); i++ {
		if line[i] != '/' || line[i+1] != '/' {
			continue
		}
		if i > 0 && line[i-1] != ' ' && line[i-1] != '\t' {
			continue
		}
		return line[:i]
	}
	return line
}

// opcode reads one name=value from the front of a line, and says what is
// left after it.
//
// The whole difficulty of reading SFZ is here. A value may contain spaces —
// sample=Solo Violin Sustain.wav is one filename — so a value does not end
// at the first space. It ends where the next opcode begins, which is the
// last word before the next "name=" on the line. Everything else in the
// format is straightforward and this is the part every reader gets wrong.
func opcode(line string) (name, value, rest string) {
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", ""
	}
	name = strings.ToLower(strings.TrimSpace(line[:eq]))
	if name == "" || strings.ContainsAny(name, " \t<>") {
		return "", "", ""
	}
	after := line[eq+1:]

	// Where does the next opcode start? At the "=" after this one, if any.
	next := strings.IndexByte(after, '=')
	if next < 0 {
		return name, strings.TrimSpace(after), ""
	}
	// The next opcode's name is the word in front of that "=". The value
	// of this one is everything before that word.
	cut := strings.LastIndexAny(after[:next], " \t")
	if cut < 0 {
		// "a=b=c": no space between, so the whole of it is this value.
		return name, strings.TrimSpace(after), ""
	}
	// A header may also end the value: "sample=a.wav <region> sample=b.wav".
	if lt := strings.IndexByte(after, '<'); lt >= 0 && lt < cut {
		return name, strings.TrimSpace(after[:lt]), after[lt:]
	}
	return name, strings.TrimSpace(after[:cut]), after[cut:]
}

/*
 * Reading what a region says
 */

// Str is an opcode's value, or "" where the region does not set it.
func (r Region) Str(name string) string { return r.Opcodes[name] }

// Has says whether the region sets this opcode at all, which is not the
// same as its value being zero: volume=0 is a setting and no volume is not.
func (r Region) Has(name string) bool {
	_, ok := r.Opcodes[name]
	return ok
}

// Int is an opcode read as a whole number, or def where it is not set or
// not one.
func (r Region) Int(name string, def int) int {
	v, ok := r.Opcodes[name]
	if !ok {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// Float is an opcode read as a number, or def.
func (r Region) Float(name string, def float64) float64 {
	v, ok := r.Opcodes[name]
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return def
	}
	return f
}

// Note is an opcode read as a MIDI note number, which SFZ writes either as
// a number or as a name: lokey=60 and lokey=c4 are the same note.
func (r Region) Note(name string, def int) int {
	v, ok := r.Opcodes[name]
	if !ok {
		return def
	}
	n, err := ParseNote(v)
	if err != nil {
		return def
	}
	return n
}

// ErrNotANote is returned by ParseNote for something that is neither a
// number nor a note name.
var ErrNotANote = errors.New("sfz: not a note")

// ParseNote turns an SFZ note into a MIDI number.
//
// Either a number — 60 — or a name: c4, c#4, db4, C4. Middle C is c4 and 60,
// which is the convention SFZ uses and which is one octave up from the
// convention some other formats use for the same number. Getting it wrong
// puts an instrument an octave out, which sounds like a mapping mistake
// rather than a parsing one and is found much later.
func ParseNote(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrNotANote
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	steps := map[byte]int{'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11}
	low := strings.ToLower(s)
	step, ok := steps[low[0]]
	if !ok {
		return 0, ErrNotANote
	}
	i := 1
	for ; i < len(low); i++ {
		switch low[i] {
		case '#':
			step++
		case 'b':
			step--
		default:
			goto octave
		}
	}
octave:
	if i >= len(low) {
		return 0, ErrNotANote
	}
	oct, err := strconv.Atoi(low[i:])
	if err != nil {
		return 0, ErrNotANote
	}
	// c4 is 60: the octave counts from C, and C minus one is the bottom.
	return (oct+1)*12 + step, nil
}

// Sample is the recording a region plays, as a path relative to the folder
// the .sfz is in.
//
// default_path from <control> is already joined on, and the separator is
// made the one this machine uses: an SFZ written on Windows says \ and is
// read everywhere else.
func (inst *Instrument) Sample(r Region) string {
	s := r.Str("sample")
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, `\`, "/")
	if base := inst.Control["default_path"]; base != "" {
		s = path.Join(strings.ReplaceAll(base, `\`, "/"), s)
	}
	return s
}
