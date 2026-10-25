/*
 * brand.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
// Twice over, because the two are wanted for different things. The drawing
// is what is printed — outlines go onto paper at the printer's resolution
// rather than at the bitmap's — and the bitmap is what the original was, 321
// pixels square, kept because the drawing was traced from it and is checked
// against it.
//
//go:embed gilbert.svg
var GilbertSVG []byte

//go:embed gilbert.png
var GilbertPNG []byte

var (
	once    sync.Once
	gilbert image.Image
	gErr    error

	drawOnce sync.Once
	drawing  *Drawing
	dErr     error
)

// Gilbert decodes the mascot as a picture, once.
func Gilbert() (image.Image, error) {
	once.Do(func() {
		gilbert, gErr = png.Decode(bytes.NewReader(GilbertPNG))
	})
	return gilbert, gErr
}

// GilbertDrawing reads the mascot as outlines, once.
func GilbertDrawing() (*Drawing, error) {
	drawOnce.Do(func() {
		drawing, dErr = ParseSVG(GilbertSVG)
	})
	return drawing, dErr
}
