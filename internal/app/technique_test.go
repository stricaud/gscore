/*
 * technique_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Putting a tab mark on a note.
 *
 * A tab says which string and how far up it, and nothing about how it is
 * played. These are the rest of the instruction, and the thing to check is
 * that they go on the note that was pointed at, come off it again, and that
 * the two groups a note can only hold one of behave like groups.
 */

package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// techniquesOn is what the first note of a staff carries.
func techniquesOn(t *testing.T, h *harness, part int) []model.Technique {
	t.Helper()
	return firstNote(t, h, part).Marks.Techniques
}

// writeOneNote puts a single note on a tab staff and says where it is.
func writeOneNote(t *testing.T, h *harness) (measure, at int) {
	t.Helper()
	h.post("/api/staff?part=0&tab=guitar")
	d := h.post("/api/fret?part=0&measure=0&at=0&string=1&fret=5&dur=quarter")
	return int(d["measure"].(float64)), int(d["at"].(float64)) + 1
}

func TestATabMarkGoesOnTheNote(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	for _, name := range []string{"hammer-on", "palm-mute"} {
		h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=%s", m, at, name))
	}

	got := techniquesOn(t, h, 0)
	if len(got) != 2 {
		t.Fatalf("the note carries %v", got)
	}
	// Both, because a note can be hammered onto and palm-muted at once.
	if got[0] != model.HammerOn || got[1] != model.PalmMute {
		t.Errorf("it carries %v", got)
	}
}

// Only one of the four that reach to the next note. A note is hammered onto
// or slid from, and a palette clicked twice must not leave both on it with
// no way to see which was meant.
func TestOnlyOneMarkThatReachesOnwardsStays(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=hammer-on", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=slide-up", m, at))

	got := techniquesOn(t, h, 0)
	if len(got) != 1 || got[0] != model.SlideUp {
		t.Errorf("the note carries %v, want just the slide", got)
	}
}

// Likewise the bends: a note bends one distance.
func TestANoteBendsOneDistance(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=bend", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=bend-half", m, at))

	got := techniquesOn(t, h, 0)
	if len(got) != 1 || got[0] != model.BendHalf {
		t.Errorf("the note carries %v, want just the semitone bend", got)
	}
}

// And a mark from one group does not displace one from the other.
func TestABendAndAHammerOnLiveTogether(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=hammer-on", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=bend", m, at))

	got := techniquesOn(t, h, 0)
	if len(got) != 2 {
		t.Errorf("the note carries %v, want both", got)
	}
}

// Taking one away leaves the others, which is what a note carrying three of
// them needs: clearing the lot to turn off a palm mute would lose the
// hammer-on with it.
func TestOneMarkCanBeTakenAway(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=hammer-on", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=palm-mute", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=palm-mute&off=1", m, at))

	got := techniquesOn(t, h, 0)
	if len(got) != 1 || got[0] != model.HammerOn {
		t.Errorf("the note carries %v, want just the hammer-on", got)
	}
}

func TestAnEmptyTechniqueClearsTheLot(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=hammer-on", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=palm-mute", m, at))
	h.post(fmt.Sprintf("/api/technique?part=0&measure=%d&at=%d&technique=", m, at))

	if got := techniquesOn(t, h, 0); len(got) != 0 {
		t.Errorf("the note still carries %v", got)
	}
}

func TestAMarkNobodyHasIsRefused(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	got := h.postFails(fmt.Sprintf(
		"/api/technique?part=0&measure=%d&at=%d&technique=whammy-gargle", m, at))
	if !strings.Contains(got, "unknown technique") {
		t.Errorf("it said %q", got)
	}
	if n := techniquesOn(t, h, 0); len(n) != 0 {
		t.Errorf("it put something on the note anyway: %v", n)
	}
}

/*
 * The dead note.
 *
 * Written as an x where the fret number would be. It is the shape of the
 * note and not a mark on it — the same Head a drum staff gives a cymbal —
 * so it goes through the notehead endpoint, and this is what says so: a
 * second, tab-only way of writing the same thing is how a file ends up
 * with two marks that mean one thing and no way to tell which was meant.
 */

func TestADeadNoteIsANoteheadAndNotAMark(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	h.post(fmt.Sprintf("/api/notehead?part=0&measure=%d&at=%d&shape=x", m, at))

	n := firstNote(t, h, 0)
	if n.Head != model.HeadX {
		t.Errorf("the note is drawn as %v", n.Head)
	}
	if len(n.Marks.Techniques) != 0 {
		t.Errorf("it also got a mark of its own: %v", n.Marks.Techniques)
	}
	// And the mark endpoint has no name for it, which is the point.
	if _, ok := model.TechniqueFromName("dead-note"); ok {
		t.Error("there is a dead-note technique as well as the notehead")
	}
}

func TestAShapeNobodyHasIsRefused(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	got := h.postFails(fmt.Sprintf("/api/notehead?part=0&measure=%d&at=%d&shape=blob", m, at))
	if !strings.Contains(got, "unknown notehead") {
		t.Errorf("it said %q", got)
	}
	if firstNote(t, h, 0).Head != model.HeadNormal {
		t.Error("it changed the note anyway")
	}
}

// The shape is called shape and not head because head already means which
// notehead of a chord: the staff step the editor read off the page. One
// request carrying both words with two meanings is the bug this guards.
func TestTheShapeDoesNotCollideWithTheNoteheadOfAChord(t *testing.T) {
	h := newHarness(t)
	m, at := writeOneNote(t, h)

	// head is a staff step here and nothing else. Asking for a cross by
	// that name asks for no shape at all, and is refused rather than
	// quietly understood — which is what would happen if one word carried
	// both meanings.
	got := h.postFails(fmt.Sprintf("/api/notehead?part=0&measure=%d&at=%d&head=x", m, at))
	if !strings.Contains(got, "unknown notehead") {
		t.Errorf("it said %q", got)
	}
	if n := firstNote(t, h, 0).Head; n != model.HeadNormal {
		t.Errorf("it drew the note as %v anyway", n)
	}
}
