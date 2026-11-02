/*
 * numbering_test.go
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

// The room kept beside a bar number agrees with whether one is printed.
//
// They were two pieces of arithmetic that had to match — one deciding
// where a number goes, the other how far to push a rehearsal mark clear of
// it — and the moment a score could choose its own numbering they stopped
// matching: a mark shoved aside for a number that was not there, or
// printed on top of one that was. They are one function now, and this is
// what says so.
func TestTheRoomBesideANumberAgreesWithWhetherThereIsOne(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	for len(p.Measures) < 14 {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{}},
		})
	}

	for _, every := range []int{0, 1, 3, 4} {
		s.Numbering.Every = every
		for _, mode := range []Mode{Strip, Paginated} {
			o := DefaultOptions()
			o.Mode = mode
			e := &engraver{opt: o, score: s}
			for mi := 0; mi < len(p.Measures); mi++ {
				_, want := e.numberedAt(mi, 0)
				got := e.numberWidth(mi, 0) > 0
				if got != want {
					t.Errorf("every=%d mode=%v bar %d: numbered=%v but room=%v",
						every, mode, mi, want, got)
				}
			}
		}
	}
}

// Numbering off means no numbers and no room kept for them.
func TestNumberingOffPrintsNothing(t *testing.T) {
	s := model.NewScore()
	s.Numbering.Every = 4
	o := DefaultOptions()
	o.ShowMeasureNumbers = false
	e := &engraver{opt: o, score: s}
	for mi := 0; mi < 8; mi++ {
		if _, ok := e.numberedAt(mi, 0); ok {
			t.Errorf("bar %d is numbered with the numbers turned off", mi)
		}
		if w := e.numberWidth(mi, 0); w != 0 {
			t.Errorf("bar %d keeps %.2f spaces for a number that is not drawn", mi, w)
		}
	}
}
