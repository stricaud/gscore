/*
 * tab_test.go
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
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A part written as tablature is a line for each string and the fret to stop
// it at — the same music as the staff of pitches it could have been written
// on, drawn the way a guitarist reads it.

// guitar is standard tuning, written from the top line down.
func guitar() *model.Tablature {
	var strings []model.Pitch
	for _, m := range []int{64, 59, 55, 50, 45, 40} { // E4 B3 G3 D3 A2 E2
		strings = append(strings, midiPitch(m))
	}
	return &model.Tablature{Strings: strings, Frets: 24}
}

// midiPitch spells a MIDI number, sharps where it must.
func midiPitch(m int) model.Pitch {
	steps := []model.Step{model.StepC, model.StepC, model.StepD, model.StepD,
		model.StepE, model.StepF, model.StepF, model.StepG, model.StepG,
		model.StepA, model.StepA, model.StepB}
	alter := []int{0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0}
	return model.Pitch{Step: steps[m%12], Octave: m/12 - 1, Alter: alter[m%12]}
}

func tabScore(notes []*model.Note) *model.Score {
	var els []model.Element
	for _, n := range notes {
		els = append(els, n)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Name: "Guitar", Clef: model.Treble, Tab: guitar(),
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func quarter(step model.Step, oct, str, fret int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: oct},
		Dur:   model.Duration{Num: 1, Den: 4}, String: str, Fret: fret,
	}
}

func TestATabStaffHasALineForEachString(t *testing.T) {
	lay := Engrave(tabScore([]*model.Note{quarter(model.StepE, 4, 1, 0)}),
		DefaultOptions())

	// The staff lines are the level ones, and a six-string tab has six of
	// them at distinct heights.
	levels := map[float64]bool{}
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) < 0.01 && l.X2-l.X1 > 4 {
			levels[math.Round(l.Y1*100)/100] = true
		}
	}
	if len(levels) != 6 {
		t.Errorf("the tab was ruled with %d lines, want one per string",
			len(levels))
	}
}

// The fret is written on its own string's line, and it is the number the
// file recorded rather than one worked out afresh: which string to play a
// note on is a decision somebody made.
func TestTheFretIsWrittenOnItsString(t *testing.T) {
	// A4 played at the seventh fret of the D string, which is the fourth
	// line down — not the open A string, which is where a bare pitch would
	// have put it.
	lay := Engrave(tabScore([]*model.Note{quarter(model.StepA, 4, 3, 7)}),
		DefaultOptions())

	var got *Text
	for i, tx := range lay.Texts {
		if tx.S == "7" {
			got = &lay.Texts[i]
		}
	}
	if got == nil {
		t.Fatal("the fret number was not written")
	}
	// String 3 of 6: two lines below the top one. The figure is set from
	// its baseline, which sits a little under the line it is on.
	top := lay.Hits[0].StaffTop
	if want := top + float64(2*tabLineGap); math.Abs(fretLine(got)-want) > 0.01 {
		t.Errorf("the 7 sits on the line at y=%.2f; the third string is at "+
			"%.2f", fretLine(got), want)
	}
	if got.Ref == 0 {
		t.Error("the number does not say which note it is, so nothing can " +
			"select it or light it up while it plays")
	}
}

// Where nothing was recorded, the fret is worked out: the highest string
// that reaches the note, which is the lowest fret that plays it.
func TestAnUnplacedNoteFindsItsFret(t *testing.T) {
	lay := Engrave(tabScore([]*model.Note{
		{Pitch: model.Pitch{Step: model.StepB, Octave: 3},
			Dur: model.Duration{Num: 1, Den: 4}},
	}), DefaultOptions())

	found := false
	for _, tx := range lay.Texts {
		if tx.S == "0" {
			found = true // B3 is the open second string
		}
	}
	if !found {
		t.Error("B3 was not written as the open second string")
	}
}

// What a tab does not have: no clef, no key signature, and no stems. The
// word TAB stands at the head instead.
func TestATabHasNoClefKeyOrStems(t *testing.T) {
	s := tabScore([]*model.Note{quarter(model.StepE, 4, 1, 0)})
	s.Parts[0].Key = model.KeySignature{Fifths: -3}
	lay := Engrave(s, DefaultOptions())

	for _, g := range lay.Glyphs {
		switch {
		case has(g.Name, "gClef"), has(g.Name, "fClef"):
			t.Errorf("a tab staff was given a %s", g.Name)
		case g.Name == "accidentalFlat":
			t.Error("a tab staff was given a key signature")
		case has(g.Name, "notehead"):
			t.Errorf("a tab staff was given a notehead (%s)", g.Name)
		}
	}
	if _, ok := glyphNamed(lay, "6stringTabClef"); !ok {
		t.Error("the word TAB is not at the head of the staff")
	}
}

// A tab is otherwise an ordinary staff: the notes have stems, so the rhythm
// can be read from it. Without them a tab-only part says which frets and
// never says when.
func TestATabKeepsItsStems(t *testing.T) {
	lay := Engrave(tabScore([]*model.Note{
		quarter(model.StepE, 4, 1, 0), quarter(model.StepB, 3, 2, 0),
	}), DefaultOptions())

	stems := 0
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) < 0.01 && math.Abs(l.Y2-l.Y1) > 1 &&
			math.Abs(l.Y2-l.Y1) < partH(&model.Part{Tab: guitar()})+2 {
			stems++
		}
	}
	if stems < 2 {
		t.Errorf("%d stems were drawn for two notes", stems)
	}
}

// The staff lines are ruled around the numbers rather than through them:
// tab is read as figures before it is read as lines.
func TestTheLinesAreBrokenBehindTheNumbers(t *testing.T) {
	lay := Engrave(tabScore([]*model.Note{
		quarter(model.StepE, 4, 1, 0), quarter(model.StepB, 3, 2, 12),
	}), DefaultOptions())

	var fret *Text
	for i, tx := range lay.Texts {
		if tx.S == "12" {
			fret = &lay.Texts[i]
		}
	}
	if fret == nil {
		t.Fatal("the 12 was not written")
	}
	line := fretLine(fret)
	on := 0
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) > 0.01 || math.Abs(l.Y1-line) > 0.01 {
			continue
		}
		on++
		if l.X1 < fret.X-0.1 && l.X2 > fret.X+0.1 {
			t.Errorf("a staff line runs from %.2f to %.2f, straight through "+
				"the 12 at %.2f", l.X1, l.X2, fret.X)
		}
	}
	if on < 2 {
		t.Errorf("the line the 12 sits on was drawn in %d pieces; a gap for "+
			"the figure makes two", on)
	}
}

// fretLine is the staff line a fret number sits on: the figure is set from
// its baseline, a little under the line.
func fretLine(t *Text) float64 { return t.Y - tabFretSize*0.36 }
