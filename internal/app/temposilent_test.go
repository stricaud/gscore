/*
 * temposilent_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A speed that is played and not printed.
 *
 * Most published scores write only the word — "Slowly" — and leave the
 * number to the player. The editor still wants a number to play the piece
 * at, and the model already keeps the two apart (model.Tempo.Silent). This
 * is the dialog's way of asking for it: a speed, a text, and a tick that
 * keeps the metronome mark off the page. Asked for as "I want to stick to
 * the original version and only display Slowly".
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// tempoTexts is every piece of text the engraver writes for a score, run
// together, so that a test can ask what the page says.
func tempoTexts(h *harness) string {
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	var b strings.Builder
	for _, t := range lay.Texts {
		b.WriteString(t.S)
		b.WriteString("|")
	}
	return b.String()
}

func TestASilentTempoPrintsTheWordsAndNotTheNumber(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=whole&kind=note")

	h.post("/api/tempo?measure=0&bpm=60&unit=quarter&text=Slowly&silent=1")
	if !h.app.score.Tempo.Silent || h.app.score.Tempo.BPM != 60 {
		t.Fatalf("the tempo is %+v, want 60 to the quarter and silent", h.app.score.Tempo)
	}

	page := tempoTexts(h)
	if !strings.Contains(page, "Slowly|") {
		t.Errorf("the page does not say Slowly: %q", page)
	}
	if strings.Contains(page, "= 60") {
		t.Errorf("the page prints the number it was asked to keep quiet: %q", page)
	}

	// And the dialog can read it back, so that reopening it shows the
	// tick where it was left.
	info := h.get("/api/info")
	if !strings.Contains(info, `"tempoSilent":true`) {
		t.Errorf("the info does not report the tempo as silent")
	}
}

func TestATempoIsPrintedUnlessAskedToBeQuiet(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/insert?part=0&measure=0&at=-1&step=6&dur=whole&kind=note")
	h.post("/api/tempo?measure=0&bpm=60&unit=quarter&text=Slowly")
	if page := tempoTexts(h); !strings.Contains(page, "= 60") {
		t.Errorf("the number is missing from a tempo that did not ask to be quiet: %q", page)
	}
}

// A change of speed partway through takes the same tick, and says so to
// the dialog by the bar it is written at.
func TestASilentTempoChangeIsReportedByItsBar(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	for i := 0; i < 2; i++ {
		h.post("/api/insert?part=0&measure=" + string(rune('0'+i)) +
			"&at=-1&step=6&dur=whole&kind=note")
	}
	h.post("/api/tempo?measure=1&bpm=48&unit=quarter&text=Slower&silent=1")
	info := h.get("/api/info")
	if !strings.Contains(info, `"silent":true`) {
		t.Errorf("the change at bar 2 is not reported as silent: %s", info)
	}
	if page := tempoTexts(h); strings.Contains(page, "= 48") || !strings.Contains(page, "Slower|") {
		t.Errorf("bar 2 should say Slower and no number: %q", page)
	}
}
