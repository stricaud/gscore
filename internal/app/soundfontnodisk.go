//go:build js

/*
 * soundfontnodisk.go
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

import "errors"

// diskSoundfonts finds none, there being nowhere to look.
func diskSoundfonts() []installed { return nil }

// fontExists is false for anything but what has been loaded into memory,
// which is answered before this is reached.
func fontExists(string) bool { return false }

var errNoDisk = errors.New("there is no filesystem in a page")

func readSettings() ([]byte, error) { return nil, errNoDisk }
func writeSettings([]byte) error    { return errNoDisk }
func settingsPath() (string, error) { return "", errNoDisk }
