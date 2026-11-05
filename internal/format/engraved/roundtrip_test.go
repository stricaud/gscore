/*
 * roundtrip_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
	rpdf "github.com/stricaud/gscore/internal/render/pdf"
)

// gscore must be able to read what gscore writes.
//
// It could not, and the way it failed is worth keeping a test for. The PDF
// writer draws a staff line as a thin filled rectangle, so each line arrives
// at the reader as two edges a fraction apart. More than half of all the
// gaps between edges were therefore the thickness of a line rather than the
// distance between lines, the tolerance for "these two edges are one line"
// was derived from the median gap, and so it came out smaller than the thing
// it was meant to span. Nothing merged, no five lines were evenly spaced, and
// a perfectly good score came back as "no staves found — a scan rather than
// an engraving?".
//
// Rendering here rather than shipping a PDF keeps the test honest: it breaks
// if either half of the pair drifts, which is the whole point of a round
// trip.
func TestGscoreReadsItsOwnPDF(t *testing.T) {
	want := []struct {
		step   model.Step
		octave int
		value  model.NoteValue
	}{
		{model.StepC, 4, model.Quarter},
		{model.StepE, 4, model.Quarter},
		{model.StepG, 4, model.Half},
		{model.StepC, 5, model.Whole},
	}

	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for _, w := range want {
		m := model.NewMeasure()
		m.Voices[0].Elements = append(m.Voices[0].Elements, &model.Note{
			Pitch: model.Pitch{Step: w.step, Octave: w.octave},
			Dur:   w.value.Duration(),
		})
		p.Measures = append(p.Measures, m)
	}

	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	o, eo := rpdf.Fit(s, rpdf.DefaultOptions(), eo)
	var buf bytes.Buffer
	err := rpdf.RenderSections(&buf, []rpdf.Section{
		{Layout: engrave.Engrave(s, eo), Opt: o},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("gscore cannot read its own PDF: %v", err)
	}
	if len(got.Parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(got.Parts))
	}
	var notes []*model.Note
	for _, m := range got.Parts[0].Measures {
		for _, v := range m.Voices {
			for _, e := range v.Elements {
				if n, ok := e.(*model.Note); ok {
					notes = append(notes, n)
				}
			}
		}
	}
	if len(notes) != len(want) {
		t.Fatalf("read %d notes, want %d", len(notes), len(want))
	}
	for i, w := range want {
		if notes[i].Pitch.Step != w.step || notes[i].Pitch.Octave != w.octave {
			t.Errorf("note %d: read %s%d, want %s%d", i+1,
				notes[i].Pitch.Step, notes[i].Pitch.Octave, w.step, w.octave)
		}
		if d := notes[i].Dur.Reduced(); d != w.value.Duration().Reduced() {
			t.Errorf("note %d: read %d/%d, want %v", i+1, d.Num, d.Den, w.value)
		}
	}
}
