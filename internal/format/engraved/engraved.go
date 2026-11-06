/*
 * engraved.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package engraved reads a score out of a PDF that was engraved rather than
// scanned.
//
// The file already contains the music. internal/pdf hands over every glyph
// with the name its font gives it and a position good to six decimal places,
// and every stroked line; this package works out what they mean. A notehead's
// pitch is how far it sits above the bottom staff line, which is subtraction,
// not recognition — so on a file from LilyPond or MuseScore the notes that
// come out are the notes that went in, not a good guess at them.
//
// What is read: clefs, key and time signatures, noteheads with their
// accidentals and augmentation dots, how long each note lasts, rests, and the
// barlines that divide them into measures.
//
// What is not, and must be added by hand: ties and slurs, which are curves
// and are not recorded at all; dynamics, articulations, text and tuplets. The
// importer says so in the score's title rather than leaving it to be
// discovered.
//
// Measured, on a 64-bar duet this project engraved and verified by hand:
// two parts, all 128 pitches equal to the source. The measure division is
// not yet perfect — 126 of the 128 measures, because on one staff two
// noteheads are drawn a hair to the left of the barline that ought to follow
// them, and land in the measure before. The pitches are unaffected and the
// cause is not yet established; it is written down here rather than guessed
// at, because a reader that quietly shifted notes between measures to make
// the count come out would be worse than one that does not.
//
// This is a different thing from internal/omr, which reads a photograph of a
// page and must guess. Where both could apply, this one is right and that one
// is approximate; the decision of which to use belongs to whatever opens the
// file, and turns on whether the page has glyphs in it or only pixels.
package engraved

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

func init() { format.RegisterImporter(importer{}) }

type importer struct{}

func (importer) Name() string                             { return "Engraved PDF" }
func (importer) Extensions() []string                     { return []string{".pdf"} }
func (importer) Import(r io.Reader) (*model.Score, error) { return Import(r) }

// Options are what the person importing knows and the page does not say.
type Options struct {
	// StavesPerSystem is how many staves one system holds — which is to say
	// how many instruments play at once. Zero means work it out from the
	// page.
	//
	// Working it out is a guess, and it is the guess this reader is worst
	// at. A system's staves are tied together by lines that cross the gap
	// between them, so that is what joined() looks for; but an engraver who
	// draws the barlines of a single-staff song right through to the staff
	// below produces the same evidence as a duet, and a duet whose system
	// happens to begin with an empty bar produces none. Get it wrong one way
	// and a duet becomes one part played twice; the other, and a song becomes
	// seven parts.
	//
	// Nobody looking at the page is in any doubt. So it can be said, and when
	// it is said the grouping stops being a guess: the staves are taken in
	// blocks of this many, in reading order, and joined() is not consulted.
	StavesPerSystem int
}

// Import reads every page of an engraved PDF into one score.
func Import(r io.Reader) (*model.Score, error) {
	return ImportWith(r, Options{})
}

// ImportWith reads an engraved PDF, told what the page cannot say.
func ImportWith(r io.Reader, opt Options) (*model.Score, error) {
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

	s := model.NewScore()
	s.Parts = nil
	for i := range pages {
		c, err := doc.Page(i)
		if err != nil {
			continue
		}
		for _, sys := range systems(c, staves(c), opt.StavesPerSystem) {
			for k, st := range sys {
				ms, key, time := readStaff(c, st)
				if len(ms) == 0 {
					continue
				}
				for len(s.Parts) <= k {
					s.Parts = append(s.Parts, &model.Part{
						Clef: model.Treble,
						Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
						Time: model.TimeSignature{Beats: 4, BeatValue: 4,
							Symbol: model.TimeNumeric},
					})
				}
				part := s.Parts[k]
				// The first staff of a part sets its signatures; a later
				// one only records them if they have changed, which is what
				// a signature written again mid-piece means.
				if len(part.Measures) == 0 {
					if key != nil {
						part.Key = *key
					}
					if time != nil {
						part.Time = *time
					}
				} else {
					if key != nil && *key != part.Key {
						ms[0].Key = key
					}
					if time != nil && *time != part.Time {
						ms[0].Time = time
					}
				}
				part.Measures = append(part.Measures, ms...)
			}
		}
	}
	if len(s.Parts) == 0 {
		return nil, fmt.Errorf("engraved: no staves found — a scan rather " +
			"than an engraving?")
	}
	s.Identity.Title = "Imported from PDF — slurs, ties and dynamics not read"
	return s, nil
}

// staff is five lines and the glyphs that belong to them.
type staff struct {
	y      [5]float64 // bottom line first, as pitch is counted from it
	x0, x1 float64
	inter  float64
}

// staves groups the page's long horizontal lines into fives.
//
// Long, because a beam and a ledger line are horizontal too and neither runs
// the width of a system. The threshold is relative to the longest line on the
// page rather than absolute, so it holds for a pocket score and a folio
// alike.
func staves(c *pdf.Content) []staff {
	var longest float64
	for _, s := range c.Segments {
		if s.Horizontal() {
			longest = math.Max(longest, math.Abs(s.X2-s.X1))
		}
	}
	if longest == 0 {
		return nil
	}
	type line struct{ y, x0, x1 float64 }
	var lines []line
	for _, s := range c.Segments {
		if !s.Horizontal() || math.Abs(s.X2-s.X1) < 0.6*longest {
			continue
		}
		x0, x1 := math.Min(s.X1, s.X2), math.Max(s.X1, s.X2)
		lines = append(lines, line{(s.Y1 + s.Y2) / 2, x0, x1})
	}
	// A staff line is not always one stroke. LilyPond strokes it; MuseScore
	// fills a thin rectangle, whose top and bottom edges both arrive here, so
	// five lines turn up as ten and nothing looks evenly spaced any more.
	//
	// The two edges of one line are far closer together than two lines are,
	// so the test is relative. An absolute tolerance cannot work: it would
	// have to be chosen for a particular staff size.
	//
	// The measure has to be the *wider* of the two spacings, not the usual
	// one. Where every line arrives twice, more than half of all the gaps
	// are the thickness of a line rather than the distance between lines,
	// so the median gap is the thickness — and a tolerance derived from it
	// is smaller than the thing it is meant to span, and merges nothing.
	// That is how gscore came to be unable to read its own exported PDFs,
	// whose staff lines are filled rectangles: ten edges where five lines
	// were expected, no five of them evenly spaced, no staff found, and a
	// page of perfectly good music reported as "a scan rather than an
	// engraving".
	//
	// The median of the larger half of the gaps is the distance between
	// lines whether they arrive once or twice, and a third of that sits
	// comfortably above any line thickness and below any interline.
	sort.Slice(lines, func(i, j int) bool { return lines[i].y < lines[j].y })
	var gaps []float64
	for i := 1; i < len(lines); i++ {
		if g := lines[i].y - lines[i-1].y; g > 0.01 {
			gaps = append(gaps, g)
		}
	}
	tol := 0.3
	if len(gaps) > 0 {
		sort.Float64s(gaps)
		wide := gaps[len(gaps)/2:]
		tol = math.Max(0.3, wide[len(wide)/2]/3)
	}
	var uniq []line
	for _, l := range lines {
		if n := len(uniq); n > 0 && math.Abs(l.y-uniq[n-1].y) < tol {
			// keep the midpoint, which is where the line really is
			uniq[n-1].y = (uniq[n-1].y + l.y) / 2
			uniq[n-1].x0 = math.Min(uniq[n-1].x0, l.x0)
			uniq[n-1].x1 = math.Max(uniq[n-1].x1, l.x1)
			continue
		}
		uniq = append(uniq, l)
	}

	var out []staff
	for i := 0; i+4 < len(uniq); {
		g := uniq[i : i+5]
		d := g[1].y - g[0].y
		ok := d > 0.5
		for k := 2; k < 5 && ok; k++ {
			if math.Abs((g[k].y-g[k-1].y)-d) > 0.2*d {
				ok = false
			}
		}
		if !ok {
			i++
			continue
		}
		var st staff
		// y grows upward in a PDF, so the first of the five is the lowest,
		// which is the line pitch is counted from
		for k := 0; k < 5; k++ {
			st.y[k] = g[k].y
		}
		st.inter = d
		st.x0, st.x1 = g[0].x0, g[0].x1
		for _, l := range g {
			st.x0 = math.Min(st.x0, l.x0)
			st.x1 = math.Max(st.x1, l.x1)
		}
		out = append(out, st)
		i += 5
	}
	// reading order is down the page, which is the opposite of PDF order
	sort.Slice(out, func(i, j int) bool { return out[i].y[0] > out[j].y[0] })
	return out
}

// degree is how many diatonic steps a glyph sits above the bottom line.
func (st staff) degree(y float64) int {
	return int(math.Round(2 * (y - st.y[0]) / st.inter))
}

// holds reports whether the glyph belongs to this staff.
//
// Half a staff's height above and below catches ledger lines without
// reaching the next staff; anything further away is somebody else's.
func (st staff) holds(g pdf.Glyph) bool {
	if g.X < st.x0-st.inter || g.X > st.x1+st.inter {
		return false
	}
	span := 4 * st.inter
	return g.Y > st.y[0]-span && g.Y < st.y[4]+span
}

// systems groups a page's staves into systems, each read left to right and
// top to bottom.
//
// The grouping is what tells a duet from a solo: staff two of a two-staff
// system is a second player and belongs in its own part, while staff two of
// a page of single-staff systems is the same player, further down the page.
// Get it wrong in one direction and a duet is one part twice as long; in the
// other, a song becomes seven parts.
//
// The test is whether a barline joins them. In an engraved score the staves
// of a system are tied together by lines that run from one down to the next
// — the barlines at least, usually a brace as well — and a staff that stands
// alone has nothing crossing the space above it. That is an exact fact in
// the file, not an estimate.
//
// The gap between staves is the tempting alternative and it does not work:
// it needs a within-system gap to compare against, and on a page where every
// system holds one staff there is no such gap to find. The spacing between
// systems varies with what hangs off the music — lyrics under one line,
// nothing under the next — so the variation gets read as structure.
// perSystem, when not zero, settles the question instead: the staves are cut
// into blocks of that many, in reading order. A page whose staff count is not
// a multiple of it is not refused — the last short block is kept, since a
// page that ends mid-system is likelier than a person who cannot count their
// own instruments.
func systems(c *pdf.Content, sts []staff, perSystem int) [][]staff {
	if len(sts) == 0 {
		return nil
	}
	if perSystem > 0 {
		var out [][]staff
		for i := 0; i < len(sts); i += perSystem {
			j := i + perSystem
			if j > len(sts) {
				j = len(sts)
			}
			out = append(out, sts[i:j])
		}
		return out
	}
	var out [][]staff
	cur := []staff{sts[0]}
	for i := 1; i < len(sts); i++ {
		if joined(c, sts[i-1], sts[i]) {
			cur = append(cur, sts[i])
			continue
		}
		out = append(out, cur)
		cur = []staff{sts[i]}
	}
	return append(out, cur)
}

// joined reports whether a line runs from the upper staff down to the lower,
// which is what makes them one system.
func joined(c *pdf.Content, upper, lower staff) bool {
	// the space between them: below the upper staff, above the lower
	top, bottom := upper.y[0], lower.y[4]
	if top <= bottom {
		return false
	}
	slack := 0.5 * upper.inter
	for _, s := range c.Segments {
		if !s.Vertical() {
			continue
		}
		lo, hi := math.Min(s.Y1, s.Y2), math.Max(s.Y1, s.Y2)
		if lo <= bottom+slack && hi >= top-slack {
			return true
		}
	}
	return false
}
