/*
 * midbarclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Which clef a notehead is drawn against, in a bar that changes clef partway
 * through.
 *
 * The columns of a bar are placed as the walk reaches them, so a clef written
 * inside it moves everything after it and nothing before. The noteheads were
 * drawn in a second pass afterwards, against whatever clef the bar had
 * finished in — so every note before the change was drawn against the clef
 * that comes after it, while its own stem was drawn against the right one.
 * Bar 149 of Danse Macabre came out as a stack of noteheads with a stem
 * hanging in the air a sixth above them.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestANoteheadIsDrawnAgainstTheClefItStandsUnder(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	treble := model.Treble

	// Two notes of the same pitch. The second carries a change of clef, so
	// the first belongs to the bass the bar opened in and the second to the
	// treble — twelve steps, six spaces, apart.
	first := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 3}, Dur: q}
	second := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 3}, Dur: q}
	second.Clef = &treble

	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Bass, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{Voices: []*model.Voice{{
				Elements: []model.Element{first, second, &model.Rest{Dur: q}, &model.Rest{Dur: q}},
			}}}},
		}},
	}

	lay := Engrave(s, DefaultOptions())
	var heads []Glyph
	for _, g := range lay.Glyphs {
		if len(g.Name) >= 8 && g.Name[:8] == "notehead" {
			heads = append(heads, g)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d noteheads drawn, want 2", len(heads))
	}
	// Six spaces apart: the same pitch read in the bass and in the treble.
	if gap := math.Abs(heads[0].Y - heads[1].Y); math.Abs(gap-6) > 0.01 {
		t.Errorf("the two notes are %.2f spaces apart, want 6 — the first was drawn "+
			"against the clef that only begins at the second", gap)
	}

	// And every stem reaches the notehead it grows out of.
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) > 0.01 || math.Abs(l.Y1-l.Y2) < 1 {
			continue
		}
		touches := false
		for _, h := range heads {
			if math.Abs(h.Y-l.Y1) < 0.7 || math.Abs(h.Y-l.Y2) < 0.7 {
				touches = true
			}
		}
		// Barlines run the height of the staff and touch nothing; a stem
		// that touches nothing is a stem drawn for a note somewhere else.
		if !touches && math.Abs(math.Abs(l.Y1-l.Y2)-4) > 0.01 {
			t.Errorf("a stem at y %.2f..%.2f reaches no notehead", l.Y1, l.Y2)
		}
	}
}
