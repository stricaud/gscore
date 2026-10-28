/*
 * sound_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading what the instrument is, not only what plays it.
 *
 * MuseScore writes both on every part: <instrumentId> says it is a
 * contrabassoon, <program> says to play it with a bassoon, because General
 * MIDI has no contrabassoon. gscore read the program and threw the rest
 * away, so Beethoven's Controfagotto arrived as a bassoon and nothing in the
 * score said otherwise.
 */

package musescore

import "testing"

func withInstrument(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part>
      <Staff id="1"></Staff>
      <trackName>Controfagotto</trackName>
      <Instrument>` + body + `</Instrument>
    </Part>
    <Staff id="1"><Measure><voice>
      <Rest><durationType>measure</durationType></Rest>
    </voice></Measure></Staff>
  </Score>
</museScore>`
}

func TestTheInstrumentIdIsRead(t *testing.T) {
	s := read(t, withInstrument(
		`<longName>Controfagotto</longName>
		 <instrumentId>wind.reed.contrabassoon</instrumentId>
		 <Channel><program value="70"/></Channel>`))

	p := s.Parts[0]
	if p.Sound != "wind.reed.contrabassoon" {
		t.Errorf("the part is written for %q, and the file says contrabassoon", p.Sound)
	}
	// What plays it is still what the file said to play it with.
	if p.Instrument.String() != "bassoon" {
		t.Errorf("it is played with %q, and the file asked for a bassoon",
			p.Instrument.String())
	}
	// But what it *is* decides the range.
	if p.SoundName() != "contrabassoon" {
		t.Errorf("it is called %q", p.SoundName())
	}
	r, ok := p.Reach()
	if !ok {
		t.Fatal("it has no range")
	}
	if r.Low >= 34 {
		t.Errorf("it reaches down to %d, which is the bassoon's bottom: the "+
			"program answered instead of the instrument", r.Low)
	}
}

// A file that names the instrument and no program can still be played: the
// name is enough to pick something.
func TestASoundWithNoProgramPicksOne(t *testing.T) {
	s := read(t, withInstrument(
		`<instrumentId>strings.violin</instrumentId>`))
	if got := s.Parts[0].Instrument.String(); got != "violin" {
		t.Errorf("a part named as a violin is played with %q", got)
	}
}

// And a program that is there wins, because the file meant it.
func TestAProgramInTheFileIsNotOverruled(t *testing.T) {
	s := read(t, withInstrument(
		`<instrumentId>strings.violin</instrumentId>
		 <Channel><program value="40"/></Channel>`))
	if got := s.Parts[0].Instrument; got != 40 {
		t.Errorf("the part is played with program %d, and the file asked for 40", got)
	}
}

func TestAFileThatNamesNoInstrumentSaysNothing(t *testing.T) {
	s := read(t, withInstrument(`<longName>Something</longName>`))
	if s.Parts[0].Sound != "" {
		t.Errorf("a sound was invented: %q", s.Parts[0].Sound)
	}
}
