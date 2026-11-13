/*
 * keychange_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * A key change that changed nothing says so.
 *
 * Asking a bar for the key it is already in is how a change at that bar is
 * taken away — and where there was no change there, it does nothing at all.
 * Both came back as "ok", so the editor reported "E major from measure 9,
 * on every staff" over a page that had not altered, which is
 * indistinguishable from the change being lost on its way to the paper.
 * Reported exactly that way: "I change and I don't see it."
 */

func scoreOfBars(t *testing.T, n int) *harness {
	t.Helper()
	h := newHarness(t)
	for m := 0; m < n; m++ {
		if m > 0 {
			h.post("/api/addmeasure")
		}
		for i := 0; i < 4; i++ {
			h.note(0, m, 2+i, "quarter")
		}
	}
	return h
}

func TestAKeyChangeIsWrittenAndDrawn(t *testing.T) {
	h := scoreOfBars(t, 12)
	h.post("/api/setkey?part=0&fifths=3&at=8")
	if h.app.score.Parts[0].Measures[8].Key == nil {
		t.Fatal("no key change was written at that bar")
	}
	// And it reaches the page, which is the only thing anybody can see.
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	sharps := 0
	for _, g := range lay.Glyphs {
		if g.Name == "accidentalSharp" {
			sharps++
		}
	}
	if sharps < 3 {
		t.Errorf("%d sharps drawn in the whole score, want at least the 3 of the change", sharps)
	}
}

func TestAskingForTheKeyAlreadyInForceSaysSo(t *testing.T) {
	h := scoreOfBars(t, 12)
	msg := h.fails("/api/setkey?part=0&fifths=0&at=8")
	if !strings.Contains(msg, "already") {
		t.Errorf("refused with %q", msg)
	}
	// Measure numbers, not indexes: the message is read by somebody
	// looking at a bar number printed over the bar.
	if !strings.Contains(msg, "measure 9") {
		t.Errorf("the message names the wrong bar: %q", msg)
	}
}

func TestSettingABarBackTakesTheChangeAway(t *testing.T) {
	// The other half of the same gesture, which must go on working: it is
	// how a change is removed.
	h := scoreOfBars(t, 12)
	h.post("/api/setkey?part=0&fifths=3&at=8")
	out := h.post("/api/setkey?part=0&fifths=0&at=8")
	if h.app.score.Parts[0].Measures[8].Key != nil {
		t.Error("the change is still there")
	}
	if n, _ := out["removed"].(float64); n < 1 {
		t.Errorf("the answer says %v were removed", out["removed"])
	}
	if n, _ := out["written"].(float64); n != 0 {
		t.Errorf("the answer says %v were written", out["written"])
	}
}

func TestAKeyCanBeSetFromAStaffThatMirrorsAnother(t *testing.T) {
	// The lower half of a notation-and-tablature pair holds no measures of
	// its own — it shows the staff above it — and it is the half somebody
	// reading the tab is looking at. Asked there, the key used to be
	// refused with "there is no such measure" about a bar plainly on the
	// page.
	h := newHarness(t)
	h.app.score.Parts[0].Tab, _ = model.TuningNamed("guitar")
	for m := 0; m < 10; m++ {
		if m > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, m, 2, "quarter")
	}
	h.post("/api/pairnotation")
	if len(h.app.score.Parts) != 2 || !h.app.score.Parts[1].Mirrors() {
		t.Fatal("the pair was not made")
	}

	h.post("/api/setkey?part=1&fifths=-2&at=8")
	if got := h.app.score.Parts[0].Measures[8].Key; got == nil || got.Fifths != -2 {
		t.Errorf("the staff that holds the music carries %v", got)
	}
	// And nothing was invented on the mirror, which holds no bars at all.
	if n := len(h.app.score.Parts[1].Measures); n != 0 {
		t.Errorf("the mirror grew %d bars of its own", n)
	}
}

func TestAKeyIsSetFromAStaffShorterThanTheRest(t *testing.T) {
	// Staves of different lengths are ordinary: a part that stops early, a
	// staff added later, a voice that rests to the end and was never given
	// the bars. The bar has to exist somewhere, not on the one staff that
	// happened to be clicked — asked that way, a key change at a bar
	// plainly on the page was refused with "there is no such measure",
	// which is true of that staff and false of the music.
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&written=pitches")
	for m := 0; m < 10; m++ {
		if m > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, m, 2, "quarter")
	}
	h.app.score.Parts[1].Measures = h.app.score.Parts[1].Measures[:5]

	h.post("/api/setkey?part=1&fifths=-2&at=8")
	if got := h.app.score.Parts[0].Measures[8].Key; got == nil || got.Fifths != -2 {
		t.Errorf("the staff that has that bar carries %v", got)
	}
	// And the short staff is left as it is rather than grown bars it
	// never had.
	if n := len(h.app.score.Parts[1].Measures); n != 5 {
		t.Errorf("the short staff now has %d bars", n)
	}
}

func TestABarPastEveryStaffSaysHowLongTheMusicIs(t *testing.T) {
	h := scoreOfBars(t, 4)
	msg := h.fails("/api/setkey?part=0&fifths=2&at=9")
	if !strings.Contains(msg, "4 bars long") {
		t.Errorf("refused with %q", msg)
	}
}

func TestAKeyAtTheBlankBarBeginsThatBar(t *testing.T) {
	// The editor draws one blank bar past the last so there is somewhere
	// to click and begin the next. It is not a bar of the piece until
	// something is written in it — and a key signature is as good a reason
	// to begin a bar as a note. Refused instead, it said "there is no
	// measure 9: the music is 8 bars long" about the bar just clicked.
	h := scoreOfBars(t, 8)
	h.post("/api/setkey?part=0&fifths=-2&at=8")

	if n := len(h.app.score.Parts[0].Measures); n != 9 {
		t.Fatalf("the music is %d bars long, want the bar to have been begun", n)
	}
	if got := h.app.score.Parts[0].Measures[8].Key; got == nil || got.Fifths != -2 {
		t.Errorf("the new bar carries %v", got)
	}
}

func TestEveryStaffGainsTheBarTogether(t *testing.T) {
	// Or the barlines stop standing in a column, which is the reason note
	// entry has always made the bar on every staff at once.
	h := newHarness(t)
	h.post("/api/addstaff?instrument=flute&written=pitches")
	for m := 0; m < 8; m++ {
		if m > 0 {
			h.post("/api/addmeasure")
		}
		h.note(0, m, 2, "quarter")
	}
	h.post("/api/setkey?part=0&fifths=3&at=8")
	for i, p := range h.app.score.Parts {
		if len(p.Measures) != 9 {
			t.Errorf("staff %d is %d bars long", i, len(p.Measures))
		}
	}
}

func TestAKeyFarPastTheEndIsStillRefused(t *testing.T) {
	// One bar past the end is beginning the next bar; fifty is a mistake,
	// not an instruction to write forty-two empty ones.
	h := scoreOfBars(t, 8)
	msg := h.fails("/api/setkey?part=0&fifths=3&at=50")
	if !strings.Contains(msg, "8 bars long") {
		t.Errorf("refused with %q", msg)
	}
	if n := len(h.app.score.Parts[0].Measures); n != 8 {
		t.Errorf("the music grew to %d bars anyway", n)
	}
}

func TestAClefAtTheBlankBarBeginsThatBarToo(t *testing.T) {
	// The same rule, because it is the same gesture: the clef refused
	// with "no such measure" at exactly the bar the key did.
	h := scoreOfBars(t, 8)
	h.post("/api/staff?part=0&at=8&clef=bass")
	if n := len(h.app.score.Parts[0].Measures); n != 9 {
		t.Fatalf("the music is %d bars long", n)
	}
	if got := h.app.score.Parts[0].Measures[8].Clef; got == nil {
		t.Error("the new bar carries no clef change")
	}
}
