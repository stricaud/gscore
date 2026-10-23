/*
 * barcolumn_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Barlines lining up is what makes a score a score: it is how the eye reads
// down a system and finds the same beat on every staff.
//
// Each staff used to draw its own barline where its own music ran out. The
// staves share their columns, but they fill them at different rates — a bar
// of four eighths and a bar of one half note are stretched differently to end
// in the same place — so the lines came out scattered across a staff space
// and more, worst at the end of a piece where nine staves finished the last
// bar at nine different places.

// scoreOfVaryingDensity gives every staff the same bars with a different
// number of notes in each, which is the ordinary case in a band score and the
// one that pulled the barlines apart.
func scoreOfVaryingDensity(staves, bars int) *model.Score {
	s := &model.Score{Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}}}
	for i := 0; i < staves; i++ {
		p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
		for j := 0; j < bars; j++ {
			v := &model.Voice{}
			// One staff holds a whole note, the next four quarters, the next
			// eight eighths, and so on round again.
			switch (i + j) % 3 {
			case 0:
				v.Elements = append(v.Elements, &model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				})
			case 1:
				for k := 0; k < 4; k++ {
					v.Elements = append(v.Elements, &model.Note{
						Pitch: model.Pitch{Step: model.Step(k % 7), Octave: 4},
						Dur:   model.Duration{Num: 1, Den: 4},
					})
				}
			default:
				for k := 0; k < 8; k++ {
					v.Elements = append(v.Elements, &model.Note{
						Pitch:      model.Pitch{Step: model.Step(k % 7), Octave: 4},
						Dur:        model.Duration{Num: 1, Den: 8},
						Accidental: model.Sharp,
					})
				}
			}
			bar := &model.Measure{Voices: []*model.Voice{v}, RightBarline: model.BarSingle}
			if j == bars-1 {
				bar.RightBarline = model.BarFinal
			}
			p.Measures = append(p.Measures, bar)
		}
		s.Parts = append(s.Parts, p)
	}
	return s
}

func TestBarlinesLineUpAcrossTheStaves(t *testing.T) {
	s := scoreOfVaryingDensity(6, 10)
	s.Normalise()

	opt := DefaultOptions()
	opt.LineWidth = 60
	lay := Engrave(s, opt)

	// Every line drawn at a bar, by the bar it closes.
	at := map[int][]float64{}
	for _, b := range lay.Barlines {
		at[b.Measure] = append(at[b.Measure], b.X)
	}
	if len(at) < 5 {
		t.Fatalf("only %d bars were drawn", len(at))
	}

	for bar, xs := range at {
		if len(xs) < 2 {
			continue
		}
		lo, hi := xs[0], xs[0]
		for _, x := range xs {
			lo, hi = math.Min(lo, x), math.Max(hi, x)
		}
		// Exactly, not nearly: there is one column and the lines are in it.
		if hi-lo > 0.001 {
			t.Errorf("bar %d closes between x=%.3f and x=%.3f on its %d staves, "+
				"a spread of %.3f spaces", bar+1, lo, hi, len(xs), hi-lo)
		}
	}
}

// And the last bar of all, which is the one that shows: a ragged final
// barline is the first thing anybody notices about a printed score.
func TestTheFinalBarlineIsOneColumn(t *testing.T) {
	s := scoreOfVaryingDensity(8, 7)
	s.Normalise()

	opt := DefaultOptions()
	opt.LineWidth = 60
	lay := Engrave(s, opt)

	last := -1
	for _, b := range lay.Barlines {
		if b.Measure > last {
			last = b.Measure
		}
	}
	var xs []float64
	for _, b := range lay.Barlines {
		if b.Measure == last {
			xs = append(xs, b.X)
		}
	}
	if len(xs) != 8 {
		t.Fatalf("the last bar is closed on %d staves, want 8", len(xs))
	}
	for _, x := range xs {
		if math.Abs(x-xs[0]) > 0.001 {
			t.Errorf("the piece ends at x=%.3f on one staff and %.3f on another", xs[0], x)
		}
	}
}

// A bar that is full has no empty part, however empty it looks.
//
// A whole-bar rest is one small glyph with four beats of white space after
// it, and a click in that space is a click over the rest — not a click at the
// end of the music. The editor could not tell the two apart, so writing there
// appended, found the bar full and put the note in the next bar instead:
// clicking one bar wrote in the one beside it.
func TestABarOfRestIsReportedAsFull(t *testing.T) {
	whole := model.Duration{Num: 1, Den: 1}
	quarter := model.Duration{Num: 1, Den: 4}

	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	p.Measures = []*model.Measure{
		// A bar of rest: drawn as nothing much, and as full as a bar gets.
		{Voices: []*model.Voice{{Elements: []model.Element{&model.Rest{Dur: whole}}}},
			RightBarline: model.BarSingle},
		// One quarter and three beats to spare.
		{Voices: []*model.Voice{{Elements: []model.Element{&model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: quarter,
		}}}}, RightBarline: model.BarSingle},
		// Four quarters, full of notes this time.
		{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: quarter},
			&model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: quarter},
			&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: quarter},
			&model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 5}, Dur: quarter},
		}}}, RightBarline: model.BarFinal},
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: quarter},
		Parts: []*model.Part{p},
	}
	lay := Engrave(s, DefaultOptions())

	full := map[int]bool{}
	for _, h := range lay.Hits {
		full[h.Measure] = h.Full
	}
	for bar, want := range map[int]bool{0: true, 1: false, 2: true} {
		if full[bar] != want {
			t.Errorf("bar %d reports full=%v, want %v", bar+1, full[bar], want)
		}
	}
}
