/*
 * pastebars_test.go
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
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Music copied with a barline in it comes back with the barline in it.
 *
 * The clipboard held a flat list of notes and paste poured the lot into the
 * one bar it was asked for: two bars of four quarters came back as eight
 * quarters inside a bar of four, which is not a bar. Reported as "when I
 * paste it does not paste the bar", about a passage picked up across a
 * barline.
 *
 * So the clipboard remembers where the barlines fell and paste puts them
 * back: one bar of the copy to each bar from the cursor on.
 */

// barsOfQuarters writes n bars of four quarter notes, each bar a step higher
// than the last so that every note can be told from every other.
func barsOfQuarters(h *harness, n int) {
	h.t.Helper()
	for m := 0; m < n; m++ {
		for i := 0; i < 4; i++ {
			h.note(0, m, m*2+i, "quarter")
		}
	}
}

// barsAsWritten is what each bar of the first staff holds, as written.
func barsAsWritten(h *harness) []string {
	h.t.Helper()
	var out []string
	for _, m := range h.app.score.Parts[0].Measures {
		var bar []string
		for _, el := range m.Voices[0].Elements {
			switch e := el.(type) {
			case *model.Note:
				bar = append(bar, e.Pitch.String())
			case *model.Rest:
				bar = append(bar, "rest")
			default:
				bar = append(bar, fmt.Sprintf("%T", el))
			}
		}
		out = append(out, strings.Join(bar, " "))
	}
	return out
}

func TestTwoBarsCopiedPasteAsTwoBars(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 4)
	was := barsAsWritten(h)

	h.post("/api/copy?sel=0:0:0,0:0:1,0:0:2,0:0:3,0:1:0,0:1:1,0:1:2,0:1:3")
	h.post("/api/paste?part=0&measure=2&at=0")

	got := barsAsWritten(h)
	if len(got) != 4 {
		t.Fatalf("%d bars after pasting two over two, want 4: %q", len(got), got)
	}
	if got[2] != was[0] || got[3] != was[1] {
		t.Errorf("bars 3 and 4 are %q and %q, want %q and %q",
			got[2], got[3], was[0], was[1])
	}
	if got[0] != was[0] || got[1] != was[1] {
		t.Errorf("the bars copied from changed: %q", got[:2])
	}
}

// The copy says how many bars it holds, so the editor can say so too.
func TestACopySaysHowManyBarsItHolds(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 2)

	one := h.post("/api/copy?sel=0:0:0,0:0:1")
	if n, _ := one["bars"].(float64); n != 1 {
		t.Errorf("a run inside one bar says %v bars", one["bars"])
	}
	two := h.post("/api/copy?sel=0:0:3,0:1:0")
	if n, _ := two["bars"].(float64); n != 2 {
		t.Errorf("a run across a barline says %v bars", two["bars"])
	}
}

// A run inside one bar is a run of notes and lands as one, wherever it is
// put: nothing here may turn an ordinary copy into a bar.
func TestARunInsideOneBarPastesAsARun(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 3)

	h.post("/api/copy?sel=0:0:0,0:0:1")
	h.post("/api/paste?part=0&measure=2&at=2")

	got := barsAsWritten(h)
	// The two notes went in at the third place of bar 3 and took the place
	// of the two that were there. The bar is still a bar.
	if n := len(strings.Fields(got[2])); n != 4 {
		t.Errorf("bar 3 holds %d notes after pasting two into it: %q", n, got[2])
	}
	if len(got) != 3 {
		t.Errorf("%d bars, want the three there were: %q", len(got), got)
	}
}

// Picked up across a barline halfway through each bar, it lands halfway
// through each bar: the barline is where it was, not the notes either side
// of it shuffled into one.
func TestAPassageAcrossABarlineKeepsTheBarline(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 4)
	was := barsAsWritten(h)

	h.post("/api/copy?sel=0:0:2,0:0:3,0:1:0,0:1:1")
	h.post("/api/paste?part=0&measure=2&at=0")

	got := barsAsWritten(h)
	firstTwo := strings.Fields(was[0])[2:]
	nextTwo := strings.Fields(was[1])[:2]
	if w := strings.Join(append(firstTwo, strings.Fields(was[2])[2:]...), " "); got[2] != w {
		t.Errorf("bar 3 is %q, want %q", got[2], w)
	}
	if w := strings.Join(append(nextTwo, strings.Fields(was[3])[2:]...), " "); got[3] != w {
		t.Errorf("bar 4 is %q, want %q", got[3], w)
	}
}

// Pasting near the end makes the bars it needs rather than piling up in the
// last one.
func TestPastingBarsPastTheEndMakesThem(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 3)

	h.post("/api/copy?sel=0:0:0,0:0:1,0:0:2,0:0:3,0:1:0,0:1:1,0:1:2,0:1:3")
	h.post("/api/paste?part=0&measure=2&at=0")

	got := barsAsWritten(h)
	if len(got) < 4 {
		t.Fatalf("%d bars after pasting two at the last one: %q", len(got), got)
	}
	for i := 2; i < 4; i++ {
		if n := len(strings.Fields(got[i])); n != 4 {
			t.Errorf("bar %d holds %d notes: %q", i+1, n, got[i])
		}
	}
}

/*
 * And a run of bars can be picked up in the first place.
 *
 * The band over a bar took one bar and had no way to say any more, so a
 * passage — which is almost never one bar — had to be picked up note by
 * note. Reported as "I want to be able to select multiple bars, here there
 * is only one".
 */

func TestARunOfBarsIsEmptiedTogether(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 4)

	out := h.post("/api/clearmeasure?part=0&measure=1&to=2")
	if n, _ := out["bars"].(float64); n != 2 {
		t.Errorf("emptying bars 2 to 3 cleared %v of them", out["bars"])
	}
	got := barsAsWritten(h)
	for _, i := range []int{1, 2} {
		if got[i] != "" && got[i] != "rest" {
			t.Errorf("bar %d still holds %q", i+1, got[i])
		}
	}
	if got[0] == "" || got[3] == "" {
		t.Errorf("the bars either side were emptied too: %q", got)
	}
}

// One bar is a run of one, so the old request still means what it did.
func TestEmptyingOneBarStillEmptiesOne(t *testing.T) {
	h := newHarness(t)
	barsOfQuarters(h, 3)

	h.post("/api/clearmeasure?part=0&measure=1")
	got := barsAsWritten(h)
	if got[1] != "" && got[1] != "rest" {
		t.Errorf("bar 2 still holds %q", got[1])
	}
	if got[0] == "" || got[2] == "" {
		t.Errorf("a neighbour was emptied too: %q", got)
	}
}

/*
 * And music copied from two lines of one staff is two lines.
 *
 * The clipboard held a flat list and paste poured it into whichever line the
 * pen was on, so a bar of a piano part came back with both hands in the
 * upper voice: the rests of one line sitting among the notes of the other,
 * at the wrong height because the height a rest is drawn at depends on which
 * line it belongs to. Reported as "I copied bar 23 in 24 as they are the
 * same, however the output was different. I had to move the rest down."
 */

// lineAsWritten is what one line of music of one bar holds.
func lineAsWritten(h *harness, measure, voice int) []string {
	h.t.Helper()
	m := h.app.score.Parts[0].Measures[measure]
	if voice >= len(m.Voices) {
		return nil
	}
	var out []string
	for _, el := range m.Voices[voice].Elements {
		switch e := el.(type) {
		case *model.Note:
			out = append(out, e.Pitch.String())
		case *model.Rest:
			out = append(out, fmt.Sprintf("rest(%d)", e.Lift))
		default:
			out = append(out, fmt.Sprintf("%T", el))
		}
	}
	return out
}

// twoLines writes a bar of a staff two voices share, in the shape of the one
// that was reported: a rest and a note in the upper line, two notes under
// them in the lower.
func twoLines(h *harness, measure int) {
	h.t.Helper()
	h.post(fmt.Sprintf(
		"/api/insert?part=0&measure=%d&at=-1&step=4&dur=half&kind=rest", measure))
	h.post(fmt.Sprintf(
		"/api/insert?part=0&measure=%d&at=-1&step=6&dur=half&kind=note", measure))
	for i := 0; i < 2; i++ {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=%d&at=-1&voice=1&step=%d&dur=half&kind=note",
			measure, i))
	}
}

func TestABarOfTwoLinesPastesAsTwoLines(t *testing.T) {
	h := newHarness(t)
	twoLines(h, 0)
	h.note(0, 1, 4, "whole")

	// The rest has been put where the writer wanted it, which is the thing
	// that made the difference visible.
	rest := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Rest)
	rest.Lift = -4

	// A whole bar, as clicking its band gives it: every element of every
	// line, in the order the bar is played.
	h.post("/api/copy?sel=0:0:0,0:0:1/0,0:0:1,0:0:1/1")
	h.post("/api/paste?part=0&measure=1&at=0")

	for _, v := range []int{0, 1} {
		was, got := lineAsWritten(h, 0, v), lineAsWritten(h, 1, v)
		if len(got) != len(was) {
			t.Errorf("line %d: copied %v and got %v", v+1, was, got)
			continue
		}
		for i := range was {
			if got[i] != was[i] {
				t.Errorf("line %d: copied %v and got %v", v+1, was, got)
				break
			}
		}
	}
}

// Where a rest was put by hand comes with it: the copy is of the music as
// written, and that includes what the writer said about it.
func TestWhereARestWasPutIsCopiedWithIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=whole&kind=rest")
	h.note(0, 1, 4, "whole")
	h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Rest).Lift = -3

	h.post("/api/copy?sel=0:0:0")
	h.post("/api/paste?part=0&measure=1&at=0")

	rest, ok := h.app.score.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Rest)
	if !ok {
		t.Fatalf("the copy came back as %T",
			h.app.score.Parts[0].Measures[1].Voices[0].Elements[0])
	}
	if rest.Lift != -3 {
		t.Errorf("the copied rest is lifted %d, want -3", rest.Lift)
	}
}

// A copy of one line of music still goes wherever the pen is, which is how a
// phrase is moved from one voice to the other.
func TestACopyOfOneLineGoesWhereThePenIs(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 4, "half")
	h.note(0, 0, 6, "half")
	h.note(0, 1, 4, "whole")

	h.post("/api/copy?sel=0:0:0,0:0:1")
	h.post("/api/paste?part=0&measure=1&at=0&voice=1")

	if got := lineAsWritten(h, 1, 1); len(got) != 2 {
		t.Errorf("the second line of bar 2 holds %v, want the two copied notes", got)
	}
	if got := lineAsWritten(h, 1, 0); len(got) != 1 {
		t.Errorf("the first line of bar 2 holds %v, want the note that was there", got)
	}
}

// The copy says how many lines of music it holds, as it says how many bars.
func TestACopySaysHowManyLinesItHolds(t *testing.T) {
	h := newHarness(t)
	twoLines(h, 0)

	one := h.post("/api/copy?sel=0:0:0,0:0:1")
	if n, _ := one["voices"].(float64); n != 1 {
		t.Errorf("a run of one line says %v lines", one["voices"])
	}
	two := h.post("/api/copy?sel=0:0:0,0:0:1/0")
	if n, _ := two["voices"].(float64); n != 2 {
		t.Errorf("a copy across two lines says %v lines", two["voices"])
	}
}
