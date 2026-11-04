/*
 * notecolour.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import "github.com/stricaud/gscore/internal/model"

// Colouring the noteheads by what note they are.
//
// Reading a staff is two things at once: working out which line or space a
// head is on, and remembering what that line or space is called. The second
// is the one that takes months, and until it comes, every note is counted up
// from a landmark — which is why a beginner reads at the speed they count.
// A colour per note name answers it directly: the head is red, so it is a C,
// and the counting goes away.
//
// It is the oldest trick in music teaching and it has a real cost, which is
// why this is off by default and is a view option rather than a property of
// the score: a reader who learns the colours and not the staff has learned
// the wrong thing, and the colours have to come off again. gscore's part in
// that is to make them easy to take away, not to decide when.
//
// By the letter, not by the pitch. C sharp is a C — it sits on the C line and
// it is called C — so it takes C's colour, which is what makes the colour
// stand for the thing the reader is trying to learn. Octave does not come
// into it either: every C in the piece is the same colour, which is most of
// the point.

// NoteColours is a colour per diatonic step, indexed by model.Step, as CSS
// hex. An empty entry leaves that note in the ordinary ink.
type NoteColours [7]string

// DefaultNoteColours is the palette the colours start at.
//
// The order is the one every coloured-notes scheme uses, because it is the
// spectrum laid along the scale: red at C and round to violet at B. The
// teaching materials a beginner is likely to meet — coloured bells, tuned
// percussion tubes, the stickers that go on a keyboard — all follow it, so a
// score coloured this way matches the instrument in front of them. That
// agreement is worth more than any nicer set of colours would be.
//
// They are darkened from the pure spectrum. Printed on white, a true yellow
// notehead is a faint smudge and a true cyan is not much better, and a
// notehead that cannot be seen is worse than one that is merely black. Each
// is dark enough to read against the staff and far enough from its
// neighbours to be told apart at the size a notehead is actually printed.
var DefaultNoteColours = NoteColours{
	"#d0312d", // C: red
	"#e8701a", // D: orange
	"#c9a227", // E: amber, not yellow — yellow on white is not a notehead
	"#3f9142", // F: green
	"#2a72b5", // G: blue
	"#4b3f8f", // A: indigo
	"#9c3f8f", // B: violet
}

// Of returns the colour a note is drawn in, or empty for the ordinary ink.
func (c NoteColours) Of(s model.Step) string {
	if s < model.StepC || s > model.StepB {
		return ""
	}
	return c[s]
}

// Empty says no colour at all was given, which is the ordinary score.
func (c NoteColours) Empty() bool {
	for _, s := range c {
		if s != "" {
			return false
		}
	}
	return true
}

// noteColour is the colour this engraving draws a note in.
//
// Off unless asked for, and off for anything that is not a pitch: a rest has
// no letter and a note on a drum staff is a sound rather than a pitch, so
// neither is coloured — colouring the snare line red because its head sits
// where a C would be is saying something untrue about it.
func (e *engraver) noteColour(n *model.Note) string {
	if n == nil || !e.opt.ColourNotes || e.tab != nil {
		return ""
	}
	if e.unpitched {
		return ""
	}
	// Turning the option on is enough on its own: somebody who has never
	// opened the colour picker still gets the palette.
	if e.opt.NoteColours.Empty() {
		return DefaultNoteColours.Of(n.Pitch.Step)
	}
	return e.opt.NoteColours.Of(n.Pitch.Step)
}
