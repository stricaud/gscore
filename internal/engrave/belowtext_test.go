/*
 * belowtext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Words written under the staff.
 *
 * The ones above it join the queue and are placed clear of whatever that
 * stretch of music turned out to need. The ones below were written at a
 * fixed distance under the bottom line — three and a bit spaces — which is
 * clear of nothing. The Rhapsody opens on a clarinet low in its register,
 * so "Solo" and "gliss" came out through the ledger lines and the
 * noteheads they belong to.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// lowOn writes a word under a note far below the staff, and says where the
// word and the lowest ink of the music ended up.
func lowOn(t *testing.T, steps int) (word, music float64) {
	t.Helper()
	score := model.NewScore()
	v := score.Parts[0].Measures[0].Voices[0]
	n := &model.Note{
		Pitch: model.PitchFromDiatonicIndex(steps),
		Dur:   model.Duration{Num: 1, Den: 2},
	}
	n.Marks.Texts = []model.TextMark{{Text: "Solo", Style: model.TextExpression}}
	v.Elements = append(v.Elements, n)

	lay := Engrave(score, DefaultOptions())
	word = math.Inf(-1)
	for _, tx := range lay.Texts {
		if tx.S == "Solo" {
			word = tx.Y
		}
	}
	if math.IsInf(word, -1) {
		t.Fatal("the word was not written at all")
	}
	// The lowest the music reaches: noteheads and the ledger lines under
	// them.
	music = math.Inf(-1)
	for _, g := range lay.Glyphs {
		music = math.Max(music, g.Y)
	}
	for _, l := range lay.Lines {
		music = math.Max(music, math.Max(l.Y1, l.Y2))
	}
	return word, music
}

// The word is below everything, wherever the note is.
func TestAWordUnderTheStaffClearsTheMusic(t *testing.T) {
	for _, steps := range []int{0, -5, -9, -12} {
		word, music := lowOn(t, steps)
		if word <= music {
			t.Errorf("a note %d steps from middle C: the word is at %.2f "+
				"and the music reaches %.2f — it is written through it",
				steps, word, music)
		}
	}
}

// And it does not stand off at the worst case when the music is high: a
// word under an ordinary note stays near the staff.
func TestAWordUnderTheStaffStaysNearIt(t *testing.T) {
	word, _ := lowOn(t, 0)
	far, _ := lowOn(t, -12)
	if word >= far {
		t.Errorf("the word is at %.2f over a middle C and %.2f over a note "+
			"two octaves lower; it should be nearer the staff for the "+
			"higher note", word, far)
	}
}
