/*
 * crossstaff.go
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

// handleCrossStaff writes the selected notes on the staff above or below
// the one they belong to.
//
// Nothing about the music changes: the same pitch, the same bar, the same
// voice, the same sound. Only where it is drawn. That is why this is a
// mark on the note and not a move to another staff — a figure written
// across the two staves of a piano is one hand's rhythm, and moving its
// notes into the other hand's voice would make it a different piece.
func (a *App) handleCrossStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	dir, ok := model.CrossStaffFromName(q.Get("to"))
	if !ok {
		http.Error(w, "write it on the staff \"above\", \"below\", or its "+
			"\"own\"", http.StatusBadRequest)
		return
	}

	marks, err := a.marksOf(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, m := range marks {
		m.Cross = dir
	}
	writeJSON(w, map[string]int{"changed": len(marks)})
}
