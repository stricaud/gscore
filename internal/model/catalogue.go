/*
 * catalogue.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// The catalogue is the list somebody picks an instrument from.
//
// It is not the sounds table and it is not General MIDI. The sounds table
// answers "what is this thing a file just told me about"; General MIDI
// answers "what shall I play it with". This answers the question somebody
// actually asks when they start a score, which is "what is this piece for" —
// and that question has an order and a shape to its answer. Nobody looks for
// a bass trombone in an alphabetical list of a hundred and twenty-eight
// program numbers between "bagpipe" and "banjo".
//
// So: clustered by what the thing is, in the order a score is laid out, and
// within each group the ordinary ones first and the rare ones after. A
// flute player looking for a flute finds it at the top of the woodwind; a
// flute player looking for an alto flute finds it three lines down, which is
// where they would look.
//
// Every id here must be in the sounds table, and every id in the sounds
// table must be a real MusicXML standard sound — both are tested. Those two
// rules together are what make a part written here survive being exported
// and come back knowing what it is. The aliases at the foot of the sounds
// table are deliberately not listed: they exist so old files still open, not
// so new ones can be written wrongly.

// Groups are the clusters, in the order they are offered.
//
// Bowed and plucked strings are apart because they are apart to everybody
// who plays one: the question "is there a harp in this piece" is never
// answered by looking through the violins. Pitched and unpitched percussion
// are apart for a harder reason — one is read by pitch and the other by
// position on a kit, which is a different staff, a different clef and a
// different way of writing.
const (
	GroupKeyboard   = "Keyboard"
	GroupVoice      = "Voice"
	GroupWoodwind   = "Woodwind"
	GroupBrass      = "Brass"
	GroupBowed      = "Strings — bowed"
	GroupPlucked    = "Strings — plucked"
	GroupTuned      = "Percussion — pitched"
	GroupUntuned    = "Percussion — unpitched"
	GroupElectronic = "Electronic"
)

// Groups in the order they are offered.
var Groups = []string{
	GroupKeyboard, GroupVoice, GroupWoodwind, GroupBrass,
	GroupBowed, GroupPlucked, GroupTuned, GroupUntuned, GroupElectronic,
}

// catalogue is every instrument offered, by group, in the order offered.
var catalogue = []struct {
	Group string
	Sound string
}{
	{GroupKeyboard, "keyboard.piano"},
	{GroupKeyboard, "keyboard.piano.grand"},
	{GroupKeyboard, "keyboard.piano.upright"},
	{GroupKeyboard, "keyboard.piano.electric"},
	{GroupKeyboard, "keyboard.organ"},
	{GroupKeyboard, "keyboard.organ.pipe"},
	{GroupKeyboard, "keyboard.organ.reed"},
	{GroupKeyboard, "keyboard.organ.drawbar"},
	{GroupKeyboard, "keyboard.organ.percussive"},
	{GroupKeyboard, "keyboard.organ.rotary"},
	{GroupKeyboard, "keyboard.harpsichord"},
	{GroupKeyboard, "keyboard.clavichord"},
	{GroupKeyboard, "keyboard.celesta"},
	{GroupKeyboard, "keyboard.accordion"},
	{GroupKeyboard, "keyboard.bandoneon"},
	{GroupKeyboard, "keyboard.concertina"},
	{GroupKeyboard, "keyboard.harmonium"},
	{GroupKeyboard, "keyboard.fortepiano"},
	{GroupKeyboard, "keyboard.virginal"},
	{GroupKeyboard, "keyboard.piano.honky-tonk"},
	{GroupKeyboard, "keyboard.piano.toy"},

	{GroupVoice, "voice.soprano"},
	{GroupVoice, "voice.mezzo-soprano"},
	{GroupVoice, "voice.alto"},
	{GroupVoice, "voice.countertenor"},
	{GroupVoice, "voice.tenor"},
	{GroupVoice, "voice.baritone"},
	{GroupVoice, "voice.bass"},
	{GroupVoice, "voice.vocals"},
	{GroupVoice, "voice.female"},
	{GroupVoice, "voice.male"},
	{GroupVoice, "voice.child"},

	{GroupWoodwind, "wind.flutes.flute"},
	{GroupWoodwind, "wind.flutes.flute.piccolo"},
	{GroupWoodwind, "wind.flutes.flute.alto"},
	{GroupWoodwind, "wind.flutes.flute.bass"},
	{GroupWoodwind, "wind.reed.oboe"},
	{GroupWoodwind, "wind.reed.english-horn"},
	{GroupWoodwind, "wind.reed.oboe-damore"},
	{GroupWoodwind, "wind.reed.oboe.bass"},
	{GroupWoodwind, "wind.reed.clarinet"},
	// The one everybody means by "clarinet", and the one that was not
	// here. Everything else about it was: the sound table names it, the
	// transposing table puts it a major second down, and the score order
	// has it between the plain clarinet and the one in A. Only the list
	// somebody picks from left it out, so a player writing for the
	// commonest clarinet there is had to choose the unqualified entry and
	// hope. The trumpets are listed this way too — plain, then in B flat,
	// then in C — which is what this was meant to match.
	{GroupWoodwind, "wind.reed.clarinet.bflat"},
	{GroupWoodwind, "wind.reed.clarinet.a"},
	{GroupWoodwind, "wind.reed.clarinet.eflat"},
	{GroupWoodwind, "wind.reed.clarinet.alto"},
	{GroupWoodwind, "wind.reed.clarinet.bass"},
	{GroupWoodwind, "wind.reed.clarinet.contra-alto"},
	{GroupWoodwind, "wind.reed.clarinet.contrabass"},
	{GroupWoodwind, "wind.reed.basset-horn"},
	{GroupWoodwind, "wind.reed.bassoon"},
	{GroupWoodwind, "wind.reed.contrabassoon"},
	{GroupWoodwind, "wind.reed.saxophone.sopranino"},
	{GroupWoodwind, "wind.reed.saxophone.soprano"},
	{GroupWoodwind, "wind.reed.saxophone.alto"},
	{GroupWoodwind, "wind.reed.saxophone.tenor"},
	{GroupWoodwind, "wind.reed.saxophone.baritone"},
	{GroupWoodwind, "wind.reed.saxophone.bass"},
	{GroupWoodwind, "wind.flutes.recorder"},
	{GroupWoodwind, "wind.flutes.recorder.sopranino"},
	{GroupWoodwind, "wind.flutes.recorder.soprano"},
	{GroupWoodwind, "wind.flutes.recorder.alto"},
	{GroupWoodwind, "wind.flutes.recorder.tenor"},
	{GroupWoodwind, "wind.flutes.recorder.bass"},
	{GroupWoodwind, "wind.reed.harmonica"},
	{GroupWoodwind, "wind.reed.melodica"},
	{GroupWoodwind, "wind.flutes.panpipes"},
	{GroupWoodwind, "wind.flutes.ocarina"},
	{GroupWoodwind, "wind.flutes.whistle.tin"},
	{GroupWoodwind, "wind.flutes.fife"},
	{GroupWoodwind, "wind.flutes.flute.irish"},
	{GroupWoodwind, "wind.flutes.shakuhachi"},
	{GroupWoodwind, "wind.reed.duduk"},
	{GroupWoodwind, "wind.reed.shawm"},
	{GroupWoodwind, "wind.reed.crumhorn"},
	{GroupWoodwind, "wind.reed.oboe-da-caccia"},

	{GroupBrass, "brass.trumpet"},
	{GroupBrass, "brass.trumpet.bflat"},
	{GroupBrass, "brass.trumpet.c"},
	{GroupBrass, "brass.trumpet.d"},
	{GroupBrass, "brass.trumpet.piccolo"},
	{GroupBrass, "brass.trumpet.bass"},
	{GroupBrass, "brass.cornet"},
	{GroupBrass, "brass.flugelhorn"},
	{GroupBrass, "brass.french-horn"},
	{GroupBrass, "brass.wagner-tuba"},
	{GroupBrass, "brass.mellophone"},
	{GroupBrass, "brass.alto-horn"},
	{GroupBrass, "brass.trombone"},
	{GroupBrass, "brass.trombone.alto"},
	{GroupBrass, "brass.trombone.tenor"},
	{GroupBrass, "brass.trombone.bass"},
	{GroupBrass, "brass.trombone.contrabass"},
	{GroupBrass, "brass.euphonium"},
	{GroupBrass, "brass.baritone-horn"},
	{GroupBrass, "brass.tuba"},
	{GroupBrass, "brass.tuba.bass"},
	{GroupBrass, "brass.tuba.subcontrabass"},
	{GroupBrass, "brass.sousaphone"},
	{GroupBrass, "brass.cimbasso"},
	{GroupBrass, "brass.group"},
	{GroupBrass, "brass.natural-horn"},
	{GroupBrass, "brass.cornett"},
	{GroupBrass, "brass.sackbutt"},
	{GroupBrass, "brass.sackbutt.alto"},
	{GroupBrass, "brass.sackbutt.tenor"},
	{GroupBrass, "brass.sackbutt.bass"},
	{GroupBrass, "brass.ophicleide"},
	{GroupBrass, "brass.serpent"},
	{GroupBrass, "brass.posthorn"},
	{GroupBrass, "brass.bugle"},
	{GroupBrass, "brass.alphorn"},

	{GroupBowed, "strings.violin"},
	{GroupBowed, "strings.viola"},
	{GroupBowed, "strings.cello"},
	{GroupBowed, "strings.contrabass"},
	{GroupBowed, "strings.group"},
	{GroupBowed, "strings.fiddle"},
	{GroupBowed, "strings.viola-damore"},
	{GroupBowed, "strings.cello.piccolo"},
	{GroupBowed, "strings.octobass"},
	{GroupBowed, "strings.viol"},
	{GroupBowed, "strings.viol.treble"},
	{GroupBowed, "strings.viol.alto"},
	{GroupBowed, "strings.viol.tenor"},
	{GroupBowed, "strings.viol.bass"},
	{GroupBowed, "strings.viol.violone"},
	{GroupBowed, "strings.hurdy-gurdy"},
	{GroupBowed, "strings.nyckelharpa"},
	{GroupBowed, "strings.erhu"},

	{GroupPlucked, "pluck.guitar"},
	{GroupPlucked, "pluck.guitar.acoustic"},
	{GroupPlucked, "pluck.guitar.nylon-string"},
	{GroupPlucked, "pluck.guitar.steel-string"},
	{GroupPlucked, "pluck.guitar.electric"},
	{GroupPlucked, "pluck.guitar.resonator"},
	{GroupPlucked, "pluck.guitar.pedal-steel"},
	{GroupPlucked, "pluck.guitar.requinto"},
	{GroupPlucked, "pluck.bass"},
	{GroupPlucked, "pluck.bass.acoustic"},
	{GroupPlucked, "pluck.bass.electric"},
	{GroupPlucked, "pluck.bass.fretless"},
	{GroupPlucked, "pluck.bass.synth"},
	{GroupPlucked, "pluck.harp"},
	{GroupPlucked, "pluck.celtic-harp"},
	{GroupPlucked, "pluck.mandolin"},
	{GroupPlucked, "pluck.mandola"},
	{GroupPlucked, "pluck.mandocello"},
	{GroupPlucked, "pluck.banjo"},
	{GroupPlucked, "pluck.banjo.tenor"},
	{GroupPlucked, "pluck.ukulele"},
	{GroupPlucked, "pluck.ukulele.tenor"},
	{GroupPlucked, "pluck.lute"},
	{GroupPlucked, "pluck.archlute"},
	{GroupPlucked, "pluck.theorbo"},
	{GroupPlucked, "pluck.vihuela"},
	{GroupPlucked, "pluck.dulcimer"},
	{GroupPlucked, "pluck.autoharp"},
	{GroupPlucked, "pluck.zither"},
	{GroupPlucked, "pluck.psaltery"},
	{GroupPlucked, "pluck.lyre"},
	{GroupPlucked, "pluck.kantele"},
	{GroupPlucked, "pluck.bouzouki"},
	{GroupPlucked, "pluck.bouzouki.irish"},
	{GroupPlucked, "pluck.balalaika"},
	{GroupPlucked, "pluck.charango"},
	{GroupPlucked, "pluck.cuatro"},
	{GroupPlucked, "pluck.cavaquinho"},
	{GroupPlucked, "pluck.tres"},
	{GroupPlucked, "pluck.oud"},
	{GroupPlucked, "pluck.saz"},
	{GroupPlucked, "pluck.sitar"},
	{GroupPlucked, "pluck.tambura"},
	{GroupPlucked, "pluck.veena"},
	{GroupPlucked, "pluck.pipa"},
	{GroupPlucked, "pluck.guzheng"},
	{GroupPlucked, "pluck.koto"},
	{GroupPlucked, "pluck.shamisen"},
	{GroupPlucked, "pluck.sanshin"},
	{GroupPlucked, "pluck.kora"},

	{GroupTuned, "drum.timpani"},
	{GroupTuned, "pitched-percussion.glockenspiel"},
	{GroupTuned, "pitched-percussion.xylophone"},
	{GroupTuned, "pitched-percussion.marimba"},
	{GroupTuned, "pitched-percussion.marimba.bass"},
	{GroupTuned, "pitched-percussion.vibraphone"},
	{GroupTuned, "pitched-percussion.xylorimba"},
	{GroupTuned, "pitched-percussion.tubular-bells"},
	{GroupTuned, "metal.crotales"},
	{GroupTuned, "pitched-percussion.handbells"},
	{GroupTuned, "pitched-percussion.handchimes"},
	{GroupTuned, "metal.bells.carillon"},
	{GroupTuned, "metal.steel-drums"},
	{GroupTuned, "metal.hang"},
	{GroupTuned, "pitched-percussion.kalimba"},
	{GroupTuned, "pitched-percussion.mbira"},
	{GroupTuned, "pitched-percussion.balafon"},
	{GroupTuned, "pitched-percussion.cimbalom"},
	{GroupTuned, "pitched-percussion.hammer-dulcimer"},
	{GroupTuned, "pitched-percussion.music-box"},
	{GroupTuned, "pitched-percussion.glass-harmonica"},
	{GroupTuned, "pitched-percussion.crystal-glasses"},
	{GroupTuned, "metal.singing-bowl"},
	{GroupTuned, "metal.musical-saw"},
	{GroupTuned, "metal.jaw-harp"},

	{GroupUntuned, "drum.group.set"},
	{GroupUntuned, "drum.snare-drum"},
	{GroupUntuned, "drum.bass-drum"},
	{GroupUntuned, "drum.tom-tom"},
	{GroupUntuned, "drum.tenor-drum"},
	{GroupUntuned, "drum.rototom"},
	{GroupUntuned, "metal.cymbal.crash"},
	{GroupUntuned, "metal.cymbal.ride"},
	{GroupUntuned, "metal.cymbal.suspended"},
	{GroupUntuned, "metal.cymbal.splash"},
	{GroupUntuned, "metal.cymbal.chinese"},
	{GroupUntuned, "metal.cymbal.sizzle"},
	{GroupUntuned, "metal.cymbal.clash"},
	{GroupUntuned, "metal.cymbal.finger"},
	{GroupUntuned, "metal.hi-hat"},
	{GroupUntuned, "metal.triangle"},
	{GroupUntuned, "drum.tambourine"},
	{GroupUntuned, "metal.tamtam"},
	{GroupUntuned, "metal.gong"},
	{GroupUntuned, "metal.bells.cowbell"},
	{GroupUntuned, "metal.bells.agogo"},
	{GroupUntuned, "metal.bells.sleigh-bells"},
	{GroupUntuned, "metal.bells.wind-chimes"},
	{GroupUntuned, "metal.bells.mark-tree"},
	{GroupUntuned, "metal.bells.almglocken"},
	{GroupUntuned, "metal.bells.church"},
	{GroupUntuned, "metal.bells.bell-plate"},
	{GroupUntuned, "metal.anvil"},
	{GroupUntuned, "metal.flexatone"},
	{GroupUntuned, "metal.thundersheet"},
	{GroupUntuned, "metal.washboard"},
	{GroupUntuned, "metal.spoons"},
	{GroupUntuned, "metal.berimbau"},
	{GroupUntuned, "wood.wood-block"},
	{GroupUntuned, "wood.temple-block"},
	{GroupUntuned, "wood.jam-block"},
	{GroupUntuned, "wood.agogo-block"},
	{GroupUntuned, "wood.claves"},
	{GroupUntuned, "wood.castanets"},
	{GroupUntuned, "wood.guiro"},
	{GroupUntuned, "wood.slapstick"},
	{GroupUntuned, "wood.bones"},
	{GroupUntuned, "wood.sand-block"},
	{GroupUntuned, "wood.drum-sticks"},
	{GroupUntuned, "rattle.maraca"},
	{GroupUntuned, "rattle.cabasa"},
	{GroupUntuned, "rattle.shaker"},
	{GroupUntuned, "rattle.shekere"},
	{GroupUntuned, "rattle.caxixi"},
	{GroupUntuned, "rattle.vibraslap"},
	{GroupUntuned, "rattle.ratchet"},
	{GroupUntuned, "rattle.rain-stick"},
	{GroupUntuned, "drum.bongo"},
	{GroupUntuned, "drum.conga"},
	{GroupUntuned, "drum.timbale"},
	{GroupUntuned, "drum.djembe"},
	{GroupUntuned, "drum.cajon"},
	{GroupUntuned, "drum.tabla"},
	{GroupUntuned, "drum.taiko"},
	{GroupUntuned, "drum.frame-drum"},
	{GroupUntuned, "drum.bodhran"},
	{GroupUntuned, "drum.darabuka"},
	{GroupUntuned, "drum.surdo"},
	{GroupUntuned, "drum.hand-drum"},
	{GroupUntuned, "effect.wind"},
	{GroupUntuned, "effect.applause"},
	{GroupUntuned, "effect.hand-clap"},
	{GroupUntuned, "effect.whistle.police"},
	{GroupUntuned, "effect.whistle.slide"},
	{GroupUntuned, "effect.bird"},
	{GroupUntuned, "effect.siren"},
	{GroupUntuned, "effect.gunshot"},
	{GroupUntuned, "effect.metronome-click"},
	{GroupUntuned, "voice.percussion.beatbox"},

	{GroupElectronic, "synth.group"},
	{GroupElectronic, "synth.pad"},
	{GroupElectronic, "synth.pad.warm"},
	{GroupElectronic, "synth.tone.square"},
	{GroupElectronic, "synth.tone.sawtooth"},
	{GroupElectronic, "synth.tone.sine"},
	{GroupElectronic, "synth.effects"},
	{GroupElectronic, "synth.theremin"},
	{GroupElectronic, "keyboard.ondes-martenot"},
	{GroupElectronic, "strings.group.synth"},
}

// Written is how a staff for an instrument is laid out.
//
// An instrument and the way it is written down are two questions, and for
// a fretted instrument they are both real: the same guitar part is written
// on a staff of pitches, as tablature, or as both one above the other.
// None of them is a different instrument — it is one player reading one
// line — so this sits beside the sound rather than inventing sounds of its
// own, and the sound written into a file is the same whichever is chosen.
type Written int

const (
	// WrittenPitches is an ordinary staff.
	WrittenPitches Written = iota
	// WrittenTab is a tab staff: a line per string, with the fret on it.
	WrittenTab
	// WrittenBoth is the pair a guitar part is usually printed as —
	// pitches above, tablature below, braced together with the barlines
	// run through, because it is one player reading one line twice over.
	WrittenBoth
)

func (w Written) String() string {
	switch w {
	case WrittenTab:
		return "tab"
	case WrittenBoth:
		return "both"
	}
	return "pitches"
}

// WrittenFromName reads back what String wrote.
func WrittenFromName(s string) (Written, bool) {
	switch s {
	case "", "pitches", "staff":
		return WrittenPitches, true
	case "tab", "tablature":
		return WrittenTab, true
	case "both", "pair":
		return WrittenBoth, true
	}
	return 0, false
}

// fretted are the instruments offered as tablature as well as on a staff,
// with the tuning each is written in unless somebody says otherwise.
//
// Not every plucked thing: a harp has no frets and a sitar has no tab
// anybody writes. These are the ones people actually ask for tab for.
var fretted = map[string]string{
	"pluck.guitar":              "guitar",
	"pluck.guitar.acoustic":     "guitar",
	"pluck.guitar.nylon-string": "guitar",
	"pluck.guitar.steel-string": "guitar",
	"pluck.guitar.electric":     "guitar",
	"pluck.bass":                "bass",
	"pluck.bass.acoustic":       "bass",
	"pluck.bass.electric":       "bass",
	"pluck.ukulele":             "ukulele",
	"pluck.mandolin":            "mandolin",
	"pluck.banjo":               "banjo — five string",
}

// DefaultTuning is the tuning a fretted instrument is written in unless
// somebody chooses another, and "" for one that is not fretted.
func DefaultTuning(sound string) string { return fretted[sound] }

// CatalogueItem is one instrument as it is offered.
type CatalogueItem struct {
	// Group is the cluster it is offered in: "Woodwind", "Brass".
	Group string `json:"group"`
	// Name is what it is called: "bass clarinet".
	Name string `json:"name"`
	// Sound is the MusicXML standard sound id, which is what gets written
	// into the file and is what says which instrument this is.
	Sound string `json:"sound"`
	// Program is General MIDI's nearest, which is what plays it.
	Program Instrument `json:"program"`
	// Percussion says it is read by position on a kit rather than by
	// pitch, so its staff is a percussion staff and it has no range.
	Percussion bool `json:"percussion,omitempty"`
	// Low and High are the written compass, as MIDI note numbers, where
	// gscore knows it. Zero and zero where it does not — a section like
	// "strings" has no one range and nothing is claimed for it.
	Low  int `json:"low,omitempty"`
	High int `json:"high,omitempty"`
	// ScoreRank is where this instrument goes down the page of a score:
	// the lower the number, the nearer the top. A different question
	// from Group, which is how the list is ordered for finding
	// something — see scoreorder.go.
	ScoreRank int `json:"scoreRank"`
	// Written is how the staff is laid out: a staff of pitches, a tab,
	// or both braced together. Only a fretted instrument is offered more
	// than one.
	Written string `json:"written,omitempty"`
	// Tuning is the tab's tuning, where it has one.
	Tuning string `json:"tuning,omitempty"`
}

// Catalogue is every instrument that can be chosen, in the order offered.
//
// Built from the sounds table every time rather than written out twice: the
// name, the program and the range are facts about the instrument and belong
// in one place, and a catalogue that repeated them would be a second place
// for them to disagree.
func Catalogue() []CatalogueItem {
	out := make([]CatalogueItem, 0, len(catalogue))
	for _, c := range catalogue {
		s, ok := sounds[c.Sound]
		if !ok {
			// Not reachable: the test walks this list and fails on an id
			// the table does not have. Skipped rather than panicking,
			// because a missing instrument is not worth refusing to open
			// the program over.
			continue
		}
		it := CatalogueItem{
			Group: c.Group, Name: s.name, Sound: c.Sound,
			Program: s.program, Percussion: s.percussion,
			ScoreRank: ScoreRank(c.Sound),
		}
		if s.hasReach {
			it.Low, it.High = s.reach.Low, s.reach.High
		}
		out = append(out, it)

		// A fretted instrument is offered the other two ways of writing
		// it down, beside the plain staff. Not separate instruments —
		// the same sound is written into the file for all three — which
		// is why they are made here rather than listed above.
		tuning, isFretted := fretted[c.Sound]
		if !isFretted {
			continue
		}
		for _, w := range []Written{WrittenTab, WrittenBoth} {
			v := it
			v.Written, v.Tuning = w.String(), tuning
			v.Name = s.name + writtenSuffix(w)
			out = append(out, v)
		}
	}
	return out
}

// writtenSuffix is how the other ways of writing an instrument down are
// told apart in a list of them.
func writtenSuffix(w Written) string {
	switch w {
	case WrittenTab:
		return " — tablature"
	case WrittenBoth:
		return " — notation and tablature"
	}
	return ""
}

// SoundForInstrument is the catalogue's name for a General MIDI program.
//
// Scores arrive knowing what plays them without knowing what gscore calls
// it: a MIDI file, a NoteWorthy staff and an ABC tune all say "program 73"
// or "Flute", and the catalogue says "wind.flutes.flute". Everything built
// on the catalogue — the score setup dialog, the per-instrument soundfonts,
// the range checks — is looking for the second of those, and a part that
// has only the first is a part they all read as a piano.
//
// By name first and by program second. Several instruments share a program
// because General MIDI has fewer sounds than the world has instruments, and
// the one that shares its name with the program is the one that program is
// for: program 58 is a tuba, and the euphonium filed under it as the
// nearest thing is not what a file saying 58 means.
//
// Percussion is left out. A drum staff is not an instrument among the
// others — the same patch number means a snare in one bank and a
// harpsichord in the other — and what makes a staff percussion here is its
// clef.
func SoundForInstrument(i Instrument) (string, bool) {
	name := i.String()
	first := ""
	for _, c := range Catalogue() {
		if c.Percussion || c.Program != i {
			continue
		}
		if c.Name == name {
			return c.Sound, true
		}
		if first == "" {
			first = c.Sound
		}
	}
	return first, first != ""
}
