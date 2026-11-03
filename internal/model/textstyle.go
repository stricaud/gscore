/*
 * textstyle.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// The kinds of writing on a page of music.
//
// Words on a score are not one thing with a font picked for each of them.
// They fall into a handful of kinds, and the kind decides everything about
// how they are set — "dolce" is italic and under the staff, "pizz." is
// upright and over it, "Allegro" is bold and over the whole system — and
// those are conventions a reader relies on without noticing. A player
// scanning a part knows what a word is before reading it, from where it
// sits and what it is set in.
//
// So the style is what is stored, and the appearance follows from it. Set
// each piece of text in whatever font and size looked right at the time
// and a score comes out with eleven kinds of writing on it and no two
// "rit."s alike; and nothing can be changed afterwards except by finding
// every one of them.
//
// The other half of the difference is who the words are for. An
// expression mark is for the player reading that staff; a tempo marking
// is for everyone, is written once over the system, and belongs in every
// part. That is not a question about typography and cannot be answered by
// one, which is why it is part of the style rather than a separate switch.

// TextStyle is the kind of writing a text mark is.
type TextStyle int

const (
	// TextExpression is how to play it: "dolce", "cantabile", "espr."
	// Italic and under the staff, where the dynamics are, because it is
	// the same kind of instruction as a dynamic.
	TextExpression TextStyle = iota

	// TextTechnique is what to do: "pizz.", "arco", "con sord.", "a2",
	// "div.". Upright and over the staff, because it is an instruction
	// about the instrument rather than about the feeling, and because it
	// has to be seen before the note it applies to is played.
	TextTechnique

	// TextTempo is the speed: "Allegro", "rit.", "a tempo". Bold and over
	// the system — everyone changes speed together, so it is written once
	// for the whole score and goes into every part.
	TextTempo

	// TextDirection is where to go and what the piece is doing: "To Coda",
	// "Solo", "Repeat ad lib." Upright over the system, for the same
	// reason as the tempo and without the weight, which is kept for the
	// speed so the two are told apart at a glance.
	TextDirection

	// TextPlain is a word with no particular kind: a note to the player, a
	// dedication, whatever somebody wanted written there. Upright over the
	// staff and left alone.
	TextPlain
)

var textStyleNames = [...]string{
	"expression", "technique", "tempo", "direction", "plain",
}

// textStyleLabels are what the styles are called where somebody is
// choosing one, with what they are for, because the names alone are a
// vocabulary not everybody has.
var textStyleLabels = [...]string{
	"Expression — dolce, cantabile",
	"Technique — pizz., con sord., a2",
	"Tempo — Allegro, rit., a tempo",
	"Direction — To Coda, Solo",
	"Plain words",
}

func (t TextStyle) String() string {
	if t < TextExpression || int(t) >= len(textStyleNames) {
		return "plain"
	}
	return textStyleNames[t]
}

// Label is what the style is called where one is being chosen.
func (t TextStyle) Label() string {
	if t < TextExpression || int(t) >= len(textStyleLabels) {
		return "Plain words"
	}
	return textStyleLabels[t]
}

// TextStyleFromName is the inverse, for files and query strings.
//
// An unknown name is plain rather than an error: a score written by a
// later gscore should open with its words on the page, set plainly,
// rather than not open.
func TextStyleFromName(s string) (TextStyle, bool) {
	for i, n := range textStyleNames {
		if n == s {
			return TextStyle(i), true
		}
	}
	return TextPlain, false
}

// TextStyles is every style there is, in the order they are offered.
func TextStyles() []TextStyle {
	out := make([]TextStyle, len(textStyleNames))
	for i := range out {
		out[i] = TextStyle(i)
	}
	return out
}

// TextLook is how a style is set: everything about the words that is
// decided by what kind of writing they are.
type TextLook struct {
	// Size is the type size in staff spaces, which is how every other
	// measurement on the page is quoted.
	Size float64
	// Italic and Bold are the slant and the weight.
	Italic, Bold bool
	// Place is where it goes when the mark itself does not say.
	Place Placement
	// System says the words are for everybody: written once over the top
	// staff of the system rather than over the staff they are attached
	// to, and belonging in every part rather than in one.
	System bool
}

// Look is how this style is set.
//
// The sizes are in staff spaces and are close to what the handbooks give:
// an expression or technique marking a little under two spaces, a tempo
// marking larger because it is read from further away and from across a
// desk of other parts.
func (t TextStyle) Look() TextLook {
	switch t {
	case TextExpression:
		return TextLook{Size: 1.9, Italic: true, Place: PlacementBelow}
	case TextTechnique:
		return TextLook{Size: 1.9, Place: PlacementAbove}
	case TextTempo:
		return TextLook{Size: 2.3, Bold: true, Place: PlacementAbove, System: true}
	case TextDirection:
		return TextLook{Size: 2.0, Place: PlacementAbove, System: true}
	}
	return TextLook{Size: 1.9, Place: PlacementAbove}
}
