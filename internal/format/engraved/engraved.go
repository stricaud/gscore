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
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
	"github.com/stricaud/gscore/internal/textmetric"
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
// ErrScan says the PDF held no music to read glyph by glyph: it is a
// picture of a page, which is read another way — see internal/format/scanned.
var ErrScan = errors.New("no staves found — a scan rather than an engraving?")

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
				// Its name, where one is written beside the staff: the
				// first system of a score names every part in full.
				if part.Name == "" {
					part.Name = partName(c, st)
				}
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
		return nil, fmt.Errorf("engraved: %w", ErrScan)
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
	rules := joinedRules(c)
	var longest float64
	for _, r := range rules {
		longest = math.Max(longest, r.x1-r.x0)
	}
	if longest == 0 {
		return nil
	}
	var lines []rule
	for _, r := range rules {
		if r.x1-r.x0 < 0.6*longest {
			continue
		}
		lines = append(lines, r)
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
	var uniq []rule
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

// partName is what is written to the left of a staff, which on the first
// system of a score is the instrument's name: "Flûte", "Clarinette en Sib 1".
//
// The name is text — glyphs of a text font, not the music font — sitting
// before the staff begins and level with it. The letters arrive one glyph
// each, in order of position, and a gap wider than a letter's worth between
// two of them is a space; the file carries no space characters as such.
func partName(c *pdf.Content, st staff) string {
	type letter struct {
		x, size, advance float64
		r                rune
	}
	var letters []letter
	lo, hi := st.y[0]-st.inter, st.y[4]+st.inter
	for _, g := range c.Glyphs {
		if g.Rune == 0 || isMusicRune(g.Rune) || g.X >= st.x0 || g.Y < lo || g.Y > hi {
			continue
		}
		if g.Rune < 0x20 { // a tab or the like, written to position words
			continue
		}
		letters = append(letters, letter{g.X, g.Size, g.Advance, g.Rune})
	}
	if len(letters) == 0 {
		return ""
	}
	sort.Slice(letters, func(i, j int) bool { return letters[i].x < letters[j].x })
	var b strings.Builder
	for i, l := range letters {
		if i > 0 {
			// A gap wider than the letter before it is a space. Measured
			// against that letter's own advance where the font gives one,
			// since an m is three times an i; against a typical letter's
			// width where it does not.
			prev := letters[i-1]
			room := 0.75 * math.Max(l.size, prev.size)
			switch est := textmetric.StdWidth("Times-Roman", prev.r); {
			case prev.advance > 0:
				room = prev.advance + 0.15*prev.size
			case est > 0:
				// No table in the file: a Roman face's width for the
				// letter is near enough to tell an m from an i.
				room = float64(est)/1000*prev.size + 0.2*prev.size
			}
			if l.x-prev.x > room {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(l.r)
	}
	return strings.TrimSpace(b.String())
}

// isMusicRune says a code point is one of a music font's own, in the range
// SMuFL reserves, or is one of the musical signs ordinary text borrows.
func isMusicRune(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) || (r >= 0x1D100 && r <= 0x1D1FF)
}

// rule is a horizontal stroke, or several of them laid end to end.
type rule struct{ y, x0, x1 float64 }

// joinedRules is the page's horizontal strokes, with those that continue
// one another joined into one, and the frame around the page left out.
//
// MuseScore 4 strokes a staff line a measure at a time, so a line that
// runs the width of the system arrives as a row of strokes each a measure
// long — and measured one by one against the longest stroke on the page,
// none of them is long enough to be a staff line, so no staff was found and
// the file was reported as a scan. Joined end to end they are the line
// they were drawn as.
//
// The same files carry a rectangle the size of the paper, drawn by Qt
// around the page. It is the longest stroke there is, and measuring the
// staff lines against it would throw out every one of them.
func joinedRules(c *pdf.Content) []rule {
	var rules []rule
	for _, s := range c.Segments {
		if !s.Horizontal() {
			continue
		}
		x0, x1 := math.Min(s.X1, s.X2), math.Max(s.X1, s.X2)
		if x1-x0 < 0.5 {
			continue // a dot, not a line
		}
		if c.Width > 0 && x1-x0 >= 0.98*c.Width {
			continue // the frame
		}
		rules = append(rules, rule{(s.Y1 + s.Y2) / 2, x0, x1})
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].y != rules[j].y {
			return rules[i].y < rules[j].y
		}
		return rules[i].x0 < rules[j].x0
	})
	var out []rule
	for _, r := range rules {
		if n := len(out); n > 0 {
			last := &out[n-1]
			// On the same line, and starting where the last left off or
			// before: a barline's width apart at most.
			if math.Abs(r.y-last.y) < 0.05 && r.x0 <= last.x1+2 {
				last.x1 = math.Max(last.x1, r.x1)
				continue
			}
		}
		out = append(out, r)
	}
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
