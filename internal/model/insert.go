/*
 * insert.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// Putting bars into the middle of a piece.
//
// Music is typed in and a bar or two is missed, and the mistake is found
// four pages later. Adding bars at the end and moving everything along by
// hand is not a repair anybody should have to make, so this is the repair:
// say where, say how many, and the rest of the piece moves over.
//
// Everything that names a bar by its number has to move with it. That is
// the whole of the work and the whole of what goes wrong when it is not
// done: the system breaks of a part, the bars a cue is taken from, and
// where the writing cursor was left are all held as indices, and bars
// pushed in underneath them leave each one pointing a little earlier in
// the music than it did.

// InsertMeasures puts count empty bars into a score after the bar at
// after, and returns how many went in.
//
// onlyPart is which staff they go into, or -1 for all of them. All of them
// is nearly always what is wanted: a bar is a bar across the score and the
// barlines have to stay in a column. One staff is the other case — a line
// that was typed a bar short against the rest — and it moves that staff's
// music along relative to everybody else's, which is exactly what it is
// for and is why it is not the default.
//
// after is the bar the new ones follow, so inserting after bar 3 leaves
// bar 3 where it is. -1 puts them at the front.
func (s *Score) InsertMeasures(after, count, onlyPart int) int {
	if s == nil || count <= 0 || len(s.Parts) == 0 {
		return 0
	}
	bars := s.MeasureCount()
	if after < -1 || after >= bars {
		return 0
	}
	at := after + 1

	for pi, p := range s.Parts {
		if onlyPart >= 0 && pi != onlyPart {
			continue
		}
		// A staff that mirrors another has no bars to put any into.
		if p.Mirrors() {
			continue
		}
		p.insertMeasures(at, count)
	}

	// The staves that did not take them are now short, so they are made
	// up at the end. A score whose staves are different lengths is a
	// score with bars that exist for one player and not for another, and
	// nothing downstream — the barlines, the bar numbers, the parts —
	// has an answer for that.
	if onlyPart >= 0 {
		want := s.MeasureCount()
		for pi, p := range s.Parts {
			if pi == onlyPart || p.Mirrors() {
				continue
			}
			for len(p.Measures) < want {
				p.AppendMeasure()
			}
		}
	}

	s.shiftBarIndices(at, count, onlyPart)
	return count
}

// insertMeasures puts count empty bars into one staff at an index.
func (p *Part) insertMeasures(at, count int) {
	if at > len(p.Measures) {
		at = len(p.Measures)
	}
	made := make([]*Measure, count)
	for i := range made {
		made[i] = NewMeasure()
	}
	p.Measures = append(p.Measures, made...)
	copy(p.Measures[at+count:], p.Measures[at:])
	for i := 0; i < count; i++ {
		p.Measures[at+i] = made[i]
	}

	// The barline the bar before was closed with belongs to the end of
	// the run, not to the middle of it.
	//
	// A repeat closing bar four, with two bars pushed in after it, is a
	// repeat of six bars and not a repeat of four with two left outside;
	// a final barline at the end of the piece is the end of the piece
	// wherever the end now is. Leaving it where it was put a double bar
	// or a repeat in the middle of the music, which is the first thing
	// anybody would have to undo by hand.
	if at > 0 && at <= len(p.Measures)-count {
		prev := p.Measures[at-1]
		if prev.RightBarline != BarSingle {
			p.Measures[at+count-1].RightBarline = prev.RightBarline
			prev.RightBarline = BarSingle
		}
	}
}

// shiftBarIndices moves everything that names a bar by its number.
//
// onlyPart says which staff the bars went into, or -1 for all. A part's
// system breaks follow the staves it prints, and a cue follows the staff
// it is taken from, so where only one staff grew only the things pointing
// at that staff move.
func (s *Score) shiftBarIndices(at, count, onlyPart int) {
	moved := func(staff int) bool { return onlyPart < 0 || staff == onlyPart }

	for si := range s.Sheets {
		sh := &s.Sheets[si]
		// Where the part begins a system. Its own staves grew, so its
		// own page turns move with them.
		prints := onlyPart < 0
		for _, st := range sh.Staves {
			if st == onlyPart {
				prints = true
			}
		}
		if prints {
			for i, b := range sh.Breaks {
				if b >= at {
					sh.Breaks[i] = b + count
				}
			}
		}
		// A cue names bars of the staff it is taken from.
		for i := range sh.Cues {
			if moved(sh.Cues[i].From) && sh.Cues[i].At >= at {
				sh.Cues[i].At += count
			}
		}
	}

	// And where the writing cursor was left, so that typing carries on
	// where it was rather than a few bars earlier.
	if s.Cursor != nil && moved(s.Cursor.Part) && s.Cursor.Measure >= at {
		s.Cursor.Measure += count
	}
}

// Taking bars out again.
//
// The other half of putting them in, and missing until somebody typed two
// bars too many at the end of a piece and found there was no way to say so:
// bars could be added, split, joined, emptied and pushed in, and never
// removed. Emptying one leaves a bar of rest, which is a bar the player
// counts; it is not the same answer at all.

// RemoveMeasures takes count bars out of a score from the bar at, and
// returns how many went.
//
// Out of every staff. A bar is a bar across the score — the barlines stay
// in a column and every player has the same bar twenty-six — so there is
// no per-staff form of this, as there is for putting them in. Taking a bar
// out of one staff would leave that staff a bar short of the others for
// the rest of the piece, which is not a thing anybody means by "remove
// this bar".
//
// A score is never left with no bars at all. The last one is emptied
// instead of going, because somewhere to write is the one thing the editor
// cannot do without: with no bars there is nothing on the page to click
// and no way back.
func (s *Score) RemoveMeasures(at, count int) int {
	if s == nil || count <= 0 || len(s.Parts) == 0 {
		return 0
	}
	bars := s.MeasureCount()
	if at < 0 || at >= bars {
		return 0
	}
	if at+count > bars {
		count = bars - at
	}
	// Everything would go. Keep one, emptied, so there is a bar to write
	// in — and then there is one fewer bar to take out.
	if count >= bars {
		count = bars - 1
		for _, p := range s.Parts {
			if p.Mirrors() || len(p.Measures) == 0 {
				continue
			}
			last := p.Measures[len(p.Measures)-1]
			for _, v := range last.Voices {
				if v != nil {
					v.Elements = nil
				}
			}
		}
		if count <= 0 {
			return 0
		}
	}

	for _, p := range s.Parts {
		if p.Mirrors() {
			continue
		}
		p.removeMeasures(at, count)
	}

	s.shiftBarIndices(at+count, -count, -1)
	// What pointed into the run that went now points at where it was.
	s.clampBarIndices(at)
	return count
}

// removeMeasures takes count bars out of one staff at an index.
func (p *Part) removeMeasures(at, count int) {
	if at >= len(p.Measures) {
		return
	}
	if at+count > len(p.Measures) {
		count = len(p.Measures) - at
	}
	// The barline the last bar of the run was closed with belongs to the
	// end of the music, not to the bars that went. A final barline on the
	// last bar of a piece is the end of the piece wherever the end now is,
	// and a repeat closing the run still closes what is left of it.
	//
	// The mirror of what insertMeasures does, for the same reason: the
	// alternative is a piece whose last bar ends on a plain line because
	// the double bar was on a bar that was deleted.
	gone := p.Measures[at+count-1]
	p.Measures = append(p.Measures[:at], p.Measures[at+count:]...)
	if gone.RightBarline != BarSingle && at > 0 && at <= len(p.Measures) {
		if at == len(p.Measures) || p.Measures[at-1].RightBarline == BarSingle {
			p.Measures[at-1].RightBarline = gone.RightBarline
		}
	}
}

// clampBarIndices brings anything left pointing inside a run of bars that
// no longer exists back to where the run was.
//
// shiftBarIndices moves what is after the run; this is for what was in it.
// A system break on a bar that has gone, left where it was, is a page turn
// in the middle of the bar that took its place.
func (s *Score) clampBarIndices(at int) {
	bars := s.MeasureCount()
	last := bars - 1
	if last < 0 {
		last = 0
	}
	hold := func(n int) int {
		if n < at {
			return n
		}
		if n > last {
			return last
		}
		return n
	}
	for si := range s.Sheets {
		sh := &s.Sheets[si]
		kept := sh.Breaks[:0]
		seen := map[int]bool{}
		for _, b := range sh.Breaks {
			b = hold(b)
			// A run of bars that went leaves several breaks on the one bar
			// that took their place, and a bar cannot be broken twice.
			if b <= 0 || seen[b] {
				continue
			}
			seen[b] = true
			kept = append(kept, b)
		}
		sh.Breaks = kept
		for i := range sh.Cues {
			sh.Cues[i].At = hold(sh.Cues[i].At)
		}
	}
	if s.Cursor != nil {
		s.Cursor.Measure = hold(s.Cursor.Measure)
	}
}
