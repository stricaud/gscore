/*
 * pdfimport.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/format/engraved"
	"github.com/stricaud/gscore/internal/model"
)

// A PDF is read differently from every other format, and this file is why.
//
// Every other importer parses a file that says what it holds: an ABC tune
// says where its parts are, a MusicXML part says which instrument it is. A
// PDF says none of that. It is a set of drawing instructions that happen to
// make the shape of a page, and what counts as a system — which is to say
// which staves are played at once by different players, and which are the
// same player further down the page — has to be inferred from how the page
// was drawn.
//
// The inference is a good one and it is not reliable. It looks for a line
// crossing the gap between two staves, because that is what ties a system
// together; where an engraver draws the barlines of each staff separately it
// finds nothing, and every staff becomes a system of its own. The whole file
// then arrives as one part thousands of bars long. That is not a subtle
// failure, but it is an invisible one: the score that comes out is perfectly
// well formed, and nothing in it says it is wrong.
//
// So two things are kept that no other format needs. The bytes, so the same
// file can be read again a different way; and the number of staves per
// system, so the answer can be given rather than guessed.

// isPDFName says whether a filename is a PDF, which is what decides whether
// any of this applies.
func isPDFName(name string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".pdf")
}

// readPDF imports a PDF and keeps what is needed to read it again.
func (a *App) readPDF(name string, data []byte, per int) ([]*model.Score, error) {
	s, err := engraved.ImportWith(bytes.NewReader(data),
		engraved.Options{StavesPerSystem: per})
	if err != nil {
		return nil, err
	}
	a.lock()
	// A copy, not the caller's slice: the caller owns a request body that is
	// recycled the moment the handler returns.
	a.source = append([]byte(nil), data...)
	a.sourceName = name
	a.perSystem = per
	a.mu.Unlock()
	return []*model.Score{s}, nil
}

// forgetPDF drops the kept bytes, which is what opening anything else means.
func (a *App) forgetPDF() {
	a.source, a.sourceName, a.perSystem = nil, "", 0
}

// handlePDFSurvey reports what the reader saw, so it can be drawn over the
// page it was read from.
//
// The answer describes the reading, not the music: where it put the staves,
// how it grouped them, and the pitch it gave each notehead. A score is the
// wrong thing to look at here — a score is what you get once the reading has
// been believed.
func (a *App) handlePDFSurvey(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	data, name, per := a.source, a.sourceName, a.perSystem
	a.mu.RUnlock()
	if len(data) == 0 {
		http.Error(w, "no PDF is open", http.StatusNotFound)
		return
	}
	if n, ok := stavesParam(r); ok {
		per = n
	}
	sv, err := engraved.Look(bytes.NewReader(data),
		engraved.Options{StavesPerSystem: per})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"filename": name,
		"survey":   sv,
	})
}

// handlePDFReread reads the open PDF again, told how many staves a system
// holds.
//
// It replaces the open score, and says so plainly rather than offering to
// merge: a different grouping is a different reading of the same page, not an
// edit of what is there, and there is nothing sensible to keep from the
// reading being thrown away.
func (a *App) handlePDFReread(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	a.mu.RLock()
	data, name := a.source, a.sourceName
	a.mu.RUnlock()
	if len(data) == 0 {
		http.Error(w, "no PDF is open", http.StatusNotFound)
		return
	}
	n, ok := stavesParam(r)
	if !ok {
		http.Error(w, "staves must be a whole number from 0 to 64",
			http.StatusBadRequest)
		return
	}
	scores, err := a.readPDF(name, data, n)
	if err != nil || len(scores) == 0 {
		msg := "could not read the PDF"
		if err != nil {
			msg = err.Error()
		}
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	a.lock()
	a.setBook(scores, name, nil)
	a.source, a.sourceName, a.perSystem = append([]byte(nil), data...), name, n
	a.mu.Unlock()
	writeJSON(w, map[string]any{
		"status": "ok", "filename": name,
		"staves": n, "parts": len(scores[0].Parts),
	})
}

// stavesParam reads the staves-per-system from a request.
//
// Zero is a legitimate answer and means "work it out", so "absent" and "zero"
// have to be told apart; hence the second return rather than a zero value
// doing double duty. The ceiling is there because the number becomes a block
// size, and a block size larger than any real score is a typo that would
// silently make the whole file one system.
func stavesParam(r *http.Request) (int, bool) {
	s := r.URL.Query().Get("staves")
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 64 {
		return 0, false
	}
	return n, true
}

// KeepPDFSource tells the application which bytes the open score was read
// from, for a score that was opened before the application existed.
//
// The command line opens its file and builds the editor around it, so there
// is no handler to go through and nothing to keep the bytes. Without this, a
// PDF named on the command line could be looked at and not read again, which
// is the one combination worth avoiding: the window would show the grouping
// was wrong and offer no way to say so.
func (a *App) KeepPDFSource(name string, data []byte) {
	if !isPDFName(name) || len(data) == 0 {
		return
	}
	a.lock()
	a.source = append([]byte(nil), data...)
	a.sourceName = name
	a.perSystem = 0
	a.mu.Unlock()
}
