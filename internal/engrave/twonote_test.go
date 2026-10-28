/*
 * twonote_test.go
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

// The two-note tremolo: two notes with the beams drawn between their stems,
// meaning "alternate between these for as long as the two of them last".
//
// It is not the tremolo on one note and must not be drawn as one — slashes
// through both stems would say that each note is repeated on its own, which
// is a different thing to play.

// tremoloPair builds a bar of two notes marked as a two-note tremolo.
func tremoloPair(strokes int, value model.Duration, lo, hi model.Step) *model.Score {
	a := &model.Note{Pitch: model.Pitch{Step: lo, Octave: 4}, Dur: value}
	b := &model.Note{Pitch: model.Pitch{Step: hi, Octave: 4}, Dur: value}
	a.Tremolo = &model.Tremolo{Strokes: strokes, Start: true}
	b.Tremolo = &model.Tremolo{Strokes: strokes, Stop: true}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{a, b}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func TestTheBeamsAreDrawnBetweenTheTwoStems(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG), opt)

	// Three beams, and nothing drawn as slashes.
	if n := len(lay.Quads); n != 3 {
		t.Errorf("%d beams drawn, want 3", n)
	}
	for _, g := range lay.Glyphs {
		if has(g.Name, "tremolo") || has(g.Name, "buzzRoll") {
			t.Errorf("a %s was drawn: the slashes belong to the one-note "+
				"tremolo, not to a pair", g.Name)
		}
	}

	// The beams run from the first note to the second: they begin over the
	// first notehead and end over the second, rather than hanging off one
	// stem the way a flag does.
	var heads []float64
	for _, g := range lay.Glyphs {
		if has(g.Name, "noteheadBlack") || has(g.Name, "noteheadHalf") {
			heads = append(heads, g.X)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d noteheads, want 2", len(heads))
	}
	first, second := math.Min(heads[0], heads[1]), math.Max(heads[0], heads[1])
	for i, q := range lay.Quads {
		x0, x1 := q.X[0], q.X[0]
		for _, x := range q.X {
			x0, x1 = math.Min(x0, x), math.Max(x1, x)
		}
		if x0 < first || x0 > first+2 {
			t.Errorf("beam %d begins at %.2f, not over the first note at %.2f",
				i, x0, first)
		}
		if x1 < second || x1 > second+2 {
			t.Errorf("beam %d ends at %.2f, not over the second note at %.2f",
				i, x1, second)
		}
	}
	// And they are all the same length, being one stack of beams.
	for i := 1; i < len(lay.Quads); i++ {
		if math.Abs(lay.Quads[i].X[0]-lay.Quads[0].X[0]) > 0.01 {
			t.Errorf("beam %d starts somewhere else", i)
		}
	}
}

// However many strokes were asked for, that is how many beams are drawn.
func TestTheStrokesAreTheBeams(t *testing.T) {
	for _, strokes := range []int{1, 2, 3, 4} {
		opt := DefaultOptions()
		opt.Mode = Strip
		lay := Engrave(tremoloPair(strokes, model.Duration{Num: 1, Den: 2},
			model.StepC, model.StepG), opt)
		if n := len(lay.Quads); n != strokes {
			t.Errorf("%d strokes asked for, %d beams drawn", strokes, n)
		}
	}
}

// Whole notes have no stems, so the beams go between the noteheads.
func TestAPairOfWholeNotesIsBeamedBetweenTheHeads(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(2, model.Duration{Num: 1, Den: 1}, model.StepC, model.StepG), opt)
	if n := len(lay.Quads); n != 2 {
		t.Fatalf("%d beams drawn between two whole notes, want 2", n)
	}
	var heads []float64
	for _, g := range lay.Glyphs {
		if has(g.Name, "noteheadWhole") {
			heads = append(heads, g.X)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d whole noteheads", len(heads))
	}
	for _, q := range lay.Quads {
		for _, x := range q.X {
			if x < math.Min(heads[0], heads[1]) || x > math.Max(heads[0], heads[1])+2 {
				t.Errorf("a beam at %.2f is outside the two heads at %.2f and %.2f",
					x, heads[0], heads[1])
			}
		}
	}
}

// A start with nothing after it is half a mark and is not drawn. Guessing
// what it was meant to pair with is how a bar comes out with beams running
// into the next one.
func TestAHalfWrittenPairIsNotDrawn(t *testing.T) {
	s := tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG)
	marks := s.Parts[0].Measures[0].Voices[0].Elements[1].(*model.Note)
	marks.Tremolo = nil // the partner never got its mark

	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)
	if n := len(lay.Quads); n != 0 {
		t.Errorf("%d beams drawn for a pair with only one end", n)
	}
}

// The beams stand clear of both stems, and of the noteheads under them.
//
// Drawn stem to stem and stacked down to the heads, the pair reads as one
// beamed group whose beams happen to be doubled, and a pianist reading bar
// 23 of the Presto saw two beamed half notes rather than an alternation
// between them. The air at each end is what says the beams belong to
// neither note but to the movement between the two.
func TestTheBeamsStandClearOfTheNotesTheyJoin(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG), opt)

	var heads []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "noteheadHalf") {
			heads = append(heads, g)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("%d noteheads, want 2", len(heads))
	}
	// The stems of those two notes, which the barline is not one of.
	lo := math.Min(heads[0].X, heads[1].X) - 0.5
	hi := math.Max(heads[0].X, heads[1].X) + 2
	var stems []Line
	for _, l := range lay.Lines {
		if math.Abs(l.X1-l.X2) < 0.01 && math.Abs(l.Y1-l.Y2) > 1 &&
			l.X1 >= lo && l.X1 <= hi {
			stems = append(stems, l)
		}
	}
	if len(stems) != 2 {
		t.Fatalf("%d stems, want the two the pair hangs from", len(stems))
	}
	left, right := math.Min(stems[0].X1, stems[1].X1), math.Max(stems[0].X1, stems[1].X1)
	for i, q := range lay.Quads {
		x0, x1 := q.X[0], q.X[0]
		for _, x := range q.X {
			x0, x1 = math.Min(x0, x), math.Max(x1, x)
		}
		if x0 <= left+0.01 {
			t.Errorf("beam %d begins at %.2f, on the left stem at %.2f", i, x0, left)
		}
		if x1 >= right-0.01 {
			t.Errorf("beam %d ends at %.2f, on the right stem at %.2f", i, x1, right)
		}
	}

	// And the lowest beam keeps off the notehead below it.
	for _, h := range heads {
		for i, q := range lay.Quads {
			for _, y := range q.Y {
				if math.Abs(h.Y-y) < 0.5 {
					t.Errorf("beam %d runs at %.2f, through the notehead at %.2f",
						i, y, h.Y)
				}
			}
		}
	}
}

// The beams are the only thing on the page that stands for the mark, so
// they are a clickable region of their own.
//
// Without one a click on them fell through to whatever was underneath, and
// there was no way to pick a tremolo up — or to see that one had been
// picked up, which from where anybody is sitting is the same thing.
func TestTheTremoloHasARegionOfItsOwn(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(3, model.Duration{Num: 1, Den: 2}, model.StepC, model.StepG), opt)

	if len(lay.Tremolos) != 1 {
		t.Fatalf("%d regions for one tremolo, want 1", len(lay.Tremolos))
	}
	tr := lay.Tremolos[0]
	if tr.Beams != 3 {
		t.Errorf("the region says %d beams, and three were drawn", tr.Beams)
	}
	if tr.From == 0 || tr.To == 0 || tr.From == tr.To {
		t.Errorf("the region names notes %d and %d, want the two it joins", tr.From, tr.To)
	}
	if tr.Width <= 0 || tr.Height <= 0 {
		t.Fatalf("the region is %.2f by %.2f, which cannot be clicked", tr.Width, tr.Height)
	}
	// It covers every beam that was drawn, which is what was clicked.
	for i, q := range lay.Quads {
		for k := range q.X {
			if q.X[k] < tr.X-0.01 || q.X[k] > tr.X+tr.Width+0.01 {
				t.Errorf("beam %d reaches to %.2f, outside the region %.2f..%.2f",
					i, q.X[k], tr.X, tr.X+tr.Width)
			}
			if q.Y[k] < tr.Y-0.01 || q.Y[k] > tr.Y+tr.Height+0.01 {
				t.Errorf("beam %d reaches to %.2f, outside the region %.2f..%.2f",
					i, q.Y[k], tr.Y, tr.Y+tr.Height)
			}
		}
	}
}

// The pair between two whole noteheads is the same mark and is picked up
// the same way.
func TestTheWholeNotePairHasARegionToo(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(tremoloPair(2, model.Duration{Num: 1, Den: 1}, model.StepC, model.StepG), opt)
	if len(lay.Tremolos) != 1 {
		t.Fatalf("%d regions between two whole notes, want 1", len(lay.Tremolos))
	}
}

// An ordinary beamed group is not a tremolo, and clicking its beams must
// not pick one up.
func TestAnOrdinaryBeamIsNotATremolo(t *testing.T) {
	s := tremoloPair(3, model.Duration{Num: 1, Den: 8}, model.StepC, model.StepG)
	for _, el := range s.Parts[0].Measures[0].Voices[0].Elements {
		el.(*model.Note).Tremolo = nil
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)
	if len(lay.Quads) == 0 {
		t.Fatal("the two eighths were not beamed, so there is nothing to tell apart")
	}
	if len(lay.Tremolos) != 0 {
		t.Errorf("%d tremolo regions over a plain beam", len(lay.Tremolos))
	}
}
