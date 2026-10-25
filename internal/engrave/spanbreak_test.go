/*
 * spanbreak_test.go
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
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tie or a slur that runs over a line break is cut at the right margin and
// picked up at the left of the next system. What it remembered of its start
// was the height on the page the note it left was at — and the next system's
// staff is a long way further down the drawing, so the continuation was drawn
// from the old staff to the new one: one near-vertical stroke the height of
// the page, through every system in between.

// tiedChain is a run of whole notes, each tied to the next, long enough to
// need several systems.
func tiedChain(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		n := &model.Note{
			Pitch:    model.Pitch{Step: model.StepB, Octave: 4},
			Dur:      model.Duration{Num: 1, Den: 1},
			TieStart: i < bars-1,
			TieStop:  i > 0,
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func TestATieBrokenOverASystemStaysOnItsOwnSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40 // narrow enough to break the chain over several lines
	lay := Engrave(tiedChain(16), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the chain spread over at least three", len(lay.Systems))
	}

	for i, c := range lay.Curves {
		if math.Abs(c.Y2-c.Y1) > staffHeight {
			t.Errorf("curve %d runs from y=%.2f to y=%.2f, %.2f spaces: "+
				"a tie reaching from one system to another",
				i, c.Y1, c.Y2, math.Abs(c.Y2-c.Y1))
		}
	}
}

func TestASlurBrokenOverASystemStaysOnItsOwnSystem(t *testing.T) {
	s := tiedChain(16)
	ms := s.Parts[0].Measures
	for i, m := range ms {
		n := m.Voices[0].Elements[0].(*model.Note)
		n.TieStart, n.TieStop = false, false
		n.SlurStart = i == 0
		n.SlurStop = i == len(ms)-1
	}

	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(s, opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the slur spread over at least three", len(lay.Systems))
	}

	for i, c := range lay.Curves {
		// A slur arches, so it is allowed more room than a tie — but never
		// the distance from one staff to the next.
		if math.Abs(c.Y2-c.Y1) > staffHeight {
			t.Errorf("curve %d runs from y=%.2f to y=%.2f: "+
				"a slur reaching from one system to another", i, c.Y1, c.Y2)
		}
	}
}

// The stub a tie leaves where a system ends, and the one that picks it up on
// the next, are barely wider than a notehead. Drawn at a tie's full arch they
// rose further than they reached across, and what printed over the note was a
// caret — which on a score is an accent, and means something else entirely.
func TestAShortTieIsAnArcAndNotASpike(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	for i, c := range lay.Curves {
		width := c.X2 - c.X1
		// How far the control point stands off the chord of the curve. It is
		// set at twice the rise, so this is generous either way.
		rise := math.Abs(c.CY - (c.Y1+c.Y2)/2)
		if rise > width {
			t.Errorf("curve %d is %.2f spaces wide and reaches %.2f out: "+
				"it is drawn as a spike, not an arc", i, width, rise)
		}
	}
}

// Where a curve is picked up at the head of a system there has to be
// somewhere to pick it up from. There was not: it began in the column the
// music begins in, which is where the note it joins already sits, so the
// continuation was a notehead wide and arrived from nowhere.
func TestAResumedCurveHasRoomToComeInFrom(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the chain broken at least twice", len(lay.Systems))
	}
	for i, c := range lay.Curves {
		if w := c.X2 - c.X1; w < 1 {
			t.Errorf("curve %d is %.2f spaces wide: there was no room to draw it", i, w)
		}
	}
}

// And the stub left behind runs to the end of the line, not to wherever the
// note that starts it happens to be. It used to be handed the width of the
// staff where it wanted the end of it, and stopped four spaces short — in the
// middle of the last bar, which reads as a tie to nothing.
func TestABrokenCurveRunsToTheEndOfItsSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	// Where the music ends on a full system: the line width plus the blank
	// border the engraver keeps at the left of it.
	end := opt.LineWidth + opt.Spacings.Margin

	// Every system but the last leaves a tie hanging, so every one of them
	// should have a curve reaching the margin.
	for i, sys := range lay.Systems[:len(lay.Systems)-1] {
		found := false
		for _, c := range lay.Curves {
			if c.Y1 < sys.Top || c.Y1 > sys.Bottom {
				continue
			}
			if math.Abs(c.X2-end) < 0.01 {
				found = true
			}
		}
		if !found {
			t.Errorf("system %d leaves a tie open but nothing reaches x=%.2f", i, end)
		}
	}
}
