/*
 * breath.go
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

// Where the player breathes.
//
// The mark was already in gscore: the model has carried Break since the
// marks were written, the engraver draws all four of them after the note and
// over the staff, the file format saves them and a MusicXML file brings them
// in. What was missing was any way to ask for one. A breath mark could only
// arrive from a file somebody else had written, so a wind part typed into
// gscore could not say where to take a breath — which is the one mark a wind
// player cannot do without.

// handleBreath sets the breath or pause taken after the selected notes.
//
// "none" takes it off, as the None entry on the menu does. A breath belongs
// to a note and not to a staff: it says the gap after this one is longer
// than the gap after the others, so it goes on the element the way a
// staccato does, and like an articulation it can be put on several at once.
func (a *App) handleBreath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	name := q.Get("breath")
	if name == "" {
		name = "none"
	}
	b, ok := model.BreathFromName(name)
	if !ok {
		http.Error(w, "unknown breath mark "+name, http.StatusBadRequest)
		return
	}

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(marks) == 0 {
		http.Error(w, "there is nothing there to breathe after",
			http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Break = b
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}
