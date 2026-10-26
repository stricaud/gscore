/*
 * text_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"testing"

	"github.com/stricaud/gscore/internal/pdf"
	"github.com/stricaud/gscore/internal/smufl"
)

// A flat sign in a staff name is a flat sign.
//
// The reading faces a PDF carries without embedding them are encoded in
// WinAnsi, which has no flat in it, so anything outside that set was printed
// as a question mark: "Clarinet in B? 1". The name of a transposing
// instrument is the one place a flat is certain to appear.

func testRenderer(t *testing.T) *renderer {
	t.Helper()
	doc := pdf.New()
	music, err := doc.Embed("Bravura", smufl.FontData())
	if err != nil {
		t.Fatal(err)
	}
	r := &renderer{doc: doc, opt: DefaultOptions(), s: 1, music: music}
	r.text = doc.Standard(pdf.Times)
	r.page = doc.NewPage(pdf.Point(r.opt.PageWidth), pdf.Point(r.opt.PageHeight))
	return r
}

func TestAFlatInAWordIsSetFromTheMusicFont(t *testing.T) {
	r := testRenderer(t)
	runs := r.split("Clarinet in B♭ 1", r.text, 10)

	if len(runs) != 3 {
		t.Fatalf("the name was cut into %d runs, want three: the words, the "+
			"sign, and the rest", len(runs))
	}
	if runs[0].f != r.text || runs[2].f != r.text {
		t.Error("the words are not set in the reading face")
	}
	if runs[1].f != r.music {
		t.Error("the flat is not set in the music face, so it will print as " +
			"a question mark")
	}
	want, _ := smufl.Glyph("accidentalFlat")
	if runs[1].s != string(want) {
		t.Errorf("the sign is %q, want the flat glyph", runs[1].s)
	}
	if runs[0].s != "Clarinet in B" || runs[2].s != " 1" {
		t.Errorf("the words came out as %q and %q", runs[0].s, runs[2].s)
	}
}

// Sharps and naturals too, and a word with none of them is left as one run —
// which is nearly every word, and should cost nothing.
func TestOnlyTheSignsAreCutOut(t *testing.T) {
	r := testRenderer(t)
	for _, c := range []struct {
		in   string
		runs int
	}{
		{"Trumpet in B♭ 2", 3},
		{"Horn in E♭", 2},
		{"C♯ minor", 3},
		{"B♮", 2},
		{"Flute 1", 1},
		{"", 0},
	} {
		if n := len(r.split(c.in, r.text, 10)); n != c.runs {
			t.Errorf("%q was cut into %d runs, want %d", c.in, n, c.runs)
		}
	}
}

// The width has to count the sign at its own size and in its own font, or a
// centred title with a flat in it is centred on the wrong number.
func TestTheSignCountsTowardsTheWidth(t *testing.T) {
	r := testRenderer(t)
	plain := r.width(r.split("Horn in E", r.text, 10))
	withFlat := r.width(r.split("Horn in E♭", r.text, 10))
	if withFlat <= plain {
		t.Errorf("a name with a flat measures %.2f and one without %.2f: the "+
			"sign was not counted", withFlat, plain)
	}
}

// Punctuation a word processor puts into words.
//
// The reading faces are encoded in WinAnsi, and what it has not got it
// prints as a question mark. A lyric typed anywhere but a plain text editor
// carries a curly apostrophe, so "ev-'ry dream gone" came out of a song
// sheet as "ev-?ry dream gone" — a question mark in the middle of a word,
// which is the one thing on a page nobody can read past.
func TestTypographicPunctuationIsSetAsPlainPunctuation(t *testing.T) {
	for _, tc := range []struct {
		in   rune
		want rune
		what string
	}{
		{'’', '\'', "a curly apostrophe"},
		{'‘', '\'', "an opening curly quote"},
		{'“', '"', "an opening double quote"},
		{'”', '"', "a closing double quote"},
		{'–', '-', "an en dash"},
		{'—', '-', "an em dash"},
		{' ', ' ', "a non-breaking space"},
		// What the faces can already set is left exactly as it is.
		{'a', 'a', "a letter"},
		{'\'', '\'', "a plain apostrophe"},
		{'-', '-', "a hyphen"},
		// And a character with no plain equivalent is not guessed at: that
		// is a font's job, not a substitution's.
		{'д', 'д', "a letter of another alphabet"},
		{'♭', '♭', "a flat, which is set from the music face"},
	} {
		if got := plainly(tc.in); got != tc.want {
			t.Errorf("%s (%q) is set as %q, want %q", tc.what, tc.in, got, tc.want)
		}
	}
}
