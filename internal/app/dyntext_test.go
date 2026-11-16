/*
 * dyntext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A dynamic written by hand.
 *
 * "mp-f" on the first note of a swell is a marking every score has and no
 * menu lists. It is written as it reads, set in the dynamics font, and
 * because a player cannot read it the person writing it says how loud it is
 * played. Reported as "it could be a custom text with the same font but ask
 * the user if it should be played mp or f".
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func writtenDynamic(t *testing.T, text, plays string) *harness {
	t.Helper()
	h := newHarness(t)
	for i := 0; i < 2; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=half&kind=note")
	}
	h.post("/api/dynamic?dynamic=" + plays + "&text=" + text + "&sel=0:0:0")
	return h
}

func TestAWrittenDynamicPlaysAsSaid(t *testing.T) {
	h := writtenDynamic(t, "mp-f", "f")
	v := velocities(h)
	if len(v) != 2 || v[0] != model.Forte.Velocity() {
		t.Errorf("mp-f played as f came out at %v, wanted %d", v, model.Forte.Velocity())
	}
	m := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if m.DynamicText != "mp-f" || m.Dynamic != model.Forte {
		t.Errorf("written as %q playing %s, wanted mp-f playing f", m.DynamicText, m.Dynamic)
	}
}

// The marking is drawn as written, and the editor is told how it sounds so
// that clicking it can say so.
func TestAWrittenDynamicIsDrawnAsWritten(t *testing.T) {
	h := writtenDynamic(t, "mp-f", "f")
	dyns := h.layoutField("dynamics")
	if len(dyns) != 1 {
		t.Fatalf("%d dynamics drawn, wanted 1", len(dyns))
	}
	if dyns[0]["name"] != "mp-f" || dyns[0]["plays"] != "f" {
		t.Errorf("drawn as %v, wanted mp-f playing f", dyns[0])
	}
}

// Writing without saying how loud is refused: a marking nothing can read is
// a marking nothing can play.
func TestAWrittenDynamicNeedsALevel(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=half&kind=note")
	if msg := h.postFails("/api/dynamic?text=mp-f&sel=0:0:0"); msg == "" {
		t.Error("mp-f with no level to play at was accepted")
	}
}

// Taking the dynamic away takes the words with it, and a standard marking
// written out by hand is the standard marking.
func TestAWrittenDynamicGoesWithTheDynamic(t *testing.T) {
	h := writtenDynamic(t, "mp-f", "f")
	h.post("/api/dynamic?dynamic=none&sel=0:0:0")
	m := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if m.DynamicText != "" || m.Dynamic != model.DynamicNone {
		t.Errorf("after removal: %q playing %s", m.DynamicText, m.Dynamic)
	}
	h.post("/api/dynamic?dynamic=mp&text=mp&sel=0:0:0")
	if m.DynamicText != "" || m.Dynamic != model.MP {
		t.Errorf("mp written by hand came out as %q playing %s", m.DynamicText, m.Dynamic)
	}
}
