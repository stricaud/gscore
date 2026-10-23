/*
 * undo.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

import (
	"net/http"

	"github.com/stricaud/gscore/internal/model"
)

// Undo.
//
// A copy of the score before each edit, rather than a record of each edit and
// how to reverse it. Reversible commands are the textbook answer and they are
// the wrong answer here: every command has to carry its own inverse, the
// inverse has to stay correct as the command grows, and the first one that is
// slightly wrong corrupts the score in a way nobody notices until they save
// it. A copy cannot be slightly wrong.
//
// The cost is a copy of the score per edit. The largest thing anybody edits
// here is some thousands of notes, and model.Score.Clone has a test that
// walks both copies and fails if they share so much as a slice — which is the
// only property that matters.
//
// Snapshots are taken by wrapping the handlers rather than by calling
// something at the top of each one. There are forty-odd handlers that change
// the score and remembering to call a function in each is a thing that works
// until it is forgotten; wrapping the route cannot be forgotten, because a
// route that is not wrapped is a route that was never listed.

// undoDepth is how far back it goes. Deep enough that nobody reaches the end
// in normal work, shallow enough that a big score cannot eat memory.
const undoDepth = 200

// snapshot is a score and where it belongs.
type snapshot struct {
	score *model.Score
	// tune is which of a book's scores this was, so undoing an edit made in
	// one exercise does not put it back over a different one.
	tune int
}

// take copies the score being edited. The caller holds the lock.
func (a *App) take() snapshot {
	return snapshot{score: a.score.Clone(), tune: a.current}
}

// restore puts a snapshot back. The caller holds the lock.
func (a *App) restore(s snapshot) {
	if s.tune >= 0 && s.tune < len(a.book) {
		a.book[s.tune] = s.score
		a.current = s.tune
	}
	a.score = s.score
}

// undoable wraps a handler so that whatever it changes can be taken back.
//
// The snapshot is thrown away when the handler refuses, because a request
// that changed nothing should not cost a press of undo to get past.
func (a *App) undoable(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h(w, r)
			return
		}

		a.mu.Lock()
		before := a.take()
		a.mu.Unlock()

		rec := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		h(rec, r)

		a.mu.Lock()
		defer a.mu.Unlock()
		if rec.code < 200 || rec.code >= 300 {
			return // nothing happened, so there is nothing to take back
		}
		a.undoStack = append(a.undoStack, before)
		if len(a.undoStack) > undoDepth {
			a.undoStack = a.undoStack[len(a.undoStack)-undoDepth:]
		}
		// A new edit is a new branch: what was undone cannot be redone now.
		a.redoStack = nil
	}
}

// statusWriter remembers what a handler answered, so the wrapper can tell a
// change from a refusal.
type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.code, w.written = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(b)
}

// handleUndo steps back one edit.
func (a *App) handleUndo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.undoStack) == 0 {
		http.Error(w, "there is nothing to undo", http.StatusBadRequest)
		return
	}
	last := a.undoStack[len(a.undoStack)-1]
	a.undoStack = a.undoStack[:len(a.undoStack)-1]
	// What is being undone has to be kept, or it cannot be redone.
	a.redoStack = append(a.redoStack, a.take())
	a.restore(last)

	writeJSON(w, map[string]any{
		"status": "ok", "tune": a.current,
		"canUndo": len(a.undoStack) > 0, "canRedo": len(a.redoStack) > 0,
	})
}

// handleRedo steps forward again.
func (a *App) handleRedo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.redoStack) == 0 {
		http.Error(w, "there is nothing to redo", http.StatusBadRequest)
		return
	}
	next := a.redoStack[len(a.redoStack)-1]
	a.redoStack = a.redoStack[:len(a.redoStack)-1]
	a.undoStack = append(a.undoStack, a.take())
	a.restore(next)

	writeJSON(w, map[string]any{
		"status": "ok", "tune": a.current,
		"canUndo": len(a.undoStack) > 0, "canRedo": len(a.redoStack) > 0,
	})
}

// forgetHistory drops the history, for when a different file is opened: the
// edits before it belong to a score that is no longer here.
func (a *App) forgetHistory() {
	a.undoStack, a.redoStack = nil, nil
}
