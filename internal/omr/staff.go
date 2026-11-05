/*
 * staff.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package omr

import (
	"math"
	"sort"
)

// Staff is five lines, top first.
//
// Top first because that is the order they come out of a downward scan, not
// because it is the musical order; Degree below puts it right.
type Staff struct {
	Lines [5]Line
}

// Interline is the distance between two neighbouring lines, measured at the
// middle of the page so that the tilt does not enter.
func (s Staff) Interline(atX float64) float64 {
	return (s.Lines[4].Y(atX) - s.Lines[0].Y(atX)) / 4
}

// Degree is the diatonic step of a point, counted from the bottom line.
//
// Zero is the bottom line, 1 the space above it, 2 the next line up. The
// bottom line is the reference rather than the top because that is how a
// musician counts, and the reading is taken at the point's own x so that the
// page's tilt never enters the answer. A whole-page deskew would be the
// obvious alternative and is worse: it resamples every pixel, blurring the
// one-pixel lines that everything else depends on.
func (s Staff) Degree(x, y float64) float64 {
	inter := s.Interline(x)
	if inter == 0 {
		return 0
	}
	return 2 * (s.Lines[4].Y(x) - y) / inter
}

// FindStaves locates every staff on the page and returns them top to bottom,
// along with the interline it measured.
//
// The method is to follow each line across the page rather than to look for
// it in one shot. A projection of the whole width — the textbook approach —
// fails here: a page tilted by two degrees smears a line over a dozen rows,
// and the peak it should make disappears. Taken a slice at a time the line is
// flat within the slice, so it makes a sharp peak; the slices are then
// chained left to right and a straight line fitted through the chain.
//
// Two details earn their place:
//
// A run of ink only counts as a line if it is thin. A notehead blackens as
// many pixels in a column as a line does, but over five or six rows rather
// than one or two, and without this test the heads pull the fit towards
// themselves.
//
// A chain may skip a few slices. Where a notehead sits on a line the run
// thickens and fails the thinness test, so the line goes missing for a slice
// or two; without the tolerance the chain breaks there and the staff is lost.
func FindStaves(b *Binary) ([]Staff, float64) {
	// Chaining needs a tolerance — how far a line may move from one slice
	// to the next and still be the same line — and the only sensible unit
	// for it is the interline, which is what we are trying to measure. So
	// we measure twice: once with a tolerance loose enough for any page,
	// which is enough to get the spacing roughly right, and once properly.
	//
	// Skipping the second pass is tempting and wrong. On a page with seven
	// pixels between lines, a tolerance of six pixels lets a chain step
	// across to its neighbour halfway along, and the line fitted through
	// the result leans at an angle no line on the page actually has.
	lines, mids := chainLines(b, 6)
	inter := spacing(mids)
	if inter <= 0 {
		return nil, 0
	}
	lines, mids = chainLines(b, 0.35*inter)
	inter = spacing(mids)
	if inter <= 0 {
		return nil, 0
	}

	if len(lines) < 5 {
		return nil, 0
	}
	order := make([]int, len(lines))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return mids[order[i]] < mids[order[j]] })

	// Assemble staves by position rather than by taking lines five at a
	// time in order.
	//
	// Taking them in order is the obvious thing and it is brittle: anything
	// else on the page straight enough to be chained — the baseline of a
	// line of type, a long slur, the rule under a heading — joins the queue,
	// and from there on every group of five is off by one. A single intruder
	// spoils the rest of the page.
	//
	// Looking for companions at one, two, three and four interlines below a
	// candidate top line cannot be derailed that way: the intruder is simply
	// a candidate that finds no companions, and the staff beneath it is
	// found exactly as if it were not there.
	// Every line is tried as a top line, every resulting group is scored,
	// and the groups are then taken best first. Taking them in page order
	// instead would let a stray line one interline above a staff claim four
	// of that staff's five lines and leave the real bottom line orphaned —
	// which is exactly what happened on the page this was built against,
	// twice out of ten staves.
	//
	// The score is how badly the group departs from what a staff looks
	// like: how far its lines sit from a perfectly even ladder, and how far
	// their slopes sit from one another. A real staff scores near nought on
	// both counts; a group built around an intruder cannot, because the
	// intruder was not engraved on the same plate and does not lean with
	// the rest.
	type group struct {
		pick [5]int
		cost float64
	}
	var cands []group
	for i := range order {
		top := mids[order[i]]
		g := group{pick: [5]int{i}}
		ok := true
		for k := 1; k < 5; k++ {
			want := top + float64(k)*inter
			best, bestD := -1, 0.35*inter
			for j := range order {
				if j == i {
					continue
				}
				if d := math.Abs(mids[order[j]] - want); d < bestD {
					best, bestD = j, d
				}
			}
			if best < 0 {
				ok = false
				break
			}
			g.pick[k] = best
			g.cost += bestD / inter
		}
		if !ok {
			continue
		}
		slopes := make([]float64, 5)
		for k, j := range g.pick {
			slopes[k] = lines[order[j]].A
		}
		med := median(slopes)
		for _, a := range slopes {
			// a slope error is counted in interlines of drift across the
			// page, so that it is comparable with the spacing error above
			g.cost += math.Abs(a-med) * float64(b.W) / inter
		}
		cands = append(cands, g)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].cost < cands[j].cost })

	used := make([]bool, len(order))
	var out []Staff
	for _, g := range cands {
		clash := false
		for _, j := range g.pick {
			if used[j] {
				clash = true
				break
			}
		}
		if clash {
			continue
		}
		var s Staff
		for k, j := range g.pick {
			s.Lines[k] = lines[order[j]]
			used[j] = true
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Lines[0].B < out[j].Lines[0].B
	})
	return out, inter
}

// thinRuns returns the mid-height of every run of ink in the slice that is
// inked across most of the slice and no more than a few rows tall.
func thinRuns(b *Binary, x0, x1 int) []float64 {
	const (
		fill     = 0.65 // of the slice's width
		maxThick = 3
	)
	var out []float64
	y := 0
	for y < b.H {
		if rowFill(b, x0, x1, y) < fill {
			y++
			continue
		}
		start := y
		for y < b.H && rowFill(b, x0, x1, y) >= fill {
			y++
		}
		if y-start <= maxThick {
			out = append(out, float64(start+y-1)/2)
		}
	}
	return out
}

func rowFill(b *Binary, x0, x1, y int) float64 {
	n := 0
	for x := x0; x < x1; x++ {
		if b.At(x, y) {
			n++
		}
	}
	return float64(n) / float64(x1-x0)
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// chainLines follows every staff line across the page and fits a straight
// line through each, returning the fits and the mean height of each.
//
// tol is how far a line may move between neighbouring slices and still count
// as the same line. It must be well under an interline or chains hop to
// their neighbours; see FindStaves for why it is passed in rather than
// chosen here.
func chainLines(b *Binary, tol float64) ([]Line, []float64) {
	const slices = 32
	step := b.W / slices
	if step < 8 {
		step = 8
	}
	type point struct{ x, y float64 }
	type chain struct {
		pts  []point
		gaps int
		open bool
	}
	var chains []*chain
	const maxGaps = 4

	for x0 := 0; x0+step <= b.W; x0 += step {
		peaks := thinRuns(b, x0, x0+step)
		cx := float64(x0) + float64(step)/2
		used := make([]bool, len(peaks))
		for _, c := range chains {
			if !c.open {
				continue
			}
			last := c.pts[len(c.pts)-1].y
			// the window widens with the slices missed, since a tilted line
			// has drifted further by the time it shows again
			window := tol * (1 + 0.5*float64(c.gaps))
			best, bestD := -1, window
			for i, p := range peaks {
				if used[i] {
					continue
				}
				if d := math.Abs(p - last); d < bestD {
					best, bestD = i, d
				}
			}
			if best >= 0 {
				used[best] = true
				c.pts = append(c.pts, point{cx, peaks[best]})
				c.gaps = 0
			} else if c.gaps++; c.gaps > maxGaps {
				c.open = false
			}
		}
		for i, p := range peaks {
			if !used[i] {
				chains = append(chains, &chain{pts: []point{{cx, p}}, open: true})
			}
		}
	}

	// a staff line crosses a good part of the width; anything shorter is a
	// stem, a bar, or a letter
	wanted := b.W / step / 3
	if wanted < 4 {
		wanted = 4
	}
	var lines []Line
	var mids []float64
	for _, c := range chains {
		if len(c.pts) < wanted {
			continue
		}
		xs := make([]float64, len(c.pts))
		ys := make([]float64, len(c.pts))
		var sum float64
		for i, p := range c.pts {
			xs[i], ys[i] = p.x, p.y
			sum += p.y
		}
		lines = append(lines, fitLine(xs, ys))
		mids = append(mids, sum/float64(len(c.pts)))
	}
	return lines, mids
}

// spacing is the typical distance between neighbouring lines.
//
// The median, because the list also holds the wide gap between one staff and
// the next and whatever strays were chained; a mean would be dragged by both.
func spacing(mids []float64) float64 {
	if len(mids) < 2 {
		return 0
	}
	s := append([]float64(nil), mids...)
	sort.Float64s(s)
	var gaps []float64
	for i := 1; i < len(s); i++ {
		if d := s[i] - s[i-1]; d > 2 && d < 200 {
			gaps = append(gaps, d)
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	return median(gaps)
}
