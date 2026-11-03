/*
 * notehead.go
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

// Changing the shape a note is drawn with.
//
// The shapes were already there — a drum staff tells a cymbal from a drum by
// giving the one an x and the other an oval, and the file format and the
// engraver have carried Head since. What was missing was any way to ask for
// one: the shape could only arrive from the drum map, so a note that was not
// on a kit could never be anything but an oval.
//
// A dead note is what wanted it. On a tab the x in place of a fret number is
// how a muted string is written, and it is the same note with the same Head
// that a drum staff would draw as a cross — so this is one endpoint and not
// a second, tab-only way of saying the same thing.

// handleNotehead sets the shape of the selected notes.
//
// The shape is called shape and not head, which would read better, because
// notesOf below already takes head: there it is the staff step of one
// notehead of a chord, which is how the editor says "this note of this
// chord and not the others". Two meanings for one word in one request is a
// bug waiting to be written, so the newer of them gives way.
func (a *App) handleNotehead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	shape, ok := model.NoteheadKindFromName(q.Get("shape"))
	if !ok {
		http.Error(w, "unknown notehead "+q.Get("shape"), http.StatusBadRequest)
		return
	}

	notes, err := a.notesOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(notes) == 0 {
		http.Error(w, "there is no note there", http.StatusBadRequest)
		return
	}
	for _, n := range notes {
		n.Head = shape
	}
	writeJSON(w, map[string]int{"changed": len(notes)})
}
