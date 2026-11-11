/*
 * build.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package dorico

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Turning Dorico's model into gscore's.
//
// The two are closer than most pairs of formats. Dorico keeps a flow — a
// movement — as a set of streams: one per voice, one per stave, one for the
// things that belong to the whole system. A voice stream is a list of
// events, each with an exact rational position and duration measured in
// crotchets, and a note event carries a MIDI pitch. There is no notated
// page in any of it: no beams, no bar numbers, no line breaks, and no bars
// either — where the barlines fall is worked out from the time signatures
// in the system stream, exactly as it is here.
//
// Two things about the pitches are worth knowing before reading what
// follows. They are MIDI numbers, so 60 is middle C. And they are what
// sounds, not what is written: a clarinet in B flat playing the opening of
// the fifth symphony stores 67, the concert G, where its own part reads A.
// gscore stores what the player reads, so a score read from here is a
// concert-pitch score and says so.

// rat is an exact position or duration in crotchets.
//
// Dorico writes them as "69/2" and "1/3", and they have to stay exact: a
// triplet quaver is a third of a crotchet, and three of them are a crotchet
// to the barline and to nothing else.
type rat struct{ n, d int64 }

func (a rat) add(b rat) rat   { return reduce(rat{a.n*b.d + b.n*a.d, a.d * b.d}) }
func (a rat) sub(b rat) rat   { return reduce(rat{a.n*b.d - b.n*a.d, a.d * b.d}) }
func (a rat) cmp(b rat) int   { l, r := a.n*b.d, b.n*a.d; return sign(l - r) }
func (a rat) zero() bool      { return a.n == 0 }
func (a rat) less(b rat) bool { return a.cmp(b) < 0 }

func sign(v int64) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

func reduce(r rat) rat {
	if r.d < 0 {
		r.n, r.d = -r.n, -r.d
	}
	g := gcd(r.n, r.d)
	if g > 1 {
		r.n, r.d = r.n/g, r.d/g
	}
	if r.n == 0 {
		r.d = 1
	}
	return r
}

func gcd(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// parseRat reads "69/2", "35" or "" as a number of crotchets.
func parseRat(s string) (rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return rat{0, 1}, false
	}
	num, den, found := strings.Cut(s, "/")
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return rat{0, 1}, false
	}
	if !found {
		return rat{n, 1}, true
	}
	d, err := strconv.ParseInt(den, 10, 64)
	if err != nil || d == 0 {
		return rat{0, 1}, false
	}
	return reduce(rat{n, d}), true
}

// dur turns a length in crotchets into gscore's own, which counts in whole
// notes: a crotchet is a quarter of one.
func (a rat) dur() model.Duration {
	r := reduce(rat{a.n, a.d * 4})
	return model.Duration{Num: int(r.n), Den: int(r.d)}
}

/* ---------- the pieces of a flow ---------- */

// stave is one staff of one instrument, which is one of gscore's parts.
type stave struct {
	instrument string // Dorico's instrument id, as text
	stave      string // and which of that instrument's staves
	player     string // the player it belongs to, for its name
	sound      string // what it plays, as a MusicXML standard sound id
	stream     string // its own stave stream, where the clefs are
	order      int    // where it comes in the score
	clef       model.Clef
	clefSet    bool
	changes    []clefChange
	dynamics   []dynamic
	swells     []swell
	voices     []string // the voice streams that write on it
}

// bar is one measure of the flow: where it starts, how long it is, and what
// signature it carries.
type bar struct {
	at     rat
	length rat
	time   *model.TimeSignature
	key    *model.KeySignature

	// inKey is the signature in force here, whether or not this bar is
	// where it changed. A pitch is a number in the file and a letter with
	// a sign on the page, and the key is what turns one into the other:
	// read against no key at all, the E flat the fifth symphony opens on
	// comes out as a D sharp.
	inKey model.KeySignature
}

// tuplet is a stretch of a voice played in less time than it is written:
// three notes in the time of two.
//
// It has to be read, because the durations alone cannot say it. A triplet
// quaver lasts a twelfth of a whole note and no notehead spells a twelfth,
// so a reader that drops the tuplet leaves gscore drawing crotchets where
// the page wants three beamed quavers under a 3 — right to the ear,
// wrong to the eye, and wrong in a way that looks like a bug in the
// engraver rather than a gap in the reading.
type tuplet struct {
	at     rat
	dur    rat
	actual int
	normal int
}

// parseTuplet reads Dorico's own way of writing one down:
//
//	steamVariantList:list: {int: 3}, {int: 2}, {list: {int: 7}, {int: 0}}, {bool: true}
//
// which is three in the time of two, counted in a unit this does not need:
// the notes carry their own lengths, and what is wanted here is only the
// ratio the written value is recovered with.
func parseTuplet(s string) (actual, normal int, ok bool) {
	var got []int
	for rest := s; len(got) < 2; {
		i := strings.Index(rest, "{int:")
		if i < 0 {
			break
		}
		rest = rest[i+len("{int:"):]
		j := strings.IndexByte(rest, '}')
		if j < 0 {
			break
		}
		n, err := strconv.Atoi(strings.TrimSpace(rest[:j]))
		if err != nil {
			break
		}
		got = append(got, n)
		rest = rest[j+1:]
	}
	if len(got) < 2 || got[0] <= 0 || got[1] <= 0 {
		return 0, 0, false
	}
	return got[0], got[1], true
}

// event is one note or rest, placed.
type event struct {
	at    rat
	dur   rat
	pitch int // MIDI, or -1 for a rest
	rest  bool
}

// build turns a decoded project into one score per flow.
func build(t *tree, info projectInfo) ([]*model.Score, error) {
	const root int32 = 0
	if t.nodes() == 0 || t.nameOf(root) != "kScore" {
		return nil, errors.New("dorico: this project has no score in it")
	}

	players := playerNames(t, root)
	sounds := instrumentSounds(t, root)
	flows := t.list(root, "flows")
	if len(flows) == 0 {
		return nil, errors.New("dorico: this project has no flows in it, " +
			"so there is no music to read")
	}

	skipped := map[string]int{}
	var out []*model.Score
	for _, f := range flows {
		s := buildFlow(t, f, players, sounds, info, skipped)
		if s != nil {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("dorico: none of the flows in this project " +
			"hold any music")
	}
	// The title goes on the first movement only, which is where it is
	// printed: a symphony's second movement is headed "II" and not headed
	// "Symphony No. 5" again.
	if info.Title != "" {
		out[0].Identity.Title = info.Title
		out[0].Identity.Subtitle = info.Subtitle
	}
	for _, s := range out {
		if s.Identity.Composer == "" {
			s.Identity.Composer = info.Composer
		}
	}
	if len(skipped) > 0 {
		return out, &PartialError{Skipped: report(skipped)}
	}
	return out, nil
}

// playerNames reads what each player is called.
func playerNames(t *tree, root int32) map[string]string {
	out := map[string]string{}
	for _, p := range t.list(root, "scorePlayers") {
		id := t.str(p, "playerID")
		name := t.str(p, "baseName")
		if name == "" {
			name = t.str(p, "displayName")
		}
		if id != "" {
			out[id] = name
		}
	}
	return out
}

// instrumentSounds works out what each of the score's instruments is, as a
// MusicXML standard sound id.
//
// Two steps, because the file keeps the two halves apart. A player holds
// instruments, and each of those names an entity — "instrument.wind.flute".
// The entity itself is in the library, with the sound id beside it, and
// that id is the same vocabulary gscore keeps in Part.Sound: Dorico writes
// "wind.flutes.flute" there because that is what MusicXML would be written
// with, which makes this the one place where the two programs already
// agree about what an instrument is.
func instrumentSounds(t *tree, root int32) map[string]instrumentDef {
	type entityDef struct {
		sound string
		clefs []model.Clef
	}
	entity := map[string]entityDef{}
	for _, def := range t.every("InstrumentEntityDefinition") {
		id := t.str(def, "entityID")
		if id == "" {
			continue
		}
		d := entityDef{sound: t.str(def, "musicXMLSoundID")}
		// The clef each of its staves is written in, in order: one for
		// most instruments, two for a piano. It is the instrument that
		// knows — a flow writes a clef event only where the clef changes,
		// so a cello that stays in bass clef all movement has none at all,
		// and a staff read from the events alone comes out in whatever
		// the reader's default was. Which is how a double bass ended up
		// in the treble clef, eight ledger lines under every note.
		for _, each := range t.children(t.path(def, "clefs",
			"clefIDsForEachStave")) {
			c, ok := clefNamed(t.str(each, "idForTransposingLayouts"))
			if !ok {
				c = model.Treble
			}
			d.clefs = append(d.clefs, c)
		}
		entity[id] = d
	}
	out := map[string]instrumentDef{}
	for _, p := range t.list(root, "scorePlayers") {
		for _, inst := range t.list(p, "instruments") {
			id := t.str(inst, "instrumentID")
			if id == "" {
				continue
			}
			name := t.str(inst, "entityID")
			if name == "" {
				name = t.str(inst, "baseInstrumentEntityID")
			}
			d := entity[name]
			out[id] = instrumentDef{sound: d.sound, clefs: d.clefs}
		}
	}
	return out
}

// instrumentDef is what one of the score's instruments is: what it sounds
// like, and what its staves are written in.
type instrumentDef struct {
	sound string
	clefs []model.Clef
}

// report turns the tally of what was not read into lines somebody can read,
// commonest first.
func report(skipped map[string]int) []string {
	kinds := sortedStrings(skipped)
	sort.SliceStable(kinds, func(i, j int) bool {
		return skipped[kinds[i]] > skipped[kinds[j]]
	})
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		if n := skipped[k]; n > 1 {
			out = append(out, fmt.Sprintf("%s (%d of them)", k, n))
		} else {
			out = append(out, k)
		}
	}
	return out
}

// buildFlow reads one movement.
func buildFlow(t *tree, flow int32, players map[string]string,
	sounds map[string]instrumentDef, info projectInfo,
	skipped map[string]int) *model.Score {

	staves := flowStaves(t, flow, players, sounds)
	if len(staves) == 0 {
		return nil
	}
	streams := flowStreams(t, flow)
	elements := elementTables(t, flow)

	// The notes first, because where the music ends decides how many bars
	// there are to make.
	// Each of Dorico's voices becomes one or more of gscore's: see lanes.
	music := map[string][][]group{} // by voice stream id
	last := rat{0, 1}
	for _, st := range staves {
		for _, v := range st.voices {
			var evs []event
			var tups []tuplet
			for _, p := range streams.of("kVoiceStream", v) {
				got, gotTups, end := readEvents(t, p.node, p.at, skipped)
				evs = append(evs, got...)
				tups = append(tups, gotTups...)
				if last.less(end) {
					last = end
				}
			}
			if len(evs) == 0 {
				continue
			}
			sortEvents(evs)
			ls := lanes(evs)
			for _, lane := range ls {
				markTuplets(lane, tups)
			}
			music[v] = ls
		}
	}

	bars := barGrid(t, streams, elements, last, skipped)
	if len(bars) == 0 {
		return nil
	}
	readClefs(t, streams, staves, bars, elements, skipped)

	s := model.NewScore()
	s.Parts = nil
	s.Identity.Title = t.str(flow, "flowName")

	tempo := readTempo(t, streams, skipped)
	for _, st := range staves {
		p := &model.Part{
			Name:  st.player,
			Sound: st.sound,
			Clef:  st.clef,
			Key:   model.KeySignature{},
			Time:  model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		if bars[0].time != nil {
			p.Time = *bars[0].time
		}
		if bars[0].key != nil {
			p.Key = *bars[0].key
		}
		// What it is played with. The sound id says which instrument this
		// is — a bass clarinet rather than a clarinet — and the General
		// MIDI program is the nearest thing that can make the noise.
		// Without it every staff of an orchestral score is a piano, which
		// is what a score read from a format that names no instruments
		// has to fall back to and is not what this one says.
		if prog, ok := model.SoundProgram(st.sound); ok {
			p.Instrument = prog
		} else if prog, ok := model.InstrumentNamed(st.player); ok {
			// Nothing in the file said, so the player's own name is
			// asked: "Flute 1" is a flute in any language gscore knows.
			p.Instrument = prog
		} else if st.sound == "" {
			skipped["what "+st.player+" plays"]++
		}
		// Every voice that writes on this staff, laid into the bars.
		var voices [][]group
		for _, v := range st.voices {
			voices = append(voices, music[v]...)
		}
		p.Measures = layOut(voices, bars)
		attach(p, bars, st.dynamics, st.swells)
		if moved := putClefs(p, bars, st.changes); moved > 0 {
			skipped["clef changes written inside a bar, put at its start"] += moved
		}
		s.Parts = append(s.Parts, p)
	}
	setTempo(s, bars, tempo)
	return s
}

// flowStaves works out who is playing in this flow and on which staves.
//
// A flow holds a flowPlayer for each player, each with the instruments that
// player is holding, each of those with the staves it is written on and the
// voices written on them. The nesting is the answer to "which staff does
// this note belong to", which is asked of every note in the file.
func flowStaves(t *tree, flow int32, players map[string]string,
	sounds map[string]instrumentDef) []*stave {
	var out []*stave
	for _, fp := range t.list(flow, "flowPlayers") {
		player := t.str(fp, "playerID")
		name := players[player]
		if name == "" {
			name = "Player " + player
		}
		for _, fpi := range t.list(fp, "flowPlayerInstruments") {
			inst := t.str(fpi, "instrumentID")
			here := map[string]*stave{}
			for _, ss := range t.list(fpi, "staveStreamInfos") {
				st := &stave{
					instrument: t.str(ss, "instrumentID"),
					stave:      t.str(ss, "staveID"),
					stream:     t.str(ss, "staveStreamID"),
					player:     name,
					order:      len(out),
					clef:       model.Treble,
				}
				// What this instrument is, and what its staves are
				// written in. The nth stave of the instrument takes the
				// nth clef: a piano's right hand is treble and its left
				// is bass, and they are listed in that order.
				was := sounds[st.instrument]
				if was.sound == "" {
					was = sounds[inst]
				}
				st.sound = was.sound
				nth, _ := strconv.Atoi(t.str(ss, "nthStaveInInstrument"))
				if nth >= 0 && nth < len(was.clefs) {
					st.clef = was.clefs[nth]
				}
				if st.instrument == "" {
					st.instrument = inst
				}
				here[st.stave] = st
				out = append(out, st)
			}
			for _, vs := range t.list(fpi, "voiceStreamInfos") {
				id := t.str(vs, "voiceStreamID")
				on := t.str(vs, "defaultLogicalStaveID")
				st := here[on]
				if st == nil {
					// A voice on a staff the file did not list: it belongs
					// to this instrument, and the first staff is where it
					// goes rather than nowhere.
					for _, any := range here {
						if st == nil || any.order < st.order {
							st = any
						}
					}
				}
				if st == nil {
					continue
				}
				st.voices = append(st.voices, id)
			}
		}
	}
	return out
}

// streams resolves a stream to the music in it.
//
// Nothing points straight at anything here, and for a good reason: a block
// of events is shared material, an instance of it is that material placed
// somewhere, and a stream is a list of such placements. A repeated passage
// is then one block with two instances rather than two copies, which is
// also why an instance carries a position — where the block sits, with its
// own events written relative to that.
//
//	eventStream{streamID, eventStreamType} → blockInstance{blockID, position}
//	                                       → block{events}
//
// Reading blockID as though it were the stream's own number happens to work
// for the first dozen streams of a project and then quietly stops, which
// looks exactly like a score whose strings have no notes in them.
type streams struct {
	block    map[string]int32    // blockID → the block
	instance map[string][]string // instanceID → blockID, position
	stream   map[string][]string // type:streamID → instance ids
}

type placed struct {
	node int32
	at   rat
}

func flowStreams(t *tree, flow int32) *streams {
	s := &streams{
		block:    map[string]int32{},
		instance: map[string][]string{},
		stream:   map[string][]string{},
	}
	for _, b := range t.list(flow, "blocks") {
		if id := t.str(b, "blockID"); id != "" {
			s.block[id] = b
		}
	}
	for _, bi := range t.list(flow, "blockInstances") {
		id := t.str(bi, "blockInstanceID")
		if id == "" {
			continue
		}
		s.instance[id] = []string{t.str(bi, "blockID"), t.str(bi, "position")}
	}
	for _, es := range t.list(flow, "eventStreams") {
		id := t.str(es, "streamID")
		kind := t.str(es, "eventStreamType")
		if id == "" || kind == "" {
			continue
		}
		s.stream[kind+":"+id] = splitIDs(t.str(es, "blockInstanceIDs"))
	}
	return s
}

// splitIDs reads a list of numbers written as one value.
func splitIDs(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	return f
}

// of lists the blocks making up one stream, with where each one sits.
func (s *streams) of(kind, id string) []placed {
	var out []placed
	for _, inst := range s.stream[kind+":"+id] {
		where := s.instance[inst]
		if len(where) != 2 {
			continue
		}
		b, ok := s.block[where[0]]
		if !ok {
			continue
		}
		at, _ := parseRat(where[1])
		out = append(out, placed{node: b, at: at})
	}
	return out
}

// ofKind lists every stream of a kind, which is how the signatures are
// found: they are on whichever stream is the global one.
func (s *streams) ofKind(kind string) []placed {
	var out []placed
	for key := range s.stream {
		k, id, _ := strings.Cut(key, ":")
		if k != kind {
			continue
		}
		out = append(out, s.of(kind, id)...)
	}
	return out
}

// elementTables indexes the definitions events refer to by number: a bar
// division event says "element 4", and the time signature is in element 4.
func elementTables(t *tree, flow int32) map[string]int32 {
	out := map[string]int32{}
	for _, table := range t.list(flow, "elementTables") {
		for _, def := range t.children(table) {
			for _, e := range t.children(def) {
				name := t.nameOf(e)
				if !strings.HasSuffix(name, "ElementDefinition") {
					continue
				}
				if id := t.str(e, "elementID"); id != "" {
					out[name+":"+id] = e
				}
			}
		}
	}
	return out
}

// readEvents reads one voice stream's notes and rests.
func readEvents(t *tree, block int32, offset rat,
	skipped map[string]int) ([]event, []tuplet, rat) {

	events := t.child(block, "events")
	if events < 0 {
		return nil, nil, rat{0, 1}
	}
	var out []event
	var tups []tuplet
	end := rat{0, 1}
	for _, e := range t.children(events) {
		kind := t.nameOf(e)
		at, ok := parseRat(t.str(e, "position"))
		if !ok {
			continue
		}
		at = at.add(offset)
		d, _ := parseRat(t.str(e, "duration"))
		switch kind {
		case "NoteEventDefinition":
			pitch, err := strconv.Atoi(t.str(e, "pitch"))
			if err != nil || d.zero() {
				continue
			}
			out = append(out, event{at: at, dur: d, pitch: pitch})
		case "RestEventDefinition":
			if d.zero() {
				continue
			}
			out = append(out, event{at: at, dur: d, rest: true})
		case "TupletEventDefinition":
			a, n, ok := parseTuplet(t.str(e, "tupletDefinition"))
			if !ok || d.zero() {
				skipped["tuplets"]++
				continue
			}
			tups = append(tups, tuplet{at: at, dur: d, actual: a, normal: n})
			continue
		case "GraceNoteEventDefinition":
			skipped["grace notes"]++
			continue
		default:
			note(skipped, kind)
			continue
		}
		if fin := at.add(d); end.less(fin) {
			end = fin
		}
	}
	sortEvents(out)
	return out, tups, end
}

// sortEvents puts a voice's events in the order they are played, and the
// notes of a chord in a settled order within that.
func sortEvents(evs []event) {
	sort.SliceStable(evs, func(i, j int) bool {
		if c := evs[i].at.cmp(evs[j].at); c != 0 {
			return c < 0
		}
		if c := evs[i].dur.cmp(evs[j].dur); c != 0 {
			return c < 0
		}
		return evs[i].pitch < evs[j].pitch
	})
}

// note records a kind of thing this reader passes over, in words rather
// than in the file's own vocabulary: somebody opening a project wants to
// know that the slurs did not come through, not that
// SlurEventDefinition did not.
func note(skipped map[string]int, kind string) {
	switch kind {
	case "SlurEventDefinition":
		skipped["slurs"]++
	case "DynamicGroupEventDefinition":
		skipped["dynamics"]++
	case "TupletEventDefinition":
		skipped["tuplets"]++
	case "TrillEventDefinition":
		skipped["trills"]++
	case "PlayingTechniqueEventDefinition":
		skipped["playing techniques"]++
	case "TextEventDefinition":
		skipped["text"]++
	case "RehearsalMarkEventDefinition":
		skipped["rehearsal marks"]++
	case "PauseEventDefinition":
		skipped["pauses"]++
	case "GenerativeBarRepeatEventDefinition":
		skipped["bar repeats"]++
	case "GradualTempoChangeEventDefinition":
		skipped["gradual tempo changes, such as rit. and accel."]++
	case "RepeatEndingLineEventDefinition":
		skipped["repeat endings, the first and second time bars"]++
	case "RepeatMarkerEventDefinition":
		skipped["repeat marks"]++
	case "CueEventDefinition":
		skipped["cues"]++
	case "CommentEventDefinition":
		skipped["comments"]++
	case "OttavaLineEventDefinition":
		skipped["octave lines, the 8va brackets"]++
	case "TonalityDivisionEventDefinition":
		// On the global stream these are the key signatures and are read.
		// Here they are a key for one staff on its own — a horn left in C
		// while the rest of the orchestra modulates.
		skipped["key changes on a single staff"]++
	case "ImmediateTempoChangeEventDefinition":
		// Read from this same stream by readTempo, which wants the whole
		// of the data rather than the fact that one went by.
	case "BreakEventDefinition", "CondensationLabelEventDefinition",
		"CondensingChangeEventDefinition":
		// How the page is laid out and how staves are condensed, which
		// gscore works out for itself. Not something that was lost.
	default:
		// Something this reader has never met. Named as the file names it,
		// which is ugly, and better than silence: it is a thing in
		// somebody's score that did not come through, and the name is
		// what makes it possible to ask about.
		if strings.HasSuffix(kind, "EventDefinition") {
			skipped["things gscore does not know, called "+
				strings.TrimSuffix(kind, "EventDefinition")+" by Dorico"]++
		}
	}
}

// barGrid works out where the barlines fall.
//
// Dorico stores no bars. It stores the time signatures, as events on the
// stream that belongs to the whole system, and the bars follow from them:
// a 2/4 lasts two crotchets, and the next barline is two crotchets after
// the last one until something says otherwise.
func barGrid(t *tree, streams *streams, elements map[string]int32,
	until rat, skipped map[string]int) []bar {

	type change struct {
		at   rat
		time *model.TimeSignature
		key  *model.KeySignature
	}
	var changes []change
	for _, p := range streams.ofKind("kGlobalTimebaseStream") {
		events := t.child(p.node, "events")
		for _, e := range t.children(events) {
			at, ok := parseRat(t.str(e, "position"))
			if !ok {
				continue
			}
			at = at.add(p.at)
			switch t.nameOf(e) {
			case "BarDivisionEventDefinition":
				def := elements["BarDivisionElementDefinition:"+
					t.str(e, "barDivisionElementID")]
				if ts, ok := timeSignature(t, def); ok {
					changes = append(changes, change{at: at, time: &ts})
				}
			case "TonalityDivisionEventDefinition":
				def := elements["TonalityDivisionElementDefinition:"+
					t.str(e, "tonalityDivisionElementID")]
				if ks, ok := keySignature(t, def); ok {
					changes = append(changes, change{at: at, key: &ks})
				}
			default:
				note(skipped, t.nameOf(e))
			}
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].at.less(changes[j].at)
	})

	now := model.TimeSignature{Beats: 4, BeatValue: 4}
	var key *model.KeySignature
	var inKey model.KeySignature
	var out []bar
	at := rat{0, 1}
	next := 0
	// A flow with no music in it still has a bar, because an empty movement
	// is still a movement and a score with no measures cannot be drawn.
	for at.less(until) || len(out) == 0 {
		b := bar{at: at}
		for next < len(changes) && !at.less(changes[next].at) {
			if c := changes[next].time; c != nil {
				// Only where it is actually a change. Dorico writes one of
				// these at every point where the barline arrangement is
				// settled — a repeat end, the join of two sections — and
				// most of them restate the meter already in force. Printed
				// as written, the finale carries ten four-fours, nine of
				// them in the middle of music that never leaves four-four.
				if *c != now {
					b.time = c
				}
				now = *c
			}
			if changes[next].key != nil {
				key = changes[next].key
				inKey = *key
				b.key = changes[next].key
			}
			next++
		}
		if len(out) == 0 {
			// The first bar carries both signatures whether or not the file
			// restated them, because a part has to open with one.
			t := now
			b.time = &t
			if key != nil {
				k := *key
				b.key = &k
			}
		}
		b.inKey = inKey
		b.length = reduce(rat{int64(now.Beats) * 4, int64(now.BeatValue)})
		if b.length.zero() {
			b.length = rat{4, 1}
		}
		out = append(out, b)
		at = at.add(b.length)
		if len(out) > 100000 {
			break // a runaway rather than a piece
		}
	}
	return out
}

// timeSignature reads a bar division's numerator and denominator.
func timeSignature(t *tree, def int32) (model.TimeSignature, bool) {
	ts := t.path(def, "barDivisionData", "timeSignature",
		"timeSignaturesAndDivisions", "timeSignatureAndDivision",
		"timeSignature")
	if ts < 0 {
		return model.TimeSignature{}, false
	}
	beats, err1 := strconv.Atoi(t.str(ts, "numerator"))
	unit, err2 := strconv.Atoi(t.str(ts, "denominator"))
	if err1 != nil || err2 != nil || beats <= 0 || unit <= 0 {
		return model.TimeSignature{}, false
	}
	return model.TimeSignature{Beats: beats, BeatValue: unit}, true
}

// keySignature reads a tonality division: which note the key is rooted on,
// what is done to that note, and whether it is major or minor.
//
// Dorico says the key the way a musician says it — "E flat major" — where
// gscore counts the sharps or flats in the signature. The circle of fifths
// is what turns one into the other.
func keySignature(t *tree, def int32) (model.KeySignature, bool) {
	ks := t.path(def, "tonalityDivisionData", "keySignature")
	if ks < 0 {
		return model.KeySignature{}, false
	}
	root := t.child(ks, "root")
	if root < 0 {
		return model.KeySignature{}, false
	}
	kind := t.str(root, "tonalityType")
	inner := t.child(root, "root")
	if inner < 0 {
		return model.KeySignature{}, false
	}
	name := strings.TrimSpace(t.str(inner, "noteName"))
	if name == "" {
		return model.KeySignature{}, false
	}
	// How far round the circle of fifths the natural note sits.
	steps := map[string]int{"F": -1, "C": 0, "G": 1, "D": 2, "A": 3, "E": 4, "B": 5}
	fifths, ok := steps[strings.ToUpper(name[:1])]
	if !ok {
		return model.KeySignature{}, false
	}
	// And a sharp or a flat on it moves seven places, which is what a
	// sharp or a flat is: seven fifths.
	switch acc := t.str(inner, "accidentalID"); {
	case strings.HasSuffix(acc, ".sharp"):
		fifths += 7
	case strings.HasSuffix(acc, ".flat"):
		fifths -= 7
	case strings.HasSuffix(acc, ".doublesharp"):
		fifths += 14
	case strings.HasSuffix(acc, ".doubleflat"):
		fifths -= 14
	}
	mode := model.Major
	if strings.Contains(kind, "Minor") {
		mode = model.Minor
		// A minor key's signature is that of its relative major, three
		// places further round.
		fifths -= 3
	}
	if fifths < -7 || fifths > 7 {
		return model.KeySignature{}, false
	}
	return model.KeySignature{Fifths: fifths, Mode: mode}, true
}

// readClefs reads the clefs a staff is written in.
//
// The one it opens with is the instrument's own — see instrumentSounds —
// because a flow writes a clef event only where something happens. What is
// here is those events: an event at the very start settles the opening
// clef, and one later in the movement is a change, which is kept against
// the bar it falls in.
//
// A cello reads bass and goes up to tenor for a high passage and back. Read
// as though the earliest event were the opening clef, that cello starts the
// movement in tenor and every note of the first page sits two ledger lines
// out.
func readClefs(t *tree, streams *streams, staves []*stave, bars []bar,
	elements map[string]int32, skipped map[string]int) {

	for _, st := range staves {
		for _, p := range streams.of("kInstrumentStaveStream", st.stream) {
			// The dynamics are on this stream too, and belong to the
			// staff rather than to any one voice on it: a crescendo is
			// for the player.
			marks, swells := readDynamics(t, p, elements, skipped)
			st.dynamics = append(st.dynamics, marks...)
			st.swells = append(st.swells, swells...)
			events := t.child(p.node, "events")
			for _, e := range t.children(events) {
				if t.nameOf(e) == "DynamicGroupEventDefinition" {
					continue // read above
				}
				if t.nameOf(e) != "ClefEventDefinition" {
					// Whatever else is on this stream — the cues, the bar
					// repeats — is said rather than passed over.
					note(skipped, t.nameOf(e))
					continue
				}
				pos, ok := parseRat(t.str(e, "position"))
				if !ok {
					continue
				}
				pos = pos.add(p.at)
				name := t.str(t.child(e, "entityIDs"),
					"idForTransposingLayouts")
				c, ok := clefNamed(name)
				if !ok {
					skipped["clefs gscore does not know, called "+
						name+" by Dorico"]++
					continue
				}
				if pos.zero() {
					st.clef, st.clefSet = c, true
					continue
				}
				st.changes = append(st.changes, clefChange{at: pos, clef: c})
			}
		}
		sort.SliceStable(st.changes, func(i, j int) bool {
			return st.changes[i].at.less(st.changes[j].at)
		})
	}
}

// clefChange is a clef taken up partway through a movement.
type clefChange struct {
	at   rat
	clef model.Clef
}

// putClefs writes the clef changes against the bars they fall in.
//
// Against the bar, because that is where gscore keeps one. A change written
// in the middle of a bar — which happens, at the end of a run that climbs —
// is put at the start of that bar instead, which moves it by a few notes
// and never by more than a bar.
func putClefs(p *model.Part, bars []bar, changes []clefChange) int {
	moved := 0
	for _, c := range changes {
		at := -1
		for i := range bars {
			if bars[i].at.cmp(c.at) <= 0 {
				at = i
				continue
			}
			break
		}
		if at < 0 || at >= len(p.Measures) {
			continue
		}
		if !bars[at].at.zero() && bars[at].at.cmp(c.at) != 0 {
			moved++
		}
		clef := c.clef
		p.Measures[at].Clef = &clef
	}
	return moved
}

func clefNamed(id string) (model.Clef, bool) {
	switch {
	case strings.HasSuffix(id, ".treble"), strings.HasSuffix(id, ".treble8vb"):
		return model.Treble, true
	case strings.HasSuffix(id, ".bass"):
		return model.Bass, true
	case strings.HasSuffix(id, ".alto"):
		return model.Alto, true
	case strings.HasSuffix(id, ".tenor"):
		return model.Tenor, true
	}
	return model.Treble, false
}

/* ---------- laying the events into bars ---------- */

// group is what becomes one element: the notes of a voice that start
// together and last the same time, which is a chord, or one note, or a
// rest.
type group struct {
	at      rat
	dur     rat
	pitches []int
	rest    bool

	// tup is the tuplet this belongs to, if any, with Start and Stop set
	// on the ends of the group so the bracket and its figure are drawn
	// over the right notes.
	tup *model.Tuplet
}

// markTuplets says which groups are played in less time than they are
// written.
//
// A group is in a tuplet when it begins inside that tuplet's span. The
// first and last in each span carry Start and Stop, which is what brackets
// the figure over them.
//
// A span holding one group alone is a measured tremolo rather than a
// triplet — six sixteenths in the time of four, written as one note with
// strokes through its stem — and a bracketed figure over a single note is
// in no printed score. The arithmetic is kept and the drawing is not.
func markTuplets(groups []group, tups []tuplet) {
	for _, tp := range tups {
		stop := tp.at.add(tp.dur)
		first, last := -1, -1
		for i := range groups {
			if groups[i].at.cmp(tp.at) < 0 || !groups[i].at.less(stop) {
				continue
			}
			if first < 0 {
				first = i
			}
			last = i
		}
		if first < 0 {
			continue
		}
		for i := first; i <= last; i++ {
			// The ratio is what recovers the written value from the
			// sounding one, so it goes on only where it gives a value
			// that can be written. A measured tremolo — six sixteenths in
			// the time of four, drawn as one note with strokes through
			// its stem — is stored as a tuplet because the arithmetic is
			// a tuplet's, and scaling its dotted minim by six quarters
			// gives a length no notehead spells. There the sounding value
			// is already drawable and is left alone.
			sounding := groups[i].dur.dur()
			written := sounding.Scale(tp.actual, tp.normal)
			if !writable(written) && writable(sounding) {
				continue
			}
			groups[i].tup = &model.Tuplet{
				Actual: tp.actual,
				Normal: tp.normal,
				Start:  i == first,
				Stop:   i == last,
			}
		}
	}
}

// lanes turns one of Dorico's voices into as many of gscore's as it needs.
//
// They are not the same thing. A Dorico voice may hold notes that overlap —
// a double stop whose upper note is held while the lower moves, a chord
// one of whose notes rings on — because its events carry their own
// positions and lengths and nothing says they may not. A gscore voice is a
// line of elements one after another, and the same music written there
// would add up to more than the bar holds.
//
// So the events are dealt into lanes: a group goes in the first lane whose
// last note has finished, and a new lane is opened when none has. Notes
// that genuinely sound together as a chord are gathered first, so that a
// chord stays one element and only a real overlap makes a second voice.
func lanes(evs []event) [][]group {
	var groups []group
	for i := 0; i < len(evs); {
		j, g := i, group{at: evs[i].at, dur: evs[i].dur, rest: evs[i].rest}
		for j < len(evs) && evs[j].at.cmp(g.at) == 0 &&
			evs[j].dur.cmp(g.dur) == 0 && evs[j].rest == g.rest {
			if !evs[j].rest {
				g.pitches = append(g.pitches, evs[j].pitch)
			}
			j++
		}
		groups = append(groups, g)
		i = j
	}

	var out [][]group
	var ends []rat
	for _, g := range groups {
		put := -1
		for i := range out {
			if ends[i].cmp(g.at) <= 0 {
				put = i
				break
			}
		}
		if put < 0 {
			out = append(out, nil)
			ends = append(ends, rat{0, 1})
			put = len(out) - 1
		}
		out[put] = append(out[put], g)
		ends[put] = g.at.add(g.dur)
	}
	return out
}

// layOut puts a staff's voices into measures.
//
// Dorico has no bars, so this is where they are made: each voice is walked
// against the grid of barlines, the gaps between its notes are filled with
// rests, and anything that runs over a barline is split and tied, because a
// note cannot be drawn across one.
func layOut(voices [][]group, bars []bar) []*model.Measure {
	out := make([]*model.Measure, 0, len(bars))
	for i := range bars {
		m := &model.Measure{}
		if bars[i].time != nil {
			ts := *bars[i].time
			m.Time = &ts
		}
		if bars[i].key != nil {
			ks := *bars[i].key
			m.Key = &ks
		}
		for _, v := range voices {
			m.Voices = append(m.Voices, &model.Voice{
				Elements: fill(v, bars[i]),
			})
		}
		if len(m.Voices) == 0 {
			m.Voices = []*model.Voice{{Elements: []model.Element{
				&model.Rest{Dur: bars[i].length.dur()},
			}}}
		}
		out = append(out, m)
	}
	return out
}

// fill is one voice's worth of one bar.
func fill(groups []group, b bar) []model.Element {
	end := b.at.add(b.length)
	var out []model.Element
	at := b.at

	for _, g := range groups {
		stop := g.at.add(g.dur)
		if stop.cmp(b.at) <= 0 || !g.at.less(end) {
			continue
		}
		// A note that began in an earlier bar is already sounding: what is
		// in this one is the rest of it, tied back.
		from := g.at
		tiedBack := false
		if from.less(b.at) {
			from, tiedBack = b.at, true
		}
		if at.less(from) {
			// The silence before this group, written the same way its
			// notes are: in pieces a rest can be drawn as. A gap inside a
			// tuplet is itself in the tuplet, or the bar does not add up
			// on the page.
			out = append(out, rests(from.sub(at), g.tup)...)
			at = from
		}
		to := stop
		tiedOn := false
		if end.less(to) {
			to, tiedOn = end, true
		}
		if d := to.sub(from); !d.zero() {
			out = append(out, elements(g, d, b.inKey, tiedBack, tiedOn)...)
			at = to
		}
	}
	if at.less(end) {
		out = append(out, rests(end.sub(at), nil)...)
	}
	if len(out) == 0 {
		out = append(out, rests(b.length, nil)...)
	}
	return out
}

// rests writes a silence as the pieces it takes to draw it.
//
// Through the same spelling the notes go through, and inside a tuplet
// against the written length for the same reason: a rest nobody can draw
// is as wrong as a note nobody can draw, and it is the commoner of the two
// — every gap between one note and the next is one of these.
func rests(d rat, tup *model.Tuplet) []model.Element {
	parts := spell(d.dur())
	if a, n, ok := tup.Ratio(); ok {
		parts = nil
		for _, p := range spell(d.dur().Scale(a, n)) {
			parts = append(parts, p.Scale(n, a))
		}
	}
	var out []model.Element
	for _, p := range parts {
		r := &model.Rest{Dur: p}
		r.Tuplet = tup
		out = append(out, r)
	}
	return out
}

// elements makes the note, chord or rest of one stretch of a voice — or
// the several tied ones it takes to write that stretch down.
//
// A stretch cut at a barline is any length at all, and most lengths have no
// notehead: five eighths of a whole note is a minim tied to a quaver, and
// written as one notehead it comes out as a crotchet, because that is what
// the engraver falls back to when a duration cannot be spelled. So it is
// spelled here instead, into the longest pieces that can be written, tied
// to each other.
func elements(g group, d rat, key model.KeySignature,
	tiedBack, tiedOn bool) []model.Element {
	// Inside a tuplet it is the written value that has to be spellable,
	// not the sounding one: what the engraver draws is the sounding
	// length scaled back up by the ratio, so that is the length to break
	// into noteheads before scaling each piece back down again.
	parts := spell(d.dur())
	if a, n, ok := g.tup.Ratio(); ok {
		parts = nil
		for _, p := range spell(d.dur().Scale(a, n)) {
			parts = append(parts, p.Scale(n, a))
		}
	}
	out := make([]model.Element, 0, len(parts))
	for i, p := range parts {
		back := tiedBack || i > 0
		on := tiedOn || i < len(parts)-1
		out = append(out, element(g, p, key, back, on))
	}
	return out
}

// writable says whether a duration can be drawn, as one notehead or as a
// few tied together.
func writable(d model.Duration) bool {
	for _, p := range spell(d) {
		if _, _, ok := p.Written(); !ok {
			return false
		}
	}
	return true
}

// spell breaks a duration into pieces a notehead can write.
//
// Only a duration counted in halves of a whole note: a third of one is a
// tuplet's business and is spelled by its ratio, not by tying smaller and
// smaller pieces together for ever. Where nothing can be done the duration
// is handed back whole, and the engraver draws what it draws.
func spell(d model.Duration) []model.Duration {
	if _, _, ok := d.Written(); ok {
		return []model.Duration{d}
	}
	r := d.Reduced()
	if r.Den <= 0 || r.Den&(r.Den-1) != 0 {
		return []model.Duration{d} // a tuplet value
	}
	var out []model.Duration
	left := r
	for i := 0; i < 8 && left.Num > 0; i++ {
		best, found := model.Duration{}, false
		for v := model.DoubleWhole; v <= model.HundredTwentyEighth; v++ {
			for dots := 0; dots <= 2; dots++ {
				cand := v.Duration().Scale(1<<(dots+1)-1, 1<<dots)
				if cand.Cmp(left) > 0 {
					continue
				}
				if !found || cand.Cmp(best) > 0 {
					best, found = cand, true
				}
			}
		}
		if !found {
			break
		}
		out = append(out, best)
		left = left.Sub(best).Reduced()
	}
	if left.Num > 0 || len(out) == 0 {
		return []model.Duration{d}
	}
	return out
}

// element makes one note, chord or rest.
func element(g group, dur model.Duration, key model.KeySignature,
	tiedBack, tiedOn bool) model.Element {
	if len(g.pitches) == 0 {
		r := &model.Rest{Dur: dur}
		r.Tuplet = g.tup
		return r
	}
	pitches := append([]int(nil), g.pitches...)
	sort.Ints(pitches)
	if len(pitches) == 1 {
		n := &model.Note{Pitch: model.PitchInKey(pitches[0], key), Dur: dur}
		n.TieStart, n.TieStop = tiedOn, tiedBack
		n.Tuplet = g.tup
		return n
	}
	ch := &model.Chord{Dur: dur}
	for _, p := range pitches {
		ch.Notes = append(ch.Notes, &model.Note{
			Pitch: model.PitchInKey(p, key), Dur: dur,
		})
	}
	ch.TieStart, ch.TieStop = tiedOn, tiedBack
	ch.Tuplet = g.tup
	return ch
}

/* ---------- the dynamics ---------- */

// Dorico keeps dynamics on the staff rather than in a voice, which is where
// they belong: a crescendo is for the player, not for one of the lines they
// happen to be reading. A group is one gesture — "p, growing, to f" is
// three sub-elements of a single group — and each sub-element carries its
// own position within it.
//
// The intensity is written as a pair: which side of the middle it is on,
// and how many letters. {int: 0} … {int: 2} is ff, {int: 1} … {int: 2} is
// pp, and nothing at all in the second place is the mezzo of that side.
// Anchored on the opening of this symphony, which is ff and is the hardest
// thing in music to be wrong about.
type dynamic struct {
	at rat
	d  model.Dynamic
}

type swell struct {
	at   rat
	dur  rat
	kind model.HairpinKind
}

// readDynamics gathers one staff's dynamics and hairpins.
func readDynamics(t *tree, p placed, elements map[string]int32,
	skipped map[string]int) ([]dynamic, []swell) {

	var marks []dynamic
	var swells []swell
	events := t.child(p.node, "events")
	for _, e := range t.children(events) {
		if t.nameOf(e) != "DynamicGroupEventDefinition" {
			continue
		}
		base, ok := parseRat(t.str(e, "position"))
		if !ok {
			continue
		}
		base = base.add(p.at)
		group := elements["DynamicGroupElementDefinition:"+
			t.str(e, "dynamicGroupID")]
		if group < 0 {
			skipped["dynamics"]++
			continue
		}
		for _, sub := range t.children(t.child(group, "subElements")) {
			at, _ := parseRat(t.str(sub, "position"))
			at = at.add(base)
			switch t.nameOf(sub) {
			case "ImmediateChangeDynamicDefinition":
				d, ok := intensity(t.str(sub, "intensity"),
					t.str(sub, "intensityModifier"), skipped)
				if !ok {
					skipped["dynamics"]++
					continue
				}
				marks = append(marks, dynamic{at: at, d: d})
			case "ForceDynamicDefinition":
				// The struck dynamics: sforzando and its relatives, which
				// are a kind of their own here rather than a loudness.
				marks = append(marks, dynamic{at: at,
					d: forced(t.str(sub, "type"), skipped)})
			case "CombinedDynamicDefinition":
				// Two dynamics in one mark. The only one gscore writes is
				// forte-piano, which is what "down" means: struck loud and
				// dropped at once.
				if t.str(sub, "direction") != "kDown" {
					skipped["combined dynamics other than fp"]++
					continue
				}
				marks = append(marks, dynamic{at: at, d: model.FortePiano})
			case "GradualChangeDynamicDefinition":
				dur, _ := parseRat(t.str(sub, "duration"))
				if dur.zero() {
					skipped["hairpins"]++
					continue
				}
				kind := model.Crescendo
				if t.str(sub, "direction") == "kGradualDynamicDown" {
					kind = model.Diminuendo
				}
				swells = append(swells, swell{at: at, dur: dur, kind: kind})
			default:
				skipped["dynamics"]++
			}
		}
	}
	return marks, swells
}

// intensity reads how loud, out of Dorico's pair of numbers.
func intensity(level, modifier string, skipped map[string]int) (model.Dynamic, bool) {
	// A modifier is something written beside the letters, and what its
	// numbers mean is not known here — the sforzandos, which would have
	// been the guess, turn out to be a kind of their own. So the level is
	// written and the modifier is reported rather than invented.
	if len(variantInts(modifier)) > 0 {
		skipped["marks written beside a dynamic, such as poco or molto"]++
	}
	n := variantInts(level)
	if len(n) == 0 {
		return model.DynamicNone, false
	}
	soft := n[0] == 1
	letters := 0
	if len(n) > 1 {
		letters = n[len(n)-1]
	}
	switch {
	case soft && letters >= 4:
		return model.PPPP, true
	case soft && letters == 3:
		return model.PPP, true
	case soft && letters == 2:
		return model.PP, true
	case soft && letters == 1:
		return model.Piano, true
	case soft:
		return model.MP, true
	case letters >= 4:
		return model.FFFF, true
	case letters == 3:
		return model.FFF, true
	case letters == 2:
		return model.FF, true
	case letters == 1:
		return model.Forte, true
	}
	return model.MF, true
}

// forced reads one of the struck dynamics.
//
// Dorico names them, which is a mercy: this file holds nothing but
// kForceDynamicSfz, and a reader that had to guess from a number would
// have had to guess between sf, sfz, fz and rfz — four marks that look
// alike on the page and are four different instructions.
//
// Tried longest first, so that Rfz is not read as fz and Sfz is not read
// as Sf.
func forced(kind string, skipped map[string]int) model.Dynamic {
	switch {
	case strings.Contains(kind, "Sfz"):
		return model.SForzando
	case strings.Contains(kind, "Rfz"), strings.Contains(kind, "Rinf"):
		return model.Rinforzando
	case strings.Contains(kind, "Sfp"), strings.Contains(kind, "Fp"):
		return model.FortePiano
	case strings.Contains(kind, "Sf"), strings.Contains(kind, "Fz"):
		return model.SForzato
	}
	skipped["struck dynamics gscore does not know, called "+kind+" by Dorico"]++
	return model.SForzato
}

// variantInts pulls the numbers out of one of Dorico's packed lists:
//
//	steamVariantList:list: {int: 0}, {}, {bool: false}, {int: 2}
func variantInts(s string) []int {
	var out []int
	for rest := s; ; {
		i := strings.Index(rest, "{int:")
		if i < 0 {
			return out
		}
		rest = rest[i+len("{int:"):]
		j := strings.IndexByte(rest, '}')
		if j < 0 {
			return out
		}
		if n, err := strconv.Atoi(strings.TrimSpace(rest[:j])); err == nil {
			out = append(out, n)
		}
		rest = rest[j+1:]
	}
}

// spot is one place in a part a mark can be hung on: the element that
// starts there, and where that is.
type spot struct {
	at    rat
	marks *model.Marks
}

// attach hangs the dynamics and hairpins on the notes they stand over.
//
// Dorico says where they are, in crotchets from the start of the flow;
// gscore holds them on the element they belong to. So the part is walked
// once, every element's place noted, and each mark put on the last element
// that begins at or before it — a dynamic written over the middle of a long
// note belongs to that note.
func attach(p *model.Part, bars []bar, marks []dynamic, swells []swell) {
	var spots []spot
	for i, m := range p.Measures {
		if i >= len(bars) {
			break
		}
		at := bars[i].at
		if len(m.Voices) == 0 {
			continue
		}
		for _, el := range m.Voices[0].Elements {
			if mk := marksOf(el); mk != nil {
				spots = append(spots, spot{at: at, marks: mk})
			}
			d := el.Duration()
			at = at.add(rat{int64(d.Num) * 4, int64(d.Den)})
		}
	}
	if len(spots) == 0 {
		return
	}
	at := func(when rat) *model.Marks {
		// The last element beginning at or before this moment.
		put := -1
		for i := range spots {
			if spots[i].at.cmp(when) <= 0 {
				put = i
				continue
			}
			break
		}
		if put < 0 {
			put = 0
		}
		return spots[put].marks
	}
	for _, m := range marks {
		at(m.at).Dynamic = m.d
	}
	for _, s := range swells {
		from, to := at(s.at), at(s.at.add(s.dur))
		if from == to {
			from.Hairpin = &model.Hairpin{Kind: s.kind, Alone: true}
			continue
		}
		from.Hairpin = &model.Hairpin{Kind: s.kind, Start: true}
		to.Hairpin = &model.Hairpin{Kind: s.kind, Stop: true}
	}
}

// marksOf gets at an element's marks, whatever kind of element it is.
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

/* ---------- the tempo ---------- */

// Dorico keeps a tempo as microseconds to the beat, which is MIDI's way of
// saying it: 555555 of them is 108 beats a minute. Which beat is a number
// of its own, counting down from a whole note — 10 is a semibreve, 9 a
// minim, 8 a crotchet — with the dots beside it.
//
// Read that way the four movements of this symphony come out at minim 108,
// quaver 92, dotted minim 96 and minim 84, which is Beethoven's own list.
type tempoMark struct {
	at   rat
	bpm  int
	unit model.Duration
	text string
	// back says the mark names no speed of its own: "Tempo I", "a tempo".
	// It carries the words and takes the speed the movement opened at.
	back bool
}

// readTempo gathers a flow's tempo marks.
func readTempo(t *tree, streams *streams, skipped map[string]int) []tempoMark {
	var out []tempoMark
	for _, p := range streams.ofKind("kGlobalTimebaseStream") {
		for _, e := range t.children(t.child(p.node, "events")) {
			if t.nameOf(e) != "ImmediateTempoChangeEventDefinition" {
				continue
			}
			at, ok := parseRat(t.str(e, "position"))
			if !ok {
				continue
			}
			at = at.add(p.at)
			data := t.child(e, "data")
			m := tempoMark{at: at, text: t.str(data, "text")}

			abs := t.child(data, "absoluteTempo")
			value, err := strconv.Atoi(t.str(abs, "tempoValue"))
			if abs < 0 || err != nil || value <= 0 {
				// "Tempo I" and "a tempo" say no number: they send the
				// player back to a speed already given.
				m.back = true
				out = append(out, m)
				continue
			}
			m.bpm = int(math.Round(60000000 / float64(value)))
			m.unit = beatUnit(t.str(abs, "beatUnits"))
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.less(out[j].at) })
	return out
}

// beatUnit reads which note the speed is counted in.
//
// A number counting down from a whole note, with the dots after it:
// {list: {int: 9}, {int: 1}} is a dotted minim. A crotchet is the answer
// for anything unrecognised, which is what nearly every metronome mark is
// counted in anyway.
func beatUnit(packed string) model.Duration {
	n := variantInts(packed)
	if len(n) == 0 {
		return model.Duration{Num: 1, Den: 4}
	}
	shift := 10 - n[0]
	if shift < 0 || shift > 7 {
		return model.Duration{Num: 1, Den: 4}
	}
	d := model.Duration{Num: 1, Den: 1 << shift}
	if len(n) > 1 {
		for i := 0; i < n[1] && i < 3; i++ {
			d = d.Scale(3, 2).Reduced()
		}
	}
	return d
}

// setTempo puts a flow's tempo marks on the score.
//
// The first goes on the score itself, since that is the speed it starts at;
// the rest go on the bar they fall in, which is where gscore keeps a change
// of speed. A mark inside a bar is moved to the head of that bar, which
// moves it by less than a bar and is how every other reader here does it.
func setTempo(s *model.Score, bars []bar, marks []tempoMark) {
	if len(marks) == 0 || len(s.Parts) == 0 {
		return
	}
	opening := model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}}
	for _, m := range marks {
		if !m.back {
			opening = model.Tempo{BPM: m.bpm, Unit: m.unit, Text: m.text}
			break
		}
	}
	for _, m := range marks {
		t := model.Tempo{BPM: m.bpm, Unit: m.unit, Text: m.text}
		if m.back {
			// The words are printed and no metronome mark with them,
			// which is how "Tempo I" is written in every score.
			t.BPM, t.Unit, t.Silent = opening.BPM, opening.Unit, true
		}
		if m.at.zero() {
			s.Tempo = t
			continue
		}
		at := -1
		for i := range bars {
			if bars[i].at.cmp(m.at) <= 0 {
				at = i
				continue
			}
			break
		}
		if at < 0 || at >= len(s.Parts[0].Measures) {
			continue
		}
		mark := t
		s.Parts[0].Measures[at].Tempo = &mark
	}
	if s.Tempo.BPM == 0 {
		s.Tempo = opening
	}
}
