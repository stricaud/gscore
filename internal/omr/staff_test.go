/*
 * staff_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package omr

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// drawPage paints staves on a white page the way a scanner would see them:
// tilted, because every bound page is, and with the lines a pixel or two
// thick. Synthetic rather than a stored scan so that the test says what it
// means — a fixture would also be testing the fixture.
func drawPage(w, h int, staves []float64, inter, slope float64) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 230 // tinted paper, not pure white
	}
	ink := func(x, y int) {
		if x >= 0 && y >= 0 && x < w && y < h {
			img.SetGray(x, y, color.Gray{Y: 20})
		}
	}
	for _, top := range staves {
		for k := 0; k < 5; k++ {
			for x := 0; x < w; x++ {
				y := top + float64(k)*inter + slope*float64(x)
				ink(x, int(math.Round(y)))
				ink(x, int(math.Round(y))+1)
			}
		}
	}
	return img
}

func TestFindStavesFlat(t *testing.T) {
	const w, h = 600, 400
	img := drawPage(w, h, []float64{60, 220}, 12, 0)
	staves, inter := FindStaves(FromImage(img).Binarise(13*12, 20))
	if len(staves) != 2 {
		t.Fatalf("got %d staves, want 2", len(staves))
	}
	if math.Abs(inter-12) > 1 {
		t.Errorf("interline %.2f, want about 12", inter)
	}
	// the top line of the first staff was drawn at y=60 (plus the second
	// inked row, so its centre is a shade below)
	if got := staves[0].Lines[0].Y(w / 2); math.Abs(got-60.5) > 1.5 {
		t.Errorf("first line at %.2f, want about 60.5", got)
	}
}

func TestFindStavesTilted(t *testing.T) {
	const w, h = 900, 500
	// two degrees of tilt is what a bound folio gives: 31 px over 900
	const slope = 0.035
	img := drawPage(w, h, []float64{70, 280}, 14, slope)
	staves, inter := FindStaves(FromImage(img).Binarise(13*14, 20))
	if len(staves) != 2 {
		t.Fatalf("got %d staves, want 2", len(staves))
	}
	if math.Abs(inter-14) > 1.5 {
		t.Errorf("interline %.2f, want about 14", inter)
	}
	for i, s := range staves {
		if math.Abs(s.Lines[0].A-slope) > 0.006 {
			t.Errorf("staff %d slope %.4f, want about %.4f", i, s.Lines[0].A, slope)
		}
	}
}

// TestDegreeIgnoresTilt is the property the whole design rests on: a note
// sitting on a given line reads as the same degree wherever it is on the
// page, however much the page leans.
func TestDegreeIgnoresTilt(t *testing.T) {
	const w, h = 900, 400
	const slope = 0.035
	const inter = 14.0
	img := drawPage(w, h, []float64{70}, inter, slope)
	staves, _ := FindStaves(FromImage(img).Binarise(13*inter, 20))
	if len(staves) != 1 {
		t.Fatalf("got %d staves, want 1", len(staves))
	}
	s := staves[0]
	for _, x := range []float64{50, 300, 600, 850} {
		// a point on the bottom line, wherever we are across the page
		y := s.Lines[4].Y(x)
		if d := s.Degree(x, y); math.Abs(d) > 0.2 {
			t.Errorf("at x=%.0f the bottom line reads degree %.2f, want 0", x, d)
		}
		// and one an interline higher, which is two degrees up
		if d := s.Degree(x, y-inter); math.Abs(d-2) > 0.25 {
			t.Errorf("at x=%.0f one interline up reads %.2f, want 2", x, d)
		}
	}
}

func TestBinariseFollowsTheTint(t *testing.T) {
	// paper that darkens steadily from left to right, as a gutter shadow
	// does, with one grey mark that is lighter than the darkest paper. A
	// global threshold must either lose the mark or drown in the shadow; the
	// local one should find it and nothing else.
	const w, h = 400, 100
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8(240 - 120*x/w)})
		}
	}
	for y := 40; y < 60; y++ {
		for x := 40; x < 60; x++ {
			img.SetGray(x, y, color.Gray{Y: 170}) // lighter than the far right
		}
	}
	b := FromImage(img).Binarise(61, 20)
	if !b.At(50, 50) {
		t.Error("the mark was not found")
	}
	if b.At(380, 50) {
		t.Error("the shadow was taken for ink")
	}
}

// TestFindStavesTightInterline is the regression test for the bug that the
// real scan found and the earlier synthetic tests did not.
//
// At seven pixels between lines — a 1024-pixel scan of a folio, which is what
// libraries often publish — a chaining tolerance fixed in pixels is most of
// an interline, and a chain steps across to its neighbour partway along the
// page. The staff is still found, so nothing looks wrong; but the line fitted
// through the strayed chain leans at an angle no line on the page has, and
// every degree read from it afterwards is wrong by a step somewhere.
//
// The tilt is the thing to assert. The positions come out close either way.
func TestFindStavesTightInterline(t *testing.T) {
	const w, h = 1024, 600
	const inter = 7.0
	const slope = -0.028 // as a bound folio leans
	img := drawPage(w, h, []float64{80, 200, 320}, inter, slope)
	staves, got := FindStaves(FromImage(img).Binarise(13*int(inter), 20))
	if len(staves) != 3 {
		t.Fatalf("got %d staves, want 3", len(staves))
	}
	if math.Abs(got-inter) > 1 {
		t.Errorf("interline %.2f, want about %.0f", got, inter)
	}
	for i, s := range staves {
		for k, l := range s.Lines {
			if math.Abs(l.A-slope) > 0.004 {
				t.Errorf("staff %d line %d leans %.4f, want about %.4f"+
					" (a chain strayed to its neighbour)", i, k, l.A, slope)
			}
		}
	}
}
