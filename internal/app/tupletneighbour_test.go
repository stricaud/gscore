/*
 * tupletneighbour_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two tuplets side by side are two groups.
 *
 * The group an element belongs to is found by walking the tuplet marks
 * either way from it. Walking back, the walk stopped when it had stepped
 * onto a note marked as a start — but it never asked whether the note it
 * began on was one. So a quadruplet freed by its own first note walked
 * straight back into the quadruplet before it, and took that one apart
 * too: nine notes freed where five were asked for, and the first group's
 * figure gone with its time.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// twoQuadruplets writes a bar of two quadruplets of 32nds back to back,
// each four in the time of three, and returns the voice they are in.
func twoQuadruplets(t *testing.T) (*harness, *model.Voice) {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	// Two dotted sixteenths, each turned into a quadruplet of 32nds.
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=sixteenth&dots=1")
	h.post("/api/insert?part=0&measure=0&at=-1&step=2&dur=sixteenth&dots=1")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=4")
	h.post("/api/tuplet?part=0&measure=0&at=5&n=4")
	v := h.app.score.Parts[0].Measures[0].Voices[0]
	if len(v.Elements) != 8 {
		t.Fatalf("the bar has %d elements, want two groups of four", len(v.Elements))
	}
	return h, v
}

// inTuplet counts the elements of a voice that carry a tuplet mark.
func inTuplet(v *model.Voice) int {
	n := 0
	for _, el := range v.Elements {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			n++
		}
	}
	return n
}

func TestFreeingATupletLeavesTheOneBeforeItAlone(t *testing.T) {
	h, v := twoQuadruplets(t)
	// Freed by its first note, which is the note the editor names when
	// the figure over the group is clicked.
	got := h.post("/api/tuplet?part=0&measure=0&at=5&free=1")
	if n := int(got["freed"].(float64)); n != 4 {
		t.Errorf("freed %d elements, want the 4 of the second group", n)
	}
	if n := inTuplet(v); n != 4 {
		t.Errorf("%d elements are still in a tuplet, want the 4 of the first group", n)
	}
}

func TestCollapsingATupletLeavesTheOneBeforeItAlone(t *testing.T) {
	h, v := twoQuadruplets(t)
	got := h.post("/api/tuplet?part=0&measure=0&at=5&n=0")
	if n := int(got["removed"].(float64)); n != 4 {
		t.Errorf("collapsed %d elements, want the 4 of the second group", n)
	}
	if len(v.Elements) != 5 || inTuplet(v) != 4 {
		t.Errorf("the bar has %d elements with %d in a tuplet, want 5 and 4: "+
			"the first group whole, the second one note",
			len(v.Elements), inTuplet(v))
	}
}
