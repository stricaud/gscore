/*
 * tokey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Transposing to a key, as against by an interval: "put it in B flat".
 */

package app

import (
	"strings"
	"testing"
)

func TestIntervalToKey(t *testing.T) {
	cases := []struct {
		from, to     int
		direction    string
		steps, semis int
	}{
		{0, 1, "up", 4, 7},        // C to G, up a fifth
		{0, 1, "closest", -3, -5}, // or down a fourth, which is nearer
		{0, -1, "closest", 3, 5},  // C to F: up a fourth
		{0, 5, "closest", -1, -1}, // C to B: down a semitone
		{3, -2, "up", 1, 1},       // A to B flat: up a semitone
		{3, -2, "down", -6, -11},  // or down a major seventh
		{0, 7, "up", 0, 1},        // C to C sharp: an augmented unison
		{0, 6, "closest", 3, 6},   // C to F sharp: a tritone, up
		{0, 0, "closest", 0, 0},   // nowhere
	}
	for _, c := range cases {
		steps, semis := intervalToKey(c.from, c.to, c.direction)
		if steps != c.steps || semis != c.semis {
			t.Errorf("from %d to %d %s: %d letters %d semitones, want %d and %d",
				c.from, c.to, c.direction, steps, semis, c.steps, c.semis)
		}
	}
}

func TestAPieceCanBePutInAKey(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(1)
	h.note(0, 0, 2, "quarter") // a G and a B, in C major
	h.note(0, 0, 4, "quarter")
	before := pitches(h)

	d := h.post("/api/transpose?tokey=1&direction=up")
	if d["fifths"] != float64(1) || d["steps"] != float64(4) || d["semitones"] != float64(7) {
		t.Errorf("to G major up: %v", d)
	}
	after := pitches(h)
	if after == before || !strings.Contains(after, "D5") {
		t.Errorf("the notes went from %q to %q", before, after)
	}

	// Back to C, down the fifth it went up: asked for the nearer way it
	// would go up a fourth instead, which is nearer.
	d = h.post("/api/transpose?tokey=0&direction=down")
	if d["fifths"] != float64(0) || d["steps"] != float64(-4) || d["semitones"] != float64(-7) {
		t.Errorf("back to C: %v", d)
	}
	if got := pitches(h); got != before {
		t.Errorf("the notes came back as %q, were %q", got, before)
	}

	// The key it is in already is nowhere to go.
	if msg := h.postFails("/api/transpose?tokey=0"); !strings.Contains(msg, "already") {
		t.Errorf("transposing to the same key was answered %q", msg)
	}
	// And a key that is not one.
	if msg := h.postFails("/api/transpose?tokey=9"); !strings.Contains(msg, "key signature") {
		t.Errorf("tokey=9 was answered %q", msg)
	}

	// Asked for C sharp major, the signature is C sharp major's seven
	// sharps and not D flat's five flats.
	d = h.post("/api/transpose?tokey=7&direction=up")
	if d["fifths"] != float64(7) {
		t.Errorf("to C sharp major: %v", d)
	}
}
