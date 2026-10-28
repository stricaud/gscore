/*
 * sound_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a staff is written for, as against what it is played with.
 *
 * General MIDI has one bassoon and no contrabassoon, one choir and no
 * soprano, one string ensemble covering everything from a double bass to a
 * violin. A part read as a program number alone loses which instrument it
 * actually is — and the range, which is the one thing about an instrument a
 * composer cannot work out for themselves, is a property of the instrument
 * and not of the program.
 */

package model

import "testing"

func TestASoundIsNamedByWhatItIsNotByWhatPlaysIt(t *testing.T) {
	name, ok := SoundName("wind.reed.contrabassoon")
	if !ok || name != "contrabassoon" {
		t.Errorf("wind.reed.contrabassoon is called %q, want contrabassoon", name)
	}
	// And it is still played with the nearest program there is.
	if prog, ok := SoundProgram("wind.reed.contrabassoon"); !ok || prog != Bassoon {
		t.Errorf("it is played with %v, and General MIDI has nothing closer than a bassoon", prog)
	}
}

// The contrabassoon sounds an octave below the bassoon. Read as one, the
// Ninth's Controfagotto was reported as playing two hundred and fifty-eight
// notes below the bottom of the instrument, every one of which it plays in
// the middle of its range.
func TestTheContrabassoonReachesAnOctaveBelowTheBassoon(t *testing.T) {
	cb, ok := SoundReach("wind.reed.contrabassoon")
	if !ok {
		t.Fatal("gscore has no range for the contrabassoon")
	}
	bn, ok := SoundReach("wind.reed.bassoon")
	if !ok {
		t.Fatal("gscore has no range for the bassoon")
	}
	if cb.Low != bn.Low-12 {
		t.Errorf("the contrabassoon starts at %d and the bassoon at %d, "+
			"and it is an octave below one", cb.Low, bn.Low)
	}
}

// An id gscore has never seen is still a reed or a drum, and that is most of
// what is worth knowing.
func TestAnUnknownIdFallsBackToItsFamily(t *testing.T) {
	if !SoundIsPercussion("metal.cymbal.crash") {
		t.Error("a crash cymbal was not recognised as percussion")
	}
	if !SoundIsPercussion("drum.snare-drum.whatever") {
		t.Error("an unheard-of drum was not recognised as one")
	}
	if name, ok := SoundName("voice.soprano.lyric"); !ok || name != "soprano" {
		t.Errorf("a lyric soprano is called %q, want it read as a soprano", name)
	}
}

// Timpani are drums with pitches, and asking them for a range is a fair
// question where asking a bass drum is not.
func TestTunedDrumsAreStillAskedAboutTheirRange(t *testing.T) {
	if SoundIsPercussion("drum.timpani") {
		t.Error("the timpani were lumped in with the kit")
	}
	if _, ok := SoundReach("drum.timpani"); !ok {
		t.Error("the timpani have no range")
	}
	if _, ok := SoundReach("drum.bass-drum"); ok {
		t.Error("a bass drum was given a pitch range")
	}
}

// A section patch is not an instrument: a staff set to it may be the first
// violins or the double basses, and a range covering both says nothing.
func TestASectionPatchIsNamedButNotJudged(t *testing.T) {
	if name, ok := SoundName("strings.group"); !ok || name != "strings" {
		t.Errorf("strings.group is called %q", name)
	}
	if _, ok := SoundReach("strings.group"); ok {
		t.Error("a range was given for a whole string section")
	}
}

func TestAnIdNobodyKnowsSaysSo(t *testing.T) {
	if _, ok := SoundName("theremin.space"); ok {
		t.Error("gscore claimed to know what a theremin is")
	}
	if _, ok := SoundReach("theremin.space"); ok {
		t.Error("and gave it a range")
	}
}

// The part asks the sound first and the program only after it.
func TestAPartPrefersWhatItIsToWhatPlaysIt(t *testing.T) {
	p := &Part{Instrument: Bassoon, Sound: "wind.reed.contrabassoon"}
	r, ok := p.Reach()
	if !ok {
		t.Fatal("the part has no range")
	}
	if bn, _ := Bassoon.Reach(); r.Low >= bn.Low {
		t.Errorf("the part reaches down to %d, which is the bassoon's %d: "+
			"the program answered instead of the instrument", r.Low, bn.Low)
	}
	if p.SoundName() != "contrabassoon" {
		t.Errorf("the part is called %q", p.SoundName())
	}
}

// With no sound named, the program is all there is and is used.
func TestAPartWithNoSoundFallsBackToItsProgram(t *testing.T) {
	p := &Part{Instrument: Flute}
	r, ok := p.Reach()
	if !ok || r.Low != 60 {
		t.Errorf("a flute with no sound named came back as %+v, %v", r, ok)
	}
	if p.SoundName() != "flute" {
		t.Errorf("it is called %q", p.SoundName())
	}
}

// A sound gscore knows and has no range for is an answer, not a gap to fall
// through to the program's range.
func TestAKnownSoundWithNoRangeStopsTheQuestion(t *testing.T) {
	p := &Part{Instrument: Violin, Sound: "strings.group"}
	if _, ok := p.Reach(); ok {
		t.Error("a string section was judged by the violin's range")
	}
}

func TestADrumStaffIsUnpitchedEitherWay(t *testing.T) {
	if !(&Part{Clef: Percussion}).Unpitched() {
		t.Error("a percussion clef is not enough to say so")
	}
	if !(&Part{Sound: "drum.bass-drum"}).Unpitched() {
		t.Error("a file naming a bass drum is not enough to say so")
	}
	if (&Part{Sound: "drum.timpani"}).Unpitched() {
		t.Error("the timpani were taken for unpitched")
	}
}
