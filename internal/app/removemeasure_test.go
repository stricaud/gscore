/*
 * removemeasure_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Taking bars out.
 *
 * The one thing that could not be done to a bar. They could be added,
 * pushed in, split, joined and emptied, and an emptied bar is not a bar
 * removed: it is a bar of rest, which the player counts. Asked for as
 * "how can I remove those last two measures?" — of two spare bars at the
 * end of a piece that had no way of going.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestTheLastBarsCanBeTakenOff(t *testing.T) {
	h := newHarness(t)
	h.bars(6)
	before := h.app.score.MeasureCount()

	h.post("/api/removemeasure?measure=" + itoa(before-2) + "&count=2")

	if got := h.app.score.MeasureCount(); got != before-2 {
		t.Errorf("the score has %d bars, want %d", got, before-2)
	}
}

// A bar is a bar across the score. Taking one out of a single staff would
// leave that staff a bar short of the others for the rest of the piece.
func TestEveryStaffLosesTheBar(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=keyboard.piano")
	h.bars(5)

	h.post("/api/removemeasure?measure=1")

	want := -1
	for i, p := range h.app.score.Parts {
		if p.Mirrors() {
			continue
		}
		if want < 0 {
			want = len(p.Measures)
			continue
		}
		if len(p.Measures) != want {
			t.Errorf("staff %d has %d bars, staff 0 has %d",
				i, len(p.Measures), want)
		}
	}
}

// The music after it moves up, which is the whole point: the bar is gone,
// not blanked.
func TestWhatFollowedMovesUp(t *testing.T) {
	h := newHarness(t)
	h.bars(4)
	// The note that was in bar 3, by identity rather than by counting:
	// what matters is that this one is now in bar 2.
	was := h.elements(0, 2)[0]

	h.post("/api/removemeasure?measure=1")

	now := h.elements(0, 1)
	if len(now) == 0 || now[0] != was {
		t.Errorf("bar 2 holds %v, want what bar 3 held", now)
	}
}

// A piece ends on its final barline wherever the end turns out to be.
// Deleting the last bar used to leave the music ending on a plain line,
// which is the first thing anybody would have to put back by hand.
func TestTheFinalBarlineStaysAtTheEnd(t *testing.T) {
	h := newHarness(t)
	h.bars(4)
	p := h.app.score.Parts[0]
	last := len(p.Measures) - 1
	p.Measures[last].RightBarline = model.BarFinal

	h.post("/api/removemeasure?measure=" + itoa(last))

	end := p.Measures[len(p.Measures)-1]
	if end.RightBarline != model.BarFinal {
		t.Errorf("the piece now ends on %v, want the final barline",
			end.RightBarline)
	}
}

// Somewhere to write is the one thing the editor cannot do without: with
// no bars there is nothing on the page to click and no way back.
func TestTheScoreIsNeverLeftWithNoBars(t *testing.T) {
	h := newHarness(t)
	h.bars(3)
	h.note(0, 0, 6, "quarter")

	h.post("/api/removemeasure?measure=0&count=99")

	if got := h.app.score.MeasureCount(); got != 1 {
		t.Fatalf("the score has %d bars, want the one that must be left", got)
	}
	// And the one left is empty, since everything was asked to go.
	for _, el := range h.elements(0, 0) {
		if _, ok := el.(*model.Rest); !ok {
			t.Errorf("the bar left behind still holds %T", el)
		}
	}
}

// A system break on a bar that has gone, left where it was, is a page turn
// in the middle of whatever bar took its place.
func TestABreakOnARemovedBarDoesNotStayBehind(t *testing.T) {
	h := newHarness(t)
	h.bars(8)
	h.post("/api/systems?breaks=3,5")

	h.post("/api/removemeasure?measure=3&count=2")

	bars := h.app.score.MeasureCount()
	for _, sh := range h.app.score.Sheets {
		for _, b := range sh.Breaks {
			if b >= bars {
				t.Errorf("a system break is on bar %d of %d", b, bars)
			}
		}
	}
}

func TestRemovingABarThatIsNotThereSaysSo(t *testing.T) {
	h := newHarness(t)
	h.bars(2)
	if why := h.postFails("/api/removemeasure?measure=99"); why == "" {
		t.Error("it was refused without saying why")
	}
}
