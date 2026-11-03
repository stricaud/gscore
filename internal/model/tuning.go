/*
 * tuning.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// The tunings a tab staff can be written for.
//
// A tablature is a tuning and a number of frets, and without the tuning it
// says nothing: the same "3" on the same line is a G on a guitar and a C on
// a ukulele. So a tab staff cannot be made without choosing one, and these
// are the ones somebody would choose.
//
// Written from the top line of the tab downwards, which is how Tablature
// stores them and how the strings are numbered: string 1 is the line at the
// top. That is a position on the page and not an order of pitch — a ukulele's
// fourth string is higher than its third, and a banjo's fifth is higher than
// all of them. Both are written here the way they are played.

// Tuning is one named way of tuning a fretted instrument.
type Tuning struct {
	// Name is what to call it: "guitar", "guitar — drop D".
	Name string
	// Strings are the open strings, from the top line of the tab down.
	Strings []Pitch
	// Frets is how many the instrument has, which is what says whether a
	// note can be played at all.
	Frets int
}

// p is one open string, written as it is spoken: "E4" is the guitar's top.
func p(s string) Pitch {
	out, _, ok := ParsePitch(s)
	if !ok {
		panic("tuning: " + s + " is not a pitch")
	}
	return out
}

// tunings are the ones offered, in the order they are offered: the common
// ones first, and within an instrument the standard tuning before the rest.
var tunings = []Tuning{
	{"guitar", []Pitch{p("E4"), p("B3"), p("G3"), p("D3"), p("A2"), p("E2")}, 24},
	{"guitar — drop D", []Pitch{p("E4"), p("B3"), p("G3"), p("D3"), p("A2"), p("D2")}, 24},
	{"guitar — DADGAD", []Pitch{p("D4"), p("A3"), p("G3"), p("D3"), p("A2"), p("D2")}, 24},
	{"guitar — open G", []Pitch{p("D4"), p("B3"), p("G3"), p("D3"), p("G2"), p("D2")}, 24},
	{"guitar — half step down", []Pitch{p("Eb4"), p("Bb3"), p("Gb3"), p("Db3"), p("Ab2"), p("Eb2")}, 24},
	{"guitar — seven string", []Pitch{p("E4"), p("B3"), p("G3"), p("D3"), p("A2"), p("E2"), p("B1")}, 24},
	{"bass", []Pitch{p("G2"), p("D2"), p("A1"), p("E1")}, 24},
	{"bass — five string", []Pitch{p("G2"), p("D2"), p("A1"), p("E1"), p("B0")}, 24},
	{"bass — six string", []Pitch{p("C3"), p("G2"), p("D2"), p("A1"), p("E1"), p("B0")}, 24},
	// Reentrant: the fourth string is higher than the third, which is what
	// gives the instrument its sound and is why these are stored by
	// position rather than sorted by pitch.
	{"ukulele", []Pitch{p("A4"), p("E4"), p("C4"), p("G4")}, 15},
	{"ukulele — baritone", []Pitch{p("E4"), p("B3"), p("G3"), p("D3")}, 15},
	{"mandolin", []Pitch{p("E5"), p("A4"), p("D4"), p("G3")}, 20},
	{"banjo — five string", []Pitch{p("D4"), p("B3"), p("G3"), p("D3"), p("G4")}, 22},
	{"banjo — tenor", []Pitch{p("A4"), p("D4"), p("G3"), p("C3")}, 22},
	{"dulcimer", []Pitch{p("D4"), p("A3"), p("D3")}, 16},
}

// Tunings is every tuning that can be chosen, in the order offered.
func Tunings() []Tuning {
	out := make([]Tuning, len(tunings))
	copy(out, tunings)
	return out
}

// TuningNamed is the tablature for a named tuning.
//
// A fresh one every time: a Tablature belongs to the staff it is on, and
// two staves handed the same one would change together.
func TuningNamed(name string) (*Tablature, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, t := range tunings {
		if strings.ToLower(t.Name) != want {
			continue
		}
		out := &Tablature{Frets: t.Frets}
		out.Strings = append(out.Strings, t.Strings...)
		return out, true
	}
	return nil, false
}

// TuningName is what a tablature is called, where it is one of the tunings
// that can be chosen, and "custom" where it is not.
//
// A file can carry any tuning at all — somebody else's program wrote it, or
// somebody retuned a string — and that is not an error. It has no name
// here, which is all this says.
func TuningName(t *Tablature) string {
	if t == nil {
		return ""
	}
	for _, known := range tunings {
		if len(known.Strings) != len(t.Strings) {
			continue
		}
		same := true
		for i, s := range known.Strings {
			if s.MIDI() != t.Strings[i].MIDI() {
				same = false
				break
			}
		}
		if same {
			return known.Name
		}
	}
	return "custom"
}

// Fretted is where a pitch is played on this tablature, preferring the
// string the music already named.
//
// Different from At, which always answers with the highest string that
// reaches. This one is asked when a note is being written and a string has
// been chosen — by the player, or by the line of the tab that was clicked —
// and it answers whether that choice can be honoured.
func (t *Tablature) Fretted(pitch Pitch, str int) (fret int, ok bool) {
	if t == nil || str < 1 || str > len(t.Strings) {
		return 0, false
	}
	fret = pitch.MIDI() - t.Strings[str-1].MIDI()
	if fret < 0 || (t.Frets > 0 && fret > t.Frets) {
		return 0, false
	}
	return fret, true
}

// PitchAt is what sounds when a string is stopped at a fret.
//
// The other direction from Fretted, and the one a player types: a tab is
// written by saying which fret, not by saying which pitch. The pitch is
// worked out and kept, because the pitch is what sounds, what transposes
// and what every other part of the program reads.
func (t *Tablature) PitchAt(str, fret int) (Pitch, bool) {
	if t == nil || str < 1 || str > len(t.Strings) {
		return Pitch{}, false
	}
	if fret < 0 || (t.Frets > 0 && fret > t.Frets) {
		return Pitch{}, false
	}
	return spelledAbove(t.Strings[str-1], fret), true
}

// Spellings of the twelve pitch classes, sharp and flat.
var (
	sharpSpelling = [12][2]int{
		{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {3, 0},
		{3, 1}, {4, 0}, {4, 1}, {5, 0}, {5, 1}, {6, 0},
	}
	flatSpelling = [12][2]int{
		{0, 0}, {1, -1}, {1, 0}, {2, -1}, {2, 0}, {3, 0},
		{4, -1}, {4, 0}, {5, -1}, {5, 0}, {6, -1}, {6, 0},
	}
)

// spelledAbove is the pitch a number of semitones above an open string,
// written the way that string is written.
//
// A fret number is a number of semitones and says nothing about spelling,
// so something has to choose, and the string it is played on is the only
// thing in the question that has an opinion. A guitar tuned down a semitone
// has its strings written E flat and A flat, and its third fret should come
// out as G flat and C flat rather than F sharp and B — the player is
// reading a part in flats.
//
// Sharps otherwise, which is the ordinary way a chromatic note is written
// going up.
func spelledAbove(open Pitch, semis int) Pitch {
	midi := open.MIDI() + semis
	table := sharpSpelling
	if open.Alter < 0 {
		table = flatSpelling
	}
	class := ((midi % 12) + 12) % 12
	step, alter := table[class][0], table[class][1]
	// The octave the letter falls in, which is not always the octave the
	// MIDI number falls in: B sharp and C flat sit either side of the line
	// that divides them.
	oct := (midi-alter)/12 - 1
	if (midi-alter) < 0 && (midi-alter)%12 != 0 {
		oct--
	}
	return Pitch{Step: Step(step), Octave: oct, Alter: alter}
}
