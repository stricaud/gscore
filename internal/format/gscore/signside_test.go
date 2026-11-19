/*
 * signside_test.go
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

// Which end of its bar a sign is written at survives a save.
func TestTheSideOfASignIsKept(t *testing.T) {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 2; i++ {
		p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{}}})
	}
	p.Measures[0].Sign = model.ToCoda
	p.Measures[0].SignAtEnd = true
	p.Measures[1].Sign = model.Segno
	s := &model.Score{Parts: []*model.Part{p}}

	var buf bytes.Buffer
	if err := Write(&buf, s, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := back.Parts[0].Measures
	if got[0].Sign != model.ToCoda || !got[0].SignAtEnd {
		t.Errorf("the To Coda came back as %v at the end %v", got[0].Sign, got[0].SignAtEnd)
	}
	if got[1].Sign != model.Segno || got[1].SignAtEnd {
		t.Errorf("the segno came back as %v at the end %v", got[1].Sign, got[1].SignAtEnd)
	}
}
