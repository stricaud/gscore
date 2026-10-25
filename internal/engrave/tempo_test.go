/*
 * tempo_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A piece is not one speed. "Slower" at the bridge and a tempo at the last
// chorus are ordinary writing, and a score that keeps a single number for the
// whole piece cannot say either of them.

// tempoScore is a run of bars with a change of speed written over one of them.
func tempoScore(at int, t *model.Tempo) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 12; i++ {
		m := &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarSingle,
		}
		if i == at {
			m.Tempo = t
		}
		p.Measures = append(p.Measures, m)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func textsAt(lay *Layout, s string) []Text {
	var out []Text
	for _, t := range lay.Texts {
		if t.S == s {
			out = append(out, t)
		}
	}
	return out
}

func TestATempoChangeIsWrittenOverItsBar(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	opt.ShowMeasureNumbers = true
	lay := Engrave(tempoScore(5, &model.Tempo{
		BPM: 60, Unit: model.Duration{Num: 3, Den: 8}, Text: "Slower",
	}), opt)

	words := textsAt(lay, "Slower")
	if len(words) != 1 {
		t.Fatalf("the marking was drawn %d times, want once", len(words))
	}
	if len(textsAt(lay, "= 60")) != 1 {
		t.Error("the metronome number was not drawn")
	}

	// A dotted beat is drawn as the note and a dot, not as a different note:
	// 6/8 goes at so many dotted quarters a minute and saying "quarter" would
	// be half the speed.
	glyphs := map[string]int{}
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "met") {
			glyphs[g.Name]++
		}
	}
	if glyphs["metNoteQuarterUp"] < 2 {
		t.Errorf("the beat is drawn as %v; the head mark and the change both need one", glyphs)
	}
	if glyphs["metAugmentationDot"] != 1 {
		t.Errorf("the dot on the beat was drawn %d times", glyphs["metAugmentationDot"])
	}

	// It belongs over the bar it starts at, which is on a later system than
	// the mark at the head of the score.
	head := textsAt(lay, "= 120")
	if len(head) != 1 {
		t.Fatalf("the score's own tempo was drawn %d times", len(head))
	}
	if words[0].Y <= head[0].Y {
		t.Errorf("the change is at y=%.2f and the head mark at y=%.2f: "+
			"it was not drawn further down the page", words[0].Y, head[0].Y)
	}
}

// The bar number is written in the same row and at the same height, so
// anything else written over that bar has to start after it. It did not, and
// a tempo marking came out with the bar number inside its first letter.
func TestAMarkingOverABarClearsTheBarNumber(t *testing.T) {
	opt := DefaultOptions()
	// One long line, where every bar carries its number, so the bar the
	// marking is over certainly has one.
	opt.Mode = Strip
	opt.ShowMeasureNumbers = true
	lay := Engrave(tempoScore(4, &model.Tempo{
		BPM: 60, Unit: model.Duration{Num: 1, Den: 4}, Text: "Slower",
	}), opt)

	words := textsAt(lay, "Slower")
	if len(words) != 1 {
		t.Fatalf("the marking was drawn %d times", len(words))
	}
	// The number of the bar the marking is over. Bars are numbered from one.
	nums := textsAt(lay, "5")
	if len(nums) == 0 {
		t.Fatal("the bar carries no number, so this test proves nothing")
	}
	for _, n := range nums {
		// Within a staff space of each other is the same row as far as the
		// eye is concerned: the number sits at 1.4 above the staff and the
		// marking at 1.6, which is not a gap, it is an overlap.
		if math.Abs(n.Y-words[0].Y) > 1 {
			continue
		}
		if words[0].X <= n.X {
			t.Errorf("the marking starts at x=%.2f and the bar number at x=%.2f: "+
				"they are printed through each other", words[0].X, n.X)
		}
	}
}

// Nothing is written where nothing changes: a bar that carries on at the
// speed it was given does not get a marking of its own.
func TestABarThatDoesNotChangeSpeedCarriesNoMark(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tempoScore(99, nil), opt)

	if n := len(textsAt(lay, "= 120")); n != 1 {
		t.Errorf("the tempo was drawn %d times, want only the one at the head", n)
	}
}
