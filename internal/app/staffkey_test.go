/*
 * staffkey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What key a staff is in when it joins a piece that is already in one.
 *
 * A key signature is not a property of a staff, it is what one player reads
 * for the key the music is in — and for an instrument in E flat that is a
 * different number of sharps from the one on the staff above it. Copying
 * the number straight across put a saxophone in the wrong key, which shows
 * up as an accidental on nearly every note it is given; and a drum staff
 * has no key at all, because its lines name instruments and a signature in
 * front of them says nothing anybody can read.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAStaffJoinsInTheKeyItsOwnPlayerReads(t *testing.T) {
	h := newHarness(t)
	// The piece is in C sharp minor: four sharps on a staff in C.
	h.post("/api/setkey?fifths=4")

	h.post("/api/addstaff?instrument=alto%20sax")
	h.post("/api/addstaff?instrument=clarinet")
	h.post("/api/addstaff?instrument=flute")
	h.post("/api/addstaff?instrument=drum%20kit")

	want := []struct {
		name   string
		fifths int
	}{
		{"the staff that was there", 4},
		{"alto sax", 7}, // a major sixth above: seven sharps
		{"clarinet", 6}, // a major second above: six
		{"flute", 4},    // in C, like the rest of us
		{"drum kit", 0}, // in no key at all
	}
	if len(h.app.score.Parts) != len(want) {
		t.Fatalf("%d staves, want %d", len(h.app.score.Parts), len(want))
	}
	for i, c := range want {
		if got := h.app.score.Parts[i].Key.Fifths; got != c.fifths {
			t.Errorf("%s reads %d sharps, want %d", c.name, got, c.fifths)
		}
	}
}

// And setting the key afterwards leaves the drums alone.
func TestSettingTheKeyLeavesADrumStaffOutOfIt(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=drum%20kit")
	h.post("/api/addstaff?instrument=alto%20sax")
	h.post("/api/setkey?fifths=-3")

	drums := h.app.score.Parts[1]
	if !drums.Unpitched() {
		t.Fatalf("the drum staff is not a drum staff: clef %v", drums.Clef)
	}
	if got := drums.Key.Fifths; got != 0 {
		t.Errorf("the drum staff was given %d flats", -got)
	}
	if got := h.app.score.Parts[2].Key.Fifths; got != 0 {
		// E flat major concert is C major for an alto saxophone.
		t.Errorf("the saxophone reads %d, want 0", got)
	}
	if got := h.app.score.Parts[0].Key.Fifths; got != -3 {
		t.Errorf("the first staff reads %d, want -3", got)
	}
}

// A key change partway through is the music changing key, so a staff that
// joins afterwards carries that change in its own reading of it too.
func TestAJoiningStaffCarriesAKeyChangeInItsOwnKey(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/addmeasure")
	h.post("/api/setkey?fifths=4")
	h.post("/api/setkey?fifths=-2&at=2")

	h.post("/api/addstaff?instrument=alto%20sax")
	sax := h.app.score.Parts[1]
	if sax.Key.Fifths != 7 {
		t.Fatalf("the saxophone starts in %d sharps, want 7", sax.Key.Fifths)
	}
	if len(sax.Measures) < 3 {
		t.Fatalf("the saxophone joined with %d bars", len(sax.Measures))
	}
	k := sax.Measures[2].Key
	if k == nil {
		t.Fatal("the key change did not reach the saxophone")
	}
	// B flat major concert is G major for an alto saxophone: one sharp.
	if k.Fifths != 1 {
		t.Errorf("it changes to %d, want 1 sharp", k.Fifths)
	}
}

// The inverse really is the inverse, for every key and every instrument
// gscore knows how to transpose.
//
// Inverse up to spelling, because the signatures run out: C sharp major is
// ten sharps for an alto saxophone and nobody writes ten, so it comes back
// as the two flats of B flat major, which is the same twelve notes. What
// has to hold is that the interval is right — that the answer is the key
// asked for, however it is spelled, and never a key a third away from it.
func TestSoundingUndoesWritten(t *testing.T) {
	for _, i := range []model.Instrument{
		model.AltoSax, model.TenorSax, model.BaritoneSax, model.Clarinet,
		model.Trumpet, model.FrenchHorn, model.EnglishHorn, model.Piccolo,
		model.Contrabass, model.Flute,
	} {
		tr := model.TranspositionFor(i)
		for f := -7; f <= 7; f++ {
			k := model.KeySignature{Fifths: f}
			got := tr.Sounding(tr.Written(k))
			if (got.Fifths-f)%12 != 0 {
				t.Errorf("%v: %d sharps came back as %d", i, f, got.Fifths)
			}
		}
	}
}
