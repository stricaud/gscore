/*
 * chordtones.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// What notes a named chord is made of.
//
// The other direction from chord.go, which reads a set of notes and says
// what they are called. This reads "Cmaj7" and says which notes that is —
// the question anybody writing an accompaniment from a lead sheet is
// asking, and the question a script cannot answer for itself.
//
// Here rather than in the editor because this is what a chord is, and the
// program should know that in one place. The bar-numbering rule was written
// out three times in this codebase and the three copies disagreed by one;
// chord spellings written out twice would disagree about something harder
// to notice than a bar number.
//
// A tone is a degree and a distance, not a distance alone. Four semitones
// above C can be written E or F flat, and which of the two it is cannot be
// worked out from the four: it is the third of the chord, so it is spelled
// on the third letter up. That is the whole reason this table has two
// numbers in it and the one in chord.go has one — naming a set of notes
// needs only the distances, spelling a chord needs the degrees.

package analysis

import "strings"

// Tone is one note of a chord: which degree of the scale above the root it
// is spelled as, counted in letters — 0 the root, 2 the third, 4 the fifth,
// 6 the seventh, 8 the ninth — and how many semitones above the root it
// sounds.
type Tone struct {
	Degree    int `json:"degree"`
	Semitones int `json:"semitones"`
}

// The chords, each written once and then named by every spelling anybody
// types for it.
//
// The ninths, elevenths and thirteenths are the ones a player would hold
// rather than every note the arithmetic allows: an eleventh has no third in
// it, because the third and the eleventh a semitone apart is the sound
// nobody means by C11, and a thirteenth leaves out the eleventh for the
// same reason. These are the voicings a lead sheet is read as, not a
// theory exercise.
var (
	tMajor = []Tone{{0, 0}, {2, 4}, {4, 7}}
	tMinor = []Tone{{0, 0}, {2, 3}, {4, 7}}
	tDim   = []Tone{{0, 0}, {2, 3}, {4, 6}}
	tAug   = []Tone{{0, 0}, {2, 4}, {4, 8}}
	tSus4  = []Tone{{0, 0}, {3, 5}, {4, 7}}
	tSus2  = []Tone{{0, 0}, {1, 2}, {4, 7}}
	tFifth = []Tone{{0, 0}, {4, 7}}

	tSixth     = []Tone{{0, 0}, {2, 4}, {4, 7}, {5, 9}}
	tMinor6    = []Tone{{0, 0}, {2, 3}, {4, 7}, {5, 9}}
	tDom7      = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 10}}
	tMajor7    = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 11}}
	tMinor7    = []Tone{{0, 0}, {2, 3}, {4, 7}, {6, 10}}
	tHalfDim   = []Tone{{0, 0}, {2, 3}, {4, 6}, {6, 10}}
	tDim7      = []Tone{{0, 0}, {2, 3}, {4, 6}, {6, 9}}
	tMinMaj7   = []Tone{{0, 0}, {2, 3}, {4, 7}, {6, 11}}
	tAug7      = []Tone{{0, 0}, {2, 4}, {4, 8}, {6, 10}}
	tDom7Flat5 = []Tone{{0, 0}, {2, 4}, {4, 6}, {6, 10}}
	tDom7Sus4  = []Tone{{0, 0}, {3, 5}, {4, 7}, {6, 10}}
	tAdd9      = []Tone{{0, 0}, {2, 4}, {4, 7}, {8, 14}}

	tNinth    = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 10}, {8, 14}}
	tMajor9   = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 11}, {8, 14}}
	tMinor9   = []Tone{{0, 0}, {2, 3}, {4, 7}, {6, 10}, {8, 14}}
	tSixNine  = []Tone{{0, 0}, {2, 4}, {4, 7}, {5, 9}, {8, 14}}
	tFlat9    = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 10}, {8, 13}}
	tSharp9   = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 10}, {8, 15}}
	tEleventh = []Tone{{0, 0}, {4, 7}, {6, 10}, {8, 14}, {10, 17}}
	tMinor11  = []Tone{{0, 0}, {2, 3}, {4, 7}, {6, 10}, {8, 14}, {10, 17}}
	tThirteen = []Tone{{0, 0}, {2, 4}, {4, 7}, {6, 10}, {8, 14}, {12, 21}}
	tMinor13  = []Tone{{0, 0}, {2, 3}, {4, 7}, {6, 10}, {8, 14}, {12, 21}}
)

// chordTones is every spelling of a quality anybody types, each pointing at
// the chord it means.
//
// Case matters in exactly one place and it matters a great deal: M7 is a
// major seventh and m7 is a minor one. So the spellings are looked up as
// they were typed first, and only then lowered — which is why both "M7" and
// "m7" are in here and why neither of them is reached by lowering the
// other.
var chordTones = map[string][]Tone{
	"":      tMajor,
	"maj":   tMajor,
	"major": tMajor,
	"M":     tMajor,
	"Maj":   tMajor,

	"m":     tMinor,
	"mi":    tMinor,
	"min":   tMinor,
	"minor": tMinor,
	"-":     tMinor,

	"dim":     tDim,
	"o":       tDim,
	"°":       tDim,
	"aug":     tAug,
	"+":       tAug,
	"sus":     tSus4,
	"sus4":    tSus4,
	"sus2":    tSus2,
	"5":       tFifth,
	"6":       tSixth,
	"maj6":    tSixth,
	"M6":      tSixth,
	"m6":      tMinor6,
	"min6":    tMinor6,
	"-6":      tMinor6,
	"7":       tDom7,
	"dom7":    tDom7,
	"maj7":    tMajor7,
	"ma7":     tMajor7,
	"M7":      tMajor7,
	"Δ":       tMajor7,
	"Δ7":      tMajor7,
	"m7":      tMinor7,
	"min7":    tMinor7,
	"-7":      tMinor7,
	"m7b5":    tHalfDim,
	"ø":       tHalfDim,
	"ø7":      tHalfDim,
	"halfdim": tHalfDim,
	"dim7":    tDim7,
	"o7":      tDim7,
	"°7":      tDim7,
	"mmaj7":   tMinMaj7,
	"mM7":     tMinMaj7,
	"-maj7":   tMinMaj7,
	"aug7":    tAug7,
	"7#5":     tAug7,
	"+7":      tAug7,
	"7b5":     tDom7Flat5,
	"7sus":    tDom7Sus4,
	"7sus4":   tDom7Sus4,
	"add9":    tAdd9,
	"9":       tNinth,
	"maj9":    tMajor9,
	"M9":      tMajor9,
	"Δ9":      tMajor9,
	"m9":      tMinor9,
	"min9":    tMinor9,
	"-9":      tMinor9,
	"6/9":     tSixNine,
	"69":      tSixNine,
	"7b9":     tFlat9,
	"7#9":     tSharp9,
	"11":      tEleventh,
	"m11":     tMinor11,
	"min11":   tMinor11,
	"-11":     tMinor11,
	"13":      tThirteen,
	"m13":     tMinor13,
	"min13":   tMinor13,
	"-13":     tMinor13,
}

// ChordTones says what notes a quality is made of — the part of a chord
// symbol after the root, so "maj7" rather than "Cmaj7".
//
// Nothing is guessed. A quality this does not know comes back as not known,
// and the caller says so; a chord filled in with the triad somebody probably
// meant is a chord that will be read, believed and played.
func ChordTones(quality string) ([]Tone, bool) {
	raw := strings.TrimSpace(quality)
	// As typed, so that M7 and m7 are told apart before anything is lowered.
	if t, ok := chordTones[raw]; ok {
		return t, true
	}
	if t, ok := chordTones[strings.ToLower(raw)]; ok {
		return t, true
	}
	return nil, false
}

// ChordQualities is every spelling this knows, for a caller that has to hand
// the whole table somewhere else — the editor's console, which looks a chord
// up without being able to ask.
//
// A copy: the table is the program's one statement about what a chord is,
// and something that was handed it should not be able to edit it.
func ChordQualities() map[string][]Tone {
	out := make(map[string][]Tone, len(chordTones))
	for name, tones := range chordTones {
		out[name] = append([]Tone(nil), tones...)
	}
	return out
}
