/*
 * pairing.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

// Pairing a tab staff with a staff of pitches.
//
// A Guitar Pro file is tablature and nothing else: every track it holds
// becomes a staff of string-and-fret numbers, because that is what the file
// says and an importer that invented anything more would be guessing. But
// tablature says where to put a finger and not what the note is, so a player
// learning to read, or anyone else in the room reading from the same page,
// has nothing to go on.
//
// The notes are there already. A fret on a string is a pitch — the importer
// works it out as it reads, which is what makes the file play — so the staff
// of pitches is not new music and not a transcription. It is the same music
// written the other way, and that is exactly what Part.MirrorOf is for: one
// staff holds the music, the other shows it, and nothing can drift because
// there is only one copy.
//
// Which staff holds it matters. The music moves up to the new staff and the
// tab becomes the mirror, rather than the other way round, because that is
// the shape the editor already builds when somebody picks "guitar — notation
// and tablature" from the instrument list. One shape, understood everywhere,
// beats two that have to be told apart.

// PairWithNotation puts a staff of pitches above every staff written as
// tablature alone, and joins the two with a rule and run-through barlines —
// the way guitar music is printed, which says one player reading one line
// written twice rather than two players.
//
// A staff already paired with something is left as it is, and so is one that
// holds no tab. It reports how many staves it added, so a caller can say
// nothing when there was nothing to do.
func (s *Score) PairWithNotation() int {
	if s == nil {
		return 0
	}
	added := 0
	// Backwards, so that each insertion only disturbs indexes past the one
	// being looked at and the ones still to come keep their numbers.
	for i := len(s.Parts) - 1; i >= 0; i-- {
		p := s.Parts[i]
		if p == nil || p.Tab == nil || p.MirrorOf != nil {
			continue
		}
		if s.mirroredBy(i) >= 0 {
			continue // already shown a second way somewhere
		}

		// The staff of pitches is this staff over again, without the
		// tuning and with the music: a tab staff carries the clef the
		// importer chose for the instrument, which is the clef the notes
		// want too — Treble8vb for a guitar, bass for a bass.
		above := *p
		above.Tab = nil
		above.MirrorOf = nil

		// The tab keeps the tuning and gives up the music. Named once, over
		// the pair: a name beside both staves says there are two guitars.
		p.Measures = nil
		p.Name = ""

		s.Parts = append(s.Parts, nil)
		copy(s.Parts[i+1:], s.Parts[i:])
		s.Parts[i] = &above
		ShiftStaffIndexes(s, i, 1)

		at := i
		p.MirrorOf = &at
		s.Groups = append(s.Groups, StaffGroup{
			First: i, Last: i + 1, Symbol: GroupLine,
			Barlines: true, Name: above.Name,
		})
		added++
	}
	return added
}

// mirroredBy is the staff showing staff i's music a second way, or -1.
func (s *Score) mirroredBy(i int) int {
	for j, p := range s.Parts {
		if p != nil && p.MirrorOf != nil && *p.MirrorOf == i {
			return j
		}
	}
	return -1
}

// ShiftStaffIndexes renumbers everything that points at a staff by index,
// after delta staves have been put in at or taken out from `at`.
//
// A tablature says which staff it shows by that staff's number, and a staff
// added or taken away above it changes every number after it. Left alone, a
// guitar's tab ends up naming whatever staff has fallen into that place — or
// itself, after which it shows nothing at all, which is how a pair came out
// empty the first time a staff was removed above it.
//
// A mirror whose staff has gone stops being one. It keeps its tuning and its
// ruling and simply holds nothing, which is a staff somebody can see and
// delete, rather than a dangling reference nothing explains.
//
// Both the mirrors and the groups, always together: they are two kinds of
// reference to the same numbering, and renumbering one without the other
// leaves a brace drawn round the wrong pair of staves.
func ShiftStaffIndexes(s *Score, at, delta int) {
	if s == nil {
		return
	}
	for i, p := range s.Parts {
		if p == nil || p.MirrorOf == nil {
			continue
		}
		n := *p.MirrorOf
		if n >= at {
			n += delta
		}
		// A staff whose music has gone shows what it has, which is
		// nothing, rather than pointing off the end of the score.
		if n < 0 || n >= len(s.Parts) || n == i {
			p.MirrorOf = nil
			continue
		}
		p.MirrorOf = &n
	}
	var kept []StaffGroup
	for _, g := range s.Groups {
		if g.First >= at {
			g.First += delta
		}
		if g.Last >= at {
			g.Last += delta
		}
		if g.First < 0 || g.Last >= len(s.Parts) || g.Last <= g.First {
			continue
		}
		kept = append(kept, g)
	}
	s.Groups = kept
}
