/*
 * meter_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The meter is written where the piece begins and wherever it changes, and
// nowhere else.
//
// The clef and the key signature are repeated at the head of every system,
// because a player landing anywhere has to know them. The meter is not:
// repeated, it says "the meter changes here" to anybody reading, and it
// costs three staff spaces of every line it is printed on.

func meterGlyphs(lay *Layout) int {
	n := 0
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "timeSig") {
			n++
		}
	}
	return n
}

// bars is a run of them in one meter, with a change part way if asked.
func meterScore(count, changeAt int, to model.TimeSignature) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < count; i++ {
		m := &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarSingle,
		}
		if i == changeAt {
			t := to
			m.Time = &t
		}
		p.Measures = append(p.Measures, m)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func TestTheMeterIsWrittenOnceAtTheHead(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 30 // narrow, so there are many systems
	lay := Engrave(meterScore(20, -1, model.TimeSignature{}), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("%d systems, so this proves nothing", len(lay.Systems))
	}
	// Four over four is two glyphs: the numerator and the denominator.
	if n := meterGlyphs(lay); n != 2 {
		t.Errorf("the meter was drawn %d times over %d systems, want once",
			n/2, len(lay.Systems))
	}
}

// And at a change, wherever that falls.
func TestAChangeOfMeterIsWritten(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 30
	lay := Engrave(meterScore(20, 9,
		model.TimeSignature{Beats: 3, BeatValue: 4}), opt)

	if n := meterGlyphs(lay); n != 4 {
		t.Errorf("%d meter glyphs: want the one at the head and the one at "+
			"the change, two glyphs each", n)
	}
}

// The clef and the key, on the other hand, are on every system.
func TestTheClefAndKeyAreRepeated(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 30
	s := meterScore(20, -1, model.TimeSignature{})
	s.Parts[0].Key = model.KeySignature{Fifths: -2}
	lay := Engrave(s, opt)

	clefs, flats := 0, 0
	for _, g := range lay.Glyphs {
		switch {
		case strings.HasPrefix(g.Name, "gClef"):
			clefs++
		case g.Name == "accidentalFlat":
			flats++
		}
	}
	if clefs != len(lay.Systems) {
		t.Errorf("%d clefs over %d systems: every system carries one",
			clefs, len(lay.Systems))
	}
	if flats != 2*len(lay.Systems) {
		t.Errorf("%d flats over %d systems: every system carries the key",
			flats, len(lay.Systems))
	}
}
