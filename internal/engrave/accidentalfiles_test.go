/*
 * accidentalfiles_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two accidentals sounding at once stand apart.
 *
 * They were all drawn at the same distance from the noteheads, one above the
 * other. A sharp is nearly three spaces tall, so two of them a second or a
 * third apart overlap and come out as one black mark — the MusicXML suite's
 * chord of a flat, a sharp and a natural was a single blot. Printed music
 * sets them in files instead, the first nearest the chord and each one after
 * it further left. Reported with a photograph of a printed page: "in this
 * situation I have a collision; I want to ... engrave it like the second
 * screenshot".
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// signsDrawn is every accidental in a layout, with where it was put.
func signsDrawn(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "accidental") &&
			!strings.HasPrefix(g.Name, "accidentalParens") {
			out = append(out, g)
		}
	}
	return out
}

// clash reports whether two signs share any ground: the same stretch across
// the page and the same stretch down it.
func clash(a, b Glyph) bool {
	aUp, aDown := glyphReach(a.Name)
	bUp, bDown := glyphReach(b.Name)
	if a.Y-aUp >= b.Y+bDown || b.Y-bUp >= a.Y+aDown {
		return false // one is clear above the other
	}
	const hair = 0.02
	aRight, bRight := a.X+smufl.Advance(a.Name), b.X+smufl.Advance(b.Name)
	return a.X < bRight-hair && b.X < aRight-hair
}

func signed(step model.Step, octave int, a model.Accidental, d model.Duration) *model.Note {
	return &model.Note{
		Pitch:      model.Pitch{Step: step, Octave: octave},
		Dur:        d,
		Accidental: a,
	}
}

func TestAChordsAccidentalsStandApart(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	chord := &model.Chord{
		Notes: []*model.Note{
			signed(model.StepE, 4, model.Flat, half),
			signed(model.StepG, 4, model.Sharp, half),
			signed(model.StepB, 4, model.Natural, half),
		},
		Dur: half,
	}
	lay := Engrave(twoVoices([]model.Element{chord}, nil), DefaultOptions())

	signs := signsDrawn(lay)
	if len(signs) != 3 {
		t.Fatalf("%d signs drawn, want the three the chord carries", len(signs))
	}
	for i := range signs {
		for j := i + 1; j < len(signs); j++ {
			if clash(signs[i], signs[j]) {
				t.Errorf("%s at %.2f,%.2f is printed over %s at %.2f,%.2f",
					signs[i].Name, signs[i].X, signs[i].Y,
					signs[j].Name, signs[j].X, signs[j].Y)
			}
		}
	}
}

// And so do two voices' signs, which is the case that was reported: what a
// reader sees as a chord is every note struck at once, however many lines of
// music they are written on.
func TestTwoVoicesAccidentalsStandApart(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	// A third apart, so neither notehead steps across the stem and the two
	// stand in one column — which is where their signs collided.
	lay := Engrave(twoVoices(
		[]model.Element{signed(model.StepA, 4, model.Sharp, half)},
		[]model.Element{signed(model.StepF, 4, model.Sharp, half)},
	), DefaultOptions())

	signs := signsDrawn(lay)
	if len(signs) != 2 {
		t.Fatalf("%d signs drawn, want two", len(signs))
	}
	if clash(signs[0], signs[1]) {
		t.Errorf("the two voices' signs are printed over each other: "+
			"%s at %.2f,%.2f and %s at %.2f,%.2f",
			signs[0].Name, signs[0].X, signs[0].Y,
			signs[1].Name, signs[1].X, signs[1].Y)
	}
	// The upper one nearest the notes, as printed music sets them.
	upper, lower := signs[0], signs[1]
	if upper.Y > lower.Y {
		upper, lower = lower, upper
	}
	if lower.X >= upper.X {
		t.Errorf("the lower sign is at %.2f and the upper at %.2f; the "+
			"upper one should be the nearer to the notes", lower.X, upper.X)
	}
	// And a whole file apart, not merely nudged: a sign standing half over
	// its neighbour is the thing being fixed, and a hair of daylight
	// between them is not what printed music does.
	if gap := upper.X - lower.X; gap < smufl.Advance(lower.Name) {
		t.Errorf("the two signs are %.2f apart and %s is %.2f wide; they "+
			"are not in files of their own",
			gap, lower.Name, smufl.Advance(lower.Name))
	}
}

// Signs far enough apart to stand in one file still do: nothing here may
// push a chord's accidentals out for the sake of it.
func TestAccidentalsWithRoomStayInOneFile(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	chord := &model.Chord{
		Notes: []*model.Note{
			signed(model.StepF, 4, model.Sharp, half),
			signed(model.StepF, 5, model.Sharp, half),
		},
		Dur: half,
	}
	lay := Engrave(twoVoices([]model.Element{chord}, nil), DefaultOptions())

	signs := signsDrawn(lay)
	if len(signs) != 2 {
		t.Fatalf("%d signs drawn, want two", len(signs))
	}
	if d := signs[0].X - signs[1].X; d > 0.01 || d < -0.01 {
		t.Errorf("two signs an octave apart were set in different files: "+
			"%.2f and %.2f", signs[0].X, signs[1].X)
	}
}

// The room for the outer files is booked when the bar is spaced, or they are
// printed over whatever stands before the chord.
func TestEveryFileOfAccidentalsIsGivenRoom(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	one := &model.Chord{
		Notes: []*model.Note{
			signed(model.StepE, 4, model.Flat, half),
			signed(model.StepE, 5, model.Flat, half),
		},
		Dur: half,
	}
	many := &model.Chord{
		Notes: []*model.Note{
			signed(model.StepE, 4, model.Flat, half),
			signed(model.StepG, 4, model.Sharp, half),
			signed(model.StepB, 4, model.Natural, half),
		},
		Dur: half,
	}
	flat := Engrave(twoVoices([]model.Element{one}, nil), DefaultOptions())
	stepped := Engrave(twoVoices([]model.Element{many}, nil), DefaultOptions())

	if stepped.Width <= flat.Width {
		t.Errorf("a chord whose signs step out into files is %.2f wide and "+
			"one whose signs stand in a single file %.2f; the files asked "+
			"for no room", stepped.Width, flat.Width)
	}
}
