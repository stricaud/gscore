/*
 * alteredunison_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two notes on one step of the staff, told apart by their accidentals.
 *
 * An F and an F sharp written in one chord stand on the same line. Drawn
 * where their step puts them they were one notehead with one sharp beside
 * it, and the page said a single note. They are set like a second: the
 * lower-pitched note on the left, the other across the stem, with each
 * sign before its own head and the signs in the same order as the heads.
 * Asked for from the MPA guide's rule (h), in the form every engraver has
 * used since: one straight stem, not two diagonals meeting.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// alteredUnison is a quarter-note chord of F natural and F sharp in the
// octave given, both accidentals written, followed by a quarter.
func alteredUnison(octave int) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	ch := &model.Chord{Dur: q, Notes: []*model.Note{
		{Pitch: model.Pitch{Step: model.StepF, Octave: octave}, Dur: q,
			Accidental: model.Natural},
		{Pitch: model.Pitch{Step: model.StepF, Octave: octave, Alter: 1}, Dur: q,
			Accidental: model.Sharp},
	}}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					ch,
					&model.Note{Pitch: pitch(model.StepA, octave), Dur: q},
					&model.Rest{Dur: model.Duration{Num: 1, Den: 2}},
				}}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
}

// xsOfGlyph is the x of every glyph of a kind, in drawing order.
func xsOfGlyph(lay *Layout, name string) []float64 {
	var out []float64
	for _, g := range lay.Glyphs {
		if g.Name == name {
			out = append(out, g.X)
		}
	}
	return out
}

func TestAnAlteredUnisonIsSetLikeASecond(t *testing.T) {
	// F4: stem up, the sharp steps right of the stem. F5: stem down, the
	// natural steps left. Either way the natural is the left-hand head.
	for _, octave := range []int{4, 5} {
		opt := DefaultOptions()
		opt.Mode = Strip
		lay := Engrave(alteredUnison(octave), opt)

		heads := xsOfGlyph(lay, "noteheadBlack")
		if len(heads) < 3 {
			t.Fatalf("octave %d: %d noteheads drawn, want the chord's two and the A", octave, len(heads))
		}
		// The chord's heads are drawn in the order its notes are listed:
		// the natural first, then the sharp.
		natural, sharp, next := heads[0], heads[1], heads[2]
		if natural >= sharp {
			t.Errorf("octave %d: the F natural is at %.2f and the F sharp at %.2f; want the lower note on the left, apart",
				octave, natural, sharp)
		}
		if d := sharp - natural; d < noteheadWidth(model.Quarter)-0.01 {
			t.Errorf("octave %d: the heads are %.2f apart, want a head's width %.2f",
				octave, d, noteheadWidth(model.Quarter))
		}
		// And the note after the chord stands clear of the stepped-out
		// head: the room was booked for it.
		if next < sharp+noteheadWidth(model.Quarter) {
			t.Errorf("octave %d: the next note at %.2f overlaps the stepped-out head at %.2f",
				octave, next, sharp)
		}

		// The signs stand before both heads, in the heads' own order:
		// natural, then sharp, then the chord.
		nat := xsOfGlyph(lay, "accidentalNatural")
		shp := xsOfGlyph(lay, "accidentalSharp")
		if len(nat) != 1 || len(shp) != 1 {
			t.Fatalf("octave %d: %d naturals and %d sharps drawn, want one each", octave, len(nat), len(shp))
		}
		if !(nat[0] < shp[0] && shp[0] < natural) {
			t.Errorf("octave %d: natural at %.2f, sharp at %.2f, heads from %.2f; want natural, sharp, heads",
				octave, nat[0], shp[0], natural)
		}
	}
}

// A unison doubled — the same pitch twice in one chord — is not two notes
// to tell apart, and its heads stay one on top of the other.
func TestADoubledUnisonStaysOneHead(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	ch := &model.Chord{Dur: q, Notes: []*model.Note{
		{Pitch: pitch(model.StepF, 4), Dur: q},
		{Pitch: pitch(model.StepF, 4), Dur: q},
	}}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{
				ch, &model.Rest{Dur: model.Duration{Num: 3, Den: 4}}}}}}}}},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	heads := xsOfGlyph(Engrave(s, opt), "noteheadBlack")
	if len(heads) != 2 || heads[0] != heads[1] {
		t.Errorf("a doubled unison came out at %v, want both heads in one place", heads)
	}
}
