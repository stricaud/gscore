/*
 * tags_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a piece is, in the words it would be looked for by.
 *
 * Typed as one line and split here rather than in the editor, because a
 * shelf built from a directory of scores has to agree with the window
 * about where one tag ends and the next begins.
 */

package app

import (
	"net/url"
	"strings"
	"testing"
)

func TestTagsAreReadTheWayPeopleWriteThem(t *testing.T) {
	for _, c := range []struct {
		line string
		want []string
	}{
		{"advent, flute duet, grade 3", []string{"advent", "flute duet", "grade 3"}},
		// A phrase is one tag: splitting on spaces would make three of
		// "flute duet for Anna".
		{"flute duet for Anna", []string{"flute duet for Anna"}},
		// Blanks, a trailing comma and stray space are not tags.
		{"  advent ,, , noel ,", []string{"advent", "noel"}},
		// The same tag twice is once, whatever the capitals, and the
		// spelling that survives is the one written first.
		{"Advent, advent, ADVENT", []string{"Advent"}},
		{"", nil},
		{"   ", nil},
	} {
		got := parseTags(c.line)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%q gave %v, want %v", c.line, got, c.want)
		}
	}
}

func TestTagsAreSetAndReportedBack(t *testing.T) {
	h := newHarness(t)
	h.post("/api/header?tags=" + url.QueryEscape("Advent, flute duet"))

	if got := h.app.score.Tags; len(got) != 2 || got[0] != "Advent" {
		t.Fatalf("the score carries %v", got)
	}
	// The editor is told, so the window can open on what is there.
	if info := h.get("/api/info"); !strings.Contains(info, "flute duet") {
		t.Error("the tags are not in what the editor is told")
	}

	// Only what is sent is changed: setting a title must not wipe them.
	h.post("/api/header?title=" + url.QueryEscape("Noel"))
	if got := h.app.score.Tags; len(got) != 2 {
		t.Errorf("setting the title left the tags as %v", got)
	}

	// And an empty line clears them, which is how a tag is removed.
	h.post("/api/header?tags=")
	if got := h.app.score.Tags; len(got) != 0 {
		t.Errorf("the tags survived being cleared: %v", got)
	}
}
