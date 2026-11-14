/*
 * flagaccidental_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * An accidental has to clear the flag before it.
 *
 * A flag hangs off the right of an up stem, past where the note's duration
 * says its ink stops, so the column is widened for it. An accidental is the
 * other way round: it is drawn backwards from its own notehead and is read
 * as part of that note, so it is tucked into whatever air the gap before it
 * already holds rather than charged to the page — otherwise a bar of four
 * equal notes with a sharp on the third prints the third late.
 *
 * The two met. The room booked for the flag was not counted as ink, so it
 * read as air, and the accidental was tucked straight back into it: bar 18
 * of the flute in this Ellington printed the third flat across the eighth
 * note's flag.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// flaggedThenAccidental is one bar: an eighth low enough to stem up, then a
// note carrying a flat. A quarter after the eighth rather than another
// eighth, because two eighths on one beat are beamed and a beamed note has
// no flag to collide with.
func flaggedThenAccidental() *model.Score {
	eighth := model.Duration{Num: 1, Den: 8}
	quarter := model.Duration{Num: 1, Den: 4}
	flat := model.Pitch{Step: model.StepB, Octave: 4, Alter: -1}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: quarter},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: eighth},
					&model.Note{Pitch: flat, Dur: quarter, Accidental: model.Flat},
					&model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: quarter},
					&model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: quarter},
					&model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: eighth},
				}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// span is a glyph's ink, left and right of where it is placed.
func span(t *testing.T, g Glyph) (float64, float64) {
	t.Helper()
	b, ok := smufl.BBox(g.Name)
	if !ok {
		t.Fatalf("%s is not measured", g.Name)
	}
	return g.X + b.Left, g.X + b.Right
}

func TestAnAccidentalClearsTheFlagBeforeIt(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(flaggedThenAccidental(), opt)

	var flag, acc *Glyph
	for i := range lay.Glyphs {
		switch {
		case has(lay.Glyphs[i].Name, "flag") && flag == nil:
			flag = &lay.Glyphs[i]
		case has(lay.Glyphs[i].Name, "accidental") && acc == nil:
			acc = &lay.Glyphs[i]
		}
	}
	if flag == nil {
		t.Fatal("the eighth was drawn with no flag")
	}
	if acc == nil {
		t.Fatal("the flat was not drawn")
	}
	_, flagRight := span(t, *flag)
	accLeft, _ := span(t, *acc)
	if accLeft < flagRight {
		t.Errorf("the flat starts at %.3f and the flag runs to %.3f: "+
			"they overlap by %.3f", accLeft, flagRight, flagRight-accLeft)
	}
	// And they are on the same staff, which is what makes the overlap a
	// collision rather than two marks on different lines of the score.
	if math.Abs(flag.Y-acc.Y) > 8 {
		t.Fatalf("the flag at y=%.2f and the flat at y=%.2f are nowhere "+
			"near each other; this test is measuring the wrong pair",
			flag.Y, acc.Y)
	}
}
