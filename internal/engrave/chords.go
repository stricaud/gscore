/*
 * chords.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import "github.com/stricaud/gscore/internal/model"

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
	// chordRise is how far above the top staff line a chord symbol sits.
	chordRise = 2.6
	// chordAccidentalSize is the type a sharp or a flat in a root is set in.
	chordAccidentalSize = 1.5

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

// drawChordSymbol writes the chord over a note.
func (e *engraver) drawChordSymbol(c *model.ChordSymbol, x float64) {
	if c == nil {
		return
	}
	y := e.y - chordRise

	if c.Text != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: c.Text, X: x, Y: y, Size: chordSize, Anchor: "start", Bold: true,
		})
		if c.Frame != nil {
			e.drawChordFrame(c.Frame, x, y-chordSize-frameRise)
		}
		return
	}

	// The root, then the quality smaller and raised, then the bass note.
	cx := x
	root, acc := splitRoot(c.Root)
	e.lay.Texts = append(e.lay.Texts, Text{
		S: root, X: cx, Y: y, Size: chordSize, Anchor: "start", Bold: true,
	})
	cx += textWidth(root, chordSize)

	if acc != "" {
		// A flat or a sharp in a chord name is the musical sign, set small
		// and raised — not the letter b, which is what a program that uses
		// plain text ends up printing.
		e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
			Name: accidentalGlyphFor(acc), X: cx, Y: y - chordSize*0.28,
			Size: chordAccidentalSize,
		})
		cx += chordAccidentalSize * 0.7
	}

	if c.Quality != "" {
		e.lay.Texts = append(e.lay.Texts, Text{
			S: c.Quality, X: cx, Y: y - chordSize*0.3,
			Size: chordQualitySize, Anchor: "start", Bold: true,
		})
		cx += textWidth(c.Quality, chordQualitySize)
	}

	if c.Bass != "" {
		bass, bacc := splitRoot(c.Bass)
		e.lay.Texts = append(e.lay.Texts, Text{
			S: "/" + bass, X: cx, Y: y, Size: chordSize, Anchor: "start", Bold: true,
		})
		cx += textWidth("/"+bass, chordSize)
		if bacc != "" {
			e.lay.Glyphs = append(e.lay.Glyphs, Glyph{
				Name: accidentalGlyphFor(bacc), X: cx, Y: y - chordSize*0.28,
				Size: chordAccidentalSize,
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
			// Not played: an x above the nut.
			e.lay.Texts = append(e.lay.Texts, Text{
				S: "×", X: sx, Y: top - 0.35, Size: 1.5, Anchor: "middle",
			})
		case fret == 0:
			// Open: a circle above the nut.
			e.lay.Texts = append(e.lay.Texts, Text{
				S: "○", X: sx, Y: top - 0.35, Size: 1.3, Anchor: "middle",
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
			// A dot, drawn as a very short very thick line, which is a
			// filled circle once the ends are rounded.
			e.lay.Lines = append(e.lay.Lines, Line{
				X1: sx, Y1: y, X2: sx, Y2: y, Thickness: frameDot * 2,
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
