/*
 * export.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

/*
 * Writing ABC.
 *
 * gscore has read ABC since the beginning and never written it, which left
 * the one format in the program that a person can read, edit and send as a
 * line of text available in one direction only. A tune that came in as ABC
 * could leave as a PDF, a MIDI file or three hundred kilobytes of MusicXML,
 * and not as the four hundred bytes it arrived as.
 *
 * What is written here is meant to be read back by the parser in abc.go and
 * come out the same score. That is the test this is held to — export, import,
 * and compare — because a writer that produces something its own reader
 * cannot understand is a writer that produces nothing.
 */

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// unitLength is the L: every file is written in.
//
// An eighth note. Every duration is then written as a multiple of it — a
// quarter is 2, a sixteenth is /2, a dotted quarter is 3 — and the common
// ones come out as a single character, which is what ABC is for. Anything
// that does not divide evenly is written as the fraction it is, so nothing
// has to be rounded.
var unitLength = model.Duration{Num: 1, Den: 8}

// Export writes a score as ABC.
func Export(w io.Writer, s *model.Score) error {
	b := &strings.Builder{}
	e := &exporter{out: b, score: s}
	e.header()
	e.body()
	_, err := io.WriteString(w, b.String())
	return err
}

type exporter struct {
	out   *strings.Builder
	score *model.Score
}

func (e *exporter) line(format string, args ...any) {
	fmt.Fprintf(e.out, format+"\n", args...)
}

/*
 * The header
 */

func (e *exporter) header() {
	id := e.score.Identity
	e.line("X:1")
	for _, t := range splitLines(id.Title) {
		e.line("T:%s", t)
	}
	for _, t := range splitLines(id.Subtitle) {
		e.line("T:%s", t)
	}
	for _, c := range splitLines(id.Composer) {
		e.line("C:%s", c)
	}
	if id.Copyright != "" {
		e.line("N:%s", oneLine(id.Copyright))
	}

	// How the staves stand together on the page: a piano's two hands are
	// one instrument and are braced, and %%score is how ABC says so.
	if line := e.staffGrouping(); line != "" {
		e.line("%s", line)
	}

	first := e.firstPart()
	if first != nil {
		e.line("M:%s", meterText(first.Time))
	}
	e.line("L:%s", fractionText(unitLength))
	if t := e.score.Tempo; t.BPM > 0 {
		e.line("Q:%s=%d", fractionText(t.Unit), t.BPM)
	}
	if first != nil {
		e.line("K:%s", keyText(first.Key))
	} else {
		e.line("K:C")
	}
}

// staffGrouping writes %%score, which says which staves are braced together.
//
// Without it a piano comes back as two unrelated staves: the music is right
// and the page is not, and on a grand staff the page is half the meaning.
func (e *exporter) staffGrouping() string {
	if len(e.score.Groups) == 0 || len(e.score.Parts) == 0 {
		return ""
	}
	inGroup := map[int]bool{}
	var parts []string
	for _, g := range e.score.Groups {
		if g.First < 0 || g.Last >= len(e.score.Parts) || g.Last <= g.First {
			continue
		}
		var ids []string
		for i := g.First; i <= g.Last; i++ {
			inGroup[i] = true
			ids = append(ids, voiceID(i))
		}
		open, close := "{", "}"
		if g.Symbol == model.GroupBracket {
			open, close = "[", "]"
		}
		parts = append(parts, open+strings.Join(ids, " ")+close)
	}
	if len(parts) == 0 {
		return ""
	}
	// Everything else stands on its own, in the order the score lists it.
	var all []string
	for i := range e.score.Parts {
		if inGroup[i] {
			continue
		}
		all = append(all, voiceID(i))
	}
	all = append(all, parts...)
	sort.SliceStable(all, func(i, j int) bool { return false })
	return "%%score " + strings.Join(all, " ")
}

func (e *exporter) firstPart() *model.Part {
	if len(e.score.Parts) == 0 {
		return nil
	}
	return e.score.Parts[0]
}

func voiceID(i int) string { return strconv.Itoa(i + 1) }

/*
 * The music
 */

func (e *exporter) body() {
	first := e.firstPart()
	for pi, p := range e.score.Parts {
		head := "V:" + voiceID(pi) + " clef=" + clefText(p.Clef)
		if p.Name != "" {
			head += " name=" + quote(p.Name)
		}
		e.line("%s", head)

		// Only where this staff differs from the one the header set, so a
		// score in one key and meter is not restated on every line.
		if first != nil && p.Key != first.Key {
			e.line("K:%s", keyText(p.Key))
		}
		if first != nil && p.Time != first.Time {
			e.line("M:%s", meterText(p.Time))
		}
		if p.Instrument != 0 {
			// The General MIDI number as it stands. abcMIDI is read both ways
			// in the wild — some writers count the programs from one — but
			// gscore's own reader takes the number as written, and a file
			// this program writes has to be one it can read back. Adding one
			// here turned every grand piano into a bright piano and every
			// flute into a recorder, once around the trip.
			e.line("%%%%MIDI program %d", int(p.Instrument))
		}
		e.part(p)
	}
}

// barsPerLine is how many measures go on one line of the file.
//
// ABC is meant to be readable as text, and a file with one enormous line per
// staff is not. Four is what most printed collections use.
const barsPerLine = 4

func (e *exporter) part(p *model.Part) {
	var line strings.Builder
	bars := 0
	flush := func() {
		if line.Len() > 0 {
			e.line("%s", strings.TrimRight(line.String(), " "))
			line.Reset()
		}
		bars = 0
	}

	for mi, m := range p.Measures {
		line.WriteString(e.measure(p, m))
		line.WriteString(barlineText(m.RightBarline))
		line.WriteByte(' ')
		bars++
		if bars >= barsPerLine && mi+1 < len(p.Measures) {
			flush()
		}
	}
	flush()
}

// measure writes one bar: its first voice, then any others overlaid on it.
func (e *exporter) measure(p *model.Part, m *model.Measure) string {
	if len(m.Voices) == 0 {
		return ""
	}
	var b strings.Builder
	for vi, v := range m.Voices {
		if v == nil {
			continue
		}
		if vi > 0 {
			// A second line of music over the same bar.
			b.WriteString("& ")
		}
		b.WriteString(e.voice(p, v))
	}
	return b.String()
}

func (e *exporter) voice(p *model.Part, v *model.Voice) string {
	var b strings.Builder
	for i := 0; i < len(v.Elements); i++ {
		el := v.Elements[i]
		if t := tupletAt(el); t != nil && t.Start {
			b.WriteString(tupletText(t, v.Elements[i:]))
		}
		b.WriteString(e.element(el))
		b.WriteByte(' ')
	}
	return b.String()
}

// tupletText opens a group: three in the time of two, and how many of them.
func tupletText(t *model.Tuplet, rest []model.Element) string {
	count := 0
	for _, el := range rest {
		u := tupletAt(el)
		if u == nil || u.Actual != t.Actual || u.Normal != t.Normal {
			break
		}
		count++
		if u.Stop {
			break
		}
	}
	if count < 2 {
		count = t.Actual
	}
	return fmt.Sprintf("(%d:%d:%d", t.Actual, t.Normal, count)
}

func tupletAt(el model.Element) *model.Tuplet {
	if m := marksOf(el); m != nil {
		return m.Tuplet
	}
	return nil
}

func marksOf(el model.Element) *model.Marks {
	switch v := el.(type) {
	case *model.Note:
		return &v.Marks
	case *model.Chord:
		return &v.Marks
	case *model.Rest:
		return &v.Marks
	}
	return nil
}

// element writes one note, chord or rest with everything attached to it.
func (e *exporter) element(el model.Element) string {
	m := marksOf(el)
	var b strings.Builder

	if m != nil {
		b.WriteString(graceText(m.Grace))
		b.WriteString(decorationsText(m))
		if m.Chord != nil && m.Chord.Root != "" {
			b.WriteString(quote(chordSymbolText(m.Chord)))
		}
		if m.SlurStart {
			b.WriteByte('(')
		}
	}

	switch v := el.(type) {
	case *model.Note:
		b.WriteString(noteText(v, written(el)))
	case *model.Chord:
		b.WriteByte('[')
		for _, n := range v.Notes {
			// Inside the brackets the noteheads carry no length of their
			// own: the chord's length follows the closing bracket.
			b.WriteString(pitchText(n))
		}
		b.WriteByte(']')
		b.WriteString(lengthText(written(el)))
		if v.TieStart {
			b.WriteByte('-')
		}
	case *model.Rest:
		if v.Hidden {
			b.WriteByte('x')
		} else {
			b.WriteByte('z')
		}
		b.WriteString(lengthText(written(el)))
	}

	if m != nil && m.SlurStop {
		b.WriteByte(')')
	}
	return b.String()
}

// written is the length as it is printed, which inside a tuplet is not the
// length it sounds: three in the time of two are written as what two would
// have been.
func written(el model.Element) model.Duration {
	d := el.Duration()
	if t := tupletAt(el); t != nil {
		if num, den, ok := t.Ratio(); ok {
			d = d.Scale(num, den)
		}
	}
	return d
}

func noteText(n *model.Note, d model.Duration) string {
	s := pitchText(n) + lengthText(d)
	if n.TieStart {
		s += "-"
	}
	return s
}

// pitchText writes a notehead: what is printed in front of it, the letter,
// and which octave it is in.
func pitchText(n *model.Note) string {
	var b strings.Builder
	switch n.Accidental {
	case model.DoubleSharp:
		b.WriteString("^^")
	case model.Sharp:
		b.WriteString("^")
	case model.DoubleFlat:
		b.WriteString("__")
	case model.Flat:
		b.WriteString("_")
	case model.Natural:
		b.WriteString("=")
	}
	letter := n.Pitch.Step.String()
	switch {
	case n.Pitch.Octave <= 4:
		b.WriteString(strings.ToUpper(letter))
		b.WriteString(strings.Repeat(",", 4-n.Pitch.Octave))
	default:
		b.WriteString(strings.ToLower(letter))
		b.WriteString(strings.Repeat("'", n.Pitch.Octave-5))
	}
	return b.String()
}

// lengthText writes a duration as a multiple of the unit length.
func lengthText(d model.Duration) string {
	if d.IsZero() {
		return ""
	}
	m := d.Scale(unitLength.Den, unitLength.Num).Reduced()
	switch {
	case m.Num == 1 && m.Den == 1:
		return ""
	case m.Den == 1:
		return strconv.Itoa(m.Num)
	case m.Num == 1:
		return "/" + strconv.Itoa(m.Den)
	}
	return strconv.Itoa(m.Num) + "/" + strconv.Itoa(m.Den)
}

// graceText writes the small notes in front of a note.
func graceText(g *model.GraceGroup) string {
	if g == nil || len(g.Notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	if g.Slashed {
		b.WriteByte('/')
	}
	for _, n := range g.Notes {
		b.WriteString(pitchText(n))
		b.WriteString(lengthText(n.Dur))
	}
	b.WriteByte('}')
	return b.String()
}

func decorationsText(m *model.Marks) string {
	var b strings.Builder
	for _, a := range m.Articulations {
		if name := articulationName(a); name != "" {
			b.WriteString("!" + name + "!")
		}
	}
	if name := dynamicName(m.Dynamic); name != "" {
		b.WriteString("!" + name + "!")
	}
	if m.Trill != nil && m.Trill.Start {
		b.WriteString("!trill!")
	}
	if m.Hairpin != nil {
		if m.Hairpin.Start {
			if m.Hairpin.Kind == model.Diminuendo {
				b.WriteString("!diminuendo(!")
			} else {
				b.WriteString("!crescendo(!")
			}
		}
		if m.Hairpin.Stop {
			if m.Hairpin.Kind == model.Diminuendo {
				b.WriteString("!diminuendo)!")
			} else {
				b.WriteString("!crescendo)!")
			}
		}
	}
	return b.String()
}

func chordSymbolText(c *model.ChordSymbol) string {
	s := c.Root + c.Quality
	if c.Bass != "" {
		s += "/" + c.Bass
	}
	return s
}

/*
 * The small pieces
 */

func barlineText(s model.BarlineStyle) string {
	switch s {
	case model.BarDouble:
		return "||"
	case model.BarFinal:
		return "|]"
	case model.BarRepeatStart:
		return "|:"
	case model.BarRepeatEnd:
		return ":|"
	case model.BarRepeatBoth:
		return "::"
	case model.BarNone:
		return ""
	}
	return "|"
}

func meterText(t model.TimeSignature) string {
	switch t.Symbol {
	case model.TimeCommon:
		return "C"
	case model.TimeCut:
		return "C|"
	}
	if t.Beats <= 0 || t.BeatValue <= 0 {
		return "4/4"
	}
	return fmt.Sprintf("%d/%d", t.Beats, t.BeatValue)
}

func fractionText(d model.Duration) string {
	d = d.Reduced()
	if d.Den == 0 {
		return "1/4"
	}
	return fmt.Sprintf("%d/%d", d.Num, d.Den)
}

// keyText names a key the way K: wants it.
func keyText(k model.KeySignature) string {
	names := map[int]string{
		-7: "Cb", -6: "Gb", -5: "Db", -4: "Ab", -3: "Eb", -2: "Bb", -1: "F",
		0: "C", 1: "G", 2: "D", 3: "A", 4: "E", 5: "B", 6: "F#", 7: "C#",
	}
	minors := map[int]string{
		-7: "Ab", -6: "Eb", -5: "Bb", -4: "F", -3: "C", -2: "G", -1: "D",
		0: "A", 1: "E", 2: "B", 3: "F#", 4: "C#", 5: "G#", 6: "D#", 7: "A#",
	}
	if k.Mode == model.Minor {
		if n, ok := minors[k.Fifths]; ok {
			return n + "m"
		}
		return "Am"
	}
	if n, ok := names[k.Fifths]; ok {
		return n
	}
	return "C"
}

func clefText(c model.Clef) string {
	switch c {
	case model.Bass:
		return "bass"
	case model.Alto:
		return "alto"
	case model.Tenor:
		return "tenor"
	case model.Percussion:
		return "perc"
	}
	return "treble"
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(oneLine(s), `"`, `'`) + `"`
}

func oneLine(s string) string {
	return strings.Join(splitLines(s), " ")
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// The decoration tables, read the other way round.
//
// Built from the very maps the parser uses, so a name this writes is a name
// that reader knows. Two hand-kept lists would agree until somebody added a
// name to one of them.
var (
	articulationNames = invert(decorationArticulations)
	dynamicNames      = invertDynamics(decorationDynamics)
)

func articulationName(a model.Articulation) string { return articulationNames[a] }
func dynamicName(d model.Dynamic) string {
	if d == model.DynamicNone {
		return ""
	}
	return dynamicNames[d]
}

// invert picks one name per articulation: the spelled-out one where there is
// one, because "!staccato!" says what it is and "!.!" has to be looked up.
func invert(m map[string]model.Articulation) map[model.Articulation]string {
	out := map[model.Articulation]string{}
	for name, a := range m {
		if cur, seen := out[a]; !seen || len(name) > len(cur) {
			out[a] = name
		}
	}
	return out
}

func invertDynamics(m map[string]model.Dynamic) map[model.Dynamic]string {
	out := map[model.Dynamic]string{}
	for name, d := range m {
		if cur, seen := out[d]; !seen || len(name) < len(cur) {
			out[d] = name
		}
	}
	return out
}
