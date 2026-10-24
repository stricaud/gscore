/*
 * chordheads_test.go
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

// Two noteheads a second apart cannot share a column: drawn there they are
// one through the other. One of each pair moves to the other side of the
// stem, and which one is not a free choice — the chord is read from the stem
// outwards, the note nearest it stays, and from there they alternate.
//
// The rule used to be "move me if anything is a second from me", which in a
// cluster of three moved the middle note and the top one both and left those
// two in the same column a second apart: the one thing the rule exists to
// prevent.

func chordOf(steps ...model.Step) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	c := &model.Chord{Dur: q}
	for i, st := range steps {
		c.Notes = append(c.Notes, &model.Note{
			Pitch: model.Pitch{Step: st, Octave: 4 + i/7}, Dur: q,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{c}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func heads(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "notehead") {
			out = append(out, g)
		}
	}
	return out
}

// noTwoSecondsInAColumn is the rule itself, asked of whatever was drawn.
func noTwoSecondsInAColumn(t *testing.T, lay *Layout, what string) {
	t.Helper()
	hs := heads(lay)
	for i := range hs {
		for j := i + 1; j < len(hs); j++ {
			a, b := hs[i], hs[j]
			if math.Abs(a.X-b.X) > 0.05 {
				continue // different columns, nothing to answer for
			}
			// Half a staff space apart in y is one step: a second.
			if math.Abs(math.Abs(a.Y-b.Y)-0.5) < 0.05 {
				t.Errorf("%s: two noteheads a second apart share the column at "+
					"x=%.3f, y=%.3f and %.3f", what, a.X, a.Y, b.Y)
			}
		}
	}
}

func TestChordHeadsASecondApartAreDisplaced(t *testing.T) {
	cases := []struct {
		what  string
		steps []model.Step
	}{
		{"a second", []model.Step{model.StepG, model.StepA}},
		{"a third", []model.Step{model.StepG, model.StepB}},
		{"a cluster of three", []model.Step{model.StepG, model.StepA, model.StepB}},
		{"a cluster of four", []model.Step{
			model.StepF, model.StepG, model.StepA, model.StepB}},
		{"a second under a third", []model.Step{
			model.StepG, model.StepA, model.StepC}},
	}
	for _, c := range cases {
		lay := Engrave(chordOf(c.steps...), DefaultOptions())
		if n := len(heads(lay)); n != len(c.steps) {
			t.Errorf("%s: %d noteheads drawn, want %d", c.what, n, len(c.steps))
		}
		noTwoSecondsInAColumn(t, lay, c.what)
	}
}

// A chord with no seconds in it is one column: nothing is moved that does
// not have to be, or a plain triad comes out looking like a cluster.
func TestAChordWithNoSecondsStaysInOneColumn(t *testing.T) {
	lay := Engrave(chordOf(model.StepC, model.StepE, model.StepG), DefaultOptions())
	hs := heads(lay)
	if len(hs) != 3 {
		t.Fatalf("%d noteheads", len(hs))
	}
	for _, h := range hs {
		if math.Abs(h.X-hs[0].X) > 0.05 {
			t.Errorf("a triad was spread over two columns: x=%.3f and %.3f",
				hs[0].X, h.X)
		}
	}
}

// The note nearest the stem is the one that stays put, because that is the
// column the stem is drawn against.
func TestTheNoteNearestTheStemIsNotMoved(t *testing.T) {
	// Low in the staff, so the stem points up and the lowest note anchors.
	lay := Engrave(chordOf(model.StepD, model.StepE), DefaultOptions())
	hs := heads(lay)
	if len(hs) != 2 {
		t.Fatalf("%d noteheads", len(hs))
	}
	// y grows downward: the lower-sounding note has the larger y.
	low, high := hs[0], hs[1]
	if low.Y < high.Y {
		low, high = high, low
	}
	if low.X > high.X {
		t.Errorf("the upper note stayed and the lower one moved: "+
			"low x=%.3f, high x=%.3f", low.X, high.X)
	}
}
