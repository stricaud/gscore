/*
 * expand.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

/*
 * Working out what the source says, without running it.
 *
 * An include is read and put in its place, a variable is looked up, a tag
 * either keeps its music or drops it, a transposition moves the pitches, a
 * repeat is written out or marked, a relative pitch is resolved against the
 * note before. What is left is a flat list of events in playing order, each
 * with its place and its length, which is what a score is made of.
 *
 * Nothing here evaluates Scheme. Where the source does something only
 * LilyPond itself could work out, the thing is left out and said, so that
 * the reading can tell what it did not carry rather than quietly drop it.
 */

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// transposed moves a pitch by the interval between two others: the letter by
// so many steps and the sound by so many semitones, and the accidental is
// whatever makes the two agree — which is how a transposed F sharp becomes
// an A sharp rather than a B flat.
func transposed(p, from, to pitch) pitch {
	steps := (to.step - from.step) + 7*(to.octave-from.octave)
	semis := to.semitones() - from.semitones()
	n := p.step + steps
	step := ((n % 7) + 7) % 7
	octave := p.octave + floorDiv(n, 7)
	want := p.semitones() + semis
	plain := (octave+1)*12 + []int{0, 2, 4, 5, 7, 9, 11}[step]
	return pitch{step: step, alter: want - plain, octave: octave}
}

// relativeTo resolves a pitch written in relative mode against the note
// before. The letter goes to the octave that puts it within a fourth of the
// reference, up or down, and the octave marks move it from there. The
// accidental plays no part, which is LilyPond's rule and the one that
// surprises everybody once.
func relativeTo(ref, p pitch) pitch {
	marks := p.octave - 3
	d := ((p.step-ref.step)%7 + 7) % 7
	if d >= 4 {
		d -= 7
	}
	n := ref.step + d
	octave := ref.octave + floorDiv(n, 7)
	return pitch{step: p.step, alter: p.alter, octave: octave + marks}
}

// tupletRef is one \tuplet, which every note inside it points at, so that
// the first and the last of them can be told apart from the middle.
type tupletRef struct {
	num, den int
	outer    *tupletRef
}

// event is one thing in a staff's stream, with where it is and how long.
type event struct {
	note   *lyNote
	at     model.Duration
	dur    model.Duration
	grace  bool
	tuplet *tupletRef
	voice  int    // which line of music on the staff
	vname  string // the voice's name, where \new Voice = "…" gave one
	staff  string // where \change Staff sent it, or ""
}

// mark is what is written between the notes: a clef, a meter, a key, a
// barline, a rehearsal mark, a tempo, a word.
type mark struct {
	kind string
	args []any
	at   model.Duration
}

// voicesItem is several lines of music at once on one staff.
type voicesItem struct {
	subs [][]any
}

// context is where files are found, what the names mean, what is kept.
type context struct {
	base      string
	variables map[string]node
	where     map[string]string // the directory each came from
	language  string
	lg        *log
	header    map[string]string
	loaded    map[string]bool
	markNums  bool
	markups   map[string]string            // name = \markup { … }: the words
	dynTexts  map[string]string            // name = #(make-dynamic-script "…"): a dynamic of the score's own
	eventFns  map[string]bool              // name = #(define-event-function …): a mark this reader cannot make
	headers   map[string]map[string]string // each file's own \header
	mainPath  string
}

func newContext(base string, lg *log) *context {
	return &context{base: base, variables: map[string]node{}, where: map[string]string{},
		language: "nederlands", lg: lg, header: map[string]string{}, loaded: map[string]bool{},
		markups: map[string]string{}, headers: map[string]map[string]string{},
		dynTexts: map[string]string{}, eventFns: map[string]bool{}}
}

var (
	assignmentRe = regexp.MustCompile(`(?m)^([a-zA-Z][a-zA-Z0-9]*(?:-[a-zA-Z0-9]+)*)\s*=\s*`)
	languageRe   = regexp.MustCompile(`\\language\s*"([^"]+)"`)
)

// definitions finds every `name = music` in a file.
//
// A definition written as a Scheme music function is taken by its body:
// the `#{ ... #}` between the braces is LilyPond like any other, and the
// arguments such functions take are spliced in as nothing.
func (ctx *context) definitions(text string) map[string]node {
	out := map[string]node{}
	for _, m := range assignmentRe.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[2]:m[3]]
		rest := text[m[1]:]
		trimmed := strings.TrimLeft(rest, " \t\r\n")
		body := ""
		switch {
		case strings.Contains(firstStatement(trimmed), "make-dynamic-script"):
			// A dynamic of the score's own: "pp leggiermente", written
			// as a dynamic is and sounding as its first word says.
			if words := dynamicWords(firstStatement(trimmed)); words != "" {
				ctx.dynTexts[name] = words
			} else {
				ctx.eventFns[name] = true
			}
			continue
		case strings.HasPrefix(trimmed, "#(define-event-function"):
			ctx.eventFns[name] = true
			continue
		case strings.HasPrefix(trimmed, "#(define-music-function"):
			at := strings.Index(rest, "#{")
			end := -1
			if at >= 0 {
				end = strings.Index(rest[at+2:], "#}")
			}
			if at >= 0 && end >= 0 {
				body = rest[at+2 : at+2+end]
			}
		case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "<<"):
			r := newReader(rest, ctx.language)
			if strings.HasPrefix(trimmed, "{") {
				body = r.balanced("{", "}")
			} else {
				body = r.balanced("<<", ">>")
			}
		case strings.HasPrefix(trimmed, `\relative`) || strings.HasPrefix(trimmed, `\transpose`) ||
			strings.HasPrefix(trimmed, `\new`) || strings.HasPrefix(trimmed, `\context`) ||
			strings.HasPrefix(trimmed, `\drummode`) || strings.HasPrefix(trimmed, `\lyricmode`) ||
			strings.HasPrefix(trimmed, `\fixed`) || strings.HasPrefix(trimmed, `\absolute`) ||
			strings.HasPrefix(trimmed, `\repeat`) || strings.HasPrefix(trimmed, `\tuplet`) ||
			strings.HasPrefix(trimmed, `\relative`) || strings.HasPrefix(trimmed, `\drums`):
			// One music expression: read from here, and take what the
			// reader took.
			r := newReader(rest, ctx.language)
			item := r.one()
			if item != nil {
				out[name] = item
			}
			continue
		case strings.HasPrefix(trimmed, `\markup`):
			// A word to be written on a note: -\nonLigato.
			r := newReader(trimmed[len(`\markup`):], ctx.language)
			ctx.markups[name] = r.markup()
			continue
		case strings.HasPrefix(trimmed, `"`):
			r := newReader(trimmed, ctx.language)
			ctx.markups[name] = r.str()
			continue
		default:
			continue
		}
		if body == "" {
			continue
		}
		body = regexp.MustCompile(`\$[a-zA-Z-]+`).ReplaceAllString(body, " ")
		music, unread, _ := read(body, ctx.language)
		for _, u := range unread {
			ctx.lg.say("something in %s could not be read: %s", name, short(u))
		}
		out[name] = music
	}
	return out
}

// firstStatement is a definition's value up to the next definition or the
// end of the file.
func firstStatement(s string) string {
	if m := assignmentRe.FindStringIndex(s[1:]); m != nil {
		return s[:m[0]+1]
	}
	return s
}

var (
	dynamicStringRe = regexp.MustCompile(`make-dynamic-script\s*"([^"]*)"`)
	dynamicMarkupRe = regexp.MustCompile(`#\{\s*\\markup([\s\S]*?)#\}`)
)

// dynamicWords is what a dynamic made by make-dynamic-script says.
func dynamicWords(s string) string {
	if m := dynamicStringRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if m := dynamicMarkupRe.FindStringSubmatch(s); m != nil {
		r := newReader(m[1], "nederlands")
		return r.markup()
	}
	return ""
}

func short(s string) string {
	if len(s) > 24 {
		return s[:24]
	}
	return s
}

// loadText reads a text as a file would be read: its language, its
// definitions, its music.
func (ctx *context) loadText(text, path string) *seqNode {
	if m := languageRe.FindStringSubmatch(text); m != nil {
		ctx.language = m[1]
	}
	found := ctx.definitions(text)
	for name, music := range found {
		ctx.variables[name] = music
		ctx.where[name] = filepath.Dir(path)
	}
	music, unread, header := read(text, ctx.language)
	for _, u := range unread {
		ctx.lg.say("something in %s could not be read: %s", filepath.Base(path), short(u))
	}
	ctx.headers[path] = header
	for k, v := range header {
		if _, have := ctx.header[k]; !have {
			ctx.header[k] = v
		}
	}
	return music
}

// load reads a file and everything it includes, and learns its definitions.
func (ctx *context) load(path string) (*seqNode, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	ctx.loaded[path] = true
	return ctx.loadText(string(b), path), true
}

// resolve finds an included file: beside the one that asks for it, or
// beside the main file.
func (ctx *context) resolve(here, name string) string {
	for _, dir := range []string{here, ctx.base} {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

/*
 * Expanding
 */

type rel struct{ ref pitch }

// hold is what a mark before a note has said about it: the sign over the
// next trill, the octave line, the words, the staff it moved to.
type hold struct {
	trillAcc    string
	ottava      *int
	lastPitch   *pitch
	lastPitches []pitch
	words       []string
	staff       string
	staffSet    bool
	tempoText   string
	tuplets     int
	dynamic     string // a dynamic written before its note
}

// state is what is in force at this point in the walk.
type walkState struct {
	transposes [][2]pitch
	tags       []string
	grace      bool
	tuplet     *tupletRef
	drop       []string
	keep       map[string]bool
	rel        *rel
	hold       *hold
	vname      string
}

func (st walkState) with(f func(*walkState)) walkState {
	c := st
	f(&c)
	return c
}

// flatten is the music as a list of events, in order, with nothing left to
// resolve. here is the directory the music came from, since an include is
// relative to the file that asks for it.
func (ctx *context) flatten(music node, here string) []any {
	var out []any
	h := &hold{}
	ctx.walk(music, here, &out, walkState{hold: h})
	closePending(h, out)
	timeStream(out, model.Duration{Num: 0, Den: 1}, model.Duration{Num: 1, Den: 4}, 1)
	return out
}

// closePending ends on a voice's last note what was written after it:
// \ottava #0 at the end of a voice ends the line there, and would
// otherwise be carried to the next note read, which is another voice's.
func closePending(h *hold, stream []any) {
	if h == nil || h.ottava == nil || *h.ottava != 0 {
		return
	}
	le := lastEvent(stream)
	if le == nil {
		return
	}
	if le.note.ottava == nil {
		z := 0
		le.note.ottava = &z
	}
	h.ottava = nil
}

func (ctx *context) walk(music node, here string, out *[]any, st walkState) {
	switch m := music.(type) {
	case nil:
		return
	case *seqNode:
		for _, item := range m.items {
			ctx.walk(item, here, out, st)
		}
	case *simNode:
		var kept [][]any
		// Every branch starts from the same note before: a << >> inside
		// \relative is read as LilyPond reads it, each branch relative to
		// what came before the bracket, and what follows it relative to
		// the end of the first branch.
		var refBefore pitch
		var refAfter *pitch
		if st.rel != nil {
			refBefore = st.rel.ref
		}
		for _, item := range m.items {
			if st.rel != nil {
				st.rel.ref = refBefore
			}
			var sub []any
			ctx.walk(item, here, &sub, st)
			closePending(st.hold, sub)
			if st.rel != nil && refAfter == nil {
				r := st.rel.ref
				refAfter = &r
			}
			if len(sub) > 0 {
				kept = append(kept, sub)
			}
		}
		if st.rel != nil && refAfter != nil {
			st.rel.ref = *refAfter
		}
		if len(kept) == 0 {
			return
		}
		sounding := 0
		for _, sub := range kept {
			if hasSound(sub) {
				sounding++
			}
		}
		if sounding >= 2 {
			// Two lines of music on one staff at once, which is what the
			// bracket says. Each is a voice of its own.
			*out = append(*out, &voicesItem{subs: kept})
			return
		}
		// One branch has the notes; the others are marks, skips or
		// nothing — Boléro's \global, a run of skips carrying the meter
		// and the rehearsal marks beside every part. Their marks are
		// put among the notes at their own places, and the music is
		// made as long as the longest of them.
		first := -1
		for i, sub := range kept {
			if hasSound(sub) {
				first = i
				break
			}
		}
		if first < 0 {
			first = 0
		}
		main := kept[first]
		for i, sub := range kept {
			if i != first {
				main = mergeBeside(main, sub)
			}
		}
		*out = append(*out, main...)
	case *relativeNode:
		// \relative with no pitch: the first note is written as if
		// absolute, which LilyPond gets by measuring from f.
		ref := pitch{step: 3, octave: 3}
		if m.pitch != nil {
			ref = *m.pitch
		}
		inner := &rel{ref: ref}
		ctx.walk(m.music, here, out, st.with(func(s *walkState) { s.rel = inner }))
		// A \relative inside a \relative is sealed: the music after it
		// goes on from the note before it, not from the last note
		// inside. That is LilyPond's rule.
	case *includeNode:
		path := ctx.resolve(here, m.path)
		if path == "" {
			ctx.lg.say("a file that is not there: %s", m.path)
			return
		}
		inner, ok := ctx.load(path)
		if !ok {
			ctx.lg.say("a file that could not be read: %s", m.path)
			return
		}
		ctx.walk(inner, filepath.Dir(path), out, st)
	case *refNode:
		got, ok := ctx.variables[m.name]
		if !ok {
			if words, isText := ctx.markups[m.name]; isText {
				st.hold.words = append(st.hold.words, words)
				return
			}
			if _, isDyn := ctx.dynTexts[m.name]; isDyn {
				st.hold.dynamic = m.name
				return
			}
			if ctx.eventFns[m.name] {
				return
			}
			ctx.lg.say(`a name nothing defines: \%s`, m.name)
			return
		}
		where := ctx.where[m.name]
		if where == "" {
			where = here
		}
		ctx.walk(got, where, out, st)
	case *tagNode:
		switch {
		case m.keep:
			// \keepWithTag: from here down, music tagged with one of
			// these names is kept and music tagged with any other is
			// dropped. Music with no tag at all is everybody's.
			keep := map[string]bool{}
			for _, n := range m.names {
				keep[n] = true
			}
			ctx.walk(m.music, here, out, st.with(func(s *walkState) { s.keep = keep }))
		case m.drop:
			ctx.walk(m.music, here, out, st)
		default:
			if st.keep != nil && len(m.names) > 0 {
				any := false
				for _, n := range m.names {
					if st.keep[n] {
						any = true
					}
				}
				if !any {
					return
				}
			}
			ctx.walk(m.music, here, out, st.with(func(s *walkState) {
				s.tags = append(append([]string(nil), st.tags...), m.names...)
			}))
		}
	case *transposeNode:
		if m.from == nil || m.to == nil {
			ctx.lg.say("a transposition that could not be read")
			ctx.walk(m.music, here, out, st)
			return
		}
		ctx.walk(m.music, here, out, st.with(func(s *walkState) {
			s.transposes = append(append([][2]pitch(nil), st.transposes...), [2]pitch{*m.from, *m.to})
		}))
	case *filterNode:
		ctx.walk(m.music, here, out, st.with(func(s *walkState) {
			s.drop = append(append([]string(nil), st.drop...), m.kinds...)
		}))
	case *repeatNode:
		switch m.kind {
		case "unfold", "percent", "tremolo":
			n := m.count
			if n < 1 {
				n = 1
			}
			if m.kind != "unfold" {
				ctx.lg.say("a %s repeat was written out in full", m.kind)
			}
			for i := 0; i < n; i++ {
				ctx.walk(m.music, here, out, st)
			}
		case "volta":
			// Written once, between repeat barlines, with its endings
			// under their brackets: a score shows a repeat, it does not
			// play it out.
			*out = append(*out, &mark{kind: "bar", args: []any{"|:"}})
			ctx.walk(m.music, here, out, st)
			if len(m.alternatives) == 0 {
				*out = append(*out, &mark{kind: "bar", args: []any{":|"}})
				return
			}
			for k, alt := range m.alternatives {
				*out = append(*out, &mark{kind: "ending", args: []any{k + 1, "start", len(m.alternatives)}})
				ctx.walk(alt, here, out, st)
				*out = append(*out, &mark{kind: "ending", args: []any{k + 1, "stop", len(m.alternatives)}})
				if k < len(m.alternatives)-1 {
					*out = append(*out, &mark{kind: "bar", args: []any{":|"}})
				}
			}
		default:
			ctx.lg.say("a repeat of a kind not known here: %s", m.kind)
			ctx.walk(m.music, here, out, st)
		}
	case *tupletNode:
		t := &tupletRef{num: m.num, den: m.den, outer: st.tuplet}
		ctx.walk(m.music, here, out, st.with(func(s *walkState) { s.tuplet = t }))
	case *graceNode:
		kind := m.kind
		ctx.walk(m.music, here, out, st.with(func(s *walkState) { s.grace = true }))
		// Which kind it was is kept on the notes themselves.
		for i := len(*out) - 1; i >= 0; i-- {
			e, ok := (*out)[i].(*event)
			if !ok {
				break
			}
			if !e.grace || e.note.texts != nil && false {
				break
			}
			if e.note.tag == nil {
				e.note.tag = []string{"grace:" + kind}
			}
		}
	case *contextNode:
		switch m.kind {
		case "Voice", "CueVoice", "NullVoice":
			ctx.walk(m.music, here, out, st.with(func(s *walkState) {
				if m.name != "" {
					s.vname = m.name
				}
			}))
		case "Lyrics":
			if ly, ok := m.music.(*lyricsNode); ok {
				*out = append(*out, ly)
			}
		case "Staff", "DrumStaff", "RhythmicStaff", "TabStaff", "PianoStaff",
			"GrandStaff", "StaffGroup", "ChoirStaff", "Devnull", "ChordNames",
			"Dynamics", "FiguredBass":
			// A staff inside a staff's music is somebody else's: it is
			// found by stavesIn, not played here.
			if m.kind == "Dynamics" || m.kind == "ChordNames" || m.kind == "FiguredBass" {
				return
			}
			ctx.walk(m.music, here, out, st)
		default:
			ctx.walk(m.music, here, out, st)
		}
	case *lyricsNode:
		*out = append(*out, m)
	case *lyNote:
		*out = append(*out, ctx.event(m, st))
	case *commandNode:
		ctx.command(m, here, out, st)
	}
}

// mergeBeside puts the marks of a stream of skips among the events of the
// stream it ran beside, each at its own place, and pads the music to the
// skips' length where they ran on longer.
func mergeBeside(main, beside []any) []any {
	mainEnd, _ := timeStream(main, model.Duration{Num: 0, Den: 1}, model.Duration{Num: 1, Den: 4}, 1)
	besideEnd, _ := timeStream(beside, model.Duration{Num: 0, Den: 1}, model.Duration{Num: 1, Den: 4}, 1)
	var marks []*mark
	for _, x := range beside {
		if mk, ok := x.(*mark); ok {
			marks = append(marks, mk)
		}
	}
	var out []any
	k := 0
	place := func(at model.Duration) {
		for k < len(marks) && marks[k].at.Cmp(at) <= 0 {
			out = append(out, marks[k])
			k++
		}
	}
	for _, x := range main {
		switch e := x.(type) {
		case *event:
			place(e.at)
		case *mark:
			place(e.at)
		case *voicesItem:
			if len(e.subs) > 0 {
				if es := eventsIn(e.subs[0]); len(es) > 0 {
					place(es[0].at)
				}
			}
		}
		out = append(out, x)
	}
	if besideEnd.Cmp(mainEnd) > 0 {
		d := besideEnd.Sub(mainEnd)
		pad := &lyNote{skip: true, dur: &d}
		out = append(out, &event{note: pad})
	}
	for k < len(marks) {
		out = append(out, marks[k])
		k++
	}
	return out
}

// hasSound says whether a stream has a note in it, as against marks and
// skips alone.
func hasSound(stream []any) bool {
	for _, x := range stream {
		switch e := x.(type) {
		case *event:
			if !e.note.skip {
				return true
			}
		case *voicesItem:
			for _, sub := range e.subs {
				if hasSound(sub) {
					return true
				}
			}
		}
	}
	return false
}

func lastEvent(out []any) *event {
	for i := len(out) - 1; i >= 0; i-- {
		switch e := out[i].(type) {
		case *event:
			return e
		case *voicesItem:
			if len(e.subs) > 0 {
				if le := lastEvent(e.subs[0]); le != nil {
					return le
				}
			}
			return nil
		}
	}
	return nil
}

func (ctx *context) command(m *commandNode, here string, out *[]any, st walkState) {
	h := st.hold
	switch m.name {
	case "after":
		// A mark reached after the note it belongs to.
		le := lastEvent(*out)
		if le == nil {
			return
		}
		switch a := m.args[0].(type) {
		case string:
			switch a {
			case "~":
				le.note.tie = true
			case "(":
				le.note.slurStart = true
			case ")":
				le.note.slurStop = true
			case "[":
				le.note.beamStart = true
			case "]":
				le.note.beamStop = true
			case `\(`:
				le.note.phraseStart = true
			case `\)`:
				le.note.phraseStop = true
			case `\<`:
				le.note.hairpin = "cresc"
			case `\>`:
				le.note.hairpin = "dim"
			case `\!`:
				le.note.hairpinStop = true
			}
		case *lyNote:
			mergeMarks(le.note, a)
		}
	case "breathe":
		if le := lastEvent(*out); le != nil {
			le.note.breath = true
		}
	case "trillacc":
		h.trillAcc, _ = m.args[0].(string)
	case "ottava":
		v, _ := m.args[0].(int)
		h.ottava = &v
	case "words":
		h.words = append(h.words, m.args[0].(string))
	case "change":
		name, _ := m.args[0].(string)
		h.staff, h.staffSet = name, true
	case "measureLength", "clef", "time", "key", "mark", "bar", "partial", "tempo",
		"numericTime", "transposition", "instrumentName", "shortInstrumentName",
		"midiInstrument":
		*out = append(*out, &mark{kind: m.name, args: m.args})
	case "markFormatter":
		if v, _ := m.args[0].(string); strings.Contains(v, "numbers") {
			ctx.markNums = true
		}
	case "drums":
		ctx.walk(m.args[0], here, out, st)
	case "unfold":
		ctx.walk(m.args[0], here, out, st)
	case "score", "book", "bookpart":
		ctx.walk(m.args[0], here, out, st)
	case "markup":
		// A markup standing on its own between notes: a word for the
		// next note.
		if text, _ := m.args[0].(string); text != "" {
			h.words = append(h.words, text)
		}
	case "voice":
	}
}

// mergeMarks copies what was written on a mark-only note onto the note it
// belongs to.
func mergeMarks(dst, src *lyNote) {
	dst.artics = append(dst.artics, src.artics...)
	dst.texts = append(dst.texts, src.texts...)
	dst.fingering = append(dst.fingering, src.fingering...)
	if src.dynamic != "" {
		dst.dynamic = src.dynamic
	}
	if src.hairpin != "" {
		dst.hairpin, dst.hairpinText = src.hairpin, src.hairpinText
	}
	dst.hairpinStop = dst.hairpinStop || src.hairpinStop
	dst.trillStart = dst.trillStart || src.trillStart
	dst.trillStop = dst.trillStop || src.trillStop
	if src.pedal != "" {
		dst.pedal = src.pedal
	}
	dst.breath = dst.breath || src.breath
	dst.gliss = dst.gliss || src.gliss
	dst.tie = dst.tie || src.tie
	dst.slurStart = dst.slurStart || src.slurStart
	dst.slurStop = dst.slurStop || src.slurStop
	dst.phraseStart = dst.phraseStart || src.phraseStart
	dst.phraseStop = dst.phraseStop || src.phraseStop
	dst.beamStart = dst.beamStart || src.beamStart
	dst.beamStop = dst.beamStop || src.beamStop
}

func (ctx *context) event(src *lyNote, st walkState) *event {
	n := *src
	n.artics = append([]string(nil), src.artics...)
	n.texts = append([]textAt(nil), src.texts...)
	n.fingering = append([]string(nil), src.fingering...)
	n.pitches = append([]pitch(nil), src.pitches...)
	if src.pitch != nil {
		p := *src.pitch
		n.pitch = &p
	}
	h := st.hold
	switch {
	case n.same:
		// A bare length after a note is that note again: "d8 8 8", or a
		// chord again. The pitches are the ones already worked out.
		if len(h.lastPitches) > 0 {
			n.pitches = append([]pitch(nil), h.lastPitches...)
		} else if h.lastPitch != nil {
			p := *h.lastPitch
			n.pitch = &p
		}
		if n.pitch == nil && len(n.pitches) == 0 && n.drum == "" {
			n.rest = true
		}
	case st.rel != nil:
		// Relative mode: a note from the one before, a chord's notes each
		// from the one before it in the chord, and what follows a chord
		// from the chord's first note.
		if n.pitch != nil {
			p := relativeTo(st.rel.ref, *n.pitch)
			n.pitch = &p
			st.rel.ref = p
		} else if len(n.pitches) > 0 {
			ref := st.rel.ref
			for i, p := range n.pitches {
				ref = relativeTo(ref, p)
				n.pitches[i] = ref
			}
			st.rel.ref = n.pitches[0]
		}
	}
	if !n.same && (n.pitch != nil || len(n.pitches) > 0) {
		h.lastPitch = nil
		if n.pitch != nil {
			p := *n.pitch
			h.lastPitch = &p
		}
		h.lastPitches = append([]pitch(nil), n.pitches...)
	}
	if n.trillStart {
		n.trillAcc = h.trillAcc
		h.trillAcc = ""
	}
	if h.ottava != nil {
		n.ottava = h.ottava
		h.ottava = nil
	}
	if len(h.words) > 0 {
		for _, w := range h.words {
			n.texts = append(n.texts, textAt{text: w, place: '^'})
		}
		h.words = nil
	}
	if h.dynamic != "" {
		if n.dynamic == "" {
			n.dynamic = h.dynamic
		}
		h.dynamic = ""
	}
	for _, t := range st.transposes {
		if n.pitch != nil {
			p := transposed(*n.pitch, t[0], t[1])
			n.pitch = &p
		}
		for i, p := range n.pitches {
			n.pitches[i] = transposed(p, t[0], t[1])
		}
	}
	for _, kind := range st.drop {
		switch kind {
		case "SlurEvent":
			n.slurStart, n.slurStop = false, false
		case "PhrasingSlurEvent":
			n.phraseStart, n.phraseStop = false, false
		case "ArticulationEvent":
			n.artics = nil
		case "TextScriptEvent":
			n.texts = nil
		case "DynamicEvent", "AbsoluteDynamicEvent":
			n.dynamic = ""
		case "CrescendoEvent", "DecrescendoEvent":
			n.hairpin, n.hairpinStop = "", false
		case "TieEvent":
			n.tie = false
		}
	}
	e := &event{note: &n, grace: st.grace, tuplet: st.tuplet, vname: st.vname}
	if h.staffSet {
		e.staff = h.staff
	}
	return e
}

// timeStream gives every event its length and its place, reading them in
// order.
//
// A note with no length written takes the one before it, which is how
// LilyPond reads them too, and the length a tuplet writes is scaled by its
// ratio: three in the time of two is each note two thirds of what it says.
//
// A voicesItem is several lines at once: each is timed from the same place,
// numbered as a voice, and the whole takes as long as the longest of them.
// The marks get their place too, so that a clef written between two notes
// keeps its spot.
func timeStream(stream []any, start, last model.Duration, voice int) (model.Duration, model.Duration) {
	at := start
	for _, x := range stream {
		switch e := x.(type) {
		case *voicesItem:
			// The length a note takes when none is written is the last
			// one written in the text, whichever voice it was in: the
			// second voice starts from the first voice's last, and what
			// follows the bracket from the last voice's.
			end := at
			for k, sub := range e.subs {
				subEnd, l := timeStream(sub, at, last, k+1)
				last = l
				if subEnd.Cmp(end) > 0 {
					end = subEnd
				}
			}
			at = end
		case *mark:
			e.at = at
		case *event:
			d := last
			if e.note.dur != nil {
				d = *e.note.dur
				if !e.grace {
					last = d
				}
			}
			for t := e.tuplet; t != nil; t = t.outer {
				if t.num > 0 && t.den > 0 {
					d = d.Scale(t.den, t.num)
				}
			}
			if e.grace {
				e.dur = model.Duration{Num: 0, Den: 1}
			} else {
				e.dur = d
			}
			e.at = at
			e.voice = voice
			at = at.Add(e.dur)
		}
	}
	return at, last
}

// eventsIn is every event in a stream, voices and all, in order.
func eventsIn(stream []any) []*event {
	var out []*event
	for _, x := range stream {
		switch e := x.(type) {
		case *event:
			out = append(out, e)
		case *voicesItem:
			for _, sub := range e.subs {
				out = append(out, eventsIn(sub)...)
			}
		}
	}
	return out
}

// marksIn is every placed mark in a stream, voices and all.
func marksIn(stream []any) []*mark {
	var out []*mark
	for _, x := range stream {
		switch e := x.(type) {
		case *mark:
			out = append(out, e)
		case *voicesItem:
			for _, sub := range e.subs {
				out = append(out, marksIn(sub)...)
			}
		}
	}
	return out
}

// lyricsOf is every run of words in a stream.
func lyricsOf(stream []any) []*lyricsNode {
	var out []*lyricsNode
	for _, x := range stream {
		switch e := x.(type) {
		case *lyricsNode:
			out = append(out, e)
		case *voicesItem:
			for _, sub := range e.subs {
				out = append(out, lyricsOf(sub)...)
			}
		}
	}
	return out
}

// describe is a pitch as a note name, for messages.
func describe(p pitch) string {
	acc := map[int]string{-2: "bb", -1: "b", 0: "", 1: "#", 2: "x"}[p.alter]
	return fmt.Sprintf("%c%s%d", "CDEFGAB"[p.step], acc, p.octave)
}
