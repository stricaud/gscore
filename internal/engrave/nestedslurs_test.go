/*
 * nestedslurs_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * More than one curve open at a time.
 *
 * A long phrase with shorter ones inside it is how a wind line is marked
 * under a string legato, and a grace note slurred into the note a phrase
 * ends on is the cadence of every keyboard piece of the period. The
 * engraver kept one open slur and no more: a second opening found one
 * already there and did nothing, and the inner phrase was never drawn.
 * MusicXML's own 33c sample is three bars long and has one in it.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// marked makes one bar of four quarter notes, with the phrase marks put on
// by fn, and draws it.
func marked(t *testing.T, fn func(notes []*model.Note)) *Layout {
	t.Helper()
	var notes []*model.Note
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.Step(i % 7), Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		notes = append(notes, n)
		els = append(els, n)
	}
	fn(notes)
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef:     model.Treble,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{Voices: []*model.Voice{{Elements: els}}}},
		}},
	}
	return Engrave(s, DefaultOptions())
}

func TestOneSlurInsideAnotherIsDrawnTwice(t *testing.T) {
	// The outer phrase covers all four notes; the inner covers the middle
	// two. Both are written, so both have to be drawn.
	outer := marked(t, func(n []*model.Note) {
		n[0].SlurStart = 1
		n[3].SlurStop = 1
	})
	nested := marked(t, func(n []*model.Note) {
		n[0].SlurStart = 1
		n[1].SlurStart = 1
		n[2].SlurStop = 1
		n[3].SlurStop = 1
	})
	if a, b := len(curves(outer)), len(curves(nested)); b != a+1 {
		t.Errorf("one slur draws %d curves and two nested draw %d, "+
			"want one more", a, b)
	}
}

func TestTheInnerPhraseClosesFirst(t *testing.T) {
	// Slurs nest and do not cross, so a close takes the curve opened most
	// recently. Paired the other way round, the outer phrase would end in
	// the middle of the bar and the inner one would run past it.
	lay := marked(t, func(n []*model.Note) {
		n[0].SlurStart = 1
		n[1].SlurStart = 1
		n[2].SlurStop = 1
		n[3].SlurStop = 1
	})
	var widths []float64
	for _, c := range curves(lay) {
		widths = append(widths, c.X2-c.X1)
	}
	if len(widths) != 2 {
		t.Fatalf("%d curves were drawn, want 2", len(widths))
	}
	// The outer one is the wider: it begins on the first note and ends on
	// the last, while the inner begins on the second and ends on the third.
	wide, narrow := widths[0], widths[1]
	if wide < narrow {
		wide, narrow = narrow, wide
	}
	if wide <= narrow {
		t.Errorf("the two curves are %g and %g wide: one does not cover "+
			"the other", wide, narrow)
	}
}

func TestTwoCurvesEndingOnOneNoteAreBothDrawn(t *testing.T) {
	// The appoggiatura at a cadence: a grace note slurred into the note a
	// phrase ends on. One of the two used to be dropped.
	one := marked(t, func(n []*model.Note) {
		n[0].SlurStart = 1
		n[3].SlurStop = 1
	})
	both := marked(t, func(n []*model.Note) {
		n[0].SlurStart = 1
		n[3].SlurStop = 2
		n[2].SlurStart = 1
	})
	if a, b := len(curves(one)), len(curves(both)); b != a+1 {
		t.Errorf("one curve ending on the note draws %d and two draw %d, "+
			"want one more", a, b)
	}
}
