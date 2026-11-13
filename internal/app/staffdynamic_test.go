/*
 * staffdynamic_test.go
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

/*
 * A dynamic belongs to the staff, not to one line of it.
 *
 * "f" under a piano's right hand is an instruction to the hand: everything
 * it plays from there is forte, the line that moves and the note held
 * underneath alike. It is written once, on one note, because writing it on
 * every voice would print it twice on the page — so the one that is written
 * has to reach all of them.
 *
 * It did not. The playback read the dynamic off whichever element it was
 * walking and walked the voices one after another, so a mark on the second
 * voice reached the second voice and left the first at whatever it had been.
 * Four quarters over a held semibreve, with the held note marked forte, came
 * out as 96 underneath and 80 above — at the same instant. Reported as
 * "setting forte on one note in one voice must apply to both without having
 * me type f to both".
 */

// twoVoiceBar is four quarters in voice 1 over a held semibreve in voice 2,
// with a dynamic written on whichever of them the test names.
func twoVoiceBar(t *testing.T, on string) *harness {
	t.Helper()
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")
	h.post("/api/dynamic?dynamic=f&sel=" + on)
	return h
}

// velocities is how loud everything comes out, in the order it is played.
func velocities(h *harness) []int {
	pb := buildPlayback(h.app.score)
	out := make([]int, 0, len(pb.Events))
	for _, e := range pb.Events {
		out = append(out, e.Velocity)
	}
	return out
}

func allSame(v []int) bool {
	for _, x := range v {
		if x != v[0] {
			return false
		}
	}
	return len(v) > 0
}

func TestADynamicOnTheSecondVoiceIsHeardByTheFirst(t *testing.T) {
	h := twoVoiceBar(t, "0:0:1/0") // on the held note, which is voice 2
	v := velocities(h)
	if len(v) != 5 {
		t.Fatalf("%d notes sounded, wanted 5", len(v))
	}
	if !allSame(v) {
		t.Errorf("the bar came out at %v; one forte applies to the whole staff", v)
	}
}

func TestADynamicOnTheFirstVoiceIsHeardByTheSecond(t *testing.T) {
	h := twoVoiceBar(t, "0:0:0") // on the first quarter, which is voice 1
	v := velocities(h)
	if len(v) != 5 {
		t.Fatalf("%d notes sounded, wanted 5", len(v))
	}
	if !allSame(v) {
		t.Errorf("the bar came out at %v", v)
	}
}

func TestItIsStillWrittenOnlyOnce(t *testing.T) {
	// Heard by every voice and printed once. Spreading it through the model
	// instead would put an f under every line of the staff, which is what
	// the person who wrote it once was avoiding.
	h := twoVoiceBar(t, "0:0:1/0")
	marked := 0
	for _, v := range h.app.score.Parts[0].Measures[0].Voices {
		for _, el := range v.Elements {
			if loudnessOf(el) != model.DynamicNone {
				marked++
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d elements carry the dynamic, wanted the 1 it was written on",
			marked)
	}
}

func TestANoteAlreadySoundingKeepsItsVolume(t *testing.T) {
	// From where it is written and no earlier. A held note that began
	// before the mark arrived keeps the volume it started at, which is
	// what a player does — it is one bow, one breath, one key held down.
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")
	// On the third quarter, halfway through the held note.
	h.post("/api/dynamic?dynamic=f&sel=0:0:2")

	pb := buildPlayback(h.app.score)
	var held, late int
	for _, e := range pb.Events {
		if e.Start == 0 && e.MIDI == 64 {
			held = e.Velocity
		}
		if e.Start > 0.9 && e.MIDI == 71 {
			late = e.Velocity
		}
	}
	if held == 0 || late == 0 {
		t.Fatalf("did not find both notes: held=%d late=%d", held, late)
	}
	if held >= late {
		t.Errorf("the held note is %d and the note under the mark %d; the "+
			"held one began before the forte and should be quieter",
			held, late)
	}
}

func TestTheDynamicCarriesIntoTheNextBar(t *testing.T) {
	// A dynamic holds until another replaces it, across the barline as
	// much as within the bar. Each voice now begins a bar at the volume
	// the bar began at, so the carry has to be the bar's last mark rather
	// than whatever the last voice walked ended on.
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=whole&kind=note")
	h.post("/api/insert?part=0&measure=1&at=-1&step=4&dur=whole&kind=note")
	h.post("/api/dynamic?dynamic=f&sel=0:0:0")

	v := velocities(h)
	if len(v) != 2 {
		t.Fatalf("%d notes sounded, wanted 2", len(v))
	}
	if v[0] != v[1] {
		t.Errorf("bar 1 is %d and bar 2 is %d; the forte should hold", v[0], v[1])
	}
}
