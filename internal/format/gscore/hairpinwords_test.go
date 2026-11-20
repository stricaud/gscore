/*
 * hairpinwords_test.go
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

// A hairpin written as the word stays the word through a save.
func TestAHairpinInWordsIsKept(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	a := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q}
	b := &model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: q}
	a.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true, Words: true, Dashes: 3}
	b.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true, Words: true, Dashes: -1}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 2, BeatValue: 4}}
	p.Measures = []*model.Measure{{Voices: []*model.Voice{{Elements: []model.Element{a, b}}}}}
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
	for i, el := range els {
		n := el.(*model.Note)
		if n.Hairpin == nil || !n.Hairpin.Words {
			t.Errorf("note %d came back with %+v, want the hairpin in words", i, n.Hairpin)
		}
	}
	// And the line's length with it: three dashes, and none.
	if d := els[0].(*model.Note).Hairpin.Dashes; d != 3 {
		t.Errorf("the start came back with %d dashes, want 3", d)
	}
	if d := els[1].(*model.Note).Hairpin.Dashes; d != -1 {
		t.Errorf("the end came back with %d dashes, want -1 (none)", d)
	}
}
