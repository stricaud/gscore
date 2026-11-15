/*
 * numbersymbol_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A bar number and a symbol want the same piece of paper.
 *
 * Both go above the staff at the head of the bar, so a chord diagram put
 * over bar 13 has the numeral 13 printed through it. Which of the two to
 * keep is not something a program can decide — a diagram is the thing a
 * reader is looking at there, and in a score being rehearsed the number is
 * the thing — so the score says, and this is the saying.
 */

package engrave

import (
	"strconv"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// numbered is a four-bar score with a symbol over the bars named, and a
// number on every bar.
func numbered(notOver bool, symbolIn ...int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	has := map[int]bool{}
	for _, i := range symbolIn {
		has[i] = true
	}
	for m := 0; m < 4; m++ {
		els := []model.Element{&model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}}
		if has[m] {
			els = append(els, &model.SymbolElement{
				Symbol: model.SymbolMark{Name: "device"},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}},
		})
	}
	return &model.Score{
		Tempo:     model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Numbering: model.MeasureNumbering{Every: 1, NotOverSymbols: notOver},
		Parts:     []*model.Part{p},
	}
}

// numbersDrawn is the bar numbers on the page.
func numbersDrawn(lay *Layout) map[string]bool {
	out := map[string]bool{}
	for _, t := range lay.Texts {
		if t.Size == measureNumberSize {
			out[t.S] = true
		}
	}
	return out
}

func TestABarWithASymbolCanKeepItsNumber(t *testing.T) {
	// Nothing said: the numbers are printed as they always were, symbol or
	// no symbol. This is what every score that has not asked still gets.
	lay := Engrave(numbered(false, 1, 3), DefaultOptions())
	got := numbersDrawn(lay)
	for _, n := range []int{2, 3, 4} {
		if !got[strconv.Itoa(n)] {
			t.Errorf("bar %d lost its number without being asked: %v", n, got)
		}
	}
}

func TestABarWithASymbolCanDropItsNumber(t *testing.T) {
	lay := Engrave(numbered(true, 1, 3), DefaultOptions())
	got := numbersDrawn(lay)
	// Bars 2 and 4 carry the symbol — counting from nought, measures 1 and
	// 3 — so their numbers go.
	for _, n := range []int{2, 4} {
		if got[strconv.Itoa(n)] {
			t.Errorf("bar %d printed its number over its symbol: %v", n, got)
		}
	}
	// And the bar between them keeps its own: this is about the bars that
	// have a symbol, not about the numbering as a whole.
	if !got["3"] {
		t.Errorf("bar 3 has no symbol and lost its number too: %v", got)
	}
}

// A symbol written under the staff is nowhere near the number, so it says
// nothing about it.
func TestASymbolBelowTheStaffKeepsTheNumber(t *testing.T) {
	score := numbered(true, 1)
	for _, el := range score.Parts[0].Measures[1].Voices[0].Elements {
		if se, ok := el.(*model.SymbolElement); ok {
			se.Symbol.Placement = model.PlacementBelow
		}
	}
	if got := numbersDrawn(Engrave(score, DefaultOptions())); !got["2"] {
		t.Errorf("a symbol under the staff took the number away: %v", got)
	}
}

// And a symbol written on a note counts as much as one standing on its own:
// it is drawn in the same place and gets in the same way.
func TestASymbolOnANoteDropsTheNumberToo(t *testing.T) {
	score := numbered(true)
	note := score.Parts[0].Measures[1].Voices[0].Elements[0].(*model.Note)
	note.Symbols = []model.SymbolMark{{Name: "device"}}

	if got := numbersDrawn(Engrave(score, DefaultOptions())); got["2"] {
		t.Errorf("the number was printed over a symbol on a note: %v", got)
	}
}
