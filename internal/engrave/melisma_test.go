/*
 * melisma_test.go
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

/*
 * A syllable sung over several notes.
 *
 * "plui ____ e" in "La Mer": one syllable on four notes, and the line under
 * it runs to the word that follows. It used to stop at the second note,
 * because the first note's sound had ended there and nothing asked what
 * the second was sung on. A note with no word of its own is sung on the
 * syllable still running.
 */

func TestAMelismaRunsToTheNextWord(t *testing.T) {
	lay := Engrave(sung("plui_", "", "", "e"), DefaultOptions())
	lines := extenders(lay)
	if len(lines) != 1 {
		t.Fatalf("%d lines ruled under the words, want 1", len(lines))
	}
	line := lines[0]

	var heads []float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			heads = append(heads, g.X)
		}
	}
	if len(heads) != 4 {
		t.Fatalf("%d noteheads, want 4", len(heads))
	}
	var next Text
	for _, s := range syllables(lay) {
		if s.S == "e" {
			next = s
		}
	}
	if next.S == "" {
		t.Fatal("the word after the melisma is not on the page")
	}
	// Past the third note, which is sung on the syllable too…
	if line.X2 <= heads[2] {
		t.Errorf("the line stops at %.2f, before the third note at %.2f: "+
			"it ends where the first note's sound does", line.X2, heads[2])
	}
	// …and short of the word that follows.
	if right := (next.X - textWidth(next.S, lyricSize)/2) - line.X2; right <= 0 {
		t.Errorf("the line runs %.2f spaces into %q", -right, next.S)
	}
}

func TestAMelismaStopsAtARest(t *testing.T) {
	note := func(word string) *model.Note {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		if word != "" {
			n.Lyrics = []model.Lyric{{Text: word, Verse: 1, Extend: word == "ah"}}
		}
		return n
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					note("ah"), note(""),
					&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
					note("oh"),
				}}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
	lay := Engrave(s, DefaultOptions())
	lines := extenders(lay)
	if len(lines) != 1 {
		t.Fatalf("%d lines ruled under the words, want 1", len(lines))
	}
	var rest float64
	for _, g := range lay.Glyphs {
		if g.Name == "restQuarter" {
			rest = g.X
		}
	}
	if rest == 0 {
		t.Fatal("the rest is not on the page")
	}
	if lines[0].X2 >= rest {
		t.Errorf("the line runs to %.2f, past the rest at %.2f: a rest is "+
			"the end of the syllable", lines[0].X2, rest)
	}
}
