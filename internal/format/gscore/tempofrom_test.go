/*
 * tempofrom_test.go
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

// A tempo written as one beat equalling another keeps the beat on the left
// of the sign through a save and a load. See model.Tempo.From.
func TestABeatEqualToAnotherSurvivesTheFile(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{}},
		Tempo: &model.Tempo{
			BPM: 96, Unit: model.Duration{Num: 3, Den: 8},
			From: model.Duration{Num: 1, Den: 4},
		},
	})

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures[len(back.Parts[0].Measures)-1].Tempo
	if got == nil {
		t.Fatal("the change of speed did not come back")
	}
	if want := (model.Duration{Num: 1, Den: 4}); got.From != want {
		t.Errorf("the beat before came back as %s, want %s", got.From, want)
	}
	if want := (model.Duration{Num: 3, Den: 8}); got.Unit != want {
		t.Errorf("the new beat came back as %s, want %s", got.Unit, want)
	}
	// And an ordinary mark says nothing about it.
	if !back.Tempo.From.IsZero() {
		t.Errorf("the score's own tempo came back with a beat before it: %s", back.Tempo.From)
	}
	if bytes.Contains(buf.Bytes(), []byte(`"from":"0/`)) {
		t.Error("a mark with a number wrote out a beat before it")
	}
}
