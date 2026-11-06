/*
 * groups.go
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
	"github.com/stricaud/gscore/internal/textmetric"
)

// What is drawn down the left of a system: the staff names, and the braces and
// brackets that say which staves belong together.
//
// A stack of five staves with nothing joining them is five pieces of music on
// one page. The brace that joins a pianist's two hands and the bracket that
// gathers the strings are not decoration: they are what says where one
// instrument ends and the next begins, and a reader uses them before reading a
// single note.

const (
	// nameSize is the type the staff names are set in, in staff spaces.
	nameSize = 1.9
	// nameGap is the air between the name and what follows it.
	nameGap = 1.2
	// groupGap is the air between a bracket and the clefs behind it.
	groupGap = 0.9
	// bracketThickness is how heavy the vertical of a bracket is.
	bracketThickness = 0.5
	// measureNumberSize is the type a bar number is set in.
	measureNumberSize = 1.5
	// numberBoxPad is the air between a boxed bar number and its box.
	numberBoxPad = 0.4
	// stripHeight is how tall the band over a bar is: enough to hit without
	// aiming, and no taller than the bar numbers that sit in it.
	stripHeight = 2.4
	// stripGap is the air between the music and the band over it: enough
	// that a click on the topmost notehead is a click on the note and not
	// on the bar, and small enough that the band stays within reach of it.
	stripGap = 0.3
	// markerRoom is how much room a boxed rehearsal mark takes, so that
	// anything else written over the same bar goes above it.
	markerRoom = 3.0

	// markRowHeight is how tall the row over a bar is, which is as tall as
	// the tallest thing that goes in it — the segno, drawn at signSize and
	// reaching half of that either way.
	//
	// Everything in the row sits on its lower edge, so that whatever is
	// placed under the row clears all of it and not just the letter. The
	// segno hanging below the band it was given is how an 8va came to have
	// a sign through it again after the row itself was placed correctly.
	markRowHeight = 3.2

	// markBoxHeight is the boxed letter, which is shorter than the row and
	// sits on the same edge.
	markBoxHeight = 2.7
)

// partLabel is what a staff is called at the head of a system.
//
// After the first system it is abbreviated, because the full name is already
// known by then and the music is what wants the room. "Violoncello" becomes
// "Vc." the way it does in every printed score.
func partLabel(p *model.Part, first bool) string {
	name := p.Name
	if name == "" {
		name = p.Instrument.String()
	}
	if name == "" || first {
		return name
	}
	return abbreviate(name)
}

// abbreviate shortens a name the way a score does: the first few letters of
// each word, with a full stop.
func abbreviate(name string) string {
	runes := []rune(name)
	if len(runes) <= 4 {
		return name
	}
	// Two words become their initials — "Alto Sax" is "A. Sx." in nobody's
	// score, so only the first word is shortened and the rest dropped.
	cut := 3
	for i, r := range runes {
		if r == ' ' {
			cut = i
			break
		}
	}
	if cut > 3 {
		cut = 3
	}
	return string(runes[:cut]) + "."
}

// labelWidth is how much room the names need, so the music starts in the same
// column on every staff.
//
// It is measured rather than guessed at because a name that does not fit is
// drawn over the clef, which is worse than not drawing it at all.
func (e *engraver) labelWidth(parts []*model.Part, first bool) float64 {
	if !e.opt.ShowPartNames {
		return 0
	}
	widest := 0.0
	braced := e.bracedStaves()
	for i, p := range parts {
		w := textWidth(partLabel(p, first), nameSize)
		if braced[i] {
			// The brace stands between this staff and its name, so the
			// margin has to hold both. A grand staff that names no group
			// labels each of its staves, and without the allowance the
			// words "Right Hand" and "Left Hand" were written across the
			// brace that joins them.
			w += braceRoom + groupGap*0.5
		}
		if w > widest {
			widest = w
		}
	}
	columns := e.groupColumns()
	for i, g := range e.score.Groups {
		if g.Name == "" {
			continue
		}
		label := g.Name
		if !first {
			label = abbreviate(label)
		}
		w := textWidth(label, nameSize)
		if g.Symbol == model.GroupBrace {
			// A brace stands a space or two out to the left of its staves
			// and the name goes outside that, not across it. A bracket is
			// a line and needs no such allowance, which is why only the
			// braces widen the margin.
			w += braceRoom
		}
		// And a group inside another stands further out again, with its
		// name outside that.
		w += -columns[i]
		if w > widest {
			widest = w
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + nameGap
}

// textWidth guesses how wide a word is.
//
// The font's own metrics are not in hand here — the names are set in the
// reader's serif face, not in Bravura — so this is an average character width,
// chosen generously. Leaving too much room pushes the music right; leaving too
// little puts a name through a clef, and only one of those is a mistake a
// reader notices.
func textWidth(s string, size float64) float64 {
	return textmetric.Width(s, size, textmetric.Style{})
}

// boldWidth is the same for text set bold, which is wider.
//
// Measuring bold text with the roman widths is how a boxed rehearsal letter
// came to sit over the right-hand edge of its own box: a bold C is 0.722 of
// the type size against the roman 0.667, and at the size a rehearsal mark is
// set that is a sixth of a staff space of difference, on a box with four
// tenths of padding.
func boldWidth(s string, size float64) float64 {
	return textmetric.Width(s, size, textmetric.Style{Bold: true})
}

// bracedStaves marks the drawn staves that have a brace between them and
// whatever is written to their left.
func (e *engraver) bracedStaves() map[int]bool {
	out := map[int]bool{}
	for _, g := range e.score.Groups {
		if g.Symbol != model.GroupBrace {
			continue
		}
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 || li <= fi {
			continue
		}
		for i := fi; i <= li; i++ {
			out[i] = true
		}
	}
	return out
}

// Nested groups, and the column each one stands in.
//
// A bracket round a family that has a piano in it is two symbols on one
// system: the bracket round the family and the brace round the pianist's
// two staves. Drawn in the same column they sit on top of each other, and
// the brace comes off worst — its arms swell out to the left, so it
// crosses the bracket rather than standing inside it.
//
// The rule in print is that the outermost goes furthest from the music and
// each one inside it steps towards the staves. So the depth of a group —
// how many others contain it — is which column it stands in, and the
// columns are spaced by the room the symbols in them actually take.

// groupDepth is how many groups contain this one.
//
// Strictly contain: two groups over the same staves are not nested, they
// are the same span said twice, and stepping one of them aside would make
// the page say there is a structure that is not there.
func (e *engraver) groupDepth(at int) int {
	g := e.score.Groups[at]
	n := 0
	for i, other := range e.score.Groups {
		if i == at {
			continue
		}
		if other.First <= g.First && other.Last >= g.Last &&
			(other.First < g.First || other.Last > g.Last) {
			n++
		}
	}
	return n
}

// groupReach is how far left of its own column a symbol swells.
//
// A brace is drawn from its column leftwards and is the only one that
// takes real room; a bracket and a line are verticals half their own
// thickness either side.
func groupReach(g model.StaffGroup) float64 {
	if g.Symbol == model.GroupBrace {
		// The widest a brace gets, rather than this brace's own width:
		// the column has to be in the same place on every system, and a
		// brace drawn over four staves is wider than one over two.
		return braceRoom
	}
	return bracketThickness
}

// groupColumns is how far left of the usual column each group stands, by
// its place in Score.Groups. Zero for a group that nothing contains.
//
// Worked out once: the groups do not change while a drawing is being made,
// and every system asks the same question.
func (e *engraver) groupColumns() []float64 {
	if e.columns != nil {
		return e.columns
	}
	out := make([]float64, len(e.score.Groups))
	if len(out) == 0 {
		e.columns = out
		return out
	}
	// The widest symbol at each depth, which is what the step between
	// that column and the one outside it has to clear.
	deepest := 0
	widest := map[int]float64{}
	depths := make([]int, len(out))
	for i := range e.score.Groups {
		d := e.groupDepth(i)
		depths[i] = d
		if d > deepest {
			deepest = d
		}
		if r := groupReach(e.score.Groups[i]); r > widest[d] {
			widest[d] = r
		}
	}
	// Innermost keeps the column everything used before, so a score with
	// no nesting is drawn exactly where it was.
	at := map[int]float64{deepest: 0}
	for d := deepest; d > 0; d-- {
		at[d-1] = at[d] - widest[d] - groupGap*0.5
	}
	for i := range out {
		out[i] = at[depths[i]]
	}
	e.columns = out
	return out
}

// groupRoom is the extra width the nesting needs, over the one column a
// score without nesting takes.
func (e *engraver) groupRoom() float64 {
	room := 0.0
	for _, x := range e.groupColumns() {
		if -x > room {
			room = -x
		}
	}
	return room
}

// drawGroups draws the braces and brackets at the left of a system, and the
// names beside them.
func (e *engraver) drawGroups(parts []*model.Part, left float64, tops []float64, first bool) {
	if len(tops) == 0 {
		return
	}

	columns := e.groupColumns()

	// How far left the ink reaches in front of each staff, so that a name
	// beside it can be put outside all of it.
	//
	// Measured from what is actually drawn and not guessed at. It used to
	// be guessed — the name of a braced staff stepped left by the width
	// of a brace over twice the staff height — and a guess is wrong the
	// moment the brace is a different height or there is a bracket
	// outside it as well. Both happened, and the words were printed
	// across the symbols.
	edge := make([]float64, len(tops))
	for i := range edge {
		edge[i] = left - groupGap - 0.4
	}
	reach := func(fi, li int, at float64) {
		for i := fi; i <= li && i < len(edge); i++ {
			if at < edge[i] {
				edge[i] = at
			}
		}
	}

	for gi, g := range e.score.Groups {
		// A group names staves by their number in the score, and the ones
		// drawn are only those not hidden. Hiding the lower half of a piano
		// should not leave its brace reaching into the staff below.
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 || li <= fi || li >= len(tops) {
			continue
		}
		top := tops[fi]
		bottom := tops[li] + partH(parts[li])
		// Its own column: the outermost furthest from the music, each one
		// inside it a step nearer the staves.
		x := left - groupGap + columns[gi]

		switch g.Symbol {
		case model.GroupBrace:
			e.drawBrace(x-0.4, top, bottom)
			reach(fi, li, x-0.4-braceWidth(bottom-top))
		case model.GroupLine:
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: top, X2: x, Y2: bottom, Thickness: bracketThickness / 2,
			})
			reach(fi, li, x-bracketThickness/4)
		default:
			e.drawBracket(x, top, bottom)
			reach(fi, li, x-bracketThickness/2)
		}
	}

	// The group names, outside everything standing in front of their
	// staves — their own symbol and any that encloses it.
	if e.opt.ShowPartNames {
		for _, g := range e.score.Groups {
			if g.Name == "" {
				continue
			}
			fi, ok1 := e.drawnAt(g.First)
			li, ok2 := e.drawnAt(g.Last)
			if !ok1 || !ok2 || li <= fi || li >= len(tops) {
				continue
			}
			label := g.Name
			if !first {
				label = abbreviate(label)
			}
			top := tops[fi]
			bottom := tops[li] + partH(parts[li])
			e.lay.Texts = append(e.lay.Texts, Text{
				S: label, X: edge[fi] - groupGap*0.5,
				Y:    (top+bottom)/2 + nameSize*0.35,
				Size: nameSize, Anchor: "end",
			})
		}
	}

	if !e.opt.ShowPartNames {
		return
	}
	// A staff inside a named group is already labelled by the group.
	named := map[int]bool{}
	for _, g := range e.score.Groups {
		if g.Name == "" {
			continue
		}
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 {
			continue
		}
		for i := fi; i <= li && i < len(tops); i++ {
			named[i] = true
		}
	}
	for i, p := range parts {
		if i >= len(tops) || named[i] {
			continue
		}
		label := partLabel(p, first)
		if label == "" {
			continue
		}
		// Outside whatever stands in front of this staff, which is what
		// edge holds. Moonlight Sonata's grand staff carries no group
		// name, so each of its two staves is labelled on its own, and
		// both words were drawn over the brace.
		at := edge[i] - groupGap*0.5
		e.lay.Texts = append(e.lay.Texts, Text{
			// Centred on the staff's middle line, which is where a name sits
			// in every printed score.
			S: label, X: at,
			Y:    tops[i] + partH(parts[i])/2 + nameSize*0.35,
			Size: nameSize, Anchor: "end",
		})
	}
}

// drawGroupBarlines runs the barline down through the gap between the staves
// of a group, so that a pianist's two hands are one instrument on the page.
func (e *engraver) drawGroupBarlines(parts []*model.Part, x float64, tops []float64, full bool) {
	d := e.opt.Defaults
	for _, g := range e.score.Groups {
		if !g.Barlines {
			continue
		}
		fi, ok1 := e.drawnAt(g.First)
		li, ok2 := e.drawnAt(g.Last)
		if !ok1 || !ok2 || li <= fi || li >= len(tops) {
			continue
		}
		if full {
			// At the head of a system there is no barline on any staff to
			// join up to: the staves simply begin. So the line is drawn
			// whole, from the top line of the first staff to the bottom
			// line of the last, which is the line a printed grand staff
			// starts with. Filling only the gaps left it hanging between
			// the staves, attached to nothing at either end.
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: tops[fi],
				X2: x, Y2: tops[li] + partH(parts[li]),
				Thickness: d.ThinBarlineThickness,
			})
			continue
		}
		// Between bars each staff has drawn its own barline through itself,
		// so only the gaps are left to fill.
		for i := fi; i < li; i++ {
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x, Y1: tops[i] + partH(parts[i]), X2: x, Y2: tops[i+1],
				Thickness: d.ThinBarlineThickness,
			})
		}
	}
}

// Sizes for the row of directions over a system.
const (
	// jumpTextSize is the type "D.C. al Fine" and "Fine" are set in. The
	// same size as a rehearsal mark, because they are read in the same
	// glance and one larger than the other would say it mattered more.
	jumpTextSize = 2.4
	// signSize is the type size of the segno and coda glyphs, in staff
	// spaces. They are drawn a little larger than the words beside them,
	// which is how every printed chart sets them.
	signSize = 3.2
	// afterMark is the gap left after something in the row before the next
	// thing may start.
	afterMark = 0.8
)

// drawMarkers writes the rehearsal marks, the signs, the tempo changes and
// the jump directions over a system.
//
// All of them go in one row above the top staff, and all of them are taken
// from whichever part has them: "from letter B", the segno, "slower" and
// "D.S. al Coda" each have to mean the same bar to every player in the room,
// so a direction that belonged to one staff would be worse than none.
//
// They share a row because they share a bar more often than not — a chart
// puts the segno at letter A and goes back to tempo there as a matter of
// course. Each is placed after the last thing written over that bar rather
// than at the bar line, which is what keeps the sign from being printed
// through the letter.
// markerText is what is printed over a bar, with the sequence worked out.
//
// A mark is stored as a place in the sequence rather than as the letter,
// so the letter has to be derived — and derived against the whole score,
// because what a mark is called depends on every mark before it. Worked
// out once and kept: this is asked for every bar of every system, and the
// answer cannot change while a drawing is being made.
func (e *engraver) markerText(mi int) string {
	if e.marks == nil {
		e.marks = e.score.RehearsalMarks(BarNumbers(e.score))
		if e.marks == nil {
			e.marks = map[int]string{}
		}
	}
	return e.marks[mi]
}

func (e *engraver) drawMarkers(parts []*model.Part, columns []float64, start, end int) {
	at := func(mi int) *model.Measure {
		for _, p := range parts {
			if mi < len(p.Measures) {
				m := p.Measures[mi]
				if m.HasMarker() || m.Sign != model.NoSign || m.Jump != nil {
					return m
				}
			}
		}
		return nil
	}
	tempoAt := func(mi int) *model.Tempo {
		for _, p := range parts {
			if mi < len(p.Measures) && p.Measures[mi].Tempo != nil {
				return p.Measures[mi].Tempo
			}
		}
		return nil
	}

	for mi := start; mi < end; mi++ {
		m := at(mi)
		tempo := tempoAt(mi)
		if m == nil && tempo == nil {
			continue
		}
		x := columns[mi-start] + e.numberWidth(mi, start)
		// One height for everything written over this bar, so that a letter,
		// a sign and a change of speed sit on one line rather than at three
		// heights. The row is found once, against whatever is under it —
		// which by now includes the bar number, the volta bracket and any
		// note reaching up out of the staff.
		right := columns[len(columns)-1]
		if mi-start+1 < len(columns) {
			right = columns[mi-start+1]
		}
		e.wait(waitingMark{
			x0: x, x1: right, height: markRowHeight, gap: 0.6, rank: rankMarks,
			draw: func(row float64) { e.drawMarkRow(m, tempo, columns, mi, start, x, row) },
		})
	}
}

// markRepeatStaves are the drawn staves that carry the rehearsal letter
// again, besides the top one.
//
// The top staff of each bracketed family. A bracket is the score saying
// "these belong together" — the strings, the woodwind, a rhythm section —
// and the top of each is where its players look. That is the rule printed
// scores follow, and taking it from the brackets means it is already
// answered by a score somebody has laid out, with nothing further to set.
//
// A brace is not a bracket here. It joins the staves one player reads, and
// a pianist does not need the letter twice.
func (e *engraver) markRepeatStaves() map[int]bool {
	out := map[int]bool{}
	if !e.opt.RepeatMarkers || e.score == nil {
		return out
	}
	for _, g := range e.score.Groups {
		if g.Symbol != model.GroupBracket {
			continue
		}
		// Only where the family actually begins below the top of the
		// page: a bracket round the whole score repeats nothing, because
		// the mark is already there.
		if fi, ok := e.drawnAt(g.First); ok && fi > 0 {
			out[fi] = true
		}
	}
	return out
}

// repeatMarkers queues the rehearsal letters again over one staff further
// down the system.
//
// Only the letter. The segno, the speed and the jump directions stay where
// they are: a conductor's score writes "D.S. al Coda" once, and printing
// it over every family would be four copies of one instruction. A letter
// is a place rather than an instruction, which is why it is the one thing
// that is repeated.
//
// Queued rather than drawn, so that it clears whatever this staff has
// reaching up out of it, the same way the row over the top staff does.
func (e *engraver) repeatMarkers(parts []*model.Part, columns []float64, start, end int) {
	for mi := start; mi < end && mi-start < len(columns); mi++ {
		text := e.markerText(mi)
		if text == "" {
			continue
		}
		// The same place along the bar as the letter at the top of the
		// page, so the repeats stand in one column down the system.
		//
		// There is no bar number to clear down here — the numbers are
		// written over the top staff only — but starting at the barline
		// instead put each repeat a little to the left of the one above
		// it, and a column of letters that does not line up reads as a
		// mistake even when nobody can say what is wrong with it.
		x := columns[mi-start] + e.numberWidth(mi, start)
		right := columns[len(columns)-1]
		if mi-start+1 < len(columns) {
			right = columns[mi-start+1]
		}
		e.wait(waitingMark{
			x0: x, x1: right, height: markRowHeight, gap: 0.6, rank: rankMarks,
			draw: func(row float64) { e.drawRehearsalMark(text, x, row) },
		})
	}
}

// drawMarkRow writes everything that belongs over one bar, once the row it
// sits in has been found: the letter, the sign, the new speed and the
// direction at the end of the bar, all on one line.
func (e *engraver) drawMarkRow(m *model.Measure, tempo *model.Tempo,
	columns []float64, mi, start int, x, row float64) {
	if m != nil {
		if text := e.markerText(mi); text != "" {
			x = e.drawRehearsalMark(text, x, row) + afterMark
		}
		if g := m.Sign.Glyph(); g != "" {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: g, X: x, Y: row - 1.6, Size: signSize,
			})
			x += smufl.Advance(g)*signSize/4 + afterMark
		}
		if label := m.Sign.Label(); label != "" {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: label, X: x, Y: row - 0.6, Size: jumpTextSize,
				Anchor: "start", Bold: true, Italic: true,
			})
			x += textWidth(label, jumpTextSize) + afterMark
		}
	}

	// The new speed, after the letter rather than through it. The mark at
	// the head of the score is the same thing written over bar one, so
	// this is drawn the same way and looks like it.
	if tempo != nil && e.opt.ShowTempoMark {
		w := e.drawTempoAt(*tempo, x, row-0.6)
		e.noteTempoBox(mi, *tempo, x, row-0.6, w)
		x += w + afterMark
	}

	// The direction is written at the end of the bar, because that is
	// where the player reaches it: a D.C. printed at the head of its bar
	// says to go back before playing the bar it is in.
	if m != nil && m.Jump != nil {
		if mi-start+1 < len(columns) {
			e.lay.Texts = append(e.lay.Texts, Text{
				S: m.Jump.Label(), X: columns[mi-start+1] - 0.4, Y: row - 0.6,
				Size: jumpTextSize, Anchor: "end", Bold: true, Italic: true,
			})
		}
	}
}

// drawRehearsalMark writes one boxed letter and returns where it ends.
//
// The box is what makes a rehearsal mark findable when somebody says "from B"
// and everybody in the room is looking for a letter.
// row is the bottom edge of the band the mark sits in, found against
// whatever is under it; everything in the band is measured from there.
func (e *engraver) drawRehearsalMark(text string, x, row float64) float64 {
	// Measured bold, because it is set bold, and centred in the box rather
	// than started at its left edge. Started there and measured roman, a C
	// ran over the right-hand edge of its own box and an M would have
	// crossed it by a staff space.
	//
	// Centred as well as measured: the two renderers set this in different
	// faces — Times in a PDF, Georgia in a page — so the width is right for
	// one of them and close for the other, and a letter centred in its box
	// is off by half of a small difference instead of all of it.
	w := boldWidth(text, jumpTextSize) + 0.8
	e.lay.Texts = append(e.lay.Texts, Text{
		S: text, X: x - 0.4 + w/2, Y: row - 0.6, Size: jumpTextSize,
		Anchor: "middle", Bold: true,
	})
	t := e.opt.Defaults.StaffLineThickness * 1.3
	y0, y1 := row-markBoxHeight, row
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: x - 0.4, Y1: y0, X2: x - 0.4 + w, Y2: y0, Thickness: t},
		Line{X1: x - 0.4, Y1: y1, X2: x - 0.4 + w, Y2: y1, Thickness: t},
		Line{X1: x - 0.4, Y1: y0, X2: x - 0.4, Y2: y1, Thickness: t},
		Line{X1: x - 0.4 + w, Y1: y0, X2: x - 0.4 + w, Y2: y1, Thickness: t})
	return x - 0.4 + w
}

// drawMeasureNumbers writes the bar numbers over a system.
//
// Where they go depends on how the music is being looked at, because the
// question they answer is different in each case.
//
// On a page, the number belongs over the first bar of each system: that is
// where a player looks when someone says "from bar forty-one", and a number
// over every bar is clutter between the staves.
//
// A continuous strip has only ever one system, so numbering its first bar
// prints a single "1" and nothing else — which is what made the menu entry
// look like it did nothing at all. Editing is also when you most want to find
// a bar by its number, so there every bar is numbered.
// numberWidth is how much room the bar number at this bar takes, so that
// whatever else is written over the bar starts after it rather than through
// it. Zero where no number is written there.
func (e *engraver) numberWidth(mi, start int) float64 {
	n, ok := e.numberedAt(mi, start)
	if !ok {
		return 0
	}
	w := textWidth(itoa(n), measureNumberSize) + 0.6
	if e.score != nil && e.score.Numbering.Boxed {
		w += numberBoxPad * 2
	}
	return w
}

// numberedAt says whether a bar carries a printed number, and what it is.
//
// One answer for both the drawing and the room reserved beside it. They
// were two pieces of arithmetic that had to agree, and the moment a score
// could ask for its own numbering they stopped agreeing: a rehearsal mark
// was pushed aside for a number that was not printed there, or printed on
// top of one that was.
func (e *engraver) numberedAt(mi, start int) (int, bool) {
	if !e.opt.ShowMeasureNumbers {
		return 0, false
	}
	// A bar swallowed by a multi-measure rest is not numbered: its column
	// is the one the run ends in, and the number would be printed on top
	// of the next bar's. The run keeps the number of the bar it begins
	// at, which is the one a player counts from.
	if e.multiHidden[mi] {
		return 0, false
	}
	n := e.barNumber(mi)
	if n <= 1 {
		return 0, false // bar one needs no telling
	}
	// Counted on the printed number and not on how far through the piece
	// the bar is, so "every five" means the bars called 5, 10, 15 — which
	// is what somebody calling them out is reading.
	if every := e.numberEvery(); every > 0 {
		return n, n%every == 0
	}
	// Nothing asked for: the first bar of each system on a page, and
	// every bar in the strip.
	if e.opt.Mode != Strip && mi != start {
		return 0, false
	}
	return n, true
}

// barNumber is the number printed over a bar.
//
// Bar one is the first full bar, so a piece that opens with a pickup has it
// as its second: an anacrusis is not counted, which is how every edition
// numbers one and what NoteWorthy's own ruler shows. Numbered from the first
// bar whatever it held, Für Elise ran one ahead of the file it came from all
// the way to the last page — the bar NoteWorthy calls 65 was printed as 66,
// which is the kind of difference that only shows when two people are
// reading from different copies and one of them says "from bar sixty-five".
func (e *engraver) barNumber(mi int) int {
	if e.numbers == nil {
		e.numbers = BarNumbers(e.score)
	}
	return numberOf(e.numbers, mi)
}

// NumberOf reads one bar's number out of the list, and counts on past
// the end of it.
//
// Past the end is a real place: the strip keeps an empty bar open after
// the music for the next note to go in, and it is numbered like any
// other. Starting again from where it sits in the score rather than
// counting on gave a piece with an upbeat the numbers 2, 3, then 5.
func NumberOf(numbers []int, at int) int {
	switch {
	case at < 0 || len(numbers) == 0:
		return at + 1
	case at < len(numbers):
		return numbers[at]
	}
	last := len(numbers) - 1
	return numbers[last] + (at - last)
}

func numberOf(numbers []int, at int) int { return NumberOf(numbers, at) }

// BarNumbers is the number printed over each bar of a score.
//
// Exported, and worked out in one pass, because more than the engraver
// needs the answer: a script that reports "a parallel fifth in bar 41"
// has to say the number a player would read, and a second copy of this
// arithmetic somewhere else is a report that sends them to the wrong
// page. There was one, and it was already wrong — it did not know about
// a numbering the file had stated for itself, so a Chopin nocturne came
// back one short all the way through.
//
// The rule, in the order it is applied:
//
//	a restart        the writer said this bar is called n, so it is, and
//	                 the ones after it count on from there. Stated beats
//	                 inferred, because somebody said it out loud.
//	a movement       numbering starts again at each one: the Presto's
//	                 fourth bar is its own fourth, not the sonata's
//	                 hundred and thirty-seventh.
//	an upbeat        is not counted, so bar one is the first full bar.
func BarNumbers(s *model.Score) []int {
	if s == nil {
		return nil
	}
	n := s.MeasureCount()
	out := make([]int, n)

	// Where each bar's movement begins, in one pass.
	from := 0
	starts := make([]int, n)
	for i := 0; i < n; i++ {
		starts[i] = from
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].EndsSection {
				from = i + 1
				break
			}
		}
	}
	// And whether each movement opens with an upbeat, asked once per
	// movement rather than once per bar.
	upbeat := map[int]bool{}
	for i := 0; i < n; i++ {
		if _, seen := upbeat[starts[i]]; !seen {
			upbeat[starts[i]] = pickupAt(s, starts[i])
		}
	}

	// A restart carries forward until the next one.
	restartAt, restartTo := -1, 0
	for i := 0; i < n; i++ {
		for _, p := range s.Parts {
			if i < len(p.Measures) && p.Measures[i].NumberFrom != nil {
				restartAt, restartTo = i, *p.Measures[i].NumberFrom
				break
			}
		}
		if restartAt >= 0 {
			out[i] = restartTo + (i - restartAt)
			continue
		}
		at := i - starts[i]
		if upbeat[starts[i]] {
			out[i] = at
		} else {
			out[i] = at + 1
		}
	}
	return out
}

// numberRestart is the nearest bar at or before this one where the writer
// restarted the numbering, and the number they restarted it at.
//
// At or before, so that every bar after a restart counts on from it until
// the next one. Walked backwards from the bar being asked about rather
// than built into a table: a score has a handful of these or none, and a
// table would be one entry per bar of the piece to hold them.
//
// Read from whichever staff carries it, the way a rehearsal mark is: "bar
// forty" has to mean the same bar to everybody in the room, so a restart
// written on one staff is a restart for the score.
func (e *engraver) numberRestart(mi int) (from, number int, ok bool) {
	for i := mi; i >= 0; i-- {
		for _, p := range e.score.Parts {
			if i < len(p.Measures) && p.Measures[i].NumberFrom != nil {
				return i, *p.Measures[i].NumberFrom, true
			}
		}
	}
	return 0, 0, false
}

// sectionFrom is the first bar of the movement a bar belongs to.
//
// Numbering restarts with every movement, because that is how a movement is
// read and how it is cited: the Presto's fourth bar is its own fourth, not
// the sonata's hundred and thirty-seventh. Every printed edition numbers
// them that way and so does MuseScore, which is where these scores are
// written.
func (e *engraver) sectionFrom(mi int) int {
	if e.sections == nil {
		n := e.score.MeasureCount()
		e.sections = make([]int, n)
		from := 0
		for i := 0; i < n; i++ {
			e.sections[i] = from
			for _, p := range e.score.Parts {
				if i < len(p.Measures) && p.Measures[i].EndsSection {
					from = i + 1
					break
				}
			}
		}
	}
	if mi < 0 || mi >= len(e.sections) {
		return 0
	}
	return e.sections[mi]
}

// opensWithPickup reports whether the piece opens with an anacrusis: a first
// bar shorter than its meter, with a full bar after it.
//
// Both halves matter. Short on its own is not enough — a bar still being
// typed into is short, and so is every bar of a piece whose meter gscore has
// read wrongly or not at all. Two of the MusicXML suite's rhythm cases open
// with a bar that is short by their own meter and go on being short, and
// renumbering those would have moved every number on the page because the
// reader could not count. A pickup is short and then whole, which is the
// one shape that means somebody began before the first beat.
func opensWithPickup(s *model.Score) bool { return pickupAt(s, 0) }

// pickupAt is the same question asked of the bar a movement opens with.
//
// Numbering restarts with every movement, so every movement gets the same
// decision about its own first bar: the Allegretto of the Moonlight sonata
// opens with a quarter-note upbeat, and its first full bar is its bar one.
func pickupAt(s *model.Score, at int) bool {
	if s == nil || s.MeasureCount() < at+2 {
		return false
	}
	bar := model.Duration{}
	for _, p := range s.Parts {
		if at >= len(p.Measures) {
			continue
		}
		bar = p.TimeAt(at).BarDuration()
		break
	}
	if bar.IsZero() {
		return false
	}
	opening := s.MeasureDuration(at)
	if opening.IsZero() || opening.Cmp(bar) >= 0 {
		return false
	}
	return s.MeasureDuration(at+1).Cmp(bar) == 0
}

func (e *engraver) drawMeasureNumbers(columns []float64, start, end int) {
	if !e.opt.ShowMeasureNumbers {
		return
	}
	boxed := e.score != nil && e.score.Numbering.Boxed
	write := func(n int, x float64) {
		w := textWidth(itoa(n), measureNumberSize)
		// Room for the box, where there is one, so that the thing placed
		// above this clears the box and not just the numeral inside it.
		h := measureNumberSize
		if boxed {
			w += numberBoxPad * 2
			h = markBoxHeight
		}
		e.wait(waitingMark{
			x0: x, x1: x + w, height: h, gap: 0.5,
			rank: rankBarNumber,
			draw: func(y float64) {
				if boxed {
					e.drawNumberBox(itoa(n), x, y)
					return
				}
				e.lay.Texts = append(e.lay.Texts, Text{
					S: itoa(n), X: x, Y: y, Size: measureNumberSize, Anchor: "start",
				})
			},
		})
	}

	for mi := start; mi < end && mi-start < len(columns); mi++ {
		if n, ok := e.numberedAt(mi, start); ok {
			write(n, columns[mi-start])
		}
	}
}

// numberEvery is how often this score asks for a bar number, or zero where
// it has not said.
func (e *engraver) numberEvery() int {
	if e.score == nil || e.score.Numbering.Every <= 0 {
		return 0
	}
	return e.score.Numbering.Every
}

// drawNumberBox writes a bar number inside a rectangle.
//
// The same box a rehearsal mark gets, and for the same reason: on a dense
// page a bare numeral is lost among the music, and a conductor calling
// "from forty-one" needs it found at a glance rather than read for.
func (e *engraver) drawNumberBox(text string, x, y float64) {
	e.lay.Texts = append(e.lay.Texts, Text{
		S: text, X: x + numberBoxPad, Y: y,
		Size: measureNumberSize, Anchor: "start",
	})
	w := textWidth(text, measureNumberSize) + numberBoxPad*2
	t := e.opt.Defaults.StaffLineThickness * 1.3
	y0, y1 := y-measureNumberSize, y+measureNumberSize*0.25
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: x, Y1: y0, X2: x + w, Y2: y0, Thickness: t},
		Line{X1: x, Y1: y1, X2: x + w, Y2: y1, Thickness: t},
		Line{X1: x, Y1: y0, X2: x, Y2: y1, Thickness: t},
		Line{X1: x + w, Y1: y0, X2: x + w, Y2: y1, Thickness: t})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// unused keeps the smufl import honest if the brace glyph is ever dropped.
var _ = smufl.Advance

// drawnAt turns a staff's number in the score into its position among the
// ones being drawn, or says it is not being drawn at all.
func (e *engraver) drawnAt(staff int) (int, bool) {
	for i, n := range e.origin {
		if n == staff {
			return i, true
		}
	}
	return 0, false
}

const (
	// bracketExtend is how far the vertical runs past the outermost staff
	// line before the claw starts. A bracket that stopped exactly on the
	// line would read as a barline with decoration.
	bracketExtend = 0.18
)

// drawBracket draws the bracket that gathers the staves of a family: the
// strings, the woodwind, a rhythm section.
//
// A heavy vertical, and at each end the claw that turns back towards the
// music. What was drawn here before was three straight lines — the vertical
// and a diagonal serif at either end — and it read as an arrow: a serif of
// even width cannot come to a point, and the corner where it met the
// vertical showed as a notch at every size.
//
// The claws are the font's, not this file's. A bracket end is a drawn shape
// with a crest and a point, and Bravura has both of them — bracketTop and
// bracketBottom, designed against the same staff space everything else here
// is measured in. Working them out again as Bézier curves was tried and got
// a shape that was recognisably almost right, which is the worst thing a
// piece of engraving can be: the font was drawn by somebody who does this,
// and SMuFL is the font interface this program already has.
//
// The vertical stays a line, because a line of even width is exactly what
// it is.
func (e *engraver) drawBracket(x, top, bottom float64) {
	if bottom <= top {
		return
	}
	// The vertical stops where the claws take over. They overlap it by
	// bracketExtend, so there is no seam at the join to show as a pale
	// line between two black things.
	e.lay.Lines = append(e.lay.Lines, Line{
		X1: x, Y1: top - bracketExtend,
		X2: x, Y2: bottom + bracketExtend,
		Thickness: bracketThickness,
	})
	// Placed on the centre line of the vertical: the glyphs are drawn
	// from there outwards, up at the top and down at the bottom.
	e.lay.Glyphs = append(e.lay.Glyphs,
		Glyph{Name: "bracketTop", X: x - bracketThickness/2, Y: top - bracketExtend},
		Glyph{Name: "bracketBottom", X: x - bracketThickness/2, Y: bottom + bracketExtend})
}

// The curly brace that joins the staves of one instrument.
//
// Set as a glyph, which this did not used to be. It was drawn by hand on
// the grounds that a brace has to stretch to however many staves it joins
// and a glyph can only be scaled both ways at once — true in general, and
// not true of this one. Bravura's brace is cut 0.277 staff spaces wide and
// 4.005 tall, which is to say drawn tall and thin on purpose so that
// scaling it up gives a brace of the right weight: at the height of a
// grand staff it comes out about 1.4 spaces across, which is where a brace
// belongs, and a taller one comes out wider, which is what a taller brace
// does in print.
//
// What the drawn one could not get was the shape. A brace is not two
// curves of varying width; it is a cut letter, with a flat on the inside
// of each arm, a swell that is not symmetrical about the cusp, and tips
// that come to a hair. That is a thing a type designer did, and the font
// has it.

const (
	// braceNaturalW and braceNaturalH are the glyph's own box, in staff
	// spaces, at the size the font is quoted at — four spaces to the em.
	// Measured from Bravura: 0 to 69 across and -1 to 1001 up, per 1000
	// units to the em.
	braceNaturalW = 0.277
	braceNaturalH = 4.005

	// braceRoom is how much width to keep in front of the staves for a
	// brace, where the height it will be drawn to is not yet known.
	//
	// A reservation and not a measurement: labelWidth runs before the
	// staves have been spaced, so it cannot ask how tall any brace is
	// going to be. Three spaces is the width of a brace over four
	// staves, which is past anything one player reads — an organist's
	// three is the most of it — so the room is there and a little of it
	// goes unused.
	braceRoom = 3.0
)

// braceWidth is how far a brace's cusp stands out to the left of the
// staves it joins, at the height it is drawn to.
//
// Read off the glyph rather than chosen: the width is whatever scaling it
// to this height makes it, and anything reserving room in front of the
// staves has to agree with what is actually drawn.
func braceWidth(h float64) float64 {
	if h <= 0 {
		return 0
	}
	return h * braceNaturalW / braceNaturalH
}

// drawBrace sets the brace, scaled so that it spans exactly the staves it
// joins.
//
// x is the column it stands in and the brace is drawn to the left of it,
// so the glyph's origin — its left edge — goes a brace's width further
// out. Its baseline is the foot of the shape, so it is set at the bottom
// and grows upwards.
func (e *engraver) drawBrace(x, top, bottom float64) {
	h := bottom - top
	if h <= 0 {
		return
	}
	e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
		Name: "brace",
		X:    x - braceWidth(h),
		Y:    bottom,
		// The em size that makes the glyph exactly h tall. Four spaces
		// to the em is what everything else here is drawn at, and the
		// glyph is a touch over four spaces tall in that em.
		Size: 4 * h / braceNaturalH,
	})
}
