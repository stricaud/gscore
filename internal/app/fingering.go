/*
 * fingering.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"strconv"
	"strings"
)

// Where a guitarist's hand goes for a named chord.
//
// Typing "Dm" and being asked for the fingering as well is asking somebody
// to look up what they already said. Most chords anyone writes are one of a
// few dozen shapes, and the rest are those shapes moved up the neck, so the
// answer can be worked out rather than listed.
//
// Two kinds of answer. The open chords are listed, because they are what a
// guitarist actually plays in first position and they are not regular — C is
// x32010 and nothing about the shape says so. Everything else is a movable
// shape with its root on the sixth string or the fifth, slid up to wherever
// the root is: that is what a barre chord is, and it covers every root and
// every quality in the table below.
//
// It offers one fingering, not the best one. There are a dozen ways to play
// C7 and which is right depends on what comes before it; this is the one a
// player would reach for first, and the field it fills in can be typed over.

// chordQuality is the sort of chord, reduced from however it was written.
type chordQuality int

const (
	qMajor chordQuality = iota
	qMinor
	qDominant7
	qMinor7
	qMajor7
	qSixth
	qMinorSixth
	qSus4
	qSus2
	qFifth
	qDiminished
	qAugmented
	qUnknown
)

// qualityOf reads the part of the name after the root.
//
// The spellings are the ones people type: "m", "min" and "-" all mean minor;
// "maj7", "Δ" and "M7" all mean a major seventh. Case matters in exactly one
// place — M7 is a major seventh and m7 is a minor one — so that is settled
// before anything is lowered, and nothing after it depends on case.
//
// Anything with more in it than a shape covers — C7♯9, Dm11 — is taken as
// the plain chord it is built on, which is the shape a hand starts from
// anyway.
func qualityOf(q string) chordQuality {
	raw := strings.TrimSpace(q)
	switch {
	case raw == "M", raw == "Maj", raw == "MAJ":
		return qMajor
	case strings.HasPrefix(raw, "M7"), strings.HasPrefix(raw, "Ma7"),
		strings.HasPrefix(raw, "Maj7"), strings.HasPrefix(raw, "MAJ7"):
		return qMajor7
	}

	s := strings.ToLower(raw)
	s = strings.NewReplacer("Δ", "maj7", "°", "dim", "ø", "m7b5",
		"+", "aug", "−", "-").Replace(s)

	switch {
	case s == "":
		return qMajor
	case strings.HasPrefix(s, "maj7"), strings.HasPrefix(s, "ma7"):
		return qMajor7
	case s == "maj", s == "major":
		return qMajor
	case strings.HasPrefix(s, "m7b5"):
		// Half-diminished. The nearest shape a hand holds is the minor
		// seventh; the flattened fifth is the player's to find.
		return qMinor7
	case strings.HasPrefix(s, "m6"), strings.HasPrefix(s, "min6"):
		return qMinorSixth
	case strings.HasPrefix(s, "m7"), strings.HasPrefix(s, "min7"),
		strings.HasPrefix(s, "-7"):
		return qMinor7
	case strings.HasPrefix(s, "m9"), strings.HasPrefix(s, "m11"),
		strings.HasPrefix(s, "m13"):
		return qMinor7 // built on the minor seventh
	case s == "m", strings.HasPrefix(s, "mi"), strings.HasPrefix(s, "-"):
		return qMinor
	case strings.HasPrefix(s, "sus2"):
		return qSus2
	case strings.HasPrefix(s, "sus"):
		return qSus4
	case s == "5":
		return qFifth
	case strings.HasPrefix(s, "6"):
		return qSixth
	case strings.HasPrefix(s, "dim"), s == "o":
		return qDiminished
	case strings.HasPrefix(s, "aug"):
		return qAugmented
	case strings.HasPrefix(s, "7"), strings.HasPrefix(s, "9"),
		strings.HasPrefix(s, "11"), strings.HasPrefix(s, "13"):
		return qDominant7 // the ninths and thirteenths sit on the same shape
	}
	return qUnknown
}

// openChords are the first-position shapes, which are not regular enough to
// be worked out: C is x32010 and nothing about the shape says so.
var openChords = map[string]string{
	"C": "x32010", "Cmaj7": "x32000", "C7": "x32310", "C6": "x32210",
	"A": "x02220", "Am": "x02210", "A7": "x02020", "Am7": "x02010",
	"Amaj7": "x02120", "Asus4": "x02230", "Asus2": "x02200", "A6": "x02222",
	"G": "320003", "G7": "320001", "Gmaj7": "320002", "G6": "320000",
	"E": "022100", "Em": "022000", "E7": "020100", "Em7": "020000",
	"Emaj7": "021100", "Esus4": "022200", "E6": "022120",
	"D": "xx0232", "Dm": "xx0231", "D7": "xx0212", "Dm7": "xx0211",
	"Dmaj7": "xx0222", "Dsus4": "xx0233", "Dsus2": "xx0230", "D6": "xx0202",
	"F": "133211", "Fmaj7": "xx3210", "F7": "131211",
	"B7": "x21202", "Bm": "x24432", "B": "x24442",
}

// shapes are the movable ones, as frets above the root. -1 is a string that
// is not played, and the root sits on the string the shape is named for.
var sixthStringShapes = map[chordQuality][]int{
	qMajor:      {0, 2, 2, 1, 0, 0},
	qMinor:      {0, 2, 2, 0, 0, 0},
	qDominant7:  {0, 2, 0, 1, 0, 0},
	qMinor7:     {0, 2, 0, 0, 0, 0},
	qMajor7:     {0, 2, 1, 1, 0, 0},
	qSixth:      {0, 2, 2, 1, 2, 0},
	qMinorSixth: {0, 2, 2, 0, 2, 0},
	qSus4:       {0, 2, 2, 2, 0, 0},
	qSus2:       {0, 2, 4, 4, -1, -1},
	qFifth:      {0, 2, 2, -1, -1, -1},
	qAugmented:  {0, 3, 2, 1, 1, 0},
	qDiminished: {0, 1, 2, 0, -1, -1},
}

var fifthStringShapes = map[chordQuality][]int{
	qMajor:      {-1, 0, 2, 2, 2, 0},
	qMinor:      {-1, 0, 2, 2, 1, 0},
	qDominant7:  {-1, 0, 2, 0, 2, 0},
	qMinor7:     {-1, 0, 2, 0, 1, 0},
	qMajor7:     {-1, 0, 2, 1, 2, 0},
	qSixth:      {-1, 0, 2, 2, 2, 2},
	qMinorSixth: {-1, 0, 2, 2, 1, 2},
	qSus4:       {-1, 0, 2, 2, 3, 0},
	qSus2:       {-1, 0, 2, 2, 0, 0},
	qFifth:      {-1, 0, 2, -1, -1, -1},
	qAugmented:  {-1, 0, 3, 2, 2, 1},
	qDiminished: {-1, 0, 1, 2, 1, -1},
}

// pitchClass is how many semitones a note letter is above C.
var pitchClass = map[string]int{
	"C": 0, "C#": 1, "Db": 1, "D": 2, "D#": 3, "Eb": 3, "E": 4, "Fb": 4,
	"E#": 5, "F": 5, "F#": 6, "Gb": 6, "G": 7, "G#": 8, "Ab": 8, "A": 9,
	"A#": 10, "Bb": 10, "B": 11, "Cb": 11, "B#": 0,
}

// guitarFingering answers with where the hand goes, or "" for a chord it
// has nothing to say about.
func guitarFingering(root, quality string) string {
	root = strings.NewReplacer("♯", "#", "♭", "b").Replace(strings.TrimSpace(root))
	if root == "" {
		return ""
	}
	q := qualityOf(quality)
	if q == qUnknown {
		return ""
	}

	// The open shape, where there is one: it is what a guitarist plays in
	// first position and it is easier than any barre.
	if frets, ok := openChords[root+canonicalQuality(q)]; ok {
		return frets
	}

	pc, ok := pitchClass[root]
	if !ok {
		return ""
	}
	// How far up the neck the root is on each of the two strings the shapes
	// are built on: the sixth is E and the fifth is A.
	sixth := (pc - pitchClass["E"] + 12) % 12
	fifth := (pc - pitchClass["A"] + 12) % 12

	// The lower hand position, and the sixth string where they are equal:
	// a barre low on the neck is easier to play and easier to read.
	shape, at := sixthStringShapes[q], sixth
	if fifth < sixth || (sixth == 0 && fifth > 0) {
		shape, at = fifthStringShapes[q], fifth
	}
	if shape == nil {
		return ""
	}
	// A shape at the nut is an open chord, and the ones worth having are in
	// the table above; sliding it to the twelfth fret instead keeps it
	// playable rather than writing a chord nobody holds that way.
	if at == 0 {
		at = 12
	}

	// Spaces between the frets only when one of them is two digits: the
	// spelling is read a string at a time, and "10" would otherwise be a
	// one and a nought on two strings.
	wide := false
	for _, f := range shape {
		if f >= 0 && f+at > 9 {
			wide = true
		}
	}
	out := make([]byte, 0, 12)
	for i, f := range shape {
		if wide && i > 0 {
			out = append(out, ' ')
		}
		if f < 0 {
			out = append(out, 'x')
			continue
		}
		out = append(out, []byte(strconv.Itoa(f+at))...)
	}
	return string(out)
}

// canonicalQuality is how the open table spells each sort of chord.
func canonicalQuality(q chordQuality) string {
	switch q {
	case qMinor:
		return "m"
	case qDominant7:
		return "7"
	case qMinor7:
		return "m7"
	case qMajor7:
		return "maj7"
	case qSixth:
		return "6"
	case qMinorSixth:
		return "m6"
	case qSus4:
		return "sus4"
	case qSus2:
		return "sus2"
	case qFifth:
		return "5"
	case qDiminished:
		return "dim"
	case qAugmented:
		return "aug"
	}
	return ""
}

// handleChordFingering answers with where the hand goes for a chord name.
//
// It is what lets the picker fill the fingering in as the name is typed:
// somebody who has already said "Dm" should not have to look up what they
// just said. The answer can be typed over — it is one way of playing the
// chord, not the only one.
func (a *App) handleChordFingering(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(r.URL.Query().Get("chord"))
	if text == "" {
		writeJSON(w, map[string]any{"frets": "", "found": false})
		return
	}
	sym := parseChordSymbol(text)
	frets := guitarFingering(sym.Root, sym.Quality)
	writeJSON(w, map[string]any{
		"chord": sym.String(), "frets": frets, "found": frets != "",
	})
}
