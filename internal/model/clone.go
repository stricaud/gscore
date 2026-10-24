/*
 * clone.go
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

// Taking a copy of a score.
//
// This is what undo is built on. The alternative — recording each edit and
// knowing how to reverse it — needs every command to carry its own inverse,
// and the first one that gets it slightly wrong corrupts the score in a way
// nobody notices until they save. A copy cannot be slightly wrong.
//
// It is only expensive in the sense that it is proportional to the score,
// which for the largest thing anyone edits here is some tens of thousands of
// small structures. A person pressing a key cannot tell.
//
// The one rule: every pointer reachable from a Score has to be copied, or the
// copy and the original share something and editing one changes the other.
// CloneTest in the tests walks a fully populated score to check exactly that.

// Clone returns a deep copy of the score, sharing nothing with it.
func (s *Score) Clone() *Score {
	if s == nil {
		return nil
	}
	out := &Score{
		Identity: s.Identity,
		Tempo:    s.Tempo,
	}
	if s.Groups != nil {
		out.Groups = append([]StaffGroup(nil), s.Groups...)
	}
	for _, p := range s.Parts {
		out.Parts = append(out.Parts, p.Clone())
	}
	return out
}

// Clone returns a deep copy of one staff.
func (p *Part) Clone() *Part {
	if p == nil {
		return nil
	}
	out := *p
	out.Measures = make([]*Measure, len(p.Measures))
	for i, m := range p.Measures {
		out.Measures[i] = m.Clone()
	}
	return &out
}

// Clone returns a deep copy of one bar.
func (m *Measure) Clone() *Measure {
	if m == nil {
		return nil
	}
	out := *m
	if m.Key != nil {
		k := *m.Key
		out.Key = &k
	}
	if m.Time != nil {
		t := *m.Time
		out.Time = &t
	}
	if m.Tempo != nil {
		t := *m.Tempo
		out.Tempo = &t
	}
	if m.Ending != nil {
		e := *m.Ending
		if m.Ending.Numbers != nil {
			e.Numbers = append([]int(nil), m.Ending.Numbers...)
		}
		out.Ending = &e
	}
	if m.Jump != nil {
		j := *m.Jump
		out.Jump = &j
	}
	out.Voices = make([]*Voice, len(m.Voices))
	for i, v := range m.Voices {
		out.Voices[i] = v.Clone()
	}
	return &out
}

// Clone returns a deep copy of one voice.
func (v *Voice) Clone() *Voice {
	if v == nil {
		return nil
	}
	out := &Voice{}
	if v.Elements != nil {
		out.Elements = make([]Element, len(v.Elements))
		for i, el := range v.Elements {
			out.Elements[i] = CloneElement(el)
		}
	}
	return out
}

// CloneElement copies a note, rest, chord or symbol.
func CloneElement(el Element) Element {
	switch e := el.(type) {
	case *Note:
		n := *e
		n.Marks = CloneMarks(e.Marks)
		return &n
	case *Rest:
		r := *e
		r.Marks = CloneMarks(e.Marks)
		return &r
	case *Chord:
		c := *e
		c.Marks = CloneMarks(e.Marks)
		c.Notes = make([]*Note, len(e.Notes))
		for i, n := range e.Notes {
			cn := *n
			cn.Marks = CloneMarks(n.Marks)
			c.Notes[i] = &cn
		}
		return &c
	case *SymbolElement:
		sy := *e
		return &sy
	}
	return el
}

// CloneMarks copies everything written on an element.
func CloneMarks(m Marks) Marks {
	out := m
	if m.Articulations != nil {
		out.Articulations = append([]Articulation(nil), m.Articulations...)
	}
	if m.Symbols != nil {
		out.Symbols = append([]SymbolMark(nil), m.Symbols...)
	}
	if m.Lyrics != nil {
		out.Lyrics = append([]Lyric(nil), m.Lyrics...)
	}
	if m.Texts != nil {
		out.Texts = append([]TextMark(nil), m.Texts...)
	}
	if m.Tuplet != nil {
		t := *m.Tuplet
		out.Tuplet = &t
	}
	if m.Tremolo != nil {
		tr := *m.Tremolo
		out.Tremolo = &tr
	}
	if m.Hairpin != nil {
		h := *m.Hairpin
		out.Hairpin = &h
	}
	if m.Octave != nil {
		o := *m.Octave
		out.Octave = &o
	}
	if m.Trill != nil {
		t := *m.Trill
		out.Trill = &t
	}
	if m.Chord != nil {
		c := *m.Chord
		if m.Chord.Frame != nil {
			f := *m.Chord.Frame
			if m.Chord.Frame.Frets != nil {
				f.Frets = append([]int(nil), m.Chord.Frame.Frets...)
			}
			if m.Chord.Frame.Fingers != nil {
				f.Fingers = append([]int(nil), m.Chord.Frame.Fingers...)
			}
			c.Frame = &f
		}
		out.Chord = &c
	}
	if m.Grace != nil {
		g := *m.Grace
		g.Notes = make([]*Note, len(m.Grace.Notes))
		for i, n := range m.Grace.Notes {
			cn := *n
			cn.Marks = CloneMarks(n.Marks)
			g.Notes[i] = &cn
		}
		out.Grace = &g
	}
	return out
}
