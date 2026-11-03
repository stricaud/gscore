/*
 * scoreorder.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "sort"

// The order the staves of a score are laid out in.
//
// A second ranking, and not the one the catalogue is in. The catalogue is
// ordered for finding an instrument — somebody looking for a cor anglais
// looks under woodwind, near the oboe — and a score is ordered by a
// convention about the page: woodwind at the top, then brass, percussion,
// harp, keyboard, voices, and the strings along the bottom. The two
// disagree even inside a family. A picker puts the flute first because it
// is the one people want; a score puts the piccolo first because it is the
// highest.
//
// Nobody can work this out from the instruments themselves. It is a
// convention, so it is written down.

// scoreOrder is the instruments a score is likely to hold, in the order
// they are laid out. Anything not here falls in with its family — see
// ScoreRank — so the list is the instruments whose place is settled by
// convention rather than every instrument there is.
var scoreOrder = []string{
	// Woodwind, high to low within each kind, flutes before reeds.
	"wind.flutes.flute.piccolo",
	"wind.flutes.flute",
	"wind.flutes.flute.alto",
	"wind.flutes.flute.bass",
	"wind.flutes.recorder.sopranino",
	"wind.flutes.recorder.soprano",
	"wind.flutes.recorder",
	"wind.flutes.recorder.alto",
	"wind.flutes.recorder.tenor",
	"wind.flutes.recorder.bass",
	"wind.reed.oboe",
	"wind.reed.oboe-damore",
	"wind.reed.english-horn",
	"wind.reed.oboe-da-caccia",
	"wind.reed.oboe.bass",
	"wind.reed.clarinet.eflat",
	"wind.reed.clarinet",
	"wind.reed.clarinet.bflat",
	"wind.reed.clarinet.a",
	"wind.reed.basset-horn",
	"wind.reed.clarinet.alto",
	"wind.reed.clarinet.bass",
	"wind.reed.clarinet.contra-alto",
	"wind.reed.clarinet.contrabass",
	"wind.reed.saxophone.sopranino",
	"wind.reed.saxophone.soprano",
	"wind.reed.saxophone.alto",
	"wind.reed.saxophone.tenor",
	"wind.reed.saxophone.baritone",
	"wind.reed.saxophone.bass",
	"wind.reed.bassoon",
	"wind.reed.contrabassoon",

	// Brass. The horn comes first, which is the one place the rule is
	// not "highest first": it sits above the trumpets because it is the
	// voice that joins the woodwind to the rest of the brass, and every
	// printed score does it.
	"brass.french-horn",
	"brass.natural-horn",
	"brass.wagner-tuba",
	"brass.trumpet.piccolo",
	"brass.trumpet",
	"brass.trumpet.bflat",
	"brass.trumpet.c",
	"brass.trumpet.d",
	"brass.trumpet.bass",
	"brass.cornet",
	"brass.flugelhorn",
	"brass.mellophone",
	"brass.alto-horn",
	"brass.trombone.alto",
	"brass.trombone",
	"brass.trombone.tenor",
	"brass.trombone.bass",
	"brass.trombone.contrabass",
	"brass.euphonium",
	"brass.baritone-horn",
	"brass.tuba",
	"brass.tuba.bass",
	"brass.tuba.subcontrabass",
	"brass.sousaphone",
	"brass.cimbasso",

	// Percussion. The timpani first and on their own — they are the one
	// percussion part an orchestra has always had, and they are written
	// above the rest of the kit rather than among it.
	"drum.timpani",
	"pitched-percussion.glockenspiel",
	"metal.crotales",
	"pitched-percussion.xylophone",
	"pitched-percussion.vibraphone",
	"pitched-percussion.marimba",
	"pitched-percussion.marimba.bass",
	"pitched-percussion.tubular-bells",
	"metal.steel-drums",
	"drum.group.set",
	"drum.snare-drum",
	"drum.tenor-drum",
	"drum.bass-drum",
	"metal.cymbal.crash",
	"metal.cymbal.suspended",
	"metal.triangle",
	"drum.tambourine",
	"metal.tamtam",
	"metal.gong",

	// Harp, then the keyboards, then the voices.
	"pluck.harp",
	"pluck.celtic-harp",
	"keyboard.celesta",
	"keyboard.piano",
	"keyboard.piano.grand",
	"keyboard.harpsichord",
	"keyboard.organ",
	"keyboard.organ.pipe",

	"voice.soprano",
	"voice.mezzo-soprano",
	"voice.alto",
	"voice.countertenor",
	"voice.tenor",
	"voice.baritone",
	"voice.bass",
	"voice.vocals",
	"voice.female",
	"voice.male",

	// And the strings along the bottom.
	"strings.violin",
	"strings.viola",
	"strings.cello",
	"strings.contrabass",
	"strings.group",
}

// familyOrder is where a family sits when none of its instruments is named
// in scoreOrder above. The numbers are spaced so the named instruments,
// which rank far below these, always come first within a family.
var familyOrder = map[string]int{
	GroupWoodwind:   10,
	GroupBrass:      20,
	GroupTuned:      30,
	GroupUntuned:    40,
	GroupPlucked:    50,
	GroupKeyboard:   60,
	GroupVoice:      70,
	GroupBowed:      80,
	GroupElectronic: 90,
}

// rank is built once from the list above.
var rank = func() map[string]int {
	out := make(map[string]int, len(scoreOrder))
	for i, id := range scoreOrder {
		out[id] = i
	}
	return out
}()

// ScoreRank is where an instrument goes down the page: the lower the
// number, the nearer the top.
//
// An instrument the list does not name falls in with its family, after
// the ones that are named there. That is the honest answer for a shakuhachi
// or a theremin — the convention has nothing to say about where they go,
// only which company they keep.
func ScoreRank(sound string) int {
	if r, ok := rank[sound]; ok {
		return r
	}
	// Past the end of the named list, so a named instrument always wins.
	//
	// Read off the raw catalogue and not off Catalogue(), which fills in
	// a rank of its own by calling this: the two would call each other
	// until the stack ran out, and did.
	base := len(scoreOrder)
	for _, c := range catalogue {
		if c.Sound != sound {
			continue
		}
		return base + familyOrder[c.Group]*100
	}
	// Not in the catalogue at all: last, rather than silently first.
	return base + 10000
}

// InScoreOrder sorts instruments into the order a score is laid out.
//
// Stable, so two of the same instrument keep the order they were chosen
// in — the first and second flute stay the first and second flute.
func InScoreOrder(sounds []string) []string {
	out := make([]string, len(sounds))
	copy(out, sounds)
	sort.SliceStable(out, func(i, j int) bool {
		return ScoreRank(out[i]) < ScoreRank(out[j])
	})
	return out
}

// ScoreFamily is the heading an instrument is bracketed under in a score.
//
// The catalogue's group, with the two kinds of percussion counted as one:
// a score brackets the percussion together, timpani and snare drum alike,
// because they are one section of players however they are read.
func ScoreFamily(sound string) string {
	// The raw list, for the reason ScoreRank reads it: Catalogue() asks
	// this file for a rank, so asking Catalogue() back is a loop.
	for _, c := range catalogue {
		if c.Sound != sound {
			continue
		}
		if c.Group == GroupUntuned {
			return GroupTuned
		}
		return c.Group
	}
	return ""
}

// GrandStaff reports an instrument normally written on two staves joined
// by a brace: the two hands of a piano, the two of a harp.
func GrandStaff(sound string) bool {
	switch sound {
	case "keyboard.piano", "keyboard.piano.grand", "keyboard.piano.upright",
		"keyboard.piano.electric", "keyboard.harpsichord", "keyboard.celesta",
		"keyboard.organ", "keyboard.organ.pipe", "keyboard.organ.reed",
		"keyboard.fortepiano", "pluck.harp", "pluck.celtic-harp",
		"keyboard.clavichord":
		return true
	}
	return false
}
