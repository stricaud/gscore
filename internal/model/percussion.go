/*
 * percussion.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// Percussion notation.
//
// A drum staff looks like a staff and is not one. The five lines carry no
// pitches: a position on them names an instrument, and which instrument it
// names is a convention, not a fact about sound. The top space is a hi-hat
// because drummers agree it is.
//
// Finale calls this a percussion layout and gives each position a "note type"
// — an instrument, a notehead and a MIDI key — with the mouse naming the
// instrument as it moves over the staff, and + and − cycling between the
// instruments that share a position. That is the right design and this is it:
// a table from staff position to instrument, with alternatives where real
// drummers write two things in the same place.
//
// The positions follow the standard drum-set notation that Weinberg set out
// and everybody since has used, so a part written here is readable by a
// drummer who has never heard of gscore.

// NoteheadKind is the shape a note is drawn with.
//
// On a pitched staff it is always the ordinary oval. On a drum staff it is
// how a cymbal is told from a drum at a glance, which matters more than it
// sounds when the two share a line.
type NoteheadKind int

const (
	HeadNormal   NoteheadKind = iota
	HeadX                     // cymbals, hi-hat, and anything struck on metal
	HeadCircleX               // open hi-hat, and the cymbals played with a bow
	HeadTriangle              // the rims and the cowbells
	HeadDiamond               // harmonics, and a cymbal left to ring
	HeadSlash                 // rhythm to be improvised on
)

var noteheadKindNames = [...]string{
	"normal", "x", "circle-x", "triangle", "diamond", "slash",
}

func (k NoteheadKind) String() string {
	if k < HeadNormal || int(k) >= len(noteheadKindNames) {
		return "normal"
	}
	return noteheadKindNames[k]
}

// NoteheadKindFromName is the inverse, for reading files and query strings.
func NoteheadKindFromName(s string) (NoteheadKind, bool) {
	for i, n := range noteheadKindNames {
		if n == s {
			return NoteheadKind(i), true
		}
	}
	return HeadNormal, false
}

// PercussionNote is one instrument of a kit: where it is written, what it is
// called, how it is drawn and which drum sounds.
type PercussionNote struct {
	// Step is the staff position, counted in half-spaces from the bottom
	// line: 0 is the bottom line, 8 the top, and the numbers run outside the
	// staff in both directions.
	Step int
	Name string
	// Key is the General MIDI percussion key — the note number that plays
	// this drum on channel 10.
	Key  int
	Head NoteheadKind
}

// DrumSet is the layout of an ordinary kit.
//
// Where two instruments share a position, the first is what a click writes and
// the others are what cycling through them offers, which is how a drummer
// writes an open hi-hat where a closed one would otherwise go.
var DrumSet = []PercussionNote{
	{Step: -2, Name: "Hi-hat with foot", Key: 44, Head: HeadX},
	{Step: -1, Name: "Bass drum", Key: 35, Head: HeadNormal},
	{Step: 1, Name: "Bass drum", Key: 36, Head: HeadNormal},
	{Step: 2, Name: "Low floor tom", Key: 41, Head: HeadNormal},
	{Step: 3, Name: "High floor tom", Key: 43, Head: HeadNormal},
	{Step: 4, Name: "Low tom", Key: 45, Head: HeadNormal},
	{Step: 5, Name: "Snare drum", Key: 38, Head: HeadNormal},
	{Step: 5, Name: "Side stick", Key: 37, Head: HeadCircleX},
	{Step: 6, Name: "Low-mid tom", Key: 47, Head: HeadNormal},
	{Step: 7, Name: "High tom", Key: 50, Head: HeadNormal},
	{Step: 8, Name: "Ride cymbal", Key: 51, Head: HeadX},
	{Step: 8, Name: "Ride bell", Key: 53, Head: HeadDiamond},
	{Step: 9, Name: "Crash cymbal", Key: 49, Head: HeadX},
	{Step: 10, Name: "Hi-hat closed", Key: 42, Head: HeadX},
	{Step: 10, Name: "Hi-hat open", Key: 46, Head: HeadCircleX},
	{Step: 11, Name: "Splash cymbal", Key: 55, Head: HeadX},
	{Step: 12, Name: "Chinese cymbal", Key: 52, Head: HeadX},
	{Step: 13, Name: "Cowbell", Key: 56, Head: HeadTriangle},
}

// PercussionAt lists every instrument written at a staff position, in the
// order a click cycles through them. It is empty where nothing is written.
func PercussionAt(step int) []PercussionNote {
	var out []PercussionNote
	for _, n := range DrumSet {
		if n.Step == step {
			out = append(out, n)
		}
	}
	return out
}

// PercussionFor is what a click at a position writes: the first instrument
// there, or the nearest one if the click landed where nothing is written.
func PercussionFor(step int) (PercussionNote, bool) {
	if at := PercussionAt(step); len(at) > 0 {
		return at[0], true
	}
	// Nothing at that exact position, so the closest: clicking slightly off
	// should write the drum that was meant, not nothing at all.
	best, found := PercussionNote{}, false
	gap := 1 << 30
	for _, n := range DrumSet {
		d := n.Step - step
		if d < 0 {
			d = -d
		}
		if d < gap {
			best, gap, found = n, d, true
		}
	}
	return best, found
}

// PercussionByKey finds the instrument a MIDI key stands for, which is what
// reading a drum part out of another program needs.
func PercussionByKey(key int) (PercussionNote, bool) {
	for _, n := range DrumSet {
		if n.Key == key {
			return n, true
		}
	}
	return PercussionNote{}, false
}

// NextPercussion cycles to the next instrument sharing a position, which is
// Finale's + and − keys.
func NextPercussion(step, key, delta int) (PercussionNote, bool) {
	at := PercussionAt(step)
	if len(at) == 0 {
		return PercussionNote{}, false
	}
	i := 0
	for j, n := range at {
		if n.Key == key {
			i = j
			break
		}
	}
	i = ((i+delta)%len(at) + len(at)) % len(at)
	return at[i], true
}
