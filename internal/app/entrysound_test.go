//go:build !js

/*
 * entrysound_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Hearing a note as it is written.
 *
 * What sounds is not what was clicked. A staff position becomes a pitch
 * through the clef, an accidental or the key signature decides what it
 * sounds like, a transposing part's written note sounds somewhere else
 * again, and a drum staff's positions are not pitches at all. All of that
 * is settled here as the note is written, so the answer says what it
 * sounds like — and the editor does not keep a second, worse copy of the
 * same arithmetic in JavaScript.
 */

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// wrote is what the editor is told about the note it just wrote.
type wrote struct {
	MIDI   *int   `json:"midi"`
	Drum   int    `json:"drum"`
	Family string `json:"family"`
}

// writeNote posts one note and reads back what it sounds like.
func writeNote(h *harness, path string) wrote {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("POST %s answered %d: %s", path, rec.Code, rec.Body.String())
	}
	var got wrote
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		h.t.Fatalf("the answer was %q: %v", rec.Body.String(), err)
	}
	return got
}

// scored is a harness over a score somebody has set up: a transposing
// part, a drum staff, an instrument.
func scored(t *testing.T, set func(*model.Score)) *harness {
	t.Helper()
	sc := model.NewScore()
	set(sc)
	a := New(sc, "")
	mux := http.NewServeMux()
	a.routes(mux)
	return &harness{t: t, app: a, mux: mux}
}

func TestANoteSaysWhatItSoundsLike(t *testing.T) {
	h := newHarness(t)
	got := writeNote(h, "/api/insert?part=0&measure=0&at=0&pitch=c4&dur=quarter&kind=note")
	if got.MIDI == nil {
		t.Fatal("the note that was written did not say what it sounds like")
	}
	if *got.MIDI != 60 {
		t.Errorf("middle C was written and sounds at %d, want 60", *got.MIDI)
	}
	if got.Family == "" {
		t.Error("the note did not say how it is sounded, so the editor " +
			"cannot pick a voice for it")
	}
}

func TestAnAccidentalIsHeard(t *testing.T) {
	h := newHarness(t)
	got := writeNote(h,
		"/api/insert?part=0&measure=0&at=0&pitch=c4&accidental=sharp&dur=quarter&kind=note")
	if got.MIDI == nil || *got.MIDI != 61 {
		t.Errorf("a C sharp sounds at %v, want 61", got.MIDI)
	}
}

func TestARestSoundsLikeNothing(t *testing.T) {
	h := newHarness(t)
	got := writeNote(h, "/api/insert?part=0&measure=0&at=0&dur=quarter&kind=rest")
	if got.MIDI != nil {
		t.Errorf("a rest came back sounding at %d", *got.MIDI)
	}
	if got.Drum != 0 {
		t.Errorf("a rest came back as drum %d", got.Drum)
	}
}

func TestATransposingPartSoundsWhereItSounds(t *testing.T) {
	// A clarinet in B flat reads a C and sounds the B flat below it. The
	// note heard while typing has to be the one the instrument makes, or
	// the editor is teaching somebody the wrong pitch.
	h := scored(t, func(sc *model.Score) {
		sc.Parts[0].Transpose = model.Transposition{Diatonic: -1, Chromatic: -2}
	})
	got := writeNote(h, "/api/insert?part=0&measure=0&at=0&pitch=c4&dur=quarter&kind=note")
	if got.MIDI == nil || *got.MIDI != 58 {
		t.Errorf("a written middle C on a B flat clarinet sounds at %v, "+
			"want 58 — the B flat below it", got.MIDI)
	}
}

func TestADrumIsTheDrumAndNotThePitch(t *testing.T) {
	// A drum staff's lines name instruments. What sounds when one is
	// written is the drum, and the notehead's pitch means nothing.
	h := scored(t, func(sc *model.Score) { sc.Parts[0].Clef = model.Percussion })
	got := writeNote(h, "/api/insert?part=0&measure=0&at=0&step=0&dur=quarter&kind=note&drum=38")
	if got.Drum != 38 {
		t.Errorf("a snare was written and came back as drum %d, want 38",
			got.Drum)
	}
	if got.MIDI != nil {
		t.Errorf("a drum came back with a pitch of %d, which would be "+
			"played as one", *got.MIDI)
	}
	if got.Family != "percussion" {
		t.Errorf("a drum is sounded as %q, want percussion", got.Family)
	}
}

func TestANoteheadAddedToAChordIsHeard(t *testing.T) {
	h := newHarness(t)
	writeNote(h, "/api/insert?part=0&measure=0&at=0&pitch=c4&dur=quarter&kind=note")
	got := writeNote(h, "/api/addpitch?part=0&measure=0&index=0&pitch=e4")
	// The notehead that was added, not the one it was added to: what was
	// just done is this note.
	if got.MIDI == nil || *got.MIDI != 64 {
		t.Errorf("the E added to a C sounds at %v, want 64", got.MIDI)
	}
}

func TestTheStaffSaysHowItIsSounded(t *testing.T) {
	// The editor picks a voice by family, and gets it from the staff
	// rather than from a note: a staff with no music on it yet still has
	// to sound like a flute the first time something is written on it.
	h := scored(t, func(sc *model.Score) {
		sc.Parts[0].Instrument = model.Instrument(73) // flute
	})

	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/info", nil))
	var got struct {
		Parts []struct {
			Family string `json:"family"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Parts) == 0 {
		t.Fatal("no staves came back")
	}
	if got.Parts[0].Family == "" {
		t.Error("the staff does not say how it is sounded")
	}
}

func TestSoundingNotesAsTheyAreWrittenCanBeTurnedOff(t *testing.T) {
	withSettings(t, func() {
		a := &App{}
		// On for somebody who has never thought about it: hearing what you
		// typed is how every instrument works.
		if !EntrySound() {
			t.Error("notes are not sounded as they are written by default")
		}
		rec := httptest.NewRecorder()
		a.handleSettings(rec, httptest.NewRequest(http.MethodPost,
			"/api/settings?entrySound=0", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("turning it off answered %d", rec.Code)
		}
		if EntrySound() {
			t.Error("it was turned off and is still on")
		}
		if !loadSettings().NoEntrySound {
			t.Error("the settings file does not hold the choice")
		}
		// And back on again.
		rec = httptest.NewRecorder()
		a.handleSettings(rec, httptest.NewRequest(http.MethodPost,
			"/api/settings?entrySound=1", nil))
		if !EntrySound() {
			t.Error("it was turned back on and is still off")
		}
	})
}
