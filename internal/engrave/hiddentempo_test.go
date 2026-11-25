/*
 * hiddentempo_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tempo marking kept off the page: nothing printed, grey in the editor so
 * that it can be clicked and shown again.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAHiddenTempoPrintsNothing(t *testing.T) {
	s := movements(4)
	s.Parts[0].Measures[2].Tempo = &model.Tempo{
		BPM: 97, Unit: model.Duration{Num: 1, Den: 4}, Hidden: true,
	}
	at := func(lay *Layout) bool {
		for _, tp := range lay.Tempos {
			if tp.Measure == 2 {
				return true
			}
		}
		return false
	}
	if lay := Engrave(s, DefaultOptions()); at(lay) {
		t.Error("a hidden tempo was printed")
	}
	opt := DefaultOptions()
	opt.ShowHidden = true
	lay := Engrave(s, opt)
	if !at(lay) {
		t.Fatal("the editor does not show the hidden tempo, so it cannot be clicked")
	}
	// The opening tempo is a crotchet too, and printed: one of each.
	grey, black := 0, 0
	for _, g := range lay.Glyphs {
		if g.Name == "metNoteQuarterUp" {
			if g.Colour == hiddenColour {
				grey++
			} else {
				black++
			}
		}
	}
	if grey != 1 || black != 1 {
		t.Errorf("%d grey and %d black metronome notes, want one of each", grey, black)
	}
}
