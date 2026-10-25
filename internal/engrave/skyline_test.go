/*
 * skyline_test.go
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

// Marks outside the staff are placed against what is already there, not at a
// fixed distance from the staff.
//
// Drawn at fixed distances they were drawn through each other: a hairpin
// through a slur and through the dynamic it ran between, a boxed rehearsal
// letter around a note on ledger lines, an 8va sign on top of a segno. There
// was an allowance for them above the staff, but one allowance shared by all
// of them rather than one each.

func TestTheSkylineStacksWhatIsPutOnIt(t *testing.T) {
	s := newSkyline(0, 20, 10, staffHeight) // a staff from y=10 to y=14

	if got := s.reach(0, 20, false); got != 10 {
		t.Errorf("an empty outline reaches %v above, want the staff at 10", got)
	}
	if got := s.reach(0, 20, true); got != 14 {
		t.Errorf("an empty outline reaches %v below, want the staff at 14", got)
	}

	// Something two spaces tall, half a space clear of the staff.
	first := s.place(2, 8, 2, 0.5, true)
	if first != 14.5 {
		t.Errorf("the first mark went to %v, want 14.5", first)
	}
	// The next one clears it, but only where they overlap.
	second := s.place(5, 12, 2, 0.5, true)
	if second != 17 {
		t.Errorf("the second mark went to %v, want 17, clear of the first", second)
	}
	// Somewhere else along the staff there is nothing to clear.
	third := s.place(15, 19, 2, 0.5, true)
	if third != 14.5 {
		t.Errorf("a mark on its own went to %v, want 14.5", third)
	}

	// And the same upwards, where clear means a smaller y.
	up := s.place(2, 8, 2, 0.5, false)
	if up != 9.5 {
		t.Errorf("a mark above went to %v, want 9.5", up)
	}
	if next := s.place(2, 8, 2, 0.5, false); next != 7 {
		t.Errorf("the one above that went to %v, want 7", next)
	}
}

// crowded is a bar with everything that wants to be above or below a staff
// at once: high notes on ledger lines, a letter, a sign, an octave line, a
// dynamic and a hairpin.
func crowded() *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	note := func(step model.Step, oct int) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: q}
	}

	a := note(model.StepG, 6) // well above the staff, on ledger lines
	a.Dynamic = model.Forte
	a.Octave = &model.OctaveShift{Amount: 1, Start: true}
	a.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
	a.SlurStart = true

	b := note(model.StepA, 6)
	c := note(model.StepB, 6)

	d := note(model.StepG, 6)
	d.Octave = &model.OctaveShift{Amount: 1, Stop: true}
	d.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
	d.SlurStop = true
	d.Dynamic = model.FF

	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{a, b, c, d}}},
				RightBarline: model.BarSingle,
				Marker:       "A",
				Sign:         model.Segno,
			}},
		}},
	}
}

// box is a drawn thing's extent, for asking whether two of them overlap.
type box struct{ x0, x1, y0, y1 float64 }

func (a box) overlaps(b box) bool {
	return a.x0 < b.x1-0.05 && b.x0 < a.x1-0.05 &&
		a.y0 < b.y1-0.05 && b.y0 < a.y1-0.05
}

func TestNothingAboveOrBelowTheStaffIsDrawnThroughAnythingElse(t *testing.T) {
	lay := Engrave(crowded(), DefaultOptions())

	// The marks that have to clear each other, each as the box it occupies.
	marks := map[string]box{}
	for _, x := range lay.Texts {
		w := textWidth(x.S, x.Size)
		marks[x.S] = box{x.X, x.X + w, x.Y - x.Size*0.8, x.Y + x.Size*0.25}
	}
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "segno", "dynamicForte", "dynamicFF":
			up, down := glyphReach(g.Name)
			marks[g.Name] = box{g.X, g.X + glyphWidth(g), g.Y - up, g.Y + down}
		}
	}
	// The hairpin: the sloping lines under the staff.
	var hair *box
	for _, l := range lay.Lines {
		if l.X1 == l.X2 || l.Y1 == l.Y2 {
			continue
		}
		b := box{
			math.Min(l.X1, l.X2), math.Max(l.X1, l.X2),
			math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2),
		}
		if hair == nil {
			hair = &b
		} else {
			hair.x0, hair.x1 = math.Min(hair.x0, b.x0), math.Max(hair.x1, b.x1)
			hair.y0, hair.y1 = math.Min(hair.y0, b.y0), math.Max(hair.y1, b.y1)
		}
	}
	if hair == nil {
		t.Fatal("the hairpin was not drawn")
	}
	marks["hairpin"] = *hair

	for _, want := range []string{"A", "segno", "8va", "dynamicForte", "hairpin"} {
		if _, ok := marks[want]; !ok {
			t.Fatalf("%s was not drawn at all", want)
		}
	}

	names := make([]string, 0, len(marks))
	for n := range marks {
		names = append(names, n)
	}
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			a, b := marks[names[i]], marks[names[j]]
			if a.overlaps(b) {
				t.Errorf("%s and %s are drawn through each other: %+v and %+v",
					names[i], names[j], a, b)
			}
		}
	}
}

// And the noteheads themselves: a letter boxed around a note is the version
// of this that shows up first, because a rehearsal mark is drawn with a box.
func TestAMarkIsNotDrawnThroughTheNotes(t *testing.T) {
	lay := Engrave(crowded(), DefaultOptions())

	var heads []box
	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		up, down := glyphReach(g.Name)
		heads = append(heads, box{g.X, g.X + glyphWidth(g), g.Y - up, g.Y + down})
	}
	if len(heads) == 0 {
		t.Fatal("no notes were drawn")
	}

	for _, x := range lay.Texts {
		if x.S != "A" && x.S != "8va" {
			continue
		}
		w := textWidth(x.S, x.Size)
		m := box{x.X, x.X + w, x.Y - x.Size*0.8, x.Y + x.Size*0.25}
		for _, h := range heads {
			if m.overlaps(h) {
				t.Errorf("%q is drawn over a notehead: %+v and %+v", x.S, m, h)
			}
		}
	}
}

// A slur has to clear the music it arcs over, not just the notes at its ends.
//
// Its ends belong to its notes and stay there; its middle is free. An arch
// of a fixed height is only clear of a phrase that happens to be flat, and
// over a rising run it cut straight through the notes between its ends —
// which is the one place a slur must not be, because the line it draws is
// the phrase.
// archedPhrase rises to a peak in the middle and comes back down, which is
// the shape that shows the bug: the slur's two ends are low and everything
// between them is higher, so an arch measured from the ends alone passes
// straight through the peak. A run that rises all the way hides it, because
// a line from its first note to its last already clears the notes between.
func archedPhrase() *model.Score {
	q := model.Duration{Num: 1, Den: 8}
	// High enough that the stems point down and the slur goes above, with a
	// peak between the ends for it to clear.
	steps := []struct {
		s model.Step
		o int
	}{
		{model.StepA, 5}, {model.StepD, 6}, {model.StepF, 6},
		{model.StepD, 6}, {model.StepA, 5},
	}
	v := &model.Voice{}
	for i, st := range steps {
		n := &model.Note{Pitch: model.Pitch{Step: st.s, Octave: st.o}, Dur: q}
		n.SlurStart = i == 0
		n.SlurStop = i == len(steps)-1
		v.Elements = append(v.Elements, n)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{v}, RightBarline: model.BarSingle,
			}},
		}},
	}
}

func TestASlurClearsTheNotesItArcsOver(t *testing.T) {
	lay := Engrave(archedPhrase(), DefaultOptions())

	cs := curves(lay)
	if len(cs) != 1 {
		t.Fatalf("got %d curves, want the one slur", len(cs))
	}
	c := cs[0]

	// Where the curve really is at its middle: a quadratic passes a quarter
	// of the way from each end and half of the way towards the control
	// point.
	mid := 0.25*c.Y1 + 0.5*c.CY + 0.25*c.Y2
	above := mid < (c.Y1+c.Y2)/2

	x0, x1 := math.Min(c.X1, c.X2), math.Max(c.X1, c.X2)
	inset := (x1 - x0) * 0.15
	checked := 0
	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		if g.X < x0+inset || g.X > x1-inset {
			continue // the notes at the ends are the slur's own
		}
		checked++
		if above && mid > g.Y-0.55 {
			t.Errorf("the slur passes at y=%.2f, through the notehead at %.2f",
				mid, g.Y)
		}
		if !above && mid < g.Y+0.55 {
			t.Errorf("the slur passes at y=%.2f, through the notehead at %.2f",
				mid, g.Y)
		}
	}
	if checked == 0 {
		t.Fatal("no notes were under the slur, so this proves nothing")
	}
}
