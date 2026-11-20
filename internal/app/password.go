/*
 * password.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A score locked under a password.
 *
 * Saved, the music is sealed under a key made from the password, the way
 * a Word or Excel document with a password is, and the file carries no
 * PDF in front of it: nothing of the piece is readable without the word.
 * Opened here, the password is asked for and kept for the session, and
 * the next save locks the file under it again.
 */

package app

import (
	"bytes"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
)

// passwordHeader is where a password travels: in a header and not in the
// address, which is written in logs.
const passwordHeader = "X-Gscore-Password"

// passwordOf reads the password a request carries, if any.
func passwordOf(r *http.Request) string { return r.Header.Get(passwordHeader) }

// isGscoreName says a file is the program's own format, which is the one
// format that can be locked.
func isGscoreName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".gscore")
}

// unsupportedFormat is an error for a file nothing here reads.
type unsupportedFormat struct{ err error }

func (u unsupportedFormat) Error() string { return u.err.Error() }
func (u unsupportedFormat) Unwrap() error { return u.err }

// readScores reads a file into scores, unlocking a .gscore with the
// password where it is locked. A locked file read without its password
// comes back as gscorefmt.ErrPasswordNeeded, and with the wrong one as
// gscorefmt.ErrWrongPassword, for the caller to ask again.
func (a *App) readScores(name string, data []byte, password string) ([]*model.Score, error) {
	if isGscoreName(name) {
		s, _, err := gscorefmt.ReadWithPassword(bytes.NewReader(data), password)
		if s == nil {
			return nil, err
		}
		return []*model.Score{s}, err
	}
	imp, err := format.ImporterFor(name)
	if err != nil {
		return nil, unsupportedFormat{err}
	}
	return format.ImportAll(imp, bytesReader(data))
}

// passwordTrouble answers a request whose file wanted a password it did not
// have, or had the wrong one: 401, with a header the interface reads as
// "ask, and try again". It reports whether it answered.
func passwordTrouble(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, gscorefmt.ErrPasswordNeeded) && !errors.Is(err, gscorefmt.ErrWrongPassword) {
		return false
	}
	w.Header().Set("X-Needs-Password", "1")
	http.Error(w, err.Error(), http.StatusUnauthorized)
	return true
}

// handlePassword sets the password the score is saved under, or with
// clear=1 takes it off. Either is a change to the document: it is not on
// disk as it now stands.
func (a *App) handlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	clear := r.URL.Query().Get("clear") == "1"
	password := passwordOf(r)
	if !clear && password == "" {
		http.Error(w, "send the password in the "+passwordHeader+" header, or clear=1 to take it off",
			http.StatusBadRequest)
		return
	}
	a.lock()
	if clear {
		password = ""
	}
	changed := a.password != password
	a.password = password
	if changed {
		a.unsaved = true
	}
	a.mu.Unlock()
	writeJSON(w, map[string]any{"protected": password != "", "changed": changed})
}
