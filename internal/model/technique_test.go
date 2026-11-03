/*
 * technique_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How a fretted instrument plays the note.
 */

package model

import "testing"

// Every mark can be written down and read back. A name that does not come
// back is a mark that is lost the first time the score is saved, and
// nothing would say so: the file would simply open with one fewer
// hammer-on than it was written with.
func TestEveryTechniqueSurvivesItsName(t *testing.T) {
	for _, want := range Techniques() {
		s := want.String()
		if s == "unknown" {
			t.Errorf("technique %d has no name", int(want))
			continue
		}
		got, ok := TechniqueFromName(s)
		if !ok || got != want {
			t.Errorf("%q came back as %v (ok=%v)", s, got, ok)
		}
	}
}

func TestANameNobodyUsesIsRefused(t *testing.T) {
	if _, ok := TechniqueFromName("whammy-gargle"); ok {
		t.Error("it accepted a mark that does not exist")
	}
}

// The groups are what the editor uses to decide which marks displace which,
// so a mark that falls into neither group or into both would be applied
// wrongly and only on the page would anybody find out.
func TestTheGroupsCoverWhatTheySay(t *testing.T) {
	links := map[Technique]bool{}
	for _, t := range LinkTechniques() {
		links[t] = true
	}
	bends := map[Technique]bool{}
	for _, t := range BendTechniques() {
		bends[t] = true
	}
	for _, tech := range Techniques() {
		if tech.Links() != links[tech] {
			t.Errorf("%v: Links says %v, the list says %v", tech, tech.Links(), links[tech])
		}
		if tech.Bends() != bends[tech] {
			t.Errorf("%v: Bends says %v, the list says %v", tech, tech.Bends(), bends[tech])
		}
		if tech.Links() && tech.Bends() {
			t.Errorf("%v is in both groups", tech)
		}
	}
}

// A hammer-on reaches to the next note and a palm mute does not, which is
// the whole of the difference between drawing an arc and writing a word.
func TestOnlyTheFourThatReachOnwardsLink(t *testing.T) {
	for _, tc := range []struct {
		t    Technique
		want bool
	}{
		{HammerOn, true}, {PullOff, true},
		{SlideUp, true}, {SlideDown, true},
		{BendFull, false}, {PalmMute, false}, {Tap, false},
	} {
		if got := tc.t.Links(); got != tc.want {
			t.Errorf("%v links: %v, want %v", tc.t, got, tc.want)
		}
	}
}

// The ones drawn rather than written carry no label, and the ones written
// carry one. A word missing is a mark nobody can read; a word on a mark
// that is a stroke is a word printed on top of the stroke.
func TestTheMarksThatAreWrittenHaveWordsAndTheRestDoNot(t *testing.T) {
	for _, tc := range []struct {
		t    Technique
		want string
	}{
		{HammerOn, "H"}, {PullOff, "P"},
		{SlideUp, ""}, {SlideDown, ""}, {Vibrato, ""}, {WideVibrato, ""},
		{BendFull, "full"}, {BendHalf, "1/2"},
		{PalmMute, "P.M."}, {LetRing, "let ring"},
		{NaturalHarmonic, "N.H."}, {ArtificialHarmonic, "A.H."},
		{PinchHarmonic, "P.H."}, {Tap, "T"},
	} {
		if got := tc.t.Label(); got != tc.want {
			t.Errorf("%v is labelled %q, want %q", tc.t, got, tc.want)
		}
	}
}

// A note can be palm-muted and hammered onto at once, so they accumulate
// rather than replace. What they do not do is accumulate copies.
func TestANoteCanCarryMoreThanOneAndNotTwice(t *testing.T) {
	var m Marks
	m.AddTechnique(PalmMute)
	m.AddTechnique(HammerOn)
	m.AddTechnique(PalmMute)

	if len(m.Techniques) != 2 {
		t.Errorf("it carries %v", m.Techniques)
	}
	if !m.HasTechnique(PalmMute) || !m.HasTechnique(HammerOn) {
		t.Errorf("one of them is missing: %v", m.Techniques)
	}
	if m.HasTechnique(Tap) {
		t.Error("it claims a tap nobody wrote")
	}
}

func TestOneCanBeTakenAwayAndTheRestStay(t *testing.T) {
	var m Marks
	m.AddTechnique(PalmMute)
	m.AddTechnique(HammerOn)

	if !m.RemoveTechnique(PalmMute) {
		t.Error("it would not take the palm mute away")
	}
	if m.RemoveTechnique(PalmMute) {
		t.Error("it took the same one away twice")
	}
	if !m.HasTechnique(HammerOn) {
		t.Error("taking the palm mute away took the hammer-on with it")
	}
}
