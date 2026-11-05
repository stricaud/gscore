/*
 * signature.go
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
	"github.com/stricaud/gscore/internal/smufl"
)

// signatures reads the key and the metre out of the head of a staff.
//
// Both live in the same place — after the clef and before the first note —
// and that position is what identifies them. An accidental standing there
// belongs to the key signature; the same glyph a little further right
// belongs to a note. Nothing else distinguishes them, which is why this is
// done by looking at where things are rather than at what they are.
func signatures(c *pdf.Content, st staff, marks []mark) (*model.KeySignature, *model.TimeSignature) {
	firstNote := math.Inf(1)
	for _, m := range marks {
		if m.s.kind == kNotehead {
			firstNote = m.x
			break
		}
	}
	var key *model.KeySignature
	sharps, flats := 0, 0
	for _, m := range marks {
		if m.x >= firstNote || m.s.kind != kAccidental {
			continue
		}
		switch m.s.acc {
		case model.Sharp:
			sharps++
		case model.Flat:
			flats++
		}
	}
	if sharps > 0 || flats > 0 {
		k := model.KeySignature{Fifths: sharps - flats, Mode: model.Major}
		key = &k
	}
	return key, timeSignature(c, st, firstNote)
}

// timeSignature reads the metre, which is drawn one of two ways.
//
// A common or cut-common sign is a glyph with a name, and is simply read. A
// numeric signature is two digits, one above the other, and LilyPond draws
// them as plain ASCII in the music font with no name attached — so the
// glyph's own code is the digit. Grouping them by height separates the
// numerator from the denominator, and grouping each half by position left to
// right spells out a number of more than one figure.
func timeSignature(c *pdf.Content, st staff, firstNote float64) *model.TimeSignature {
	var top, bottom []digit
	for _, g := range c.Glyphs {
		if !st.holds(g) || g.X >= firstNote {
			continue
		}
		if n := glyphName(g); n != "" {
			switch {
			case n == "timesig.C44" || n == "timeSigCommon":
				return &model.TimeSignature{Beats: 4, BeatValue: 4,
					Symbol: model.TimeCommon}
			case n == "timesig.C22" || n == "timeSigCutCommon":
				return &model.TimeSignature{Beats: 2, BeatValue: 2,
					Symbol: model.TimeCut}
			}
		}
		v, ok := digitOf(g)
		if !ok {
			continue
		}
		d := digit{g.X, g.Y, v}
		if st.degree(g.Y) >= 4 {
			top = append(top, d)
		} else {
			bottom = append(bottom, d)
		}
	}
	if len(top) == 0 || len(bottom) == 0 {
		return nil
	}
	return &model.TimeSignature{
		Beats:     number(top),
		BeatValue: number(bottom),
		Symbol:    model.TimeNumeric,
	}
}

// digit is one figure of a numeric time signature.
type digit struct {
	x, y float64
	v    int
}

// number spells out digits read left to right.
func number(ds []digit) int {
	sort.Slice(ds, func(i, j int) bool { return ds[i].x < ds[j].x })
	n := 0
	for _, d := range ds {
		n = n*10 + d.v
	}
	return n
}

// glyphName is the name the file gives a glyph, by either convention.
func glyphName(g pdf.Glyph) string {
	if g.Name != "" {
		return g.Name
	}
	if g.Rune != 0 {
		if n, ok := smufl.ByRune(g.Rune); ok {
			return n
		}
	}
	return ""
}

// digitOf reads a glyph that stands for a figure.
//
// LilyPond writes the digits of a time signature as the ASCII codes for them
// in its music font, giving them no name; SMuFL has a glyph per digit, named.
func digitOf(g pdf.Glyph) (int, bool) {
	if n := glyphName(g); len(n) == 8 && n[:7] == "timeSig" {
		// timeSig0 … timeSig9
		if c := n[7]; c >= '0' && c <= '9' {
			return int(c - '0'), true
		}
	}
	if g.Name == "" && g.Code >= '0' && g.Code <= '9' {
		return int(g.Code - '0'), true
	}
	return 0, false
}
