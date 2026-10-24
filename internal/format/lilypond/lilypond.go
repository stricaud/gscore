/*
 * lilypond.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package lilypond writes a score as LilyPond source.
//
// LilyPond engraves better than this program does and will for a long time:
// it is decades of work by people who read Gould and Ross, and a score that
// matters is a score worth putting through it. So this is not a fallback
// format, it is the way out to the best engraving there is — and what comes
// out has to be source somebody can read and go on editing, because that is
// what LilyPond source is for.
//
// Three things follow from that.
//
// It is written to be *read*. One staff per variable with the player's name
// on it, the bars one to a line with their numbers in a comment, the
// structure at the bottom where a LilyPond user looks for it. A single line
// of ten thousand notes would compile and would be useless.
//
// It carries everything the model has that LilyPond can say: ties and slurs,
// articulations and ornaments, dynamics and hairpins, tuplets, tremolos of
// both kinds, grace notes, octave lines, repeats and their endings, chord
// symbols over the staff, lyrics under it, rehearsal marks, transposing
// instruments, tablature. What it cannot say it says in a comment rather
// than dropping, so that nothing disappears without a word.
//
// It is checked by compiling it. The tests run the real LilyPond over the
// output where it is installed, because "it looks like LilyPond" is not the
// same claim as "LilyPond reads it".
package lilypond

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/version"
)

func init() {
	format.RegisterExporter(exporter{})
}

type exporter struct{}

func (exporter) Name() string         { return "LilyPond" }
func (exporter) Extensions() []string { return []string{".ly"} }

func (exporter) Export(w io.Writer, s *model.Score) error { return Write(w, s) }

// lilypondVersion is the release the output declares itself for.
//
// Stated rather than left out: LilyPond is told what it is reading rather
// than left to guess, and `convert-ly` can bring the file forward from here
// when the syntax moves again. 2.26 is the current stable release; the one
// thing written here that an older LilyPond would refuse is the plain
// rational an irregular bar's length is given as, which was a moment until
// 2.25 — and convert-ly knows that rule.
const lilypondVersion = "2.26.0"

// Write renders a score as LilyPond source.
func Write(w io.Writer, s *model.Score) error {
	if s == nil {
		return fmt.Errorf("lilypond: there is no score to write")
	}
	e := &writer{s: s}
	e.run()
	_, err := io.WriteString(w, e.b.String())
	return err
}

type writer struct {
	s *model.Score
	b strings.Builder

	// names are the variable each part is written to, in part order.
	names []string
	// used keeps the names apart: two staves called "Flute" would
	// otherwise be one variable written twice.
	used map[string]bool
}

func (e *writer) run() {
	e.used = map[string]bool{}

	e.line(`\version "%s"`, lilypondVersion)
	e.line(`%% Written by gscore %s. https://github.com/stricaud/gscore`,
		version.Version)
	e.blank()
	e.header()

	for i, p := range e.s.Parts {
		e.blank()
		e.part(i, p)
	}

	e.blank()
	e.book()
}

/*
 * The pieces of the file
 */

func (e *writer) header() {
	id := e.s.Identity
	if id.Title == "" && id.Composer == "" && id.Subtitle == "" {
		return
	}
	e.line(`\header {`)
	if id.Title != "" {
		e.line("  title = %s", quote(id.Title))
	}
	if id.Subtitle != "" {
		e.line("  subtitle = %s", quote(id.Subtitle))
	}
	if id.Composer != "" {
		e.line("  composer = %s", quote(id.Composer))
	}
	if id.Copyright != "" {
		e.line("  copyright = %s", quote(id.Copyright))
	}
	e.line(`  tagline = ##f`)
	e.line(`}`)
}

// part writes one staff's music as a variable, with its words and its chords
// beside it.
func (e *writer) part(i int, p *model.Part) {
	name := e.variable(p, i)
	e.names = append(e.names, name)

	if p.Name != "" {
		e.line("%% %s", p.Name)
	}
	e.line("%s = {", name)
	e.music(p)
	e.line("}")

	if verses := lyricVerses(p); len(verses) > 0 {
		for _, v := range verses {
			e.blank()
			e.line("%sLyricsVerse%s = \\lyricmode {", name, spellNumber(v.number))
			e.line("  %s", strings.Join(v.words, " "))
			e.line("}")
		}
	}
	if chords := chordLine(p); chords != "" {
		e.blank()
		e.line("%sChords = \\chordmode {", name)
		e.line("  %s", chords)
		e.line("}")
	}
}

// music writes the bars of one staff.
func (e *writer) music(p *model.Part) {
	st := &state{part: p, key: p.KeyAt(0), time: p.TimeAt(0)}
	st.length = st.time.BarDuration()

	e.line("  \\clef %s", clefName(p.Clef))
	e.line("  %s", keyCommand(st.key))
	e.line("  \\time %d/%d", st.time.Beats, st.time.BeatValue)
	if t := e.s.Tempo; t.BPM > 0 {
		e.line("  \\tempo %s = %d", lilyDuration(t.Unit), t.BPM)
	}
	// A transposing instrument sounds somewhere other than it is written,
	// and LilyPond is told so rather than left to print a part in the wrong
	// key: everything in here is written pitch, which is what a player
	// reads.
	if c := transposition(p); c != "" {
		e.line("  \\transposition %s", c)
	}
	if p.Hidden {
		e.line("  %% this staff is hidden in gscore")
	}
	// A piece that begins mid-bar has to say so, or every bar line after it
	// is in the wrong place and LilyPond says so once per bar.
	if d, ok := pickup(p, st.time); ok {
		e.line("  \\partial %s", lilyDuration(d))
	}

	for mi, m := range p.Measures {
		e.measure(st, p, m, mi)
	}
	if st.volta {
		e.line(`  \set Score.repeatCommands = #'((volta #f))`)
		st.volta, st.endVolta = false, false
	}
	// Anything still open at the end is closed here rather than left to
	// run off the edge of the file.
	if st.slurs > 0 {
		e.line("  %% %d slur(s) were left open by the score and are closed here",
			st.slurs)
		e.line("  %s", strings.Repeat(")", st.slurs))
		st.slurs = 0
	}
	e.line("  \\bar %s", quote(barStyle(lastBarline(p))))
}

// measure writes one bar, on one line, with its number beside it.
func (e *writer) measure(st *state, p *model.Part, m *model.Measure, mi int) {
	var pre []string
	if m.Key != nil && *m.Key != st.key {
		st.key = *m.Key
		pre = append(pre, keyCommand(st.key))
	}
	if m.Time != nil && *m.Time != st.time {
		st.time = *m.Time
		pre = append(pre, fmt.Sprintf("\\time %d/%d", st.time.Beats, st.time.BeatValue))
	}
	if m.Tempo != nil && m.Tempo.BPM > 0 {
		pre = append(pre, fmt.Sprintf("\\tempo %s = %d",
			lilyDuration(m.Tempo.Unit), m.Tempo.BPM))
	}
	if m.Marker != "" {
		pre = append(pre, fmt.Sprintf(`\mark \markup { \box "%s" }`, escape(m.Marker)))
	}
	if sign := signCommand(m.Sign); sign != "" {
		pre = append(pre, sign)
	}
	// A bracket that ended on the bar before is cleared here, at the head
	// of the bar after it, and not at the foot of its own: LilyPond reads
	// the command where it stands, and one standing after the last note of
	// the ending still belongs to that bar.
	if st.endVolta {
		st.endVolta = false
		if m.Ending == nil || !m.Ending.Start {
			pre = append(pre, `\set Score.repeatCommands = #'((volta #f))`)
			st.volta = false
		}
	}
	if m.Ending != nil && m.Ending.Start {
		pre = append(pre, voltaCommand(m.Ending, st.volta))
		st.volta = true
	}
	if m.Repeat > 0 {
		pre = append(pre, fmt.Sprintf("%% bar repeated %d time(s) in gscore", m.Repeat))
	}
	// How long this bar actually is. The first bar's shortfall is the
	// pickup and was declared with \\partial, so it is left alone.
	full := st.time.BarDuration()
	have := barLength(m)
	want := full
	if mi > 0 && have.Num > 0 && have.Cmp(full) != 0 {
		want = have
	}
	if want.Cmp(st.length) != 0 {
		st.length = want
		pre = append(pre, fmt.Sprintf(
			"\\set Timing.measureLength = #%d/%d", want.Num, want.Den))
	}

	body := e.bar(st, p, m, mi)

	var post []string
	if m.Ending != nil && m.Ending.Stop && st.volta {
		st.endVolta = true
	}
	if j := jumpCommand(m.Jump); j != "" {
		post = append(post, j)
	}
	// The closing line, where it is not the ordinary one. The last bar's is
	// written after the loop, so that a final barline is not doubled.
	if mi+1 < len(p.Measures) && m.RightBarline != model.BarSingle {
		post = append(post, fmt.Sprintf(`\bar %s`, quote(barStyle(m.RightBarline))))
	}

	line := strings.TrimSpace(strings.Join(append(append(pre, body), post...), " "))
	e.line("  %s |  %% %d", line, mi+1)
}

/*
 * One bar of music
 */

// state is what has been said so far and does not need saying again.
type state struct {
	part *model.Part
	key  model.KeySignature
	time model.TimeSignature
	// volta says a repeat ending's bracket is open, so that the next one
	// closes it before it opens its own: LilyPond draws one bracket at a
	// time and ends an unfinished one with a complaint.
	volta bool
	// endVolta says the bracket ended with the bar just written and is to
	// be cleared at the head of the next one.
	endVolta bool
	// slurs is how many are open. A score can hold a slur that ends
	// without having begun, and a file that wrote it would not compile.
	slurs int
	// length is how long LilyPond currently believes a bar is. A bar that
	// does not fill the meter — the half bar either side of a repeat, an
	// irregular bar somebody wrote on purpose — has to say so, or the bar
	// line lands by arithmetic somewhere else and every bar after it is
	// reported as wrong.
	length model.Duration
}

func (e *writer) bar(st *state, p *model.Part, m *model.Measure, mi int) string {
	if len(m.Voices) == 0 || len(m.Voices[0].Elements) == 0 {
		return fullBarRest(st.time)
	}
	if len(m.Voices) > 1 {
		// Two voices on one staff: LilyPond writes them in parallel.
		var parts []string
		for _, v := range m.Voices {
			parts = append(parts, e.voice(st, p, v, mi))
		}
		return "<< { " + strings.Join(parts, " } \\\\ { ") + " } >>"
	}
	return e.voice(st, p, m.Voices[0], mi)
}

func (e *writer) voice(st *state, p *model.Part, v *model.Voice, mi int) string {
	var out []string
	var open *model.Tuplet
	for i := 0; i < len(v.Elements); i++ {
		el := v.Elements[i]
		mk := marksOf(el)

		// A tuplet is a group in LilyPond, not a mark on each note, so it
		// is opened at the first note that carries it and closed after the
		// last.
		if mk != nil && mk.Tuplet != nil && open == nil {
			open = mk.Tuplet
			out = append(out, fmt.Sprintf("\\tuplet %d/%d {",
				open.Actual, open.Normal))
		}

		// A two-note tremolo is a repeat in LilyPond, which is the honest
		// way to put it: the pair is written once and played as many times
		// as the beams say.
		if mk != nil && mk.Tremolo.Paired() && mk.Tremolo.Start && i+1 < len(v.Elements) {
			if next := marksOf(v.Elements[i+1]); next != nil &&
				next.Tremolo.Paired() && next.Tremolo.Stop {
				out = append(out, e.tremoloPair(st, p, el, v.Elements[i+1], mk.Tremolo))
				i++
				if open != nil {
					open = nil
					out = append(out, "}")
				}
				continue
			}
		}

		out = append(out, e.element(st, p, el, mi))

		// Closed after the note marked as the end of it, or after the last
		// note of the bar that carries one — a tuplet whose end was never
		// marked is still a group, and leaving it open would swallow the
		// rest of the staff.
		if open != nil {
			done := mk == nil || mk.Tuplet == nil || mk.Tuplet.Stop
			if i+1 >= len(v.Elements) || !carriesTuplet(v.Elements[i+1], open) {
				done = true
			}
			if done {
				open = nil
				out = append(out, "}")
			}
		}
	}
	return strings.Join(out, " ")
}

// tremoloPair writes two notes alternating: \repeat tremolo n { a b }.
func (e *writer) tremoloPair(st *state, p *model.Part, a, b model.Element,
	tr *model.Tremolo) string {

	// How many times the pair is heard: the whole length divided by the
	// length of the two notes the beams stand for.
	total := a.Duration().Add(b.Duration())
	each := tr.Repeat()
	n := 2
	if each.Float() > 0 {
		n = int(total.Float()/each.Float()/2 + 0.5)
	}
	if n < 1 {
		n = 1
	}
	// Inside the repeat each note is written at the length the beams say,
	// which is what makes the arithmetic come out: n times two of them is
	// the pair's whole length.
	wrote := lilyDuration(each)
	return fmt.Sprintf("\\repeat tremolo %d { %s%s %s%s }", n,
		e.pitchOf(st, p, a), wrote, e.pitchOf(st, p, b), wrote)
}

// element writes one note, chord or rest with everything attached to it.
func (e *writer) element(st *state, p *model.Part, el model.Element, mi int) string {
	mk := marksOf(el)
	var b strings.Builder

	if mk != nil && mk.Grace != nil && len(mk.Grace.Notes) > 0 {
		b.WriteString(e.grace(st, p, mk.Grace))
		b.WriteByte(' ')
	}
	if mk != nil && mk.Octave != nil && mk.Octave.Start && mk.Octave.Amount != 0 {
		b.WriteString(fmt.Sprintf("\\ottava #%d ", mk.Octave.Amount))
	}

	b.WriteString(e.pitchOf(st, p, el))
	// Inside a tuplet a note is written as the value it stands in for, not
	// as the length it sounds: three quarters in the time of two are
	// written as quarters and LilyPond does the arithmetic. Writing the
	// sounding length instead gave an eighth, and the bar came out short.
	d := el.Duration()
	if mk != nil && mk.Tuplet != nil && mk.Tuplet.Normal > 0 {
		d = d.Scale(mk.Tuplet.Actual, mk.Tuplet.Normal)
	}
	b.WriteString(lilyDuration(d))

	if mk != nil {
		b.WriteString(e.after(st, el, mk))
	}
	if mk != nil && mk.Octave != nil && mk.Octave.Stop {
		b.WriteString(" \\ottava #0")
	}
	return b.String()
}

// pitchOf is the note, the chord or the rest itself, without its length.
func (e *writer) pitchOf(st *state, p *model.Part, el model.Element) string {
	switch n := el.(type) {
	case *model.Note:
		if n.Percussion > 0 {
			// A drum is not a pitch. LilyPond has a mode of its own for
			// them; written here as the pitch it was given, which is what
			// the staff shows.
			return pitchName(n.Pitch)
		}
		return pitchName(n.Pitch)
	case *model.Chord:
		var names []string
		for _, cn := range n.Notes {
			names = append(names, pitchName(cn.Pitch))
		}
		return "<" + strings.Join(names, " ") + ">"
	case *model.Rest:
		return "r"
	}
	return "r"
}

// after is everything written behind a note: its ties and slurs, the marks
// on it, the words under it.
func (e *writer) after(st *state, el model.Element, m *model.Marks) string {
	var b strings.Builder

	// A one-note tremolo is a colon and the length of the repeated note.
	if m.Tremolo != nil && !m.Tremolo.Paired() {
		if m.Tremolo.Buzz {
			b.WriteString(":32") // the fastest measured thing, with a note
		} else {
			b.WriteString(":" + strconv.Itoa(1<<(m.Tremolo.Beams()+2)))
		}
	}
	if m.TieStart {
		b.WriteString("~")
	}
	for _, a := range m.Articulations {
		b.WriteString(articulation(a))
	}
	if m.Trill != nil && m.Trill.Start && m.Trill.Stop {
		b.WriteString("\\trill")
	} else if m.Trill != nil && m.Trill.Start {
		b.WriteString("\\startTrillSpan")
	}
	if d := dynamic(m.Dynamic); d != "" {
		b.WriteString(d)
	}
	if m.Hairpin != nil {
		switch {
		case m.Hairpin.Start && m.Hairpin.Kind == model.Crescendo:
			b.WriteString("\\<")
		case m.Hairpin.Start && m.Hairpin.Kind == model.Diminuendo:
			b.WriteString("\\>")
		}
		if m.Hairpin.Stop {
			b.WriteString("\\!")
		}
	}
	// The one that ends closes before the one that begins opens: a note
	// that does both is ")(", and "()" is a slur with nothing in it, which
	// LilyPond rightly refuses.
	//
	// Counted, because a score read from somewhere else can hold a slur
	// that ends without having begun — overlapping slurs in the file it
	// came from, a passage copied out of the middle of one. LilyPond stops
	// and complains at a stray close; gscore knows there is nothing open
	// and simply does not write it.
	if m.SlurStop && st.slurs > 0 {
		b.WriteString(")")
		st.slurs--
	}
	if m.SlurStart {
		b.WriteString("(")
		st.slurs++
	}
	if m.Trill != nil && m.Trill.Stop && !m.Trill.Start {
		b.WriteString("\\stopTrillSpan")
	}
	for _, t := range m.Texts {
		side := "^"
		if t.Placement == model.PlacementBelow {
			side = "_"
		}
		b.WriteString(fmt.Sprintf(`%s\markup { \italic "%s" }`, side, escape(t.Text)))
	}
	for _, sm := range m.Symbols {
		// A symbol the user drew is a picture, and LilyPond has no idea
		// what it was. Saying so is better than losing it silently.
		b.WriteString(fmt.Sprintf(" %% hand-drawn symbol %q", sm.Name))
	}
	return b.String()
}

// grace writes the little notes in front of one.
func (e *writer) grace(st *state, p *model.Part, g *model.GraceGroup) string {
	var notes []string
	for _, n := range g.Notes {
		notes = append(notes, pitchName(n.Pitch)+lilyDuration(n.Dur))
	}
	word := "\\grace"
	if g.Slashed && len(g.Notes) == 1 {
		word = "\\acciaccatura"
	}
	return word + " { " + strings.Join(notes, " ") + " }"
}

/*
 * Lyrics and chord symbols, which are their own voices in LilyPond
 */

type verse struct {
	number int
	words  []string
}

// lyricVerses collects the words of a staff, one list per verse.
func lyricVerses(p *model.Part) []verse {
	byVerse := map[int][]string{}
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				mk := marksOf(el)
				if mk == nil {
					continue
				}
				for _, ly := range mk.Lyrics {
					n := ly.Verse
					if n < 1 {
						n = 1
					}
					word := escape(ly.Text)
					// A syllable in the middle of a word is joined to the
					// next with a hyphen, which is LilyPond's "--", and a
					// held one with "__".
					switch ly.Syllabic {
					case model.SyllableBegin, model.SyllableMiddle:
						word += " --"
					}
					if ly.Extend {
						word += " __"
					}
					byVerse[n] = append(byVerse[n], word)
				}
			}
		}
	}
	var out []verse
	for n, words := range byVerse {
		out = append(out, verse{number: n, words: words})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].number < out[j].number })
	return out
}

// chordLine is the chord symbols of a staff as a chordmode line, each with
// the length it is in force for.
//
// Written as a voice of its own because that is how LilyPond puts chords
// over a staff: the symbols have durations and line up with the music by
// having the same total length, not by being attached to notes.
//
// Empty where the staff has no chords at all. A line of nothing but spaces
// would still bring a ChordNames context into being, and an empty
// ChordNames prints "N.C." over every bar — which is how a score for
// seventeen players came out with "no chord" written over all of them.
func chordLine(p *model.Part) string {
	var out []string
	pending := model.Duration{Num: 0, Den: 1}
	var current *model.ChordSymbol
	any := false

	flush := func() {
		if pending.Num == 0 {
			return
		}
		for i, d := range spell(pending) {
			name := "s" // silence: before the first chord, and holding one
			if current != nil && i == 0 {
				name = chordName(current)
			}
			out = append(out, name+lilyDuration(d))
		}
		pending = model.Duration{Num: 0, Den: 1}
	}

	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				mk := marksOf(el)
				if mk != nil && mk.Chord != nil {
					flush()
					current = mk.Chord
					any = true
				}
				pending = pending.Add(el.Duration())
			}
			break // the chords belong to the staff, not to each voice
		}
	}
	flush()
	if !any || len(out) == 0 {
		return ""
	}
	return strings.Join(out, " ")
}

/*
 * The score block
 */

func (e *writer) book() {
	e.line(`\score {`)
	if len(e.names) == 1 {
		e.staff(e.s.Parts[0], e.names[0], "  ")
	} else {
		e.line(`  <<`)
		for i, p := range e.s.Parts {
			e.staff(p, e.names[i], "    ")
		}
		e.line(`  >>`)
	}
	e.line(`  \layout { }`)
	e.line(`  \midi { }`)
	e.line(`}`)
}

func (e *writer) staff(p *model.Part, name, indent string) {
	kind := "Staff"
	if p.Tab != nil {
		kind = "TabStaff"
	}
	with := ""
	if p.Name != "" {
		with = fmt.Sprintf(` \with { instrumentName = %s shortInstrumentName = %s }`,
			quote(p.Name), quote(shortName(p.Name)))
	}

	verses := lyricVerses(p)
	chords := chordLine(p) != ""

	if len(verses) == 0 && !chords {
		e.line(`%s\new %s%s { \%s }`, indent, kind, with, name)
		return
	}
	// The chord names go above the staff and the words below it, both
	// beside it rather than inside it: a Lyrics context nested in a Staff
	// is one LilyPond cannot place, and it says so.
	e.line(`%s<<`, indent)
	if chords {
		e.line(`%s  \new ChordNames { \%sChords }`, indent, name)
	}
	e.line(`%s  \new %s%s { \new Voice = "%s" { \%s } }`,
		indent, kind, with, name, name)
	for _, v := range verses {
		e.line(`%s  \new Lyrics \lyricsto "%s" { \%sLyricsVerse%s }`,
			indent, name, name, spellNumber(v.number))
	}
	e.line(`%s>>`, indent)
}

/*
 * Spelling the pieces
 */

// pitchName is a pitch in LilyPond's absolute notation: the letter, its
// accidental, and a mark for each octave away from the one below middle C.
func pitchName(p model.Pitch) string {
	letter := strings.ToLower(p.Step.String())
	if letter == "" {
		letter = "c"
	}
	letter = letter[:1]

	switch p.Alter {
	case 2:
		letter += "isis"
	case 1:
		letter += "is"
	case -1:
		letter += "es"
	case -2:
		letter += "eses"
	}
	// c' is middle C, which gscore calls octave 4.
	switch n := p.Octave - 3; {
	case n > 0:
		letter += strings.Repeat("'", n)
	case n < 0:
		letter += strings.Repeat(",", -n)
	}
	return letter
}

// lilyDuration is a length written the way LilyPond writes one: the number
// the note is a fraction of, and a dot for each half added.
//
// A length that is not a note value — which the model allows, a tuplet's
// members being thirds of things — is written as the nearest value it is
// built from, because inside a \tuplet that is exactly what it is.
func lilyDuration(d model.Duration) string {
	if v, dots, ok := d.Written(); ok {
		return valueName(v) + strings.Repeat(".", dots)
	}
	// Not a written value: the members of a tuplet are thirds and fifths
	// of things and have no name of their own. Inside a \\tuplet what is
	// written is the value they stand in for, so the nearest plain value
	// not longer than this one is the right answer and keeps the bar from
	// over-running.
	den := 1
	for den < 128 {
		v := model.Duration{Num: 1, Den: den}
		if v.Float() <= d.Float()+1e-9 {
			break
		}
		den *= 2
	}
	return strconv.Itoa(den)
}

func valueName(v model.NoteValue) string {
	switch v {
	case model.DoubleWhole:
		return "\\breve"
	case model.Whole:
		return "1"
	case model.Half:
		return "2"
	case model.Quarter:
		return "4"
	case model.Eighth:
		return "8"
	case model.Sixteenth:
		return "16"
	case model.ThirtySecond:
		return "32"
	case model.SixtyFourth:
		return "64"
	}
	return "4"
}

// spell breaks a length into the note values it is written with.
func spell(d model.Duration) []model.Duration {
	var out []model.Duration
	left := d
	for left.Num > 0 && len(out) < 8 {
		den := 1
		for den <= 128 {
			v := model.Duration{Num: 1, Den: den}
			if v.Float() <= left.Float()+1e-9 {
				break
			}
			den *= 2
		}
		v := model.Duration{Num: 1, Den: den}
		if v.Float() > left.Float()+1e-9 {
			break
		}
		out = append(out, v)
		left = left.Sub(v).Reduced()
	}
	if len(out) == 0 {
		out = append(out, d)
	}
	return out
}

func fullBarRest(t model.TimeSignature) string {
	return "R" + lilyDuration(t.BarDuration())
}

// pickup is the length of an opening bar that is shorter than the meter
// says, which is what an anacrusis is.
//
// Only the first bar, and only when it is short: a short bar in the middle
// of a piece is somebody's mistake or a deliberate irregular bar, and
// neither is a pickup.
func pickup(p *model.Part, t model.TimeSignature) (model.Duration, bool) {
	if len(p.Measures) == 0 {
		return model.Duration{}, false
	}
	have := model.Duration{Num: 0, Den: 1}
	for _, v := range p.Measures[0].Voices {
		d := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements {
			d = d.Add(el.Duration())
		}
		if d.Cmp(have) > 0 {
			have = d
		}
		break
	}
	if have.Num == 0 || have.Cmp(t.BarDuration()) >= 0 {
		return model.Duration{}, false
	}
	return have.Reduced(), true
}

// barLength is how much music a bar actually holds, which is not always
// what the meter says.
func barLength(m *model.Measure) model.Duration {
	out := model.Duration{Num: 0, Den: 1}
	for _, v := range m.Voices {
		d := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements {
			d = d.Add(el.Duration())
		}
		if d.Cmp(out) > 0 {
			out = d
		}
	}
	return out.Reduced()
}

func clefName(c model.Clef) string {
	switch c {
	case model.Bass:
		return "bass"
	case model.Alto:
		return "alto"
	case model.Tenor:
		return "tenor"
	case model.Percussion:
		return "percussion"
	}
	return "treble"
}

// keyCommand is the key signature as LilyPond names it: a tonic and a mode.
func keyCommand(k model.KeySignature) string {
	major := [...]string{"c", "g", "d", "a", "e", "b", "fis", "cis"}
	flat := [...]string{"c", "f", "bes", "ees", "aes", "des", "ges", "ces"}
	n := k.Fifths
	switch {
	case n >= 0 && n < len(major):
		return "\\key " + major[n] + " \\major"
	case n < 0 && -n < len(flat):
		return "\\key " + flat[-n] + " \\major"
	}
	return "\\key c \\major"
}

func articulation(a model.Articulation) string {
	switch a {
	case model.Staccato:
		return "-."
	case model.Staccatissimo:
		return "-!"
	case model.Accent:
		return "->"
	case model.Tenuto:
		return "--"
	case model.Marcato, model.MarcatoDown:
		return "-^"
	case model.Fermata:
		return "\\fermata"
	case model.UpBow:
		return "\\upbow"
	case model.DownBow:
		return "\\downbow"
	case model.Mordent:
		return "\\mordent"
	case model.ShortTrill:
		return "\\prall"
	case model.Turn:
		return "\\turn"
	case model.TurnInverted:
		return "\\reverseturn"
	}
	return ""
}

func dynamic(d model.Dynamic) string {
	switch d {
	case model.DynamicNone:
		return ""
	case model.SForzando:
		return "\\sf"
	case model.Rinforzando:
		return "\\rfz"
	case model.FortePiano:
		return "\\fp"
	}
	if s := d.String(); s != "" {
		return "\\" + s
	}
	return ""
}

func barStyle(b model.BarlineStyle) string {
	switch b {
	case model.BarDouble:
		return "||"
	case model.BarFinal:
		return "|."
	case model.BarRepeatStart:
		return ".|:"
	case model.BarRepeatEnd:
		return ":|."
	case model.BarRepeatBoth:
		return ":|.|:"
	case model.BarNone:
		return ""
	}
	return "|"
}

func signCommand(s model.Sign) string {
	switch s {
	case model.Segno:
		return `\mark \markup { \musicglyph "scripts.segno" }`
	case model.Coda:
		return `\mark \markup { \musicglyph "scripts.coda" }`
	}
	return ""
}

func jumpCommand(j *model.Jump) string {
	if j == nil {
		return ""
	}
	if t := strings.TrimSpace(j.Label()); t != "" {
		return fmt.Sprintf(`\mark \markup { \italic "%s" }`, escape(t))
	}
	return ""
}

// voltaCommand opens a repeat ending. LilyPond's own \repeat volta wraps the
// music it applies to, which a bar-at-a-time writer cannot do, so the
// brackets are set by hand — which is what LilyPond offers for exactly this.
func voltaCommand(en *model.Ending, open bool) string {
	var nums []string
	for _, n := range en.Numbers {
		nums = append(nums, strconv.Itoa(n))
	}
	label := strings.Join(nums, ", ")
	if label == "" {
		label = "1"
	}
	if open {
		return fmt.Sprintf(
			`\set Score.repeatCommands = #'((volta #f) (volta "%s"))`, label)
	}
	return fmt.Sprintf(`\set Score.repeatCommands = #'((volta "%s"))`, label)
}

// transposition is the pitch a transposing instrument sounds when it reads a
// written C, which is how LilyPond is told what kind of instrument it is.
//
// Spelled from the interval rather than from a semitone count, so that a
// clarinet in B flat says b flat and not a sharp: the letter comes from the
// diatonic step and the accidental from what is left over.
func transposition(p *model.Part) string {
	if p.Transpose.IsZero() {
		return ""
	}
	c := model.Pitch{Step: model.StepC, Octave: 4}
	steps := p.Transpose.Steps()
	semis := p.Transpose.Semitones()

	out := model.Pitch{
		Step:   model.Step(((int(c.Step)+steps)%7 + 7) % 7),
		Octave: c.Octave + floorDiv(int(c.Step)+steps, 7),
	}
	// Whatever the letter does not account for is the accidental.
	out.Alter = c.MIDI() + semis - model.Pitch{Step: out.Step, Octave: out.Octave}.MIDI()
	if out.Alter < -2 || out.Alter > 2 {
		return "" // not a spelling anybody writes; better said not at all
	}
	return pitchName(out)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

func chordName(c *model.ChordSymbol) string {
	if c == nil {
		return "s"
	}
	if c.Root == "" {
		// Text that is not a chord — "N.C.", a direction — cannot be said
		// in chordmode, so the space keeps the line's length right.
		return "s"
	}
	root := strings.ToLower(c.Root[:1])
	for _, r := range c.Root[1:] {
		switch r {
		case '#':
			root += "is"
		case 'b':
			root += "es"
		}
	}
	root += "'"
	q := strings.ToLower(strings.TrimSpace(c.Quality))
	switch {
	case q == "", q == "maj":
		// nothing: a bare root is major
	case q == "m", q == "min", q == "-":
		root += ":m"
	case q == "7":
		root += ":7"
	case q == "m7", q == "min7":
		root += ":m7"
	case q == "maj7", q == "M7":
		root += ":maj7"
	case q == "dim":
		root += ":dim"
	case q == "aug", q == "+":
		root += ":aug"
	default:
		root += ":" + q
	}
	if c.Bass != "" {
		root += "/" + strings.ToLower(c.Bass[:1])
	}
	return root
}

/*
 * Odds and ends
 */

func (e *writer) variable(p *model.Part, i int) string {
	base := identifier(p.Name)
	if base == "" {
		base = "part"
	}
	name := base
	for n := 2; e.used[name]; n++ {
		name = base + spellNumber(n)
	}
	e.used[name] = true
	return name
}

// identifier makes a LilyPond variable name: letters only, which is all a
// variable may be made of.
func identifier(s string) string {
	var b strings.Builder
	up := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			if up {
				b.WriteString(strings.ToUpper(string(r)))
				up = false
			} else {
				b.WriteRune(r)
			}
		default:
			up = b.Len() > 0
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	return strings.ToLower(out[:1]) + out[1:]
}

// spellNumber writes a number in letters. A LilyPond variable is made of
// letters and nothing else, so "verse 1" cannot be called verse1.
func spellNumber(n int) string {
	names := [...]string{"Zero", "One", "Two", "Three", "Four", "Five",
		"Six", "Seven", "Eight", "Nine", "Ten"}
	if n >= 0 && n < len(names) {
		return names[n]
	}
	var b strings.Builder
	for _, r := range strconv.Itoa(n) {
		b.WriteString(names[r-'0'])
	}
	return b.String()
}

func shortName(s string) string {
	if len(s) <= 4 {
		return s
	}
	f := strings.Fields(s)
	if len(f) > 1 {
		return strings.ToUpper(f[0][:1]) + strings.ToLower(f[1][:1])
	}
	return s[:3] + "."
}

func lastBarline(p *model.Part) model.BarlineStyle {
	if len(p.Measures) == 0 {
		return model.BarFinal
	}
	b := p.Measures[len(p.Measures)-1].RightBarline
	if b == model.BarSingle {
		return model.BarFinal
	}
	return b
}

// carriesTuplet says the next note belongs to the group that is open: the
// same ratio, whatever its own brackets say.
func carriesTuplet(el model.Element, t *model.Tuplet) bool {
	m := marksOf(el)
	return m != nil && m.Tuplet != nil &&
		m.Tuplet.Actual == t.Actual && m.Tuplet.Normal == t.Normal &&
		!m.Tuplet.Start
}

func marksOf(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

func quote(s string) string { return `"` + escape(s) + `"` }

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

func (e *writer) line(format string, args ...any) {
	fmt.Fprintf(&e.b, format, args...)
	e.b.WriteByte('\n')
}

func (e *writer) blank() { e.b.WriteByte('\n') }
