/*
 * coveroption_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"regexp"
	"strings"
	"testing"
)

/*
 * A cover is wanted on some copies and not on others.
 *
 * It is the sheet a publisher wraps a score in, so it belongs on the copy
 * somebody is handed — and it is in the way on a working copy, the one read
 * at a stand, marked up, or sent to somebody to look over. Which of the two
 * an export is is not something the score can know, and the alternative was
 * taking the cover out of the score to get a PDF without one and putting it
 * back afterwards: editing the piece to change how one copy of it prints.
 */

// pageCount is how many sheets a PDF holds, read off the file.
func pageCount(raw string) int {
	return len(regexp.MustCompile(`/Type /Page[ /]`).FindAllString(raw, -1))
}

func TestTheCoverIsTheFirstPageUnlessItIsTurnedOff(t *testing.T) {
	h := newHarness(t)
	plain := pageCount(h.get("/api/pdf"))
	if plain < 1 {
		t.Fatalf("a score with no cover came out as %d pages", plain)
	}

	h.postBody("/api/cover", fakePNG(60, 80))
	if got := pageCount(h.get("/api/pdf")); got != plain+1 {
		t.Errorf("with a cover the PDF is %d pages, wanted %d", got, plain+1)
	}
	if got := pageCount(h.get("/api/pdf?cover=0")); got != plain {
		t.Errorf("asked without the cover it is %d pages, wanted %d", got, plain)
	}
	// And asking for it again gets it back: leaving it out is a choice
	// about one export and not a change to the score.
	if h.app.score.Cover == nil {
		t.Fatal("exporting without the cover took it out of the score")
	}
	if got := pageCount(h.get("/api/pdf?cover=1")); got != plain+1 {
		t.Errorf("asked for again it is %d pages, wanted %d", got, plain+1)
	}
}

func TestTurningOffACoverThatIsNotThereChangesNothing(t *testing.T) {
	h := newHarness(t)
	with := pageCount(h.get("/api/pdf"))
	if got := pageCount(h.get("/api/pdf?cover=0")); got != with {
		t.Errorf("a score with no cover came out as %d pages without one and "+
			"%d pages with", got, with)
	}
}

func TestTheStampIsPrintedEitherWay(t *testing.T) {
	// Only the cover is a page of its own. The stamp sits in the bottom
	// margin of the last page, out of the way of the music, so there is
	// nothing to be gained by leaving it out and no question to ask.
	h := newHarness(t)
	h.postBody("/api/stamp?align=right", fakePNG(200, 40))
	before := pageCount(h.get("/api/pdf"))
	if got := pageCount(h.get("/api/pdf?cover=0")); got != before {
		t.Errorf("without the cover the PDF is %d pages, wanted %d", got, before)
	}
	if h.app.score.Stamp == nil {
		t.Error("the stamp was dropped")
	}
}

// TestTheExportDialogOffersTheCoverOnlyWhenThereIsOne reads the page rather
// than the server: the choice is in the dialog, and a checkbox wired to
// nothing is the way a setting comes to do nothing at all.
func TestTheExportDialogOffersTheCoverOnlyWhenThereIsOne(t *testing.T) {
	html := readUI(t, "index.html")
	if !strings.Contains(html, `id="print-cover-row" hidden`) {
		t.Error("the cover row is not hidden to start with, so a score " +
			"without a cover is asked about one")
	}
	if !strings.Contains(html, `id="print-cover"`) {
		t.Fatal("the dialog has no cover checkbox")
	}

	js := readUI(t, "app.js")
	for _, want := range []string{
		`$("#print-cover-row").hidden`, // shown or hidden by what the score has
		`state.info?.cover`,            // and the answer comes from the score
		`q.set("cover", "0")`,          // and reaches the server
	} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js never does %s", want)
		}
	}
}
