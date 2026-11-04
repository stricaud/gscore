/*
 * embedheight_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How tall a frame with music in it should be.
 *
 * Asked for as "can we by default have the height that matches the number
 * of instruments while allowing the user to make it shorter (yet have the
 * scroll bars)?" — a frame opened at a guessed height either cuts a system
 * in half or stands mostly empty, and the right height depends on the
 * score: a song is one staff and a quartet is four.
 *
 * The editor works it out from where the staves were drawn, which it reads
 * from /api/hits. So the thing that has to hold here is that /api/hits
 * answers about the picture on the screen and not about some other one:
 * the systems of an embedded score are wrapped to the width of the frame,
 * and a measurement taken from an unwrapped layout describes a score
 * nobody is looking at.
 */

package app

import (
	"encoding/json"
	"testing"
)

// layoutOf is the size of the drawing, in staff spaces.
func layoutOf(h *harness, query string) (w, ht float64) {
	h.t.Helper()
	var got map[string]float64
	if err := json.Unmarshal([]byte(h.get("/api/layout?"+query)), &got); err != nil {
		h.t.Fatalf("layout %s: %v", query, err)
	}
	return got["width"], got["height"]
}

// hitsWidth is how wide the picture the clickable places belong to is: the
// rightmost edge anything was drawn at.
func hitsWidth(h *harness, query string) float64 {
	h.t.Helper()
	var got struct {
		Hits []struct {
			X float64 `json:"x"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/hits?"+query)), &got); err != nil {
		h.t.Fatalf("hits %s: %v", query, err)
	}
	right := 0.0
	for _, hit := range got.Hits {
		if hit.X > right {
			right = hit.X
		}
	}
	return right
}

// A frame of so many pixels gets a drawing of so many pixels.
func TestAFrameGetsTheWidthItAskedFor(t *testing.T) {
	h := newHarness(t)
	h.bars(12)
	for m := 0; m < 12; m++ {
		h.note(0, m, 0, "quarter")
	}

	w, _ := layoutOf(h, "mode=page&space=15&fit=600")
	if got := w * 15; got < 590 || got > 610 {
		t.Errorf("a frame of 600px was drawn %.0fpx wide, want 600", got)
	}
}

// A frame narrower than a couple of bars used to be ignored altogether,
// and the score came back at its full width — the one layout that
// certainly does not fit. It is squeezed to the narrowest the engraver
// will go instead.
func TestAVeryNarrowFrameIsNotIgnored(t *testing.T) {
	h := newHarness(t)
	h.bars(12)
	for m := 0; m < 12; m++ {
		h.note(0, m, 0, "quarter")
	}

	loose, _ := layoutOf(h, "mode=page&space=15")
	tight, _ := layoutOf(h, "mode=page&space=15&fit=120")
	if tight >= loose {
		t.Errorf("a 120px frame was drawn %.1f spaces wide, no narrower than "+
			"the %.1f it is given when nothing is asked for", tight, loose)
	}
}

// The measurement and the drawing are the same picture.
//
// /api/hits used to read the mode and nothing else, so an embedded score —
// whose systems are wrapped to the frame — reported its notes at the
// coordinates of a layout that was never drawn. A click landed in the
// wrong bar, and a frame sized from it opened at the height of somebody
// else's score.
func TestTheClickablePlacesBelongToThePictureOnTheScreen(t *testing.T) {
	h := newHarness(t)
	h.bars(12)
	for m := 0; m < 12; m++ {
		h.note(0, m, 0, "quarter")
	}

	const q = "mode=page&space=15&fit=420"
	w, _ := layoutOf(h, q)
	if right := hitsWidth(h, q); right > w {
		t.Errorf("a note is reported at %.1f spaces, in a drawing %.1f wide",
			right, w)
	}

	// And the wrapping itself: a narrow frame makes more systems, so the
	// same music is reported lower down.
	wide := lowestStaff(h, "mode=page&space=15&fit=900")
	narrow := lowestStaff(h, "mode=page&space=15&fit=300")
	if narrow <= wide {
		t.Errorf("wrapped into a 300px frame the last staff sits at %.1f, "+
			"no lower than the %.1f of a 900px one — the hits were taken "+
			"from a picture that was not wrapped", narrow, wide)
	}
}

// lowestStaff is how far down the drawing the last staff was put.
func lowestStaff(h *harness, query string) float64 {
	h.t.Helper()
	var got struct {
		Hits []struct {
			StaffTop float64 `json:"staffTop"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/hits?"+query)), &got); err != nil {
		h.t.Fatalf("hits %s: %v", query, err)
	}
	low := 0.0
	for _, hit := range got.Hits {
		if hit.StaffTop > low {
			low = hit.StaffTop
		}
	}
	return low
}
