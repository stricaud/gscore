/*
 * systemroom_test.go
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

// What is written under a staff belongs to that staff.
//
// The marks below one are placed against whatever is already there, so they
// end up as far down as they have to go. If nothing has reserved that room
// they carry on down into the system underneath, and a crescendo that began
// four bars ago arrives over somebody else's first bar — which is what it
// looks like to whoever is reading the page.

// underlined is a long run of bars, each with a dynamic, a hairpin and a
// slur dipping below the staff: everything that goes under one, so that the
// marks have to stack.
func underlined(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		var els []model.Element
		for k := 0; k < 4; k++ {
			// High, so the stems point down and the slur is drawn below.
			n := &model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 5},
				Dur:   model.Duration{Num: 1, Den: 4},
			}
			if k == 0 {
				n.Dynamic = model.Forte
				n.SlurStart = true
				n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
			}
			if k == 3 {
				n.SlurStop = true
				n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
			}
			els = append(els, n)
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

// hairpins are the sloped lines: nothing else in a layout slopes.
func hairpins(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) < 0.01 || math.Abs(l.X1-l.X2) < 0.01 {
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestWhatIsWrittenUnderAStaffStaysInItsSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 60
	lay := Engrave(underlined(24), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("%d systems, so nothing is below anything", len(lay.Systems))
	}
	if len(hairpins(lay)) == 0 {
		t.Fatal("no hairpins were drawn, so this proves nothing")
	}

	// Everything drawn between one system's top and the next one's belongs
	// to the first of them, and must be inside what it says its extent is.
	// Something drawn past that is in the gap, where it reads as belonging
	// to the system below — and a page is cut between systems, so it can
	// end up on the next page.
	for i, sp := range lay.Systems {
		next := math.Inf(1)
		if i+1 < len(lay.Systems) {
			next = lay.Systems[i+1].Top
		}
		low, what := sp.Bottom, ""
		note := func(y float64, kind string) {
			if y >= sp.Top && y < next && y > low {
				low, what = y, kind
			}
		}
		for _, l := range lay.Lines {
			note(math.Max(l.Y1, l.Y2), "a line")
		}
		for _, g := range lay.Glyphs {
			note(g.Y, g.Name)
		}
		for _, c := range lay.Curves {
			note(math.Max(c.Y1, math.Max(c.Y2, c.CY)), "a curve")
		}
		if what != "" {
			t.Errorf("system %d says it ends at %.2f, but %s is drawn at "+
				"%.2f — below its own system and into the gap under it",
				i, sp.Bottom, what, low)
		}
	}
}

// And the systems themselves do not overlap, which is the same rule stated
// from the other side.
func TestSystemsDoNotOverlap(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 60
	lay := Engrave(underlined(24), opt)

	for i := 1; i < len(lay.Systems); i++ {
		if lay.Systems[i].Top < lay.Systems[i-1].Bottom {
			t.Errorf("system %d ends at %.2f and system %d starts at %.2f",
				i-1, lay.Systems[i-1].Bottom, i, lay.Systems[i].Top)
		}
	}
}
