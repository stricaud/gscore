/*
 * flagroom_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A flag takes room, and the note after it has to start past the flag.
//
// An eighth that is not beamed to anything carries a flag, and a flag hangs
// off the right of the stem, which on an up-stem note is already at the right
// edge of the head. So such a note is half a space wider than its head, and a
// spacing rule that only knows about durations does not know that: the next
// notehead is set where the duration says and the flag prints across it.

// flagged builds a bar of dotted quarters and eighths at a pitch low enough
// that every stem points up.
//
// The eighth falls at the end of a beat with nothing to beam to, so it is
// flagged, and the note after it starts the next beat.
func flagged() *model.Score {
	dottedQuarter := model.Duration{Num: 3, Den: 8}
	eighth := model.Duration{Num: 1, Den: 8}
	v := &model.Voice{}
	for i := 0; i < 2; i++ {
		for _, d := range []model.Duration{dottedQuarter, eighth} {
			v.Elements = append(v.Elements, &model.Note{
				Pitch: pitch(model.StepD, 4), Dur: d,
			})
		}
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

func TestAFlagDoesNotPrintOnTheNextNote(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(flagged(), opt)

	glyphs := append([]Glyph(nil), lay.Glyphs...)
	sort.Slice(glyphs, func(i, j int) bool { return glyphs[i].X < glyphs[j].X })

	flags := 0
	for i, g := range glyphs {
		if !has(g.Name, "flag") {
			continue
		}
		flags++
		end := g.X + glyphWidth(g)
		for _, next := range glyphs[i+1:] {
			if !has(next.Name, "notehead") || next.X <= g.X {
				continue
			}
			if gap := next.X - end; gap < 0 {
				t.Errorf("%s ends at %.2f and the next notehead starts at %.2f: "+
					"it prints %.2f spaces into it",
					g.Name, end, next.X, -gap)
			}
			break
		}
	}
	if flags == 0 {
		t.Fatal("no flags were drawn, so this proves nothing")
	}
}
