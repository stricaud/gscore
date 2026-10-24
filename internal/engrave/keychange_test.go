/*
 * keychange_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// A key change is two things, and has to look like two.
//
// The naturals cancel the old key and the accidentals state the new one. The
// accidentals of a signature are set close enough to read as one block, and
// set that close to each other the two blocks read as one block of mixed
// signs — which at a glance is a key signature nobody can name.

// keyChangeScore is a bar in one key followed by a bar in another.
func keyChangeScore(from, to int) *model.Score {
	bar := func(k *model.KeySignature) *model.Measure {
		return &model.Measure{
			Key: k,
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarSingle,
		}
	}
	first := model.KeySignature{Fifths: from}
	second := model.KeySignature{Fifths: to}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Key: first,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{bar(nil), bar(&second)},
		}},
	}
}

func TestACancellationIsSetApartFromTheNewKey(t *testing.T) {
	// Two sharps to three flats: everything in the old key is cancelled and
	// a new one stated, which is the case that crowds.
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(keyChangeScore(2, -3), opt)

	var lastNatural, firstFlat *Glyph
	for i, g := range lay.Glyphs {
		switch g.Name {
		case "accidentalNatural":
			if lastNatural == nil || g.X > lastNatural.X {
				lastNatural = &lay.Glyphs[i]
			}
		case "accidentalFlat":
			if firstFlat == nil || g.X < firstFlat.X {
				firstFlat = &lay.Glyphs[i]
			}
		}
	}
	if lastNatural == nil {
		t.Fatal("the old key was not cancelled, so this proves nothing")
	}
	if firstFlat == nil {
		t.Fatal("the new key was not written")
	}

	// The air between the last natural and the first flat: where the natural
	// stops, to where the flat starts.
	gap := firstFlat.X - (lastNatural.X + smufl.Advance("accidentalNatural"))
	if gap < 0.5 {
		t.Errorf("the cancellation ends at %.2f and the new key starts at "+
			"%.2f, leaving %.2f between them: the two blocks run together",
			lastNatural.X+smufl.Advance("accidentalNatural"), firstFlat.X, gap)
	}

	// And the accidentals within each block stay close, or the signature
	// stops reading as one thing.
	var flats []float64
	for _, g := range lay.Glyphs {
		if g.Name == "accidentalFlat" {
			flats = append(flats, g.X)
		}
	}
	if len(flats) < 2 {
		t.Fatalf("%d flats were drawn for a key of three", len(flats))
	}
	for i := 1; i < len(flats); i++ {
		if d := flats[i] - flats[i-1]; d > smufl.Advance("accidentalFlat")+0.4 {
			t.Errorf("two flats of the same signature are %.2f apart: the "+
				"block has been spread out as well", d)
		}
	}
}

// Nothing is set apart where there is nothing to set apart from: a key
// change with no cancelling, or a cancellation with no new key.
func TestAKeyChangeWithoutBothBlocksIsNotPaddedOut(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip

	// Nothing to cancel: C major to three flats.
	lay := Engrave(keyChangeScore(0, -3), opt)
	for _, g := range lay.Glyphs {
		if g.Name == "accidentalNatural" {
			t.Error("naturals were written where there was no key to cancel")
		}
	}
	var flats []float64
	for _, g := range lay.Glyphs {
		if g.Name == "accidentalFlat" {
			flats = append(flats, g.X)
		}
	}
	if len(flats) != 3 {
		t.Fatalf("%d flats for a key of three", len(flats))
	}
	for i := 1; i < len(flats); i++ {
		if d := flats[i] - flats[i-1]; d > smufl.Advance("accidentalFlat")+0.4 {
			t.Errorf("the new key was spread out: %.2f between two flats", d)
		}
	}
}
