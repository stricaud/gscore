/*
 * spacing_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How low a chord may be spaced before it turns to mud.
 *
 * The same physics as the roughness panel, asked the other way round. Low
 * notes have their partials packed close together in frequency, so an
 * interval that is clear in the middle of the piano beats against itself an
 * octave and a half down: a major third on middle C is a chord, and the same
 * third two octaves lower is a rumble with no third in it.
 *
 * Offered as advice and never as a fault. A composer writing a low third in
 * the bassoons may want exactly that rumble.
 */

package analysis

import "testing"

func pitchNumber(midi int) string { return itoa(midi) }

// chord sounds a set of pitches together for one beat in one bar.
func chord(barAt int, midis ...int) []Sound {
	var out []Sound
	for i, m := range midis {
		out = append(out, Sound{
			Line: i, Bar: barAt, At: float64(barAt) * 0.25,
			Until: float64(barAt)*0.25 + 0.25, MIDI: m, Ref: barAt*10 + i + 1,
		})
	}
	return out
}

// The narrower the interval, the higher it has to be written. That shape is
// what the books agree about even where they differ by a semitone.
func TestTheNarrowerTheIntervalTheHigherItMustSit(t *testing.T) {
	second, _ := LowLimit(2)
	third, _ := LowLimit(4)
	fifth, _ := LowLimit(7)
	octave, _ := LowLimit(12)
	if !(second > third && third > fifth && fifth > octave) {
		t.Errorf("the limits run %d %d %d %d, and they should fall as the "+
			"interval widens", second, third, fifth, octave)
	}
}

// Wider than an octave there is no limit: the two notes are far enough apart
// for their partials to stay out of each other's way, and a tenth low in the
// bass is ordinary writing.
func TestNothingWiderThanAnOctaveHasALimit(t *testing.T) {
	for _, gap := range []int{13, 15, 19, 24} {
		if _, ok := LowLimit(gap); ok {
			t.Errorf("an interval of %d semitones was given a limit", gap)
		}
	}
}

func TestALowThirdIsReported(t *testing.T) {
	// A major third on C2, well under the limit for one.
	got := Spacing(chord(0, 36, 40), pitchNumber)
	if len(got) != 1 {
		t.Fatalf("%d findings for a major third on C2: %+v", len(got), got)
	}
	if got[0].Semitones != 4 || got[0].Low != 36 {
		t.Errorf("it is about %+v", got[0])
	}
	if got[0].Below <= 0 {
		t.Errorf("it is said to be %d below the limit", got[0].Below)
	}
	if got[0].Ref == 0 {
		t.Error("it points at no notehead")
	}
}

// The same third in the middle of the piano is a chord and nothing is said.
func TestTheSameThirdHigherUpIsNothing(t *testing.T) {
	if got := Spacing(chord(0, 60, 64), pitchNumber); len(got) != 0 {
		t.Errorf("a major third on middle C was reported: %+v", got)
	}
}

// An octave goes almost as low as the piano does, which is why basses and
// cellos double at one.
func TestAnOctaveLowDownIsFine(t *testing.T) {
	if got := Spacing(chord(0, 36, 48), pitchNumber); len(got) != 0 {
		t.Errorf("an octave on C2 was reported: %+v", got)
	}
}

// Only the bottom of the chord. What is stacked above the lowest two notes
// is a different question, and the harmonics panel answers it.
func TestOnlyTheBottomTwoNotesAreJudged(t *testing.T) {
	// A wide, clear bottom with a close second above it.
	if got := Spacing(chord(0, 36, 48, 49), pitchNumber); len(got) != 0 {
		t.Errorf("a close interval above a clear bottom was reported: %+v", got)
	}
}

// A passage spaced that way for twenty bars is one thing to know, not twenty.
func TestARunOfItIsOneFinding(t *testing.T) {
	var sounds []Sound
	for i := 0; i < 20; i++ {
		sounds = append(sounds, chord(i, 36, 40)...)
	}
	got := Spacing(sounds, pitchNumber)
	if len(got) != 1 {
		t.Fatalf("%d findings for twenty bars of the same spacing", len(got))
	}
	if got[0].FromBar != 0 || got[0].ToBar != 19 || got[0].Times != 20 {
		t.Errorf("the run came out as %+v", got[0])
	}
}

// A different low interval is a different thing to be told about.
func TestTwoDifferentLowIntervalsAreTwoFindings(t *testing.T) {
	sounds := append(chord(0, 36, 40), chord(1, 38, 41)...)
	if got := Spacing(sounds, pitchNumber); len(got) != 2 {
		t.Errorf("%d findings for two different low intervals: %+v", len(got), got)
	}
}

func TestOneNoteIsNotASpacing(t *testing.T) {
	if got := Spacing(chord(0, 36), pitchNumber); len(got) != 0 {
		t.Errorf("one note was given a spacing: %+v", got)
	}
	if got := Spacing(nil, pitchNumber); got != nil {
		t.Errorf("silence was: %+v", got)
	}
}

// The sentence has to name the interval and both ends of the advice, or it
// is a warning nobody can act on.
func TestTheSentenceSaysWhatAndHowLow(t *testing.T) {
	got := Spacing(chord(0, 36, 40), pitchNumber)
	if len(got) != 1 {
		t.Fatal("nothing to read")
	}
	text := got[0].Text
	for _, want := range []string{"major third", "36", "46"} {
		if !contains(text, want) {
			t.Errorf("%q does not say %q", text, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
