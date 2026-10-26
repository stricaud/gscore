/*
 * harmonic_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package analysis

import (
	"math"
	"testing"
)

func note(midi int) Note { return Note{MIDI: midi, Velocity: 100} }

func TestConcertAIsFourFortyAndTheOctaveIsDouble(t *testing.T) {
	if got := Frequency(69); math.Abs(got-440) > 1e-9 {
		t.Errorf("A4 is %.4f Hz", got)
	}
	if got := Frequency(81); math.Abs(got-880) > 1e-9 {
		t.Errorf("the octave above it is %.4f Hz", got)
	}
	if got := Cents(440, 880); math.Abs(got-1200) > 1e-9 {
		t.Errorf("an octave is %.2f cents", got)
	}
}

// The order the ear puts intervals in, which is the whole point of the
// measure: if a fifth does not come out smoother than a third, and a third
// smoother than a second, nothing built on it means anything.
func TestTheIntervalsComeOutInTheRightOrder(t *testing.T) {
	r := func(midi ...int) float64 {
		var ns []Note
		for _, m := range midi {
			ns = append(ns, note(m))
		}
		return Of(ns, DefaultOptions()).Roughness
	}

	octave := r(60, 72)
	fifth := r(60, 67)
	fourth := r(60, 65)
	third := r(60, 64)
	second := r(60, 62)
	semitone := r(60, 61)

	for _, step := range []struct {
		name     string
		lo, hi   float64
		loN, hiN string
	}{
		{"octave under fifth", octave, fifth, "octave", "fifth"},
		{"fifth under fourth", fifth, fourth, "fifth", "fourth"},
		{"fourth under third", fourth, third, "fourth", "major third"},
		{"third under second", third, second, "major third", "major second"},
		{"second under semitone", second, semitone, "major second", "semitone"},
	} {
		if step.lo >= step.hi {
			t.Errorf("the %s measures %.3f and the %s %.3f: the wrong way round",
				step.loN, step.lo, step.hiN, step.hi)
		}
	}
}

// The number is in semitones, so a semitone is one.
func TestASemitoneIsTheUnit(t *testing.T) {
	got := Of([]Note{note(60), note(61)}, DefaultOptions()).Roughness
	if math.Abs(got-1) > 1e-9 {
		t.Errorf("a semitone measures %.4f, and it is the yardstick", got)
	}
}

// How hard it is played does not change what it is.
func TestLoudnessDoesNotChangeTheVerdict(t *testing.T) {
	quiet := Of([]Note{{MIDI: 60, Velocity: 20}, {MIDI: 61, Velocity: 20}}, DefaultOptions())
	loud := Of([]Note{{MIDI: 60, Velocity: 127}, {MIDI: 61, Velocity: 127}}, DefaultOptions())
	if math.Abs(quiet.Roughness-loud.Roughness) > 1e-9 {
		t.Errorf("quietly it is %.3f and loudly %.3f; a quiet minor second is still a minor second",
			quiet.Roughness, loud.Roughness)
	}
}

// A chord is not rougher for having more notes in it.
//
// The sum runs over pairs of partials, so a major triad — three intervals,
// every one of them consonant — came out rougher than a minor second simply
// by having three of them.
func TestAChordIsNotRoughForBeingBig(t *testing.T) {
	triad := Of([]Note{note(60), note(64), note(67)}, DefaultOptions())
	semitone := Of([]Note{note(60), note(61)}, DefaultOptions())
	if triad.Roughness >= semitone.Roughness {
		t.Errorf("a major triad measures %.3f and a minor second %.3f",
			triad.Roughness, semitone.Roughness)
	}
	// And a triad spread over two octaves is smoother than a close one.
	spread := Of([]Note{note(48), note(64), note(79)}, DefaultOptions())
	if spread.Roughness >= triad.Roughness {
		t.Errorf("spread out it measures %.3f, closed up %.3f",
			spread.Roughness, triad.Roughness)
	}
}

// A fifth is consonant because the partials meet: the third harmonic of the
// lower note is the second of the upper.
func TestWhyAFifthIsConsonant(t *testing.T) {
	h := Of([]Note{note(60), note(67)}, DefaultOptions())
	if len(h.Coincident) == 0 {
		t.Fatal("nothing coincides in a perfect fifth")
	}
	first := h.Coincident[0]
	if first.NA != 3 || first.NB != 2 {
		t.Errorf("the nearest meeting is harmonic %d against %d, want the third against the second",
			first.NA, first.NB)
	}
	// Equal temperament is two cents narrow of a 3:2, which is why a fifth
	// is nearly but not quite beatless.
	if math.Abs(first.Cents) > 3 {
		t.Errorf("they are %.2f cents apart", first.Cents)
	}
	if first.Beats > 2 {
		t.Errorf("they beat %.2f times a second, which would be audible", first.Beats)
	}
}

// And a minor second is rough because nothing meets.
func TestWhyASecondIsRough(t *testing.T) {
	h := Of([]Note{note(60), note(61)}, DefaultOptions())
	if len(h.Coincident) != 0 {
		t.Errorf("%d partials coincide in a minor second", len(h.Coincident))
	}
}

// What equal temperament did to each interval, which is the other half of
// the question: a tempered major third is sharp of the ratio it is named
// after, and that is why a triad beats on a piano and does not in a choir.
func TestHowFarTemperamentBendsTheIntervals(t *testing.T) {
	h := Of([]Note{note(60), note(64), note(67)}, DefaultOptions())

	by := map[string]Interval{}
	for _, iv := range h.Intervals {
		by[iv.Name] = iv
	}

	third, ok := by["major third"]
	if !ok {
		t.Fatalf("no major third found in %v", h.Intervals)
	}
	if third.Ratio != "5:4" {
		t.Errorf("the major third is named after %s", third.Ratio)
	}
	// 400 cents against 386.31: the tempered third is about fourteen cents
	// sharp.
	if math.Abs(third.Cents-13.69) > 0.1 {
		t.Errorf("the tempered major third is %.2f cents from just, want about +13.69", third.Cents)
	}

	fifth := by["perfect fifth"]
	if math.Abs(fifth.Cents+1.96) > 0.1 {
		t.Errorf("the tempered fifth is %.2f cents from just, want about -1.96", fifth.Cents)
	}
}

// An interval wider than an octave is still named, and its ratio carried up.
func TestIntervalsWiderThanAnOctave(t *testing.T) {
	h := Of([]Note{note(60), note(79)}, DefaultOptions()) // an octave and a fifth
	if len(h.Intervals) != 1 {
		t.Fatalf("%d intervals", len(h.Intervals))
	}
	iv := h.Intervals[0]
	if iv.Semitones != 19 || iv.Name != "perfect fifth" {
		t.Errorf("nineteen semitones came out as %q over %d", iv.Name, iv.Semitones)
	}
	if math.Abs(iv.Cents+1.96) > 0.1 {
		t.Errorf("a twelfth is %.2f cents from just, want the fifth's -1.96", iv.Cents)
	}
}

// Nothing sounding, and one note sounding, are not errors.
func TestSilenceAndASingleNote(t *testing.T) {
	if got := Of(nil, DefaultOptions()); got.Roughness != 0 || len(got.Partials) != 0 {
		t.Errorf("silence measured %.3f with %d partials", got.Roughness, len(got.Partials))
	}
	one := Of([]Note{note(60)}, DefaultOptions())
	if one.Roughness != 0 {
		t.Errorf("one note measured %.3f", one.Roughness)
	}
	if len(one.Partials) != DefaultOptions().Partials {
		t.Errorf("one note has %d partials", len(one.Partials))
	}
}
