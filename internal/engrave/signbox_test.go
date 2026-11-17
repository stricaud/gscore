/*
 * signbox_test.go
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

// Everything that sends the player somewhere can be pointed at: the signs
// and the directions get a box each, and so does every volta bracket. A
// D.C. nobody could point at was a D.C. nobody could remove.
func TestSignsDirectionsAndBracketsCanBePointedAt(t *testing.T) {
	q := model.Duration{Num: 1, Den: 1}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 6; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	p.Measures[1].Sign = model.Segno
	p.Measures[2].Ending = &model.Ending{Numbers: []int{1}, Start: true, Stop: true}
	p.Measures[2].RightBarline = model.BarRepeatEnd
	p.Measures[3].Ending = &model.Ending{Numbers: []int{2}, Start: true, Stop: true, Open: true}
	p.Measures[4].Sign = model.Fine
	p.Measures[5].Jump = &model.Jump{From: model.FromSegno, Until: model.UntilFine}

	lay := Engrave(&model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}, DefaultOptions())

	byKind := map[string][]SignPos{}
	for _, s := range lay.Signs {
		byKind[s.Kind] = append(byKind[s.Kind], s)
		if s.Width <= 0 || s.Height <= 0 {
			t.Errorf("%s %q at bar %d has no size to click on", s.Kind, s.Text, s.Measure)
		}
	}
	if n := len(byKind["sign"]); n != 2 {
		t.Errorf("%d signs boxed, want the segno and the Fine", n)
	}
	if n := len(byKind["jump"]); n != 1 {
		t.Fatalf("%d directions boxed, want the one D.S. al Fine", n)
	}
	if j := byKind["jump"][0]; j.Measure != 5 || j.Text != "D.S. al Fine" {
		t.Errorf("the direction is boxed as %q at bar %d, want D.S. al Fine at bar 6", j.Text, j.Measure)
	}
	// The sign at the head of its bar, the direction at the end of it: the
	// box of the D.S. ends no further right than the bar does.
	for _, s := range byKind["sign"] {
		if s.Measure != 1 && s.Measure != 4 {
			t.Errorf("a sign is boxed at bar %d, where none was written", s.Measure)
		}
	}

	if n := len(lay.Endings); n != 2 {
		t.Fatalf("%d brackets boxed, want 2", n)
	}
	first, second := lay.Endings[0], lay.Endings[1]
	if first.Measure != 2 || first.To != 2 || first.Text != "1." {
		t.Errorf("the first bracket is boxed as %+v", first)
	}
	if second.Measure != 3 || second.To != 3 || second.Text != "2." {
		t.Errorf("the second bracket is boxed as %+v", second)
	}
	if first.X+first.Width > second.X+0.01 {
		t.Errorf("the brackets overlap: the first ends at %.2f and the second starts at %.2f",
			first.X+first.Width, second.X)
	}
	for _, v := range lay.Endings {
		if v.Width <= 0 || v.Height <= 0 {
			t.Errorf("the bracket at bar %d has no size to click on", v.Measure)
		}
	}
}
