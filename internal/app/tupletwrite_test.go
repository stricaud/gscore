/*
 * tupletwrite_test.go
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

// Writing over a member of a tuplet keeps it a member.
//
// A triplet asked for on a quarter is three eighths: the note and two rests
// to be written over. The quarter-and-eighth triplet of every swing bass
// line is made by writing a quarter over the first of them — and that put
// a plain quarter where the member was, figure gone, with both rests still
// after it. The bar came out an eighth over, and the rests could not be
// taken away either, since a group losing a member stops being a group.

// tripletOfEighths is a bar of 4/4 whose last beat is a triplet of eighths
// with nothing written in it yet.
func tripletOfEighths(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}
	h.post("/api/tuplet?part=0&measure=0&at=4&n=3")
	h.post("/api/insert?part=0&measure=0&at=3&step=4&dur=eighth&kind=rest&replace=1")
	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/4", "note6 1/4", "note6 1/4",
		"rest 1/12 3:2 start", "rest 1/12 3:2", "rest 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the bar holds %v, want %v", got, want)
	}
	return h
}

func TestAQuarterWrittenOverATripletsFirstEighthSwallowsTheSecond(t *testing.T) {
	h := tripletOfEighths(t)
	d := h.post("/api/insert?part=0&measure=0&at=3&step=0&dur=quarter&kind=note&replace=1")
	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/4", "note6 1/4", "note6 1/4",
		// A quarter in a triplet lasts a sixth, and it is written over the
		// rest after it as well as the one it replaced.
		"note2 1/6 3:2 start", "rest 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
	if want := (model.Duration{Num: 1, Den: 1}); total(h, 0, 0).Reduced().Cmp(want) != 0 {
		t.Errorf("the bar lasts %s, want %s", total(h, 0, 0).Reduced(), want)
	}
	if len(h.elements(0, 1)) != 0 {
		t.Errorf("something spilled into the second bar: %v", spell(t, h, 0, 1))
	}
	// The cursor is left on the rest still to be written.
	if d["at"] != float64(4) {
		t.Errorf("the cursor was left at %v, want 4", d["at"])
	}

	// And the eighth that finishes the group goes over that rest.
	h.post("/api/insert?part=0&measure=0&at=4&step=1&dur=eighth&kind=note&replace=1")
	got = spell(t, h, 0, 0)
	want = []string{
		"note6 1/4", "note6 1/4", "note6 1/4",
		"note2 1/6 3:2 start", "note3 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
}

// Written over the last member, a longer note has no rest to swallow and
// the group is then longer than its figure says — the writer's business,
// the same as an overfull bar is. It is still a member.
func TestALongerNoteOverTheLastMemberStillBelongsToTheGroup(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3")
	h.post("/api/insert?part=0&measure=0&at=2&step=0&dur=quarter&kind=note&replace=1")
	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/12 3:2 start", "rest 1/12 3:2", "note2 1/6 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
}

// A shorter note than the member leaves what is over as a rest inside the
// group, so the group still adds up and the rest is there to write over.
func TestAShorterNoteOverAMemberLeavesARestInTheGroup(t *testing.T) {
	h := tripletOfEighths(t)
	h.post("/api/insert?part=0&measure=0&at=3&step=0&dur=quarter&kind=note&replace=1")
	// Second thoughts: an eighth, not a quarter.
	h.post("/api/insert?part=0&measure=0&at=3&step=1&dur=eighth&kind=note&replace=1")
	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/4", "note6 1/4", "note6 1/4",
		"note3 1/12 3:2 start", "rest 1/12 3:2", "rest 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
	if want := (model.Duration{Num: 1, Den: 1}); total(h, 0, 0).Reduced().Cmp(want) != 0 {
		t.Errorf("the bar lasts %s, want %s", total(h, 0, 0).Reduced(), want)
	}
}
