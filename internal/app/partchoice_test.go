/*
 * partchoice_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Exporting some of the parts rather than all of them.
 *
 * A piece for eleven players is a score and eleven parts, and "Score and
 * parts" used to be all twelve documents or nothing. Somebody who needs the
 * second violin's copy to send to the second violin wants one of them, so
 * the request may name the staves it wants and may leave the score itself
 * out. Asked for as "a user friendly way to select the instrument(s) I want
 * to export to pdf, I might need only one or two parts".
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/pdf"
)

// namedTrio is a score of three named staves, a whole note apiece.
func namedTrio(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/header?title=A+Trio")
	h.post("/api/staff?part=0&name=Flute")
	h.post("/api/addstaff?part=1&name=Oboe")
	h.post("/api/addstaff?part=2&name=Bassoon")
	for i := 0; i < 3; i++ {
		h.post("/api/insert?part=" + string(rune('0'+i)) +
			"&measure=0&at=-1&step=6&dur=whole&kind=note")
	}
	if n := len(h.app.score.Parts); n != 3 {
		t.Fatalf("the score has %d staves, want three", n)
	}
	return h
}

// labels is the word in the margin of each page of a PDF — the part's
// name where the page is a part's first — or "" where there is none.
func labels(t *testing.T, raw string) []string {
	t.Helper()
	doc, err := pdf.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("the PDF cannot be read back: %v", err)
	}
	var out []string
	for n := range doc.Pages() {
		got := wordsOn(t, raw, n)
		name := ""
		for _, w := range got {
			switch w {
			case "Flute", "Oboe", "Bassoon":
				name = w
			}
		}
		out = append(out, name)
	}
	return out
}

func TestEverythingIsExportedUnlessTheRequestSaysWhich(t *testing.T) {
	h := namedTrio(t)
	got := labels(t, h.get("/api/pdf?what=parts"))
	// The score, then a page a part. The score's page names its staves
	// at the head of the system, so the first page carries a name too;
	// what matters is that three parts follow it.
	if len(got) != 4 {
		t.Fatalf("the document has %d pages, want the score and three parts: %q",
			len(got), got)
	}
	if got[1] != "Flute" || got[2] != "Oboe" || got[3] != "Bassoon" {
		t.Errorf("the parts come out as %q, want flute, oboe, bassoon", got[1:])
	}
}

func TestOnlyTheStavesAskedForAreExported(t *testing.T) {
	h := namedTrio(t)
	got := labels(t, h.get("/api/pdf?what=parts&parts=0,2"))
	if len(got) != 3 {
		t.Fatalf("the document has %d pages, want the score and two parts: %q",
			len(got), got)
	}
	if got[1] != "Flute" || got[2] != "Bassoon" {
		t.Errorf("the parts come out as %q, want flute and bassoon", got[1:])
	}
}

func TestThePartAloneIsOnePartWithNoScoreInFront(t *testing.T) {
	h := namedTrio(t)
	got := labels(t, h.get("/api/pdf?what=parts&parts=1&score=0"))
	if len(got) != 1 || got[0] != "Oboe" {
		t.Errorf("the document is %q, want the oboe's page and nothing else", got)
	}
}

// Asking for nothing is told, not written. An empty PDF would be opened
// and looked at before anybody understood what happened.
func TestExportingNothingIsRefusedInWords(t *testing.T) {
	h := namedTrio(t)
	msg := h.getFails("/api/pdf?what=parts&score=0&parts=7")
	if msg == "" {
		t.Fatal("an export of no documents at all was written")
	}
}

// The print view takes the same choice, read the same way.
func TestThePrintViewTakesTheSameChoice(t *testing.T) {
	h := namedTrio(t)
	all := countSections(h.get("/print?what=parts"))
	one := countSections(h.get("/print?what=parts&parts=1&score=0"))
	if all != 4 || one != 1 {
		t.Errorf("the print view has %d sections for everything and %d for one part, want 4 and 1",
			all, one)
	}
}

func countSections(page string) int {
	n := 0
	for i := 0; i+9 <= len(page); i++ {
		if page[i:i+9] == "<section>" {
			n++
		}
	}
	return n
}
