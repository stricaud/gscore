/*
 * chordtie_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tie on one note of a chord, and the way it was turned, survive a save.
 *
 * Reported as "I put this tie to up, I save, I open again and it is down
 * like it was before": the chord's notes were written with their pitches
 * and nothing of their own ties.
 */

package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestATieOnOneNoteOfAChordIsKept(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	note := func(step model.Step, oct int) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: q}
	}
	ch := &model.Chord{Dur: q, Notes: []*model.Note{
		note(model.StepC, 4), note(model.StepE, 4), note(model.StepG, 4), note(model.StepB, 4),
	}}
	// The second from the top held and its tie turned up; the bottom one
	// held too, left to the engraver; the rest free.
	ch.Notes[2].TieStart, ch.Notes[2].TieDir = true, model.StemUp
	ch.Notes[0].TieStart = true
	next := &model.Chord{Dur: q, Notes: []*model.Note{
		note(model.StepC, 4), note(model.StepE, 4), note(model.StepG, 4), note(model.StepB, 4),
	}}
	next.Notes[2].TieStop = true
	next.Notes[0].TieStop = true

	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{ch, next}}}}}
	s := &model.Score{Parts: []*model.Part{p}}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	els := back.Parts[0].Measures[0].Voices[0].Elements
	got, ok := els[0].(*model.Chord)
	if !ok || len(got.Notes) != 4 {
		t.Fatalf("the chord came back as %#v", els[0])
	}
	if !got.Notes[2].TieStart || got.Notes[2].TieDir != model.StemUp {
		t.Errorf("the G came back tied %v, turned %v; want tied and up", got.Notes[2].TieStart, got.Notes[2].TieDir)
	}
	if !got.Notes[0].TieStart || got.Notes[0].TieDir != model.StemAuto {
		t.Errorf("the C came back tied %v, turned %v; want tied and left to the engraver", got.Notes[0].TieStart, got.Notes[0].TieDir)
	}
	if got.Notes[1].TieStart || got.Notes[3].TieStart {
		t.Error("a note that was not tied came back tied")
	}
	if got.TieStart {
		t.Error("the chord itself came back tied, when only two of its notes were")
	}
	after := els[1].(*model.Chord)
	if !after.Notes[2].TieStop || !after.Notes[0].TieStop || after.Notes[1].TieStop {
		t.Errorf("the ties end on the wrong notes: %v %v %v", after.Notes[0].TieStop, after.Notes[1].TieStop, after.Notes[2].TieStop)
	}
}
