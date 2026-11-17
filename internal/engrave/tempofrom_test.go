/*
 * tempofrom_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// A tempo written as one beat equalling another is two notes with an equals
// sign between them and no number: "♩ = ♩." over the bar where 4/4 turns
// into 12/8. See model.Tempo.From.
func TestABeatEqualToAnotherIsDrawnAsTwoNotes(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tempoScore(5, &model.Tempo{
		BPM: 120, Unit: model.Duration{Num: 3, Den: 8},
		From: model.Duration{Num: 1, Den: 4},
	}), opt)

	// The number is the head mark's alone.
	if n := len(textsAt(lay, "= 120")); n != 1 {
		t.Errorf("\"= 120\" was drawn %d times, want once, at the head", n)
	}
	if n := len(textsAt(lay, "=")); n != 1 {
		t.Errorf("the equals sign between two beats was drawn %d times, want once", n)
	}

	glyphs := map[string]int{}
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "met") || g.Name == "augmentationDot" {
			glyphs[g.Name]++
		}
	}
	// The head mark's quarter, and the change's two: the quarter before
	// and the dotted quarter it equals.
	if glyphs["metNoteQuarterUp"] != 3 {
		t.Errorf("the beats are drawn as %v, want three quarters", glyphs)
	}
	if glyphs["augmentationDot"] != 1 {
		t.Errorf("the dot on the new beat was drawn %d times", glyphs["augmentationDot"])
	}
	// The two notes stand either side of the sign.
	var eq Text
	for _, tx := range lay.Texts {
		if tx.S == "=" {
			eq = tx
		}
	}
	if eq.S == "" {
		t.Fatal("no equals sign between the beats")
	}
	var left, right int
	for _, g := range lay.Glyphs {
		if g.Name != "metNoteQuarterUp" || g.Y != eq.Y {
			continue
		}
		if g.X < eq.X {
			left++
		} else {
			right++
		}
	}
	if left != 1 || right != 1 {
		t.Errorf("%d notes left of the sign and %d right of it, want one each", left, right)
	}

	// And the dot stands clear of the notehead it belongs to, where a
	// reader sees it: drawn hard against the head, "♩ = ♩." read as "♩ = ♩".
	var note, dot float64
	for _, g := range lay.Glyphs {
		if g.Y != eq.Y {
			continue
		}
		if g.Name == "metNoteQuarterUp" && g.X > eq.X {
			note = g.X
		}
		if g.Name == "augmentationDot" {
			dot = g.X
		}
	}
	if gap := dot - (note + smufl.Advance("metNoteQuarterUp")); gap < 0.25 {
		t.Errorf("the dot is %.2f spaces after its notehead, want air before it", gap)
	}
}
