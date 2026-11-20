/*
 * hairpinwords_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A hairpin written as its word: "cresc." and a broken line to where it
 * stops, in place of the wedge. Asked for as "I want to use the cresc. text
 * instead of the < but still have it working the same way".
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// wordedSwell is one bar of four quarters with a hairpin from the first to
// the last, written as words or as a wedge.
func wordedSwell(kind model.HairpinKind, words bool) *Layout {
	q := model.Duration{Num: 1, Den: 4}
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q}
		switch i {
		case 0:
			n.Hairpin = &model.Hairpin{Kind: kind, Start: true, Words: words}
		case 3:
			n.Hairpin = &model.Hairpin{Kind: kind, Stop: true, Words: words}
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

// slanted counts the lines that are neither level nor upright: the two
// sides of a wedge, and nothing else in a bar of plain quarters.
func slanted(lay *Layout) int {
	n := 0
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) > 0.01 && math.Abs(l.Y1-l.Y2) > 0.01 {
			n++
		}
	}
	return n
}

func TestAHairpinAsWordsIsTheWordAndABrokenLine(t *testing.T) {
	lay := wordedSwell(model.Crescendo, true)

	var word *Text
	for i := range lay.Texts {
		if lay.Texts[i].S == "cresc." {
			word = &lay.Texts[i]
		}
	}
	if word == nil {
		t.Fatal("no \"cresc.\" written")
	}
	if !word.Italic {
		t.Error("the word is not italic")
	}
	if slanted(lay) != 0 {
		t.Errorf("%d slanted lines drawn: the wedge is still there", slanted(lay))
	}
	var dashed *Line
	for i := range lay.Lines {
		if lay.Lines[i].Dash > 0 {
			dashed = &lay.Lines[i]
		}
	}
	if dashed == nil {
		t.Fatal("no broken line drawn after the word")
	}
	if dashed.X1 <= word.X {
		t.Errorf("the line begins at %.2f, not after the word at %.2f", dashed.X1, word.X)
	}
	if math.Abs(dashed.Y1-dashed.Y2) > 0.01 {
		t.Error("the broken line is not level")
	}
	// It reaches to under the last note, as the wedge does.
	wedge := wordedSwell(model.Crescendo, false)
	if len(wedge.Hairpins) != 1 || len(lay.Hairpins) != 1 {
		t.Fatalf("%d and %d hairpins reported, want one each", len(wedge.Hairpins), len(lay.Hairpins))
	}
	wEnd := wedge.Hairpins[0].X + wedge.Hairpins[0].Width
	if math.Abs(dashed.X2-wEnd) > 0.05 {
		t.Errorf("the line ends at %.2f and the wedge would end at %.2f", dashed.X2, wEnd)
	}
	// Reported to the editor as a crescendo it can pick up, like the wedge.
	if h := lay.Hairpins[0]; h.Kind != "crescendo" || h.Width <= 0 || h.Height <= 0 {
		t.Errorf("the words are reported as %+v", h)
	}
	// Under the staff, where a hairpin goes: below the bottom line.
	bottom := math.Inf(-1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X2-l.X1 > 8 && l.Y1 > bottom {
			bottom = l.Y1
		}
	}
	if word.Y <= bottom {
		t.Errorf("the word sits at %.2f, above the bottom line at %.2f", word.Y, bottom)
	}
}

func TestADiminuendoAsWordsSaysDim(t *testing.T) {
	lay := wordedSwell(model.Diminuendo, true)
	found := false
	for _, tx := range lay.Texts {
		if tx.S == "dim." {
			found = true
		}
	}
	if !found {
		t.Error("no \"dim.\" written")
	}
	if slanted(lay) != 0 {
		t.Error("a wedge was drawn as well")
	}
}
