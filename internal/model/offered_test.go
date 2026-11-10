/*
 * offered_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * An instrument a score knows where to put has to be one somebody can pick.
 *
 * The two lists are written for different jobs — the catalogue is ordered
 * for finding an instrument and the score order for reading one — and
 * nothing tied them together, so an instrument could be in one and not the
 * other. The clarinet in B flat was: the sounds table named it, the
 * transposing table put it a major second down, and the score order had it
 * between the plain clarinet and the one in A, but the list somebody picks
 * from had every clarinet except that one. Reported as "in the instruments
 * list I am missing clarinet in B♭".
 */

package model

import "testing"

func TestEveryInstrumentAScoreOrdersCanBeChosen(t *testing.T) {
	offered := map[string]bool{}
	for _, c := range catalogue {
		offered[c.Sound] = true
	}
	for _, id := range scoreOrder {
		if !offered[id] {
			name, _ := SoundName(id)
			t.Errorf("a score knows where to put %s (%q) and nobody can "+
				"choose it: it is in the score order and not in the "+
				"catalogue", id, name)
		}
	}
}

// And the clarinet itself, by name, because that is how it was asked for.
func TestTheClarinetInBFlatIsOffered(t *testing.T) {
	const id = "wind.reed.clarinet.bflat"
	var found *CatalogueItem
	for i, c := range Catalogue() {
		if c.Sound == id {
			found = &Catalogue()[i]
			break
		}
	}
	if found == nil {
		t.Fatal("the clarinet in B flat is not in the list to pick from")
	}
	if found.Name != "clarinet in B♭" {
		t.Errorf("it is listed as %q", found.Name)
	}
	if found.Group != GroupWoodwind {
		t.Errorf("it is in the %q group", found.Group)
	}
	// A major second below what is written, which is what makes it the
	// instrument it is rather than a clarinet with a different label.
	want := Transposition{Diatonic: -1, Chromatic: -2}
	if got, ok := SoundTransposition(id); !ok || got != want {
		t.Errorf("it transposes %+v (known: %v), want %+v", got, ok, want)
	}
	// And it is between the plain clarinet and the one in A down the page,
	// where a score puts it.
	plain, a := ScoreRank("wind.reed.clarinet"), ScoreRank("wind.reed.clarinet.a")
	if r := ScoreRank(id); r <= plain || r >= a {
		t.Errorf("it ranks %d, with the plain clarinet at %d and the one in "+
			"A at %d", r, plain, a)
	}
}
