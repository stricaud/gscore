/*
 * sound.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// A sound is what a staff is written for, named the way MusicXML's standard
// instrument sounds name it: "wind.reed.contrabassoon", "voice.soprano",
// "drum.bass-drum". MuseScore writes one on every part and MusicXML carries
// the same list, so it survives being exported and comes back.
//
// It says something a General MIDI program number cannot. General MIDI has
// one bassoon and no contrabassoon, one choir and no soprano, one string
// ensemble covering everything from a double bass to a violin — so a part
// read as a program alone loses which instrument it actually is. Beethoven's
// Controfagotto came in as a bassoon and was reported as playing two hundred
// and fifty-eight notes below the bottom of one, every one of which a
// contrabassoon plays in the middle of its range.
//
// The program is still what it is played with, and is still read from the
// file. This is what it is, which is a different question.

// soundInfo is what gscore knows about one sound.
type soundInfo struct {
	// name is what to call it, where that is better than the program's
	// name: "contrabassoon" rather than "bassoon".
	name string
	// program is what to play it with, for a file that names a sound and
	// no program.
	program Instrument
	// reach is how far it plays. Zero means gscore does not know, and
	// nothing is claimed.
	reach    Reach
	hasReach bool
	// percussion says its staff is read by position on a kit rather than
	// by pitch, so there is no range to ask about.
	percussion bool
}

func pitched(name string, prog Instrument, low, high, easyLow, easyHigh int) soundInfo {
	return soundInfo{
		name: name, program: prog, hasReach: true,
		reach: Reach{Low: low, High: high, EasyLow: easyLow, EasyHigh: easyHigh},
	}
}

func unpitched(name string) soundInfo {
	return soundInfo{name: name, percussion: true}
}

// sounds is what gscore knows, by the id the file gives.
//
// Not the whole list, which runs to several hundred and most of which gscore
// has nothing to say about. An id that is not here is looked up again
// without its last piece — "wind.reed.clarinet.a" falls back to
// "wind.reed.clarinet", and "metal.cymbal.crash" to "metal" — so the table
// stays the size of what is actually known and still answers for the rest.
var sounds = map[string]soundInfo{
	"keyboard.piano":       pitched("piano", AcousticGrandPiano, 21, 108, 28, 96),
	"keyboard.harpsichord": pitched("harpsichord", Harpsichord, 29, 89, 29, 89),
	"keyboard.celesta":     pitched("celesta", Celesta, 60, 108, 60, 108),
	"keyboard.organ":       pitched("organ", ChurchOrgan, 36, 96, 36, 96),
	"keyboard.accordion":   pitched("accordion", Accordion, 41, 93, 45, 89),

	"wind.flutes.flute":         pitched("flute", Flute, 60, 98, 62, 91),
	"wind.flutes.flute.piccolo": pitched("piccolo", Piccolo, 74, 108, 76, 103),
	"wind.flutes.flute.alto":    pitched("alto flute", Flute, 55, 93, 57, 86),
	"wind.flutes.recorder":      pitched("recorder", Recorder, 72, 98, 72, 96),
	"wind.flutes.panpipes":      pitched("pan flute", PanFlute, 60, 96, 62, 93),
	"wind.flutes.ocarina":       pitched("ocarina", Ocarina, 69, 89, 69, 86),

	"wind.reed.oboe":              pitched("oboe", Oboe, 58, 93, 60, 86),
	"wind.reed.oboe.english-horn": pitched("english horn", EnglishHorn, 52, 81, 55, 76),
	// A clarinet in B flat, which is what "clarinet" means unqualified.
	"wind.reed.clarinet":           pitched("clarinet", Clarinet, 50, 94, 52, 84),
	"wind.reed.clarinet.bflat":     pitched("clarinet in B♭", Clarinet, 50, 94, 52, 84),
	"wind.reed.clarinet.a":         pitched("clarinet in A", Clarinet, 49, 93, 51, 83),
	"wind.reed.clarinet.eflat":     pitched("clarinet in E♭", Clarinet, 55, 99, 57, 91),
	"wind.reed.clarinet.bass":      pitched("bass clarinet", Clarinet, 38, 82, 40, 74),
	"wind.reed.bassoon":            pitched("bassoon", Bassoon, 34, 76, 38, 72),
	"wind.reed.contrabassoon":      pitched("contrabassoon", Bassoon, 22, 64, 26, 58),
	"wind.reed.saxophone.soprano":  pitched("soprano sax", SopranoSax, 56, 88, 58, 86),
	"wind.reed.saxophone.alto":     pitched("alto sax", AltoSax, 49, 80, 51, 77),
	"wind.reed.saxophone.tenor":    pitched("tenor sax", TenorSax, 44, 76, 46, 74),
	"wind.reed.saxophone.baritone": pitched("baritone sax", BaritoneSax, 37, 68, 39, 65),
	"wind.reed.harmonica":          pitched("harmonica", Harmonica, 60, 96, 62, 93),

	"brass.trumpet":        pitched("trumpet", Trumpet, 52, 86, 55, 82),
	"brass.trumpet.bflat":  pitched("trumpet in B♭", Trumpet, 52, 86, 55, 82),
	"brass.trumpet.c":      pitched("trumpet in C", Trumpet, 54, 86, 55, 82),
	"brass.trumpet.d":      pitched("trumpet in D", Trumpet, 54, 88, 57, 84),
	"brass.french-horn":    pitched("french horn", FrenchHorn, 35, 77, 43, 72),
	"brass.natural-horn":   pitched("natural horn", FrenchHorn, 35, 77, 43, 72),
	"brass.trombone":       pitched("trombone", Trombone, 40, 77, 43, 70),
	"brass.trombone.tenor": pitched("tenor trombone", Trombone, 40, 77, 43, 70),
	"brass.trombone.alto":  pitched("alto trombone", Trombone, 45, 82, 48, 74),
	"brass.trombone.bass":  pitched("bass trombone", Trombone, 34, 74, 38, 65),
	"brass.tuba":           pitched("tuba", Tuba, 26, 65, 29, 58),

	"strings.violin":     pitched("violin", Violin, 55, 100, 55, 88),
	"strings.viola":      pitched("viola", Viola, 48, 88, 48, 81),
	"strings.cello":      pitched("cello", Cello, 36, 84, 36, 76),
	"strings.contrabass": pitched("contrabass", Contrabass, 28, 67, 28, 55),
	"strings.harp":       pitched("harp", Harp, 23, 104, 28, 100),
	"strings.guitar":     pitched("guitar", AcousticGuitar, 40, 88, 40, 83),
	// A section rather than an instrument: it may be the first violins or
	// the double basses, and a range covering both would say nothing.
	"strings.group": {name: "strings", program: StringEnsemble},

	// Voices. General MIDI has one choir; which voice it is decides the
	// range, and the range is the whole question anybody asks about a
	// vocal line.
	// Choral ranges, not soloists': what a section of them can be asked
	// for night after night. A bass singing the F above middle C is high
	// and ordinary, and a table that called it impossible — as this one
	// did, with the Ninth's Basso ten notes over the top — is a table
	// nobody will believe the next time it says something.
	"voice.soprano":       pitched("soprano", ChoirAahs, 60, 84, 64, 77),
	"voice.mezzo-soprano": pitched("mezzo-soprano", ChoirAahs, 57, 79, 60, 74),
	"voice.alto":          pitched("alto", ChoirAahs, 53, 76, 57, 72),
	"voice.contralto":     pitched("contralto", ChoirAahs, 50, 74, 55, 69),
	"voice.tenor":         pitched("tenor", ChoirAahs, 46, 72, 50, 65),
	"voice.baritone":      pitched("baritone", ChoirAahs, 43, 67, 48, 62),
	"voice.bass":          pitched("bass", ChoirAahs, 38, 66, 43, 60),
	"voice":               pitched("voice", ChoirAahs, 38, 84, 43, 79),

	// Tuned percussion is read by pitch like anything else.
	"pitched-percussion.timpani":      pitched("timpani", Timpani, 38, 60, 40, 57),
	"drum.timpani":                    pitched("timpani", Timpani, 38, 60, 40, 57),
	"pitched-percussion.glockenspiel": pitched("glockenspiel", Glockenspiel, 79, 108, 79, 108),
	"pitched-percussion.marimba":      pitched("marimba", Marimba, 36, 96, 41, 93),
	"pitched-percussion.vibraphone":   pitched("vibraphone", Vibraphone, 53, 89, 53, 89),
	"pitched-percussion.xylophone":    pitched("xylophone", Marimba, 65, 108, 65, 105),
	"pitched-percussion.celesta":      pitched("celesta", Celesta, 60, 108, 60, 108),

	// And the rest of the kit is read by position on it, so there is no
	// range to ask about. The families are here rather than every drum in
	// them: the fallback finds "metal" from "metal.cymbal.crash".
	"drum":   unpitched("drum"),
	"metal":  unpitched("percussion"),
	"wood":   unpitched("percussion"),
	"effect": unpitched("percussion"),
}

// soundOf finds what gscore knows about an id, trying it whole and then
// without its last piece, and so on: an instrument gscore has never heard of
// is still a reed or a drum, and that is most of what is worth knowing.
func soundOf(id string) (soundInfo, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for id != "" {
		if s, ok := sounds[id]; ok {
			return s, true
		}
		i := strings.LastIndexByte(id, '.')
		if i < 0 {
			break
		}
		id = id[:i]
	}
	return soundInfo{}, false
}

// SoundName is what an instrument id is called, and whether gscore knows it.
func SoundName(id string) (string, bool) {
	s, ok := soundOf(id)
	if !ok || s.name == "" {
		return "", false
	}
	return s.name, true
}

// SoundProgram is what to play an id with, for a file that names a sound and
// no program of its own.
func SoundProgram(id string) (Instrument, bool) {
	s, ok := soundOf(id)
	if !ok || s.percussion {
		return 0, false
	}
	return s.program, true
}

// SoundIsPercussion says an id names something read by position on a kit
// rather than by pitch.
func SoundIsPercussion(id string) bool {
	s, ok := soundOf(id)
	return ok && s.percussion
}

// SoundReach is how far an id plays, and whether gscore knows.
func SoundReach(id string) (Reach, bool) {
	s, ok := soundOf(id)
	if !ok || !s.hasReach {
		return Reach{}, false
	}
	return s.reach, true
}
