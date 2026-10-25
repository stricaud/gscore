/*
 * scoresdir.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os"
	"path/filepath"
	"sync"
)

/*
 * Where scores live.
 *
 * A program that asks "where shall I put this?" and offers the home
 * directory is asking a question the person opening a save panel for the
 * first time has no reason to have an answer to, and the answer they give
 * under that pressure is wherever the panel happened to be pointing. So
 * gscore has a folder of its own — Gscore, in Documents, beside the folders
 * every other program of this kind makes — and both the open panel and the
 * save panel start there.
 *
 * Made on the way past rather than by the installer. A Windows installer
 * runs once, as whoever is installing, and the folder belongs to each person
 * who runs the program; a macOS application is copied into place with no
 * script running at all; and somebody who built from source never ran an
 * installer. Asking for it the first time it is needed covers all of them,
 * costs one stat call, and leaves no trace on a machine where gscore is only
 * ever opened to look at somebody's link.
 */

// ScoresFolder is the name of the folder, under the user's documents.
const ScoresFolder = "Gscore"

var scoresDirOnce struct {
	sync.Once
	path string
}

// ScoresDir is the folder gscore saves to and opens from, made if it is not
// there yet.
//
// It falls back rather than failing: a home directory that cannot be found or
// a Documents folder that cannot be written to is unusual but not a reason to
// refuse to save a score. The panel then opens where it used to.
func ScoresDir() string {
	scoresDirOnce.Do(func() {
		scoresDirOnce.path = makeScoresDir()
	})
	return scoresDirOnce.path
}

func makeScoresDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	// Documents is where it goes on all three systems gscore runs on. On a
	// machine whose home has no Documents — a server account, a stripped
	// container — the folder is made beside whatever else is there rather
	// than inventing a Documents to put it in.
	parent := filepath.Join(home, "Documents")
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		parent = home
	}
	dir := filepath.Join(parent, ScoresFolder)
	if st, err := os.Stat(dir); err == nil {
		if st.IsDir() {
			return dir
		}
		// Something that is not a folder is already called that. Leave it
		// alone: it is not gscore's to move.
		return home
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return home
	}
	return dir
}
