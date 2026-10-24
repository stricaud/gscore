/*
 * synth_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package synth

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/midi"
)

func notesOf(parts ...int) []midi.Note {
	var out []midi.Note
	for _, p := range parts {
		out = append(out, midi.Note{Part: p, MIDI: 60})
	}
	return out
}

// Every staff gets a channel of its own, because two staves on one channel
// share a program: set the second one's instrument and the first one changes
// too.
func TestEachStaffPlaysOnItsOwnChannel(t *testing.T) {
	got := channelsFor(notesOf(0, 1, 2))
	seen := map[int]bool{}
	for part, ch := range got {
		if seen[ch] {
			t.Errorf("staff %d shares channel %d", part, ch)
		}
		seen[ch] = true
	}
	if len(got) != 3 {
		t.Fatalf("%d staves got channels, want 3", len(got))
	}
}

// The tenth channel is the drum channel whatever anyone wants, so a melodic
// part must never be given it: a flute on channel 10 plays a drum kit.
func TestThePercussionChannelIsLeftForPercussion(t *testing.T) {
	var notes []midi.Note
	for p := 0; p < 12; p++ {
		notes = append(notes, midi.Note{Part: p, MIDI: 60})
	}
	got := channelsFor(notes)
	for part, ch := range got {
		if ch == 9 {
			t.Errorf("staff %d, which is not percussion, was given channel 10", part)
		}
	}

	// And a drum part is given it, wherever it comes in the score.
	notes = append(notes, midi.Note{Part: 99, MIDI: 38, Drum: 38})
	if got := channelsFor(notes); got[99] != 9 {
		t.Errorf("the drum staff went to channel %d, want 9", got[99]+1)
	}
}

// Mixing down is one number over the whole recording, so the quiet passages
// stay exactly as far below the loud ones as they were written.
func TestBringingTheLevelUpKeepsTheDynamics(t *testing.T) {
	left := make([]float64, 200)
	right := make([]float64, 200)
	for i := range left {
		// A loud half and a half a quarter of it.
		v := 0.2
		if i >= 100 {
			v = 0.05
		}
		left[i], right[i] = v, v
	}
	out := mixdown(left, right, 1, true)

	loud, quiet := float64(out[0]), float64(out[2*150])
	if ratio := loud / quiet; math.Abs(ratio-4) > 0.01 {
		t.Errorf("the loud passage came out %.3f times the quiet one, was 4", ratio)
	}
	if peak := loud / 32767; math.Abs(peak-fullScale) > 0.001 {
		t.Errorf("the loudest moment is at %.3f of full scale, want %.3f",
			peak, fullScale)
	}
}

// Left alone, a mix that would clip is brought down and nothing else is
// touched: that is the one thing normalising off still has to do.
func TestAMixThatWouldClipIsBroughtDown(t *testing.T) {
	left := []float64{1.6, -1.6, 0.4}
	right := []float64{1.6, -1.6, 0.4}
	out := mixdown(left, right, 1, false)
	for i, s := range out {
		if s == -32768 || s == 32767 {
			continue
		}
		if math.Abs(float64(s)) > 32767 {
			t.Fatalf("sample %d is %d", i, s)
		}
	}
	if float64(out[0])/32767 > 0.995 {
		t.Errorf("the peak was left at %.4f of full scale", float64(out[0])/32767)
	}
}

func TestSilenceDoesNotDivideByItself(t *testing.T) {
	out := mixdown(make([]float64, 32), make([]float64, 32), 1, true)
	for _, s := range out {
		if s != 0 {
			t.Fatalf("silence came out as %d", s)
		}
	}
}
