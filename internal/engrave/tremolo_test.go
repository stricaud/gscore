/*
 * tremolo_test.go
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

// The slashes of a tremolo go across the stem, not beside it.
//
// They are one glyph rather than a drawn stack, and the tremolo glyphs are
// registered on their own centre — unlike most of the font, whose glyphs
// start at their left edge. Taking half the advance off, the way the rest of
// the engraver does, hung them off the side of the stem.

func tremoloBar(dur model.Duration, t *model.Tremolo, step model.Step, oct int) *model.Score {
	n := &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: dur}
	n.Tremolo = t
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{n}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func glyphNamed(lay *Layout, name string) (Glyph, bool) {
	for _, g := range lay.Glyphs {
		if g.Name == name {
			return g, true
		}
	}
	return Glyph{}, false
}

// stems are the vertical lines, which is what a tremolo has to sit on.
func stems(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) < 0.01 && math.Abs(l.Y1-l.Y2) > 0.5 {
			out = append(out, l)
		}
	}
	return out
}

func TestTheSlashesSitOnTheStem(t *testing.T) {
	for _, c := range []struct {
		what string
		step model.Step
		oct  int
	}{
		{"a note below the middle line, with the stem up", model.StepG, 4},
		{"a note above it, with the stem down", model.StepE, 5},
	} {
		lay := Engrave(tremoloBar(model.Duration{Num: 1, Den: 2},
			&model.Tremolo{Strokes: 3}, c.step, c.oct), DefaultOptions())

		g, ok := glyphNamed(lay, "tremolo3")
		if !ok {
			t.Errorf("%s: no slashes were drawn", c.what)
			continue
		}
		// The stem the note grows: the one vertical line near the notehead.
		var stem *Line
		for i, l := range stems(lay) {
			if math.Abs(l.X1-g.X) < 1.5 {
				stem = &stems(lay)[i]
				break
			}
		}
		if stem == nil {
			t.Errorf("%s: the slashes at x=%.2f are not near any stem", c.what, g.X)
			continue
		}
		if math.Abs(g.X-stem.X1) > 0.01 {
			t.Errorf("%s: the slashes are drawn at x=%.2f and the stem is at "+
				"%.2f — they are beside it, not across it", c.what, g.X, stem.X1)
		}
		// And between the ends of the stem, not off either one.
		lo, hi := math.Min(stem.Y1, stem.Y2), math.Max(stem.Y1, stem.Y2)
		if g.Y < lo-0.01 || g.Y > hi+0.01 {
			t.Errorf("%s: the slashes are at y=%.2f, off a stem running %.2f..%.2f",
				c.what, g.Y, lo, hi)
		}
	}
}

// The number of slashes is what was asked for, and a buzz roll is the z.
func TestTheRightMarkIsDrawn(t *testing.T) {
	for _, c := range []struct {
		tr   *model.Tremolo
		want string
	}{
		{&model.Tremolo{Strokes: 1}, "tremolo1"},
		{&model.Tremolo{Strokes: 2}, "tremolo2"},
		{&model.Tremolo{Strokes: 3}, "tremolo3"},
		{&model.Tremolo{Strokes: 4}, "tremolo4"},
		{&model.Tremolo{Buzz: true}, "buzzRoll"},
	} {
		lay := Engrave(tremoloBar(model.Duration{Num: 1, Den: 4}, c.tr,
			model.StepG, 4), DefaultOptions())
		if _, ok := glyphNamed(lay, c.want); !ok {
			t.Errorf("%v was drawn as something other than %s", c.tr, c.want)
		}
	}
}

// A whole note has no stem to put them on, so they go over the notehead —
// which is where every published score puts them.
func TestAStemlessNoteStillGetsItsSlashes(t *testing.T) {
	lay := Engrave(tremoloBar(model.Duration{Num: 1, Den: 1},
		&model.Tremolo{Strokes: 3}, model.StepG, 4), DefaultOptions())

	g, ok := glyphNamed(lay, "tremolo3")
	if !ok {
		t.Fatal("a whole note's tremolo was not drawn at all")
	}
	head, ok := glyphNamed(lay, "noteheadWhole")
	if !ok {
		t.Fatal("no whole notehead was drawn")
	}
	if g.Y >= head.Y {
		t.Errorf("the slashes are at y=%.2f and the notehead at %.2f: they "+
			"should be above it", g.Y, head.Y)
	}
	if math.Abs(g.X-(head.X+noteheadWidth(model.Whole)/2)) > 0.3 {
		t.Errorf("the slashes are at x=%.2f, not centred over the notehead "+
			"at %.2f", g.X, head.X)
	}
}
