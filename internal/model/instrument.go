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

// InstrumentNamed guesses the instrument from the word written over the
// staff.
//
// Two kinds of file need this. Some say which instrument a staff is only in
// its name — Finale names a code from a library that is not in the file — and
// some say one thing in the name and another in the MIDI program, which is
// how a part called "Voice" comes out sounding like an oboe: its program is
// 69, and 69 is the oboe. The name is what a musician reads, so where the two
// disagree the name is the one to believe.
//
// French, Italian and German names are tried as well as English ones. A
// saxophone that plays back as a piano is a worse answer than a guess.
//
// The order matters, because these are substrings and the first match wins.
// The saxophones come before the voices, so that a soprano saxophone is not a
// soprano; the voices come before the instruments, so that a chorus — "coro"
// — is not a horn, "cor" being inside it.
//
// What is deliberately not here is "alto" on its own. In an Italian or French
// score it is the viola and in a vocal one it is a singer, and nothing in the
// word says which. A reader that knows what sort of score it is reading can
// decide for itself; this cannot, and a confident wrong answer is worse than
// none.
func InstrumentNamed(name string) (Instrument, bool) {
	if name == "" {
		return 0, false
	}
	plain := strings.ToLower(strings.TrimSpace(name))
	for from, to := range map[string]string{
		"é": "e", "è": "e", "ê": "e", "û": "u", "ù": "u", "ô": "o",
		"î": "i", "à": "a", "ç": "c", "œ": "oe", "ü": "u", "ä": "a", "ö": "o",
	} {
		plain = strings.ReplaceAll(plain, from, to)
	}
	if inst, ok := InstrumentFromName(plain); ok {
		return inst, true
	}
	for _, alias := range []struct {
		match string
		inst  Instrument
	}{
		// The saxophones first: a soprano saxophone is not a soprano.
		{"soprano sax", SopranoSax},
		{"alto sax", AltoSax},
		{"tenor sax", TenorSax},
		{"baritone sax", BaritoneSax},
		{"saxophone soprano", SopranoSax},
		{"saxophone alto", AltoSax},
		{"saxophone tenor", TenorSax},
		{"saxophone baryton", BaritoneSax},

		// Then the voices, ahead of anything they sit inside.
		{"soprano solo", VoiceOohs},
		{"alto solo", VoiceOohs},
		{"tenore solo", VoiceOohs},
		{"tenor solo", VoiceOohs},
		{"basso solo", VoiceOohs},
		{"bass solo", VoiceOohs},
		{"voice", VoiceOohs},
		{"voix", VoiceOohs},
		{"vocal", VoiceOohs},
		{"singstimme", VoiceOohs},
		{"gesang", VoiceOohs},
		{"canto", VoiceOohs},
		{"chant", VoiceOohs},
		{"choir", ChoirAahs},
		{"chorus", ChoirAahs},
		{"choeur", ChoirAahs},
		{"coro", ChoirAahs},
		{"soprano", VoiceOohs},

		// Then the instruments.
		{"flute", Flute},
		{"piccolo", Piccolo},
		{"hautbois", Oboe},
		{"oboe", Oboe},
		{"clarinette basse", Clarinet},
		{"clarinette", Clarinet},
		{"clarinet", Clarinet},
		// The big bassoons before the small ones, and both before anything
		// that is merely a bass: "contrabassoon" has "contrabass" inside it.
		{"contrabassoon", Bassoon},
		{"contrebasson", Bassoon},
		{"controfagotto", Bassoon},
		{"contrafagotto", Bassoon},
		{"basson", Bassoon},
		{"bassoon", Bassoon},
		{"fagotto", Bassoon},
		{"trompette", Trumpet},
		{"trumpet", Trumpet},
		{"cor", FrenchHorn},
		{"horn", FrenchHorn},
		{"trombone", Trombone},
		{"tuba", Tuba},

		// The cellos and the double basses before the violins. Every one of
		// their names has "violon" inside it — violoncello, violoncelle,
		// violoncelli — so matched the other way round a cello came out a
		// violin, which is what this list did for as long as it has existed.
		// Beethoven's ninth has a staff called Violoncelli on it.
		{"violoncell", Cello},
		{"violonchelo", Cello},
		{"cello", Cello},
		{"contrabbass", Contrabass},
		{"contrabass", Contrabass},
		{"kontrabass", Contrabass},
		{"double bass", Contrabass},
		{"violon", Violin},
		{"violin", Violin},
		{"viola", Viola},
		{"viole", Viola},
		{"contrebasse", Contrabass},
		{"timbales", Timpani},
		{"timpani", Timpani},
		{"orgue", ChurchOrgan},
		{"piano", AcousticGrandPiano},
	} {
		if strings.Contains(plain, alias.match) {
			return alias.inst, true
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
