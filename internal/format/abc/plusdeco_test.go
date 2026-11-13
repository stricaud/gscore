/*
 * plusdeco_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * +name+ is a decoration, and reading it as notes is worse than not reading
 * it at all.
 *
 * ABC 2.0 wrote decorations as +trill+. 2.1 moved to !trill! and left the
 * older spelling legal, and abcm2ps and abc2svg have read both ever since,
 * so files in the wild use either.
 *
 * gscore read only the second. The + was stepped over as a character the
 * reader did not know, and then the letters of the decoration's name were
 * read as pitches: f, e, a, c and the rest of the word became notes. A mark
 * silently lost is bad; a bar of music silently invented is worse, because
 * the page gives no sign of it and the file plays.
 */

func importOne(t *testing.T, src string) *model.Score {
	t.Helper()
	scores, err := ImportAll(strings.NewReader(src))
	if len(scores) == 0 {
		t.Fatalf("nothing imported: %v", err)
	}
	return scores[0]
}

func TestPlusDecorationsAreMarksAndNotNotes(t *testing.T) {
	const src = "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n+fermata+C +>+D E F|\n"
	s := importOne(t, src)
	notes := notesOf(s)
	// Four notes were written. The word "fermata" is not music.
	if len(notes) != 4 {
		got := make([]string, len(notes))
		for i, n := range notes {
			got[i] = n.Pitch.String()
		}
		t.Fatalf("parsed %d notes (%v), want the 4 that are written",
			len(notes), got)
	}
	if !notes[0].HasArticulation(model.Fermata) {
		t.Error("+fermata+ did not put a fermata on the C")
	}
	if !notes[1].HasArticulation(model.Accent) {
		t.Error("+>+ did not put an accent on the D")
	}
}

func TestPlusAndBangDecorationsAgree(t *testing.T) {
	// The same music written the two ways has to come out the same, or a
	// file converted from one spelling to the other changes meaning.
	head := "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n"
	a := notesOf(importOne(t, head+"+fermata+C +staccato+D +>+E F|\n"))
	b := notesOf(importOne(t, head+"!fermata!C !staccato!D !>!E F|\n"))
	if len(a) != len(b) {
		t.Fatalf("+...+ gave %d notes and !...! gave %d", len(a), len(b))
	}
	for i := range a {
		if len(a[i].Articulations) != len(b[i].Articulations) {
			t.Errorf("note %d: +...+ gave %v, !...! gave %v",
				i, a[i].Articulations, b[i].Articulations)
		}
	}
}

func TestAStrayPlusDoesNotEatTheBar(t *testing.T) {
	// A + that closes nothing is a typing mistake, and tune books have
	// them. Treated as the opening of a decoration it would run to the end
	// of the line hunting for its partner and take the music with it, so a
	// + with no closing + on the line is stepped over as it always was.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nC + D E|\n")
	if n := len(notesOf(s)); n != 3 {
		t.Errorf("parsed %d notes around a stray +, want 3", n)
	}
}
