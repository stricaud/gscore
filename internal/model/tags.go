/*
 * tags.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import "strings"

/*
 * What counts as a tag, and what counts as the same tag.
 *
 * Here rather than in whichever program is writing them, because the
 * editor's tag window and the command line both put tags on the same file
 * and they have to agree about where one tag ends and the next begins. Two
 * answers to that means a score tagged from a script and a score tagged by
 * hand are filed differently, and nobody finds out until a search comes back
 * short.
 */

// ParseTags reads a line of tags the way somebody writes one.
//
// Separated by commas, because a tag is a phrase — "flute duet", "to learn
// by Easter" — and splitting on spaces would make three tags of each.
// Emptied of blanks and of repeats, so that a line typed with a trailing
// comma or the same word twice does not put either in the file.
//
// Compared without regard to capitals when deciding what is a repeat, and
// kept as typed otherwise: "Advent" and "advent" are one tag, and the one
// that survives is the one that was written first.
func ParseTags(line string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(line, ",") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// HasTag says whether the score already carries this tag.
//
// Without regard to capitals, for the same reason ParseTags ignores them: a
// score tagged "Advent" is not also tagged "advent", and asking for the
// second is asking for the first.
func (s *Score) HasTag(tag string) bool {
	want := strings.ToLower(strings.TrimSpace(tag))
	if want == "" {
		return false
	}
	for _, t := range s.Tags {
		if strings.ToLower(t) == want {
			return true
		}
	}
	return false
}

// AddTags puts on the score whichever of these tags it does not carry
// already, and says which ones it added.
//
// The ones already there are left exactly as they are, spelling included: a
// score tagged "Advent" that is handed "advent" keeps the capital. Somebody
// who tags a shelf of scores in one run should not find that the run
// rewrote the tags it found on the way.
//
// What it returns is what changed, so a caller can say so rather than
// reporting work it did not do.
func (s *Score) AddTags(tags []string) []string {
	var added []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || s.HasTag(t) {
			continue
		}
		s.Tags = append(s.Tags, t)
		added = append(added, t)
	}
	return added
}
