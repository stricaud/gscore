/*
 * pdfimport_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// enginePDF is a small engraved PDF to open: gscore's own export, so the
// test needs no fixture checked in and reads something it is certain is an
// engraving rather than a scan.
func enginePDF(t *testing.T) []byte {
	t.Helper()
	for _, p := range []string{
		"../format/engraved/testdata/durations.pdf",
		"../format/engraved/testdata/signature.pdf",
	} {
		if b, err := os.ReadFile(filepath.FromSlash(p)); err == nil {
			return b
		}
	}
	t.Skip("no engraved PDF to read")
	return nil
}

// Opening a PDF has to keep the bytes, or the reading can be complained
// about and not corrected.
func TestAPDFCanBeReadAgain(t *testing.T) {
	h := newHarness(t)
	data := enginePDF(t)

	h.postBody("/api/open?name=read.pdf", string(data))

	// The survey must answer, and must describe a reading rather than a
	// score: staves with positions, not measures with notes.
	var got struct {
		Filename string `json:"filename"`
		Survey   struct {
			Parts     int `json:"parts"`
			PerSystem int `json:"perSystem"`
			Detected  int `json:"detected"`
			Pages     []struct {
				Width  float64 `json:"width"`
				Height float64 `json:"height"`
				Staves []struct {
					Inter  float64    `json:"inter"`
					Y      [5]float64 `json:"y"`
					System int        `json:"system"`
					Part   int        `json:"part"`
				} `json:"staves"`
				Marks []struct {
					Kind  string `json:"kind"`
					Pitch string `json:"pitch"`
				} `json:"marks"`
			} `json:"pages"`
		} `json:"survey"`
	}
	body := h.get("/api/pdfsurvey")
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("the survey is not JSON: %v\n%s", err, body)
	}
	if len(got.Survey.Pages) == 0 {
		t.Fatal("the survey describes no pages")
	}
	p := got.Survey.Pages[0]
	if p.Width <= 0 || p.Height <= 0 {
		t.Errorf("page measures %gx%g, so nothing can be drawn on it",
			p.Width, p.Height)
	}
	if len(p.Staves) == 0 {
		t.Fatal("the survey found no staves")
	}
	notes := 0
	for _, m := range p.Marks {
		if m.Kind == "note" {
			notes++
			if m.Pitch == "" {
				t.Error("a notehead was reported without the pitch read")
			}
		}
	}
	if notes == 0 {
		t.Error("the survey reports no noteheads")
	}
	if got.Survey.Detected == 0 {
		t.Error("the survey does not say what the page-reading guess was")
	}

	// And it must be possible to overrule the guess and read again.
	out := h.postBody("/api/pdfreread?staves=1", "")
	if n, _ := out["staves"].(float64); int(n) != 1 {
		t.Errorf("read again with %v staves per system, asked for 1",
			out["staves"])
	}
	if n, _ := out["parts"].(float64); int(n) != 1 {
		t.Errorf("one staff per system gave %v parts", out["parts"])
	}
}

// The bytes are kept for a PDF and for nothing else. A copy of every file
// ever opened would be a memory leak with a feature's name on it, and a
// survey of an ABC file would be an answer to a question nobody asked.
func TestOnlyAPDFIsKeptForRereading(t *testing.T) {
	h := newHarness(t)
	data := enginePDF(t)

	h.postBody("/api/open?name=read.pdf", string(data))
	if body := h.get("/api/pdfsurvey"); !strings.Contains(body, "pages") {
		t.Fatalf("a PDF was opened and the survey said %q", body)
	}

	// Opening anything else must let it go.
	h.postBody("/api/open?name=after.abc", "X:1\nT:After\nK:C\nabcd|\n")
	if body := h.getFails("/api/pdfsurvey"); !strings.Contains(body, "no PDF") {
		t.Errorf("after opening an ABC file the survey said %q", body)
	}
	if out := h.postFails("/api/pdfreread?staves=2"); !strings.Contains(out, "no PDF") {
		t.Errorf("reading again with no PDF open said %q", out)
	}
}

// A staves-per-system that is not a number, or is absurd, is refused rather
// than quietly taken as nought — which would look like working and read the
// file the way it was already read.
func TestAbsurdStaffCountsAreRefused(t *testing.T) {
	h := newHarness(t)
	data := enginePDF(t)
	h.postBody("/api/open?name=read.pdf", string(data))

	for _, bad := range []string{"", "two", "-1", "65", "1.5"} {
		out := h.postFails("/api/pdfreread?staves=" + bad)
		if !strings.Contains(out, "whole number") {
			t.Errorf("staves=%q was answered with %q", bad, out)
		}
	}
}
