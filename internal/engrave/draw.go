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
	"sort"
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

	// stemDir is the direction this note's stem is really drawn in: 1 up,
	// -1 down, 0 before it has been settled.
	//
	// It is not the same question as "which way would this note point on its
	// own". A beam serves a whole group and can only point one way, so a low
	// note inside a group of high ones has its stem turned down against its
	// own inclination — and everything placed on the other side from the
	// stem has to follow the stem that is there, not the one the note would
	// have had alone. That was how a slur over a rising run came to be drawn
	// underneath it, through the beam.
	stemDir int
}

// marks are what this element carries: its articulations, its dynamic, and
// the ends of anything reaching across it.
//
// A rest has them too, and saying it had none lost every one. A crescendo
// that closes on a rest is ordinary writing — a player holds the swell to the
// end of the phrase and the phrase ends in silence — and the engraver never
// saw the mark that closed it, so the hairpin stayed open and was drawn to
// the right margin of that system and of every system after it, for the rest
// of the piece. The same went for a fermata over a rest, a dynamic written at
// one, and a chord symbol over a bar's rest in a chart.
func (p *placed) marks() *model.Marks {
	switch {
	case p.note != nil:
		return &p.note.Marks
	case p.chord != nil:
		return &p.chord.Marks
	case p.rest != nil:
		return &p.rest.Marks
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

// barlineAt is where a measure's closing barline goes: at the column the next
// bar starts in, less the air that belongs after the line.
//
// Not where this staff's own music happened to reach. Every staff of a system
// shares the columns, and barlines lining up is the whole point of a score —
// it is how the eye reads down a system and finds the same beat on every
// staff. Each staff drew its own line at the end of its own notes, so a
// system came out with a ragged edge rather than a column: over a staff space
// of disagreement in a band score, and worst at the end of the piece, where
// nine staves finished the last bar at nine different places.
// It is measured with the system's stretch and not the staff's own. Each
// staff fills its column at whatever rate its content needs — a bar of four
// eighths and a bar of one half note are stretched differently to end in the
// same place — and scaling the gap after the barline by that rate is the same
// mistake one step further on.
func (e *engraver) barlineAt(end, right float64) float64 {
	return end - (e.opt.Spacings.AfterBarline+right)*e.stretch
}

// drawMeasure lays out and draws one measure between two columns, returning
// the x it ends at.
//
// x is where the bar's own music starts and end is where the next bar starts;
// the barline goes between them, at the place every staff agrees on.
func (e *engraver) drawMeasure(part *model.Part, b measureBox, x, end float64, stretch float64) float64 {
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
			// A rest is drawn on the middle line, and anything placed
			// against it — a fermata over it, a hairpin closing on it —
			// is placed from the same position rather than from the bottom
			// line, which is where a step of zero would have put it.
			p.step, p.lo, p.hi = middleStep, middleStep, middleStep
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

	// A bar marked as a repeat of the one before it is drawn as the sign and
	// nothing else. Whatever is in it is not printed, because the sign is
	// what is read; keeping the notes means they can be put back by taking
	// the sign away, which is why they are not thrown out instead.
	if b.m.Repeat > 0 {
		e.drawMeasureRepeat(b, start, x)
		_, right := e.barlineRoom(b.m.RightBarline)
		x = e.barlineAt(end, right)
		e.drawBarline(b.m.RightBarline, x)
		e.lay.Barlines = append(e.lay.Barlines, BarlinePos{
			Part: e.partIndex, Measure: b.index, X: x,
			Top: e.y, Bottom: e.y + staffHeight,
			Style: b.m.RightBarline.String(),
		})
		e.lay.Hits = append(e.lay.Hits, Hit{
			Part: e.partIndex, Measure: b.index,
			XMin: start, XMax: x, StaffTop: e.y, Clef: part.Clef.String(),
			Fifths: part.KeyAt(b.index).Fifths,
			Full:   barIsFull(part, b),
		})
		return end
	}

	// Which way the stems point is settled before anything is drawn: the
	// dots, the articulations and the slurs all sit on the side away from
	// the stem, and all of them used to work it out again from the note
	// alone — which gave a different answer from the beam.
	e.setStemDirections(items, part.Time)

	// Noteheads first, then stems and beams, which need the whole group.
	for _, p := range items {
		e.drawHeads(part, p)
		e.drawGrace(part, p)
	}
	e.drawStemsAndBeams(items, part.Time)
	for _, p := range items {
		if m := p.marks(); m != nil {
			e.drawArticulations(m, p.x+noteheadWidth(p.value)/2, p.step, e.stemUp(p), p.index)
			e.drawDynamic(m, p)
			e.spanAt(m, p)
			e.slurAt(m, p)
			e.drawLyrics(m, p)
			e.drawTexts(m, p)
			e.drawChordSymbol(m.Chord, p.x)
		}
	}

	e.drawTuplets(items, part.Time)

	hit := Hit{
		Part: e.partIndex, Measure: b.index,
		XMin: start, XMax: x, StaffTop: e.y,
		Clef:       part.Clef.String(),
		KeyChange:  b.m.Key != nil,
		TimeChange: b.m.Time != nil,
		Fifths:     part.KeyAt(b.index).Fifths,
		Full:       barIsFull(part, b),
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
		// Against the bar's real length, which is the model's one answer to
		// that question — the same one playback uses to decide when the bar
		// begins. Measuring against the meter instead left the cursor and
		// the note it points at drifting apart inside the bar.
		at := 0.0
		if bar := e.score.MeasureDuration(b.index); !bar.IsZero() {
			at = p.start.Float() / bar.Float()
			if at > 1 {
				at = 1
			}
		}
		hit.Elements = append(hit.Elements, ElementPos{
			Ref: p.index, Index: i, X: p.x, Y: y, Y2: y2, At: at,
			Width: noteheadWidth(p.value), Kind: kind, Steps: steps,
		})
	}
	e.lay.Hits = append(e.lay.Hits, hit)

	// A barline takes as much room as it actually draws: a repeat is a thick
	// stroke, a thin one and two dots, and the dots want a space of their
	// own on the side the music is. That room was budgeted into the column
	// when its width was worked out, which is why the line can be placed
	// from the column rather than from the music.
	_, right := e.barlineRoom(b.m.RightBarline)
	x = e.barlineAt(end, right)
	// The bar drawn past the end of the music has no barline: it is not a
	// bar of the piece until something is written in it.
	if !b.ahead {
		e.drawBarline(b.m.RightBarline, x)
		e.lay.Barlines = append(e.lay.Barlines, BarlinePos{
			Part: e.partIndex, Measure: b.index, X: x,
			Top: e.y, Bottom: e.y + staffHeight,
			Style: b.m.RightBarline.String(),
		})
	}
	return end
}

// barIsFull reports whether a bar already holds as much time as its meter
// allows, which is a different question from whether it looks busy: a bar of
// rest is drawn as one small glyph and is as full as a bar can be.
func barIsFull(part *model.Part, b measureBox) bool {
	if b.ahead || len(b.m.Voices) == 0 {
		return false
	}
	bar := part.TimeAt(b.index).BarDuration()
	if bar.IsZero() {
		return false
	}
	return b.m.Voices[0].Duration().Cmp(bar) >= 0
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
		if len(naturals) > 0 && len(accidentals) > 0 {
			x += sp.AfterKeyCancel
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

// drawMeasureRepeat draws the sign that stands for the bar before.
func (e *engraver) drawMeasureRepeat(b measureBox, left, right float64) {
	name := "repeat1Bar"
	switch {
	case b.m.Repeat >= 4:
		name = "repeat4Bars"
	case b.m.Repeat >= 2:
		name = "repeat2Bars"
	}
	// Centred on the bar and on the middle line, which is where it is read
	// from and why it is as legible at a glance as it is.
	w := smufl.Advance(name)
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: (left+right)/2 - w/2, Y: e.y + stepY(4),
	})
	if b.m.Repeat >= 2 {
		// The two- and four-bar signs carry the count under them, because
		// the sign alone does not say how far back to go.
		e.lay.Texts = append(e.lay.Texts, Text{
			S: itoa(b.m.Repeat), X: (left + right) / 2, Y: e.y - 0.8,
			Size: 1.6, Anchor: "middle", Bold: true,
		})
	}
}

// written picks the notehead for a duration. Callers pass the duration the
// note is *written* with, which inside a tuplet is not the one it sounds for.
// How a tuplet is written over its notes.
const (
	tupletNumberSize = 1.5
	// How far the bracket's ends turn in towards the notes.
	tupletHookDepth = 0.6
	// The gap left in the bracket for the number to sit in, either side of
	// it. A line drawn through the figure is a line drawn through it.
	tupletNumberGap = 0.5
)

// drawTuplets writes the ratio over each tuplet group, and brackets it where
// the beam does not already say where the group begins and ends.
//
// Without the number a triplet is indistinguishable from three ordinary
// notes that do not add up. Without the bracket, a group of notes too long to
// beam — three quarter notes in the time of two — has a figure floating over
// it saying three of what, from where to where.
//
// A group that is beamed together needs no bracket and conventionally does
// not get one: the beam already draws the line and marks both ends, and a
// bracket over it is a second statement of the same thing. This is the rule
// in Gould and in every published score.
func (e *engraver) drawTuplets(items []*placed, t model.TimeSignature) {
	// Which notes the beamer joined, so a beamed group can be told from one
	// that only looks like it.
	beam := map[*placed]int{}
	for i, g := range beamGroups(items, t) {
		for _, p := range g {
			beam[p] = i + 1
		}
	}

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
		e.drawTuplet(items[start:i+1], m.Tuplet, beam)
		start = -1
	}
}

// drawTuplet writes one group's figure and, where it is wanted, its bracket.
func (e *engraver) drawTuplet(group []*placed, tp *model.Tuplet, beam map[*placed]int) {
	first, last := group[0], group[len(group)-1]

	// The figure goes on the side the stems point, clear of the beam.
	up := e.stemUp(first)
	hi, lo := first.hi, first.lo
	for _, q := range group {
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

	x0 := first.x
	x1 := last.x + noteheadWidth(last.value)
	cx := (x0 + x1) / 2

	e.lay.Texts = append(e.lay.Texts, Text{
		S: strconv.Itoa(tp.Actual), X: cx, Y: y,
		Size: tupletNumberSize, Anchor: "middle", Italic: true, Bold: true,
	})

	if oneBeam(group, beam) {
		return // the beam says it already
	}

	// The bracket sits on the figure's own line, with a gap cut in it for
	// the figure, and a hook at each end turning in towards the notes.
	ly := y - tupletNumberSize*0.35 // the middle of the figure, not its baseline
	half := textWidth(strconv.Itoa(tp.Actual), tupletNumberSize)/2 + tupletNumberGap
	hook := tupletHookDepth
	if !up {
		hook = -hook
	}
	th := e.opt.Defaults.StemThickness
	add := func(xa, ya, xb, yb float64) {
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: xa, Y1: ya, X2: xb, Y2: yb, Thickness: th,
		})
	}
	add(x0, ly+hook, x0, ly) // the hook at the near end
	add(x0, ly, cx-half, ly) // up to the figure
	add(cx+half, ly, x1, ly) // and on from it
	add(x1, ly, x1, ly+hook) // the hook at the far end
}

// oneBeam says whether the whole group is joined by a single beam, which is
// what makes a bracket unnecessary.
func oneBeam(group []*placed, beam map[*placed]int) bool {
	g := beam[group[0]]
	if g == 0 {
		return false
	}
	for _, p := range group[1:] {
		if beam[p] != g {
			return false
		}
	}
	return true
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
	// Settled already, by the beam group this note belongs to.
	if p.stemDir != 0 {
		return p.stemDir > 0
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
	// A rest has no stem for the dots to clear.
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
	e.drawDots(p.x+smufl.Advance(head)+e.dotGap(e.stemUp(p)), float64(p.step), p.dots)
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
	shifted := displacedHeads(steps, up)

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
		if shifted[i] {
			offset = smufl.Advance(head)
		}
		// Every notehead carries the chord's own ref, not just the first.
		// A chord is one element: selecting it must light all of it up, and
		// clicking any of its heads must find it.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: smufl.NoteheadGlyphOf(p.value, n.Head),
			X:    p.x + offset,
			Y:    e.y + stepY(float64(step)),
			Ref:  p.index,
		})
		if i == 0 {
			e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(float64(step)), Staff: e.y}
		}
		e.ledgerLines(p.x, smufl.Advance(head), step)
	}
	e.drawDots(p.x+smufl.Advance(head)+e.dotGap(e.stemUp(p)), float64(p.hi), p.dots)
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

// setStemDirections settles which way every stem in a measure points.
//
// It runs before anything is drawn, because the answer is needed by more than
// the stems: the dots after a notehead, the articulations, and the slurs and
// ties all sit on the side away from the stem. Asking each of them to work it
// out again from the note alone is how they came to disagree with the stem
// actually on the page.
func (e *engraver) setStemDirections(items []*placed, t model.TimeSignature) {
	for _, g := range beamGroups(items, t) {
		dir := -1
		if beamGroupUp(g) {
			dir = 1
		}
		for _, p := range g {
			p.stemDir = dir
		}
	}
	// Whatever is left stands alone and keeps its own inclination. Setting
	// it here as well means nothing downstream has to know the difference.
	for _, p := range items {
		if p.stemDir == 0 && p.rest == nil {
			if e.stemUp(p) {
				p.stemDir = 1
			} else {
				p.stemDir = -1
			}
		}
	}
}

// beamGroupUp is the one rule for which way a beamed group points.
//
// A beam cannot point two ways, so an explicit direction anywhere in the
// group settles it; otherwise it goes with how far the notes run either side
// of the middle line.
func beamGroupUp(g []*placed) bool {
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
	if forced {
		return forcedUp
	}
	return below >= above
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

// stemEnd is where a stem finishes, given the notehead it grows out of.
//
// A stem is stemLength long, except that it always reaches the middle line
// of the staff. A note out on ledger lines with a stem of the standard
// length has a stem that stops in mid air — for anything from the third
// ledger line out it never arrives at the staff at all, and the note reads
// as something floating above the music rather than as a high note on it.
// Reaching back to the middle line ties it to the staff, which is why every
// engraver does it; it is Gould's rule in Behind Bars and LilyPond's in
// stem.cc.
//
// This only ever lengthens a stem. A note inside the staff is already past
// the middle line at the standard length, so nothing it does is undone.
func (e *engraver) stemEnd(base float64, up bool) float64 {
	mid := e.y + stepY(middleStep)
	if up {
		return math.Min(base-stemLength, mid)
	}
	return math.Max(base+stemLength, mid)
}

func (e *engraver) drawSingleStem(p *placed) {
	if p.value <= model.Whole {
		// No stem. A tremolo on one still has to go somewhere, and over the
		// notehead is where it goes.
		e.drawTremolo(p, 0, 0, 0)
		return
	}
	d := e.opt.Defaults
	up := e.stemUp(p)
	head := noteheadWidth(p.value)

	loY := e.y + stepY(float64(p.lo))
	hiY := e.y + stepY(float64(p.hi))

	sx := p.x + head - d.StemThickness/2
	y1, y2 := loY, e.stemEnd(hiY, up)
	if !up {
		sx = p.x + d.StemThickness/2
		y1, y2 = hiY, e.stemEnd(loY, up)
	}
	e.lay.Lines = append(e.lay.Lines, Line{X1: sx, Y1: y1, X2: sx, Y2: y2, Thickness: d.StemThickness})

	if g := smufl.FlagGlyph(p.value, up); g != "" {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: g, X: sx, Y: y2})
	}
	e.drawTremolo(p, sx, y1, y2)
}

// How the slashes of a tremolo are drawn.
const (
	// How far from the notehead the slashes start, along the stem. Drawn
	// against the head they read as part of it.
	tremoloFromHead = 1.1
	// And how far they are kept from the end of the stem, so they do not
	// run into a flag or a beam.
	tremoloFromEnd = 0.9
)

// drawTremolo draws the slashes that stand for a repeated note.
//
// They sit on the stem, between the notehead and whatever is at the other
// end of it, and they are one glyph rather than a drawn stack: the font
// carries tremolo1 to tremolo5 with the slant, the thickness and the spacing
// already decided, and a hand-drawn set of parallelograms would be a second
// opinion about all three.
//
// A note with no stem to put them on — a whole note — has them centred over
// the notehead instead, which is where every published score puts them.
func (e *engraver) drawTremolo(p *placed, sx, headY, endY float64) {
	m := p.marks()
	if m == nil || m.Tremolo == nil {
		return
	}
	name := tremoloGlyph(m.Tremolo)
	if name == "" {
		return
	}
	if p.value <= model.Whole {
		// No stem: over the notehead, clear of it.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: name, X: p.x + noteheadWidth(p.value)/2,
			Y: e.y + stepY(float64(p.hi)) - 2.2,
		})
		return
	}

	// Along the stem, kept off both ends. A short stem gets them in the
	// middle, which is the best that can be done with the room there is.
	lo, hi := headY, endY
	if lo > hi {
		lo, hi = hi, lo
	}
	y := (lo + hi) / 2
	if headY < endY { // stem down: the head is at the top
		if to := headY + tremoloFromHead; y < to {
			y = to
		}
		if to := endY - tremoloFromEnd; y > to {
			y = to
		}
	} else {
		if to := headY - tremoloFromHead; y > to {
			y = to
		}
		if to := endY + tremoloFromEnd; y < to {
			y = to
		}
	}
	// Drawn from the stem itself, not from half a glyph's width to the left
	// of it: the tremolo glyphs are registered on their own centre, so that
	// is where the stem goes through them. Taking half the advance off — the
	// way a glyph with its origin at its left edge would need — hung the
	// slashes off the side of the stem instead of across it.
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: sx, Y: y})
}

// tremoloGlyph names the mark: the slashes, or the z of a buzz roll.
func tremoloGlyph(t *model.Tremolo) string {
	if t == nil {
		return ""
	}
	if t.Buzz {
		return "buzzRoll"
	}
	return "tremolo" + strconv.Itoa(t.Beams())
}

// drawBeamGroup draws the stems of a beamed run and the beams across them.
func (e *engraver) drawBeamGroup(g []*placed) {
	d := e.opt.Defaults

	// One direction serves the whole group, decided in the one place that
	// decides it, so that what is drawn here and what the slurs and dots
	// were placed against cannot come apart.
	up := beamGroupUp(g)

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
		ends = append(ends, end{x: sx, y: e.stemEnd(base, up), p: p})
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
		// A beamed note can carry a tremolo too, and then the slashes go
		// between the notehead and the beam rather than on a bare stem.
		e.drawTremolo(en.p, en.x, base, beamY(en.x))
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
	// Vertical ends, flush with the stems. This was a thick stroked line,
	// on the grounds that the difference did not show at these sizes — and
	// it does: a line's caps are square to the line, so a beam sloping by a
	// third of a space per space came out with a point at each end hanging
	// past the outer stem. dir says which side of the join the beam's body
	// lies on, so the near edge runs exactly where the stems stop.
	e.lay.Quads = append(e.lay.Quads, Quad{
		X: [4]float64{x0, x1, x1, x0},
		Y: [4]float64{y0, y1, y1 + dir*thickness, y0 + dir*thickness},
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
		// Past the dot just drawn, then the gap. Advancing by the gap alone
		// left each dot after the first sitting on top of the one before it,
		// by the width of a dot less the gap — which a double-dotted note
		// showed as a single fat blot.
		x += dotStep(e.opt.Spacings)
	}
}

// dotStep is the distance from one dot to the next: the dot itself, then the
// air after it.
func dotStep(sp Spacings) float64 {
	return smufl.Advance("augmentationDot") + sp.BetweenDots
}

// dotGap is how far after a notehead its dots begin.
//
// Measured from whatever is furthest right, which for a note whose stem goes
// up is the stem rather than the notehead: the stem rises from the notehead's
// right edge, so a dot placed off the notehead alone is crowded against it.
func (e *engraver) dotGap(stemUp bool) float64 {
	gap := e.opt.Spacings.DotToNote
	if stemUp {
		gap += e.opt.Defaults.StemThickness + 0.12
	}
	return gap
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
	// Below the staff, clear of whatever is under this stretch of it: the
	// stems that point down, the ledger lines, the words. Where that is
	// cannot be known yet, so the dynamic waits for the staff to finish.
	x := p.x + noteheadWidth(p.value)/2 - smufl.Advance(name)/2
	w := smufl.Advance(name)
	e.wait(waitingMark{
		x0: x, x1: x + w, height: 1.4, gap: 0.7, below: true, rank: rankDynamic,
		draw: func(y float64) {
			// place gives the top of the band; a dynamic is drawn from its
			// baseline, which sits near the bottom of its own letters.
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: x, Y: y + 1.0})
		},
	})
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
		y := e.y - 2.2 - e.textRise
		if t.Placement == model.PlacementBelow {
			y = e.y + staffHeight + 3.4
		} else if p.hi > 10 && e.textRise == 0 {
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
	// Articulations sit on the side away from the stem, clear of both the
	// note and the staff.
	//
	// Clear of *both*: whichever of the two is further out wins. A note
	// inside the staff needs its mark pushed out past the lines, and a note
	// above them needs it further out still. Forcing the mark to the staff's
	// edge did the first and undid the second — for anything above the top
	// line the edge of the staff is below the notehead, so the mark was
	// dragged down onto the note it belonged to.
	above := !stemUp
	noteY := e.y + stepY(float64(step))
	gap := 1.3
	var y float64
	if above {
		// y grows downward, so further out is smaller.
		y = math.Min(noteY-gap, e.y-1.2)
	} else {
		y = math.Max(noteY+gap, e.y+staffHeight+1.2)
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

// displacedHeads says which noteheads of a chord are pushed to the other
// side of the stem.
//
// Two noteheads a second apart cannot share a column — they would be drawn
// one through the other — so one of each pair moves across. Which one is not
// a free choice: the chord is read from the stem outwards, the note nearest
// the stem stays where it is, and from there they alternate.
//
// Alternate, which is the part that was missing. The old rule was "move me
// if anything is a second away from me", and in a cluster of three that
// moved the middle note and the top one both — so the top two ended up in
// the same column a second apart, which is the one thing the rule exists to
// prevent. A cluster of a third note has to come back to the first column.
func displacedHeads(steps []int, up bool) []bool {
	out := make([]bool, len(steps))
	if len(steps) < 2 {
		return out
	}
	// Read from the stem outwards: up the chord for a stem that points up,
	// down it for one that points down.
	order := make([]int, len(steps))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		if up {
			return steps[order[a]] < steps[order[b]]
		}
		return steps[order[a]] > steps[order[b]]
	})

	prev := steps[order[0]]
	prevShifted := false
	for _, i := range order[1:] {
		// A second from the note before it, and only if that one stayed:
		// two moved notes in a row would land in the same column again.
		shift := abs(steps[i]-prev) == 1 && !prevShifted
		out[i] = shift
		prev, prevShifted = steps[i], shift
	}
	return out
}
