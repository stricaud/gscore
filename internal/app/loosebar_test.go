/*
 * loosebar_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Bars that do not add up, on purpose.
 *
 * Keeping bars to the meter is what makes typing music in feel like typing:
 * what will not fit moves along, and the arithmetic looks after itself. It
 * is also wrong often enough to be a switch. A figure of twelve notes in
 * the time of eight is written in one bar in every edition there is, and so
 * is plenty of old music barred by phrase rather than by beat.
 *
 * Turning the switch off stopped the carrying over afterwards but not the
 * writing point stepping over the barline in the first place, so a run
 * longer than its bar still broke in two at the line — which is the one
 * thing somebody turning it off is trying to stop.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// looseBarlines turns the arithmetic off for one test, in memory: writing
// the setting out would go to the person's own configuration file.
func looseBarlines(t *testing.T) {
	t.Helper()
	stored.Lock()
	was, wasRead := stored.settings.LooseBarlines, stored.read
	stored.settings.LooseBarlines, stored.read = true, true
	stored.Unlock()
	t.Cleanup(func() {
		stored.Lock()
		stored.settings.LooseBarlines, stored.read = was, wasRead
		stored.Unlock()
	})
}

// looseVoice is the nth bar of the first staff, or an empty one where
// the music does not reach that far.
func looseVoice(h *harness, n int) *model.Voice {
	p := h.app.score.Parts[0]
	if n >= len(p.Measures) {
		return &model.Voice{}
	}
	return p.Measures[n].Voices[0]
}

// Forty-eight thirty-seconds is a bar and a half of four four. Written
// with the barlines kept, it fills a bar and goes on into the next;
// written with them loose, it is one bar of music however long that bar
// comes out.
func TestARunLongerThanTheBarStaysInItWhenBarlinesAreLoose(t *testing.T) {
	looseBarlines(t)
	h := newHarness(t)

	for i := 0; i < 48; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=32nd")
	}

	if got := len(looseVoice(h, 0).Elements); got != 48 {
		t.Errorf("the first bar holds %d of the 48 notes", got)
	}
	if got := len(looseVoice(h, 1).Elements); got != 0 {
		t.Errorf("%d notes were carried over a barline that was to be left alone", got)
	}
}

// And the other half of the switch, which is the half that is on by
// default: the arithmetic is kept and the overflow moves along.
func TestTheSameRunBreaksAtTheBarlineWhenTheyAreKept(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < 48; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=32nd")
	}

	first := looseVoice(h, 0).Duration()
	if bar := h.app.score.Parts[0].Time.BarDuration(); first.Cmp(bar) > 0 {
		t.Errorf("the first bar is %v long, past the %v the meter allows", first, bar)
	}
	if got := len(looseVoice(h, 1).Elements); got == 0 {
		t.Error("nothing went into the second bar")
	}
}

// The answer has to say where the note went, since the cursor follows it:
// a note that stayed in the bar being typed and was reported in the next
// one would leave the caret a bar ahead of the music.
func TestTheAnswerSaysTheNoteStayedInTheBar(t *testing.T) {
	looseBarlines(t)
	h := newHarness(t)

	var last map[string]any
	for i := 0; i < 48; i++ {
		last = h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=32nd")
	}
	if got, _ := last["measure"].(float64); got != 0 {
		t.Errorf("the last note says it went into bar %v", last["measure"])
	}
	if got, _ := last["at"].(float64); int(got) != 48 {
		t.Errorf("the caret is at %v, want 48", last["at"])
	}
}

// A tab staff is typed by fret rather than by pitch and goes through its
// own handler, which had the same rule written out a second time.
func TestAFretStaysInTheBarTooWhenBarlinesAreLoose(t *testing.T) {
	looseBarlines(t)
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=tab")

	tab := -1
	for i, p := range h.app.score.Parts {
		if p.Tab != nil {
			tab = i
		}
	}
	if tab < 0 {
		t.Fatal("no tablature staff to type into")
	}
	for i := 0; i < 48; i++ {
		h.post("/api/fret?part=" + itoa(tab) + "&measure=0&at=-1&string=1&fret=3&dur=32nd")
	}
	p := h.app.score.Parts[tab]
	if got := len(p.Measures[0].Voices[0].Elements); got != 48 {
		t.Errorf("the first bar holds %d of the 48 frets", got)
	}
}
