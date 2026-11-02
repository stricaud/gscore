/*
 * sheets.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The parts.
 *
 * A score is read by the person in front of it and played from by everybody
 * else, and what everybody else needs is not the score: it is their own
 * line, engraved large enough to read at a stand, with the bars they do not
 * play counted rather than printed, and with the page turns where they have
 * time to turn.
 *
 * A part here is a view, the way concert pitch is a view. It holds no music
 * — only which staves it prints and how it wants them laid out — and what
 * it shows is the score's own notes. That is the whole design, and it is
 * the answer to the one thing that goes wrong with parts everywhere else:
 * a score and a part that come to disagree, because somebody fixed a note
 * in one of them. There is nothing to keep in step because there is one
 * copy of the music.
 *
 * Which part is on the screen belongs to the window and not to the file:
 * two people looking at the same score over a shared server are not
 * necessarily looking at the same part, and a score reopened tomorrow
 * should show the score. What is in the file is which parts exist and what
 * each of them wants, which is work somebody did and would not want to do
 * again.
 */

// sheetCache holds the copy of the score that is one part, made when it is
// asked for and kept until the music changes under it.
//
// The same discipline as the concert-pitch copy and for the same reason: the
// layout cache behind it keys on the score it is handed, so a fresh clone on
// every draw would engrave the whole piece again at every scroll.
type sheetCache struct {
	mu    sync.Mutex
	of    *model.Score // what it was made from
	gen   uint64       // and when
	which int          // and which part it is
	score *model.Score
}

// asSheet is the score as one of its parts, where a part is being shown,
// and the score itself where it is not.
func (a *App) asSheet(s *model.Score) *model.Score {
	if a.sheet < 0 || s == nil || a.sheet >= len(s.Sheets) {
		return s
	}
	gen := a.gen.Load()
	a.sheet0.mu.Lock()
	defer a.sheet0.mu.Unlock()
	if a.sheet0.score != nil && a.sheet0.of == s &&
		a.sheet0.gen == gen && a.sheet0.which == a.sheet {
		return a.sheet0.score
	}
	out := sheetOf(s, a.sheet)
	a.sheet0.of, a.sheet0.gen = s, gen
	a.sheet0.which, a.sheet0.score = a.sheet, out
	return out
}

// sheetOf is a copy of a score showing one part: its staves and nobody
// else's, laid out the way that part asked for.
//
// The staves it does not print are hidden rather than removed. The engraver
// already drops hidden staves and already remembers which staff of the
// score each drawn one is, so a note clicked on a part goes into the staff
// the score calls that — which is the whole of what makes a part editable
// rather than a picture.
func sheetOf(s *model.Score, which int) *model.Score {
	if which < 0 || which >= len(s.Sheets) {
		return s
	}
	sh := s.Sheets[which]
	out := s.Clone()

	on := map[int]bool{}
	for _, i := range sh.Staves {
		on[i] = true
	}
	for i, p := range out.Parts {
		p.Hidden = !on[i]
	}

	// What the part wants done differently, over what the score wanted.
	// Over rather than instead of: a part that asks for a larger staff is
	// asking for one change to this score's house style and not for the
	// program's defaults back.
	if len(sh.Engraving) > 0 {
		if out.Engraving == nil {
			out.Engraving = map[string]float64{}
		}
		for k, v := range sh.Engraving {
			out.Engraving[k] = v
		}
	}
	out.Numbering = sh.Numbering

	// Its own page turns, in place of the score's. A part with no breaks of
	// its own is laid out as the bars fall, which is right: the score's
	// turns were put where the score's fuller systems ran out of room and
	// mean nothing on a page with a quarter of the staves.
	for _, p := range out.Parts {
		for _, m := range p.Measures {
			m.SystemBreak = false
		}
	}
	for _, at := range sh.Breaks {
		for _, p := range out.Parts {
			if at >= 0 && at < len(p.Measures) {
				p.Measures[at].SystemBreak = true
			}
		}
	}

	for _, cue := range sh.Cues {
		writeCue(out, sh, cue, s)
	}
	return out
}

// barIsSilent reports whether a bar has nothing in it but rests, which is
// where a cue may go.
func barIsSilent(m *model.Measure) bool {
	if m == nil {
		return true
	}
	for _, v := range m.Voices {
		for _, el := range v.Elements {
			if _, ok := el.(*model.Rest); !ok {
				return false
			}
		}
	}
	return true
}

// writeCue puts a few bars of another staff's line into this part, small.
//
// Into the copy the part is drawn from, never into the score: what is
// stored is "show me bars 38 to 41 of the oboe", and the notes are fetched
// from the oboe every time the part is drawn. A cue whose notes had been
// copied into the part would go stale the first time somebody edited the
// line it came from, and would be wrong in the one place nobody thinks to
// look.
//
// from is the score the music comes from — the real one, not the copy —
// since the copy's own staves have been hidden and its measures are about
// to be written into.
func writeCue(out *model.Score, sh model.Sheet, cue model.Cue, from *model.Score) {
	if cue.Bars <= 0 || cue.From < 0 || cue.From >= len(from.Parts) {
		return
	}
	// Onto the part's own first staff, which is the one the player reads. A
	// cue on the second staff of a piano part would be under the music
	// rather than in the player's eye.
	if len(sh.Staves) == 0 || sh.Staves[0] >= len(out.Parts) {
		return
	}
	onto := out.Parts[sh.Staves[0]]
	src := from.Parts[cue.From]

	label := cue.Label
	if label == "" {
		label = staffName(from, cue.From)
	}

	first := true
	for i := 0; i < cue.Bars; i++ {
		at := cue.At + i
		if at < 0 || at >= len(onto.Measures) || at >= len(src.Measures) {
			break
		}
		// Not over music of your own. A cue is for a bar you are not
		// playing, and printing one over a bar you are would hide the thing
		// you are holding the page for. A cue that runs into your entry
		// simply stops there, which is where it was going to stop anyway.
		if !barIsSilent(onto.Measures[at]) {
			continue
		}
		// The bar as the other player has it, with every note of it marked
		// as not yours to play.
		bar := src.Measures[at].Clone()
		for _, v := range bar.Voices {
			for _, el := range v.Elements {
				if m := marksOn(el); m != nil {
					m.Small = true
					// A cue is not the place for the other part's dynamics,
					// lyrics or chord symbols: they belong to whoever is
					// playing it, and printed here they read as
					// instructions to the person holding this page.
					m.Dynamic = model.DynamicNone
					m.Lyrics, m.Texts, m.Chord = nil, nil, nil
					m.Hairpin = nil
				}
			}
		}
		// Its own bar keeps what the bar is: the key, the meter, the
		// barline and anything written on it are this part's, not the
		// staff the notes were borrowed from.
		mine := onto.Measures[at]
		mine.Voices = bar.Voices

		if first {
			first = false
			// Whose line it is, over the first bar of it. Without the name
			// a cue is a few notes that appear from nowhere, which is worse
			// than no cue: the player does not know who to listen for.
			for _, v := range mine.Voices {
				if len(v.Elements) == 0 {
					continue
				}
				if m := marksOn(v.Elements[0]); m != nil {
					m.Texts = append(m.Texts, model.TextMark{
						Text:      label,
						Placement: model.PlacementAbove,
						Italic:    true,
					})
				}
				break
			}
		}
	}
}

// makeSheets writes one part for every player, where the score has none.
//
// One per staff, except where a group of staves is braced with its barlines
// run through the gap — which is what says "one player reads both of these"
// and is exactly a piano, an organ or a harp. Two staves beside each other
// without that are two players and two parts.
//
// Named after the staff, because that is what the staff is called and
// somebody has already named it.
func makeSheets(s *model.Score) []model.Sheet {
	taken := map[int]bool{}
	var out []model.Sheet

	for _, g := range s.Groups {
		if !g.Barlines || g.First < 0 || g.Last >= len(s.Parts) || g.Last <= g.First {
			continue
		}
		var staves []int
		for i := g.First; i <= g.Last; i++ {
			staves = append(staves, i)
			taken[i] = true
		}
		name := g.Name
		if name == "" {
			name = staffName(s, g.First)
		}
		out = append(out, newSheet(name, staves))
	}

	for i := range s.Parts {
		if taken[i] {
			continue
		}
		out = append(out, newSheet(staffName(s, i), []int{i}))
	}

	// In the order the staves stand in the score, which is score order:
	// flutes before oboes before strings, because somebody already put them
	// in that order and it is the order every printed set is in.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Staves[0] < out[i].Staves[0] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// newSheet is a part with the defaults a part wants: its rests counted, and
// a number on every bar often enough to find a place from.
func newSheet(name string, staves []int) model.Sheet {
	return model.Sheet{
		Name:       name,
		Staves:     staves,
		MultiRests: true,
		// Larger than the score, because it is read at arm's length from a
		// stand rather than at a foot and a half from a desk.
		StaffSpace: 10,
		// Every five bars, boxed. A player who has just counted thirty-two
		// bars of rest wants to be told where they have got to, and a
		// rehearsal that begins "from bar ninety" wants the nineties
		// findable without counting.
		Numbering: model.MeasureNumbering{Every: 5},
	}
}

func staffName(s *model.Score, i int) string {
	if i < 0 || i >= len(s.Parts) {
		return "Part " + strconv.Itoa(i+1)
	}
	if n := strings.TrimSpace(s.Parts[i].Name); n != "" {
		return n
	}
	// The instrument, where the staff was never named at all — a part has
	// to be called something, and what it plays is the best guess there is.
	if n := s.Parts[i].Instrument.String(); n != "" {
		return capitalised(n)
	}
	return "Staff " + strconv.Itoa(i+1)
}

// capitalised is a name written the way a name is written, for the ones
// gscore makes up itself out of a table.
//
// Only the first letter, and only when it is a letter: "flute" becomes
// "Flute" and "2nd violin" is left as somebody wrote it.
func capitalised(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// staffList reads "1,3" as the staves it names, counted from one the way
// the page is numbered.
//
// A part with no staves in it is not a part: it would print a page of
// nothing and would be impossible to tell from a part whose staves had all
// been hidden, so it is refused rather than allowed and puzzled over.
func staffList(raw string, have int) ([]int, error) {
	var out []int
	seen := map[int]bool{}
	for _, word := range strings.Split(raw, ",") {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		n, err := strconv.Atoi(word)
		if err != nil || n < 1 || n > have {
			return nil, &barError{word}
		}
		if seen[n-1] {
			continue
		}
		seen[n-1] = true
		out = append(out, n-1)
	}
	if len(out) == 0 {
		return nil, errNoStaves
	}
	sort.Ints(out)
	return out, nil
}

// errNoStaves is a part with nothing on it.
var errNoStaves = &noStaves{}

type noStaves struct{}

func (*noStaves) Error() string { return "a part needs a staff on it" }

// sheetInfo is one part as the editor is told about it.
type sheetInfo struct {
	Name   string `json:"name"`
	Staves []int  `json:"staves"`
	// Showing is whether this is the one on the screen.
	Showing    bool      `json:"showing"`
	MultiRests bool      `json:"multiRests"`
	Breaks     int       `json:"breaks"`
	StaffSpace float64   `json:"staffSpace,omitempty"`
	Cues       []cueInfo `json:"cues,omitempty"`
}

// cueInfo is one cue as the editor is told about it: which bars of whose
// line, and what is written over it.
type cueInfo struct {
	At    int    `json:"at"`
	Bars  int    `json:"bars"`
	From  int    `json:"from"`
	Label string `json:"label"`
}

// handleSheets lists the parts, makes them, or shows one.
func (a *App) handleSheets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.sheetsSet(w, r)
		return
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	a.writeSheets(w)
}

func (a *App) sheetsSet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Which part to show is not a change to the score, so it does not take
	// the lock that records an edit and does not make the file unsaved.
	if v, ok := q["show"]; ok && len(v) > 0 {
		a.mu.Lock()
		defer a.mu.Unlock()
		// "score" is the whole thing, which is what a part is not.
		if v[0] == "score" || v[0] == "" || v[0] == "-1" {
			a.sheet = -1
			a.writeSheets(w)
			return
		}
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 0 || n >= len(a.score.Sheets) {
			http.Error(w, "there is no part "+v[0], http.StatusBadRequest)
			return
		}
		a.sheet = n
		a.writeSheets(w)
		return
	}

	a.lock()
	defer a.mu.Unlock()

	if q.Get("make") == "1" {
		// Made once. Running it again on a score that has parts would throw
		// away whatever was named, re-laid-out or re-sized in them, which is
		// the work this was meant to save.
		if len(a.score.Sheets) > 0 {
			http.Error(w, "this score already has parts", http.StatusBadRequest)
			return
		}
		a.score.Sheets = makeSheets(a.score)
		a.writeSheets(w)
		return
	}

	// Renaming one, or changing what it wants.
	at := -1
	if v := q.Get("at"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(a.score.Sheets) {
			http.Error(w, "there is no part "+v, http.StatusBadRequest)
			return
		}
		at = n
	}
	if at < 0 {
		http.Error(w, "which part?", http.StatusBadRequest)
		return
	}
	sh := &a.score.Sheets[at]
	if v, ok := q["name"]; ok && len(v) > 0 {
		name := strings.TrimSpace(v[0])
		if name == "" {
			http.Error(w, "a part needs a name", http.StatusBadRequest)
			return
		}
		sh.Name = name
	}
	if v, ok := q["multirest"]; ok && len(v) > 0 {
		sh.MultiRests = v[0] == "1" || v[0] == "true"
	}
	// How large it is drawn. Not an engraving measurement — nothing about
	// the layout changes — but the one setting every printed part changes,
	// being read at arm's length rather than at a desk.
	if v, ok := q["size"]; ok && len(v) > 0 {
		if v[0] == "" {
			sh.StaffSpace = 0
		} else {
			size, err := strconv.ParseFloat(v[0], 64)
			// Four is about as small as a staff can be read at all and
			// twenty-four about as large as a page can hold. Outside that
			// is a typing mistake rather than a decision.
			if err != nil || size < 4 || size > 24 {
				http.Error(w, v[0]+" is not a staff size — 4 to 24",
					http.StatusBadRequest)
				return
			}
			sh.StaffSpace = size
		}
	}
	// Which staves it prints. A part is usually one staff and occasionally
	// several, and which is a thing somebody changes: a divisi written on
	// two staves goes onto one stand.
	if v, ok := q["staves"]; ok && len(v) > 0 {
		staves, err := staffList(v[0], len(a.score.Parts))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sh.Staves = staves
	}
	// A cue: a few bars of somebody else's line, printed small so the
	// player can hear their entry coming.
	if v, ok := q["cue"]; ok && len(v) > 0 {
		from, err := strconv.Atoi(v[0])
		if err != nil || from < 0 || from >= len(a.score.Parts) {
			http.Error(w, "there is no staff "+v[0], http.StatusBadRequest)
			return
		}
		// at already names the part, so the cue's own first bar is "bar".
		at, bars := 0, 2
		if raw := q.Get("bar"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 || n >= a.score.MeasureCount() {
				http.Error(w, "there is no bar "+raw, http.StatusBadRequest)
				return
			}
			at = n
		}
		if raw := q.Get("bars"); raw != "" {
			n, err := strconv.Atoi(raw)
			// More than a handful is not a cue, it is the other part
			// printed in this one: the point is a few bars to listen for.
			if err != nil || n < 1 || n > 16 {
				http.Error(w, raw+" is not a number of bars — 1 to 16",
					http.StatusBadRequest)
				return
			}
			bars = n
		}
		// Not from a staff this part already prints: the player can see
		// their own line, and a cue of it would be the same bars twice.
		for _, own := range sh.Staves {
			if own == from {
				http.Error(w, "that is a staff this part already prints",
					http.StatusBadRequest)
				return
			}
		}
		// A cue has to have somewhere to go. Told now rather than drawn as
		// nothing: somebody who asks for a cue and sees no change on the
		// page has no way to know which of the two happened.
		room := false
		if len(sh.Staves) > 0 && sh.Staves[0] < len(a.score.Parts) {
			mine := a.score.Parts[sh.Staves[0]].Measures
			for i := 0; i < bars; i++ {
				if n := at + i; n < len(mine) && barIsSilent(mine[n]) {
					room = true
					break
				}
			}
		}
		if !room {
			http.Error(w, "this part is playing in those bars, so there is "+
				"nowhere to print a cue", http.StatusBadRequest)
			return
		}
		sh.Cues = append(sh.Cues, model.Cue{
			At: at, Bars: bars, From: from, Label: strings.TrimSpace(q.Get("label")),
		})
		a.writeSheets(w)
		return
	}
	if v, ok := q["uncue"]; ok && len(v) > 0 {
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 0 || n >= len(sh.Cues) {
			http.Error(w, "there is no cue "+v[0], http.StatusBadRequest)
			return
		}
		sh.Cues = append(sh.Cues[:n], sh.Cues[n+1:]...)
		a.writeSheets(w)
		return
	}

	if q.Get("remove") == "1" {
		a.score.Sheets = append(a.score.Sheets[:at], a.score.Sheets[at+1:]...)
		if a.sheet >= len(a.score.Sheets) {
			a.sheet = -1
		}
	}
	a.writeSheets(w)
}

// cuesOf is a part's cues, each with the name that is actually written over
// it — the staff's own where the cue did not say, which is what it almost
// always is.
func cuesOf(s *model.Score, sh model.Sheet) []cueInfo {
	var out []cueInfo
	for _, c := range sh.Cues {
		label := c.Label
		if label == "" {
			label = staffName(s, c.From)
		}
		out = append(out, cueInfo{At: c.At, Bars: c.Bars, From: c.From, Label: label})
	}
	return out
}

// writeSheets sends the list. The caller holds the lock.
func (a *App) writeSheets(w http.ResponseWriter) {
	out := []sheetInfo{}
	for i, sh := range a.score.Sheets {
		out = append(out, sheetInfo{
			Name: sh.Name, Staves: append([]int(nil), sh.Staves...),
			Showing: i == a.sheet, MultiRests: sh.MultiRests,
			Breaks: len(sh.Breaks), StaffSpace: sh.StaffSpace,
			Cues: cuesOf(a.score, sh),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"sheets":  out,
		"showing": a.sheet,
	})
}
