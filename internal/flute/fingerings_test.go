/*
 * fingerings_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The flute's fingerings.
 *
 * The table is generated, so what is worth testing is not its contents
 * but that the generated numbers still mean what the program thinks they
 * mean: the key order is the format, and a key inserted into the middle
 * of it would silently turn every fingering in the table into a
 * different one.
 */

package flute

import "testing"

func TestTheTableIsThere(t *testing.T) {
	if n := len(Names()); n < 36 {
		t.Fatalf("the flute has %d notes in the table", n)
	}
	// Lowest first, which is the order the window puts the buttons in.
	names := Names()
	if names[0] != "C4" || names[len(names)-1] != "C7" {
		t.Errorf("the range runs %s to %s", names[0], names[len(names)-1])
	}
	for i := 1; i < len(names); i++ {
		if pitchOf(names[i]) <= pitchOf(names[i-1]) {
			t.Fatalf("%s comes after %s", names[i], names[i-1])
		}
	}
}

// The key order is the format. These are the fingerings every flute
// player knows, so if the numbers ever stop spelling them the table and
// the program have come apart.
func TestTheStandardFingeringsAreTheOnesEverybodyLearns(t *testing.T) {
	for _, want := range []struct{ note, spell string }{
		// Low C takes the C key and not the C sharp one, which is what
		// the source says and what the roller under the right little
		// finger is for.
		{"C4", "Th 1 2 3 | 1 2 3 C"},
		{"D5", "Th 2 3 | 1 2 3"},
		{"A4", "Th 1 2 | D#"},
		{"B4", "Th 1 | D#"},
	} {
		n, ok := Of(want.note)
		if !ok {
			t.Errorf("no %s in the table", want.note)
			continue
		}
		if got := Spell(n.Standard.Keys); got != want.spell {
			t.Errorf("%s is %q, want %q", want.note, got, want.spell)
		}
	}
}

// The table is worth having for its alternatives, so it is worth knowing
// how many there are: a generated file that came out empty would still
// load, still answer, and still be useless.
//
// Low C has none, and that is the instrument rather than a gap in the
// scrape: there is one way to play the bottom note of a flute.
func TestTheAlternativesAreAllThere(t *testing.T) {
	total := 0
	for _, name := range Names() {
		n, _ := Of(name)
		total += len(n.Others)
		if len(n.Others) == 0 && name != "C4" {
			t.Errorf("%s has no alternative fingering at all", name)
		}
	}
	if total < 25000 {
		t.Errorf("the table holds %d alternatives; it should hold some "+
			"twenty-nine thousand", total)
	}
}

// Away is what orders the alternatives, so it is what makes a table of
// five thousand readable.
func TestAwayCountsTheKeysThatDiffer(t *testing.T) {
	n, _ := Of("D5")
	if got := Away(n.Standard.Keys, n.Standard.Keys); got != 0 {
		t.Errorf("a fingering is %d keys from itself", got)
	}
	near := 0
	for _, f := range n.Others {
		if Away(f.Keys, n.Standard.Keys) == 1 {
			near++
		}
	}
	if near == 0 {
		t.Error("nothing in the table is one key from the D5 chart")
	}
}

/*
 * A trill is not something the source labels, and it does not have to be:
 * it is a fingering of each note differing by a single key. The ones
 * below are the trills out of the back of a method book, which is the
 * check that the derivation finds real fingerings and not arithmetic.
 */
func TestTheDerivedTrillsAreTheOnesInTheBook(t *testing.T) {
	for _, want := range []struct {
		from, to, hold, key string
	}{
		// Lift the third finger: the plain D–E trill.
		{"D5", "E5", "Th 2 3 | 1 2 3", "3"},
		// The first trill key, which is what it is for: C–D is not
		// playable by moving the fingers of the C fingering.
		{"C5", "D5", "1 | D#", "tr1"},
		// Lift the second finger.
		{"A4", "B4", "Th 1 2 | D#", "2"},
	} {
		got := Trills(want.from, want.to)
		if len(got) == 0 {
			t.Errorf("no way of trilling %s to %s", want.from, want.to)
			continue
		}
		first := got[0]
		if Spell(first.Hold.Keys) != want.hold || first.Key != want.key {
			t.Errorf("%s-%s: holds %q and shakes %q, want %q and %q",
				want.from, want.to, Spell(first.Hold.Keys), first.Key,
				want.hold, want.key)
		}
		// The best one is on the chart, which is why it is the best one.
		if first.Away != 0 {
			t.Errorf("%s-%s: the first way given is %d keys off the chart",
				want.from, want.to, first.Away)
		}
	}
}

func TestATrillBetweenNotesThatAreNotThereIsNothing(t *testing.T) {
	if got := Trills("D5", "Z9"); got != nil {
		t.Errorf("trilling to a note that does not exist gave %d ways", len(got))
	}
}
