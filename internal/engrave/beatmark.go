/*
 * beatmark.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// The beat marks: an arrow over every beat of the music.
//
// A score says how fast it goes once, at the top, and then never again. That
// is enough for a player who already hears the pulse and no help at all to
// somebody who does not: a tied-over downbeat, a bar of 7/8, a line of
// syncopation across a bar — the question is not how fast the beats are but
// where they are, and the page does not answer it. A row of marks over the
// staff does, and it is the same answer the metronome is ticking: both are
// counted in model.TimeSignature.BeatUnit, so what is seen and what is heard
// cannot drift apart.
//
// Not notation. It is drawn in a colour of its own and it is off unless asked
// for, because a reader who found these printed in black would look for what
// the composer meant by them.

// beatMarkGlyph is the arrow drawn over a beat.
//
// A filled arrow pointing down at the music, which is the shape a conductor's
// beat is drawn as and the one thing on the page that could not be mistaken
// for a mark of notation: nothing in SMuFL's musical repertoire points
// straight down at a note from above.
const beatMarkGlyph = "arrowBlackDown"

// beatMarkSize is the type size the arrow is drawn at, in staff spaces.
//
// Smaller than the four spaces a notehead is set at. The row is read out of
// the corner of the eye while the notes are read properly, and an arrow as
// big as the music competes with it.
const beatMarkSize = 2.6

// beatMarkColour is what the row is drawn in: a muted blue, dark enough to
// read on white and far enough from black that nobody takes it for ink.
const beatMarkColour = "#6f9fd8"

// beatMarkGap is the air between the top of the music and the foot of the
// arrows, in staff spaces.
const beatMarkGap = 0.6

// beatMarkHeight is how tall the arrow is, for the room the row books in the
// outline. The glyph is about four fifths of its type size.
const beatMarkHeight = beatMarkSize * 0.8

// beatMarkHalf is half the arrow's width, to centre it on its beat.
//
// A glyph is advanced from its left edge and the font measures that advance
// at SMuFL's standard four spaces to the em, so it has to be scaled to the
// size the arrow is actually drawn at before half of it means anything.
func beatMarkHalf() float64 {
	return smufl.Advance(beatMarkGlyph) * (beatMarkSize / 4) / 2
}

// beatAnchor is a moment in a bar and the x the drawing put it at.
type beatAnchor struct {
	at model.Duration
	x  float64
}

// noteBeats works out where the beats of one bar fell and keeps them for the
// row drawn over the staff.
//
// anchors are the columns of the bar, in order, each with the moment it
// begins and the x its noteheads were drawn at; end is where the bar's music
// finished. A beat that is a column of its own lands exactly on it, which is
// the whole point — an arrow a little to the left of the note it is pointing
// at is an arrow pointing at the wrong note. A beat with nothing written on
// it falls between two columns and is placed proportionally between them,
// which is as good an answer as there is: there is no note there to be right
// about.
func (e *engraver) noteBeats(part *model.Part, b measureBox, anchors []beatAnchor, start, end float64) {
	if !e.collectBeats {
		return
	}
	unit := part.TimeAt(b.index).BeatUnit()
	if unit.IsZero() {
		return
	}
	// How much of the bar there is to beat through.
	//
	// What is written in it, not what the meter says should be: a pickup is
	// a bar of two beats however the signature reads, and a bar half typed
	// in the editor has beats as far as it has notes. Marking the beats of a
	// bar that is not there yet puts arrows over empty paper.
	filled := barFilled(b)
	if filled.IsZero() {
		return
	}

	// The columns, with the end of the music closing the last gap, so that a
	// beat after the final notehead has something to be placed between.
	points := make([]beatAnchor, 0, len(anchors)+2)
	if len(anchors) == 0 || anchors[0].at.Cmp(model.Duration{Num: 0, Den: 1}) > 0 {
		points = append(points, beatAnchor{at: model.Duration{Num: 0, Den: 1}, x: start})
	}
	points = append(points, anchors...)
	points = append(points, beatAnchor{at: filled, x: end})

	for n := 0; ; n++ {
		at := unit.Scale(n, 1)
		if at.Cmp(filled) >= 0 {
			break
		}
		e.beatMarks = append(e.beatMarks, beatX(points, at))
	}
}

// barFilled is how much music the bar actually holds, across its voices.
//
// The longest voice, as everywhere else: a bar is as long as the most that
// was written into it, and a second voice that stops early stops early.
func barFilled(b measureBox) model.Duration {
	longest := model.Duration{Num: 0, Den: 1}
	if b.m == nil {
		return longest
	}
	for i := range b.m.Voices {
		if d := b.m.Voices[i].Duration(); d.Cmp(longest) > 0 {
			longest = d
		}
	}
	return longest
}

// beatX places a moment along a bar that has been drawn.
//
// Between the two columns it falls between, in proportion to the time
// between them. Linear in time and not in the bar: the spacing of a bar is
// not proportional to its durations — a half note is nothing like twice the
// width of a quarter — so a beat placed at its fraction of the bar's width
// would miss every note it was supposed to be over.
func beatX(points []beatAnchor, at model.Duration) float64 {
	if len(points) == 0 {
		return 0
	}
	for i := 0; i < len(points); i++ {
		switch c := points[i].at.Cmp(at); {
		case c == 0:
			// The beat is a column of its own: the note is there, and the
			// arrow goes exactly over it.
			return points[i].x
		case c > 0:
			if i == 0 {
				return points[0].x
			}
			lo, hi := points[i-1], points[i]
			span := hi.at.Sub(lo.at).Float()
			if span <= 0 {
				return lo.x
			}
			return lo.x + (hi.x-lo.x)*(at.Sub(lo.at).Float()/span)
		}
	}
	return points[len(points)-1].x
}

// drawBeatMarks lays the collected beats out as one row over the staff.
//
// One row and not one mark each. Placed separately, every arrow would find
// its own height against the music under it, and a run climbing over the
// staff would leave the row stepping up and down the page after it — which
// reads as a mistake rather than as a rule. The pulse is the one steady thing
// in the music and it is drawn as the one steady thing on the page, so the
// whole row clears the whole system and the marks stay level with each other.
//
// Queued rather than drawn, because what it has to clear is not known until
// the staff is finished: it is booked across the system's full width so that
// the bar numbers and everything above them clear the row in turn.
func (e *engraver) drawBeatMarks(x0, x1 float64) {
	marks := e.beatMarks
	e.beatMarks = nil
	if !e.opt.ShowBeatMarks || len(marks) == 0 {
		return
	}
	e.wait(waitingMark{
		x0: x0, x1: x1, height: beatMarkHeight, gap: beatMarkGap,
		rank: rankBeatMark,
		draw: func(y float64) {
			// Centred on the beat. The arrow is advanced from its left edge
			// like every other glyph, and a row of them hung from their left
			// edges points half an arrow to the right of everything it means.
			half := beatMarkHalf()
			for _, x := range marks {
				e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
					Name:   beatMarkGlyph,
					X:      x - half,
					Y:      y,
					Size:   beatMarkSize,
					Colour: beatMarkColour,
				})
			}
		},
	})
}
