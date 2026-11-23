/*
 * lyricsclear_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// The words go under a slur hung off a low second voice, not through it.
// The test suite's two voices with lyrics: the lower voice stems down from a
// ledger-line chord and is slurred under, and the curve came down through
// "of Voice2".
func TestTheWordsGoUnderASlurBelowTheStaff(t *testing.T) {
	s, err := load("../format/musicxml/testdata/suite/42a-MultiVoice-TwoVoicesOnStaff-Lyrics.musicxml")
	if err != nil {
		t.Fatal(err)
	}
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	if len(lay.Curves) == 0 {
		t.Fatal("no curves drawn")
	}
	for _, c := range lay.Curves {
		for i := 0; i <= 20; i++ {
			u := float64(i) / 20
			x := (1-u)*(1-u)*c.X1 + 2*u*(1-u)*c.CX + u*u*c.X2
			y := (1-u)*(1-u)*c.Y1 + 2*u*(1-u)*c.CY + u*u*c.Y2
			for _, tx := range lay.Texts {
				if tx.Y < y-3 || tx.X > x+1 || tx.X+float64(len(tx.S))*tx.Size*0.5 < x-1 {
					continue
				}
				if top := tx.Y - tx.Size*0.8; y > top && y < tx.Y {
					t.Errorf("the curve at (%.2f, %.2f) runs through %q", x, y, tx.S)
				}
			}
		}
	}
}
