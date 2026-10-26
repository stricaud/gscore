/*
 * nwc_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package nwc

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A staff that has a bar off writes one whole rest, whatever the meter — so
// a three-four bar of silence says "whole rest" and means "this bar". Read
// literally it held the bar open for a whole note's worth of time, and since
// the staves of a NoteWorthy score share their barlines, everything written
// under it played late.
func TestABarOfSilenceInTextLastsOneBar(t *testing.T) {
	const text = `!NoteWorthyComposer(2.0)
|AddStaff|Name:"Right"
|Clef|Type:Treble
|TimeSig|Signature:3/4
|Rest|Dur:Whole
|Bar
|Note|Dur:4th|Pos:0
|Note|Dur:4th|Pos:1
|Note|Dur:4th|Pos:2
|Bar
!NoteWorthyComposer-End
`
	score, err := Import(strings.NewReader(text))
	if err != nil {
		t.Fatalf("%v", err)
	}
	bar := model.Duration{Num: 3, Den: 4}
	if got := score.MeasureDuration(0); got.Cmp(bar) != 0 {
		t.Errorf("the silent bar lasts %v, want the %v its meter says", got, bar)
	}
	if got := score.MeasureDuration(1); got.Cmp(bar) != 0 {
		t.Errorf("the bar of notes lasts %v, want %v", got, bar)
	}
}
