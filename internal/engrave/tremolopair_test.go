/*
 * tremolopair_test.go
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

// The strokes between two whole notes shaken against each other stop short
// of the second note's accidental, and lean from the first towards the
// second. Bar 55 of Black and Tan Fantasy: a chord shaken against a D flat
// below it, with the strokes level and drawn through the flat.
func TestTheStrokesBetweenTwoWholeNotesLeanAndClearTheFlat(t *testing.T) {
	w := model.Duration{Num: 1, Den: 1}
	pt := func(s model.Step, o, alt int) model.Pitch { return model.Pitch{Step: s, Octave: o, Alter: alt} }
	c := &model.Chord{Dur: w, Notes: []*model.Note{
		{Pitch: pt(model.StepF, 4, 0), Dur: w}, {Pitch: pt(model.StepD, 5, 0), Dur: w}}}
	c.Tremolo = &model.Tremolo{Strokes: 3, Start: true}
	n := &model.Note{Pitch: pt(model.StepD, 4, -1), Dur: w}
	n.Tremolo = &model.Tremolo{Strokes: 3, Stop: true}
	sc := &model.Score{
		Tempo: model.Tempo{BPM: 60, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{Clef: model.Treble,
			Time: model.TimeSignature{Beats: 2, BeatValue: 1},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{c, n}}},
				RightBarline: model.BarFinal,
			}}}},
	}
	sc.SpellAccidentals()
	lay := Engrave(sc, DefaultOptions())
	flat := math.Inf(1)
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "accidentalFlat") {
			flat = math.Min(flat, g.X)
		}
	}
	if math.IsInf(flat, 1) {
		t.Fatal("no flat drawn")
	}
	if len(lay.Quads) != 3 {
		t.Fatalf("%d strokes, want 3", len(lay.Quads))
	}
	for _, q := range lay.Quads {
		right := math.Max(math.Max(q.X[0], q.X[1]), math.Max(q.X[2], q.X[3]))
		if right > flat {
			t.Errorf("a stroke reaches %.2f, into the flat at %.2f", right, flat)
		}
	}
	// The second note is lower, so the strokes go down to it: further down
	// the page at their right end than at their left.
	q := lay.Quads[0]
	left, right := 0.0, 0.0
	for i := range q.X {
		if q.X[i] == math.Min(math.Min(q.X[0], q.X[1]), math.Min(q.X[2], q.X[3])) {
			left = q.Y[i]
		} else {
			right = q.Y[i]
		}
	}
	if right <= left {
		t.Errorf("the strokes do not lean down to the lower note: %.2f at the left, %.2f at the right", left, right)
	}
}
