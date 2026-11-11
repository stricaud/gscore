//go:build js

/*
 * soundfontnodisk.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

// The same questions, in a page, where there is no disk to ask them of.
//
// Not stubs that pretend: there really is no filesystem here. The soundfont
// is fetched by the page and handed to the core as bytes, and what gscore
// remembers is remembered by the page — in its own store, which outlives a
// visit, rather than in a file.
//
// This exists as its own build rather than as a check at each call because
// the failure it prevents is not an error. Go's filesystem calls in a
// browser are answered through the event loop, and the core is already
// inside it answering a request: the call waits for a reply that cannot
// arrive until the call returns, and the program stops dead.

import (
	"errors"
	"os"
	"path/filepath"
)

// soundfontDir is the name of a folder nothing here will ever open.
//
// The page reports it in the recording dialog and that is all it is for:
// in a browser there is no disk to download to, and the answer only has to
// be the same words it has always been. Left exactly as it was when the
// folder on a real machine moved into Documents — that move is about a
// folder somebody has to find in a Finder, and there is no Finder here.
//
// Worked out from the environment and nothing else. os.UserConfigDir reads
// a variable; anything that asks the filesystem a question stops the
// program dead, for the reason at the head of this file.
func soundfontDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Gscore", "soundfonts"), nil
}

// oldSoundfontDir is the same folder: there was never another one here.
func oldSoundfontDir() (string, error) { return soundfontDir() }

// diskSoundfonts finds none, there being nowhere to look.
func diskSoundfonts() []installed { return nil }

// fileExists is false for everything: there is no disk to look on, and
// anything loaded into memory is answered before this is reached.
func fileExists(string) bool { return false }

var errNoDisk = errors.New("there is no filesystem in a page")

func readSettings() ([]byte, error) { return nil, errNoDisk }
func writeSettings([]byte) error    { return errNoDisk }
func settingsPath() (string, error) { return "", errNoDisk }
