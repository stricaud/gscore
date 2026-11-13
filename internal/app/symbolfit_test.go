/*
 * symbolfit_test.go
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
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

/*
 * A picture imported as a symbol comes in whole.
 *
 * It used to come in as its top left corner. A picture larger than the
 * editor's grid was refused outright, and one that fitted was laid into the
 * grid at its own size and cut off at the edge — so a logo imported as a
 * symbol was the corner of a logo, which is not the logo.
 *
 * It is scaled here rather than in the page because here is where there is
 * still a picture to scale: by the time the grid reaches the browser it is
 * ink and nothing, and a line thrown away in the shrinking cannot be got
 * back by stretching what is left.
 */

func picturePNG(w, h int) string {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// A black border, so that something survives at any size and the
	// proportions can be read off what comes back.
	for x := 0; x < w; x++ {
		im.Set(x, 0, color.RGBA{0, 0, 0, 255})
		im.Set(x, h-1, color.RGBA{0, 0, 0, 255})
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.String()
}

func TestABigPictureIsScaledNotRefused(t *testing.T) {
	h := newHarness(t)
	out := h.postBody("/api/symbolimport?fit=32", picturePNG(900, 600))
	w, _ := out["w"].(float64)
	ht, _ := out["h"].(float64)
	if int(w) != 32 {
		t.Errorf("the picture came back %v wide, want 32", out["w"])
	}
	// Proportions kept: 900x600 is three to two.
	if int(ht) != 21 {
		t.Errorf("the picture came back %v tall, want 21 — three to two", out["h"])
	}
}

func TestATallPictureFitsByItsHeight(t *testing.T) {
	h := newHarness(t)
	out := h.postBody("/api/symbolimport?fit=24", picturePNG(300, 900))
	if got, _ := out["h"].(float64); int(got) != 24 {
		t.Errorf("the picture came back %v tall, want 24", out["h"])
	}
	if got, _ := out["w"].(float64); int(got) != 8 {
		t.Errorf("the picture came back %v wide, want 8", out["w"])
	}
}

func TestTheWholePictureSurvivesTheShrink(t *testing.T) {
	// Averaged rather than sampled: a line one pixel wide in a picture
	// shrunk thirty times falls between the samples and disappears, which
	// is how a scanned device came through as a blank grid.
	h := newHarness(t)
	out := h.postBody("/api/symbolimport?fit=16", picturePNG(640, 640))
	pix, _ := out["pixels"].([]any)
	ink := 0
	for _, v := range pix {
		if n, _ := v.(float64); n != 0 {
			ink++
		}
	}
	if ink == 0 {
		t.Error("the picture came back empty: the lines fell between the samples")
	}
}

func TestAPictureWithNoSizeAskedForIsStillRefusedIfHuge(t *testing.T) {
	// Without a size there is nothing to scale to, and a 900-pixel grid is
	// not something the drawing window can show. The message says what to
	// do about it rather than only that it will not.
	h := newHarness(t)
	msg := h.postBodyFails("/api/symbolimport", picturePNG(900, 600))
	if !strings.Contains(msg, "which size") {
		t.Errorf("refused with %q", msg)
	}
}
