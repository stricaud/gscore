/*
 * stems_test.go
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

// A stem always reaches the middle line of the staff.
//
// At the standard length a note out on ledger lines has a stem that stops in
// mid air, and from the third ledger line out it never arrives at the staff
// at all: the note reads as something floating above the music rather than
// as a high note on it.

// run is one note per bar, so every stem stands on its own.
func run(pitches []model.Pitch, dur model.Duration) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for _, pi := range pitches {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: pi, Dur: dur},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

// stemOf is the stem growing out of a notehead: the vertical line that
// starts at it. Barlines are vertical too, which is why starting at the head
// is the test and not simply being near it.
func stemOf(lay *Layout, g Glyph) (Line, bool) {
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) > 0.01 {
			continue // horizontal: a staff line, a ledger line or a beam
		}
		if l.X1 < g.X-0.3 || l.X1 > g.X+2 {
			continue
		}
		if math.Abs(l.Y1-g.Y) < 0.2 {
			return l, true
		}
	}
	return Line{}, false
}

// scale is every note from two ledger lines below to three above, with B4 —
// the middle line in the treble — among them to say where that line is.
func scale() ([]model.Pitch, model.Pitch) {
	middle := model.Pitch{Step: model.StepB, Octave: 4}
	var out []model.Pitch
	for _, o := range []int{3, 4, 5, 6} {
		for _, s := range []model.Step{model.StepC, model.StepE, model.StepG} {
			out = append(out, model.Pitch{Step: s, Octave: o})
		}
	}
	return append(out, middle, model.Pitch{Step: model.StepC, Octave: 7}), middle
}

func middleLineY(t *testing.T, lay *Layout, pitches []model.Pitch, middle model.Pitch) float64 {
	t.Helper()
	// The middle line is wherever the note that sits on it was drawn.
	at := -1
	for i, p := range pitches {
		if p == middle {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the score has no note on the middle line, so nothing can be measured")
	}
	var hs []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "notehead") {
			hs = append(hs, g)
		}
	}
	if len(hs) != len(pitches) {
		t.Fatalf("%d noteheads drawn for %d notes", len(hs), len(pitches))
	}
	return hs[at].Y
}

func TestAStemReachesTheMiddleLine(t *testing.T) {
	pitches, middle := scale()
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(run(pitches, model.Duration{Num: 1, Den: 4}), opt)
	mid := middleLineY(t, lay, pitches, middle)

	n := 0
	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		s, ok := stemOf(lay, g)
		if !ok {
			t.Errorf("the note at y=%.2f has no stem", g.Y)
			continue
		}
		n++
		if s.Y2 > g.Y { // drawn downwards, from a note above the middle
			if s.Y2 < mid-0.01 {
				t.Errorf("the note at y=%.2f has a stem ending at %.2f, short of "+
					"the middle line at %.2f", g.Y, s.Y2, mid)
			}
		} else if s.Y2 > mid+0.01 {
			t.Errorf("the note at y=%.2f has a stem ending at %.2f, short of "+
				"the middle line at %.2f", g.Y, s.Y2, mid)
		}
	}
	if n != len(pitches) {
		t.Errorf("%d stems found for %d notes", n, len(pitches))
	}
}

// And a note inside the staff keeps the plain stem: reaching the middle line
// only ever lengthens one, or an ordinary tune comes out with its stems all
// different lengths for no reason anybody reading it could name.
func TestANoteInTheStaffKeepsTheStandardStem(t *testing.T) {
	pitches := []model.Pitch{
		{Step: model.StepE, Octave: 4}, // bottom line
		{Step: model.StepG, Octave: 4},
		{Step: model.StepC, Octave: 5},
		{Step: model.StepF, Octave: 5}, // top line
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(run(pitches, model.Duration{Num: 1, Den: 4}), opt)

	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		s, ok := stemOf(lay, g)
		if !ok {
			t.Fatalf("the note at y=%.2f has no stem", g.Y)
		}
		if l := math.Abs(s.Y2 - s.Y1); math.Abs(l-stemLength) > 0.01 {
			t.Errorf("the note at y=%.2f has a stem %.2f long, want %.2f",
				g.Y, l, stemLength)
		}
	}
}

// A beamed run is held to the same rule. The beam is drawn through the stem
// ends, so a group of high notes with short stems floats off the staff
// together, beam and all.
func TestABeamedRunHighUpReachesTheStaff(t *testing.T) {
	q := model.Duration{Num: 1, Den: 8}
	var els []model.Element
	for _, p := range []model.Pitch{
		{Step: model.StepC, Octave: 6}, {Step: model.StepE, Octave: 6},
		{Step: model.StepG, Octave: 6}, {Step: model.StepC, Octave: 7},
	} {
		els = append(els, &model.Note{Pitch: p, Dur: q})
	}
	// With a note on the middle line in the same bar, to measure against.
	els = append(els, &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 2},
	})
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
			}},
		}},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)

	var hs []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "notehead") {
			hs = append(hs, g)
		}
	}
	if len(hs) != 5 {
		t.Fatalf("%d noteheads drawn", len(hs))
	}
	mid := hs[4].Y // the half note on the middle line
	for _, g := range hs[:4] {
		s, ok := stemOf(lay, g)
		if !ok {
			t.Errorf("the beamed note at y=%.2f has no stem", g.Y)
			continue
		}
		if s.Y2 < mid-0.01 {
			t.Errorf("the beamed note at y=%.2f has a stem ending at %.2f, "+
				"short of the middle line at %.2f", g.Y, s.Y2, mid)
		}
	}
}
