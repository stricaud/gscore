/*
 * numbering_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the bar numbers are printed.
 *
 * Numbering was on or off and the engraver decided where they fell. That
 * is right for a song and not for a long score, where rehearsing is most
 * of what the numbers are for.
 */

package app

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// bars writes n bars of a quarter note each.
func (h *harness) bars(n int) {
	for i := 0; i < n; i++ {
		if i > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, i, 2, "quarter")
	}
}

// printed is every bar number drawn, as the numerals they are.
func printed(lay *engrave.Layout) []string {
	var out []string
	for _, t := range lay.Texts {
		if t.S == "" {
			continue
		}
		if strings.IndexFunc(t.S, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			continue
		}
		out = append(out, t.S)
	}
	return out
}

func numberingNow(t *testing.T, h *harness) numberingState {
	t.Helper()
	var s numberingState
	if err := json.Unmarshal([]byte(h.get("/api/numbering")), &s); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	return s
}

// The default is what it always was, which is what every score that never
// opens the window relies on.
func TestNumberingStartsAsItAlwaysWas(t *testing.T) {
	h := newHarness(t)
	h.bars(8)

	got := numberingNow(t, h)
	if got.Every != 0 || got.Boxed || len(got.Restarts) != 0 {
		t.Errorf("a new score says %+v", got)
	}

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	// The strip numbers every bar but the first, which needs no telling,
	// so the numbers run 2, 3, 4 … with nothing missed out.
	//
	// Counted against what was drawn rather than against the score: the
	// strip draws one empty bar past the end of the music for the next
	// note to go in, and that bar is numbered like any other.
	numbers := printed(engrave.Engrave(h.app.score, opt))
	if len(numbers) < h.app.score.MeasureCount()-1 {
		t.Fatalf("%d numbers for %d bars: %v",
			len(numbers), h.app.score.MeasureCount(), numbers)
	}
	for i, n := range numbers {
		if n != strconv.Itoa(i+2) {
			t.Fatalf("the numbers printed were %v, which is not every bar "+
				"counting from two", numbers)
		}
	}
}

func TestEveryNthBarCarriesANumber(t *testing.T) {
	h := newHarness(t)
	h.bars(20)
	h.post("/api/numbering?every=5")

	for _, mode := range []struct {
		name string
		mode engrave.Mode
	}{{"the strip", engrave.Strip}, {"a page", engrave.Paginated}} {
		opt := engrave.DefaultOptions()
		opt.Mode = mode.mode
		got := printed(engrave.Engrave(h.app.score, opt))
		want := []string{"5", "10", "15", "20"}
		if len(got) != len(want) {
			t.Errorf("%s: %v, want %v", mode.name, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: %v, want %v", mode.name, got, want)
				break
			}
		}
	}
}

// Boxed means a rectangle round each number, which is four more lines than
// the same score draws without one.
func TestBoxedNumbersAreBoxed(t *testing.T) {
	h := newHarness(t)
	h.bars(6)
	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip

	before := engrave.Engrave(h.app.score, opt)
	plain, numbers := len(before.Lines), len(printed(before))
	if numbers == 0 {
		t.Fatal("no numbers were printed to box")
	}
	h.post("/api/numbering?boxed=1")
	boxed := len(engrave.Engrave(h.app.score, opt).Lines)

	// Four lines to a box.
	if boxed-plain != numbers*4 {
		t.Errorf("boxing %d numbers added %d lines, want %d",
			numbers, boxed-plain, numbers*4)
	}
	if !numberingNow(t, h).Boxed {
		t.Error("the endpoint does not say they are boxed")
	}
}

func TestTheCountCanBeStartedAgain(t *testing.T) {
	h := newHarness(t)
	h.bars(8)
	// From the fifth bar on, the piece is numbered from one again.
	h.post("/api/numbering?at=4&number=1")

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	got := printed(engrave.Engrave(h.app.score, opt))
	// The first four bars print 2, 3, 4; the fifth is called 1 and so
	// prints nothing; then the count runs up from there again. What is
	// looked for is that one fall, because that is the whole of what a
	// restart does — and looking for it rather than for the exact list
	// leaves the strip free to draw an empty bar past the end.
	if line := strings.Join(got, " "); !strings.HasPrefix(line, "2 3 4 2 3") {
		t.Errorf("the numbers printed were %v, which does not start again "+
			"at the fifth bar", got)
	}

	if n := numberingNow(t, h).Restarts["4"]; n != 1 {
		t.Errorf("the endpoint reports the restarts as %v",
			numberingNow(t, h).Restarts)
	}

	// And it can be taken away again.
	h.post("/api/numbering?at=4&number=none")
	if len(numberingNow(t, h).Restarts) != 0 {
		t.Error("the restart survived being removed")
	}
}

// A restart is written on every staff: "from bar forty" has to mean the
// same bar to everybody in the room.
func TestARestartIsWrittenOnEveryStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&clef=treble")
	h.bars(4)
	h.post("/api/numbering?at=2&number=20")

	for i, p := range h.app.score.Parts {
		if len(p.Measures) <= 2 {
			continue
		}
		got := p.Measures[2].NumberFrom
		if got == nil || *got != 20 {
			t.Errorf("staff %d has %v, want 20", i, got)
		}
	}
}

func TestNonsenseNumberingIsRefused(t *testing.T) {
	h := newHarness(t)
	h.bars(4)
	for _, q := range []string{
		"every=-1", "every=abc", "every=100000",
		"at=99&number=1", "at=1&number=-5", "at=1&number=xyz",
	} {
		if why := h.postFails("/api/numbering?" + q); why == "" {
			t.Errorf("%q was accepted", q)
		}
	}
	got := numberingNow(t, h)
	if got.Every != 0 || len(got.Restarts) != 0 {
		t.Errorf("a refused request changed something: %+v", got)
	}
}
