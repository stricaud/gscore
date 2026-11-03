/*
 * setup_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Laying a score out for the instruments chosen.
 *
 * Three conventions about the page that nobody should have to remember:
 * the order the staves go in, the bracket round each family, and the two
 * staves and the brace a piano gets.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func setup(t *testing.T, h *harness, q string) {
	t.Helper()
	h.post("/api/setup?instruments=" + q)
}

func staffNames(h *harness) []string {
	var out []string
	for _, p := range h.app.score.Parts {
		out = append(out, p.Name)
	}
	return out
}

// The order of a score is not the order of the catalogue, and not the
// order they were asked for.
func TestTheStavesComeOutInScoreOrder(t *testing.T) {
	h := newHarness(t)
	// Asked for backwards and jumbled.
	setup(t, h, "strings.cello,brass.trumpet,wind.flutes.flute,drum.timpani,strings.violin")

	want := []string{"Flute", "Trumpet", "Timpani", "Violin", "Cello"}
	got := staffNames(h)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the score reads %v, want %v", got, want)
	}
}

// The horn sits above the trumpets, which is the one place the rule is
// not "highest first" and is what every printed score does.
func TestTheHornIsAboveTheTrumpets(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "brass.trumpet,brass.french-horn,brass.trombone,brass.tuba")

	// Capitalised the way gscore capitalises a name from its own table:
	// the first letter, and no more.
	want := []string{"French horn", "Trumpet", "Trombone", "Tuba"}
	if got := staffNames(h); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the brass reads %v, want %v", got, want)
	}
}

// The piccolo is above the flute in a score, and below it in the picker.
func TestThePiccoloIsAboveTheFlute(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "wind.flutes.flute,wind.flutes.flute.piccolo")
	if got := staffNames(h); got[0] != "Piccolo" {
		t.Errorf("the woodwind reads %v, want the piccolo first", got)
	}
}

// Two of the same instrument are two staves, and they keep the order they
// were asked in: the first flute stays the first flute.
func TestTwoOfTheSameAreTwoStaves(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "wind.flutes.flute,wind.flutes.flute")
	if n := len(h.app.score.Parts); n != 2 {
		t.Errorf("%d staves, want 2", n)
	}
}

/*
 * The brackets and the braces.
 */

func TestEachFamilyIsBracketed(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "wind.flutes.flute,wind.reed.oboe,brass.trumpet,brass.tuba,"+
		"strings.violin,strings.cello")

	brackets := 0
	for _, g := range h.app.score.Groups {
		if g.Symbol == model.GroupBracket {
			brackets++
		}
	}
	if brackets != 3 {
		t.Errorf("%d brackets, want one each for woodwind, brass and strings", brackets)
	}
}

// A family of one is not a section, and a bracket round it would say
// there is a group of one.
func TestAFamilyOfOneIsNotBracketed(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "wind.flutes.flute,brass.trumpet,strings.violin")
	for _, g := range h.app.score.Groups {
		if g.Symbol == model.GroupBracket {
			t.Errorf("a lone staff was bracketed: %+v", g)
		}
	}
}

func TestAPianoGetsTwoStavesAndABrace(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "keyboard.piano")

	if n := len(h.app.score.Parts); n != 2 {
		t.Fatalf("a piano got %d staves", n)
	}
	if got := h.app.score.Parts[1].Clef; got != model.Bass {
		t.Errorf("the left hand is in %v", got)
	}
	// Named once: "Piano" beside both staves says there are two pianos.
	if h.app.score.Parts[1].Name != "" {
		t.Errorf("the second staff is called %q", h.app.score.Parts[1].Name)
	}
	if len(h.app.score.Groups) != 1 {
		t.Fatalf("%d groups", len(h.app.score.Groups))
	}
	g := h.app.score.Groups[0]
	if g.Symbol != model.GroupBrace || !g.Barlines {
		t.Errorf("the piano is joined by %+v", g)
	}
}

// Percussion is one section however its staves are read, so the timpani
// and the snare drum are bracketed together.
func TestThePercussionIsOneSection(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "drum.timpani,drum.snare-drum")

	if n := len(h.app.score.Groups); n != 1 {
		t.Fatalf("%d groups, want the percussion bracketed as one", n)
	}
	if got := h.app.score.Groups[0].Symbol; got != model.GroupBracket {
		t.Errorf("the percussion is joined by a %v", got)
	}
}

/*
 * What each staff is.
 */

func TestEachStaffKnowsItsInstrument(t *testing.T) {
	h := newHarness(t)
	setup(t, h, "wind.reed.clarinet.a,strings.cello,drum.snare-drum")

	// In score order, so: clarinet, snare drum, cello.
	clar, drum, cello := h.app.score.Parts[0], h.app.score.Parts[1], h.app.score.Parts[2]
	if clar.Sound != "wind.reed.clarinet.a" {
		t.Errorf("the clarinet's sound is %q", clar.Sound)
	}
	// Its own transposition, which a program number could not tell from
	// a clarinet in B flat.
	if clar.Transpose.Chromatic != -3 {
		t.Errorf("the clarinet in A transposes %+v", clar.Transpose)
	}
	if got := cello.Clef; got != model.Bass {
		t.Errorf("the cello is in %v", got)
	}
	if drum.Clef != model.Percussion || drum.Key.Fifths != 0 {
		t.Errorf("the snare drum is %v in %d sharps", drum.Clef, drum.Key.Fifths)
	}
}

/*
 * What it will not do.
 */

// Making the staves is a few seconds of work and losing a morning's is
// not, so the one that cannot be undone by hand is the one that asks.
func TestItWillNotThrowAwayMusic(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&step=4&dur=quarter")

	if got := h.postFails("/api/setup?instruments=strings.violin"); !strings.Contains(got, "already has music") {
		t.Errorf("it said %q", got)
	}
	// And does it when told plainly.
	h.post("/api/setup?instruments=strings.violin,strings.cello&replace=1")
	if n := len(h.app.score.Parts); n != 2 {
		t.Errorf("%d staves after replace=1", n)
	}
}

func TestNonsenseIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{"", "instruments=", "instruments=wind.reed.kazoo"} {
		if got := h.postFails("/api/setup?" + q); strings.Contains(got, "meant to be refused") {
			t.Errorf("%q was allowed", q)
		}
	}
}

// Somebody who arranged the staves themselves is not second-guessed.
func TestTheirOwnOrderIsKept(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setup?order=keep&instruments=strings.cello,wind.flutes.flute")
	if got := staffNames(h); got[0] != "Cello" {
		t.Errorf("the order reads %v, want the cello first as asked", got)
	}
}

// A score set up after the key has been chosen is in that key, with each
// transposing instrument reading its own.
func TestTheStavesJoinTheKeyTheScoreIsIn(t *testing.T) {
	h := newHarness(t)
	// One flat, concert. Set on the score rather than through an
	// endpoint: what is being checked is that new staves join the key
	// the score is already in, not how it got into it.
	h.app.score.Parts[0].Key = model.KeySignature{Fifths: -1, Mode: model.Major}
	setup(t, h, "wind.flutes.flute,wind.reed.clarinet")

	if got := h.app.score.Parts[0].Key.Fifths; got != -1 {
		t.Errorf("the flute reads %d flats, want 1", -got)
	}
	// A clarinet in B flat reads a major second higher: one flat
	// concert is one sharp for the player.
	if got := h.app.score.Parts[1].Key.Fifths; got != 1 {
		t.Errorf("the clarinet reads %d, want 1 sharp", got)
	}
}

/*
 * Bracketing by hand.
 *
 * The families are a good guess and not always the answer: a continuo
 * group, a solo pair, an antiphonal choir. Where somebody has arranged
 * the groups themselves, theirs are the ones drawn.
 */

func TestTheGroupsGivenReplaceTheFamilies(t *testing.T) {
	h := newHarness(t)
	// Flute, oboe, violin, cello — two families, which would be two
	// brackets left to itself. Asked for one round the middle pair.
	h.post("/api/setup?order=keep&brackets=1-2&instruments=" +
		"wind.flutes.flute,wind.reed.oboe,strings.violin,strings.cello")

	if n := len(h.app.score.Groups); n != 1 {
		t.Fatalf("%d groups, want the one that was asked for", n)
	}
	g := h.app.score.Groups[0]
	if g.First != 1 || g.Last != 2 || g.Symbol != model.GroupBracket {
		t.Errorf("the bracket is %+v, want staves 1 to 2", g)
	}
}

func TestSeveralGroupsCanBeGiven(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setup?order=keep&brackets=0-1,2-3&instruments=" +
		"wind.flutes.flute,wind.reed.oboe,strings.violin,strings.cello")
	if n := len(h.app.score.Groups); n != 2 {
		t.Errorf("%d groups, want 2", n)
	}
}

// A bracket round an instrument that reads two staves reaches round both.
func TestAGroupReachesRoundAGrandStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setup?order=keep&brackets=0-1&instruments=" +
		"keyboard.piano,strings.cello")

	var bracket *model.StaffGroup
	for i := range h.app.score.Groups {
		if h.app.score.Groups[i].Symbol == model.GroupBracket {
			bracket = &h.app.score.Groups[i]
		}
	}
	if bracket == nil {
		t.Fatal("no bracket was drawn")
	}
	// The piano is staves 0 and 1, the cello is staff 2.
	if bracket.First != 0 || bracket.Last != 2 {
		t.Errorf("the bracket is %+v, want staves 0 to 2", *bracket)
	}
}

// A group of one staff is not a group, and is let through rather than
// refused: a chooser should not have to know that.
func TestAGroupOfOneDrawsNothing(t *testing.T) {
	h := newHarness(t)
	h.post("/api/setup?order=keep&brackets=0-0&instruments=" +
		"wind.flutes.flute,wind.reed.oboe")
	for _, g := range h.app.score.Groups {
		if g.Symbol == model.GroupBracket {
			t.Errorf("a bracket was drawn round one staff: %+v", g)
		}
	}
}

func TestANonsenseGroupIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{"brackets=0-9", "brackets=x-2", "brackets=-1-0"} {
		got := h.postFails("/api/setup?order=keep&" + q +
			"&instruments=wind.flutes.flute,wind.reed.oboe")
		if strings.Contains(got, "meant to be refused") {
			t.Errorf("%q was allowed", q)
		}
	}
}
