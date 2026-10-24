/*
 * duration_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

import (
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A dot is part of how long a note is, not a mark on it.
//
// A dotted half is three quarters of a whole note, and nothing in the model
// needs to know it was written with a dot: the engraver works the other way
// round, finding the notehead and the dots that spell a duration. So the
// only thing that has to be right here is the arithmetic.

func TestADotLengthensTheNote(t *testing.T) {
	for _, c := range []struct {
		dur, dots string
		want      model.Duration
	}{
		{"whole", "0", model.Duration{Num: 1, Den: 1}},
		{"whole", "", model.Duration{Num: 1, Den: 1}},
		{"half", "1", model.Duration{Num: 3, Den: 4}},
		{"quarter", "1", model.Duration{Num: 3, Den: 8}},
		{"eighth", "1", model.Duration{Num: 3, Den: 16}},
		// Two dots add half again and then a quarter: seven sixteenths of a
		// whole note, not fifteen thirty-seconds.
		{"quarter", "2", model.Duration{Num: 7, Den: 16}},
		// And nobody writes a third.
		{"quarter", "3", model.Duration{Num: 7, Den: 16}},
	} {
		got := dotted(durationNames[c.dur], c.dots)
		if got.Cmp(c.want) != 0 {
			t.Errorf("a %s with %q dots is %s, want %s",
				c.dur, c.dots, got, c.want)
		}
	}
}

// And the note that is written is the one that was asked for, through the
// handler a keystroke actually reaches.
func TestADottedNoteIsWrittenAsOne(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=half&dots=1&kind=note")

	els := h.elements(0, 0)
	if len(els) != 1 {
		t.Fatalf("%d elements were written", len(els))
	}
	want := model.Duration{Num: 3, Den: 4}
	if got := els[0].Duration(); got.Cmp(want) != 0 {
		t.Errorf("the note is %s long, want %s — a dotted half", got, want)
	}
}

// Every value the number row offers writes the note it names.
func TestTheNumberRowsValuesAllWrite(t *testing.T) {
	for _, c := range []struct {
		key       string
		dur, dots string
		want      model.Duration
	}{
		{"1", "whole", "0", model.Duration{Num: 1, Den: 1}},
		{"2", "half", "0", model.Duration{Num: 1, Den: 2}},
		{"3", "half", "1", model.Duration{Num: 3, Den: 4}},
		{"4", "quarter", "0", model.Duration{Num: 1, Den: 4}},
		{"5", "quarter", "1", model.Duration{Num: 3, Den: 8}},
		{"8", "eighth", "0", model.Duration{Num: 1, Den: 8}},
		{"9", "sixteenth", "0", model.Duration{Num: 1, Den: 16}},
		{"0", "32nd", "0", model.Duration{Num: 1, Den: 32}},
	} {
		h := newHarness(t)
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=6&dur=%s&dots=%s&kind=note",
			c.dur, c.dots))
		els := h.elements(0, 0)
		if len(els) != 1 {
			t.Errorf("key %s: %d elements", c.key, len(els))
			continue
		}
		if got := els[0].Duration(); got.Cmp(c.want) != 0 {
			t.Errorf("key %s writes %s, want %s", c.key, got, c.want)
		}
	}
}

// The dot aimed at a note already written lengthens that note by half, and
// pressing it again takes it back: one button, pressed twice, leaves the
// note as it was.
func TestTheDotLengthensTheNoteItIsAimedAt(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "half")

	h.post("/api/dot?part=0&measure=0&at=1")
	if got, want := h.elements(0, 0)[0].Duration(),
		(model.Duration{Num: 3, Den: 4}); got.Cmp(want) != 0 {
		t.Fatalf("the half came out %s, want %s", got, want)
	}

	h.post("/api/dot?part=0&measure=0&at=1")
	if got, want := h.elements(0, 0)[0].Duration(),
		(model.Duration{Num: 1, Den: 2}); got.Cmp(want) != 0 {
		t.Errorf("dotting it twice left it %s, want the half it was", got)
	}
}

// A length no notehead spells cannot be dotted, and says so rather than
// writing something that cannot be drawn.
func TestALengthWithNoNoteheadIsNotDotted(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3") // thirds of a quarter

	if said := h.postFails("/api/dot?part=0&measure=0&at=1"); said == "" {
		t.Error("it was refused without saying why")
	}
}
