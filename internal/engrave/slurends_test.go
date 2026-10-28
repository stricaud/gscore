/*
 * slurends_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where a slur begins and ends, and which side of the music it arcs over.
 *
 * Its ends belong to its own two notes. They used to be dragged out to the
 * outermost notehead anywhere under the slur, on the reasoning that the
 * curve has to clear what it covers — which is right for a flat phrase and
 * wrong for any other. The Presto of the Moonlight sonata arpeggiates two
 * octaves upward in a bar, so a slur drawn below it began a tenth under its
 * own first note and swept diagonally up through the staff to the last one.
 * Clearing what is between the ends is the arch's business; see clearArch.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// phrase writes notes at the given pitches, slurred from the first to the
// last, and returns the drawing.
func phrase(t *testing.T, steps []model.Step, octaves []int) (*Layout, []*model.Note) {
	t.Helper()
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	var notes []*model.Note
	for i := range steps {
		n := &model.Note{
			Pitch: model.Pitch{Step: steps[i], Octave: octaves[i]},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		notes = append(notes, n)
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	notes[0].Marks.SlurStart = true
	notes[len(notes)-1].Marks.SlurStop = true
	return Engrave(s, DefaultOptions()), notes
}

// theSlur is the one curve in a drawing that is not a tie.
func theSlur(t *testing.T, lay *Layout) Curve {
	t.Helper()
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one slur", len(lay.Curves))
	}
	return lay.Curves[0]
}

func TestASlurBeginsOnItsOwnFirstNote(t *testing.T) {
	// Low, very high, middling: the stems point both ways, so the slur goes
	// above, and the highest note is nowhere near the first one.
	lay, _ := phrase(t,
		[]model.Step{model.StepC, model.StepC, model.StepE},
		[]int{4, 6, 4})
	sl := theSlur(t, lay)

	// The noteheads, in the order they were written.
	var heads []float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			heads = append(heads, g.Y)
		}
	}
	if len(heads) != 3 {
		t.Fatalf("%d noteheads drawn, want 3", len(heads))
	}
	first, highest, last := heads[0], heads[1], heads[2]

	// y grows downward, so the high C is the smallest.
	if highest >= first {
		t.Fatalf("the test's own notes are not where it thinks: heads %v", heads)
	}
	// The curve leaves from near its first note, not from the top of the
	// phrase. Two spaces of lift is the most it is allowed.
	if math.Abs(sl.Y1-first) > 2.5 {
		t.Errorf("the slur begins at %.2f and its first note is at %.2f; the "+
			"highest note under it is at %.2f, which is where it used to begin",
			sl.Y1, first, highest)
	}
	if math.Abs(sl.Y2-last) > 2.5 {
		t.Errorf("the slur ends at %.2f and its last note is at %.2f", sl.Y2, last)
	}
	// And it still clears what it arcs over: the control point is pulled
	// well above the highest note between the ends.
	if sl.CY >= highest {
		t.Errorf("the slur arches to %.2f and the note it covers is at %.2f, "+
			"so the curve goes through it", sl.CY, highest)
	}
}

func TestASlurOverStemsPointingBothWaysGoesAbove(t *testing.T) {
	// A slur goes on the side away from the stems. Where they point both
	// ways there is no such side, and every engraver puts it above —
	// Gould's rule in Behind Bars. Counted as a majority instead, a phrase
	// of mostly-up stems with a few down ones went below and had to climb
	// over the ones pointing the other way.
	lay, _ := phrase(t,
		[]model.Step{model.StepC, model.StepD, model.StepE, model.StepC},
		[]int{4, 4, 4, 6}) // three low, one high: the majority points up
	sl := theSlur(t, lay)

	var top float64 = math.Inf(1)
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Y < top {
			top = g.Y
		}
	}
	// Above means a smaller y than the notes it covers.
	if sl.CY > top {
		t.Errorf("the slur arches to %.2f, below the top notehead at %.2f: "+
			"with stems pointing both ways it belongs above", sl.CY, top)
	}
}

func TestASlurOverStemsAllPointingUpGoesBelow(t *testing.T) {
	// The ordinary rule, unchanged: every stem up, so the slur goes under
	// the noteheads.
	lay, _ := phrase(t,
		[]model.Step{model.StepC, model.StepD, model.StepE},
		[]int{4, 4, 4})
	sl := theSlur(t, lay)

	var low float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Y > low {
			low = g.Y
		}
	}
	if sl.CY < low {
		t.Errorf("the slur arches to %.2f, above the lowest notehead at %.2f: "+
			"with every stem up it belongs below", sl.CY, low)
	}
}
