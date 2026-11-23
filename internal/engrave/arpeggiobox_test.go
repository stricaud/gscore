/*
 * arpeggiobox_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A rolled chord tells the editor where its sign is — in front of the chord
// and as tall as it — so the sign can be clicked and taken off.
func TestARollTellsTheEditorWhereItsSignIs(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	c := &model.Chord{Dur: q}
	for _, s := range []model.Step{model.StepC, model.StepE, model.StepG} {
		c.Notes = append(c.Notes, &model.Note{Pitch: pitch(s, 4), Dur: q})
	}
	c.Arpeggio = &model.Arpeggio{}
	lay := Engrave(pianoWith([]model.Element{c, &model.Rest{Dur: model.Duration{Num: 3, Den: 4}}},
		[]model.Element{&model.Rest{Dur: model.Duration{Num: 1, Den: 1}}}), DefaultOptions())
	if len(lay.Arpeggios) != 1 {
		t.Fatalf("%d signs reported, want 1", len(lay.Arpeggios))
	}
	a := lay.Arpeggios[0]
	var chordX float64
	for _, h := range lay.Hits {
		for _, el := range h.Elements {
			if el.Ref == a.Ref {
				chordX = el.X
			}
		}
	}
	if chordX == 0 {
		t.Fatal("the sign does not name the chord it rolls")
	}
	if a.X+a.Width > chordX || a.Height < 2 {
		t.Errorf("the sign is at %.2f, %.2f wide and %.2f tall; the chord is at %.2f",
			a.X, a.Width, a.Height, chordX)
	}
}
