/*
 * twonote_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The two-note tremolo: two notes with the beams drawn between their stems,
// meaning "alternate between these for as long as the two of them last".
//
// It is not the tremolo on one note and must not be drawn as one — slashes
// through both stems would say that each note is repeated on its own, which
// is a different thing to play.

// tremoloPair builds a bar of two notes marked as a two-note tremolo.
func tremoloPair(strokes int, value model.Duration, lo, hi model.Step) *model.Score {
	a := &model.Note{Pitch: model.Pitch{Step: lo, Octave: 4}, Dur: value}
	b := &model.Note{Pitch: model.Pitch{Step: hi, Octave: 4}, Dur: value}
	a.Tremolo = &model.Tremolo{Strokes: strokes, Start: true}
	b.Tremolo = &model.Tremolo{Strokes: strokes, Stop: true}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{a, b}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func TestTheBeamsAreDrawnBetweenTheTwoStems(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG), opt)

	// Three beams, and nothing drawn as slashes.
	if n := len(lay.Quads); n != 3 {
		t.Errorf("%d beams drawn, want 3", n)
	}
	for _, g := range lay.Glyphs {
		if has(g.Name, "tremolo") || has(g.Name, "buzzRoll") {
			t.Errorf("a %s was drawn: the slashes belong to the one-note "+
				"tremolo, not to a pair", g.Name)
		}
	}

	// The beams run from the first note to the second: they begin over the
	// first notehead and end over the second, rather than hanging off one
	// stem the way a flag does.
	var heads []float64
	for _, g := range lay.Glyphs {
		if has(g.Name, "noteheadBlack") || has(g.Name, "noteheadHalf") {
			heads = append(heads, g.X)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d noteheads, want 2", len(heads))
	}
	first, second := math.Min(heads[0], heads[1]), math.Max(heads[0], heads[1])
	for i, q := range lay.Quads {
		x0, x1 := q.X[0], q.X[0]
		for _, x := range q.X {
			x0, x1 = math.Min(x0, x), math.Max(x1, x)
		}
		if x0 < first || x0 > first+2 {
			t.Errorf("beam %d begins at %.2f, not over the first note at %.2f",
				i, x0, first)
		}
		if x1 < second || x1 > second+2 {
			t.Errorf("beam %d ends at %.2f, not over the second note at %.2f",
				i, x1, second)
		}
	}
	// And they are all the same length, being one stack of beams.
	for i := 1; i < len(lay.Quads); i++ {
		if math.Abs(lay.Quads[i].X[0]-lay.Quads[0].X[0]) > 0.01 {
			t.Errorf("beam %d starts somewhere else", i)
		}
	}
}

// However many strokes were asked for, that is how many beams are drawn.
func TestTheStrokesAreTheBeams(t *testing.T) {
	for _, strokes := range []int{1, 2, 3, 4} {
		opt := DefaultOptions()
		opt.Mode = Strip
		lay := Engrave(tremoloPair(strokes, model.Duration{Num: 1, Den: 2},
			model.StepC, model.StepG), opt)
		if n := len(lay.Quads); n != strokes {
			t.Errorf("%d strokes asked for, %d beams drawn", strokes, n)
		}
	}
}

// Whole notes have no stems, so the beams go between the noteheads.
func TestAPairOfWholeNotesIsBeamedBetweenTheHeads(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(2, model.Duration{Num: 1, Den: 1}, model.StepC, model.StepG), opt)
	if n := len(lay.Quads); n != 2 {
		t.Fatalf("%d beams drawn between two whole notes, want 2", n)
	}
	var heads []float64
	for _, g := range lay.Glyphs {
		if has(g.Name, "noteheadWhole") {
			heads = append(heads, g.X)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d whole noteheads", len(heads))
	}
	for _, q := range lay.Quads {
		for _, x := range q.X {
			if x < math.Min(heads[0], heads[1]) || x > math.Max(heads[0], heads[1])+2 {
				t.Errorf("a beam at %.2f is outside the two heads at %.2f and %.2f",
					x, heads[0], heads[1])
			}
		}
	}
}

// A start with nothing after it is half a mark and is not drawn. Guessing
// what it was meant to pair with is how a bar comes out with beams running
// into the next one.
func TestAHalfWrittenPairIsNotDrawn(t *testing.T) {
	s := tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG)
	marks := s.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	marks.Tremolo = nil // the partner never got its mark

	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)
	if n := len(lay.Quads); n != 0 {
		t.Errorf("%d beams drawn for a pair with only one end", n)
	}
}
