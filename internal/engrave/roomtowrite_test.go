/*
 * roomtowrite_test.go
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

/*
 * Room at the end of a bar that is only half written.
 *
 * Two beats written into a bar of four are two beats and a gap, and the gap
 * is where the next two go. Spaced by what is in the bar, that gap is a
 * sliver between the last notehead and the barline, and putting the caret
 * in it means aiming at something a space wide.
 *
 * For writing, not for printing. A short bar in a printed score is short on
 * purpose — a pickup, or the last bar of a piece — so the option is off
 * unless the editor asks for it.
 */

// halfWritten is a bar of four with two quarter notes in it, then a bar
// with its four.
func halfWritten(beats int) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	var els []model.Element
	for i := 0; i < beats; i++ {
		els = append(els, &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		})
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: els}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)
	return s
}

// barWidth is how wide the first bar came out: from the start of the music
// to the barline that closes it.
func barWidth(l *Layout) float64 {
	if len(l.Barlines) == 0 {
		return 0
	}
	first := l.Barlines[0]
	left := first.X
	for _, g := range l.Glyphs {
		if g.X < left {
			left = g.X
		}
	}
	return first.X - left
}

func roomOpts(on bool) Options {
	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	o.RoomToWrite = on
	return o
}

func TestAHalfWrittenBarKeepsRoomForTheRestOfIt(t *testing.T) {
	half := halfWritten(2)

	tight := barWidth(Engrave(half, roomOpts(false)))
	roomy := barWidth(Engrave(half, roomOpts(true)))
	if roomy <= tight {
		t.Fatalf("the bar is %v wide with room to write and %v without: "+
			"there is nowhere to put the caret for the next two beats",
			roomy, tight)
	}

	// And the room is the room the missing music would take, near enough:
	// two quarters' worth, not a token gap.
	full := barWidth(Engrave(halfWritten(4), roomOpts(false)))
	if roomy < tight+(full-tight)/2 {
		t.Errorf("a half-written bar is %v wide where a full one is %v: "+
			"the room is too small to aim at", roomy, full)
	}
}

// A bar with all its beats in it is spaced as it always was, printed or
// not: there is nothing missing to leave room for.
func TestAFullBarIsSpacedAsItAlwaysWas(t *testing.T) {
	full := halfWritten(4)
	if a, b := barWidth(Engrave(full, roomOpts(false))),
		barWidth(Engrave(full, roomOpts(true))); a != b {
		t.Errorf("a full bar is %v wide while writing and %v when printed", b, a)
	}
}

// Off unless asked for, so a printed pickup bar is still a pickup bar.
func TestPrintingKeepsTheTightSpacing(t *testing.T) {
	if DefaultOptions().RoomToWrite {
		t.Error("room to write is on by default, so printed scores get it too")
	}
}

// Room nobody can click in is no room at all.
//
// A bar reached as far as its music and no further, so the gap at the end
// of a half-written bar was not part of the bar: a click in it found
// nothing under the pointer. The only way to put the caret after the last
// note was to click the note itself.
func TestAHalfWrittenBarCanBeClickedUpToItsBarline(t *testing.T) {
	lay := Engrave(halfWritten(2), roomOpts(true))
	if len(lay.Hits) == 0 || len(lay.Barlines) == 0 {
		t.Fatal("the bar was not drawn")
	}
	hit, bar := lay.Hits[0], lay.Barlines[0]
	if hit.XMax < bar.X {
		t.Errorf("the bar can be clicked as far as %v and its barline is at %v: "+
			"the gap where the next two beats go is not part of the bar",
			hit.XMax, bar.X)
	}
}

// A full bar keeps the old reach: the white space after a whole-bar rest is
// the four beats it lasts, and a click there means "write over that".
func TestAFullBarStopsAtItsMusic(t *testing.T) {
	lay := Engrave(halfWritten(4), roomOpts(true))
	if len(lay.Hits) == 0 || len(lay.Barlines) == 0 {
		t.Fatal("the bar was not drawn")
	}
	if hit, bar := lay.Hits[0], lay.Barlines[0]; hit.XMax >= bar.X {
		t.Errorf("a full bar reaches to %v, past its last note and up to the "+
			"barline at %v", hit.XMax, bar.X)
	}
}
