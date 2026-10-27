/*
 * clefat_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The clef in force at a bar, when the change was written inside an earlier
 * one.
 *
 * A hand that drops into the bass partway through a bar is still in the bass
 * at the next barline. Read only from the heads of bars, that was forgotten
 * at every one: the music after such a change was placed against whatever
 * clef the staff last opened a bar with. In Danse Macabre it put the left
 * hand's chords nine to sixteen steps above a staff that is eight steps
 * tall, on towers of ledger lines, for pages at a time.
 */

package model

import "testing"

func TestAClefChangedInsideABarIsStillInForceAtTheNext(t *testing.T) {
	quarter := Duration{Num: 1, Den: 4}
	bass := Bass
	treble := Treble

	note := func(clef *Clef) Element {
		n := &Note{Pitch: Pitch{Step: StepG, Octave: 4}, Dur: quarter}
		n.Clef = clef
		return n
	}
	bar := func(els ...Element) *Measure {
		return &Measure{Voices: []*Voice{{Elements: els}}}
	}

	// Opens in the treble, drops into the bass halfway through bar one,
	// and says nothing at all in bars two and three.
	p := &Part{Clef: Treble, Measures: []*Measure{
		bar(note(nil), note(&bass)),
		bar(note(nil), note(nil)),
		bar(note(nil), note(&treble)),
		bar(note(nil), note(nil)),
	}}

	for i, want := range []Clef{Treble, Bass, Bass, Treble} {
		if got := p.ClefAt(i); got != want {
			t.Errorf("bar %d is read in the %v, want the %v", i+1, got, want)
		}
	}
}

// Where two lines on one staff each move the clef, the one that counts at
// the barline is whichever moved it last.
func TestTheLastClefInABarIsTheOneThatCarriesOn(t *testing.T) {
	quarter := Duration{Num: 1, Den: 4}
	half := Duration{Num: 1, Den: 2}
	bass, alto := Bass, Alto

	early := &Note{Pitch: Pitch{Step: StepG, Octave: 4}, Dur: half}
	early.Clef = &alto
	late := &Note{Pitch: Pitch{Step: StepG, Octave: 4}, Dur: quarter}
	late.Clef = &bass

	p := &Part{Clef: Treble, Measures: []*Measure{
		{Voices: []*Voice{
			{Elements: []Element{early, &Rest{Dur: half}}},
			{Elements: []Element{&Rest{Dur: half}, &Rest{Dur: quarter}, late}},
		}},
		{Voices: []*Voice{{Elements: []Element{&Rest{Dur: half}}}}},
	}}
	if got := p.ClefAt(1); got != Bass {
		t.Errorf("the next bar is read in the %v, want the %v the later line left it in", got, Bass)
	}
}
