/*
 * cue.go
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

/*
 * Notes printed smaller than the music.
 *
 * A cue: a few bars of somebody else's line, printed in a part during a long
 * rest so the player can hear their entry coming. It is the one thing on a
 * part that is read and not played, and the smaller size is how the page
 * says so — a cue at full size is a cue somebody plays.
 *
 * What changes and what does not. The size changes: the notehead, the stem,
 * the flag, the beams, the ledger lines, the dots and the accidental all
 * come out smaller. Where things are does not. A note two ledger lines over
 * the staff is still two ledger lines over the staff, because that is which
 * note it is — scaling the position as well as the size would move it to a
 * different pitch, which is the mistake that makes this worth writing down
 * rather than doing with a transform.
 *
 * So the scale multiplies measurements and never coordinates. It is held on
 * the engraver rather than passed down, because a cue is whole bars at a
 * time and every size in the drawing of one is scaled by the same amount:
 * threading it through thirty functions as an argument would be thirty
 * chances to forget it, and one of them forgotten is a full-size stem on a
 * small notehead.
 */

// cueScale is how big a cue note is against a note to be played.
//
// Larger than a grace note, which is 0.6: a grace note is an ornament read
// in passing and a cue is read as music — it has to be followable at a
// glance while counting, and the convention in printed parts is around
// three quarters.
const cueScale = 0.72

// scaleOf is the size this element is drawn at, where one is full size.
func (p *placed) scaleOf() float64 {
	if m := p.marks(); m != nil && m.Small {
		return cueScale
	}
	return 1
}

/*
 * The measurements, each scaled by whatever the engraver is drawing at.
 *
 * Used in place of the bare smufl and Defaults values everywhere a note's
 * own size is being measured — and not where a clef, a key signature or a
 * barline is, since those belong to the staff rather than to the note and
 * are full size in a bar of cues as much as anywhere else.
 */

// sc is the size being drawn at.
//
// Zero reads as full size, so an engraver built without going through run —
// which the tests do — draws the music rather than nothing.
func (e *engraver) sc() float64 {
	if e.scale == 0 {
		return 1
	}
	return e.scale
}

// adv is how wide a glyph is at the size being drawn.
func (e *engraver) adv(name string) float64 {
	return smufl.Advance(name) * e.sc()
}

// headW is how wide a notehead of this value is at the size being drawn.
func (e *engraver) headW(v model.NoteValue) float64 {
	return noteheadWidth(v) * e.sc()
}

// glyphSize is what to put in a glyph's Size, or zero where it is full size
// — which the renderer reads as "the ordinary size" and is what every glyph
// outside a cue carries.
func (e *engraver) glyphSize() float64 {
	if e.sc() == 1 {
		return 0
	}
	return 4 * e.sc()
}

// stemTh and stemLen are a stem's thickness and length at this size.
func (e *engraver) stemTh() float64 {
	return e.opt.Defaults.StemThickness * e.sc()
}

func (e *engraver) stemLen() float64 {
	return stemLength * e.sc()
}

// beamTh and beamGap are a beam's thickness and the air between two of them.
func (e *engraver) beamTh() float64 {
	return e.opt.Defaults.BeamThickness * e.sc()
}

func (e *engraver) beamGap() float64 {
	return e.opt.Defaults.BeamSpacing * e.sc()
}

// drawingAt runs fn with the engraver drawing at a size, and puts the size
// back afterwards however fn leaves.
//
// A scale left switched on is a page of small music, which is a worse
// failure than a cue drawn full size: the cue is one bar and the leak is the
// rest of the piece.
func (e *engraver) drawingAt(scale float64, fn func()) {
	was := e.sc()
	e.scale = scale
	defer func() { e.scale = was }()
	fn()
}
