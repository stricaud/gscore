/*
 * head_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The head of a system: the clefs down the left, the meters in a column.
 *
 * Each staff's clef, key and meter used to end where everybody else's did,
 * so a staff with no sharps began its clef a key signature's width in from
 * the margin: beside a bassoon in three sharps the horn's clef floated. A
 * reader's eye goes down the clefs, and they are a column too — the room a
 * staff does not need for sharps belongs between its key and its meter.
 * Asked for in those words: "we need the key all the way to the left".
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// headScore is two staves, one in three sharps and one in none, each with a
// whole note in it.
func headScore() *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for _, c := range []struct {
		clef   model.Clef
		fifths int
	}{{model.Bass, 3}, {model.Treble, 0}} {
		p := &model.Part{
			Name: "Staff", Clef: c.clef,
			Key:  model.KeySignature{Fifths: c.fifths, Mode: model.Major},
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		p.Measures = []*model.Measure{{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepC, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarFinal,
		}}
		s.Parts = append(s.Parts, p)
	}
	return s
}

// glyphXs is the x of every glyph of a name, one per staff, by height.
func glyphXs(lay *Layout, names ...string) map[float64]float64 {
	out := map[float64]float64{}
	for _, g := range lay.Glyphs {
		for _, n := range names {
			if g.Name == n {
				out[math.Round(g.Y)] = g.X
			}
		}
	}
	return out
}

func TestTheClefsStandInOneColumn(t *testing.T) {
	lay := Engrave(headScore(), DefaultOptions())
	clefs := glyphXs(lay, "gClef", "fClef")
	if len(clefs) != 2 {
		t.Fatalf("%d clefs drawn, want one per staff: %v", len(clefs), clefs)
	}
	var xs []float64
	for _, x := range clefs {
		xs = append(xs, x)
	}
	if math.Abs(xs[0]-xs[1]) > 0.01 {
		t.Errorf("the clefs stand at x=%.2f and x=%.2f; they belong in one column", xs[0], xs[1])
	}
}

func TestTheMetersStandInOneColumn(t *testing.T) {
	// And the room the staff in C does not need for sharps is between its
	// clef and its meter, so that the music begins in one column too.
	lay := Engrave(headScore(), DefaultOptions())
	meters := glyphXs(lay, "timeSig4")
	if len(meters) != 2 {
		t.Fatalf("%d meters drawn, want one per staff: %v", len(meters), meters)
	}
	var xs []float64
	for _, x := range meters {
		xs = append(xs, x)
	}
	if math.Abs(xs[0]-xs[1]) > 0.01 {
		t.Errorf("the meters stand at x=%.2f and x=%.2f; they belong in one column", xs[0], xs[1])
	}
	// The sharps sit between the clef and the meter of the staff that has
	// them, and nothing sits in front of a clef.
	clefs := glyphXs(lay, "gClef", "fClef")
	for y, x := range glyphXs(lay, "accidentalSharp") {
		if cx, ok := clefs[y]; ok && x <= cx {
			t.Errorf("a sharp at x=%.2f stands in front of its clef at x=%.2f", x, cx)
		}
	}
}
