/*
 * playorder.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package model

// The order the bars are actually played in.
//
// A score is not read from the first bar to the last. A repeat sends you
// back, a first-time bar is skipped on the way through again, and a player
// who ignored all that would finish well before everybody else. Playback has
// to follow the same instructions, and so does the cursor — a line that runs
// down the page once while the music goes round twice is worse than no line.
//
// This turns the written score into the sequence of bars a player performs,
// which both of them then walk. Working it out once, here, is what keeps the
// two agreeing: the drawn score and the played score are different things and
// the only safe way to relate them is a list that says which is which.
//
// A measure's RightBarline is its closing barline, so an opening repeat on
// bar five is recorded as the closing barline of bar four.

// maxPlayedBars stops a malformed score — a repeat that always sends you back
// to a bar that sends you back — from running forever.
const maxPlayedBars = 20000

// PlayOrder returns the bars in the order they are played, as indices into
// the score. A bar played twice appears twice.
func (s *Score) PlayOrder() []int {
	n := s.MeasureCount()
	if n == 0 {
		return nil
	}

	// The barlines and endings, taken from whichever staff has them: they
	// belong to the piece, and the editor writes them on every staff.
	closing := make([]BarlineStyle, n)
	endings := make([]*Ending, n)
	for i := 0; i < n; i++ {
		for _, p := range s.Parts {
			if i >= len(p.Measures) {
				continue
			}
			m := p.Measures[i]
			if m.RightBarline != BarSingle && closing[i] == BarSingle {
				closing[i] = m.RightBarline
			}
			if m.Ending != nil && endings[i] == nil {
				endings[i] = m.Ending
			}
		}
	}

	var out []int
	// Where a repeat sends you back to, and how many times each closing
	// repeat has already sent you.
	back := 0
	taken := map[int]int{}
	// Which time through the current repeated section this is, counted from
	// one, which is what a volta's numbers mean.
	pass := 1

	for i := 0; i < n && len(out) < maxPlayedBars; {
		// A bar inside a volta that is not for this time through is skipped,
		// along with the rest of its bracket.
		if e := endings[i]; e != nil && !playsOn(e, pass) {
			j := i
			for j < n && endings[j] != nil && !playsOn(endings[j], pass) {
				// Past this bracket, and past any that follows it and is
				// also not for this time through.
				if endings[j].Stop {
					j++
					break
				}
				j++
			}
			if j <= i {
				j = i + 1
			}
			i = j
			continue
		}

		out = append(out, i)

		switch closing[i] {
		case BarRepeatStart:
			// The repeat opens at the next bar.
			back = i + 1
			i++
		case BarRepeatEnd, BarRepeatBoth:
			if taken[i] == 0 {
				taken[i] = 1
				pass++
				target := back
				if closing[i] == BarRepeatBoth {
					// Closes one section and opens another, so coming back
					// to it later returns here.
					back = i + 1
				}
				i = target
				continue
			}
			// Already been round; carry on, and a later repeat in the piece
			// starts counting again from here.
			if closing[i] == BarRepeatBoth {
				back = i + 1
			}
			pass = 1
			i++
		case BarFinal:
			// A final barline ends the piece unless a repeat has yet to send
			// us back, which the loop above has already settled.
			i++
		default:
			i++
		}
	}
	return out
}

// playsOn reports whether a volta bracket is taken on a given time through.
//
// A bracket with no numbers is taken whenever it is reached, which is how a
// last ending with nothing written on it behaves.
func playsOn(e *Ending, pass int) bool {
	if len(e.Numbers) == 0 {
		return true
	}
	for _, n := range e.Numbers {
		if n == pass {
			return true
		}
	}
	// Past the highest number written, the last bracket is the one taken:
	// a piece marked 1. and 2. and played a third time takes the second.
	highest := e.Numbers[0]
	for _, n := range e.Numbers {
		if n > highest {
			highest = n
		}
	}
	return pass > highest && e.Open
}
