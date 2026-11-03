/*
 * tags_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package model

import (
	"strings"
	"testing"
)

func TestATagIsAPhraseAndNotAWord(t *testing.T) {
	got := ParseTags("flute duet, to learn by Easter")
	want := []string{"flute duet", "to learn by Easter"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBlanksAndRepeatsDoNotReachTheFile(t *testing.T) {
	got := ParseTags(" advent ,, Advent,  advent,")
	if len(got) != 1 || got[0] != "advent" {
		t.Errorf("got %q, want the one tag as it was first written", got)
	}
}

// Running the same command over the same shelf twice is the whole point of
// asking for tags that are not there already: the second run has to be a
// no-op, down to the spelling of what was there.
func TestAddingATagTwiceAddsItOnce(t *testing.T) {
	s := &Score{Tags: []string{"Advent"}}

	added := s.AddTags([]string{"advent", "grade 3"})
	if strings.Join(added, "|") != "grade 3" {
		t.Errorf("added %q, want only the tag that was missing", added)
	}
	if strings.Join(s.Tags, "|") != "Advent|grade 3" {
		t.Errorf("tags are %q: the capital on the one already there was not kept",
			s.Tags)
	}

	if again := s.AddTags([]string{"ADVENT", "Grade 3"}); len(again) != 0 {
		t.Errorf("a second run added %q, and should have added nothing", again)
	}
	if len(s.Tags) != 2 {
		t.Errorf("tags are %q after a second run, want the same two", s.Tags)
	}
}

func TestATagThatIsNothingAtAllIsNotATag(t *testing.T) {
	s := &Score{}
	if added := s.AddTags([]string{"", "   "}); len(added) != 0 {
		t.Errorf("added %q from blanks", added)
	}
	if len(s.Tags) != 0 {
		t.Errorf("tags are %q, want none", s.Tags)
	}
}

func TestHasTagIgnoresCapitalsAndSpaceAroundIt(t *testing.T) {
	s := &Score{Tags: []string{"To Learn"}}
	for _, ask := range []string{"to learn", "TO LEARN", " to learn "} {
		if !s.HasTag(ask) {
			t.Errorf("HasTag(%q) said no", ask)
		}
	}
	if s.HasTag("learn") {
		t.Error(`HasTag("learn") said yes to a tag that reads "To Learn"`)
	}
}
