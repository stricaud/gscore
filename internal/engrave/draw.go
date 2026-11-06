/*
 * draw.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"sort"
	"strconv"
	"strings"

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
	// voice is which line of music on this staff it belongs to. Zero for
	// the only one, which is nearly always. It decides which way the stem
	// goes and what may be beamed to what.
	voice int

	note  *model.Note
	chord *model.Chord
	rest  *model.Rest
	// symbol is a mark standing in the bar on its own rather than
	// attached to a note: a sign somebody drew and put where they wanted
	// it. It has no duration and takes no slot of its own.
	symbol *model.SymbolElement

	step   int // staff step of a note, or the outer note of a chord
	lo, hi int // a chord's range; equal to step for a single note
	// clef is the one this element was placed against, which is the one in
	// force where it stands rather than the one the bar happens to end in.
	//
	// The columns of a bar are placed as the walk reaches them, so a clef
	// written partway through moves everything after it and nothing
	// before. The noteheads were drawn in a second pass afterwards, from
	// whatever clef that bar had finished in — so in a bar that changes
	// clef, every note before the change was drawn against the clef that
	// comes after it, while its own stem was drawn against the right one.
	// Bar 149 of Danse Macabre came out as four noteheads on one staff
	// with a stem hanging in the air a sixth above them.
	clef  model.Clef
	value model.NoteValue
	dots  int
	start model.Duration // position from the start of the measure
	width float64

	// fullBar says this rest is the whole of its bar's silence.
	//
	// A bar nobody plays is written as a semibreve rest hung under the
	// fourth line, whatever the meter is — that is the sign for "this bar
	// is silent" and not a note value, which is why it is the same mark
	// in 3/4 as in 4/4 and why no dot is written after it. Drawn from the
	// length instead, a silent bar of 3/4 came out as a dotted minim rest
	// and one of 6/8 as a dotted crotchet: the right arithmetic and a
	// sign no edition prints.
	fullBar bool

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

	// cross says this element is written on the staff above or below the
	// one it belongs to, and crossDY how far that is. See crossstaff.go.
	// Its step has already been read against that staff's clef, so
	// everything that works in steps goes on working; only the y it is
	// drawn at has to move.
	cross   model.CrossStaff
	crossDY float64
}

// placedOf describes one element: what it is, how it is written, and where it
// sits on the staff. Not where it sits along the bar — that is the one thing
// about an element the spacing decides rather than reads, and the spacing has
// to be able to ask these same questions before it can decide it.
func (e *engraver) placedOf(clef model.Clef, el model.Element, at model.Duration) *placed {
	// Written on the staff next door, where it says so and there is one.
	// The clef is swapped before anything is read off it, because which
	// line a pitch sits on is the clef's answer and this note is being
	// read on another staff: a C that is a ledger line under the treble
	// staff is the third space of the bass staff.
	cross := e.crossOf(el)
	if cross != model.OnOwnStaff {
		if c, ok := e.cross.clefFor(cross); ok {
			clef = c
		}
	}
	p := &placed{start: at, clef: clef, cross: cross,
		crossDY: e.cross.shiftFor(cross)}
	switch el := el.(type) {
	case *model.Note:
		p.note = el
		p.step = e.noteStep(clef, el)
		p.lo, p.hi = p.step, p.step
		p.value, p.dots = written(el.WrittenDuration(el.Dur))
	case *model.Chord:
		p.chord = el
		p.lo, p.hi = e.chordRange(clef, el)
		p.step = p.hi
		p.value, p.dots = written(el.WrittenDuration(el.Dur))
	case *model.SymbolElement:
		// It sits where a note would, so that it is placed against the
		// staff and not against the bottom line, and so that a bar of
		// nothing but a symbol still gives it somewhere to stand.
		p.symbol = el
		p.value = model.Quarter
		p.step, p.lo, p.hi = middleStep, middleStep, middleStep
	case *model.Rest:
		p.rest = el
		p.value, p.dots = written(el.WrittenDuration(el.Dur))
		// A rest is drawn on the middle line, and anything placed against
		// it — a fermata over it, a hairpin closing on it — is placed from
		// the same position rather than from the bottom line, which is
		// where a step of zero would have put it.
		p.step, p.lo, p.hi = middleStep, middleStep, middleStep
	default:
		p.value = model.Quarter
	}
	return p
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
	// A note in a two-note tremolo carries beams of its own, drawn between
	// its stem and its partner's. Beaming it to its neighbours as well
	// would put two sets of beams on one stem.
	if m := p.marks(); m != nil && m.Tremolo.Paired() {
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
	e.barIndex = b.index
	sp := e.opt.Spacings
	start := x

	// The clef this bar is read against. A staff is not one clef: a hand
	// drops into the bass for a phrase and comes back, and every notehead
	// after the sign is placed by it.
	if b.m.Clef != nil {
		e.clef = *b.m.Clef
	}

	// A clef, key or meter change is written at the head of the bar it takes
	// effect in, before any of its music.
	if b.prefix > 0 {
		x = e.drawSignatureChange(part, b, x)
	}

	// Where the bar's own music begins, after any key or meter change written
	// at its head: the first beat belongs over the first note, not over the
	// new key signature in front of it.
	music := x
	// The columns, for the row of beat marks over the staff. Collected here
	// because this is the only place that knows where a column ended up.
	var anchors []beatAnchor

	items := make([]*placed, 0, len(b.slots))
	for i := range b.slots {
		s := &b.slots[i]
		x += s.lead * stretch
		if e.collectBeats {
			anchors = append(anchors, beatAnchor{at: s.at, x: x})
		}

		// Everything that starts at this moment, whatever voice it is in,
		// at the same x. That is what makes two lines of music on one staff
		// read as two lines rather than as one long one.
		// A clef written in the middle of a bar: the sign goes in front of
		// the note it belongs to, and everything from there on is read
		// against it. The room for it was booked by clefLead.
		for _, v := range s.elems {
			m := marksOf(v.el)
			if m == nil || m.Clef == nil || e.opt.HideClef {
				continue
			}
			if *m.Clef == e.clef {
				// A change to the clef that is already here changes
				// nothing, and the sign for it says nothing. MuseScore
				// writes the clef again at the head of each movement of a
				// sonata, where the staff is in that clef already — so the
				// Presto of the Moonlight opened with a bass clef at the
				// head of the system and a second bass clef printed
				// against the first note, one behind the other, as though
				// something had changed between them.
				break
			}
			e.clef = *m.Clef
			name, _ := smufl.ClefGlyph(e.clef)
			w := smufl.Advance(name) * clefChangeScale
			// In front of the accidental as well as the note. A sharp
			// stands to the left of the notehead it belongs to, in the
			// same place the clef was being drawn, and the two were
			// printed on top of each other — which is what the left hand
			// of Danse Macabre at bar 149 looked like. clefLead books the
			// room for both, so there is somewhere to put it.
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: name,
				X:    x - e.accidentalLead(v.el) - w - e.opt.Spacings.ClefAfter,
				Y:    e.y + stepY(clefRefStep[e.clef]),
				Size: clefChangeSize,
			})
			break
		}

		col := make([]*placed, 0, len(s.elems))
		for _, v := range s.elems {
			p := e.placedOf(e.clef, v.el, s.at)
			p.x, p.index, p.width = x, v.index, s.width*stretch
			p.voice = v.voice
			items = append(items, p)
			col = append(col, p)
		}
		e.separateColumn(col)

		x += s.width * stretch
	}
	if len(b.slots) == 0 {
		x += sp.NoteBase * 2 * stretch
	}
	// Now that the bar has been laid out, where its beats fell can be read
	// off it. Before the whole-bar rest is centred below, so that the first
	// beat is where the bar begins rather than where the rest was moved to:
	// a semibreve rest sits in the middle of the bar and beat one does not.
	e.noteBeats(part, b, anchors, music, x)

	// A rest that is the whole bar is centred in it.
	//
	// It was placed where the first note of the bar would have gone, hard
	// against the clef, with the four beats it lasts as white space to the
	// right of it. That is not how a bar of rest is engraved, and in the
	// editor it also put the sign where nobody clicks: the middle of the
	// bar — which is where you look for it — was the empty part, and a
	// click there wrote a note instead of picking the rest up. So "I cannot
	// select a pause" was true of the only place it was natural to try.
	//
	// Only p.x moves. The bar is as wide as it was and the music still ends
	// where it ended, so nothing the skyline or the next bar depends on
	// changes.
	if len(items) == 1 && items[0].rest != nil && !items[0].rest.Hidden &&
		!b.ahead && barIsFull(part, b) {
		p := items[0]
		// The same test says which sign to draw as says where to draw it:
		// a bar whose whole content is one rest is a silent bar, and a
		// silent bar is a semibreve rest in the middle of it. See
		// placed.fullBar.
		p.fullBar = true
		_, right := e.barlineRoom(b.m.RightBarline)
		mid := (p.x + e.barlineAt(end, right)) / 2
		if w := smufl.Advance(smufl.RestGlyph(model.Whole)); mid-w/2 > p.x {
			p.x = mid - w/2
		}
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
			Top: e.y, Bottom: e.y + e.staffH(),
			Style: b.m.RightBarline.String(),
		})
		e.lay.Hits = append(e.lay.Hits, Hit{
			Part: e.partIndex, Measure: b.index,
			XMin: start, XMax: x, StaffTop: e.y, StaffHeight: e.staffH(),
			Clef:   e.clef.String(),
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
		// The arpeggio goes in before the noteheads it rolls, because the
		// room for it was booked in front of the column and the line is
		// read before them.
		if m := p.marks(); m != nil {
			e.drawArpeggio(p, m.Arpeggio)
		}
		e.drawHeads(part, p)
		e.drawGrace(part, p)
		e.drawLoneSymbol(p)
	}
	e.drawStemsAndBeams(items, part.Time)
	for _, p := range items {
		if m := p.marks(); m != nil {
			e.drawArticulations(m, p.x+noteheadWidth(p.value)/2, p.lo, p.hi, e.stemUp(p), p.index)
			e.drawDynamic(m, p, b.index)
			e.spanAt(m, p, b.index)
			e.slurAt(part, m, p, b.index)
			e.drawLyrics(m, p)
			e.drawTexts(m, p)
			// How the note is played: a bend, a mute, a harmonic. The
			// hammer-ons and the slides are not here — they need the
			// note after this one, so spanAt carries them.
			e.drawTechniques(m, p)
			e.drawFingering(p, m.Fingering)
			// After the beams and the articulations, so a breath taken over
			// a high run is set clear of them rather than through them.
			e.drawBreath(p, m.Break, x)
			e.waitChord(m.Chord, p.x, b.index, p.index)
		}
	}

	e.drawWholeNoteTremolos(items)
	e.drawTuplets(items, part.Time)

	// How far the bar can be clicked in: to the end of its music, or to the
	// barline where the bar is not full yet.
	//
	// The gap between the last note of a half-written bar and its barline
	// is where the rest of the bar goes, and it was not part of the bar at
	// all: a click in it found no bar under the pointer and did nothing.
	// The caret could only be put after the last note by clicking the note
	// itself, which means aiming at a notehead to say "after this". A bar
	// that is full has no such gap — the white space after a whole-bar rest
	// is the four beats it lasts — and there the reach stops at the music,
	// as it always did.
	reach := x
	if !barIsFull(part, b) && !b.ahead {
		_, room := e.barlineRoom(b.m.RightBarline)
		if at := e.barlineAt(end, room); at > reach {
			reach = at
		}
	}
	hit := Hit{
		Part: e.partIndex, Measure: b.index,
		XMin: start, XMax: reach, StaffTop: e.y,
		StaffHeight: e.staffH(),
		Clef:        e.clef.String(),
		KeyChange:   b.m.Key != nil,
		TimeChange:  b.m.Time != nil,
		Fifths:      part.KeyAt(b.index).Fifths,
		Full:        barIsFull(part, b),
		Ahead:       b.ahead,
	}
	for i, p := range items {
		kind := "rest"
		y := e.y + stepY(middleStep)
		y2 := y
		var steps []int
		var alters []int
		switch {
		case p.note != nil:
			kind = "note"
			y = e.y + stepY(float64(p.step))
			y2 = y
			steps = []int{p.step}
			alters = []int{p.note.Pitch.Alter}
		case p.chord != nil:
			// A chord reaches from its lowest notehead to its highest, and a
			// click anywhere between them means the chord.
			kind = "chord"
			y = e.y + stepY(float64(p.hi))
			y2 = e.y + stepY(float64(p.lo))
			for _, n := range p.chord.Notes {
				steps = append(steps, e.noteStep(p.clef, n))
				alters = append(alters, n.Pitch.Alter)
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
		chord, frets := "", ""
		if m := p.marks(); m != nil && m.Chord != nil {
			chord = m.Chord.String()
			frets = m.Chord.Frame.Spell()
		}
		// Only where something is bent. A bar of plain letters would
		// otherwise carry a row of zeroes in every answer.
		plain := true
		for _, a := range alters {
			if a != 0 {
				plain = false
			}
		}
		if plain {
			alters = nil
		}
		hit.Elements = append(hit.Elements, ElementPos{
			Ref: p.index, Index: i, X: p.x, Y: y, Y2: y2, At: at,
			Width: noteheadWidth(p.value), Kind: kind,
			Steps: steps, Alters: alters,
			Chord: chord, ChordFrets: frets,
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
			Top: e.y, Bottom: e.y + e.staffH(),
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

	// The clef first: it says what everything after it is read against, so
	// it is written before the key that follows it.
	if b.change.hasClef {
		name, _ := smufl.ClefGlyph(*b.change.clef)
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: name, X: x,
			Y:    e.y + stepY(clefRefStep[*b.change.clef]),
			Size: clefChangeSize,
		})
		x += smufl.Advance(name)*clefChangeScale + sp.ClefAfter
	}

	if b.change.hasKey {
		naturals, accidentals := keyChangeSteps(e.clefAt(part, b.index), b.change.from, *b.change.key)
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
	// The air between a beam and the figure written outside it.
	tupletFigureGap = 0.4
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
	if tp.Hidden {
		// Counted, not written: the notes keep their tuplet time and
		// nothing is drawn over them. See model.Tuplet.
		return
	}
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

	// Clear of the beam, where the group has one.
	//
	// The height above was worked out from the highest notehead and a
	// stem's length, which is where the beam is for an ordinary group and
	// nowhere near it for a wide one: a beam is drawn through the ends of
	// the outermost stems, so a run climbing two octaves has it a long way
	// from the notes at one end. The 17 over the Rhapsody's opening run
	// came out on top of its own beam, unreadable.
	//
	// Outwards only. A group whose beam sits where a stem's length would
	// put it keeps the figure exactly where it had it, so this moves
	// nothing that was not already wrong.
	if b, ok := e.beamed[first]; ok {
		half := textWidth(strconv.Itoa(tp.Actual), tupletNumberSize)/2 + tupletNumberGap
		edge := math.Min(b.at(cx-half), b.at(cx+half))
		if !up {
			edge = math.Max(b.at(cx-half), b.at(cx+half))
		}
		th := e.beamTh()
		if up {
			if clear := edge - th/2 - tupletFigureGap; clear < y {
				y = clear
			}
		} else if clear := edge + th/2 + tupletNumberSize; clear > y {
			y = clear
		}
	}

	e.lay.Texts = append(e.lay.Texts, Text{
		S: strconv.Itoa(tp.Actual), X: cx, Y: y,
		Size: tupletNumberSize, Anchor: "middle", Italic: true, Bold: true,
	})

	// Where it was drawn, so it can be pointed at. The figure's own box
	// first; a bracketed group widens it below, since then the bracket is
	// part of the mark and is drawn in clear air over the whole run.
	fw := textWidth(strconv.Itoa(tp.Actual), tupletNumberSize)
	box := TupletPos{
		Part: e.partIndex, Measure: e.barIndex,
		From: group[0].index, To: group[len(group)-1].index,
		X: cx - fw/2, Y: y - tupletNumberSize,
		Width: fw, Height: tupletNumberSize * 1.25,
		Figure: tp.Actual,
	}

	if oneBeam(group, beam) {
		e.lay.Tuplets = append(e.lay.Tuplets, box)
		return // the beam says it already
	}
	box.X, box.Width, box.Bracketed = x0, x1-x0, true
	e.lay.Tuplets = append(e.lay.Tuplets, box)

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

// beamRun is where a beam was drawn: the centreline of its outermost beam,
// from end to end, and which side of the notes it is on.
type beamRun struct {
	x0, y0, x1, y1 float64
	up             bool
}

// at is the centreline of the beam at an x along it.
func (b beamRun) at(x float64) float64 {
	if b.x1 == b.x0 {
		return b.y0
	}
	return b.y0 + (b.y1-b.y0)*(x-b.x0)/(b.x1-b.x0)
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

func (e *engraver) chordRange(c model.Clef, ch *model.Chord) (lo, hi int) {
	if len(ch.Notes) == 0 {
		return 0, 0
	}
	lo = e.noteStep(c, ch.Notes[0])
	hi = lo
	for _, n := range ch.Notes {
		s := e.noteStep(c, n)
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
	//
	// Strictly furthest, so that a tie goes down: a note sitting on the
	// middle line stems down, and so does a chord whose outer notes are the
	// same distance either side of it. That is the rule every book states —
	// "for notes on the middle line and above, the stem is down" — and it
	// was ">=" here, which stemmed the middle line up. One character, and
	// it was wrong on the commonest note on a treble staff.
	return (middleStep - p.lo) > (p.hi - middleStep)
}

// drawHeads draws an element's noteheads, at whatever size that element is
// written at: full size for the music, smaller for a cue. See cue.go.
func (e *engraver) drawHeads(part *model.Part, p *placed) {
	// The notehead, its accidental, its dots and its ledger lines all go
	// on the other staff together. Moving the whole drawing rather than
	// each of them is what keeps them from coming apart: a ledger line
	// left behind on the home staff is a line through the middle of the
	// music with no note on it.
	e.onCrossStaff(p, func() {
		e.drawingAt(p.scaleOf(), func() {
			switch {
			case p.rest != nil:
				e.drawRestAt(p)
			case p.note != nil:
				e.drawNoteHead(p)
			case p.chord != nil:
				e.drawChordHeads(p)
			}
		})
	})
}

// onCrossStaff draws something at the neighbouring staff's height.
//
// By moving the staff rather than the thing: everything here measures
// from e.y, and sixty places that each add an offset of their own is
// sixty places for one of them to be forgotten.
func (e *engraver) onCrossStaff(p *placed, fn func()) {
	if p == nil || p.crossDY == 0 {
		fn()
		return
	}
	was := e.y
	e.y += p.crossDY
	defer func() { e.y = was }()
	fn()
}

func (e *engraver) drawRestAt(p *placed) {
	if p.rest.Hidden {
		return
	}
	value, dots := p.value, p.dots
	if p.fullBar {
		// A silent bar, whatever its length. See placed.fullBar.
		value, dots = model.Whole, 0
	}
	name := smufl.RestGlyph(value)
	step := 4.0
	if value == model.Whole {
		step = 6
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: name, X: p.x, Y: e.y + stepY(step), Ref: p.index,
		Size: e.glyphSize(),
	})
	e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(step), Staff: e.y}
	// A rest has no stem for the dots to clear.
	e.drawDots(p.x+e.adv(name)+e.opt.Spacings.DotToNote, step, dots)
}

// drawAccidental writes a note's sign in front of it, in brackets where the
// note asks for them.
//
// at is the left edge of the notehead the sign belongs to: the sign is
// written to the left of it, and a bracketed one reaches further left still,
// which is why the room is booked before the music is drawn rather than
// taken from whatever happens to be beside it.
func (e *engraver) drawAccidental(n *model.Note, at, y float64) {
	g := smufl.AccidentalGlyph(n.Accidental)
	if g == "" {
		return
	}
	size := e.glyphSize()
	x := at - e.opt.Spacings.AccidentalToNote - e.adv(g)
	if !n.Parens {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: g, X: x, Y: y, Size: size})
		return
	}
	left, right := "accidentalParensLeft", "accidentalParensRight"
	lw, rw := e.adv(left), e.adv(right)
	x -= lw + rw
	e.lay.Glyphs = append(e.lay.Glyphs,
		Glyph{Name: left, X: x, Y: y, Size: size},
		Glyph{Name: g, X: x + lw, Y: y, Size: size},
		Glyph{Name: right, X: x + lw + e.adv(g), Y: y, Size: size})
}

func (e *engraver) drawNoteHead(p *placed) {
	n := p.note
	e.drawAccidental(n, p.x, e.y+stepY(float64(p.step)))
	head := smufl.NoteheadGlyphOf(p.value, n.Head)
	y := e.y + stepY(float64(p.step))
	if e.tab != nil && e.fretNumber(n, p.x+e.headW(p.value)/2, y, p.index) {
		e.lay.Refs[p.index] = RefPos{X: p.x, Y: y, Staff: e.y}
		e.drawDots(p.x+e.adv(head)+e.dotGap(e.stemUp(p)),
			float64(p.step), p.dots)
		return
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: head, X: p.x, Y: y, Ref: p.index, Size: e.glyphSize(),
		Colour: e.noteColour(n),
	})
	e.lay.Refs[p.index] = RefPos{X: p.x, Y: y, Staff: e.y}
	e.ledgerLines(p.x, e.adv(head), p.step)
	e.drawDots(p.x+e.adv(head)+e.dotGap(e.stemUp(p)), float64(p.step), p.dots)
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
		steps[i] = e.noteStep(p.clef, n)
	}
	shifted := displacedHeads(steps, up)

	// Which side a displaced notehead steps to: across the stem, and the
	// stem is on the far side of the chord from where it points.
	//
	// A chord is read from its stem end — upwards from the bottom note of a
	// stem-up chord, downwards from the top note of a stem-down one — and
	// the note that gives way is the second one of the pair as it is read.
	// It steps across the stem, so with the stem up it goes right of a stem
	// standing at the right of the column, and with the stem down it goes
	// left of a stem standing at the left. Moving it right either way put a
	// stem-down chord's lower note out on the wrong side, hanging off the
	// chord with the stem running down between them: reported as "the A
	// should be on the left, otherwise the chord looks weird".
	step := e.adv(head)
	if !up {
		step = -step
	}
	leftmost := p.x
	if step < 0 {
		for _, sh := range shifted {
			if sh {
				leftmost = p.x + step
				break
			}
		}
	}

	for i, n := range ch.Notes {
		at := steps[i]
		// Noteheads a second apart cannot sit on the same side of the stem.
		offset := 0.0
		if shifted[i] {
			offset = step
		}
		if e.tab != nil {
			y := e.y + stepY(float64(at))
			if e.fretNumber(n, p.x+e.headW(p.value)/2, y, p.index) {
				if i == 0 {
					e.lay.Refs[p.index] = RefPos{X: p.x, Y: y, Staff: e.y}
				}
				continue
			}
		}
		// Clear of the whole chord, the stepped-out head included. The
		// accidentals of a chord stand in a column of their own, so they
		// all move together or the lowest of them is printed on the
		// notehead that stepped left.
		e.drawAccidental(n, leftmost, e.y+stepY(float64(at)))
		// Every notehead carries the chord's own ref, not just the first.
		// A chord is one element: selecting it must light all of it up, and
		// clicking any of its heads must find it.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: smufl.NoteheadGlyphOf(p.value, n.Head),
			X:    p.x + offset,
			Y:    e.y + stepY(float64(at)),
			Ref:  p.index,
			Size: e.glyphSize(),
			// Each head of a chord by its own letter: a C major triad is
			// three different notes and comes out in three colours, which
			// is exactly what somebody learning to read one needs to see.
			Colour: e.noteColour(n),
		})
		if i == 0 {
			e.lay.Refs[p.index] = RefPos{X: p.x, Y: e.y + stepY(float64(at)), Staff: e.y}
		}
		// Under the notehead it carries, which for a stepped-out head is
		// not the column: a ledger line drawn at the column left that head
		// floating beside it with a line under thin air.
		e.ledgerLines(p.x+offset, e.adv(head), at)
	}
	e.drawChordDots(p, steps, shifted, head, up)
}

// drawChordDots puts a dot beside every notehead of a dotted chord.
//
// The dot is part of how long a note is, and every note of the chord is that
// long: a dot drawn beside the top notehead alone says the chord is dotted
// without saying which of its notes are, and leaves the other heads reading
// as the plain value.
//
// They line up in one column clear of the whole chord, a notehead pushed
// across the stem included, and no two of them share a space — a dot cannot
// sit on a staff line, so a notehead on a line takes the space above it
// unless the note a second higher has already taken that space, and then it
// takes the space below. Two noteheads at the same pitch share one dot.
func (e *engraver) drawChordDots(p *placed, steps []int, shifted []bool, head string, up bool) {
	if p.dots <= 0 || len(steps) == 0 {
		return
	}
	// Past the furthest-right notehead, not merely past the column: a dot
	// placed off the unshifted heads is drawn on top of a shifted one. Only
	// a stem-up chord's heads step right; a stem-down chord's step left,
	// out of the dots' way.
	right := p.x + e.adv(head)
	if up {
		for _, s := range shifted {
			if s {
				right += e.adv(head)
				break
			}
		}
	}
	x := right + e.dotGap(up)
	for _, ds := range chordDotSteps(steps) {
		e.dotsAt(x, float64(ds), p.dots)
	}
}

// chordDotSteps gives the step each of a chord's dots sits at, one per
// distinct notehead, from the bottom of the chord up.
func chordDotSteps(steps []int) []int {
	seen := map[int]bool{}
	var heads []int
	for _, s := range steps {
		if seen[s] {
			continue
		}
		seen[s] = true
		heads = append(heads, s)
	}
	sort.Ints(heads)

	// A notehead in a space has only one place to put its dot, so those are
	// claimed first and the ones on lines fit around them.
	taken := map[int]bool{}
	for _, s := range heads {
		if s%2 != 0 {
			taken[s] = true
		}
	}
	out := make([]int, 0, len(heads))
	for _, s := range heads {
		if s%2 != 0 {
			out = append(out, s)
			continue
		}
		at := s + 1
		if taken[at] && !taken[s-1] {
			at = s - 1
		}
		taken[at] = true
		out = append(out, at)
	}
	return out
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
//
// The rule itself lives on the meter, because the beaming is not the only
// thing that asks what a beat is: so do the marks drawn over the beats and
// the metronome that ticks on them. See model.TimeSignature.BeatUnit.
func beatDuration(t model.TimeSignature) model.Duration {
	return t.BeatUnit()
}

// beamGroups splits a measure into runs of notes that share beams.
//
// One voice at a time. Two lines of music on a staff are beamed separately
// however close together they run: a beam joining a note of the upper voice
// to a note of the lower says they are one line of music, which is the one
// thing the notation is there to deny.
func beamGroups(items []*placed, t model.TimeSignature) [][]*placed {
	most := 0
	for _, p := range items {
		if p.voice > most {
			most = p.voice
		}
	}
	if most > 0 {
		var out [][]*placed
		for v := 0; v <= most; v++ {
			var mine []*placed
			for _, p := range items {
				if p.voice == v {
					mine = append(mine, p)
				}
			}
			out = append(out, beamGroupsOfOne(mine, t)...)
		}
		return out
	}
	return beamGroupsOfOne(items, t)
}

func beamGroupsOfOne(items []*placed, t model.TimeSignature) [][]*placed {
	beat := beatDuration(t)
	var out [][]*placed
	var cur []*placed

	// Whether the group being built is inside a tuplet. A tuplet is read
	// as one thing — the figure over it says so — and the beat does not
	// break it: twenty-two notes in the time of a 6/4 bar are one beam in
	// every printed edition, and broken at the beat they came out as two
	// beams with a bracket slung under both to carry the 22.
	//
	// Only within one group: two triplets in a row are two tuplets, each
	// starting and stopping, so they still break between themselves.
	inTuplet := false
	flush := func() {
		if len(cur) > 1 {
			out = append(out, cur)
		}
		cur = nil
		inTuplet = false
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
		if len(cur) > 0 && beatOf(p.start) != beatOf(cur[0].start) && !inTuplet {
			// A group opened by hand is not broken by the beat.
			if m := cur[0].marks(); m == nil || m.Beam != model.BeamStart {
				flush()
			}
		}
		cur = append(cur, p)
		if m := p.marks(); m != nil && m.Tuplet != nil {
			if m.Tuplet.Start {
				inTuplet = true
			}
			if m.Tuplet.Stop {
				inTuplet = false
			}
		}
	}
	flush()
	return mergeBeamUnits(out, t)
}

/*
 * Beats joined into the unit the metre is read in.
 *
 * Breaking a beam at every beat is readable and is not what printed music
 * does. The handbooks are specific about it: simple quadruple metre is read
 * as two units of duple — beats one and two, then three and four — and
 * "these two units must always be clearly distinguished", which means eight
 * eighths in four four are beamed four and four, never eight ones and twos.
 * Simple duple is one unit of the whole bar. Simple triple keeps its three
 * beats and must not be beamed as compound duple, which is why three four is
 * left alone. Compound metre is already grouped by its pulse, because
 * beatDuration gives a dotted beat there.
 *
 * The exception, and it is the reason this is a second pass rather than a
 * wider beat: a beat that is uneven is not joined to its neighbour. A unit
 * of a dotted eighth, a sixteenth and two eighths beamed as one says the bar
 * is in something other than what it is in — and a unit of all the same
 * value says exactly what it is. So the test is that every note across the
 * two beats is written the same, which is what the examples in the books all
 * have in common.
 */

// beamUnitBeats is how many beats are read as one unit in this metre.
func beamUnitBeats(t model.TimeSignature) int {
	// Compound is grouped by pulse already: beatDuration returns the dotted
	// beat, so one of those beats is the three the handbooks join.
	if t.Beats%3 == 0 && t.Beats > 3 {
		return 1
	}
	switch t.Beats {
	case 2, 4:
		return 2
	}
	return 1
}

// mergeBeamUnits joins the beat-runs that belong to one metrical unit.
func mergeBeamUnits(groups [][]*placed, t model.TimeSignature) [][]*placed {
	per := beamUnitBeats(t)
	if per < 2 || len(groups) < 2 {
		return groups
	}
	beat := beatDuration(t)
	if beat.Float() == 0 {
		return groups
	}
	unitOf := func(p *placed) int {
		return int(math.Floor(p.start.Float()/(beat.Float()*float64(per)) + 1e-9))
	}

	out := make([][]*placed, 0, len(groups))
	for _, g := range groups {
		if len(out) == 0 {
			out = append(out, g)
			continue
		}
		last := out[len(out)-1]
		if joinableBeamUnit(last, g, unitOf) {
			out[len(out)-1] = append(last, g...)
			continue
		}
		out = append(out, g)
	}
	return out
}

// writtenLength is how long an element is as it is written: its value, and
// half as much again for each dot.
func writtenLength(p *placed) model.Duration {
	d := p.value.Duration()
	for i := 0; i < p.dots; i++ {
		d = model.Duration{Num: d.Num * 3, Den: d.Den * 2}.Reduced()
	}
	return d
}

// joinableBeamUnit says whether one beat's beam carries on into the next.
func joinableBeamUnit(a, b []*placed, unitOf func(*placed) int) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// The same unit of the metre, or there is nothing to join.
	if unitOf(a[0]) != unitOf(b[0]) {
		return false
	}
	// Touching in time: anything between them — a rest, a longer note — is
	// a break somebody can see, and beaming over it would deny it.
	if a[len(a)-1].start.Add(writtenLength(a[len(a)-1])).Cmp(b[0].start) != 0 {
		return false
	}
	// All of one value. An uneven beat joined to its neighbour reads as a
	// bar in some other metre.
	v, d := a[0].value, a[0].dots
	for _, p := range append(append([]*placed{}, a...), b...) {
		if p.value != v || p.dots != d {
			return false
		}
		// A beam somebody wrote by hand is theirs, and the metre does not
		// get to extend it.
		if m := p.marks(); m != nil && m.Beam != model.BeamAuto {
			return false
		}
		// And a tuplet is one thing already.
		if m := p.marks(); m != nil && m.Tuplet != nil {
			return false
		}
	}
	return true
}

// setStemDirections settles which way every stem in a measure points.
//
// It runs before anything is drawn, because the answer is needed by more than
// the stems: the dots after a notehead, the articulations, and the slurs and
// ties all sit on the side away from the stem. Asking each of them to work it
// out again from the note alone is how they came to disagree with the stem
// actually on the page.
func (e *engraver) setStemDirections(items []*placed, t model.TimeSignature) {
	// More than one line of music on the staff settles the question before
	// the notes get a say: the upper voice takes its stems up and the lower
	// takes them down, however high or low either happens to sit. That is
	// what tells a reader which note belongs to which line, and it is the
	// whole reason two voices can share a staff at all.
	multi := false
	for _, p := range items {
		if p.voice > 0 {
			multi = true
			break
		}
	}
	// Which voice is actually the upper one.
	//
	// Normally the first, which is what the numbering means and what the
	// rule above assumes. Not always: a part may write the held note as the
	// second voice and the moving line as the first, and then the first
	// voice is the lower one. Altes ends that way — the left hand holds an
	// E and a G in its second voice while its first plays a triplet on the
	// C below — and stems forced up by number sent the triplet's beam over
	// the top of the chord it is written under, where the original prints
	// it below the notes, as anyone would.
	//
	// Only where the inversion is plain: every note of the one voice below
	// every note of the other. Voices that cross or share a pitch are not a
	// case of the numbering being the wrong way round, and the numbering is
	// what tells the reader which line is which.
	inverted := false
	if multi {
		lo := map[int][2]int{} // voice -> lowest, highest step
		for _, p := range items {
			if p.rest != nil {
				continue
			}
			r, ok := lo[p.voice]
			if !ok {
				lo[p.voice] = [2]int{p.lo, p.hi}
				continue
			}
			if p.lo < r[0] {
				r[0] = p.lo
			}
			if p.hi > r[1] {
				r[1] = p.hi
			}
			lo[p.voice] = r
		}
		if a, ok := lo[0]; ok {
			if b, ok := lo[1]; ok && a[1] < b[0] {
				inverted = true
			}
		}
	}

	byVoice := func(p *placed) int {
		// Unless the score itself asked for a direction, which still wins.
		if m := p.marks(); m != nil && m.Stem == model.StemUp {
			return 1
		} else if m != nil && m.Stem == model.StemDown {
			return -1
		}
		up := p.voice%2 == 0
		if inverted {
			up = !up
		}
		if up {
			return 1
		}
		return -1
	}

	for _, g := range beamGroups(items, t) {
		dir := -1
		if multi {
			dir = byVoice(g[0])
		} else if beamGroupUp(g) {
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
			switch {
			case multi:
				p.stemDir = byVoice(p)
			case e.stemUp(p):
				p.stemDir = 1
			default:
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
	// A two-note tremolo is a group of its own. Its beams are drawn
	// between the two stems, which means the two stems have to arrive at
	// one line — and arriving at one line is the whole of what a beam
	// group does to a stem. So it is one, and the stems come out right for
	// the same reason a beamed pair's do.
	groups = append(groups, tremoloPairs(items)...)
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
		// On the staff it is drawn on, which for a note written across
		// is the one next door: a stem left behind on the home staff is
		// a stem joined to nothing.
		e.onCrossStaff(p, func() {
			e.drawingAt(p.scaleOf(), func() { e.drawSingleStem(p) })
		})
	}
	for _, g := range groups {
		// A group is beamed together, so it is drawn at one size: the
		// first note's. A cue is whole bars of somebody else's line, so a
		// group with one small note in it is a group of small notes.
		e.drawingAt(g[0].scaleOf(), func() { e.drawBeamGroup(g) })
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
		return math.Min(base-e.stemLen(), mid)
	}
	return math.Max(base+e.stemLen(), mid)
}

func (e *engraver) drawSingleStem(p *placed) {
	if p.value <= model.Whole {
		// No stem. A tremolo on one still has to go somewhere, and over the
		// notehead is where it goes.
		e.drawTremolo(p, 0, 0, 0)
		return
	}
	up := e.stemUp(p)
	head := e.headW(p.value)
	th := e.stemTh()

	loY := e.y + stepY(float64(p.lo))
	hiY := e.y + stepY(float64(p.hi))

	sx := p.x + head - th/2
	y1, y2 := loY, e.stemEnd(hiY, up)
	if !up {
		sx = p.x + th/2
		y1, y2 = hiY, e.stemEnd(loY, up)
	}
	// Which notehead the slashes have to clear: the far one.
	//
	// A stem grows out of the nearest notehead of the chord and runs past
	// the rest of them, so for a stem up it is the topmost head that stands
	// between the slashes and the room above. Measured from the one the
	// stem starts at, a chord a fifth wide put its slashes a space and a
	// bit above the bottom note — which is on top of the top note. The
	// cellos of Beethoven's ninth are octave chords throughout.
	clear := hiY
	if !up {
		clear = loY
	}
	// Before the stem is drawn and before the flag is hung on the end of
	// it, because both go where the end goes.
	y2 = e.roomForTremolo(p, clear, y2, up)
	e.lay.Lines = append(e.lay.Lines, Line{X1: sx, Y1: y1, X2: sx, Y2: y2, Thickness: th})

	if g := smufl.FlagGlyph(p.value, up); g != "" {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: g, X: sx, Y: y2, Size: e.glyphSize(),
		})
	}
	e.drawTremolo(p, sx, clear, y2)
}

// drawWholeNoteTremolos draws the pairs that have no stems.
//
// Two whole notes alternating: the beams go in the space between the two
// heads, level with the music and centred on it, because there is nowhere
// else for them and nothing to hang them from.
func (e *engraver) drawWholeNoteTremolos(items []*placed) {
	d := e.opt.Defaults
	for i := 0; i+1 < len(items); i++ {
		a, b := items[i], items[i+1]
		ma, mb := a.marks(), b.marks()
		if ma == nil || mb == nil || !ma.Tremolo.Paired() || !mb.Tremolo.Paired() {
			continue
		}
		if !ma.Tremolo.Start || !mb.Tremolo.Stop {
			continue
		}
		if a.value > model.Whole && b.value > model.Whole {
			continue // it has stems, and was drawn with them
		}
		levels := ma.Tremolo.Beams()
		if levels < 1 {
			continue
		}
		// Between the heads, with a little air at each end so the beams do
		// not grow out of the noteheads.
		x0 := a.x + noteheadWidth(a.value) + 0.5
		x1 := b.x - 0.5
		if x1-x0 < 1 {
			continue // nothing readable would fit
		}
		y := (e.y + stepY(float64(a.step)) + e.y + stepY(float64(b.step))) / 2
		top, bottom := math.Inf(1), math.Inf(-1)
		for level := 1; level <= levels; level++ {
			// Centred on that line: the stack grows evenly either side
			// rather than hanging off it.
			off := (float64(level) - float64(levels+1)/2) *
				(d.BeamThickness + d.BeamSpacing)
			e.beamQuad(x0, y+off, x1, y+off, d.BeamThickness, 1)
			top = math.Min(top, y+off)
			bottom = math.Max(bottom, y+off+d.BeamThickness)
		}
		e.markTremolo([]*placed{a, b}, ma.Tremolo, x0, top, x1-x0, bottom-top)
		i++
	}
}

// How the slashes of a tremolo are drawn.
const (
	// How far from the notehead the slashes start, along the stem. Drawn
	// against the head they read as part of it.
	tremoloFromHead = 1.1
	// And how far they are kept from the end of the stem, so they do not
	// run into a flag or a beam.
	tremoloFromEnd = 0.9

	// A two-note tremolo's beams run between the two stems rather than
	// across them, and are kept off both: air at each end, and more air
	// between the innermost beam and the noteheads.
	//
	// Drawn stem to stem and stacked down to the heads, the pair reads as
	// one beamed group whose beams happen to be doubled — which is what
	// bar 23 of the Presto looked like, where what a pianist has to see at
	// a glance is that those two half notes are an alternation and not a
	// pair of beamed notes. Standing clear of both stems is what says the
	// beams belong to neither note but to the movement between them.
	tremoloPairGap = 0.45
	// Never more than this share of the gap between the stems, so a close
	// pair keeps a beam long enough to read as one.
	tremoloPairShare = 0.2
)

// roomForTremolo lengthens a stem until the slashes on it have somewhere to
// go.
//
// They sit along the stem, clear of the notehead at one end and of the flag
// or beam at the other, and a stem of the ordinary length leaves about a
// space and a half between those two. Three strokes are nearly two spaces
// tall. So they were pushed up against the notehead and read as part of it
// rather than as a mark on the stem — which is what the cellos of
// Beethoven's ninth look like for most of the first movement, since every
// one of their notes carries a measured tremolo.
//
// Lengthening the stem is what every engraver does about it: Gould gives
// the rule in Behind Bars and LilyPond does the same in stem-tremolo.cc.
// Only ever lengthens, so a stem that already reaches the middle line of
// the staff keeps whatever it had.
func (e *engraver) roomForTremolo(p *placed, head, end float64, up bool) float64 {
	m := p.marks()
	if m == nil || m.Tremolo == nil || m.Tremolo.Paired() {
		return end
	}
	want := tremoloFromHead + tremoloRun(m.Tremolo) + tremoloFromEnd
	if up {
		if to := head - want; to < end {
			return to
		}
		return end
	}
	if to := head + want; to > end {
		return to
	}
	return end
}

// tremoloRun is how much of the stem a run of slashes takes up, in staff
// spaces: half a space each, and a little for the thickness of the outer
// two.
func tremoloRun(t *model.Tremolo) float64 {
	n := t.Beams()
	if n < 1 {
		n = 1
	}
	return 0.5*float64(n) + 0.35
}

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
	if m == nil || m.Tremolo == nil || m.Tremolo.Paired() {
		return // a two-note tremolo is drawn between the stems, not through one
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
	// One direction serves the whole group, and it was decided in the one
	// place that decides it — setStemDirections — so that what is drawn
	// here and what the slurs and dots were placed against cannot come
	// apart. This used to ask beamGroupUp again instead of reading the
	// answer. That gave the same result for as long as there was only ever
	// one rule; once a staff could carry two voices there were two, and the
	// beam went one way while the stems went the other. The stems came out
	// drawn from a notehead to a beam that was not there, which on the page
	// looks like a chord with no stem at all.
	up := e.stemUp(g[0])

	type end struct {
		x, y float64
		p    *placed
	}
	// How many beams the group carries, which the stems have to be long
	// enough for.
	//
	// Beams stack inwards, towards the noteheads, so a third and a fourth
	// crowd the very notes they belong to: a run of 32nds came out with its
	// lowest beam sitting on the noteheads. The stems grow by the height of
	// the stack past two, which is about the staff space the handbooks say
	// to add.
	deepest := 0
	for _, p := range g {
		if f := p.flags(); f > deepest {
			deepest = f
		}
	}
	extra := 0.0
	if deepest > 2 {
		extra = float64(deepest-2) * (e.beamTh() + e.beamGap())
	}

	// A group written across two staves: the beam goes between them and
	// every stem reaches it, each from its own side.
	//
	// Which way a stem points is then not a decision at all — it is
	// wherever the beam is from the notehead — so the one direction that
	// serves an ordinary group does not serve this one.
	betweenY, crossed := e.crossBeamY(g)

	ends := make([]end, 0, len(g))
	for _, p := range g {
		head := e.headW(p.value)
		if crossed {
			// Down from a note above the beam and up from one below it.
			stemUp := e.y+p.crossDY+stepY(float64(p.lo)) > betweenY
			sx := p.x + e.stemTh()/2
			if stemUp {
				sx = p.x + head - e.stemTh()/2
			}
			ends = append(ends, end{x: sx, y: betweenY, p: p})
			continue
		}
		sx := p.x + head - e.stemTh()/2
		base := e.y + stepY(float64(p.hi))
		if !up {
			sx = p.x + e.stemTh()/2
			base = e.y + stepY(float64(p.lo))
		}
		// Past the middle line the stem already reaches, rather than
		// instead of it: stemEnd lengthens a stem to touch that line and
		// this lengthens it again for the beams beyond the second.
		y := e.stemEnd(base, up)
		if up {
			y -= extra
		} else {
			y += extra
		}
		ends = append(ends, end{x: sx, y: y, p: p})
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
		// And then held to what the interval between the two ends allows.
		//
		// The slope cap alone bounds how steep a beam is and not how far it
		// falls, which on a wide group is not a bound at all: a run across
		// a bar of the Altes came out slanting 3.8 staff spaces, where the
		// books say half a space to two and very seldom more. See
		// beamSlantFor, which is the table they give.
		if most := e.beamSlantFor(g, up); math.Abs(y1-y0) > most {
			if y1 > y0 {
				y1 = y0 + most
			} else {
				y1 = y0 - most
			}
		}
	}
	beamY := func(x float64) float64 {
		if x1 == x0 {
			return y0
		}
		return y0 + (y1-y0)*(x-x0)/(x1-x0)
	}

	// Every stem has to reach the beam, so the line is pushed out to clear the
	// furthest notehead rather than cutting through it.
	//
	// Not for a beam between two staves: it is already clear of both of
	// them by being in the gap, and "out" has no meaning when there are
	// noteheads on each side of it. Pushed, it would run into one staff
	// to get away from the other.
	shift := 0.0
	for _, en := range ends {
		if crossed {
			break
		}
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
	// A two-note tremolo stacks its beams inwards from the line, towards
	// the noteheads, so the line is pushed further out to leave room for
	// them. Without this a pair of half notes a third apart has three
	// beams drawn through its own noteheads.
	if tr := pairTremolo(g); tr != nil {
		stack := float64(tr.Beams()) * (e.beamTh() + e.beamGap())
		if room := stack - e.beamTh() + tremoloPairGap; room > 0 {
			shift += room
		}
	}
	if up {
		y0 -= shift
		y1 -= shift
	} else {
		y0 += shift
		y1 += shift
	}

	// Where the beam ended up, for the figure a tuplet writes over it.
	// Only here is it known: the line is drawn through the outermost stem
	// ends and then pushed out to clear the furthest notehead, which on a
	// wide run is a long way from where a stem's length would put it.
	if e.beamed == nil {
		e.beamed = map[*placed]beamRun{}
	}
	e.beamed[g[0]] = beamRun{x0: x0, y0: y0, x1: x1, y1: y1, up: up}

	// Stems, now that the beam line is fixed.
	//
	// A stem runs the whole height of its chord: it starts at the notehead
	// furthest from the beam and finishes on it, passing the rest on the
	// way. The end nearest the beam is what decides how long the stem is —
	// that is the one ends[] measured from — but it is the far one the line
	// has to start at. Taking the near one at both ends drew a stem that
	// began at the top of the chord and went up, leaving every note beneath
	// it hanging in the air with nothing joining it to anything: a chord
	// that looks unstemmed, which is what the opening of the Altes does with
	// its C-E flat-C triplets.
	for _, en := range ends {
		// On its own staff, which for a note written across is the one
		// next door — and the stem runs from the far side of the chord,
		// which is the side away from the beam and so depends on which
		// side of the beam the note is.
		staffY := e.y + en.p.crossDY
		base := staffY + stepY(float64(en.p.lo))
		if (crossed && staffY+stepY(float64(en.p.lo)) < betweenY) || (!crossed && !up) {
			base = staffY + stepY(float64(en.p.hi))
		}
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: en.x, Y1: base, X2: en.x, Y2: beamY(en.x), Thickness: e.stemTh(),
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
		offset := float64(level-1) * (e.beamTh() + e.beamGap()) * -dir
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
				//
				// As long as a notehead is wide, which is what the handbooks
				// give: it was a flat one space, which is a little short of a
				// notehead and read as a clipped beam rather than a
				// deliberate fractional one.
				stub := e.headW(g[i].value)
				if i > 0 {
					bx0 = ends[i].x - stub
				} else {
					bx1 = ends[i].x + stub
				}
			}
			e.beamQuad(bx0, beamY(bx0)+offset, bx1, beamY(bx1)+offset, e.beamTh(), dir)
			i = j + 1
		}
	}

	// And the tremolo's own beams, continuing the stack inwards: they are
	// the beams the written-out notes would have carried, drawn once
	// between the two the player alternates between.
	if tr := pairTremolo(g); tr != nil {
		x0, x1 := ends[0].x, ends[len(ends)-1].x
		if inset := math.Min(tremoloPairGap, (x1-x0)*tremoloPairShare); inset > 0 {
			x0, x1 = x0+inset, x1-inset
		}
		top, bottom := math.Inf(1), math.Inf(-1)
		for level := maxFlags + 1; level <= maxFlags+tr.Beams(); level++ {
			offset := float64(level-1) * (e.beamTh() + e.beamGap()) * -dir
			y0, y1 := beamY(x0)+offset, beamY(x1)+offset
			e.beamQuad(x0, y0, x1, y1, e.beamTh(), dir)
			top = math.Min(top, math.Min(y0, y1)+math.Min(0, dir*e.beamTh()))
			bottom = math.Max(bottom, math.Max(y0, y1)+math.Max(0, dir*e.beamTh()))
		}
		e.markTremolo(g, tr, x0, top, x1-x0, bottom-top)
	}
}

/*
 * How far a beam may fall across its group.
 *
 * Plate engravers set this from the interval between the notes at the two
 * ends, not from the geometry: a beam that followed the notes exactly would
 * stand on end over a wide leap and lie flat over a narrow one spread across
 * a bar. The table below is the one printed in the notation handbooks —
 * roughly half a space for a second, one for a third, up to two for an
 * octave or more — and it is a ceiling rather than a target, because the
 * same books say the slant decreases as the notes are given more room.
 *
 * So this only ever reduces a slant. A group whose stems already sit nearly
 * level is left alone; one that was about to fall four staff spaces is held
 * to what its interval allows.
 */

// beamSlantFor is the most a beam may fall across this group, in staff
// spaces.
func (e *engraver) beamSlantFor(g []*placed, up bool) float64 {
	if len(g) < 2 {
		return 0
	}
	// The note at each end that the beam is drawn from: the top one of a
	// chord for a beam above, the bottom one for a beam below. A chord's
	// other notes do not decide where the beam goes.
	side := func(p *placed) int {
		if up {
			return p.hi
		}
		return p.lo
	}
	first, last := side(g[0]), side(g[len(g)-1])

	// Notes out on ledger lines slant half a space, whatever the interval:
	// the stems there are long and a steep beam over them reads as a wedge.
	for _, p := range g {
		if p.lo < 0 || p.hi > 8 {
			return 0.5
		}
	}

	// Diatonic steps between the two ends, which is the interval: one step
	// is a second, two a third, seven an octave.
	d := last - first
	if d < 0 {
		d = -d
	}
	switch {
	case d == 0:
		// The same pitch at both ends is a horizontal beam, which every
		// book states outright.
		return 0
	case d == 1: // a second
		return 0.5
	case d == 2: // a third
		return 1
	case d <= 6: // a fourth through a seventh
		return 1.5
	default: // an octave or more
		return 2
	}
}

// markTremolo records where a two-note tremolo's beams ended up, so that the
// mark can be clicked and shown as picked up. It is the only thing on the
// page that stands for it.
func (e *engraver) markTremolo(g []*placed, tr *model.Tremolo, x, y, w, h float64) {
	e.lay.Tremolos = append(e.lay.Tremolos, TremoloPos{
		Part: e.partIndex, Measure: e.barIndex,
		From: g[0].index, To: g[len(g)-1].index,
		X: x, Y: y, Width: w, Height: h, Beams: tr.Beams(),
	})
}

// tremoloPairs finds the two-note tremolos in a bar: a note marked as the
// start with its partner written next to it.
//
// Anything else — a start with nothing after it, a stop beginning a bar — is
// half a mark. It is left alone rather than guessed at, and then nothing is
// drawn between stems that were never a pair.
func tremoloPairs(items []*placed) [][]*placed {
	var out [][]*placed
	for i := 0; i+1 < len(items); i++ {
		a, b := items[i], items[i+1]
		ma, mb := a.marks(), b.marks()
		if ma == nil || mb == nil {
			continue
		}
		if !ma.Tremolo.Paired() || !mb.Tremolo.Paired() ||
			!ma.Tremolo.Start || !mb.Tremolo.Stop {
			continue
		}
		if a.value <= model.Whole || b.value <= model.Whole {
			// A whole note has no stem for a beam to arrive at. That pair
			// is drawn between the noteheads instead, and making a group
			// of it here would give both of them stems they do not have.
			continue
		}
		out = append(out, []*placed{a, b})
		i++
	}
	return out
}

// pairTremolo is the mark a group carries where it is a two-note tremolo,
// and nil where it is an ordinary beam group.
func pairTremolo(g []*placed) *model.Tremolo {
	if len(g) != 2 {
		return nil
	}
	m := g[0].marks()
	if m == nil || !m.Tremolo.Paired() || !m.Tremolo.Start {
		return nil
	}
	return m.Tremolo
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

// ledgerRun is one ledger line as it was drawn: where it ended up in the
// layout, and the notehead it carries, which it can never be shorter than.
type ledgerRun struct {
	line         int
	y            float64
	headL, headR float64
}

// ledgerGap is the air left between the ledger lines of neighbouring notes.
//
// A ledger line runs four tenths of a space past the notehead at each end,
// and two notes may be set as close as two spaces apart — which leaves two
// hundredths of a space between one line and the next. That is nothing at
// reading size, so a run of notes above the staff came out with an unbroken
// rule through the whole of it and read as a staff line that is not there.
//
// Three tenths is about twice the thickness of the line, which is what makes
// it read as a gap rather than as a join.
const ledgerGap = 0.3

func (e *engraver) ledgerLines(x, headWidth float64, step int) {
	d := e.opt.Defaults
	// A cue's ledger lines are as much smaller as its noteheads are: a
	// full-size rule under a small head reads as a staff line.
	ext := d.LegerLineExtension * e.sc()
	draw := func(s int) {
		y := e.y + stepY(float64(s))
		e.ledgers = append(e.ledgers, ledgerRun{
			line: len(e.lay.Lines), y: y, headL: x, headR: x + headWidth,
		})
		e.lay.Lines = append(e.lay.Lines, Line{
			X1: x - ext, Y1: y, X2: x + headWidth + ext, Y2: y,
			Thickness: d.LegerLineThickness * e.sc(),
		})
	}
	for s := 10; s <= step; s += 2 {
		draw(s)
	}
	for s := -2; s >= step; s -= 2 {
		draw(s)
	}
}

// separateLedgers pulls the ledger lines of neighbouring notes apart.
//
// Each is drawn on its own, from a little before its notehead to a little
// after, and at close spacing the little after of one note reaches the
// little before of the next. The lines then join, and a run climbing above
// the staff is read as a grid rather than as a note on four ledger lines
// followed by a note on five — which is the whole of what those lines are
// for.
//
// Widening the spacing would fix it and cost the music: the closest two
// notes may be set is what makes a run of thirty-seconds fit a bar at all.
// So the lines give way instead, which is what an engraver does by hand, and
// neither the notes nor the bar move.
//
// A line is never pulled back inside the notehead it carries. Where two
// noteheads are themselves nearly touching there is no gap to be had, and
// ending both lines at their noteheads is the best there is.
func (e *engraver) separateLedgers() {
	byRow := map[int][]int{}
	for i, l := range e.ledgers {
		// Rounded, because two lines meant for the same row are computed
		// from the same arithmetic and land on the same value — but only
		// to within what floating point gives back.
		byRow[int(math.Round(l.y*1000))] = append(byRow[int(math.Round(l.y*1000))], i)
	}
	for _, row := range byRow {
		sort.Slice(row, func(i, j int) bool {
			return e.ledgers[row[i]].headL < e.ledgers[row[j]].headL
		})
		for k := 1; k < len(row); k++ {
			a, b := e.ledgers[row[k-1]], e.ledgers[row[k]]
			// The same column: a chord's noteheads, or two voices meeting.
			// They share the row rather than crowding it, and pulling them
			// apart would be pulling one note's own line off it.
			if b.headL < a.headR {
				continue
			}
			la, lb := &e.lay.Lines[a.line], &e.lay.Lines[b.line]
			if lb.X1-la.X2 >= ledgerGap {
				continue
			}
			mid := (a.headR + b.headL) / 2
			if end := mid - ledgerGap/2; end < la.X2 {
				la.X2 = math.Max(end, a.headR)
			}
			if start := mid + ledgerGap/2; start > lb.X1 {
				lb.X1 = math.Min(start, b.headL)
			}
		}
	}
}

func (e *engraver) drawDots(x, step float64, dots int) {
	// A dot never sits on a line, so one that would moves up into the space.
	ds := step
	if int(ds)%2 == 0 {
		ds++
	}
	e.dotsAt(x, ds, dots)
}

// dotsAt draws a note's dots at the step given, without asking whether that
// step is a line: a chord works out where each of its dots goes before any
// of them is drawn, because they have to keep out of each other's way.
func (e *engraver) dotsAt(x, ds float64, dots int) {
	if dots <= 0 {
		return
	}
	for i := 0; i < dots; i++ {
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: "augmentationDot", X: x, Y: e.y + stepY(ds),
			Size: e.glyphSize(),
		})
		// Past the dot just drawn, then the gap. Advancing by the gap alone
		// left each dot after the first sitting on top of the one before it,
		// by the width of a dot less the gap — which a double-dotted note
		// showed as a single fat blot.
		x += e.drawnDotStep()
	}
}

// dotStep is the distance from one dot to the next: the dot itself, then the
// air after it.
//
// The bare one is what the room for a bar is booked from, before anything is
// drawn and before the size it will be drawn at is known; drawnDotStep is
// what the drawing walks by, which is smaller in a cue.
func dotStep(sp Spacings) float64 {
	return smufl.Advance("augmentationDot") + sp.BetweenDots
}

func (e *engraver) drawnDotStep() float64 {
	return e.adv("augmentationDot") + e.opt.Spacings.BetweenDots
}

// dotGap is how far after a notehead its dots begin.
//
// Measured from whatever is furthest right, which for a note whose stem goes
// up is the stem rather than the notehead: the stem rises from the notehead's
// right edge, so a dot placed off the notehead alone is crowded against it.
func (e *engraver) dotGap(stemUp bool) float64 {
	gap := e.opt.Spacings.DotToNote
	if stemUp {
		gap += e.stemTh() + 0.12
	}
	return gap
}

// drawDynamic writes a volume marking under the staff.
//
// Dynamics belong below the staff for most instruments and are aligned on
// their own line rather than hung off each note, so that a row of them reads
// as one. The glyph is centred on the note it belongs to.
func (e *engraver) drawDynamic(m *model.Marks, p *placed, measure int) {
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
		x0: x, x1: x + w, height: 1.4, gap: 1.2, below: true, rank: rankDynamic,
		draw: func(y float64) {
			// place gives the top of the band; a dynamic is drawn from its
			// baseline, which sits near the bottom of its own letters.
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: x, Y: y + 1.0})
			// Where it ended up, so the editor can let it be clicked. Only
			// now is that known: until the staff is finished there is no
			// saying how far down this stretch of music pushed it.
			e.lay.Dynamics = append(e.lay.Dynamics, DynamicPos{
				Part: e.partIndex, Measure: measure, Ref: p.index,
				X: x, Y: y, Width: w, Height: 1.4,
				Name: m.Dynamic.String(),
			})
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
		// How this kind of writing is set, and where it goes. Everything
		// below reads the style rather than a constant, so that words of
		// one kind come out alike wherever they are on the page: see
		// model.TextStyle.
		look := t.Look()
		textSize := look.Size
		// Far enough apart that the lines of one block do not touch at
		// whatever size the style is set in.
		textLine := textSize * 1.16
		// A direction can be several lines — a dedication, a note to the
		// player — and the newlines in it are real. Drawn as one string
		// they vanish, because nothing in SVG or PDF breaks a line for
		// you: "Allein Gott in der Höhe\nund Dank" came out as
		// "Allein Gott in der Höheund Dank", run together, in one line
		// across the page.
		lines := strings.Split(t.Text, "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " \t\r")
		}

		x := p.x + t.OffsetX
		widest := 0.0
		for _, line := range lines {
			if w := textWidth(line, textSize); w > widest {
				widest = w
			}
		}

		if look.Place == model.PlacementBelow {
			// Below the staff it joins the queue, for the same reason the
			// words above it do: what is under a stretch of staff is not
			// known until the staff is finished. It used to be written at
			// a fixed distance under the bottom line — three and a bit
			// spaces — which is clear of nothing. The Rhapsody's opening
			// is a clarinet low in its register, so "Solo" and "gliss"
			// came out written through the ledger lines and the noteheads
			// they belong to.
			e.wait(waitingMark{
				x0: x, x1: x + widest,
				height: textSize + float64(len(lines)-1)*textLine,
				gap:    0.6, below: true, rank: rankText,
				draw: func(y float64) {
					// place gives the top of the band, and words are drawn
					// from their baselines, so the first line sits a
					// line's height into it.
					for i, line := range lines {
						if line == "" {
							continue
						}
						e.lay.Texts = append(e.lay.Texts, Text{
							S: line, X: x,
							Y:    y + textSize + t.OffsetY + float64(i)*textLine,
							Size: textSize, Anchor: "start",
							Italic: look.Italic, Bold: look.Bold,
						})
					}
				},
			})
			continue
		}
		// Words for everybody go over the top staff of the system rather
		// than over the staff they are attached to. A tempo marking is
		// read by the whole orchestra and is written once, at the top,
		// whichever staff somebody happened to type it on — and a "rit."
		// printed over the third horn is a "rit." most of the players
		// will not see.
		if look.System && e.systemTop < e.y {
			// Kept back rather than drawn here. It goes above the top
			// staff, which is not the staff being drawn — and anything
			// put on the page during a staff's pass counts as that
			// staff's own ink. A tempo marking drawn from the second
			// violin's pass made the second violins look thirty spaces
			// tall, and everything the staff had queued was then placed
			// clear of that: the "pizz." written two bars later came out
			// above the flutes.
			e.systemTexts = append(e.systemTexts, pendingSystemText{
				t: t, lines: lines, look: look, x: x, line: textLine,
			})
			continue
		}

		// Above the staff it joins the queue, so that it clears the music,
		// the chord symbols over it and anything else already there rather
		// than being written at a distance decided in advance. "rit." over
		// a bar that also carries a chord symbol used to be printed through
		// it whenever the fixed distances happened to coincide.
		// The whole block claims its room at once, so that the lines stay
		// together and whatever is placed next clears all of them.
		e.wait(waitingMark{
			x0: x, x1: x + widest,
			height: textSize + float64(len(lines)-1)*textLine,
			gap:    0.6, rank: rankText,
			draw: func(y float64) {
				// Upwards from the bottom line: the block hangs from where
				// the queue put it, which is just above the music.
				top := y - float64(len(lines)-1)*textLine
				for i, line := range lines {
					if line == "" {
						continue
					}
					e.lay.Texts = append(e.lay.Texts, Text{
						S: line, X: x, Y: top + t.OffsetY + float64(i)*textLine,
						Size: textSize, Anchor: "start",
						Italic: look.Italic, Bold: look.Bold,
					})
				}
			},
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
		y := e.y + e.staffH() + lyricDrop + float64(verse-1)*lyricLine
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

func (e *engraver) drawArticulations(m *model.Marks, x float64, lo, hi int, stemUp bool, ref int) {
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
	// From the edge of the chord the mark is on, not from the chord's top.
	// A chord is a range of noteheads, and a mark written under one has to
	// clear its lowest note, not its highest: placed from the top, a marcato
	// under a chord sitting below the staff came out level with the chord's
	// own noteheads, printed through them. The Entertainer does it at bar
	// 29, where the left hand's chords are on ledger lines under the staff.
	step := hi
	if !above {
		step = lo
	}
	noteY := e.y + stepY(float64(step))
	gap := 1.3
	var y float64
	if above {
		// y grows downward, so further out is smaller.
		y = math.Min(noteY-gap, e.y-1.2)
	} else {
		y = math.Max(noteY+gap, e.y+e.staffH()+1.2)
	}
	// And clear of the other line of music. Two voices on one staff sound
	// at the same moment and are drawn at the same x, so a mark under the
	// upper voice has the lower voice's noteheads beneath it — and was
	// drawn straight through them. The noteheads are already on the page by
	// now, so the question can be asked of the drawing rather than guessed
	// at from the model.
	y = e.clearOfNoteheads(x, y, above)

	emit := func(name string) {
		if name == "" {
			return
		}
		// A name the font does not know is one of the user's own symbols; the
		// renderer resolves it from the symbol registry.
		origin, _, _ := centred(name, x)
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{Name: name, X: origin, Y: y})
		if above {
			y -= gap
		} else {
			y += gap
		}
	}
	// The bow and pick strokes go over the staff whichever way the stems
	// point, so they are stacked from there rather than with the rest.
	//
	// Always above: a down-bow written under a stem-up note would be read
	// as belonging to a lower line of music, and the two strokes are the
	// same symbols a guitarist reads as down-stroke and up-stroke, which
	// the handbooks put above the staff without exception. Everything else
	// — the staccato dot, the accent, the tenuto — belongs to the notehead
	// and sits on the side away from the stem.
	overY := math.Min(e.y-1.2, e.y+stepY(float64(hi))-gap)
	overY = e.clearOfNoteheads(x, overY, true)
	for _, a := range m.Articulations {
		if a.Ornament() {
			e.ornament(a, x)
			continue
		}
		if a == model.UpBow || a == model.DownBow {
			if name := smufl.ArticulationGlyph(a, true); name != "" {
				origin, _, _ := centred(name, x)
				e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
					Name: name, X: origin, Y: overY,
				})
				overY -= gap
			}
			continue
		}
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

// drawLoneSymbol draws a symbol that stands in the bar on its own.
//
// One that is attached to a note is drawn with that note's marks, and this
// is the other case: a sign put into the music where somebody wanted it
// rather than onto a particular note. It was in the model, saved to the
// file and carried through every copy, and never drawn — so inserting one
// anywhere but onto a note put a mark into the score that could not be
// seen and therefore could not be moved or taken away either.
//
// Over the staff by default, because that is where a mark goes unless it
// says otherwise, and carrying its own ref so the editor can point at it.
func (e *engraver) drawLoneSymbol(p *placed) {
	if p.symbol == nil {
		return
	}
	sm := p.symbol.Symbol
	y := e.y - symbolRise
	if sm.Placement == model.PlacementBelow {
		y = e.y + e.staffH() + symbolRise
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: sm.Name,
		X:    p.x + sm.OffsetX,
		Y:    y + sm.OffsetY,
		Ref:  p.index,
	})
}

// symbolRise is how far above the staff a symbol standing on its own is
// put before anybody moves it: clear of the lines, and near enough that it
// reads as belonging to the bar it is in.
const symbolRise = 2.0

// clearOfNoteheads moves a mark out past any notehead already drawn at this
// point of this staff, in whichever direction it is travelling.
func (e *engraver) clearOfNoteheads(x, y float64, above bool) float64 {
	const (
		reach = 1.1 // how near along the bar counts as "under this mark"
		air   = 1.3 // what to leave between the notehead and the mark
	)
	top, bot := e.y-8, e.y+e.staffH()+8
	// From where this staff began, not from the start of the drawing.
	//
	// The question is about the noteheads on this staff, and everything
	// drawn before it began is on another staff or another system. Asked
	// of the whole page, it is a walk down every glyph placed so far for
	// every mark placed after them: Beethoven's ninth is two hundred
	// thousand glyphs and spent a fifth of its layout here.
	from := e.staffGlyphs
	if from > len(e.lay.Glyphs) {
		from = len(e.lay.Glyphs)
	}
	for _, g := range e.lay.Glyphs[from:] {
		// The cheap tests first: a coordinate compares in an instruction
		// and a name does not.
		if g.Y < top || g.Y > bot || !strings.HasPrefix(g.Name, "notehead") {
			continue
		}
		if g.X > x+reach || g.X+smufl.Advance(g.Name) < x-reach {
			continue
		}
		if above {
			if want := g.Y - air; want < y {
				y = want
			}
		} else if want := g.Y + air; want > y {
			y = want
		}
	}
	return y
}

// ornament writes a mordent, a turn or the like above the staff.
//
// Above, always, whichever way the stem points. An ornament is not an
// articulation and does not follow the stem the way a staccato dot does:
// it is a direction to play notes that are not written, it is read along
// the top of the line with the trills and the rehearsal letters, and it is
// written above the staff in every printed score there is.
//
// Which is why it cannot be placed beside the notehead. A note with its
// stem up has the stem in exactly the place the ornament has to go, and a
// mordent drawn there comes out with the stem through the middle of it. So
// it waits for the staff to be finished and is then placed clear of
// everything on it — the stem, the beam over it, the ledger lines, the slur
// — the same way the dynamics and the octave lines are.
// centred is where to put a glyph's origin so that its ink sits centred on
// x, and how far its ink then reaches either side.
//
// A glyph is drawn from its origin, which is its left-hand edge and not its
// middle, so a mark meant to sit over a note has to be moved back by half of
// itself. Put at the note's centre instead, a tenuto landed 0.674 spaces to
// the right of it — more than half a notehead, which is what it looks like
// on the page: a row of dashes that do not line up with the notes under
// them.
//
// Centred on the *ink* rather than on the advance width, because the ink is
// what a reader sees and the two are not the same for every glyph. Where the
// font declares no box the advance is the best there is.
func centred(name string, x float64) (origin, left, right float64) {
	if b, ok := smufl.BBox(name); ok {
		origin = x - (b.Left+b.Right)/2
		return origin, origin + b.Left, origin + b.Right
	}
	w := smufl.Advance(name)
	return x - w/2, x - w/2, x + w/2
}

func (e *engraver) ornament(a model.Articulation, x float64) {
	name := smufl.ArticulationGlyph(a, true)
	if name == "" {
		return
	}
	// Centred on the note rather than starting at it: an ornament is a wide
	// sign and sits over the middle of what it ornaments.
	origin, x0, x1 := centred(name, x)
	e.wait(waitingMark{
		x0: x0, x1: x1,
		height: ornamentHeight, gap: 0.5, rank: rankOrnament,
		draw: func(y float64) {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: name, X: origin, Y: y,
			})
		},
	})
}

// ornamentHeight is how tall the ornament signs are, in staff spaces: a
// zigzag with a stroke through it is about one space of ink.
const ornamentHeight = 1.1

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

// separateColumn moves a voice aside where it would be written on top of the
// one above it.
//
// Two voices sounding together at a second — or at the same note — cannot
// both sit on the same side of their stems: the noteheads would be drawn
// over each other and the reader could not tell there were two of them. The
// lower one steps to the left, which is where an engraver puts it, and takes
// its stem with it.
//
// A third apart or more they clear each other and nothing is done, which is
// the common case and why most two-voice music looks untouched by this.
func (e *engraver) separateColumn(col []*placed) {
	if len(col) < 2 {
		return
	}
	// How high and how low each voice reaches at this moment. Rests have no
	// notehead to clash with, and are spaced by drawRest instead.
	type reach struct {
		lo, hi int
		ps     []*placed
	}
	byVoice := map[int]*reach{}
	var voices []int
	for _, p := range col {
		if p.rest != nil {
			continue
		}
		r, ok := byVoice[p.voice]
		if !ok {
			r = &reach{lo: p.lo, hi: p.hi}
			byVoice[p.voice] = r
			voices = append(voices, p.voice)
		}
		if p.lo < r.lo {
			r.lo = p.lo
		}
		if p.hi > r.hi {
			r.hi = p.hi
		}
		r.ps = append(r.ps, p)
	}
	if len(voices) < 2 {
		return
	}
	// Down the stack by where the voices actually sit, highest first, with
	// their numbering only to break a tie.
	//
	// Not by the numbering itself, which is what this did. The first voice
	// is usually the upper one and then the two orders agree — but a part
	// may write the held note as its second voice and the moving line as
	// its first, and Altes ends that way: the left hand holds an E and a G
	// while the voice numbered before them plays a triplet on the C below.
	// Read in number order, the C was taken for the upper note and the held
	// chord for the lower, the two were found to clash when a third apart
	// clears perfectly well, and the chord was pushed a notehead to the
	// left of the note it sounds with.
	sort.SliceStable(voices, func(a, b int) bool {
		ra, rb := byVoice[voices[a]], byVoice[voices[b]]
		if ra.hi != rb.hi {
			return ra.hi > rb.hi
		}
		if ra.lo != rb.lo {
			return ra.lo > rb.lo
		}
		return voices[a] < voices[b]
	})

	// Down the stack, each voice against the one above it. A larger step is
	// higher on the staff.
	for i := 1; i < len(voices); i++ {
		above, below := byVoice[voices[i-1]], byVoice[voices[i]]
		if above.lo-below.hi > 1 {
			continue
		}
		for _, p := range below.ps {
			p.x -= noteheadWidth(p.value)
		}
	}
}

// pendingSystemText is a text for the whole system, waiting for the system
// to be finished so it can be put above the top staff of it.
type pendingSystemText struct {
	t     model.TextMark
	lines []string
	look  model.TextLook
	x     float64
	line  float64
}

// placeSystemTexts writes the words that belong to the whole system over
// the top staff of it.
//
// After every staff has been drawn, for two reasons. A staff's outline is
// whatever went onto the page during its pass, so words put above the top
// staff from a lower staff's pass would make that lower staff appear to
// reach the top of the system — and everything it had queued would then be
// placed clear of a staff thirty spaces tall. And the top staff's own
// reach is not known until it has been drawn.
//
// Stacked where they would otherwise collide: two of them over the same
// bar — "Allegro" and "To Coda" at a double bar — were written at the same
// height and came out on top of each other.
func (e *engraver) placeSystemTexts() {
	if len(e.systemTexts) == 0 {
		return
	}
	// Nearest the staff first, so the stacking runs outwards from it.
	rows := []float64{} // the top of each row, and how far right it reaches
	right := []float64{}
	for _, p := range e.systemTexts {
		widest := 0.0
		for _, line := range p.lines {
			if w := textWidth(line, p.look.Size); w > widest {
				widest = w
			}
		}
		height := p.look.Size + float64(len(p.lines)-1)*p.line

		// The first row that is free where this text goes.
		row := 0
		for row < len(rows) && p.x < right[row]+textGap {
			row++
		}
		if row == len(rows) {
			top := e.systemTopReach - systemTextGap
			for i := 0; i < row; i++ {
				top -= rows[i]
			}
			rows = append(rows, height+systemTextGap)
			right = append(right, p.x+widest)
			e.writeSystemText(p, top, height)
			continue
		}
		top := e.systemTopReach - systemTextGap
		for i := 0; i < row; i++ {
			top -= rows[i]
		}
		if p.x+widest > right[row] {
			right[row] = p.x + widest
		}
		e.writeSystemText(p, top, height)
	}
	e.systemTexts = e.systemTexts[:0]
}

// writeSystemText puts one of them down, its bottom line at y.
func (e *engraver) writeSystemText(p pendingSystemText, y, height float64) {
	top := y - height + p.look.Size
	for i, line := range p.lines {
		if line == "" {
			continue
		}
		e.lay.Texts = append(e.lay.Texts, Text{
			S: line, X: p.x, Y: top + p.t.OffsetY + float64(i)*p.line,
			Size: p.look.Size, Anchor: "start",
			Italic: p.look.Italic, Bold: p.look.Bold,
		})
	}
}

const (
	// systemTextGap is the air between the music and the words over the
	// system, and between one row of them and the next.
	systemTextGap = 0.6
	// textGap is how far apart two of them have to be along the line
	// before they count as not colliding.
	textGap = 1.0
)
