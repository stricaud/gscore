/*
 * pairing_test.go
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
 * A tab staff paired with a staff of pitches.
 *
 * A Guitar Pro file is tablature and nothing else, and tablature says where
 * to put a finger rather than what the note is. The pitches are known all
 * the same — they are what the file plays — so the staff above is the same
 * music written the other way.
 *
 * What these ask is that it be the same music and not a second copy of it:
 * one set of measures, held by the upper staff, with the tab showing them.
 * Two copies is the failure the whole arrangement exists to avoid, because
 * two copies do not stay in step.
 */

// tabScore is one guitar written as tablature alone, with a bar of music.
func tabScore(names ...string) *Score {
	q := Duration{Num: 1, Den: 4}
	var parts []*Part
	for _, n := range names {
		p := &Part{
			Name: n, Clef: Treble8vb,
			Time: TimeSignature{Beats: 4, BeatValue: 4},
			Tab:  &Tablature{Frets: 22},
			Measures: []*Measure{{
				Voices: []*Voice{{Elements: []Element{
					&Note{Pitch: Pitch{Step: StepE, Octave: 3}, Dur: q},
				}}},
				RightBarline: BarSingle,
			}},
		}
		p.Tab.Strings = []Pitch{
			{Step: StepE, Octave: 4}, {Step: StepB, Octave: 3},
			{Step: StepG, Octave: 3}, {Step: StepD, Octave: 3},
			{Step: StepA, Octave: 2}, {Step: StepE, Octave: 2},
		}
		parts = append(parts, p)
	}
	return &Score{Parts: parts}
}

func TestPairingPutsTheMusicOnTheStaffAbove(t *testing.T) {
	s := tabScore("Guitar")
	if n := s.PairWithNotation(); n != 1 {
		t.Fatalf("added %d staves, wanted 1", n)
	}
	if len(s.Parts) != 2 {
		t.Fatalf("%d staves, wanted 2", len(s.Parts))
	}
	above, below := s.Parts[0], s.Parts[1]
	if above.Tab != nil {
		t.Error("the upper staff is written as tab; it should be pitches")
	}
	if below.Tab == nil {
		t.Error("the lower staff lost its tuning")
	}
	if len(above.Measures) != 1 {
		t.Errorf("the upper staff holds %d bars, wanted the music",
			len(above.Measures))
	}
	// The one thing the arrangement exists for: not a second copy.
	if len(below.Measures) != 0 {
		t.Errorf("the tab holds %d bars of its own; it should hold none",
			len(below.Measures))
	}
	if below.MirrorOf == nil || *below.MirrorOf != 0 {
		t.Errorf("the tab mirrors %v, wanted staff 0", below.MirrorOf)
	}
	if s.MusicOf(1) != above {
		t.Error("the tab does not read its music from the staff above it")
	}
	// Named once, over the pair.
	if above.Name != "Guitar" || below.Name != "" {
		t.Errorf("names are %q and %q, wanted %q and empty",
			above.Name, below.Name, "Guitar")
	}
	if len(s.Groups) != 1 {
		t.Fatalf("%d groups, wanted the two staves joined", len(s.Groups))
	}
	g := s.Groups[0]
	if g.First != 0 || g.Last != 1 || g.Symbol != GroupLine || !g.Barlines {
		t.Errorf("joined as %+v, wanted staves 0-1 by a rule with barlines", g)
	}
}

func TestPairingLeavesAnAlreadyPairedStaffAlone(t *testing.T) {
	s := tabScore("Guitar")
	s.PairWithNotation()
	was := len(s.Parts)
	if n := s.PairWithNotation(); n != 0 {
		t.Errorf("added %d staves the second time, wanted none", n)
	}
	if len(s.Parts) != was {
		t.Errorf("%d staves, wanted %d", len(s.Parts), was)
	}
}

func TestPairingSeveralStavesKeepsTheNumbersStraight(t *testing.T) {
	// The part that goes wrong by hand: every staff put in renumbers the
	// ones after it, and a mirror names its staff by number.
	s := tabScore("Rhythm", "Lead", "Bass")
	if n := s.PairWithNotation(); n != 3 {
		t.Fatalf("added %d staves, wanted 3", n)
	}
	if len(s.Parts) != 6 {
		t.Fatalf("%d staves, wanted 6", len(s.Parts))
	}
	for _, want := range []struct {
		at   int
		name string
	}{{0, "Rhythm"}, {2, "Lead"}, {4, "Bass"}} {
		above := s.Parts[want.at]
		if above.Name != want.name || above.Tab != nil {
			t.Errorf("staff %d is %q tab=%v, wanted %q on pitches",
				want.at, above.Name, above.Tab != nil, want.name)
		}
		below := s.Parts[want.at+1]
		if below.MirrorOf == nil || *below.MirrorOf != want.at {
			t.Errorf("staff %d mirrors %v, wanted %d",
				want.at+1, below.MirrorOf, want.at)
		}
		if s.MusicOf(want.at+1) != above {
			t.Errorf("staff %d reads its music from the wrong staff", want.at+1)
		}
	}
	if len(s.Groups) != 3 {
		t.Fatalf("%d groups, wanted 3", len(s.Groups))
	}
	for _, g := range s.Groups {
		if g.Last != g.First+1 || g.First%2 != 0 {
			t.Errorf("a group joins staves %d-%d, which is not a pair",
				g.First, g.Last)
		}
	}
}

func TestPairingLeavesAStaffOfPitchesAlone(t *testing.T) {
	s := tabScore("Guitar")
	s.Parts[0].Tab = nil
	if n := s.PairWithNotation(); n != 0 {
		t.Errorf("added %d staves to a score with no tab in it, wanted none", n)
	}
}
