/*
 * chordsplit_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A second chord over a rest that already carries one splits the rest.
//
// A bar of rest in a chart has two chords over it as often as one, and
// with the whole bar one rest there was one place to hang a symbol: the
// second chord replaced the first. Reported as "it replaces it instead of
// adding it".
func TestASecondChordOverARestSplitsTheRest(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&dur=whole&kind=rest")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=D7")
	d := h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=C%237")
	if d["split"] != true {
		t.Errorf("the answer did not say the rest was split: %v", d)
	}

	els := h.elements(0, 0)
	if len(els) != 2 {
		t.Fatalf("the bar holds %d elements, want two half rests", len(els))
	}
	for i, el := range els {
		r, ok := el.(*model.Rest)
		if !ok {
			t.Fatalf("element %d is a %T, want a rest", i, el)
		}
		if want := (model.Duration{Num: 1, Den: 2}); r.Dur.Cmp(want) != 0 {
			t.Errorf("rest %d lasts %s, want %s", i, r.Dur, want)
		}
	}
	first, second := marksOfElement(els[0]).Chord, marksOfElement(els[1]).Chord
	if first == nil || first.String() != "D7" {
		t.Errorf("the first half carries %v, want D7", first)
	}
	if second == nil || second.String() != "C#7" {
		t.Errorf("the second half carries %v, want C#7", second)
	}

	// Both are on the page, apart.
	boxes := h.chordBoxes()
	if len(boxes) != 2 {
		t.Fatalf("%d boxes drawn, want one for each chord", len(boxes))
	}
	if boxes[0]["name"] != "D7" || boxes[1]["name"] != "C#7" {
		t.Errorf("the boxes are %q and %q", boxes[0]["name"], boxes[1]["name"])
	}
}

// The same chord written again over the rest is a change to that chord,
// not a second one, and a note is never split.
func TestTheSameChordAgainIsNotASecondChord(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&dur=whole&kind=rest")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=D7")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=D7&frame=xx0212")
	if n := len(h.elements(0, 0)); n != 1 {
		t.Errorf("the bar holds %d elements, want the one rest", n)
	}
	if c := chordOn(h, 0, 0); c == nil || c.Frame == nil {
		t.Error("the fingering was not written onto the chord")
	}

	h.note(0, 1, 4, "whole")
	h.post("/api/chordsymbol?part=0&measure=1&at=1&chord=D7")
	h.post("/api/chordsymbol?part=0&measure=1&at=1&chord=G7")
	if n := len(h.elements(0, 1)); n != 1 {
		t.Errorf("the bar holds %d elements, want the one note", n)
	}
	if c := chordOn(h, 0, 1); c == nil || c.String() != "G7" {
		t.Errorf("the note carries %v, want the G7 written last", c)
	}
}

// A rest that will not halve into a writable length is written over
// rather than split: there is no half of five quarters to draw.
func TestARestThatDoesNotHalveIsWrittenOver(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&time=5/4")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=whole&kind=rest")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=D7")
	d := h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=G7")
	if d["split"] == true {
		t.Error("a bar of 5/4 was split")
	}
	if n := len(h.elements(0, 0)); n != 1 {
		t.Errorf("the bar holds %d elements, want the one rest", n)
	}
}

// The accidental of a root is typed as on a typewriter: # for a sharp, b
// for a flat, and = for a natural, which has no key of its own.
func TestANaturalIsTypedAsAnEqualsSign(t *testing.T) {
	for _, c := range []struct{ in, root, quality string }{
		{"C#7", "C#", "7"},
		{"Bb7", "Bb", "7"},
		{"F=7", "F=", "7"},
		{"F♮maj7", "F♮", "maj7"},
	} {
		got := parseChordSymbol(c.in)
		if got.Root != c.root || got.Quality != c.quality {
			t.Errorf("%q read as root %q quality %q, want %q and %q",
				c.in, got.Root, got.Quality, c.root, c.quality)
		}
	}
}
