/*
 * asciitab.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package asciitab reads the tablature guitarists type into text files.
//
//	e|-----0-----3-----|-5h7---------|
//	B|-----1-----0-----|-------5-----|
//	G|-----0-----0-----|-------5-----|
//	D|-----2-----0-----|-------7-----|
//	A|-----3-----2-----|-------7-----|
//	E|-----------3-----|-------5-----|
//
// One line per string, the top line the highest string, hyphens for the
// string and figures for the frets, a pipe for a barline, and letters between
// the figures for what the hand does: h for a hammer-on, p for a pull-off, b
// for a bend. That much every one of them shares. Nothing else is fixed — the
// format was made by people for people and never written down, so tunings,
// rhythms, chord names and lyrics turn up where their writer put them, or not
// at all.
//
// What this reader does with that:
//
// The strings are found by their shape, not by a header: a run of three to
// eight lines that look like strings is a system, and a file is as many
// systems as it has, one after another. The tuning is read off the letters
// at the heads of the lines where there are any, and taken as a guitar's
// otherwise, or a bass's for four lines.
//
// The rhythm is the one thing an ASCII tab does not say, and the one thing a
// score cannot do without. What a tab does say, by how it is spaced, is the
// order and the relative length of its notes: a note lasts from its column
// to the next column anything is struck in. Measured against the bar it is
// in, that gives a fraction of a bar, and the nearest length a note can be
// written in is what it becomes. A tab spaced carelessly comes out with its
// notes in the right order and some of their lengths wrong, which is what the
// page the tab was typed from would have been to a reader too.
//
// Everything the reader passed over is listed, line by line, in the error it
// returns beside the score — a file read in part is a score with a note in it
// saying which lines did not come through, not a refusal.
package asciitab

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

func init() {
	format.RegisterImporter(importer{})
}

type importer struct{}

func (importer) Name() string { return "ASCII tab" }

// Extensions are the ones tabs are saved under. Most are plain .txt, which
// is why that is here: a text file that holds strings of hyphens and frets
// is a tab, and what it is called is up to whoever typed it.
func (importer) Extensions() []string { return []string{".tab", ".btab", ".txt"} }

func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Partial is what is returned beside a score some of whose lines were not
// read: which lines, and why, so that whoever typed the tab can find them.
type Partial struct {
	Lines []string
}

func (p *Partial) Error() string {
	n := len(p.Lines)
	if n == 1 {
		return "asciitab: one line of the file was not read"
	}
	return fmt.Sprintf("asciitab: %d lines of the file were not read", n)
}

func (p *Partial) Partial()          {}
func (p *Partial) Details() []string { return p.Lines }

// A string line, as typed: an optional note name at the head, an optional
// pipe, and then the string itself — hyphens, frets, the marks between them,
// pipes for the bars. The body has to be mostly hyphens and figures, which is
// what tells a string from a line of lyrics with a dash in it.
var stringLine = regexp.MustCompile(
	`^\s*(?:([A-Ga-g][#b]?)\s*)?[|:]?\s*([-0-9hHpPbBrRsS/\\~^xX()tT*=\[\]|:.vV ]+?)\s*$`)

// Import reads a tab into a score of one tablature staff.
func Import(r io.Reader) (*model.Score, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), " \t\r"))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("asciitab: %w", err)
	}

	systems, missed, title := findSystems(lines)
	if len(systems) == 0 {
		return nil, fmt.Errorf("asciitab: no tablature in this file — " +
			"a tab is lines of hyphens with fret numbers on them, one line per string")
	}

	// The tuning is the first system's: a file that changes tuning between
	// systems is a file this reader has not met.
	tab, labelled := tuningOf(systems[0])
	s := model.NewScore()
	s.Identity.Title = title
	p := s.Parts[0]
	p.Name = "Guitar"
	p.Sound = "pluck.guitar.acoustic"
	p.Instrument = model.SteelGuitar
	// Four strings are a bass, and so are five whose lowest reaches below a
	// guitar's bottom E: a five-string bass has a low B, a five-string
	// banjo does not.
	const lowE = 40 // E2, a guitar's bottom string
	if n := len(tab.Strings); n == 4 || n == 5 && tab.Strings[n-1].MIDI() < lowE {
		p.Name = "Bass"
		p.Sound = "pluck.bass.electric"
		p.Instrument = model.ElectricBass
	}
	p.Tab = tab
	p.Clef = model.Treble8vb
	if p.Name == "Bass" {
		p.Clef = model.Bass
	}
	p.Measures = nil

	var notes []string
	for _, sys := range systems {
		if len(sys.lines) != len(tab.Strings) {
			notes = append(notes, fmt.Sprintf(
				"lines %d–%d: a system of %d strings under one of %d; it was read on the first %d",
				sys.first+1, sys.first+len(sys.lines), len(sys.lines), len(tab.Strings),
				min(len(sys.lines), len(tab.Strings))))
		}
		for _, m := range readSystem(sys, tab, &notes) {
			p.Measures = append(p.Measures, m)
		}
	}
	if len(p.Measures) == 0 {
		p.Measures = []*model.Measure{{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}}
	}
	p.Measures[len(p.Measures)-1].RightBarline = model.BarFinal
	_ = labelled
	s.Normalise()

	notes = append(notes, missed...)
	if len(notes) > 0 {
		return s, &Partial{Lines: notes}
	}
	return s, nil
}

// system is one run of string lines: a line of the page.
type system struct {
	first  int      // the line it starts at, counting from zero
	lines  []string // the strings, top first
	heads  []string // the note name at the head of each, or ""
	bodies []string
}

// findSystems picks the runs of string lines out of the text.
//
// Three or more consecutive lines that look like strings are a system. The
// first line of text before any system is taken for the title; every other
// line that is not a string — a chord name over a bar, a verse, a note about
// the tuning — is listed as not read.
func findSystems(lines []string) (systems []system, missed []string, title string) {
	i := 0
	for i < len(lines) {
		if head, body, ok := stringParts(lines[i]); ok {
			sys := system{first: i}
			for i < len(lines) {
				h, b, ok := stringParts(lines[i])
				if !ok || len(sys.lines) >= 8 {
					break
				}
				sys.lines = append(sys.lines, lines[i])
				sys.heads = append(sys.heads, h)
				sys.bodies = append(sys.bodies, b)
				i++
				_ = head
				_ = body
			}
			if len(sys.lines) >= 3 {
				systems = append(systems, sys)
			} else {
				// One or two string-like lines on their own are not a
				// tab — a row of hyphens under a heading, most often.
				for j, l := range sys.lines {
					missed = append(missed, fmt.Sprintf("line %d: %s", sys.first+j+1, strings.TrimSpace(l)))
				}
			}
			continue
		}
		text := strings.TrimSpace(lines[i])
		if text != "" {
			if title == "" && len(systems) == 0 {
				title = text
			} else {
				missed = append(missed, fmt.Sprintf("line %d: %s", i+1, text))
			}
		}
		i++
	}
	return systems, missed, title
}

// stringParts splits a string line into the note name at its head and the
// string itself, and says whether the line is one at all.
func stringParts(line string) (head, body string, ok bool) {
	m := stringLine.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	body = m[2]
	// A string has hyphens in it: a line of lyrics with a dash in it has
	// letters the class above does not allow, which is what keeps it out,
	// and this keeps out a fret number typed on its own. Not "mostly"
	// hyphens — a bar of sixteenths with slides and bends between them is
	// more figure than string, and the top line of a busy system was
	// being dropped for it, which read the system on the wrong strings.
	hyphens := strings.Count(body, "-")
	if hyphens < 3 || hyphens*4 < len(body) {
		return "", "", false
	}
	return m[1], body, true
}

// tuningOf reads the tuning off the heads of a system's lines, or takes the
// one the number of strings suggests.
//
// The heads are note names without octaves — "e B G D A E" — so the octave
// has to be worked out: the top string is taken as high as a guitar's top
// (E4) or lower, and each string below is the next such note downwards.
// Where there are no heads at all, six lines are a guitar, four a bass, five
// a five-string bass and seven a seven-string guitar.
func tuningOf(sys system) (*model.Tablature, bool) {
	n := len(sys.lines)
	labelled := true
	for _, h := range sys.heads {
		if h == "" {
			labelled = false
			break
		}
	}
	if !labelled {
		name := "guitar"
		switch n {
		case 4:
			name = "bass"
		case 5:
			name = "bass — five string"
		case 7:
			name = "guitar — seven string"
		}
		t, _ := model.TuningNamed(name)
		if t != nil && len(t.Strings) == n {
			return t, false
		}
		// An odd count: a guitar's strings as far as they go, and the
		// lowest repeated down a fourth for the rest.
		g, _ := model.TuningNamed("guitar")
		out := &model.Tablature{Frets: 24}
		for i := 0; i < n; i++ {
			if i < len(g.Strings) {
				out.Strings = append(out.Strings, g.Strings[i])
			} else {
				out.Strings = append(out.Strings, pitchFromMIDI(out.Strings[i-1].MIDI()-5))
			}
		}
		return out, false
	}
	// Named strings. The top one sits in the octave that keeps it at or
	// below a guitar's top E, unless there are four strings, where the top
	// is a bass's G2 — and each one after it is the next such note below.
	out := &model.Tablature{Frets: 24}
	ceiling := 64 // E4
	if n == 4 || n == 5 {
		ceiling = 43 // G2
	}
	prev := ceiling + 1
	for _, h := range sys.heads {
		pc, ok := pitchClass(h)
		if !ok {
			pc = 4
		}
		// The highest note of that class strictly below the one before.
		m := prev - 1
		for m%12 != pc {
			m--
		}
		out.Strings = append(out.Strings, pitchFromMIDI(m))
		prev = m
	}
	// A reentrant instrument — a ukulele's low G, a banjo's short fifth
	// string — reads as a dive here; its name would say better, but the
	// tab does not carry it.
	return out, true
}

// pitchClass is a note name as a number of semitones above C.
func pitchClass(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	base := map[byte]int{'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11}
	pc, ok := base[strings.ToLower(name)[0]]
	if !ok {
		return 0, false
	}
	if strings.HasSuffix(name, "#") {
		pc++
	}
	if strings.HasSuffix(name, "b") && len(name) > 1 {
		pc--
	}
	return (pc + 12) % 12, true
}

// pitchFromMIDI spells a note number with sharps, which is how a fretted
// instrument's players name the notes between the naturals.
func pitchFromMIDI(m int) model.Pitch {
	steps := []model.Step{model.StepC, model.StepC, model.StepD, model.StepD, model.StepE,
		model.StepF, model.StepF, model.StepG, model.StepG, model.StepA, model.StepA, model.StepB}
	alters := []int{0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0}
	pc := ((m % 12) + 12) % 12
	return model.Pitch{Step: steps[pc], Octave: m/12 - 1, Alter: alters[pc]}
}

// event is everything struck in one column of a system: the frets on each
// string, and what the hand does on the way to the next one.
type event struct {
	col   int
	frets map[int]int // string (from the top, 0) → fret
	dead  map[int]bool
	ghost map[int]bool
	tech  map[int]model.Technique
	vib   map[int]bool
}

// readSystem turns one system into its bars.
func readSystem(sys system, tab *model.Tablature, notes *[]string) []*model.Measure {
	n := len(tab.Strings)
	if len(sys.bodies) < n {
		n = len(sys.bodies)
	}
	width := 0
	for _, b := range sys.bodies[:n] {
		if len(b) > width {
			width = len(b)
		}
	}
	// The bars are where the pipes are, taken from whichever string has
	// them: most tabs put them on every string, a few on one.
	barCols := map[int]model.BarlineStyle{}
	for _, b := range sys.bodies[:n] {
		for c := 0; c < len(b); c++ {
			switch b[c] {
			case '|':
				style := model.BarSingle
				if c+1 < len(b) && b[c+1] == '|' {
					style = model.BarDouble
				}
				if c > 0 && b[c-1] == ':' {
					style = model.BarRepeatEnd
				}
				if c+1 < len(b) && b[c+1] == ':' {
					if style == model.BarRepeatEnd {
						style = model.BarRepeatBoth
					} else {
						style = model.BarRepeatStart
					}
				}
				if have, ok := barCols[c]; !ok || have == model.BarSingle {
					barCols[c] = style
				}
			}
		}
	}

	// Every column something is struck in, across all the strings.
	events := map[int]*event{}
	at := func(c int) *event {
		e := events[c]
		if e == nil {
			e = &event{col: c, frets: map[int]int{}, dead: map[int]bool{},
				ghost: map[int]bool{}, tech: map[int]model.Technique{}, vib: map[int]bool{}}
			events[c] = e
		}
		return e
	}
	for si := 0; si < n; si++ {
		b := sys.bodies[si]
		var pending model.Technique
		havePending := false
		for c := 0; c < len(b); c++ {
			ch := b[c]
			switch {
			case ch >= '0' && ch <= '9':
				start := c
				for c+1 < len(b) && b[c+1] >= '0' && b[c+1] <= '9' {
					c++
				}
				fret, _ := strconv.Atoi(b[start : c+1])
				// A fret past what the instrument has is two figures
				// that were typed next to each other: "12" on a bass
				// with 20 frets is fine, "97" is a 9 and a 7.
				if fret > tab.Frets && c > start {
					c = start
					fret, _ = strconv.Atoi(b[start : start+1])
				}
				e := at(start)
				e.frets[si] = fret
				if start > 0 && b[start-1] == '(' {
					e.ghost[si] = true
				}
				if havePending {
					e.tech[si] = pending
					havePending = false
				}
			case ch == 'x' || ch == 'X':
				e := at(c)
				e.frets[si] = 0
				e.dead[si] = true
			case ch == 'h' || ch == 'H':
				pending, havePending = model.HammerOn, true
			case ch == 'p' || ch == 'P':
				pending, havePending = model.PullOff, true
			case ch == '/' || ch == 's' || ch == 'S':
				pending, havePending = model.SlideUp, true
			case ch == '\\':
				pending, havePending = model.SlideDown, true
			case ch == 'b' || ch == 'B' || ch == '^':
				pending, havePending = model.BendFull, true
			case ch == 'r' || ch == 'R':
				pending, havePending = model.BendRelease, true
			case ch == 't' || ch == 'T':
				pending, havePending = model.Tap, true
			case ch == '~' || ch == 'v' || ch == 'V':
				// Vibrato belongs to the note before it.
				last := -1
				for cc := range events {
					if cc < c && cc > last {
						if _, ok := events[cc].frets[si]; ok {
							last = cc
						}
					}
				}
				if last >= 0 {
					events[last].vib[si] = true
				}
			}
		}
	}
	if len(events) == 0 {
		return nil
	}

	cols := make([]int, 0, len(events))
	for c := range events {
		cols = append(cols, c)
	}
	sort.Ints(cols)
	bars := make([]int, 0, len(barCols))
	for c := range barCols {
		bars = append(bars, c)
	}
	sort.Ints(bars)

	// The bars, each from one pipe to the next. A system with no pipes is
	// one bar from end to end; a system whose first pipe is at its left
	// edge, as nearly all are, has nothing before it.
	type span struct {
		from, to int
		style    model.BarlineStyle
	}
	var spans []span
	left := 0
	for _, c := range bars {
		if c > left {
			spans = append(spans, span{left, c, barCols[c]})
		} else if c == left && barCols[c] != model.BarSingle && len(spans) > 0 {
			spans[len(spans)-1].style = barCols[c]
		}
		left = c + 1
		if barCols[c] == model.BarDouble {
			left++
		}
	}
	if left < width {
		spans = append(spans, span{left, width, model.BarSingle})
	}

	var out []*model.Measure
	for _, sp := range spans {
		var inBar []int
		for _, c := range cols {
			if c >= sp.from && c < sp.to {
				inBar = append(inBar, c)
			}
		}
		m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: sp.style}
		if len(inBar) == 0 {
			// An empty bar is a bar of rest.
			m.Voices[0].Elements = []model.Element{&model.Rest{Dur: model.Duration{Num: 1, Den: 1}}}
			out = append(out, m)
			continue
		}
		// Each note lasts to the next event, the last to the barline, and
		// the bar is the whole of its width: what fraction of the bar each
		// note takes is what it is written as.
		barWidth := sp.to - sp.from
		// Room before the first note is a rest only where it is a note's
		// worth: a tab pads its bars with a hyphen or two after the pipe,
		// and that is spacing, not silence. The smallest gap between two
		// notes in the bar is what a note's worth is here.
		lead := inBar[0] - sp.from
		gap := barWidth
		for i := 1; i < len(inBar); i++ {
			if g := inBar[i] - inBar[i-1]; g < gap {
				gap = g
			}
		}
		if lead > 0 && lead >= gap && lead*8 >= barWidth {
			m.Voices[0].Elements = append(m.Voices[0].Elements,
				&model.Rest{Dur: quantise(lead, barWidth)})
		}
		for i, c := range inBar {
			var width int
			switch {
			case i+1 < len(inBar):
				width = inBar[i+1] - c
			case i > 0:
				// The last note of a bar is as long as the one before it,
				// not as long as the space left to the barline: a tab pads
				// its bars unevenly, and measured to the pipe the last of
				// four even quarters came out a dotted eighth.
				width = c - inBar[i-1]
			default:
				width = sp.to - c
			}
			dur := quantise(width, barWidth)
			m.Voices[0].Elements = append(m.Voices[0].Elements, element(events[c], tab, dur, n))
		}
		out = append(out, m)
	}
	return out
}

// quantise turns a width in columns, against the width of its bar, into the
// nearest length a note can be written in.
//
// A bar is a whole note's worth of time whatever the meter — which is the
// guess this makes, since the tab does not say — and a note that takes a
// quarter of the bar's columns is a quarter note. The lengths offered are the
// plain values and their dotted forms, down to a thirty-second.
func quantise(cols, barWidth int) model.Duration {
	if barWidth <= 0 || cols <= 0 {
		return model.Duration{Num: 1, Den: 4}
	}
	frac := float64(cols) / float64(barWidth)
	best, bestD := 1e9, model.Duration{Num: 1, Den: 4}
	for _, d := range []model.Duration{
		{Num: 1, Den: 1}, {Num: 3, Den: 4}, {Num: 1, Den: 2}, {Num: 3, Den: 8},
		{Num: 1, Den: 4}, {Num: 3, Den: 16}, {Num: 1, Den: 8}, {Num: 3, Den: 32},
		{Num: 1, Den: 16}, {Num: 1, Den: 32},
	} {
		if diff := abs(frac - d.Float()); diff < best {
			best, bestD = diff, d
		}
	}
	return bestD
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// element makes the note, or chord, struck in one column.
func element(e *event, tab *model.Tablature, dur model.Duration, strings int) model.Element {
	var ns []*model.Note
	for si := 0; si < strings; si++ {
		fret, ok := e.frets[si]
		if !ok {
			continue
		}
		n := &model.Note{
			Pitch:  pitchFromMIDI(tab.Strings[si].MIDI() + fret),
			Dur:    dur,
			String: si + 1,
			Fret:   fret,
		}
		if e.dead[si] {
			n.Head = model.HeadX
		}
		if e.ghost[si] {
			n.Small = true
		}
		if t, ok := e.tech[si]; ok {
			n.Techniques = append(n.Techniques, t)
		}
		if e.vib[si] {
			n.Techniques = append(n.Techniques, model.Vibrato)
		}
		ns = append(ns, n)
	}
	if len(ns) == 1 {
		return ns[0]
	}
	ch := &model.Chord{Dur: dur}
	for _, n := range ns {
		ch.Notes = append(ch.Notes, n)
	}
	return ch
}
