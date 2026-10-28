/*
 * key.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// What key the music is in, and where it changes.
//
// Not what the key signature says. A signature is written once and stands
// until somebody writes another one; the music moves through keys inside it
// all the time, and where it moves is most of what there is to understand
// about a piece. The development of a sonata is the signature of the
// exposition and almost none of its keys.
//
// The method is Krumhansl and Schmuckler's and it is nearly forty years old:
// add up how long each of the twelve pitch classes sounds over a stretch of
// music, and compare that against a profile of how long each degree sounds
// in a piece known to be in a given key. The profiles came out of
// Krumhansl and Kessler's listening experiments in 1982 — people were played
// a context and asked how well each note fitted it — so what is being
// matched against is what listeners actually report, not a theory of what
// they ought to report.
//
// Twenty-four comparisons, one per major and minor key, and the best
// correlation wins. It is arithmetic on the written notes: it answers before
// anything is played and answers the same way every time.

package analysis

import "math"

// Key is a tonic and a mode.
type Key struct {
	// Tonic is the pitch class, 0 for C.
	Tonic int  `json:"tonic"`
	Minor bool `json:"minor"`
}

var keyNames = [12]string{
	"C", "D♭", "D", "E♭", "E", "F", "F♯", "G", "A♭", "A", "B♭", "B",
}

// Sharp spellings, for the keys that are written with sharps. A piece in
// C♯ minor is not in D♭ minor, whatever the keyboard says.
var sharpNames = [12]string{
	"C", "C♯", "D", "D♯", "E", "F", "F♯", "G", "G♯", "A", "A♯", "B",
}

// Name is the key written out, spelled the way the music around it is
// spelled.
//
// A piece in four sharps calls the chord on its dominant G♯ major; the same
// key written in flats is A♭ major, and they are the same twelve notes.
// fifths is the key signature in force where the answer is being given:
// positive for sharps, negative for flats, zero for no opinion.
func (k Key) Name(fifths int) string {
	pc := ((k.Tonic % 12) + 12) % 12
	names := keyNames
	switch {
	case fifths > 0:
		names = sharpNames
	case fifths == 0 && k.Minor:
		// The minor keys that are usually written with sharps whatever is
		// around them: nobody writes D♭ minor.
		switch pc {
		case 1, 6, 8:
			names = sharpNames
		}
	}
	if k.Minor {
		return names[pc] + " minor"
	}
	return names[pc] + " major"
}

func (k Key) String() string { return k.Name(0) }

// Weights is how long each pitch class sounds over a stretch of music, in
// whole notes. Length and not a count of notes: a semibreve on the tonic
// says more about the key than six passing sixteenths, and counting notes
// says the opposite.
type Weights [12]float64

// Total is how much music the weights were taken from.
func (w Weights) Total() float64 {
	t := 0.0
	for _, v := range w {
		t += v
	}
	return t
}

// The Krumhansl and Kessler profiles: how strongly each degree fits a
// context in that mode, from the 1982 probe-tone experiments.
var (
	majorProfile = [12]float64{
		6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88,
	}
	minorProfile = [12]float64{
		6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17,
	}
)

// Fit is how good an answer is, in two numbers that mean different things.
//
// Correlation says how like a key the music is at all. Margin says how far
// clear the winner is of the next best key, and it is the one that says
// whether there is an answer or a tie: a single C correlates 0.68 with all
// twenty-four keys at once and is nothing at all clear of any of them, and
// a bare fifth is a tenth clear. A phrase that really is in a key comes out
// a quarter or more clear of everything else.
//
// Reported, not ruled on. "F♯ minor, barely" is a true thing to say about
// two notes; "F♯ minor" in the voice used for a whole exposition is not.
type Fit struct {
	Correlation float64 `json:"correlation"`
	Margin      float64 `json:"margin"`
}

// KeyMargin is how far clear of the next best key an answer has to stand
// before it is worth saying plainly. Measured rather than chosen: a scale
// comes out about 0.27 clear, a lone triad about 0.07.
const KeyMargin = 0.15

// Clear says the answer stands far enough above the rest to be stated as one.
func (f Fit) Clear() bool { return f.Margin >= KeyMargin }

// KeyOf is the key a stretch of music is most likely in, and how well that
// answer stands up against the alternatives.
func KeyOf(w Weights) (Key, Fit) {
	best := Key{}
	first, second := math.Inf(-1), math.Inf(-1)
	for tonic := 0; tonic < 12; tonic++ {
		for _, minor := range []bool{false, true} {
			profile := majorProfile
			if minor {
				profile = minorProfile
			}
			var rotated [12]float64
			for i := 0; i < 12; i++ {
				rotated[i] = profile[((i-tonic)%12+12)%12]
			}
			r := correlate(w[:], rotated[:])
			switch {
			case r > first:
				best, first, second = Key{Tonic: tonic, Minor: minor}, r, first
			case r > second:
				second = r
			}
		}
	}
	if math.IsInf(first, -1) {
		return Key{}, Fit{}
	}
	if math.IsInf(second, -1) {
		second = first
	}
	return best, Fit{Correlation: first, Margin: first - second}
}

// correlate is Pearson's r between two runs of numbers of the same length.
func correlate(a, b []float64) float64 {
	n := float64(len(a))
	if n == 0 {
		return 0
	}
	ma, mb := 0.0, 0.0
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma, mb = ma/n, mb/n
	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		x, y := a[i]-ma, b[i]-mb
		num += x * y
		da += x * x
		db += y * y
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / math.Sqrt(da*db)
}

// KeySpan is a stretch of bars in one key.
type KeySpan struct {
	FromBar int    `json:"fromBar"`
	ToBar   int    `json:"toBar"`
	Key     Key    `json:"key"`
	Name    string `json:"name"`
	// Fit is how good the answer is. A span with a small margin is a
	// stretch of music that does not settle anywhere, which is a true
	// thing to know about it.
	Fit Fit `json:"fit"`
}

// KeyWindow is how many bars are looked at to decide the key at one bar.
//
// Four. One bar is a chord and a chord is in a dozen keys at once; a whole
// phrase is long enough to have a key and short enough to notice when it
// changes. Longer and a modulation is smeared across the bars either side of
// where it happens, which is exactly the thing somebody is looking for.
const KeyWindow = 4

// KeyRun is how many bars in a row have to agree before a change of key is
// believed.
//
// A secondary dominant is not a modulation. Two bars of it will tip a
// four-bar window into the key it points at, and a reading that changes key
// every time the harmony leans somewhere is a reading nobody can follow.
const KeyRun = 2

// Keys reads the key through a piece, bar by bar, and gathers the answer
// into the stretches where it stays the same.
//
// bars is one set of weights per bar of the score. Empty bars count as
// empty: a staff resting does not change the key.
func Keys(bars []Weights) []KeySpan {
	if len(bars) == 0 {
		return nil
	}
	// The key at each bar, from the window of bars around it.
	at := make([]Key, len(bars))
	how := make([]Fit, len(bars))
	for i := range bars {
		var w Weights
		for j := i; j < i+KeyWindow && j < len(bars); j++ {
			for pc := 0; pc < 12; pc++ {
				w[pc] += bars[j][pc]
			}
		}
		// A window with nothing in it reaches back for its context rather
		// than inventing one.
		if w.Total() == 0 && i > 0 {
			at[i], how[i] = at[i-1], how[i-1]
			continue
		}
		at[i], how[i] = KeyOf(w)
	}

	var out []KeySpan
	i := 0
	for i < len(at) {
		j := i
		for j+1 < len(at) && at[j+1] == at[i] {
			j++
		}
		// Too short to believe: a secondary dominant tipping the window
		// over for a bar is not a change of key. It is folded into what
		// came before, or into what comes after where it opens the piece.
		if j-i+1 < KeyRun && len(out) > 0 {
			out[len(out)-1].ToBar = j
			i = j + 1
			continue
		}
		span := KeySpan{
			FromBar: i, ToBar: j, Key: at[i], Name: at[i].String(),
			Fit: how[i],
		}
		// The best the key fitted anywhere in the stretch: the bars around
		// a modulation are the ambiguous ones, and judging the whole
		// stretch by them would call every settled key doubtful.
		for k := i; k <= j; k++ {
			if how[k].Margin > span.Fit.Margin {
				span.Fit = how[k]
			}
		}
		out = append(out, span)
		i = j + 1
	}
	// A run that was folded into the one before it may have left two
	// neighbours in the same key.
	return joinKeys(out)
}

func joinKeys(in []KeySpan) []KeySpan {
	var out []KeySpan
	for _, s := range in {
		if n := len(out); n > 0 && out[n-1].Key == s.Key {
			out[n-1].ToBar = s.ToBar
			if s.Fit.Margin > out[n-1].Fit.Margin {
				out[n-1].Fit = s.Fit
			}
			continue
		}
		out = append(out, s)
	}
	return out
}
