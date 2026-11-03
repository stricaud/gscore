/*
 * rehearsal.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strconv"

// What the rehearsal marks are called.
//
// A mark is stored as a place in a sequence and not as the letter itself,
// and this is where the letter comes from. The reason is editing: a letter
// written down is right until somebody puts a mark in before it, and then
// the page reads A, B, D, C and nothing says which of them is wrong.
// Delete one and the sequence has a hole. Worked out from where the marks
// fall, all of that comes out right with nobody asked to fix it.
//
// Marks written out as themselves are left alone. A chorus headed "Verse",
// an insert called "1a", and everything read out of a file — which arrives
// already written out — are not in the sequence and are not renamed by it.
// They do not advance it either: a letter is a place in a list the
// conductor counts through, and a word in the middle of it is not one of
// the places.

// MarkerStyle is what the marks in the sequence are called.
type MarkerStyle int

const (
	// MarkerLetters is A, B, C — what most scores use.
	MarkerLetters MarkerStyle = iota
	// MarkerNumbers is 1, 2, 3, counting the marks rather than the bars.
	MarkerNumbers
	// MarkerBarNumbers names each mark after the bar it stands on, which
	// is how film and television scores are called.
	MarkerBarNumbers
)

func (m MarkerStyle) String() string {
	switch m {
	case MarkerNumbers:
		return "numbers"
	case MarkerBarNumbers:
		return "bars"
	}
	return "letters"
}

// MarkerStyleFromName reads back what String wrote.
func MarkerStyleFromName(s string) (MarkerStyle, bool) {
	switch s {
	case "", "letters":
		return MarkerLetters, true
	case "numbers":
		return MarkerNumbers, true
	case "bars", "bar-numbers":
		return MarkerBarNumbers, true
	}
	return 0, false
}

// HasMarker reports that a bar carries a rehearsal mark of either kind.
//
// What everything asking "is this bar named" wants: a mark breaks a run of
// counted rests and takes room over the bar whether it was typed or
// counted to.
func (m *Measure) HasMarker() bool {
	return m != nil && (m.Marker != "" || m.MarkerAuto)
}

// RehearsalLetter is the nth name in the sequence, counting from zero.
//
// A to Z, and then AA, BB, CC rather than AB — doubling is what a score
// does, because a conductor shouting "A B" across a hall is not heard as
// one thing.
//
// I is skipped. It is read as a one at a glance, which is the one thing a
// rehearsal mark must never be mistaken for in a room where people are
// also calling out bar numbers.
func RehearsalLetter(n int) string {
	const letters = "ABCDEFGHJKLMNOPQRSTUVWXYZ" // no I
	if n < 0 {
		return ""
	}
	per := len(letters)
	c := letters[n%per : n%per+1]
	out := c
	for i := 0; i < n/per; i++ {
		out += c
	}
	return out
}

// RehearsalMarks is what every marked bar is called, by its index.
//
// Both kinds are in it: the ones in the sequence, named from where they
// fall, and the ones written out as themselves, named by what they say. So
// a caller that wants to know what is printed over a bar asks this and
// does not have to know which kind it was.
//
// The sequence restarts at a movement. A symphony's second movement begins
// at letter A again, which is what a player turning to it expects and what
// every printed score does; the alternative is a slow movement that starts
// at letter P.
// bars is what each bar is numbered, for the style that names a mark
// after its bar; nil where the caller has not worked them out, and then a
// bar is called by its place. The numbers come in rather than being
// counted here because counting them is the engraver's — it knows about
// upbeats, the numbering a file states for itself and where a movement
// starts again, and a second copy of that arithmetic would be a mark
// pointing at the wrong page.
func (s *Score) RehearsalMarks(bars []int) map[int]string {
	out := map[int]string{}
	if s == nil || len(s.Parts) == 0 {
		return out
	}
	count := s.MeasureCount()
	n := 0
	for i := 0; i < count; i++ {
		// A movement boundary starts the letters again. Read off the bar
		// before, because EndsSection marks the last bar of a movement
		// rather than the first of the next.
		if i > 0 && s.endsSectionAt(i-1) {
			n = 0
		}
		m := s.markedBar(i)
		if m == nil {
			continue
		}
		switch {
		case !m.MarkerAuto:
			// Written out as itself, and not one of the places the
			// sequence counts through.
			out[i] = m.Marker
		default:
			out[i] = s.markerName(n, i, bars)
			n++
		}
	}
	return out
}

// markerName is what the nth mark of a movement is called.
func (s *Score) markerName(n, bar int, bars []int) string {
	switch s.Markers {
	case MarkerNumbers:
		return strconv.Itoa(n + 1)
	case MarkerBarNumbers:
		if bar >= 0 && bar < len(bars) {
			return strconv.Itoa(bars[bar])
		}
		return strconv.Itoa(bar + 1)
	}
	return RehearsalLetter(n)
}

// markedBar is the first staff's measure at a bar that carries a mark, or
// nothing. A mark belongs to the bar across the whole score, so whichever
// staff carries it is the bar's.
func (s *Score) markedBar(i int) *Measure {
	for _, p := range s.Parts {
		if i < len(p.Measures) && p.Measures[i].HasMarker() {
			return p.Measures[i]
		}
	}
	return nil
}

// endsSectionAt reports that a movement ends at this bar.
func (s *Score) endsSectionAt(i int) bool {
	for _, p := range s.Parts {
		if i < len(p.Measures) && p.Measures[i].EndsSection {
			return true
		}
	}
	return false
}

// MarkerAt is what is printed over a bar, or empty.
//
// A convenience over RehearsalMarks for the callers that want one bar. It
// builds the whole sequence to answer, because the name of a mark depends
// on every mark before it — which is the point of keeping them this way.
func (s *Score) MarkerAt(i int, bars []int) string {
	return s.RehearsalMarks(bars)[i]
}

// NextMarkerIsLetter is what a mark put at this bar would be called, for
// the editor to show before anything is written.
func (s *Score) NextMarkerIsLetter(at int, bars []int) string {
	if s == nil {
		return ""
	}
	// Count the sequence marks before this bar, in this movement.
	n := 0
	for i := 0; i < at && i < s.MeasureCount(); i++ {
		if i > 0 && s.endsSectionAt(i-1) {
			n = 0
		}
		if m := s.markedBar(i); m != nil && m.MarkerAuto {
			n++
		}
	}
	return s.markerName(n, at, bars)
}

// SetMarker puts a mark on a bar across every staff, or takes it away.
//
// Across every staff because that is what a rehearsal mark is: "from
// letter B" has to find the same bar for everybody in the room, and a mark
// on one staff is a mark nobody else can see.
func (s *Score) SetMarker(at int, text string, auto bool) bool {
	found := false
	for _, p := range s.Parts {
		if at < 0 || at >= len(p.Measures) {
			continue
		}
		p.Measures[at].Marker = text
		p.Measures[at].MarkerAuto = auto
		found = true
	}
	return found
}
