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
	// The air between two wedges that meet on one note, so that the page
	// shows two markings rather than one bowtie.
	hairpinMeetGap = 0.6

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
	//
	// The hook is as deep as it is so that the number fits inside the
	// bracket: a 1. written tight under the line has the line through
	// the top of its digits, and a hook shallower than the number leaves
	// the number hanging out of the bracket it belongs to.
	voltaRise = 3.0
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
		for _, sl := range open.slurs {
			if sl.x < 0 {
				return true
			}
		}
		for _, t := range open.ties {
			if t.x < 0 {
				return true
			}
		}
		for _, ex := range open.extends {
			if ex.x < 0 {
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
		// What is open closes before anything new opens, so that a note
		// which ends one hairpin and begins another is the point where the
		// two meet. Opening first made it the middle of one long wedge: a
		// crescendo asked for where a diminuendo ended took that
		// diminuendo's end away, and the diminuendo ran on through the
		// crescendo to wherever it finally found a stop.
		//
		// Which is also why the kind is read from the note a hairpin opens
		// on and nowhere else: where two of them meet, the one kind a note
		// can carry is the kind of the one that starts there.
		wasOpen := open.hairpin != nil
		// A wedge on this note and no further. Said outright by Alone, and
		// read from the two ends together for a file written before there
		// was a word for it — which is only those two ends where nothing
		// else was open, since otherwise they are a meeting.
		alone := h.Alone || (h.Start && h.Stop && !wasOpen)

		// The line a hairpin beginning here belongs on: the one the last
		// hairpin ended on, where it ended exactly here, and a line of its
		// own otherwise. See waitingMark.join.
		row := func() int {
			if open.hadHairpin && math.Abs(open.lastHairpinX-x) < 0.001 {
				return open.lastHairpinRow
			}
			e.hairpinRows++
			return e.hairpinRows
		}
		ended := func(at float64, r int) {
			open.lastHairpinX, open.lastHairpinRow, open.hadHairpin = at, r, true
		}

		// Where two wedges meet they are pulled apart a little, so that the
		// page shows two markings and not one bowtie: the first ends short
		// of the note and the second begins past it. Every edition leaves
		// that air, and two points touching read as one shape.
		meets := h.Stop && wasOpen && (alone || h.Start)
		closeAt, openAt := x, x
		if meets {
			closeAt -= hairpinMeetGap / 2
			openAt += hairpinMeetGap / 2
			if closeAt <= open.hairpin.x {
				closeAt, openAt = x, x
			}
		}

		if h.Stop && wasOpen {
			open.hairpin.toRef = p.index
			e.drawHairpin(open.hairpin, closeAt, true)
			// The note is what the next hairpin is measured against, not
			// the point this one was pulled back to: they meet at the note
			// whether or not the ink does.
			ended(x, open.hairpin.row)
			open.hairpin = nil
		}
		switch {
		case alone:
			// Across the room the note itself takes, stopping short of the
			// next one: a swell on a whole note is as wide as the bar it
			// fills, and a wedge the width of a notehead under it would
			// read as a mistake.
			r := row()
			to := x + p.width*0.9
			e.drawHairpin(&openSpan{
				x: openAt, kind: h.Kind, toRef: p.index, row: r,
				ref: p.index, part: e.partIndex, measure: measure,
				words: h.Words, dashes: h.Dashes,
			}, to, true)
			ended(to, r)
		case h.Start && open.hairpin == nil:
			open.hairpin = &openSpan{
				x: openAt, kind: h.Kind, row: row(),
				ref: p.index, part: e.partIndex, measure: measure,
				words: h.Words, dashes: h.Dashes,
			}
		}
	}

	if tr := m.Trill; tr != nil {
		// Both ends on one note while a trill is open is the old trill
		// ending here and a new one beginning here — the Arietta's long
		// trill starts again on the same D with a flat over it — and not
		// a shake on this note alone inside the trill it is part of.
		wasOpen := open.trill != nil
		if tr.Start && tr.Stop && wasOpen {
			e.drawTrill(open.trill, x-0.8, true)
			open.trill = nil
		}
		if tr.Start && open.trill == nil {
			open.trill = &openSpan{x: x - 0.5, accidental: tr.Accidental}
		}
		if tr.Stop && open.trill != nil && !(tr.Start && wasOpen) {
			e.drawTrill(open.trill, x+noteheadWidth(p.value)/2, true)
			open.trill = nil
		}
	}

	// A hammer-on, a pull-off or a slide on a fretted instrument, which
	// reaches from this note to the one after it.
	//
	// Closed before it is opened, and in that order: a run of hammer-ons
	// is every note marked, and each one both finishes the link from the
	// note before and starts the link to the note after.
	if open.tech != nil {
		e.drawTechLink(*open.tech, x, e.y+stepY(float64(p.step)))
		open.tech = nil
	}
	if t, ok := linkFrom(m); ok {
		open.tech = &techLink{kind: t, x: x, y: e.y + stepY(float64(p.step))}
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
				hidden: pd.Hidden,
			}
			open.notches = nil
		}
		if pd.Change && open.pedal != nil {
			open.notches = append(open.notches, x)
		}
		if pd.Stop && open.pedal != nil {
			open.pedal.toRef = p.index
			if !open.pedal.hidden {
				e.drawPedal(open.pedal, x, open.notches, true)
			}
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
		open.hairpin.carried = true
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
	for _, sl := range open.slurs {
		e.drawTieOrSlur(sl, right, sl.y, slurLift, true, false)
		sl.x = -1
	}
	// Up the staff, for the same reason: a chord held over a line break
	// leaves one stub per note, and which was drawn first decided the order
	// of the whole layout.
	for _, step := range sortedSteps(open.ties) {
		t := open.ties[step]
		e.drawTieOrSlur(t, right, t.y, tieLift, false, false)
		t.x = -1
	}
	// A held syllable's line runs to the margin only if the sound it belongs
	// to carries on past it, or the next note is sung on the same syllable.
	// Where the syllable finished on this system the line finished with it,
	// and there is nothing to pick up again.
	for _, verse := range sortedVerses(open.extends) {
		ex := open.extends[verse]
		if ex.holding || ex.melisma {
			e.drawExtender(ex, right)
			ex.x = -1
			continue
		}
		e.drawExtender(ex, math.Min(right, ex.limit))
		delete(open.extends, verse)
	}
}

// sortedVerses lists the verses holding a syllable, top line of words first.
// In order because this draws: see breakSpans.
func sortedVerses(ex map[int]*openSpan) []int {
	out := make([]int, 0, len(ex))
	for verse := range ex {
		out = append(out, verse)
	}
	sort.Ints(out)
	return out
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
	for _, sl := range open.slurs {
		if sl.x < 0 {
			sl.x = left
			sl.y = e.y + sl.rel
		}
	}
	for _, t := range open.ties {
		if t.x < 0 {
			t.x = left
			t.y = e.y + t.rel
		}
	}
	for _, ex := range open.extends {
		if ex.x < 0 {
			ex.x = left
			ex.y = e.y + ex.rel
			// Where the sound ends is not known again until the next note
			// of it is placed on this system, and until then nothing may
			// cut the line short.
			ex.limit = math.Inf(1)
		}
	}
}

// drawHairpin draws the wedge. A crescendo opens to the right from a point; a
// diminuendo starts open and closes to a point. A hairpin asked for as words
// is drawn by drawHairpinWords instead.
func (e *engraver) drawHairpin(h *openSpan, to float64, complete bool) {
	if to <= h.x {
		return
	}
	if h.words {
		e.drawHairpinWords(h, to, complete)
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
	// What it asks to be kept clear is the room its mouth takes, not the
	// whole of its length: a wedge has no height at its point, so there is
	// nothing there for anything to collide with.
	claim0, claim1 := h.x, to
	if h.kind == model.Diminuendo {
		claim1 -= skylineStep
	} else {
		claim0 += skylineStep
	}
	if claim1 <= claim0 {
		claim0, claim1 = h.x, to
	}
	e.wait(waitingMark{
		x0: claim0, x1: claim1, height: hairpinMouth, gap: 0.6,
		join:  h.row,
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

// hairpinWordSize is the size "cresc." and "dim." are set at: the size of a
// word in a dynamic, which is what they are.
const hairpinWordSize = dynamicTextSize

// drawHairpinWords draws a hairpin written as its word: "cresc." or "dim."
// in italics where it begins, and a broken line from the word to where it
// stops, the way an edition writes a crescendo too long for a wedge. A
// piece carried over from an earlier system has had its word already and
// carries the line alone, as an octave line does.
//
// It takes the same room under the music as a wedge would, on the same
// row, and is reported to the editor as the same kind of thing: a click on
// the word or the line picks it up and Backspace takes it away.
func (e *engraver) drawHairpinWords(h *openSpan, to float64, complete bool) {
	word := "cresc."
	kind := "crescendo"
	if h.kind == model.Diminuendo {
		word, kind = "dim.", "diminuendo"
	}
	size := hairpinWordSize
	wordWidth := 0.0
	if !h.carried {
		wordWidth = italicWidth(word, size)
	}
	t := e.opt.Defaults.StaffLineThickness * 1.2
	// Where the broken line begins and ends. After the word, where there
	// is one; to where the hairpin stops, or for exactly the dashes asked
	// for — the renderers draw a dash and then eight tenths of one of air,
	// so n dashes are n dashes and n-1 gaps long — or not at all.
	x0 := h.x
	if wordWidth > 0 {
		x0 += wordWidth + 0.6
	}
	x1 := to
	switch {
	case h.dashes < 0 || (h.dashes > 0 && h.carried):
		x1 = x0
	case h.dashes > 0:
		x1 = x0 + hairpinDash*(1.8*float64(h.dashes)-0.8)
	}
	reach := math.Max(math.Max(to, h.x+wordWidth), x1)
	e.wait(waitingMark{
		x0: h.x, x1: reach, height: size, gap: 0.6,
		join:  h.row,
		below: true, rank: rankHairpin,
		draw: func(top float64) {
			if complete {
				e.lay.Hairpins = append(e.lay.Hairpins, HairpinPos{
					Part: h.part, Measure: h.measure,
					From: h.ref, To: h.toRef,
					X: h.x, Y: top, Width: reach - h.x, Height: size,
					Kind: kind, Words: true, Dashes: h.dashes,
				})
			}
			// The word sits on a baseline a little above the bottom of its
			// band, so that its descender and the line's dashes both fit.
			if !h.carried {
				e.lay.Texts = append(e.lay.Texts, Text{
					S: word, X: h.x, Y: top + size*0.8, Size: size,
					Anchor: "start", Italic: true,
				})
			}
			// The broken line, at the height of the word's middle. Not
			// drawn where there is no room for a dash: a swell on one note
			// is the word alone.
			if x1-x0 >= hairpinDash {
				y := top + size*0.5
				e.lay.Lines = append(e.lay.Lines, Line{
					X1: x0, Y1: y, X2: x1, Y2: y, Thickness: t, Dash: hairpinDash,
				})
			}
		},
	})
}

// hairpinDash is the length of one dash of the line after "cresc.".
const hairpinDash = 0.6

// drawOctave draws the broken line with its name at the left, and the hook at
// the end that says where the shift stops.
func (e *engraver) drawOctave(o *openSpan, to float64, complete bool) {
	if to <= o.x {
		return
	}
	t := e.opt.Defaults.StaffLineThickness * 1.1
	above := o.amount > 0
	label := o.label
	// Where it begins, taken now: the drawing below runs once the staff is
	// finished, and by then a line that reaches the margin has been marked
	// as broken, with its x set to -1 for the next system to pick up. Read
	// then, every label and every carried line began at the left margin.
	from := o.x

	// The sign and its line run over a stretch of music and have to clear
	// all of it — the notes on their ledger lines above all, since an
	// octave line is written exactly where the music is already high. What
	// that stretch holds is not known until the staff is finished.
	e.wait(waitingMark{
		// Further off the music than most marks. The line is the outermost
		// thing over a passage that is already high, and half a space from
		// the words under it — "sempre più rinforzando" at bar 149 of
		// Danse Macabre — read as part of them.
		x0: from, x1: to, height: octaveBand, gap: 0.9,
		below: !above, rank: rankOctave,
		draw: func(edge float64) {
			// place gives the edge nearest the staff; the line sits at the
			// far side of the band, with the sign hanging off its left end.
			y := edge - 0.7
			if !above {
				y = edge + 1.6
			}
			x := from
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

// voltaHook and voltaGap are the measurements the brackets are drawn by,
// which a score may set for itself: see Spacings.
func (e *engraver) voltaHook() float64 {
	if v := e.opt.Spacings.VoltaHook; v > 0 {
		return v
	}
	return 1.6
}

func (e *engraver) voltaGap() float64 {
	if v := e.opt.Spacings.VoltaGap; v > 0 {
		return v
	}
	return 0.8
}

// drawVoltas draws the first-time and second-time brackets over a system.
//
// They are drawn once, above the top staff, because a repeat belongs to the
// piece and not to one instrument in it: every player goes back at the same
// bar. The measures are taken from the first part that has them, which is
// where MusicXML keeps them too.
//
// A run of brackets at a time, rather than all of a system's at one height.
// Within a run — a first ending and the second beside it — one height is
// right, and a bracket that stepped up and down over the bars it covers
// would not read as one bracket. Across a whole system it is wrong, and in
// the editor's strip, where the system is the entire piece, it is badly
// wrong: the brackets at bar 85 of the Tico-Tico were lifted fifteen staff
// spaces to clear the highest note anywhere in the piece, and floated there
// over nothing.
func (e *engraver) drawVoltas(parts []*model.Part, columns []float64, start, end int) {
	if len(parts) == 0 || end <= start {
		return
	}
	has := func(mi int) bool {
		for _, p := range parts {
			if mi < len(p.Measures) && p.Measures[mi].Ending != nil {
				return true
			}
		}
		return false
	}
	for mi := start; mi < end; mi++ {
		if !has(mi) {
			continue
		}
		run := mi
		for run+1 < end && has(run+1) {
			run++
		}
		first, last := mi, run
		// The ink hangs below the line — the hook turns down and the
		// number is written inside it — so the band is claimed as what
		// is actually drawn, and the line goes at the top of it. Claimed
		// above the line and drawn below it, a bracket reserved empty
		// air and then hung its hook into the music underneath.
		height := e.voltaHook() + 0.2
		e.wait(waitingMark{
			x0: columns[first-start], x1: e.voltaEnd(parts, columns, start, last),
			height: height, gap: e.voltaGap(), rank: rankVolta,
			draw: func(y float64) {
				e.drawVoltasAt(parts, columns, start, first, last+1, y-height)
			},
		})
		mi = run
	}
}

// voltaEnd is where the bracket over a bar stops: at the barline that ends
// it, which is where the reader's eye leaves the ending.
//
// Not at the next bar's first note, which is what the column is. Between
// the two lies the barline and the air around it, so a bracket closed on
// the column overshot the repeat it belongs to by three staff spaces and
// the next bracket began that far inside the bar after it.
func (e *engraver) voltaEnd(parts []*model.Part, columns []float64, start, mi int) float64 {
	next := columns[mi-start+1]
	for _, p := range parts {
		if mi < len(p.Measures) {
			_, right := e.barlineRoom(p.Measures[mi].RightBarline)
			return e.barlineAt(next, right)
		}
	}
	return next
}

// drawVoltasAt draws the brackets of one run once its height is known.
func (e *engraver) drawVoltasAt(parts []*model.Part, columns []float64, start, from, to int, y float64) {
	t := e.opt.Defaults.StaffLineThickness * 1.4
	depth := e.voltaHook()
	// The bar a bracket is being closed at, for the box that records it.
	closeMeasure := from

	var open *model.Ending
	openX := 0.0
	openAt := 0

	closeAt := func(x float64, hook bool) {
		if open == nil {
			return
		}
		// Where it was drawn, so that it can be pointed at: the bracket,
		// its hook and the number under the line.
		e.lay.Endings = append(e.lay.Endings, EndingPos{
			Measure: openAt, To: closeMeasure, Text: open.Label(),
			X: openX, Y: y, Width: x - openX, Height: depth,
		})
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: openX, Y1: y, X2: x, Y2: y, Thickness: t},
			Line{X1: openX, Y1: y, X2: openX, Y2: y + depth, Thickness: t})
		if hook && !open.Open {
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: x, Y1: y, X2: x, Y2: y + depth, Thickness: t})
		}
		if s := open.Label(); s != "" {
			// Inside the bracket and under the line, not on it.
			// The number is sized by the hook it sits in, so that
			// changing the one does not leave the other hanging out of
			// the bracket.
			e.lay.Texts = append(e.lay.Texts, Text{
				S: s, X: openX + 0.5, Y: y + depth*0.9,
				Size: depth, Anchor: "start",
			})
		}
		open = nil
	}

	for mi := from; mi < to; mi++ {
		var ending *model.Ending
		for _, p := range parts {
			if mi < len(p.Measures) && p.Measures[mi].Ending != nil {
				ending = p.Measures[mi].Ending
				break
			}
		}
		if ending == nil {
			continue
		}
		if ending.Start {
			// A bracket already open ends at the barline of the bar
			// before this one, with its hook: a first ending closes at
			// the repeat it sends the player back from, and the second
			// begins after it. Closed at this bar's column instead, the
			// two met three spaces inside the second ending and the
			// repeat sign sat under the first ending's bracket.
			if mi > from {
				closeMeasure = mi - 1
				closeAt(e.voltaEnd(parts, columns, start, mi-1), true)
			} else {
				closeMeasure = mi
				closeAt(columns[mi-start], false)
			}
			open, openX, openAt = ending, columns[mi-start], mi
		}
		if ending.Stop {
			closeMeasure = mi
			closeAt(e.voltaEnd(parts, columns, start, mi), true)
		}
	}
	// A bracket still open at the end of the run is cut where the run
	// ends and has no closing hook; the next system opens one again.
	closeMeasure = to - 1
	closeAt(e.voltaEnd(parts, columns, start, to-1), false)
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
	// tieGap is the air between a tie's end and the notehead it leaves or
	// arrives at.
	//
	// Both ends used to be put at the middle of their noteheads, which is
	// where a tie is measured from and not where it is drawn from: the
	// curve set out from under the notehead, so at tieLift below the middle
	// of a head half a space tall it came out of the bottom of it. Reported
	// from the page as the curve touching the note. A tie joins two notes
	// and is not part of either, so it stands off both of them.
	tieGap = 0.2
	// tieLeast is the shortest curve worth drawing, which is what two notes
	// close enough to leave no room between them get instead of nothing.
	tieLeast = 0.8

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
	// Into every phrase that is open, since each of them is deciding its
	// own side from the stems of the notes it covers.
	for _, sl := range open.slurs {
		if e.stemUp(p) {
			sl.up++
		} else {
			sl.down++
		}
	}

	// The side this note alone would put the curve on, which is what a tie
	// uses and what a slur falls back to when it covers one note.
	above := e.curveAbove(p)

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
	if len(open.slurs) == 0 {
		if gx, gstep, gstem, ok := e.graceSlurAnchor(part, p); ok {
			gy := e.y + stepY(float64(gstep))
			open.slurs = append(open.slurs, &openSpan{
				x: gx, y: gy, rel: gy - e.y, above: true, grace: true,
				fromHi: gstep, fromLo: gstep,
				// A grace note's stem always points up, whatever the note
				// it leans on does. Saying so matters because the side the
				// curve falls on is settled later, from the stems of the
				// whole phrase: where it comes out below, the end is hung
				// off the stem, and a stem left unsaid was read as pointing
				// down and ending at y zero — the top of the drawing.
				fromStemUp: true, fromStemY: gstem,
				ref: p.index, part: e.partIndex, measure: measure,
			})
		}
	}

	for n := m.SlurStop; n > 0 && len(open.slurs) > 0; n-- {
		// The innermost: the one opened most recently and still open.
		sl := open.slurs[len(open.slurs)-1]
		// A grace note slurred into the note it leans on: the small curve
		// under the two of them, from under the grace note into the lowest
		// notehead of its note, as printed copies write it. Through the
		// phrase's rules it was turned over by the note's stem and pushed
		// out past the accent above, and came out as a hook over the
		// chord's top note — when the curve belongs to the bottom one.
		if sl.grace && sl.ref == p.index && sl.measure == measure {
			sl.above = false
			sl.toRef = p.index
			e.drawTieOrSlur(sl, p.x-tieGap, e.y+stepY(float64(p.lo)), graceSlurLift, true, true)
			open.slurs = open.slurs[:len(open.slurs)-1]
			continue
		}
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
			// Unless the staff is shared, where the stems say which voice a
			// note belongs to and nothing about which way it leans. Then the
			// phrase keeps to its own voice's side, which is where it was
			// put when it opened. See curveAbove.
			if e.sharedStaff {
				sl.above = e.curveAbove(p)
			}
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
		toStemY := e.stemTip(p, toUp)
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
		if c := e.lay.Curves[len(e.lay.Curves)-1]; c.X2 == x {
			e.liftOutside(c, sl.above)
		}
		open.slurs = open.slurs[:len(open.slurs)-1]
	}
	for n := m.SlurStart; n > 0; n-- {
		edge := p.hi
		if !above {
			edge = p.lo
		}
		y := e.y + stepY(float64(edge))
		up := e.stemUp(p)
		sl := &openSpan{
			x: x, y: y, rel: y - e.y, above: above,
			fromHi: p.hi, fromLo: p.lo,
			fromStemUp: up, fromStemY: e.stemTip(p, up),
			ref: p.index, part: e.partIndex, measure: measure,
		}
		open.slurs = append(open.slurs, sl)
		if e.stemUp(p) {
			sl.up++
		} else {
			sl.down++
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

	// The two ends of a tie are beside the noteheads and not under them: it
	// leaves the first note and arrives at the second. See tieGap.
	leaves, arrives := p.x+noteheadWidth(p.value)+tieGap, p.x-tieGap
	// And beside the dots, not through them. A dotted note's dot is part
	// of the note, and a tie that set out from the notehead ran straight
	// through it: four tied dotted half notes read as four dots strung on
	// their own curves. The dots were drawn before this, so where they
	// finish is known. Reported as "the tie should start after the dot".
	if p.dotsEnd > 0 {
		leaves = math.Max(leaves, p.dotsEnd+tieGap)
	}

	heads := e.tiedHeads(p, m)
	// Which noteheads this element has to arrive on.
	has := map[int]bool{}
	for _, h := range heads {
		has[h.step] = true
	}
	for _, h := range heads {
		if h.stop {
			if t, ok := tied.ties[h.step]; ok {
				t.toRef = p.index
				if t.voice != p.voice {
					e.crossingSide(t, p)
				}
				e.closeTie(t, arrives, e.y+stepY(float64(h.step)))
				delete(tied.ties, h.step)
			}
		}
		if h.start {
			if tied.ties == nil {
				tied.ties = map[int]*openSpan{}
			}
			if _, already := tied.ties[h.step]; !already {
				tied.ties[h.step] = &openSpan{
					x: leaves, y: e.y + stepY(float64(h.step)),
					rel: stepY(float64(h.step)), above: h.above,
					voice: p.voice, step: h.step, sideFixed: h.fixed,
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
		if t.ref == p.index && t.measure == measure {
			continue // opened by this element, on another notehead
		}
		// Arriving where the note of that pitch is drawn, which is not
		// always this element's own column. A chord held into a bar where
		// its notes have parted arrives partly in the other voice — and
		// where that voice stepped a notehead to the left, a curve ended
		// at this column ran on into its notehead.
		to := arrives
		if !has[step] {
			if q := e.headBeside(p, step); q != nil {
				to = math.Min(to, q.x-tieGap)
				to = e.clearOfSigns(p, to, e.y+stepY(float64(step)))
				e.crossingSide(t, q)
			}
		}
		t.toRef = p.index
		e.closeTie(t, to, e.y+stepY(float64(step)))
		delete(tied.ties, step)
	}
}

// clearOfSigns moves the end of a tie arriving at height y back in front of
// any accidental written at the moment p is struck that stands at that
// height between where the tie set out and where it is arriving.
//
// The signs of a moment are set in files to the left of every notehead
// struck then, so a tie arriving on the voice that stepped aside comes in
// across them: the flat of the voice above was drawn on the curve.
func (e *engraver) clearOfSigns(p *placed, to, y float64) float64 {
	for _, q := range e.barItems {
		if q.start.Cmp(p.start) != 0 {
			continue
		}
		var notes []*model.Note
		switch {
		case q.note != nil:
			notes = []*model.Note{q.note}
		case q.chord != nil:
			notes = q.chord.Notes
		}
		for _, n := range notes {
			spot, ok := e.accSpots[n]
			g := smufl.AccidentalGlyph(n.Accidental)
			if !ok || g == "" {
				continue
			}
			gy := e.y + stepY(float64(e.noteStep(q.clef, n)))
			up, down := glyphReach(g)
			if y < gy-up-tieGap || y > gy+down+tieGap {
				continue
			}
			x := spot.left - spot.shift - e.opt.Spacings.AccidentalToNote - e.adv(g)
			if n.Parens {
				x -= e.adv("accidentalParensLeft") + e.adv("accidentalParensRight")
			}
			to = math.Min(to, x-tieGap)
		}
	}
	return to
}

// crossingSide puts a tie that ends in another voice on that voice's side of
// the staff: below where it ends in the lower voice, above in the upper.
//
// It set out on the side of the voice it began in, and a held note that
// goes on in the voice below arched up over the notes between — through the
// noteheads of the very voice it was arriving in. Printed copies hang it
// under. A side asked for is left as it is.
func (e *engraver) crossingSide(t *openSpan, to *placed) {
	if t.sideFixed {
		return
	}
	t.above = e.curveAbove(to)
}

// headBeside is the element on the staff, struck at the same moment as p,
// that has a notehead at step, or nil.
func (e *engraver) headBeside(p *placed, step int) *placed {
	for _, q := range e.barItems {
		if q == p || q.rest != nil || q.start.Cmp(p.start) != 0 {
			continue
		}
		var notes []*model.Note
		switch {
		case q.note != nil:
			notes = []*model.Note{q.note}
		case q.chord != nil:
			notes = q.chord.Notes
		}
		for _, n := range notes {
			if e.noteStep(q.clef, n) == step {
				return q
			}
		}
	}
	return nil
}

// graceSlurLift is how far below the two noteheads a grace note's slur
// runs, in staff spaces: clear of the heads, and no further.
const graceSlurLift = 0.6

// graceSlurArch is how far below the lower of its ends the curve's control
// point is set, which bows it by half as much.
const graceSlurArch = 0.8

// closeTie draws a tie that has found the note it runs to.
//
// Both ends are moved off their noteheads first — see tieGap — unless the two
// notes stand so close together that there is no room left between them, and
// then the curve is centred in what room there is. A short tie is a tie; no
// tie at all is a note that looks held and is not.
func (e *engraver) closeTie(t *openSpan, arrives, y float64) {
	if room := arrives - t.x; room < tieLeast {
		mid := (t.x + arrives) / 2
		t.x, arrives = mid-tieLeast/2, mid+tieLeast/2
	}
	e.drawTieOrSlur(t, arrives, y, tieLift, false, true)
}

// tieSide is which way a tie arcs: what was asked for where anything was,
// and otherwise the answer the engraver worked out. See model.Marks.TieDir.
func tieSide(want model.Stem, auto bool) bool {
	switch want {
	case model.StemUp:
		return true
	case model.StemDown:
		return false
	}
	return auto
}

// tiedHead is one notehead of an element, which way its tie arcs, and
// whether a tie begins or ends on it.
type tiedHead struct {
	step        int
	above       bool
	start, stop bool
	// fixed is a side that was asked for, on the note or the notehead.
	fixed bool
}

// tiedHeads lists the noteheads of an element that carry a tie.
//
// Each notehead of a chord is tied on its own account. Holding the top note
// of a chord while the others move is ordinary writing, so a tie on a chord
// cannot mean "all of it" and nothing else — the notes keep their own marks
// and a mark on the chord means all of them, which is the common case said
// briefly.
//
// A single note's tie arcs away from its stem, as a slur does — see
// curveAbove, which is also what decides it where two voices share the staff.
// A chord's ties arc outward from the middle: the upper notes curve up and
// the lower ones down, so the curves open away from each other instead of
// crossing the noteheads between them. On a shared staff they do not fan: the
// whole chord belongs to one of the two lines of music, and its curves go on
// that line's own side, clear of the other.
func (e *engraver) tiedHeads(p *placed, m *model.Marks) []tiedHead {
	if p.note != nil {
		return []tiedHead{{
			step: p.step, above: tieSide(m.TieDir, e.curveAbove(p)),
			start: m.TieStart, stop: m.TieStop,
			fixed: m.TieDir != model.StemAuto,
		}}
	}
	if p.chord == nil {
		return nil
	}
	middle := float64(p.lo+p.hi) / 2
	out := make([]tiedHead, 0, len(p.chord.Notes))
	for _, n := range p.chord.Notes {
		step := StaffStep(e.clef, n.Pitch)
		above := float64(step) >= middle
		if e.sharedStaff {
			above = e.curveAbove(p)
		}
		// Said of the chord, or of this notehead alone: a chord is tied
		// notehead by notehead, so one of its curves can be turned over
		// while the others stay where the fan puts them.
		above = tieSide(m.TieDir, above)
		above = tieSide(n.TieDir, above)
		out = append(out, tiedHead{
			step: step, above: above,
			// The chord's own mark covers every notehead; a notehead's own
			// mark covers just itself.
			start: m.TieStart || n.TieStart,
			stop:  m.TieStop || n.TieStop,
			fixed: m.TieDir != model.StemAuto || n.TieDir != model.StemAuto,
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
	// A grace note's curve is the short one under it, not a phrase to be
	// fitted over the music. See slurAt. It bows below the lower of its two
	// ends: into a note well under the small one, a curve bowed from the
	// midway height is a straight line drawn down between them.
	if slur && !from.grace {
		y1, y2, cy = e.fitSlur(from.x, toX, y1, y2, dir)
	}
	if from.grace {
		cy = math.Max(y1, y2) + graceSlurArch
	}
	// A tie that has music between its two notes arches over it. Between
	// two notes side by side there is nothing, and a tie keeps its flat
	// shape; but a tie into the next bar, or into the other voice, can run
	// past a note of the other voice whose stem comes down across it — and
	// a curve cutting a stem reads as neither.
	if !slur && !from.grace {
		cy = e.tieOverStems(from.x, toX, y1, y2, cy, dir)
	}

	if slur {
		// Named before it is drawn, so the outline the next slur is
		// placed against can leave this one out.
		e.noteSlur(len(e.lay.Curves))
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
	for i, g := range e.lay.Glyphs[from:] {
		if !strings.HasPrefix(g.Name, "artic") {
			continue
		}
		if math.Abs(g.X-x) > near {
			continue
		}
		// Past the stem the mark goes outside the slur, which is lifted
		// over it afterwards. See liftOutside.
		if _, ok := e.outsideSlur[from+i]; ok {
			continue
		}
		up, down := glyphHeight(g)
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

// tieOverStems deepens a tie's arch so that it passes the end of any stem
// standing between its two notes and running across it, and says where the
// control point is then.
//
// Only a stem that crosses the curve as it is: one that stops short of it is
// not in its way, and the tie keeps its shape. And only between the notes,
// not at them — a tie leaves its note beside the stem, not through it.
func (e *engraver) tieOverStems(x0, x1, y1, y2, cy, dir float64) float64 {
	const (
		inset = 0.8  // the tie's own notes, at either end
		air   = 0.35 // between the end of the stem and the curve
		least = 0.15 // nearer the ends than this the arch cannot help
	)
	if x1-x0 <= 2*inset || e.staffStrokes > len(e.lay.Lines) {
		return cy
	}
	// The curve's x is near enough even along it: its control point is
	// halfway across, so t is how far along it is.
	at := func(t, c float64) float64 {
		return (1-t)*(1-t)*y1 + 2*t*(1-t)*c + t*t*y2
	}
	for _, l := range e.lay.Lines[e.staffStrokes:] {
		if math.Abs(l.X1-l.X2) > 0.01 || l.X1 < x0+inset || l.X1 > x1-inset {
			continue
		}
		// A stem, not a barline: a tie over the bar crosses its barline
		// by being a tie over the bar.
		if l.Thickness > e.opt.Defaults.StemThickness+1e-9 || e.atBarline(l.X1) {
			continue
		}
		t := (l.X1 - x0) / (x1 - x0)
		if t < least || t > 1-least {
			continue
		}
		lo, hi := math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2)
		y := at(t, cy)
		if y < lo-air || y > hi+air {
			continue // it stops short of the curve
		}
		// The end of the stem on the side the tie bows to.
		want := hi + air
		if dir < 0 {
			want = lo - air
		}
		c := (want - (1-t)*(1-t)*y1 - t*t*y2) / (2 * t * (1 - t))
		if (dir > 0 && c > cy) || (dir < 0 && c < cy) {
			cy = c
		}
	}
	return cy
}

// atBarline says a barline of the staff being drawn stands at x.
func (e *engraver) atBarline(x float64) bool {
	for i := len(e.lay.Barlines) - 1; i >= 0; i-- {
		b := e.lay.Barlines[i]
		if b.Part != e.partIndex {
			break
		}
		if math.Abs(b.X-x) < 0.3 {
			return true
		}
	}
	return false
}

// liftOutside moves the marks that go outside a slur out over it, where the
// slur c runs past them on their side of the notes.
//
// Which marks those are is the engraver's convention as much as anything,
// and this one is the printed parts': an accent stands over the slur, which
// runs from the note under it. Set the other way, the curve's end was pushed
// out over the accent and the slur started up in the air beside it. And a
// mark past the end of a stem, where the slur is hung off that stem, goes
// outside for the same reason. A staccato dot or a tenuto stays between the
// note and the curve; see slurEndClear.
func (e *engraver) liftOutside(c Curve, above bool) {
	const (
		near = 1.0 // past the ends, a mark on the end note still counts
		air  = 0.5
	)
	if c.X2 <= c.X1 {
		return
	}
	// Where the curve is at x. Its control point is halfway across, so x
	// runs evenly with t.
	curveAt := func(x float64) float64 {
		t := (x - c.X1) / (c.X2 - c.X1)
		t = math.Max(0, math.Min(1, t))
		return (1-t)*(1-t)*c.Y1 + 2*t*(1-t)*c.CY + t*t*c.Y2
	}
	// By note, so that two marks stacked on one note move together and
	// stay stacked rather than landing on each other.
	shift := map[int]float64{}
	mine := map[int][]int{}
	from := e.staffGlyphs
	for gi, ai := range e.outsideSlur {
		if gi < from || gi >= len(e.lay.Glyphs) || ai >= len(e.lay.Articulations) {
			continue
		}
		if e.lay.Articulations[ai].Above != above {
			continue
		}
		g := e.lay.Glyphs[gi]
		mid := g.X + smufl.Advance(g.Name)/2
		if mid < c.X1-near || mid > c.X2+near {
			continue
		}
		y := curveAt(mid)
		up, down := glyphHeight(g)
		s := shift[g.Ref]
		if above {
			if bottom := g.Y + down; bottom > y-air {
				s = math.Min(s, y-air-bottom)
			}
		} else if top := g.Y - up; top < y+air {
			s = math.Max(s, y+air-top)
		}
		shift[g.Ref] = s
		mine[g.Ref] = append(mine[g.Ref], gi)
	}
	for ref, gis := range mine {
		s := shift[ref]
		if s == 0 {
			continue
		}
		for _, gi := range gis {
			e.lay.Glyphs[gi].Y += s
			e.lay.Articulations[e.outsideSlur[gi]].Y += s
			// The outlines already holding it hold it where it was. They
			// only ever grow, so where it is now is added and what comes
			// after is placed clear of both.
			if e.staffSky != nil && gi < e.staffSkyAt.glyphs {
				g := e.lay.Glyphs[gi]
				up, down := glyphHeight(g)
				w := glyphWidth(g)
				e.staffSky.add(g.X, g.X+w, g.Y-up, g.Y+down)
				e.staffInk.add(g.X, g.X+w, g.Y-up, g.Y+down)
			}
		}
	}
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
	ink := e.slurInkline().reach(x-near, x+near, !above)
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
	edge := e.slurInkline().reach(a, b, dir > 0)
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
	sky := e.slurInkline()

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

// The accidental before a "tr": a teeny glyph, as LilyPond sets it, lifted
// so that it sits level with the top of the letters.
const (
	trillAccidentalScale = 0.6
	trillAccidentalSize  = 4 * trillAccidentalScale
	trillAccidentalRaise = 0.65
)

var trillAccidentalGlyph = map[model.Accidental]string{
	model.Sharp: "accidentalSharp", model.Flat: "accidentalFlat",
	model.Natural: "accidentalNatural", model.DoubleSharp: "accidentalDoubleSharp",
	model.DoubleFlat: "accidentalDoubleFlat",
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
				// The accidental of the upper note, where there is one,
				// is written small and raised before the "tr", as the
				// Henle and LilyPond trills of Op. 111 have it.
				if g := trillAccidentalGlyph[t.accidental]; g != "" {
					e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
						Name: g, X: x, Y: y - trillAccidentalRaise,
						Size: trillAccidentalSize,
					})
					x += smufl.Advance(g)*trillAccidentalScale + 0.1
				}
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
