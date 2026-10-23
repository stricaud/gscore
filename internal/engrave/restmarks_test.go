/*
 * restmarks_test.go
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
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A rest carries marks, and the engraver used to read none of them.
//
// A crescendo that closes on a rest is ordinary writing: a player holds the
// swell to the end of the phrase and the phrase ends in silence. The mark
// that closed it sat on the rest, the engraver asked the rest what it carried
// and was told nothing, and the hairpin stayed open — drawn to the right
// margin of that system, and of every system after it, to the end of the
// piece. A conductor's score came out with flat lines running under half the
// staves on every page.

// swellScore is a note that opens a crescendo, a rest that closes it, and
// then enough empty bars to need several systems.
func swellScore(stopOnRest bool, bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}

	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepG, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 2},
	}
	n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}

	closer := &model.Rest{Dur: model.Duration{Num: 1, Den: 2}}
	var second model.Element = closer
	if !stopOnRest {
		// The same shape with the mark on a note, which always worked, so
		// the two can be compared.
		m := &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 2},
		}
		m.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
		second = m
	} else {
		closer.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
	}

	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: []model.Element{n, second}}},
		RightBarline: model.BarSingle,
	})
	for i := 1; i < bars; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

// wedges returns the sloping lines a hairpin is drawn with. In a score of one
// note and a lot of rests they are the only sloping lines there are: stems
// are vertical and staff lines are level.
func wedges(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if l.X1 != l.X2 && l.Y1 != l.Y2 {
			out = append(out, l)
		}
	}
	return out
}

func TestAHairpinClosesOnARest(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(swellScore(true, 24), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the score spread over several", len(lay.Systems))
	}
	w := wedges(lay)
	if len(w) == 0 {
		t.Fatal("the hairpin was not drawn at all")
	}

	// Every part of it belongs to the system it was written in. A hairpin
	// hangs below its staff, so the test is whether any of it has reached
	// the system below — which is what starting again on a system it has no
	// business on looks like.
	next := lay.Systems[1].Top
	for i, l := range w {
		if l.Y1 >= next || l.Y2 >= next {
			t.Errorf("part %d of the hairpin is at y=%.2f, on or past the next "+
				"system at %.2f: it never closed", i, l.Y1, next)
		}
	}
}

// And it is drawn the same way whichever kind of element closes it, which is
// the point: a rest is a place in the music like any other.
func TestAHairpinOnARestIsDrawnLikeOneOnANote(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	onRest := wedges(Engrave(swellScore(true, 24), opt))
	onNote := wedges(Engrave(swellScore(false, 24), opt))

	if len(onRest) != len(onNote) {
		t.Fatalf("a hairpin closing on a rest is drawn with %d lines and one "+
			"closing on a note with %d", len(onRest), len(onNote))
	}
}

// The same hole swallowed everything else a rest can carry. A fermata over a
// rest is how a bar of silence is held, and a chord symbol over a resting bar
// is how a chart tells a player what the band is on.
func TestARestCanCarryItsOtherMarks(t *testing.T) {
	r := &model.Rest{Dur: model.Duration{Num: 1, Den: 1}}
	r.AddArticulation(model.Fermata)
	r.Dynamic = model.Piano
	r.Chord = &model.ChordSymbol{Root: "B", Quality: "m7"}

	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{r}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
	lay := Engrave(s, DefaultOptions())

	glyphs := map[string]bool{}
	for _, g := range lay.Glyphs {
		glyphs[g.Name] = true
	}
	if !glyphs["fermataAbove"] && !glyphs["fermataBelow"] {
		t.Error("the fermata over the rest was not drawn")
	}
	if !glyphs["dynamicPiano"] {
		t.Error("the dynamic at the rest was not drawn")
	}

	texts := ""
	for _, x := range lay.Texts {
		texts += x.S
	}
	if texts == "" {
		t.Error("the chord symbol over the resting bar was not drawn")
	}
}
