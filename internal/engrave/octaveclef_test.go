/*
 * octaveclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The treble clef with an 8 under it.
 *
 * Every tenor line in every choral score is written in it, and so is every
 * guitar part. The notes on the page are a treble staff's notes — that is
 * the whole purpose of the sign — and the octave is in the sound, which is
 * where Transpose keeps it.
 *
 * Read as a plain treble clef the music is still drawn in the right places
 * and sung an octave too high; written at sounding pitch under the right
 * sign it is drawn an octave low, hanging off the bottom of the staff. Both
 * were wrong in different directions, which is why the positions and the
 * sign are tested together.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

func tenorLine(clef model.Clef) *model.Score {
	v := &model.Voice{Elements: []model.Element{
		&model.Note{
			Pitch: model.Pitch{Step: model.StepA, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		},
	}}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Name: "Tenor", Clef: clef,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
		}},
	}
}

func TestTheOctaveClefIsDrawnWithItsEight(t *testing.T) {
	name, _ := smufl.ClefGlyph(model.Treble8vb)
	if name != "gClef8vb" {
		t.Errorf("the clef is drawn as %q", name)
	}
	lay := Engrave(tenorLine(model.Treble8vb), DefaultOptions())
	found := false
	for _, g := range lay.Glyphs {
		if g.Name == "gClef8vb" {
			found = true
		}
		if g.Name == "gClef" {
			t.Error("a plain treble clef was drawn beside it")
		}
	}
	if !found {
		t.Error("the clef was not drawn")
	}
}

func TestTheOctaveClefWritesWhereATrebleClefWrites(t *testing.T) {
	// The same note on the same line. Anything else would be a different
	// staff, and the sign would be saying something about the writing that
	// it does not say.
	for _, p := range []model.Pitch{
		{Step: model.StepE, Octave: 4}, {Step: model.StepA, Octave: 4},
		{Step: model.StepF, Octave: 5},
	} {
		if got, want := StaffStep(model.Treble8vb, p),
			StaffStep(model.Treble, p); got != want {
			t.Errorf("%v%d sits at step %d under the octave clef and %d "+
				"under the treble", p.Step, p.Octave, got, want)
		}
	}
}

func TestTheNotesAreInTheSamePlaceAsATrebleStaff(t *testing.T) {
	heads := func(c model.Clef) []float64 {
		var out []float64
		for _, g := range Engrave(tenorLine(c), DefaultOptions()).Glyphs {
			if g.Name == "noteheadBlack" {
				out = append(out, g.Y)
			}
		}
		return out
	}
	plain, octave := heads(model.Treble), heads(model.Treble8vb)
	if len(plain) != 1 || len(octave) != 1 {
		t.Fatalf("%d noteheads and %d", len(plain), len(octave))
	}
	if plain[0] != octave[0] {
		t.Errorf("the note is at y=%.2f under the octave clef and y=%.2f "+
			"under the treble", octave[0], plain[0])
	}
}
