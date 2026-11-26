/*
 * ties.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// A tie is one sound written as two notes, so the note it lands on is the
// note it leaves: same letter, same octave, same alteration. These find the
// notes a tie joins, so that what is done to one end can be done to the
// other.

// HeldFrom is the note a tie holds into n, which stands at index at of the
// given voice of the given bar, or nil when nothing is tied into it.
func (p *Part) HeldFrom(bar, voice, at int, n *Note) *Note {
	el := p.elementAt(bar, voice, at)
	if n == nil || el == nil || !tieStops(el, n) {
		return nil
	}
	prev, _, _, ok := p.stepNote(bar, voice, at, -1)
	if !ok {
		return nil
	}
	from := headLike(prev, n.Pitch)
	if from == nil || !tieStarts(prev, from) {
		return nil
	}
	return from
}

// HeldInto is the notes a tie from n holds on into, in order, through as many
// ties as follow one another.
func (p *Part) HeldInto(bar, voice, at int, n *Note) []*Note {
	var out []*Note
	el := p.elementAt(bar, voice, at)
	for n != nil && el != nil && tieStarts(el, n) && len(out) < 1024 {
		next, nb, ni, ok := p.stepNote(bar, voice, at, 1)
		if !ok {
			break
		}
		to := headLike(next, n.Pitch)
		if to == nil || !tieStops(next, to) {
			break
		}
		out = append(out, to)
		el, n, bar, at = next, to, nb, ni
	}
	return out
}

func (p *Part) elementAt(bar, voice, at int) Element {
	if p == nil || bar < 0 || bar >= len(p.Measures) || p.Measures[bar] == nil {
		return nil
	}
	m := p.Measures[bar]
	if voice < 0 || voice >= len(m.Voices) || m.Voices[voice] == nil {
		return nil
	}
	els := m.Voices[voice].Elements
	if at < 0 || at >= len(els) {
		return nil
	}
	return els[at]
}

// stepNote is the note or chord next to the one at index at of the voice,
// before it or after it, across a barline if need be. A rest in between
// means there is none: nothing is held through a silence.
func (p *Part) stepNote(bar, voice, at, dir int) (Element, int, int, bool) {
	at += dir
	for bar >= 0 && bar < len(p.Measures) {
		m := p.Measures[bar]
		var els []Element
		if m != nil && voice < len(m.Voices) && m.Voices[voice] != nil {
			els = m.Voices[voice].Elements
		}
		if dir < 0 && at == maxIndex {
			at = len(els) - 1
		}
		for ; at >= 0 && at < len(els); at += dir {
			switch els[at].(type) {
			case *Note, *Chord:
				return els[at], bar, at, true
			case *Rest:
				return nil, 0, 0, false
			}
		}
		bar += dir
		if dir > 0 {
			at = 0
		} else {
			at = maxIndex
		}
	}
	return nil, 0, 0, false
}

const maxIndex = int(^uint(0) >> 1)

// headLike is the notehead of el on the same letter and octave as pitch.
func headLike(el Element, pitch Pitch) *Note {
	same := func(n *Note) bool {
		return n != nil && n.Pitch.Step == pitch.Step && n.Pitch.Octave == pitch.Octave
	}
	switch e := el.(type) {
	case *Note:
		if same(e) {
			return e
		}
	case *Chord:
		for _, n := range e.Notes {
			if same(n) {
				return n
			}
		}
	}
	return nil
}

// tieStarts and tieStops read a tie off a notehead, a tie written on a chord
// as a whole counting for each of its noteheads.
func tieStarts(el Element, n *Note) bool {
	if c, ok := el.(*Chord); ok && c.TieStart {
		return true
	}
	return n.TieStart
}

func tieStops(el Element, n *Note) bool {
	if c, ok := el.(*Chord); ok && c.TieStop {
		return true
	}
	return n.TieStop
}
