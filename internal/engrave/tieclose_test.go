/*
 * tieclose_test.go
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

// A tie that is never closed is closed on the note after it.
//
// Nothing stopped one before: an open tie is drawn to the margin at every
// line break and picked up again on the next system, so a tie whose other
// half is missing was drawn across the rest of the piece. Any file can say
// a note is held and not say into what — Finale leaves the mark on a note
// whose neighbour has changed pitch, and the MusicXML suite has a case of it
// on purpose — and the page then showed a curve sweeping over whole systems
// with the music underneath it.

// oneBar puts a line of notes in a single bar, so every curve in the layout
// belongs to them.
func tieTestBar(notes ...*model.Note) *model.Score {
	els := make([]model.Element, 0, len(notes))
	for _, n := range notes {
		els = append(els, n)
	}
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: len(notes), BeatValue: 4},
		Measures: []*model.Measure{{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarFinal,
		}},
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func tiedTestNote(step model.Step, octave int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: octave},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
}

func TestATieWithNoEndStopsAtTheNextNote(t *testing.T) {
	held := tiedTestNote(model.StepC, 4)
	held.TieStart = true
	score := tieTestBar(held, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepE, 4), tiedTestNote(model.StepF, 4))

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}

	// It has to end on the next note and not run on past the ones after it.
	hit := lay.Hits[0]
	if len(hit.Elements) < 2 {
		t.Fatal("the bar did not come out with its notes in it")
	}
	second, third := hit.Elements[1].X, hit.Elements[2].X
	if end := lay.Curves[0].X2; end > third {
		t.Errorf("the tie ends at %.2f, past the third note at %.2f", end, third)
	}
	if end := lay.Curves[0].X2; end < second {
		t.Errorf("the tie ends at %.2f, before the note it is tied to at %.2f", end, second)
	}
}

// And it stays a tie: one curve between two notes, not a line across the
// system.
func TestAnUnendedTieIsNotDrawnAcrossTheSystem(t *testing.T) {
	held := tiedTestNote(model.StepG, 4)
	held.TieStart = true
	var notes []*model.Note
	notes = append(notes, held)
	for i := 0; i < 15; i++ {
		notes = append(notes, tiedTestNote(model.StepA, 4))
	}
	score := tieTestBar(notes...)

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want one", len(lay.Curves))
	}
	width := lay.Curves[0].X2 - lay.Curves[0].X1
	if width > 4 {
		t.Errorf("the tie is %.2f spaces wide, which is a line across the music", width)
	}
}

// A tie that is closed is left exactly as it was.
func TestARealTieIsUntouched(t *testing.T) {
	first := tiedTestNote(model.StepC, 4)
	first.TieStart = true
	second := tiedTestNote(model.StepC, 4)
	second.TieStop = true
	score := tieTestBar(first, second, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepE, 4))

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}
	hit := lay.Hits[0]
	// From the first note to the second, which is what a tie is.
	if end, want := lay.Curves[0].X2, hit.Elements[2].X; end > want {
		t.Errorf("the tie ends at %.2f, past the note after the one it joins at %.2f", end, want)
	}
}

// A note held through several bars keeps its chain: each note stops the tie
// before it and starts the next, and none of them is cut short.
func TestATieChainIsUnbroken(t *testing.T) {
	var measures []*model.Measure
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}
		n.TieStart = i < 3
		n.TieStop = i > 0
		measures = append(measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		})
	}
	score := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: measures,
		}},
	}

	lay := Engrave(score, DefaultOptions())
	// Three ties join four notes.
	if len(lay.Curves) != 3 {
		t.Errorf("%d curves for a note held through four bars, want three", len(lay.Curves))
	}
}

// Every notehead of a chord is tied on its own account, and an unended one
// among them does not drag the others with it.
func TestAnUnendedTieInAChord(t *testing.T) {
	lower := tiedTestNote(model.StepC, 4)
	upper := tiedTestNote(model.StepE, 4)
	upper.TieStart = true
	chord := &model.Chord{
		Notes: []*model.Note{lower, upper},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{
			Voices: []*model.Voice{{Elements: []model.Element{
				chord, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepF, 4), tiedTestNote(model.StepG, 4),
			}}},
			RightBarline: model.BarFinal,
		}},
	}
	score := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves, want the one tie", len(lay.Curves))
	}
	if width := lay.Curves[0].X2 - lay.Curves[0].X1; width > 4 {
		t.Errorf("the chord's tie is %.2f spaces wide", width)
	}
}
