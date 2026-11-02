/*
 * knobs.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"

	"github.com/stricaud/gscore/internal/model"
)

// The measurements a score may set for itself.
//
// Spacings and smufl.EngravingDefaults are the two tables the whole of this
// package reads its numbers from, and until now there was no way to change
// one except by editing Go. They are exactly what an engraver argues about
// — how wide a quarter note's slot is, how heavy a beam is, how far apart
// two staves sit — and the right place to settle them is per score, because
// a song and an orchestral part want different answers.
//
// One list, here, and three things read it: the dialog builds itself from
// it, the endpoint validates against it, and the layout applies it. A
// setting added here shows up in all three without anything else being
// touched, which is the only way a list of fifty numbers stays in step with
// a window that shows them.
//
// Each knob reaches into Options through a pair of closures rather than
// naming a field by a string. That is what makes the compiler check them: a
// field renamed in Spacings breaks this file, where a map of field names
// would go on compiling and quietly stop working.

// Knob is one measurement a score may set, and how to read and write it.
type Knob struct {
	// Name is what a file and the endpoint call it. It is written into
	// saved scores, so it never changes once it has shipped.
	Name string
	// Label is what the dialog shows, and Group is the heading it sits
	// under.
	Label string
	Group string
	// Min and Max are the range this measurement is usually in: what the
	// window's arrows step through and what it says in the tooltip, so that
	// somebody setting a beam thickness is told that most scores are
	// between 0.2 and 0.9 of a space.
	//
	// Advice, not a wall. They were a wall, and that was wrong: a house
	// style is somebody's taste and the program has no business telling
	// them their taste is out of range. A number outside this is accepted
	// and drawn — see Clamp, which refuses only what cannot be drawn at
	// all.
	Min, Max float64
	// Unit is what the number is in, for the dialog to show after the
	// field. Nearly everything here is in staff spaces.
	Unit string

	get func(*Options) float64
	set func(*Options, float64)
}

// Knobs is every measurement a score may set, in the order a dialog shows
// them.
var Knobs = []Knob{
	// Notes and rests
	{"noteBase", "Width of a quarter note", "Notes and rests", 1.2, 8, "spaces",
		func(o *Options) float64 { return o.Spacings.NoteBase },
		func(o *Options, v float64) { o.Spacings.NoteBase = v }},
	{"noteExponent", "How width follows duration", "Notes and rests", 0.3, 1, "",
		func(o *Options) float64 { return o.Spacings.NoteExponent },
		func(o *Options, v float64) { o.Spacings.NoteExponent = v }},
	{"minNote", "Narrowest slot", "Notes and rests", 1.2, 6, "spaces",
		func(o *Options) float64 { return o.Spacings.MinNote },
		func(o *Options, v float64) { o.Spacings.MinNote = v }},

	// Accidentals
	{"beforeAccidental", "Space before an accidental", "Accidentals", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.BeforeAccidental },
		func(o *Options, v float64) { o.Spacings.BeforeAccidental = v }},
	{"accidentalToNote", "Space between accidental and note", "Accidentals", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.AccidentalToNote },
		func(o *Options, v float64) { o.Spacings.AccidentalToNote = v }},
	{"betweenAccidentals", "Space between accidentals on a note", "Accidentals", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.BetweenAccidentals },
		func(o *Options, v float64) { o.Spacings.BetweenAccidentals = v }},

	// Augmentation dots
	{"dotToNote", "Space between dot and note", "Augmentation dots", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.DotToNote },
		func(o *Options, v float64) { o.Spacings.DotToNote = v }},
	{"betweenDots", "Space between dots", "Augmentation dots", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.BetweenDots },
		func(o *Options, v float64) { o.Spacings.BetweenDots = v }},

	// Clefs
	{"clefBefore", "Space before", "Clefs", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.ClefBefore },
		func(o *Options, v float64) { o.Spacings.ClefBefore = v }},
	{"clefAfter", "Space after", "Clefs", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.ClefAfter },
		func(o *Options, v float64) { o.Spacings.ClefAfter = v }},

	// Key signatures
	{"betweenKeyAccidentals", "Space between key signature accidentals", "Key signatures", 0, 2, "spaces",
		func(o *Options) float64 { return o.Spacings.BetweenKeyAccidentals },
		func(o *Options, v float64) { o.Spacings.BetweenKeyAccidentals = v }},
	{"afterKeyCancel", "Space between a cancelled key and the new one", "Key signatures", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.AfterKeyCancel },
		func(o *Options, v float64) { o.Spacings.AfterKeyCancel = v }},
	{"afterKeySignature", "Space after key signature", "Key signatures", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.AfterKeySignature },
		func(o *Options, v float64) { o.Spacings.AfterKeySignature = v }},

	// Time signatures
	{"afterTimeSignature", "Space after time signature", "Time signatures", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.AfterTimeSignature },
		func(o *Options, v float64) { o.Spacings.AfterTimeSignature = v }},

	// Barlines
	{"thinBarlineThickness", "Thin line thickness", "Barlines", 0.06, 0.5, "spaces",
		func(o *Options) float64 { return o.Defaults.ThinBarlineThickness },
		func(o *Options, v float64) { o.Defaults.ThinBarlineThickness = v }},
	{"thickBarlineThickness", "Heavy line thickness", "Barlines", 0.2, 1.2, "spaces",
		func(o *Options) float64 { return o.Defaults.ThickBarlineThickness },
		func(o *Options, v float64) { o.Defaults.ThickBarlineThickness = v }},
	{"barlineSeparation", "Space within double barlines", "Barlines", 0.1, 1.5, "spaces",
		func(o *Options) float64 { return o.Defaults.BarlineSeparation },
		func(o *Options, v float64) { o.Defaults.BarlineSeparation = v }},
	{"afterBarline", "Space after barline", "Barlines", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.AfterBarline },
		func(o *Options, v float64) { o.Spacings.AfterBarline = v }},
	{"betweenMeasures", "Space left between measures", "Barlines", 0, 3, "spaces",
		func(o *Options) float64 { return o.Spacings.BetweenMeasures },
		func(o *Options, v float64) { o.Spacings.BetweenMeasures = v }},

	// Beams
	{"beamThickness", "Beam thickness", "Beams", 0.2, 0.9, "spaces",
		func(o *Options) float64 { return o.Defaults.BeamThickness },
		func(o *Options, v float64) { o.Defaults.BeamThickness = v }},
	{"beamSpacing", "Space between beams", "Beams", 0.1, 0.8, "spaces",
		func(o *Options) float64 { return o.Defaults.BeamSpacing },
		func(o *Options, v float64) { o.Defaults.BeamSpacing = v }},

	// Stems and ledger lines
	{"stemThickness", "Stem thickness", "Stems and ledger lines", 0.04, 0.4, "spaces",
		func(o *Options) float64 { return o.Defaults.StemThickness },
		func(o *Options, v float64) { o.Defaults.StemThickness = v }},
	{"legerLineThickness", "Ledger line thickness", "Stems and ledger lines", 0.04, 0.4, "spaces",
		func(o *Options) float64 { return o.Defaults.LegerLineThickness },
		func(o *Options, v float64) { o.Defaults.LegerLineThickness = v }},
	{"legerLineExtension", "Ledger line overhang", "Stems and ledger lines", 0.1, 1.2, "spaces",
		func(o *Options) float64 { return o.Defaults.LegerLineExtension },
		func(o *Options, v float64) { o.Defaults.LegerLineExtension = v }},

	// Slurs and ties
	{"slurMidpointThickness", "Slur thickness at its middle", "Slurs and ties", 0.08, 0.6, "spaces",
		func(o *Options) float64 { return o.Defaults.SlurMidpointThickness },
		func(o *Options, v float64) { o.Defaults.SlurMidpointThickness = v }},
	{"tieMidpointThickness", "Tie thickness at its middle", "Slurs and ties", 0.08, 0.6, "spaces",
		func(o *Options) float64 { return o.Defaults.TieMidpointThickness },
		func(o *Options, v float64) { o.Defaults.TieMidpointThickness = v }},

	// Lines
	{"staffLineThickness", "Staff line thickness", "Lines", 0.04, 0.4, "spaces",
		func(o *Options) float64 { return o.Defaults.StaffLineThickness },
		func(o *Options, v float64) { o.Defaults.StaffLineThickness = v }},
	{"hairpinThickness", "Hairpin thickness", "Lines", 0.06, 0.5, "spaces",
		func(o *Options) float64 { return o.Defaults.HairpinThickness },
		func(o *Options, v float64) { o.Defaults.HairpinThickness = v }},
	{"octaveLineThickness", "Octave line thickness", "Lines", 0.06, 0.5, "spaces",
		func(o *Options) float64 { return o.Defaults.OctaveLineThickness },
		func(o *Options, v float64) { o.Defaults.OctaveLineThickness = v }},
	{"tupletBracketThickness", "Tuplet bracket thickness", "Lines", 0.06, 0.5, "spaces",
		func(o *Options) float64 { return o.Defaults.TupletBracketThickness },
		func(o *Options, v float64) { o.Defaults.TupletBracketThickness = v }},

	// The page
	{"staffGap", "Space between staves", "The page", 3, 20, "spaces",
		func(o *Options) float64 { return o.Spacings.StaffGap },
		func(o *Options, v float64) { o.Spacings.StaffGap = v }},
	{"systemGap", "Space between systems", "The page", 2, 24, "spaces",
		func(o *Options) float64 { return o.Spacings.SystemGap },
		func(o *Options, v float64) { o.Spacings.SystemGap = v }},
	{"margin", "Margin at the sides", "The page", 0, 20, "spaces",
		func(o *Options) float64 { return o.Spacings.Margin },
		func(o *Options, v float64) { o.Spacings.Margin = v }},
	{"marginY", "Margin top and bottom", "The page", 0, 20, "spaces",
		func(o *Options) float64 { return o.Spacings.MarginY },
		func(o *Options, v float64) { o.Spacings.MarginY = v }},
}

// KnobNamed finds one by the name a file or the endpoint calls it.
func KnobNamed(name string) (Knob, bool) {
	for _, k := range Knobs {
		if k.Name == name {
			return k, true
		}
	}
	return Knob{}, false
}

// Get reads this knob out of a set of options.
func (k Knob) Get(o Options) float64 { return k.get(&o) }

// Default is what this knob is before a score has said otherwise.
func (k Knob) Default() float64 {
	o := DefaultOptions()
	return k.get(&o)
}

// Limits are the numbers this measurement may actually take: not what is
// usual, but what can be drawn at all.
//
// Nothing here is a matter of taste. A negative space is not a small space,
// it is shapes drawn through each other backwards; and a bar some thousands
// of staff spaces wide is a drawing nothing can show, which on a long score
// is megabytes of shapes nobody asked for. Everything between those two is
// somebody's house style and none of the program's business, however odd it
// looks — see Min and Max, which are what is usual and are only advice.
func (k Knob) Limits() (float64, float64) {
	// The one measurement that is not a length: how width follows
	// duration, which is an exponent. Past four a whole note is hundreds of
	// times a sixteenth and the bar cannot be laid out.
	if k.Unit == "" {
		return 0, 4
	}
	return 0, 200
}

// Clamp brings a number inside what can be drawn, leaving taste alone.
func (k Knob) Clamp(v float64) float64 {
	lo, hi := k.Limits()
	// A number that is not a number at all — the field was emptied, or
	// something unparseable arrived — is not clamped to a bound, because
	// there is nothing to clamp. The caller reads NaN as "go back to the
	// house style for this one".
	if math.IsNaN(v) {
		return v
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// withScoreSettings is opt with the score's own measurements applied.
//
// Called by Engrave rather than by everything that engraves. There are a
// dozen places that build Options — the editor, the printer, each exporter,
// the little pictures in the pickers — and a setting that only reached some
// of them would be a score that prints differently from the way it looks,
// which is the one thing a page of music must not do.
//
// A name this version does not know is skipped rather than refused: a file
// written by a later gscore opens here, with the settings this one
// understands and the house style for the rest.
func withScoreSettings(s *model.Score, opt Options) Options {
	if s == nil || len(s.Engraving) == 0 {
		return opt
	}
	for _, k := range Knobs {
		if v, ok := s.Engraving[k.Name]; ok {
			k.set(&opt, k.Clamp(v))
		}
	}
	return opt
}

// Settings is what a score has set, with everything it has not set left
// out.
//
// Only the differences, because that is what the file keeps: a score that
// wrote down every measurement would freeze the day's defaults into itself
// and never see an improvement to them again.
func Settings(s *model.Score) map[string]float64 {
	out := map[string]float64{}
	if s == nil {
		return out
	}
	for _, k := range Knobs {
		if v, ok := s.Engraving[k.Name]; ok && v != k.Default() {
			out[k.Name] = k.Clamp(v)
		}
	}
	return out
}
