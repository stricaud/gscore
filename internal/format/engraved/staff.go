/*
 * staff.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"math"
	"sort"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

// mark is one thing read off a staff, with where it was.
type mark struct {
	x, y float64
	s    sense
}

// readStaff turns one staff into measures.
func readStaff(c *pdf.Content, st staff) ([]*model.Measure, *model.KeySignature, *model.TimeSignature) {
	var marks []mark
	for _, g := range c.Glyphs {
		if !st.holds(g) {
			continue
		}
		s := readGlyph(g)
		if s.kind == kOther {
			continue
		}
		marks = append(marks, mark{g.X, g.Y, s})
	}
	if len(marks) == 0 {
		return nil, nil, nil
	}
	sort.Slice(marks, func(i, j int) bool { return marks[i].x < marks[j].x })

	bars := barlines(c, st)
	// the clef in force is whatever was drawn before the first note
	cur_clef := model.Treble
	for _, m := range marks {
		if m.s.kind == kClef {
			cur_clef = m.s.clef
			break
		}
	}

	// Walk the measures the barlines mark out and gather what falls in each,
	// rather than walking the marks and closing a measure whenever a barline
	// is passed. The two sound equivalent and are not: a clef, a key or a
	// rest sitting just the wrong side of a barline shifts everything after
	// it by a measure in the second, which is how the lower staff of a duet
	// came out two measures short while holding all its notes.
	// The measures are the gaps between barlines, with one open-ended gap at
	// each end: before the first barline, in case the staff does not open
	// with one, and after the last, in case it does not close with one. An
	// empty gap is not a measure and is dropped, which disposes of the
	// opening barline without having to recognise it as one.
	bounds := make([]float64, 0, len(bars)+2)
	bounds = append(bounds, math.Inf(-1))
	bounds = append(bounds, bars...)
	bounds = append(bounds, math.Inf(1))

	var out []*model.Measure
	for b := 0; b+1 < len(bounds); b++ {
		lo, hi := bounds[b], bounds[b+1]
		m := model.NewMeasure()
		for i, mk := range marks {
			// Measured from the middle of the mark, not its left edge.
			//
			// A glyph's recorded position is where the font sets it down,
			// which for a notehead is its left side; the head then extends
			// to the right of it. A head sitting just after a barline can
			// therefore have its origin a fraction to the *left* of that
			// barline and be counted into the measure before — which is how
			// a 64-bar part came out with 62 bars while holding all its
			// notes.
			at := mk.x + 0.55*st.inter
			if at <= lo || at > hi {
				continue
			}
			switch mk.s.kind {
			case kClef:
				if len(out) > 0 {
					cc := mk.s.clef
					m.Clef = &cc
				}
				cur_clef = mk.s.clef
			case kNotehead:
				n := &model.Note{
					Pitch: pitchAt(st, cur_clef, mk.y),
					Dur: dotted(valueOf(c, st, marks, i).Duration(),
						dotsOn(st, marks, i)),
				}
				if a, ok := accidentalFor(marks, i, st); ok {
					n.Accidental = a
					n.Pitch.Alter = alterOf(a)
				}
				m.Voices[0].Elements = append(m.Voices[0].Elements, n)
			case kRest:
				m.Voices[0].Elements = append(m.Voices[0].Elements,
					&model.Rest{Dur: dotted(mk.s.value.Duration(),
						dotsOn(st, marks, i))})
			}
		}
		if len(m.Voices[0].Elements) > 0 {
			out = append(out, m)
		}
	}
	key, time := signatures(c, st, marks)
	return out, key, time
}

// pitchAt reads a notehead's pitch off its height.
func pitchAt(st staff, c model.Clef, y float64) model.Pitch {
	base := bottomPitch(c)
	return model.PitchFromDiatonicIndex(base.DiatonicIndex() + st.degree(y))
}

func alterOf(a model.Accidental) int {
	switch a {
	case model.Sharp:
		return 1
	case model.Flat:
		return -1
	case model.DoubleSharp:
		return 2
	case model.DoubleFlat:
		return -2
	}
	return 0
}

// accidentalFor finds the accidental written in front of a notehead.
//
// In front, and at the same height: an accidental belongs to the note it
// precedes on the same line or space, and the one written for the note above
// is a different sign a long way up. Two staff spaces of slack vertically,
// and three to the left, which is where engravers put it.
func accidentalFor(marks []mark, i int, st staff) (model.Accidental, bool) {
	note := marks[i]
	for j := i - 1; j >= 0 && note.x-marks[j].x < 3*st.inter; j-- {
		if marks[j].s.kind != kAccidental {
			continue
		}
		if math.Abs(marks[j].y-note.y) < 0.6*st.inter {
			return marks[j].s.acc, true
		}
	}
	return 0, false
}

// barlines finds the x of every line that divides the staff into measures.
//
// A barline is a vertical stroke spanning the staff from the bottom line to
// the top. A stem is vertical too and that is why the span is tested rather
// than the mere fact of being upright: a stem starts at its notehead and
// reaches about three spaces, where a barline covers all four and starts and
// ends on a line.
func barlines(c *pdf.Content, st staff) []float64 {
	var xs []float64
	height := st.y[4] - st.y[0]
	for _, s := range c.Segments {
		if !s.Vertical() {
			continue
		}
		lo, hi := math.Min(s.Y1, s.Y2), math.Max(s.Y1, s.Y2)
		if hi-lo < 0.9*height {
			continue
		}
		if lo > st.y[0]+0.2*st.inter || hi < st.y[4]-0.2*st.inter {
			continue
		}
		x := (s.X1 + s.X2) / 2
		if x < st.x0-st.inter || x > st.x1+st.inter {
			continue
		}
		xs = append(xs, x)
	}
	sort.Float64s(xs)
	// a thick barline, or a repeat, is drawn as several strokes side by side
	var out []float64
	for _, x := range xs {
		if n := len(out); n > 0 && x-out[n-1] < 0.8*st.inter {
			continue
		}
		out = append(out, x)
	}
	return out
}
