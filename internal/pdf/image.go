/*
 * image.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package pdf

import (
	"fmt"
	"image"
	"image/color"
)

// Pictures in a PDF, which for gscore means the symbols somebody drew by hand.
//
// A drawn symbol is a grid of pixels, so it goes in as an image rather than as
// an outline. PDF draws images into the unit square and leaves the placing to
// a matrix, which suits a pixel grid: the same grid prints at any size without
// being resampled, and the reader does the scaling at the device's own
// resolution.

// Image is a picture the document can draw.
type Image struct {
	key string
	obj int
}

// Image adds a picture, or returns the one already added under that name.
//
// Anything with an alpha channel gets a soft mask, because a symbol drawn on
// nothing should not arrive as a symbol drawn on a white block sitting over
// the staff lines.
func (d *Doc) Image(name string, img image.Image) *Image {
	key := "Im" + sanitise(name)
	if im, ok := d.images[key]; ok {
		return im
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}

	rgb := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)
	opaque := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			rgb = append(rgb, c.R, c.G, c.B)
			alpha = append(alpha, c.A)
			if c.A != 0xFF {
				opaque = false
			}
		}
	}

	mask := 0
	if !opaque {
		mask = d.addStream(fmt.Sprintf(
			"/Type /XObject /Subtype /Image /Width %d /Height %d "+
				"/ColorSpace /DeviceGray /BitsPerComponent 8", w, h), alpha)
	}

	dict := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d "+
		"/ColorSpace /DeviceRGB /BitsPerComponent 8", w, h)
	if mask != 0 {
		dict += fmt.Sprintf(" /SMask %d 0 R", mask)
	}
	// Interpolation off: a hand-drawn grid is meant to look like the pixels
	// it is, not like a blur of them.
	dict += " /Interpolate false"

	obj := d.addStream(dict, rgb)
	im := &Image{key: key, obj: obj}
	d.images[key] = im
	d.imageKeys = append(d.imageKeys, key)
	return im
}
