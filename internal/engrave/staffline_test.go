/*
 * staffline_test.go
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

	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// An eighth on the treble staff joined to an eighth rest on the bass staff:
// one line, from beside the note down to beside the rest.
func TestALineRunsFromOneStaffToTheOther(t *testing.T) {
	e8 := model.Duration{Num: 1, Den: 8}
	n := eighth(model.StepE, 4)
	n.StaffLine = &model.StaffLine{Start: true}
	r := &model.Rest{Dur: e8}
	r.StaffLine = &model.StaffLine{Stop: true}
	rest := func(d int) model.Element { return &model.Rest{Dur: model.Duration{Num: 1, Den: d}} }
	with := Engrave(pianoWith(
		[]model.Element{n, rest(8), rest(4), rest(2)},
		[]model.Element{rest(8), r, rest(4), rest(2)}), DefaultOptions())
	n.StaffLine, r.StaffLine = nil, nil
	without := Engrave(pianoWith(
		[]model.Element{n, rest(8), rest(4), rest(2)},
		[]model.Element{rest(8), rest(8), rest(4), rest(2)}), DefaultOptions())
	if len(with.Lines) != len(without.Lines)+1 {
		t.Fatalf("%d lines with the mark and %d without; want one more",
			len(with.Lines), len(without.Lines))
	}
	if len(with.StaffLines) != 1 || with.StaffLines[0].From == 0 || with.StaffLines[0].To == 0 {
		t.Errorf("the editor is told of %+v; want the one line, with both its ends", with.StaffLines)
	}
	upper, lower := staffTopsOf(t, with)
	l := with.Lines[len(with.Lines)-1]
	for _, c := range with.Lines {
		if c.Y2-c.Y1 > 4 && c.X2-c.X1 > 0.5 {
			l = c // the slanted one
		}
	}
	if l.Y1 > upper+6 || l.Y2 < lower-2 || l.X2 <= l.X1 {
		t.Errorf("the line runs from (%.2f, %.2f) to (%.2f, %.2f); "+
			"the staves are at %.2f and %.2f", l.X1, l.Y1, l.X2, l.Y2, upper, lower)
	}
}

// From beamed chords stemmed down towards the other staff, the line leaves
// from past the beam, not from the notehead down through the stem and the
// beam. Bar 29 of Black and Tan Fantasy.
func TestALineLeavesFromBeyondTheBeam(t *testing.T) {
	e8 := model.Duration{Num: 1, Den: 8}
	chord := func(steps ...model.Step) *model.Chord {
		c := &model.Chord{Dur: e8}
		for _, s := range steps {
			c.Notes = append(c.Notes, &model.Note{Pitch: model.Pitch{Step: s, Octave: 4}, Dur: e8})
		}
		c.Stem = model.StemDown
		return c
	}
	a, b := chord(model.StepF, model.StepA), chord(model.StepE, model.StepG)
	b.StaffLine = &model.StaffLine{Start: true}
	r := &model.Rest{Dur: e8}
	r.StaffLine = &model.StaffLine{Stop: true}
	rest := func(d int) model.Element { return &model.Rest{Dur: model.Duration{Num: 1, Den: d}} }
	lay := Engrave(pianoWith(
		[]model.Element{a, b, rest(4), rest(2)},
		[]model.Element{rest(4), r, rest(8), rest(2)}), DefaultOptions())
	beamBottom := 0.0
	for _, q := range lay.Quads {
		for _, y := range q.Y {
			beamBottom = math.Max(beamBottom, y)
		}
	}
	var line *Line
	for i, l := range lay.Lines {
		if l.Y2-l.Y1 > 2 && l.X2 > l.X1 {
			line = &lay.Lines[i]
		}
	}
	if line == nil {
		t.Fatal("no line drawn")
	}
	if line.Y1 <= beamBottom {
		t.Errorf("the line starts at %.2f, inside the beam that reaches %.2f", line.Y1, beamBottom)
	}
}
