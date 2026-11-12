/*
 * midbarkey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A key that turns in the middle of a bar.
 *
 * The double bar in the middle of a measure is ordinary in opera and in
 * anything written to a cue: the scene turns, the key turns with it, and
 * the bar it lands in is left as long as it was. Written at the head of the
 * bar instead, every note before the turn is read in the new key.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// turning draws a bar of four notes, with the key turning before the third
// where a key is given.
func turning(t *testing.T, to *model.KeySignature) *Layout {
	t.Helper()
	s := model.NewScore()
	p := s.Parts[0]
	p.Key = model.KeySignature{Fifths: -3}
	m := p.Measures[0]
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		if i == 2 && to != nil {
			n.Key = to
		}
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	return Engrave(s, DefaultOptions())
}

// signs counts the accidentals drawn as a signature rather than on a note:
// the ones standing in a row at the same height as each other.
func signs(lay *Layout) (naturals, flats, sharps int) {
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "accidentalNatural":
			naturals++
		case "accidentalFlat":
			flats++
		case "accidentalSharp":
			sharps++
		}
	}
	return
}

func TestAKeyTurnsInTheMiddleOfABar(t *testing.T) {
	two := model.KeySignature{Fifths: 2}
	plain := turning(t, nil)
	turned := turning(t, &two)

	n0, f0, s0 := signs(plain)
	n1, f1, s1 := signs(turned)
	// Three flats cancelled, two sharps written.
	if n1-n0 != 3 {
		t.Errorf("%d naturals were drawn to cancel three flats", n1-n0)
	}
	if s1-s0 != 2 {
		t.Errorf("%d sharps were drawn for a key of two", s1-s0)
	}
	if f1 != f0 {
		t.Errorf("the flats changed by %d, and the old key is cancelled "+
			"rather than reprinted", f1-f0)
	}
}

func TestATurnToTheSameKeySaysNothing(t *testing.T) {
	// A change to the key already in force changes nothing, and the sign
	// for it says nothing — the same rule the clef follows.
	same := model.KeySignature{Fifths: -3}
	plain := turning(t, nil)
	turned := turning(t, &same)
	n0, f0, s0 := signs(plain)
	n1, f1, s1 := signs(turned)
	if n0 != n1 || f0 != f1 || s0 != s1 {
		t.Errorf("a turn to the key already in force drew %d/%d/%d against "+
			"%d/%d/%d", n1, f1, s1, n0, f0, s0)
	}
}

func TestTheTurnMakesRoomForItself(t *testing.T) {
	// The signature is written in front of the note it belongs to, so the
	// bar has to be wider for it. Without the room it is drawn over the
	// notehead before it.
	two := model.KeySignature{Fifths: 2}
	plain := turning(t, nil)
	turned := turning(t, &two)
	if turned.Width <= plain.Width {
		t.Errorf("the bar is %g wide with a key change in it and %g "+
			"without, so nothing made room for the signature",
			turned.Width, plain.Width)
	}
}

func TestTheKeyHoldsPastTheBarline(t *testing.T) {
	// A key turned partway through a bar is still turned at the next
	// barline. Read only from the heads of bars, the music after it goes
	// back to the key the staff opened with.
	s := model.NewScore()
	p := s.Parts[0]
	p.Key = model.KeySignature{Fifths: -3}
	two := model.KeySignature{Fifths: 2}
	first := p.Measures[0]
	for i := 0; i < 2; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 2},
		}
		if i == 1 {
			n.Key = &two
		}
		first.Voices[0].Elements = append(first.Voices[0].Elements, n)
	}
	p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{
		Elements: []model.Element{&model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}},
	}}})
	if got := p.KeyAt(1); got.Fifths != 2 {
		t.Errorf("the bar after the turn is in %d, want the two sharps it "+
			"was turned to", got.Fifths)
	}
	if got := p.KeyAt(0); got.Fifths != -3 {
		t.Errorf("the bar the turn happens in opens in %d, want the three "+
			"flats it began with", got.Fifths)
	}
}
