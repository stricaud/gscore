/*
 * tempohidden_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tempo marking made invisible: picked up and hidden like a note, the
 * music still played at it. Asked for as "I might want to turn the tempo
 * invisible, I cannot."
 */

package app

import (
	"strings"
	"testing"
)

func TestATempoCanBeMadeInvisibleAndShownAgain(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=whole&kind=note")
	h.post("/api/tempo?measure=0&bpm=97&unit=quarter&text=Slowly")

	d := h.post("/api/tempo?measure=0&hidden=toggle")
	if d["hidden"] != true || !h.app.score.Tempo.Hidden {
		t.Fatalf("toggled once it answered %v and the tempo is %+v, want hidden", d, h.app.score.Tempo)
	}
	if page := tempoTexts(h); strings.Contains(page, "Slowly") || strings.Contains(page, "= 97") {
		t.Errorf("the page still prints the hidden tempo: %q", page)
	}
	if h.app.score.Tempo.BPM != 97 {
		t.Errorf("hiding it changed the speed to %d", h.app.score.Tempo.BPM)
	}

	h.post("/api/tempo?measure=0&hidden=toggle")
	if h.app.score.Tempo.Hidden {
		t.Error("toggled twice it is still hidden")
	}
	if page := tempoTexts(h); !strings.Contains(page, "Slowly") {
		t.Errorf("shown again, the page does not print it: %q", page)
	}
}
