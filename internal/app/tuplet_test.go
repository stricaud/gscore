/*
 * tuplet_test.go
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

// Making a tuplet is a change to the music, not a mark on it.
//
// The note at the cursor becomes n notes played in its time. Done the other
// way round — writing three notes and then calling them a triplet — the bar
// comes out a beat short and the person who asked is left wondering what
// they did wrong.

// barLength is what a bar adds up to, which a tuplet must not change.
func barLength(els []model.Element) model.Duration {
	out := model.Duration{Num: 0, Den: 1}
	for _, el := range els {
		out = out.Add(el.Duration())
	}
	return out.Reduced()
}

func TestATripletIsThreeNotesInTheTimeOfOne(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter") // one quarter note, on the middle line

	before := barLength(h.elements(0, 0))
	got := h.post("/api/tuplet?part=0&measure=0&at=1&n=3")
	if got["tuplet"] != 3.0 || got["normal"] != 2.0 {
		t.Errorf("a triplet came out as %v in the time of %v",
			got["tuplet"], got["normal"])
	}

	els := h.elements(0, 0)
	if len(els) != 3 {
		t.Fatalf("the bar holds %d elements, want 3", len(els))
	}
	// The note that was there keeps its pitch; the other two are rests to
	// be written over.
	if _, ok := els[0].(*model.Note); !ok {
		t.Errorf("the first member is %T, want the note that was there", els[0])
	}
	for i, el := range els[1:] {
		if _, ok := el.(*model.Rest); !ok {
			t.Errorf("member %d is %T, want a rest", i+2, el)
		}
	}
	// Each sounds a third of a quarter, and together they still fill the
	// time the one note did.
	if after := barLength(els); after.Cmp(before) != 0 {
		t.Errorf("the bar was %s and is now %s: a tuplet must not change "+
			"how long the music is", before, after)
	}
	// And they are marked as one group, so the figure and the bracket know
	// where it starts and stops.
	for i, el := range els {
		m := marksOfElement(el)
		if m == nil || m.Tuplet == nil {
			t.Fatalf("member %d carries no tuplet", i+1)
		}
		if m.Tuplet.Actual != 3 || m.Tuplet.Normal != 2 {
			t.Errorf("member %d is %d:%d", i+1, m.Tuplet.Actual, m.Tuplet.Normal)
		}
		if got, want := m.Tuplet.Start, i == 0; got != want {
			t.Errorf("member %d: start %v", i+1, got)
		}
		if got, want := m.Tuplet.Stop, i == len(els)-1; got != want {
			t.Errorf("member %d: stop %v", i+1, got)
		}
	}
}

// Every size, not only three. Asked for a group that does not divide the
// beat the way the textbook ratio wants, it comes down to one that does
// rather than refusing: somebody asking for four notes in this space means
// the four notes, and the figure printed over them says which ratio it is.
func TestEverySizeOfTupletCanBeMade(t *testing.T) {
	for n := 2; n <= 9; n++ {
		h := newHarness(t)
		h.note(0, 0, 6, "quarter")
		before := barLength(h.elements(0, 0))

		got := h.post(fmt.Sprintf("/api/tuplet?part=0&measure=0&at=1&n=%d", n))
		els := h.elements(0, 0)
		if len(els) != n {
			t.Errorf("a group of %d came out as %d notes", n, len(els))
			continue
		}
		normal := int(got["normal"].(float64))
		if normal >= n || normal < 1 {
			t.Errorf("%d in the time of %d is not a tuplet", n, normal)
		}
		// Whatever the ratio, the written value has to be a real notehead.
		w := els[0].Duration().Scale(n, normal)
		if _, _, ok := w.Written(); !ok {
			t.Errorf("a group of %d writes its notes as %s, which is not a "+
				"notehead", n, w)
		}
		if after := barLength(els); after.Cmp(before) != 0 {
			t.Errorf("a group of %d changed the bar from %s to %s", n, before, after)
		}
	}
}

// And back again: the group becomes the one note it was made from, of the
// time the group filled.
func TestATupletCanBeTakenBack(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	before := barLength(h.elements(0, 0))

	h.post("/api/tuplet?part=0&measure=0&at=1&n=5")
	if n := len(h.elements(0, 0)); n != 5 {
		t.Fatalf("the quintuplet is %d notes", n)
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=0")

	els := h.elements(0, 0)
	if len(els) != 1 {
		t.Fatalf("taking the tuplet back left %d elements, want 1", len(els))
	}
	if m := marksOfElement(els[0]); m != nil && m.Tuplet != nil {
		t.Error("the note is still marked as part of a tuplet")
	}
	if after := barLength(els); after.Cmp(before) != 0 {
		t.Errorf("the bar was %s and came back as %s", before, after)
	}
}

// A note already in a group is not made into a group of groups.
func TestATupletIsNotMadeInsideATuplet(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3")
	if said := h.postFails("/api/tuplet?part=0&measure=0&at=1&n=3"); said == "" {
		t.Error("it was refused without saying why")
	}
}
