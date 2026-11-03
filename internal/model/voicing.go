/*
 * voicing.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "sort"

// Taking one player's line off a staff that carries two.
//
// Wind and brass are written two to a staff: "Flute 1, 2" is one line of
// the score, carrying a chord where the players differ and a single note
// where they agree. That is right for the score and useless at a stand —
// the second flute needs the lower line and nothing else, on its own page,
// with its own rests counted.
//
// What is done here is the taking, and nothing else. It reads a measure and
// returns what one part prints of it; it never writes to the score, and the
// part it is working for holds no music of its own. The rule is stored and
// the notes are fetched through it every time the part is drawn, so a note
// changed in the score is changed in both parts at once and there is
// nothing to keep in step.
//
// Two rules rather than one, because two players share a staff in two
// different ways. Where they move together they are written as chords, and
// which note belongs to whom is a matter of high and low. Where they are
// rhythmically apart they are written as layers with their stems opposed,
// and which belongs to whom is a matter of which layer. A real score has
// both, bar by bar, so each bar is asked what it is.

// Voiced is the measure as one part prints it, or m itself where the rule
// changes nothing.
//
// The measure returned is new, and the elements in it are new wherever they
// differ from the score's; anything printed unchanged is shared. Nothing
// here writes to m.
func (v Voicing) Voiced(m *Measure) *Measure {
	if m == nil || v.IsPlain() {
		return m
	}

	// Which rule applies is a question about the bar, not about the part.
	// A bar written in layers is answered by the layer rule and a bar
	// written as chords by the chord rule, and a part that crosses
	// between the two — which is most of them — gets the right answer in
	// each bar without anybody saying so.
	if len(m.Voices) > 1 {
		if v.Layer <= 0 {
			return m
		}
		out := *m
		if v.Layer <= len(m.Voices) {
			out.Voices = []*Voice{m.Voices[v.Layer-1]}
		} else {
			// The bar has layers but not that one: this player is not
			// in it, and silence is the honest answer. A bar left empty
			// would be a bar of nothing at all, which is not the same
			// as a bar of rest and would not be counted into a multirest.
			out.Voices = []*Voice{{Elements: []Element{
				&Rest{Dur: m.Voices[0].Duration()},
			}}}
		}
		return &out
	}

	if v.Pick == VoicingAll || len(m.Voices) == 0 {
		return m
	}
	out := *m
	out.Voices = []*Voice{{Elements: v.pickFrom(m.Voices[0].Elements)}}
	return &out
}

// pickFrom takes this part's notes out of one voice's elements.
func (v Voicing) pickFrom(in []Element) []Element {
	out := make([]Element, 0, len(in))
	for _, el := range in {
		switch e := el.(type) {
		case *Chord:
			out = append(out, v.fromChord(e))
		case *Note:
			// A note alone on the staff. Both players are playing it —
			// that is what writing it once means — so it belongs to the
			// top and to the bottom alike, and only a rule counting
			// positions through a chord has to think about it.
			if v.Pick != VoicingNth || v.Single || v.wantsFirst() {
				out = append(out, e)
				continue
			}
			out = append(out, &Rest{Dur: e.Dur})
		default:
			out = append(out, el)
		}
	}
	return out
}

// wantsFirst reports whether the rule asks for the note nearest whichever
// end it counts from, which is the only position a lone note occupies.
func (v Voicing) wantsFirst() bool {
	for _, n := range v.Nth {
		if n == 1 {
			return true
		}
	}
	return false
}

// fromChord is the one element this part prints where the score has a
// chord: a note, a smaller chord, or a rest where the part has nothing.
func (v Voicing) fromChord(c *Chord) Element {
	if len(c.Notes) == 0 {
		return c
	}
	// Highest first, so "top" and "bottom" mean what they say whatever
	// order the notes were written in. Sorted on a copy: the score's
	// chord keeps the order somebody typed.
	notes := make([]*Note, len(c.Notes))
	copy(notes, c.Notes)
	sort.SliceStable(notes, func(i, j int) bool {
		return notes[i].Pitch.MIDI() > notes[j].Pitch.MIDI()
	})

	var kept []*Note
	switch v.Pick {
	case VoicingTop:
		kept = notes[:1]
	case VoicingBottom:
		kept = notes[len(notes)-1:]
	case VoicingNth:
		from := notes
		if v.FromBottom {
			from = make([]*Note, len(notes))
			for i, n := range notes {
				from[len(notes)-1-i] = n
			}
		}
		for _, want := range v.Nth {
			if want >= 1 && want <= len(from) {
				kept = append(kept, from[want-1])
			}
		}
	default:
		return c
	}

	switch len(kept) {
	case 0:
		// Nothing of this chord is this player's. A rest, not an absent
		// bar: the time still passes and the player still counts it.
		return &Rest{Dur: c.Dur}
	case 1:
		// One note is a note and not a chord of one. A chord carrying a
		// single note draws the same but is a different thing to
		// everything downstream — the stem, the ties, the player — and
		// the part should read as though it had been written that way.
		n := *kept[0]
		n.Dur = c.Dur
		// The marks belong to the chord as a whole — its slur, its
		// accent, its dynamic — and the part is now that chord.
		n.Marks = mergedMarks(c.Marks, kept[0].Marks)
		return &n
	default:
		out := *c
		out.Notes = kept
		return &out
	}
}

// mergedMarks is the chord's marks with the note's own on top.
//
// A chord written with an accent and a slur carries them for the chord; the
// note inside it may carry a tie of its own, which is a fact about that
// note and not about the chord. Reduced to one note, the part needs both.
func mergedMarks(chord, note Marks) Marks {
	out := chord
	if note.TieStart {
		out.TieStart = true
	}
	if note.TieStop {
		out.TieStop = true
	}
	return out
}
