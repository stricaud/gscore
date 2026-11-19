/*
 * signside_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A sign written at the end of its bar.
 *
 * "To Coda" is where the music is left, and many a printed copy sets it
 * against the barline the player goes out through rather than at the head
 * of the bar. Which side is a choice, and the page has to honour it.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// signedBars is three bars of eighth notes — wide enough for the words to
// fit inside one — with a To Coda at bar 2, written at the head or the end
// of it, and a D.S. al Coda at bar 3.
func signedBars(atEnd bool) *Layout {
	e := model.Duration{Num: 1, Den: 8}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 3; i++ {
		var els []model.Element
		for k := 0; k < 8; k++ {
			els = append(els, &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: e})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	p.Measures[1].Sign = model.ToCoda
	p.Measures[1].SignAtEnd = atEnd
	p.Measures[2].Jump = &model.Jump{From: model.FromSegno, Until: model.UntilCoda}
	return Engrave(&model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}, DefaultOptions())
}

// signBoxes is the boxes of the sign at a bar, symbol and words, in order.
func signBoxes(lay *Layout, bar int) []SignPos {
	var out []SignPos
	for _, s := range lay.Signs {
		if s.Kind == "sign" && s.Measure == bar {
			out = append(out, s)
		}
	}
	return out
}

// barEdges is where a bar begins and ends on the staff, from the band the
// editor picks it up by.
func barEdges(t *testing.T, lay *Layout, bar int) (float64, float64) {
	t.Helper()
	for _, s := range lay.Strips {
		if s.Measure == bar {
			return s.X0, s.X1
		}
	}
	t.Fatalf("bar %d has no band", bar)
	return 0, 0
}

func TestASignAskedForAtTheEndSitsAgainstTheBarline(t *testing.T) {
	head := signBoxes(signedBars(false), 1)
	end := signBoxes(signedBars(true), 1)
	if len(head) != 2 || len(end) != 2 {
		t.Fatalf("a To Coda is boxed as %d then %d pieces, want the sign and the words both times", len(head), len(end))
	}
	lay := signedBars(true)
	left, right := barEdges(t, lay, 1)
	last := end[len(end)-1]
	if got := last.X + last.Width; got > right || right-got > 1.0 {
		t.Errorf("at the end, the words finish at %.2f against a barline at %.2f", got, right)
	}
	if end[0].X <= head[0].X {
		t.Errorf("the sign at the end starts at %.2f, no further right than at the head, %.2f", end[0].X, head[0].X)
	}
	if end[0].X < left {
		t.Errorf("the sign at the end starts at %.2f, before its bar begins at %.2f", end[0].X, left)
	}
	// And the box says which side it is on, for the dialog that moves it.
	for _, s := range end {
		if !s.AtEnd {
			t.Errorf("the box of %q does not say it is at the end", s.Text)
		}
	}
	for _, s := range head {
		if s.AtEnd {
			t.Errorf("the box of %q says it is at the end when it is at the head", s.Text)
		}
	}
	// The symbol stays in front of the words, with air between them.
	if end[0].X+end[0].Width >= end[1].X {
		t.Errorf("the coda sign ends at %.2f and the words begin at %.2f", end[0].X+end[0].Width, end[1].X)
	}
}

// With a direction in the same bar the sign at the end gives way to it: the
// D.S. keeps the barline and the sign ends before the D.S. begins.
func TestASignAtTheEndStandsBeforeTheDirection(t *testing.T) {
	q := model.Duration{Num: 1, Den: 1}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 3; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	p.Measures[1].Sign = model.Fine
	p.Measures[1].SignAtEnd = true
	p.Measures[1].Jump = &model.Jump{From: model.FromStart, Until: model.UntilEnd}
	lay := Engrave(&model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}, DefaultOptions())
	var fine, dc *SignPos
	for i := range lay.Signs {
		s := &lay.Signs[i]
		switch {
		case s.Kind == "sign" && s.Measure == 1:
			fine = s
		case s.Kind == "jump" && s.Measure == 1:
			dc = s
		}
	}
	if fine == nil || dc == nil {
		t.Fatalf("Fine and D.C. not both boxed: %+v", lay.Signs)
	}
	if fine.X+fine.Width >= dc.X {
		t.Errorf("Fine ends at %.2f and the D.C. begins at %.2f", fine.X+fine.Width, dc.X)
	}
}
