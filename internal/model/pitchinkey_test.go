/*
 * pitchinkey_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Spelling a key that was pressed.
 *
 * A MIDI keyboard gives a number and nothing else — the same number for D
 * sharp and E flat — so something has to choose. The key signature is the
 * only thing in the question with an opinion, and this is it being asked.
 */

package model

import "testing"

func key(fifths int) KeySignature { return KeySignature{Fifths: fifths} }

// The same black key, spelled by where the music is.
func TestTheSameKeyIsSpelledByTheKeyItIsIn(t *testing.T) {
	const blackAboveD = 63 // D sharp or E flat, just above middle C
	for _, tc := range []struct {
		fifths int
		want   string
	}{
		{3, "D#4"},  // A major: the key leans on sharps
		{-3, "Eb4"}, // E flat major: on flats
		{0, "D#4"},  // C major: no lean, so the sharp a player means
	} {
		if got := PitchInKey(blackAboveD, key(tc.fifths)).String(); got != tc.want {
			t.Errorf("%+d sharps: %d came out as %s, want %s",
				tc.fifths, blackAboveD, got, tc.want)
		}
	}
}

// A note the signature already contains is written plain: in two sharps,
// 66 is the key's own F sharp and not a G flat.
func TestANoteTheKeyContainsIsSpelledAsThatNote(t *testing.T) {
	for _, tc := range []struct {
		midi, fifths int
		want         string
	}{
		{66, 2, "F#4"},  // D major
		{70, -2, "Bb4"}, // B flat major
		{61, 7, "C#4"},  // C sharp major: every note is sharpened
		{59, -7, "Cb4"}, // C flat major: and here they are all flattened
		{60, 0, "C4"},   // middle C, in C
	} {
		if got := PitchInKey(tc.midi, key(tc.fifths)).String(); got != tc.want {
			t.Errorf("%d in %+d came out as %s, want %s",
				tc.midi, tc.fifths, got, tc.want)
		}
	}
}

// Whatever it is spelled as, it has to sound what was played. A spelling
// that comes back as another note is worse than an ugly one.
func TestEverySpellingSoundsTheNoteThatWasPlayed(t *testing.T) {
	for fifths := -7; fifths <= 7; fifths++ {
		for midi := 21; midi <= 108; midi++ { // the piano
			p := PitchInKey(midi, key(fifths))
			if p.MIDI() != midi {
				t.Errorf("%d in %+d came back as %s, which sounds %d",
					midi, fifths, p, p.MIDI())
			}
		}
	}
}

// The octave is worked out from the number and not from midi/12, because
// the two disagree at the ends: B sharp and C flat sit either side of the
// line between one octave and the next.
func TestTheOctaveHoldsAcrossTheLineBetweenOctaves(t *testing.T) {
	// C flat major: the B under middle C is written C flat, and it keeps
	// the octave of the letter C rather than the octave of the sound.
	if got := PitchInKey(59, key(-7)).String(); got != "Cb4" {
		t.Errorf("59 in seven flats is %s, want Cb4", got)
	}
	// C sharp major: middle C is written B sharp, in the octave below —
	// the same line crossed the other way.
	if got := PitchInKey(60, key(7)).String(); got != "B#3" {
		t.Errorf("60 in seven sharps is %s, want B#3", got)
	}
}
