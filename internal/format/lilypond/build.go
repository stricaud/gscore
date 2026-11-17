/*
 * build.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

/*
 * From a tree of music to a score.
 *
 * The staves are found first: every \new Staff under a \score, with the
 * groups that brace them and the words that go under them. Each staff's
 * music is expanded into a stream, the streams are laid into one grid of
 * bars, and the bars are written into the model one voice at a time, with
 * everything that was hung on a note put where the model keeps it.
 */

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// staffSpec is one staff as the source arranged it.
type staffSpec struct {
	kind   string
	name   string
	with   withBlock
	music  node
	here   string
	lyrics []*lyricsNode
}

type groupSpec struct {
	kind        string
	first, last int
	name        string
}

// defaultBPM is the speed of a score that names none.
const defaultBPM = 60

// movement is one \score: its staves and what the file around it said.
type movement struct {
	name   string
	staves []*staffSpec
	groups []groupSpec
	header map[string]string
	here   string
	lyrics []*lyricsNode // words beside the staves, to be matched by voice
}

var staffKinds = map[string]bool{
	"Staff": true, "DrumStaff": true, "RhythmicStaff": true, "TabStaff": true,
	"GregorianTranscriptionStaff": true, "MensuralStaff": true, "VaticanaStaff": true,
}

var groupKinds = map[string]bool{
	"PianoStaff": true, "GrandStaff": true, "StaffGroup": true, "ChoirStaff": true,
}

// stavesIn finds every staff in a tree of music.
func (ctx *context) stavesIn(music node, here string, mv *movement) {
	switch m := music.(type) {
	case nil:
	case *includeNode:
		if path := ctx.resolve(here, m.path); path != "" {
			if inner, ok := ctx.load(path); ok {
				ctx.stavesIn(inner, filepath.Dir(path), mv)
			}
		}
	case *seqNode:
		for _, item := range m.items {
			ctx.stavesIn(item, here, mv)
		}
	case *simNode:
		for _, item := range m.items {
			ctx.stavesIn(item, here, mv)
		}
	case *refNode:
		if got, ok := ctx.variables[m.name]; ok {
			where := ctx.where[m.name]
			if where == "" {
				where = here
			}
			ctx.stavesIn(got, where, mv)
		}
	case *contextNode:
		switch {
		case staffKinds[m.kind]:
			name := m.name
			if name == "" {
				name = fmt.Sprintf("Staff %d", len(mv.staves)+1)
			}
			mv.staves = append(mv.staves, &staffSpec{kind: m.kind, name: name, with: m.with, music: m.music, here: here})
		case groupKinds[m.kind] || strings.HasPrefix(m.kind, "StaffGroup") || strings.HasPrefix(m.kind, "PianoStaff"):
			first := len(mv.staves)
			ctx.stavesIn(m.music, here, mv)
			if len(mv.staves) > first {
				mv.groups = append(mv.groups, groupSpec{kind: m.kind, first: first,
					last: len(mv.staves) - 1, name: m.with.instrumentName})
			}
		case m.kind == "Lyrics":
			if ly, ok := m.music.(*lyricsNode); ok {
				mv.lyrics = append(mv.lyrics, ly)
			} else {
				ctx.stavesIn(m.music, here, mv)
			}
		default:
			ctx.stavesIn(m.music, here, mv)
		}
	case *lyricsNode:
		// \addlyrics after a staff's music: the words of that staff.
		if n := len(mv.staves); n > 0 {
			mv.staves[n-1].lyrics = append(mv.staves[n-1].lyrics, m)
		} else {
			mv.lyrics = append(mv.lyrics, m)
		}
	case *relativeNode:
		ctx.stavesIn(m.music, here, mv)
	case *transposeNode:
		ctx.stavesIn(m.music, here, mv)
	case *tagNode:
		ctx.stavesIn(m.music, here, mv)
	case *repeatNode:
		ctx.stavesIn(m.music, here, mv)
	case *commandNode:
		switch m.name {
		case "score", "book", "bookpart", "drums", "unfold":
			if len(m.args) > 0 {
				if inner, ok := m.args[0].(node); ok {
					ctx.stavesIn(inner, here, mv)
				}
			}
		}
	}
}

// movementsOf finds the score's movements: each \score block holding
// staves is one, and a file whose music stands outside any \score is one on
// its own.
func (ctx *context) movementsOf(top node, here string) []*movement {
	var out []*movement
	var loose *movement
	var walk func(music node, here string, inScore bool)
	walk = func(music node, here string, inScore bool) {
		switch m := music.(type) {
		case nil:
		case *includeNode:
			path := ctx.resolve(here, m.path)
			if path == "" {
				return
			}
			inner, ok := ctx.load(path)
			if !ok {
				return
			}
			before := len(out)
			walk(inner, filepath.Dir(path), inScore)
			for _, mv := range out[before:] {
				if mv.name == "" {
					mv.name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				}
				if len(mv.header) == 0 {
					mv.header = ctx.headers[path]
				}
			}
		case *seqNode:
			for _, item := range m.items {
				walk(item, here, inScore)
			}
		case *commandNode:
			switch m.name {
			case "score":
				if inScore {
					return
				}
				mv := &movement{here: here, header: map[string]string{}}
				if len(m.args) > 0 {
					if inner, ok := m.args[0].(node); ok {
						ctx.stavesIn(inner, here, mv)
						if len(mv.staves) == 0 && hasNotes(inner) {
							// A score with no staff of its own: the
							// music is one staff.
							mv.staves = append(mv.staves, &staffSpec{kind: "Staff", name: "", music: inner, here: here})
						}
					}
				}
				if len(mv.staves) > 0 {
					out = append(out, mv)
				}
			case "book", "bookpart":
				if len(m.args) > 0 {
					if inner, ok := m.args[0].(node); ok {
						walk(inner, here, inScore)
					}
				}
			default:
				if loose == nil {
					loose = &movement{here: here, header: map[string]string{}}
				}
				ctx.stavesIn(m, here, loose)
			}
		default:
			if loose == nil {
				loose = &movement{here: here, header: map[string]string{}}
			}
			ctx.stavesIn(m, here, loose)
		}
	}
	walk(top, here, false)
	if len(out) == 0 && loose != nil {
		if len(loose.staves) == 0 && hasNotes(top) {
			loose.staves = append(loose.staves, &staffSpec{kind: "Staff", music: top, here: here})
		}
		if len(loose.staves) > 0 {
			out = append(out, loose)
		}
	}
	return out
}

// hasNotes says whether there is a note anywhere in a tree, as against
// settings alone.
func hasNotes(music node) bool {
	switch m := music.(type) {
	case *lyNote:
		return !m.skip
	case *seqNode:
		for _, item := range m.items {
			if hasNotes(item) {
				return true
			}
		}
	case *simNode:
		for _, item := range m.items {
			if hasNotes(item) {
				return true
			}
		}
	case *refNode:
		return true // whatever it names is probably music
	case *relativeNode:
		return hasNotes(m.music)
	case *transposeNode:
		return hasNotes(m.music)
	case *tagNode:
		return hasNotes(m.music)
	case *repeatNode:
		return hasNotes(m.music)
	case *tupletNode:
		return hasNotes(m.music)
	case *graceNode:
		return hasNotes(m.music)
	case *contextNode:
		return hasNotes(m.music)
	case *commandNode:
		if m.name == "drums" && len(m.args) > 0 {
			if inner, ok := m.args[0].(node); ok {
				return hasNotes(inner)
			}
		}
	}
	return false
}

/*
 * Building the model
 */

// voiceState is what carries from one bar to the next within a voice.
type voiceState struct {
	tie     bool // the note before was tied on
	beam    bool // a beam is open
	tuplet  *tupletRef
	last    model.Element // the element before, for a tuplet's end
	graces  []*model.Note
	slashed bool
	gliss   bool
	trill   bool
	pending *lyNote // the marks of an empty chord, for the next note
}

type builder struct {
	ctx    *context
	mv     *movement
	bars   []bar
	marks  int
	numers bool
	tempo  *model.Tempo
}

// build makes a score of a movement.
func (ctx *context) build(mv *movement) *model.Score {
	b := &builder{ctx: ctx, mv: mv}
	// A score that gives no speed plays at a crotchet a second: slow
	// enough to hear, and nothing on the page says otherwise.
	s := &model.Score{Tempo: model.Tempo{BPM: defaultBPM, Unit: model.Duration{Num: 1, Den: 4}}}
	b.identity(s)

	streams := make([][]any, len(mv.staves))
	for i, st := range mv.staves {
		streams[i] = ctx.flatten(st.music, st.here)
	}
	b.bars = grid(streams)
	for _, stream := range streams {
		for _, mk := range marksIn(stream) {
			if mk.kind == "numericTime" {
				b.numers = true
			}
		}
	}
	names := make([]string, len(mv.staves))
	index := map[string]int{}
	for i, st := range mv.staves {
		names[i] = st.name
		index[st.name] = i
	}
	// A voice that \change Staff sent to another staff is that staff's
	// music for as long as it stays: it goes in as a voice of its own
	// there, with its own stems and beams, which is how LilyPond draws
	// it and how the Allegro of Op. 111 reads for bars at a time.
	own := make([][]*event, len(mv.staves))
	for i, stream := range streams {
		for _, e := range eventsIn(stream) {
			if e.staff == "" || e.staff == names[i] {
				own[i] = append(own[i], e)
				continue
			}
			j, ok := index[e.staff]
			if !ok {
				b.ctx.lg.say(`\change Staff = "%s" names a staff that is not there`, e.staff)
				own[i] = append(own[i], e)
				continue
			}
			moved := *e
			moved.voice += 10 * (i + 1)
			own[j] = append(own[j], &moved)
		}
	}
	for i, st := range mv.staves {
		measures := layOutEvents(own[i], marksIn(streams[i]), b.bars)
		p := b.part(i, st, streams[i], measures, names)
		s.Parts = append(s.Parts, p)
	}
	if b.tempo != nil {
		s.Tempo = *b.tempo
	}
	// Repeats and endings are the score's, written on every staff: the
	// first staff's are taken, which is where a piano's are written.
	for _, g := range mv.groups {
		sym := model.GroupBracket
		lines := true
		switch {
		case strings.HasPrefix(g.kind, "PianoStaff") || g.kind == "GrandStaff":
			sym = model.GroupBrace
		case g.kind == "ChoirStaff":
			lines = false
		case strings.Contains(g.kind, "NoBar"):
			lines = false
		}
		if strings.Contains(g.kind, "NoBracket") {
			sym = model.GroupLine
		}
		s.Groups = append(s.Groups, model.StaffGroup{First: g.first, Last: g.last, Symbol: sym, Barlines: lines, Name: g.name})
	}
	b.words(s, streams)
	return s
}

func (b *builder) identity(s *model.Score) {
	// The main file's header, then the movement's own: a subtitle in the
	// second movement's file is not the first movement's.
	h := map[string]string{}
	for k, v := range b.ctx.headers[b.ctx.mainPath] {
		h[k] = v
	}
	for k, v := range b.mv.header {
		h[k] = v
	}
	for _, k := range []string{"title", "composer", "arranger", "poet", "lyricist", "copyright"} {
		if h[k] == "" {
			h[k] = b.ctx.header[k]
		}
	}
	s.Identity.Title = h["title"]
	s.Identity.Subtitle = h["subtitle"]
	if s.Identity.Subtitle == "" {
		s.Identity.Subtitle = h["piece"]
	}
	if s.Identity.Subtitle == "" {
		s.Identity.Subtitle = h["opus"]
	} else if h["opus"] != "" && !strings.Contains(s.Identity.Subtitle, h["opus"]) {
		s.Identity.Subtitle += " — " + h["opus"]
	}
	if s.Identity.Title == "" {
		s.Identity.Title = b.mv.name
	}
	s.Identity.Composer = h["composer"]
	s.Identity.Arranger = h["arranger"]
	s.Identity.Lyricist = h["poet"]
	if s.Identity.Lyricist == "" {
		s.Identity.Lyricist = h["lyricist"]
	}
	s.Identity.Copyright = h["copyright"]
}

var clefNames = map[string]model.Clef{
	"treble": model.Treble, "violin": model.Treble, "g": model.Treble, "g2": model.Treble,
	"french": model.Treble, "bass": model.Bass, "f": model.Bass, "f4": model.Bass,
	"alto": model.Alto, "c": model.Alto, "c3": model.Alto, "viola": model.Alto,
	"tenor": model.Tenor, "c4": model.Tenor, "percussion": model.Percussion,
	"drum": model.Percussion, "treble_8": model.Treble8vb, "g_8": model.Treble8vb,
	"tab": model.Treble, "subbass": model.Bass, "varbaritone": model.Bass,
	"baritone": model.Bass, "mezzosoprano": model.Alto, "soprano": model.Treble,
}

func (b *builder) clefOf(name string) model.Clef {
	key := strings.ToLower(strings.Trim(name, `"`))
	if c, ok := clefNames[key]; ok {
		return c
	}
	b.ctx.lg.say("a clef not drawn here, read as a treble clef: %s", name)
	return model.Treble
}

var stepFifths = [7]int{0, 2, 4, -1, 1, 3, 5}

func keyOf(tonic *pitch, mode string) model.KeySignature {
	if tonic == nil {
		return model.KeySignature{}
	}
	n := stepFifths[tonic.step] + 7*tonic.alter
	m := model.Major
	switch mode {
	case "minor":
		n -= 3
		m = model.Minor
	case "dorian":
		n -= 2
		m = model.Dorian
	case "mixolydian":
		n--
		m = model.Mixolydian
	case "lydian":
		n++
		m = model.Lydian
	case "phrygian":
		n -= 4
		m = model.Phrygian
	case "locrian":
		n -= 5
		m = model.Locrian
	case "ionian", "major", "":
	}
	if n > 7 {
		n = 7
	}
	if n < -7 {
		n = -7
	}
	return model.KeySignature{Fifths: n, Mode: m}
}

func (b *builder) timeOf(t [2]int) model.TimeSignature {
	ts := model.TimeSignature{Beats: t[0], BeatValue: t[1]}
	if !b.numers {
		switch {
		case t[0] == 4 && t[1] == 4:
			ts.Symbol = model.TimeCommon
		case t[0] == 2 && t[1] == 2:
			ts.Symbol = model.TimeCut
		}
	}
	return ts
}

// part builds one staff.
func (b *builder) part(index int, st *staffSpec, stream []any, measures []*measure, names []string) *model.Part {
	p := &model.Part{ID: fmt.Sprintf("P%d", index+1), Clef: model.Treble,
		Key: model.KeySignature{}, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	if len(b.bars) > 0 && b.bars[0].time != nil {
		p.Time = b.timeOf(*b.bars[0].time)
	}
	name := st.with.instrumentName
	drums := st.kind == "DrumStaff"
	midi := st.with.midiInstrument
	for _, m := range measures {
		for _, a := range m.attrs {
			switch a.kind {
			case "instrumentName":
				if name == "" {
					name, _ = a.args[0].(string)
				}
			case "midiInstrument":
				if midi == "" {
					midi, _ = a.args[0].(string)
				}
			}
		}
	}
	for _, e := range eventsIn(stream) {
		if e.note.drum != "" {
			drums = true
			break
		}
	}
	if name == "" && st.name != "" && !strings.HasPrefix(st.name, "Staff ") {
		name = st.name
	}
	p.Name = name
	if drums {
		p.Clef = model.Percussion
		p.Sound = "drum.group.set"
	} else {
		if inst, ok := model.InstrumentNamed(midi); ok && midi != "" {
			p.Instrument = inst
		}
		if sound, ok := model.GuessSound(name); ok {
			p.Sound = sound
			if midi == "" {
				if inst, ok := model.SoundProgram(sound); ok {
					p.Instrument = inst
				}
			}
		}
	}
	if st.with.lines > 0 && st.with.lines < 5 {
		p.StaffLines = st.with.lines
	}
	if st.kind == "RhythmicStaff" {
		p.StaffLines = 1
	}

	states := map[int]*voiceState{}
	stateOf := func(v int) *voiceState {
		if _, ok := states[v]; !ok {
			states[v] = &voiceState{}
		}
		return states[v]
	}
	// Which line of the staff each voice number is, the same in every
	// bar: a slur that opens in the second voice of one bar and closes in
	// the next, where that voice is the only one with notes, has to find
	// its other end on the same line.
	numbers := []int{}
	seen := map[int]bool{}
	for _, m := range measures {
		for _, pl := range m.events {
			if !seen[pl.e.voice] {
				seen[pl.e.voice] = true
				numbers = append(numbers, pl.e.voice)
			}
		}
	}
	sort.Ints(numbers)
	if len(numbers) == 0 {
		numbers = []int{1}
	}
	clef, key := p.Clef, p.Key
	first := true
	for mi, m := range measures {
		out := model.NewMeasure()
		out.Voices = nil
		b.measureMarks(p, out, mi, m, &clef, &key, first)
		first = false

		var voices []*model.Voice
		for _, vn := range numbers {
			voice := &model.Voice{}
			vs := stateOf(vn)
			for _, pl := range m.events {
				if pl.e.voice != vn {
					continue
				}
				if el := b.element(index, pl, vs, names); el != nil {
					voice.Elements = append(voice.Elements, el)
				}
			}
			voices = append(voices, voice)
		}
		// The lines that are empty at the end of the bar are left off;
		// one that is empty between two that are not stays, so that the
		// numbering holds.
		last := 0
		for i, v := range voices {
			if len(v.Elements) > 0 {
				last = i
			}
		}
		out.Voices = voices[:last+1]
		// Marks written partway through the bar, onto the note they
		// precede in the first voice.
		b.midBarMarks(out, m, &clef, &key)
		p.Measures = append(p.Measures, out)
	}
	// A tuplet still open at the end is closed on its last note.
	for _, vs := range states {
		if vs.tuplet != nil && vs.last != nil {
			if mk := marksOfElement(vs.last); mk != nil && mk.Tuplet != nil {
				mk.Tuplet.Stop = true
			}
		}
	}
	b.lyrics(p, st, stream)
	return p
}

func marksOfElement(el model.Element) *model.Marks {
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

// measureMarks writes what stands at the head of a bar: its meter, a clef
// or key written there, the barlines, a tempo, a rehearsal mark.
func (b *builder) measureMarks(p *model.Part, out *model.Measure, mi int, m *measure, clef *model.Clef, key *model.KeySignature, first bool) {
	bar := b.bars[mi]
	if m.time != nil && !first {
		t := b.timeOf(*m.time)
		out.Time = &t
	}
	for _, a := range m.attrs {
		atStart := a.at.Cmp(bar.at) == 0
		switch a.kind {
		case "clef":
			if !atStart {
				continue
			}
			c := b.clefOf(a.args[0].(string))
			if first {
				p.Clef = c
			} else if c != *clef {
				cc := c
				out.Clef = &cc
			}
			*clef = c
		case "key":
			if !atStart {
				continue
			}
			tonic, _ := a.args[0].(*pitch)
			mode, _ := a.args[1].(string)
			k := keyOf(tonic, mode)
			if first {
				p.Key = k
			} else if k != *key {
				kk := k
				out.Key = &kk
			}
			*key = k
		case "tempo":
			text, _ := a.args[0].(string)
			bpm, _ := a.args[2].(int)
			t := model.Tempo{Text: text, BPM: bpm, Unit: model.Duration{Num: 1, Den: 4}}
			if unit, ok := a.args[1].(model.Duration); ok && unit.Num > 0 {
				t.Unit = unit
			}
			if bpm <= 0 {
				// Words alone: the speed stays what it was, the words
				// are printed and no number is.
				t.BPM = defaultBPM
				if b.tempo != nil {
					t.BPM, t.Unit = b.tempo.BPM, b.tempo.Unit
				}
				t.Silent = true
			}
			if mi == 0 && atStart {
				if b.tempo == nil {
					tt := t
					b.tempo = &tt
				}
			} else if out.Tempo == nil {
				tt := t
				out.Tempo = &tt
			}
		case "mark":
			label, _ := a.args[0].(string)
			b.marks++
			if label == "default" || label == "" {
				if b.ctx.markNums {
					label = strconv.Itoa(b.marks)
				} else {
					label = markLetter(b.marks)
				}
			} else if n, err := strconv.Atoi(label); err == nil {
				b.marks = n
			}
			out.Marker = label
		case "transposition":
			if tp, ok := a.args[0].(*pitch); ok && tp != nil {
				diatonic := tp.step + 7*(tp.octave-4)
				chromatic := tp.semitones() - 60
				p.Transpose = model.Transposition{Diatonic: diatonic, Chromatic: chromatic}
			}
		}
	}
	out.RightBarline = barlineOf(m.barline)
	if m.repeatStart && mi > 0 {
		prev := p.Measures[mi-1]
		switch prev.RightBarline {
		case model.BarRepeatEnd:
			prev.RightBarline = model.BarRepeatBoth
		default:
			prev.RightBarline = model.BarRepeatStart
		}
	}
	for _, en := range m.endings {
		if out.Ending == nil {
			out.Ending = &model.Ending{}
		}
		if len(out.Ending.Numbers) == 0 || out.Ending.Numbers[0] != en[0] {
			out.Ending.Numbers = []int{en[0]}
		}
		if en[1] == 1 {
			out.Ending.Start = true
		} else {
			out.Ending.Stop = true
			if en[0] == en[2] && out.RightBarline != model.BarRepeatEnd {
				out.Ending.Open = true
			}
		}
	}
}

// markLetter is the A, B, C … of a rehearsal mark, as LilyPond numbers them
// by default: I is left out, as the engravers do.
func markLetter(n int) string {
	letters := "ABCDEFGHJKLMNOPQRSTUVWXYZ"
	if n <= 0 {
		n = 1
	}
	n--
	out := string(letters[n%len(letters)])
	for n >= len(letters) {
		n = n/len(letters) - 1
		out = string(letters[n%len(letters)]) + out
	}
	return out
}

func barlineOf(style string) model.BarlineStyle {
	switch style {
	case "|.", "|]", ".|.":
		return model.BarFinal
	case "||":
		return model.BarDouble
	case ":|", ":|.", ":|]":
		return model.BarRepeatEnd
	case ".|:", "[|:":
		return model.BarRepeatStart
	case ":|.|:", ":|.:", ":|][|:", ":..:", ":.|.:":
		return model.BarRepeatBoth
	case "", "|":
		return model.BarSingle
	case ".|", ".":
		return model.BarHeavyLight
	case "!", ";", "'":
		return model.BarSingle
	}
	return model.BarSingle
}

// midBarMarks puts a clef or a key written between the notes onto the note
// that follows it.
func (b *builder) midBarMarks(out *model.Measure, m *measure, clef *model.Clef, key *model.KeySignature) {
	bar := b.bars[0]
	_ = bar
	if len(out.Voices) == 0 {
		return
	}
	for _, a := range m.attrs {
		if a.kind != "clef" && a.kind != "key" {
			continue
		}
		var at model.Duration
		for i := range b.bars {
			if b.bars[i].at.Cmp(a.at) <= 0 && a.at.Cmp(b.bars[i].at.Add(b.bars[i].length)) < 0 {
				at = a.at.Sub(b.bars[i].at)
				break
			}
		}
		if at.IsZero() {
			continue
		}
		// The first element of the first voice at or after the place.
		var target *model.Marks
		pos := model.Duration{Num: 0, Den: 1}
		for _, el := range out.Voices[0].Elements {
			if pos.Cmp(at) >= 0 {
				target = marksOfElement(el)
				break
			}
			pos = pos.Add(el.Duration())
		}
		if target == nil {
			if n := len(out.Voices[0].Elements); n > 0 {
				target = marksOfElement(out.Voices[0].Elements[n-1])
			}
		}
		if target == nil {
			continue
		}
		switch a.kind {
		case "clef":
			c := b.clefOf(a.args[0].(string))
			if c != *clef {
				cc := c
				target.Clef = &cc
			}
			*clef = c
		case "key":
			tonic, _ := a.args[0].(*pitch)
			mode, _ := a.args[1].(string)
			k := keyOf(tonic, mode)
			if k != *key {
				kk := k
				target.Key = &kk
			}
			*key = k
		}
	}
}

var drumKeys = map[string]int{
	"bd": 36, "bda": 35, "sn": 38, "sna": 40, "ss": 37, "sidestick": 37,
	"hh": 42, "hhc": 42, "hho": 46, "hhp": 44, "hhho": 46, "hhhc": 42,
	"cymc": 49, "cymca": 57, "cymr": 51, "cymra": 59, "cymch": 52, "cyms": 55,
	"cymcb": 53, "rb": 53, "tomh": 50, "tommh": 48, "tomml": 47, "toml": 45,
	"tomfh": 43, "tomfl": 41, "cb": 56, "tamb": 54, "tri": 81, "trio": 81,
	"trim": 80, "cl": 75, "hc": 39, "mar": 70, "cab": 69, "gui": 73, "guis": 73,
	"guil": 74, "wbh": 76, "wbl": 77, "cgh": 63, "cgho": 63, "cghm": 62, "cgl": 64,
	"cglo": 64, "cglm": 64, "boh": 60, "bol": 61, "timh": 65, "timl": 66,
	"agh": 67, "agl": 68, "vibs": 58, "cuim": 78, "cuio": 79, "whs": 71, "whl": 72,
	"tamtam": 52, "bass": 36, "snare": 38, "hihat": 42, "crash": 49, "ride": 51,
	"toma": 50, "tomb": 47, "tomc": 45, "tomd": 43, "tome": 41,
}

// drumOf is the model's note for a kit piece: where it sits on the staff and
// what its head is.
func (b *builder) drumOf(name string, dur model.Duration) *model.Note {
	key, ok := drumKeys[name]
	if !ok {
		b.ctx.lg.say("a drum not known here, written as a snare: %s", name)
		key = 38
	}
	n := &model.Note{Dur: dur}
	for _, d := range model.DrumSet {
		if d.Key == key {
			n.Percussion = key
			n.Head = d.Head
			n.Pitch = model.PitchFromDiatonicIndex(d.Step - 4)
			return n
		}
	}
	n.Percussion = key
	n.Pitch = model.PitchFromDiatonicIndex(1)
	return n
}

func pitchOf(p pitch) model.Pitch {
	return model.Pitch{Step: model.Step(p.step), Octave: p.octave, Alter: p.alter}
}

func accidentalOf(alter int) model.Accidental {
	switch alter {
	case 1:
		return model.Sharp
	case 2:
		return model.DoubleSharp
	case -1:
		return model.Flat
	case -2:
		return model.DoubleFlat
	}
	return model.Natural
}

// element makes one element of a voice from a placed event, and writes
// everything hung on the note onto it.
func (b *builder) element(staff int, pl *placed, vs *voiceState, names []string) model.Element {
	e := pl.e
	n := e.note
	dur := e.dur

	// <>\pp: an empty chord, which exists to carry a mark to the note
	// after it.
	if n.skip && dur.IsZero() && !e.grace {
		if vs.pending == nil {
			vs.pending = &lyNote{}
		}
		mergeMarks(vs.pending, n)
		return nil
	}
	if vs.pending != nil && !e.grace {
		nn := *n
		mergeMarks(&nn, vs.pending)
		n = &nn
		vs.pending = nil
	}

	// A grace note waits for the note it leans on.
	if e.grace {
		gd := model.Duration{Num: 1, Den: 16}
		if n.dur != nil && n.dur.Num > 0 {
			gd = *n.dur
		}
		var gn *model.Note
		switch {
		case n.drum != "":
			gn = b.drumOf(n.drum, gd)
		case n.pitch != nil:
			gn = &model.Note{Pitch: pitchOf(*n.pitch), Dur: gd}
		case len(n.pitches) > 0:
			gn = &model.Note{Pitch: pitchOf(n.pitches[0]), Dur: gd}
		default:
			return nil
		}
		if n.forced || n.cautionary {
			gn.Accidental = accidentalOf(gn.Pitch.Alter)
			gn.Forced, gn.Parens = n.forced, n.cautionary
		}
		gn.SlurStart = model.SlurEnd(n.slurStart || n.phraseStart)
		for _, t := range n.tag {
			if t == "grace:acciaccatura" || t == "grace:slashedGrace" {
				vs.slashed = true
			}
		}
		vs.graces = append(vs.graces, gn)
		// What else was written on the grace note belongs to the note it
		// leans on: \afterGrace e2\startTrillSpan { d16\stopTrillSpan e }
		// ends the trill where the next note begins, and a dynamic on a
		// grace note is the dynamic of the note after it.
		carried := *n
		carried.slurStart, carried.slurStop = false, false
		carried.phraseStart, carried.phraseStop = false, false
		carried.beamStart, carried.beamStop, carried.tie = false, false, false
		carried.artics, carried.fingering = nil, nil
		if vs.pending == nil {
			vs.pending = &lyNote{}
		}
		mergeMarks(vs.pending, &carried)
		return nil
	}

	var el model.Element
	var mk *model.Marks
	switch {
	case n.skip:
		r := &model.Rest{Dur: dur, Hidden: true}
		el, mk = r, &r.Marks
	case n.rest:
		r := &model.Rest{Dur: dur}
		el, mk = r, &r.Marks
	case n.drum != "":
		note := b.drumOf(n.drum, dur)
		el, mk = note, &note.Marks
	case n.pitch != nil:
		note := &model.Note{Pitch: pitchOf(*n.pitch), Dur: dur}
		if n.forced || n.cautionary {
			note.Accidental = accidentalOf(note.Pitch.Alter)
			note.Forced, note.Parens = n.forced, n.cautionary
		}
		el, mk = note, &note.Marks
	case len(n.pitches) > 0:
		c := &model.Chord{Dur: dur}
		for _, p := range n.pitches {
			note := &model.Note{Pitch: pitchOf(p), Dur: dur}
			if n.forced || n.cautionary {
				note.Accidental = accidentalOf(note.Pitch.Alter)
				note.Forced, note.Parens = n.forced, n.cautionary
			}
			c.Notes = append(c.Notes, note)
		}
		el, mk = c, &c.Marks
	default:
		return nil
	}
	sounds := !n.skip && !n.rest

	// The grace notes waiting for this note.
	if len(vs.graces) > 0 && !pl.tiedBack {
		mk.Grace = &model.GraceGroup{Notes: vs.graces, Slashed: vs.slashed}
		vs.graces, vs.slashed = nil, false
	}

	// Ties, from the note before and to the note after.
	if vs.tie && sounds {
		mk.TieStop = true
	}
	vs.tie = false
	if (n.tie || pl.tiedOn) && sounds {
		mk.TieStart = true
		vs.tie = true
	}
	if vs.gliss && sounds {
		mk.Glissando = &model.Glissando{Stop: true}
		vs.gliss = false
	}

	// What was written on the note. A piece tied back from the bar before
	// carries none of the marks that begin something, and one tied on to
	// the next none that end something.
	begins, ends := !pl.tiedBack, !pl.tiedOn
	if begins {
		mk.SlurStart = model.SlurEnd(n.slurStart) + model.SlurEnd(n.phraseStart)
		for _, a := range n.artics {
			b.articulation(mk, a)
		}
		if n.dynamic != "" {
			b.dynamic(mk, n.dynamic)
		}
		if n.hairpin != "" {
			kind := model.Crescendo
			if n.hairpin == "dim" {
				kind = model.Diminuendo
			}
			if n.hairpinText {
				word := "cresc."
				if n.hairpin == "dim" {
					word = "dim."
				}
				mk.Texts = append(mk.Texts, model.TextMark{Text: word, Placement: model.PlacementBelow, Italic: true})
			} else {
				mk.Hairpin = &model.Hairpin{Kind: kind, Start: true}
			}
		}
		for _, t := range n.texts {
			if strings.HasPrefix(t.text, "\\") {
				// A markup named by a variable: its words, if it has any.
				words, ok := b.ctx.markups[t.text[1:]]
				if !ok {
					b.ctx.lg.say("a markup nothing defines: %s", t.text)
					continue
				}
				t.text = words
			}
			tm := model.TextMark{Text: t.text}
			switch t.place {
			case '^':
				tm.Placement = model.PlacementAbove
			case '_':
				tm.Placement = model.PlacementBelow
			}
			mk.Texts = append(mk.Texts, tm)
		}
		mk.Fingering = append(mk.Fingering, n.fingering...)
		if n.trillStart {
			if mk.Trill == nil {
				mk.Trill = &model.Trill{}
			}
			mk.Trill.Start = true
			switch n.trillAcc {
			case "flat":
				mk.Trill.Accidental = model.Flat
			case "sharp":
				mk.Trill.Accidental = model.Sharp
			case "natural":
				mk.Trill.Accidental = model.Natural
			}
			vs.trill = true
		}
		if n.pedal == "start" {
			mk.Pedal = &model.PedalMark{Start: true}
		}
		if n.ottava != nil {
			if *n.ottava == 0 {
				mk.Octave = &model.OctaveShift{Stop: true}
			} else {
				mk.Octave = &model.OctaveShift{Amount: *n.ottava, Start: true}
			}
		}
		if n.breath {
			mk.Break = model.BreathComma
		}
		if n.arpeggio {
			mk.Arpeggio = &model.Arpeggio{}
		}
		if n.tremolo > 0 {
			mk.Tremolo = &model.Tremolo{Strokes: n.tremolo}
		}
	}
	if ends {
		mk.SlurStop = model.SlurEnd(n.slurStop) + model.SlurEnd(n.phraseStop)
		if n.hairpinStop {
			if mk.Hairpin == nil {
				mk.Hairpin = &model.Hairpin{Stop: true}
			} else {
				mk.Hairpin.Stop = true
			}
		}
		if n.trillStop {
			if mk.Trill == nil {
				mk.Trill = &model.Trill{}
			}
			mk.Trill.Stop = true
			vs.trill = false
		}
		if n.pedal == "stop" {
			if mk.Pedal == nil {
				mk.Pedal = &model.PedalMark{}
			}
			mk.Pedal.Stop = true
		}
		if n.gliss && sounds {
			mk.Glissando = &model.Glissando{Start: true}
			vs.gliss = true
		}
	}

	// Beams, as written: [ opens one and ] closes it.
	if sounds && dur.Cmp(model.Duration{Num: 1, Den: 4}) < 0 {
		switch {
		case n.beamStart && n.beamStop:
		case n.beamStart && begins:
			mk.Beam = model.BeamStart
			vs.beam = true
		case n.beamStop && ends:
			if vs.beam {
				mk.Beam = model.BeamStop
			}
			vs.beam = false
		case vs.beam:
			mk.Beam = model.BeamContinue
		}
	}

	// Tuplets: every note in one says the ratio, the first and the last
	// say where it begins and ends.
	t := e.tuplet
	if t != vs.tuplet {
		if vs.tuplet != nil && vs.last != nil {
			if lm := marksOfElement(vs.last); lm != nil && lm.Tuplet != nil {
				lm.Tuplet.Stop = true
			}
		}
		if t != nil {
			mk.Tuplet = &model.Tuplet{Actual: t.num, Normal: t.den, Start: true}
			if t.outer != nil {
				b.ctx.lg.say("a tuplet inside a tuplet: the inner one's bracket is drawn")
			}
		}
		vs.tuplet = t
	} else if t != nil {
		mk.Tuplet = &model.Tuplet{Actual: t.num, Normal: t.den}
	}
	vs.last = el

	return el
}

var articulationsOf = map[string]model.Articulation{
	"staccato": model.Staccato, "staccatissimo": model.Staccatissimo,
	"accent": model.Accent, "tenuto": model.Tenuto, "marcato": model.Marcato,
	"fermata": model.Fermata, "shortfermata": model.Fermata, "longfermata": model.Fermata,
	"verylongfermata": model.Fermata, "upbow": model.UpBow, "downbow": model.DownBow,
	"turn": model.Turn, "reverseturn": model.TurnInverted, "mordent": model.Mordent,
	"prall": model.ShortTrill, "prallprall": model.ShortTrill, "upprall": model.ShortTrill,
	"downprall": model.ShortTrill, "prallmordent": model.ShortTrill, "lineprall": model.ShortTrill,
}

func (b *builder) articulation(mk *model.Marks, name string) {
	switch name {
	case "trill":
		if mk.Trill == nil {
			mk.Trill = &model.Trill{}
		}
		mk.Trill.Start, mk.Trill.Stop = true, true
	case "portato":
		mk.AddArticulation(model.Tenuto)
		mk.AddArticulation(model.Staccato)
	case "arpeggio":
		mk.Arpeggio = &model.Arpeggio{}
	case "noBeam":
	case "espressivo":
		mk.Texts = append(mk.Texts, model.TextMark{Text: "espr.", Italic: true})
	case "coda", "segno", "varcoda":
		mk.Texts = append(mk.Texts, model.TextMark{Text: name})
	default:
		if a, ok := articulationsOf[name]; ok {
			mk.AddArticulation(a)
			return
		}
		if words, ok := b.ctx.markups[name]; ok {
			mk.Texts = append(mk.Texts, model.TextMark{Text: words})
			return
		}
		if _, ok := b.ctx.dynTexts[name]; ok {
			b.dynamic(mk, name)
			return
		}
		if b.ctx.eventFns[name] || name == "once" {
			return
		}
		b.ctx.lg.say("an articulation not drawn here, left out: %s", name)
	}
}

var dynamicsOf = map[string]model.Dynamic{
	"ppppp": model.PPPP, "pppp": model.PPPP, "ppp": model.PPP, "pp": model.PP,
	"p": model.Piano, "mp": model.MP, "mf": model.MF, "f": model.Forte,
	"ff": model.FF, "fff": model.FFF, "ffff": model.FFFF, "fffff": model.FFFF,
	"sf": model.SForzato, "sfz": model.SForzando, "fp": model.FortePiano,
	"sffz": model.SForzando, "rfz": model.Rinforzando, "rf": model.Rinforzando,
	"spp": model.PP, "sp": model.Piano, "sfp": model.SForzando, "sff": model.SForzando,
	"sfff": model.SForzando, "fz": model.SForzato, "n": model.Niente,
}

func (b *builder) dynamic(mk *model.Marks, name string) {
	d, ok := dynamicsOf[name]
	if !ok {
		// A dynamic of the score's own: "pp leggiermente" sounds as its
		// first word, and is written as the whole.
		words, isOwn := b.ctx.dynTexts[name]
		if !isOwn {
			return
		}
		first := strings.Fields(words)
		if len(first) == 0 {
			return
		}
		d, ok = dynamicsOf[strings.ToLower(first[0])]
		if !ok {
			d = model.MF
		}
		mk.Dynamic = d
		mk.DynamicText = words
		return
	}
	mk.Dynamic = d
	if d.String() != name {
		mk.DynamicText = name
	}
}

// lyrics writes the words under a staff onto its notes.
func (b *builder) lyrics(p *model.Part, st *staffSpec, stream []any) {
	runs := append([]*lyricsNode(nil), lyricsOf(stream)...)
	runs = append(runs, st.lyrics...)
	// Words beside the staves, named for a voice of this staff.
	voices := map[string]bool{}
	for _, e := range eventsIn(stream) {
		if e.vname != "" {
			voices[e.vname] = true
		}
	}
	for _, ly := range b.mv.lyrics {
		if ly.to != "" && voices[ly.to] {
			runs = append(runs, ly)
		}
	}
	if len(runs) == 0 {
		return
	}
	for verse, ly := range runs {
		if len(ly.syllables) == 0 {
			continue
		}
		b.verse(p, stream, ly, verse+1)
	}
}

func (b *builder) verse(p *model.Part, stream []any, ly *lyricsNode, verse int) {
	// Which voice the words go to: the one named, else the first.
	voice := 1
	if ly.to != "" {
		for _, e := range eventsIn(stream) {
			if e.vname == ly.to {
				voice = e.voice
				break
			}
		}
	}
	// The notes of that voice, in order, that take a syllable: not rests,
	// not the second half of a tie.
	var targets []*model.Marks
	for _, m := range p.Measures {
		vi := voice - 1
		if vi < 0 || vi >= len(m.Voices) {
			continue
		}
		for _, el := range m.Voices[vi].Elements {
			mk := marksOfElement(el)
			if mk == nil {
				continue
			}
			if _, isRest := el.(*model.Rest); isRest {
				continue
			}
			if mk.TieStop {
				continue
			}
			targets = append(targets, mk)
		}
	}
	k := 0
	inWord := false
	for _, sy := range ly.syllables {
		if k >= len(targets) {
			break
		}
		if sy.skip {
			k++
			continue
		}
		l := model.Lyric{Text: sy.text, Verse: verse, Extend: sy.extend}
		switch {
		case inWord && sy.hyphen:
			l.Syllabic = model.SyllableMiddle
		case inWord:
			l.Syllabic = model.SyllableEnd
		case sy.hyphen:
			l.Syllabic = model.SyllableBegin
		default:
			l.Syllabic = model.SyllableSingle
		}
		inWord = sy.hyphen
		targets[k].Lyrics = append(targets[k].Lyrics, l)
		k++
	}
}

// words is the marks that belong to the whole score rather than a staff.
func (b *builder) words(s *model.Score, streams [][]any) {
	// A rehearsal mark or a tempo written on one staff is the score's:
	// it goes on every staff of the bar.
	if len(s.Parts) == 0 {
		return
	}
	for mi := range s.Parts[0].Measures {
		var marker string
		var tempo *model.Tempo
		for _, p := range s.Parts {
			if mi >= len(p.Measures) {
				continue
			}
			if marker == "" {
				marker = p.Measures[mi].Marker
			}
			if tempo == nil {
				tempo = p.Measures[mi].Tempo
			}
		}
		for _, p := range s.Parts {
			if mi >= len(p.Measures) {
				continue
			}
			if marker != "" && p.Measures[mi].Marker == "" {
				p.Measures[mi].Marker = marker
			}
			if tempo != nil && p.Measures[mi].Tempo == nil {
				t := *tempo
				p.Measures[mi].Tempo = &t
			}
		}
	}
}
