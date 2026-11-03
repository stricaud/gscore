/*
 * group_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Joining staves.
 *
 * A bracket gathers a family of staves and a brace gathers the staves one
 * player reads. Which staves is a question with more answers than "the two
 * at the top": a score has strings, woodwind and a rhythm section, and each
 * of them is a run somewhere in the middle of the page.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func stavesFor(t *testing.T, h *harness, n int) {
	t.Helper()
	for len(h.app.score.Parts) < n {
		h.post("/api/addstaff?sound=strings.violin")
	}
}

func TestABracketCanCoverAnyRunOfStaves(t *testing.T) {
	h := newHarness(t)
	stavesFor(t, h, 6)

	h.post("/api/group?first=2&last=4&symbol=bracket")
	if n := len(h.app.score.Groups); n != 1 {
		t.Fatalf("%d groups", n)
	}
	g := h.app.score.Groups[0]
	if g.First != 2 || g.Last != 4 {
		t.Errorf("the bracket covers %d..%d", g.First, g.Last)
	}
	if g.Symbol != model.GroupBracket {
		t.Errorf("it is a %v", g.Symbol)
	}
	// A bracket does not run the barlines through the gap. That says "one
	// player reads both of these", which is a brace's job and is wrong
	// for a family of different instruments.
	if g.Barlines {
		t.Error("the bracket runs the barlines through")
	}
}

// The strings and the woodwind are two groups on one page, and neither is
// at the top of it.
func TestAScoreCanCarrySeveralGroups(t *testing.T) {
	h := newHarness(t)
	stavesFor(t, h, 8)

	h.post("/api/group?first=0&last=2&symbol=bracket")
	h.post("/api/group?first=3&last=5&symbol=bracket")
	h.post("/api/group?first=6&last=7&symbol=brace")
	if n := len(h.app.score.Groups); n != 3 {
		t.Fatalf("%d groups, want 3", n)
	}
}

// Asking again for the same staves changes the group rather than drawing a
// second one over the first. Appending left both symbols on the page, and
// the only way out was to take the group away and start again.
func TestJoiningTheSameStavesTwiceChangesTheGroup(t *testing.T) {
	h := newHarness(t)
	stavesFor(t, h, 4)

	h.post("/api/group?first=1&last=3&symbol=bracket")
	h.post("/api/group?first=1&last=3&symbol=brace")

	if n := len(h.app.score.Groups); n != 1 {
		t.Fatalf("%d groups, want the one", n)
	}
	if got := h.app.score.Groups[0].Symbol; got != model.GroupBrace {
		t.Errorf("the group is a %v", got)
	}
}

func TestAGroupNeedsTwoStaves(t *testing.T) {
	h := newHarness(t)
	stavesFor(t, h, 3)

	if got := h.postFails("/api/group?first=1&last=1&symbol=bracket"); got == "" {
		t.Error("a group of one staff was allowed")
	}
	if n := len(h.app.score.Groups); n != 0 {
		t.Errorf("%d groups", n)
	}
}

func TestAGroupCanBeTakenAway(t *testing.T) {
	h := newHarness(t)
	stavesFor(t, h, 4)

	h.post("/api/group?first=1&last=3&symbol=bracket")
	h.post("/api/group?first=1&last=3&symbol=none")
	if n := len(h.app.score.Groups); n != 0 {
		t.Errorf("%d groups left", n)
	}
}
