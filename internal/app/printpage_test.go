/*
 * printpage_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What comes out of the printer.
 */

package app

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The printed page is cut where the engraver cuts it, not where the paper
// runs out.
//
// Reported from a printed part with a staff sliced in half along the page
// break. The whole score went to the browser as one tall drawing, and a
// browser knows nothing about systems: it breaks wherever the sheet ends.
// So each page is its own drawing now, cropped to the band the engraver
// would have put on that page.
func TestEachPrintedPageIsItsOwnDrawing(t *testing.T) {
	h := newHarness(t)
	// Enough music for several pages.
	for i := 0; i < 700; i++ {
		h.note(0, 0, i%8, "quarter")
	}

	page := h.get("/print?what=score")
	sections := strings.Count(page, "<section>")
	if sections < 2 {
		t.Fatalf("%d printed pages for 175 bars, want more than one", sections)
	}
	// One drawing per page, each cropped to its own band: the viewBox of
	// the second starts where the first left off rather than at zero.
	boxes := regexp.MustCompile(`viewBox="([\d.]+) ([\d.]+) ([\d.]+) ([\d.]+)"`).
		FindAllStringSubmatch(page, -1)
	if len(boxes) != sections {
		t.Fatalf("%d drawings for %d pages", len(boxes), sections)
	}
	last := -1.0
	for i, b := range boxes {
		top, _ := strconv.ParseFloat(b[2], 64)
		height, _ := strconv.ParseFloat(b[4], 64)
		if height <= 0 {
			t.Errorf("page %d has no height", i+1)
		}
		if top <= last {
			t.Errorf("page %d starts at %g, which is not past the page before it",
				i+1, top)
		}
		last = top
	}
	// And the paper is stated in the page's own units, so the browser lays
	// out the sheet the bands were cut for.
	if !strings.Contains(page, "@page { size: 595.3pt 841.9pt;") {
		t.Error("the print page does not say which paper it is for")
	}

	// The maker's mark goes on the first sheet, where the PDF of the same
	// score puts it, and on no other: on every sheet it stops being a
	// maker's mark and becomes something printed over the music.
	if n := strings.Count(page, `class="stamp"`); n != 1 {
		t.Errorf("the mark is on %d of the %d printed pages, want 1", n, sections)
	}
	if strings.Index(page, `class="stamp"`) > strings.Index(page, "</section>") {
		t.Error("the mark is not on the first page")
	}
}
