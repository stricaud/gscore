/*
 * crossingvoices_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two voices crossing at a second, and what is written around them.
 *
 * The lower voice steps a notehead to the left, and two things drawn
 * against where it had been ran into it: the upper voice's flat, placed
 * from the upper voice's noteheads alone, and a tie held into the lower
 * voice's note, which ended at the upper voice's column — inside the
 * notehead of the voice that had stepped aside. Reported from bar 25
 * of Black and Tan Fantasy: "the first note there is an engraving
 * collision".
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

func pitched(step model.Step, octave int, d model.Duration) *model.Note {
	return &model.Note{Pitch: model.Pitch{Step: step, Octave: octave}, Dur: d}
}

// The upper voice's flat stands clear of the lower voice's noteheads, which
// stepped to the left of it and carry no sign of their own.
func TestAFlatClearsTheVoiceThatSteppedAside(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	upper := &model.Chord{Dur: half, Notes: []*model.Note{
		signed(model.StepA, 4, model.Flat, half), pitched(model.StepF, 5, half),
	}}
	lower := &model.Chord{Dur: half, Notes: []*model.Note{
		pitched(model.StepF, 4, half), pitched(model.StepB, 4, half),
	}}
	lay := Engrave(twoVoices([]model.Element{upper}, []model.Element{lower}),
		DefaultOptions())

	signs := signsDrawn(lay)
	if len(signs) != 1 {
		t.Fatalf("%d signs drawn, want the one flat", len(signs))
	}
	flat := signs[0]
	left := 1e9
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") && g.X < left {
			left = g.X
		}
	}
	if right := flat.X + smufl.Advance(flat.Name); right > left+0.02 {
		t.Errorf("the flat reaches %.2f and the leftmost notehead starts at "+
			"%.2f: it is printed on the voice that stepped aside", right, left)
	}
}

// A chord held into a bar where its notes have parted: the lower note
// arrives in the other voice, which stepped a notehead to the left of the
// column, and its tie ends in front of that notehead and not inside it —
// and in front of the upper voice's flat, which now stands before both.
func TestATieIntoAVoiceThatSteppedAsideEndsInFrontOfIt(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	held := &model.Chord{Dur: half, Notes: []*model.Note{
		pitched(model.StepF, 4, half), pitched(model.StepF, 5, half),
	}}
	held.TieStart = true
	held.Notes[1].TieStart = true
	top := &model.Chord{Dur: half, Notes: []*model.Note{
		signed(model.StepA, 4, model.Flat, half), pitched(model.StepF, 5, half),
	}}
	top.Notes[1].TieStop = true
	// Crossing the upper voice at a second, so it steps to the left.
	under := &model.Chord{Dur: half, Notes: []*model.Note{
		pitched(model.StepF, 4, half), pitched(model.StepB, 4, half),
	}}
	s := &model.Score{Parts: []*model.Part{{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 2, BeatValue: 4},
		Measures: []*model.Measure{
			{Voices: []*model.Voice{{Elements: []model.Element{held}}},
				RightBarline: model.BarSingle},
			{Voices: []*model.Voice{
				{Elements: []model.Element{top}},
				{Elements: []model.Element{under}},
			}, RightBarline: model.BarFinal},
		},
	}}}
	lay := Engrave(s, DefaultOptions())

	f4 := StaffStep(model.Treble, model.Pitch{Step: model.StepF, Octave: 4})
	// The leftmost notehead of the second bar, which is the lower voice's.
	firstBar := 1e9
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") && g.X < firstBar {
			firstBar = g.X
		}
	}
	left := 1e9
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") && g.X > firstBar+0.5 && g.X < left {
			left = g.X
		}
	}
	found := false
	for _, a := range lay.Arcs {
		if a.Kind != "tie" || a.Step != f4 {
			continue
		}
		found = true
		if a.X2 > left {
			t.Errorf("the F's tie ends at %.2f, inside the notehead that "+
				"starts at %.2f", a.X2, left)
		}
		// Hung under, on the side of the voice it arrives in: arched over,
		// it ran through the noteheads of that voice between the two Fs.
		if a.CY <= a.Y1 {
			t.Errorf("the F's tie arches upward (ends %.2f, middle %.2f); it "+
				"ends in the lower voice and belongs under", a.Y1, a.CY)
		}
		// And in front of the upper voice's flat, which stands in the way.
		for _, g := range signsDrawn(lay) {
			if a.X2 > g.X {
				t.Errorf("the F's tie ends at %.2f, through the %s at %.2f",
					a.X2, g.Name, g.X)
			}
		}
	}
	if !found {
		t.Fatal("the F's tie was not drawn")
	}
}

// A single note tied into a different one keeps its curve: that is what the
// file says, and it is one curve between two notes.
func TestALoneTieIntoAnotherPitchIsStillDrawn(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	a, b := pitched(model.StepE, 4, q), pitched(model.StepG, 4, q)
	a.TieStart = true
	s := model.NewScore()
	s.Parts[0].Measures[0].Voices[0].Elements = []model.Element{
		a, b, &model.Rest{Dur: model.Duration{Num: 1, Den: 2}},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	if n := len(Engrave(s, opt).Arcs); n != 1 {
		t.Errorf("%d ties drawn, want the one the file asked for", n)
	}
}
