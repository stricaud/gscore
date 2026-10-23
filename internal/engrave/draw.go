/*
 * draw.go
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

package engrave

import (
	"math"
	"strconv"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// placed is one element with its position worked out, held until the whole
// measure is known.
//
// Drawing cannot happen element by element, because a beam belongs to a group
// of notes and its slope depends on where the first and last of them sit. The
// C engine drew straight to the screen inside realize_object() and so could
// never beam across anything.
type placed struct {
	x     float64
	index int

	note  *model.Note
	chord *model.Chord
	rest  *model.Rest

	step   int // staff step of a note, or the outer note of a chord
	lo, hi int // a chord's range; equal to step for a single note
	value  model.NoteValue
	dots   int
	start  model.Duration // position from the start of the measure
	width  float64
}

func (p *placed) marks() *model.Marks {
	switch {
	case p.note != nil:
		return &p.note.Marks
	case p.chord != nil:
		return &p.chord.Marks
	}
	return nil
}

// beamable reports whether the element can carry a beam: a note or chord of an
// eighth or shorter. Rests and longer notes break a group.
func (p *placed) beamable() bool {
	if p.rest != nil {
		return false
	}
	return p.value >= model.Eighth
}

func (p *placed) flags() int { return p.value.Flags() }

// drawMeasure lays out and draws one measure, returning the x it ends at.
func (e *engraver) drawMeasure(part *model.Part, b measureBox, x float64, stretch float64) float64 {
	sp := e.opt.Spacings
	start := x

	// A key or meter change is written at the head of the bar it takes effect
	// in, before any of its music.
	if b.prefix > 0 {
		x = e.drawSignatureChange(part, b, x)
	}

	items := make([]*placed, 0, len(b.slots))
	at := model.Duration{Num: 0, Den: 1}
	for i := range b.slots {
		s := &b.slots[i]
		x += s.lead * stretch

		p := &placed{x: x, index: s.index, start: at, width: s.width * stretch}
		switch el := s.elem.(type) {
		case *model.Note:
			p.note = el
			p.step = StaffStep(part.Clef, el.Pitch)
			p.lo, p.hi = p.step, p.step
			p.value, p.dots = written(el.WrittenDuration(el.Dur))
		case *model.Chord:
			p.chord = el
			p.lo, p.hi = chordRange(part.Clef, el)
			p.step = p.hi
			p.value, p.dots = written(el.WrittenDuration(el.Dur))
		case *model.Rest:
			p.rest = el
			p.value, p.dots = written(el.WrittenDuration(el.Dur))
		default:
			p.value = model.Quarter
		}
		items = append(items, p)
		at = at.Add(s.elem.Duration())

		x += s.width * stretch
	}
	if len(b.slots) == 0 {
		x += sp.NoteBase * 2 * stretch
	}

	// Noteheads first, then stems and beams, which need the whole group.
	for _, p := range items {
		e.drawHeads(part, p)
	}
	e.drawStemsAndBeams(items, part.Time)
	for _, p := range items {
		if m := p.marks(); m != nil {
			e.drawArticulations(m, p.x+noteheadWidth(p.value)/2, p.step, e.stemUp(p), p.index)
			e.drawDynamic(m, p)
			e.spanAt(m, p)
			e.drawLyrics(m, p)
			e.drawTexts(m, p)
			e.drawChordSymbol(m.Chord, p.x)
		}
	}

	e.drawTupletNumbers(items)

	hit := Hit{
		Part: e.partIndex, Measure: b.index,
		XMin: start, XMax: x, StaffTop: e.y,
		Clef: part.Clef.String(),
	}
	for i, p := range items {
		kind := "rest"
		y := e.y + stepY(middleStep)
		y2 := y
		var steps []int
		switch {
		case p.note != nil:
			kind = "note"
			y = e.y + stepY(float64(p.step))
			y2 = y
			steps = []int{p.step}
		case p.chord != nil:
			// A chord reaches from its lowest notehead to its highest, and a
			// click anywhere between them means the chord.
			kind = "chord"
			y = e.y + stepY(float64(p.hi))
			y2 = e.y + stepY(float64(p.lo))
			for _, n := range p.chord.Notes {
				steps = append(steps, StaffStep(e.clef, n.Pitch))
			}
		case p.rest == nil:
			kind = "symbol"
		}
		hit.Elements = append(hit.Elements, ElementPos{
			Ref: p.index, Index: i, X: p.x, Y: y, Y2: y2,
			Width: noteheadWidth(p.value), Kind: kind, Steps: steps,
		})
	}
	e.lay.Hits = append(e.lay.Hits, hit)

	x += sp.BetweenMeasures * stretch
	e.drawBarline(b.m.RightBarline, x)
	e.lay.Barlines = append(e.lay.Barlines, BarlinePos{
		Part: e.partIndex, Measure: b.index, X: x,
		Top: e.y, Bottom: e.y + staffHeight,
		Style: b.m.RightBarline.String(),
	})
	return x + sp.AfterBarline*stretch
}

// drawSignatureChange writes a new key or meter in the middle of the music.
func (e *engraver) drawSignatureChange(part *model.Part, b measureBox, x float64) float64 {
	sp := e.opt.Spacings
	x += sp.ClefBefore

	if b.change.hasKey {
		naturals, accidentals := keyChangeSteps(part.Clef, b.change.from, *b.change.key)
		for _, step := range naturals {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: "accidentalNatural", X: x, Y: e.y + stepY(float64(step)),
			})
			x += smufl.Advance("accidentalNatural") + sp.BetweenKeyAccidentals
		}
		glyph := "accidentalSharp"
		if b.change.key.Fifths < 0 {
			glyph = "accidentalFlat"
		}
		for _, step := range accidentals {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: glyph, X: x, Y: e.y + stepY(float64(step)),
			})
			x += smufl.Advance(glyph) + sp.BetweenKeyAccidentals
		}
		if len(naturals)+len(accidentals) > 0 {
			x += sp.AfterKeySignature
		}
	}

	if b.change.hasTime {
		x = e.placeTimeSignature(*b.change.time, x)
	}
	return x
}

// written picks the notehead for a duration. Callers pass the duration the
// note is *written* with, which inside a tuplet is not the one it sounds for.
// drawTupletNumbers writes the ratio over each tuplet group. Without it a
// triplet is indistinguishable from three ordinary notes that do not add up.
func (e *engraver) drawTupletNumbers(items []*placed) {
	start := -1
	for i, p := range items {
		m := p.marks()
		if m == nil || m.Tuplet == nil {
			start = -1
			continue
		}
		if m.Tuplet.Start || start < 0 {
			start = i
		}
		if !m.Tuplet.Stop {
			continue
		}

		first, last := items[start], items[i]
		// The number goes on the side the stems point, clear of the beam.
		up := e.stemUp(first)
		hi, lo := first.hi, first.lo
		for _, q := range items[start : i+1] {
			if q.hi > hi {
				hi = q.hi
			}
			if q.lo < lo {
				lo = q.lo
			}
		}
		y := e.y + stepY(float64(hi)) - stemLength - 0.9
		if !up {
			y = e.y + stepY(float64(lo)) + stemLength + 1.4
		}
		cx := (first.x + last.x + noteheadWidth(last.value)) / 2

		e.lay.Texts = append(e.lay.Texts, Text{
			S: strconv.Itoa(m.Tuplet.Actual), X: cx, Y: y,
			Size: 1.5, Anchor: "middle", Italic: true, Bold: true,
		})
		start = -1
	}
}

func written(d model.Duration) (model.NoteValue, int) {
	v, dots, ok := d.Written()
	if !ok {
		// An odd fraction that no notehead spells still has to be drawn.
		return model.Quarter, 0
	}
	return v, dots
}

func chordRange(c model.Clef, ch *model.Chord) (lo, hi int) {
	if len(ch.Notes) == 0 {
		return 0, 0
	}
	lo = StaffStep(c, ch.Notes[0].Pitch)
	hi = lo
	for _, n := range ch.Notes {
		s := StaffStep(c, n.Pitch)
		if s < lo {
			lo = s
		}
		if s > hi {
			hi = s
		}
	}
	return lo, hi
}

func noteheadWidth(v model.NoteValue) float64 {
	return smufl.Advance(smufl.NoteheadGlyph(v))
}

// stemUp decides which way a stem points.
//
// An explicit direction on the note wins, exactly as it did in the C engine:
// realize_area.c works out a direction from the middle line and then lets
// O_STEMUP or O_STEMDOWN override it. Without the override two voices on one
// staff cannot be written at all.
func (e *engraver) stemUp(p *placed) bool {
	if p.rest != nil {
		return true
	}
	if m := p.marks(); m != nil {
		switch m.Stem {
		case model.StemUp:
			return true
		case model.StemDown:
			return false
		}
	}
	// Otherwise the middle line is the pivot, generalised to a chord by
	// taking whichever outer note is furthest from it.
	return (middleStep - p.lo) >= (p.hi - middleStep)
}

func (e *engraver) drawHeads(part *model.Part, p *placed) {
	switch {
	case p.rest != nil:
		e.drawRestAt(p)
	case p.note != nil:
		e.drawNoteHead(p)
	case p.chord != nil:
		e.drawChordHeads(p)
	}
}

func (e *engraver) drawRestAt(p *placed) {
	if p.rest.Hidden {
		return
	}
	name := smufl.RestGlyph(p.value)
	step := 4.0
	if p.value == model.Whole {
		step = 6
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: p.x, Y: e.y + stepY(step), Ref: p.index,
	})
	e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(step), Staff: e.y}
	e.drawDots(p.x+smufl.Advance(name)+e.opt.Spacings.DotToNote, step, p.dots)
}

func (e *engraver) drawNoteHead(p *placed) {
	n := p.note
	if g := smufl.AccidentalGlyph(n.Accidental); g != "" {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: g,
			X:    p.x - smufl.Advance(g) - e.opt.Spacings.AccidentalToNote,
			Y:    e.y + stepY(float64(p.step)),
		})
	}
	head := smufl.NoteheadGlyphOf(p.value, n.Head)
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: head, X: p.x, Y: e.y + stepY(float64(p.step)), Ref: p.index,
	})
	e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(float64(p.step)), Staff: e.y}
	e.ledgerLines(p.x, smufl.Advance(head), p.step)
	e.drawDots(p.x+smufl.Advance(head)+e.opt.Spacings.DotToNote, float64(p.step), p.dots)
}

func (e *engraver) drawChordHeads(p *placed) {
	ch := p.chord
	head := smufl.NoteheadGlyph(p.value)
	if len(ch.Notes) > 0 {
		head = smufl.NoteheadGlyphOf(p.value, ch.Notes[0].Head)
	}
	up := e.stemUp(p)

	steps := make([]int, len(ch.Notes))
	for i, n := range ch.Notes {
		steps[i] = StaffStep(e.clef, n.Pitch)
	}

	for i, n := range ch.Notes {
		step := steps[i]
		if g := smufl.AccidentalGlyph(n.Accidental); g != "" {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: g, X: p.x - smufl.Advance(g) - e.opt.Spacings.AccidentalToNote,
				Y: e.y + stepY(float64(step)),
			})
		}
		// Noteheads a second apart cannot sit on the same side of the stem.
		offset := 0.0
		for j, other := range steps {
			if j != i && abs(other-step) == 1 {
				if (up && step == p.lo) || (!up && step == p.hi) {
					break
				}
				offset = smufl.Advance(head)
				break
			}
		}
		// Every notehead carries the chord's own ref, not just the first.
		// A chord is one element: selecting it must light all of it up, and
		// clicking any of its heads must find it.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name:  smufl.NoteheadGlyphOf(p.value, n.Head),
			X:     p.x + offset,
			Y:     e.y + stepY(float64(step)),
			Ref:   p.index,
		})
		if i == 0 {
			e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(float64(step)), Staff: e.y}
		}
		e.ledgerLines(p.x, smufl.Advance(head), step)
	}
	e.drawDots(p.x+smufl.Advance(head)+e.opt.Spacings.DotToNote, float64(p.hi), p.dots)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

/*
 * Stems, flags and beams
 */

// beatDuration returns the span that notes are grouped within. Compound
// meters beat in threes, so 6/8 groups by a dotted quarter rather than an
// eighth, which is what makes its beaming look right.
func beatDuration(t model.TimeSignature) model.Duration {
	if t.BeatValue == 0 {
		return model.Duration{Num: 1, Den: 4}
	}
	if t.Beats%3 == 0 && t.Beats > 3 {
		return model.Duration{Num: 3, Den: t.BeatValue}
	}
	return model.Duration{Num: 1, Den: t.BeatValue}
}

// beamGroups splits a measure into runs of notes that share beams.
func beamGroups(items []*placed, t model.TimeSignature) [][]*placed {
	beat := beatDuration(t)
	var out [][]*placed
	var cur []*placed

	flush := func() {
		if len(cur) > 1 {
			out = append(out, cur)
		}
		cur = nil
	}
	beatOf := func(d model.Duration) int {
		if beat.Float() == 0 {
			return 0
		}
		return int(math.Floor(d.Float()/beat.Float() + 1e-9))
	}

	for _, p := range items {
		if !p.beamable() {
			flush()
			continue
		}
		// A beam written by hand overrides the grouping by beat. Music is
		// phrased against the beat often enough that the engraver's guess has
		// to be arguable with.
		beam := model.BeamAuto
		if m := p.marks(); m != nil {
			beam = m.Beam
		}
		switch beam {
		case model.BeamNone:
			flush()
			continue
		case model.BeamStart:
			flush()
			cur = append(cur, p)
			continue
		case model.BeamContinue:
			cur = append(cur, p)
			continue
		case model.BeamStop:
			cur = append(cur, p)
			flush()
			continue
		}
		if len(cur) > 0 && beatOf(p.start) != beatOf(cur[0].start) {
			// A group opened by hand is not broken by the beat.
			if m := cur[0].marks(); m == nil || m.Beam != model.BeamStart {
				flush()
			}
		}
		cur = append(cur, p)
	}
	flush()
	return out
}

func (e *engraver) drawStemsAndBeams(items []*placed, t model.TimeSignature) {
	groups := beamGroups(items, t)
	inGroup := map[*placed]bool{}
	for _, g := range groups {
		for _, p := range g {
			inGroup[p] = true
		}
	}

	for _, p := range items {
		if p.rest != nil || inGroup[p] {
			continue
		}
		e.drawSingleStem(p)
	}
	for _, g := range groups {
		e.drawBeamGroup(g)
	}
}

func (e *engraver) drawSingleStem(p *placed) {
	if p.value <= model.Whole {
		return // whole notes and breves have no stem
	}
	d := e.opt.Defaults
	up := e.stemUp(p)
	head := noteheadWidth(p.value)

	loY := e.y + stepY(float64(p.lo))
	hiY := e.y + stepY(float64(p.hi))

	sx := p.x + head - d.StemThickness/2
	y1, y2 := loY, hiY-stemLength
	if !up {
		sx = p.x + d.StemThickness/2
		y1, y2 = hiY, loY+stemLength
	}
	e.lay.Lines = append(e.lay.Lines, Line{X1: sx, Y1: y1, X2: sx, Y2: y2, Thickness: d.StemThickness})

	if g := smufl.FlagGlyph(p.value, up); g != "" {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: g, X: sx, Y: y2})
	}
}

// drawBeamGroup draws the stems of a beamed run and the beams across them.
func (e *engraver) drawBeamGroup(g []*placed) {
	d := e.opt.Defaults

	// One direction serves the whole group. An explicit direction anywhere in
	// it settles the matter, since a beam cannot point two ways; otherwise it
	// goes with how far the notes run either side of the middle line.
	above, below := 0, 0
	forced, forcedUp := false, false
	for _, p := range g {
		if m := p.marks(); m != nil && m.Stem != model.StemAuto && !forced {
			forced, forcedUp = true, m.Stem == model.StemUp
		}
		if p.hi > middleStep {
			above++
		}
		if p.lo < middleStep {
			below++
		}
	}
	up := below >= above
	if forced {
		up = forcedUp
	}

	type end struct {
		x, y float64
		p    *placed
	}
	ends := make([]end, 0, len(g))
	for _, p := range g {
		head := noteheadWidth(p.value)
		sx := p.x + head - d.StemThickness/2
		base := e.y + stepY(float64(p.hi))
		if !up {
			sx = p.x + d.StemThickness/2
			base = e.y + stepY(float64(p.lo))
		}
		reach := base - stemLength
		if !up {
			reach = base + stemLength
		}
		ends = append(ends, end{x: sx, y: reach, p: p})
	}

	// The beam is a straight line through the outermost stem ends, with its
	// slope held back so that a wide leap does not tilt it absurdly.
	x0, x1 := ends[0].x, ends[len(ends)-1].x
	y0, y1 := ends[0].y, ends[len(ends)-1].y
	if x1 != x0 {
		maxSlope := 0.3
		slope := (y1 - y0) / (x1 - x0)
		if slope > maxSlope {
			slope = maxSlope
		}
		if slope < -maxSlope {
			slope = -maxSlope
		}
		y1 = y0 + slope*(x1-x0)
	}
	beamY := func(x float64) float64 {
		if x1 == x0 {
			return y0
		}
		return y0 + (y1-y0)*(x-x0)/(x1-x0)
	}

	// Every stem has to reach the beam, so the line is pushed out to clear the
	// furthest notehead rather than cutting through it.
	shift := 0.0
	for _, en := range ends {
		want := beamY(en.x)
		if up && want > en.y-0 {
			if delta := want - en.y; delta > shift {
				shift = delta
			}
		}
		if !up && want < en.y {
			if delta := en.y - want; delta > shift {
				shift = delta
			}
		}
	}
	if up {
		y0 -= shift
		y1 -= shift
	} else {
		y0 += shift
		y1 += shift
	}

	// Stems, now that the beam line is fixed.
	for _, en := range ends {
		base := e.y + stepY(float64(en.p.hi))
		if !up {
			base = e.y + stepY(float64(en.p.lo))
		}
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: en.x, Y1: base, X2: en.x, Y2: beamY(en.x), Thickness: d.StemThickness,
		})
	}

	// Beam levels: one line per flag the notes would otherwise carry. A level
	// that only some notes reach is drawn over the runs that do, with a stub
	// where a run is a single note.
	maxFlags := 0
	for _, p := range g {
		if f := p.flags(); f > maxFlags {
			maxFlags = f
		}
	}
	dir := 1.0
	if up {
		dir = -1
	}
	for level := 1; level <= maxFlags; level++ {
		offset := float64(level-1) * (d.BeamThickness + d.BeamSpacing) * -dir
		i := 0
		for i < len(g) {
			if g[i].flags() < level {
				i++
				continue
			}
			j := i
			for j+1 < len(g) && g[j+1].flags() >= level {
				j++
			}
			bx0, bx1 := ends[i].x, ends[j].x
			if i == j {
				// A lone note at this level gets a stub, pointing back toward
				// the group it belongs to.
				stub := 1.0
				if i > 0 {
					bx0 = ends[i].x - stub
				} else {
					bx1 = ends[i].x + stub
				}
			}
			e.beamQuad(bx0, beamY(bx0)+offset, bx1, beamY(bx1)+offset, d.BeamThickness, dir)
			i = j + 1
		}
	}
}

// beamQuad draws a beam as a filled parallelogram, which is what gives it an
// even thickness along a slope.
func (e *engraver) beamQuad(x0, y0, x1, y1, thickness, dir float64) {
	// A thick line with butt caps is indistinguishable from the real shape at
	// these sizes and keeps the layout to primitives a renderer already has.
	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x0, Y1: y0 + dir*thickness/2,
		X2: x1, Y2: y1 + dir*thickness/2,
		Thickness: thickness,
	})
}

func (e *engraver) ledgerLines(x, headWidth float64, step int) {
	d := e.opt.Defaults
	ext := d.LegerLineExtension
	draw := func(s int) {
		y := e.y + stepY(float64(s))
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x - ext, Y1: y, X2: x + headWidth + ext, Y2: y, Thickness: d.LegerLineThickness,
		})
	}
	for s := 10; s <= step; s += 2 {
		draw(s)
	}
	for s := -2; s >= step; s -= 2 {
		draw(s)
	}
}

func (e *engraver) drawDots(x, step float64, dots int) {
	if dots <= 0 {
		return
	}
	// A dot never sits on a line, so one that would moves up into the space.
	ds := step
	if int(ds)%2 == 0 {
		ds++
	}
	for i := 0; i < dots; i++ {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "augmentationDot", X: x, Y: e.y + stepY(ds),
		})
		x += e.opt.Spacings.BetweenDots
	}
}

// drawDynamic writes a volume marking under the staff.
//
// Dynamics belong below the staff for most instruments and are aligned on
// their own line rather than hung off each note, so that a row of them reads
// as one. The glyph is centred on the note it belongs to.
func (e *engraver) drawDynamic(m *model.Marks, p *placed) {
	if m.Dynamic == model.DynamicNone {
		return
	}
	name := smufl.DynamicGlyph(m.Dynamic)
	if name == "" {
		return
	}
	// Below the lowest line, far enough down to clear a stem that points that
	// way and any ledger lines under it.
	y := e.y + staffHeight + dynamicDrop
	if p.lo < 0 {
		y += float64(-p.lo) / 2
	}
	x := p.x + noteheadWidth(p.value)/2 - smufl.Advance(name)/2
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: x, Y: y})
}

// drawTexts writes the words attached to a note: "rit.", "solo", a reminder to
// the player.
//
// Above the staff unless told otherwise, because that is where a direction to
// the player goes and below is where the dynamics and the words already are.
func (e *engraver) drawTexts(m *model.Marks, p *placed) {
	for _, t := range m.Texts {
		if t.Text == "" {
			continue
		}
		y := e.y - 2.2
		if t.Placement == model.PlacementBelow {
			y = e.y + staffHeight + 3.4
		} else if p.hi > 10 {
			// Clear of a passage that climbs above the staff.
			y = e.y + stepY(float64(p.hi)) - 1.8
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: t.Text, X: p.x + t.OffsetX, Y: y + t.OffsetY,
			Size: 1.9, Anchor: "start", Italic: t.Italic, Bold: t.Bold,
		})
	}
}

// drawLyrics writes the words under the staff, one line per verse.
//
// The syllable is centred on the notehead it is sung on, and a word broken
// across notes carries a hyphen to the next one — without it "Glo-ri-a" reads
// as three words. A held syllable draws the line that says to keep singing it.
func (e *engraver) drawLyrics(m *model.Marks, p *placed) {
	if len(m.Lyrics) == 0 {
		return
	}
	x := p.x + noteheadWidth(p.value)/2
	for _, ly := range m.Lyrics {
		verse := ly.Verse
		if verse < 1 {
			verse = 1
		}
		y := e.y + staffHeight + lyricDrop + float64(verse-1)*lyricLine
		if ly.Text != "" {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: ly.Text, X: x, Y: y, Size: lyricSize, Anchor: "middle",
			})
		}
		// A hyphen between the halves of a word, drawn midway to the next
		// note rather than hard against this one.
		if ly.NeedsHyphen() {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: "-", X: x + p.width*0.55, Y: y, Size: lyricSize, Anchor: "middle",
			})
		}
		if ly.Extend && p.width > 1 {
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x + float64(len(ly.Text))*lyricSize*0.3, Y1: y + 0.25,
				X2: x + p.width*0.9, Y2: y + 0.25,
				Thickness: e.opt.Defaults.StaffLineThickness,
			})
		}
	}
}

func (e *engraver) drawArticulations(m *model.Marks, x float64, step int, stemUp bool, ref int) {
	if len(m.Articulations) == 0 && len(m.Symbols) == 0 {
		return
	}
	// Articulations sit on the side away from the stem, clear of the staff
	// when the note is already outside it.
	above := !stemUp
	y := e.y + stepY(float64(step))
	gap := 1.3
	if above {
		y -= gap
		if step > 9 {
			y = e.y - 1.2
		}
	} else {
		y += gap
		if step < -1 {
			y = e.y + staffHeight + 1.2
		}
	}
	emit := func(name string) {
		if name == "" {
			return
		}
		// A name the font does not know is one of the user's own symbols; the
		// renderer resolves it from the symbol registry.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: x, Y: y})
		if above {
			y -= gap
		} else {
			y += gap
		}
	}
	for _, a := range m.Articulations {
		emit(smufl.ArticulationGlyph(a, above))
	}
	for _, sm := range m.Symbols {
		// A nudge moves the symbol from where it would otherwise land. It
		// carries the ref of the note it belongs to so that it can be
		// selected, highlighted and dragged like anything else drawn.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: sm.Name, X: x + sm.OffsetX, Y: y + sm.OffsetY, Ref: ref,
		})
		if above {
			y -= gap
		} else {
			y += gap
		}
	}
}
