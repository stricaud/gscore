/*
 * arcs_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * Where the ties and slurs went.
 *
 * A curve belongs to the two notes at its ends and to none of the ones it
 * arcs over, so nothing on the page pointed at it: there was no way to click
 * a slur, and the only way to take one off was to find the notes under it
 * and ask again for no slur. The engraver is the only thing that knows where
 * a curve went — a slur's side comes from the stems beneath it and its height
 * from how far it had to climb to clear them — so it writes it down.
 */

// arcQuarter is one bar of quarter notes with marks put on by the caller.
func arcScore(t *testing.T, mark func(i int, n *model.Note), n int) *Layout {
	t.Helper()
	q := model.Duration{Num: 1, Den: 4}
	s := model.NewScore()
	p := s.Parts[0]
	var els []model.Element
	for i := 0; i < n; i++ {
		note := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q,
		}
		mark(i, note)
		els = append(els, note)
	}
	p.Measures[0].Voices[0].Elements = els
	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	return Engrave(s, opt)
}

// Every curve on the page is written down, and each says which two notes it
// runs between.
func TestATieAndASlurSayWhereTheyAre(t *testing.T) {
	lay := arcScore(t, func(i int, n *model.Note) {
		switch i {
		case 0:
			n.TieStart = true
		case 1:
			n.TieStop = true
			n.SlurStart = true
		case 3:
			n.SlurStop = true
		}
	}, 4)

	if len(lay.Arcs) != len(lay.Curves) {
		t.Fatalf("%d curves drawn and %d written down", len(lay.Curves), len(lay.Arcs))
	}
	if len(lay.Arcs) != 2 {
		t.Fatalf("%d arcs, want a tie and a slur", len(lay.Arcs))
	}
	want := []struct {
		kind     string
		from, to int
	}{{"tie", 1, 2}, {"slur", 2, 4}}
	for i, w := range want {
		got := lay.Arcs[i]
		if got.Kind != w.kind || got.From != w.from || got.To != w.to {
			t.Errorf("arc %d is a %s from %d to %d, want a %s from %d to %d",
				i, got.Kind, got.From, got.To, w.kind, w.from, w.to)
		}
	}
}

// A curve is written down as the curve, not as a box round it.
//
// Its ends and its control point are what the editor needs to tell a click
// on a slur from a click on the music under it: anything enclosing the curve
// encloses that music too.
func TestAnArcIsTheCurveItWasDrawnAs(t *testing.T) {
	lay := arcScore(t, func(i int, n *model.Note) {
		if i == 0 {
			n.TieStart = true
		}
		if i == 1 {
			n.TieStop = true
		}
	}, 2)

	if len(lay.Arcs) != 1 || len(lay.Curves) != 1 {
		t.Fatalf("%d arcs for %d curves, want one of each", len(lay.Arcs), len(lay.Curves))
	}
	a, c := lay.Arcs[0], lay.Curves[0]
	if a.X1 != c.X1 || a.Y1 != c.Y1 || a.X2 != c.X2 || a.Y2 != c.Y2 ||
		a.CX != c.CX || a.CY != c.CY {
		t.Errorf("the arc written down is not the curve drawn:\n  arc   %v\n  curve %v", a, c)
	}
}

// A chord is tied notehead by notehead, so each of its ties is its own curve
// and says which notehead it left.
func TestAChordsTiesAreOnePerNotehead(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	chord := func() *model.Chord {
		return &model.Chord{Dur: q, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: q},
			{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q},
		}}
	}
	a, b := chord(), chord()
	a.TieStart, b.TieStop = true, true

	s := model.NewScore()
	p := s.Parts[0]
	p.Measures[0].Voices[0].Elements = []model.Element{
		a, b, &model.Rest{Dur: model.Duration{Num: 1, Den: 2}},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	if len(lay.Arcs) != 2 {
		t.Fatalf("%d arcs for a two-note chord held over, want one per notehead",
			len(lay.Arcs))
	}
	steps := map[int]bool{}
	for _, arc := range lay.Arcs {
		if arc.Kind != "tie" {
			t.Errorf("a chord's hold came out as a %s", arc.Kind)
		}
		if steps[arc.Step] {
			t.Errorf("two ties both say they left the notehead at step %d", arc.Step)
		}
		steps[arc.Step] = true
	}
	for _, want := range []int{0, 2} {
		if !steps[want] {
			t.Errorf("no tie left the notehead at step %d; got %v", want, steps)
		}
	}
}

// A curve cut at a line break is two pieces on the page, and both of them
// have to be clickable.
//
// The piece left at the margin is drawn before the note the curve runs to is
// known, so it is written down with nowhere to run to and told afterwards.
// Without that, clicking the first half of a slur that spans a page turn
// reached a curve with no end, and taking it away would have left the half
// on the next line behind.
func TestBothHalvesOfABrokenCurveKnowWhereItRuns(t *testing.T) {
	whole := model.Duration{Num: 1, Den: 1}
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < 24; i++ {
		n := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: whole}
		if i == 2 {
			n.SlurStart = true
		}
		if i == 14 {
			n.SlurStop = true
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{n}}}})
	}
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	if len(lay.Systems) < 2 {
		t.Fatalf("the music fell on %d system; this needs a line break",
			len(lay.Systems))
	}
	if len(lay.Arcs) != 2 {
		t.Fatalf("%d arcs for a slur cut in two, want both halves", len(lay.Arcs))
	}
	for i, a := range lay.Arcs {
		if a.From != 3 || a.To != 15 {
			t.Errorf("half %d runs from %d to %d, want 3 to 15: a piece that "+
				"does not know the whole curve cannot take it away",
				i, a.From, a.To)
		}
	}
}
