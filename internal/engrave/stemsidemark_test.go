/*
 * stemsidemark_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// An accent asked for over two stem-up eighths beamed together goes over the
// beam, not in among the stems under it.
func TestAMarkOnTheStemSideClearsTheBeam(t *testing.T) {
	d := func(n, den int) model.Duration { return model.Duration{Num: n, Den: den} }
	a := &model.Note{Pitch: pitch(model.StepE, 4), Dur: d(1, 8)}
	a.AddArticulation(model.Accent)
	a.PlaceArticulation(model.Accent, model.PlacementAbove)
	b := &model.Note{Pitch: pitch(model.StepF, 4), Dur: d(1, 8)}
	sc := &model.Score{
		Tempo: model.Tempo{BPM: 60, Unit: d(1, 4)},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 1, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{a, b}}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
	lay := Engrave(sc, DefaultOptions())
	beamTop := math.Inf(1)
	for _, q := range lay.Quads {
		for _, y := range q.Y {
			beamTop = math.Min(beamTop, y)
		}
	}
	if math.IsInf(beamTop, 1) {
		t.Fatal("no beam drawn")
	}
	found := false
	for _, g := range lay.Glyphs {
		if !strings.HasPrefix(g.Name, "articAccent") {
			continue
		}
		found = true
		if _, down := glyphHeight(g); g.Y+down > beamTop {
			t.Errorf("accent reaches down to %.2f, into the beam at %.2f", g.Y+down, beamTop)
		}
	}
	if !found {
		t.Fatal("no accent drawn")
	}
}
