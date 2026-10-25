/*
 * label_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// What is written in the margins: whose part this is, and which sheet of it.
//
// A part leaves the document on its own — printed, photographed, mailed to
// one player — so its own first page has to say what it is, and its pages
// have to be numbered within it rather than within the document it came out
// of. A player holding four sheets needs to know they have all four of
// theirs, not that they are holding sheets 23 to 26 of something.

// pageText pulls the drawn strings back out of a written PDF, one entry per
// page, so that a test can read what was actually printed.
//
// The content streams are flate-compressed, so this inflates them and picks
// out the arguments of the text operators.
func pageText(t *testing.T, data []byte) []string {
	t.Helper()
	var out []string
	show := regexp.MustCompile(`\(((?:[^()\\]|\\.)*)\)\s*Tj`)
	for _, chunk := range bytes.Split(data, []byte("/FlateDecode")) {
		i := bytes.Index(chunk, []byte("stream"))
		if i < 0 {
			continue
		}
		body := chunk[i+len("stream"):]
		body = bytes.TrimLeft(body, "\r\n")
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			continue
		}
		plain, err := io.ReadAll(zr)
		zr.Close()
		if err != nil && len(plain) == 0 {
			continue
		}
		var words []string
		for _, m := range show.FindAllStringSubmatch(string(plain), -1) {
			words = append(words, strings.NewReplacer(`\(`, "(", `\)`, ")",
				`\\`, `\`).Replace(m[1]))
		}
		out = append(out, strings.Join(words, " "))
	}
	return out
}

// twoSections is a score and one part of it, as the parts export builds them.
func twoSections(t *testing.T, label string, numbers bool) []byte {
	t.Helper()
	full := ensemble(3, 200)
	one := &model.Score{Identity: full.Identity, Tempo: full.Tempo,
		Parts: []*model.Part{full.Parts[0]}}

	make := func(s *model.Score, o Options) Section {
		eo := engrave.DefaultOptions()
		eo.Mode = engrave.Paginated
		eo.ShowPartNames = len(s.Parts) > 1
		o.PageNumbers = numbers
		o, eo = Fit(s, o, eo)
		return Section{Layout: engrave.Engrave(s, eo), Opt: o}
	}
	score := make(full, DefaultOptions())
	part := make(one, DefaultOptions())
	part.Opt.Label = label

	var buf bytes.Buffer
	if err := RenderSections(&buf, []Section{score, part}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAPartIsNamedOnItsFirstPageOnly(t *testing.T) {
	pages := pageText(t, twoSections(t, "Clarinet in Bb", false))
	if len(pages) < 3 {
		t.Fatalf("%d pages, which proves nothing", len(pages))
	}
	named := 0
	for i, text := range pages {
		if strings.Contains(text, "Clarinet in Bb") {
			named++
			if i == 0 {
				t.Errorf("the score's own first page carries the part's name")
			}
		}
	}
	if named != 1 {
		t.Errorf("the part is named on %d pages, want exactly 1", named)
	}
}

// The numbers start again for each part, and say how many that part has.
func TestPageNumbersAreCountedWithinEachPart(t *testing.T) {
	data := twoSections(t, "Flute", true)
	pages := pageText(t, data)

	num := regexp.MustCompile(`\b(\d+)/(\d+)\b`)
	var seen [][2]string
	for _, text := range pages {
		if m := num.FindStringSubmatch(text); m != nil {
			seen = append(seen, [2]string{m[1], m[2]})
		}
	}
	if len(seen) < 3 {
		t.Fatalf("only %d pages were numbered: %v", len(seen), seen)
	}
	// Somewhere in the document the count starts again, which is the whole
	// claim: a part is numbered within itself.
	restarted := false
	for i := 1; i < len(seen); i++ {
		if seen[i][0] == "1" {
			restarted = true
			if seen[i][1] == seen[i-1][1] {
				t.Errorf("the numbering restarted but the total did not "+
					"change: %v then %v", seen[i-1], seen[i])
			}
		}
	}
	if !restarted {
		t.Errorf("the numbering ran straight through the document: %v", seen)
	}
}

// Turned off, nothing is written.
func TestPageNumbersCanBeTurnedOff(t *testing.T) {
	pages := pageText(t, twoSections(t, "Flute", false))
	num := regexp.MustCompile(`\b\d+/\d+\b`)
	for i, text := range pages {
		if num.MatchString(text) {
			t.Errorf("page %d was numbered although numbering was off: %q",
				i+1, text)
		}
	}
}

// And a piece of one page is not numbered "1/1", which tells nobody
// anything.
func TestASinglePageIsNotNumbered(t *testing.T) {
	s := ensemble(1, 4)
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	o := DefaultOptions()
	o.PageNumbers = true
	o, eo = Fit(s, o, eo)

	var buf bytes.Buffer
	if err := Render(&buf, engrave.Engrave(s, eo), o); err != nil {
		t.Fatal(err)
	}
	for _, text := range pageText(t, buf.Bytes()) {
		if regexp.MustCompile(`\b\d+/\d+\b`).MatchString(text) {
			t.Errorf("a one-page piece was numbered: %q", text)
		}
	}
}
