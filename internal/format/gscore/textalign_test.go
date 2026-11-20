/*
 * textalign_test.go
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

// Where along the bar a text is set survives a save.
func TestTheAlignmentOfWordsIsKept(t *testing.T) {
	n := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 1}}
	n.Texts = []model.TextMark{
		{Text: "Fine", Align: model.AlignRight},
		{Text: "Trio", Align: model.AlignCentre},
		{Text: "dolce"},
	}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{n}}}}}
	var buf bytes.Buffer
	if err := Write(&buf, &model.Score{Parts: []*model.Part{p}}, nil); err != nil {
		t.Fatal(err)
	}
	back, _, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	texts := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note).Texts
	want := []model.TextAlign{model.AlignRight, model.AlignCentre, model.AlignNote}
	for i, w := range want {
		if i >= len(texts) || texts[i].Align != w {
			t.Errorf("text %d came back aligned %v, want %v", i, texts[i].Align, w)
		}
	}
}
