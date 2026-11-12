/*
 * marks.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package dorico

import (
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

// Everything written over the notes that is not a note.
//
// They come off three different streams, which is Dorico saying who each
// one belongs to. A trill is one player's, so it is on a voice. A pizz. is
// the staff's, since it holds until an arco cancels it. A fermata and a
// rehearsal mark belong to the whole system, because the orchestra holds
// together and "from B" has to mean the same bar to everybody in the room.
//
// gscore keeps all of them on elements or on measures, so what is here is
// mostly a matter of finding the note a moment lands on — which attach
// already does for the dynamics.

// pointMark is a mark written at one moment: words, a technique, a pause.
type pointMark struct {
	at   rat
	text string
	// fermata holds the note instead of writing anything.
	fermata bool
}

// spanMark is a mark over a stretch: a trill, an octave line.
type spanMark struct {
	at   rat
	dur  rat
	kind string // "trill" or "octave"
	// amount is how many octaves an octave line shifts, up or down.
	amount int
}

// readTextMark reads the words out of a text event.
//
// A Dorico text is a document — paragraphs of fragments, each with its own
// styling — because somebody may have written a sentence in three fonts.
// What gscore keeps is the words, so the fragments are run together.
func readTextMark(t *tree, e int32) string {
	var out []string
	for _, para := range t.children(t.path(e, "content", "paragraphs")) {
		for _, frag := range t.children(t.child(para, "fragments")) {
			if s := t.str(frag, "content"); s != "" {
				out = append(out, s)
			}
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// techniqueNamed is what one of Dorico's playing techniques is called.
//
// The names are in its library as glyphs and texts; what is wanted here is
// the word a player reads, and for the ones that matter the id says it.
// Anything else is reported rather than written as its own id, which would
// put "pta.something" on the page.
func techniqueNamed(id string) (string, bool) {
	switch {
	case strings.Contains(id, "pizz"):
		return "pizz.", true
	case strings.Contains(id, "arco"):
		return "arco", true
	case strings.Contains(id, "mute"), strings.Contains(id, "conSord"):
		return "con sord.", true
	case strings.Contains(id, "senzaSord"), strings.Contains(id, "open"):
		return "senza sord.", true
	case strings.Contains(id, "tremolo"):
		return "trem.", true
	case strings.Contains(id, "naturale"):
		return "nat.", true
	}
	return "", false
}

// octaveAmount reads how far an octave line shifts, from its id:
// "line.ottava.up.8" is one octave up, "line.ottava.down.15" is two down.
func octaveAmount(id string) (int, bool) {
	n := 1
	switch {
	case strings.Contains(id, ".15"):
		n = 2
	case strings.Contains(id, ".22"):
		n = 3
	case strings.Contains(id, ".8"):
		n = 1
	default:
		return 0, false
	}
	if strings.Contains(id, "down") {
		n = -n
	}
	return n, true
}

// global is everything the whole system shares: the pauses, the rehearsal
// marks, the repeat brackets and the words over the top of the score.
type global struct {
	fermatas  []rat
	rehearsal []rat
	texts     []pointMark
	endings   []ending
	signs     []signAt
}

type ending struct {
	at  rat
	dur rat
	// segments are the lengths of each time through, so a volta with a
	// first and a second ending has two of them.
	segments []rat
}

type signAt struct {
	at   rat
	sign model.Sign
}

// readGlobal gathers what belongs to the whole system.
func readGlobal(t *tree, streams *streams, skipped map[string]int) global {
	var g global
	for _, p := range streams.ofKind("kGlobalTimebaseStream") {
		for _, e := range t.children(t.child(p.node, "events")) {
			at, ok := parseRat(t.str(e, "position"))
			if !ok {
				continue
			}
			at = at.add(p.at)
			switch t.nameOf(e) {
			case "PauseEventDefinition":
				// Only the fermata. The others — a caesura, a breath mark
				// — are a different instruction and gscore has no place
				// for them yet.
				if kind := t.str(t.child(e, "pauseData"), "pauseType"); kind != "kFermata" {
					skipped["pauses that are not fermatas"]++
					continue
				}
				g.fermatas = append(g.fermatas, at)
			case "RehearsalMarkEventDefinition":
				g.rehearsal = append(g.rehearsal, at)
			case "GradualTempoChangeEventDefinition":
				// The words are written over the music. The speed is not
				// changed: gscore holds a tempo as a number at a bar and
				// has no way to say "and slower from here to there", so
				// the playback runs on at the tempo in force.
				words := t.str(t.child(e, "data"), "text")
				if words == "" {
					words = "rit."
				}
				g.texts = append(g.texts, pointMark{at: at, text: words})
				skipped["gradual tempo changes, written as words and not "+
					"played as a slowing"]++
			case "RepeatEndingLineEventDefinition":
				dur, _ := parseRat(t.str(e, "duration"))
				g.endings = append(g.endings, ending{
					at: at, dur: dur, segments: endingSegments(t, e),
				})
			case "RepeatMarkerEventDefinition":
				sign := model.NoSign
				switch t.str(e, "sectionType") {
				case "kFine":
					sign = model.Fine
				case "kSegno":
					sign = model.Segno
				case "kCoda":
					sign = model.Coda
				}
				if sign == model.NoSign {
					skipped["repeat marks gscore does not know"]++
					continue
				}
				g.signs = append(g.signs, signAt{at: at, sign: sign})
			}
		}
	}
	return g
}

// endingSegments reads how long each time through a volta lasts.
//
// They are in the event's property table rather than beside it, one entry
// per segment: a first-time bar of two crotchets and a second of three.
func endingSegments(t *tree, e int32) []rat {
	var out []rat
	for _, entry := range t.children(t.path(e, "propertyTable", "entries")) {
		if t.str(entry, "propertyType") != "kRepeatEndingLineSegmentDuration" {
			continue
		}
		// Written as a typed value: `string: "2"`.
		v := t.str(entry, "value")
		if i := strings.IndexByte(v, '"'); i >= 0 {
			v = v[i+1:]
			if j := strings.IndexByte(v, '"'); j >= 0 {
				v = v[:j]
			}
		}
		if r, ok := parseRat(v); ok && !r.zero() {
			out = append(out, r)
		}
	}
	return out
}

// putGlobal writes the system's marks onto the score.
func putGlobal(s *model.Score, bars []bar, g global, skipped map[string]int) {
	if len(s.Parts) == 0 {
		return
	}
	// A fermata is held by everybody, so it goes on every staff.
	for _, at := range g.fermatas {
		for _, p := range s.Parts {
			if mk := markAt(p, bars, at); mk != nil {
				mk.Articulations = append(mk.Articulations, model.Fermata)
			}
		}
	}
	// The words go over the top staff, which is where a conductor's
	// instruction is printed.
	for _, w := range g.texts {
		if mk := markAt(s.Parts[0], bars, w.at); mk != nil {
			mk.Texts = append(mk.Texts, model.TextMark{Text: w.text})
		}
	}
	for _, at := range g.rehearsal {
		if i := barAt(bars, at); i >= 0 && i < len(s.Parts[0].Measures) {
			// Counted along with the others rather than written out, so
			// that putting one in later renumbers the rest. See
			// Measure.MarkerAuto.
			s.Parts[0].Measures[i].MarkerAuto = true
		}
	}
	for _, sg := range g.signs {
		if i := barAt(bars, sg.at); i >= 0 && i < len(s.Parts[0].Measures) {
			s.Parts[0].Measures[i].Sign = sg.sign
		}
	}
	for _, e := range g.endings {
		putEnding(s, bars, e, skipped)
	}
}

// putEnding draws a volta: the first-time and second-time bars of a repeat.
func putEnding(s *model.Score, bars []bar, e ending, skipped map[string]int) {
	segs := e.segments
	if len(segs) == 0 {
		segs = []rat{e.dur}
	}
	at := e.at
	for n, seg := range segs {
		from, to := barAt(bars, at), barAt(bars, at.add(seg).sub(rat{1, 1000}))
		if from < 0 || to < from {
			skipped["repeat endings over bars that are not there"]++
			at = at.add(seg)
			continue
		}
		// Every part carries it, as MusicXML stores it, and it is drawn
		// once above the system.
		for _, p := range s.Parts {
			if to >= len(p.Measures) {
				continue
			}
			for i := from; i <= to; i++ {
				p.Measures[i].Ending = &model.Ending{
					Numbers: []int{n + 1},
					Start:   i == from,
					Stop:    i == to,
					// The last time through has nothing to go back for.
					Open: n == len(segs)-1 && i == to,
				}
			}
		}
		at = at.add(seg)
	}
}

// barAt is the bar a moment falls in.
func barAt(bars []bar, at rat) int {
	put := -1
	for i := range bars {
		if bars[i].at.cmp(at) <= 0 {
			put = i
			continue
		}
		break
	}
	return put
}

// markAt finds the marks of the element a moment lands on, in a part's
// first voice.
func markAt(p *model.Part, bars []bar, at rat) *model.Marks {
	i := barAt(bars, at)
	if i < 0 || i >= len(p.Measures) || len(p.Measures[i].Voices) == 0 {
		return nil
	}
	where := bars[i].at
	var found *model.Marks
	for _, el := range p.Measures[i].Voices[0].Elements {
		if where.cmp(at) > 0 {
			break
		}
		if mk := marksOf(el); mk != nil {
			found = mk
		}
		d := el.Duration()
		where = where.add(rat{int64(d.Num) * 4, int64(d.Den)})
	}
	return found
}

// putMarks writes a staff's own marks onto its music: the words, the
// techniques, the trills and the octave lines.
func putMarks(p *model.Part, bars []bar, points []pointMark,
	spans []spanMark, skipped map[string]int) {

	for _, w := range points {
		mk := markAt(p, bars, w.at)
		if mk == nil {
			skipped["words written where there is no note"]++
			continue
		}
		if w.fermata {
			mk.Articulations = append(mk.Articulations, model.Fermata)
			continue
		}
		mk.Texts = append(mk.Texts, model.TextMark{Text: w.text})
	}

	for _, sp := range spans {
		from := markAt(p, bars, sp.at)
		to := markAt(p, bars, sp.at.add(sp.dur))
		if from == nil || to == nil {
			skipped["marks over notes that are not there"]++
			continue
		}
		switch sp.kind {
		case "trill":
			// A trill on one note carries both ends, which is how "tr"
			// over a single note is written.
			if from == to {
				from.Trill = &model.Trill{Start: true, Stop: true}
				continue
			}
			from.Trill = &model.Trill{Start: true}
			to.Trill = &model.Trill{Stop: true}
		case "octave":
			// The notes under the line are written where they are drawn
			// and sound where the line says. Dorico stores what sounds,
			// so they are already at the sounding pitch: the bracket is
			// drawn and nothing is shifted again.
			if from == to {
				from.Octave = &model.OctaveShift{Amount: sp.amount,
					Start: true, Stop: true, SoundsAsWritten: true}
				continue
			}
			from.Octave = &model.OctaveShift{Amount: sp.amount,
				Start: true, SoundsAsWritten: true}
			to.Octave = &model.OctaveShift{Amount: sp.amount,
				Stop: true, SoundsAsWritten: true}
		}
	}
}

// barRepeat is the sign that says "play the bar before again".
//
// It belongs to one staff and not to the score: a drummer repeating a bar
// while the horns play something new is the ordinary case, and in fact
// most of what the sign is for.
type barRepeat struct {
	at   rat
	dur  rat
	bars int
}

// staffKey is a key signature for one staff alone — a horn left in C while
// the orchestra modulates round it.
type staffKey struct {
	at  rat
	key model.KeySignature
}

// putStaffMarks writes what belongs to this staff and no other.
func putStaffMarks(p *model.Part, bars []bar, repeats []barRepeat,
	keys []staffKey) {

	for _, r := range repeats {
		from := barAt(bars, r.at)
		to := barAt(bars, r.at.add(r.dur).sub(rat{1, 1000}))
		if from < 0 {
			continue
		}
		for i := from; i <= to && i < len(p.Measures); i++ {
			p.Measures[i].Repeat = r.bars
		}
	}
	for _, k := range keys {
		if i := barAt(bars, k.at); i >= 0 && i < len(p.Measures) {
			key := k.key
			p.Measures[i].Key = &key
		}
	}
}

// cue is a few bars of somebody else's line, to be printed small in this
// staff's part so the player can hear their entry coming.
//
// Dorico stores it as a span and a staff to take it from, and fills in the
// notes when it draws — the music is already on the other staff, and
// keeping a copy here would be a copy to keep in step. gscore does the same
// thing, in model.Cue, so the reading is a matter of saying which bars of
// whose line rather than of copying any music.
type cue struct {
	at   rat
	dur  rat
	from string // the staveID the notes come from
}

// readCue reads one cue event.
func readCue(t *tree, e int32, at rat) (cue, bool) {
	c := cue{at: at}
	d, _ := parseRat(t.str(e, "duration"))
	c.dur = d
	for _, entry := range t.children(t.path(e, "propertyTable", "entries")) {
		if t.str(entry, "propertyType") != "kCuedStave" {
			continue
		}
		// Written as a typed value: `int: 125`.
		v := t.str(entry, "value")
		if i := strings.LastIndexByte(v, ' '); i >= 0 {
			v = v[i+1:]
		}
		c.from = strings.TrimSpace(v)
	}
	return c, c.from != "" && !c.dur.zero()
}

// putCues gives each part the cues written into it.
//
// A cue is kept on the part and not on the score, which is where it
// belongs: the other player's line already stands on its own staff at full
// size, and printing it again in the middle of the score would be printing
// the same music twice. gscore draws it when the part is made — see
// writeCue in the editor — so what is written here is which bars of whose
// line, and that is all.
func putCues(s *model.Score, bars []bar, staves []*stave,
	skipped map[string]int) {

	// Which part each of Dorico's staves became.
	byStave := map[string]int{}
	for i, st := range staves {
		if i < len(s.Parts) && st.stave != "" {
			byStave[st.stave] = i
		}
	}
	for i, st := range staves {
		if i >= len(s.Parts) || len(st.cues) == 0 {
			continue
		}
		var mine []model.Cue
		for _, c := range st.cues {
			from, ok := byStave[c.from]
			if !ok || from == i {
				skipped["cues taken from a staff that is not in this flow"]++
				continue
			}
			at := barAt(bars, c.at)
			to := barAt(bars, c.at.add(c.dur).sub(rat{1, 1000}))
			if at < 0 || to < at {
				skipped["cues over bars that are not there"]++
				continue
			}
			mine = append(mine, model.Cue{
				At: at, Bars: to - at + 1, From: from,
			})
		}
		if len(mine) == 0 {
			continue
		}
		// The part to print them in. A score read from a file has no
		// parts made yet, so the first cue makes one: a sheet of this
		// staff alone, which is what a part is.
		s.Sheets = append(s.Sheets, model.Sheet{
			Name:   s.Parts[i].Name,
			Staves: []int{i},
			Cues:   mine,
		})
	}
}
