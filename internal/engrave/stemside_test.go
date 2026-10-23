/*
 * stemside_test.go
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

// A beam serves a whole group and can only point one way, so a note inside
// one does not necessarily get the stem it would have had alone. Everything
// drawn on the side away from the stem — the slurs, the ties, the
// articulations, the dots — has to follow the stem that is really there.
//
// It did not. Each of them worked the direction out again from the note by
// itself, and on a rising run, where the group is stemmed down because most
// of its notes are high but the first note is low, the slur was placed as if
// that first note were stemmed up: underneath, straight through the beam.

// runScore is a rising run of beamed sixteenths under one slur, which is the
// shape the bug showed up in.
func runScore(steps []model.Step, octaves []int) *model.Score {
	v := &model.Voice{}
	for i := range steps {
		n := &model.Note{
			Pitch: model.Pitch{Step: steps[i], Octave: octaves[i]},
			Dur:   model.Duration{Num: 1, Den: 16},
		}
		if i == 0 {
			n.SlurStart = true
		}
		if i == len(steps)-1 {
			n.SlurStop = true
		}
		v.Elements = append(v.Elements, n)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{v},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// risingRun is A4 up to A5: one note below the middle line and the rest above
// it, so the group is beamed downwards while its first note, on its own,
// would point up.
func risingRun() *model.Score {
	return runScore(
		[]model.Step{model.StepA, model.StepB, model.StepC, model.StepD,
			model.StepE, model.StepF, model.StepG, model.StepA},
		[]int{4, 4, 5, 5, 5, 5, 5, 5},
	)
}

// curves finds the slurs and ties in a drawing. They are the only quadratic
// curves the engraver draws.
func curves(lay *Layout) []Curve { return lay.Curves }

// stemsOf returns the vertical lines that are stems: tall enough not to be a
// ledger line and thin enough not to be a barline.
func stemsOf(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if l.X1 != l.X2 {
			continue
		}
		if math.Abs(l.Y2-l.Y1) < 1.5 {
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestASlurArcsAwayFromTheBeam(t *testing.T) {
	lay := Engrave(risingRun(), DefaultOptions())

	cs := curves(lay)
	if len(cs) != 1 {
		t.Fatalf("got %d curves, want the one slur", len(cs))
	}
	c := cs[0]

	// The beam is below the noteheads here, so the slur has to be above
	// them. y grows downward, so "above" is a smaller number.
	var lowestHead float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Y > lowestHead {
			lowestHead = g.Y
		}
	}
	if c.CY >= lowestHead {
		t.Errorf("the slur's arch is at y=%.2f, at or below the lowest notehead at y=%.2f: "+
			"it is on the same side as the beam", c.CY, lowestHead)
	}

	// And it really is the stems that are down, not the test being wrong
	// about the shape.
	beams := lay.Beams()
	if len(beams) == 0 {
		t.Fatal("the run was not beamed")
	}
	_, beamY := beams[0].Left()
	if beamY < lowestHead {
		t.Fatalf("the beam is above the noteheads; this test needs a group stemmed down")
	}
	if c.CY > beamY {
		t.Errorf("the slur is below the beam")
	}
}

func TestATieArcsAwayFromTheBeam(t *testing.T) {
	// The same question for a tie, which takes the same path.
	s := risingRun()
	notes := notesOfScore(s)
	notes[0].SlurStart, notes[len(notes)-1].SlurStop = false, false
	notes[6].Pitch = notes[7].Pitch
	notes[6].TieStart, notes[7].TieStop = true, true

	lay := Engrave(s, DefaultOptions())
	cs := curves(lay)
	if len(cs) != 1 {
		t.Fatalf("got %d curves, want the one tie", len(cs))
	}

	var tiedHeadY float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			tiedHeadY = g.Y // the last one drawn is the highest note
		}
	}
	if cs[0].CY >= tiedHeadY {
		t.Errorf("the tie arches to y=%.2f, below its noteheads at y=%.2f", cs[0].CY, tiedHeadY)
	}
}

func TestEveryNoteInABeamGetsTheGroupsStem(t *testing.T) {
	// The direct statement of the rule: a note inside a beamed group is
	// stemmed the way the group is, whatever it would have done alone.
	lay := Engrave(risingRun(), DefaultOptions())

	stems := stemsOf(lay)
	if len(stems) < 8 {
		t.Fatalf("got %d stems over 8 notes", len(stems))
	}
	// Every stem in a downward group runs from the notehead downwards, so
	// each one's lower end is below its upper end by a stem's length. If one
	// note had kept its own inclination it would point the other way.
	down, up := 0, 0
	for _, g := range lay.Glyphs {
		if g.Name != "noteheadBlack" {
			continue
		}
		for _, st := range stems {
			if math.Abs(st.Y1-g.Y) > 0.01 {
				continue
			}
			if st.Y2 > st.Y1 {
				down++
			} else {
				up++
			}
		}
	}
	if up != 0 {
		t.Errorf("%d of the group's stems point up and %d down: the group must agree", up, down)
	}
}

func TestArticulationsFollowTheGroupsStem(t *testing.T) {
	// The same bug reached the articulations: a staccato dot sits on the
	// side away from the stem, and it asked the note rather than the beam.
	s := risingRun()
	notes := notesOfScore(s)
	notes[0].SlurStart, notes[len(notes)-1].SlurStop = false, false
	for _, n := range notes {
		n.AddArticulation(model.Staccato)
	}
	lay := Engrave(s, DefaultOptions())

	// Stems down, so every dot goes above its notehead.
	for _, g := range lay.Glyphs {
		if g.Name != "articStaccatoAbove" && g.Name != "articStaccatoBelow" {
			continue
		}
		if g.Name == "articStaccatoBelow" {
			t.Errorf("a staccato dot at x=%.2f is below the notehead, on the beam's side", g.X)
		}
	}
}

func TestADownwardRunIsStemmedUpAndSlurredBelow(t *testing.T) {
	// The mirror image, so the fix is a rule and not a special case: a run
	// coming down from above the middle line to below it is beamed upwards,
	// and the slur goes under.
	s := runScore(
		[]model.Step{model.StepA, model.StepG, model.StepF, model.StepE,
			model.StepD, model.StepC, model.StepB, model.StepA},
		[]int{4, 4, 4, 4, 4, 4, 3, 3},
	)
	lay := Engrave(s, DefaultOptions())

	beams := lay.Beams()
	if len(beams) == 0 {
		t.Fatal("the run was not beamed")
	}
	var highestHead float64 = 1e9
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Y < highestHead {
			highestHead = g.Y
		}
	}
	if _, beamY := beams[0].Left(); beamY > highestHead {
		t.Fatalf("the beam is below the noteheads; this test needs a group stemmed up")
	}
	cs := curves(lay)
	if len(cs) != 1 {
		t.Fatalf("got %d curves, want the one slur", len(cs))
	}
	if cs[0].CY <= highestHead {
		t.Errorf("the slur arches to y=%.2f, above the highest notehead at y=%.2f: "+
			"it is on the same side as the beam", cs[0].CY, highestHead)
	}
}
