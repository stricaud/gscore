/*
 * hairpindashes_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The line after "cresc." can be as long as asked: to the end, none, or so
 * many dashes. "If I double click on cresc. I should be able to choose the
 * number of dashes after it, I might want 0."
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func dashedSwell(dashes int) *Layout {
	q := model.Duration{Num: 1, Den: 4}
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q}
		switch i {
		case 0:
			n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true, Words: true, Dashes: dashes}
		case 3:
			n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true, Words: true, Dashes: dashes}
		}
		els = append(els, n)
	}
	return Engrave(&model.Score{Parts: []*model.Part{{
		Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		}},
	}}}, DefaultOptions())
}

func brokenLine(lay *Layout) *Line {
	for i := range lay.Lines {
		if lay.Lines[i].Dash > 0 {
			return &lay.Lines[i]
		}
	}
	return nil
}

func TestTheWordCanStandAlone(t *testing.T) {
	lay := dashedSwell(-1)
	if l := brokenLine(lay); l != nil {
		t.Errorf("a line was drawn from %.2f to %.2f when none was asked for", l.X1, l.X2)
	}
	found := false
	for _, tx := range lay.Texts {
		if tx.S == "cresc." {
			found = true
		}
	}
	if !found {
		t.Error("the word is gone with the line")
	}
	if len(lay.Hairpins) != 1 || !lay.Hairpins[0].Words || lay.Hairpins[0].Dashes != -1 {
		t.Errorf("the box says %+v, want the words with no line", lay.Hairpins)
	}
}

func TestTheLineCanBeSoManyDashes(t *testing.T) {
	three := brokenLine(dashedSwell(3))
	whole := brokenLine(dashedSwell(0))
	if three == nil || whole == nil {
		t.Fatal("no broken line drawn")
	}
	// Three dashes and two gaps of eight tenths of a dash.
	want := hairpinDash * (3 + 2*0.8)
	if got := three.X2 - three.X1; math.Abs(got-want) > 0.01 {
		t.Errorf("three dashes are %.2f long, want %.2f", got, want)
	}
	if three.X2 >= whole.X2 {
		t.Errorf("three dashes reach %.2f, no shorter than the line to the end at %.2f", three.X2, whole.X2)
	}
	if three.X1 != whole.X1 {
		t.Errorf("the short line starts at %.2f and the long one at %.2f", three.X1, whole.X1)
	}
}
