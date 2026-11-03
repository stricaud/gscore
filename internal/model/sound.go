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

	"wind.reed.oboe": pitched("oboe", Oboe, 58, 93, 60, 86),
	// The standard spells it at the top of the reeds and not under the
	// oboe. Written the other way it fell back to "wind.reed.oboe", so
	// every english horn ever imported was reported as an oboe and
	// measured against an oboe's range, which is five notes higher at the
	// bottom. The old spelling is kept below, where the aliases are.
	"wind.reed.english-horn": pitched("english horn", EnglishHorn, 52, 81, 55, 76),
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
	// A harp and a guitar are plucked, and the standard files them under
	// pluck rather than strings — where this table used to look for them
	// and not find them, so a MuseScore harp arrived with no range and no
	// name of its own. Both old spellings are kept below.
	"pluck.harp":   pitched("harp", Harp, 23, 104, 28, 100),
	"pluck.guitar": pitched("guitar", AcousticGuitar, 40, 88, 40, 83),
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
	"rattle": unpitched("percussion"),

	/*
	 * The rest of what people write for.
	 *
	 * Everything above was what the importers had met. What follows is
	 * the catalogue somebody picks from when they start a score, and it
	 * is a different question: not "what turned up in a file" but "what
	 * is this piece for". An orchestrator wants a bass trombone and a
	 * contrabass clarinet; a folk player wants a bouzouki; neither is
	 * served by a list of thirty-eight General MIDI programs.
	 *
	 * The ids are MusicXML 4.0's standard sounds, which is a published
	 * list and the one MuseScore, Dorico and Finale all write — so a part
	 * named here survives the trip out and back. Nothing is invented: an
	 * id gscore makes up would be written into a file that no other
	 * program could read, which is worse than saying nothing.
	 *
	 * The program beside each is General MIDI's nearest, and often it is
	 * not near. There is no cornet in General MIDI and no sackbutt at
	 * all; a trumpet plays them. That is a statement about what can be
	 * made audible, not about what the instrument is, and the name is
	 * what the instrument is.
	 *
	 * The ranges are the written compass of an ordinary instrument and
	 * its ordinary player: the wide pair is what the instrument can do,
	 * the narrow pair what can be asked for night after night. Where
	 * gscore does not know, nothing is claimed — see strings.group.
	 */

	// Keyboards.
	"keyboard.piano.grand":      pitched("grand piano", AcousticGrandPiano, 21, 108, 28, 96),
	"keyboard.piano.upright":    pitched("upright piano", AcousticGrandPiano, 21, 108, 28, 96),
	"keyboard.piano.electric":   pitched("electric piano", ElectricPiano, 28, 103, 28, 96),
	"keyboard.piano.honky-tonk": pitched("honky-tonk piano", HonkyTonkPiano, 21, 108, 28, 96),
	"keyboard.piano.toy":        pitched("toy piano", Celesta, 60, 96, 60, 96),
	"keyboard.fortepiano":       pitched("fortepiano", AcousticGrandPiano, 29, 101, 29, 101),
	"keyboard.clavichord":       pitched("clavichord", Clavinet, 36, 84, 36, 84),
	"keyboard.virginal":         pitched("virginal", Harpsichord, 41, 86, 41, 86),
	"keyboard.organ.pipe":       pitched("pipe organ", ChurchOrgan, 36, 96, 36, 96),
	"keyboard.organ.reed":       pitched("reed organ", ReedOrgan, 36, 96, 36, 96),
	"keyboard.organ.drawbar":    pitched("drawbar organ", DrawbarOrgan, 36, 96, 36, 96),
	"keyboard.organ.percussive": pitched("percussive organ", PercussiveOrgan, 36, 96, 36, 96),
	"keyboard.organ.rotary":     pitched("rotary organ", RockOrgan, 36, 96, 36, 96),
	"keyboard.harmonium":        pitched("harmonium", ReedOrgan, 36, 96, 36, 96),
	"keyboard.bandoneon":        pitched("bandoneon", TangoAccordion, 41, 89, 41, 89),
	"keyboard.concertina":       pitched("concertina", Accordion, 48, 93, 48, 93),

	// Voices. A section's compass, as above.
	"voice.countertenor": pitched("countertenor", ChoirAahs, 48, 74, 53, 70),
	"voice.child":        pitched("child voice", ChoirAahs, 55, 79, 57, 76),
	"voice.female":       pitched("women", ChoirAahs, 53, 84, 57, 79),
	"voice.male":         pitched("men", ChoirAahs, 38, 72, 43, 67),
	"voice.vocals":       pitched("voice", ChoirAahs, 38, 84, 43, 79),

	// Flutes.
	"wind.flutes.flute.bass":         pitched("bass flute", Flute, 48, 84, 50, 79),
	"wind.flutes.flute.irish":        pitched("irish flute", Flute, 62, 86, 62, 84),
	"wind.flutes.recorder.sopranino": pitched("sopranino recorder", Recorder, 79, 105, 79, 103),
	"wind.flutes.recorder.soprano":   pitched("descant recorder", Recorder, 72, 98, 72, 96),
	"wind.flutes.recorder.alto":      pitched("treble recorder", Recorder, 65, 91, 65, 89),
	"wind.flutes.recorder.tenor":     pitched("tenor recorder", Recorder, 60, 86, 60, 84),
	"wind.flutes.recorder.bass":      pitched("bass recorder", Recorder, 53, 79, 53, 77),
	"wind.flutes.whistle.tin":        pitched("tin whistle", Whistle, 74, 98, 74, 95),
	"wind.flutes.shakuhachi":         pitched("shakuhachi", Shakuhachi, 60, 89, 62, 86),
	"wind.flutes.fife":               pitched("fife", Piccolo, 74, 100, 74, 98),

	// Reeds.
	"wind.reed.oboe.bass":            pitched("bass oboe", Oboe, 46, 81, 48, 77),
	"wind.reed.oboe-damore":          pitched("oboe d'amore", Oboe, 56, 88, 58, 84),
	"wind.reed.oboe-da-caccia":       pitched("oboe da caccia", Oboe, 53, 84, 55, 81),
	"wind.reed.clarinet.alto":        pitched("alto clarinet", Clarinet, 45, 89, 47, 81),
	"wind.reed.clarinet.contra-alto": pitched("contra-alto clarinet", Clarinet, 33, 77, 35, 70),
	"wind.reed.clarinet.contrabass":  pitched("contrabass clarinet", Clarinet, 26, 70, 28, 63),
	"wind.reed.basset-horn":          pitched("basset horn", Clarinet, 46, 89, 48, 81),
	"wind.reed.saxophone.sopranino":  pitched("sopranino sax", SopranoSax, 61, 93, 63, 89),
	"wind.reed.saxophone.bass":       pitched("bass sax", BaritoneSax, 32, 70, 34, 65),
	"wind.reed.melodica":             pitched("melodica", Harmonica, 53, 89, 53, 89),
	"wind.reed.duduk":                pitched("duduk", Shanai, 58, 82, 58, 79),
	"wind.reed.shawm":                pitched("shawm", Shanai, 55, 84, 55, 81),
	"wind.reed.crumhorn":             pitched("crumhorn", Shanai, 53, 77, 53, 75),

	// Brass.
	"brass.trumpet.piccolo":     pitched("piccolo trumpet", Trumpet, 60, 93, 62, 89),
	"brass.trumpet.bass":        pitched("bass trumpet", Trumpet, 40, 72, 43, 70),
	"brass.cornet":              pitched("cornet", Trumpet, 52, 86, 55, 82),
	"brass.flugelhorn":          pitched("flugelhorn", Trumpet, 52, 82, 55, 79),
	"brass.posthorn":            pitched("post horn", Trumpet, 55, 84, 55, 81),
	"brass.bugle":               pitched("bugle", Trumpet, 55, 79, 55, 76),
	"brass.alphorn":             pitched("alphorn", Trumpet, 43, 79, 46, 74),
	"brass.mellophone":          pitched("mellophone", FrenchHorn, 48, 79, 50, 76),
	"brass.alto-horn":           pitched("alto horn", FrenchHorn, 45, 77, 48, 74),
	"brass.trombone.contrabass": pitched("contrabass trombone", Trombone, 28, 65, 31, 60),
	"brass.sackbutt":            pitched("sackbutt", Trombone, 40, 74, 43, 70),
	"brass.sackbutt.alto":       pitched("alto sackbutt", Trombone, 45, 79, 48, 74),
	"brass.sackbutt.tenor":      pitched("tenor sackbutt", Trombone, 40, 74, 43, 70),
	"brass.sackbutt.bass":       pitched("bass sackbutt", Trombone, 34, 70, 38, 65),
	"brass.euphonium":           pitched("euphonium", Tuba, 34, 77, 38, 72),
	"brass.baritone-horn":       pitched("baritone horn", Tuba, 34, 74, 38, 70),
	"brass.tuba.bass":           pitched("bass tuba", Tuba, 28, 67, 31, 60),
	"brass.tuba.subcontrabass":  pitched("subcontrabass tuba", Tuba, 20, 55, 22, 50),
	"brass.sousaphone":          pitched("sousaphone", Tuba, 26, 65, 29, 58),
	"brass.cimbasso":            pitched("cimbasso", Tuba, 29, 65, 31, 60),
	"brass.ophicleide":          pitched("ophicleide", Tuba, 31, 70, 34, 65),
	"brass.serpent":             pitched("serpent", Tuba, 31, 70, 34, 65),
	"brass.cornett":             pitched("cornett", Trumpet, 55, 86, 57, 81),
	"brass.wagner-tuba":         pitched("wagner tuba", FrenchHorn, 34, 74, 38, 69),
	// A section rather than an instrument, like strings.group above.
	"brass.group": {name: "brass section", program: BrassSection},

	// Bowed strings.
	"strings.viol":          pitched("viol", Cello, 36, 79, 38, 74),
	"strings.viol.treble":   pitched("treble viol", Viola, 55, 93, 55, 86),
	"strings.viol.alto":     pitched("alto viol", Viola, 48, 86, 48, 79),
	"strings.viol.tenor":    pitched("tenor viol", Cello, 43, 81, 43, 74),
	"strings.viol.bass":     pitched("bass viol", Cello, 36, 76, 36, 69),
	"strings.viol.violone":  pitched("violone", Contrabass, 28, 67, 28, 60),
	"strings.viola-damore":  pitched("viola d'amore", Viola, 48, 88, 48, 81),
	"strings.cello.piccolo": pitched("piccolo cello", Cello, 43, 91, 43, 83),
	"strings.octobass":      pitched("octobass", Contrabass, 16, 48, 16, 43),
	"strings.hurdy-gurdy":   pitched("hurdy-gurdy", Fiddle, 48, 84, 48, 79),
	"strings.nyckelharpa":   pitched("nyckelharpa", Fiddle, 48, 84, 48, 79),
	"strings.fiddle":        pitched("fiddle", Fiddle, 55, 98, 55, 88),
	"strings.erhu":          pitched("erhu", Fiddle, 55, 91, 55, 86),
	"strings.group.synth":   {name: "synth strings", program: SynthStrings},

	// Plucked strings.
	"pluck.guitar.acoustic":     pitched("acoustic guitar", SteelGuitar, 40, 88, 40, 83),
	"pluck.guitar.nylon-string": pitched("classical guitar", AcousticGuitar, 40, 88, 40, 83),
	"pluck.guitar.steel-string": pitched("steel-string guitar", SteelGuitar, 40, 88, 40, 83),
	"pluck.guitar.electric":     pitched("electric guitar", ElectricGuitar, 40, 91, 40, 86),
	"pluck.guitar.resonator":    pitched("resonator guitar", SteelGuitar, 40, 86, 40, 83),
	"pluck.guitar.pedal-steel":  pitched("pedal steel guitar", CleanGuitar, 40, 88, 40, 83),
	"pluck.guitar.requinto":     pitched("requinto guitar", AcousticGuitar, 45, 93, 45, 88),
	"pluck.bass":                pitched("bass guitar", ElectricBass, 28, 67, 28, 62),
	"pluck.bass.acoustic":       pitched("acoustic bass guitar", AcousticBass, 28, 67, 28, 62),
	"pluck.bass.electric":       pitched("electric bass", ElectricBass, 28, 67, 28, 62),
	"pluck.bass.fretless":       pitched("fretless bass", FretlessBass, 28, 67, 28, 62),
	"pluck.bass.synth":          pitched("synth bass", SynthBass, 28, 67, 28, 62),
	"pluck.lute":                pitched("lute", AcousticGuitar, 40, 83, 40, 79),
	"pluck.archlute":            pitched("archlute", AcousticGuitar, 33, 83, 36, 79),
	"pluck.theorbo":             pitched("theorbo", AcousticGuitar, 28, 79, 31, 74),
	"pluck.vihuela":             pitched("vihuela", AcousticGuitar, 40, 83, 40, 79),
	"pluck.mandolin":            pitched("mandolin", AcousticGuitar, 55, 96, 55, 88),
	"pluck.mandola":             pitched("mandola", AcousticGuitar, 48, 88, 48, 81),
	"pluck.mandocello":          pitched("mandocello", AcousticGuitar, 36, 76, 36, 72),
	"pluck.banjo":               pitched("banjo", Banjo, 48, 88, 48, 83),
	"pluck.banjo.tenor":         pitched("tenor banjo", Banjo, 48, 88, 48, 83),
	"pluck.ukulele":             pitched("ukulele", AcousticGuitar, 60, 93, 60, 88),
	"pluck.ukulele.tenor":       pitched("tenor ukulele", AcousticGuitar, 55, 91, 55, 86),
	"pluck.celtic-harp":         pitched("celtic harp", Harp, 36, 93, 36, 88),
	"pluck.dulcimer":            pitched("appalachian dulcimer", Dulcimer, 55, 86, 55, 83),
	"pluck.autoharp":            pitched("autoharp", Dulcimer, 48, 86, 48, 83),
	"pluck.zither":              pitched("zither", Dulcimer, 48, 93, 48, 88),
	"pluck.psaltery":            pitched("psaltery", Dulcimer, 55, 91, 55, 86),
	"pluck.lyre":                pitched("lyre", Harp, 55, 86, 55, 83),
	"pluck.kantele":             pitched("kantele", Harp, 55, 91, 55, 86),
	"pluck.sitar":               pitched("sitar", Sitar, 48, 91, 48, 86),
	"pluck.oud":                 pitched("oud", Sitar, 36, 79, 38, 74),
	"pluck.bouzouki":            pitched("bouzouki", AcousticGuitar, 48, 88, 48, 83),
	"pluck.bouzouki.irish":      pitched("irish bouzouki", AcousticGuitar, 43, 83, 43, 79),
	"pluck.balalaika":           pitched("balalaika", Banjo, 57, 91, 57, 86),
	"pluck.charango":            pitched("charango", AcousticGuitar, 57, 93, 57, 88),
	"pluck.cuatro":              pitched("cuatro", AcousticGuitar, 50, 88, 50, 83),
	"pluck.cavaquinho":          pitched("cavaquinho", AcousticGuitar, 62, 96, 62, 91),
	"pluck.tres":                pitched("tres", AcousticGuitar, 48, 88, 48, 83),
	"pluck.saz":                 pitched("saz", Sitar, 43, 83, 43, 79),
	"pluck.tambura":             pitched("tambura", Sitar, 36, 72, 36, 69),
	"pluck.veena":               pitched("veena", Sitar, 36, 84, 38, 79),
	"pluck.pipa":                pitched("pipa", Koto, 50, 89, 50, 84),
	"pluck.guzheng":             pitched("guzheng", Koto, 38, 86, 38, 81),
	"pluck.koto":                pitched("koto", Koto, 41, 84, 41, 79),
	"pluck.shamisen":            pitched("shamisen", Shamisen, 48, 84, 48, 79),
	"pluck.sanshin":             pitched("sanshin", Shamisen, 48, 84, 48, 79),
	"pluck.kora":                pitched("kora", Harp, 48, 86, 48, 81),

	// Tuned percussion.
	"pitched-percussion.tubular-bells":   pitched("tubular bells", TubularBells, 53, 77, 53, 77),
	"pitched-percussion.bells":           pitched("bells", TubularBells, 53, 77, 53, 77),
	"pitched-percussion.handbells":       pitched("handbells", TubularBells, 48, 96, 48, 93),
	"pitched-percussion.handchimes":      pitched("handchimes", TubularBells, 48, 96, 48, 93),
	"pitched-percussion.xylorimba":       pitched("xylorimba", Marimba, 48, 108, 53, 103),
	"pitched-percussion.marimba.bass":    pitched("bass marimba", Marimba, 29, 72, 31, 69),
	"pitched-percussion.music-box":       pitched("music box", MusicBox, 72, 96, 72, 96),
	"pitched-percussion.kalimba":         pitched("kalimba", Kalimba, 60, 84, 60, 84),
	"pitched-percussion.mbira":           pitched("mbira", Kalimba, 60, 84, 60, 84),
	"pitched-percussion.hammer-dulcimer": pitched("hammered dulcimer", Dulcimer, 48, 91, 48, 86),
	"pitched-percussion.cimbalom":        pitched("cimbalom", Dulcimer, 41, 89, 41, 84),
	"pitched-percussion.glass-harmonica": pitched("glass harmonica", Celesta, 60, 96, 60, 96),
	"pitched-percussion.balafon":         pitched("balafon", Marimba, 55, 96, 55, 91),
	"pitched-percussion.crystal-glasses": pitched("crystal glasses", Celesta, 60, 96, 60, 96),
	"metal.crotales":                     pitched("crotales", Glockenspiel, 84, 108, 84, 108),
	"metal.steel-drums":                  pitched("steel drums", SteelDrums, 48, 84, 48, 81),
	"metal.bells.carillon":               pitched("carillon", TubularBells, 48, 96, 48, 93),
	"metal.hang":                         pitched("hang", SteelDrums, 53, 77, 53, 77),
	"metal.singing-bowl":                 pitched("singing bowl", TubularBells, 53, 77, 53, 77),
	"metal.musical-saw":                  pitched("musical saw", Whistle, 55, 84, 55, 79),
	"metal.jaw-harp":                     pitched("jaw harp", Kalimba, 48, 72, 48, 72),

	// The kit, and everything struck or shaken that is read by position
	// rather than by pitch.
	"drum.group.set":           unpitched("drum kit"),
	"drum.snare-drum":          unpitched("snare drum"),
	"drum.bass-drum":           unpitched("bass drum"),
	"drum.tom-tom":             unpitched("tom-toms"),
	"drum.tenor-drum":          unpitched("tenor drum"),
	"drum.rototom":             unpitched("roto-toms"),
	"drum.timbale":             unpitched("timbales"),
	"drum.bongo":               unpitched("bongos"),
	"drum.conga":               unpitched("congas"),
	"drum.djembe":              unpitched("djembe"),
	"drum.cajon":               unpitched("cajón"),
	"drum.tabla":               unpitched("tabla"),
	"drum.taiko":               unpitched("taiko"),
	"drum.frame-drum":          unpitched("frame drum"),
	"drum.bodhran":             unpitched("bodhrán"),
	"drum.darabuka":            unpitched("darabuka"),
	"drum.surdo":               unpitched("surdo"),
	"drum.tambourine":          unpitched("tambourine"),
	"drum.hand-drum":           unpitched("hand drum"),
	"metal.triangle":           unpitched("triangle"),
	"metal.anvil":              unpitched("anvil"),
	"metal.flexatone":          unpitched("flexatone"),
	"metal.tamtam":             unpitched("tam-tam"),
	"metal.gong":               unpitched("gong"),
	"metal.hi-hat":             unpitched("hi-hat"),
	"metal.thundersheet":       unpitched("thunder sheet"),
	"metal.washboard":          unpitched("washboard"),
	"metal.spoons":             unpitched("spoons"),
	"metal.cymbal.crash":       unpitched("crash cymbal"),
	"metal.cymbal.ride":        unpitched("ride cymbal"),
	"metal.cymbal.suspended":   unpitched("suspended cymbal"),
	"metal.cymbal.splash":      unpitched("splash cymbal"),
	"metal.cymbal.chinese":     unpitched("china cymbal"),
	"metal.cymbal.sizzle":      unpitched("sizzle cymbal"),
	"metal.cymbal.clash":       unpitched("clash cymbals"),
	"metal.cymbal.finger":      unpitched("finger cymbals"),
	"metal.bells.cowbell":      unpitched("cowbell"),
	"metal.bells.agogo":        unpitched("agogo bells"),
	"metal.bells.sleigh-bells": unpitched("sleigh bells"),
	"metal.bells.wind-chimes":  unpitched("wind chimes"),
	"metal.bells.mark-tree":    unpitched("mark tree"),
	"metal.bells.bell-plate":   unpitched("bell plate"),
	"metal.bells.almglocken":   unpitched("almglocken"),
	"metal.bells.church":       unpitched("church bells"),
	"metal.berimbau":           unpitched("berimbau"),
	"wood.wood-block":          unpitched("wood block"),
	"wood.temple-block":        unpitched("temple blocks"),
	"wood.claves":              unpitched("claves"),
	"wood.castanets":           unpitched("castanets"),
	"wood.guiro":               unpitched("güiro"),
	"wood.slapstick":           unpitched("whip"),
	"wood.bones":               unpitched("bones"),
	"wood.jam-block":           unpitched("jam block"),
	"wood.sand-block":          unpitched("sand blocks"),
	"wood.agogo-block":         unpitched("agogo blocks"),
	"wood.drum-sticks":         unpitched("drum sticks"),
	"rattle.maraca":            unpitched("maracas"),
	"rattle.cabasa":            unpitched("cabasa"),
	"rattle.shaker":            unpitched("shaker"),
	"rattle.vibraslap":         unpitched("vibraslap"),
	"rattle.ratchet":           unpitched("ratchet"),
	"rattle.rain-stick":        unpitched("rain stick"),
	"rattle.shekere":           unpitched("shekere"),
	"rattle.caxixi":            unpitched("caxixi"),
	"effect.wind":              unpitched("wind machine"),
	"effect.thunder":           unpitched("thunder sheet"),
	"effect.applause":          unpitched("applause"),
	"effect.hand-clap":         unpitched("hand claps"),
	"effect.whistle.police":    unpitched("police whistle"),
	"effect.whistle.slide":     unpitched("slide whistle"),
	"effect.bird":              unpitched("bird whistle"),
	"effect.siren":             unpitched("siren"),
	"effect.gunshot":           unpitched("gunshot"),
	"effect.metronome-click":   unpitched("metronome"),
	"voice.percussion.beatbox": unpitched("beatbox"),

	// Electronic.
	"keyboard.ondes-martenot": pitched("ondes martenot", SquareLead, 36, 96, 36, 93),
	"synth.theremin":          pitched("theremin", SawtoothLead, 36, 96, 41, 91),
	"synth.tone.square":       pitched("square lead", SquareLead, 24, 108, 36, 96),
	"synth.tone.sawtooth":     pitched("sawtooth lead", SawtoothLead, 24, 108, 36, 96),
	"synth.tone.sine":         pitched("sine tone", SquareLead, 24, 108, 36, 96),
	"synth.pad.warm":          pitched("warm pad", WarmPad, 24, 108, 36, 96),
	"synth.pad":               pitched("synth pad", WarmPad, 24, 108, 36, 96),
	"synth.group":             {name: "synthesizer", program: SquareLead},
	"synth.effects":           {name: "effect synth", program: 96},

	/*
	 * Aliases.
	 *
	 * Spellings that are not in the standard list but have been written
	 * into files — three of them by gscore itself, before this table was
	 * checked against the standard and found to be guessing. They are
	 * kept so those files still open knowing what they are, and they are
	 * not offered in the catalogue: nothing new should be written with
	 * them.
	 */
	// The two that were already above — "pitched-percussion.timpani" and
	// "pitched-percussion.celesta" — are aliases too; they are left where
	// they are, with the tuned percussion, so the diff against the
	// standard reads in one place.
	"wind.reed.oboe.english-horn":    pitched("english horn", EnglishHorn, 52, 81, 55, 76),
	"strings.harp":                   pitched("harp", Harp, 23, 104, 28, 100),
	"strings.guitar":                 pitched("guitar", AcousticGuitar, 40, 88, 40, 83),
	"pitched-percussion.steel-drums": pitched("steel drums", SteelDrums, 48, 84, 48, 81),
	"pitched-percussion.crotales":    pitched("crotales", Glockenspiel, 84, 108, 84, 108),
}

// transposeFor is the interval between what a staff reads and what is
// heard, for the instruments where that is not nothing.
//
// Kept by sound and not by General MIDI program, because the program cannot
// tell them apart: a clarinet in A and a clarinet in B-flat are both program
// 71, and reading the transposition off the program put every clarinet in A
// a semitone out. Which is the whole reason the catalogue offers sounds
// rather than programs.
//
// Only the transposing instruments are here. Most are not, and an id that is
// missing is read as a staff that reads what it sounds — which is the right
// answer for a piano, a violin, a trombone and nearly everything else.
var transposeFor = map[string]Transposition{
	// In B flat: a major second below.
	"wind.reed.clarinet":          {Diatonic: -1, Chromatic: -2},
	"wind.reed.clarinet.bflat":    {Diatonic: -1, Chromatic: -2},
	"wind.reed.saxophone.soprano": {Diatonic: -1, Chromatic: -2},
	"brass.trumpet":               {Diatonic: -1, Chromatic: -2},
	"brass.trumpet.bflat":         {Diatonic: -1, Chromatic: -2},
	"brass.cornet":                {Diatonic: -1, Chromatic: -2},
	"brass.flugelhorn":            {Diatonic: -1, Chromatic: -2},

	// In A: a minor third below.
	"wind.reed.clarinet.a":  {Diatonic: -2, Chromatic: -3},
	"wind.reed.oboe-damore": {Diatonic: -2, Chromatic: -3},

	// In E flat, sounding above what is written: a minor third up.
	"wind.reed.clarinet.eflat":      {Diatonic: 2, Chromatic: 3},
	"wind.reed.saxophone.sopranino": {Diatonic: 2, Chromatic: 3},

	// In F: a perfect fifth below.
	"brass.french-horn":        {Diatonic: -4, Chromatic: -7},
	"brass.natural-horn":       {Diatonic: -4, Chromatic: -7},
	"brass.wagner-tuba":        {Diatonic: -4, Chromatic: -7},
	"brass.mellophone":         {Diatonic: -4, Chromatic: -7},
	"wind.reed.english-horn":   {Diatonic: -4, Chromatic: -7},
	"wind.reed.basset-horn":    {Diatonic: -4, Chromatic: -7},
	"wind.reed.oboe-da-caccia": {Diatonic: -4, Chromatic: -7},

	// In E flat: a major sixth below.
	"wind.reed.saxophone.alto": {Diatonic: -5, Chromatic: -9},
	"wind.reed.clarinet.alto":  {Diatonic: -5, Chromatic: -9},
	"brass.alto-horn":          {Diatonic: -5, Chromatic: -9},

	// In B flat, an octave lower again: a major ninth below.
	"wind.reed.saxophone.tenor": {Diatonic: -1, Chromatic: -2, OctaveChange: -1},
	"wind.reed.clarinet.bass":   {Diatonic: -1, Chromatic: -2, OctaveChange: -1},
	"brass.trumpet.bass":        {Diatonic: -1, Chromatic: -2, OctaveChange: -1},
	"brass.baritone-horn":       {Diatonic: -1, Chromatic: -2, OctaveChange: -1},

	// In E flat, an octave and a major sixth below.
	"wind.reed.saxophone.baritone":   {Diatonic: -5, Chromatic: -9, OctaveChange: -1},
	"wind.reed.clarinet.contra-alto": {Diatonic: -5, Chromatic: -9, OctaveChange: -1},

	// Two octaves down from the B flat instruments.
	"wind.reed.saxophone.bass":      {Diatonic: -1, Chromatic: -2, OctaveChange: -2},
	"wind.reed.clarinet.contrabass": {Diatonic: -1, Chromatic: -2, OctaveChange: -2},

	// The rest of the transposing winds.
	"brass.trumpet.d":        {Diatonic: 1, Chromatic: 2},
	"brass.trumpet.piccolo":  {Diatonic: -1, Chromatic: -2, OctaveChange: 1},
	"wind.flutes.flute.alto": {Diatonic: -3, Chromatic: -5},
	"wind.flutes.flute.bass": {OctaveChange: -1},

	// Written an octave away from where they sound, and nothing else.
	"wind.flutes.flute.piccolo":          {OctaveChange: 1},
	"wind.flutes.recorder.sopranino":     {OctaveChange: 1},
	"wind.flutes.recorder.soprano":       {OctaveChange: 1},
	"wind.reed.contrabassoon":            {OctaveChange: -1},
	"strings.contrabass":                 {OctaveChange: -1},
	"strings.octobass":                   {OctaveChange: -1},
	"pluck.guitar":                       {OctaveChange: -1},
	"pluck.guitar.acoustic":              {OctaveChange: -1},
	"pluck.guitar.nylon-string":          {OctaveChange: -1},
	"pluck.guitar.steel-string":          {OctaveChange: -1},
	"pluck.guitar.electric":              {OctaveChange: -1},
	"pluck.guitar.resonator":             {OctaveChange: -1},
	"pluck.guitar.requinto":              {OctaveChange: -1},
	"pluck.bass":                         {OctaveChange: -1},
	"pluck.bass.acoustic":                {OctaveChange: -1},
	"pluck.bass.electric":                {OctaveChange: -1},
	"pluck.bass.fretless":                {OctaveChange: -1},
	"pluck.bass.synth":                   {OctaveChange: -1},
	"pitched-percussion.xylophone":       {OctaveChange: 1},
	"pitched-percussion.xylorimba":       {OctaveChange: 1},
	"keyboard.celesta":                   {OctaveChange: 1},
	"pitched-percussion.handbells":       {OctaveChange: 1},
	"pitched-percussion.glockenspiel":    {OctaveChange: 2},
	"metal.crotales":                     {OctaveChange: 2},
	"pitched-percussion.marimba.bass":    {OctaveChange: -1},
	"pitched-percussion.hammer-dulcimer": {},
}

// SoundTransposition is the interval between what a staff written for an id
// reads and what is heard, and whether gscore knows of one.
//
// Exact rather than by family: "wind.reed.clarinet.a" must not fall back to
// the B-flat clarinet's interval, which is the mistake this table exists to
// prevent. An id with no entry does not transpose.
func SoundTransposition(id string) (Transposition, bool) {
	t, ok := transposeFor[strings.ToLower(strings.TrimSpace(id))]
	return t, ok
}

// SoundKnown reports whether an id is one the table has an entry for
// exactly, rather than one it can only place by family.
//
// What the editor validates a chosen instrument with. The family fallback is
// right when reading a file — an unknown reed is still a reed — and wrong
// when somebody is picking from a list, where an id that is not offered is a
// mistake and should say so.
func SoundKnown(id string) bool {
	_, ok := sounds[strings.ToLower(strings.TrimSpace(id))]
	return ok
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
