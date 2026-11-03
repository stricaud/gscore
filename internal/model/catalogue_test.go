/*
 * catalogue_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The catalogue has to agree with the sounds table, because an entry that
 * does not is an instrument somebody can pick and gscore then knows nothing
 * about: no name, no range, and a sound id written into the file that the
 * program reading it back cannot place.
 */

package model

import (
	"strings"
	"testing"
)

func TestEveryCatalogueInstrumentIsKnown(t *testing.T) {
	for _, c := range catalogue {
		s, ok := sounds[c.Sound]
		if !ok {
			t.Errorf("%s is in the catalogue but not in the sounds table", c.Sound)
			continue
		}
		if s.name == "" {
			t.Errorf("%s has no name to show", c.Sound)
		}
	}
}

// Catalogue() drops anything it cannot resolve, so every row has to come
// out at least once. It comes out more than once for a fretted
// instrument, which is offered on a staff, as tablature, and as both.
func TestCatalogueDropsNothing(t *testing.T) {
	got := Catalogue()
	seen := map[string]bool{}
	for _, it := range got {
		seen[it.Sound] = true
	}
	for _, c := range catalogue {
		if !seen[c.Sound] {
			t.Errorf("%s is in the list and not in the catalogue", c.Sound)
		}
	}
	if want := len(catalogue) + 2*len(fretted); len(got) != want {
		t.Errorf("Catalogue() gave %d entries, want %d", len(got), want)
	}
}

// A fretted instrument is offered three ways, and they are all the same
// instrument: the sound written into a file is the same for all of them.
func TestAFrettedInstrumentIsOfferedThreeWays(t *testing.T) {
	var ways []string
	for _, it := range Catalogue() {
		if it.Sound == "pluck.guitar" {
			ways = append(ways, it.Written)
			if it.Name == "" {
				t.Error("a way of writing it has no name")
			}
		}
	}
	// The plain one carries no way of its own: empty is a staff of
	// pitches, which is what everything else in the list is too.
	want := []string{"", "tab", "both"}
	if len(ways) != 3 {
		t.Fatalf("the guitar is offered %d ways: %v", len(ways), ways)
	}
	for i, w := range want {
		if ways[i] != w {
			t.Errorf("the ways are %v, want %v", ways, want)
			break
		}
	}
}

// The ones that are not fretted are offered once, and nothing claims a
// tuning it has not got.
func TestOnlyFrettedInstrumentsAreOfferedAsTab(t *testing.T) {
	for _, it := range Catalogue() {
		if it.Written == "" || it.Written == "pitches" {
			if it.Tuning != "" {
				t.Errorf("%s claims the tuning %q", it.Name, it.Tuning)
			}
			continue
		}
		if DefaultTuning(it.Sound) == "" {
			t.Errorf("%s is offered as tab and has no frets", it.Name)
		}
		if _, ok := TuningNamed(it.Tuning); !ok {
			t.Errorf("%s is tuned to %q, which is not a tuning", it.Name, it.Tuning)
		}
	}
}

func TestWrittenNamesRoundTrip(t *testing.T) {
	for _, w := range []Written{WrittenPitches, WrittenTab, WrittenBoth} {
		got, ok := WrittenFromName(w.String())
		if !ok || got != w {
			t.Errorf("%v came back as %v (%v)", w, got, ok)
		}
	}
	if _, ok := WrittenFromName("sideways"); ok {
		t.Error("a way nobody has heard of was accepted")
	}
}

func TestCatalogueGroupsAreDeclared(t *testing.T) {
	known := map[string]bool{}
	for _, g := range Groups {
		known[g] = true
	}
	for _, c := range catalogue {
		if !known[c.Group] {
			t.Errorf("%s is in group %q, which is not in Groups", c.Sound, c.Group)
		}
	}
	// And every declared group has something in it: an empty heading in
	// the list is a heading somebody opens and finds nothing under.
	for _, g := range Groups {
		found := false
		for _, c := range catalogue {
			if c.Group == g {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("group %q has no instruments in it", g)
		}
	}
}

func TestCatalogueListsNothingTwice(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range catalogue {
		if seen[c.Sound] {
			t.Errorf("%s is in the catalogue twice", c.Sound)
		}
		seen[c.Sound] = true
	}
}

// The catalogue is grouped, and a group has to be in one piece: a list that
// went Brass, Woodwind, Brass again would draw two Brass headings.
func TestCatalogueGroupsAreContiguous(t *testing.T) {
	var order []string
	for _, c := range catalogue {
		if len(order) == 0 || order[len(order)-1] != c.Group {
			order = append(order, c.Group)
		}
	}
	seen := map[string]bool{}
	for _, g := range order {
		if seen[g] {
			t.Errorf("group %q is broken into more than one run", g)
		}
		seen[g] = true
	}
}

// A sound id is lower case and dotted. This catches the kind of typo that
// otherwise shows up as an instrument quietly missing from the list.
func TestCatalogueIdsAreWellFormed(t *testing.T) {
	for _, c := range catalogue {
		if c.Sound != strings.ToLower(c.Sound) {
			t.Errorf("%s is not lower case", c.Sound)
		}
		if !strings.Contains(c.Sound, ".") {
			t.Errorf("%s is not a dotted sound id", c.Sound)
		}
		if strings.HasPrefix(c.Sound, ".") || strings.HasSuffix(c.Sound, ".") {
			t.Errorf("%s begins or ends with a dot", c.Sound)
		}
	}
}

// The aliases exist so old files still open; they must not be offered,
// because anything written from the catalogue should use the standard
// spelling.
func TestAliasesAreNotOffered(t *testing.T) {
	aliases := []string{
		"wind.reed.oboe.english-horn",
		"strings.harp",
		"strings.guitar",
		"pitched-percussion.timpani",
		"pitched-percussion.celesta",
		"pitched-percussion.steel-drums",
		"pitched-percussion.crotales",
	}
	for _, a := range aliases {
		if _, ok := sounds[a]; !ok {
			t.Errorf("alias %s has gone from the sounds table — old files need it", a)
		}
		for _, c := range catalogue {
			if c.Sound == a {
				t.Errorf("%s is an alias and should not be in the catalogue", a)
			}
		}
	}
}

// The three ids that were wrong before this table was checked against the
// standard. Each has to resolve to itself rather than falling back to its
// family, which is what was happening and is why an english horn was
// measured against an oboe's range.
func TestTheCorrectedIdsResolve(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"wind.reed.english-horn", "english horn"},
		{"pluck.harp", "harp"},
		{"pluck.guitar", "guitar"},
		{"drum.timpani", "timpani"},
		{"keyboard.celesta", "celesta"},
	} {
		got, ok := SoundName(tc.id)
		if !ok || got != tc.want {
			t.Errorf("SoundName(%q) = %q, %v; want %q", tc.id, got, ok, tc.want)
		}
	}
}

// A guitar and a harp used to have no range at all, because the ids they
// were filed under are not the ids files use.
func TestPluckedInstrumentsHaveTheirReach(t *testing.T) {
	for _, id := range []string{"pluck.harp", "pluck.guitar", "pluck.guitar.electric"} {
		if _, ok := SoundReach(id); !ok {
			t.Errorf("SoundReach(%q) knows nothing", id)
		}
	}
}
