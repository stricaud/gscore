/*
 * readingface_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The face the words are drawn in is the face they were measured in.
 *
 * The engraver reserves the room a word needs and puts the next thing after
 * it, reading the widths from a table of Times metrics. The drawing asked
 * for Georgia, which is about a fifth wider in the lower case, so the next
 * thing landed inside the word: a metronome mark was drawn with its note
 * over the last letter of "Molto moderato".
 */

package svg

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
)

// theTextRule is the stylesheet rule the words on a score are set by.
func theTextRule(t *testing.T) string {
	t.Helper()
	out := render(t, &engrave.Layout{
		Width: 20, Height: 20,
		Texts: []engrave.Text{{S: "Molto moderato", X: 1, Y: 5, Size: 2}},
	})
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ".t{") {
			return line
		}
	}
	t.Fatal("the drawing has no rule for its words")
	return ""
}

func TestTheWordsAreSetInTheFaceTheyWereMeasuredIn(t *testing.T) {
	rule := theTextRule(t)
	// Times New Roman is drawn to the metrics the engraver measures with,
	// and Liberation Serif is drawn to Times New Roman's, for a machine
	// that has not got it.
	for _, want := range []string{`"Times New Roman"`, `Liberation Serif`} {
		if !strings.Contains(rule, want) {
			t.Errorf("the words are set by %q, which does not offer %s",
				rule, want)
		}
	}
	// Georgia is a fifth wider in the lower case than the table the
	// engraver measures from, which is the width of a letter or two over a
	// word of any length.
	if strings.Contains(rule, "Georgia") {
		t.Errorf("the words are set by %q, and Georgia is not the face they "+
			"were measured in", rule)
	}
}

// Whatever the face is asked for first, a serif is asked for last: a machine
// with none of the named faces should not fall back to the browser's
// sans-serif for the title of a score.
func TestTheLastResortIsStillASerif(t *testing.T) {
	if rule := theTextRule(t); !strings.Contains(rule, "serif;") {
		t.Errorf("the words are set by %q, which ends in no serif", rule)
	}
}
