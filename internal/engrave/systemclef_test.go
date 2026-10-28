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
	"math"
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

// barOfRests is a part of plain whole-bar rests, with a clef written on the
// first element of bar two.
func barOfRests(change *model.Clef, bars int) *model.Score {
	var measures []*model.Measure
	for i := 0; i < bars; i++ {
		r := &model.Rest{Dur: model.Duration{Num: 1, Den: 1}}
		if i == 1 && change != nil {
			r.Clef = change
		}
		measures = append(measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{r}}},
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Bass, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: measures,
		}},
	}
}

func clefsDrawn(lay *Layout) (bass, treble int) {
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "fClef":
			bass++
		case "gClef":
			treble++
		}
	}
	return
}

// A change to the clef the staff is already in changes nothing, and the sign
// for it says nothing.
//
// MuseScore writes the clef again at the head of each movement of a sonata,
// where the staff is in that clef already. Read as a change, the Presto of
// the Moonlight opened with a bass clef at the head of the system and a
// second bass clef printed against its first note, one behind the other, as
// though something had happened between them.
func TestAClefChangeToTheClefAlreadyInForceIsNotDrawn(t *testing.T) {
	bass := model.Bass
	lay := Engrave(barOfRests(&bass, 4), DefaultOptions())
	if n, _ := clefsDrawn(lay); n != 1 {
		t.Errorf("%d bass clefs drawn, want the one at the head of the staff", n)
	}
}

// And it takes up no room, which is the other half of not being drawn.
func TestAClefThatIsNotDrawnTakesNoRoom(t *testing.T) {
	bass := model.Bass
	with := Engrave(barOfRests(&bass, 4), DefaultOptions())
	without := Engrave(barOfRests(nil, 4), DefaultOptions())
	if math.Abs(with.Width-without.Width) > 0.01 {
		t.Errorf("the staff is %.2f wide with the clef and %.2f without it: "+
			"room was booked for a sign that is never printed",
			with.Width, without.Width)
	}
}

// A change that is a change is still drawn, and still given room.
func TestARealClefChangeIsStillDrawn(t *testing.T) {
	treble := model.Treble
	lay := Engrave(barOfRests(&treble, 4), DefaultOptions())
	bass, drawn := clefsDrawn(lay)
	if bass < 1 {
		t.Errorf("%d bass clefs, want the one the staff opens with", bass)
	}
	if drawn < 1 {
		t.Error("the change up to the treble was not drawn at all")
	}
	plain := Engrave(barOfRests(nil, 4), DefaultOptions())
	if lay.Width <= plain.Width {
		t.Errorf("the staff is %.2f wide with the change and %.2f without it: "+
			"the sign was given no room", lay.Width, plain.Width)
	}
}
