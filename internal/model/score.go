/*
 * score.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package model holds the musical content of a score and nothing else.
//
// Nothing here knows about pixels, widgets, fonts or files. The 0.0.9 Score_t
// carried width, height, staff_extremity_end_x, staff_startx and a window
// title alongside the music; all of that was display state, and its absence
// here is the point of the package. Engraving reads this model and produces
// geometry separately, which is what makes both halves testable.
package model

import "fmt"

// Clef fixes which pitch each staff line carries.
type Clef int

// The clefs legacy/src/staff.c supported.
const (
	Treble Clef = iota
	Bass
	Alto
	Tenor
	Percussion
)

var clefNames = [...]string{"treble", "bass", "alto", "tenor", "percussion"}

func (c Clef) String() string {
	if c < Treble || int(c) >= len(clefNames) {
		return "unknown"
	}
	return clefNames[c]
}

// Mode is the scale a key signature is understood in. Two keys can share a
// signature and differ in mode, so the mode is kept rather than inferred.
type Mode int

// The church modes, with Major and Minor as the usual names for two of them.
const (
	Major Mode = iota
	Minor
	Dorian
	Phrygian
	Lydian
	Mixolydian
	Locrian
)

var modeNames = [...]string{
	"major", "minor", "dorian", "phrygian", "lydian", "mixolydian", "locrian",
}

func (m Mode) String() string {
	if m < Major || int(m) >= len(modeNames) {
		return "unknown"
	}
	return modeNames[m]
}

// KeySignature is held as a position on the circle of fifths: 0 is C, positive
// counts sharps and negative counts flats, so the range is -7 to 7.
type KeySignature struct {
	Fifths int
	Mode   Mode
}

// sharpOrder and flatOrder give the steps a signature alters, in the order the
// accidentals are written on the staff.
var (
	sharpOrder = [7]Step{StepF, StepC, StepG, StepD, StepA, StepE, StepB}
	flatOrder  = [7]Step{StepB, StepE, StepA, StepD, StepG, StepC, StepF}
)

// AlterFor returns the alteration the signature applies to a step, in
// semitones. This is what turns a written note into a sounding pitch.
func (k KeySignature) AlterFor(s Step) int {
	switch {
	case k.Fifths > 0:
		for i := 0; i < k.Fifths && i < len(sharpOrder); i++ {
			if sharpOrder[i] == s {
				return 1
			}
		}
	case k.Fifths < 0:
		for i := 0; i < -k.Fifths && i < len(flatOrder); i++ {
			if flatOrder[i] == s {
				return -1
			}
		}
	}
	return 0
}

// TimeSymbol distinguishes a time signature printed as digits from the two
// that have their own glyph.
type TimeSymbol int

// How a time signature is drawn.
const (
	TimeNumeric TimeSymbol = iota
	TimeCommon             // the C glyph, for 4/4
	TimeCut                // the slashed C, for 2/2
)

// TimeSignature is the meter. Beats over BeatValue, as in 6 over 8.
//
// The 0.0.9 Staff_t declared guint8 time_signature[2] and then wrote to index
// 2 in staff.c, one past the end; it never crashed only because struct padding
// happened to absorb the byte. Named fields make that class of bug
// unrepresentable.
type TimeSignature struct {
	Beats     int
	BeatValue int
	Symbol    TimeSymbol
}

// BarDuration is how much time a full measure of this meter holds.
func (t TimeSignature) BarDuration() Duration {
	if t.BeatValue == 0 {
		return Duration{0, 1}
	}
	return Duration{t.Beats, t.BeatValue}.Reduced()
}

func (t TimeSignature) String() string { return fmt.Sprintf("%d/%d", t.Beats, t.BeatValue) }

// BarlineStyle is the kind of barline closing a measure.
type BarlineStyle int

// The barline styles the ABC samples use, plus repeats.
const (
	BarSingle BarlineStyle = iota
	BarDouble
	BarFinal
	BarRepeatStart
	BarRepeatEnd
	BarRepeatBoth
	BarNone
)

var barlineNames = [...]string{
	"single", "double", "final", "repeat-start", "repeat-end", "repeat-both", "none",
}

func (b BarlineStyle) String() string {
	if b < BarSingle || int(b) >= len(barlineNames) {
		return "unknown"
	}
	return barlineNames[b]
}

// Tempo is a metronome mark: Unit notes per minute, optionally with a text
// indication such as "Allegro" beside it.
type Tempo struct {
	BPM  int
	Unit Duration
	Text string
}

// Identity is the bibliographic header of a score, from the 0.0.9 Identity_t.
type Identity struct {
	Title     string
	Subtitle  string
	Composer  string
	Copyright string
}

// Voice is one monophonic line of music within a measure.
//
// A measure holds a slice of these so that two independent lines can share a
// staff. The 0.0.9 model had no concept of a voice, which is why its own ABC
// sample, carrying V:0 and V:1, could not round-trip.
type Voice struct {
	Elements []Element
}

// Duration totals the voice's elements.
func (v *Voice) Duration() Duration {
	total := Duration{0, 1}
	for _, e := range v.Elements {
		total = total.Add(e.Duration())
	}
	return total
}

// Measure is one bar of one part.
type Measure struct {
	Voices []*Voice

	// RightBarline closes the measure. The opening barline of a measure is the
	// closing barline of the one before it, so only one is stored.
	RightBarline BarlineStyle
}

// Part is one instrumental line, drawn on its own staff.
type Part struct {
	ID   string
	Name string
	// Instrument is what this staff sounds as, carried over from
	// Staff_t.midi_instrument.
	Instrument Instrument
	Clef       Clef
	Key        KeySignature
	Time       TimeSignature
	Measures   []*Measure
}

// Score is a complete piece of music.
type Score struct {
	Identity Identity
	Tempo    Tempo
	Parts    []*Part
}

// NewScore returns the score a new document starts from: one treble part in C
// major and common time, holding a single empty measure.
//
// The empty measure is deliberate and follows the C program, where
// create_staff set both measure_number and total_measures to 1. A score with
// no measures at all has nowhere to put the first note, so there would be
// nothing to draw and nothing to click.
func NewScore() *Score {
	return &Score{
		Tempo: Tempo{BPM: 120, Unit: Duration{1, 4}},
		Parts: []*Part{{
			Clef:     Treble,
			Key:      KeySignature{Fifths: 0, Mode: Major},
			Time:     TimeSignature{Beats: 4, BeatValue: 4, Symbol: TimeNumeric},
			Measures: []*Measure{NewMeasure()},
		}},
	}
}

// NewMeasure returns an empty measure holding one empty voice.
func NewMeasure() *Measure {
	return &Measure{
		Voices:       []*Voice{{}},
		RightBarline: BarSingle,
	}
}

// AppendMeasure adds an empty measure to the part and returns it.
func (p *Part) AppendMeasure() *Measure {
	m := NewMeasure()
	p.Measures = append(p.Measures, m)
	return m
}
