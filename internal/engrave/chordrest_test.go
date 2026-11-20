/*
 * chordrest_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A chord over a bar of rest is written at the head of the bar.
 *
 * The rest itself is moved to the middle of the bar, which is how a bar of
 * rest is engraved. The chord hung on it went to the middle with it, and a
 * run of three chords over the bar began halfway along and ran out through
 * the barline into the next bar — reported as "why does it render like in
 * the second screenshot", against a printed copy with the three chords set
 * from the start of the bar.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAChordOverABarOfRestStandsAtItsHead(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 3; i++ {
		var els []model.Element
		for k := 0; k < 4; k++ {
			els = append(els, &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	r := &model.Rest{Dur: model.Duration{Num: 1, Den: 1}}
	r.Marks.Chord = &model.ChordSymbol{Root: "G#", Quality: "7#5(b9) G#7b5(b9) C#7"}
	p.Measures[1].Voices[0].Elements = []model.Element{r}
	lay := Engrave(&model.Score{Parts: []*model.Part{p}}, DefaultOptions())

	if len(lay.Chords) != 1 {
		t.Fatalf("%d chords drawn, want 1", len(lay.Chords))
	}
	c := lay.Chords[0]
	var bar *StripPos
	for i := range lay.Strips {
		if lay.Strips[i].Measure == 1 {
			bar = &lay.Strips[i]
		}
	}
	if bar == nil {
		t.Fatal("bar 2 has no band")
	}
	// Where the first note of the bar would be: inside the first few
	// spaces of it, well before the middle.
	if mid := (bar.X0 + bar.X1) / 2; c.X >= mid-4 {
		t.Errorf("the chord starts at %.2f in a bar from %.2f to %.2f: over the rest, not at the head", c.X, bar.X0, bar.X1)
	}
	if c.X < bar.X0 {
		t.Errorf("the chord starts at %.2f, before its bar at %.2f", c.X, bar.X0)
	}
	// And the bar is wide enough for it.
	if end := c.X + c.Width; end > bar.X1-chordSymbolGap+0.01 {
		t.Errorf("the chord ends at %.2f and its bar at %.2f", end, bar.X1)
	}
	// The rest stayed in the middle.
	for _, g := range lay.Glyphs {
		if g.Name == "restWhole" && (g.X < bar.X0+(bar.X1-bar.X0)/3 || g.X > bar.X1-(bar.X1-bar.X0)/3) {
			t.Errorf("the rest is at %.2f, not in the middle of %.2f..%.2f", g.X, bar.X0, bar.X1)
		}
	}
}
