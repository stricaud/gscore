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
	}
	return out
}
