/*
 * tupletdot_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Dots and tuplets, which are two ways of saying how long a note is and
 * had never been told about each other.
 *
 * A note inside a tuplet is written as one thing and lasts another: a
 * thirty-second in twelve-in-the-time-of-eight is stored as a forty-eighth,
 * because that is what it plays for. The dot belongs to the written note —
 * it is drawn beside that notehead and read as half of it again — so the
 * group's figure has to be divided out before the dot is added and put back
 * afterwards.
 *
 * And a tuplet that loses a note is not that tuplet any more. The notes
 * left behind kept the mark and the scaled length, which left a bar that
 * could not be dotted, could not be grouped, and had no figure drawn over
 * it to explain why. Both were reported on the same bar of the Altes.
 */

package app

import (
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// written is what a reader sees: the stored length with the group's figure
// divided back out.
func writtenOf(el model.Element) model.Duration {
	if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
		if actual, normal, ok := mk.Tuplet.Ratio(); ok {
			return el.Duration().Scale(actual, normal).Reduced()
		}
	}
	return el.Duration().Reduced()
}

// triplet writes three eighths and reads them as a triplet.
func triplet(t *testing.T, h *harness) {
	t.Helper()
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")
}

func TestANoteInsideATupletCanBeDotted(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)

	got := h.post("/api/dot?part=0&measure=0&at=1")
	if dots, _ := got["dots"].(float64); dots != 1 {
		t.Fatalf("the answer says %v dots", got["dots"])
	}

	el := h.elements(0, 0)[0]
	// What a reader sees: a dotted eighth, inside a triplet, as written.
	value, dots, ok := writtenOf(el).Written()
	if !ok || value != model.Eighth || dots != 1 {
		t.Errorf("it is written as %v (%v, %d dots, spelled=%v)",
			writtenOf(el), value, dots, ok)
	}
	// And what it plays for: a dotted eighth three in the time of two.
	if want := (model.Duration{Num: 1, Den: 8}); el.Duration().Reduced().Cmp(want) != 0 {
		t.Errorf("it lasts %s, want %s", el.Duration(), want)
	}
	// It is still in the group, which the dot was never about.
	if mk := marksOfElement(el); mk == nil || mk.Tuplet == nil {
		t.Error("dotting it took it out of the triplet")
	}
}

// The second dot is where reading the stored length instead of the written
// one goes wrong without saying so. A dotted eighth in a triplet is stored
// as an eighth — exactly what a plain eighth is stored as — so the old
// arithmetic would have read it as an undotted eighth and made it a dotted
// one all over again, silently.
func TestTheDotsGoRoundOnTheNoteAsItIsWritten(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)

	want := []struct {
		dots int
		dur  model.Duration // as written
	}{
		{1, model.Duration{Num: 3, Den: 16}},
		{2, model.Duration{Num: 7, Den: 32}},
		{3, model.Duration{Num: 15, Den: 64}},
		{0, model.Duration{Num: 1, Den: 8}},
	}
	for i, step := range want {
		got := h.post("/api/dot?part=0&measure=0&at=1")
		if dots, _ := got["dots"].(float64); int(dots) != step.dots {
			t.Fatalf("press %d: the answer says %v dots, want %d",
				i+1, got["dots"], step.dots)
		}
		el := h.elements(0, 0)[0]
		if got := writtenOf(el); got.Cmp(step.dur) != 0 {
			t.Errorf("press %d: it is written as %s, want %s", i+1, got, step.dur)
		}
	}
}

// Three in the time of two with one of them gone is two notes with a 3
// over them, and no arithmetic that holds.
func TestATupletThatLosesANoteStopsBeingATuplet(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)
	h.post("/api/delete?part=0&measure=0&at=1")

	for i, el := range h.elements(0, 0) {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			t.Errorf("element %d still says it is in a tuplet", i)
		}
	}
}

// Nothing moves on the page. A member drawn as an eighth goes on being
// drawn as an eighth; only the arithmetic changes, from false to true.
func TestTheNotesLeftAreTheNotesThatWereDrawn(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)
	before := []model.Duration{}
	for _, el := range h.elements(0, 0)[:3] {
		before = append(before, writtenOf(el))
	}

	h.post("/api/delete?part=0&measure=0&at=1")

	left := h.elements(0, 0)
	for i, want := range []model.Duration{before[0], before[2]} {
		if got := writtenOf(left[i]); got.Cmp(want) != 0 {
			t.Errorf("note %d is written as %s, want %s", i, got, want)
		}
	}
}

// What the person reported, in the two ways they met it.
func TestWhatIsLeftOfABrokenTupletCanBeGroupedAgain(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)
	h.post("/api/delete?part=0&measure=0&at=1")

	// h.post fails the test if the call is refused, which is the whole
	// point: this is the call that answered "part of that selection is
	// already in a tuplet" about notes with no tuplet anywhere near them.
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1")

	for i, el := range h.elements(0, 0) {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			if actual := mk.Tuplet.Actual; actual != 3 {
				t.Errorf("element %d is in a group of %d", i, actual)
			}
		}
	}
}

func TestWhatIsLeftOfABrokenTupletCanBeDotted(t *testing.T) {
	h := newHarness(t)
	triplet(t, h)
	h.post("/api/delete?part=0&measure=0&at=1")

	got := h.post("/api/dot?part=0&measure=0&at=1")
	if dots, _ := got["dots"].(float64); dots != 1 {
		t.Fatalf("the answer says %v dots", got["dots"])
	}
	if want := (model.Duration{Num: 3, Den: 16}); writtenOf(h.elements(0, 0)[0]).Cmp(want) != 0 {
		t.Errorf("it is written as %s, want %s", writtenOf(h.elements(0, 0)[0]), want)
	}
}

// Deleting several at once goes through its own handler, and from the back
// of the bar forwards — so freeing the group must not move anything.
func TestDeletingSeveralMembersAtOnceLeavesNoTuplet(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 12; i++ {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=32nd&kind=note", i%7))
	}
	sel := ""
	for i := 0; i < 12; i++ {
		if i > 0 {
			sel += ","
		}
		sel += fmt.Sprintf("0:0:%d", i)
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=12&sel=" + sel)

	h.post("/api/deleteselection?sel=0:0:4,0:0:5,0:0:6")

	for i, el := range h.elements(0, 0) {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			t.Errorf("element %d still says it is in a tuplet", i)
		}
		if _, _, ok := el.Duration().Written(); !ok {
			t.Errorf("element %d is %s long, which no notehead spells",
				i, el.Duration())
		}
	}
}
