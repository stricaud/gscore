/*
 * spanners.go
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

// The marks that reach from one note to another: hairpins, octave lines and
// the volta brackets over the bars of a repeat.
//
// None of these can be drawn where they begin, because where they end is not
// known yet and may be on a later line. So each staff carries what is open,
// and the mark is drawn when it closes — or cut off at the right margin and
// started again on the next system, which is how a crescendo spanning a page
// turn is written on paper.

const (
	// hairpinDrop is how far below the bottom staff line a hairpin sits. It
	// is under the dynamics, since a hairpin usually runs from one to
	// another and crossing them would be unreadable.
	hairpinDrop = 4.6
	// hairpinMouth is how far the wedge opens, top to bottom.
	hairpinMouth = 1.1

	// octaveRise is how far above the top staff line an 8va line runs, and
	// octaveFall how far below the bottom line an 8vb does. Both are now
	// only the least it will be: the line is placed clear of the music
	// under it, which where an octave line is wanted is usually higher.
	octaveRise = 3.4
	octaveFall = 4.6

	// octaveBand is how tall the sign and its line are together: the line,
	// and the "8va" hanging off its left end.
	octaveBand = 2.3

	// trillBand is how tall "tr" and its wavy line are together.
	trillBand = 1.8

	// voltaRise is how far above the top staff of the system the brackets of
	// a repeat are drawn, and voltaHook how far their ends turn down.
	voltaRise = 3.0
	voltaHook = 1.1
)

// resumeRoom is the blank held back at the head of a system, after the meter
// and before the first bar, for the ties and slurs carried over from the
// system above, in staff spaces.
//
// Without it a continuation had nowhere to start but the column the music
// starts in, which is where the note it joins already is: the curve came out
// a notehead wide, and a curve that short reads as an accent over the note
// rather than as something arriving from the line above.
const resumeRoom = 1.6

// carriedCurves says whether any staff is waiting to pick a slur or a tie up
// again, which is what the room at the head of a system is reserved for.
func (e *engraver) carriedCurves() bool {
	for _, open := range e.spans {
		if open.slur != nil && open.slur.x < 0 {
			return true
		}
		for _, t := range open.ties {
			if t.x < 0 {
				return true
			}
		}
	}
	return false
}

// allVoices is the voice number of the record a staff keeps in common.
//
// Most spanners belong to a voice: a slur is a phrase, and two voices on one
// staff phrase separately. A tie does not. It joins two notes of the same
// pitch, and in piano writing a chord in one voice routinely resolves into
// notes that the next bar has split between two — the Altes does it at bar
// 17, where an F and an A flat are tied together and arrive in different
// voices. Looked for in its own voice, the F's tie found no note to end on
// and was drawn to the end of the system: a line that starts and never ends.
const allVoices = -1

// staffSpans returns what is open on a staff, making the record if this is the
// first spanner it has seen.
func (e *engraver) staffSpans(part, voice int) *openSpans {
	key := [2]int{part, voice}
	s, ok := e.spans[key]
	if !ok {
		s = &openSpans{}
		e.spans[key] = s
	}
	return s
}

// spanAt handles the spanners that open or close at one element.
func (e *engraver) spanAt(m *model.Marks, p *placed) {
	if m == nil {
		return
	}
	// Hairpins and octave lines belong to the staff rather than to one of
	// its voices, and are kept with the voice they were written in.
	open := e.staffSpans(e.partIndex, p.voice)
	// The middle of the notehead, so a hairpin between two notes reaches from
	// under one to under the other rather than from their left edges.
	x := p.x + noteheadWidth(p.value)/2

	if h := m.Hairpin; h != nil {
		if h.Start && open.hairpin == nil {
			open.hairpin = &openSpan{x: x, kind: h.Kind}
		}
		if h.Stop && open.hairpin != nil {
			e.drawHairpin(open.hairpin, x, true)
			open.hairpin = nil
		}
	}

	if tr := m.Trill; tr != nil {
		if tr.Start && open.trill == nil {
			open.trill = &openSpan{x: x - 0.5}
		}
		if tr.Stop && open.trill != nil {
			e.drawTrill(open.trill, x+noteheadWidth(p.value)/2, true)
			open.trill = nil
		}
	}

	if o := m.Octave; o != nil {
		if o.Start && open.octave == nil {
			open.octave = &openSpan{x: x - 0.5, amount: o.Amount, label: o.Label()}
		}
		if o.Stop && open.octave != nil {
			e.drawOctave(open.octave, x+noteheadWidth(p.value)/2, true)
			open.octave = nil
		}
	}
}

// breakSpans ends every open spanner at the right margin of a system and
// leaves it open, so the next system starts it again at its own left edge.
func (e *engraver) breakSpans(part int, right float64) {
	for key, open := range e.spans {
		if key[0] != part {
			continue
		}
		e.breakSpansOf(open, right)
	}
}

func (e *engraver) breakSpansOf(open *openSpans, right float64) {
	if open.hairpin != nil {
		e.drawHairpin(open.hairpin, right, false)
		open.hairpin.x = -1 // resumed at the start of the next system
	}
	if open.octave != nil {
		e.drawOctave(open.octave, right, false)
		open.octave.x = -1
	}
	if open.trill != nil {
		e.drawTrill(open.trill, right, false)
		open.trill.x = -1
		// The "tr" is written once, where the trill begins; the next system
		// carries the wavy line alone, as it is written on paper.
		open.trill.label = "done"
	}
	// A slur that runs over a line break is drawn to the margin and picked
	// up again on the next line, which is how it is written on paper.
	if open.slur != nil {
		e.drawTieOrSlur(open.slur, right, open.slur.y, slurLift, true)
		open.slur.x = -1
	}
	for _, t := range open.ties {
		e.drawTieOrSlur(t, right, t.y, tieLift, false)
		t.x = -1
	}
}

// resumeSpans picks a broken spanner up again at the left of a new system.
func (e *engraver) resumeSpans(part int, left float64) {
	for key, open := range e.spans {
		if key[0] != part {
			continue
		}
		e.resumeSpansOf(open, left)
	}
}

func (e *engraver) resumeSpansOf(open *openSpans, left float64) {
	if open.hairpin != nil && open.hairpin.x < 0 {
		open.hairpin.x = left
	}
	if open.octave != nil && open.octave.x < 0 {
		open.octave.x = left
		// The "8va" is written once, where the line begins. A continuation
		// carries the line alone, as it is written on paper.
		open.octave.label = ""
	}
	// A slur or a tie comes back at the height it left, on this system's
	// staff rather than on the one it started from.
	if open.trill != nil && open.trill.x < 0 {
		open.trill.x = left
	}
	if open.slur != nil && open.slur.x < 0 {
		open.slur.x = left
		open.slur.y = e.y + open.slur.rel
	}
	for _, t := range open.ties {
		if t.x < 0 {
			t.x = left
			t.y = e.y + t.rel
		}
	}
}

// drawHairpin draws the wedge. A crescendo opens to the right from a point; a
// diminuendo starts open and closes to a point.
func (e *engraver) drawHairpin(h *openSpan, to float64, complete bool) {
	if to <= h.x {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.2
	half := hairpinMouth / 2

	// An unfinished wedge is cut at the margin rather than brought to a
	// point, which is what says "this goes on".
	openAt, pointAt := to, h.x
	if h.kind == model.Diminuendo {
		openAt, pointAt = h.x, to
	}
	spread := half
	if !complete && openAt == to {
		// Opening but cut short: it has not reached its full mouth yet.
		spread = half * 0.8
	}
	cut := !complete && pointAt == to

	// A hairpin runs under a stretch of music and has to clear all of it —
	// the stems, the slurs that dip below the staff, the dynamics it runs
	// between. None of that is known until the staff is finished, and a
	// hairpin is not even drawn until the bar it closes in, so it waits.
	e.wait(waitingMark{
		x0: h.x, x1: to, height: hairpinMouth, gap: 0.6,
		below: true, rank: rankHairpin,
		draw: func(top float64) {
			y := top + half
			if cut {
				// Closing but cut short: it never reaches the point.
				e.lay.Lines = append(e.lay.Lines,
					Line{X1: openAt, Y1: y - half, X2: to, Y2: y - half*0.3, Thickness: t},
					Line{X1: openAt, Y1: y + half, X2: to, Y2: y + half*0.3, Thickness: t})
				return
			}
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: pointAt, Y1: y, X2: openAt, Y2: y - spread, Thickness: t},
				Line{X1: pointAt, Y1: y, X2: openAt, Y2: y + spread, Thickness: t})
		},
	})
}

// drawOctave draws the broken line with its name at the left, and the hook at
// the end that says where the shift stops.
func (e *engraver) drawOctave(o *openSpan, to float64, complete bool) {
	if to <= o.x {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.1
	above := o.amount > 0
	label := o.label

	// The sign and its line run over a stretch of music and have to clear
	// all of it — the notes on their ledger lines above all, since an
	// octave line is written exactly where the music is already high. What
	// that stretch holds is not known until the staff is finished.
	e.wait(waitingMark{
		x0: o.x, x1: to, height: octaveBand, gap: 0.5,
		below: !above, rank: rankOctave,
		draw: func(edge float64) {
			// place gives the edge nearest the staff; the line sits at the
			// far side of the band, with the sign hanging off its left end.
			y := edge - 0.7
			if !above {
				y = edge + 1.6
			}
			x := o.x
			if label != "" {
				e.lay.Texts = append(e.lay.Texts, Text{
					S: label, X: x, Y: y + 0.4, Size: 1.9,
					Anchor: "start", Italic: true, Bold: true,
				})
				x += float64(len(label)) * 0.95
			}
			if to <= x {
				return
			}
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: x, Y1: y, X2: to, Y2: y, Thickness: t, Dash: 0.5})
			if complete {
				// The hook turns towards the staff: down for a line above
				// it, up for one below.
				dy := 0.9
				if !above {
					dy = -0.9
				}
				e.lay.Lines = append(e.lay.Lines,
					Line{X1: to, Y1: y, X2: to, Y2: y + dy, Thickness: t})
			}
		},
	})
}

// drawVoltas draws the first-time and second-time brackets over a system.
//
// They are drawn once, above the top staff, because a repeat belongs to the
// piece and not to one instrument in it: every player goes back at the same
// bar. The measures are taken from the first part that has them, which is
// where MusicXML keeps them too.
func (e *engraver) drawVoltas(parts []*model.Part, columns []float64, start, end int) {
	if len(parts) == 0 || end <= start {
		return
	}
	// Nothing to draw unless some bar of this system carries a bracket; a
	// system without one must not reserve a row for it.
	any := false
	for _, p := range parts {
		for mi := start; mi < end && mi < len(p.Measures); mi++ {
			if p.Measures[mi].Ending != nil {
				any = true
			}
		}
	}
	if !any {
		return
	}
	e.wait(waitingMark{
		x0: columns[0], x1: columns[len(columns)-1],
		height: voltaHook + 0.4, gap: 0.8, rank: rankVolta,
		draw: func(y float64) { e.drawVoltasAt(parts, columns, start, end, y) },
	})
}

// drawVoltasAt draws the brackets once the row they share has been found.
//
// One height for the whole system's brackets, clear of everything under any
// of them: a bracket that stepped up and down over the bars it covers would
// not read as one bracket.
func (e *engraver) drawVoltasAt(parts []*model.Part, columns []float64, start, end int, y float64) {
	t := e.opt.Defaults.StaffLineThickness * 1.4

	var open *model.Ending
	openX := 0.0

	closeAt := func(x float64, hook bool) {
		if open == nil {
			return
		}
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: openX, Y1: y, X2: x, Y2: y, Thickness: t},
			Line{X1: openX, Y1: y, X2: openX, Y2: y + voltaHook, Thickness: t})
		if hook && !open.Open {
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: x, Y1: y, X2: x, Y2: y + voltaHook, Thickness: t})
		}
		if s := open.Label(); s != "" {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: s, X: openX + 0.5, Y: y + voltaHook - 0.15, Size: 1.9, Anchor: "start",
			})
		}
		open = nil
	}

	for mi := start; mi < end; mi++ {
		var end *model.Ending
		for _, p := range parts {
			if mi < len(p.Measures) && p.Measures[mi].Ending != nil {
				end = p.Measures[mi].Ending
				break
			}
		}
		if end == nil {
			continue
		}
		if end.Start {
			closeAt(columns[mi-start], false)
			open, openX = end, columns[mi-start]
		}
		if end.Stop {
			closeAt(columns[mi-start+1], true)
		}
	}
	// A bracket still open at the end of the line is cut at the margin and
	// has no closing hook; the next system opens one again.
	closeAt(columns[end-start], false)
}

/*
 * Slurs and ties
 */

// A slur arcs over the notes it covers; a tie joins two noteheads of the same
// pitch. They are drawn the same way and mean different things, which is why
// they are kept apart in the model and share their geometry here.
//
// Both arc away from the stems, as articulations sit away from them: a curve
// on the stem side has to clear the whole stem and ends up somewhere no
// reader expects. Where the stems disagree, the note the curve starts on
// decides, because that is where the reader's eye picks it up.

const (
	// slurLift is how far a slur's ends stand off the noteheads.
	slurLift = 0.9
	// slurArch is how far the middle rises above a line drawn between the
	// ends, before the span is taken into account.
	slurArch = 1.0
	// slurArchPerSpace is how much more it rises for each space it covers,
	// so a long slur is not a flat smear.
	slurArchPerSpace = 0.09
	// slurArchMax keeps a very long slur from arching off the page.
	slurArchMax = 3.2

	// A tie is shorter and flatter: it joins two noteheads rather than
	// covering a phrase, so it stays close to them.
	tieLift = 0.55
	tieArch = 0.7

	// slurClearance is the air a slur leaves between itself and the music it
	// arcs over.
	slurClearance = 0.6

	// archPerWidth is the most of its own width a curve may rise by, which
	// is what keeps a very short one — the stub left at a line break — an
	// arc rather than a spike.
	archPerWidth = 0.35
)

// slurAt handles a slur or a tie opening or closing at one element.
func (e *engraver) slurAt(part *model.Part, m *model.Marks, p *placed) {
	if m == nil {
		return
	}
	open := e.staffSpans(e.partIndex, p.voice)
	x := p.x + noteheadWidth(p.value)/2

	// Which way this note's stem points, counted into whatever slur is open
	// so that the side can be settled from the phrase rather than from the
	// note it happened to begin on.
	if open.slur != nil {
		if e.stemUp(p) {
			open.slur.up++
		} else {
			open.slur.down++
		}
		if p.lo < open.slur.lowest {
			open.slur.lowest = p.lo
		}
		if p.hi > open.slur.highest {
			open.slur.highest = p.hi
		}
	}

	// The side this note alone would put the curve on, which is what a tie
	// uses and what a slur falls back to when it covers one note.
	above := !e.stemUp(p)

	// Closing first, then opening.
	//
	// A note in the middle of a run of slurs carries both: it ends the one
	// before it and begins the one after. Opening first found a slur
	// already open and did nothing, and then the close took that one away —
	// so a phrase slurred bar by bar came out with its first slur drawn and
	// the rest missing. Four whole notes joined by three slurs showed one.
	// An appoggiatura's slur starts on the small note and stops on this
	// one, so it has to be opened before the closing below or the stop
	// finds nothing open and the curve is never drawn.
	if open.slur == nil {
		if gx, gstep, ok := e.graceSlurAnchor(part, p); ok {
			gy := e.y + stepY(float64(gstep))
			open.slur = &openSpan{
				x: gx, y: gy, rel: gy - e.y, above: true,
				lowest: gstep, highest: gstep,
			}
		}
	}

	if m.SlurStop && open.slur != nil {
		sl := open.slur
		// Away from the stems of the notes it covers. A tie between them
		// settles it the way one note would.
		sl.above = sl.down >= sl.up
		edge, toEdge := sl.highest, p.hi
		if !sl.above {
			edge, toEdge = sl.lowest, p.lo
		}
		sl.y = e.y + stepY(float64(edge))
		e.drawTieOrSlur(sl, x, e.y+stepY(float64(toEdge)), slurLift, true)
		open.slur = nil
	}
	if m.SlurStart && open.slur == nil {
		edge := p.hi
		if !above {
			edge = p.lo
		}
		y := e.y + stepY(float64(edge))
		open.slur = &openSpan{
			x: x, y: y, rel: y - e.y, above: above,
			lowest: p.lo, highest: p.hi,
		}
		if e.stemUp(p) {
			open.slur.up++
		} else {
			open.slur.down++
		}
	}

	// A tie is written on both notes: it stops on one and may start again on
	// the same one, which is how a note held through several bars is
	// spelled. Each notehead is tied separately, so a chord gets one curve
	// per note rather than one for the whole chord.
	//
	// Held for the staff and not for the voice: see allVoices.
	tied := e.staffSpans(e.partIndex, allVoices)
	for _, h := range e.tiedHeads(p, m) {
		if h.stop {
			if t, ok := tied.ties[h.step]; ok {
				e.drawTieOrSlur(t, x, e.y+stepY(float64(h.step)), tieLift, false)
				delete(tied.ties, h.step)
			}
		}
		if h.start {
			if tied.ties == nil {
				tied.ties = map[int]*openSpan{}
			}
			if _, already := tied.ties[h.step]; !already {
				tied.ties[h.step] = &openSpan{
					x: x, y: e.y + stepY(float64(h.step)),
					rel: stepY(float64(h.step)), above: h.above,
				}
			}
		}
	}
}

// tiedHead is one notehead of an element, which way its tie arcs, and
// whether a tie begins or ends on it.
type tiedHead struct {
	step        int
	above       bool
	start, stop bool
}

// tiedHeads lists the noteheads of an element that carry a tie.
//
// Each notehead of a chord is tied on its own account. Holding the top note
// of a chord while the others move is ordinary writing, so a tie on a chord
// cannot mean "all of it" and nothing else — the notes keep their own marks
// and a mark on the chord means all of them, which is the common case said
// briefly.
//
// A single note's tie arcs away from its stem, as a slur does. A chord's ties
// arc outward from the middle: the upper notes curve up and the lower ones
// down, so the curves open away from each other instead of crossing the
// noteheads between them.
func (e *engraver) tiedHeads(p *placed, m *model.Marks) []tiedHead {
	if p.note != nil {
		return []tiedHead{{
			step: p.step, above: !e.stemUp(p),
			start: m.TieStart, stop: m.TieStop,
		}}
	}
	if p.chord == nil {
		return nil
	}
	middle := float64(p.lo+p.hi) / 2
	out := make([]tiedHead, 0, len(p.chord.Notes))
	for _, n := range p.chord.Notes {
		step := StaffStep(e.clef, n.Pitch)
		out = append(out, tiedHead{
			step: step, above: float64(step) >= middle,
			// The chord's own mark covers every notehead; a notehead's own
			// mark covers just itself.
			start: m.TieStart || n.TieStart,
			stop:  m.TieStop || n.TieStop,
		})
	}
	return out
}

// drawTieOrSlur draws the arc. A quadratic is enough: a slur is one curve,
// and its shape is decided by where it starts, where it ends and how far it
// reaches.
func (e *engraver) drawTieOrSlur(from *openSpan, toX, toY, lift float64, slur bool) {
	if toX <= from.x {
		return
	}
	dir := -1.0 // y grows downward, so "above" is negative
	if !from.above {
		dir = 1
	}

	y1 := from.y + dir*lift
	y2 := toY + dir*lift

	span := toX - from.x
	arch := tieArch
	if slur {
		arch = slurArch + span*slurArchPerSpace
		if arch > slurArchMax {
			arch = slurArchMax
		}
	}
	// An arc is only an arc while it is wider than it is tall. The stub a
	// tie leaves at a line break is barely wider than a notehead, and rising
	// its full height over that reads as a caret stabbing the note rather
	// than as a curve — which is what it was drawn as.
	if lid := span * archPerWidth; arch > lid {
		arch = lid
	}
	// The control point of a quadratic pulls the curve about halfway
	// towards it, so it is set twice as far out as the curve should go.
	cy := (y1+y2)/2 + dir*arch*2

	// A slur has to clear the music it arcs over.
	//
	// Its ends belong to its notes and stay there, but its middle is free,
	// and an arch of a fixed height is only clear of a phrase that happens
	// to be flat. Over a rising run it cut straight through the notes
	// between its ends — which is exactly where a slur must not be, because
	// the line it draws is the phrase.
	//
	// Not a tie: a tie joins two noteheads of one pitch and there is nothing
	// between them to clear.
	if slur {
		cy = e.clearArch(from.x, toX, y1, y2, cy, dir)
	}

	e.lay.Curves = append(e.lay.Curves, Curve{
		X1: from.x, Y1: y1,
		CX: (from.x + toX) / 2, CY: cy,
		X2: toX, Y2: y2,
		Thickness: e.opt.Defaults.StaffLineThickness * 1.6,
	})
}

// clearArch lifts a slur's control point until the curve clears what it arcs
// over, and leaves it alone when it already does.
//
// A quadratic passes through the point a quarter of the way from each end
// and half of the way towards the control point, so the height of the arch
// at its middle is that average; turning the wanted middle back into a
// control point is the same arithmetic the other way round.
func (e *engraver) clearArch(x0, x1, y1, y2, cy, dir float64) float64 {
	if x1 <= x0 {
		return cy
	}
	sky := e.outline(e.staffMark, e.staffX0, e.staffX1, e.staffTop)

	// Only what lies between the ends: the notes the slur starts and ends
	// on are its own, and a slur is drawn from them, not over them.
	inset := (x1 - x0) * 0.15
	a, b := x0+inset, x1-inset
	if b <= a {
		return cy
	}

	above := dir < 0
	want := sky.reach(a, b, !above)
	if above {
		want -= slurClearance
	} else {
		want += slurClearance
	}

	// The middle of the curve as it stands, and the control point that would
	// put the middle where it is wanted instead.
	mid := 0.25*y1 + 0.5*cy + 0.25*y2
	if (above && want >= mid) || (!above && want <= mid) {
		return cy // already clear
	}
	return 2*want - 0.5*(y1+y2)
}

// drawTrill writes "tr" and the wavy line that holds it.
//
// A trill over one note is the ornament alone; over a stretch of music it is
// the ornament and a line to where it stops, and the line is what says to go
// on trilling. The wavy line is a glyph repeated rather than a drawn curve,
// so it looks like the font it is set in at any size.
//
// It goes above the staff and nearer to it than anything else written there,
// which is LilyPond's order and the one printed music uses: a trill belongs
// to its note, so it stays with it and the rest stacks over the top.
func (e *engraver) drawTrill(t *openSpan, to float64, complete bool) {
	if to <= t.x {
		return
	}
	first := t.label == "" // the "tr" is written once, at the start
	from := t.x
	e.wait(waitingMark{
		x0: from, x1: to, height: trillBand, gap: 0.5, rank: rankTrill,
		draw: func(edge float64) {
			y := edge - 0.4
			x := from
			if first {
				e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
					Name: "ornamentTrill", X: x, Y: y,
				})
				x += smufl.Advance("ornamentTrill") + 0.2
			}
			// The wave is only drawn where there is room for it: a trill on
			// a single note is the ornament and nothing else.
			w := smufl.Advance("wiggleTrill")
			if w <= 0 {
				return
			}
			for ; x+w <= to; x += w {
				e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
					Name: "wiggleTrill", X: x, Y: y,
				})
			}
		},
	})
}
