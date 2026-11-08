/*
 * rehearsal_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The rehearsal marks, saved and opened again.
 *
 * A mark in the sequence is kept as a place and not as the letter, so that
 * putting one in before the others moves every letter after it along by
 * itself. The letter is derived; the bar holds nothing but "this is one of
 * the places". The file wrote down the letter — which for those bars is the
 * empty string — and so every mark made as "the next letter" was gone the
 * next time the score was opened. Reported as "I added a rehearsal mark, I
 * save, I open, my rehearsal marks disappeared".
 */

package gscore

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// marked is a score of a few bars with marks of both kinds on it.
func marked(t *testing.T) *model.Score {
	t.Helper()
	s := model.NewScore()
	p := s.Parts[0]
	for len(p.Measures) < 6 {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{}},
		})
	}
	// Two places in the sequence, and one mark written out as itself.
	s.SetMarker(1, "", true)
	s.SetMarker(3, "Chorus", false)
	s.SetMarker(4, "", true)
	return s
}

func reopen(t *testing.T, s *model.Score) *model.Score {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func TestTheRehearsalMarksAreStillThereWhenTheScoreIsOpenedAgain(t *testing.T) {
	back := reopen(t, marked(t))

	marks := back.RehearsalMarks(nil)
	want := map[int]string{1: "A", 3: "Chorus", 4: "B"}
	for bar, name := range want {
		if marks[bar] != name {
			t.Errorf("bar %d says %q, want %q", bar+1, marks[bar], name)
		}
	}
	if len(marks) != len(want) {
		t.Errorf("the score came back with %d marks, want %d: %v",
			len(marks), len(want), marks)
	}

	// And as places rather than as letters, which is what keeps them in
	// order: a mark put in before them has to rename them.
	if !back.Parts[0].Measures[1].MarkerAuto {
		t.Fatal("the mark came back written out as itself, not as a place " +
			"in the sequence")
	}
	back.SetMarker(0, "", true)
	marks = back.RehearsalMarks(nil)
	if marks[0] != "A" || marks[1] != "B" || marks[4] != "C" {
		t.Errorf("after a mark was put in front of them the sequence reads "+
			"%v, want A at bar 1, B at bar 2 and C at bar 5", marks)
	}
}

// What the marks are called is a decision about the whole score, and it is
// the companion of the bars that say they are places in the sequence: kept
// for them and lost with them, a score called by its bar numbers opened as
// A, B, C.
func TestHowTheMarksAreNamedIsKeptWithThem(t *testing.T) {
	for _, style := range []model.MarkerStyle{
		model.MarkerLetters, model.MarkerNumbers, model.MarkerBarNumbers,
	} {
		s := marked(t)
		s.Markers = style
		back := reopen(t, s)
		if back.Markers != style {
			t.Errorf("a score named by %v came back named by %v",
				style, back.Markers)
		}
	}

	// Read by the name in the file, so the numbers are the bars' own.
	s := marked(t)
	s.Markers = model.MarkerBarNumbers
	marks := reopen(t, s).RehearsalMarks(nil)
	if marks[4] != "5" {
		t.Errorf("the mark at bar 5 is called %q, want 5", marks[4])
	}
}

// A file written before any of this existed has no such word in it, and
// opens as a score whose marks are lettered and whose bars are not places.
func TestAFileThatSaysNothingAboutTheSequenceStillOpens(t *testing.T) {
	s := model.NewScore()
	s.Parts[0].Measures[0].Marker = "A"
	back := reopen(t, s)
	if back.Markers != model.MarkerLetters {
		t.Errorf("the marks are named by %v, want letters", back.Markers)
	}
	if back.Parts[0].Measures[0].MarkerAuto {
		t.Error("a mark written out as itself came back in the sequence")
	}
	if got := back.RehearsalMarks(nil)[0]; got != "A" {
		t.Errorf("the mark says %q, want A", got)
	}
}
