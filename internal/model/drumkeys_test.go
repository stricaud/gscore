/*
 * drumkeys_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "testing"

/*
 * Every drum General MIDI names has somewhere to be written.
 *
 * Two of them did not. A Guitar Pro file of Stairway came in with a hundred
 * and five strokes "on instruments the standard kit has no staff position
 * for": four on key 48, the hi-mid tom, and eighty-seven on key 59, the
 * second ride. Both are ordinary General MIDI drums and both were missing
 * from the kit, so every one of those strokes was written on the middle
 * line — which on a drum staff does not mean "unknown", it means the snare.
 */

func TestEveryGeneralMidiDrumHasAPosition(t *testing.T) {
	// The whole range, so that the next one missing is found here and not
	// in a warning on somebody's score.
	//
	// The large kit, because that is the one that offers everything: the
	// basic and standard kits are deliberate selections, and a key only the
	// large kit places is still a key the reader can place.
	large := DrumKitByID("large")
	placed := map[int]bool{}
	for _, n := range large.Notes {
		placed[n.Key] = true
	}
	var missing []int
	for k := PercussionLowKey; k <= PercussionHighKey; k++ {
		if !placed[k] {
			missing = append(missing, k)
		}
	}
	// Not an empty list — General MIDI names eighty-one instruments and a
	// drum staff is five lines — but the ones a kit is expected to carry
	// are below, and this reports the rest so the gap is visible.
	t.Logf("%d of General MIDI's %d percussion keys have no position",
		len(missing), PercussionHighKey-PercussionLowKey+1)
}

func TestTheDrumsAKitMustCarry(t *testing.T) {
	// The drum set proper: everything a kit in front of a drummer has.
	for _, want := range []struct {
		key  int
		name string
	}{
		{35, "Bass drum"}, {36, "Bass drum"}, {37, "Side stick"},
		{38, "Snare drum"}, {41, "Low floor tom"}, {42, "Hi-hat closed"},
		{43, "High floor tom"}, {44, "Hi-hat with foot"}, {45, "Low tom"},
		{46, "Hi-hat open"}, {47, "Low-mid tom"}, {48, "Hi-mid tom"},
		{49, "Crash cymbal"}, {50, "High tom"}, {51, "Ride cymbal"},
		{52, "Chinese cymbal"}, {53, "Ride bell"}, {55, "Splash cymbal"},
		{56, "Cowbell"}, {59, "Ride cymbal 2"},
	} {
		n, ok := PercussionByKey(want.key)
		if !ok {
			t.Errorf("key %d (%s) has no staff position", want.key, want.name)
			continue
		}
		if n.Name != want.name {
			t.Errorf("key %d is called %q, want %q", want.key, n.Name, want.name)
		}
	}
}

func TestTheTwoRidesShareTheRideLine(t *testing.T) {
	// A drummer with two rides reads them both on the ride line, which is
	// where the first one already was.
	one, _ := PercussionByKey(51)
	two, _ := PercussionByKey(59)
	if one.Step != two.Step {
		t.Errorf("the rides are on steps %d and %d", one.Step, two.Step)
	}
	if two.Head != one.Head {
		t.Errorf("the second ride is drawn %v, the first %v", two.Head, one.Head)
	}
}

func TestTheHiMidTomIsNotOnTheHighTomsLine(t *testing.T) {
	// Six toms, five places. The hi-mid shares with the low-mid so that
	// the highest tom keeps a line of its own: the two ends of a tom fill
	// are what a reader most needs to tell apart.
	mid, _ := PercussionByKey(48)
	low, _ := PercussionByKey(47)
	high, _ := PercussionByKey(50)
	if mid.Step != low.Step {
		t.Errorf("the hi-mid tom is at step %d, the low-mid at %d",
			mid.Step, low.Step)
	}
	if mid.Step == high.Step {
		t.Errorf("the hi-mid tom shares the high tom's line, step %d", mid.Step)
	}
}
