/*
 * instrument.go
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

	"github.com/stricaud/gscore/internal/model"
)

// Working out what a staff is, when the file does not say.
//
// NWCTXT records the instrument as a General MIDI patch number and that is
// read straight. The 1.7x binary format must hold it somewhere too, but the
// record it is in is not one this reader has identified: the object type that
// looked like it carries a patch does not appear in the files to hand at all,
// so every staff of a 1.7x file arrives with no instrument and plays as a
// piano. Mozart's Requiem as five pianos is not a reading of the file, it is
// a failure to read it.
//
// Until that record is found, the staff's name is used. This is less of a
// guess than it sounds: people name a staff after what plays it, which is why
// the name is there, and "Soprano Solo", "Tenor" and "1st violins" are not
// ambiguous to anybody. A name that means nothing here leaves the staff as it
// was, so a wrong guess is never made in place of no guess.
//
// This runs only where nothing else has set an instrument, so a file that
// does say is never overridden.

// instrumentFromName guesses what plays a staff from what it is called.
func instrumentFromName(name string) (model.Instrument, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return 0, false
	}
	// Longest first, so "bass clarinet" is a clarinet and not a double bass,
	// and "english horn" is not a french one.
	for _, e := range nameHints {
		if strings.Contains(n, e.word) {
			return e.inst, true
		}
	}
	return 0, false
}

var nameHints = []struct {
	word string
	inst model.Instrument
}{
	{"bass clarinet", model.Clarinet},
	{"english horn", model.EnglishHorn},
	{"french horn", model.FrenchHorn},
	{"alto sax", model.AltoSax},
	{"tenor sax", model.TenorSax},
	{"soprano sax", model.SopranoSax},
	{"double bass", model.Contrabass},
	{"contrabass", model.Contrabass},
	{"string bass", model.Contrabass},
	{"acoustic bass", model.AcousticBass},
	{"electric bass", model.AcousticBass},
	{"harpsichord", model.Harpsichord},
	{"glockenspiel", model.Glockenspiel},
	{"vibraphone", model.Vibraphone},
	{"timpani", model.Timpani},
	{"trombone", model.Trombone},
	{"trumpet", model.Trumpet},
	{"clarinet", model.Clarinet},
	{"bassoon", model.Bassoon},
	{"piccolo", model.Piccolo},
	{"recorder", model.Recorder},
	{"marimba", model.Marimba},
	{"celesta", model.Celesta},
	{"accordion", model.Accordion},
	{"harmonica", model.Harmonica},
	{"guitar", model.AcousticGuitar},
	{"violin", model.Violin},
	{"viola", model.Viola},
	{"cello", model.Cello},
	{"violoncello", model.Cello},
	{"organ", model.ChurchOrgan},
	{"strings", model.StringEnsemble},
	{"flute", model.Flute},
	{"oboe", model.Oboe},
	{"harp", model.Harp},
	{"tuba", model.Tuba},
	{"piano", model.AcousticGrandPiano},

	// Voices. A choral part is sung, and the nearest General MIDI has to a
	// choir is the one called that; "aahs" is not a description of Mozart
	// but it is a great deal closer than a grand piano.
	{"soprano", model.ChoirAahs},
	{"mezzo", model.ChoirAahs},
	{"contralto", model.ChoirAahs},
	{"treble", model.ChoirAahs},
	{"descant", model.ChoirAahs},
	{"alto", model.ChoirAahs},
	{"tenor", model.ChoirAahs},
	{"baritone", model.ChoirAahs},
	{"bariton", model.ChoirAahs},
	{"bass", model.ChoirAahs},
	{"voice", model.VoiceOohs},
	{"vocal", model.VoiceOohs},
	{"choir", model.ChoirAahs},
	{"chorus", model.ChoirAahs},
	{"satb", model.ChoirAahs},
}

// nameInstruments fills in whatever the file did not say.
func nameInstruments(s *model.Score) {
	if s == nil {
		return
	}
	for _, p := range s.Parts {
		if p.Instrument != model.AcousticGrandPiano {
			continue // the file said, and the file wins
		}
		if inst, ok := instrumentFromName(p.Name); ok {
			p.Instrument = inst
		}
	}
}
