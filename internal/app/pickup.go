/*
 * pickup.go
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

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The bar before the first bar.
 *
 * A piece that begins before the first beat opens with a short bar, and
 * everything else follows from that: it is not numbered, the numbering
 * starts at the first full bar, and the last bar of the piece is often
 * short by the same amount to make up for it. The engraver already works
 * all of that out — a bar short of its meter followed by a full one is an
 * upbeat and nothing else is — so declaring one is a matter of making the
 * first bar hold that much time, on every staff, and letting the rest
 * follow.
 *
 * Every staff, because bars stand in the same columns on all of them: a
 * first bar of two beats on one staff and four on another is not a score.
 */

// handlePickup makes the first bar of the piece an upbeat of the length
// asked for, or a whole bar again.
func (a *App) handlePickup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()

	a.lock()
	defer a.mu.Unlock()

	if len(a.score.Parts) == 0 || len(a.score.Parts[0].Measures) == 0 {
		http.Error(w, "there is nothing written yet", http.StatusBadRequest)
		return
	}
	bar := a.score.Parts[0].TimeAt(0).BarDuration()
	if bar.IsZero() {
		http.Error(w, "the piece has no meter to be short of", http.StatusBadRequest)
		return
	}

	// "none" puts the first bar back to its full length, which is what
	// taking the upbeat away means.
	want := bar
	if name := q.Get("dur"); name != "" && name != "none" {
		d, ok := durationNames[name]
		if !ok {
			http.Error(w, name+" is not a note length", http.StatusBadRequest)
			return
		}
		want = dotted(d, q.Get("dots"))
	}
	if want.Cmp(bar) > 0 {
		http.Error(w, "an upbeat is shorter than a bar, not longer",
			http.StatusBadRequest)
		return
	}

	// Nothing is thrown away. A first bar with more music in it than the
	// upbeat would hold is a bar somebody has written in, and shortening
	// it would have to decide which note to cut.
	for _, p := range a.score.Parts {
		if len(p.Measures) == 0 {
			continue
		}
		if held := soundingIn(p.Measures[0]); held.Cmp(want) > 0 {
			http.Error(w, "there is more music in the first bar than that "+
				"— take some of it out first", http.StatusBadRequest)
			return
		}
	}

	for _, p := range a.score.Parts {
		if len(p.Measures) == 0 {
			continue
		}
		setBarLength(p.Measures[0], want)
	}
	// How much of a bar the upbeat is, as a fraction, which is what the
	// editor says back: "an upbeat of a quarter of the bar" is readable
	// where a pair of numbers is not.
	writeJSON(w, map[string]any{
		"status": "ok",
		"of":     want.Float() / bar.Float(),
		"full":   want.Cmp(bar) == 0,
	})
}

// soundingIn is how much of a bar is music rather than silence.
//
// Rests at the end of the first bar are what an upbeat is written against:
// a bar holding a quarter note and three quarter rests is a bar with one
// beat of music in it, and making it an upbeat of one beat throws nothing
// away.
func soundingIn(m *model.Measure) model.Duration {
	most := model.Duration{Num: 0, Den: 1}
	for _, v := range m.Voices {
		if v == nil {
			continue
		}
		at := model.Duration{Num: 0, Den: 1}
		last := at
		for _, el := range v.Elements {
			at = at.Add(el.Duration())
			if _, isRest := el.(*model.Rest); !isRest {
				last = at
			}
		}
		if last.Cmp(most) > 0 {
			most = last
		}
	}
	return most
}

// setBarLength makes a bar hold exactly this much, keeping its music and
// spelling the rest of it as rests.
func setBarLength(m *model.Measure, want model.Duration) {
	for _, v := range m.Voices {
		if v == nil {
			continue
		}
		// Everything up to the last thing that sounds, and then silence
		// to fill out the bar. The rests that were there are dropped
		// rather than kept and added to: they are the old length of the
		// bar, which is the thing being changed.
		var kept []model.Element
		at := model.Duration{Num: 0, Den: 1}
		held := model.Duration{Num: 0, Den: 1}
		for _, el := range v.Elements {
			at = at.Add(el.Duration())
			if _, isRest := el.(*model.Rest); isRest {
				continue
			}
			kept = append(kept, el)
			held = at
		}
		// A voice of nothing but rests keeps none of them, and a voice
		// with music keeps what came before its last note as well.
		if len(kept) > 0 {
			kept = nil
			at = model.Duration{Num: 0, Den: 1}
			for _, el := range v.Elements {
				if at.Cmp(held) >= 0 {
					break
				}
				kept = append(kept, el)
				at = at.Add(el.Duration())
			}
		}
		if left := want.Sub(held).Reduced(); left.Cmp(model.Duration{Num: 0, Den: 1}) > 0 {
			rests, _ := restsFor(left)
			kept = append(kept, rests...)
		}
		v.Elements = kept
	}
}
