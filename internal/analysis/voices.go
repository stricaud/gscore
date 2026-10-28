/*
 * voices.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// How the lines move against each other.
//
// This is the one part of the analysis that judges rather than measures, and
// it has to be honest about that. Parallel fifths are not wrong; they are
// wrong *in a tradition* — roughly, European art music between Palestrina
// and Brahms — and they are the whole sound of organum, of Debussy, of most
// film music and of every power chord ever played. Saying "error" would be
// saying something false about every one of those.
//
// So nothing here is an error. Each finding says what it saw, where, and in
// which pair of lines, and leaves the composer to decide whether it is what
// they meant. Where it is what they meant they will ignore it, which costs
// them a glance; where it is not, they have been handed the one thing that
// is genuinely hard to see in your own work, because you know what you meant
// by it.
//
// What is found here is only what is unarguable once the tradition is
// granted: two lines moving in parallel perfect fifths or octaves, a line
// crossing under one below it, and a line moving past where its neighbour
// just was. The contested rules — hidden fifths, unresolved leading tones,
// unrecovered leaps — are left out rather than guessed at.

package analysis

import "sort"

// Line is one line of music through the piece: a voice of a staff, and
// where several notes are written in one voice, the nth down from the top
// of them.
//
// A chorale written on two staves has four lines in two voices: the soprano
// and alto are the top and bottom notes of the treble staff's chords. That
// is how such music is written and so that is how it is read.
type Line struct {
	Part, Voice, Rank int
	// Name is what to call it when a finding is about it.
	Name string
}

// Sound is one line sounding one note.
type Sound struct {
	// Line is which line, by its place in the lines given.
	Line int
	// Bar is which bar it starts in; At and Until bracket it along the
	// piece, in whole notes from the start.
	Bar       int
	At, Until float64
	MIDI, Ref int
	// Name is the note as it is written, spelling and all. Named from its
	// sounding number instead, D to E sharp came back as "D to F" — a
	// minor third, under a heading saying it was an augmented second.
	Name string
	// Letter is where the note sits on the staff, counted in letter names
	// rather than semitones. It is the one thing sounding pitch cannot
	// say: A to C and A to B sharp are the same three semitones and the
	// same sound, and one is a third a singer takes in their stride while
	// the other is the step they have complained about for four hundred
	// years.
	Letter int
}

// Motion is one thing worth saying about how two lines moved.
type Motion struct {
	// Kind is what was seen: "parallel fifths", "parallel octaves",
	// "crossing" or "overlap".
	Kind string `json:"kind"`
	// Text is the whole of it in a sentence.
	Text string `json:"text"`
	// FromBar and ToBar bracket it, and Times is how often it happened in
	// there. A left hand playing in octaves for three hundred bars is one
	// line doubled, and three hundred findings about it are three hundred
	// ways of not saying so — so a run of the same thing between the same
	// two lines is reported once, with how far it runs and how often.
	FromBar int `json:"fromBar"`
	ToBar   int `json:"toBar"`
	Times   int `json:"times"`
	// Upper and Lower are the two lines it is about, by name.
	Upper string `json:"upper"`
	Lower string `json:"lower"`
	// Ref is a notehead to point at, from the later of the two moments.
	Ref int `json:"ref"`
}

// Moving reads how the lines move against each other.
//
// sounds may be in any order. Lines are given top to bottom, which is what
// decides which of a pair is expected above the other.
func Moving(lines []Line, sounds []Sound) []Motion {
	if len(lines) < 2 || len(sounds) < 2 {
		return nil
	}
	byLine := make([][]Sound, len(lines))
	for _, s := range sounds {
		if s.Line < 0 || s.Line >= len(lines) {
			continue
		}
		byLine[s.Line] = append(byLine[s.Line], s)
	}
	for i := range byLine {
		sort.Slice(byLine[i], func(a, b int) bool {
			return byLine[i][a].At < byLine[i][b].At
		})
	}

	var out []Motion
	for a := 0; a < len(lines); a++ {
		for b := a + 1; b < len(lines); b++ {
			out = append(out, between(lines[a], lines[b], byLine[a], byLine[b])...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].FromBar < out[j].FromBar })
	return out
}

// between walks two lines together and reports what they do.
//
// Together: the two are stepped through in time, and a pair of moments is
// only judged where both lines sound in both of them and at least one of
// them moves. A line holding a note while another moves under it is oblique
// motion and is nobody's idea of a parallel.
func between(upper, lower Line, us, ls []Sound) []Motion {
	var out []Motion
	// Every moment either line begins or ends a note. The beginnings are
	// where a pair can be compared; the endings matter because a line that
	// stops breaks the pair, and without them two notes either side of a
	// rest were read as one line moving to the other.
	when := make([]float64, 0, 2*(len(us)+len(ls)))
	for _, s := range us {
		when = append(when, s.At, s.Until)
	}
	for _, s := range ls {
		when = append(when, s.At, s.Until)
	}
	sort.Float64s(when)

	at := func(line []Sound, t float64) (Sound, bool) {
		i := sort.Search(len(line), func(k int) bool { return line[k].At > t })
		if i == 0 {
			return Sound{}, false
		}
		s := line[i-1]
		if t >= s.Until {
			return Sound{}, false
		}
		return s, true
	}

	var lastU, lastL Sound
	have := false
	moves := 0
	for i, t := range when {
		if i > 0 && t == when[i-1] {
			continue
		}
		u, okU := at(us, t)
		l, okL := at(ls, t)
		if !okU || !okL {
			have = false
			continue
		}
		if !have {
			lastU, lastL, have = u, l, true
			continue
		}
		if u.Ref == lastU.Ref && l.Ref == lastL.Ref {
			continue // neither has moved: the same two notes still sounding
		}
		moves++
		if m, ok := judge(upper, lower, lastU, lastL, u, l); ok {
			out = append(out, m...)
		}
		lastU, lastL = u, l
	}
	_ = moves
	return runsOf(out)
}

// runsOf folds a run of the same thing between the same two lines into one
// finding.
//
// The two lines of a left hand playing in octaves do the same thing in every
// bar for pages at a time. Reported one by one that is hundreds of findings
// saying the same sentence, which buries the single parallel fifth
// somewhere in the middle of it — the one thing anybody wanted to be told.
func runsOf(found []Motion) []Motion {
	var out []Motion
	for _, m := range found {
		n := len(out)
		if n > 0 && out[n-1].Kind == m.Kind && out[n-1].Upper == m.Upper &&
			out[n-1].Lower == m.Lower {
			out[n-1].ToBar = m.ToBar
			out[n-1].Times++
			continue
		}
		m.Times = 1
		out = append(out, m)
	}
	return out
}

// judge is what the two lines did between one moment and the next.
func judge(upper, lower Line, u0, l0, u1, l1 Sound) ([]Motion, bool) {
	var out []Motion
	add := func(kind, text string, bar, ref int) {
		out = append(out, Motion{
			Kind: kind, Text: text, FromBar: bar, ToBar: bar,
			Upper: upper.Name, Lower: lower.Name, Ref: ref,
		})
	}

	// Crossing: the line written above is below the one written under it.
	if u1.MIDI < l1.MIDI && u0.MIDI >= l0.MIDI {
		add("crossing",
			upper.Name+" crosses below "+lower.Name,
			u1.Bar, u1.Ref)
	}
	// Overlap: the lower line moves above where the upper one just was, or
	// the upper moves below where the lower just was. The two lines have
	// changed places without crossing, which reads as one line jumping.
	if l1.MIDI > u0.MIDI && u1.MIDI >= l1.MIDI {
		add("overlap",
			lower.Name+" moves above where "+upper.Name+" just was",
			l1.Bar, l1.Ref)
	}

	du, dl := u1.MIDI-u0.MIDI, l1.MIDI-l0.MIDI
	if du == 0 || dl == 0 || (du > 0) != (dl > 0) {
		return out, len(out) > 0 // oblique or contrary: no parallel to find
	}
	was, now := u0.MIDI-l0.MIDI, u1.MIDI-l1.MIDI
	if was < 0 {
		was = -was
	}
	if now < 0 {
		now = -now
	}
	switch {
	case was%12 == 7 && now%12 == 7:
		add("parallel fifths",
			upper.Name+" and "+lower.Name+" move in parallel fifths",
			u1.Bar, u1.Ref)
	case was%12 == 0 && now%12 == 0:
		what := "octaves"
		if was == 0 || now == 0 {
			what = "unisons"
		}
		add("parallel "+what,
			upper.Name+" and "+lower.Name+" move in parallel "+what,
			u1.Bar, u1.Ref)
	}
	return out, len(out) > 0
}
