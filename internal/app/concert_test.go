/*
 * concert_test.go
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
 * Showing the score as it sounds.
 *
 * What is stored is what each player reads, which is right and is also
 * unreadable as a score: a clarinet in B flat reads a C and a B flat comes
 * out, so three staves playing one line are written in three keys and look
 * like three lines.
 *
 * The two things that have to hold are that the picture moves and the file
 * does not, and that anything written while looking at the picture goes
 * into the file as what the player reads.
 */

// clarinetPart makes the one staff a B flat clarinet: it reads a tone
// above what it sounds.
func clarinetPart(h *harness, fifths int, pitches ...string) {
	p := h.app.score.Parts[0]
	p.Transpose = model.Transposition{Diatonic: -1, Chromatic: -2}
	tuneIn(h, fifths, pitches...)
	// tuneIn sets the key, so the transposition goes back on after it.
	p.Transpose = model.Transposition{Diatonic: -1, Chromatic: -2}
}

// shown is how the first bar reads in whatever way the score is being
// drawn: as written, or as it sounds.
func shown(a *App) []string {
	var out []string
	s := a.sounding(a.score)
	for _, el := range s.Parts[0].Measures[0].Voices[0].Elements {
		for _, n := range notesIn(el) {
			out = append(out, n.Pitch.String())
		}
	}
	return out
}

func TestTheScoreCanBeShownAsItSounds(t *testing.T) {
	h := newHarness(t)
	// A clarinet part written in D major, which sounds in C.
	clarinetPart(h, 2, "d4", "e4", "f#4")

	if got := shown(h.app); got[0] != "D4" || got[2] != "F#4" {
		t.Fatalf("as written it reads %v, want D4 E4 F#4", got)
	}

	h.post("/api/view?concert=1")
	got := shown(h.app)
	if len(got) != 3 || got[0] != "C4" || got[1] != "D4" || got[2] != "E4" {
		t.Errorf("as it sounds it reads %v, want C4 D4 E4", got)
	}

	// And the file is untouched: what is stored is still what the player
	// reads, whatever the screen is showing.
	if got := pitches(h); got != "D4 E4 F#4" {
		t.Errorf("the score itself now holds %q", got)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != 2 {
		t.Errorf("the part's own key is now %d sharps, want the two it reads", got)
	}
}

// The signature moves with the notes, or the picture is in the wrong key
// and every note of it carries an accidental.
func TestTheKeyMovesWithTheNotes(t *testing.T) {
	h := newHarness(t)
	clarinetPart(h, 2, "d4")
	h.post("/api/view?concert=1")

	if got := h.app.sounding(h.app.score).Parts[0].Key.Fifths; got != 0 {
		t.Errorf("the sounding key has %d sharps, want C major's none", got)
	}
}

// A staff that sounds as it is written is the same staff either way, and
// most staves are that.
func TestAStaffInCIsUntouched(t *testing.T) {
	h := newHarness(t)
	tuneIn(h, 0, "c4", "e4")
	h.post("/api/view?concert=1")

	if got := shown(h.app); got[0] != "C4" || got[1] != "E4" {
		t.Errorf("a staff in C came out as %v", got)
	}
	// Nothing was copied either: there was nothing to move.
	if h.app.sounding(h.app.score) != h.app.score {
		t.Error("a score with nothing to transpose was copied anyway")
	}
}

// What is clicked on a staff showing concert pitch is a sounding pitch,
// and what is stored is what the player reads.
func TestWritingWhileLookingAtConcertPitchGoesInAsWritten(t *testing.T) {
	h := newHarness(t)
	clarinetPart(h, 2, "d4")
	h.post("/api/view?concert=1")

	// Asking for the C that sounds: the clarinet reads a D.
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=c4&dur=quarter&kind=note")

	if got := pitches(h); got != "D4 D4" {
		t.Errorf("the file holds %q, want the written D4 for a sounding C4", got)
	}
	if got := shown(h.app); len(got) != 2 || got[1] != "C4" {
		t.Errorf("the screen shows %v, want the C that was asked for", got)
	}
}

// Turning it off puts the written score back, not a transposed one: the
// view never changed anything to be put back.
func TestTurningItOffShowsWhatIsWrittenAgain(t *testing.T) {
	h := newHarness(t)
	clarinetPart(h, 2, "d4", "e4")
	h.post("/api/view?concert=1")
	h.post("/api/view?concert=0")

	if got := shown(h.app); got[0] != "D4" || got[1] != "E4" {
		t.Errorf("back as written it reads %v, want D4 E4", got)
	}
	if out := h.post("/api/view?concert=0"); out["concert"] != false {
		t.Errorf("the switch answered %v", out["concert"])
	}
}

// The copy is made once and kept, or every scroll engraves the whole piece
// again from a fresh clone.
func TestTheSoundingCopyIsKeptUntilTheMusicChanges(t *testing.T) {
	h := newHarness(t)
	clarinetPart(h, 2, "d4")
	h.post("/api/view?concert=1")

	first := h.app.sounding(h.app.score)
	if second := h.app.sounding(h.app.score); first != second {
		t.Error("it made a second copy with nothing changed in between")
	}
	h.post("/api/insert?part=0&measure=0&at=-1&pitch=e4&dur=quarter&kind=note")
	if after := h.app.sounding(h.app.score); after == first {
		t.Error("the old copy was handed out after the music changed")
	}
}
