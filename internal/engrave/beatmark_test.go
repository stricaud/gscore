/*
 * beatmark_test.go
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
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// The row of beat marks. What matters about it is not that arrows appear but
// where they appear: one per beat, over the note that falls on that beat, and
// level with each other across the system.

// beatBars builds one staff in a given meter, each bar holding the elements a
// builder hands back.
func beatBars(t model.TimeSignature, bars int, fill func(i int) []model.Element) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: t}
	for i := 0; i < bars; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: fill(i)}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func quarters(n int) []model.Element {
	var out []model.Element
	for i := 0; i < n; i++ {
		out = append(out, &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	return out
}

// beatMarksOf is every beat arrow in a drawing, left to right.
func beatMarksOf(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if g.Name == beatMarkGlyph {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].X < out[j].X })
	return out
}

func beatOptions() Options {
	opt := DefaultOptions()
	opt.ShowBeatMarks = true
	opt.WriteAhead = false
	return opt
}

func TestNoBeatMarksUnlessAskedFor(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 2,
		func(int) []model.Element { return quarters(4) })

	opt := DefaultOptions()
	opt.WriteAhead = false
	if got := beatMarksOf(Engrave(s, opt)); len(got) != 0 {
		t.Fatalf("%d beat marks drawn with the option off, want none", len(got))
	}
}

func TestOneBeatMarkPerBeat(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 3,
		func(int) []model.Element { return quarters(4) })

	got := beatMarksOf(Engrave(s, beatOptions()))
	if len(got) != 12 {
		t.Fatalf("%d beat marks over three bars of 4/4, want 12", len(got))
	}
	// Drawn in their own colour, so nobody reads them as notation.
	for _, g := range got {
		if g.Colour == "" {
			t.Fatal("a beat mark was drawn in the ink of the score")
		}
	}
}

// The arrow has to be over the note, not merely over the bar. A mark a
// notehead's width to the side of the beat it names is pointing at the note
// after it.
func TestABeatMarkSitsOverTheNoteOnThatBeat(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 1,
		func(int) []model.Element { return quarters(4) })

	lay := Engrave(s, beatOptions())
	marks := beatMarksOf(lay)
	if len(marks) != 4 {
		t.Fatalf("%d beat marks in a bar of four quarters, want 4", len(marks))
	}

	// Where the noteheads went, left to right.
	var heads []float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			heads = append(heads, g.X)
		}
	}
	sort.Float64s(heads)
	if len(heads) != 4 {
		t.Fatalf("%d noteheads drawn, want 4", len(heads))
	}

	half := beatMarkHalf()
	for i := range marks {
		// The arrow is centred on the beat and the notehead is advanced from
		// its left edge, so they are compared at their centres.
		head := heads[i] + smufl.Advance("noteheadBlack")/2
		if d := math.Abs((marks[i].X + half) - head); d > 0.6 {
			t.Errorf("beat %d: arrow at %.2f, notehead at %.2f, %.2f spaces apart",
				i+1, marks[i].X+half, head, d)
		}
	}
}

// A level row. Placed one by one, every arrow would find its own height
// against whatever is under it and a run climbing the staff would leave the
// row stepping up the page behind it.
func TestTheBeatMarksMakeALevelRow(t *testing.T) {
	// A bar of low notes and a bar that climbs two octaves above the staff.
	high := []model.Element{}
	for i := 0; i < 4; i++ {
		high = append(high, &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 6},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 2, func(i int) []model.Element {
		if i == 1 {
			return high
		}
		return quarters(4)
	})

	marks := beatMarksOf(Engrave(s, beatOptions()))
	if len(marks) != 8 {
		t.Fatalf("%d beat marks, want 8", len(marks))
	}
	for _, g := range marks[1:] {
		if math.Abs(g.Y-marks[0].Y) > 0.01 {
			t.Fatalf("the row is not level: %.2f against %.2f", g.Y, marks[0].Y)
		}
	}
}

// 6/8 is two beats of a dotted quarter, not six of an eighth. Six arrows over
// a jig is a rattle, and it is not what anybody is counting.
func TestCompoundMetreIsMarkedInThrees(t *testing.T) {
	eighths := func(int) []model.Element {
		var out []model.Element
		for i := 0; i < 6; i++ {
			out = append(out, &model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 8},
			})
		}
		return out
	}
	s := beatBars(model.TimeSignature{Beats: 6, BeatValue: 8}, 2, eighths)

	if got := beatMarksOf(Engrave(s, beatOptions())); len(got) != 4 {
		t.Fatalf("%d beat marks over two bars of 6/8, want 4", len(got))
	}
}

// A bar that is not full is not beaten to the end. A pickup of one quarter is
// one beat, whatever the signature over it says, and a bar half typed into
// the editor has no beats where it has no music.
func TestAShortBarIsMarkedOnlyAsFarAsItGoes(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 2, func(i int) []model.Element {
		if i == 0 {
			return quarters(1)
		}
		return quarters(4)
	})

	if got := beatMarksOf(Engrave(s, beatOptions())); len(got) != 5 {
		t.Fatalf("%d beat marks over a one-beat pickup and a full bar, want 5", len(got))
	}
}

// A bar of rest is still a bar of beats: counting through one is the whole
// reason somebody wants the marks.
func TestARestingBarIsStillBeaten(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 1, func(int) []model.Element {
		return []model.Element{&model.Rest{Dur: model.Duration{Num: 1, Den: 1}}}
	})

	if got := beatMarksOf(Engrave(s, beatOptions())); len(got) != 4 {
		t.Fatalf("%d beat marks over a bar of rest in 4/4, want 4", len(got))
	}
}

// The marks belong to the system, not to every staff on it. A row over each
// of twenty staves is the same statement made twenty times.
func TestTheBeatMarksAreDrawnOverTheTopStaffOnly(t *testing.T) {
	s := beatBars(model.TimeSignature{Beats: 4, BeatValue: 4}, 1,
		func(int) []model.Element { return quarters(4) })
	second := &model.Part{Clef: model.Bass, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	second.Measures = append(second.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: quarters(4)}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, second)

	if got := beatMarksOf(Engrave(s, beatOptions())); len(got) != 4 {
		t.Fatalf("%d beat marks over a two-staff system, want 4", len(got))
	}
}
