/*
 * tab_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing tablature.
 *
 * gscore could draw a tab before it could write one. What is typed is a
 * fret and what is stored is a pitch, because the pitch is what sounds and
 * what every other part of the program reads — so the thing to check is
 * that the two never disagree.
 */

package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func firstNote(t *testing.T, h *harness, part int) *model.Note {
	t.Helper()
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					return n
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return nil
}

// noteAt is where the first note of a staff sits: which bar and which
// place in it. A new score opens with a bar of rest already in it, so
// nothing written is at index zero.
func noteAt(t *testing.T, h *harness, part int) (measure, index int) {
	t.Helper()
	for mi, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for ei, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					return mi, ei
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return 0, 0
}

// notesOn counts what has actually been written on a staff, which is not
// the same as counting its elements: the bar of rest was there already.
func notesWritten(h *harness, part int) int {
	n := 0
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, ok := el.(*model.Note); ok {
					n++
				}
			}
		}
	}
	return n
}

func TestTheTuningsAreListed(t *testing.T) {
	h := newHarness(t)
	var out []struct {
		Name    string   `json:"name"`
		Strings []string `json:"strings"`
		Frets   int      `json:"frets"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/tunings")), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) < 8 {
		t.Errorf("only %d tunings", len(out))
	}
	if out[0].Name != "guitar" || len(out[0].Strings) != 6 {
		t.Errorf("the first tuning is %+v, want a six-string guitar", out[0])
	}
}

func TestAStaffCanBeWrittenAsTablature(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")

	p := h.app.score.Parts[0]
	if p.Tab == nil {
		t.Fatal("the staff is not a tab")
	}
	if p.Tab.Lines() != 6 {
		t.Errorf("it has %d strings", p.Tab.Lines())
	}
	// And back again. The notes keep their pitches either way: a tab is
	// how the music is shown and not what it is.
	h.post("/api/staff?part=0&tab=off")
	if h.app.score.Parts[0].Tab != nil {
		t.Error("the staff is still a tab")
	}
}

func TestAnUnknownTuningIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/staff?part=0&tab=sitar"); !strings.Contains(got, "tuning") {
		t.Errorf("it said %q", got)
	}
	if h.app.score.Parts[0].Tab != nil {
		t.Error("it made a tab anyway")
	}
}

// Turning a staff into a tab puts the notes already on it somewhere on the
// neck, at the first place that reaches them.
func TestExistingNotesGetAPlaceOnTheNeck(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter") // some pitch
	h.post("/api/staff?part=0&tab=guitar")

	n := firstNote(t, h, 0)
	if n.String < 1 || n.String > 6 {
		t.Errorf("the note is on string %d", n.String)
	}
	// And the fret it was given really is that pitch on that string.
	want, ok := h.app.score.Parts[0].Tab.Fretted(n.Pitch, n.String)
	if !ok || want != n.Fret {
		t.Errorf("%v on string %d is fret %d, but it says fret %d",
			n.Pitch, n.String, want, n.Fret)
	}
}

/*
 * Typing a fret.
 */

func TestAFretWritesTheNoteItStops(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=3&dur=quarter")

	n := firstNote(t, h, 0)
	if n.String != 6 || n.Fret != 3 {
		t.Errorf("it went to string %d fret %d", n.String, n.Fret)
	}
	// Third fret of the bottom E is G2.
	if got, want := n.Pitch.MIDI(), 43; got != want {
		t.Errorf("it sounds %v (%d), want G2 (%d)", n.Pitch, got, want)
	}
}

func TestAnOpenStringIsFretZero(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=1&fret=0&dur=quarter")

	if got, want := firstNote(t, h, 0).Pitch.MIDI(), 64; got != want {
		t.Errorf("the open top string sounds %d, want E4 (%d)", got, want)
	}
}

func TestAFretThatIsNotThereIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")

	for _, q := range []string{
		"part=0&measure=0&string=9&fret=0&dur=quarter",
		"part=0&measure=0&string=1&fret=99&dur=quarter",
		"part=0&measure=0&string=1&fret=-1&dur=quarter",
		"part=0&measure=0&fret=3&dur=quarter",
		"part=0&measure=0&string=1&dur=quarter",
	} {
		if got := h.postFails("/api/fret?" + q); strings.Contains(got, "meant to be refused") {
			t.Errorf("%q was allowed", q)
		}
	}
	if n := notesWritten(h, 0); n != 0 {
		t.Errorf("a refused fret wrote %d notes", n)
	}
}

// A fret means nothing on a staff of pitches, and saying so is better than
// guessing which string was meant.
func TestAFretOnAnOrdinaryStaffIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/fret?part=0&measure=0&string=1&fret=3&dur=quarter"); !strings.Contains(got, "tablature") {
		t.Errorf("it said %q", got)
	}
}

/*
 * Moving a note along the neck.
 *
 * The same pitch is in several places and they do not finger alike: the
 * open B string and the fourth fret of the G are the same note and a
 * different thing to play.
 */

func TestANoteCanBeMovedToAnotherString(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=2&fret=0&dur=quarter") // open B
	was := firstNote(t, h, 0).Pitch
	mi, at := noteAt(t, h, 0)

	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&string=3", mi, at))
	n := firstNote(t, h, 0)
	if n.String != 3 || n.Fret != 4 {
		t.Errorf("it went to string %d fret %d, want string 3 fret 4", n.String, n.Fret)
	}
	// And it is the same note. That is the whole point: the hand moves
	// and the music does not.
	if n.Pitch != was {
		t.Errorf("it now sounds %v, was %v", n.Pitch, was)
	}
}

func TestANoteCanBeSteppedAlongTheNeck(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=2&fret=0&dur=quarter")
	mi, at := noteAt(t, h, 0)

	// Down the page is towards the lower strings.
	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&move=down", mi, at))
	if got := firstNote(t, h, 0).String; got != 3 {
		t.Errorf("moving down went to string %d, want 3", got)
	}
	h.post(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&move=up", mi, at))
	if got := firstNote(t, h, 0).String; got != 2 {
		t.Errorf("moving back up went to string %d, want 2", got)
	}
}

// A string that cannot reach the note says so, and nothing moves.
func TestANoteWillNotMoveToAStringThatCannotReachIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=0&dur=quarter") // low E
	mi, at := noteAt(t, h, 0)

	got := h.postFails(fmt.Sprintf("/api/restring?part=0&measure=%d&at=%d&string=1", mi, at))
	if !strings.Contains(got, "cannot be played") {
		t.Errorf("it said %q", got)
	}
	if n := firstNote(t, h, 0); n.String != 6 || n.Fret != 0 {
		t.Errorf("it moved anyway, to string %d fret %d", n.String, n.Fret)
	}
}

func TestOnlyANoteIsPlayedOnAString(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/insert?part=0&measure=0&kind=rest&dur=quarter")

	if got := h.postFails("/api/restring?part=0&measure=0&at=0&string=2"); got == "" {
		t.Error("a rest was moved to a string")
	}
}

// Retuning a staff moves the frets under the notes, because the notes are
// the pitches and the frets are where those pitches now are.
func TestRetuningMovesTheFretsAndNotTheNotes(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&string=6&fret=3&dur=quarter") // G2
	was := firstNote(t, h, 0).Pitch

	h.post("/api/staff?part=0&tab=guitar%20%E2%80%94%20drop%20D")
	n := firstNote(t, h, 0)
	if n.Pitch != was {
		t.Errorf("retuning changed the note to %v, was %v", n.Pitch, was)
	}
	// The sixth string is now D2, so G2 is at the fifth fret.
	if n.String != 6 || n.Fret != 5 {
		t.Errorf("after dropping to D it reads string %d fret %d, want 6 and 5",
			n.String, n.Fret)
	}
}

// What a tab says about itself, for the dialog that sets it.
func TestAStaffSaysWhatItIsTunedTo(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=ukulele")

	got := tabOf(h.app.score.Parts[0])
	if got == nil {
		t.Fatal("it says nothing")
	}
	if got.Tuning != "ukulele" || len(got.Strings) != 4 || got.Frets != 15 {
		t.Errorf("it says %+v", got)
	}
	if tabOf(&model.Part{}) != nil {
		t.Error("a staff of pitches claims a tuning")
	}
}
