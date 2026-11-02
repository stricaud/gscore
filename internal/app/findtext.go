/*
 * findtext.go
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
	"strings"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Finding words in a score, and changing them.
 *
 * Words are scattered through a score in a dozen unrelated places — the
 * title block, the staff names, the lyrics under every note, the chord
 * symbols over them, the rehearsal marks, the movement titles, the word
 * beside a metronome mark, and every "rit." or "solo" somebody has
 * written. Renaming an instrument or fixing a spelling meant finding each
 * one by eye and clicking it.
 *
 * One walk of the score serves both halves. Searching and replacing that
 * disagree about where the words are is a replace that misses what the
 * search found, so there is one list of places and both read it.
 */

// TextKinds are the kinds of writing a search can turn up, in the words
// the editor shows beside each match.
//
// A list rather than literals scattered through eachText, because these
// reach the screen: the window writes them beside every match, so they
// are words to be translated, and the test that checks nothing on screen
// is left in English reads them from here. A kind named in place would be
// a word nobody could find to translate.
var TextKinds = struct {
	Title, Subtitle, Composer, Copyright string
	Tempo, Group, Staff                  string
	Marker, Movement                     string
	Lyric, Text                          string
	Root, Quality, Bass, Symbol          string
}{
	Title: "Title", Subtitle: "Subtitle", Composer: "Composer",
	Copyright: "Copyright", Tempo: "Tempo", Group: "Group name",
	Staff: "Staff name", Marker: "Rehearsal mark",
	Movement: "Movement title", Lyric: "Lyric", Text: "Text",
	Root: "Chord root", Quality: "Chord quality", Bass: "Chord bass",
	Symbol: "Chord symbol",
}

// AllTextKinds is the same list to read through, for the test that checks
// every word on screen has a translation.
var AllTextKinds = []string{
	TextKinds.Title, TextKinds.Subtitle, TextKinds.Composer,
	TextKinds.Copyright, TextKinds.Tempo, TextKinds.Group, TextKinds.Staff,
	TextKinds.Marker, TextKinds.Movement, TextKinds.Lyric, TextKinds.Text,
	TextKinds.Root, TextKinds.Quality, TextKinds.Bass, TextKinds.Symbol,
}

// textSpot is one piece of writing in a score, and how to change it.
type textSpot struct {
	// What kind of writing it is, in the words the editor shows.
	Where string
	// Where it is, for the editor to go to. Measure is -1 for the things
	// that belong to the whole score rather than to a bar.
	Part    int
	Measure int
	// Verse numbers a lyric, so that two verses on one note are two spots.
	Verse int

	get func() string
	set func(string)
}

// eachText walks every piece of writing in a score.
//
// The order is the order somebody reads them in: the title block first,
// then the staff names, then the music from the beginning. A list of
// matches in any other order is a list nobody can follow.
func eachText(s *model.Score, fn func(textSpot)) {
	if s == nil {
		return
	}
	id := &s.Identity
	for _, f := range []struct {
		where string
		at    *string
	}{
		{TextKinds.Title, &id.Title},
		{TextKinds.Subtitle, &id.Subtitle},
		{TextKinds.Composer, &id.Composer},
		{TextKinds.Copyright, &id.Copyright},
	} {
		at := f.at
		fn(textSpot{Where: f.where, Part: -1, Measure: -1,
			get: func() string { return *at }, set: func(v string) { *at = v }})
	}
	if s.Tempo.Text != "" {
		fn(textSpot{Where: TextKinds.Tempo, Part: -1, Measure: -1,
			get: func() string { return s.Tempo.Text },
			set: func(v string) { s.Tempo.Text = v }})
	}
	for i := range s.Groups {
		g := &s.Groups[i]
		fn(textSpot{Where: TextKinds.Group, Part: -1, Measure: -1,
			get: func() string { return g.Name },
			set: func(v string) { g.Name = v }})
	}

	for pi, p := range s.Parts {
		part := p
		fn(textSpot{Where: TextKinds.Staff, Part: pi, Measure: -1,
			get: func() string { return part.Name },
			set: func(v string) { part.Name = v }})

		for mi, m := range p.Measures {
			meas := m
			fn(textSpot{Where: TextKinds.Marker, Part: pi, Measure: mi,
				get: func() string { return meas.Marker },
				set: func(v string) { meas.Marker = v }})
			fn(textSpot{Where: TextKinds.Movement, Part: pi, Measure: mi,
				get: func() string { return meas.SectionTitle },
				set: func(v string) { meas.SectionTitle = v }})
			if meas.Tempo != nil {
				tp := meas.Tempo
				fn(textSpot{Where: TextKinds.Tempo, Part: pi, Measure: mi,
					get: func() string { return tp.Text },
					set: func(v string) { tp.Text = v }})
			}

			for _, v := range m.Voices {
				for _, el := range v.Elements {
					marks := marksOn(el)
					if marks == nil {
						continue
					}
					for li := range marks.Lyrics {
						ly := &marks.Lyrics[li]
						fn(textSpot{Where: TextKinds.Lyric, Part: pi, Measure: mi,
							Verse: ly.Verse,
							get:   func() string { return ly.Text },
							set:   func(s string) { ly.Text = s }})
					}
					for ti := range marks.Texts {
						tm := &marks.Texts[ti]
						fn(textSpot{Where: TextKinds.Text, Part: pi, Measure: mi,
							get: func() string { return tm.Text },
							set: func(s string) { tm.Text = s }})
					}
					if c := marks.Chord; c != nil {
						// Each part of a chord symbol on its own, so that
						// replacing "m7" in a quality cannot reach into a
						// root that happens to spell the same letters.
						for _, f := range []struct {
							where string
							at    *string
						}{
							{TextKinds.Root, &c.Root},
							{TextKinds.Quality, &c.Quality},
							{TextKinds.Bass, &c.Bass},
							{TextKinds.Symbol, &c.Text},
						} {
							at := f.at
							fn(textSpot{Where: f.where, Part: pi, Measure: mi,
								get: func() string { return *at },
								set: func(v string) { *at = v }})
						}
					}
				}
			}
		}
	}
}

// marksOn is the marks an element carries, where it is the kind that
// carries any. The model keeps its own copy of this unexported, and a
// type switch here is shorter than a reason to export it.
func marksOn(el model.Element) *model.Marks {
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

// textHit is one match, as the editor is told about it.
type textHit struct {
	Where   string `json:"where"`
	Part    int    `json:"part"`
	Measure int    `json:"measure"`
	Verse   int    `json:"verse,omitempty"`
	Text    string `json:"text"`
}

// handleFind searches the score's words, or changes them.
//
// GET finds, POST replaces. The same search in both, so that what is
// replaced is exactly what was listed.
func (a *App) handleFind(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	what := q.Get("q")
	if what == "" {
		http.Error(w, "there is nothing to look for", http.StatusBadRequest)
		return
	}
	fold := q.Get("case") != "1" // matching is insensitive unless asked

	if r.Method == http.MethodPost {
		with, ok := q["with"]
		if !ok {
			http.Error(w, "there is nothing to put in its place",
				http.StatusBadRequest)
			return
		}

		a.lock()
		defer a.mu.Unlock()

		changed := 0
		eachText(a.score, func(sp textSpot) {
			was := sp.get()
			now := replaceText(was, what, with[0], fold)
			if now != was {
				sp.set(now)
				changed++
			}
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"changed": changed})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	hits := []textHit{}
	eachText(a.score, func(sp textSpot) {
		if matchesText(sp.get(), what, fold) {
			hits = append(hits, textHit{
				Where: sp.Where, Part: sp.Part, Measure: sp.Measure,
				Verse: sp.Verse, Text: sp.get(),
			})
		}
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"hits": hits})
}

// matchesText says whether a piece of writing holds what is being looked
// for. Empty writing never matches, so that searching does not turn up
// every unwritten title in the score.
func matchesText(in, what string, fold bool) bool {
	if in == "" {
		return false
	}
	if fold {
		return strings.Contains(strings.ToLower(in), strings.ToLower(what))
	}
	return strings.Contains(in, what)
}

// replaceText puts one string in place of another, matching the same way
// the search does.
//
// Written out rather than handed to strings.Replacer because a fold-case
// search has to find "Flute" for "flute" and leave the rest of the words
// exactly as they were — the replacement goes in as it was typed and
// everything around it is untouched.
func replaceText(in, what, with string, fold bool) string {
	if in == "" || what == "" {
		return in
	}
	if !fold {
		return strings.ReplaceAll(in, what, with)
	}
	var b strings.Builder
	lower, lowerWhat := strings.ToLower(in), strings.ToLower(what)
	for {
		i := strings.Index(lower, lowerWhat)
		if i < 0 {
			b.WriteString(in)
			return b.String()
		}
		b.WriteString(in[:i])
		b.WriteString(with)
		in, lower = in[i+len(what):], lower[i+len(lowerWhat):]
	}
}
