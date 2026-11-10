/*
 * concertkey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The key signature a staff is reported as being in, while the score is
 * being shown as it sounds.
 *
 * What is stored is what the player reads: a clarinet staff in a piece in F
 * major is stored as one sharp. In the concert-pitch view that staff is
 * drawn with one flat like every other, and the key picker opens on the
 * number this endpoint reports — so it opened on one sharp over a staff
 * reading one flat, and pressing OK without touching the slider moved the
 * whole piece from F major to A major.
 *
 * The rule everywhere else in the editor is that what is clicked is what is
 * seen. handleSetKey already reads the number back that way round; this is
 * the other half of it.
 */

package app

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// band is a flute, a clarinet in B flat and an alto saxophone, in a piece in
// F major — one flat as it sounds, so one sharp on the clarinet and two on
// the saxophone.
func band(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	part := func(name string, tr model.Transposition) *model.Part {
		return &model.Part{
			Name: name, Clef: model.Treble, Transpose: tr,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Key:      tr.Written(model.KeySignature{Fifths: -1}),
			Measures: []*model.Measure{model.NewMeasure()},
		}
	}
	h.app.score.Parts = []*model.Part{
		part("Flute", model.Transposition{}),
		part("Clarinet", model.Transposition{Diatonic: -1, Chromatic: -2}),
		part("Alto sax", model.Transposition{Diatonic: -5, Chromatic: -9}),
	}
	return h
}

// reported is the key each staff comes back as, from the endpoint the picker
// and the inspector both read.
func reported(h *harness) []string {
	h.t.Helper()
	var out struct {
		Parts []struct{ Key string } `json:"parts"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &out); err != nil {
		h.t.Fatal(err)
	}
	keys := make([]string, len(out.Parts))
	for i, p := range out.Parts {
		keys[i] = p.Key
	}
	return keys
}

// fifthsOf reads a reported signature back into the number the picker's
// slider holds — "1♯ major" is 1, "2♭ major" is -2, "♮ major" is 0 — the
// same way the editor's own keyFifthsOf does.
func fifthsOf(t *testing.T, text string) int {
	t.Helper()
	switch {
	case strings.HasPrefix(text, "♮"):
		return 0
	case strings.Contains(text, "♯"):
		n, _ := strconv.Atoi(strings.TrimSpace(strings.Split(text, "♯")[0]))
		return n
	case strings.Contains(text, "♭"):
		n, _ := strconv.Atoi(strings.TrimSpace(strings.Split(text, "♭")[0]))
		return -n
	}
	t.Fatalf("%q is not a signature", text)
	return 0
}

func TestTheKeyReportedIsTheOneTheStaffIsShowing(t *testing.T) {
	h := band(t)

	// Written: each player's own signature.
	want := []string{"1♭ major", "1♯ major", "2♯ major"}
	for i, got := range reported(h) {
		if got != want[i] {
			t.Errorf("staff %d reads %q, want %q", i, got, want[i])
		}
	}

	// As it sounds: one flat on every staff, which is what is drawn.
	h.post("/api/view?concert=1")
	for i, got := range reported(h) {
		if got != "1♭ major" {
			t.Errorf("in the concert-pitch view staff %d is reported as %q, "+
				"and the staff on screen is showing one flat", i, got)
		}
	}
}

// The consequence, which is the bug as it was met: the picker opens on what
// this reports, and setting that same value back is meant to change nothing.
func TestAcceptingTheKeyThePickerOpensOnChangesNothing(t *testing.T) {
	h := band(t)
	h.post("/api/view?concert=1")

	before := make([]model.KeySignature, len(h.app.score.Parts))
	for i, p := range h.app.score.Parts {
		before[i] = p.Key
	}
	// The clarinet: the staff a conductor is most likely to click, and the
	// one the fault showed on. Opened on whatever the endpoint reports for
	// it, which is what the picker does, and sent straight back.
	h.post("/api/setkey?part=1&fifths=" + strconv.Itoa(fifthsOf(t, reported(h)[1])))
	for i, p := range h.app.score.Parts {
		if p.Key != before[i] {
			t.Errorf("staff %d went from %v to %v when the key picker was "+
				"opened on the clarinet and accepted unchanged",
				i, before[i], p.Key)
		}
	}
}

// And the view still only shows: what is stored stays what each player reads.
func TestTheConcertViewDoesNotRewriteWhatIsStored(t *testing.T) {
	h := band(t)
	h.post("/api/view?concert=1")
	h.post("/api/view?concert=0")
	want := []int{-1, 1, 2}
	for i, p := range h.app.score.Parts {
		if p.Key.Fifths != want[i] {
			t.Errorf("staff %d is stored in %d fifths, want %d",
				i, p.Key.Fifths, want[i])
		}
	}
}
