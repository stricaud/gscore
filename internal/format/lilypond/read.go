/*
 * read.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

/*
 * Reading LilyPond source: pitches, durations, and a tree of music.
 *
 * LilyPond source is a program, and this is not LilyPond: nothing here runs
 * Scheme or evaluates a music function. The text is read as text, which is
 * enough for what a score is made of — the notes, their lengths, what is
 * written on them and how the staves are arranged — and whatever cannot be
 * worked out by reading is written down to be said, not guessed at.
 *
 * The reading comes back as a tree. Nothing is resolved at this stage: a
 * variable is a ref, an include is an include, a tag is a tag around its
 * music. expand.go does that part, so that reading stays separate from
 * working out what a name stood for.
 */

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// pitch is a written pitch: which letter, how many semitones off it, which
// octave. step is 0..6 for C..B, alter is in semitones, and octave is
// scientific — LilyPond's c' is middle C, which is octave 4 here.
type pitch struct {
	step, alter, octave int
}

func (p pitch) semitones() int {
	return (p.octave+1)*12 + []int{0, 2, 4, 5, 7, 9, 11}[p.step] + p.alter
}

// A note name is a letter and an accidental spelled onto it, and each
// language spells them its own way. The bare name is the octave below
// middle C; each ' goes up an octave and each , down.
type noteNames struct {
	steps    map[string]int // the letters, longest first where one is a prefix of another
	suffixes map[string]int // the accidentals
	whole    map[string]pitch
}

var languages = map[string]*noteNames{}

func init() {
	latin := map[string]int{"do": 0, "re": 1, "mi": 2, "fa": 3, "sol": 4, "la": 5, "si": 6}
	letters := map[string]int{"c": 0, "d": 1, "e": 2, "f": 3, "g": 4, "a": 5, "b": 6}
	german := map[string]int{"c": 0, "d": 1, "e": 2, "f": 3, "g": 4, "a": 5, "h": 6}
	languages["nederlands"] = &noteNames{steps: letters, suffixes: map[string]int{
		"": 0, "is": 1, "isis": 2, "es": -1, "eses": -2, "s": -1, "ses": -2}}
	languages["english"] = &noteNames{steps: letters, suffixes: map[string]int{
		"": 0, "s": 1, "ss": 2, "x": 2, "f": -1, "ff": -2,
		"sharp": 1, "sharpsharp": 2, "flat": -1, "flatflat": -2}}
	languages["deutsch"] = &noteNames{steps: german, suffixes: map[string]int{
		"": 0, "is": 1, "isis": 2, "es": -1, "eses": -2, "s": -1, "ses": -2},
		whole: map[string]pitch{"b": {6, -1, 0}, "heses": {6, -2, 0}}}
	languages["norsk"] = &noteNames{steps: german, suffixes: map[string]int{
		"": 0, "is": 1, "iss": 1, "isis": 2, "ississ": 2, "es": -1, "ess": -1,
		"eses": -2, "essess": -2, "s": -1, "ss": -1, "ses": -2},
		whole: map[string]pitch{"b": {6, -1, 0}}}
	languages["suomi"] = languages["deutsch"]
	languages["svenska"] = &noteNames{steps: german, suffixes: map[string]int{
		"": 0, "iss": 1, "ississ": 2, "ess": -1, "essess": -2, "ss": -1},
		whole: map[string]pitch{"b": {6, -1, 0}}}
	languages["italiano"] = &noteNames{steps: latin, suffixes: map[string]int{
		"": 0, "d": 1, "dd": 2, "b": -1, "bb": -2}}
	french := map[string]int{"do": 0, "re": 1, "ré": 1, "mi": 2, "fa": 3, "sol": 4, "la": 5, "si": 6}
	languages["français"] = &noteNames{steps: french, suffixes: map[string]int{
		"": 0, "d": 1, "dd": 2, "x": 2, "b": -1, "bb": -2}}
	languages["francais"] = languages["français"]
	languages["español"] = &noteNames{steps: latin, suffixes: map[string]int{
		"": 0, "s": 1, "ss": 2, "x": 2, "b": -1, "bb": -2}}
	languages["espanol"] = languages["español"]
	languages["catalan"] = &noteNames{steps: latin, suffixes: map[string]int{
		"": 0, "d": 1, "s": 1, "dd": 2, "ss": 2, "b": -1, "bb": -2}}
	languages["català"] = languages["catalan"]
	languages["portugues"] = &noteNames{steps: latin, suffixes: map[string]int{
		"": 0, "s": 1, "ss": 2, "b": -1, "bb": -2}}
	languages["português"] = languages["portugues"]
	languages["vlaams"] = &noteNames{steps: latin, suffixes: map[string]int{
		"": 0, "k": 1, "kk": 2, "b": -1, "bb": -2}}
}

// parsePitch reads one note name with its octave marks.
func parsePitch(word string, language string) (pitch, bool) {
	names := languages[language]
	if names == nil {
		names = languages["nederlands"]
	}
	i := 0
	for i < len(word) && word[i] != '\'' && word[i] != ',' {
		i++
	}
	name, marks := word[:i], word[i:]
	octave := 3 + strings.Count(marks, "'") - strings.Count(marks, ",")
	if p, ok := names.whole[name]; ok {
		p.octave += octave
		return p, true
	}
	// The longest letter that starts the name: "sol" before "s", "ré"
	// before "re".
	best, bestStep, found := "", 0, false
	for base, step := range names.steps {
		if strings.HasPrefix(name, base) && len(base) > len(best) {
			if alter, ok := names.suffixes[name[len(base):]]; ok {
				_ = alter
				best, bestStep, found = base, step, true
			}
		}
	}
	if !found {
		return pitch{}, false
	}
	return pitch{step: bestStep, alter: names.suffixes[name[len(best):]], octave: octave}, true
}

// durationOf reads 4, 8., 2.., 1*2, \breve — the length of one note, as a
// fraction of a whole note.
var durationRe = regexp.MustCompile(`^(\\?[a-z]+|\d+)(\.*)((?:\*\d+(?:/\d+)?)*)$`)

func durationOf(token string) (model.Duration, bool) {
	m := durationRe.FindStringSubmatch(token)
	if m == nil {
		return model.Duration{}, false
	}
	var value model.Duration
	switch m[1] {
	case `\breve`:
		value = model.Duration{Num: 2, Den: 1}
	case `\longa`:
		value = model.Duration{Num: 4, Den: 1}
	case `\maxima`:
		value = model.Duration{Num: 8, Den: 1}
	default:
		n, err := strconv.Atoi(m[1])
		if err != nil || n == 0 || n&(n-1) != 0 {
			return model.Duration{}, false // not a power of two: not a length
		}
		value = model.Duration{Num: 1, Den: n}
	}
	// A dot adds half of what is there, twice over for two dots.
	whole := value
	half := value
	for range m[2] {
		half = half.Scale(1, 2)
		whole = whole.Add(half)
	}
	for _, each := range regexp.MustCompile(`\*(\d+)(?:/(\d+))?`).FindAllStringSubmatch(m[3], -1) {
		num, _ := strconv.Atoi(each[1])
		den := 1
		if each[2] != "" {
			den, _ = strconv.Atoi(each[2])
		}
		if den > 0 {
			whole = whole.Scale(num, den)
		}
	}
	return whole, true
}

/*
 * The tree
 */

// textAt is a word written on a note, and which side it was asked for.
type textAt struct {
	text  string
	place byte // '^' above, '_' below, '-' either
}

// lyNote is a note, a chord, a rest or a skip as written, with everything
// hung on it.
type lyNote struct {
	pitch                   *pitch
	pitches                 []pitch // a chord has several
	drum                    string  // the kit piece, in a drum staff
	dur                     *model.Duration
	rest                    bool
	skip                    bool // s, which takes time and draws nothing
	multi                   bool // R, a bar's rest or several
	same                    bool // a bare length: the note before again
	tie                     bool
	slurStart, slurStop     bool
	phraseStart, phraseStop bool
	beamStart, beamStop     bool
	artics                  []string
	dynamic                 string
	hairpin                 string // "cresc" or "dim", where one opens
	hairpinStop             bool
	hairpinText             bool // \cresc: the word, not the wedge
	breath                  bool
	gliss                   bool
	texts                   []textAt
	cautionary              bool
	forced                  bool
	trillStart              bool
	trillStop               bool
	trillAcc                string
	pedal                   string // "start", "stop"
	ottava                  *int
	fingering               []string
	arpeggio                bool
	tremolo                 int // strokes, for c4:32
	tempoText               string
	tag                     []string // the tag written on the note itself
}

type seqNode struct{ items []node }
type simNode struct{ items []node }
type refNode struct{ name string }
type includeNode struct{ path string }
type relativeNode struct {
	pitch *pitch
	music node
}
type tagNode struct {
	names []string
	music node
	keep  bool // \keepWithTag, which filters rather than labels
	drop  bool // \removeWithTag
}
type transposeNode struct {
	from, to *pitch
	music    node
}
type repeatNode struct {
	kind         string
	count        int
	music        node
	alternatives []node
}
type tupletNode struct {
	num, den int
	music    node
}
type graceNode struct {
	kind  string // grace, acciaccatura, appoggiatura, slashedGrace, after
	music node
}
type filterNode struct {
	kinds []string
	music node
}

// contextNode is \new Kind = "name" \with { ... } music.
type contextNode struct {
	kind, name string
	with       withBlock
	music      node
}

type withBlock struct {
	instrumentName, shortName string
	midiInstrument            string
	lines                     int
	text                      string
}

// lyricsNode is a run of words: \addlyrics { }, \lyricmode { }, or the
// body of \lyricsto "voice" { }.
type lyricsNode struct {
	to        string // the voice named by \lyricsto
	syllables []syllable
}

type syllable struct {
	text   string
	hyphen bool // -- after it: the word goes on
	extend bool // __ after it: the syllable is held
	skip   bool // _ : a note with no word
	dur    *model.Duration
}

// commandNode is anything else: \clef "treble", \time 3/4, \mark \default,
// \bar "|.", and the marks reached out of order that belong to the note
// before.
type commandNode struct {
	name string
	args []any
}

type node any

/*
 * The reader
 */

type reader struct {
	s         string
	i, n      int
	language  string
	unread    []string
	lastChord []pitch
	drums     bool
	lastDrum  string
	lyrics    bool
	header    map[string]string
}

func newReader(text, language string) *reader {
	if language == "" {
		language = "nederlands"
	}
	return &reader{s: text, n: len(text), language: language, header: map[string]string{}}
}

func (r *reader) skipSpace() {
	for r.i < r.n {
		c := r.s[r.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			r.i++
		case c == '%':
			if strings.HasPrefix(r.s[r.i:], "%{") {
				end := strings.Index(r.s[r.i+2:], "%}")
				if end < 0 {
					r.i = r.n
				} else {
					r.i += 2 + end + 2
				}
			} else {
				end := strings.IndexByte(r.s[r.i:], '\n')
				if end < 0 {
					r.i = r.n
				} else {
					r.i += end + 1
				}
			}
		default:
			return
		}
	}
}

func (r *reader) peek() byte {
	r.skipSpace()
	if r.i < r.n {
		return r.s[r.i]
	}
	return 0
}

func (r *reader) at(c byte) bool { return r.i < r.n && r.s[r.i] == c }

func (r *reader) startsWith(what string) bool { return strings.HasPrefix(r.s[r.i:], what) }

func (r *reader) take(what string) bool {
	r.skipSpace()
	if r.startsWith(what) {
		r.i += len(what)
		return true
	}
	return false
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

// word is a bare word: a command name, a pitch, a mode. Letters only, with
// the accented vowels of the romance note names.
func (r *reader) word() string {
	r.skipSpace()
	j := r.i
	for j < r.n {
		c := r.s[j]
		if isLetter(c) {
			j++
			continue
		}
		// é, ñ, ç and the like, as UTF-8.
		if c >= 0xC3 && j+1 < r.n && (r.s[j+1]&0xC0) == 0x80 {
			j += 2
			continue
		}
		break
	}
	w := r.s[r.i:j]
	r.i = j
	return w
}

// dashedWord is a word with dashes in it: one-section-one, center-column.
func (r *reader) dashedWord() string {
	w := r.word()
	for r.i+1 < r.n && r.s[r.i] == '-' && isLetter(r.s[r.i+1]) {
		r.i++
		w += "-" + r.word()
	}
	return w
}

func (r *reader) str() string {
	r.skipSpace()
	if !r.at('"') {
		return ""
	}
	var out strings.Builder
	r.i++
	for r.i < r.n && r.s[r.i] != '"' {
		if r.s[r.i] == '\\' && r.i+1 < r.n {
			r.i++
		}
		out.WriteByte(r.s[r.i])
		r.i++
	}
	r.i++
	return out.String()
}

// schemeValue steps over one #value: a number, 'symbol, "string" or (list),
// and gives back its text.
func (r *reader) schemeValue() string {
	start := r.i
	if r.at('#') {
		r.i++
	}
	r.skipSpace()
	if r.at('\'') || r.at('`') {
		r.i++
	}
	switch {
	case r.at('('):
		r.balanced("(", ")")
	case r.at('"'):
		r.str()
	default:
		j := r.i
		for j < r.n && !strings.ContainsRune(" \t\r\n{}<>", rune(r.s[j])) {
			j++
		}
		r.i = j
	}
	return r.s[start:r.i]
}

// schemeList is #'(a b c) or #'sym — the symbols a tag or a filter is given.
func (r *reader) schemeList() []string {
	r.skipSpace()
	if !r.at('#') {
		return nil
	}
	j := r.i + 1
	for j < r.n && (r.s[j] == ' ' || r.s[j] == '\t' || r.s[j] == '\n') {
		j++
	}
	if j < r.n && r.s[j] == '\'' {
		j++
	}
	if j < r.n && r.s[j] == '(' {
		end := strings.IndexByte(r.s[j:], ')')
		if end < 0 {
			return nil
		}
		inside := r.s[j+1 : j+end]
		r.i = j + end + 1
		return strings.Fields(inside)
	}
	k := j
	for k < r.n && (isLetter(r.s[k]) || isDigit(r.s[k]) || strings.ContainsRune("_!?-", rune(r.s[k]))) {
		k++
	}
	if k == j {
		return nil
	}
	r.i = k
	return []string{r.s[j:k]}
}

// balanced is the text of a bracketed run, brackets and all, for skipping
// or for reading again.
func (r *reader) balanced(opener, closer string) string {
	r.skipSpace()
	if !r.startsWith(opener) {
		return ""
	}
	depth, start, j := 0, r.i, r.i
	for j < r.n {
		switch {
		case strings.HasPrefix(r.s[j:], opener):
			depth++
			j += len(opener)
		case strings.HasPrefix(r.s[j:], closer):
			depth--
			j += len(closer)
			if depth == 0 {
				r.i = j
				return r.s[start:j]
			}
		case r.s[j] == '"':
			j++
			for j < r.n && r.s[j] != '"' {
				if r.s[j] == '\\' {
					j++
				}
				j++
			}
			j++
		case r.s[j] == '%':
			// A comment, which may hold an unmatched bracket.
			if strings.HasPrefix(r.s[j:], "%{") {
				end := strings.Index(r.s[j+2:], "%}")
				if end < 0 {
					j = r.n
				} else {
					j += 2 + end + 2
				}
			} else {
				end := strings.IndexByte(r.s[j:], '\n')
				if end < 0 {
					j = r.n
				} else {
					j += end + 1
				}
			}
		default:
			j++
		}
	}
	r.i = r.n
	return r.s[start:]
}

/*
 * The music
 */

// music is everything up to the end of the text, or to a closing bracket.
func (r *reader) music() *seqNode {
	out := &seqNode{}
	for {
		r.skipSpace()
		if r.i >= r.n || r.at('}') || r.startsWith(">>") {
			break
		}
		if item := r.one(); item != nil {
			out.items = append(out.items, item)
		}
	}
	return out
}

// one is one item of music: a note, a bracketed run, a command.
func (r *reader) one() node {
	r.skipSpace()
	if r.i >= r.n {
		return nil
	}
	c := r.s[r.i]

	if r.startsWith("<<") {
		r.i += 2
		out := &simNode{}
		for {
			r.skipSpace()
			if r.i >= r.n {
				break
			}
			if r.startsWith(">>") {
				r.i += 2
				break
			}
			if r.startsWith(`\\`) {
				// The voice separator: each run between them is a voice.
				r.i += 2
				continue
			}
			if item := r.one(); item != nil {
				out.items = append(out.items, item)
			}
		}
		return out
	}
	switch {
	case c == '{':
		r.i++
		inner := r.music()
		r.take("}")
		return inner
	case c == '<':
		return r.chord()
	case c == '\\':
		return r.command()
	case c == '|':
		r.i++
		return nil // a bar check: a claim about the bar, not music
	case c == '(' || c == ')' || c == '[' || c == ']' || c == '~':
		// A mark that belongs to the note before it, reached out of
		// order: the expander attaches it.
		r.i++
		return &commandNode{name: "after", args: []any{string(c)}}
	case c == '#':
		// A Scheme expression. At the top of a file these are settings
		// rather than notes, so they are stepped over whole.
		r.schemeValue()
		return nil
	case c == '$':
		// A music function's argument, spliced in by LilyPond: nothing
		// here can say what it was.
		r.i++
		r.word()
		return nil
	case isDigit(c):
		if r.drums {
			return r.drumAgain()
		}
		// "d8 8 8": a length on its own is the note before, again. A
		// number that is not a length — 3, or 120 — is stepped over, so
		// that the reading always moves on.
		start := r.i
		n := &lyNote{same: true}
		r.attach(n)
		if r.i == start || n.dur == nil {
			j := r.i
			for j < r.n && (isDigit(r.s[j]) || r.s[j] == '.' || r.s[j] == '/') {
				j++
			}
			if j == r.i {
				j++
			}
			r.unread = append(r.unread, r.s[start:j])
			r.i = j
			return nil
		}
		return n
	case isLetter(c):
		return r.noteOrWord()
	case c == '-' || c == '^' || c == '_':
		r.i++
		n := &lyNote{}
		r.attachScript(n, c)
		return &commandNode{name: "after", args: []any{n}}
	case c == '=':
		// An assignment whose name was read as a note or a word: the
		// value is stepped over.
		r.i++
		r.skipDefinition()
		return nil
	}
	// Anything else is not music this reader knows.
	end := r.i + 20
	if end > r.n {
		end = r.n
	}
	r.unread = append(r.unread, strings.TrimSpace(r.s[r.i:end]))
	r.i++
	return nil
}

// drumAgain is 8 or 16 on its own, inside a drum staff: the same drum again.
func (r *reader) drumAgain() node {
	n := &lyNote{drum: r.lastDrum}
	if r.lastDrum == "" {
		r.unread = append(r.unread, "a length with no drum before it")
		n.rest = true
	}
	r.attach(n)
	return n
}

// chord is <do mi sol>4 — several notes in one voice at one moment.
func (r *reader) chord() node {
	r.i++ // <
	var pitches []pitch
	var ties, forced, caut bool
	for r.i < r.n && !r.at('>') {
		r.skipSpace()
		if r.at('>') {
			break
		}
		if r.at('\\') || r.at('-') || r.at('^') || r.at('_') {
			// A mark on one note of the chord: taken as the chord's.
			inner := &lyNote{}
			r.attach(inner)
			if inner.tie {
				ties = true
			}
			if r.i < r.n && !isLetter(r.s[r.i]) && !r.at('>') && !r.at('\\') {
				r.i++
			}
			continue
		}
		w := r.word()
		if w == "" {
			r.i++
			continue
		}
		marks := ""
		for r.i < r.n && (r.s[r.i] == '\'' || r.s[r.i] == ',') {
			marks += string(r.s[r.i])
			r.i++
		}
		if r.drums {
			r.lastDrum = w
			pitches = append(pitches, pitch{})
		} else if p, ok := parsePitch(w+marks, r.language); ok {
			pitches = append(pitches, p)
		} else {
			r.unread = append(r.unread, w)
		}
		for r.i < r.n && (r.s[r.i] == '!' || r.s[r.i] == '?') {
			forced = forced || r.s[r.i] == '!'
			caut = caut || r.s[r.i] == '?'
			r.i++
		}
		// A note of the chord with a mark of its own: c~ or e\p.
		for r.i < r.n && (r.s[r.i] == '~' || r.s[r.i] == '\\' || r.s[r.i] == '-' || r.s[r.i] == '^' || r.s[r.i] == '_') {
			if r.s[r.i] == '~' {
				ties = true
				r.i++
				continue
			}
			inner := &lyNote{}
			save := r.i
			r.attach(inner)
			if r.i == save {
				r.i++
			}
			if inner.tie {
				ties = true
			}
		}
	}
	r.take(">")
	if len(pitches) > 0 {
		r.lastChord = append([]pitch(nil), pitches...)
	}
	n := &lyNote{pitches: pitches, forced: forced, cautionary: caut}
	if len(pitches) == 0 {
		// <>\pp — an empty chord, which exists to carry a mark.
		n.skip = true
		n.dur = &model.Duration{Num: 0, Den: 1}
	}
	if r.drums && len(pitches) > 0 {
		n.pitches = nil
		n.drum = r.lastDrum
	}
	r.attach(n)
	n.tie = n.tie || ties
	return n
}

var dashedAssignment = regexp.MustCompile(`^(?:-[A-Za-z]+)*\s*=`)

// noteOrWord is a note, a rest, a skip — or a word this reader does not know.
func (r *reader) noteOrWord() node {
	start := r.i
	w := r.word()
	marks := ""
	for r.i < r.n && (r.s[r.i] == '\'' || r.s[r.i] == ',') {
		marks += string(r.s[r.i])
		r.i++
	}

	// A definition rather than a note: `themeA = { ... }`. The definitions
	// are gathered separately, by name, so here the whole statement is
	// stepped over.
	if marks == "" {
		save := r.i
		if m := dashedAssignment.FindString(r.s[r.i:]); m != "" {
			r.i += len(m)
			r.skipDefinition()
			return nil
		}
		r.i = save
	}

	n := &lyNote{}
	switch {
	case w == "r":
		n.rest = true
	case w == "R":
		n.rest = true
		n.multi = true
	case w == "s":
		n.skip = true
	case w == "q":
		// The chord before this one, played again.
		if len(r.lastChord) > 0 {
			n.pitches = append([]pitch(nil), r.lastChord...)
		} else {
			r.unread = append(r.unread, "q with no chord before it")
			n.rest = true
		}
	case r.drums:
		n.drum = w
		r.lastDrum = w
	default:
		p, ok := parsePitch(w+marks, r.language)
		if !ok {
			r.i = start
			r.unread = append(r.unread, w)
			r.i += len(w)
			if len(w) == 0 {
				r.i++
			}
			return nil
		}
		n.pitch = &p
	}
	for r.i < r.n && (r.s[r.i] == '!' || r.s[r.i] == '?') {
		n.forced = n.forced || r.s[r.i] == '!'
		n.cautionary = n.cautionary || r.s[r.i] == '?'
		r.i++
	}
	r.attach(n)
	return n
}

var lengthRe = regexp.MustCompile(`^\s*(\d+|\\breve|\\longa|\\maxima)(\.*)((?:\*\d+(?:/\d+)?)*)`)

// attach reads everything written behind a note: its length and its marks.
func (r *reader) attach(n *lyNote) {
	if m := lengthRe.FindStringSubmatch(r.s[r.i:]); m != nil {
		if d, ok := durationOf(m[1] + m[2] + m[3]); ok {
			n.dur = &d
			r.i += len(m[0])
		}
	}
	for r.i < r.n {
		r.skipSpace()
		if r.i >= r.n {
			return
		}
		c := r.s[r.i]
		switch {
		case c == '~':
			n.tie = true
			r.i++
		case c == '(':
			n.slurStart = true
			r.i++
		case c == ')':
			n.slurStop = true
			r.i++
		case c == '[':
			n.beamStart = true
			r.i++
		case c == ']':
			n.beamStop = true
			r.i++
		case r.startsWith(`\(`):
			n.phraseStart = true
			r.i += 2
		case r.startsWith(`\)`):
			n.phraseStop = true
			r.i += 2
		case c == ':':
			// c4:32 — a tremolo, repeated at that length.
			r.i++
			j := r.i
			for j < r.n && isDigit(r.s[j]) {
				j++
			}
			if v, err := strconv.Atoi(r.s[r.i:j]); err == nil && v >= 8 {
				strokes := 0
				for x := v; x > 4; x /= 2 {
					strokes++
				}
				n.tremolo = strokes
			}
			r.i = j
		case c == '-' || c == '^' || c == '_':
			r.i++
			r.attachScript(n, c)
		case c == '\\':
			if !r.attachCommand(n) {
				return
			}
		default:
			return
		}
	}
}

var scriptNames = map[byte]string{
	'.': "staccato", '>': "accent", '^': "marcato", '_': "tenuto",
	'-': "tenuto", '!': "staccatissimo", '+': "stopped",
}

// attachScript reads -. -> -^ -_ -3 -"text" or -\markup, written on the
// note before, with the direction it was given.
func (r *reader) attachScript(n *lyNote, place byte) {
	r.skipSpace()
	if r.i >= r.n {
		return
	}
	c := r.s[r.i]
	switch {
	case isDigit(c):
		j := r.i
		for j < r.n && isDigit(r.s[j]) {
			j++
		}
		n.fingering = append(n.fingering, r.s[r.i:j])
		r.i = j
	case strings.IndexByte(".->^_!+", c) >= 0:
		n.artics = append(n.artics, scriptNames[c])
		r.i++
	case c == '"':
		n.texts = append(n.texts, textAt{text: r.str(), place: place})
	case c == '\\':
		save := r.i
		r.i++
		name := r.dashedWord()
		switch name {
		case "markup":
			n.texts = append(n.texts, textAt{text: r.markup(), place: place})
		case "tag":
			// -\tag #'x ^"1°" — a mark that belongs to some parts only.
			r.schemeList()
			r.skipSpace()
			if r.at('-') || r.at('^') || r.at('_') {
				p := r.s[r.i]
				r.i++
				r.attachScript(n, p)
			}
		case "tweak", "single", "once":
			// -\tweak Y-offset #3.6 ^4: a nudge to the mark after it. The
			// nudge is layout; the mark is kept. \single \override and
			// \once \override are the same shape.
			r.skipSpace()
			if name == "single" || name == "once" {
				if r.at('\\') {
					r.i++
					r.word()
					r.skipSpace()
				}
			}
			j := r.i
			for j < r.n && (isLetter(r.s[j]) || isDigit(r.s[j]) || r.s[j] == '.' || r.s[j] == '-' || r.s[j] == '_') {
				j++
			}
			r.i = j
			r.skipSpace()
			if r.at('=') {
				r.i++
				r.skipSpace()
			}
			switch {
			case r.at('#'):
				r.schemeValue()
			case r.at('"'):
				r.str()
			default:
				j := r.i
				if j < r.n && r.s[j] == '-' {
					j++
				}
				for j < r.n && (isDigit(r.s[j]) || r.s[j] == '.') {
					j++
				}
				r.i = j
			}
			r.skipSpace()
			if r.at('-') || r.at('^') || r.at('_') {
				p := r.s[r.i]
				r.i++
				r.attachScript(n, p)
			} else if name == "tweak" && r.at('\\') {
				// The mark the tweak was for: -\tweak … \markup "x".
				// A \once or a \single is complete in itself, and
				// whatever follows it is the next thing in the music.
				r.attachScript(n, place)
			}
		case "textDecresc", "textCresc", "textDim":
			words := r.str()
			if words == "" {
				words = "dim."
				if strings.Contains(name, "Cresc") {
					words = "cresc."
				}
			}
			n.texts = append(n.texts, textAt{text: words, place: place})
		case "textCrescEnd", "textDecrescEnd", "textDimEnd":
			if words := r.str(); words != "" {
				n.texts = append(n.texts, textAt{text: words, place: place})
			}
		case "":
			r.i = save
		default:
			// -\finger 3, -\pp, -\staccato: the same command, with a
			// direction.
			r.i = save
			if !r.attachCommand(n) {
				r.i = save + 1
				r.dashedWord()
				n.artics = append(n.artics, name)
			}
		}
	}
}

var dynamics = map[string]bool{
	"ppppp": true, "pppp": true, "ppp": true, "pp": true, "p": true,
	"mp": true, "mf": true, "f": true, "ff": true, "fff": true, "ffff": true,
	"fffff": true, "sf": true, "sfz": true, "fp": true, "sffz": true,
	"rfz": true, "spp": true, "sp": true, "sfp": true, "sff": true,
	"sfff": true, "fz": true, "rf": true, "n": true,
}

var plainArticulations = map[string]bool{
	"staccato": true, "accent": true, "tenuto": true, "marcato": true,
	"staccatissimo": true, "portato": true, "upbow": true, "downbow": true,
	"trill": true, "fermata": true, "shortfermata": true, "longfermata": true,
	"verylongfermata": true, "turn": true, "reverseturn": true,
	"mordent": true, "prall": true, "prallprall": true, "prallmordent": true,
	"upprall": true, "downprall": true, "espressivo": true, "flageolet": true,
	"open": true, "stopped": true, "thumb": true, "snappizzicato": true,
	"lheel": true, "rheel": true, "ltoe": true, "rtoe": true, "coda": true,
	"segno": true, "varcoda": true, "halfopen": true, "signumcongruentiae": true,
	"arpeggio": true, "noBeam": true,
}

// attachCommand reads a \command written on a note. False where it is not
// one of those.
func (r *reader) attachCommand(n *lyNote) bool {
	save := r.i
	r.i++ // the backslash
	if r.i < r.n && (r.s[r.i] == '<' || r.s[r.i] == '>' || r.s[r.i] == '!') {
		// The hairpins are punctuation rather than words.
		switch r.s[r.i] {
		case '!':
			n.hairpinStop = true
		case '<':
			n.hairpin = "cresc"
		case '>':
			n.hairpin = "dim"
		}
		r.i++
		return true
	}
	name := r.dashedWord()
	switch {
	case dynamics[name]:
		n.dynamic = name
	case name == "cresc" || name == "crescTextCresc":
		n.hairpin, n.hairpinText = "cresc", true
	case name == "crescHairpin":
		n.hairpin = "cresc"
	case name == "dim" || name == "decr" || name == "decresc" || name == "dimTextDim" ||
		name == "dimTextDecr" || name == "dimTextDecresc":
		n.hairpin, n.hairpinText = "dim", true
	case name == "endcr" || name == "enddecr" || name == "endcresc" || name == "enddim":
		n.hairpinStop = true
	case name == "finger":
		r.skipSpace()
		if r.startsWith(`\markup`) {
			r.i += len(`\markup`)
			words := r.markup()
			digits := regexp.MustCompile(`\d+`).FindAllString(words, -1)
			if len(digits) > 0 {
				n.fingering = append(n.fingering, strings.Join(digits, "-"))
			}
		} else {
			if r.at('"') {
				n.fingering = append(n.fingering, r.str())
			} else {
				j := r.i
				for j < r.n && isDigit(r.s[j]) {
					j++
				}
				if j > r.i {
					n.fingering = append(n.fingering, r.s[r.i:j])
					r.i = j
				}
			}
		}
	case name == "rest":
		// b32\rest: a rest drawn where that note would be. The pitch
		// stays, since in relative mode it is the next note's reference.
		n.rest = true
	case name == "startTrillSpan":
		n.trillStart = true
	case name == "stopTrillSpan":
		n.trillStop = true
	case name == "sustainOn":
		n.pedal = "start"
	case name == "sustainOff":
		n.pedal = "stop"
	case name == "breathe":
		n.breath = true
	case name == "glissando":
		n.gliss = true
	case name == "laissezVibrer" || name == "repeatTie":
		// A tie with one end: drawn in LilyPond, said here as nothing a
		// reader would miss.
	case name == "fermataMarkup":
		n.artics = append(n.artics, "fermata")
	case name == "markup":
		n.texts = append(n.texts, textAt{text: r.markup(), place: '-'})
	case plainArticulations[name]:
		n.artics = append(n.artics, name)
	case name == "tweak" || name == "single" || name == "once":
		// c4\tweak … ^"x": the same as -\tweak, read from the backslash.
		r.i = save
		r.attachScript(n, '-')
	default:
		r.i = save
		return false
	}
	return true
}

// markup reads the words of a markup, without the way they are set.
func (r *reader) markup() string {
	r.skipSpace()
	text := ""
	if r.at('{') {
		text = r.balanced("{", "}")
	} else {
		// A single argument: \markup\bold "word" or \markup\x Word. A
		// command's Scheme arguments are stepped over, and the strings
		// among them kept: \tied-finger-up #"4" #"5" is the figures 4
		// and 5.
		var found []string
		for r.i < r.n {
			r.skipSpace()
			if r.at('\\') {
				r.i++
				name := r.dashedWord()
				r.skipSpace()
				if r.at('\\') || r.i >= r.n || r.at('|') {
					// \markup \twoOpens followed by music: the name is
					// a markup of its own, whose words are looked up
					// later, and the music is not its argument.
					if len(found) == 0 && text == "" {
						return "\\" + name
					}
					break
				}
				continue
			}
			if r.at('#') {
				if r.startsWith(`#"`) {
					r.i++
					found = append(found, r.str())
				} else {
					r.schemeValue()
				}
				continue
			}
			if len(found) > 0 {
				var q []string
				for _, f := range found {
					q = append(q, `"`+f+`"`)
				}
				text = strings.Join(q, " ")
				break
			}
			if r.at('"') {
				text = r.str()
				break
			}
			if r.at('{') {
				text = r.balanced("{", "}")
				break
			}
			j := r.i
			for j < r.n && !strings.ContainsRune(" \t\r\n{}<>\\\"", rune(r.s[j])) {
				j++
			}
			text = r.s[r.i:j]
			r.i = j
			break
		}
	}
	return markupWords(text)
}

var (
	markupCommandRe = regexp.MustCompile(`\\[A-Za-z-]+|#[^\s{}"]+`)
	markupWordRe    = regexp.MustCompile(`"([^"]*)"|([^\s{}"\\#]+)`)
)

// markupWords is the words of a markup's text: the commands that set them —
// \italic, \center-column — and their Scheme arguments are not words; the
// strings and the bare words are.
func markupWords(text string) string {
	text = markupCommandRe.ReplaceAllString(text, " ")
	var words []string
	for _, m := range markupWordRe.FindAllStringSubmatch(text, -1) {
		w := m[1]
		if w == "" {
			w = m[2]
		}
		if w = strings.TrimSpace(w); w != "" {
			words = append(words, w)
		}
	}
	return strings.Join(words, " ")
}

/*
 * Commands
 */

// silentCommands say how the page looks rather than what is played.
var silentCommands = map[string]bool{
	"break": true, "pageBreak": true, "noPageBreak": true, "allowPageTurn": true,
	"noBreak": true, "haraKiri": true, "pageTurn": true, "slurUp": true,
	"slurDown": true, "slurNeutral": true, "tieUp": true, "tieDown": true,
	"tieNeutral": true, "dynamicUp": true, "dynamicDown": true,
	"dynamicNeutral": true, "textLengthOn": true, "textLengthOff": true,
	"autoBeamOn": true, "autoBeamOff": true, "cadenzaOn": true,
	"cadenzaOff": true, "smallCaps": true, "tupletUp": true,
	"tupletDown": true, "tupletNeutral": true, "explicitTuplets": true,
	"implicitTuplets": true, "trillSpanUp": true, "trillSpanDown": true,
	"arpeggioArrowUp": true, "arpeggioArrowDown": true, "arpeggioNormal": true,
	"arpeggioBracket": true, "mergeDifferentlyDottedOn": true,
	"mergeDifferentlyDottedOff": true, "mergeDifferentlyHeadedOn": true,
	"mergeDifferentlyHeadedOff": true, "hideNotes": true, "unHideNotes": true,
	"autoPageBreaksOff": true, "autoPageBreaksOn": true, "stemUp": true,
	"stemDown": true, "stemNeutral": true, "phrasingSlurUp": true,
	"phrasingSlurDown": true, "phrasingSlurNeutral": true, "shiftOn": true,
	"shiftOff": true, "shiftOnn": true, "shiftOnnn": true, "accidentalStyle": true,
	"defaultTimeSignature": true, "compressMMRests": true, "compressEmptyMeasures": true,
	"expandEmptyMeasures": true, "crossStaff": true, "bracketCrossStaff": true,
	"textSpannerUp": true, "textSpannerDown": true, "startTextSpan": true,
	"stopTextSpan": true, "easyHeadsOn": true, "easyHeadsOff": true,
	"improvisationOn": true, "improvisationOff": true, "pointAndClickOff": true,
	"pointAndClickOn": true, "sacredHarpHeads": true, "aikenHeads": true,
	"pedalUp": true, "pedalDown": true, "hide": true, "omit": true,
	"newSpacingSection": true, "alterBroken": true, "shape": true,
	"magnifyMusic": true, "magnifyStaff": true, "balloonGrobText": true,
	"balloonText": true, "stopStaff": true, "startStaff": true,
	"featherDurations": true, "scaleDurations": true, "unfoldRepeats": true,
	"hideStaffSwitch": true, "showStaffSwitch": true, "RemoveEmptyStaves": true,
	"RemoveAllEmptyStaves": true, "noBeam": true,
}

var measureLengthRe = regexp.MustCompile(`^\s*Timing\.measureLength\s*=\s*#\(ly:make-moment\s+(\d+)\s*/\s*(\d+)\s*\)`)
var measureLengthRatRe = regexp.MustCompile(`^\s*Timing\.measureLength\s*=\s*#?(\d+)\s*/\s*(\d+)`)

func (r *reader) command() node {
	r.i++ // the backslash
	if r.i < r.n {
		c := r.s[r.i]
		if c == '(' || c == ')' || c == '<' || c == '>' || c == '!' {
			r.i++
			return &commandNode{name: "after", args: []any{"\\" + string(c)}}
		}
		if c == '\\' {
			r.i++
			return nil // the voice separator, outside << >>
		}
		if c == '[' || c == ']' {
			r.i++
			return nil // a manual beam of the old kind
		}
	}
	name := r.dashedWord()
	if name == "" {
		r.i++
		return nil
	}

	switch name {
	case "version", "bookOutputName", "bookOutputSuffix":
		r.str()
		return nil
	case "include":
		return &includeNode{path: r.str()}
	case "language":
		if lang := r.str(); lang != "" {
			r.language = lang
		}
		return &commandNode{name: "language", args: []any{r.language}}
	case "tag", "keepWithTag", "removeWithTag", "pushToTag", "appendToTag":
		names := r.schemeList()
		music := r.one()
		return &tagNode{names: names, music: music, keep: name == "keepWithTag", drop: name == "removeWithTag"}
	case "transpose":
		from := r.pitchArg()
		to := r.pitchArg()
		return &transposeNode{from: from, to: to, music: r.one()}
	case "relative":
		return &relativeNode{pitch: r.pitchArg(), music: r.one()}
	case "absolute", "fixed", "notemode", "notes":
		if name == "fixed" {
			r.pitchArg()
		}
		return r.one()
	case "shape", "alterBroken":
		r.skipSpace()
		if r.at('#') {
			r.schemeValue()
		}
		r.skipSpace()
		if r.at('#') {
			r.schemeValue()
		}
		j := r.i
		for j < r.n && (isLetter(r.s[j]) || r.s[j] == '.') {
			j++
		}
		r.i = j
		return nil
	case "afterGrace":
		// \afterGrace d8. { c32 d32 }: the grace notes come after the
		// note, and are drawn before whatever follows it.
		r.skipSpace()
		if r.at('#') {
			r.schemeValue() // the fraction of the note they take
		}
		main := r.one()
		return &seqNode{items: []node{main, &graceNode{kind: "after", music: r.one()}}}
	case "ottava":
		r.skipSpace()
		if r.at('#') {
			r.i++
		}
		j := r.i
		if j < r.n && r.s[j] == '-' {
			j++
		}
		for j < r.n && isDigit(r.s[j]) {
			j++
		}
		v, _ := strconv.Atoi(r.s[r.i:j])
		r.i = j
		return &commandNode{name: "ottava", args: []any{v}}
	case "omit", "hide", "undo", "offset", "overrideProperty",
		"shortestNoteDuration", "magnifyStaff", "magnifyMusic":
		switch name {
		case "overrideProperty":
			r.restOfStatement()
		case "undo":
			r.skipSpace()
			undone := ""
			if r.at('\\') {
				r.i++
				undone = r.word()
			}
			r.dashedWord()
			if undone == "offset" {
				r.skipSpace()
				if r.at('#') {
					r.schemeValue()
				}
				r.dashedWord()
			}
		case "offset":
			r.skipSpace()
			j := r.i
			for j < r.n && (isLetter(r.s[j]) || r.s[j] == '.' || r.s[j] == '-') {
				j++
			}
			r.i = j
			r.skipSpace()
			if r.at('#') {
				r.schemeValue()
			}
			r.skipSpace()
			j = r.i
			for j < r.n && (isLetter(r.s[j]) || r.s[j] == '.' || r.s[j] == '-') {
				j++
			}
			r.i = j
		case "shortestNoteDuration":
			r.number()
		case "magnifyStaff", "magnifyMusic":
			r.skipSpace()
			if r.at('#') {
				r.schemeValue()
			}
			if name == "magnifyMusic" {
				return r.one()
			}
		default:
			r.dashedWord()
		}
		return nil
	case "flatTrill", "sharpTrill", "naturalTrill", "regularTrill":
		// A score's own names for the sign over a trill, defined as
		// overrides of the trill line's text.
		acc := strings.TrimSuffix(name, "Trill")
		if name == "regularTrill" {
			acc = ""
		}
		return &commandNode{name: "trillacc", args: []any{acc}}
	case "filterMusic":
		return &filterNode{kinds: r.schemeList(), music: r.one()}
	case "repeat":
		kind := r.word()
		if kind == "" {
			kind = r.str()
		}
		count := r.number()
		music := r.one()
		save := r.i
		r.skipSpace()
		if r.startsWith(`\alternative`) {
			r.i += len(`\alternative`)
			alts := r.one()
			var items []node
			if sq, ok := alts.(*seqNode); ok {
				items = sq.items
			} else {
				items = []node{alts}
			}
			if kind == "volta" {
				return &repeatNode{kind: kind, count: count, music: music, alternatives: items}
			}
			return &seqNode{items: []node{&repeatNode{kind: kind, count: count, music: music}, alts}}
		}
		r.i = save
		return &repeatNode{kind: kind, count: count, music: music}
	case "tuplet":
		num, den := r.fraction()
		// \tuplet 3/2 4 { } — an optional length for the bracket.
		r.skipSpace()
		j := r.i
		for j < r.n && (isDigit(r.s[j]) || r.s[j] == '.') {
			j++
		}
		if j > r.i && j < r.n && (r.s[j] == ' ' || r.s[j] == '{' || r.s[j] == '\\') {
			r.i = j
		}
		return &tupletNode{num: num, den: den, music: r.one()}
	case "times":
		num, den := r.fraction()
		return &tupletNode{num: den, den: num, music: r.one()}
	case "grace", "acciaccatura", "appoggiatura", "slashedGrace":
		return &graceNode{kind: name, music: r.one()}
	case "clef":
		return &commandNode{name: "clef", args: []any{r.strOrWord()}}
	case "time":
		r.skipSpace()
		if r.at('#') {
			r.schemeValue() // a beat structure, before the fraction
		}
		num, den := r.fraction()
		return &commandNode{name: "time", args: []any{num, den}}
	case "numericTimeSignature":
		return &commandNode{name: "numericTime"}
	case "key":
		tonic := r.pitchArg()
		mode := ""
		r.skipSpace()
		if r.at('\\') {
			r.i++
			mode = r.word()
		}
		return &commandNode{name: "key", args: []any{tonic, mode}}
	case "tempo":
		return r.tempo()
	case "midiTempo":
		r.skipSpace()
		if r.at('#') {
			r.i++
		}
		bpm := r.number()
		return &commandNode{name: "tempo", args: []any{"", model.Duration{Num: 1, Den: 4}, bpm}}
	case "drummode", "drums":
		was := r.drums
		r.drums = true
		music := r.one()
		r.drums = was
		return &commandNode{name: "drums", args: []any{music}}
	case "breathe":
		return &commandNode{name: "breathe"}
	case "transposition":
		return &commandNode{name: "transposition", args: []any{r.pitchArg()}}
	case "bar":
		return &commandNode{name: "bar", args: []any{r.str()}}
	case "mark":
		r.skipSpace()
		if r.startsWith(`\default`) {
			r.i += len(`\default`)
			return &commandNode{name: "mark", args: []any{"default"}}
		}
		return &commandNode{name: "mark", args: []any{r.restOfArgument()}}
	case "partial":
		return &commandNode{name: "partial", args: []any{r.durationArg()}}
	case "skip":
		d := r.durationArg()
		n := &lyNote{skip: true}
		if d != nil {
			n.dur = d
		}
		r.attach(n)
		return n
	case "new", "context":
		return r.context()
	case "change":
		// \change Staff = "left": the voice goes on on another staff.
		r.word()
		r.skipSpace()
		if r.at('=') {
			r.i++
		}
		return &commandNode{name: "change", args: []any{r.strOrWord()}}
	case "score", "book", "bookpart":
		// What a score holds is music under the staves it names; the
		// layout, paper and header blocks inside it are read as met.
		return &commandNode{name: name, args: []any{r.one()}}
	case "header":
		r.readHeader()
		return nil
	case "with", "layout", "midi", "paper":
		r.balanced("{", "}")
		return nil
	case "addlyrics", "lyricmode", "lyrics":
		return r.lyricsBlock("")
	case "lyricsto":
		to := r.strOrWord()
		return r.lyricsBlock(to)
	case "chordmode", "chords", "figuremode", "figures":
		r.unread = append(r.unread, `\`+name)
		r.one()
		return nil
	case "set":
		save := r.i
		if m := measureLengthRe.FindStringSubmatch(r.s[r.i:]); m != nil {
			r.i += len(m[0])
			num, _ := strconv.Atoi(m[1])
			den, _ := strconv.Atoi(m[2])
			return &commandNode{name: "measureLength", args: []any{model.Duration{Num: num, Den: den}}}
		}
		if m := measureLengthRatRe.FindStringSubmatch(r.s[r.i:]); m != nil {
			r.i += len(m[0])
			num, _ := strconv.Atoi(m[1])
			den, _ := strconv.Atoi(m[2])
			return &commandNode{name: "measureLength", args: []any{model.Duration{Num: num, Den: den}}}
		}
		r.i = save
		return r.setting()
	case "override", "revert", "unset", "once", "tweak", "single", "temporary":
		if name == "once" || name == "single" || name == "temporary" {
			// \once \override …: the override is what follows.
			return nil
		}
		r.restOfStatement()
		return nil
	case "voiceOne", "voiceTwo", "voiceThree", "voiceFour", "oneVoice":
		return &commandNode{name: "voice", args: []any{name}}
	case "markup", "markuplist":
		return &commandNode{name: "markup", args: []any{r.markup()}}
	case "parallelMusic":
		r.schemeList()
		r.one()
		r.unread = append(r.unread, `\parallelMusic`)
		return nil
	case "partcombine", "partCombine":
		a := r.one()
		b := r.one()
		return &simNode{items: []node{a, b}}
	case "unfoldRepeats":
		return &commandNode{name: "unfold", args: []any{r.one()}}
	}
	if silentCommands[name] {
		return nil
	}
	if strings.HasPrefix(name, "sempre") && len(name) > 6 {
		// \semprePp, \sempreLegato: "sempre pp", "sempre legato".
		return &commandNode{name: "words", args: []any{"sempre " + strings.ToLower(name[6:])}}
	}
	// Anything else with a backslash is a variable: \themeA, \global.
	return &refNode{name: name}
}

// setting reads \set Context.property = value, keeping the ones that say
// something about the music.
func (r *reader) setting() node {
	r.skipSpace()
	j := r.i
	for j < r.n && (isLetter(r.s[j]) || r.s[j] == '.' || isDigit(r.s[j])) {
		j++
	}
	prop := r.s[r.i:j]
	r.i = j
	r.skipSpace()
	if !r.at('=') {
		r.restOfStatement()
		return nil
	}
	r.i++
	r.skipSpace()
	var value string
	switch {
	case r.at('"'):
		value = r.str()
	case r.at('#'):
		value = r.schemeValue()
	case r.startsWith(`\markup`):
		r.i += len(`\markup`)
		value = r.markup()
	default:
		value = r.restOfArgument()
	}
	short := prop
	if k := strings.LastIndexByte(prop, '.'); k >= 0 {
		short = prop[k+1:]
	}
	switch short {
	case "instrumentName", "shortInstrumentName", "midiInstrument":
		if strings.HasPrefix(value, "#") {
			value = markupWords(value)
		}
		return &commandNode{name: short, args: []any{strings.Trim(value, `"`)}}
	case "markFormatter":
		return &commandNode{name: "markFormatter", args: []any{value}}
	case "tupletSpannerDuration":
	}
	return nil
}

// tempo reads \tempo "Allegro" 4 = 120, in any of its shapes.
func (r *reader) tempo() node {
	text := ""
	r.skipSpace()
	switch {
	case r.at('"'):
		text = r.str()
	case r.startsWith(`\markup`):
		r.i += len(`\markup`)
		text = r.markup()
	}
	r.skipSpace()
	m := regexp.MustCompile(`^(\d+\.*)\s*=\s*(\d+)(?:\s*[-~]\s*(\d+))?`).FindStringSubmatch(r.s[r.i:])
	if m == nil {
		return &commandNode{name: "tempo", args: []any{text, nil, 0}}
	}
	r.i += len(m[0])
	unit, _ := durationOf(m[1])
	bpm, _ := strconv.Atoi(m[2])
	return &commandNode{name: "tempo", args: []any{text, unit, bpm}}
}

// context reads \new Kind = "name" \with { ... } music.
func (r *reader) context() node {
	kind := r.word()
	r.skipSpace()
	name := ""
	if r.at('=') {
		r.i++
		name = r.strOrWord()
	}
	r.skipSpace()
	var with withBlock
	if r.startsWith(`\with`) {
		r.i += len(`\with`)
		with = parseWith(r.balanced("{", "}"))
	}
	var music node
	r.skipSpace()
	if r.startsWith(`\lyricsto`) || r.startsWith(`\lyricmode`) || r.startsWith(`\addlyrics`) {
		music = r.command()
	} else {
		music = r.one()
	}
	return &contextNode{kind: kind, name: name, with: with, music: music}
}

var (
	withNameRe  = regexp.MustCompile(`(instrumentName|shortInstrumentName|midiInstrument)\s*=\s*`)
	withLinesRe = regexp.MustCompile(`line-count\s*=\s*#?(\d+)`)
)

// parseWith reads what a \with block says about a staff: its name, its
// sound, how it is ruled.
func parseWith(text string) withBlock {
	w := withBlock{text: text}
	for _, m := range withNameRe.FindAllStringSubmatchIndex(text, -1) {
		sub := newReader(text[m[1]:], "nederlands")
		value := sub.restOfArgument()
		if text[m[2]:m[3]] == "instrumentName" {
			value = firstLineOf(text[m[1]:], value)
		}
		switch text[m[2]:m[3]] {
		case "instrumentName":
			if w.instrumentName == "" {
				w.instrumentName = value
			}
		case "shortInstrumentName":
			w.shortName = value
		case "midiInstrument":
			w.midiInstrument = value
		}
	}
	if m := withLinesRe.FindStringSubmatch(text); m != nil {
		w.lines, _ = strconv.Atoi(m[1])
	}
	return w
}

// firstLineOf is the name a staff is headed with: a markup that goes on
// to list the players in smaller type — Flûtes, then "2 Grandes Flûtes" —
// is cut at the first \column or \fontsize, which is where the list begins.
func firstLineOf(raw, words string) string {
	cut := len(raw)
	for _, marker := range []string{`\column`, `\fontsize`, `\smaller`, `\tiny`, `\small`, `\line`} {
		if i := strings.Index(raw, marker); i >= 0 && i < cut && i > 0 {
			cut = i
		}
	}
	if cut == len(raw) {
		return words
	}
	head := markupWords(raw[:cut])
	if head == "" {
		return words
	}
	return head
}

// readHeader reads \header { title = "…" composer = "…" }.
func (r *reader) readHeader() {
	text := r.balanced("{", "}")
	if len(text) < 2 {
		return
	}
	sub := newReader(text[1:len(text)-1], r.language)
	for {
		sub.skipSpace()
		if sub.i >= sub.n {
			break
		}
		key := sub.dashedWord()
		if key == "" {
			sub.i++
			continue
		}
		sub.skipSpace()
		if !sub.at('=') {
			continue
		}
		sub.i++
		sub.skipSpace()
		value := ""
		switch {
		case sub.at('"'):
			value = sub.str()
		case sub.startsWith(`\markup`):
			sub.i += len(`\markup`)
			value = sub.markup()
		case sub.at('#'):
			v := sub.schemeValue()
			if strings.HasPrefix(v, `#"`) {
				value = strings.Trim(v[1:], `"`)
			}
		case sub.at('\\'):
			sub.i++
			sub.dashedWord()
			continue
		default:
			value = sub.word()
		}
		if _, have := r.header[key]; !have && value != "" {
			r.header[key] = value
		}
	}
}

// lyricsBlock reads the words of \addlyrics { }, \lyricmode { } or
// \lyricsto "voice" { }.
func (r *reader) lyricsBlock(to string) node {
	r.skipSpace()
	out := &lyricsNode{to: to}
	// \lyricsto "v" \new Lyrics { } or \lyricsto "v" \lyricmode { }: the
	// words are inside whatever follows.
	for r.at('\\') {
		save := r.i
		r.i++
		name := r.word()
		switch name {
		case "lyricmode", "lyrics":
		case "new", "context":
			r.word()
			r.skipSpace()
			if r.at('=') {
				r.i++
				r.strOrWord()
			}
			r.skipSpace()
			if r.startsWith(`\with`) {
				r.i += len(`\with`)
				r.balanced("{", "}")
			}
		default:
			r.i = save
			// \addlyrics \someVariable: the words are elsewhere.
			return &lyricsNode{to: to, syllables: nil}
		}
		r.skipSpace()
	}
	if !r.at('{') {
		return out
	}
	r.i++
	r.lyrics = true
	defer func() { r.lyrics = false }()
	for {
		r.skipSpace()
		if r.i >= r.n || r.at('}') {
			r.i++
			break
		}
		c := r.s[r.i]
		switch {
		case r.startsWith("--"):
			r.i += 2
			if k := len(out.syllables); k > 0 {
				out.syllables[k-1].hyphen = true
			}
		case r.startsWith("__"):
			r.i += 2
			if k := len(out.syllables); k > 0 {
				out.syllables[k-1].extend = true
			}
		case c == '_' && (r.i+1 >= r.n || !isLetter(r.s[r.i+1])):
			r.i++
			sy := syllable{skip: true}
			r.lyricDuration(&sy)
			out.syllables = append(out.syllables, sy)
		case c == '|':
			r.i++
		case c == '\\':
			save := r.i
			r.i++
			name := r.dashedWord()
			switch name {
			case "markup":
				out.syllables = append(out.syllables, syllable{text: r.markup()})
			case "skip":
				r.durationArg()
				out.syllables = append(out.syllables, syllable{skip: true})
			case "set", "override", "once", "revert", "unset", "tweak":
				r.restOfStatement()
			case "":
				r.i = save + 1
			default:
				// \lyricmode \bold, or a variable holding more words.
			}
		case c == '"':
			sy := syllable{text: r.str()}
			r.lyricDuration(&sy)
			out.syllables = append(out.syllables, sy)
		case c == '{':
			r.i++
		case c == '<' || c == '>':
			r.i++
		default:
			j := r.i
			for j < r.n && !strings.ContainsRune(" \t\r\n{}|", rune(r.s[j])) {
				if r.s[j] == '-' && strings.HasPrefix(r.s[j:], "--") {
					break
				}
				if r.s[j] == '_' && strings.HasPrefix(r.s[j:], "__") {
					break
				}
				if isDigit(r.s[j]) && j > r.i {
					break
				}
				j++
			}
			if j == r.i {
				r.i++
				continue
			}
			sy := syllable{text: r.s[r.i:j]}
			r.i = j
			r.lyricDuration(&sy)
			out.syllables = append(out.syllables, sy)
		}
	}
	return out
}

func (r *reader) lyricDuration(sy *syllable) {
	if m := lengthRe.FindStringSubmatch(r.s[r.i:]); m != nil && strings.TrimSpace(m[0]) == m[0] {
		if d, ok := durationOf(m[1] + m[2] + m[3]); ok {
			sy.dur = &d
			r.i += len(m[0])
		}
	}
}

func (r *reader) strOrWord() string {
	r.skipSpace()
	if r.at('"') {
		return r.str()
	}
	j := r.i
	for j < r.n && (isLetter(r.s[j]) || isDigit(r.s[j]) || r.s[j] == '_' || r.s[j] == '^') {
		j++
	}
	w := r.s[r.i:j]
	r.i = j
	return w
}

func (r *reader) pitchArg() *pitch {
	r.skipSpace()
	w := r.word()
	marks := ""
	for r.i < r.n && (r.s[r.i] == '\'' || r.s[r.i] == ',') {
		marks += string(r.s[r.i])
		r.i++
	}
	p, ok := parsePitch(w+marks, r.language)
	if !ok {
		return nil
	}
	return &p
}

func (r *reader) durationArg() *model.Duration {
	r.skipSpace()
	m := regexp.MustCompile(`^(\d+)(\.*)((?:\*\d+(?:/\d+)?)*)`).FindStringSubmatch(r.s[r.i:])
	if m == nil {
		return nil
	}
	r.i += len(m[0])
	d, ok := durationOf(m[0])
	if !ok {
		return nil
	}
	return &d
}

func (r *reader) number() int {
	r.skipSpace()
	j := r.i
	for j < r.n && isDigit(r.s[j]) {
		j++
	}
	v, _ := strconv.Atoi(r.s[r.i:j])
	r.i = j
	return v
}

func (r *reader) fraction() (int, int) {
	r.skipSpace()
	m := regexp.MustCompile(`^(\d+)\s*/\s*(\d+)`).FindStringSubmatch(r.s[r.i:])
	if m == nil {
		return 4, 4
	}
	r.i += len(m[0])
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return a, b
}

func (r *reader) restOfArgument() string {
	r.skipSpace()
	switch {
	case r.at('"'):
		return r.str()
	case r.at('#'):
		v := r.schemeValue()
		return strings.Trim(strings.TrimPrefix(v, "#"), `"'`)
	case r.startsWith(`\markup`):
		r.i += len(`\markup`)
		return r.markup()
	case r.at('\\'):
		// \shortinstr Fl., or a name: the words of whatever it is.
		r.i++
		r.dashedWord()
		return r.restOfArgument()
	case r.at('{'):
		return markupWords(r.balanced("{", "}"))
	}
	return r.word()
}

// skipDefinition steps over what a name is defined as, whatever shape it has.
func (r *reader) skipDefinition() {
	r.skipSpace()
	if r.i >= r.n {
		return
	}
	c := r.s[r.i]
	switch {
	case c == '#':
		r.i++
		r.skipSpace()
		if r.at('(') {
			r.balanced("(", ")")
			return
		}
		r.schemeValue()
	case r.startsWith("<<"):
		r.balanced("<<", ">>")
	case c == '{':
		r.balanced("{", "}")
	case c == '"':
		r.str()
	case c == '\\':
		r.i++
		name := r.dashedWord()
		switch name {
		case "override", "revert", "set", "unset", "once", "tweak",
			"overrideProperty", "omit", "magnifyStaff":
			r.restOfStatement()
		case "markup", "markuplist", "with", "layout", "score", "header", "paper", "midi":
			r.skipSpace()
			if r.at('{') {
				r.balanced("{", "}")
			} else {
				end := strings.IndexByte(r.s[r.i:], '\n')
				if end < 0 {
					r.i = r.n
				} else {
					r.i += end
				}
			}
		case "relative", "transpose", "fixed":
			r.pitchArg()
			if name == "transpose" {
				r.pitchArg()
			}
			r.skipDefinition()
		case "new", "context":
			r.word()
			r.skipSpace()
			if r.at('=') {
				r.i++
				r.strOrWord()
			}
			r.skipSpace()
			if r.startsWith(`\with`) {
				r.i += len(`\with`)
				r.balanced("{", "}")
			}
			r.skipDefinition()
		}
	default:
		end := strings.IndexByte(r.s[r.i:], '\n')
		if end < 0 {
			r.i = r.n
		} else {
			r.i += end
		}
	}
}

// restOfStatement skips a setting: up to the = and whatever it is set to.
func (r *reader) restOfStatement() {
	depth := 0
	for r.i < r.n {
		c := r.s[r.i]
		switch {
		case c == '{' || c == '<':
			depth++
		case c == '}' || c == '>':
			if depth == 0 {
				return
			}
			depth--
		case c == '"':
			r.str()
			continue
		case c == '#' && depth == 0:
			r.schemeValue()
			return
		case c == '\n' && depth == 0:
			return
		}
		r.i++
	}
}

// read reads a stretch of LilyPond, and says what could not be read.
func read(text, language string) (*seqNode, []string, map[string]string) {
	r := newReader(text, language)
	music := r.music()
	return music, r.unread, r.header
}
