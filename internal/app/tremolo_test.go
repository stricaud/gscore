/*
 * tremolo_test.go
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
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tremolo is shorthand for a bar of repeated notes, and has to be heard as
// one.
//
// Three slashes through a half note mean sixteen thirty-second notes. Drawn
// and then ignored — which is what a mark the player never hears amounts to —
// the shorthand silently means something different from what it says.

func TestATremoloIsHeardAsTheNotesItStandsFor(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "half")
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3")

	h.app.mu.RLock()
	pb := buildPlayback(h.app.score)
	h.app.mu.RUnlock()

	// A half note at the score's own tempo, divided into thirty-seconds:
	// eight of them to the half note at 120, and sixteen of them here.
	if len(pb.Events) != 16 {
		t.Fatalf("the tremolo sounded %d notes; three slashes on a half note "+
			"are sixteen thirty-seconds", len(pb.Events))
	}
	// All the same pitch, evenly spaced, and together they last exactly as
	// long as the note they are written on.
	first := pb.Events[0]
	var last Event
	for i, ev := range pb.Events {
		if ev.MIDI != first.MIDI {
			t.Errorf("note %d is a different pitch: a tremolo repeats one note", i)
		}
		last = ev
	}
	total := last.Start + last.Dur - first.Start
	if want := 1.0; math.Abs(total-want) > 1e-6 {
		t.Errorf("the tremolo lasts %.4fs, want %.4f — the half note's own length",
			total, want)
	}
	gap := pb.Events[1].Start - pb.Events[0].Start
	if want := 1.0 / 16; math.Abs(gap-want) > 1e-6 {
		t.Errorf("the repeats are %.4fs apart, want %.4f", gap, want)
	}
}

// The number of slashes is the number of beams of the note being repeated,
// whatever note they are drawn through: one is eighths, three is
// thirty-seconds. Twice as many notes for each slash added.
func TestEachSlashHalvesTheRepeatedNote(t *testing.T) {
	var last int
	for strokes := 1; strokes <= 4; strokes++ {
		h := newHarness(t)
		h.note(0, 0, 6, "half")
		h.post("/api/tremolo?part=0&measure=0&at=1&strokes=" + itoa(strokes))

		h.app.mu.RLock()
		pb := buildPlayback(h.app.score)
		h.app.mu.RUnlock()

		want := 1 << (strokes + 1) // a half note holds 4 eighths, 8 sixteenths…
		if len(pb.Events) != want {
			t.Errorf("%d slashes sounded %d notes, want %d",
				strokes, len(pb.Events), want)
		}
		if last > 0 && len(pb.Events) != last*2 {
			t.Errorf("%d slashes gave %d notes after %d: each slash doubles them",
				strokes, len(pb.Events), last)
		}
		last = len(pb.Events)
	}
}

// A buzz roll has no division to count, so it is taken as fast as the
// instrument answers rather than as some number of notes.
func TestABuzzRollIsUnmeasured(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "half")
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=0&buzz=1")

	h.app.mu.RLock()
	pb := buildPlayback(h.app.score)
	h.app.mu.RUnlock()

	// Faster than any written division of the note, which is the point of it.
	if len(pb.Events) <= 16 {
		t.Errorf("a buzz roll over a half note sounded %d strokes; it is meant "+
			"to be faster than the thirty-seconds three slashes would give",
			len(pb.Events))
	}
	el := h.elements(0, 0)[0]
	m := marksOfElement(el)
	if m == nil || m.Tremolo == nil || !m.Tremolo.Buzz {
		t.Error("the note is not marked as a buzz roll")
	}
}

// A tremolo on a chord strikes the whole chord again and again. Taken
// notehead by notehead it would be an arpeggio, which is a different mark.
func TestATremoloOnAChordRepeatsTheWholeChord(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 4, "half")
	h.post("/api/addpitch?part=0&measure=0&at=1&step=6")
	h.post("/api/addpitch?part=0&measure=0&at=1&step=8")
	if _, ok := h.elements(0, 0)[0].(*model.Chord); !ok {
		t.Fatalf("the test did not build a chord: %T", h.elements(0, 0)[0])
	}
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=2")

	h.app.mu.RLock()
	pb := buildPlayback(h.app.score)
	h.app.mu.RUnlock()

	// Eight sixteenths, three notes in each.
	if len(pb.Events) != 8*3 {
		t.Fatalf("the chord tremolo sounded %d events, want %d", len(pb.Events), 24)
	}
	// The three of each repeat start together, which is what makes it a
	// chord rather than a roll through it.
	for i := 0; i < len(pb.Events); i += 3 {
		a, b, c := pb.Events[i], pb.Events[i+1], pb.Events[i+2]
		if math.Abs(a.Start-b.Start) > 1e-9 || math.Abs(a.Start-c.Start) > 1e-9 {
			t.Errorf("repeat %d is spread over time: %.4f %.4f %.4f",
				i/3, a.Start, b.Start, c.Start)
		}
	}
}

// Taking it off leaves the note sounding once, as it is written.
func TestATremoloCanBeTakenOff(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "half")
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3")
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=0")

	h.app.mu.RLock()
	pb := buildPlayback(h.app.score)
	h.app.mu.RUnlock()
	if len(pb.Events) != 1 {
		t.Errorf("the note sounds %d times after the tremolo was removed",
			len(pb.Events))
	}
}
