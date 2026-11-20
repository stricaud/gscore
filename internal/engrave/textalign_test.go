/*
 * textalign_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Words set against the bar rather than beside their note: at its left,
 * over its middle, or ending at its barline.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// alignedText is two bars of quarters with "Fine" on the first note of
// the second bar, set as asked.
func alignedText(align model.TextAlign) (*Layout, TextPos, StripPos) {
	q := model.Duration{Num: 1, Den: 4}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for b := 0; b < 2; b++ {
		var els []model.Element
		for i := 0; i < 4; i++ {
			n := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q}
			if b == 1 && i == 0 {
				n.Texts = []model.TextMark{{Text: "Fine", Style: model.TextExpression, Align: align}}
			}
			els = append(els, n)
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	lay := Engrave(&model.Score{Parts: []*model.Part{p}}, DefaultOptions())
	var tp TextPos
	for _, t := range lay.TextMarks {
		if t.Text == "Fine" {
			tp = t
		}
	}
	var bar StripPos
	for _, s := range lay.Strips {
		if s.Measure == 1 {
			bar = s
		}
	}
	return lay, tp, bar
}

func TestWordsCanBeSetAgainstTheBar(t *testing.T) {
	_, beside, bar := alignedText(model.AlignNote)
	_, left, _ := alignedText(model.AlignLeft)
	_, centre, _ := alignedText(model.AlignCentre)
	_, right, _ := alignedText(model.AlignRight)
	for _, tp := range []TextPos{beside, left, centre, right} {
		if tp.Width <= 0 {
			t.Fatalf("the words were not drawn: %+v", tp)
		}
	}
	if !(left.X < centre.X && centre.X < right.X) {
		t.Errorf("left, centre and right begin at %.2f, %.2f and %.2f: not in that order", left.X, centre.X, right.X)
	}
	if left.X > beside.X+0.01 {
		t.Errorf("at the left of the bar the words begin at %.2f, after the first note's %.2f", left.X, beside.X)
	}
	if mid, want := centre.X+centre.Width/2, (bar.X0+bar.X1)/2; math.Abs(mid-want) > 2 {
		t.Errorf("in the middle the words are centred at %.2f, the bar at %.2f", mid, want)
	}
	if end := right.X + right.Width; end > bar.X1+0.01 || end < bar.X1-2 {
		t.Errorf("at the right the words end at %.2f, the bar at %.2f", end, bar.X1)
	}
	// And the box says where it is set, for the window that edits it.
	if beside.Align != "" || left.Align != "left" || centre.Align != "centre" || right.Align != "right" {
		t.Errorf("the boxes say %q %q %q %q", beside.Align, left.Align, centre.Align, right.Align)
	}
}
