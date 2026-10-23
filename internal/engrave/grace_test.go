/*
 * grace_test.go
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
)

// graceScore is one bar with two grace notes in front of its second note.
func graceScore(slashed bool) *model.Score {
	note := func(step model.Step, oct int) *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: step, Octave: oct},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
	}
	main := note(model.StepE, 4)
	main.Grace = &model.GraceGroup{
		Notes:   []*model.Note{note(model.StepG, 4), note(model.StepA, 4)},
		Slashed: slashed,
	}
	v := &model.Voice{Elements: []model.Element{note(model.StepC, 4), main}}
	return &model.Score{Parts: []*model.Part{{
		Clef:     model.Treble,
		Time:     model.TimeSignature{Beats: 2, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
	}}}
}

func TestGraceNotesAreDrawn(t *testing.T) {
	// Parsed into the model but never drawn is the same loss as not read at
	// all, with the added insult of the file looking as though it opened.
	plain := Engrave(graceScore(false), DefaultOptions())

	heads := 0
	flags := 0
	for _, g := range plain.Glyphs {
		switch g.Name {
		case "noteheadBlack":
			if g.Size > 0 && g.Size < 1 {
				heads++
			}
		case "flag8thUp":
			flags++
		}
	}
	if heads != 2 {
		t.Errorf("got %d small noteheads, want 2", heads)
	}
	if flags != 2 {
		t.Errorf("got %d grace flags, want 2 — a grace note is written as an eighth", flags)
	}
}

func TestGraceNotesSitBeforeTheirNote(t *testing.T) {
	lay := Engrave(graceScore(false), DefaultOptions())

	// The note they lean on is the second full-size notehead in the bar.
	var mainX float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Size == 0 {
			mainX = g.X // the last full-size head is the one with the graces
		}
	}
	if mainX == 0 {
		t.Fatal("no full-size notehead was drawn")
	}
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Size > 0 && g.X >= mainX {
			t.Errorf("a grace note at x=%.2f is not before its note at x=%.2f", g.X, mainX)
		}
	}
}

func TestSlashIsDrawnOnlyForAnAcciaccatura(t *testing.T) {
	// The stroke through the stem is the only thing that tells a player to
	// crush the note in before the beat rather than take its written value
	// out of the note it precedes. Drawing it either way round is wrong.
	count := func(s *model.Score) int {
		lay := Engrave(s, DefaultOptions())
		n := 0
		for _, l := range lay.Lines {
			if l.X1 != l.X2 && l.Y1 != l.Y2 {
				n++ // a diagonal stroke: nothing else in this score has one
			}
		}
		return n
	}
	if got := count(graceScore(true)); got != 1 {
		t.Errorf("{/ge}: got %d strokes, want 1", got)
	}
	if got := count(graceScore(false)); got != 0 {
		t.Errorf("{ge}: got %d strokes, want none", got)
	}
}

func TestGraceNotesDoNotOverlapWhatComesBefore(t *testing.T) {
	// The room has to be budgeted in the layout, not taken out of the note
	// before: two grace notes drawn into the gap would print over it.
	lay := Engrave(graceScore(false), DefaultOptions())

	var firstRight, graceLeft float64
	graceLeft = 1e9
	for _, g := range lay.Glyphs {
		switch {
		case g.Name == "noteheadBlack" && g.Size == 0 && firstRight == 0:
			firstRight = g.X + 1.0 // a notehead is about one space wide
		case g.Name == "noteheadBlack" && g.Size > 0 && g.X < graceLeft:
			graceLeft = g.X
		}
	}
	if graceLeft < firstRight {
		t.Errorf("the first grace note is at x=%.2f, inside the note before it, which ends at x=%.2f",
			graceLeft, firstRight)
	}
}
