/*
 * dyntext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A dynamic written by hand is set in the dynamics font, letter by letter,
 * so that "mp-f" reads as a dynamic and not as a word in italics.
 */

package engrave

import "testing"

func TestAWrittenDynamicIsSetInTheDynamicsFont(t *testing.T) {
	got := dynamicPieces("mp-f")
	want := []string{"dynamicMezzo", "dynamicPiano", "dynamicCombinedSeparatorHyphen", "dynamicForte"}
	if len(got) != len(want) {
		t.Fatalf("mp-f set as %d pieces, wanted %d: %+v", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i].glyph != name {
			t.Errorf("piece %d is %q, wanted %s", i, got[i].glyph, name)
		}
		if got[i].width <= 0 {
			t.Errorf("piece %d has no width", i)
		}
	}
}

// A word that is not made of dynamic letters is a word, set in type beside
// the letters rather than spelt out in glyphs that do not have it.
func TestAWordInAWrittenDynamicIsSetInType(t *testing.T) {
	got := dynamicPieces("più f")
	if len(got) != 3 {
		t.Fatalf("più f set as %d pieces, wanted a word, a gap and a glyph: %+v", len(got), got)
	}
	if got[0].text != "più" || got[0].glyph != "" {
		t.Errorf("the word came out as %+v", got[0])
	}
	if got[1].width <= 0 || got[1].glyph != "" || got[1].text != "" {
		t.Errorf("the space came out as %+v", got[1])
	}
	if got[2].glyph != "dynamicForte" {
		t.Errorf("the f came out as %+v", got[2])
	}
}
