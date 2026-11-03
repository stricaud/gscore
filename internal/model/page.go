/*
 * page.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

// The paper a score is printed on.
//
// This is the one thing about a score that is not about the music and
// cannot be left to the program. A4 and Letter are different shapes, and a
// part laid out for one and printed on the other comes out either short of
// the bottom margin or over it; a conductor's score of fourteen staves
// wants the paper turned on its side; a pocket score wants smaller type
// and more bars to the line. None of that can be worked out from the
// notes, and all of it changes what the engraver has to fit into.
//
// It belongs to the score and not to the export, because it is a decision
// somebody made about the document. Asked again at every print, it would
// be re-made differently every time and the file would not remember what
// the piece looks like.
//
// Millimetres, because that is what paper is sold and specified in and
// what every page-setup dialog in the world shows. The renderer works in
// points and converts once, at the edge. The type size is the exception
// and stays in points: it is an engraving measurement rather than a
// measurement of the page, and the rest of the program already quotes it
// that way.

// Page is a score's paper and margins.
//
// A zero Page means "whatever the program would do", which is what a score
// written before any of this existed has and what a new one starts as. So
// every field is an override and none of them has to be filled in.
type Page struct {
	// Paper is the size by name: "a4", "letter", and the rest of Papers.
	// Empty is the default, which is A4.
	//
	// By name rather than by two numbers, so a file says what somebody
	// chose. A4 is 210 by 297 millimetres and will not stop being that,
	// but "a4" survives a rounding change in a way that 595.28 does not,
	// and it is what the dialog has to show anyway.
	Paper string `json:",omitempty"`

	// Custom is the size in millimetres, used when Paper is "custom".
	CustomWidth, CustomHeight float64 `json:",omitempty"`

	// Landscape turns the paper on its side. A full score is wider than
	// it is tall — fourteen staves to a system — which is the case this
	// is for.
	Landscape bool `json:",omitempty"`

	// Margins in millimetres. Zero means the default for that edge.
	//
	// Four of them and not one, because they are not the same: the inner
	// margin of a bound part has to clear the staples, and the bottom of
	// a page carries the page number.
	MarginLeft, MarginRight float64 `json:",omitempty"`
	MarginTop, MarginBottom float64 `json:",omitempty"`

	// StaffSpace is how big one staff space is printed, in POINTS, or
	// zero to let the program choose and shrink to fit.
	//
	// Points rather than millimetres, unlike everything above it: this is
	// the type size, which is an engraving measurement, and the engraver,
	// the exporter and the knobs all already quote it in points. Seven
	// points is about what a printed part uses and five a pocket score.
	StaffSpace float64 `json:",omitempty"`
}

// Paper is one named size, in millimetres.
type Paper struct {
	Name          string
	Label         string
	Width, Height float64
}

// Papers are the sizes offered, portrait, in millimetres.
//
// The ISO A series and the North American sizes, which between them are
// what music is printed on. B4 is here because it is the traditional size
// for orchestral parts in Japan and turns up in scores from there.
var Papers = []Paper{
	{"a4", "A4 — 210 × 297 mm", 210, 297},
	{"a3", "A3 — 297 × 420 mm", 297, 420},
	{"a5", "A5 — 148 × 210 mm", 148, 210},
	{"b4", "B4 — 250 × 353 mm", 250, 353},
	{"letter", "Letter — 8.5 × 11 in", 215.9, 279.4},
	{"legal", "Legal — 8.5 × 14 in", 215.9, 355.6},
	{"tabloid", "Tabloid — 11 × 17 in", 279.4, 431.8},
}

// DefaultPaper is what a score with nothing said is printed on.
const DefaultPaper = "a4"

// Default margins in millimetres, which are what the renderer's points
// worked out to and are kept so that nothing moves on an old score.
const (
	DefaultMarginLeft   = 16.9
	DefaultMarginRight  = 14.1
	DefaultMarginTop    = 16.2
	DefaultMarginBottom = 16.2
)

// PaperNamed finds a size, and says whether there is one.
func PaperNamed(name string) (Paper, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, p := range Papers {
		if p.Name == want {
			return p, true
		}
	}
	return Paper{}, false
}

// Size is the paper this page is on, in millimetres, the way round it is
// to be printed.
//
// Everything unset falls back, so a zero Page answers with upright A4 and
// a Page that names only its paper answers with that paper upright.
func (p Page) Size() (width, height float64) {
	if strings.EqualFold(p.Paper, "custom") &&
		p.CustomWidth > 0 && p.CustomHeight > 0 {
		width, height = p.CustomWidth, p.CustomHeight
	} else {
		paper, ok := PaperNamed(p.Paper)
		if !ok {
			paper, _ = PaperNamed(DefaultPaper)
		}
		width, height = paper.Width, paper.Height
	}
	if p.Landscape && width < height {
		width, height = height, width
	}
	return width, height
}

// Margins are the four margins in millimetres, each falling back on its
// own default rather than on the set of them: somebody who widened the
// left margin and said nothing about the others meant the others to stay
// as they were.
func (p Page) Margins() (left, right, top, bottom float64) {
	pick := func(v, fallback float64) float64 {
		if v > 0 {
			return v
		}
		return fallback
	}
	return pick(p.MarginLeft, DefaultMarginLeft),
		pick(p.MarginRight, DefaultMarginRight),
		pick(p.MarginTop, DefaultMarginTop),
		pick(p.MarginBottom, DefaultMarginBottom)
}

// MMToPoints converts, there being 72 points to the inch and 25.4
// millimetres to it.
func MMToPoints(mm float64) float64 { return mm * 72 / 25.4 }

// PointsToMM is the way back.
func PointsToMM(pt float64) float64 { return pt * 25.4 / 72 }

// IsZero says nothing was chosen, so the program's own defaults stand.
func (p Page) IsZero() bool { return p == Page{} }
