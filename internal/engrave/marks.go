/*
 * marks.go
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
	"github.com/stricaud/gscore/internal/smufl"
)

/*
 * The marks written on a note that are not articulations.
 *
 * Three of them, and they go in three different places, which is why they
 * are not one function: an arpeggio stands in front of the chord, a breath
 * goes after the note and over the staff, and a fingering sits beside the
 * notehead it belongs to. What they have in common is only that they hang
 * off a note rather than spanning a stretch of music.
 */

const (
	// arpeggioGap is the air between the wavy line and the leftmost
	// notehead of the chord it rolls.
	arpeggioGap = 0.5
	// arpeggioSegment is how tall one segment of the line is: the glyph is
	// a single wiggle meant to be stacked, so this is how far apart to set
	// them for the line to read as continuous.
	arpeggioSegment = 1.0
	// arpeggioHeight is how tall one segment is, so the line can be stopped
	// where its top reaches the chord rather than two spaces past it.
	arpeggioHeight = 2.0
	// arpeggioAfter is the air between the line and the notehead it rolls
	// up to, which is what keeps it from reading as part of the chord.
	arpeggioAfter = 0.3
	// arpeggioOver is how far past the chord's outer noteheads the line
	// reaches at each end, so that it brackets the whole chord rather than
	// stopping level with it.
	arpeggioOver = 0.5

	// breathGap is the air between a breath mark and the staff or the note
	// it follows, and breathAfter how far past the notehead it is set.
	breathGap   = 0.4
	breathAfter = 1.1

	// fingeringGap is the air between a fingering and the notehead it
	// belongs to, and fingeringSize the type the figures are set in. Smaller
	// than a lyric: a fingering is read off the note it is beside and must
	// not compete with the music.
	fingeringGap  = 0.9
	fingeringSize = 1.5
)

// arpeggioLead is the room an element needs in front of it for its arpeggio
// line.
//
// Booked with the accidentals and the grace notes, before anything is drawn.
// Without it the line is drawn into whatever stands to the left — on a pair
// of rolled chords a space apart, into the first chord's own noteheads.
func arpeggioLead(el model.Element) float64 {
	m := marksOf(el)
	if m == nil || m.Arpeggio == nil {
		return 0
	}
	return arpeggioGap + smufl.Advance("arpeggiato") + arpeggioAfter
}

// drawArpeggio draws the wavy line that says to roll a chord.
//
// In front of the whole chord, from its lowest notehead to its highest: the
// line is what says which notes are rolled, so it has to reach all of them.
// A chord whose line stopped short of its top note would be asking for half
// a roll.
func (e *engraver) drawArpeggio(p *placed, a *model.Arpeggio) {
	if a == nil {
		return
	}
	// Left of the leftmost notehead, which for a chord with a note pushed
	// across the stem is not the column.
	x := p.x - arpeggioGap - e.adv("arpeggiato")
	if p.chord != nil && e.displacedLeft(p) {
		x -= e.headW(p.value)
	}

	top := e.y + stepY(float64(p.hi)) - arpeggioOver
	bottom := e.y + stepY(float64(p.lo)) + arpeggioOver
	// At least one segment: an arpeggio on a two-note interval is still an
	// arpeggio, and a line shorter than one wiggle is not a line.
	if bottom-top < arpeggioSegment {
		bottom = top + arpeggioSegment
	}

	// Stacked from the foot of the chord up. Each segment is drawn upward
	// from where it is put and is about two spaces tall, so they are set a
	// space apart — overlapping by half, which is what makes the line read
	// as continuous rather than as a column of wiggles — and the last one
	// goes where its own top reaches the chord's top note.
	//
	// arpeggiato and not arpeggiatoUp: that one carries an arrowhead of its
	// own, and a plain line is what says to roll upward. The wiggle pieces
	// are no use here at all — the font draws those lying down, to be
	// turned on their side by whatever uses them.
	foot := bottom
	if a.Down {
		// The arrowhead at the foot, which is the whole of what says to
		// come down through the chord instead.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "arpeggiatoDown", X: x, Y: bottom, Size: e.glyphSize(),
		})
		foot = bottom - arpeggioSegment
	}
	for y := foot; y >= top+arpeggioHeight; y -= arpeggioSegment {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "arpeggiato", X: x, Y: y, Size: e.glyphSize(),
		})
	}
}

// displacedLeft reports whether a chord has a notehead pushed out to the
// left of the column, which is where a stem-down chord puts the second note
// of a cluster.
func (e *engraver) displacedLeft(p *placed) bool {
	if p.chord == nil || e.stemUp(p) {
		return false
	}
	steps := make([]int, len(p.chord.Notes))
	for i, n := range p.chord.Notes {
		steps[i] = e.noteStep(p.clef, n)
	}
	for _, sh := range displacedHeads(steps, false) {
		if sh {
			return true
		}
	}
	return false
}

// drawBreath draws the breath or pause taken after a note.
//
// After the note and over the staff, which is where every one of them goes
// whichever way the stems point: a breath is a direction to the player about
// the gap between two notes, not a mark on either of them, and the top of
// the staff is where the eye looks for it.
func (e *engraver) drawBreath(p *placed, b model.Breath, end float64) {
	if b == model.NoBreath {
		return
	}
	name := smufl.BreathGlyph(b)
	if name == "" {
		return
	}
	// Just past the notehead, and never past the barline: at the end of a
	// measure the mark is set just before the line rather than on it.
	x := p.x + e.headW(p.value) + breathAfter
	if end > 0 && x+e.adv(name) > end {
		x = end - e.adv(name)
	}
	y := e.y - breathGap
	// Clear of whatever is already above the staff here — a beam over a
	// high run, an articulation — rather than through it.
	if edge := e.staffInkline().reach(x, x+e.adv(name), false); !math.IsInf(edge, 0) {
		if up := edge - breathGap; up < y {
			y = up
		}
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: x, Y: y, Size: e.glyphSize(),
	})
}

// drawFingering writes which fingers play a note, beside the noteheads.
//
// Beside, not above: a fingering belongs to one notehead of a chord and a
// column of figures over the chord says nothing about which finger takes
// which note. They are set on the side away from the stem, from the bottom
// note up, which is the order a player reads a chord in.
func (e *engraver) drawFingering(p *placed, fing []string) {
	if len(fing) == 0 {
		return
	}
	up := e.stemUp(p)
	// Away from the stem, so the figures are never in among the beams.
	side := 1.0
	if up {
		side = -1
	}

	// The notehead each figure belongs to, bottom to top.
	steps := []int{p.step}
	if p.chord != nil {
		steps = steps[:0]
		for _, n := range p.chord.Notes {
			steps = append(steps, e.noteStep(p.clef, n))
		}
		sortInts(steps)
	}

	// A chord's figures stand in a column of their own beside it, each at
	// the height of the notehead it belongs to. Centred over the noteheads
	// they would be printed on the notes above them, which on a chord a
	// third apart is every figure but the top one.
	column := p.chord != nil && len(fing) > 1
	for i, text := range fing {
		if text == "" || i >= len(steps) {
			continue
		}
		y := e.y + stepY(float64(steps[i]))
		x := p.x + e.headW(p.value)/2
		anchor := "middle"
		if column {
			// Beside the chord, on the side away from the stem.
			x = p.x - fingeringGap
			anchor = "end"
			if !up {
				x = p.x + e.headW(p.value) + fingeringGap
				anchor = "start"
			}
			// Centred on the notehead rather than over it.
			y += fingeringSize * 0.35
		} else {
			y += side * fingeringGap
			// A figure for a note on a line is nudged into the space, where
			// one centred on the line would be cut by it.
			if steps[i]%2 == 0 {
				y += side * 0.2
			}
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: text, X: x, Y: y, Size: fingeringSize, Anchor: anchor,
		})
	}
}

// sortInts puts a handful of staff steps in order, bottom to top. A chord
// has at most a dozen notes, so the simplest sort is the right one.
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
