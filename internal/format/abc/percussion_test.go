/*
 * percussion_test.go
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
 * A drum voice is read as drums.
 *
 * ABC has no way of its own to say a drum, so on a clef=perc voice a note
 * is the drum at its position, as gscore's own kit lays them out: the hi-hat
 * in the space over the staff with a cross for a head, the snare on the
 * middle line, the kick in the bottom space. Read as pitches, which is what
 * they were, a drum part played as notes and looked like none.
 */

func TestADrumVoiceIsReadAsDrums(t *testing.T) {
	s, err := Import(strings.NewReader("X:1\nM:4/4\nL:1/8\nK:C\nV:1 clef=perc name=\"Drums\"\n[BG,] B [BD] !open!B |\n"))
	if s == nil {
		t.Fatal(err)
	}
	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 4 {
		t.Fatalf("%d elements, want 4", len(els))
	}
	first, ok := els[0].(*model.Chord)
	if !ok || len(first.Notes) != 2 {
		t.Fatalf("the first beat is %T, want a chord of two", els[0])
	}
	hat, kick := first.Notes[0], first.Notes[1]
	if hat.Percussion != 42 || hat.Head != model.HeadX {
		t.Errorf("the hi-hat came out as key %d head %v", hat.Percussion, hat.Head)
	}
	if kick.Percussion != 36 || kick.Head != model.HeadNormal {
		t.Errorf("the kick came out as key %d head %v", kick.Percussion, kick.Head)
	}
	third := els[2].(*model.Chord)
	if snare := third.Notes[1]; snare.Percussion != 38 {
		t.Errorf("the snare came out as key %d", snare.Percussion)
	}
	if open := els[3].(*model.Note); open.Percussion != 46 || open.Head != model.HeadCircleX {
		t.Errorf("the open hi-hat came out as key %d head %v", open.Percussion, open.Head)
	}
	// And a pitched voice is untouched by any of this.
	s, _ = Import(strings.NewReader("X:1\nM:4/4\nL:1/8\nK:C\nV:1 clef=treble\nB8 |\n"))
	if n := s.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note); n.Percussion != 0 || n.Head != model.HeadNormal {
		t.Errorf("a treble B was read as a drum: key %d head %v", n.Percussion, n.Head)
	}
}
