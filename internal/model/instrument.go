/*
 * instrument.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// Instrument is what a staff sounds as, as a General MIDI program number.
//
// gscore 0.0.9 had this already — Staff_t carried a midi_instrument and the
// Staff menu had "Set instrument" — but nothing ever played it. General MIDI
// numbering is used because it is what MusicXML, NoteWorthy and every MIDI
// file already speak, so an instrument survives being exported and comes back.
type Instrument int

// The General MIDI programs gscore names. Any number from 0 to 127 is valid;
// these are the ones worth having a word for.
const (
	AcousticGrandPiano Instrument = 0
	BrightPiano        Instrument = 1
	ElectricPiano      Instrument = 4
	Harpsichord        Instrument = 6
	Celesta            Instrument = 8
	Glockenspiel       Instrument = 9
	MusicBox           Instrument = 10
	Vibraphone         Instrument = 11
	Marimba            Instrument = 12
	ChurchOrgan        Instrument = 19
	Accordion          Instrument = 21
	Harmonica          Instrument = 22
	AcousticGuitar     Instrument = 24
	ElectricGuitar     Instrument = 27
	AcousticBass       Instrument = 32
	Violin             Instrument = 40
	Viola              Instrument = 41
	Cello              Instrument = 42
	Contrabass         Instrument = 43
	Harp               Instrument = 46
	Timpani            Instrument = 47
	StringEnsemble     Instrument = 48
	ChoirAahs          Instrument = 52
	VoiceOohs          Instrument = 53
	Trumpet            Instrument = 56
	Trombone           Instrument = 57
	Tuba               Instrument = 58
	FrenchHorn         Instrument = 60
	SopranoSax         Instrument = 64
	AltoSax            Instrument = 65
	TenorSax           Instrument = 66
	BaritoneSax        Instrument = 67
	Oboe               Instrument = 68
	EnglishHorn        Instrument = 69
	Bassoon            Instrument = 70
	Clarinet           Instrument = 71
	Piccolo            Instrument = 72
	Flute              Instrument = 73
	Recorder           Instrument = 74
	PanFlute           Instrument = 75
	Ocarina            Instrument = 79
)

// instrumentNames gives the names gscore shows and accepts.
var instrumentNames = map[Instrument]string{
	AcousticGrandPiano: "piano", BrightPiano: "bright piano",
	ElectricPiano: "electric piano", Harpsichord: "harpsichord",
	Celesta: "celesta", Glockenspiel: "glockenspiel", MusicBox: "music box",
	Vibraphone: "vibraphone", Marimba: "marimba", ChurchOrgan: "organ",
	Accordion: "accordion", Harmonica: "harmonica",
	AcousticGuitar: "guitar", ElectricGuitar: "electric guitar",
	AcousticBass: "bass", Violin: "violin", Viola: "viola", Cello: "cello",
	Contrabass: "contrabass", Harp: "harp", Timpani: "timpani",
	StringEnsemble: "strings", ChoirAahs: "choir", VoiceOohs: "voice",
	Trumpet: "trumpet", Trombone: "trombone", Tuba: "tuba",
	FrenchHorn: "french horn", SopranoSax: "soprano sax", AltoSax: "alto sax",
	TenorSax: "tenor sax", BaritoneSax: "baritone sax",
	Oboe: "oboe", EnglishHorn: "english horn",
	Bassoon: "bassoon", Clarinet: "clarinet", Piccolo: "piccolo",
	Flute: "flute", Recorder: "recorder", PanFlute: "pan flute",
	Ocarina: "ocarina",
}

func (i Instrument) String() string {
	if n, ok := instrumentNames[i]; ok {
		return n
	}
	if i < 0 || i > 127 {
		return "unknown"
	}
	return "program " + itoa(int(i))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// InstrumentFromName reads an instrument written the way a musician names it.
func InstrumentFromName(s string) (Instrument, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	for k, v := range instrumentNames {
		if v == s {
			return k, true
		}
	}
	return 0, false
}

// Family groups instruments by how they are sounded, which is what a
// synthesiser needs to know: the difference between a flute and a violin is
// how the note starts and whether it keeps going, not which note it is.
type Family int

// The families gscore distinguishes.
const (
	// Keyboard and plucked instruments: struck, then dying away.
	FamilyPlucked Family = iota
	// Bowed strings: swelling, sustained.
	FamilyBowed
	// Flutes and whistles: breathy, nearly pure, sustained.
	FamilyFlute
	// Reeds: sustained with a harder edge.
	FamilyReed
	// Brass: sustained, bright, with a firm attack.
	FamilyBrass
	// Tuned percussion: struck, decaying fast, bell-like.
	FamilyTuned
	// Organ and voice: steady, no decay.
	FamilySustained
	// Piano: struck and left to ring. Kept apart from the plucked strings
	// it used to be counted among, because it does not sound like one. A
	// guitar is bright at the moment it is picked and gone very soon after;
	// a piano note is softer at the front and rings on for as long as the
	// written note lasts. Played as a pluck it came out buzzing and then
	// cut off halfway through the note, which is the first thing anybody
	// hears, since a piano is what most scores open as.
	FamilyPiano
)

var familyNames = [...]string{
	"plucked", "bowed", "flute", "reed", "brass", "tuned", "sustained",
	"piano",
}

func (f Family) String() string {
	if f < FamilyPlucked || int(f) >= len(familyNames) {
		return "unknown"
	}
	return familyNames[f]
}

// Family returns which family an instrument belongs to, following the General
// MIDI groupings.
func (i Instrument) Family() Family {
	switch {
	case i >= 0 && i <= 7: // pianos
		return FamilyPiano
	case i >= 8 && i <= 15: // chromatic percussion
		return FamilyTuned
	case i >= 16 && i <= 23: // organs
		return FamilySustained
	case i >= 24 && i <= 39: // guitars and basses
		return FamilyPlucked
	case i >= 40 && i <= 47: // solo strings and harp
		if i == Harp || i == Timpani {
			return FamilyPlucked
		}
		return FamilyBowed
	case i >= 48 && i <= 51: // string ensembles
		return FamilyBowed
	case i >= 52 && i <= 55: // voices
		return FamilySustained
	case i >= 56 && i <= 63: // brass
		return FamilyBrass
	case i >= 64 && i <= 71: // reeds
		return FamilyReed
	case i >= 72 && i <= 79: // pipes
		return FamilyFlute
	}
	return FamilySustained
}
