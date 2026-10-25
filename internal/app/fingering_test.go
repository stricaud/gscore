/*
 * fingering_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"strings"
	"testing"
)

// Typing "Dm" and then being asked for the fingering is asking somebody to
// look up what they have just said.
//
// These are the shapes a guitarist reaches for first. They are not the only
// way to play any of them — which is why the field they fill in can be typed
// over — but a wrong one would be worse than none, so the common ones are
// checked against what a player would actually hold.

func TestTheUsualChordsAreKnown(t *testing.T) {
	for _, c := range []struct{ root, quality, want string }{
		// First position: what a beginner learns, and what nothing regular
		// would predict — C is x32010 and no shape says so.
		{"C", "", "x32010"},
		{"A", "m", "x02210"},
		{"G", "", "320003"},
		{"E", "m", "022000"},
		{"D", "7", "xx0212"},
		{"D", "m", "xx0231"},
		{"C", "7", "x32310"},
		{"A", "7", "x02020"},
		{"E", "", "022100"},

		// Barre chords: the same shapes slid up to the root.
		{"F", "", "133211"},      // E shape at the first fret
		{"B", "b", ""},           // not a quality; nothing to say
		{"Bb", "", "x13331"},     // A shape at the first
		{"B", "m", "x24432"},     // A shape, minor, at the second
		{"F#", "m", "244222"},    // E shape, minor, at the second
		{"Ab", "maj7", "465544"}, // E shape, major seventh, at the fourth
		{"C", "m", "x35543"},     // A shape, minor, at the third
		{"C", "m7", "x35343"},    // and the minor seventh
		{"Eb", "", "x68886"},     // past the fifth fret, still one shape

		// The spellings people actually type.
		{"C", "min", "x35543"},
		{"C", "-", "x35543"},
		{"C", "M7", "x32000"},
		{"C", "sus4", "x35563"},
		{"C", "5", "x35xxx"},

		// A chord with more in it than a shape covers is given the shape it
		// is built on, which is where a hand starts anyway.
		{"C", "7#9", "x32310"},
		{"D", "m11", "xx0211"},

		// And something that is not a chord at all gets no answer.
		{"", "", ""},
		{"H", "", ""},
		{"N.C.", "", ""},
	} {
		got := guitarFingering(c.root, c.quality)
		if got != c.want {
			t.Errorf("%s%s came out %q, want %q", c.root, c.quality, got, c.want)
		}
	}
}

// Every root and every quality gets an answer, or the field is empty for
// chords somebody certainly writes.
func TestEveryRootHasAShape(t *testing.T) {
	roots := []string{"C", "C#", "Db", "D", "D#", "Eb", "E", "F", "F#",
		"Gb", "G", "G#", "Ab", "A", "A#", "Bb", "B"}
	qualities := []string{"", "m", "7", "m7", "maj7", "6", "sus4", "sus2", "5"}
	for _, r := range roots {
		for _, q := range qualities {
			got := guitarFingering(r, q)
			if got == "" {
				t.Errorf("%s%s has no shape", r, q)
				continue
			}
			// Six strings, whichever way it is spelled.
			n := len(strings.Fields(got))
			if n <= 1 {
				n = len([]rune(got))
			}
			if n != 6 {
				t.Errorf("%s%s came out %q, which is %d strings", r, q, got, n)
			}
		}
	}
}

// And the picker can ask for it the way the dialog does.
func TestThePickerAnswersWithAFingering(t *testing.T) {
	h := newHarness(t)
	got := h.get("/api/chordfingering?chord=Dm")
	if !strings.Contains(got, `"frets":"xx0231"`) {
		t.Errorf("the picker answered %s", strings.TrimSpace(got))
	}
	if !strings.Contains(got, `"found":true`) {
		t.Error("it did not say it had found one")
	}

	none := h.get("/api/chordfingering?chord=" + "N.C.")
	if !strings.Contains(none, `"found":false`) {
		t.Errorf("it claimed to know a fingering for N.C.: %s",
			strings.TrimSpace(none))
	}
}

// M7 is a major seventh and m7 is a minor one. The case is the only thing
// that says which, so it is settled before the name is lowered — and this
// is the test that found it being lowered first.
func TestCapitalMIsAMajorSeventh(t *testing.T) {
	for _, c := range []struct{ quality, want string }{
		{"M7", "x32000"}, // C major seventh, open
		{"Maj7", "x32000"},
		{"maj7", "x32000"},
		{"m7", "x35343"}, // C minor seventh, a barre at the third
		{"min7", "x35343"},
		{"M", "x32010"}, // and a capital M alone is a plain major
		{"maj", "x32010"},
	} {
		if got := guitarFingering("C", c.quality); got != c.want {
			t.Errorf("C%s came out %q, want %q", c.quality, got, c.want)
		}
	}
}
