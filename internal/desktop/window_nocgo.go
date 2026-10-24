//go:build !cgo || nowebview

/*
 * window_nocgo.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package desktop

import "errors"

// Available reports that no native window can be opened in this build.
//
// Two builds end up here. Without cgo there is no web view to wrap, so
// gscore serves the interface to a browser rather than failing. And with the
// nowebview tag there is one deliberately: the macOS application in
// ui/macosx brings its own window, its own menu bar and its own panels, and
// linking a second web view into it would be two of everything.
const Available = false

// Open always fails in a build with no window of its own.
func Open(url, title string, width, height int) error {
	return errors.New("desktop: this build has no native window")
}
