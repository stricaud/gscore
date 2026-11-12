/*
 * stafflines_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A staff ruled with fewer lines than five.
 *
 * One line is the ordinary way to write a drum that has no pitch, and a
 * score can be full of them: Ravel's Boléro gives the snare, the bass drum,
 * the cymbals and the tam-tam a line each, five one-line staves down every
 * page. Drawn on five lines they read as four instruments gone wrong.
 *
 * What is drawn changes and where the notes go does not. The staff keeps the
 * height and the step arithmetic of a five-line staff, and only the middle
 * line is ruled — so a note, a stem, a beam and a rest land where they
 * always did, and everything that measures from the staff goes on working.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// drumScore is one staff of quarter notes, ruled with the given lines.
func drumScore(lines int) *model.Score {
	v := &model.Voice{}
	for i := 0; i < 4; i++ {
		v.Elements = append(v.Elements, &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Name:       "Tambour",
			Clef:       model.Percussion,
			StaffLines: lines,
			Time:       model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{v},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// staffRules are the long horizontal lines of a staff: the ruled lines, as
// against the stems, the barlines and the beams.
func staffRules(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) > 0.01 {
			continue
		}
		if l.X2-l.X1 < 10 {
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestAStaffOfOneLineIsRuledWithOne(t *testing.T) {
	lay := Engrave(drumScore(1), DefaultOptions())
	if n := len(staffRules(lay)); n != 1 {
		t.Errorf("a one-line staff was ruled with %d lines", n)
	}
}

func TestAStaffThatSaysNothingIsRuledWithFive(t *testing.T) {
	lay := Engrave(drumScore(0), DefaultOptions())
	if n := len(staffRules(lay)); n != 5 {
		t.Errorf("an ordinary staff was ruled with %d lines", n)
	}
}

func TestTheOneLineIsWhereTheMiddleLineWas(t *testing.T) {
	// The notes do not move, so the line has to come to them: a drum
	// written on the middle line of a five-line staff keeps that height
	// when the other four lines go.
	five := staffRules(Engrave(drumScore(0), DefaultOptions()))
	one := staffRules(Engrave(drumScore(1), DefaultOptions()))
	if len(five) != 5 || len(one) != 1 {
		t.Fatalf("%d lines and %d lines", len(five), len(one))
	}
	middle := five[2].Y1
	if math.Abs(one[0].Y1-middle) > 0.01 {
		t.Errorf("the line is at y=%.2f and the middle line was at y=%.2f",
			one[0].Y1, middle)
	}
}

func TestTheNotesDoNotMoveWithTheLines(t *testing.T) {
	// The whole of the arrangement: taking four lines away changes what is
	// drawn under the notes and nothing about the notes.
	heads := func(lines int) []Glyph {
		var out []Glyph
		for _, g := range Engrave(drumScore(lines), DefaultOptions()).Glyphs {
			if g.Name == "noteheadBlack" {
				out = append(out, g)
			}
		}
		return out
	}
	five, one := heads(0), heads(1)
	if len(five) != 4 || len(one) != 4 {
		t.Fatalf("%d noteheads and %d", len(five), len(one))
	}
	for i := range five {
		if math.Abs(five[i].Y-one[i].Y) > 0.01 {
			t.Errorf("notehead %d is at y=%.2f on one line and y=%.2f on five",
				i+1, one[i].Y, five[i].Y)
		}
	}
}

func TestAOneLineStaffTakesTheRoomOfAStaff(t *testing.T) {
	// Height is not line count. A staff given no height would have the next
	// one drawn through it, and the notes of a drum sit around their line
	// the way any others do.
	p := drumScore(1).Parts[0]
	if got := partH(p); got != staffHeight {
		t.Errorf("a one-line staff is %.2f tall, want the usual %.2f",
			got, float64(staffHeight))
	}
}

func TestATabStaffIsStillRuledPerString(t *testing.T) {
	// The other staff that is not five lines, which goes the other way:
	// a line per string, further apart, and not centred on anything.
	s := drumScore(0)
	s.Parts[0].Tab = &model.Tablature{
		Strings: []model.Pitch{
			{Step: model.StepE, Octave: 4}, {Step: model.StepB, Octave: 3},
			{Step: model.StepG, Octave: 3}, {Step: model.StepD, Octave: 3},
			{Step: model.StepA, Octave: 2}, {Step: model.StepE, Octave: 2},
		},
	}
	if got := partLines(s.Parts[0]); got != 6 {
		t.Errorf("a six-string tab has %d lines", got)
	}
}
