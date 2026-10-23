//go:build !cgo

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
// The native window needs cgo, since it wraps the platform's own web view.
// Built without it, gscore falls back to serving the interface to a browser
// rather than failing.
const Available = false

// Open always fails in a build without cgo.
func Open(url, title string, width, height int) error {
	return errors.New("desktop: this build has no native window (built without cgo)")
}
