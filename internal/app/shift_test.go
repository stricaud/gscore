/*
 * shift_test.go
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

// Moving a passage along the music.
//
// Ctrl or Cmd with the left and right arrows; the plain ones move which notes
// are picked up and leave the music alone. A bar with room in it — left by a
// tuplet, or by shortening a note — wants the phrase after it walked back
// into that room, and the way to do that was to cut and paste.

// bar says what is in a bar as a row of pitches and rests, which is the only
// thing these tests are about.
func bar(h *harness, part, measure int) []string {
	var out []string
	for _, el := range h.elements(part, measure) {
		switch n := el.(type) {
		case *model.Note:
			out = append(out, fmt.Sprintf("%d", n.Pitch.Step))
		default:
			out = append(out, "r")
		}
	}
	return out
}

func TestAPassageMovesLaterPastTheRestAfterIt(t *testing.T) {
	h := newHarness(t)
	// Two notes, then a rest, in 4/4: C D rest.
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=quarter&kind=rest")

	if got, want := bar(h, 0, 0), []string{"0", "1", "r"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the bar starts as %v, want %v", got, want)
	}
	h.post("/api/shift?dir=1&sel=0:0:0,0:0:1")
	// The rest is now in front of them: the pair moved one place later and
	// kept its own order.
	if got, want := bar(h, 0, 0), []string{"r", "0", "1"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("after moving later the bar is %v, want %v", got, want)
	}
}

func TestAPassageMovesEarlierPastTheRestBeforeIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=quarter&kind=rest")
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")

	h.post("/api/shift?dir=-1&sel=0:0:1,0:0:2")
	if got, want := bar(h, 0, 0), []string{"0", "1", "r"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("after moving earlier the bar is %v, want %v", got, want)
	}
}

// Where the notes ended up, because the selection has to follow them: they
// are not where it named them by any more.
func TestMovingAPassageSaysWhereItWent(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=quarter&kind=rest")

	d := h.post("/api/shift?dir=1&sel=0:0:0,0:0:1")
	now, ok := d["now"].([]any)
	if !ok || len(now) != 2 {
		t.Fatalf("it reported %v, want two places", d["now"])
	}
	if got, want := fmt.Sprint(now), "[0:0:1 0:0:2]"; got != want {
		t.Errorf("the notes are reported at %s, want %s", got, want)
	}
}

// A passage at a barline moves across it, because a phrase does not stop
// being a phrase where the bar ends — and the bar it left does not stay a
// beat short.
func TestAPassageMovesAcrossABarline(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	}
	h.post("/api/insert?part=0&measure=1&at=-1&step=0&dur=quarter&kind=rest")
	h.post("/api/insert?part=0&measure=1&at=-1&step=6&dur=quarter&kind=note")

	h.post("/api/shift?dir=-1&sel=0:1:0")
	if got := bar(h, 0, 0); got[len(got)-1] != "r" {
		t.Errorf("the first bar ends %v, want it to end with the rest", got)
	}
}

// A note in a tuplet is part of a group, and the figure is printed over the
// group: moving one member out of it would leave the figure over a note that
// is somewhere else.
func TestAPassageWillNotMovePastATupletMember(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=quarter&kind=note")
	h.post("/api/tuplet?part=0&measure=0&at=3&n=3")

	if msg := h.postFails("/api/shift?dir=1&sel=0:0:0"); msg == "" {
		t.Error("a note moved into the middle of a tuplet")
	}
}

// The ends of the music are the ends: there is nowhere further to go, and
// saying so is better than quietly doing nothing.
func TestAPassageAtTheEndsOfTheMusicSaysSo(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=5&dur=quarter&kind=note")

	if msg := h.postFails("/api/shift?dir=-1&sel=0:0:0"); msg == "" {
		t.Error("the first note of the piece moved earlier")
	}
	if msg := h.postFails("/api/shift?dir=1&sel=0:0:0"); msg == "" {
		t.Error("the last note of the piece moved later")
	}
}

// Moving it there and back leaves the music as it was, which is the thing
// that makes the arrows safe to lean on.
func TestMovingAPassageAndBackLeavesItAsItWas(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{5, 6, 7} {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=0&at=-1&step=%d&dur=quarter&kind=note", step))
	}
	h.post("/api/insert?part=0&measure=0&at=-1&step=0&dur=quarter&kind=rest")
	was := fmt.Sprint(bar(h, 0, 0))

	h.post("/api/shift?dir=1&sel=0:0:0,0:0:1,0:0:2")
	h.post("/api/shift?dir=-1&sel=0:0:1,0:0:2,0:0:3")
	if got := fmt.Sprint(bar(h, 0, 0)); got != was {
		t.Errorf("there and back gives %s, want %s", got, was)
	}
}
