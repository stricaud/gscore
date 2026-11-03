/*
 * fretchord_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing a chord by the shape of the hand.
 *
 * The shape is written lowest-sounding string first, which is how every
 * chord diagram is written and is the other way round from how a tablature
 * numbers its strings — string one is the top line. That turning-round is
 * the thing most likely to be wrong and the thing least likely to show:
 * a shape read backwards is still six notes and still a chord, just not
 * the one anybody asked for.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// builtChord is the first chord written on a staff. Not chordOn, which
// belongs to the chord *symbol* tests and answers about the words over a
// staff rather than the notes on it.
func builtChord(t *testing.T, h *harness, part int) *model.Chord {
	t.Helper()
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if c, ok := el.(*model.Chord); ok {
					return c
				}
			}
		}
	}
	t.Fatal("no chord was written")
	return nil
}

func aGuitar(t *testing.T, h *harness) {
	t.Helper()
	h.post("/api/staff?part=0&tab=guitar")
}

// x02210 is an A minor, and the program has to say so — that is what the
// builder shows while somebody is still moving fingers about.
func TestAShapeIsNamed(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)

	for _, tc := range []struct{ frets, want string }{
		{"x02210", "A minor"},
		{"320003", "G major"},
		{"x32010", "C major"},
		{"xx0232", "D major"},
	} {
		out := h.post("/api/fretchord?part=0&measure=0&frets=" + tc.frets + "&preview=1")
		if got, _ := out["name"].(string); got != tc.want {
			t.Errorf("%s is %q, want %q", tc.frets, got, tc.want)
		}
	}
}

// A preview writes nothing. The builder asks on every click, and a dialog
// that wrote a chord each time somebody tried a finger somewhere would be
// unusable.
func TestAPreviewWritesNothing(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	h.post("/api/fretchord?part=0&measure=0&frets=x02210&preview=1")

	for _, m := range h.app.score.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, ok := el.(*model.Chord); ok {
					t.Fatal("the preview wrote a chord")
				}
			}
		}
	}
}

// The shape is read lowest string first, so the x in x02210 mutes the
// bottom string and not the top one.
//
// Read the other way round it is still five notes and still a chord — a
// different chord, on different strings — so only looking at where the
// fingers landed catches it.
func TestTheShapeIsReadLowestStringFirst(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	h.post("/api/fretchord?part=0&measure=0&at=0&frets=x02210&dur=quarter")

	c := builtChord(t, h, 0)
	got := map[int]int{} // string -> fret
	for _, n := range c.Notes {
		got[n.String] = n.Fret
	}
	// x 0 2 2 1 0 against strings 6 5 4 3 2 1.
	want := map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}
	if len(got) != len(want) {
		t.Fatalf("it wrote %d notes: %v", len(got), got)
	}
	for str, fret := range want {
		if got[str] != fret {
			t.Errorf("string %d is at fret %d, want %d (all: %v)",
				str, got[str], fret, got)
		}
	}
	if _, muted := got[6]; muted {
		t.Error("the bottom string was played; the x mutes it")
	}
}

// A chord is drawn and read from the bottom up, so the noteheads are kept
// in that order.
func TestTheChordIsStoredFromTheBottomUp(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	h.post("/api/fretchord?part=0&measure=0&at=0&frets=320003&dur=quarter")

	c := builtChord(t, h, 0)
	for i := 1; i < len(c.Notes); i++ {
		if c.Notes[i].Pitch.MIDI() < c.Notes[i-1].Pitch.MIDI() {
			t.Fatalf("note %d is below the one before it: %v", i, noteNames(c))
		}
	}
}

// And by what they sound rather than by which string they are on, which is
// not the same question on a ukulele: its fourth string is a G above the
// third string's C. A reentrant tuning is the character of the instrument,
// not an oddity, and string order there is not pitch order.
func TestAReentrantTuningIsStillStoredFromTheBottomUp(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=ukulele")
	if h.app.score.Parts[0].Tab == nil {
		t.Skip("this build has no ukulele tuning")
	}
	// Check the tuning really is reentrant, or the test proves nothing.
	strs := h.app.score.Parts[0].Tab.Strings
	if len(strs) < 4 || strs[3].MIDI() <= strs[2].MIDI() {
		t.Skipf("this ukulele is not reentrant: %v", strs)
	}

	h.post("/api/fretchord?part=0&measure=0&at=0&frets=0003&dur=quarter")
	c := builtChord(t, h, 0)
	for i := 1; i < len(c.Notes); i++ {
		if c.Notes[i].Pitch.MIDI() < c.Notes[i-1].Pitch.MIDI() {
			t.Fatalf("note %d is below the one before it: %v", i, noteNames(c))
		}
	}
}

// Frets past nine need separating, because a chord high on the neck is not
// unusual and "x0221000" cannot say a tenth fret.
func TestASeparatedShapeReachesPastTheNinthFret(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	h.post("/api/fretchord?part=0&measure=0&at=0&frets=x,0,2,2,10,0&dur=quarter")

	c := builtChord(t, h, 0)
	found := false
	for _, n := range c.Notes {
		if n.String == 2 && n.Fret == 10 {
			found = true
		}
	}
	if !found {
		t.Errorf("there is no tenth fret in it: %v", c.Notes)
	}
}

func TestAShapeThatIsNotTheInstrumentIsRefused(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	for _, tc := range []struct{ frets, says string }{
		{"x022", "strings"},
		{"x0221000", "strings"},
		{"xxxxxx", "muted"},
		{"x0221q", "not a fret"},
		{"", "shape"},
	} {
		got := h.postFails("/api/fretchord?part=0&measure=0&frets=" + tc.frets)
		if !strings.Contains(got, tc.says) {
			t.Errorf("%q said %q, want something about %q", tc.frets, got, tc.says)
		}
	}
}

func TestAChordOnAStaffOfPitchesIsRefused(t *testing.T) {
	h := newHarness(t)
	got := h.postFails("/api/fretchord?part=0&measure=0&frets=x02210")
	if !strings.Contains(got, "tablature") {
		t.Errorf("it said %q", got)
	}
}

// A fret past the end of the neck is refused rather than written as a note
// nobody can play.
func TestAShapeOffTheEndOfTheNeckIsRefused(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)
	if got := h.postFails(
		"/api/fretchord?part=0&measure=0&frets=x,0,2,2,1,99"); got == "" {
		t.Error("a 99th fret was allowed")
	}
}

// It goes in where a single fret would, and says where it landed, so the
// caret can follow it: into the bar rather than behind its rest, and on
// into the next bar when this one is full.
func TestAChordFillsTheBarAndThenTheNextOne(t *testing.T) {
	h := newHarness(t)
	aGuitar(t, h)

	var last map[string]any
	for i := 0; i < 5; i++ {
		last = h.post("/api/fretchord?part=0&measure=0&at=" +
			itoa(i) + "&frets=x02210&dur=quarter")
	}
	if got, ok := last["measure"].(float64); !ok || int(got) != 1 {
		t.Errorf("the fifth chord says bar %v, want bar 1", last["measure"])
	}
	// Four in the first bar and nothing left over in front of them.
	first := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if len(first) != 4 {
		t.Errorf("the first bar holds %d things, want 4", len(first))
	}
}
