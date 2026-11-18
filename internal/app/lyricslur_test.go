/*
 * lyricslur_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A held syllable is sung through its slur.
 *
 * "'cause you're mine._ Ba" over E F A, a slur from the A to an F, and
 * then a C: the underscore says "mine." is held, and the slur says how far
 * — over the F. "Ba" is the next thing to sing, and the next thing to sing
 * it on is the C. It was put on the F, under the curve. Reported as "it
 * wrote the Ba on the wrong note, _ carries with the tie/slur".
 */

package app

import (
	"net/url"
	"testing"
)

// slurredPhrase writes E F A F | C, with a slur from the A to the F.
func slurredPhrase(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	for _, step := range []int{2, 3, 5, 3} { // E F A F
		h.post("/api/insert?part=0&measure=0&at=-1&step=" +
			string(rune('0'+step)) + "&dur=quarter&kind=note")
	}
	h.post("/api/insert?part=0&measure=1&at=-1&step=0&dur=whole&kind=note")
	v := h.app.score.Parts[0].Measures[0].Voices[0]
	marksOfElement(v.Elements[2]).SlurStart = 1
	marksOfElement(v.Elements[3]).SlurStop = 1
	return h
}

// wordsOnNotes is the syllable on each sung note of the first staff, in
// order, "" where there is none and "_" for a bare extender.
func wordsOnNotes(h *harness) []string {
	var out []string
	for _, mk := range singableNotes(h.app.score.Parts[0]) {
		if len(mk.Lyrics) == 0 {
			out = append(out, "")
			continue
		}
		ly := mk.Lyrics[0]
		if ly.Text == "" && ly.Extend {
			out = append(out, "_")
		} else {
			out = append(out, ly.Text)
		}
	}
	return out
}

func TestAHeldSyllableIsSungThroughItsSlur(t *testing.T) {
	h := slurredPhrase(t)
	h.post("/api/lyrics?part=0&verse=1&text=" +
		url.QueryEscape("'cause you're mine._ Ba"))
	got := wordsOnNotes(h)
	want := []string{"'cause", "you're", "mine.", "", "Ba"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("the words fall %q, want %q", got, want)
		}
	}
}

// A "_" standing on its own still names the slurred note: that is how a
// melisma over separate notes is written, and it must land there.
func TestABareExtenderStillTakesTheSlurredNote(t *testing.T) {
	h := slurredPhrase(t)
	h.post("/api/lyrics?part=0&verse=1&text=" +
		url.QueryEscape("'cause you're mine. _ Ba"))
	got := wordsOnNotes(h)
	want := []string{"'cause", "you're", "mine.", "_", "Ba"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("the words fall %q, want %q", got, want)
		}
	}
}

// Without the underscore the slur changes nothing: the next word goes on
// the next note, curve or no curve.
func TestWithoutAnUnderscoreTheSlurIsNotAMelisma(t *testing.T) {
	h := slurredPhrase(t)
	h.post("/api/lyrics?part=0&verse=1&text=" +
		url.QueryEscape("'cause you're mine. Ba doo"))
	got := wordsOnNotes(h)
	want := []string{"'cause", "you're", "mine.", "Ba", "doo"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("the words fall %q, want %q", got, want)
		}
	}
}
