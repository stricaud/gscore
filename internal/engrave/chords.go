/*
 * chords.go
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

	"github.com/stricaud/gscore/internal/smufl"

	"github.com/stricaud/gscore/internal/model"
)

// Chord symbols and the fingering diagrams above them.
//
// A chord symbol is not one piece of text. The root is full size, the quality
// is smaller and sits raised, and a bass note after a slash is full size
// again: "Cmaj7" set in one size at one height is how you tell at a glance
// that a program's author never looked at a lead sheet. The sizes here are the
// ones every printed fake book uses.
//
// The diagram above it is six vertical lines and a handful of horizontal ones,
// with a dot where a finger goes, an o over a string played open and an x over
// one not played at all. It is the single most useful thing on a page of music
// for somebody who is still learning the instrument, and it costs about thirty
// lines to draw.

const (
	// chordSize is the type the root of a chord symbol is set in.
	chordSize = 2.3
	// chordQualitySize is the smaller type the quality is set in.
	chordQualitySize = 1.75
	// chordRise is how far above the top staff line a chord symbol sits
	// when nothing is in the way.
	chordRise = 2.6
	// chordGap is the air between the music and the chord symbols over it,
	// where the music reaches up past the staff.
	chordGap = 0.8
	// chordCapHeight is how tall a capital stands above the baseline, as a
	// fraction of the type size. Times Bold's own, which is the face a PDF
	// with nothing embedded in it sets this text in.
	chordCapHeight = 0.662
	// chordAccidentalHeight is how tall the sharp or flat of a root is
	// drawn, against the capital it follows.
	//
	// Larger than the letters around it, not smaller. A staff accidental is
	// a thin glyph with a small bowl, and set at the size of the type it
	// sits between it reads as a speck: the flat of "A♭-6" came out no
	// bigger than the hyphen after it.
	chordAccidentalHeight = 1.05

	// frameWidth and frameHeight are the fingering diagram's size.
	frameWidth  = 4.0
	frameHeight = 4.4
	// frameRise is how far above the chord symbol the diagram sits.
	frameRise = 1.3
	// frameFrets is how many frets a diagram shows.
	frameFrets = 4
	// frameDot is the radius of the dot marking a stopped string.
	frameDot = 0.38
)

// systemChordRoom is how much room the chord symbols of one system need above
// the top staff line.
//
// Everything written over a staff has to be stacked rather than piled in the
// same place: a rehearsal mark, a direction to the player and a chord symbol
// all want to be just above the music, and all three in one spot is what made
// "Moderately" print through the letter A.
func (e *engraver) systemChordRoom(parts []*model.Part, start, end int) float64 {
	room := 0.0
	for _, p := range parts {
		if r := e.partChordRoom(p, start, end); r > room {
			room = r
		}
	}
	return room
}

// partChordRoom is how much room one staff's chord symbols need above it.
//
// Per staff, not per system. A chart has the chords over the piano and
// nothing over the drums, and widening every gap in the score by the height
// of a guitar diagram pushes a pianist's two staves half a page apart.
func (e *engraver) partChordRoom(p *model.Part, start, end int) float64 {
	room := 0.0
	{
		for mi := start; mi < end && mi < len(p.Measures); mi++ {
			for _, v := range p.Measures[mi].Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil {
						continue
					}
					if h := chordHeight(mk.Chord); h > room {
						room = h
					}
				}
			}
		}
	}
	return room
}

// partVerses is how many lines of words are written under one staff, counted
// once per part: counting walks every element of it, and the answer is wanted
// again for every system the part appears on.
func (e *engraver) partVerses(p *model.Part) int {
	if n, ok := e.versesOf[p]; ok {
		return n
	}
	n := partVerses(p)
	e.versesOf[p] = n
	return n
}

// partVerses is how many lines of words are written under one staff.
func partVerses(p *model.Part) int {
	most := 0
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				mk := marksOf(el)
				if mk == nil {
					continue
				}
				for _, ly := range mk.Lyrics {
					if ly.Verse > most {
						most = ly.Verse
					}
				}
			}
		}
	}
	return most
}

// roomBelow is the space one staff needs under it for its own words.
func (e *engraver) roomBelow(p *model.Part) float64 {
	if n := e.partVerses(p); n > 0 {
		return lyricDrop + float64(n-1)*lyricLine
	}
	return 0
}

// chordHeight is how much room a chord symbol and its diagram need above the
// staff, so the system above can be moved out of the way.
func chordHeight(c *model.ChordSymbol) float64 {
	if c == nil {
		return 0
	}
	h := chordRise + chordSize
	if c.Frame != nil {
		h += frameRise + frameHeight + 1.2 // the row of o and x marks on top
	}
	return h
}

// pendingChord is a chord symbol waiting for the staff under it to be
// finished, so that it can be written clear of it.
type pendingChord struct {
	c *model.ChordSymbol
	x float64
	// Which element it belongs to, so that what is drawn can be pointed at
	// afterwards.
	part, measure, ref int
}

// waitChord holds a chord symbol back until the staff is drawn.
//
// Where it goes is not a question about the note it names. A chord symbol
// sits above the music, and how high "above the music" is depends on the
// music: a staff whose notes stay between the lines has room at two and a
// half spaces, and one with a run of notes three ledger lines up has not —
// which is how "Am7" came to be printed across its own notehead.
func (e *engraver) waitChord(c *model.ChordSymbol, x float64, measure, ref int) {
	if c == nil {
		return
	}
	e.chordRow = append(e.chordRow, pendingChord{
		c: c, x: x, part: e.partIndex, measure: measure, ref: ref,
	})
}

// placeChords writes the staff's chord symbols, all on one line, above
// everything already drawn on it.
//
// One line, and not each symbol placed over its own bar: a row of chords
// stepping up and down the page as the tune does is unreadable, and every
// published chart sets them on a rule. So the whole staff is measured and
// they are written at the one height that clears all of it.
//
// They become part of the outline afterwards, so that the bar numbers and
// everything else placed above the staff clears them in turn.
func (e *engraver) placeChords(sky *skyline, x0, x1 float64) {
	row := e.chordRow
	e.chordRow = nil
	if len(row) == 0 {
		return
	}
	// The top of everything on this staff, and the chord row above that.
	y := sky.reach(x0, x1, false) - chordGap
	// Never lower than the old fixed height: a staff with nothing above it
	// still has its chords where they have always been.
	if plain := e.y - chordRise; y > plain {
		y = plain
	}
	for _, p := range row {
		e.drawChordSymbol(p.c, p.x, y)
		w := chordWidth(p.c)
		top := y - chordSize
		if p.c.Frame != nil {
			top -= frameRise + frameHeight
		}
		bottom := y + chordSize*0.25
		sky.add(p.x, p.x+w, top, bottom)
		// Where it landed, so the editor can point at it. The same box the
		// outline uses, since that is the room the symbol occupies.
		e.lay.Chords = append(e.lay.Chords, ChordPos{
			Part: p.part, Measure: p.measure, Ref: p.ref,
			X: p.x, Y: top, Width: w, Height: bottom - top,
			Name: p.c.String(), Frets: p.c.Frame.Spell(),
		})
	}
}

// chordWidth is how wide a written chord symbol is, for the outline.
func chordWidth(c *model.ChordSymbol) float64 {
	if c == nil {
		return 0
	}
	_, w := chordPieces(c)
	if c.Frame != nil && frameWidth > 0 {
		return math.Max(frameWidth, w)
	}
	return w
}

/*
 * Setting a chord symbol
 *
 * One place works out where every piece of it goes, because the drawing and
 * the measuring have to agree: a symbol measured narrower than it is drawn
 * is a symbol the next one is allowed to sit on top of.
 *
 * The widths are real ones. textWidth averages over ordinary words — mostly
 * lowercase, where a letter is about half the type size — and a chord symbol
 * is nothing like ordinary words: it is a bold capital, which is half as
 * wide again, followed by digits and little else. Advanced by the average,
 * the quality was set across the root it belongs to: "G7" came out with the
 * seven sitting on the G's shoulder, which is what was reported.
 */

// chordCharWidth is how wide each character of a chord symbol is, as a
// fraction of the type size.
//
// Times Bold's own widths, which is the face a PDF with nothing embedded in
// it uses and close enough to what a browser puts up for the same text.
var chordCharWidth = map[rune]float64{
	'A': 0.722, 'B': 0.667, 'C': 0.722, 'D': 0.722, 'E': 0.667,
	'F': 0.611, 'G': 0.778, 'N': 0.722, 'C' + 32: 0.444,
	'b': 0.556, '#': 0.500, '/': 0.278, '+': 0.570, '-': 0.333,
	'0': 0.500, '1': 0.500, '2': 0.500, '3': 0.500, '4': 0.500,
	'5': 0.500, '6': 0.500, '7': 0.500, '8': 0.500, '9': 0.500,
	'm': 0.889, 'a': 0.500, 'j': 0.333, 's': 0.389, 'u': 0.556,
	'o': 0.500, 'd': 0.556, 'i': 0.278, 'n': 0.556, '(': 0.333,
	')': 0.333, '°': 0.400, 'ø': 0.500, 'Δ': 0.612, ' ': 0.250,
}

// chordTextWidth is how far a stretch of a chord symbol moves the pen.
func chordTextWidth(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		if v, ok := chordCharWidth[r]; ok {
			w += v
		} else {
			w += 0.5
		}
	}
	return w * size
}

/*
 * Setting the sharps and flats of a chord name
 *
 * An accidental is not type and does not behave like it. It is a staff glyph
 * drawn about the note it alters, so where its ink falls is a fact about the
 * shape: a flat hangs its bowl below that point and throws its ascender well
 * above, while a sharp is symmetric about it. Given one size and one height,
 * they land in different places and stand to different heights — and a row
 * of chord symbols set that way has a ragged top edge, which is most of what
 * makes it hard to read at a glance.
 *
 * So each is sized by how tall its own ink is and placed by where its own
 * ink ends, and they all finish level with the capital they follow. Nothing
 * in a chord symbol rises above its root.
 */

// accidentalInk is how far each accidental's ink reaches above and below the
// point it is drawn about, in staff spaces at its natural size — a SMuFL em
// of four spaces. Bravura's own bounding boxes.
var accidentalInk = map[string][2]float64{
	"accidentalFlat":        {1.868, -0.672},
	"accidentalDoubleFlat":  {1.868, -0.672},
	"accidentalSharp":       {1.400, -1.400},
	"accidentalNatural":     {1.400, -1.400},
	"accidentalDoubleSharp": {0.400, -0.400},
}

// accidentalSize is the size one is drawn at, chosen so that every kind of
// accidental stands the same height: a sharp's ink is taller than a flat's,
// so set at one size the sharp of "F♯7" would tower over the flat of "B♭7"
// in the bar beside it.
func accidentalSize(glyph string) float64 {
	ink, ok := accidentalInk[glyph]
	if !ok {
		return chordAccidentalHeight * chordCapHeight * chordSize
	}
	height := ink[0] - ink[1] // the ink, in spaces at natural size
	return chordAccidentalHeight * chordCapHeight * chordSize / height * 4
}

// accidentalRise is how far above the baseline it is drawn about, so that
// its ink finishes level with the top of the capital before it.
func accidentalRise(glyph string) float64 {
	top := chordCapHeight * chordSize
	ink, ok := accidentalInk[glyph]
	if !ok {
		return chordSize * 0.28
	}
	return top - ink[0]*accidentalSize(glyph)/4
}

// accidentalAdvance is how far it moves the pen along: the font's own width
// for it, at the size it is set in.
//
// A glyph drawn at size 4 is drawn at its natural size, so its advance
// scales from there — the same arithmetic a clef change uses. Guessed at
// instead, as seven tenths of the type size, the sign stood too far from the
// letter it belongs to and the quality after it further still.
func accidentalAdvance(glyph string) float64 {
	return smufl.Advance(glyph) * accidentalSize(glyph) / 4
}

// chordPieceGap is the air between one piece of a symbol and the next, as a
// fraction of the root's type size.
//
// Every join needs it, the one between a letter and its accidental most of
// all: a SMuFL glyph is drawn from its own left edge, so a flat set at the
// end of the letter's width begins exactly where the letter stops and the
// two touch. "B♭7" printed with the flat against the B's bowl.
const chordPieceGap = 0.12

// chordPiece is one part of a symbol: a stretch of text, or the glyph of an
// accidental, placed from the symbol's left edge.
type chordPiece struct {
	text  string
	glyph string
	x     float64
	dy    float64 // above the baseline
	size  float64
}

// chordPieces lays a symbol out left to right, and says how wide it came to.
func chordPieces(c *model.ChordSymbol) ([]chordPiece, float64) {
	if c == nil {
		return nil, 0
	}
	if c.Text != "" {
		w := chordTextWidth(c.Text, chordSize)
		return []chordPiece{{text: c.Text, size: chordSize}}, w
	}

	var out []chordPiece
	x := 0.0
	root, acc := splitRoot(c.Root)
	if root != "" {
		out = append(out, chordPiece{text: root, x: x, size: chordSize})
		x += chordTextWidth(root, chordSize)
	}
	if acc != "" {
		// A flat or a sharp in a chord name is the musical sign — not the
		// letter b, which is what a program that uses plain text ends up
		// printing.
		if len(out) > 0 {
			x += chordPieceGap * chordSize
		}
		glyph := accidentalGlyphFor(acc)
		out = append(out, chordPiece{
			glyph: glyph, x: x,
			dy: accidentalRise(glyph), size: accidentalSize(glyph),
		})
		x += accidentalAdvance(glyph)
	}
	if c.Quality != "" {
		if len(out) > 0 {
			x += chordPieceGap * chordSize
		}
		out = append(out, chordPiece{
			// Raised so that its top is the root's top: set higher, the
			// smaller type floated above the letter it belongs to and the
			// eye had three different heights to follow along a line of
			// chords.
			text: c.Quality, x: x,
			dy:   chordCapHeight * (chordSize - chordQualitySize),
			size: chordQualitySize,
		})
		x += chordTextWidth(c.Quality, chordQualitySize)
	}
	if c.Bass != "" {
		bass, bacc := splitRoot(c.Bass)
		x += chordPieceGap * chordSize
		out = append(out, chordPiece{text: "/" + bass, x: x, size: chordSize})
		x += chordTextWidth("/"+bass, chordSize)
		if bacc != "" {
			x += chordPieceGap * chordSize
			glyph := accidentalGlyphFor(bacc)
			out = append(out, chordPiece{
				glyph: glyph, x: x,
				dy: accidentalRise(glyph), size: accidentalSize(glyph),
			})
			x += accidentalAdvance(glyph)
		}
	}
	return out, x
}

// drawChordSymbol writes the chord over a note, at the height it was given.
func (e *engraver) drawChordSymbol(c *model.ChordSymbol, x, y float64) {
	if c == nil {
		return
	}
	pieces, _ := chordPieces(c)
	for _, p := range pieces {
		switch {
		case p.glyph != "":
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: p.glyph, X: x + p.x, Y: y - p.dy, Size: p.size,
			})
		default:
			e.lay.Texts = append(e.lay.Texts, Text{
				S: p.text, X: x + p.x, Y: y - p.dy, Size: p.size,
				Anchor: "start", Bold: true,
			})
		}
	}
	if c.Frame != nil {
		e.drawChordFrame(c.Frame, x, y-chordSize-frameRise)
	}
}

// splitRoot separates a root's letter from its accidental, so the accidental
// can be set as the sign it is.
func splitRoot(s string) (letter, accidental string) {
	if s == "" {
		return "", ""
	}
	r := []rune(s)
	letter = string(r[0])
	if len(r) > 1 {
		accidental = string(r[1:])
	}
	return letter, accidental
}

func accidentalGlyphFor(s string) string {
	switch s {
	case "b", "♭":
		return "accidentalFlat"
	case "#", "♯":
		return "accidentalSharp"
	case "bb", "♭♭":
		return "accidentalDoubleFlat"
	case "##", "x", "♯♯":
		return "accidentalDoubleSharp"
	}
	return "accidentalNatural"
}

// drawChordFrame draws the fingering diagram, with its bottom at y.
func (e *engraver) drawChordFrame(f *model.ChordFrame, x, bottom float64) {
	strings := len(f.Frets)
	if strings == 0 {
		strings = 6
	}
	t := e.opt.Defaults.StaffLineThickness
	top := bottom - frameHeight
	stepX := frameWidth / float64(strings-1)
	stepY := frameHeight / frameFrets

	// The nut is drawn heavy when the diagram starts at the first fret, which
	// is what says "this is the top of the neck" without a number beside it.
	nut := t
	if f.FirstFret <= 1 {
		nut = t * 4
	}
	e.lay.Lines = append(e.lay.Lines,
		Line{X1: x, Y1: top, X2: x + frameWidth, Y2: top, Thickness: nut})
	for i := 1; i <= frameFrets; i++ {
		y := top + float64(i)*stepY
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: x, Y1: y, X2: x + frameWidth, Y2: y, Thickness: t})
	}
	for i := 0; i < strings; i++ {
		sx := x + float64(i)*stepX
		e.lay.Lines = append(e.lay.Lines,
			Line{X1: sx, Y1: top, X2: sx, Y2: bottom, Thickness: t})
	}

	// A chord up the neck says which fret it starts at, rather than being
	// drawn with eleven empty frets above it.
	if f.FirstFret > 1 {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: itoa(f.FirstFret) + "fr", X: x + frameWidth + 0.35,
			Y: top + stepY*0.8, Size: 1.3, Anchor: "start",
		})
	}

	// A barre: one finger across several strings, drawn as the bar it is.
	if f.Barre > 0 {
		row := f.Barre - maxInt(f.FirstFret, 1)
		if row >= 0 && row < frameFrets {
			y := top + (float64(row)+0.5)*stepY
			from, to := f.BarreFrom, f.BarreTo
			if to <= from {
				from, to = 0, strings-1
			}
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: x + float64(from)*stepX, Y1: y,
				X2: x + float64(to)*stepX, Y2: y,
				Thickness: frameDot * 1.8,
			})
		}
	}

	for i, fret := range f.Frets {
		sx := x + float64(i)*stepX
		switch {
		case fret < 0:
			// Not played: a cross above the nut, drawn rather than typed.
			// The character for it is not in the encodings a PDF's built-in
			// fonts use, so a typed one prints as a question mark over the
			// string — which is what it did.
			const d = 0.3
			cy := top - 0.62
			e.lay.Lines = append(e.lay.Lines,
				Line{X1: sx - d, Y1: cy - d, X2: sx + d, Y2: cy + d, Thickness: t * 1.6},
				Line{X1: sx - d, Y1: cy + d, X2: sx + d, Y2: cy - d, Thickness: t * 1.6})
		case fret == 0:
			// Open: a ring above the nut.
			e.lay.Circles = append(e.lay.Circles, Circle{
				X: sx, Y: top - 0.62, Radius: 0.3, Thickness: t * 1.6,
			})
		default:
			row := fret - maxInt(f.FirstFret, 1)
			if row < 0 || row >= frameFrets {
				continue
			}
			if f.Barre == fret {
				continue // already drawn as part of the barre
			}
			y := top + (float64(row)+0.5)*stepY
			e.lay.Circles = append(e.lay.Circles, Circle{
				X: sx, Y: y, Radius: frameDot, Filled: true,
			})
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
