/*
 * printname_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a printed score says its lines are.
 *
 * The editor leaves a single staff unlabelled: there is no other line to
 * tell it from and the word only pushes the music right. Paper is not the
 * screen. A printed sheet is read away from the program that knows, often
 * by somebody who did not write it, and "which instrument is this" is not a
 * question to leave to the person holding it. Asked for as "when I print a
 * score make sure the instrument name is printed if there is only one staff
 * too".
 */

package app

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/pdf"
)

// soloNamed is a one-staff score whose staff has a name.
func soloNamed(t *testing.T, name string) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/staff?part=0&name=" + name)
	for i := 0; i < 8; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	}
	if n := len(h.app.score.Parts); n != 1 {
		t.Fatalf("the score has %d staves, and this is about the one-staff case", n)
	}
	return h
}

// words is every line of text on a page of a PDF, as the letters it is set
// in. The engraver's own font is in there too, so the music comes back as
// the names of its glyphs and only the words have runes.
func words(t *testing.T, raw string) []string {
	t.Helper()
	return wordsOn(t, raw, 0)
}

// wordsOn is the same for a page other than the first.
func wordsOn(t *testing.T, raw string, n int) []string {
	t.Helper()
	doc, err := pdf.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("the PDF cannot be read back: %v", err)
	}
	page, err := doc.Page(n)
	if err != nil {
		t.Fatalf("page %d: %v", n+1, err)
	}
	// Letters that sit on one line and run left to right are one word.
	var out []string
	var line strings.Builder
	lastX, lastY := -1000.0, -1000.0
	for _, g := range page.Glyphs {
		r := g.Rune
		if r == 0 && g.Code >= 32 && g.Code < 127 {
			// One of the standard text fonts, which carries no map of its
			// own: the code in the string is the letter.
			r = rune(g.Code)
		}
		// Nothing but words. The music is set in the engraver's own font,
		// whose glyphs land in the private use area.
		if r == 0 || (r >= 0xE000 && r <= 0xF8FF) {
			continue
		}
		if g.Y != lastY || g.X < lastX || g.X > lastX+3*g.Size {
			if line.Len() > 0 {
				out = append(out, line.String())
			}
			line.Reset()
		}
		line.WriteRune(r)
		lastX, lastY = g.X, g.Y
	}
	if line.Len() > 0 {
		out = append(out, line.String())
	}
	return out
}

func TestAPrintedScoreOfOneStaffSaysWhatItIs(t *testing.T) {
	h := soloNamed(t, "Flute")
	got := words(t, h.get("/api/pdf"))
	if !has(got, "Flute") {
		t.Errorf("the printed page does not say what the staff is: %q", got)
	}
}

// Once, at the head, the way a solo is printed — not "Flu." down the side
// of every system, which on a piece of one line is a word in the way on
// every page of it.
func TestTheNameOfALoneStaffIsAHeadingAndNotALabel(t *testing.T) {
	h := soloNamed(t, "Flute")
	// Enough music for several systems.
	for i := 1; i < 40; i++ {
		h.post("/api/addmeasure")
	}
	if n := h.app.score.MeasureCount(); n < 20 {
		t.Fatalf("the score is %d bars long, which may be one system", n)
	}
	got := words(t, h.get("/api/pdf"))
	n := 0
	for _, w := range got {
		if strings.HasPrefix(w, "Flu") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the staff is named %d times on the first page, want once: %q",
			n, got)
	}
}

// And the printed page and the exported file are one document: what the
// print path draws has to be what the PDF draws, or printing from the
// window and sending somebody the file are two different scores.
func TestThePrintedPageNamesTheStaffTheSameWay(t *testing.T) {
	h := soloNamed(t, "Flute")
	page := h.get("/print")
	if !strings.Contains(page, ">Flute<") {
		t.Error("the page that is printed does not name the staff")
	}
}

func has(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}

// A part keeps the subtitle of the score it came from.
//
// It used to be overwritten with the name of the instrument, which is a
// second place to say what is already in the margin of the part's first
// page — and it was said instead of what the piece is, so a part of "From
// Walt Disney Pictures' TARZAN" came out subtitled "Alto sax" and the
// player's copy no longer said where the music is from. Reported as "there
// is no need to remove the subtitle and replace it with the instrument
// name".
func TestAPartKeepsWhatThePieceSaysItIs(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/header?title=A+Song&subtitle=" +
		url.QueryEscape("From a film"))
	h.post("/api/staff?part=0&name=Flute")
	h.post("/api/addstaff?part=1&name=Alto+sax")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=whole&kind=note")
	h.post("/api/insert?part=1&measure=0&at=-1&step=4&dur=whole&kind=note")

	part := onePart(h.app.score, h.app.score.Parts[1])
	if got := part.Identity.Subtitle; got != "From a film" {
		t.Errorf("the part is subtitled %q, want the score's own subtitle", got)
	}

	// And on paper. The document is the score and then a page a part:
	// the last of them is the alto's, and it carries both the subtitle
	// under the title and the instrument's name in the margin.
	raw := h.get("/api/pdf?what=parts")
	doc, err := pdf.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	last := len(doc.Pages()) - 1
	if last < 2 {
		t.Fatalf("the document has %d pages, want the score and two parts",
			last+1)
	}
	got := wordsOn(t, raw, last)
	if !has(got, "From a film") {
		t.Errorf("the part does not say what the piece is: %q", got)
	}
	if !has(got, "Alto sax") {
		t.Errorf("the part does not say which instrument it is: %q", got)
	}
}
