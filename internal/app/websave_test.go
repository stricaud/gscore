/*
 * websave_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/*
 * Saving where there is no disk.
 *
 * gscore in a page is the whole program with nothing underneath it: no
 * machine to write to, no path to write at. Save there means "give me the
 * file", and the panel that asks where it goes is the browser's own — which
 * is why saving on the web is always a save-as.
 *
 * It answered "say where to save it" instead, and the menu entry did
 * nothing. Reported as "on the web the save and the exports do not work".
 */

// served is gscore answering as it does when it is not on your machine:
// reached over a network, or compiled into a page.
func served(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.app.local = false
	h.app.native = false
	return h
}

// save asks for the document and hands back the answer.
func (h *harness) save(query string) *httptest.ResponseRecorder {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/save"+query, nil))
	return rec
}

func TestSavingInAPageHandsBackTheFile(t *testing.T) {
	h := served(t)
	h.note(0, 0, 2, "quarter")

	res := h.save("")
	if res.Code != http.StatusOK {
		t.Fatalf("save answered %d %s", res.Code, strings.TrimSpace(res.Body.String()))
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, ".gscore") {
		t.Errorf("the answer is %q, and nothing tells the browser to keep it", got)
	}
	// A .gscore is a zip, and one announced as text arrives broken.
	if got := res.Header().Get("Content-Type"); strings.HasPrefix(got, "text/") {
		t.Errorf("the file is offered as %q", got)
	}
	if _, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()),
		int64(res.Body.Len())); err != nil {
		t.Errorf("what came back is not a .gscore: %v", err)
	}
}

// And the score is saved, so the star beside the name goes away.
func TestSavingInAPageLeavesNothingUnsaved(t *testing.T) {
	h := served(t)
	h.note(0, 0, 2, "quarter")
	if !h.app.unsaved {
		t.Fatal("writing a note left the score looking saved")
	}

	h.save("")

	if h.app.unsaved {
		t.Error("the score still counts as unsaved after it was handed over")
	}
}

// Every format the menu offers comes back as itself.
func TestSavingInAPageWritesEveryFormat(t *testing.T) {
	for _, tc := range []struct {
		as    string
		magic string // what the first bytes of that format look like
	}{
		// A .gscore leads with a PDF of the score, so that the one file is
		// both; the zip is behind it and the test above reads it.
		{"gscore", "%PDF"},
		{"mxl", "PK"},
		{"abc", "X:"},
		{"musicxml", "<?xml"},
		{"ly", "\\version"},
	} {
		h := served(t)
		res := h.save("?as=" + tc.as)
		if res.Code != http.StatusOK {
			t.Errorf("%s: %d %s", tc.as, res.Code, strings.TrimSpace(res.Body.String()))
			continue
		}
		if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, "."+tc.as) {
			t.Errorf("%s: offered as %q", tc.as, got)
		}
		if body := res.Body.String(); !strings.HasPrefix(body, tc.magic) {
			head := body
			if len(head) > 24 {
				head = head[:24]
			}
			t.Errorf("%s: the file starts %q, want it to start %q", tc.as, head, tc.magic)
		}
	}
}

// A drawing or a recording is not the document.
//
// Asking for a PDF of the score leaves the edits still unsaved, on the web
// exactly as on a disk: a picture of the music is not the music.
func TestAPictureOfTheScoreIsNotTheScore(t *testing.T) {
	h := served(t)
	h.note(0, 0, 2, "quarter")

	res := h.save("?as=svg")
	if res.Code != http.StatusOK {
		t.Fatalf("svg: %d %s", res.Code, strings.TrimSpace(res.Body.String()))
	}
	if !h.app.unsaved {
		t.Error("exporting a drawing counted as saving the score")
	}
}

// A format gscore does not write is refused as the caller's mistake, not
// reported as something that went wrong inside it.
func TestAFormatGscoreDoesNotWriteIsRefused(t *testing.T) {
	h := served(t)
	if res := h.save("?as=docx"); res.Code != http.StatusBadRequest {
		t.Errorf("asking for .docx answered %d, want 400", res.Code)
	}
}
