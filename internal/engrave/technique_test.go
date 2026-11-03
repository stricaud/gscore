/*
 * technique_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Drawing how a fretted instrument plays the note.
 *
 * Three shapes between them: the marks that join two notes are drawn in the
 * gap between them, a bend is an arrow leaving the one note, and the rest
 * are a word set over the staff. What is checked here is that each kind
 * comes out as the kind it is, and that a mark reaching to a note that is
 * not there does not draw a line to nowhere.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// texts is every word in a drawing.
func texts(lay *Layout) []string {
	var out []string
	for _, t := range lay.Texts {
		out = append(out, t.S)
	}
	return out
}

func hasText(lay *Layout, want string) bool {
	for _, s := range texts(lay) {
		if s == want {
			return true
		}
	}
	return false
}

func glyphCount(lay *Layout, name string) int {
	n := 0
	for _, g := range lay.Glyphs {
		if g.Name == name {
			n++
		}
	}
	return n
}

// A hammer-on is the slur a guitar book draws with an H over it, between
// the note it leaves and the note it arrives at.
func TestAHammerOnIsDrawnBetweenTheTwoNotes(t *testing.T) {
	a := quarter(model.StepE, 4, 1, 0)
	a.AddTechnique(model.HammerOn)
	b := quarter(model.StepG, 4, 1, 3)
	lay := Engrave(tabScore([]*model.Note{a, b}), DefaultOptions())

	if !hasText(lay, "H") {
		t.Errorf("no H was written: %v", texts(lay))
	}
	if len(lay.Curves) == 0 {
		t.Fatal("no curve was drawn")
	}
	// Between them, which is what the mark means: it starts after the
	// first fret number and finishes before the second.
	c := lay.Curves[len(lay.Curves)-1]
	if !(c.X1 > 0 && c.X2 > c.X1) {
		t.Errorf("the curve runs from %.2f to %.2f", c.X1, c.X2)
	}
	// And arcs upwards, so the letter over it can be read.
	if c.CY >= c.Y1 {
		t.Errorf("the curve bulges downwards: Y1=%.2f CY=%.2f", c.Y1, c.CY)
	}
}

func TestAPullOffIsWrittenWithAP(t *testing.T) {
	a := quarter(model.StepG, 4, 1, 3)
	a.AddTechnique(model.PullOff)
	b := quarter(model.StepE, 4, 1, 0)
	lay := Engrave(tabScore([]*model.Note{a, b}), DefaultOptions())

	if !hasText(lay, "P") {
		t.Errorf("no P was written: %v", texts(lay))
	}
}

// A slide is the stroke itself and carries no letter: the lean of the line
// is the whole instruction, the way / and \ are in a plain-text tab.
func TestASlideIsAStrokeAndNotALetter(t *testing.T) {
	for _, tc := range []struct {
		tech model.Technique
		down bool
	}{
		{model.SlideUp, false},
		{model.SlideDown, true},
	} {
		a := quarter(model.StepE, 4, 1, 0)
		a.AddTechnique(tc.tech)
		b := quarter(model.StepG, 4, 1, 3)
		lay := Engrave(tabScore([]*model.Note{a, b}), DefaultOptions())

		if hasText(lay, "H") || hasText(lay, "P") {
			t.Errorf("%v was given a letter: %v", tc.tech, texts(lay))
		}
		// The two notes are on one string, so they are level, and the
		// stroke between them is the only sloping line on the staff.
		// Neither level nor upright: the staff lines are the one and the
		// stems are the other.
		found := false
		for _, l := range lay.Lines {
			if l.Y1 == l.Y2 || l.X1 == l.X2 {
				continue
			}
			found = true
			if down := l.Y2 > l.Y1; down != tc.down {
				t.Errorf("%v leans the wrong way: %.2f to %.2f",
					tc.tech, l.Y1, l.Y2)
			}
		}
		if !found {
			t.Errorf("%v drew no stroke", tc.tech)
		}
	}
}

// A mark that reaches to a note that is not there draws nothing.
//
// The last note of a part has nothing to hammer onto, and a curve drawn to
// where the next note would have been is a line running off the end of the
// music — which is what every spanner here gets wrong first.
func TestAHammerOnWithNothingToHammerOntoDrawsNothing(t *testing.T) {
	a := quarter(model.StepE, 4, 1, 0)
	a.AddTechnique(model.HammerOn)
	lay := Engrave(tabScore([]*model.Note{a}), DefaultOptions())

	if hasText(lay, "H") {
		t.Error("it wrote an H for a hammer-on onto nothing")
	}
	if len(lay.Curves) != 0 {
		t.Errorf("it drew %d curves to a note that is not there", len(lay.Curves))
	}
}

// A bend is an arrow going up from the note, with how far it goes at the
// head. The distance is the instruction — a whole tone or a semitone,
// whatever fret it starts from — so it is written out.
func TestABendIsAnArrowWithItsDistanceOnIt(t *testing.T) {
	for _, tc := range []struct {
		tech model.Technique
		want string
	}{
		{model.BendFull, "full"},
		{model.BendHalf, "1/2"},
	} {
		n := quarter(model.StepE, 4, 1, 0)
		n.AddTechnique(tc.tech)
		lay := Engrave(tabScore([]*model.Note{n}), DefaultOptions())

		if !hasText(lay, tc.want) {
			t.Errorf("%v is labelled %v, want %q", tc.tech, texts(lay), tc.want)
		}
		// Up: the arrow's far end is above the note it leaves.
		rising := false
		for _, l := range lay.Lines {
			if l.Y2 < l.Y1-1 {
				rising = true
			}
		}
		if !rising {
			t.Errorf("%v drew no arrow going up", tc.tech)
		}
	}
}

// Bend and release goes up and comes back, which is the whole instruction:
// without the second arrow it is a bend that is never let go.
func TestABendAndReleaseComesBackDown(t *testing.T) {
	n := quarter(model.StepE, 4, 1, 0)
	n.AddTechnique(model.BendRelease)
	lay := Engrave(tabScore([]*model.Note{n}), DefaultOptions())

	up, down := false, false
	for _, l := range lay.Lines {
		if l.Y2 < l.Y1-1 {
			up = true
		}
		if l.Y2 > l.Y1+1 {
			down = true
		}
	}
	if !up || !down {
		t.Errorf("it rises: %v, it falls: %v — want both", up, down)
	}
}

// The words: a palm mute, a let ring, the harmonics and the tap are all
// written over the staff rather than drawn.
func TestTheSpokenMarksAreWrittenOverTheStaff(t *testing.T) {
	for _, tc := range []struct {
		tech model.Technique
		want string
	}{
		{model.PalmMute, "P.M."},
		{model.LetRing, "let ring"},
		{model.NaturalHarmonic, "N.H."},
		{model.ArtificialHarmonic, "A.H."},
		{model.PinchHarmonic, "P.H."},
		{model.Tap, "T"},
	} {
		n := quarter(model.StepE, 4, 1, 0)
		n.AddTechnique(tc.tech)
		lay := Engrave(tabScore([]*model.Note{n}), DefaultOptions())

		if !hasText(lay, tc.want) {
			t.Errorf("%v is written %v, want %q", tc.tech, texts(lay), tc.want)
		}
	}
}

// Vibrato is set from the font, because Bravura cuts the stroke to be laid
// end to end into a wave of any length — and the wide one is a glyph of its
// own rather than the same wave drawn deeper.
func TestVibratoIsSetFromTheFont(t *testing.T) {
	for _, tc := range []struct {
		tech model.Technique
		want string
	}{
		{model.Vibrato, "guitarVibratoStroke"},
		{model.WideVibrato, "guitarWideVibratoStroke"},
	} {
		n := quarter(model.StepE, 4, 1, 0)
		n.AddTechnique(tc.tech)
		lay := Engrave(tabScore([]*model.Note{n}), DefaultOptions())

		if glyphCount(lay, tc.want) == 0 {
			t.Errorf("%v drew no %s", tc.tech, tc.want)
		}
	}
}

// A note can carry more than one, and each is drawn.
func TestANoteCanBeMutedAndHammeredAtOnce(t *testing.T) {
	a := quarter(model.StepE, 4, 1, 0)
	a.AddTechnique(model.PalmMute)
	a.AddTechnique(model.HammerOn)
	b := quarter(model.StepG, 4, 1, 3)
	lay := Engrave(tabScore([]*model.Note{a, b}), DefaultOptions())

	if !hasText(lay, "P.M.") || !hasText(lay, "H") {
		t.Errorf("only some of it was drawn: %v", texts(lay))
	}
}

// A dead note is written as an x where the fret number would be: the string
// is damped, so there is no pitch and no fret to print. It is the shape of
// the note and not a mark of its own, which is why there is no technique
// for it and this reads Head.
func TestADeadNoteIsWrittenAsAnXOnTheTab(t *testing.T) {
	n := quarter(model.StepG, 4, 1, 3)
	n.Head = model.HeadX
	lay := Engrave(tabScore([]*model.Note{n}), DefaultOptions())

	if !hasText(lay, "x") {
		t.Errorf("the tab reads %v, want an x", texts(lay))
	}
	if hasText(lay, "3") {
		t.Error("it printed the fret number as well as the x")
	}
}
