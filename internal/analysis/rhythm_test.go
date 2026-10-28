/*
 * rhythm_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the notes fall against the beat.
 *
 * A bar of four with the notes on one and three is the metre spoken plainly;
 * the same notes a quaver late are the same notes and a different piece.
 * Which is happening is a description and never a verdict — syncopation is
 * not a fault, it is most of what makes music move.
 */

package analysis

import "testing"

// fourFour builds notes in a bar of four-four from (start, length) pairs,
// both in whole notes.
func fourFour(barAt int, each ...[2]float64) []Beat {
	var out []Beat
	for i, p := range each {
		out = append(out, Beat{
			Bar: barAt, At: p[0], Held: p[1],
			Beats: 4, Unit: 0.25, Part: 0, Ref: barAt*100 + i + 1,
		})
	}
	return out
}

func one(ps []Pulse) Pulse { return ps[0] }

func TestNotesOnTheBeatAreCountedAsSuch(t *testing.T) {
	p := one(Pulses(fourFour(0,
		[2]float64{0, 0.25},    // the downbeat
		[2]float64{0.25, 0.25}, // beat two
		[2]float64{0.5, 0.25},  // beat three
		[2]float64{0.75, 0.25}, // beat four
	), []string{"Piano"}, 1))

	if p.Downbeats != 1 {
		t.Errorf("%d downbeats, want 1", p.Downbeats)
	}
	if p.OnBeat != 3 {
		t.Errorf("%d notes on other beats, want 3", p.OnBeat)
	}
	if p.OffBeat != 0 || p.Syncopated != 0 {
		t.Errorf("a bar squarely on the beat came out %d off and %d syncopated",
			p.OffBeat, p.Syncopated)
	}
}

// A note between beats that stops before the next one is off the beat and
// not syncopated: an offbeat quaver in a stream of quavers is the stream,
// not a pull against it.
func TestAnOffbeatNoteThatStopsInTimeIsNotSyncopation(t *testing.T) {
	p := one(Pulses(fourFour(0,
		[2]float64{0.125, 0.125},
		[2]float64{0.375, 0.125},
	), []string{"Piano"}, 1))

	if p.OffBeat != 2 {
		t.Errorf("%d off the beat, want 2", p.OffBeat)
	}
	if p.Syncopated != 0 {
		t.Errorf("%d syncopated, and neither note holds across a beat", p.Syncopated)
	}
}

// Starting off the beat and holding across the next one is syncopation
// proper, and it is what makes the metre audible by contradicting it.
func TestANoteHeldAcrossTheBeatIsSyncopation(t *testing.T) {
	p := one(Pulses(fourFour(0,
		[2]float64{0.125, 0.25}, // an eighth late, held a quarter: over beat two
	), []string{"Piano"}, 1))

	if p.Syncopated != 1 {
		t.Errorf("%d syncopated, want 1", p.Syncopated)
	}
	if p.Bars[0] != 1 {
		t.Errorf("the bar is marked %d, want 1", p.Bars[0])
	}
}

// Three-four and six-eight count their beats differently, and the answer
// follows what the bar says rather than assuming four of them.
func TestTheBarsOwnBeatIsWhatIsCountedAgainst(t *testing.T) {
	// Six-eight: six eighth notes, so an eighth is the unit.
	var beats []Beat
	for i := 0; i < 6; i++ {
		beats = append(beats, Beat{
			Bar: 0, At: float64(i) * 0.125, Held: 0.125,
			Beats: 6, Unit: 0.125, Ref: i + 1,
		})
	}
	p := one(Pulses(beats, []string{"Flute"}, 1))
	if p.OffBeat != 0 {
		t.Errorf("%d notes off the beat in a bar of six-eight written in eighths",
			p.OffBeat)
	}
}

// Thirds of a beat add up to a beat, and floating point does not quite
// agree. A triplet landing on the next beat is on it.
func TestATripletLandsOnTheBeatItLandsOn(t *testing.T) {
	third := 0.25 / 3
	p := one(Pulses(fourFour(0,
		[2]float64{0, third},
		[2]float64{third, third},
		[2]float64{third * 2, third},
		[2]float64{third * 3, 0.25}, // beat two, reached by adding thirds
	), []string{"Piano"}, 1))

	if p.OnBeat != 1 {
		t.Errorf("%d notes on a beat: the one reached by adding thirds was missed",
			p.OnBeat)
	}
}

func TestAStaffWithNothingOnItIsStillReported(t *testing.T) {
	got := Pulses(nil, []string{"Oboe", "Bassoon"}, 8)
	if len(got) != 2 {
		t.Fatalf("%d staves, want both", len(got))
	}
	if got[0].Notes != 0 || len(got[0].Bars) != 8 {
		t.Errorf("the empty staff came out as %+v", got[0])
	}
}

// Four very short notes in a row is a passage and has to be played as
// written; one is an ornament, and every ornament is played by leaning on
// it rather than by counting it.
func TestARunTooFastToPlayIsReported(t *testing.T) {
	var beats []Beat
	for i := 0; i < 6; i++ {
		beats = append(beats, Beat{
			Bar: 3, At: float64(i) * 0.03125, Held: 0.03125,
			Beats: 4, Unit: 0.25, Part: 0, Ref: i + 1,
		})
	}
	got := Rushed(beats, []string{"Tuba"},
		func(b Beat) float64 { return b.Held * 2 }, // a whole note is two seconds
		func(int) (float64, bool) { return 0.1, true }, nil)

	if len(got) != 1 {
		t.Fatalf("%d findings for six notes of 62 milliseconds: %+v", len(got), got)
	}
	if got[0].Notes != 6 || got[0].FromBar != 3 {
		t.Errorf("the run came out as %+v", got[0])
	}
	if got[0].Each >= got[0].Limit {
		t.Errorf("it is said to be %.3fs against a limit of %.3fs",
			got[0].Each, got[0].Limit)
	}
}

func TestOneShortNoteIsAnOrnamentAndIsLeftAlone(t *testing.T) {
	beats := []Beat{
		{Bar: 0, At: 0, Held: 0.01, Beats: 4, Unit: 0.25, Ref: 1},
		{Bar: 0, At: 0.01, Held: 0.49, Beats: 4, Unit: 0.25, Ref: 2},
	}
	got := Rushed(beats, []string{"Tuba"},
		func(b Beat) float64 { return b.Held * 2 },
		func(int) (float64, bool) { return 0.1, true }, nil)
	if len(got) != 0 {
		t.Errorf("a grace note was reported as unplayable: %+v", got)
	}
}

// An instrument gscore has no limit for is not judged, the same way one it
// has no range for is not.
func TestAnInstrumentWithNoLimitIsNotJudged(t *testing.T) {
	var beats []Beat
	for i := 0; i < 8; i++ {
		beats = append(beats, Beat{Bar: 0, At: float64(i) * 0.01, Held: 0.01, Ref: i + 1})
	}
	got := Rushed(beats, []string{"Theremin"},
		func(b Beat) float64 { return b.Held },
		func(int) (float64, bool) { return 0, false }, nil)
	if len(got) != 0 {
		t.Errorf("an instrument with no limit was judged: %+v", got)
	}
}
