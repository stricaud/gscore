/*
 * voicecross_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two voices whose stems would run through each other step aside.
 *
 * The first voice stems up and the second down, by number. Where the first
 * dips under the second — a held E in the right hand under the tune — its
 * stem goes up through the other voice's notehead, and the tune's stem
 * comes down through the held note. Reported as "when I entered the 16th
 * note in 2nd voice over the half that is in 1st voice, I got a collision",
 * with the stems turned by hand to get round it. The lower voice steps to
 * the left instead, as it does at a second, so the two stems fall in one
 * line with a head on either side.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// headX is the x of the notehead glyph at a staff step, among the glyphs of
// one value's head; the test knows there is one of each it asks for.
func headX(t *testing.T, lay *Layout, name string, step int) float64 {
	t.Helper()
	var xs []float64
	for _, g := range lay.Glyphs {
		if g.Name == name && math.Abs(g.Y-stepYOf(lay, step)) < 0.01 {
			xs = append(xs, g.X)
		}
	}
	if len(xs) != 1 {
		t.Fatalf("%d %s heads at step %d, want one", len(xs), name, step)
	}
	return xs[0]
}

// stepYOf is the page y of a staff step on the only staff: the staff lines
// are the long horizontal lines, the top one is where the staff is drawn
// from, and stepY says how far below it a step falls.
func stepYOf(lay *Layout, step int) float64 {
	top := math.Inf(1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X2-l.X1 > 8 && l.Y1 < top {
			top = l.Y1
		}
	}
	return top + stepY(float64(step))
}

func TestCrossedVoicesStepAside(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	quarter := model.Duration{Num: 1, Den: 4}
	// First voice: a chord up to C5, then an E4. Second voice: E5, down to
	// A4, then C5 over the E4. The voices overlap in range — the first
	// reaches above the second's lowest note — so the bar is not an
	// inversion and the stems stay as the numbering gives them: the first
	// voice up, the second down, and crossed at both ends of the bar.
	first := []model.Element{
		stackAt(half, pitchAt(model.StepF, 4), pitchAt(model.StepA, 4), pitchAt(model.StepC, 5)),
		noteAt(model.StepE, 4, half),
	}
	second := []model.Element{
		noteAt(model.StepE, 5, quarter),
		noteAt(model.StepA, 4, quarter),
		noteAt(model.StepC, 5, half),
	}
	lay := Engrave(twoVoices(first, second), DefaultOptions())

	head := smufl.NoteheadGlyph(model.Half)
	w := smufl.Advance(head)
	// Treble, counted from the bottom line, which is E4: E4 is step 0, F4
	// step 1, C5 step 5 and E5 step 7.
	// Two C5 halves: the chord's, and the second voice's over the E4,
	// which is the later one.
	e4, c5 := headX(t, lay, head, 0), 0.0
	for _, g := range lay.Glyphs {
		if g.Name == head && math.Abs(g.Y-stepYOf(lay, 5)) < 0.01 && g.X > c5 {
			c5 = g.X
		}
	}
	if got := c5 - e4; math.Abs(got-w) > 0.05 {
		t.Errorf("the E4 under the C5 sits %.2f to its left, want a notehead's width %.2f", got, w)
	}
	// The first column too: the E5's stem comes down through the chord
	// unless the chord steps aside.
	black := smufl.NoteheadGlyph(model.Quarter)
	fa := headX(t, lay, head, 1) // the chord's F4
	e5 := headX(t, lay, black, 7)
	if got := e5 - fa; math.Abs(got-smufl.Advance(head)) > 0.05 {
		t.Errorf("the chord under the E5 sits %.2f to its left, want %.2f", got, smufl.Advance(head))
	}
}

// With the stems the right way about nothing moves: a third apart and the
// upper voice stemming up, the lower down, is the ordinary case.
func TestVoicesAThirdApartWithStemsAwayStayPut(t *testing.T) {
	half := model.Duration{Num: 1, Den: 2}
	first := []model.Element{noteAt(model.StepE, 5, half), noteAt(model.StepE, 5, half)}
	second := []model.Element{noteAt(model.StepC, 5, half), noteAt(model.StepC, 5, half)}
	lay := Engrave(twoVoices(first, second), DefaultOptions())
	head := smufl.NoteheadGlyph(model.Half)
	var xs []float64
	for _, g := range lay.Glyphs {
		if g.Name == head {
			xs = append(xs, g.X)
		}
	}
	if len(xs) != 4 {
		t.Fatalf("%d half-note heads, want 4", len(xs))
	}
	// Two columns, each with both heads at one x.
	if math.Abs(xs[0]-xs[1]) > 0.01 && math.Abs(xs[0]-xs[2]) > 0.01 && math.Abs(xs[0]-xs[3]) > 0.01 {
		t.Errorf("no head shares a column with the first: %v", xs)
	}
}

func TestVoicesClear(t *testing.T) {
	cases := []struct {
		apart           int
		lowerUp, upDown bool
		clear           bool
	}{
		{2, false, false, true},  // a third, stems away
		{1, false, false, false}, // a second
		{0, false, false, false}, // a unison
		{2, true, false, false},  // a third, the lower stemming up
		{5, false, true, false},  // a sixth, the upper stemming down
		{8, true, true, true},    // past the stem's reach
	}
	for _, c := range cases {
		if got := voicesClear(c.apart, c.lowerUp, c.upDown); got != c.clear {
			t.Errorf("voicesClear(%d, %v, %v) = %v, want %v", c.apart, c.lowerUp, c.upDown, got, c.clear)
		}
	}
}
