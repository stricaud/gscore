/*
 * catalogue_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Choosing an instrument.
 *
 * The list is clustered by what the thing is, and what comes back from a
 * choice is the instrument itself — not the General MIDI program that is
 * only how it is made audible. A bass clarinet staff has to know it is a
 * bass clarinet: that is what decides its range, its transposition and what
 * is written into the file.
 */

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

type catalogueReply struct {
	Groups      []string `json:"groups"`
	Instruments []struct {
		Group      string `json:"group"`
		Name       string `json:"name"`
		Sound      string `json:"sound"`
		Program    int    `json:"program"`
		Percussion bool   `json:"percussion"`
		Low        int    `json:"low"`
		High       int    `json:"high"`
	} `json:"instruments"`
}

func catalogueOf(t *testing.T, h *harness) catalogueReply {
	t.Helper()
	var out catalogueReply
	if err := json.Unmarshal([]byte(h.get("/api/instruments")), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheInstrumentListIsClustered(t *testing.T) {
	h := newHarness(t)
	list := catalogueOf(t, h)

	if len(list.Groups) < 8 {
		t.Errorf("only %d groups: %v", len(list.Groups), list.Groups)
	}
	// Every instrument is in one of the declared groups, so the list can
	// be drawn under headings with nothing left over.
	known := map[string]bool{}
	for _, g := range list.Groups {
		known[g] = true
	}
	for _, in := range list.Instruments {
		if !known[in.Group] {
			t.Errorf("%s is in group %q, which is not one of the groups", in.Name, in.Group)
		}
	}

	// And the ones that were missing before are there: General MIDI has
	// none of these, which is why the old list could not offer them.
	for _, want := range []string{
		"bass clarinet", "contrabassoon", "cornet", "euphonium",
		"bass trombone", "mandolin", "ukulele", "bodhrán",
	} {
		found := false
		for _, in := range list.Instruments {
			if in.Name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the list has no %s", want)
		}
	}
}

// A staff made from the catalogue keeps the sound, which is the thing that
// says what it is. Keeping only the program loses it: General MIDI has one
// bassoon, so a contrabassoon that kept only its program would come back as
// a bassoon and be measured against a bassoon's range.
func TestAStaffKeepsTheInstrumentItWasGiven(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=wind.reed.contrabassoon")

	p := h.app.score.Parts[1]
	if p.Sound != "wind.reed.contrabassoon" {
		t.Errorf("the staff's sound is %q", p.Sound)
	}
	if p.Name != "Contrabassoon" {
		t.Errorf("the staff is called %q", p.Name)
	}
	// General MIDI's nearest, which is a bassoon, and that is correct:
	// it is what plays it.
	if p.Instrument != model.Bassoon {
		t.Errorf("the staff plays as %v", p.Instrument)
	}
	// It reads an octave above where it sounds.
	if p.Transpose.OctaveChange != -1 {
		t.Errorf("the contrabassoon transposes %+v", p.Transpose)
	}
	// And it answers about its own range rather than a bassoon's. A
	// bassoon stops at B flat below the bass staff; this goes lower.
	r, ok := p.Reach()
	if !ok {
		t.Fatal("the staff knows no range")
	}
	if r.Low > 24 {
		t.Errorf("its bottom note is %d, which is a bassoon's, not a contrabassoon's", r.Low)
	}
}

// The clarinets are the case the sound exists for: both are program 71, and
// a program cannot tell them apart.
func TestClarinetsInAAndBFlatTransposeDifferently(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=wind.reed.clarinet.bflat")
	h.post("/api/addstaff?sound=wind.reed.clarinet.a")

	bflat := h.app.score.Parts[1]
	inA := h.app.score.Parts[2]
	if bflat.Instrument != inA.Instrument {
		t.Fatal("the two clarinets should share a program")
	}
	if bflat.Transpose.Chromatic != -2 {
		t.Errorf("the B flat clarinet transposes %+v", bflat.Transpose)
	}
	if inA.Transpose.Chromatic != -3 {
		t.Errorf("the clarinet in A transposes %+v", inA.Transpose)
	}
}

// The clef follows the instrument, because nobody wants to tell a program
// that a cello reads the bass clef.
func TestTheClefFollowsTheInstrument(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct {
		sound string
		want  model.Clef
	}{
		{"strings.cello", model.Bass},
		{"strings.viola", model.Alto},
		{"wind.flutes.flute", model.Treble},
		{"brass.tuba", model.Bass},
		{"drum.snare-drum", model.Percussion},
	} {
		h.post("/api/addstaff?sound=" + tc.sound)
		p := h.app.score.Parts[len(h.app.score.Parts)-1]
		if p.Clef != tc.want {
			t.Errorf("%s is written in %v, want %v", tc.sound, p.Clef, tc.want)
		}
	}
}

// An unpitched instrument gives a staff whose lines name instruments rather
// than pitches, and no key.
func TestAnUnpitchedInstrumentGivesADrumStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?sound=drum.group.set")

	p := h.app.score.Parts[1]
	if p.Clef != model.Percussion {
		t.Errorf("the kit is written in %v", p.Clef)
	}
	if p.Key.Fifths != 0 {
		t.Errorf("the kit is in %d sharps", p.Key.Fifths)
	}
}

// An instrument nobody has heard of is a mistake worth hearing about, not a
// staff that quietly came out as a piano.
func TestAnUnknownInstrumentIsRefused(t *testing.T) {
	h := newHarness(t)
	before := len(h.app.score.Parts)
	if got := h.postFails("/api/addstaff?sound=wind.reed.sousaphone-whistle"); !strings.Contains(got, "unknown instrument") {
		t.Errorf("adding a staff for nothing said %q", got)
	}
	if len(h.app.score.Parts) != before {
		t.Error("it added the staff anyway")
	}
}

// Changing an existing staff's instrument does the same, and moves the
// music so that what is heard does not change.
func TestChangingAStaffToAnotherInstrument(t *testing.T) {
	h := newHarness(t)
	h.post("/api/staff?part=0&sound=wind.reed.saxophone.alto")

	p := h.app.score.Parts[0]
	if p.Sound != "wind.reed.saxophone.alto" {
		t.Errorf("the staff's sound is %q", p.Sound)
	}
	if p.Transpose.Chromatic != -9 {
		t.Errorf("the alto sax transposes %+v", p.Transpose)
	}
}
