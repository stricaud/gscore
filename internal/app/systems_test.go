/*
 * systems_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the lines of music end.
 *
 * The engraver fits as many bars to a line as will go. That is right until
 * somebody wants otherwise — four bars to a line through a study, a page
 * turn moved off a passage nobody can spare a hand for — and none of it
 * can be worked out from the notes.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// firstBars is the first bar of each line, as the engraver laid it out.
func firstBars(lay *engrave.Layout) []int {
	var out []int
	seen := map[float64]bool{}
	for _, hit := range lay.Hits {
		if hit.Part != 0 || seen[hit.StaffTop] {
			continue
		}
		seen[hit.StaffTop] = true
		out = append(out, hit.Measure)
	}
	return out
}

func systemsNow(t *testing.T, h *harness) systemsState {
	t.Helper()
	var s systemsState
	if err := json.Unmarshal([]byte(h.get("/api/systems")), &s); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	return s
}

func TestBarsFallWhereTheyWillUntilToldOtherwise(t *testing.T) {
	h := newHarness(t)
	h.bars(24)
	if got := systemsNow(t, h); len(got.Breaks) != 0 {
		t.Errorf("a new score already has lines set: %v", got.Breaks)
	}
}

// The whole of what was asked for: so many bars to a line.
func TestBarsCanBeLaidOutSoManyToALine(t *testing.T) {
	h := newHarness(t)
	h.bars(24)
	h.post("/api/systems?fit=4")

	got := systemsNow(t, h)
	want := []int{4, 8, 12, 16, 20}
	if len(got.Breaks) < len(want) {
		t.Fatalf("the breaks are %v, want at least %v", got.Breaks, want)
	}
	for i, bar := range want {
		if got.Breaks[i] != bar {
			t.Fatalf("the breaks are %v, want them every four from %v",
				got.Breaks, want)
		}
	}

	// And the engraver draws it that way.
	opt := engrave.DefaultOptions()
	lines := firstBars(engrave.Engrave(h.app.score, opt))
	if len(lines) < 2 {
		t.Fatalf("the score came out as %d lines", len(lines))
	}
	for i, at := range lines {
		if at != i*4 {
			t.Errorf("the lines begin at %v, want every fourth bar", lines)
			break
		}
	}
}

// A line that has been set is a line the engraver no longer moves, which
// is the other half of what "fit" is for.
func TestASetLineIsNotMovedAbout(t *testing.T) {
	h := newHarness(t)
	h.bars(24)
	opt := engrave.DefaultOptions()
	loose := firstBars(engrave.Engrave(h.app.score, opt))

	h.post("/api/systems?fit=3")
	tight := firstBars(engrave.Engrave(h.app.score, opt))

	if len(tight) <= len(loose) {
		t.Errorf("three bars to a line gave %d lines and letting them fall "+
			"gave %d; the breaks were not obeyed", len(tight), len(loose))
	}
}

func TestOneLineCanBeSetAndUnset(t *testing.T) {
	h := newHarness(t)
	h.bars(12)

	h.post("/api/systems?at=5")
	if got := systemsNow(t, h); len(got.Breaks) != 1 || got.Breaks[0] != 5 {
		t.Fatalf("the breaks are %v, want just bar 5", got.Breaks)
	}
	// On every staff, because a line of music is a line on every staff.
	for i, p := range h.app.score.Parts {
		if len(p.Measures) > 5 && !p.Measures[5].SystemBreak {
			t.Errorf("staff %d has no break at bar 5", i)
		}
	}

	h.post("/api/systems?at=5&on=0")
	if got := systemsNow(t, h); len(got.Breaks) != 0 {
		t.Errorf("the break survived being taken away: %v", got.Breaks)
	}
}

func TestLettingItReflowTakesEveryBreakAway(t *testing.T) {
	h := newHarness(t)
	h.bars(16)
	h.post("/api/systems?fit=2")
	if len(systemsNow(t, h).Breaks) == 0 {
		t.Fatal("nothing was set to clear")
	}
	h.post("/api/systems?clear=1")
	if got := systemsNow(t, h); len(got.Breaks) != 0 {
		t.Errorf("%v survived", got.Breaks)
	}
}

// Only the stretch asked for, so that laying out one section does not undo
// the one before it.
func TestLayingOutAStretchLeavesTheRestAlone(t *testing.T) {
	h := newHarness(t)
	h.bars(20)
	h.post("/api/systems?at=2")
	h.post("/api/systems?fit=3&from=8&to=16&clear=1")

	got := systemsNow(t, h)
	if len(got.Breaks) == 0 || got.Breaks[0] != 2 {
		t.Errorf("the break at bar 2, outside the stretch, is gone: %v", got.Breaks)
	}
	for _, at := range got.Breaks {
		if at > 2 && (at < 8 || at > 16) {
			t.Errorf("a break was written at bar %d, outside the stretch: %v",
				at, got.Breaks)
		}
	}
}

func TestNonsenseLayoutIsRefused(t *testing.T) {
	h := newHarness(t)
	h.bars(8)
	for _, q := range []string{
		"fit=0", "fit=abc", "fit=99", "at=99", "at=-1",
		// The first bar begins a line already; a break there could never
		// be seen or taken away again.
		"at=0",
	} {
		if why := h.postFails("/api/systems?" + q); why == "" {
			t.Errorf("%q was accepted", q)
		}
	}
	if got := systemsNow(t, h); len(got.Breaks) != 0 {
		t.Errorf("a refused request changed something: %v", got.Breaks)
	}
}
