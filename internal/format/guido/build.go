/*
 * build.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package guido

import (
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * From what the file said to a score.
 *
 * Three passes, because each one needs what the one before it worked out.
 *
 * The walker turns a voice's symbols into a flat stream of elements and
 * directions, resolving the sticky octave and duration and hanging the tags
 * onto the notes they cover. It cannot decide what accidental to print,
 * because that depends on the bar, and it does not know where the bars are.
 *
 * The barrer cuts that stream into measures — see the package comment for
 * why this is the hard part — and it is there that the spelling is settled,
 * since by then the key and the bar are both known.
 *
 * The assembler groups the voices onto staves and builds the parts.
 */

// step is one thing in a voice's stream: an element to play, a direction to
// obey, or an element with the way the file spelled it.
type step struct {
	el    model.Element
	dir   *direction
	spell []written
	// grace is how the file spelled the little notes leaning on this one.
	// They are not elements of the voice — they hang off their principal —
	// so without this they never reach the pass that works out what a note
	// sounds, and every flat in a grace group was lost.
	grace []written
	line  int
}

// direction is everything that happens between notes.
type direction struct {
	isBar bool
	bar   model.BarlineStyle

	clef     *model.Clef
	key      *model.KeySignature
	time     *model.TimeSignature
	autoBars *bool
	tempo    *model.Tempo

	repeatStart bool
	repeatEnd   bool
	voltaStart  []int
	voltaStop   bool

	sign   model.Sign
	marker string
	jump   *model.Jump
}

/*
 * The walker
 */

type walker struct {
	rd *reader

	// The sticky values. The spec: an omitted octave or duration repeats
	// the last one given in this voice, and neither carries from one voice
	// to the next — which is why these live here and not on the reader.
	octave int
	dur    model.Duration

	staff string
	name  string // what \instr called the voice
	sound model.Instrument

	steps []step
	elems []int // which steps hold an element

	// pending are the marks waiting for the next element: a dynamic, a
	// fermata between notes, a group of grace notes.
	pending []func(*model.Marks)
	// graceSpell goes with them, for the group of grace notes.
	graceSpell []written

	// spans are the Begin/End ranges still open, by tag name and number.
	// The value is the index into elems of the first element covered.
	spans map[string]int
	// opened is those same keys in the order they were opened, so that
	// the ones a file never closes can be dealt with in order at the end.
	opened []string

	// openEnded is set while a span that was never closed is applied: it
	// marks where the span begins and leaves it without an end, which is
	// what the file said. Closing them at the last note instead would
	// invent an end nobody wrote; dropping them, which is what happened
	// before, lost every hairpin of a piece whose last one runs off the
	// end — and that is most pieces, since a dangling start is what an
	// editor leaves behind when a hairpin is half typed.
	openEnded bool

	// What the running tags have set until something says otherwise.
	stem    model.Stem
	beamOff bool
	octave8 int // \oct<n> in its position form
	small   bool
}

// jump records a D.C. or D.S. where it was written.
func (w *walker) jump(j *model.Jump) { w.direct(&direction{jump: j}) }

func newWalker(rd *reader) *walker {
	return &walker{
		rd:     rd,
		octave: 1, // the spec's default when a voice does not say
		dur:    model.Duration{Num: 1, Den: 4},
		spans:  map[string]int{},
	}
}

// marksOf is the attachment point of an element, whatever kind it is.
func marksOf(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	}
	return nil
}

// emit adds an element to the stream and gives it whatever was waiting.
func (w *walker) emit(el model.Element, spell []written, line int) {
	if mk := marksOf(el); mk != nil {
		for _, fn := range w.pending {
			fn(mk)
		}
		if w.stem != model.StemAuto {
			mk.Stem = w.stem
		}
		if w.beamOff {
			mk.Beam = model.BeamNone
		}
		if w.small {
			mk.Small = true
		}
	}
	w.elems = append(w.elems, len(w.steps))
	w.steps = append(w.steps, step{
		el: el, spell: spell, grace: w.graceSpell, line: line,
	})
	w.pending, w.graceSpell = nil, nil
}

func (w *walker) direct(d *direction) {
	w.steps = append(w.steps, step{dir: d})
}

// attach queues a mark for the next element to arrive.
func (w *walker) attach(fn func(*model.Marks)) {
	w.pending = append(w.pending, fn)
}

// over applies a mark to a stretch of elements, by their place in elems.
func (w *walker) over(first, last int, fn func(mk *model.Marks, first, last bool)) {
	if first < 0 || last >= len(w.elems) || first > last {
		return
	}
	for i := first; i <= last; i++ {
		if mk := marksOf(w.steps[w.elems[i]].el); mk != nil {
			fn(mk, i == first, i == last && !w.openEnded)
		}
	}
}

// walk reads a voice's symbols into the stream.
func (w *walker) walk(syms []symbol) {
	for _, sym := range syms {
		switch s := sym.(type) {
		case *event:
			w.event(s)
		case *tag:
			w.tag(s)
		}
	}
}

// event turns one note, rest or chord into an element.
func (w *walker) event(e *event) {
	dur := w.dur
	if e.hasDur {
		dur = e.dur
		w.dur = dur
	}

	if e.rest {
		w.emit(&model.Rest{Dur: dur}, nil, e.line)
		return
	}

	spell := make([]written, 0, len(e.notes))
	notes := make([]*model.Note, 0, len(e.notes))
	hidden := false
	for _, n := range e.notes {
		if n.hasOct {
			w.octave = n.octave
		} else {
			n.octave = w.octave
		}
		if n.percent {
			hidden = true
			continue
		}
		spell = append(spell, n)
		notes = append(notes, &model.Note{
			// Octave and alteration are settled in the barring pass, where
			// the key signature and the rest of the bar are known.
			Pitch: model.Pitch{Step: n.step, Octave: n.octave + octaveOffset},
			Dur:   dur,
		})
	}
	switch {
	case hidden && len(notes) == 0:
		// "empty" is a note that takes its time and is not printed, which
		// is a hidden rest and nothing else.
		w.emit(&model.Rest{Dur: dur, Hidden: true}, nil, e.line)
	case len(notes) == 1:
		w.emit(notes[0], spell, e.line)
	case len(notes) > 1:
		w.emit(&model.Chord{Notes: notes, Dur: dur}, spell, e.line)
	}
}

// octaveOffset turns a GUIDO octave into a scientific one.
//
// The spec fixes a1 as the 440 Hz A, which everybody else calls A4, and
// every octave starts at C. So the numbers differ by three throughout, and
// getting this wrong puts a whole file three octaves out — which is not
// subtle, but is the kind of thing that is only noticed by playing it.
const octaveOffset = 3

/*
 * Tags
 */

func (w *walker) tag(t *tag) {
	name := strings.ToLower(t.name)

	// A volta is the one range whose ends are not marks on notes but
	// properties of bars, so it is handled before the machinery that puts
	// a mark on the first and last note of a group — which would open the
	// bracket after the music it covers.
	switch name {
	case "volta":
		w.direct(&direction{voltaStart: voltaNumbers(t)})
		w.walk(t.body)
		w.direct(&direction{voltaStop: true})
		return
	case "voltabegin":
		w.direct(&direction{voltaStart: voltaNumbers(t)})
		return
	case "voltaend":
		w.direct(&direction{voltaStop: true})
		return
	}

	// The ranges that open and close with separate tags. gscore marks a
	// span on its first and last element, which is the same shape, so the
	// pair is remembered here and applied when it closes.
	if base, opening, ok := beginEnd(name); ok {
		key := base + ":" + strconv.Itoa(t.num)
		if opening {
			w.spans[key] = len(w.elems)
			w.opened = append(w.opened, key)
			return
		}
		first, open := w.spans[key]
		if !open {
			w.rd.complain(t.line, "\\%s closes a %s that never opened",
				t.name, base)
			return
		}
		w.forget(key)
		w.span(base, t, first, len(w.elems)-1)
		return
	}

	if t.ranged {
		first := len(w.elems)
		// Grace notes and ornament auxiliaries are read out of the body
		// rather than played from it, so they never reach the stream.
		if w.swallows(name, t) {
			return
		}
		w.walk(t.body)
		w.span(name, t, first, len(w.elems)-1)
		return
	}

	w.position(name, t)
}

// forget takes a span off the open list.
func (w *walker) forget(key string) {
	delete(w.spans, key)
	for i, k := range w.opened {
		if k == key {
			w.opened = append(w.opened[:i], w.opened[i+1:]...)
			return
		}
	}
}

// loose applies the spans the file opened and never closed.
//
// They mark the note they begin on and nothing after it. Running them to
// the end of the voice is what the engine would draw, but it is not what
// the file says and it is ruinous for a tie: a tie group ties each of its
// notes to the next, so one unclosed \tieBegin tied every remaining note
// of the piece to the one after it — forty bars of the 10me Solo came back
// as a single held note.
func (w *walker) loose() {
	w.openEnded = true
	defer func() { w.openEnded = false }()
	for _, key := range append([]string(nil), w.opened...) {
		at := w.spans[key]
		base := key[:strings.LastIndex(key, ":")]
		w.forget(key)
		if at >= len(w.elems) {
			continue
		}
		w.rd.complain(w.steps[w.elems[at]].line,
			"a %s opens here and the file never closes it", base)
		w.span(base, &tag{name: base}, at, at)
	}
}

// beginEnd splits \slurBegin and \slurEnd into "slur" and which end it is.
func beginEnd(name string) (base string, opening, ok bool) {
	switch {
	case strings.HasSuffix(name, "begin"):
		base = strings.TrimSuffix(name, "begin")
	case strings.HasSuffix(name, "end"):
		base = strings.TrimSuffix(name, "end")
	default:
		return "", false, false
	}
	// \repeatBegin, \repeatEnd and \endBar are whole words that happen to
	// end this way: they are barlines, not the two ends of a range.
	if base == "" || base == "repeat" {
		return "", false, false
	}
	return base, strings.HasSuffix(name, "begin"), true
}

// swallows handles the ranges whose notes are not notes of the music.
func (w *walker) swallows(name string, t *tag) bool {
	switch name {
	case "grace":
		// The little notes lean on the note that comes after them, and
		// take no time out of the bar — which is exactly how gscore holds
		// them, hanging off their principal.
		sub := newWalker(w.rd)
		sub.octave, sub.dur = w.octave, w.dur
		sub.walk(t.body)
		var notes []*model.Note
		var spell []written
		for _, i := range sub.elems {
			n, ok := sub.steps[i].el.(*model.Note)
			if !ok || len(sub.steps[i].spell) == 0 {
				continue
			}
			notes = append(notes, n)
			spell = append(spell, sub.steps[i].spell[0])
		}
		w.octave, w.dur = sub.octave, sub.dur
		if len(notes) == 0 {
			return true
		}
		w.graceSpell = append(w.graceSpell, spell...)
		w.attach(func(mk *model.Marks) {
			if mk.Grace == nil {
				mk.Grace = &model.GraceGroup{Slashed: true}
			}
			mk.Grace.Notes = append(mk.Grace.Notes, notes...)
		})
		return true

	case "trill", "mord", "mordent", "turn":
		// Written as chords whose first note is the real one and whose
		// others are the auxiliaries. Taken literally that is a chord
		// nobody wrote, so the auxiliaries are dropped and the ornament is
		// put on the note itself.
		sub := newWalker(w.rd)
		sub.octave, sub.dur = w.octave, w.dur
		sub.walk(t.body)
		w.octave, w.dur = sub.octave, sub.dur
		for _, i := range sub.elems {
			el := sub.steps[i].el
			spell := sub.steps[i].spell
			if c, ok := el.(*model.Chord); ok && len(c.Notes) > 0 {
				first := c.Notes[0]
				first.Marks = c.Marks
				el = first
				if len(spell) > 0 {
					spell = spell[:1]
				}
			}
			mk := marksOf(el)
			if mk != nil {
				switch name {
				case "trill":
					mk.Trill = &model.Trill{Start: true, Stop: true}
				case "mord", "mordent":
					mk.Articulations = append(mk.Articulations, model.Mordent)
				case "turn":
					mk.Articulations = append(mk.Articulations, model.Turn)
				}
			}
			w.emit(el, spell, sub.steps[i].line)
		}
		return true
	}
	return false
}

// span applies a range tag to the elements it covers.
func (w *walker) span(name string, t *tag, first, last int) {
	if last < first {
		return
	}
	switch name {
	case "slur", "sl":
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			mk.SlurStart = mk.SlurStart || isFirst
			mk.SlurStop = mk.SlurStop || isLast
		})
	case "tie":
		// A tie joins each note to the next, so every note of the group but
		// the last starts one and every note but the first stops one.
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			mk.TieStart = mk.TieStart || !isLast
			mk.TieStop = mk.TieStop || !isFirst
		})
	case "stacc", "staccato":
		w.article(first, last, model.Staccato)
	case "accent":
		w.article(first, last, model.Accent)
	case "ten", "tenuto":
		w.article(first, last, model.Tenuto)
	case "marcato":
		w.article(first, last, model.Marcato)
	case "fermata":
		w.article(first, last, model.Fermata)
	case "pizz", "pizzicato", "harmonic", "bow", "breathmark":
		// Nothing in the model says these as marks of their own; the ones
		// that matter to a player are kept as words over the note.
		w.text(first, name)
	case "beam", "bm", "b":
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			switch {
			case isFirst && isLast:
				mk.Beam = model.BeamNone
			case isFirst:
				mk.Beam = model.BeamStart
			case isLast:
				mk.Beam = model.BeamStop
			default:
				mk.Beam = model.BeamContinue
			}
		})
	case "cresc", "crescendo", "dim", "diminuendo", "decresc", "decrescendo":
		kind := model.Crescendo
		if name != "cresc" && name != "crescendo" {
			kind = model.Diminuendo
		}
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			if isFirst || isLast {
				mk.Hairpin = &model.Hairpin{Kind: kind, Start: isFirst, Stop: isLast}
			}
		})
		// The dynamic a hairpin arrives at is written on the note after it,
		// which is where a reader looks for it.
		if p, ok := pick(t.params, "", 0); ok && p.text != "" {
			if d, ok := dynamicNamed(p.text); ok {
				w.attach(func(mk *model.Marks) { mk.Dynamic = d })
			}
		}
	case "trem", "tremolo":
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			mk.Tremolo = &model.Tremolo{Strokes: 3, Start: isFirst, Stop: isLast}
		})
	case "trill":
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			mk.Trill = &model.Trill{Start: isFirst, Stop: isLast}
		})
	case "tuplet":
		w.tuplet(t, first, last)
	case "oct", "octava":
		amount := 0
		if p, ok := pick(t.params, "i", 0); ok && p.isNum {
			amount = int(p.num)
		}
		if amount == 0 {
			return
		}
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			if isFirst || isLast {
				mk.Octave = &model.OctaveShift{
					Amount: amount, Start: isFirst, Stop: isLast,
				}
			}
		})
	case "cue":
		w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
			mk.Small = true
		})
	case "text", "t", "label", "mark", "fing", "fingering", "lyrics", "harmony":
		w.words(name, t, first)
	case "intens", "intensity", "i":
		if p, ok := pick(t.params, "s", 0); ok {
			if d, ok := dynamicNamed(p.str()); ok {
				w.over(first, first, func(mk *model.Marks, _, _ bool) {
					mk.Dynamic = d
				})
			}
		}
	case "staff", "accol", "accolade", "noteformat", "restformat", "dispdur",
		"displayduration", "barformat", "stafformat", "space", "set", "auto":
		// Layout, and nothing a score has to carry.
	default:
		w.rd.complain(t.line, "\\%s is not understood; its notes are kept",
			t.name)
	}
}

func (w *walker) article(first, last int, a model.Articulation) {
	w.over(first, last, func(mk *model.Marks, _, _ bool) {
		mk.Articulations = append(mk.Articulations, a)
	})
}

// text writes a word over one element, for the marks that have no model of
// their own. Losing them silently would be worse: they are instructions.
func (w *walker) text(first int, word string) {
	w.over(first, first, func(mk *model.Marks, _, _ bool) {
		mk.Texts = append(mk.Texts, model.TextMark{Text: word})
	})
}

// words handles the tags whose point is their string.
func (w *walker) words(name string, t *tag, first int) {
	p, ok := pick(t.params, "s", 0)
	if !ok || p.str() == "" {
		return
	}
	text := p.str()
	apply := func(fn func(mk *model.Marks)) {
		if t.ranged && first < len(w.elems) {
			w.over(first, first, func(mk *model.Marks, _, _ bool) { fn(mk) })
			return
		}
		w.attach(fn)
	}
	switch name {
	case "lyrics":
		apply(func(mk *model.Marks) {
			mk.Lyrics = append(mk.Lyrics, model.Lyric{Text: text, Verse: 1})
		})
	case "harmony":
		apply(func(mk *model.Marks) {
			mk.Chord = &model.ChordSymbol{Root: text}
		})
	case "fing", "fingering":
		apply(func(mk *model.Marks) {
			mk.Fingering = append(mk.Fingering, text)
		})
	case "mark":
		w.direct(&direction{marker: text})
	default:
		apply(func(mk *model.Marks) {
			mk.Texts = append(mk.Texts, model.TextMark{Text: text})
		})
	}
}

// tuplet reads the bracket's format string: "3", "-3-", "3:2", "-3:2-".
func (w *walker) tuplet(t *tag, first, last int) {
	spec := ""
	if p, ok := pick(t.params, "format", 0); ok {
		spec = p.str()
	}
	actual, normal := tupletRatio(spec)
	if actual <= 0 {
		// The format says only how to draw it. Work the ratio out from the
		// notes instead, which is what a reader has to do anyway for the
		// older files that have no \tuplet tag at all.
		return
	}
	w.over(first, last, func(mk *model.Marks, isFirst, isLast bool) {
		mk.Tuplet = &model.Tuplet{
			Actual: actual, Normal: normal, Start: isFirst, Stop: isLast,
		}
	})
}

// tupletRatio reads "x" or "x:y" out of a format string, ignoring the
// dashes that say which brackets to draw.
func tupletRatio(spec string) (actual, normal int) {
	spec = strings.Trim(spec, "-")
	if spec == "" {
		return 0, 0
	}
	parts := strings.SplitN(spec, ":", 2)
	a, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || a <= 1 {
		return 0, 0
	}
	if len(parts) == 2 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && n > 0 {
			return a, n
		}
	}
	return a, inTheTimeOf(a)
}

// inTheTimeOf is how many ordinary notes a tuplet of n stands in for, when
// the file does not say: the largest power of two below it. Three in the
// time of two, five in the time of four, seven in the time of four.
func inTheTimeOf(n int) int {
	p := 1
	for p*2 < n {
		p *= 2
	}
	return p
}

// voltaNumbers reads which times through a bracket is played out of its
// mark: "1." and "1,2" and "1-3" are all written in real files.
func voltaNumbers(t *tag) []int {
	p, ok := pick(t.params, "mark", 0)
	if !ok {
		return []int{1}
	}
	var out []int
	for _, field := range strings.FieldsFunc(p.str(), func(r rune) bool {
		return r == ',' || r == ' ' || r == '.'
	}) {
		if n, err := strconv.Atoi(strings.TrimSpace(field)); err == nil {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return []int{1}
	}
	return out
}

// position handles the tags that mark a point rather than a stretch.
func (w *walker) position(name string, t *tag) {
	switch name {
	case "bar", "doublebar", "endbar":
		style := model.BarSingle
		switch name {
		case "doublebar":
			style = model.BarDouble
		case "endbar":
			style = model.BarFinal
		}
		w.direct(&direction{isBar: true, bar: style})

	case "repeatbegin":
		w.direct(&direction{repeatStart: true})
	case "repeatend":
		w.direct(&direction{repeatEnd: true})

	case "clef":
		if p, ok := pick(t.params, "type", 0); ok {
			if c, ok := clefNamed(p.str()); ok {
				w.direct(&direction{clef: &c})
			} else {
				w.rd.complain(t.line, "there is no clef called %q", p.str())
			}
		}
	case "key":
		if p, ok := pick(t.params, "key", 0); ok {
			if k, ok := keyNamed(p); ok {
				w.direct(&direction{key: &k})
			} else {
				w.rd.complain(t.line, "there is no key called %q", p.str())
			}
		}
	case "meter":
		p, ok := pick(t.params, "type", 0)
		if !ok {
			return
		}
		ts, good := meterNamed(p.str())
		if !good {
			w.rd.complain(t.line, "there is no meter called %q", p.str())
			return
		}
		d := &direction{time: &ts}
		// autoBarlines is on unless the tag says otherwise, and turning it
		// off is a file saying "the barlines are where I put them".
		if a, ok := pick(t.params, "autoBarlines", -1); ok {
			off := isOff(a.str())
			on := !off
			d.autoBars = &on
		}
		w.direct(d)

	case "tempo":
		w.direct(&direction{tempo: tempoFrom(t)})

	case "staff":
		if p, ok := pick(t.params, "id", 0); ok {
			w.staff = p.str()
		}
	case "instr", "instrument":
		if p, ok := pick(t.params, "s", 0); ok {
			w.name = p.str()
		}
		if p, ok := pick(t.params, "s2", 1); ok {
			if n, ok := midiProgram(p.str()); ok {
				w.sound = model.Instrument(n)
			}
		}

	case "title":
		if p, ok := pick(t.params, "s", 0); ok {
			w.rd.title = p.str()
		}
	case "composer":
		if p, ok := pick(t.params, "s", 0); ok {
			w.rd.composer = p.str()
		}

	case "intens", "intensity", "i":
		if p, ok := pick(t.params, "s", 0); ok {
			if d, ok := dynamicNamed(p.str()); ok {
				w.attach(func(mk *model.Marks) { mk.Dynamic = d })
			}
		}
	case "fermata":
		w.attach(func(mk *model.Marks) {
			mk.Articulations = append(mk.Articulations, model.Fermata)
		})
	case "text", "t", "mark", "label", "lyrics", "harmony", "fing", "fingering":
		w.words(name, t, len(w.elems))

	case "segno":
		w.direct(&direction{sign: model.Segno})
	case "coda":
		w.direct(&direction{sign: model.Coda})
	case "fine":
		w.direct(&direction{sign: model.Fine})

	case "stemsup":
		w.stem = model.StemUp
	case "stemsdown":
		w.stem = model.StemDown
	case "stemsauto":
		w.stem = model.StemAuto
	case "beamsoff":
		w.beamOff = true
	case "beamsauto", "beamsfull":
		w.beamOff = false

	case "oct", "octava":
		// The position form opens a transposed section that runs to the
		// next \oct. Closing it is \oct<0>.
		amount := 0
		if p, ok := pick(t.params, "i", 0); ok && p.isNum {
			amount = int(p.num)
		}
		if amount == w.octave8 {
			return
		}
		was := w.octave8
		w.octave8 = amount
		w.attach(func(mk *model.Marks) {
			if was != 0 {
				mk.Octave = &model.OctaveShift{Amount: was, Stop: true}
			}
			if amount != 0 {
				mk.Octave = &model.OctaveShift{Amount: amount, Start: true}
			}
		})

	case "dacapo", "dacapoalfine":
		until := model.UntilEnd
		if name == "dacapoalfine" {
			until = model.UntilFine
		}
		w.jump(&model.Jump{From: model.FromStart, Until: until})
	case "dalsegno", "dalsegnoalfine":
		until := model.UntilEnd
		if name == "dalsegnoalfine" {
			until = model.UntilFine
		}
		w.jump(&model.Jump{From: model.FromSegno, Until: until})
	case "dacoda":
		w.direct(&direction{sign: model.ToCoda})

	case "newline", "newsystem", "newpage", "pageformat", "systemformat",
		"staffformat", "staffon", "staffoff", "barformat", "space", "set",
		"auto", "accol", "accolade", "dotformat", "noteformat", "restformat",
		"headsnormal", "headscenter", "headsleft", "headsright", "headsreverse",
		"tactus":
		// Layout and engine settings: nothing the music carries.

	default:
		w.rd.complain(t.line, "\\%s is not understood", t.name)
	}
}

func isOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "false", "0", "no":
		return true
	}
	return false
}

/*
 * The names a file uses
 */

func clefNamed(s string) (model.Clef, bool) {
	t := strings.ToLower(strings.TrimSpace(s))
	// A transposition suffix changes the sounding octave and not the sign.
	for _, suffix := range []string{"+8", "-8", "+15", "-15"} {
		t = strings.TrimSuffix(t, suffix)
	}
	switch t {
	case "g", "g2", "treble", "violin", "violino", "gg", "gg2", "g1", "g3",
		"g4", "g5":
		return model.Treble, true
	case "f", "f4", "bass", "basso", "f3", "f5", "f1", "f2":
		return model.Bass, true
	case "c", "c3", "alto":
		return model.Alto, true
	case "c4", "tenor":
		return model.Tenor, true
	case "c1", "c2", "c5", "c0":
		return model.Alto, true
	case "perc", "perc1", "perc2", "perc3", "perc4", "perc5":
		return model.Percussion, true
	case "none", "off", "tab":
		// Nothing to draw, and nothing that changes where a note sits.
		return model.Treble, true
	}
	return model.Treble, false
}

// keyNamed reads \key<-3> and \key<"e&">.
//
// The signed form is the number of sharps or flats and says nothing about
// the mode. The spelled form does: the spec gives an uppercase letter for a
// major key and a lowercase one for a minor, so "C" and "c" are different
// keys with different signatures.
func keyNamed(p param) (model.KeySignature, bool) {
	if p.isNum {
		n := int(p.num)
		if n < -7 || n > 7 {
			return model.KeySignature{}, false
		}
		return model.KeySignature{Fifths: n, Mode: model.Major}, true
	}
	s := strings.TrimSpace(p.text)
	if s == "" {
		return model.KeySignature{}, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n < -7 || n > 7 {
			return model.KeySignature{}, false
		}
		return model.KeySignature{Fifths: n, Mode: model.Major}, true
	}
	mode := model.Major
	if s[0] >= 'a' && s[0] <= 'z' {
		mode = model.Minor
	}
	step, ok := model.StepFromLetter(s[0])
	if !ok {
		return model.KeySignature{}, false
	}
	// h is the German B, as in a note name.
	if strings.EqualFold(string(s[0]), "h") {
		step = model.StepB
	}
	alter := 0
	for _, c := range s[1:] {
		switch c {
		case '#':
			alter++
		case '&', 'b':
			alter--
		}
	}
	fifths, ok := fifthsFor(step, alter, mode)
	if !ok {
		return model.KeySignature{}, false
	}
	return model.KeySignature{Fifths: fifths, Mode: mode}, true
}

// fifthsFor turns a key's tonic into its place on the circle of fifths.
func fifthsFor(step model.Step, alter int, mode model.Mode) (int, bool) {
	// The naturals, counted in fifths from C.
	base := map[model.Step]int{
		model.StepC: 0, model.StepG: 1, model.StepD: 2, model.StepA: 3,
		model.StepE: 4, model.StepB: 5, model.StepF: -1,
	}
	n, ok := base[step]
	if !ok {
		return 0, false
	}
	n += alter * 7
	if mode == model.Minor {
		// A minor key is written with its relative major's signature,
		// three places back round the circle: A minor has none, and E
		// flat minor has the six flats of G flat major.
		n -= 3
	}
	if n < -7 || n > 7 {
		return 0, false
	}
	return n, true
}

func meterNamed(s string) (model.TimeSignature, bool) {
	t := strings.TrimSpace(s)
	switch strings.ToUpper(t) {
	case "C":
		return model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeCommon}, true
	case "C/":
		return model.TimeSignature{Beats: 2, BeatValue: 2, Symbol: model.TimeCut}, true
	}
	// A complex meter — "3+2/8" — adds up to one bar, which is what gscore
	// needs to know; how it is printed is beyond the model.
	num, den, ok := 0, 0, false
	if i := strings.LastIndex(t, "/"); i > 0 {
		for _, piece := range strings.Split(t[:i], "+") {
			n, err := strconv.Atoi(strings.TrimSpace(piece))
			if err != nil {
				return model.TimeSignature{}, false
			}
			num += n
		}
		d, err := strconv.Atoi(strings.TrimSpace(t[i+1:]))
		if err != nil || d <= 0 {
			return model.TimeSignature{}, false
		}
		den, ok = d, true
	}
	if !ok || num <= 0 {
		return model.TimeSignature{}, false
	}
	return model.TimeSignature{Beats: num, BeatValue: den}, true
}

// tempoFrom reads \tempo<"Allegro","1/4=120">.
//
// The speed is usually the second argument, with a word over it in the
// first. It need not be: a metronome mark with no word is written as the
// mark alone, and the engine draws the note and the number from it. Read
// as a word that would come out as the characters "1/4=120" printed over
// the staff, which is what it did.
func tempoFrom(t *tag) *model.Tempo {
	out := &model.Tempo{Unit: model.Duration{Num: 1, Den: 4}}
	if p, ok := pick(t.params, "s", 0); ok {
		out.Text = p.str()
	}
	spec := ""
	if p, ok := pick(t.params, "s2", 1); ok {
		spec = p.str()
	} else if beatSpec(out.Text) {
		spec, out.Text = out.Text, ""
	}
	if i := strings.Index(spec, "="); i > 0 {
		unit, left := spec[:i], spec[i+1:]
		if j := strings.Index(unit, "/"); j > 0 {
			n, _ := strconv.Atoi(strings.TrimSpace(unit[:j]))
			d, _ := strconv.Atoi(strings.TrimSpace(unit[j+1:]))
			if n > 0 && d > 0 {
				out.Unit = model.Duration{Num: n, Den: d}
			}
		}
		if bpm, err := strconv.Atoi(strings.TrimSpace(left)); err == nil {
			out.BPM = bpm
		}
	}
	if out.BPM == 0 && out.Text == "" {
		return nil
	}
	return out
}

// beatSpec reports a string that is a metronome mark and nothing else:
// "1/4=120", the form the spec gives for the speed argument.
func beatSpec(s string) bool {
	i := strings.Index(s, "=")
	j := strings.Index(s, "/")
	if i <= 0 || j <= 0 || j > i {
		return false
	}
	for _, piece := range []string{s[:j], s[j+1 : i], s[i+1:]} {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			return false
		}
		for k := 0; k < len(piece); k++ {
			if !isDigit(piece[k]) {
				return false
			}
		}
	}
	return true
}

func midiProgram(s string) (int, bool) {
	t := strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(t, "midi") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(t[4:]))
	if err != nil || n < 0 || n > 127 {
		return 0, false
	}
	return n, true
}

var dynamicNames = map[string]model.Dynamic{
	"pppp": model.PPPP, "ppp": model.PPP, "pp": model.PP, "p": model.Piano,
	"mp": model.MP, "mf": model.MF, "f": model.Forte, "ff": model.FF,
	"fff": model.FFF, "ffff": model.FFFF, "sfz": model.SForzando,
	"sf": model.SForzato, "fp": model.FortePiano, "rfz": model.Rinforzando,
	"niente": model.Niente,
}

func dynamicNamed(s string) (model.Dynamic, bool) {
	d, ok := dynamicNames[strings.ToLower(strings.TrimSpace(s))]
	return d, ok
}

/*
 * The barrer
 */

// barrer cuts one voice's stream into measures.
type barrer struct {
	rd *reader

	key      model.KeySignature
	time     model.TimeSignature
	hasTime  bool
	autoBars bool
	clef     model.Clef

	// firstKey and firstTime are what the voice opened with, which is what
	// the part is written in.
	firstKey  *model.KeySignature
	firstTime *model.TimeSignature
	firstClef *model.Clef

	measures []*model.Measure
	cur      *model.Measure
	voice    *model.Voice
	filled   model.Duration

	// accs is what the bar has already said about a note: a sharp written
	// on one C makes the next C in that bar sharp too, as a player reads
	// it, and what is printed follows from the difference.
	accs map[accKey]int

	// waiting holds the signature changes that arrived mid-bar and belong
	// to the next one.
	waitKey  *model.KeySignature
	waitTime *model.TimeSignature

	openVolta []int
	openedAt  int

	// pending is a clef change read mid-bar, waiting for the note it is
	// drawn in front of.
	pending []*model.Clef
}

type accKey struct {
	step   model.Step
	octave int
}

func newBarrer(rd *reader) *barrer {
	return &barrer{
		rd:       rd,
		autoBars: true,
		accs:     map[accKey]int{},
		clef:     model.Treble,
		time:     model.TimeSignature{Beats: 4, BeatValue: 4},
	}
}

func (b *barrer) open() {
	if b.cur != nil {
		return
	}
	b.voice = &model.Voice{}
	b.cur = &model.Measure{Voices: []*model.Voice{b.voice}, RightBarline: model.BarSingle}
	b.filled = model.Duration{Num: 0, Den: 1}
	b.accs = map[accKey]int{}
	if b.waitKey != nil {
		b.cur.Key, b.waitKey = b.waitKey, nil
	}
	if b.waitTime != nil {
		b.cur.Time, b.waitTime = b.waitTime, nil
	}
}

func (b *barrer) close(style model.BarlineStyle) {
	if b.cur == nil {
		// A barline with nothing before it: the opening of the piece
		// written out, which is not a bar.
		return
	}
	b.cur.RightBarline = style
	b.measures = append(b.measures, b.cur)
	b.cur, b.voice = nil, nil
}

// closeWith is a barline the file asked for.
//
// It is not the same as close, because the meter may have cut the bar
// already: a file that writes \meter<"4/4"> and then four quarters and then
// \doubleBar has said one thing twice, and the bar was closed by the fourth
// quarter before the tag was read. Closing again there would append an
// empty measure and lose the double bar with it, which is what it did —
// every explicit barline that agreed with the meter went missing, repeats
// included.
//
// So a barline arriving at a bar that holds nothing yet restyles the one
// just closed, and leaves the new bar open with whatever has been set on
// it: a key change read between the two is still a key change.
func (b *barrer) closeWith(style model.BarlineStyle) {
	if b.cur != nil && len(b.voice.Elements) > 0 {
		b.close(style)
		return
	}
	if n := len(b.measures); n > 0 {
		b.measures[n-1].RightBarline = mergeBar(b.measures[n-1].RightBarline, style)
	}
}

// mergeBar is one barline position asked for twice.
func mergeBar(was, now model.BarlineStyle) model.BarlineStyle {
	switch {
	case was == now:
		return was
	case was == model.BarSingle:
		return now
	case now == model.BarSingle:
		return was
	case was == model.BarRepeatEnd && now == model.BarRepeatStart,
		was == model.BarRepeatStart && now == model.BarRepeatEnd:
		// One section closing where the next opens.
		return model.BarRepeatBoth
	}
	return now
}

// finish closes the last bar of a voice.
//
// A direction written after the last note — a Fine, a D.C., the close of a
// volta — opened a bar of its own that has nothing in it. An empty bar at
// the end of a piece is a bar of ruled lines after the final barline, so
// what it carries is moved back onto the bar that does have music.
func (b *barrer) finish() {
	if b.cur == nil {
		return
	}
	if len(b.voice.Elements) > 0 || len(b.measures) == 0 {
		b.close(model.BarSingle)
		return
	}
	last := b.measures[len(b.measures)-1]
	if b.cur.Sign != model.NoSign {
		last.Sign = b.cur.Sign
	}
	if b.cur.Marker != "" {
		last.Marker = b.cur.Marker
	}
	if b.cur.Jump != nil {
		last.Jump = b.cur.Jump
	}
	b.cur, b.voice = nil, nil
}

// run barres one voice.
func (b *barrer) run(steps []step) []*model.Measure {
	for _, s := range steps {
		if s.dir != nil {
			b.direction(s.dir)
			continue
		}
		if s.el == nil {
			continue
		}
		b.open()
		b.spell(s)
		b.voice.Elements = append(b.voice.Elements, s.el)
		b.filled = b.filled.Add(s.el.Duration())

		if !b.hasTime || !b.autoBars {
			continue
		}
		full := b.time.BarDuration()
		switch b.filled.Cmp(full) {
		case 0:
			b.close(model.BarSingle)
		case 1:
			// Over-full. The music is kept whole — splitting a note across
			// a barline it was not written across would be inventing a tie
			// nobody wrote — and the bar is closed where it stands.
			b.rd.complain(s.line,
				"this bar holds %s of music where %s says %s",
				b.filled, b.time, full)
			b.close(model.BarSingle)
		}
	}
	b.finish()
	return b.measures
}

func (b *barrer) direction(d *direction) {
	switch {
	case d.isBar:
		b.closeWith(d.bar)
	case d.repeatEnd:
		b.closeWith(model.BarRepeatEnd)
	case d.repeatStart:
		// The sign opens the bar that follows, and gscore keeps a barline
		// on the measure it closes — so it goes on the one before. A piece
		// that opens with a repeat has no bar before it to carry the sign;
		// playback still goes back to the beginning, which is what the
		// repeat means, and the writer puts the sign back.
		b.closeWith(model.BarRepeatStart)
	case d.clef != nil:
		if b.firstClef == nil {
			b.firstClef = d.clef
		}
		b.clef = *d.clef
		if b.cur == nil || len(b.voice.Elements) == 0 {
			b.open()
			b.cur.Clef = d.clef
		} else {
			// Mid-bar, which is where a clef change actually happens: the
			// sign is drawn in front of the note it applies to.
			b.pendClef(d.clef)
		}
	case d.key != nil:
		if b.firstKey == nil {
			b.firstKey = d.key
		}
		b.key = *d.key
		b.accs = map[accKey]int{}
		if b.cur == nil || len(b.voice.Elements) == 0 {
			b.open()
			b.cur.Key = d.key
		} else {
			b.waitKey = d.key
		}
	case d.time != nil:
		if b.firstTime == nil {
			b.firstTime = d.time
		}
		b.time, b.hasTime = *d.time, true
		if d.autoBars != nil {
			b.autoBars = *d.autoBars
		}
		if b.cur == nil || len(b.voice.Elements) == 0 {
			b.open()
			b.cur.Time = d.time
		} else {
			b.waitTime = d.time
		}
	case d.autoBars != nil:
		b.autoBars = *d.autoBars
	case d.tempo != nil:
		b.open()
		b.cur.Tempo = d.tempo
	case d.marker != "":
		b.open()
		b.cur.Marker = d.marker
	case d.sign != model.NoSign:
		b.open()
		b.cur.Sign = d.sign
	case d.jump != nil:
		// A direction written at the end of a bar sends the player back
		// from there, so it belongs to the bar it was read in — or to the
		// one just closed, where it came after the barline.
		if b.cur != nil {
			b.cur.Jump = d.jump
		} else if n := len(b.measures); n > 0 {
			b.measures[n-1].Jump = d.jump
		}
	case d.voltaStart != nil:
		b.open()
		b.cur.Ending = &model.Ending{Numbers: d.voltaStart, Start: true}
		b.openVolta = d.voltaStart
		b.openedAt = len(b.measures)
	case d.voltaStop:
		// The bracket closes at the end of the bar the music had reached,
		// which is the open one where there is music in it and the one
		// just closed where the meter has already cut it.
		if b.cur != nil && len(b.voice.Elements) > 0 {
			b.cur.Ending = endingOn(b.cur.Ending, b.openVolta)
		} else if n := len(b.measures); n > 0 {
			b.measures[n-1].Ending = endingOn(b.measures[n-1].Ending, b.openVolta)
		}
		b.openVolta = nil
	}
}

func endingOn(cur *model.Ending, nums []int) *model.Ending {
	if cur == nil {
		cur = &model.Ending{Numbers: nums}
	}
	cur.Stop = true
	return cur
}

// pendClef puts a clef change on the next note, which is where it is drawn.
func (b *barrer) pendClef(c *model.Clef) {
	b.pending = append(b.pending, c)
}

// spell settles what a note sounds and what is printed on it.
//
// GUIDO repeats an accidental on every altered note, because the spec says
// one is valid only where it is written. Conventional notation prints it
// once a bar. So the written alteration decides the sound, and what is
// printed is the difference between that and what the bar has already said.
func (b *barrer) spell(s step) {
	if len(b.pending) > 0 {
		if mk := marksOf(s.el); mk != nil {
			mk.Clef = b.pending[0]
		}
		b.pending = nil
	}
	// The grace notes first: they are played before the note they lean on,
	// so an accidental in them is read before one on the principal.
	if mk := marksOf(s.el); mk != nil && mk.Grace != nil {
		b.resolve(mk.Grace.Notes, s.grace)
	}
	b.resolve(notesOf(s.el), s.spell)
}

// resolve settles each notehead against the key and the bar.
func (b *barrer) resolve(notes []*model.Note, spell []written) {
	for i, w := range spell {
		if i >= len(notes) {
			break
		}
		n := notes[i]
		k := accKey{step: w.step, octave: n.Pitch.Octave}

		// What the bar has said about this note so far, or what the key
		// says where it has said nothing.
		was, seen := b.accs[k]
		if !seen {
			was = b.key.AlterFor(w.step)
		}

		// What the note sounds is what it spells, and nothing else.
		//
		// A GUIDO pitch is absolute: the key signature is drawn but does
		// not alter the notes, which is why the specification's own
		// examples write h& on every B flat of a piece in F major and f#
		// on every F sharp of one in G. Taking the signature as an
		// alteration read those as B double flat and F double sharp, and
		// left a bare h in F major sounding a semitone low.
		sounds := w.alter
		n.Pitch.Alter = sounds
		if sounds != was {
			n.Accidental = accidentalFor(sounds)
		}
		b.accs[k] = sounds
	}
}

func notesOf(el model.Element) []*model.Note {
	switch e := el.(type) {
	case *model.Note:
		return []*model.Note{e}
	case *model.Chord:
		return e.Notes
	}
	return nil
}

func accidentalFor(alter int) model.Accidental {
	switch alter {
	case -2:
		return model.DoubleFlat
	case -1:
		return model.Flat
	case 0:
		return model.Natural
	case 1:
		return model.Sharp
	case 2:
		return model.DoubleSharp
	}
	return model.NoAccidental
}

/*
 * Putting the score together
 */

// build turns the parsed voices into a score.
func (rd *reader) build(voices [][]symbol) *model.Score {
	type line struct {
		staff    string
		name     string
		sound    model.Instrument
		measures []*model.Measure
		key      *model.KeySignature
		time     *model.TimeSignature
		clef     *model.Clef
	}

	lines := make([]line, 0, len(voices))
	for i, syms := range voices {
		w := newWalker(rd)
		w.walk(syms)
		w.loose()
		b := newBarrer(rd)
		ms := b.run(w.steps)

		staff := w.staff
		if staff == "" {
			// No \staff: the spec's own examples give one per voice, and a
			// file that says nothing means one staff each.
			staff = "voice " + strconv.Itoa(i+1)
		}
		lines = append(lines, line{
			staff: staff, name: w.name, sound: w.sound, measures: ms,
			key: b.firstKey, time: b.firstTime, clef: b.firstClef,
		})
	}

	// The staves, in the order they are first mentioned.
	var order []string
	byStaff := map[string][]int{}
	for i, l := range lines {
		if _, seen := byStaff[l.staff]; !seen {
			order = append(order, l.staff)
		}
		byStaff[l.staff] = append(byStaff[l.staff], i)
	}

	s := &model.Score{
		Identity: model.Identity{Title: rd.title, Composer: rd.composer},
		Tempo:    model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}
	if rd.tempo != nil {
		s.Tempo = *rd.tempo
	}

	for _, name := range order {
		idx := byStaff[name]
		lead := lines[idx[0]]
		p := &model.Part{
			ID:         name,
			Name:       lead.name,
			Instrument: lead.sound,
			Clef:       model.Treble,
			Key:        model.KeySignature{Fifths: 0, Mode: model.Major},
			Time:       model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		if lead.clef != nil {
			p.Clef = *lead.clef
		}
		if lead.key != nil {
			p.Key = *lead.key
		}
		if lead.time != nil {
			p.Time = *lead.time
		}
		p.Measures = lead.measures
		for _, m := range p.Measures {
			// The first voice too: a staff can be silent for a bar while
			// the others play, and it said so the same way.
			if len(m.Voices) == 1 && onlySilence(m.Voices[0]) {
				m.Voices[0] = &model.Voice{}
			}
		}

		// The other voices of this staff share its bars.
		for _, j := range idx[1:] {
			other := lines[j].measures
			for len(p.Measures) < len(other) {
				p.Measures = append(p.Measures, model.NewMeasure())
			}
			for k, m := range other {
				for _, v := range m.Voices {
					if onlySilence(v) {
						continue
					}
					p.Measures[k].Voices = append(p.Measures[k].Voices, v)
				}
			}
		}
		if len(p.Measures) == 0 {
			p.Measures = []*model.Measure{model.NewMeasure()}
		}
		// A piece ends with a finish bar unless it said otherwise, which is
		// what the engine does with a GMN file.
		last := p.Measures[len(p.Measures)-1]
		if last.RightBarline == model.BarSingle {
			last.RightBarline = model.BarFinal
		}
		s.Parts = append(s.Parts, p)
	}

	// The signature a part opens in belongs to the part and not to its
	// first bar, which would print it twice.
	for _, p := range s.Parts {
		first := p.Measures[0]
		if first.Key != nil && *first.Key == p.Key {
			first.Key = nil
		}
		if first.Time != nil && *first.Time == p.Time {
			first.Time = nil
		}
		if first.Clef != nil && *first.Clef == p.Clef {
			first.Clef = nil
		}
		// A tempo written at the first bar is the speed the piece starts
		// in, which the score itself holds. Left on the bar as well it is
		// printed twice, once by each.
		if first.Tempo != nil {
			if rd.tempo == nil {
				s.Tempo = *first.Tempo
			}
			first.Tempo = nil
		}
	}

	findTuplets(s, rd)
	s.Normalise()
	return s
}

// onlySilence reports a bar of a voice that holds nothing but the notes
// that take time without being printed.
//
// A voice silent for a bar has to say so in the file, or the bar is not
// there at all: its two barlines fall together, a reader sees one, and
// every later bar of that voice sits a place to the left of where it
// belongs. What it says is GUIDO's "empty" — a note that takes its time
// and is not printed — and this is where that is read back as the silence
// it stands for.
//
// Which means a bar somebody deliberately filled with one invisible rest
// comes back as an empty bar. On the page and in the ear they are the
// same bar; the difference is in the file and nowhere else.
func onlySilence(v *model.Voice) bool {
	if len(v.Elements) == 0 {
		return true
	}
	for _, el := range v.Elements {
		r, ok := el.(*model.Rest)
		if !ok || !r.Hidden {
			return false
		}
	}
	return true
}

// findTuplets works out the tuplets nobody wrote down.
//
// Basic GUIDO has no \tuplet tag: a triplet is three notes of 1/12, and
// that is the whole of it. gscore needs the ratio, because a duration of
// one twelfth is not a notehead — it cannot be drawn, it cannot be dotted,
// and the engraver has nothing to print. So a run of neighbouring notes
// whose lengths no notehead spells is tried against the ordinary ratios,
// and the first that makes every one of them writable is the tuplet the
// file meant.
//
// Only where the file did not say. An explicit \tuplet is believed.
func findTuplets(s *model.Score, rd *reader) {
	ratios := [][2]int{{3, 2}, {5, 4}, {6, 4}, {7, 4}, {9, 8}, {11, 8}, {13, 8}}
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for i := 0; i < len(v.Elements); {
					if !unwritable(v.Elements[i]) {
						i++
						continue
					}
					j := i
					for j < len(v.Elements) && unwritable(v.Elements[j]) {
						j++
					}
					run := v.Elements[i:j]
					fixed := false
					for _, r := range ratios {
						if !writableWith(run, r[0], r[1]) {
							continue
						}
						for k, el := range run {
							if mk := marksOf(el); mk != nil {
								mk.Tuplet = &model.Tuplet{
									Actual: r[0], Normal: r[1],
									Start: k == 0, Stop: k == len(run)-1,
								}
							}
						}
						fixed = true
						break
					}
					if !fixed {
						rd.complain(0,
							"a run of %d notes has lengths no notehead spells",
							len(run))
					}
					i = j
				}
			}
		}
	}
}

// unwritable reports an element whose length no notehead can be drawn as,
// and which is therefore part of a tuplet nobody declared.
func unwritable(el model.Element) bool {
	if mk := marksOf(el); mk != nil && mk.Tuplet != nil {
		return false
	}
	d := el.Duration()
	if d.IsZero() {
		return false
	}
	_, _, ok := d.Written()
	return !ok
}

func writableWith(run []model.Element, actual, normal int) bool {
	for _, el := range run {
		if _, _, ok := el.Duration().Scale(actual, normal).Reduced().Written(); !ok {
			return false
		}
	}
	return true
}
