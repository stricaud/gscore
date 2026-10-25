/*
 * drumkit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// Drum kits of different sizes.
//
// A kit is not one thing. A rock drummer writes on five instruments and wants
// five pads in front of them; somebody scoring a Latin chart needs congas,
// timbales, agogo and a cabasa as well, and a pad grid holding forty entries
// is slower to use than a list. So the size is the player's choice, and the
// pads follow it.
//
// Every kit draws from the same table of General MIDI percussion keys, so a
// part written on the small kit and reopened on the large one is the same
// part: changing kits changes which pads are offered, never what is written.

// DrumKit is a named set of pads.
type DrumKit struct {
	// ID is the name this kit is asked for by, and Name is what it is called
	// in front of a person.
	ID, Name string
	// Description says who the kit is for, since "large" alone does not.
	Description string
	Notes       []PercussionNote
}

// basicKit is the five instruments nearly every drum part is written on.
//
// A beginner writing their first groove needs the bass drum, the snare, the
// closed and open hi-hat and a crash, and being shown eighteen pads instead
// of five is how a program teaches somebody that notation is difficult.
var basicKit = []PercussionNote{
	{Step: 10, Name: "Hi-hat closed", Key: 42, Head: HeadX},
	{Step: 10, Name: "Hi-hat open", Key: 46, Head: HeadCircleX},
	{Step: 9, Name: "Crash cymbal", Key: 49, Head: HeadX},
	{Step: 8, Name: "Ride cymbal", Key: 51, Head: HeadX},
	{Step: 5, Name: "Snare drum", Key: 38, Head: HeadNormal},
	{Step: 1, Name: "Bass drum", Key: 36, Head: HeadNormal},
}

// extraPercussion is what the large kit adds: the hand percussion a Latin or
// pit chart calls for, written above the kit's own lines.
//
// The positions carry on upwards past the staff rather than crowding onto the
// lines the kit already uses. A drum part is read by position, so two
// instruments in one place means a player has to stop and look at a key.
var extraPercussion = []PercussionNote{
	{Step: 14, Name: "Tambourine", Key: 54, Head: HeadX},
	{Step: 15, Name: "Hand clap", Key: 39, Head: HeadX},
	{Step: 16, Name: "Triangle open", Key: 81, Head: HeadTriangle},
	{Step: 17, Name: "Triangle muted", Key: 80, Head: HeadTriangle},
	{Step: 18, Name: "Claves", Key: 75, Head: HeadNormal},
	{Step: 19, Name: "Woodblock high", Key: 76, Head: HeadNormal},
	{Step: 20, Name: "Woodblock low", Key: 77, Head: HeadNormal},
	{Step: 21, Name: "Cabasa", Key: 69, Head: HeadX},
	{Step: 22, Name: "Maracas", Key: 70, Head: HeadX},
	{Step: 23, Name: "Guiro long", Key: 74, Head: HeadX},
	{Step: 24, Name: "Vibraslap", Key: 58, Head: HeadX},
	{Step: -3, Name: "Conga high", Key: 62, Head: HeadNormal},
	{Step: -4, Name: "Conga low", Key: 64, Head: HeadNormal},
	{Step: -5, Name: "Bongo high", Key: 60, Head: HeadNormal},
	{Step: -6, Name: "Bongo low", Key: 61, Head: HeadNormal},
	{Step: -7, Name: "Timbale high", Key: 65, Head: HeadNormal},
	{Step: -8, Name: "Timbale low", Key: 66, Head: HeadNormal},
	{Step: -9, Name: "Agogo high", Key: 67, Head: HeadTriangle},
}

// DrumKits are the kits that can be chosen, smallest first.
var DrumKits = []DrumKit{
	{
		ID: "basic", Name: "Basic kit",
		Description: "Bass drum, snare, hi-hat and cymbals — six pads.",
		Notes:       basicKit,
	},
	{
		ID: "standard", Name: "Standard kit",
		Description: "The full drum set: toms, both bass drums, ride bell and side stick.",
		Notes:       DrumSet,
	},
	{
		ID: "large", Name: "Large kit",
		Description: "The drum set with hand percussion — congas, bongos, timbales, claves.",
		Notes:       append(append([]PercussionNote(nil), DrumSet...), extraPercussion...),
	},
}

// DefaultDrumKitID is the kit a new drum staff is offered.
//
// The standard set, because it is the one a drum part is usually written on:
// the basic kit is a deliberate simplification and the large one is a
// deliberate expansion, and defaulting to either would be an opinion about
// music somebody has not written yet.
const DefaultDrumKitID = "standard"

// DrumKitByID finds a kit by name, falling back to the standard one so that
// an unknown name opens something usable rather than nothing.
func DrumKitByID(id string) DrumKit {
	for _, k := range DrumKits {
		if k.ID == id {
			return k
		}
	}
	for _, k := range DrumKits {
		if k.ID == DefaultDrumKitID {
			return k
		}
	}
	return DrumKits[0]
}

// PadShortcuts are the keys a pad is typed with, in the order the pads are
// offered.
//
// Letters, because a drummer typing a groove is typing it with one hand and
// cannot be reaching for F7. They are assigned by position in the kit rather
// than stored against each instrument: a kit with six pads should use the six
// easiest keys, not the six that happen to belong to those drums in a kit
// nobody chose.
const PadShortcuts = "asdfghjklqwertyuiopzxcvbnm"

// shortcutOrder is the order the letters are handed out in: the drums a part
// is mostly made of, first.
//
// A groove is bass drum, snare and hi-hat, so those get the keys under the
// fingers. Handing the letters out by staff position instead gave "a" to the
// cowbell and "s" to the Chinese cymbal, and left the snare drum on "j".
var shortcutOrder = []int{
	36, 38, 42, 46, // bass drum, snare, hi-hat closed and open
	49, 51, // crash, ride
	43, 45, 47, 50, // the toms, low to high
	35, 37, 53, // the second bass drum, side stick, ride bell
	41, 44, 52, 55, 56, // low floor tom, foot hi-hat, chinese, splash, cowbell
}

// shortcutRank is where a drum comes in that order. Anything not named comes
// after everything named, in the order its kit declares it.
func shortcutRank(key int) int {
	for i, k := range shortcutOrder {
		if k == key {
			return i
		}
	}
	return len(shortcutOrder)
}

// Pad is one key of the grid: an instrument, and the letter that types it.
type Pad struct {
	PercussionNote
	Shortcut string
}

// Pads lists a kit's instruments, ready to be shown and typed.
//
// Two different orders are at work here, and running them together was a
// mistake worth naming. The grid is ordered by staff position, so it reads
// down the staff the way the kit sits on it — cymbals at the top, the bass
// drum at the bottom. The letters are handed out by how much a drum is used,
// so the common ones are under the fingers. Ordering the letters by position
// as well is what put the cowbell on "a".
func (k DrumKit) Pads() []Pad {
	out := make([]Pad, len(k.Notes))
	for i, n := range k.Notes {
		out[i] = Pad{PercussionNote: n}
	}

	// Who gets which letter: by rank, ties broken by the kit's own order, so
	// the assignment is the same every time the kit is asked for.
	byRank := make([]int, len(out))
	for i := range byRank {
		byRank[i] = i
	}
	for i := 1; i < len(byRank); i++ {
		for j := i; j > 0; j-- {
			a, b := out[byRank[j]], out[byRank[j-1]]
			if shortcutRank(a.Key) >= shortcutRank(b.Key) {
				break
			}
			byRank[j], byRank[j-1] = byRank[j-1], byRank[j]
		}
	}
	for letter, i := range byRank {
		out[i].Shortcut = Shortcut(letter)
	}

	// How they are shown: highest on the staff first. A stable insertion sort,
	// so two instruments sharing a line stay in the order the kit wrote them
	// — the first of them is the ordinary one.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Step > out[j-1].Step; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Shortcut is the key that types the pad at this index, or the empty string
// once the letters run out. A kit larger than the alphabet is still usable
// with the mouse, which is why running out is not an error.
func Shortcut(i int) string {
	if i < 0 || i >= len(PadShortcuts) {
		return ""
	}
	return string(PadShortcuts[i])
}
