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
	"strings"
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

// The marks on a slur's own notes go inside the curve, not through it.
//
// A slur and the articulations under it are written on the same side of the
// same notes, and which goes outside is settled: the marks belong to their
// noteheads and the slur to the phrase, so the marks sit between the notes
// and the curve. What the slur arcs over is lifted clear by its arch, but
// the ends are not — they belong to the slur's own two notes — so a phrase
// whose first and last notes were accented had its curve drawn through
// both. Bar 55 of the Rossini: three marcato marks under one slur, the
// middle one gone over and the outer two gone through.
func TestASlurGoesRoundTheMarksOnItsOwnNotes(t *testing.T) {
	// Low notes, so the stems point up and both the slur and the marks are
	// written below. Marked and unmarked, to see what the marks did.
	build := func(marked bool) *Layout {
		s := model.NewScore()
		m := s.Parts[0].Measures[0]
		for _, step := range []model.Step{model.StepC, model.StepE, model.StepC} {
			n := &model.Note{
				Pitch: model.Pitch{Step: step, Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 4},
			}
			if marked {
				n.Marks.Articulations = []model.Articulation{model.Marcato}
			}
			m.Voices[0].Elements = append(m.Voices[0].Elements, n)
		}
		els := m.Voices[0].Elements
		els[0].(*model.Note).Marks.SlurStart = true
		els[2].(*model.Note).Marks.SlurStop = true
		return Engrave(s, DefaultOptions())
	}

	plain := theSlur(t, build(false))
	marked := build(true)
	curve := theSlur(t, marked)

	// Below, so clear means a larger y: the marks push the ends out, and
	// with nothing on the notes the ends stay where they were.
	if curve.Y1 <= plain.Y1 || curve.Y2 <= plain.Y2 {
		t.Errorf("the marks did not move the slur's ends: plain %.2f..%.2f, "+
			"marked %.2f..%.2f", plain.Y1, plain.Y2, curve.Y1, curve.Y2)
	}
	// And every mark is inside the curve: above its ends, which are its
	// highest points when it hangs below.
	outer := math.Max(curve.Y1, curve.Y2)
	for _, g := range marked.Glyphs {
		if !strings.HasPrefix(g.Name, "artic") {
			continue
		}
		_, down := glyphReach(g.Name)
		if bottom := g.Y + down; bottom > outer {
			t.Errorf("the mark at x=%.2f reaches to %.2f, below the slur's "+
				"end at %.2f", g.X, bottom, outer)
		}
	}
}
