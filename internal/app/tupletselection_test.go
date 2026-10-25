/*
 * tupletselection_test.go
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

// A passage written out plain and then read as a tuplet.
//
// The single-note form of this call makes a group by throwing the note away
// and putting n rests where it was, which is right when the group is what you
// are about to write. It is the wrong answer once the notes are written: the
// way to turn three eighths into a triplet was to delete them and type them
// again. So a selection keeps its notes and only changes their arithmetic.

// spell says what is actually in a bar, which is the only thing these tests
// are about.
func spell(t *testing.T, h *harness, part, measure int) []string {
	t.Helper()
	var out []string
	for _, el := range h.elements(part, measure) {
		kind := "rest"
		if n, ok := el.(*model.Note); ok {
			kind = fmt.Sprintf("note%d", n.Pitch.Step)
		}
		tup := ""
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			tup = fmt.Sprintf(" %d:%d", mk.Tuplet.Actual, mk.Tuplet.Normal)
			if mk.Tuplet.Start {
				tup += " start"
			}
			if mk.Tuplet.Stop {
				tup += " stop"
			}
		}
		out = append(out, kind+" "+el.Duration().String()+tup)
	}
	return out
}

func total(h *harness, part, measure int) model.Duration {
	sum := model.Duration{Num: 0, Den: 1}
	for _, el := range h.elements(part, measure) {
		sum = sum.Add(el.Duration())
	}
	return sum
}

func TestThreeEighthsBecomeATripletAndKeepTheirPitches(t *testing.T) {
	h := newHarness(t)
	// Staff steps. On a treble staff 4 is B, 5 is C and 6 is D, which is
	// what the model stores and so what these tests read back.
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")

	got := spell(t, h, 0, 0)
	want := []string{
		// Each one still a twelfth of a whole note, which is an eighth
		// written and played three in the time of two.
		"note6 1/12 3:2 start",
		"note0 1/12 3:2",
		"note1 1/12 3:2 stop",
		// Three eighths were three eighths; the triplet is two, so an
		// eighth comes back as a rest.
		"rest 1/8",
	}
	if len(got) != len(want) {
		t.Fatalf("the bar holds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("element %d is %q, want %q", i, got[i], want[i])
		}
	}
	// And the bar still adds up to what it did: three eighths.
	if want := (model.Duration{Num: 3, Den: 8}); total(h, 0, 0).Cmp(want) != 0 {
		t.Errorf("the bar now lasts %s, want %s", total(h, 0, 0), want)
	}
}

// Fewer notes than the group needs: the rest of the group comes in as rests
// to be written over, and nothing is given back because the group is exactly
// as long as the notes were.
func TestTwoEighthsBecomeATripletWithOneLeftToWrite(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	d := h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1")
	if got, want := d["members"], 3.0; got != want {
		t.Errorf("the group has %v members, want %v", got, want)
	}

	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/12 3:2 start",
		"note0 1/12 3:2",
		"rest 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
	// Three triplet eighths are a quarter, which is what two eighths were.
	if want := (model.Duration{Num: 1, Den: 4}); total(h, 0, 0).Cmp(want) != 0 {
		t.Errorf("the bar now lasts %s, want %s", total(h, 0, 0), want)
	}
}

// The notes keep whatever else is on them, because the group is a way of
// reading them and not a rewrite.
func TestTheNotesInAGroupKeepTheirMarks(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/accidental?part=0&measure=0&at=2&accidental=sharp&sel=0:0:0")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")

	els := h.elements(0, 0)
	n, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the first element is %T, want a note", els[0])
	}
	if n.Accidental != model.Sharp {
		t.Errorf("the sharp is gone: the note reads %v", n.Accidental)
	}
}

// Notes with a gap between them are not a group: grouping them would mean
// moving the music about rather than reading it differently.
func TestNotesWithSomethingBetweenThemAreNotAGroup(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	if msg := h.postFails("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:2"); msg == "" {
		t.Error("two notes with one between them were accepted as a group")
	}
}

// More notes than the group holds has no sensible answer, so it says so
// rather than quietly dropping the ones that did not fit.
func TestMoreNotesThanTheGroupHoldsIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6, 7} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=sixteenth&kind=note", step))
	}
	msg := h.postFails("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2,0:0:3")
	if msg == "" {
		t.Error("four notes were accepted as a triplet")
	}
}

// One note picked up still means the old thing: make a group out of this
// note's time. The two forms share a call and must not have traded places.
func TestOneNotePickedUpStillExpandsIntoTheGroup(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0")

	got := spell(t, h, 0, 0)
	want := []string{
		"note6 1/12 3:2 start",
		"rest 1/12 3:2",
		"rest 1/12 3:2 stop",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v, want %v", got, want)
	}
}

// A larger group over a longer run, since 13 in the time of 8 is the case
// that started this.
func TestAThirteenTupletKeepsAllThirteenNotes(t *testing.T) {
	h := newHarness(t)
	sel := ""
	for i := 0; i < 13; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=32nd&kind=note")
		if i > 0 {
			sel += ","
		}
		sel += fmt.Sprintf("0:0:%d", i)
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=13&sel=" + sel)

	els := h.elements(0, 0)
	notes := 0
	for _, el := range els {
		if _, ok := el.(*model.Note); ok {
			notes++
		}
	}
	if notes != 13 {
		t.Errorf("%d of the 13 notes survived", notes)
	}
	for i := 0; i < 13; i++ {
		mk := marksOfElement(els[i])
		if mk == nil || mk.Tuplet == nil {
			t.Fatalf("note %d is not in the group", i)
		}
		if mk.Tuplet.Actual != 13 {
			t.Errorf("note %d reads %d in the time of %d",
				i, mk.Tuplet.Actual, mk.Tuplet.Normal)
		}
	}
}

// Whatever a group gives back has to be writable as rests, or the bar would
// hold a duration no notehead spells.
func TestWhatAGroupGivesBackIsWritable(t *testing.T) {
	for _, c := range []struct {
		d    model.Duration
		want int
	}{
		{model.Duration{Num: 1, Den: 8}, 1},
		{model.Duration{Num: 3, Den: 8}, 1}, // a dotted quarter
		{model.Duration{Num: 5, Den: 8}, 2}, // a half and an eighth
		{model.Duration{Num: 0, Den: 1}, 0},
	} {
		got, left := restsFor(c.d)
		if !left.IsZero() {
			t.Errorf("%s left %s unspelled", c.d, left)
		}
		if len(got) != c.want {
			t.Errorf("%s spells as %d rests, want %d", c.d, len(got), c.want)
		}
		sum := model.Duration{Num: 0, Den: 1}
		for _, el := range got {
			if _, _, ok := el.Duration().Written(); !ok {
				t.Errorf("%s gave a rest of %s, which has no notehead",
					c.d, el.Duration())
			}
			sum = sum.Add(el.Duration())
		}
		if sum.Cmp(c.d) != 0 {
			t.Errorf("%s spelled as rests comes to %s", c.d, sum)
		}
	}
}

// Changing the length of notes that are already in a group.
//
// A group is read against its own arithmetic: a triplet eighth is printed as
// an eighth and lasts a twelfth. Asked for a shorter note, this used to
// refuse — it skipped anything in a tuplet, so selecting the notes of a
// triplet and clicking a sixteenth did nothing at all, with nothing said. The
// group now follows the notes, and the time it gives back becomes a rest
// after it, exactly as it does when the group is first made.

func TestShorteningTheNotesOfATripletKeepsTheTriplet(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")
	// Three triplet eighths and the eighth they gave back.
	if got, want := total(h, 0, 0), (model.Duration{Num: 3, Den: 8}); got.Cmp(want) != 0 {
		t.Fatalf("the bar lasts %s before the change, want %s", got, want)
	}

	// The mistake being corrected: they should have been sixteenths.
	h.post("/api/duration?part=0&measure=0&at=1&value=sixteenth&sel=0:0:0,0:0:1,0:0:2")

	got := spell(t, h, 0, 0)
	want := []string{
		// Still a triplet, still three notes, still their own pitches —
		// and a twenty-fourth each, which is a sixteenth played three in
		// the time of two.
		"note6 1/24 3:2 start",
		"note0 1/24 3:2",
		"note1 1/24 3:2 stop",
		// An eighth of the bar came back when the group was made, and the
		// group is now half as long, so the eighth it was holding comes
		// back too.
		"rest 1/8",
		"rest 1/8",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the bar holds %v,\n                 want %v", got, want)
	}
	// And the bar still lasts what it did, so nothing was lost or invented.
	if want := (model.Duration{Num: 3, Den: 8}); total(h, 0, 0).Cmp(want) != 0 {
		t.Errorf("the bar now lasts %s, want %s", total(h, 0, 0), want)
	}
}

// The notehead is what was asked for. This is the part that would be wrong if
// the plain length were written straight onto the note: the note would last a
// sixteenth and be printed as a dotted thirty-second, and the group would no
// longer add up to anything.
func TestATupletMemberShowsTheNoteheadThatWasAskedFor(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")
	h.post("/api/duration?part=0&measure=0&at=1&value=32nd&sel=0:0:0,0:0:1,0:0:2")

	for i, el := range h.elements(0, 0)[:3] {
		mk := marksOfElement(el)
		if mk == nil || mk.Tuplet == nil {
			t.Fatalf("note %d left the group", i)
		}
		written := mk.WrittenDuration(el.Duration())
		value, dots, ok := written.Written()
		if !ok {
			t.Errorf("note %d shows %s, which is not a notehead", i, written)
			continue
		}
		if value != model.ThirtySecond || dots != 0 {
			t.Errorf("note %d shows a %s with %d dots, want a thirty-second",
				i, value, dots)
		}
	}
}

// Only some of the group: a triplet of a quarter and two eighths is ordinary
// writing, and the time that one longer note gives back is still the bar's.
func TestChangingOneMemberOfAGroupGivesBackItsOwnTime(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=eighth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")
	h.post("/api/duration?part=0&measure=0&at=1&value=sixteenth&sel=0:0:1")

	els := h.elements(0, 0)
	if mk := marksOfElement(els[1]); mk == nil || mk.Tuplet == nil {
		t.Fatal("the note that changed left the group")
	}
	// The outer two are untouched twelfths; the middle one is a twenty-fourth.
	for i, want := range []model.Duration{{Num: 1, Den: 12}, {Num: 1, Den: 24}, {Num: 1, Den: 12}} {
		if got := els[i].Duration(); got.Cmp(want) != 0 {
			t.Errorf("note %d lasts %s, want %s", i, got, want)
		}
	}
	if want := (model.Duration{Num: 3, Den: 8}); total(h, 0, 0).Cmp(want) != 0 {
		t.Errorf("the bar now lasts %s, want %s", total(h, 0, 0), want)
	}
}

// Making them longer is the other direction: the group takes more time than
// it had, the bar overflows, and that is the ordinary overfull bar the
// barline tracking already handles. Nothing is given back, so no rest comes.
func TestLengtheningTheNotesOfATripletAddsNoRest(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{4, 5, 6} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=sixteenth&kind=note", step))
	}
	h.post("/api/tuplet?part=0&measure=0&at=1&n=3&sel=0:0:0,0:0:1,0:0:2")
	before := len(h.elements(0, 0))
	h.post("/api/duration?part=0&measure=0&at=1&value=eighth&sel=0:0:0,0:0:1,0:0:2")

	rests := 0
	for _, el := range h.elements(0, 0) {
		if _, ok := el.(*model.Rest); ok {
			rests++
		}
	}
	if got := len(h.elements(0, 0)); got > before {
		t.Errorf("the bar gained %d elements and now has %d rests; "+
			"a longer group gives nothing back", got-before, rests)
	}
}
