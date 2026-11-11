//go:build !js

/*
 * soundfontdir_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where soundfonts are kept, and what is offered to download.
 *
 * They used to go under the program's settings, which on a Mac is inside
 * the Library and hidden from the Finder. A soundfont is not a thing the
 * program alone touches: it is downloaded, swapped, borrowed from another
 * program, sent by somebody you play with. A folder nobody can find is a
 * folder nobody can put anything in, so it moved in beside the scores.
 */

package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSoundfontsAreKeptBesideTheScores(t *testing.T) {
	dir, err := soundfontDir()
	if err != nil {
		t.Fatalf("there is nowhere to keep a soundfont: %v", err)
	}
	if got := filepath.Base(dir); got != "soundfonts" {
		t.Errorf("the folder is called %q", got)
	}
	// Inside the folder the open and save panels already start in, so that
	// somebody told "put it in your Gscore folder" has one place to look.
	if parent := filepath.Dir(dir); parent != ScoresDir() {
		t.Errorf("soundfonts go in %q and scores in %q, which are two "+
			"folders to find instead of one", parent, ScoresDir())
	}
}

// The old folder is still read, and is not the new one.
//
// Nothing was moved when this changed. A download somebody waited twenty
// minutes for is not something to shuffle around behind their back, so the
// old place is simply still looked in.
func TestTheOldFolderIsStillLookedIn(t *testing.T) {
	old, err := oldSoundfontDir()
	if err != nil {
		t.Skipf("this machine has no config folder: %v", err)
	}
	now, err := soundfontDir()
	if err != nil {
		t.Fatal(err)
	}
	if old == now {
		t.Fatal("the two folders are the same, so this test proves nothing")
	}
	if !strings.HasSuffix(filepath.ToSlash(old), "Gscore/soundfonts") {
		t.Errorf("the old folder was %q, which is not where they were", old)
	}
}

func TestEverySoundfontOfferedCanBeFetched(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range catalogue {
		if s.ID == "" || seen[s.ID] {
			t.Errorf("%q is not an id of its own", s.ID)
		}
		seen[s.ID] = true
		if s.Name == "" {
			t.Errorf("%s has no name to show", s.ID)
		}
		// https and nothing else: handleSoundfontFetch refuses the rest,
		// so an entry that is not would be a row in the list that fails
		// when it is clicked.
		if !strings.HasPrefix(s.URL, "https://") {
			t.Errorf("%s is fetched from %q", s.ID, s.URL)
		}
		// The size is shown before anybody waits for the download, so it
		// has to be the real one rather than a guess.
		if s.Size < 1<<20 {
			t.Errorf("%s says it is %d bytes, which is no soundfont",
				s.ID, s.Size)
		}
		// What it is and whether you may use it. A list of names tells
		// nobody anything; this is the part worth reading.
		if s.About == "" || s.License == "" {
			t.Errorf("%s says nothing about what it is or how it is licensed",
				s.ID)
		}
	}
}

// Lightest first, because that is the choice being made: they are all good
// and the question is how long you are willing to wait for one.
func TestTheListRunsLightestFirst(t *testing.T) {
	for i := 1; i < len(catalogue); i++ {
		if catalogue[i].Size < catalogue[i-1].Size {
			t.Errorf("%s (%d) is offered after %s (%d), which is the bigger "+
				"of the two", catalogue[i].ID, catalogue[i].Size,
				catalogue[i-1].ID, catalogue[i-1].Size)
		}
	}
}

// Only the lightest is carried with the application. The rest are a
// download, which is what the dialog is for.
func TestOnlyTheLightestIsWorthShipping(t *testing.T) {
	if len(catalogue) == 0 {
		t.Fatal("nothing is offered at all")
	}
	// The one the Makefile bundles, by the name it is saved under there.
	const bundled = "GeneralUser GS"
	if catalogue[0].Name != bundled {
		t.Errorf("the first of the list is %q and the application ships "+
			"%q; the one that is carried should be the one offered first",
			catalogue[0].Name, bundled)
	}
	for _, s := range catalogue[1:] {
		if s.Size <= catalogue[0].Size {
			t.Errorf("%s is no bigger than the one that ships, so the "+
				"reason for not shipping it has gone", s.ID)
		}
	}
}

// A soundfont is called the same thing everywhere it is named.
//
// It carries a name of its own inside the file and it is often not the one
// anybody knows it by: the bank inside NAProGM.sf2 calls itself
// "NAProPercussionFull". A status line reading the one while the dialog it
// was picked in reads the other is two names for one thing, and looks like
// two soundfonts.
func TestASoundfontIsNamedTheSameEverywhere(t *testing.T) {
	// Not installed, so there is nothing to look it up by: the file's own
	// name is all there is.
	if got := soundfontDisplayName("/nowhere/Thing.sf2", "A Bank"); got != "A Bank" {
		t.Errorf("an unknown path came out as %q, want the file's own name", got)
	}
	// Nor that: something has to be said.
	if got := soundfontDisplayName("", ""); got == "" {
		t.Error("a soundfont with no name at all came out as nothing")
	}
	// And where it is installed, the name the lists use wins.
	for _, f := range listSoundfonts() {
		if f.Path == "" || f.Name == "" {
			continue
		}
		if got := soundfontDisplayName(f.Path, "something else"); got != f.Name {
			t.Errorf("%s is called %q in the lists and %q while it plays",
				f.Path, f.Name, got)
		}
		break
	}
}
