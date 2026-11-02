/*
 * engraving_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A score engraved by its own measurements.
 *
 * The two tables the engraver works from were settled in Go and could not
 * be changed. They are what an engraver argues about, and the answers
 * belong to a score rather than to the program: a song and an orchestral
 * part want different ones, and the answer has to travel in the file.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// knobsNow reads the endpoint and files the answer by name.
func knobsNow(t *testing.T, h *harness) map[string]engravingKnob {
	t.Helper()
	var out struct {
		Knobs []engravingKnob `json:"knobs"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/engraving")), &out); err != nil {
		t.Fatalf("the endpoint did not answer with a list: %v", err)
	}
	by := map[string]engravingKnob{}
	for _, k := range out.Knobs {
		by[k.Name] = k
	}
	return by
}

func TestAScoreStartsOnTheHouseStyle(t *testing.T) {
	h := newHarness(t)
	by := knobsNow(t, h)
	if len(by) != len(engrave.Knobs) {
		t.Fatalf("%d measurements came back, the engraver has %d",
			len(by), len(engrave.Knobs))
	}
	for _, k := range engrave.Knobs {
		got, ok := by[k.Name]
		if !ok {
			t.Errorf("%s is missing from the list", k.Name)
			continue
		}
		if got.Set {
			t.Errorf("%s is marked as chosen on a score that has chosen nothing", k.Name)
		}
		if got.Value != got.Default {
			t.Errorf("%s reads %v, the house style is %v", k.Name, got.Value, got.Default)
		}
		if got.Label == "" || got.Group == "" {
			t.Errorf("%s has nothing to show in a dialog: %+v", k.Name, got)
		}
		if got.Min >= got.Max {
			t.Errorf("%s allows nothing: %v..%v", k.Name, got.Min, got.Max)
		}
	}
}

// The whole point: a measurement set has to reach the page.
func TestASetMeasurementChangesTheEngraving(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	before := engrave.Engrave(h.app.score, engrave.DefaultOptions())

	h.post("/api/engraving?noteBase=5.5")
	after := engrave.Engrave(h.app.score, engrave.DefaultOptions())

	if after.Width <= before.Width {
		t.Errorf("a bar %.2f wide at a note width of 2.8 is %.2f wide at 5.5",
			before.Width, after.Width)
	}
	by := knobsNow(t, h)
	if !by["noteBase"].Set || by["noteBase"].Value != 5.5 {
		t.Errorf("the endpoint does not say it was set: %+v", by["noteBase"])
	}
	// And nothing else moved with it.
	if by["beamThickness"].Set {
		t.Error("setting one measurement marked another as chosen")
	}
}

// Engrave applies them itself, so that the dozen places that build Options
// cannot each forget. A score that printed differently from the way it
// looks is the one thing a page of music must not do.
func TestEveryWayOfEngravingUsesTheScoresMeasurements(t *testing.T) {
	// Two staves, because the gap between them is what this sets. The
	// same music both times, so the only difference is the setting.
	build := func(t *testing.T, gap bool) *harness {
		t.Helper()
		h := newHarness(t)
		h.post("/api/addstaff?instrument=flute&clef=treble")
		for part := 0; part < 2; part++ {
			for i := 0; i < 4; i++ {
				h.note(part, 0, 2+i, "quarter")
			}
		}
		if gap {
			h.post("/api/engraving?staffGap=16")
		}
		return h
	}
	plainScore := build(t, false).app.score
	wideScore := build(t, true).app.score

	for _, mode := range []struct {
		name string
		opt  engrave.Options
	}{
		{"the editor's strip", func() engrave.Options {
			o := engrave.DefaultOptions()
			o.Mode = engrave.Strip
			return o
		}()},
		{"a printed page", engrave.DefaultOptions()},
	} {
		plain := engrave.Engrave(plainScore, mode.opt)
		wide := engrave.Engrave(wideScore, mode.opt)
		if wide.Height <= plain.Height {
			t.Errorf("%s: %.2f tall with the staves 16 apart and %.2f with the "+
				"house style — the setting did not reach it",
				mode.name, wide.Height, plain.Height)
		}
	}
}

func TestAMeasurementOutOfBoundsIsRefusedWhole(t *testing.T) {
	h := newHarness(t)
	h.post("/api/engraving?beamThickness=0.55")

	// One good and one impossible, together. Nothing may be applied: half
	// of a request is the worst answer, because the page moves and the
	// refusal says nothing about which half took.
	//
	// Impossible means impossible, not merely unusual: a beam thicker than
	// anybody would print is somebody's own business and is accepted, and
	// what is refused here is a measurement nothing could be drawn at.
	if why := h.postFails("/api/engraving?noteBase=3.4&beamThickness=100000"); why == "" {
		t.Fatal("a beam of a hundred thousand spaces was accepted")
	}
	by := knobsNow(t, h)
	if by["noteBase"].Set {
		t.Error("the good half of a refused request was applied anyway")
	}
	if by["beamThickness"].Value != 0.55 {
		t.Errorf("the beam was left at %v, not the 0.55 it was before",
			by["beamThickness"].Value)
	}

	if why := h.postFails("/api/engraving?noSuchThing=1"); why == "" {
		t.Error("a measurement nobody has heard of was accepted")
	}
	if why := h.postFails("/api/engraving?noteBase=wide"); why == "" {
		t.Error("a word was accepted as a measurement")
	}
}

// A measurement outside what is usual is somebody's house style, and the
// window has no business overruling it.
//
// The ranges in the table used to be a wall. Somebody setting the space
// after a clef found they could not go past three, which is not a limit of
// the engraver — it is a guess at what they would want, enforced.
func TestAnUnusualMeasurementIsAccepted(t *testing.T) {
	h := newHarness(t)
	after, _ := engrave.KnobNamed("clefAfter")
	beyond := after.Max + 4

	h.post("/api/engraving?clefAfter=" + trimFloat(beyond))
	by := knobsNow(t, h)
	if got := by["clefAfter"].Value; got != beyond {
		t.Errorf("a clef space of %v came out as %v", beyond, got)
	}
	if !by["clefAfter"].Set {
		t.Error("it was not recorded as the score's own")
	}
	// The window is told both: what it may be, and what is usual.
	if by["clefAfter"].Ceiling <= after.Max {
		t.Errorf("the field is held to %v, which is no wider than the usual "+
			"%v", by["clefAfter"].Ceiling, after.Max)
	}
	if by["clefAfter"].Floor < 0 {
		t.Error("the field would take a negative space")
	}

	// And it really reaches the page.
	o := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	_ = o
	if v := engrave.Settings(h.app.score)["clefAfter"]; v != beyond {
		t.Errorf("the score records %v", v)
	}
}

func TestAMeasurementCanBePutBack(t *testing.T) {
	h := newHarness(t)
	h.post("/api/engraving?noteBase=5&beamThickness=0.6")

	// An empty value is one of them put back, which is how a single field
	// is cleared without resetting the rest.
	h.post("/api/engraving?noteBase=")
	by := knobsNow(t, h)
	if by["noteBase"].Set {
		t.Error("the note width was still marked as chosen")
	}
	if !by["beamThickness"].Set {
		t.Error("clearing one measurement cleared another")
	}

	h.post("/api/engraving?reset")
	by = knobsNow(t, h)
	for name, k := range by {
		if k.Set {
			t.Errorf("%s survived a reset", name)
		}
	}
	if h.app.score.Engraving != nil {
		t.Errorf("the score still carries %v", h.app.score.Engraving)
	}
}

// A name this version does not know is skipped rather than refused, so a
// file written by a later gscore still opens.
func TestAMeasurementFromALaterVersionIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.app.score.Engraving = map[string]float64{
		"noteBase":           4.0,
		"somethingFromLater": 12,
	}
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	if lay == nil || lay.Width <= 0 {
		t.Fatal("a score carrying a measurement this version does not know did not engrave")
	}
	by := knobsNow(t, h)
	if by["noteBase"].Value != 4.0 {
		t.Errorf("the one it does know came back as %v", by["noteBase"].Value)
	}
}
