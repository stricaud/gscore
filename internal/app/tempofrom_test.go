/*
 * tempofrom_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tempo written as one beat equalling another.
//
// Where 4/4 turns into 12/8 the score says "♩ = ♩.": the pulse carries on
// and only the subdivision changes. There is no number to type, because the
// number is the one the old beat already had, counted in the new one.

func TestTheBeatBeforeCanEqualTheNewOne(t *testing.T) {
	h := newHarness(t)
	h.post("/api/tempo?bpm=96&unit=quarter")
	for i := 0; i < 8; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}

	// A dotted quarter from bar two, equal to the quarter before it.
	d := h.post("/api/tempo?measure=1&unit=quarter&dotted=1&from=before")
	if d["bpm"] != float64(96) {
		t.Errorf("the new beat goes at %v, want the 96 the old one had", d["bpm"])
	}

	got := h.app.score.TempoAt(1)
	if want := (model.Duration{Num: 1, Den: 4}); got.From != want {
		t.Errorf("the beat before is %s, want %s", got.From, want)
	}
	if want := (model.Duration{Num: 3, Den: 8}); got.Unit != want {
		t.Errorf("the new beat is %s, want %s", got.Unit, want)
	}
	if got.BPM != 96 {
		t.Errorf("the speed is %d, want 96", got.BPM)
	}

	// The editor is told, so the dialog opens showing the box ticked.
	var info struct {
		TempoChanges map[string]struct {
			From   string `json:"from"`
			Dotted bool   `json:"dotted"`
			Unit   string `json:"unit"`
		} `json:"tempoChanges"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatal(err)
	}
	c, ok := info.TempoChanges["1"]
	if !ok {
		t.Fatalf("bar 2 reports no change of speed: %v", info.TempoChanges)
	}
	if c.From != "quarter" || c.Unit != "quarter" || !c.Dotted {
		t.Errorf("bar 2 reports %+v, want a dotted quarter equal to a quarter", c)
	}
}

// A beat named outright, with the number worked out: an eighth equal to
// the quarter before it goes twice as fast.
func TestABeatNamedOutrightSetsTheSpeedFromTheOneBefore(t *testing.T) {
	h := newHarness(t)
	h.post("/api/tempo?bpm=60&unit=half")
	for i := 0; i < 8; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note")
	}
	d := h.post("/api/tempo?measure=1&unit=quarter&from=eighth")
	// A half at 60 is an eighth at 240, and a quarter equal to that eighth
	// goes at 240.
	if d["bpm"] != float64(240) {
		t.Errorf("the new beat goes at %v, want 240", d["bpm"])
	}
	got := h.app.score.TempoAt(1)
	if want := (model.Duration{Num: 1, Den: 8}); got.From != want {
		t.Errorf("the beat before is %s, want %s", got.From, want)
	}
}

// There is nothing before bar one to carry on from.
func TestNothingComesBeforeTheFirstBar(t *testing.T) {
	h := newHarness(t)
	if msg := h.postFails("/api/tempo?unit=quarter&dotted=1&from=before"); msg == "" {
		t.Error("it was taken")
	}
}

// The mark is said the way it is written.
func TestTheMarkIsSaidAsTwoBeats(t *testing.T) {
	got := tempoText(model.Tempo{
		BPM: 96, Unit: model.Duration{Num: 3, Den: 8},
		From: model.Duration{Num: 1, Den: 4},
	})
	if want := "1/4 = 3/8"; got != want {
		t.Errorf("said as %q, want %q", got, want)
	}
}
