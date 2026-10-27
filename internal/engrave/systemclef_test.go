/*
 * systemclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A clef change is still in force on the next system, and the sign at the
// head of that system says so.
//
// Drawn from the part's own clef instead, the left hand of Danse Macabre
// carried a bass clef at the head of every system while its notes were
// placed in the treble — the sign saying one thing and the notes another,
// which is worse than either being wrong on its own.
func TestTheClefInForceOpensEverySystem(t *testing.T) {
	treble := model.Treble
	var measures []*model.Measure
	for i := 0; i < 40; i++ {
		m := &model.Measure{Voices: []*model.Voice{{
			Elements: []model.Element{&model.Rest{Dur: model.Duration{Num: 1, Den: 1}}},
		}}}
		if i == 1 {
			// Up into the treble at bar two and there it stays.
			m.Clef = &treble
		}
		measures = append(measures, m)
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Bass, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: measures,
		}},
	}

	// The only bass clef in the piece is the one it opens with. Every
	// system after the change heads with the treble, so forty bars over
	// several systems draw one bass clef and a treble on each of them.
	bass, treble2 := 0, 0
	for _, g := range Engrave(s, DefaultOptions()).Glyphs {
		switch g.Name {
		case "fClef":
			bass++
		case "gClef":
			treble2++
		}
	}
	if bass != 1 {
		t.Errorf("%d bass clefs drawn, want the one the piece opens with", bass)
	}
	if treble2 < 2 {
		t.Errorf("%d treble clefs drawn, want one at the change and one at each system after it", treble2)
	}
}
