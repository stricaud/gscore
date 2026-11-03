/*
 * technique.go
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

// handleTechnique puts a fretted-instrument playing mark on the selected
// notes, or takes one away.
//
// technique= names it, and an empty one clears the lot — the same shape as
// the articulation endpoint, because it is the same kind of thing: a mark
// that belongs to a note and that a player either sees or does not.
//
// off=1 takes one named mark away and leaves the rest, which is what a
// palette that shows what is on and lets it be switched off needs. Clearing
// everything to turn off a palm mute would also lose the hammer-on.
func (a *App) handleTechnique(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("technique")

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if name == "" {
		for _, m := range marks {
			m.Techniques = nil
		}
		writeJSON(w, map[string]int{"changed": len(marks)})
		return
	}
	t, ok := model.TechniqueFromName(name)
	if !ok {
		http.Error(w, "unknown technique "+name, http.StatusBadRequest)
		return
	}
	changed := 0
	for _, m := range marks {
		if q.Get("off") == "1" {
			if m.RemoveTechnique(t) {
				changed++
			}
			continue
		}
		// Only one of the four that reach to the next note can be on a
		// note at a time. A note is hammered onto or slid from, not both,
		// and without this a palette clicked twice would leave a slide
		// under a hammer-on with no way to see which was meant.
		if t.Links() {
			for _, other := range model.LinkTechniques() {
				if other != t {
					m.RemoveTechnique(other)
				}
			}
		}
		// Likewise the bends: a note bends one distance.
		if t.Bends() {
			for _, other := range model.BendTechniques() {
				if other != t {
					m.RemoveTechnique(other)
				}
			}
		}
		m.AddTechnique(t)
		changed++
	}
	writeJSON(w, map[string]int{"changed": changed})
}
