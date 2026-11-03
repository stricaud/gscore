/*
 * midiin_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Playing the music in.
 *
 * A key pressed gives a number and nothing else. Two things have to happen
 * to it before it is a note on a page, and both of them are silent when
 * they go wrong: it is a SOUNDING pitch and the model keeps what is READ,
 * so a transposing part has to move it; and the same number is a D sharp
 * or an E flat depending on the key, so something has to spell it.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAPlayedNoteIsWrittenWhereTheCursorIs(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&midi=60&dur=quarter")

	n := firstNote(t, h, 0)
	if got := n.Pitch.String(); got != "C4" {
		t.Errorf("middle C came out as %s", got)
	}
}

// The key spells it. This is the whole reason the number alone is not
// enough: a part in three flats full of D sharps is a part nobody can read.
func TestTheKeySpellsWhatWasPlayed(t *testing.T) {
	for _, tc := range []struct {
		fifths int
		want   string
	}{
		{3, "D#4"},
		{-3, "Eb4"},
	} {
		h := newHarness(t)
		h.post("/api/setkey?fifths=" + itoa(tc.fifths))
		h.post("/api/insert?part=0&measure=0&at=0&midi=63&dur=quarter")

		if got := firstNote(t, h, 0).Pitch.String(); got != tc.want {
			t.Errorf("in %+d sharps, 63 came out as %s, want %s",
				tc.fifths, got, tc.want)
		}
	}
}

// A note the key already contains needs no sign written on it, and one it
// does not contain does.
func TestAPlayedNoteCarriesASignOnlyWhereOneIsNeeded(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setkey?fifths=2") // D major: F sharp and C sharp
	h.post("/api/insert?part=0&measure=0&at=0&midi=66&dur=quarter")
	if got := firstNote(t, h, 0).Accidental; got != model.NoAccidental {
		t.Errorf("the key's own F sharp was written with %v", got)
	}

	plain := newHarness(t)
	plain.post("/api/insert?part=0&measure=0&at=0&midi=66&dur=quarter")
	if got := firstNote(t, plain, 0).Accidental; got != model.Sharp {
		t.Errorf("an F sharp in C was written with %v, want a sharp", got)
	}
}

// What arrives is what sounds, and what is stored is what the player
// reads. A part that kept the number as it came would sound wrong for
// every transposing instrument there is.
func TestAPlayedNoteIsMovedToWhatThePlayerReads(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=brass.trumpet.bflat")
	part := len(h.app.score.Parts) - 1
	if h.app.score.Parts[part].Transpose.IsZero() {
		t.Skip("this build's B flat trumpet does not transpose")
	}

	h.post("/api/insert?part=" + itoa(part) + "&measure=0&at=0&midi=60&dur=quarter")

	n := firstNote(t, h, part)
	// It has to SOUND middle C, whatever it is written as.
	if got := h.app.score.Parts[part].Transpose.Apply(n.Pitch).MIDI(); got != 60 {
		t.Errorf("it is written %v, which sounds %d, want 60", n.Pitch, got)
	}
	// And it is not written as middle C, or nothing moved.
	if n.Pitch.MIDI() == 60 {
		t.Error("it was stored as it arrived; a B flat trumpet reads a tone up")
	}
}

// Keys pressed together are one chord: the second goes onto the note the
// first wrote rather than after it.
func TestKeysPressedTogetherMakeAChord(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&midi=60&dur=quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&midi=64")
	h.post("/api/addpitch?part=0&measure=0&index=0&midi=67")

	el := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0]
	c, ok := el.(*model.Chord)
	if !ok {
		t.Fatalf("the bar holds a %T, want a chord", el)
	}
	if len(c.Notes) != 3 {
		t.Fatalf("the chord has %d notes", len(c.Notes))
	}
	var got []string
	for _, n := range c.Notes {
		got = append(got, n.Pitch.String())
	}
	if strings.Join(got, " ") != "C4 E4 G4" {
		t.Errorf("the chord is %v, want C4 E4 G4", got)
	}
}

func TestANumberThatIsNoNoteIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{"midi=-1", "midi=128", "midi=sharp"} {
		got := h.postFails("/api/insert?part=0&measure=0&at=0&dur=quarter&" + q)
		if !strings.Contains(got, "0 to 127") {
			t.Errorf("%s said %q", q, got)
		}
	}
}
