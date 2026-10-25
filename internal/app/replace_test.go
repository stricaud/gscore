/*
 * replace_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Clicking a note and typing is correcting it, not adding to it.
//
// Writing in front of what was picked up pushed the rest of the bar along,
// over the barline and into bars that had to be made to hold it — so fixing
// one wrong note in a finished piece rewrote everything after it.

func (h *harness) elements(part, measure int) []model.Element {
	h.t.Helper()
	h.app.mu.RLock()
	defer h.app.mu.RUnlock()
	p := h.app.score.Parts[part]
	if measure >= len(p.Measures) {
		return nil
	}
	return p.Measures[measure].Voices[0].Elements
}

func steps(els []model.Element) string {
	out := ""
	for _, el := range els {
		switch e := el.(type) {
		case *model.Note:
			out += fmt.Sprintf("n%d ", e.Pitch.MIDI())
		case *model.Rest:
			out += "r "
		default:
			out += "? "
		}
	}
	return out
}

// fourQuarters writes a full bar, which is the case that used to overflow.
func fourQuarters(h *harness) {
	for _, step := range []int{0, 2, 4, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=quarter&kind=note", step))
	}
}

func TestTypingOverASelectedNoteReplacesIt(t *testing.T) {
	h := newHarness(t)
	fourQuarters(h)
	before := steps(h.elements(0, 0))

	out := h.post("/api/insert?part=0&measure=0&at=1&step=10&dur=quarter&kind=note&replace=1")

	els := h.elements(0, 0)
	if len(els) != 4 {
		t.Fatalf("the bar holds %d notes, want the four it started with: %s",
			len(els), steps(els))
	}
	// Only the second one changed.
	if steps(els) == before {
		t.Error("nothing was replaced")
	}
	n, ok := els[1].(*model.Note)
	if !ok {
		t.Fatalf("the second element is %T", els[1])
	}
	// Ten steps above the bottom line of a treble staff — E4 — is A5.
	if got := n.Pitch.MIDI(); got != 81 {
		t.Errorf("the replacement sounds %d", got)
	}
	// And nothing was pushed into a bar of its own.
	if n := len(h.app.score.Parts[0].Measures); n != 1 {
		t.Errorf("the score grew to %d bars", n)
	}
	// The cursor is left after what was written over, ready for the next
	// thing rather than for another replacement.
	if out["at"] != float64(2) {
		t.Errorf("the cursor came back at %v, want 2", out["at"])
	}
}

// Writing at the cursor still writes: only what was pointed at is replaced,
// and the two have to stay different or there is no way to add a note.
func TestWritingWithoutReplaceStillInserts(t *testing.T) {
	h := newHarness(t)
	fourQuarters(h)

	h.post("/api/insert?part=0&measure=0&at=1&step=10&dur=quarter&kind=note")

	if els := h.elements(0, 0); len(els) != 4 {
		t.Fatalf("the first bar holds %d notes", len(els))
	}
	// The bar was full, so the fifth note went over the barline.
	if n := len(h.app.score.Parts[0].Measures); n < 2 {
		t.Fatalf("the music did not spill into a second bar")
	}
	if els := h.elements(0, 1); len(els) != 1 {
		t.Errorf("the second bar holds %d notes, want the one pushed over", len(els))
	}
}

// A longer note than the one it replaces still has to go somewhere.
func TestAReplacementThatIsTooLongSpillsOver(t *testing.T) {
	h := newHarness(t)
	fourQuarters(h)

	h.post("/api/insert?part=0&measure=0&at=1&step=10&dur=half&kind=note&replace=1")

	first := h.elements(0, 0)
	if len(first) != 3 {
		t.Fatalf("the first bar holds %s, want three", steps(first))
	}
	if els := h.elements(0, 1); len(els) != 1 {
		t.Errorf("the bar after holds %s, want the note that no longer fitted",
			steps(els))
	}
}

// Replacing past the end of the bar is not replacing anything, so it writes.
func TestReplacingNothingWritesInstead(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=quarter&kind=note")

	h.post("/api/insert?part=0&measure=0&at=9&step=4&dur=quarter&kind=note&replace=1")

	if els := h.elements(0, 0); len(els) != 2 {
		t.Errorf("the bar holds %s, want both notes", steps(els))
	}
}
