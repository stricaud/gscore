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
	"encoding/json"
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

// A slur can end on the far side of a tie, which is not a note a word can be
// sung on. The hold still reaches exactly as far as the slur: "do;_" on an
// F sharp slurred to an A tied over, and "I" goes on the note after the tie.
// It went two bars late — the end of the slur was hunted for among the
// notes that take words, never found there, and the words ran on to the end
// of some other slur, coming back with a <2> in them nobody typed.
func slurOverATie(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	// Bar 1: E (you) F# (do;) A A, the F# slurred to the second A, the
	// two As tied. Bar 2: C# C# E C#. Bar 3: B A.
	for _, step := range []int{2, 3, 5, 5} {
		h.post("/api/insert?part=0&measure=0&at=-1&step=" +
			string(rune('0'+step)) + "&dur=quarter&kind=note")
	}
	for _, step := range []int{0, 0, 2, 0} {
		h.post("/api/insert?part=0&measure=1&at=-1&step=" +
			string(rune('0'+step)) + "&dur=quarter&kind=note")
	}
	for _, step := range []int{6, 5} {
		h.post("/api/insert?part=0&measure=2&at=-1&step=" +
			string(rune('0'+step)) + "&dur=half&kind=note")
	}
	v := h.app.score.Parts[0].Measures[0].Voices[0]
	marksOfElement(v.Elements[1]).SlurStart = 1
	marksOfElement(v.Elements[2]).TieStart = true
	marksOfElement(v.Elements[3]).TieStop = true
	marksOfElement(v.Elements[3]).SlurStop = 1
	return h
}

func TestAHoldReachesOverASlurThatEndsOnATiedNote(t *testing.T) {
	h := slurOverATie(t)
	typed := "you do;_ I ain't ly-in'. No, I"
	h.post("/api/lyrics?part=0&verse=1&text=" + url.QueryEscape(typed))
	got := wordsOnNotes(h)
	want := []string{"you", "do;", "", "I", "ain't", "ly", "in'.", "No,", "I"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("the words fall %q, want %q", got, want)
		}
	}
	// And they come back as they were typed, with no bars to jump.
	var d map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/getlyrics?part=0&verse=1")), &d); err != nil {
		t.Fatal(err)
	}
	if d["text"] != typed {
		t.Errorf("the verse is read back as %q, want %q", d["text"], typed)
	}
}

// A melisma that covers a whole bar: the bar is crossed by the hold, not
// jumped, so it is not written back as a marker — which, typed in again,
// would jump it on top of the hold and move every word after it.
func TestABarSungUnderASlurIsNotWrittenBackAsAJump(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	for m := 0; m < 4; m++ {
		for _, step := range []int{0, 2} {
			h.post("/api/insert?part=0&measure=" + string(rune('0'+m)) +
				"&at=-1&step=" + string(rune('0'+step)) + "&dur=half&kind=note")
		}
	}
	// A slur from the last note of bar 1 across bar 2 to the first of bar 3.
	marksOfElement(h.app.score.Parts[0].Measures[0].Voices[0].Elements[1]).SlurStart = 1
	marksOfElement(h.app.score.Parts[0].Measures[2].Voices[0].Elements[0]).SlurStop = 1
	typed := "A men_ so be"
	want := []string{"A", "men", "", "", "", "so", "be"}
	for round := 0; round < 2; round++ {
		h.post("/api/lyrics?part=0&verse=1&text=" + url.QueryEscape(typed))
		got := wordsOnNotes(h)
		for i := range want {
			if i >= len(got) || got[i] != want[i] {
				t.Fatalf("round %d: the words fall %q, want %q", round+1, got, want)
			}
		}
		var d map[string]any
		if err := json.Unmarshal([]byte(h.get("/api/getlyrics?part=0&verse=1")), &d); err != nil {
			t.Fatal(err)
		}
		if d["text"] != typed {
			t.Fatalf("round %d: the verse is read back as %q, want %q", round+1, d["text"], typed)
		}
		typed, _ = d["text"].(string)
	}
}

// A slur nobody closed holds nothing: the next word goes on the next note,
// rather than every word after it being carried off the end of the staff.
func TestAnUnclosedSlurHoldsNothing(t *testing.T) {
	h := slurredPhrase(t)
	marksOfElement(h.app.score.Parts[0].Measures[0].Voices[0].Elements[3]).SlurStop = 0
	h.post("/api/lyrics?part=0&verse=1&text=" +
		url.QueryEscape("'cause you're mine._ Ba doo"))
	got := wordsOnNotes(h)
	want := []string{"'cause", "you're", "mine.", "Ba", "doo"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("the words fall %q, want %q", got, want)
		}
	}
}
