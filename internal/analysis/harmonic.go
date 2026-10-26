/*
 * harmonic.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package analysis looks at what is sounding at one moment and says what the
// harmony is made of.
//
// Not a picture of the loudspeaker. A spectrum taken off the audio shows the
// soundfont — its samples, its filters, the reverb — and changes if you swap
// the instrument for another one playing the same notes. What a composer is
// asking when they ask whether a chord works is a question about the notes:
// every note carries a series of partials above it, and whether a chord is
// smooth or rough depends on whether those partials fall on each other,
// stand clear of each other, or land close enough to beat.
//
// That is arithmetic on the written pitches and it has been understood since
// Helmholtz. Three things come out of it:
//
//   - where the partials are, and which of them coincide: a fifth is
//     consonant because the third partial of the lower note is the second
//     partial of the upper, and they reinforce rather than fight
//   - how rough the result is, by the Plomp and Levelt curve as Sethares
//     applies it to whole spectra: two partials a critical band apart are
//     maximally rough, and further apart or much closer they are not
//   - how far equal temperament has bent each interval away from the whole
//     number ratio it is named after, in cents
//
// None of it needs the sound to be made, so it answers before you press
// play, and it answers the same way every time.
package analysis

import (
	"math"
	"sort"
	"sync"
)

// A4 is the tuning the frequencies are counted from.
const A4 = 440.0

// Frequency is what a MIDI note sounds at, in equal temperament.
func Frequency(midi int) float64 {
	return A4 * math.Pow(2, float64(midi-69)/12)
}

// Cents is the interval between two frequencies, in hundredths of a
// semitone: the unit small intervals are argued about in.
func Cents(from, to float64) float64 {
	if from <= 0 || to <= 0 {
		return 0
	}
	return 1200 * math.Log2(to/from)
}

// Note is one sounding note.
type Note struct {
	// MIDI is the pitch as it sounds, so a clarinet's written C arrives
	// here as the B flat a listener hears.
	MIDI int `json:"midi"`
	// Name is that pitch written out, for the panel to label with.
	Name string `json:"name"`
	// Freq is what it sounds at.
	Freq float64 `json:"freq"`
	// Part is the staff it is on, and Velocity how hard it was struck.
	Part     int `json:"part"`
	Velocity int `json:"velocity"`
	// Ref is the notehead it came from, so the panel can light it up.
	Ref int `json:"ref"`
}

// Partial is one component of one note's sound.
type Partial struct {
	// Note is the index in Harmonics.Notes this partial belongs to, and N
	// which harmonic it is: 1 is the note itself, 2 the octave above it, 3
	// the twelfth, and so on up.
	Note int     `json:"note"`
	N    int     `json:"n"`
	Freq float64 `json:"freq"`
	Amp  float64 `json:"amp"`
}

// Match is two partials of different notes landing on the same frequency.
//
// This is what consonance is made of. Where they coincide exactly the notes
// share a partial and reinforce each other; where they are a few cents apart
// the sum beats at the difference, slowly enough to be heard as a wobble
// rather than as roughness.
type Match struct {
	// A and B are the notes, NA and NB which harmonic of each.
	A     int     `json:"a"`
	B     int     `json:"b"`
	NA    int     `json:"na"`
	NB    int     `json:"nb"`
	Freq  float64 `json:"freq"`
	Cents float64 `json:"cents"`
	// Beats is how many times a second the two partials beat against each
	// other, which is what you hear when they are close but not equal.
	Beats float64 `json:"beats"`
}

// Interval is the distance between two of the notes, and how far equal
// temperament has moved it from the ratio it is named after.
type Interval struct {
	// A and B are the two notes, lower first.
	A         int    `json:"a"`
	B         int    `json:"b"`
	Semitones int    `json:"semitones"`
	Name      string `json:"name"`
	Ratio     string `json:"ratio"`
	// Cents is how far the tempered interval is from the just one: positive
	// means wider. A tempered major third is fourteen cents sharp of a 5:4,
	// which is why it beats where the fifth does not.
	Cents float64 `json:"cents"`
}

// Harmonics is the whole picture at one moment.
type Harmonics struct {
	At       float64   `json:"at"`
	Notes    []Note    `json:"notes"`
	Partials []Partial `json:"partials"`
	// Coincident lists the partials that meet, nearest first.
	Coincident []Match    `json:"coincident"`
	Intervals  []Interval `json:"intervals"`
	// Roughness is the Plomp and Levelt dissonance of the whole sound,
	// scaled so that a single note is 0 and a seriously beating cluster
	// approaches 1. It is a number to compare chords with, not a verdict:
	// a rough chord in the right place is the point of the piece.
	Roughness float64 `json:"roughness"`
	// Verdict puts that number in words, for reading at a glance.
	Verdict string `json:"verdict"`
}

// Options are the assumptions the analysis is made under.
type Options struct {
	// Partials is how far up each note's series to look. Eight reaches the
	// seventh harmonic, which is as far as the ear picks partials out
	// individually on most instruments.
	Partials int
	// Rolloff is how fast the partials get quieter going up, as the power
	// the harmonic number is raised to. One is the textbook sawtooth, where
	// the nth partial is a nth as loud; a flute is steeper and a reed is
	// shallower, and the choice moves the roughness a little and the
	// coincidences not at all.
	Rolloff float64
	// Tolerance is how close two partials must be, in cents, to count as
	// coinciding.
	Tolerance float64
}

// DefaultOptions are what the panel asks for.
func DefaultOptions() Options {
	return Options{Partials: 8, Rolloff: 1, Tolerance: 35}
}

// Of works out the harmony of a set of sounding notes.
func Of(notes []Note, opt Options) Harmonics {
	if opt.Partials <= 0 {
		opt.Partials = DefaultOptions().Partials
	}
	if opt.Rolloff <= 0 {
		opt.Rolloff = DefaultOptions().Rolloff
	}
	if opt.Tolerance <= 0 {
		opt.Tolerance = DefaultOptions().Tolerance
	}

	out := Harmonics{Notes: notes}
	for i := range out.Notes {
		if out.Notes[i].Freq == 0 {
			out.Notes[i].Freq = Frequency(out.Notes[i].MIDI)
		}
	}

	// The partials of every note, which is the spectrum the chord would
	// have if each instrument were a plain harmonic tone.
	out.Partials = partialsOf(out.Notes, opt)

	out.Coincident = coincidences(out.Partials, opt.Tolerance)
	out.Intervals = intervals(out.Notes)
	out.Roughness = roughness(out.Partials, len(out.Notes))
	out.Verdict = verdictFor(out.Roughness, len(out.Notes))
	return out
}

// coincidences finds the partials of different notes that land together.
func coincidences(partials []Partial, tolerance float64) []Match {
	var out []Match
	for i := 0; i < len(partials); i++ {
		for j := i + 1; j < len(partials); j++ {
			a, b := partials[i], partials[j]
			if a.Note == b.Note {
				continue // a note's own partials are not a coincidence
			}
			c := Cents(a.Freq, b.Freq)
			if math.Abs(c) > tolerance {
				continue
			}
			out = append(out, Match{
				A: a.Note, B: b.Note, NA: a.N, NB: b.N,
				Freq: (a.Freq + b.Freq) / 2, Cents: c,
				Beats: math.Abs(a.Freq - b.Freq),
			})
		}
	}
	// Exact meetings first: those are the ones holding the chord together.
	sort.SliceStable(out, func(i, j int) bool {
		return math.Abs(out[i].Cents) < math.Abs(out[j].Cents)
	})
	return out
}

// justRatios are the whole number ratios the intervals of the scale are
// named after, by how many semitones they are rounded to.
//
// The ratio is what the ear recognises; the tempered interval is an
// approximation of it, and the difference is why some chords are restless
// in equal temperament and the same chords are not on a violin.
var justRatios = map[int]struct {
	name  string
	ratio string
	num   float64
	den   float64
}{
	0:  {"unison", "1:1", 1, 1},
	1:  {"minor second", "16:15", 16, 15},
	2:  {"major second", "9:8", 9, 8},
	3:  {"minor third", "6:5", 6, 5},
	4:  {"major third", "5:4", 5, 4},
	5:  {"perfect fourth", "4:3", 4, 3},
	6:  {"tritone", "7:5", 7, 5},
	7:  {"perfect fifth", "3:2", 3, 2},
	8:  {"minor sixth", "8:5", 8, 5},
	9:  {"major sixth", "5:3", 5, 3},
	10: {"minor seventh", "16:9", 16, 9},
	11: {"major seventh", "15:8", 15, 8},
}

// intervals names the distance between every pair of notes and says how far
// the tempered version is from the just one.
func intervals(notes []Note) []Interval {
	var out []Interval
	for i := 0; i < len(notes); i++ {
		for j := i + 1; j < len(notes); j++ {
			lo, hi := notes[i], notes[j]
			a, b := i, j
			if lo.MIDI > hi.MIDI {
				lo, hi = hi, lo
				a, b = b, a
			}
			semis := hi.MIDI - lo.MIDI
			octaves := semis / 12
			within := semis % 12
			r, ok := justRatios[within]
			if !ok {
				continue
			}
			// The just interval, carried up by however many octaves the
			// two notes are actually apart.
			just := r.num / r.den * math.Pow(2, float64(octaves))
			name := r.name
			if octaves > 0 && within == 0 {
				name = "octave"
				if octaves > 1 {
					name = "octaves"
				}
			}
			out = append(out, Interval{
				A: a, B: b, Semitones: semis, Name: name, Ratio: r.ratio,
				Cents: Cents(lo.Freq*just, hi.Freq),
			})
		}
	}
	return out
}

// roughness is how much the partials grate against each other, measured in
// semitones.
//
// Plomp and Levelt measured how rough two pure tones sound at every
// separation and found the worst is about a quarter of a critical band
// apart — close, but not close enough to fuse. Sethares' formula below fits
// their curve, and a whole sound is the sum of it over every pair of
// partials in it.
//
// That sum is a number with no units, which tells a composer nothing. So it
// is given in semitones: 1 is exactly as rough as two notes a semitone
// apart in the middle of the staff, the interval everybody already has an
// opinion about. A fifth comes out around a tenth of that, a major third
// about a quarter, a cluster above 1.
//
// Loudness is deliberately left out. The curve scales with amplitude, so
// the same chord played quietly would measure smoother — and a quiet minor
// second is still a minor second. The partials are weighed against each
// other within the chord, never against the dynamic.
func roughness(partials []Partial, notes int) float64 {
	// Per pair of notes, not per chord.
	//
	// The sum grows with the number of pairs, so a major triad — three
	// intervals, all of them consonant — came out rougher than one
	// semitone simply by having three of them, and read as the worst sound
	// in the piece. What a composer is asking is how rough the chord is,
	// not how much chord there is, so the answer is the average over the
	// intervals in it.
	pairs := notes * (notes - 1) / 2
	if pairs <= 0 {
		return 0
	}
	if r := rawRoughness(partials) / semitoneUnit() / float64(pairs); r > 0 {
		return r
	}
	return 0
}

// rawRoughness is the Plomp and Levelt sum over every pair of partials
// belonging to different notes, with the loudest partial counted as 1.
func rawRoughness(partials []Partial) float64 {
	const (
		b1 = 3.5
		b2 = 5.75
		// dstar is where the curve peaks, as a fraction of the critical
		// band; s1 and s2 turn a frequency into the width of that band.
		dstar = 0.24
		s1    = 0.0207
		s2    = 18.96
	)

	loudest := 0.0
	for _, p := range partials {
		if p.Amp > loudest {
			loudest = p.Amp
		}
	}
	if loudest == 0 {
		return 0
	}

	total := 0.0
	for i := 0; i < len(partials); i++ {
		for j := i + 1; j < len(partials); j++ {
			a, b := partials[i], partials[j]
			if a.Note == b.Note {
				// A note is not rough against itself: its own partials are
				// what make it the sound it is.
				continue
			}
			amp := math.Min(a.Amp, b.Amp) / loudest
			f1, f2 := a.Freq, b.Freq
			if f1 > f2 {
				f1, f2 = f2, f1
			}
			if f1 <= 0 {
				continue
			}
			s := dstar / (s1*f1 + s2)
			d := f2 - f1
			total += amp * (math.Exp(-b1*s*d) - math.Exp(-b2*s*d))
		}
	}
	return total
}

// semitoneUnit is what two notes a semitone apart in the middle of the staff
// measure, which is the unit roughness is given in.
//
// Worked out from the same formula rather than written down as a constant,
// so that changing how many partials are counted, or how fast they fall
// away, moves the yardstick along with the measurement.
var semitoneUnit = sync.OnceValue(func() float64 {
	opt := DefaultOptions()
	return rawRoughness(partialsOf(
		[]Note{{MIDI: 60, Freq: Frequency(60)}, {MIDI: 61, Freq: Frequency(61)}},
		opt))
})

// partialsOf is each note's harmonic series, as far up as the options ask.
func partialsOf(notes []Note, opt Options) []Partial {
	var out []Partial
	for i, n := range notes {
		loud := float64(n.Velocity) / 127
		if n.Velocity == 0 {
			loud = 1
		}
		f := n.Freq
		if f == 0 {
			f = Frequency(n.MIDI)
		}
		for h := 1; h <= opt.Partials; h++ {
			out = append(out, Partial{
				Note: i, N: h,
				Freq: f * float64(h),
				Amp:  loud / math.Pow(float64(h), opt.Rolloff),
			})
		}
	}
	return out
}

// verdictFor puts the number in words.
//
// The thresholds are not laws. They are where the curve stops sounding like
// a chord and starts sounding like two notes arguing, on the instruments
// most people write for, and they are here so the panel can say something
// rather than only show a bar.
func verdictFor(r float64, notes int) string {
	switch {
	case notes == 0:
		return "nothing sounding"
	case notes == 1:
		return "one note: nothing to beat against"
	case r < 0.2:
		return "smooth — the partials line up"
	case r < 0.5:
		return "mild — some beating between the partials"
	case r < 1:
		return "rough — the partials are fighting"
	default:
		return "very rough — as close as a semitone, or closer"
	}
}
