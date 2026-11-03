/*
 * retune_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Moving a tablature part.
 *
 * Two edits wear the same words. "Put the guitar up a tone" can mean the
 * notes change and the hand goes somewhere else to play them, or it can
 * mean the hand stays and the instrument is tuned somewhere else. What is
 * checked here is that each one does its own thing and not the other's —
 * because both of them move what is heard, so the sound alone cannot tell
 * them apart and a test that only listened would pass either way.
 */

package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// onlyNote is the first note of a staff, with where it is played.
func onlyNote(t *testing.T, h *harness, part int) (midi, str, fret int) {
	t.Helper()
	n := firstNote(t, h, part)
	p := h.app.score.Parts[part]
	return p.Transpose.Apply(n.Pitch).MIDI(), n.String, n.Fret
}

// aGuitarWithANote is a tab staff with one note on it: the third fret of
// the bottom string, which is a G.
func aGuitarWithANote(t *testing.T, h *harness) {
	t.Helper()
	h.post("/api/staff?part=0&tab=guitar")
	h.post("/api/fret?part=0&measure=0&at=0&string=6&fret=3&dur=quarter")
}

// Retuning keeps the fingering and lets what sounds follow it. This is drop
// tuning and it is what a capo does upwards: the shapes are played exactly
// as written.
func TestRetuningKeepsTheFingeringAndMovesWhatSounds(t *testing.T) {
	h := newHarness(t)
	aGuitarWithANote(t, h)
	was, str, fret := onlyNote(t, h, 0)

	h.post("/api/retune?part=0&semitones=-2")

	now, str2, fret2 := onlyNote(t, h, 0)
	if str2 != str || fret2 != fret {
		t.Errorf("the hand moved: string %d fret %d became string %d fret %d",
			str, fret, str2, fret2)
	}
	if now != was-2 {
		t.Errorf("it sounds %d, want %d — two semitones below %d", now, was-2, was)
	}
}

// And the instrument really is tuned somewhere else afterwards, so the next
// note typed on it lands where the new tuning says.
func TestRetuningMovesEveryString(t *testing.T) {
	h := newHarness(t)
	aGuitarWithANote(t, h)
	before := append([]model.Pitch(nil), h.app.score.Parts[0].Tab.Strings...)

	h.post("/api/retune?part=0&semitones=-2")

	after := h.app.score.Parts[0].Tab.Strings
	if len(after) != len(before) {
		t.Fatalf("it came back with %d strings, want %d", len(after), len(before))
	}
	for i := range before {
		if got, want := after[i].MIDI(), before[i].MIDI()-2; got != want {
			t.Errorf("string %d is %d, want %d", i+1, got, want)
		}
	}
}

// Going down gives flats and going up gives sharps, because that is how a
// retuning is spoken: a guitar is taken down to E flat, never up to D
// sharp.
func TestARetunedStringIsSpelledTheWayItIsSpoken(t *testing.T) {
	// A guitar taken down a semitone: the top string is an E flat.
	down := newHarness(t)
	aGuitarWithANote(t, down)
	down.post("/api/retune?part=0&semitones=-1")
	if s := down.app.score.Parts[0].Tab.Strings[0]; s.Alter != -1 || s.Step != model.StepE {
		t.Errorf("down a semitone the top string is %v, want E flat", s)
	}

	// And up a tone it is an F sharp, not a G flat.
	up := newHarness(t)
	aGuitarWithANote(t, up)
	up.post("/api/retune?part=0&semitones=2")
	if s := up.app.score.Parts[0].Tab.Strings[0]; s.Alter != 1 || s.Step != model.StepF {
		t.Errorf("up a tone the top string is %v, want F sharp", s)
	}
}

// Either half of a pair will do. A guitar written both ways is one
// instrument on two staves and the cursor is usually on the half with the
// notes, so asking it of that half has to work.
func TestEitherHalfOfAPairCanBeRetuned(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")
	notation := len(h.app.score.Parts) - 2
	tab := len(h.app.score.Parts) - 1
	h.post(fmt.Sprintf(
		"/api/fret?part=%d&measure=0&at=0&string=6&fret=3&dur=quarter", tab))

	was, str, fret := onlyNote(t, h, notation)
	h.post(fmt.Sprintf("/api/retune?part=%d&semitones=-2", notation))

	now, str2, fret2 := onlyNote(t, h, notation)
	if str2 != str || fret2 != fret {
		t.Errorf("the hand moved: %d/%d became %d/%d", str, fret, str2, fret2)
	}
	if now != was-2 {
		t.Errorf("it sounds %d, want %d", now, was-2)
	}
	// The tuning belongs to the half that draws it.
	if h.app.score.Parts[tab].Tab == nil {
		t.Error("the tab half lost its tuning")
	}
}

func TestRetuningAStaffOfPitchesIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/retune?part=0&semitones=2"); !strings.Contains(got, "tablature") {
		t.Errorf("it said %q", got)
	}
}

func TestARetuningOfNothingIsRefused(t *testing.T) {
	h := newHarness(t)
	aGuitarWithANote(t, h)
	for _, q := range []string{
		"part=0&semitones=0",
		"part=0",
		"part=0&semitones=99",
		"part=0&semitones=-99",
	} {
		if got := h.postFails("/api/retune?" + q); got == "" {
			t.Errorf("%q was allowed", q)
		}
	}
	// And nothing moved.
	if _, str, fret := onlyNote(t, h, 0); str != 6 || fret != 3 {
		t.Errorf("a refused retuning left the note at string %d fret %d", str, fret)
	}
}

/*
 * The other edit: the music moves and the hand follows.
 *
 * A tablature used to be skipped by transposition altogether, which was
 * two faults in one. A tab on its own could not be transposed and said so;
 * a guitar written as a staff of pitches with a tab under it had its music
 * on the staff, which *was* transposed, while the tab was left out — so
 * the notes moved and the fret numbers under them did not, and nothing
 * reported it.
 */

func TestTransposingATabMovesTheFingering(t *testing.T) {
	h := newHarness(t)
	aGuitarWithANote(t, h)
	was, _, fret := onlyNote(t, h, 0)

	h.post("/api/transpose?steps=1&semitones=2")

	now, str, fret2 := onlyNote(t, h, 0)
	if now != was+2 {
		t.Errorf("it sounds %d, want %d", now, was+2)
	}
	if fret2 != fret+2 {
		t.Errorf("it is at fret %d, want %d — two semitones further up the "+
			"same string", fret2, fret+2)
	}
	if str != 6 {
		t.Errorf("it moved to string %d; the same string reaches it", str)
	}
}

// The pair is where this went wrong silently, so it gets its own test.
func TestTransposingAPairMovesTheTabUnderIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=pluck.guitar&written=both")
	notation := len(h.app.score.Parts) - 2
	tab := len(h.app.score.Parts) - 1
	h.post(fmt.Sprintf(
		"/api/fret?part=%d&measure=0&at=0&string=6&fret=3&dur=quarter", tab))

	was, _, fret := onlyNote(t, h, notation)
	h.post("/api/transpose?steps=1&semitones=2")

	now, _, fret2 := onlyNote(t, h, notation)
	if now != was+2 {
		t.Errorf("the notes did not move: %d became %d", was, now)
	}
	if fret2 != fret+2 {
		t.Errorf("the notes moved and the fingering did not: still fret %d", fret2)
	}
}

// A note the new key puts out of reach of the string it was on moves to a
// string that can reach it, rather than keeping a fret that does not exist.
func TestTransposingPastTheEndOfAStringMovesTheNote(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&tab=guitar")
	// The top string, open. There is nothing above it to go up to, so the
	// note has to stay on it and climb.
	h.post("/api/fret?part=0&measure=0&at=0&string=1&fret=0&dur=quarter")

	h.post("/api/transpose?steps=-1&semitones=-2")

	// Down a tone from the open top E is a D, which that string cannot
	// reach — it is below the open string. It belongs on the string below.
	_, str, fret := onlyNote(t, h, 0)
	if str == 1 {
		t.Errorf("it stayed on the top string at fret %d, which is below the nut", fret)
	}
	if fret < 0 {
		t.Errorf("it is at fret %d", fret)
	}
}

// A drum staff is still left alone: its lines name instruments, so moving
// a snare up a tone moves it to a tom.
func TestADrumStaffIsStillNotTransposed(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=drum.group.set")
	kit := len(h.app.score.Parts) - 1
	if transposable(h.app.score.Parts[kit]) {
		t.Error("the kit would be transposed")
	}
}
