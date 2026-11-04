/*
 * flute.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/flute"
)

/*
 * The flute's fingerings, to be looked through.
 *
 * The chart in the back of a method book gives one fingering a note. The
 * table behind this gives every one the instrument has — some twenty-nine
 * thousand — and the question is not how to store them but how to find
 * anything in them. Two orderings do nearly all of it: how far a fingering
 * is from the one everybody learns, and how far out of tune it comes out.
 * The alternate a player would reach for is one key from the chart and in
 * tune; the ones eleven keys away are the acoustic model saying a note can
 * be got out of a shape nobody would use.
 *
 * Trills are asked for as a pair of notes rather than looked up, because
 * that is what a trill is: one fingering for each note, differing by the
 * single key that shakes. See flute.Trills.
 */

// fingeringOut is one fingering as the editor reads it.
type fingeringOut struct {
	Keys  uint32 `json:"keys"`
	Cents int    `json:"cents"`
	Spell string `json:"spell"`
	Away  int    `json:"away"`
}

// handleFlute answers about the instrument, one of its notes, or a trill.
func (a *App) handleFlute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// The instrument itself: what the keys are called and what notes there
	// are. Everything the window draws comes from here rather than being
	// written out a second time in the editor.
	base := map[string]any{
		"keys":  flute.Keys(),
		"left":  flute.LeftHand(),
		"notes": flute.Names(),
	}

	from, to := q.Get("from"), q.Get("to")
	if from != "" || to != "" {
		if _, ok := flute.Of(from); !ok {
			http.Error(w, "no note called "+from, http.StatusBadRequest)
			return
		}
		if _, ok := flute.Of(to); !ok {
			http.Error(w, "no note called "+to, http.StatusBadRequest)
			return
		}
		all := flute.Trills(from, to)
		limit, offset := pageOf(q, 60)
		shown := all
		if offset < len(shown) {
			shown = shown[offset:]
		} else {
			shown = nil
		}
		if len(shown) > limit {
			shown = shown[:limit]
		}
		out := make([]map[string]any, 0, len(shown))
		for _, t := range shown {
			out = append(out, map[string]any{
				"hold":  fingeringOut{Keys: t.Hold.Keys, Cents: t.Hold.Cents, Spell: flute.Spell(t.Hold.Keys), Away: t.Away},
				"key":   t.Key,
				"down":  t.Down,
				"upper": t.Upper,
			})
		}
		base["from"], base["to"] = from, to
		base["total"], base["trills"] = len(all), out
		writeJSON(w, base)
		return
	}

	name := q.Get("note")
	if name == "" {
		writeJSON(w, base)
		return
	}
	note, ok := flute.Of(name)
	if !ok {
		http.Error(w, "no note called "+name, http.StatusBadRequest)
		return
	}

	// What to leave out. away is how many keys may differ from the chart
	// and within how far out of tune it may be; both are "no limit" when
	// they are not asked for, because the whole point of the table is that
	// everything in it can be looked at.
	away := -1
	if v := q.Get("away"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, v+" is not a number of keys", http.StatusBadRequest)
			return
		}
		away = n
	}
	within := -1
	if v := q.Get("within"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, v+" is not a number of cents", http.StatusBadRequest)
			return
		}
		within = n
	}
	using := splitKeys(q.Get("using"))
	without := splitKeys(q.Get("without"))

	kept := make([]fingeringOut, 0, 64)
	for _, f := range note.Others {
		d := flute.Away(f.Keys, note.Standard.Keys)
		if away >= 0 && d > away {
			continue
		}
		if within >= 0 && (f.Cents > within || f.Cents < -within) {
			continue
		}
		skip := false
		for _, k := range using {
			if !flute.Has(f.Keys, k) {
				skip = true
				break
			}
		}
		for _, k := range without {
			if flute.Has(f.Keys, k) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		kept = append(kept, fingeringOut{
			Keys: f.Keys, Cents: f.Cents, Spell: flute.Spell(f.Keys), Away: d,
		})
	}
	// Nearest the chart first, and among those the ones nearest in tune.
	// The order is the whole of what makes this usable: a list of five
	// thousand fingerings in the order they were scraped is not a list
	// anybody can read.
	sortFingerings(kept)

	limit, offset := pageOf(q, 60)
	shown := kept
	if offset < len(shown) {
		shown = shown[offset:]
	} else {
		shown = nil
	}
	if len(shown) > limit {
		shown = shown[:limit]
	}

	base["note"] = name
	base["standard"] = fingeringOut{
		Keys:  note.Standard.Keys,
		Cents: note.Standard.Cents,
		Spell: flute.Spell(note.Standard.Keys),
	}
	base["total"], base["shown"] = len(kept), shown
	writeJSON(w, base)
}

// sortFingerings puts the plausible ones first: nearest the standard
// chart, then nearest in tune, then in a settled order so that paging
// through them does not shuffle.
func sortFingerings(fs []fingeringOut) {
	less := func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Away != b.Away {
			return a.Away < b.Away
		}
		ca, cb := a.Cents, b.Cents
		if ca < 0 {
			ca = -ca
		}
		if cb < 0 {
			cb = -cb
		}
		if ca != cb {
			return ca < cb
		}
		return a.Keys < b.Keys
	}
	sort.SliceStable(fs, less)
}

// pageOf reads how much of a long answer to send.
func pageOf(q map[string][]string, byDefault int) (limit, offset int) {
	limit, offset = byDefault, 0
	if v := get1(q, "limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	if v := get1(q, "offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func get1(q map[string][]string, name string) string {
	if v, ok := q[name]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// splitKeys reads a comma-separated list of key names, leaving out the
// empty pieces a trailing comma leaves behind.
func splitKeys(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
