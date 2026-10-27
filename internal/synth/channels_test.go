/*
 * channels_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A channel each.
 *
 * MIDI has sixteen channels and a symphony has fifty-two staves. They used
 * to be handed out round the sixteen and begin again, so Beethoven's ninth
 * put its first violins on a trombone's channel: a channel carries one
 * instrument, so whichever of the two set its program last decided what
 * both of them sounded like. The violins came out as a trombone.
 *
 * Sharing costs more than the sound, too. A note-off is a channel and a
 * key, so two staves on one channel playing the same note cut each other
 * short — and a score is full of staves playing the same note.
 */

package synth

import (
	"testing"

	"github.com/stricaud/gscore/internal/midi"
)

// orchestra is one note per staff, which is all this has to decide about.
func orchestra(staves int, drums ...int) []midi.Note {
	isDrum := map[int]bool{}
	for _, d := range drums {
		isDrum[d] = true
	}
	var out []midi.Note
	for p := 0; p < staves; p++ {
		n := midi.Note{Part: p, MIDI: 60, Start: float64(p), Dur: 1, Velocity: 80}
		if isDrum[p] {
			n.Drum = 38
		}
		out = append(out, n)
	}
	return out
}

// melodicPartsIn is how many staves in one share need a channel of their own.
func melodicPartsIn(notes []midi.Note) int {
	seen := map[int]bool{}
	for _, n := range notes {
		if n.Drum == 0 {
			seen[n.Part] = true
		}
	}
	return len(seen)
}

func TestEveryStaffGetsAChannelOfItsOwn(t *testing.T) {
	for _, staves := range []int{1, 15, 16, 52, 100} {
		notes := orchestra(staves)
		shares := splitForChannels(notes)

		kept := 0
		for i, share := range shares {
			kept += len(share)
			if n := melodicPartsIn(share); n > melodicChannels {
				t.Errorf("%d staves: share %d holds %d of them, and a synthesizer "+
					"can only keep %d apart", staves, i, n, melodicChannels)
			}
		}
		if kept != len(notes) {
			t.Errorf("%d staves: %d notes came back out of %d", staves, kept, len(notes))
		}

		// And no staff is split between two of them: its notes would be
		// played by two synthesizers and mixed twice.
		where := map[int]int{}
		for i, share := range shares {
			for _, n := range share {
				if at, ok := where[n.Part]; ok && at != i {
					t.Fatalf("%d staves: staff %d is in shares %d and %d",
						staves, n.Part, at, i)
				}
				where[n.Part] = i
			}
		}
	}
}

func TestAScoreThatFitsIsNotDividedAtAll(t *testing.T) {
	// Fifteen staves and one synthesizer: dividing a score that fits would
	// mean rendering and mixing more of them for nothing.
	notes := orchestra(15)
	if got := len(splitForChannels(notes)); got != 1 {
		t.Errorf("fifteen staves were played by %d synthesizers, want one", got)
	}
	if got := len(splitForChannels(orchestra(16))); got != 2 {
		t.Errorf("sixteen staves were played by %d synthesizers, want two", got)
	}
}

func TestDrumsDoNotUseUpAChannel(t *testing.T) {
	// Percussion is told apart by which key it is rather than by which
	// channel, so every drum staff shares the tenth and none of them costs
	// one of the fifteen.
	notes := orchestra(16, 0, 3)
	shares := splitForChannels(notes)
	if len(shares) != 1 {
		t.Fatalf("fourteen instruments and two drum kits were played by %d "+
			"synthesizers, want one", len(shares))
	}
	if n := melodicPartsIn(shares[0]); n != 14 {
		t.Errorf("%d staves needed a channel, want the 14 that are not drums", n)
	}
}

func TestTheChannelsInOneShareAreAllDifferent(t *testing.T) {
	// The whole point: inside a synthesizer, no two staves on one channel.
	for _, share := range splitForChannels(orchestra(52, 7)) {
		ch := channelsFor(share)
		seen := map[int]int{}
		for part, c := range ch {
			if c == 9 {
				continue // the drum channel is shared on purpose
			}
			if other, ok := seen[c]; ok {
				t.Errorf("staves %d and %d are both on channel %d", other, part, c)
			}
			seen[c] = part
		}
	}
}
