/*
 * viewstate_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// How a score was last being looked at travels in the file with it.
//
// The point of the feature is that it survives being written to disk and
// read back, so that is what these check — through the real writer and the
// real reader, not through the struct.

func viewScore() *model.Score {
	p := &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 4},
			},
		}}},
		RightBarline: model.BarSingle,
	})
	return &model.Score{
		Identity: model.Identity{Title: "A view"},
		Tempo:    model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
}

// roundTrip writes a score and reads it back, as the editor does.
func roundTrip(t *testing.T, s *model.Score) *model.Score {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatalf("writing: %v", err)
	}
	got, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	return got
}

func TestTheViewSurvivesTheFile(t *testing.T) {
	s := viewScore()
	s.View = &model.ViewState{
		Paginated:      true,
		StaffNames:     true,
		MeasureNumbers: true,
		MultiRests:     true,
		TempoMark:      true,
		BeatMarks:      true,
		ConcertPitch:   true,
		ColourNotes:    true,
		NoteColours: map[string]string{
			"C": "#d0312d", "D": "#e8701a", "E": "#c9a227",
			"F": "#3f9142", "G": "#2a72b5", "A": "#4b3f8f", "B": "#9c3f8f",
		},
	}

	got := roundTrip(t, s)
	if got.View == nil {
		t.Fatal("the view did not survive the file at all")
	}
	if !got.View.Paginated || !got.View.StaffNames ||
		!got.View.MeasureNumbers || !got.View.MultiRests ||
		!got.View.TempoMark || !got.View.BeatMarks ||
		!got.View.ConcertPitch || !got.View.ColourNotes {
		t.Errorf("an option was lost: %+v", *got.View)
	}
	if len(got.View.NoteColours) != 7 {
		t.Fatalf("%d colours came back, want 7", len(got.View.NoteColours))
	}
	if got.View.NoteColours["C"] != "#d0312d" {
		t.Errorf("C came back %q", got.View.NoteColours["C"])
	}
}

// Every option off is a thing somebody chose, and is not the same as a file
// that has never been asked. The first has to come back as itself.
func TestEveryOptionOffIsNotNothing(t *testing.T) {
	s := viewScore()
	s.View = &model.ViewState{} // all false, no colours

	got := roundTrip(t, s)
	if got.View == nil {
		t.Fatal("a view with everything off came back as no view at all")
	}
	if got.View.MeasureNumbers {
		t.Error("an option that was off came back on")
	}
}

// A score that has never been asked carries nothing, so that every file
// written before this existed goes on being drawn however the reader's own
// settings say. Reading those as "everything off" would turn the bar numbers
// off on every score anybody already has.
func TestAScoreThatSaysNothingCarriesNothing(t *testing.T) {
	got := roundTrip(t, viewScore())
	if got.View != nil {
		t.Errorf("a score with no view came back with one: %+v", *got.View)
	}

	var buf bytes.Buffer
	if err := Write(&buf, viewScore(), nil); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte(`"view"`)) {
		t.Error("the file names a view it was never given")
	}
}

// The palette is copied on the way in and on the way out. A map shared with
// the editor is a map the editor can change under the encoder, and a map
// shared with the reader is one the score can change under the file it came
// from.
func TestThePaletteIsNotSharedWithTheFile(t *testing.T) {
	s := viewScore()
	cols := map[string]string{"C": "#d0312d"}
	s.View = &model.ViewState{ColourNotes: true, NoteColours: cols}

	got := roundTrip(t, s)
	cols["C"] = "#000000" // the editor changes its palette afterwards
	if got.View.NoteColours["C"] != "#d0312d" {
		t.Errorf("the score that came back followed a map it does not own: %q",
			got.View.NoteColours["C"])
	}
}
