/*
 * brand.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package brand holds what the program is called and what it looks like.
//
// One copy, in one place, because the name and the mascot are printed by
// things that have nothing else to do with each other — the editor serves
// Gilbert to a browser, the PDF writer stamps him on paper — and two copies
// of a logo are two logos as soon as one of them is redrawn.
package brand

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

// Name is the program's name as it is written for somebody to read: in a
// window title, a menu bar, a document's properties, an about box.
//
// Capitalised, like any other name. The lower-case gscore is the command,
// the file extension and the package path, and those stay as they are.
const Name = "Gscore"

// Gilbert is the mascot: the bug who has been the program's face since the
// first version, drawn as an X pixmap in 2001 and kept.
//
//go:embed gilbert.png
var GilbertPNG []byte

var (
	once    sync.Once
	gilbert image.Image
	gErr    error
)

// Gilbert decodes the mascot, once.
func Gilbert() (image.Image, error) {
	once.Do(func() {
		gilbert, gErr = png.Decode(bytes.NewReader(GilbertPNG))
	})
	return gilbert, gErr
}
