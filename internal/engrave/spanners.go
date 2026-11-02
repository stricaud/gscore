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
	"math"
	"sort"
	"strings"

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
func (e *engraver) spanAt(m *model.Marks, p *placed, measure int) {
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
			open.hairpin = &openSpan{
				x: x, kind: h.Kind,
				ref: p.index, part: e.partIndex, measure: measure,
			}
		}
		if h.Stop && open.hairpin != nil {
			open.hairpin.toRef = p.index
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

	// A slide from this note to the next. Drawn at the moment it closes,
	// because a line between two noteheads needs both of them placed.
	if g := m.Glissando; g != nil {
		if g.Start && open.gliss == nil {
			open.gliss = &openSpan{
				x: x, y: e.y + stepY(float64(p.step)), ref: p.index,
				part: e.partIndex, measure: measure,
				label: g.Text, above: g.Wavy,
			}
		}
		if g.Stop && open.gliss != nil {
			open.gliss.toRef = p.index
			e.drawGlissando(open.gliss, x, e.y+stepY(float64(p.step)), p)
			open.gliss = nil
		}
	}

	// The sustaining pedal.
	if pd := m.Pedal; pd != nil {
		if pd.Start && open.pedal == nil {
			open.pedal = &openSpan{
				x: x, ref: p.index, part: e.partIndex, measure: measure,
				above: pd.Text, amount: boolToInt(pd.Half),
			}
			open.notches = nil
		}
		if pd.Change && open.pedal != nil {
			open.notches = append(open.notches, x)
		}
		if pd.Stop && open.pedal != nil {
			open.pedal.toRef = p.index
			e.drawPedal(open.pedal, x, open.notches, true)
			open.pedal = nil
			open.notches = nil
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
//
// In voice order, because this draws. A map in Go is walked in a different
// order every run, and a drawing whose marks come out in a different order
// each time is a drawing that cannot be compared with itself: the golden
// layouts hashed one order on one machine and another on the next, and the
// score was identical both times.
func (e *engraver) breakSpans(part int, right float64) {
	keys := make([][2]int, 0, len(e.spans))
	for key := range e.spans {
		if key[0] == part {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a][1] < keys[b][1] })
	for _, key := range keys {
		e.breakSpansOf(e.spans[key], right)
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
		e.drawTieOrSlur(open.slur, right, open.slur.y, slurLift, true, false)
		open.slur.x = -1
	}
	// Up the staff, for the same reason: a chord held over a line break
	// leaves one stub per note, and which was drawn first decided the order
	// of the whole layout.
	for _, step := range sortedSteps(open.ties) {
		t := open.ties[step]
		e.drawTieOrSlur(t, right, t.y, tieLift, false, false)
		t.x = -1
	}
}

// sortedSteps lists a tie map's noteheads from the bottom of the staff up.
func sortedSteps(ties map[int]*openSpan) []int {
	out := make([]int, 0, len(ties))
	for step := range ties {
		out = append(out, step)
	}
	sort.Ints(out)
	return out
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
	kind := "crescendo"
	if h.kind == model.Diminuendo {
		kind = "diminuendo"
	}
	e.wait(waitingMark{
		x0: h.x, x1: to, height: hairpinMouth, gap: 0.6,
		below: true, rank: rankHairpin,
		draw: func(top float64) {
			// Where it ended up, so the editor can let it be clicked. Only
			// here is that known: a hairpin waits for the staff to finish
			// before it is told how far down it goes.
			if complete {
				e.lay.Hairpins = append(e.lay.Hairpins, HairpinPos{
					Part: h.part, Measure: h.measure,
					From: h.ref, To: h.toRef,
					X: h.x, Y: top, Width: to - h.x, Height: hairpinMouth,
					Kind: kind,
				})
			}
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
		// Further off the music than most marks. The line is the outermost
		// thing over a passage that is already high, and half a space from
		// the words under it — "sempre più rinforzando" at bar 149 of
		// Danse Macabre — read as part of them.
		x0: o.x, x1: to, height: octaveBand, gap: 0.9,
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

	// slurRiseMax is how far the middle of a slur may stand off the line
	// between its ends before the ends are brought up to the music
	// instead. Past this a curve stops reading as a slur and starts
	// reading as a dome over the staff.
	slurRiseMax = 2.6
	// slurEndLift is the most an end may rise off its own note to meet a
	// phrase that climbs away from it. Further than this and the curve is
	// no longer attached to anything a reader can see.
	slurEndLift = 4.0
	// slurOffCentre is how far from the middle of a slur the worst of what
	// it has to clear must stand before the ends are lifted rather than the
	// arch raised.
	//
	// Which of the two to move is not a matter of taste. A quadratic's
	// control point moves the curve by twice t(1-t): the whole of it in the
	// middle, and less and less towards the ends, so clearing a note at the
	// middle costs twice its height in control point and clearing one a
	// sixth of the way along costs six times. In the middle the arch is the
	// cheap answer and the ends should stay on their notes; out towards an
	// end it is ruinous — the Altes has a five-note phrase over a beam two
	// thirds of the way along, and arching over that one beam threw the
	// middle of the curve eight spaces above the staff. There the ends are
	// the cheap answer.
	//
	// This replaced a rule that asked how wide the slur was. Width says
	// nothing about the question: a slur over two whole notes is wider than
	// a run of five eighths, and it was the run — refused its lift — that
	// came out as a spike.
	slurOffCentre = 0.15

	// slurShoulderLift is the most a slur's end may rise off its own note to
	// get over what stands beside it.
	//
	// Much less than slurEndLift, and for a different reason. That one is a
	// phrase climbing away from where it began, and the whole curve goes
	// with it; this is one beam or one stem next to the note the slur ends
	// on, and the curve comes back down to the staff straight afterwards.
	// An end raised far for a local obstacle reads as belonging to no note
	// at all.
	slurShoulderLift = 2.0

	// slurColumnLift is the most a slur's end may rise to clear what else
	// is written in its own note's column.
	//
	// The length of a stem and a little over, because a stem with a beam
	// on the end of it is the tallest thing a note's column can hold, and
	// an end that has risen past the top of that is no longer rising over
	// anything.
	slurColumnLift = stemLength + 1.5

	// slurOwnColumn is how much of each end of a slur is its own note's,
	// and so not something the curve has to clear: a notehead and the stem
	// alongside it.
	slurOwnColumn = 1.6
	// slurShoulderIn is how far in from each end the arch takes over. What
	// lies outside it, between the end's own note and that point, is the
	// shoulder, and is cleared by raising the end rather than by arching.
	slurShoulderIn = 0.25

	// archPerWidth is the most of its own width a curve may rise by, which
	// is what keeps a very short one — the stub left at a line break — an
	// arc rather than a spike.
	archPerWidth = 0.35
)

// stemSlurInset is how far inside the stem's end a slur hangs.
//
// Not level with the tip: a curve that starts exactly at the end of the
// stem reads as part of it. A little way down the stem and the two are
// plainly different things, which is where printed music puts it.
func stemSlurInset(above bool) float64 {
	if above {
		return 0.4
	}
	return -0.4
}

// slurAt handles a slur or a tie opening or closing at one element.
func (e *engraver) slurAt(part *model.Part, m *model.Marks, p *placed, measure int) {
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
				fromHi: gstep, fromLo: gstep,
				ref: p.index, part: e.partIndex, measure: measure,
			}
		}
	}

	if m.SlurStop && open.slur != nil {
		sl := open.slur
		// A slur that came over a line break has no note at its left end.
		//
		// That end is the margin this system started at, put there by
		// resumeSpans at the height the curve left the system above, and the
		// note the phrase began on is on another system — on another page as
		// often as not. Everything below works the left end out from that
		// note's own pitch, stem and staff, which on this system is a place
		// hundreds of spaces away: the curve came out as a straight line
		// drawn diagonally across the page, from where the phrase began to
		// where it ends. The piece already drawn above also settled which
		// side the curve arcs on, so that is left alone too — the two halves
		// of one slur cannot arch opposite ways.
		carried := len(sl.stubs) > 0
		if !carried {
			// Away from the stems of the notes it covers — and above wherever
			// they point both ways, there being no side away from them then.
			sl.above = sl.down > 0 || sl.up == 0
		}
		// Each end on its own note. What lies between them is lifted over
		// by the arch, not by dragging the ends out to meet it.
		edge, toEdge := sl.fromHi, p.hi
		if !sl.above {
			edge, toEdge = sl.fromLo, p.lo
		}
		// From the stem, where the slur is on the side the stems are on.
		//
		// A slur goes away from the stems and usually that settles it, but
		// two lines of music on one staff do not allow it: the upper voice
		// stems up and is slurred above all the same. Anchored at the
		// notehead there, the curve has to climb over its own stems, and a
		// phrase of three notes came out as a dome higher than the staff.
		// Printed music hangs it off the stem ends instead, which is what
		// makes those slurs the shallow arcs they are.
		fromY := e.y + stepY(float64(edge))
		fromOnStem := sl.above == sl.fromStemUp
		if fromOnStem {
			fromY = sl.fromStemY + stemSlurInset(sl.above)
		}
		toUp := e.stemUp(p)
		toBase := e.y + stepY(float64(toEdge))
		toStemY := e.stemEnd(e.y+stepY(float64(p.hi)), toUp)
		if !toUp {
			toStemY = e.stemEnd(e.y+stepY(float64(p.lo)), toUp)
		}
		toOnStem := sl.above == toUp
		if toOnStem {
			toBase = toStemY + stemSlurInset(sl.above)
		}
		// And each end outside whatever else is written in its own note's
		// column, and outside the marks on the note itself.
		fromY = e.slurEndColumn(sl.x, fromY, sl.fromStemY, sl.above, fromOnStem)
		toBase = e.slurEndColumn(x, toBase, toStemY, sl.above, toOnStem)
		if !carried {
			sl.y = e.slurEndClear(sl.x, fromY, sl.above)
		}
		sl.toRef = p.index
		toY := e.slurEndClear(x, toBase, sl.above)
		e.drawTieOrSlur(sl, x, toY, slurLift, true, true)
		open.slur = nil
	}
	if m.SlurStart && open.slur == nil {
		edge := p.hi
		if !above {
			edge = p.lo
		}
		y := e.y + stepY(float64(edge))
		up := e.stemUp(p)
		base := e.y + stepY(float64(p.hi))
		if !up {
			base = e.y + stepY(float64(p.lo))
		}
		open.slur = &openSpan{
			x: x, y: y, rel: y - e.y, above: above,
			fromHi: p.hi, fromLo: p.lo,
			fromStemUp: up, fromStemY: e.stemEnd(base, up),
			ref: p.index, part: e.partIndex, measure: measure,
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

	// A tie that is never closed is closed on the note after it.
	//
	// Nothing stops one otherwise: an open tie is drawn to the margin at
	// every line break and picked up again on the next system, so a tie
	// whose other half is missing is drawn across the rest of the piece. It
	// happens whenever a file says a note is held and does not say into
	// what — Finale leaves the mark on a note whose neighbour has changed
	// pitch — and the page then shows a curve sweeping over five bars
	// instead of the one short tie that was meant.
	//
	// So the following note ends it, whatever it is. A tie between two
	// different pitches is odd notation, but it is one curve between two
	// notes and it says what the file said; a line to the end of the system
	// says nothing and hides the music under it.
	//
	// What is open before this element, so that a tie this element itself
	// opens is not closed by it.
	var closable []int
	for step, t := range tied.ties {
		if t.voice == p.voice {
			closable = append(closable, step)
		}
	}
	sort.Ints(closable)

	for _, h := range e.tiedHeads(p, m) {
		if h.stop {
			if t, ok := tied.ties[h.step]; ok {
				t.toRef = p.index
				e.drawTieOrSlur(t, x, e.y+stepY(float64(h.step)), tieLift, false, true)
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
					voice: p.voice, step: h.step,
					ref: p.index, part: e.partIndex, measure: measure,
				}
			}
		}
	}

	// Whatever was open before this element and is open still was not
	// closed by it: this is the note it is tied to.
	for _, step := range closable {
		t, ok := tied.ties[step]
		if !ok || t.x < 0 {
			continue // closed above, or waiting to resume after a break
		}
		if t.x == x {
			continue // opened by this element, on another notehead
		}
		t.toRef = p.index
		e.drawTieOrSlur(t, x, e.y+stepY(float64(step)), tieLift, false, true)
		delete(tied.ties, step)
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
// complete says whether this is the whole curve or the stub left where it
// runs off the end of a system: a stub is drawn and recorded like any other
// piece of the curve, but the note it is running to is not known yet.
func (e *engraver) drawTieOrSlur(from *openSpan, toX, toY, lift float64, slur, complete bool) {
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
	// A tie joins two noteheads of one pitch: it is short, flat, and has
	// nothing between its ends to clear. A slur covers a phrase and is
	// placed against what that phrase does — see fitSlur.
	cy := (y1+y2)/2 + dir*tieArch*2
	if lid := span * archPerWidth; tieArch > lid {
		cy = (y1+y2)/2 + dir*lid*2
	}
	if slur {
		y1, y2, cy = e.fitSlur(from.x, toX, y1, y2, dir)
	}

	e.lay.Curves = append(e.lay.Curves, Curve{
		X1: from.x, Y1: y1,
		CX: (from.x + toX) / 2, CY: cy,
		X2: toX, Y2: y2,
		Thickness: e.opt.Defaults.StaffLineThickness * 1.6,
	})

	// Where it ended up, so the editor can let it be clicked. Only here is
	// that known: how high a slur arches depends on the music under it, and
	// which side it falls on depends on the stems of every note it covers.
	kind := "tie"
	if slur {
		kind = "slur"
	}
	e.lay.Arcs = append(e.lay.Arcs, ArcPos{
		Part: from.part, Measure: from.measure,
		From: from.ref, To: from.toRef, Step: from.step, Kind: kind,
		X1: from.x, Y1: y1, CX: (from.x + toX) / 2, CY: cy, X2: toX, Y2: y2,
	})
	if !complete {
		// A piece of a curve that is still running. It is on the page and
		// has to be clickable like the rest of it, so it waits to be told
		// where it was going.
		from.stubs = append(from.stubs, len(e.lay.Arcs)-1)
		return
	}
	for _, i := range from.stubs {
		e.lay.Arcs[i].To = from.toRef
	}
	from.stubs = nil
}

// slurEndClear moves a slur's end out past the marks written on the note it
// begins or ends on.
//
// A slur and the articulations under it are written on the same side of the
// same notes, and which goes outside is not a matter of taste: the marks
// belong to their noteheads and the slur belongs to the phrase, so the
// marks sit between the notes and the curve and the curve goes round them.
//
// The ends are where this has to be done by hand. What a slur arcs over is
// lifted clear by its arch, but clearArch leaves the ends alone on purpose
// — what is there belongs to the slur's own two notes — so a phrase whose
// first and last notes are accented had its curve drawn through both of
// them. Bar 55 of the Rossini, where the saxophones have three marcato
// marks under one slur: the middle one the curve went over and the outer
// two it went through.
func (e *engraver) slurEndClear(x, y float64, above bool) float64 {
	// Half a notehead: the mark and the curve are written at the same x,
	// which is the middle of the note they are both on.
	const near = 0.6
	from := e.staffGlyphs
	if from > len(e.lay.Glyphs) {
		from = len(e.lay.Glyphs)
	}
	for _, g := range e.lay.Glyphs[from:] {
		if !strings.HasPrefix(g.Name, "artic") {
			continue
		}
		if math.Abs(g.X-x) > near {
			continue
		}
		up, down := glyphReach(g.Name)
		if above {
			if top := g.Y - up; top < y {
				y = top
			}
		} else if bottom := g.Y + down; bottom > y {
			y = bottom
		}
	}
	return y
}

// slurEndColumn moves a slur's end out past whatever else is written in the
// same column of the staff as the note it begins or ends on.
//
// Two voices on one staff is what this is for. A slur goes away from the
// stems of the notes it covers, and with two voices under it there is no
// such side: the lower voice stems up, the upper voice stems down, and the
// slur over the upper one is drawn on the side the lower one's stems are
// on. Bar 48 of the eighth Altes solo, in the piano: both voices slurred
// between the same two chords, and the upper slur — started at its own
// notehead, with the lower voice's stems standing three spaces over it —
// came out as a lens with two stems run through it.
//
// It is the note's own column and not its neighbourhood: half a notehead
// either way, which is what is written on the note rather than beside it.
// What is beside it is slurShoulder's, and what is between the two notes is
// the arch's.
//
// onStem says the slur is on the side its own note's stem points, and then
// that stem is the tallest thing in the column and is not in the way: the
// curve is hung off the end of it on purpose. Only something standing past
// it counts.
func (e *engraver) slurEndColumn(x, y, ownStem float64, above, onStem bool) float64 {
	const near = 0.6
	ink := e.staffInkline().reach(x-near, x+near, !above)
	if math.IsInf(ink, 0) {
		return y // nothing drawn there at all
	}
	if onStem && ((above && ink >= ownStem) || (!above && ink <= ownStem)) {
		return y
	}
	// No air asked for here: drawTieOrSlur lifts every end by slurLift
	// afterwards, and that is the air.
	lift := y - ink
	if !above {
		lift = ink - y
	}
	if lift <= 0 {
		return y
	}
	if lift > slurColumnLift {
		lift = slurColumnLift
	}
	if above {
		return y - lift
	}
	return y + lift
}

// fitSlur places a slur so that it clears the music it arcs over.
//
// A curve fitted to its two ends alone is clear of a phrase that happens
// to be flat and of nothing else. A run climbing two octaves leaves its
// first note behind in a few notes, and a curve drawn from that note to
// the last one goes straight through everything between — which is where
// the line a slur draws must never be, because that line is the phrase.
//
// Three things are adjusted, in that order:
//
//	the shoulders what stands beside the slur's own two notes, in the
//	             quarter of the span nearest each end. A control point
//	             barely reaches there, so each end is lifted over its own
//	             neighbour rather than arched over it — see slurShoulder.
//	the arch     the middle is free, so it is lifted until the curve
//	             clears every place along it, not merely at its own
//	             middle where a quadratic stands furthest from the
//	             straight line between its ends.
//	the ends     and where that alone would need an arch taller than a
//	             slur should ever be, and the worst of what it is clearing
//	             is off to one side, the ends come up to meet the music
//	             instead. They stay over their own notes — the reader
//	             picks the phrase up from the note it starts on — but
//	             they rise off them, which is what an engraver does with
//	             a run that climbs away from where it began.
func (e *engraver) fitSlur(x0, x1, y1, y2, dir float64) (float64, float64, float64) {
	if x1 <= x0 {
		return y1, y2, (y1+y2)/2 + dir*e.slurArchFor(x1-x0)*2
	}
	// The shoulders first, because the arch is fitted to the ends and has
	// to be fitted to where they finally are.
	y1 = e.slurShoulder(x0+slurOwnColumn, x0+(x1-x0)*slurShoulderIn, y1, dir)
	y2 = e.slurShoulder(x1-(x1-x0)*slurShoulderIn, x1-slurOwnColumn, y2, dir)
	cy := (y1+y2)/2 + dir*e.slurArchFor(x1-x0)*2
	// Twice: lifting the ends changes what the arch has to be, and the
	// new arch is what says whether the ends are high enough yet.
	for pass := 0; pass < 2; pass++ {
		need, at := e.archOver(x0, x1, y1, y2, cy, dir)
		cy = need
		rise := ((y1+y2)/2 - (0.25*y1 + 0.5*cy + 0.25*y2)) * -dir
		over := rise - slurRiseMax
		if over <= 0 || pass == 1 || math.Abs(at-0.5) < slurOffCentre {
			break
		}
		// The ends share the lift by how much each of them moves the
		// curve where the trouble is: a quadratic near its start is
		// nearly all its first end, and near its finish nearly all its
		// last.
		if over > slurEndLift {
			over = slurEndLift
		}
		w0, w1 := (1-at)*(1-at), at*at
		most := math.Max(w0, w1)
		if most <= 0 {
			break
		}
		y1 += dir * over * w0 / most
		y2 += dir * over * w1 / most
		cy = (y1+y2)/2 + dir*e.slurArchFor(x1-x0)*2
	}
	return y1, y2, cy
}

// slurShoulder lifts one end of a slur over whatever stands just inside
// it, and says where that end now is.
//
// The arch cannot do this. A quadratic's control point moves the curve by
// twice t(1-t), which is the whole arch in the middle and nothing at all at
// the ends, so clearing something a tenth of the way along by arching asks
// for a control point five times as far out as the thing being cleared —
// and a run of beamed notes ending on a low one came out as a spike over
// the staff for the sake of the last beam before it. The end is the part of
// the curve that is there, so the end is what moves.
//
// It moves by no more than slurShoulderLift, and that is on purpose: past
// that the curve has left its own note and the reader can no longer see
// where the phrase begins. A slur that cannot clear what is beside it
// grazes it, which is the lesser of the two faults.
func (e *engraver) slurShoulder(a, b, y, dir float64) float64 {
	if b <= a {
		return y
	}
	edge := e.staffInkline().reach(a, b, dir > 0)
	if math.IsInf(edge, 0) {
		return y // nothing drawn there at all
	}
	want := edge + dir*slurClearance
	// How far the end has to move to get there, in the direction the slur
	// arches. Positive means there is something to clear.
	//
	// (want - y), not (y - want): with the subtraction the other way round
	// this came out negative whenever there was anything to clear and
	// positive when there was not, so the shoulder never lifted an end in
	// either direction. It read as working because the arch clears most of
	// what the shoulder is for, and only showed up where the arch cannot
	// reach — a note just inside the end, which is exactly the case this
	// function exists for.
	lift := (want - y) * dir
	if lift <= 0 {
		return y
	}
	if lift > slurShoulderLift {
		lift = slurShoulderLift
	}
	return y + dir*lift
}

// slurArchFor is the arch a slur of this width starts with, before the
// music under it has had its say.
func (e *engraver) slurArchFor(span float64) float64 {
	arch := slurArch + span*slurArchPerSpace
	if arch > slurArchMax {
		arch = slurArchMax
	}
	// An arc is only an arc while it is wider than it is tall. The stub a
	// tie leaves at a line break is barely wider than a notehead, and
	// rising its full height over that reads as a caret stabbing the note
	// rather than as a curve.
	if lid := span * archPerWidth; arch > lid {
		arch = lid
	}
	return arch
}

// archOver is the control point that clears everything under the curve,
// and where along it the worst of it was.
//
// Every place along the span is asked, not only the middle. A quadratic
// stands at its furthest from the line between its ends in the middle and
// has barely left that line near its ends, so the note that a curve cuts
// through is rarely the highest one: it is the high one nearest an end.
//
// The curve's control point sits at the middle in x, which makes x move
// evenly with t, so the place along the span is the parameter and the
// arithmetic below is the Bézier read backwards for the control point.
func (e *engraver) archOver(x0, x1, y1, y2, cy, dir float64) (float64, float64) {
	above := dir < 0
	// The music, without the staff: a slur is drawn among the notes and
	// has no business clearing the lines they are written on.
	sky := e.staffInkline()

	// Only what lies between the ends: the notes the slur starts and ends
	// on are its own, and a slur is drawn from them, not over them.
	//
	// A quarter in from each end, not a tenth. A curve has barely left its
	// first note at a tenth of the way along, so clearing something there
	// asks for a control point four times as far out — and a stem that
	// belongs to the slur's own first note would throw the whole curve
	// across the page.
	//
	// The quarter at each end is not ignored; it is the end's own business.
	// A control point has almost no purchase that near a corner, so what
	// lies there is cleared by lifting the end over it — see slurShoulder.
	inset := (x1 - x0) * slurShoulderIn
	a, b := x0+inset, x1-inset
	if b <= a {
		return cy, 0.5
	}

	const steps = 24
	width := (b - a) / steps
	want, at := cy, 0.5
	for i := 0; i <= steps; i++ {
		x := a + float64(i)*width
		edge := sky.reach(x-width/2, x+width/2, !above)
		if math.IsInf(edge, 0) {
			continue // nothing drawn here at all
		}
		if above {
			edge -= slurClearance
		} else {
			edge += slurClearance
		}
		t := (x - x0) / (x1 - x0)
		pull := 2 * t * (1 - t)
		if pull < 1e-6 {
			continue
		}
		// What the control point would have to be for the curve to pass
		// exactly there.
		need := (edge - (1-t)*(1-t)*y1 - t*t*y2) / pull
		if (above && need < want) || (!above && need > want) {
			want, at = need, t
		}
	}
	return want, at
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
