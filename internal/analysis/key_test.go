/*
 * key_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What key the music is in.
 *
 * Not what the key signature says: a signature is written once and the music
 * moves through keys inside it all the time, and where it moves is most of
 * what there is to understand about a piece.
 */

package analysis

import "testing"

// scale builds the weights of a run up a scale, a beat on each degree.
func scale(tonic int, steps ...int) Weights {
	var w Weights
	for _, s := range steps {
		w[((tonic+s)%12+12)%12] += 0.25
	}
	return w
}

var majorScale = []int{0, 2, 4, 5, 7, 9, 11}
var minorScale = []int{0, 2, 3, 5, 7, 8, 11} // harmonic, as music is written

func TestAMajorScaleIsReadAsItsOwnKey(t *testing.T) {
	for tonic := 0; tonic < 12; tonic++ {
		w := scale(tonic, majorScale...)
		// Weight the tonic and dominant, as a piece does.
		w[tonic] += 0.75
		w[(tonic+7)%12] += 0.5
		k, fit := KeyOf(w)
		if k.Tonic != tonic || k.Minor {
			t.Errorf("a scale on %d came out as %v", tonic, k)
		}
		if !fit.Clear() {
			t.Errorf("a plain scale on %d is only %.3f clear of the next key",
				tonic, fit.Margin)
		}
	}
}

func TestAMinorScaleIsReadAsMinor(t *testing.T) {
	w := scale(9, minorScale...) // A minor
	w[9] += 0.75
	w[4] += 0.5
	k, _ := KeyOf(w)
	if k.Tonic != 9 || !k.Minor {
		t.Errorf("an A minor scale came out as %v", k)
	}
}

// How sure the answer is is what makes it usable, and the correlation alone
// does not say: a bare fifth correlates 0.81 with C major, which sounds like
// certainty and is not. What says it is the gap to the next best key, and
// for a bare fifth there barely is one.
func TestTwoNotesAreNotClearOfAnything(t *testing.T) {
	var w Weights
	w[0], w[7] = 1, 1
	_, fit := KeyOf(w)
	if fit.Correlation < 0.7 {
		t.Errorf("a bare fifth correlates %.2f, and the point is that it is high",
			fit.Correlation)
	}
	if fit.Clear() {
		t.Errorf("a bare fifth is called a key, %.3f clear of the next one",
			fit.Margin)
	}
}

// One note is in every key at once, and the arithmetic says so exactly.
func TestOneNoteIsClearOfNothingAtAll(t *testing.T) {
	var w Weights
	w[0] = 1
	if _, fit := KeyOf(w); fit.Margin > 0.001 {
		t.Errorf("one note is %.3f clear of a second key", fit.Margin)
	}
}

func TestNothingSoundingHasNoKey(t *testing.T) {
	if _, fit := KeyOf(Weights{}); fit.Correlation != 0 || fit.Margin != 0 {
		t.Errorf("silence came out as %+v", fit)
	}
}

func TestAKeyIsNamedTheWayItIsWritten(t *testing.T) {
	for _, c := range []struct {
		k    Key
		want string
	}{
		{Key{Tonic: 0}, "C major"},
		{Key{Tonic: 1, Minor: true}, "C♯ minor"},
		{Key{Tonic: 3}, "E♭ major"},
		{Key{Tonic: 9, Minor: true}, "A minor"},
		{Key{Tonic: 6, Minor: true}, "F♯ minor"},
	} {
		if got := c.k.String(); got != c.want {
			t.Errorf("%+v is called %q, want %q", c.k, got, c.want)
		}
	}
}

// A piece that stays put is reported as one stretch, not as a bar-by-bar
// commentary.
func TestAPieceInOneKeyIsOneSpan(t *testing.T) {
	bars := make([]Weights, 16)
	for i := range bars {
		bars[i] = scale(0, majorScale...)
		bars[i][0] += 0.75
		bars[i][7] += 0.5
	}
	got := Keys(bars)
	if len(got) != 1 {
		t.Fatalf("%d stretches for a piece that never leaves C: %+v", len(got), got)
	}
	if got[0].Key.Tonic != 0 || got[0].FromBar != 0 || got[0].ToBar != 15 {
		t.Errorf("the piece is %+v", got[0])
	}
}

// And a piece that moves is reported as moving.
func TestAModulationIsFound(t *testing.T) {
	bars := make([]Weights, 24)
	for i := range bars {
		tonic := 0 // C
		if i >= 12 {
			tonic = 7 // and then G
		}
		bars[i] = scale(tonic, majorScale...)
		bars[i][tonic] += 0.75
		bars[i][(tonic+7)%12] += 0.5
	}
	got := Keys(bars)
	if len(got) < 2 {
		t.Fatalf("a modulation to the dominant was not found: %+v", got)
	}
	if got[0].Key.Tonic != 0 {
		t.Errorf("the piece opens in %v", got[0].Key)
	}
	last := got[len(got)-1]
	if last.Key.Tonic != 7 {
		t.Errorf("it ends in %v, want G", last.Key)
	}
	// Within a window of where it really happens. The window is four bars
	// wide, so the change is seen as the new key comes into it.
	if last.FromBar < 8 || last.FromBar > 13 {
		t.Errorf("the change is reported at bar %d, and it is written at 12",
			last.FromBar)
	}
}

// A secondary dominant is not a modulation, and a reading that changes key
// every time the harmony leans somewhere is a reading nobody can follow.
func TestOneBarOfLeaningIsNotAChangeOfKey(t *testing.T) {
	bars := make([]Weights, 16)
	for i := range bars {
		bars[i] = scale(0, majorScale...)
		bars[i][0] += 0.75
	}
	// One bar with an F sharp in it, the way a V/V is written.
	bars[8] = Weights{}
	bars[8][2], bars[8][6], bars[8][9] = 1, 1, 1

	got := Keys(bars)
	for _, s := range got {
		if s.ToBar-s.FromBar == 0 && s.Key.Tonic != 0 {
			t.Errorf("a one-bar lean was reported as a key: %+v", s)
		}
	}
}

// Bars with nothing in them do not change the key: a staff resting is not a
// modulation to silence.
func TestRestsCarryTheKeyThrough(t *testing.T) {
	bars := make([]Weights, 12)
	for i := range bars {
		if i >= 4 && i <= 7 {
			continue // four bars of rest
		}
		bars[i] = scale(0, majorScale...)
		bars[i][0] += 0.75
	}
	got := Keys(bars)
	if len(got) != 1 {
		t.Errorf("the rests broke the piece into %d keys: %+v", len(got), got)
	}
}
