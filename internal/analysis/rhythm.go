/*
 * rhythm.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Where the notes fall against the beat.
//
// A metre is a pattern of strong and weak, and almost everything a rhythm
// does it does by agreeing with that pattern or pulling against it. A bar of
// four with the notes on one and three is the metre spoken plainly; the same
// notes a quaver late are the same notes and a different piece.
//
// Which of those is happening is arithmetic on where each note starts and
// how long it holds, and it is the sort of arithmetic nobody does by eye
// over four hundred bars. What comes out is a description and not a verdict:
// syncopation is not a fault, it is most of what makes music move.
//
// Two things here are checks rather than descriptions, and they are kept
// apart from it. A bar holding more or less than its metre says is a fact
// about the file, usually an import that went wrong or an edit that did.
// A passage written faster than it can be articulated is a fact about the
// player, like a note below the bottom of an instrument.

package analysis

import "sort"

// Beat is one note's place in its bar.
type Beat struct {
	Bar int
	// At is where in the bar it starts, in whole notes, and Held how long
	// it lasts.
	At, Held float64
	// Beats is how many beats the bar has and Unit how long one is, in
	// whole notes: 4 and 0.25 for four-four.
	Beats int
	Unit  float64
	Part  int
	Ref   int
}

// Pulse is how one staff sits against the metre.
type Pulse struct {
	Part int    `json:"part"`
	Name string `json:"name"`
	// Notes is how many notes it has, and the three counts below say where
	// they start. They add up to Notes.
	Notes int `json:"notes"`
	// Downbeats start on the first beat of a bar, OnBeat on any other
	// beat, OffBeat between beats.
	Downbeats int `json:"downbeats"`
	OnBeat    int `json:"onBeat"`
	OffBeat   int `json:"offBeat"`
	// Syncopated is how many of them start off the beat and hold across
	// the next one — which is syncopation proper, as against a note that
	// merely falls between beats and stops before the next.
	Syncopated int `json:"syncopated"`
	// Bars is how many notes pull against the metre in each bar, for
	// drawing the piece as a strip.
	Bars []int `json:"bars"`
}

// Pulses reads how each staff sits against the metre.
//
// beats may be in any order. staves is how many staves the score has and
// bars how many bars, so that a staff with nothing in it is still reported
// and every strip is the same length.
func Pulses(beats []Beat, names []string, bars int) []Pulse {
	out := make([]Pulse, len(names))
	for i, n := range names {
		out[i] = Pulse{Part: i, Name: n, Bars: make([]int, bars)}
	}
	for _, b := range beats {
		if b.Part < 0 || b.Part >= len(out) || b.Unit <= 0 {
			continue
		}
		p := &out[b.Part]
		p.Notes++
		switch {
		case near(b.At, 0):
			p.Downbeats++
		case onBeat(b.At, b.Unit):
			p.OnBeat++
		default:
			p.OffBeat++
			if holdsOver(b) {
				p.Syncopated++
				if b.Bar >= 0 && b.Bar < len(p.Bars) {
					p.Bars[b.Bar]++
				}
			}
		}
	}
	return out
}

// near says two positions are the same, allowing for the rounding that
// comes of adding thirds of a beat together.
func near(a, b float64) bool {
	d := a - b
	return d > -1e-9 && d < 1e-9
}

// onBeat says a position falls on one of the bar's beats.
func onBeat(at, unit float64) bool {
	n := at / unit
	return near(n, float64(int(n+0.5)))
}

// holdsOver says a note starting off the beat is still sounding when the
// next beat arrives, which is what makes it syncopation rather than a note
// that merely falls between beats.
func holdsOver(b Beat) bool {
	next := (float64(int(b.At/b.Unit)) + 1) * b.Unit
	return b.At+b.Held > next+1e-9
}

// Odd is one bar holding more or less than its metre says.
type Odd struct {
	Bar int `json:"bar"`
	// Held is what the bar's longest voice holds and Says what the metre
	// says, both in whole notes.
	Held float64 `json:"held"`
	Says float64 `json:"says"`
	Text string  `json:"text"`
}

// Hurried is a stretch written faster than it can be articulated.
type Hurried struct {
	Part    int    `json:"part"`
	Name    string `json:"name"`
	FromBar int    `json:"fromBar"`
	ToBar   int    `json:"toBar"`
	Notes   int    `json:"notes"`
	// Each is how long one of them lasts, in seconds, and Limit the
	// shortest the instrument can articulate.
	Each  float64 `json:"each"`
	Limit float64 `json:"limit"`
	Text  string  `json:"text"`
	Ref   int     `json:"ref"`
}

// LeastHurried is how many notes in a row have to be too short before it is
// worth saying so.
//
// Four. One very short note is an ornament, a grace note or the tail of a
// turn, and every one of those is played by leaning on it rather than by
// counting it. Four in a row is a passage, and a passage has to be played
// as written.
const LeastHurried = 4

// Rushed finds where a line is written faster than it can be played.
//
// seconds says how long each note lasts, which is a question for the tempo
// and not for the notation: the same thirty-second notes are comfortable at
// sixty and impossible at a hundred and eighty. limit is the shortest note
// the instrument can articulate, which is why it is asked for per staff.
func Rushed(beats []Beat, names []string, seconds func(Beat) float64,
	limit func(part int) (float64, bool), name func(midi int) string) []Hurried {

	_ = name
	sort.SliceStable(beats, func(i, j int) bool {
		if beats[i].Part != beats[j].Part {
			return beats[i].Part < beats[j].Part
		}
		if beats[i].Bar != beats[j].Bar {
			return beats[i].Bar < beats[j].Bar
		}
		return beats[i].At < beats[j].At
	})

	var out []Hurried
	i := 0
	for i < len(beats) {
		part := beats[i].Part
		least, ok := limit(part)
		if !ok {
			for i < len(beats) && beats[i].Part == part {
				i++
			}
			continue
		}
		j := i
		for j < len(beats) && beats[j].Part == part && seconds(beats[j]) < least {
			j++
		}
		if j-i >= LeastHurried {
			run := beats[i:j]
			worst := seconds(run[0])
			for _, b := range run {
				if s := seconds(b); s < worst {
					worst = s
				}
			}
			out = append(out, Hurried{
				Part: part, Name: names[part],
				FromBar: run[0].Bar, ToBar: run[len(run)-1].Bar,
				Notes: len(run), Each: worst, Limit: least, Ref: run[0].Ref,
			})
			i = j
			continue
		}
		if j == i {
			i++
			continue
		}
		i = j
	}
	return out
}
