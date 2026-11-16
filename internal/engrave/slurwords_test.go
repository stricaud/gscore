/*
 * slurwords_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * A slur is drawn between its notes, and the words are not in its way.
 *
 * Reported from "La Mer": every slur in it began under the lyrics and
 * climbed back up to the note it ended on. A slur under the notes was being
 * fitted against everything written in its first note's column, and the
 * syllable sung on that note is in that column — so the start was pushed
 * down to the lyric line, while the end, on a note with no word of its own,
 * stayed at its notehead.
 *
 * The words are set below the staff at a fixed drop, and nothing a slur
 * does moves them, so a slur has nothing to clear there. Both ends belong
 * beside their own noteheads.
 */

func TestASlurIsNotPushedDownByTheWordUnderItsFirstNote(t *testing.T) {
	// Two low notes slurred: stems up, so the curve goes below, which is
	// the side the words are on. The first carries a held syllable, with
	// the line that holds it on; the second carries nothing, as the second
	// note of a melisma does.
	lay, notes := phraseWithWords(t,
		[]model.Step{model.StepF, model.StepA}, []int{4, 4},
		[]model.Lyric{{Text: "la", Verse: 1, Extend: true}, {Text: "la", Verse: 2, Extend: true}})
	sl := theSlur(t, lay)

	var heads []float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			heads = append(heads, g.Y)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d noteheads drawn, want 2", len(heads))
	}
	first, last := heads[0], heads[1]
	if len(notes[0].Marks.Lyrics) != 2 {
		t.Fatal("the test's own first note lost its words")
	}
	// Each end hangs just under its own notehead. Half a notehead of air
	// plus the lift the curve is given is well under two spaces; the lyric
	// line is over six spaces below the staff.
	if d := sl.Y1 - first; d < 0 || d > 2 {
		t.Errorf("the slur begins %.2f below its first notehead; the words "+
			"under that note used to push it down to them", d)
	}
	if d := sl.Y2 - last; d < 0 || d > 2 {
		t.Errorf("the slur ends %.2f below its last notehead", d)
	}
	// And the two ends are level with each other's notes: the curve is
	// not a diagonal from the words up to a note.
	if math.Abs((sl.Y1-first)-(sl.Y2-last)) > 1 {
		t.Errorf("the slur's ends hang unequally under their notes: %.2f and %.2f",
			sl.Y1-first, sl.Y2-last)
	}
}

// phraseWithWords is phrase with syllables on the first note.
func phraseWithWords(t *testing.T, steps []model.Step, octaves []int, words []model.Lyric) (*Layout, []*model.Note) {
	t.Helper()
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	var notes []*model.Note
	for i := range steps {
		n := &model.Note{
			Pitch: model.Pitch{Step: steps[i], Octave: octaves[i]},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		notes = append(notes, n)
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	notes[0].Marks.SlurStart = 1
	notes[0].Marks.Lyrics = words
	notes[len(notes)-1].Marks.SlurStop = 1
	return Engrave(s, DefaultOptions()), notes
}
