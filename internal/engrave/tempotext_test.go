/*
 * tempotext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The room over the staff, for the words written in it.
 *
 * Words are placed by their baselines and the room under a block of them
 * was measured to that baseline, so the tail of a g or a p hung into the
 * gap that was meant to keep the words clear of the music. "Allegro" was
 * placed over the notes with its g written through the top line of the
 * staff, and a "pizz." with the p through a notehead.
 *
 * A tempo marking stands further off than the rest besides: it is set
 * larger, it is read from across a desk, and the row above it carries the
 * metronome mark and the rehearsal letters. Reported as "when I add text
 * for tempo, it is written too close to the note and tempo".
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// over writes a word over a note high in the staff, and says how far the
// bottom of the letters ended up above the highest ink of the music.
func over(t *testing.T, word string, style model.TextStyle) float64 {
	t.Helper()
	score := model.NewScore()
	v := score.Parts[0].Measures[0].Voices[0]
	// A note at the top of the staff, so the words have something to
	// clear and the measurement is not of empty air.
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 5},
		Dur:   model.Duration{Num: 1, Den: 2},
	}
	n.Marks.Texts = []model.TextMark{{Text: word, Style: style}}
	v.Elements = append(v.Elements, n)

	// Without the mark at the head of the score: it is drawn in the title
	// block, above everything, and the highest ink on the page would then
	// be a metronome mark rather than the music these words are over.
	opt := DefaultOptions()
	opt.ShowTempoMark = false
	lay := Engrave(score, opt)

	baseline := math.Inf(1)
	size := 0.0
	for _, tx := range lay.Texts {
		if tx.S == word {
			baseline, size = tx.Y, tx.Size
		}
	}
	if math.IsInf(baseline, 1) {
		t.Fatalf("%q was not written at all", word)
	}
	// The highest the music reaches. Everything but the words themselves:
	// the noteheads, the stem, the staff lines and the clef.
	music := math.Inf(1)
	for _, g := range lay.Glyphs {
		music = math.Min(music, g.Y)
	}
	for _, l := range lay.Lines {
		music = math.Min(music, math.Min(l.Y1, l.Y2))
	}
	// y grows downwards, and the tail of a letter hangs below its
	// baseline: what has to clear the music is the bottom of the ink.
	return music - (baseline + size*textDescent)
}

func TestWordsOverTheStaffClearItByTheirTails(t *testing.T) {
	for _, c := range []struct {
		name  string
		word  string
		style model.TextStyle
		least float64
	}{
		// The tempo stands off furthest: it is read from across a desk
		// and the metronome mark and the letters are read with it.
		{"a tempo marking", "Allegro", model.TextTempo, systemTextGap},
		{"a direction", "To Coda", model.TextDirection, systemTextGap},
		// The markings that belong to one staff sit close in, where what
		// they are about is — but still clear of it.
		{"a technique marking", "pizz.", model.TextTechnique, staffTextGap},
		{"a word with no kind", "gliss. up", model.TextPlain, staffTextGap},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := over(t, c.word, c.style)
			if got < c.least-0.001 {
				t.Errorf("%q clears the music by %.2f spaces, want %.2f",
					c.word, got, c.least)
			}
			// And not so far off that it is floating: a marking belongs
			// to the music under it.
			if got > c.least+1.5 {
				t.Errorf("%q stands %.2f spaces off the music, which is a "+
					"marking nothing is under", c.word, got)
			}
		})
	}
}

// The tempo is the one that is set larger, so the one whose tail reaches
// furthest: measuring the room to the baseline gave it the least clearance
// of anything on the page and it was the first thing anybody saw.
func TestTheTempoStandsFurtherOffThanAMarkingOnOneStaff(t *testing.T) {
	tempo := over(t, "Allegro", model.TextTempo)
	staff := over(t, "pizz.", model.TextTechnique)
	if tempo <= staff {
		t.Errorf("the tempo clears the music by %.2f and a technique marking "+
			"by %.2f; the tempo is read from further off and should stand "+
			"further off", tempo, staff)
	}
}
