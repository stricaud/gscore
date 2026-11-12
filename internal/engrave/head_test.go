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

// clefXs is where each staff's clef stands, keyed by the clef's height,
// which is what says which staff a glyph belongs to.
func clefXs(lay *Layout) map[float64]float64 {
	out := map[float64]float64{}
	for _, g := range lay.Glyphs {
		if g.Name == "gClef" || g.Name == "fClef" {
			out[g.Y] = g.X
		}
	}
	return out
}

// staffClefX is the x of the clef nearest in height to a glyph: the clef of
// the staff it is drawn on.
func staffClefX(clefs map[float64]float64, y float64) float64 {
	best, at := math.Inf(1), 0.0
	for cy, x := range clefs {
		if d := math.Abs(cy - y); d < best {
			best, at = d, x
		}
	}
	return at
}

// headGlyphs is every glyph of a name.
func headGlyphs(lay *Layout, name string) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if g.Name == name {
			out = append(out, g)
		}
	}
	return out
}

func TestTheClefsStandInOneColumn(t *testing.T) {
	lay := Engrave(headScore(), DefaultOptions())
	clefs := clefXs(lay)
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
	// A 4/4 is two figures on each staff, four glyphs in all, and every
	// one of them stands at the same x.
	figures := headGlyphs(lay, "timeSig4")
	if len(figures) != 4 {
		t.Fatalf("%d meter figures drawn, want two per staff", len(figures))
	}
	for _, g := range figures[1:] {
		if math.Abs(g.X-figures[0].X) > 0.01 {
			t.Errorf("meter figures stand at x=%.2f and x=%.2f; they belong in one column",
				figures[0].X, g.X)
		}
	}
	// The sharps sit between the clef and the meter of the staff that has
	// them: after its clef, before the column.
	clefs := clefXs(lay)
	for _, sh := range headGlyphs(lay, "accidentalSharp") {
		if cx := staffClefX(clefs, sh.Y); sh.X <= cx {
			t.Errorf("a sharp at x=%.2f stands in front of its clef at x=%.2f", sh.X, cx)
		}
		if sh.X >= figures[0].X {
			t.Errorf("a sharp at x=%.2f stands past the meter at x=%.2f", sh.X, figures[0].X)
		}
	}
}
