/*
 * survey.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engraved

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

// A Survey is what the reader saw, in the page's own coordinates, so that it
// can be drawn back over the page it came from.
//
// It exists because the reader's mistakes are invisible in its output. A
// score with seven parts where there is one, or a part transposed because a
// clef was missed, is a perfectly well-formed score: nothing in it says that
// it is wrong. The only way to know is to put what was read back where it was
// read from and look.
//
// What comes back is not a picture of the page — nothing here rasterises a
// PDF. It is the page as the reader perceived it: the lines it took for
// staves, the way it grouped them into systems, and every glyph it recognised
// with the pitch it assigned. Where that drawing and the real page disagree,
// the disagreement is the bug, and it is visible at a glance.
type Survey struct {
	Pages []SurveyPage `json:"pages"`
	// Parts is how many parts the same reading would produce, so the count
	// shown beside the drawing is the count that would be imported.
	Parts int `json:"parts"`
	// PerSystem is the staves-per-system actually used: what was asked for,
	// or what was worked out when nothing was asked.
	PerSystem int `json:"perSystem"`
	// Detected is what the page-reading heuristic concluded, whether or not
	// it was used. Showing both is what lets somebody see that the guess was
	// wrong.
	Detected int `json:"detected"`
}

// SurveyPage is one page's worth.
type SurveyPage struct {
	Width  float64       `json:"width"`
	Height float64       `json:"height"`
	Staves []SurveyStaff `json:"staves"`
	Marks  []SurveyMark  `json:"marks"`
	// Rules are the strokes and filled edges the page is drawn with — staff
	// lines, barlines, stems, beams, ledger lines. Drawn faintly, they are
	// what makes the overlay recognisable as the page.
	Rules []SurveyRule `json:"rules"`
}

// SurveyStaff is one five-line staff and where it was put.
type SurveyStaff struct {
	// Y holds the five lines, bottom first, as pitch is counted from the
	// bottom.
	Y      [5]float64 `json:"y"`
	X0     float64    `json:"x0"`
	X1     float64    `json:"x1"`
	Inter  float64    `json:"inter"`
	System int        `json:"system"`
	// Part is which part this staff's music went into — the number that is
	// wrong when the grouping is wrong.
	Part int `json:"part"`
}

// SurveyMark is one glyph, with what the reader made of it.
type SurveyMark struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	// Kind is "note", "rest", "clef", "accidental", "flag" or "dot".
	Kind string `json:"kind"`
	// Pitch is filled for a notehead only: "g4", "c5". It is the whole point
	// of the drawing — a notehead sitting on the third line labelled c5 says
	// the clef was misread, and says it in one glance.
	Pitch string `json:"pitch,omitempty"`
	// Staff is the index into the page's Staves.
	Staff int `json:"staff"`
}

// SurveyRule is one straight stroke of the page.
type SurveyRule struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
	W  float64 `json:"w"`
}

// maxSurveyPages caps the drawing. A survey is looked at, not processed, and
// nobody checks a reading by scrolling through four hundred pages; the first
// few say whether the grouping is right, which is what the drawing is for.
const maxSurveyPages = 4

// Look reads a PDF and reports what it saw, without building a score.
func Look(r io.Reader, opt Options) (*Survey, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	doc, err := pdf.Parse(data)
	if err != nil {
		return nil, err
	}
	pages := doc.Pages()
	if len(pages) == 0 {
		return nil, fmt.Errorf("engraved: no pages")
	}

	sv := &Survey{PerSystem: opt.StavesPerSystem}
	parts := 0
	for i := range pages {
		c, err := doc.Page(i)
		if err != nil {
			continue
		}
		sts := staves(c)
		if i == 0 {
			sv.Detected = commonest(systems(c, sts, 0))
			if sv.PerSystem == 0 {
				sv.PerSystem = sv.Detected
			}
		}
		groups := systems(c, sts, opt.StavesPerSystem)
		for _, g := range groups {
			if len(g) > parts {
				parts = len(g)
			}
		}
		if i >= maxSurveyPages {
			continue
		}
		w, h := doc.Size(i)
		sp := SurveyPage{Width: w, Height: h}
		for gi, g := range groups {
			for k, st := range g {
				sp.Staves = append(sp.Staves, SurveyStaff{
					Y: st.y, X0: st.x0, X1: st.x1, Inter: st.inter,
					System: gi, Part: k,
				})
			}
		}
		sp.Marks = marksOf(c, sp.Staves)
		sp.Rules = rulesOf(c)
		sv.Pages = append(sv.Pages, sp)
	}
	sv.Parts = parts
	return sv, nil
}

// commonest is how many staves most systems hold.
//
// Not the largest: one short system at the end of a piece, or a solo line in
// the middle of a duet, would then speak for the whole page. The usual case
// is what the page is.
func commonest(groups [][]staff) int {
	if len(groups) == 0 {
		return 0
	}
	n := map[int]int{}
	for _, g := range groups {
		n[len(g)]++
	}
	best, count := 0, -1
	for k, c := range n {
		if c > count || (c == count && k < best) {
			best, count = k, c
		}
	}
	return best
}

// marksOf reads every glyph of a page against the staff it belongs to.
func marksOf(c *pdf.Content, sts []SurveyStaff) []SurveyMark {
	var out []SurveyMark
	for _, g := range c.Glyphs {
		s := readGlyph(g)
		if s.kind == kOther {
			continue
		}
		k := nearestStaff(sts, g)
		if k < 0 {
			continue
		}
		m := SurveyMark{X: g.X, Y: g.Y, Kind: kindName(s.kind), Staff: k}
		if s.kind == kNotehead {
			st := sts[k]
			// The clef in force on that staff, which is what makes a
			// notehead's height into a pitch. Read it the same way the
			// importer does: whatever was drawn before this note.
			cl := clefAt(c, st, g.X)
			base := bottomPitch(cl)
			deg := int(math.Round(2 * (g.Y - st.Y[0]) / st.Inter))
			p := model.PitchFromDiatonicIndex(base.DiatonicIndex() + deg)
			m.Pitch = fmt.Sprintf("%s%d", lower(p.Step.String()), p.Octave)
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Staff != out[j].Staff {
			return out[i].Staff < out[j].Staff
		}
		return out[i].X < out[j].X
	})
	return out
}

// clefAt is the clef in force at an x on a staff.
func clefAt(c *pdf.Content, st SurveyStaff, x float64) model.Clef {
	cur := model.Treble
	best := math.Inf(-1)
	for _, g := range c.Glyphs {
		if g.X > x || g.X < best {
			continue
		}
		if g.Y < st.Y[0]-4*st.Inter || g.Y > st.Y[4]+4*st.Inter {
			continue
		}
		if g.X < st.X0-st.Inter || g.X > st.X1+st.Inter {
			continue
		}
		if s := readGlyph(g); s.kind == kClef {
			cur, best = s.clef, g.X
		}
	}
	return cur
}

// nearestStaff is the staff a glyph belongs to, or -1 when it belongs to
// none — a title, a page number, a footer.
func nearestStaff(sts []SurveyStaff, g pdf.Glyph) int {
	best, dist := -1, math.Inf(1)
	for i, st := range sts {
		if g.X < st.X0-st.Inter || g.X > st.X1+st.Inter {
			continue
		}
		span := 4 * st.Inter
		if g.Y <= st.Y[0]-span || g.Y >= st.Y[4]+span {
			continue
		}
		mid := (st.Y[0] + st.Y[4]) / 2
		if d := math.Abs(g.Y - mid); d < dist {
			best, dist = i, d
		}
	}
	return best
}

// rulesOf keeps the page's straight strokes, deduplicated.
func rulesOf(c *pdf.Content) []SurveyRule {
	seen := map[[4]float64]bool{}
	var out []SurveyRule
	for _, s := range c.Segments {
		key := [4]float64{round4(s.X1), round4(s.Y1), round4(s.X2), round4(s.Y2)}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, SurveyRule{s.X1, s.Y1, s.X2, s.Y2, s.Width})
		if len(out) >= 20000 {
			break // a drawing, not an archive
		}
	}
	return out
}

func round4(v float64) float64 { return math.Round(v*4) / 4 }

func kindName(k kind) string {
	switch k {
	case kNotehead:
		return "note"
	case kRest:
		return "rest"
	case kClef:
		return "clef"
	case kAccidental:
		return "accidental"
	case kFlag:
		return "flag"
	case kDot:
		return "dot"
	}
	return "other"
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
