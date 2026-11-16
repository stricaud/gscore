/*
 * arpeggioaccidental_test.go
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

/*
 * An arpeggio stands in front of the accidentals, not through them.
 *
 * The wavy line was set at the noteheads' own distance from the column,
 * and a chord with a flat has its flat there: the line ran down the
 * middle of it. Reported from MuseScore's own arpeggio test score, where
 * the flat was hard to see.
 */

func TestAnArpeggioClearsTheAccidentalsInFrontOfItsChord(t *testing.T) {
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	c := &model.Chord{
		Dur: model.Duration{Num: 1, Den: 2},
		Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepE, Octave: 4}},
			{Pitch: model.Pitch{Step: model.StepG, Octave: 4}},
			{Pitch: model.Pitch{Step: model.StepB, Octave: 4, Alter: -1}, Accidental: model.Flat, Forced: true},
		},
	}
	c.Marks.Arpeggio = &model.Arpeggio{}
	m.Voices[0].Elements = append(m.Voices[0].Elements, c)
	lay := Engrave(s, DefaultOptions())

	var flatX, lineRight float64
	flat, line := false, false
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "accidentalFlat":
			flatX, flat = g.X, true
		case "arpeggiato":
			if r := g.X + glyphWidth(g); !line || r > lineRight {
				lineRight = r
			}
			line = true
		}
	}
	if !flat || !line {
		t.Fatalf("flat drawn: %v, arpeggio drawn: %v", flat, line)
	}
	if lineRight > flatX {
		t.Errorf("the arpeggio reaches to %.2f and the flat begins at %.2f: "+
			"the line is drawn through the accidental", lineRight, flatX)
	}
}
