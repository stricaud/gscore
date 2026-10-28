/*
 * voices_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How the lines move against each other.
 *
 * The one part of the analysis that judges rather than measures. Parallel
 * fifths are not wrong; they are wrong in a tradition, and they are the
 * whole sound of organum, of Debussy and of every power chord ever played.
 * So these tests are about what was *seen*, never about what is allowed.
 */

package analysis

import "testing"

var soprano = Line{Part: 0, Voice: 0, Rank: 0, Name: "soprano"}
var bass = Line{Part: 1, Voice: 0, Rank: 0, Name: "bass"}

// pair builds two lines of equal-length notes from two runs of pitches.
func pair(top, bottom []int) []Sound {
	var out []Sound
	for i := range top {
		at := float64(i) * 0.25
		out = append(out,
			Sound{Line: 0, Bar: i / 4, At: at, Until: at + 0.25,
				MIDI: top[i], Ref: 100 + i},
			Sound{Line: 1, Bar: i / 4, At: at, Until: at + 0.25,
				MIDI: bottom[i], Ref: 200 + i})
	}
	return out
}

func lines() []Line { return []Line{soprano, bass} }

func kinds(ms []Motion) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Kind)
	}
	return out
}

func has(ms []Motion, kind string) bool {
	for _, m := range ms {
		if m.Kind == kind {
			return true
		}
	}
	return false
}

func TestParallelFifthsAreSeen(t *testing.T) {
	// C–G then D–A: both up a tone, a fifth apart throughout.
	got := Moving(lines(), pair([]int{67, 69}, []int{60, 62}))
	if !has(got, "parallel fifths") {
		t.Errorf("parallel fifths were not seen: %v", kinds(got))
	}
	if len(got) > 0 && got[0].Upper != "soprano" {
		t.Errorf("the finding is about %q and %q", got[0].Upper, got[0].Lower)
	}
}

// Across an octave and a fifth it is the same interval and the same
// parallel: a twelfth is a fifth.
func TestParallelTwelfthsAreParallelFifths(t *testing.T) {
	got := Moving(lines(), pair([]int{79, 81}, []int{60, 62}))
	if !has(got, "parallel fifths") {
		t.Errorf("a parallel twelfth was not seen as a fifth: %v", kinds(got))
	}
}

func TestParallelOctavesAreSeen(t *testing.T) {
	got := Moving(lines(), pair([]int{72, 74}, []int{60, 62}))
	if !has(got, "parallel octaves") {
		t.Errorf("parallel octaves were not seen: %v", kinds(got))
	}
}

func TestParallelUnisonsAreNamedAsUnisons(t *testing.T) {
	got := Moving(lines(), pair([]int{60, 62}, []int{60, 62}))
	if !has(got, "parallel unisons") {
		t.Errorf("parallel unisons were not named as such: %v", kinds(got))
	}
}

// Contrary motion into a fifth is not a parallel, and neither is similar
// motion between other intervals.
func TestContraryMotionIsNotAParallel(t *testing.T) {
	got := Moving(lines(), pair([]int{67, 65}, []int{60, 62}))
	if has(got, "parallel fifths") {
		t.Errorf("contrary motion was called a parallel: %v", kinds(got))
	}
}

func TestSimilarMotionBetweenOtherIntervalsIsNothing(t *testing.T) {
	// A third to a sixth, both rising.
	got := Moving(lines(), pair([]int{64, 69}, []int{60, 61}))
	if len(got) != 0 {
		t.Errorf("ordinary similar motion was reported: %v", kinds(got))
	}
}

// A line holding a note while another moves under it is oblique motion and
// is nobody's idea of a parallel.
func TestAHeldNoteIsNotAParallel(t *testing.T) {
	sounds := []Sound{
		{Line: 0, Bar: 0, At: 0, Until: 0.5, MIDI: 67, Ref: 1},
		{Line: 1, Bar: 0, At: 0, Until: 0.25, MIDI: 60, Ref: 2},
		{Line: 1, Bar: 0, At: 0.25, Until: 0.5, MIDI: 55, Ref: 3},
	}
	if got := Moving(lines(), sounds); len(got) != 0 {
		t.Errorf("a held note over a moving one was reported: %v", kinds(got))
	}
}

// Where one of the two lines is not sounding there is no pair to judge.
func TestARestBreaksThePair(t *testing.T) {
	sounds := []Sound{
		{Line: 0, Bar: 0, At: 0, Until: 0.25, MIDI: 67, Ref: 1},
		{Line: 1, Bar: 0, At: 0, Until: 0.25, MIDI: 60, Ref: 2},
		// The bass rests, then both move in fifths across the gap.
		{Line: 0, Bar: 0, At: 0.5, Until: 0.75, MIDI: 69, Ref: 3},
		{Line: 1, Bar: 0, At: 0.5, Until: 0.75, MIDI: 62, Ref: 4},
	}
	if got := Moving(lines(), sounds); has(got, "parallel fifths") {
		t.Errorf("a parallel was found across a rest: %v", kinds(got))
	}
}

// The line written above going below the one written under it.
func TestCrossingIsSeen(t *testing.T) {
	got := Moving(lines(), pair([]int{64, 55}, []int{60, 59}))
	if !has(got, "crossing") {
		t.Errorf("a crossing was not seen: %v", kinds(got))
	}
}

// And the lower line moving above where the upper one just was, without
// passing it: the two have changed places and it reads as one line jumping.
func TestOverlapIsSeen(t *testing.T) {
	sounds := []Sound{
		{Line: 0, Bar: 0, At: 0, Until: 0.25, MIDI: 64, Ref: 1},
		{Line: 1, Bar: 0, At: 0, Until: 0.25, MIDI: 55, Ref: 2},
		{Line: 0, Bar: 1, At: 0.25, Until: 0.5, MIDI: 72, Ref: 3},
		{Line: 1, Bar: 1, At: 0.25, Until: 0.5, MIDI: 67, Ref: 4},
	}
	got := Moving(lines(), sounds)
	if !has(got, "overlap") {
		t.Errorf("an overlap was not seen: %v", kinds(got))
	}
}

// Every finding points at a bar and a notehead, because a finding nobody
// can go to is a finding nobody can act on.
func TestEveryFindingPointsSomewhere(t *testing.T) {
	got := Moving(lines(), pair([]int{67, 69}, []int{60, 62}))
	if len(got) == 0 {
		t.Fatal("nothing was found to check")
	}
	for i, m := range got {
		if m.Ref == 0 {
			t.Errorf("finding %d points at no notehead", i)
		}
		if m.Text == "" {
			t.Errorf("finding %d says nothing", i)
		}
		if m.Upper == "" || m.Lower == "" {
			t.Errorf("finding %d does not say which lines it is about", i)
		}
	}
}

// Nothing to compare, nothing said.
func TestOneLineHasNothingToMoveAgainst(t *testing.T) {
	if got := Moving([]Line{soprano}, pair([]int{60, 62}, []int{60, 62})); got != nil {
		t.Errorf("one line was found fault with: %v", kinds(got))
	}
}

// Every pair of lines is looked at, not only the outer two: the fifths
// between an inner alto and tenor are the ones nobody notices.
func TestEveryPairIsLookedAt(t *testing.T) {
	three := []Line{
		{Name: "soprano"}, {Name: "alto"}, {Name: "bass"},
	}
	sounds := []Sound{
		// Soprano and alto in fifths; the bass goes the other way.
		{Line: 0, Bar: 0, At: 0, Until: 0.25, MIDI: 74, Ref: 1},
		{Line: 1, Bar: 0, At: 0, Until: 0.25, MIDI: 67, Ref: 2},
		{Line: 2, Bar: 0, At: 0, Until: 0.25, MIDI: 55, Ref: 3},
		{Line: 0, Bar: 0, At: 0.25, Until: 0.5, MIDI: 76, Ref: 4},
		{Line: 1, Bar: 0, At: 0.25, Until: 0.5, MIDI: 69, Ref: 5},
		{Line: 2, Bar: 0, At: 0.25, Until: 0.5, MIDI: 53, Ref: 6},
	}
	got := Moving(three, sounds)
	found := false
	for _, m := range got {
		if m.Kind == "parallel fifths" && m.Upper == "soprano" && m.Lower == "alto" {
			found = true
		}
	}
	if !found {
		t.Errorf("fifths between two inner lines were missed: %v", got)
	}
}
