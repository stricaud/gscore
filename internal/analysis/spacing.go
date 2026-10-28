/*
 * spacing.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// How low a chord may be spaced before it turns to mud.
//
// This is the roughness of the harmonics panel pointed outward. That panel
// answers "is this chord rough" with a number; an orchestrator is asking the
// same question the other way round — "will this spacing speak down there" —
// and wants a warning on a bar, not a number in a dialog.
//
// The two are the same physics. Low notes have their partials packed close
// together in frequency, so an interval that is clear in the middle of the
// piano beats against itself an octave and a half down: a major third on
// middle C is a chord, and the same third two octaves lower is a rumble with
// no third in it. Where exactly that happens for each interval is the low
// interval limit, and orchestrators have been writing it down since
// Rimsky-Korsakov.
//
// The numbers below are the conventional ones and they vary a little between
// books — Adler, Piston and Rimsky-Korsakov do not agree to the semitone.
// What they agree about is the shape of it: the narrower the interval, the
// higher it has to be written. So this is offered as advice and never as a
// fault, the way the voice leading is: a composer writing a low third in the
// bassoons may want exactly that rumble, and plenty of music is made of it.

package analysis

import "sort"

// lowLimits is the lowest note each interval can be written on before it
// stops speaking clearly, by how many semitones wide the interval is.
//
// Intervals wider than an octave are not in it: a tenth low in the bass is
// ordinary writing and always has been, because the two notes are far
// enough apart for their partials to stay out of each other's way.
var lowLimits = map[int]int{
	1:  52, // a minor second, no lower than E3
	2:  51, // a major second, E♭3
	3:  48, // a minor third, C3
	4:  46, // a major third, B♭2
	5:  46, // a fourth, B♭2
	6:  46, // a tritone, B♭2
	7:  34, // a fifth, B♭1 — the open interval goes much lower
	8:  43, // a minor sixth, G2
	9:  41, // a major sixth, F2
	10: 41, // a minor seventh, F2
	11: 41, // a major seventh, F2
	12: 28, // an octave, E1
}

// LowLimit is the lowest note an interval can be written on, and whether
// there is a limit for it at all.
func LowLimit(semitones int) (int, bool) {
	at, ok := lowLimits[semitones]
	return at, ok
}

// intervalNames are the words for what is between two notes, for saying
// which interval is too low.
var intervalNames = [13]string{
	"a unison", "a minor second", "a major second", "a minor third",
	"a major third", "a fourth", "a tritone", "a fifth", "a minor sixth",
	"a major sixth", "a minor seventh", "a major seventh", "an octave",
}

// Thick is one stretch where the bottom of the harmony is spaced too low to
// speak clearly.
type Thick struct {
	// Text is the whole of it in a sentence.
	Text    string `json:"text"`
	FromBar int    `json:"fromBar"`
	ToBar   int    `json:"toBar"`
	Times   int    `json:"times"`
	// Low and High are the two notes at the bottom of the chord, and
	// Semitones what is between them.
	Low       int `json:"low"`
	High      int `json:"high"`
	Semitones int `json:"semitones"`
	// Limit is the lowest the interval is usually written, and Below how
	// far under it this one is.
	Limit int `json:"limit"`
	Below int `json:"below"`
	Ref   int `json:"ref"`
}

// LeastHeld is how long a low spacing has to be held before it is worth
// saying anything about, in whole notes.
//
// A quarter note. The limit is about sonorities, not about notes going past:
// a bass moving through a low second on its way somewhere is a passing tone
// and nobody hears mud in it, and reported anyway it buried Beethoven's
// ninth under eight hundred and eighty-one findings. What is heard as thick
// is a chord *held* down there.
const LeastHeld = 0.25

// LeastBelow is how far under its limit a spacing has to sit before it is
// reported, in semitones.
//
// Two. The limits are conventional and the books do not agree to the
// semitone — Adler, Piston and Rimsky-Korsakov each put them a little
// differently — so a spacing one semitone under one book's number is not a
// finding, it is the width of the disagreement.
const LeastBelow = 2

// Spacing finds where the bottom of the harmony is written too low for the
// interval it is spaced at.
//
// The bottom of it, and only the bottom: the limit is about the two lowest
// notes sounding, because they are the ones whose partials are packed close
// enough together to fight. What is stacked above them is a different
// question and the harmonics panel answers it.
//
// name says what a note is called, for the sentence.
func Spacing(sounds []Sound, name func(midi int) string) []Thick {
	if name == nil {
		name = func(int) string { return "" }
	}
	if len(sounds) < 2 {
		return nil
	}
	when := make([]float64, 0, len(sounds))
	for _, s := range sounds {
		when = append(when, s.At)
	}
	sort.Float64s(when)

	order := append([]Sound(nil), sounds...)
	sort.Slice(order, func(i, j int) bool { return order[i].At < order[j].At })

	var out []Thick
	// A sweep: the notes that have started and not yet stopped, carried
	// from one moment to the next rather than found again at each of them.
	// Looked for from the beginning every time, the ninth symphony took
	// half a second to answer and the cost grew with the square of the
	// piece.
	var active []Sound
	fed := 0
	for i := 0; i < len(when); i++ {
		if i > 0 && when[i] == when[i-1] {
			continue
		}
		t := when[i]
		for fed < len(order) && order[fed].At <= t {
			active = append(active, order[fed])
			fed++
		}
		keep := active[:0]
		for _, s := range active {
			if t < s.Until {
				keep = append(keep, s)
			}
		}
		active = keep

		var bottom, next Sound
		found := 0
		for _, s := range active {
			switch {
			case found == 0:
				bottom, found = s, 1
				next = Sound{}
			case s.MIDI < bottom.MIDI:
				next = bottom
				bottom, found = s, 2
			case next.Until == 0 || s.MIDI < next.MIDI:
				next, found = s, 2
			}
		}
		if found < 2 || next.MIDI <= bottom.MIDI {
			continue
		}
		low, high := bottom.MIDI, next.MIDI
		ref, bar := bottom.Ref, bottom.Bar
		gap := high - low
		limit, ok := LowLimit(gap)
		if !ok || limit-low < LeastBelow {
			continue
		}
		// How long the two of them stay together. A bass moving through a
		// low second on its way somewhere is a passing tone, and what is
		// heard as mud is a chord held down there.
		until := bottom.Until
		if next.Until < until {
			until = next.Until
		}
		if until-t < LeastHeld {
			continue
		}
		thick := Thick{
			FromBar: bar, ToBar: bar, Times: 1,
			Low: low, High: high, Semitones: gap,
			Limit: limit, Below: limit - low, Ref: ref,
		}
		thick.Text = intervalNames[gap] + " on " + name(low) +
			", which is usually written no lower than " + name(limit)
		out = append(out, thick)
	}
	return thickRuns(out)
}

// thickRuns folds a stretch of the same low interval into one finding, the
// way a run of anything else is folded: a passage spaced that way for twenty
// bars is one thing to know and not twenty.
func thickRuns(found []Thick) []Thick {
	var out []Thick
	for _, t := range found {
		n := len(out)
		if n > 0 && out[n-1].Semitones == t.Semitones && out[n-1].Low == t.Low &&
			t.FromBar-out[n-1].ToBar <= 1 {
			out[n-1].ToBar = t.ToBar
			out[n-1].Times++
			continue
		}
		out = append(out, t)
	}
	return out
}
