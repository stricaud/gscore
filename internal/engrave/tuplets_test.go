/*
 * tuplets_test.go
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

// A tuplet is written as a figure, and bracketed where the beam does not
// already say where the group begins and ends.
//
// Without the figure a triplet is three notes that do not add up. Without the
// bracket, a group too long to beam — three quarter notes in the time of two
// — has a figure floating over it saying three of what, from where to where.
// A group that is beamed needs no bracket and conventionally does not get
// one: the beam draws the line and marks both ends already.

// tupletBar is one bar holding a group of n notes of a value, marked as a
// tuplet, with the rest of the bar left empty.
func tupletBar(n int, dur model.Duration, actual, normal int) *model.Score {
	var els []model.Element
	for i := 0; i < n; i++ {
		note := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: dur,
		}
		note.Tuplet = &model.Tuplet{
			Actual: actual, Normal: normal,
			Start: i == 0, Stop: i == n-1,
		}
		els = append(els, note)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// horizontals are the level lines drawn, which is where a bracket shows up.
func horizontals(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) < 0.01 && math.Abs(l.X1-l.X2) > 0.01 {
			out = append(out, l)
		}
	}
	return out
}

// bracketLines are the horizontals that sit outside the staff, which the
// staff's own lines and a beam never are.
func bracketLines(lay *Layout, numberY float64) []Line {
	var out []Line
	for _, l := range horizontals(lay) {
		if math.Abs(l.Y1-numberY) < 1.2 {
			out = append(out, l)
		}
	}
	return out
}

func TestAnUnbeamedTupletIsBracketed(t *testing.T) {
	// Three quarter notes in the time of two: too long to beam, so the
	// bracket is the only thing that says where the group is.
	lay := Engrave(tupletBar(3, model.Duration{Num: 1, Den: 6}, 3, 2), DefaultOptions())

	nums := textsAt(lay, "3")
	if len(nums) != 1 {
		t.Fatalf("the figure was drawn %d times, want once", len(nums))
	}
	lines := bracketLines(lay, nums[0].Y)
	if len(lines) < 2 {
		t.Fatalf("the group is not bracketed: %d lines on the figure's own "+
			"level", len(lines))
	}
	// The bracket leaves a gap for the figure rather than being drawn
	// through it.
	for _, l := range lines {
		x0, x1 := math.Min(l.X1, l.X2), math.Max(l.X1, l.X2)
		if x0 < nums[0].X-0.1 && x1 > nums[0].X+0.1 {
			t.Errorf("a bracket line runs from %.2f to %.2f, straight through "+
				"the figure at %.2f", x0, x1, nums[0].X)
		}
	}
	// And it has a hook at each end, turning in towards the notes: two
	// verticals at the group's ends, on the figure's level.
	hooks := 0
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) < 0.01 && math.Abs(l.Y1-nums[0].Y) < 1.5 {
			hooks++
		}
	}
	if hooks < 2 {
		t.Errorf("the bracket has %d hooks, want one at each end", hooks)
	}
}

// A beamed group is left alone: the beam says it, and a bracket over it is
// the same statement twice.
func TestABeamedTupletIsNotBracketed(t *testing.T) {
	// Three eighths in the time of two, which the engraver beams.
	lay := Engrave(tupletBar(3, model.Duration{Num: 1, Den: 12}, 3, 2), DefaultOptions())

	nums := textsAt(lay, "3")
	if len(nums) != 1 {
		t.Fatalf("the figure was drawn %d times", len(nums))
	}
	if lines := bracketLines(lay, nums[0].Y); len(lines) != 0 {
		t.Errorf("a beamed triplet was bracketed as well: %d lines on the "+
			"figure's level", len(lines))
	}
}

// The figure says how many, for any size of group.
func TestTheFigureIsTheNumberOfNotes(t *testing.T) {
	for _, c := range []struct {
		n, actual, normal int
		dur               model.Duration
		want              string
	}{
		{5, 5, 4, model.Duration{Num: 1, Den: 20}, "5"},
		{6, 6, 4, model.Duration{Num: 1, Den: 24}, "6"},
		{7, 7, 4, model.Duration{Num: 1, Den: 28}, "7"},
	} {
		lay := Engrave(tupletBar(c.n, c.dur, c.actual, c.normal), DefaultOptions())
		if n := len(textsAt(lay, c.want)); n != 1 {
			t.Errorf("a group of %d is written %q %d times, want once",
				c.n, c.want, n)
		}
	}
}

/* ---------- the figure is something to point at ---------- */

// The figure belongs to the whole run and to no one note of it, so nothing
// in the music can be clicked to reach it. The engraver is the only thing
// that knows where it went, so it says.
func TestTheTupletFigureIsReportedAsSomethingToClick(t *testing.T) {
	score := model.NewScore()
	v := score.Parts[0].Measures[0].Voices[0]
	for i := 0; i < 3; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 12},
		}
		n.Marks.Tuplet = &model.Tuplet{
			Actual: 3, Normal: 2, Start: i == 0, Stop: i == 2,
		}
		v.Elements = append(v.Elements, n)
	}

	lay := Engrave(score, DefaultOptions())
	if len(lay.Tuplets) != 1 {
		t.Fatalf("the layout reports %d figures, want 1", len(lay.Tuplets))
	}
	box := lay.Tuplets[0]
	if box.Figure != 3 {
		t.Errorf("the figure is %d, want 3", box.Figure)
	}
	if box.Width <= 0 || box.Height <= 0 {
		t.Errorf("the box is %v by %v", box.Width, box.Height)
	}
	// The numbers the engraver gives elements, which the editor uses to
	// find them again — counted from one, as every other Ref here is.
	if box.From != 1 || box.To != 3 {
		t.Errorf("it names notes %d to %d, want 1 to 3", box.From, box.To)
	}
}

// Over a beamed group the figure is all that is drawn, and the notes are
// directly under it: a box as wide as the group would swallow every click
// meant for one of them.
func TestTheFigureOverABeamedGroupIsNotAsWideAsTheGroup(t *testing.T) {
	score := model.NewScore()
	v := score.Parts[0].Measures[0].Voices[0]
	for i := 0; i < 3; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 12},
		}
		n.Marks.Tuplet = &model.Tuplet{
			Actual: 3, Normal: 2, Start: i == 0, Stop: i == 2,
		}
		v.Elements = append(v.Elements, n)
	}

	lay := Engrave(score, DefaultOptions())
	box := lay.Tuplets[0]
	if box.Bracketed {
		t.Fatal("a beamed group of three eighths was given a bracket")
	}
	// The run of three eighths is several staff spaces wide; the figure is
	// one character.
	if box.Width > 3 {
		t.Errorf("the figure's box is %v spaces wide, which is the group "+
			"rather than the number", box.Width)
	}
}

// The figure goes outside the beam, not on it.
//
// Its height was worked out from the highest notehead and a stem's length,
// which is where the beam is for an ordinary group and nowhere near it for
// a wide one: a beam is drawn through the ends of the outermost stems, so a
// run climbing two octaves has it a long way from the notes at one end. The
// 17 over the opening run of the Rhapsody came out on top of its own beam.
func TestTheFigureClearsTheBeamOfAWideRun(t *testing.T) {
	score := model.NewScore()
	v := score.Parts[0].Measures[0].Voices[0]
	// Seventeen thirty-seconds climbing two octaves, beamed as one group.
	for i := 0; i < 17; i++ {
		n := &model.Note{
			Pitch: model.PitchFromDiatonicIndex(-7 + i),
			Dur:   model.Duration{Num: 1, Den: 48},
		}
		n.Marks.Tuplet = &model.Tuplet{
			Actual: 17, Normal: 8, Start: i == 0, Stop: i == 16,
		}
		v.Elements = append(v.Elements, n)
	}

	lay := Engrave(score, DefaultOptions())
	var figure *Text
	for i := range lay.Texts {
		if lay.Texts[i].S == "17" {
			figure = &lay.Texts[i]
		}
	}
	if figure == nil {
		t.Fatal("no figure was written over the group")
	}

	// The beams are the thick level lines across the run. The figure must
	// be clear of every one of them that is under it.
	th := DefaultOptions().Defaults.BeamThickness
	for _, l := range lay.Lines {
		if math.Abs(l.Thickness-th) > 1e-6 {
			continue // a stem or a staff line, not a beam
		}
		lo, hi := math.Min(l.X1, l.X2), math.Max(l.X1, l.X2)
		if figure.X < lo || figure.X > hi {
			continue
		}
		top := math.Min(l.Y1, l.Y2) - th/2
		if figure.Y > top {
			t.Errorf("the figure's baseline is at %.2f, which is %.2f into "+
				"a beam whose top edge is %.2f", figure.Y, figure.Y-top, top)
			break
		}
	}
}
