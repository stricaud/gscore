/*
 * systemend_test.go
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

// Where a system stops.
//
// A measure's width includes the gap after its barline, which is there to
// keep the next bar's music off the line. The last bar of a system has no
// next bar, and the gap was drawn anyway: the staff lines ran a space past
// the barline that closes the system, and at the end of a piece past the
// one that ends it.

// ending is a score of n staves and bars whose last bar closes with style.
func ending(staves, bars int, style model.BarlineStyle) *model.Score {
	s := &model.Score{Identity: model.Identity{Title: "Ending"}}
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for j := 0; j < bars; j++ {
			var els []model.Element
			for k := 0; k < 4; k++ {
				els = append(els, &model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 4},
				})
			}
			bar := model.BarSingle
			if j == bars-1 {
				bar = style
			}
			p.Measures = append(p.Measures, &model.Measure{
				Voices: []*model.Voice{{Elements: els}}, RightBarline: bar,
			})
		}
		s.Parts = append(s.Parts, p)
	}
	s.Normalise()
	return s
}

// edges is how far right the staff lines reach and how far right any
// barline stands.
func edges(lay *Layout) (staff, barline float64) {
	staff, barline = math.Inf(-1), math.Inf(-1)
	for _, ln := range lay.Lines {
		switch {
		case math.Abs(ln.Y1-ln.Y2) < 0.001: // a staff line, drawn across
			staff = math.Max(staff, math.Max(ln.X1, ln.X2))
		case math.Abs(ln.X1-ln.X2) < 0.001: // a barline, drawn down
			barline = math.Max(barline, ln.X1)
		}
	}
	return staff, barline
}

// The staff lines stop at the barline that closes the system. Past it they
// are ruling that carries on after the music has stopped.
func TestTheStaffStopsAtTheBarlineThatClosesIt(t *testing.T) {
	for _, style := range []model.BarlineStyle{
		model.BarSingle, model.BarDouble, model.BarFinal,
	} {
		o := DefaultOptions()
		o.Mode = Paginated
		lay := Engrave(ending(1, 24, style), o)
		staff, barline := edges(lay)
		if over := staff - barline; math.Abs(over) > 0.05 {
			t.Errorf("%v: the staff reaches %.2f and the last barline "+
				"stands at %.2f, %.2f past it", style, staff, barline, over)
		}
	}
}

// And the line is filled to the margin by the barline, not by the empty
// space behind it: stretching the gap out to the edge left the sign that
// closes the system standing short of it.
func TestTheClosingBarlineLandsOnTheMargin(t *testing.T) {
	o := DefaultOptions()
	o.Mode = Paginated
	lay := Engrave(ending(1, 24, model.BarSingle), o)
	_, barline := edges(lay)
	want := o.Spacings.Margin + o.LineWidth
	if math.Abs(barline-want) > 0.05 {
		t.Errorf("the system closes at %.2f, want the margin at %.2f",
			barline, want)
	}
}

// A barline made of two strokes is two strokes all the way down a system
// whose staves are barred together. Told only "a line goes here", the
// gap-filling drew a plain thin stroke whatever the staves had drawn, so the
// final barline of a bracketed score came out thin through the gaps and
// thin-thick on the staves — the heavy half of the sign stopping at every
// staff and starting again at the next.
func TestAHeavyBarlineRunsThroughTheWholeSystem(t *testing.T) {
	s := ending(2, 8, model.BarFinal)
	s.Groups = []model.StaffGroup{
		{First: 0, Last: 1, Symbol: model.GroupBracket, Barlines: true},
	}
	o := DefaultOptions()
	o.Mode = Paginated
	lay := Engrave(s, o)

	_, barline := edges(lay)
	// The thick half of the final barline, wherever it was drawn.
	heavy := 0
	for _, ln := range lay.Lines {
		if math.Abs(ln.X1-ln.X2) < 0.001 && math.Abs(ln.X1-barline) < 0.001 {
			heavy++
		}
	}
	// One for each of the two staves, and one for the gap between them.
	if heavy < 3 {
		t.Errorf("the heavy half of the final barline is drawn %d times, "+
			"want it on both staves and through the gap between them", heavy)
	}
}

// A bar that carries a rehearsal mark is not also numbered.
//
// The mark is the name a conductor calls that bar by, and a bar does not
// need two names. Where the mark is the bar's own number — which is what
// Finale's boxed rehearsal number is — the page read "5" beside a boxed
// "5": the same thing said twice, and looking like a mistake.
func TestABarWithARehearsalMarkIsNotAlsoNumbered(t *testing.T) {
	s := ending(1, 8, model.BarFinal)
	s.Numbering.Every = 1 // number every bar, so the question is only the mark
	s.Parts[0].Measures[2].Marker = "A"

	o := DefaultOptions()
	o.Mode = Paginated
	o.ShowMeasureNumbers = true
	lay := Engrave(s, o)

	seen := map[string]bool{}
	for _, tx := range lay.Texts {
		seen[tx.S] = true
	}
	if !seen["A"] {
		t.Fatal("the rehearsal mark was not drawn at all")
	}
	if seen["3"] {
		t.Error("the marked bar is numbered as well as named")
	}
	if !seen["2"] || !seen["4"] {
		t.Error("the bars either side of it lost their numbers too")
	}
}
