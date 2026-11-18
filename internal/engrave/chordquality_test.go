/*
 * chordquality_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The altered notes of a chord symbol are signs, like the root's accidental.
 *
 * "G#7b5(b9)" is typed with the letter b because that is what a keyboard
 * has. The root's sharp was already set as the sign; the flats before the 5
 * and the 9 were printed as the letter, so the symbol read as two words.
 * Asked for as "make sure the lowercase b is translated into a flat".
 */

package engrave

import (
	"math"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAlteredNotesOfAQualityAreSetAsSigns(t *testing.T) {
	pieces, _ := chordPieces(&model.ChordSymbol{Root: "G#", Quality: "7b5(b9)"})

	var glyphs []string
	var texts []string
	for _, p := range pieces {
		if p.glyph != "" {
			glyphs = append(glyphs, p.glyph)
		} else {
			texts = append(texts, p.text)
		}
	}
	// The root's sharp, then a flat before the 5 and one before the 9.
	if got := strings.Join(glyphs, " "); got != "accidentalSharp accidentalFlat accidentalFlat" {
		t.Errorf("the signs are %q, want the sharp of the root and two flats", got)
	}
	// And not one letter b left in the text between them.
	for _, s := range texts {
		if strings.ContainsAny(s, "b#") {
			t.Errorf("%q is still printed as a letter", s)
		}
	}
	if got := strings.Join(texts, "|"); got != "G|7|5(|9)" {
		t.Errorf("the text comes out as %q, want G|7|5(|9)", got)
	}
}

// The quality's signs are set to the quality's size, not the root's, and
// their ink finishes level with the top of the figures beside them.
func TestAQualitySignIsSetToTheQualitysSize(t *testing.T) {
	pieces, _ := chordPieces(&model.ChordSymbol{Root: "C", Quality: "m7b5"})
	var rootSign, qualitySign *chordPiece
	var figure *chordPiece
	for i := range pieces {
		p := &pieces[i]
		switch {
		case p.glyph != "":
			qualitySign = p
		case p.text == "m7":
			figure = p
		}
	}
	rootPieces, _ := chordPieces(&model.ChordSymbol{Root: "Bb"})
	for i := range rootPieces {
		if rootPieces[i].glyph != "" {
			rootSign = &rootPieces[i]
		}
	}
	if qualitySign == nil || rootSign == nil || figure == nil {
		t.Fatalf("pieces missing: %+v", pieces)
	}
	if qualitySign.size >= rootSign.size {
		t.Errorf("the quality's flat is set at %.2f, no smaller than the root's %.2f",
			qualitySign.size, rootSign.size)
	}
	top := figure.dy + chordCapHeight*figure.size
	ink := accidentalInk[qualitySign.glyph][0] * qualitySign.size / 4
	if reach := qualitySign.dy + ink; math.Abs(reach-top) > 0.001 {
		t.Errorf("the flat's ink reaches %.3f, want the figures' top %.3f", reach, top)
	}
}

// What is not a sign stays text: a b with no number after it, and the
// signs written as themselves are taken the same as the typed letters.
func TestOnlyASignBeforeANumberIsASign(t *testing.T) {
	for _, c := range []struct {
		quality string
		signs   int
	}{
		{"sus4", 0}, {"add9", 0}, {"maj7#11", 1}, {"7♭9", 1}, {"13(b9)", 1},
		{"7b5b9", 2}, {"m7b5", 1}, {"7#9#5", 2}, {"6/9", 0},
	} {
		n := 0
		for _, p := range qualityParts(c.quality) {
			if p.glyph != "" {
				n++
			}
		}
		if n != c.signs {
			t.Errorf("%q: %d signs, want %d", c.quality, n, c.signs)
		}
	}
}
