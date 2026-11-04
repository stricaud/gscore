/*
 * export.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package guido

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Writing GMN.
 *
 * The easy direction. gscore keeps its music in bars and GUIDO keeps it in
 * a stream, and turning bars into a stream is only a matter of writing a
 * barline between them — it is the other way round, in build.go, that has
 * to work out where the bars were.
 *
 * Two things decide the shape of what comes out.
 *
 * It is written to be read. One voice to a line of bars, the bar number in
 * a comment down the left, the tags in the order a musician reads them.
 * GMN is a text format and the point of a text format is that a person can
 * open it.
 *
 * Spans are written as Begin and End pairs rather than as bracketed ranges
 * wherever they can cross a barline. A slur from the last note of one bar
 * to the first of the next is ordinary music, and written as \slur( … ) it
 * would have to enclose the barline between them — which nests the one
 * thing that cannot nest. \slurBegin and \slurEnd say the same thing with
 * nothing to nest, which is why the language has them.
 */

type exporter struct{}

func (exporter) Name() string         { return "GUIDO" }
func (exporter) Extensions() []string { return []string{".gmn"} }

func (exporter) Export(w io.Writer, s *model.Score) error { return Write(w, s) }

// Write renders a score as GUIDO Music Notation.
func Write(w io.Writer, s *model.Score) error {
	if s == nil {
		return fmt.Errorf("guido: there is no score to write")
	}
	e := &writer{s: s, open: map[string][]openSpan{}}
	e.run()
	_, err := io.WriteString(w, e.b.String())
	return err
}

type writer struct {
	s *model.Score
	b strings.Builder

	// The sticky values of the voice being written. GUIDO repeats the last
	// octave and the last duration, so writing them on every note would be
	// correct and unreadable; they are written when they change.
	oct    int
	dur    model.Duration
	hasOct bool
	hasDur bool

	// The spans left open at the end of a bar, so that a run of notes
	// inside one tuplet bracket is closed before the barline.
	tuplet bool

	// at counts the notes of the voice being written, for pairing the
	// two ends of a span.
	at int

	// meter is the bar length in force, for the voices that have to fill
	// a bar they are silent for.
	meter model.Duration

	// The spans that are open, by what they queue under, oldest first.
	// See openSpan.
	open map[string][]openSpan
	next int
}

// openSpan is a span waiting for its end: the number it was opened with,
// and the tag it was opened by.
//
// The tag is kept because a hairpin's two ends queue together but are
// written with different words. gscore marks a hairpin's end with the kind
// it thinks it is, and in a real score that disagrees with the beginning:
// the 10me Solo has five places where a diminuendo opens and what closes is
// marked a crescendo. Queued apart by kind, those five closed nothing, the
// diminuendos ran to the end of the piece and the crescendos vanished.
// Queued together, each one closes the hairpin that is open — with the word
// that opened it, so the file still says the same thing twice.
type openSpan struct {
	id  int
	tag string
	at  int // which note of the voice opened it
}

// number is why every span carries a :n.
//
// A note can close one slur and open the next — the commonest thing in a
// phrase — and written plainly that is "\slurBegin a \slurEnd", which pairs
// the two tags with each other: the new slur closes immediately and the old
// one is left hanging. Two hairpins can overlap for a note, and then a
// \dimBegin stands between a \crescBegin and its \crescEnd.
//
// GUIDO numbers tags for exactly this, and the number is what says which
// end belongs to which beginning. Found by writing out the 10me Solo and
// reading it back: a hundred and twenty of its slurs, ties and hairpins
// came back attached to the wrong notes or not at all.
//
// The open ones are a queue and not a stack. gscore marks the ends of a
// span on the notes and says nothing about which end belongs to which
// beginning, so two slurs that overlap — one from the first note to the
// fifth, another from the third to the eighth, which is ordinary phrasing
// — are four flags with no pairing in them. Closing the most recent start
// paired the fifth note with the third and left the eighth with nothing
// open, and that slur was dropped. Closing the oldest reads them as what
// they are: the one that began first ends first.
func (e *writer) openSpan(queue, tag string) string {
	e.next++
	e.open[queue] = append(e.open[queue], openSpan{id: e.next, tag: tag, at: e.at})
	return fmt.Sprintf("\\%sBegin:%d ", tag, e.next)
}

// closeSpan writes the end of the span this one closes, and nothing at all
// when none is open: an end on its own says nothing a reader can use, and
// the engine refuses a file for it.
//
// Which one it closes is the whole question, because gscore marks the ends
// of a span on the notes and says nothing about the pairing. Two rules,
// because two kinds of span mean different things by overlapping.
//
// A tie joins one note to the next and nothing else, so a tie that ends
// here began on the note immediately before, and one that began anywhere
// else is not this note's tie at all. The 10me Solo has both loose ends —
// a tie opening into a bar of rests, and one closing six bars later with
// nothing adjacent to it — and pairing them with each other read a tie
// group across the whole passage, which ties every note in it to the next.
// So the tie whose start is the note before is the only one that closes
// here, and the rest are left as what they are.
//
// A slur or a hairpin can cover anything. One that opens and closes on the
// same note is itself; otherwise the one that began first ends first,
// which is how two overlapping phrase marks read.
func (e *writer) closeSpan(queue string) string {
	open := e.open[queue]
	if len(open) == 0 {
		return ""
	}
	pick := -1
	if queue == "tie" {
		for i := len(open) - 1; i >= 0; i-- {
			if open[i].at == e.at-1 {
				pick = i
				break
			}
		}
	} else {
		for i := len(open) - 1; i >= 0; i-- {
			if open[i].at == e.at {
				pick = i
				break
			}
		}
		if pick < 0 {
			pick = 0
		}
	}
	if pick < 0 {
		return ""
	}
	got := open[pick]
	e.open[queue] = append(open[:pick:pick], open[pick+1:]...)
	return fmt.Sprintf(" \\%sEnd:%d", got.tag, got.id)
}

func (e *writer) p(format string, a ...any) { fmt.Fprintf(&e.b, format, a...) }

// closeTuplet shuts the bracket, with no space in front of it: every note
// is written with a space after it and "c d e )" reads badly.
func (e *writer) closeTuplet() {
	s := strings.TrimRight(e.b.String(), " ")
	e.b.Reset()
	e.b.WriteString(s)
	e.p(") ")
	e.tuplet = false
}

func (e *writer) run() {
	e.p("%% GUIDO Music Notation, written by gscore\n")
	e.p("{\n")
	first := true
	for pi, part := range e.s.Parts {
		for vi := 0; vi < voiceCount(part); vi++ {
			if !first {
				e.p(",\n")
			}
			first = false
			e.voice(pi, vi, part)
		}
	}
	e.p("\n}\n")
}

// voiceCount is how many lines of music share this staff.
func voiceCount(p *model.Part) int {
	n := 1
	for _, m := range p.Measures {
		if len(m.Voices) > n {
			n = len(m.Voices)
		}
	}
	return n
}

func (e *writer) voice(pi, vi int, part *model.Part) {
	// Every voice starts afresh: the spec says the octave and the length
	// do not carry from one to the next, and a span cannot either.
	e.oct, e.hasOct, e.hasDur, e.tuplet = 0, false, false, false
	e.open = map[string][]openSpan{}

	e.p("  [\n")
	e.p("    \\staff<%d>", pi+1)
	if part.Name != "" {
		e.p(" \\instr<%s>", quote(part.Name))
	}

	// The header belongs to the piece and is written once, on the first
	// voice of the first staff, which is where the spec says to put it.
	if pi == 0 && vi == 0 {
		if t := e.s.Identity.Title; t != "" {
			e.p(" \\title<%s>", quote(t))
		}
		if c := e.s.Identity.Composer; c != "" {
			e.p(" \\composer<%s>", quote(c))
		}
		if tempo := tempoText(e.s.Tempo); tempo != "" {
			e.p(" %s", tempo)
		}
	}
	e.p("\n")

	// One voice of a staff carries its clef, key and meter; a second voice
	// on the same staff would only set them again.
	if vi == 0 {
		e.p("    \\clef<%s> \\key<%d> \\meter<%s>\n",
			quote(clefText(part.Clef)), part.Key.Fifths, quote(meterText(part.Time)))
	}

	// A piece that opens with a repeat cannot say so through a barline,
	// because the sign belongs to the bar before and there is none. It is
	// written here instead, which is where it is printed.
	if opensWithRepeat(part) {
		e.p("    \\repeatBegin\n")
	}

	for mi, m := range part.Measures {
		v := voiceAt(m, vi)
		if m.Time != nil {
			e.meter = m.Time.BarDuration()
		}
		e.p("    %% bar %d\n    ", mi+1)
		if vi == 0 {
			e.attributes(m)
		}
		if m.Ending != nil && m.Ending.Start {
			e.p("\\voltaBegin<%s> ", quote(m.Ending.Label()))
		}
		if len(v.Elements) == 0 {
			// A bar this voice is silent for still has to take its time.
			//
			// Written as nothing it is not a bar at all: the two barlines
			// fall together, the reader sees one, and every later bar of
			// the voice sits one place to the left of the staff's other
			// voice. In the 10me Solo that put the whole of the second
			// line of the piano part a bar ahead of the first.
			//
			// GUIDO's "empty" is the note for it — it takes its time and
			// is not printed — and that is what a bar with no music in it
			// is.
			e.p("empty%s ", durationText(barLength(e.meter, m), nil))
			e.hasDur = false
		}
		e.elements(v)
		if e.tuplet {
			// A tuplet that runs to the end of a bar: the bracket closes
			// before the barline, never across it.
			e.closeTuplet()
		}
		if m.Ending != nil && m.Ending.Stop {
			e.p("\\voltaEnd ")
		}
		e.barline(m, mi == len(part.Measures)-1)
		if m.Jump != nil {
			e.p(" %s", jumpText(m.Jump))
		}
		e.p("\n")
	}
	e.p("  ]")
}

// voiceAt is the music of one line of a staff in one bar, which may be
// nothing at all where the other line has notes.
func voiceAt(m *model.Measure, vi int) *model.Voice {
	if vi < len(m.Voices) {
		return m.Voices[vi]
	}
	return &model.Voice{}
}

// barLength is how long a bar actually is, which is what a silent voice
// has to fill: the meter where it is kept, and what the other voices of
// the staff hold where the bar is irregular.
func barLength(meter model.Duration, m *model.Measure) model.Duration {
	for _, v := range m.Voices {
		if d := v.Duration(); !d.IsZero() {
			return d.Reduced()
		}
	}
	return meter
}

// attributes writes what changes at the head of a bar.
func (e *writer) attributes(m *model.Measure) {
	if m.Clef != nil {
		e.p("\\clef<%s> ", quote(clefText(*m.Clef)))
	}
	if m.Key != nil {
		e.p("\\key<%d> ", m.Key.Fifths)
	}
	if m.Time != nil {
		e.p("\\meter<%s> ", quote(meterText(*m.Time)))
	}
	if m.Tempo != nil {
		if t := tempoText(*m.Tempo); t != "" {
			e.p("%s ", t)
		}
	}
	if m.Marker != "" {
		e.p("\\mark<%s> ", quote(m.Marker))
	}
	switch m.Sign {
	case model.Segno:
		e.p("\\segno ")
	case model.Coda:
		e.p("\\coda ")
	case model.Fine:
		e.p("\\fine ")
	case model.ToCoda:
		e.p("\\daCoda ")
	}
}

// barline writes what closes the bar.
//
// A repeat that opens is written as the tag rather than as a barline,
// because in GUIDO \repeatBegin is itself the bar: it draws the heavy line
// and the dots where it stands.
func (e *writer) barline(m *model.Measure, last bool) {
	switch m.RightBarline {
	case model.BarDouble:
		e.p("\\doubleBar")
	case model.BarFinal:
		e.p("\\endBar")
	case model.BarRepeatEnd:
		e.p("\\repeatEnd")
	case model.BarRepeatStart:
		e.p("\\repeatBegin")
	case model.BarRepeatBoth:
		e.p("\\repeatEnd \\repeatBegin")
	case model.BarNone:
		// Nothing is drawn and nothing is written.
	default:
		if !last {
			e.p("|")
		}
	}
}

// opensWithRepeat reports a part whose music goes back to its first bar.
//
// gscore keeps a barline on the measure it closes, so the sign that opens a
// repeat lives on the bar before the one it opens — and a repeat at bar one
// has no bar before it to live on. It is still a repeat: playback goes back
// to the beginning. Writing it out here is what makes a file that opened
// with \repeatBegin come back with it.
func opensWithRepeat(p *model.Part) bool {
	for _, m := range p.Measures {
		switch m.RightBarline {
		case model.BarRepeatStart, model.BarRepeatBoth:
			return false
		case model.BarRepeatEnd:
			return true
		}
	}
	return false
}

/*
 * The notes
 */

func (e *writer) elements(v *model.Voice) {
	for i, el := range v.Elements {
		e.element(el, i == 0)
	}
}

func (e *writer) element(el model.Element, firstInBar bool) {
	mk := marksOf(el)
	if mk == nil {
		return
	}
	e.at++

	// A tuplet bracket opens at its first note and closes at its last.
	if t := mk.Tuplet; t != nil && t.Start && !e.tuplet {
		actual, normal, ok := t.Ratio()
		if ok && !t.Hidden {
			e.p("\\tuplet<%s>(", quote(fmt.Sprintf("%d:%d", actual, normal)))
			e.tuplet = true
		}
	}

	e.before(mk)

	// The articulations wrap the note. Nested rather than listed, because
	// that is the only form the language has for them: an accented
	// staccato note is \accent(\stacc(c/4)).
	//
	// Written outermost last, so that reading them back — which applies
	// the innermost first — puts them on the note in the order they were
	// written. Written the other way round, a staccato accent came back
	// as an accented staccato on every note that had both.
	wraps := articulationTags(mk.Articulations)
	for i := len(wraps) - 1; i >= 0; i-- {
		e.p("\\%s(", wraps[i])
	}
	e.note(el)
	for range wraps {
		e.p(")")
	}

	e.after(mk)
	e.p(" ")

	if t := mk.Tuplet; t != nil && t.Stop && e.tuplet {
		e.closeTuplet()
	}
}

// before writes the tags that stand in front of a note.
func (e *writer) before(mk *model.Marks) {
	if mk.Clef != nil {
		e.p("\\clef<%s> ", quote(clefText(*mk.Clef)))
	}
	if mk.Dynamic != model.DynamicNone {
		e.p("\\intens<%s> ", quote(mk.Dynamic.String()))
	}
	if h := mk.Hairpin; h != nil && h.Start {
		tag := "cresc"
		if h.Kind == model.Diminuendo {
			tag = "dim"
		}
		e.p("%s", e.openSpan("hairpin", tag))
	}
	if o := mk.Octave; o != nil && o.Start {
		e.p("\\oct<%d> ", o.Amount)
	}
	if mk.SlurStart {
		e.p("%s", e.openSpan("slur", "slur"))
	}
	if mk.TieStart {
		e.p("%s", e.openSpan("tie", "tie"))
	}
	if t := mk.Trill; t != nil && t.Start {
		e.p("%s", e.openSpan("trill", "trill"))
	}
	if t := mk.Tremolo; t != nil && t.Start {
		e.p("%s", e.openSpan("trem", "trem"))
	}
	if c := mk.Chord; c != nil && c.String() != "" {
		e.p("\\harmony<%s> ", quote(c.String()))
	}
	for _, f := range mk.Fingering {
		e.p("\\fing<%s> ", quote(f))
	}
	for _, l := range mk.Lyrics {
		e.p("\\lyrics<%s> ", quote(l.Text))
	}
	for _, t := range mk.Texts {
		e.p("\\text<%s> ", quote(t.Text))
	}
	if g := mk.Grace; g != nil && len(g.Notes) > 0 {
		e.p("\\grace(")
		for i, n := range g.Notes {
			if i > 0 {
				e.p(" ")
			}
			e.note(n)
		}
		e.p(") ")
	}
}

// after writes the tags that close behind a note.
func (e *writer) after(mk *model.Marks) {
	if t := mk.Tremolo; t != nil && t.Stop {
		e.p("%s", e.closeSpan("trem"))
	}
	if t := mk.Trill; t != nil && t.Stop {
		e.p("%s", e.closeSpan("trill"))
	}
	if mk.TieStop {
		e.p("%s", e.closeSpan("tie"))
	}
	if mk.SlurStop {
		e.p("%s", e.closeSpan("slur"))
	}
	if o := mk.Octave; o != nil && o.Stop {
		e.p(" \\oct<0>")
	}
	if h := mk.Hairpin; h != nil && h.Stop {
		e.p("%s", e.closeSpan("hairpin"))
	}
}

// articulationTags names the ranges that wrap a note.
//
// Only the ones the language has. A mark it cannot say is better written as
// the word it is than dropped without a sign that anything was lost.
func articulationTags(as []model.Articulation) []string {
	var out []string
	for _, a := range as {
		switch a {
		case model.Staccato, model.Staccatissimo:
			out = append(out, "stacc")
		case model.Accent:
			out = append(out, "accent")
		case model.Tenuto:
			out = append(out, "ten")
		case model.Marcato, model.MarcatoDown:
			out = append(out, "marcato")
		case model.Fermata:
			out = append(out, "fermata")
		case model.Mordent, model.ShortTrill:
			out = append(out, "mord")
		case model.Turn, model.TurnInverted:
			out = append(out, "turn")
		}
	}
	return out
}

// note writes one note, rest or chord.
func (e *writer) note(el model.Element) {
	switch n := el.(type) {
	case *model.Note:
		e.p("%s", e.pitch(n))
		e.p("%s", e.length(n.Dur, el))
	case *model.Rest:
		if n.Hidden {
			// A rest that spaces the music without being printed is
			// GUIDO's "empty" note, which is exactly that.
			e.p("empty")
		} else {
			e.p("_")
		}
		e.p("%s", e.length(n.Dur, el))
	case *model.Chord:
		e.p("{")
		for i, m := range n.Notes {
			if i > 0 {
				e.p(", ")
			}
			e.p("%s", e.pitch(m))
			if i == 0 {
				e.p("%s", e.length(n.Dur, el))
			}
		}
		e.p("}")
	}
}

// pitch spells a notehead, with its octave where that has changed.
func (e *writer) pitch(n *model.Note) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(n.Pitch.Step.String()))
	// The accidental is what sounds and not what is printed: GUIDO has no
	// natural sign and no accidental that carries through a bar, so every
	// altered note says so for itself. A note printed without an accidental
	// because the key already sharpened it still has to be written sharp
	// here, or it comes back a semitone low.
	alter := n.Pitch.Alter
	for i := 0; i < alter; i++ {
		b.WriteByte('#')
	}
	for i := 0; i > alter; i-- {
		b.WriteByte('&')
	}
	oct := n.Pitch.Octave - octaveOffset
	if !e.hasOct || oct != e.oct {
		b.WriteString(strconv.Itoa(oct))
		e.oct, e.hasOct = oct, true
	}
	return b.String()
}

// length writes a duration, left out where it has not changed.
func (e *writer) length(d model.Duration, el model.Element) string {
	if e.hasDur && d.Cmp(e.dur) == 0 {
		return ""
	}
	e.dur, e.hasDur = d, true
	return durationText(d, el)
}

// durationText spells a duration the way GUIDO writes one.
//
// A note value with dots keeps them: the spec is explicit that 1/4. and 3/8
// are different, and a dotted quarter written as three eighths is a
// different note on the page even though it lasts as long.
func durationText(d model.Duration, el model.Element) string {
	// A tuplet member's stored length is already what it sounds — a
	// triplet eighth is one twelfth — and that is what GUIDO writes too.
	// Only the bracket says three in the time of two.
	plain := true
	if mk := marksOf(el); mk != nil && mk.Tuplet != nil {
		plain = false
	}
	if plain {
		if value, dots, ok := d.Written(); ok {
			base := value.Duration()
			out := ""
			if base.Num == 1 {
				out = "/" + strconv.Itoa(base.Den)
			} else {
				out = "*" + strconv.Itoa(base.Num) + "/" + strconv.Itoa(base.Den)
			}
			return out + strings.Repeat(".", dots)
		}
	}
	r := d.Reduced()
	if r.Num == 1 {
		return "/" + strconv.Itoa(r.Den)
	}
	return "*" + strconv.Itoa(r.Num) + "/" + strconv.Itoa(r.Den)
}

/*
 * The names GUIDO uses
 */

func clefText(c model.Clef) string {
	switch c {
	case model.Bass:
		return "f4"
	case model.Alto:
		return "c3"
	case model.Tenor:
		return "c4"
	case model.Percussion:
		return "perc"
	}
	return "g2"
}

func meterText(t model.TimeSignature) string {
	switch t.Symbol {
	case model.TimeCommon:
		return "C"
	case model.TimeCut:
		return "C/"
	}
	if t.Beats <= 0 || t.BeatValue <= 0 {
		return "4/4"
	}
	return fmt.Sprintf("%d/%d", t.Beats, t.BeatValue)
}

func tempoText(t model.Tempo) string {
	if t.BPM <= 0 && t.Text == "" {
		return ""
	}
	text := t.Text
	unit := t.Unit
	if unit.Den <= 0 {
		unit = model.Duration{Num: 1, Den: 4}
	}
	if t.BPM <= 0 {
		return fmt.Sprintf("\\tempo<%s>", quote(text))
	}
	beat := fmt.Sprintf("%d/%d=%d", unit.Num, unit.Den, t.BPM)
	// A metronome mark with no word over it is written as the mark alone,
	// which the engine draws as the note and the number. Written twice —
	// once as the text and once as the speed — it came out as the bare
	// fraction printed over the staff.
	if text == "" {
		return fmt.Sprintf("\\tempo<%s>", quote(beat))
	}
	return fmt.Sprintf("\\tempo<%s,%s>", quote(text), quote(beat))
}

func jumpText(j *model.Jump) string {
	if j == nil {
		return ""
	}
	if j.From == model.FromSegno {
		if j.Until == model.UntilFine {
			return "\\dalSegnoAlFine"
		}
		return "\\dalSegno"
	}
	if j.Until == model.UntilFine {
		return "\\daCapoAlFine"
	}
	return "\\daCapo"
}

// quote writes a tag's string argument.
//
// On one line, whatever it was. A composer field carrying a note about
// the transcription came through with a newline in the middle of it, and
// a string broken across two lines is a tag the parser reads to the end
// of the file looking for its closing quote.
func quote(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, `"`, `'`)), " ")
	return `"` + s + `"`
}
