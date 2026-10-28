/*
 * form.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// The shape of a piece: who is playing when, and what repeats what.
//
// Both are questions about the whole of it rather than about a moment, and
// gscore could answer nothing of that kind. A symphony is fifty-three staves
// and two and a half thousand bars, and there was no way to see any of it at
// once — not which instruments are carrying a passage, not where the
// material comes back, not where the piece is dense and where it thins out.
//
// What repeats what is the nearer thing to an explanation of a piece. Music
// is made of things coming back, and a list of where they come back is the
// skeleton somebody listening is trying to hear. It is also the thing a
// composer most often cannot see in their own work, because they know what
// they meant by it.

package analysis

import (
	"sort"
	"strconv"
	"strings"
)

// BarShape is one bar in a form two bars can be compared in.
type BarShape struct {
	// Shape is what the bar holds with its pitches counted from its own
	// lowest note, so the same phrase a fifth higher has the same shape
	// and a list of repeats finds a sequence as well as a literal return.
	Shape string
	// Base is that lowest note, so two bars of one shape can say how far
	// apart they are.
	Base int
	// Notes is how many notes are in the bar. One with none has no shape
	// and is a repeat of nothing: a run of rests is not a return of
	// anything, however exactly it matches the rests before it.
	Notes int
}

// Shaped turns one bar's notes into a shape.
//
// notes are every sounding note in the bar, each with which staff it is on,
// where in the bar it starts and how long it lasts. The order they arrive in
// does not matter; they are sorted into the one order two bars can be
// compared in.
func Shaped(notes []BarNote) BarShape {
	if len(notes) == 0 {
		return BarShape{}
	}
	base := notes[0].MIDI
	for _, n := range notes {
		if n.MIDI < base {
			base = n.MIDI
		}
	}
	sorted := append([]BarNote(nil), notes...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		switch {
		case a.At != b.At:
			return a.At < b.At
		case a.Part != b.Part:
			return a.Part < b.Part
		case a.MIDI != b.MIDI:
			return a.MIDI < b.MIDI
		}
		return a.Held < b.Held
	})
	var b strings.Builder
	for _, n := range sorted {
		b.WriteString(strconv.Itoa(n.Part))
		b.WriteByte(':')
		b.WriteString(strconv.FormatFloat(n.At, 'f', 4, 64))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(n.MIDI - base))
		b.WriteByte(':')
		b.WriteString(strconv.FormatFloat(n.Held, 'f', 4, 64))
		b.WriteByte(' ')
	}
	return BarShape{Shape: b.String(), Base: base, Notes: len(notes)}
}

// BarNote is one note of a bar, as the shape of the bar is read from.
type BarNote struct {
	Part int
	MIDI int
	// At is where in the bar it starts and Held how long it lasts, both in
	// whole notes.
	At, Held float64
}

// Echo is a stretch of bars that has been heard before.
type Echo struct {
	FromBar int `json:"fromBar"`
	ToBar   int `json:"toBar"`
	// LikeBar is where the same music was first heard.
	LikeBar int `json:"likeBar"`
	Bars    int `json:"bars"`
	// Transposed is how far it has moved, in semitones. Zero is the same
	// notes again; anything else is a sequence or a restatement in
	// another key, which is a different thing to be told.
	Transposed int `json:"transposed"`
}

// LeastEcho is how many bars a return has to run to before it is worth
// saying so.
//
// Two. One bar coming back is an accompaniment figure, and a list of every
// bar of an Alberti bass that matches the bar before it is a list of nothing.
// Two bars in a row is the shortest thing anybody hears as a return.
const LeastEcho = 2

// Echoes finds every stretch of bars that is a stretch heard earlier.
//
// Each bar is reported once, as part of the longest return it belongs to,
// and against the earliest place that music was heard: the recapitulation of
// a sonata is a return of the exposition and not of the development's own
// quotation of it.
func Echoes(shapes []BarShape, least int) []Echo {
	if least < 1 {
		least = LeastEcho
	}
	// Where each shape has been seen, so that a bar is only compared
	// against bars that could possibly match it. Without this a symphony
	// is two and a half thousand bars compared against each other.
	where := map[string][]int{}
	for i, s := range shapes {
		if s.Notes == 0 {
			continue
		}
		where[s.Shape] = append(where[s.Shape], i)
	}

	var out []Echo
	for i := 0; i < len(shapes); {
		best := Echo{}
		for _, j := range where[shapes[i].Shape] {
			if j >= i {
				break // the earlier one has to be earlier
			}
			shift := shapes[i].Base - shapes[j].Base
			n := 0
			for i+n < len(shapes) && j+n < i &&
				shapes[i+n].Notes > 0 &&
				shapes[j+n].Shape == shapes[i+n].Shape &&
				shapes[i+n].Base-shapes[j+n].Base == shift {
				n++
			}
			if n > best.Bars {
				best = Echo{
					FromBar: i, ToBar: i + n - 1, LikeBar: j,
					Bars: n, Transposed: shift,
				}
			}
		}
		if best.Bars >= least {
			out = append(out, best)
			i = best.ToBar + 1
			continue
		}
		i++
	}
	return out
}

// Busy is how much each staff plays in each bar.
type Busy struct {
	Part int    `json:"part"`
	Name string `json:"name"`
	// Bars is how many notes the staff has in each bar of the piece. Not
	// how loud it is and not how important: how much is written there,
	// which is what shows an entry, a tutti and a staff that rests for
	// eighty bars.
	Bars []int `json:"bars"`
	// Notes is the whole staff's share of the piece, and Playing how many
	// bars it plays in at all.
	Notes   int `json:"notes"`
	Playing int `json:"playing"`
}

// Busiest is the most notes any one staff has in any one bar, which is what
// a picture of the others has to be drawn against.
func Busiest(all []Busy) int {
	most := 0
	for _, b := range all {
		for _, n := range b.Bars {
			if n > most {
				most = n
			}
		}
	}
	return most
}
