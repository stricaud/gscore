/*
 * playorder.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	d := s.directions(n)

	var out []int
	// Where a repeat sends you back to, and how many times each closing
	// repeat has already sent you.
	back := 0
	taken := map[int]int{}
	// Which time through the current repeated section this is, counted from
	// one, which is what a volta's numbers mean.
	pass := 1
	// Each D.C. or D.S. is taken once. Taking it again is how a chart that
	// ends "D.S. al Coda" would otherwise play for ever.
	jumped := map[int]bool{}

	// After a D.C. or D.S. the piece is read differently: the written
	// repeats are not observed again, and Fine and To Coda, which meant
	// nothing on the way down, become live. That second reading is what the
	// directions are for.
	afterJump := false
	stopAtFine := false
	leaveAtCoda := false

	for i := 0; i < n && len(out) < maxPlayedBars; {
		// A bar inside a volta that is not for this time through is skipped,
		// along with the rest of its bracket.
		if e := d.endings[i]; e != nil && !playsOn(e, pass, d.lastEnding) {
			j := i
			for j < n && d.endings[j] != nil && !playsOn(d.endings[j], pass, d.lastEnding) {
				// Past this bracket, and past any that follows it and is
				// also not for this time through.
				if d.endings[j].Stop {
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

		if afterJump {
			// Fine ends the piece. The last bar written is not the last bar
			// played in a piece marked al Fine.
			if stopAtFine && d.signs[i] == Fine {
				break
			}
			// To Coda leaves the music here and goes on at the coda.
			if leaveAtCoda && d.signs[i] == ToCoda && d.coda >= 0 {
				i = d.coda
				continue
			}
		}

		if j := d.jumps[i]; j != nil && !jumped[i] {
			jumped[i] = true
			afterJump = true
			stopAtFine = j.Until == UntilFine
			leaveAtCoda = j.Until == UntilCoda

			target := 0
			if j.From == FromSegno && d.segno >= 0 {
				target = d.segno
			}
			// On the way through again a repeated section is played once,
			// through its last ending: a piece marked 1. and 2. and reached
			// a third time goes through the second.
			pass = d.lastEnding + 1
			i = target
			continue
		}

		if afterJump {
			i++
			continue
		}

		switch d.closing[i] {
		case BarRepeatStart:
			// The repeat opens at the next bar.
			back = i + 1
			i++
		case BarRepeatEnd, BarRepeatBoth:
			if taken[i] == 0 {
				taken[i] = 1
				pass++
				target := back
				if d.closing[i] == BarRepeatBoth {
					// Closes one section and opens another, so coming back
					// to it later returns here.
					back = i + 1
				}
				i = target
				continue
			}
			// Already been round; carry on, and a later repeat in the piece
			// starts counting again from here.
			if d.closing[i] == BarRepeatBoth {
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

// directions is everything about a bar that decides what is played next.
//
// It is gathered from whichever staff has it, because all of it belongs to
// the piece rather than to one player: everybody goes back at the same bar,
// and the editor writes these on every part.
type directions struct {
	closing []BarlineStyle
	endings []*Ending
	signs   []Sign
	jumps   []*Jump
	// segno and coda are where the signs are, or -1.
	segno, coda int
	// lastEnding is the highest volta number written anywhere, so that a
	// section reached once more than it has brackets takes the last of them.
	lastEnding int
}

func (s *Score) directions(n int) directions {
	d := directions{
		closing: make([]BarlineStyle, n),
		endings: make([]*Ending, n),
		signs:   make([]Sign, n),
		jumps:   make([]*Jump, n),
		segno:   -1,
		coda:    -1,
	}
	for i := 0; i < n; i++ {
		for _, p := range s.Parts {
			if i >= len(p.Measures) {
				continue
			}
			m := p.Measures[i]
			if m.RightBarline != BarSingle && d.closing[i] == BarSingle {
				d.closing[i] = m.RightBarline
			}
			if m.Ending != nil && d.endings[i] == nil {
				d.endings[i] = m.Ending
			}
			if m.Sign != NoSign && d.signs[i] == NoSign {
				d.signs[i] = m.Sign
			}
			if m.Jump != nil && d.jumps[i] == nil {
				d.jumps[i] = m.Jump
			}
		}
		switch d.signs[i] {
		case Segno:
			if d.segno < 0 {
				d.segno = i
			}
		case Coda:
			if d.coda < 0 {
				d.coda = i
			}
		}
		if e := d.endings[i]; e != nil {
			for _, num := range e.Numbers {
				if num > d.lastEnding {
					d.lastEnding = num
				}
			}
		}
	}
	return d
}

// playsOn reports whether a volta bracket is taken on a given time through.
//
// A bracket with no numbers is taken whenever it is reached, which is how a
// last ending with nothing written on it behaves.
//
// Past the highest number written anywhere in the piece, the bracket holding
// that number is the one taken: a section marked 1. and 2. and reached a
// third time — which is what a D.S. does — goes through the second ending and
// not the first.
func playsOn(e *Ending, pass, lastEnding int) bool {
	if len(e.Numbers) == 0 {
		return true
	}
	for _, n := range e.Numbers {
		if n == pass {
			return true
		}
		if pass > lastEnding && n == lastEnding {
			return true
		}
	}
	return false
}
