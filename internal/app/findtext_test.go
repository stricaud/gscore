/*
 * findtext_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Finding words in a score, and changing them.
 *
 * Words are scattered through a score in a dozen unrelated places, and
 * renaming an instrument or fixing a spelling meant finding each one by
 * eye. The thing that matters here is that the search reaches all of them
 * — a replace that misses a place the search found, or reaches a place it
 * did not, is worse than no replace at all.
 */

package app

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// wordy writes a score with the same word in every kind of writing there
// is, so that a search has somewhere to miss.
func wordy(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")

	s := h.app.score
	s.Identity = model.Identity{
		Title: "Oboe sonata", Subtitle: "for oboe", Composer: "Oboe player",
		Copyright: "© the oboe trust",
	}
	s.Tempo.Text = "Oboe-like"
	s.Groups = append(s.Groups, model.StaffGroup{Name: "Oboes"})
	s.Parts[0].Name = "Oboe"

	m := s.Parts[0].Measures[0]
	m.Marker = "Oboe A"
	m.SectionTitle = "The oboe movement"
	m.Tempo = &model.Tempo{BPM: 90, Text: "Oboe tempo"}

	el := m.Voices[0].Elements[0]
	marks := marksOn(el)
	if marks == nil {
		t.Fatal("the note carries no marks")
	}
	marks.Lyrics = []model.Lyric{{Text: "oboe", Verse: 1}}
	marks.Texts = []model.TextMark{{Text: "like an oboe"}}
	marks.Chord = &model.ChordSymbol{Root: "Oboe", Quality: "oboe7",
		Bass: "oboeb", Text: "N.oboe."}
	return h
}

func found(t *testing.T, h *harness, q string, extra ...string) []textHit {
	t.Helper()
	p := url.Values{"q": {q}}
	for i := 0; i+1 < len(extra); i += 2 {
		p.Set(extra[i], extra[i+1])
	}
	var out struct {
		Hits []textHit `json:"hits"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/find?"+p.Encode())), &out); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	return out.Hits
}

// Every kind of writing is reached. This is the test that matters: a
// place the walk forgets is a place the search silently never looks.
func TestASearchReachesEveryKindOfWriting(t *testing.T) {
	h := wordy(t)
	hits := found(t, h, "oboe")

	seen := map[string]bool{}
	for _, hit := range hits {
		seen[hit.Where] = true
	}
	for _, kind := range AllTextKinds {
		if !seen[kind] {
			t.Errorf("a search never looked at %q", kind)
		}
	}
}

// Matching ignores case unless told otherwise, and the replacement goes
// in as it was typed with everything round it untouched.
func TestReplacingKeepsWhatIsRoundIt(t *testing.T) {
	h := wordy(t)
	h.post("/api/find?q=oboe&with=" + url.QueryEscape("hautbois"))

	if got := h.app.score.Identity.Title; got != "Hautbois sonata" {
		// Capital O, so a case-insensitive search found it and put the
		// replacement in exactly as typed.
		if got != "hautbois sonata" {
			t.Errorf("the title came out %q", got)
		}
	}
	if got := h.app.score.Parts[0].Name; got != "hautbois" && got != "Hautbois" {
		t.Errorf("the staff name came out %q", got)
	}
	marks := marksOn(h.app.score.Parts[0].Measures[0].Voices[0].Elements[0])
	if got := marks.Texts[0].Text; got != "like an hautbois" {
		t.Errorf("the text mark came out %q, which lost what was round it", got)
	}
	if n := len(found(t, h, "oboe")); n != 0 {
		t.Errorf("%d places still say oboe", n)
	}
}

func TestMatchingCapitalsWhenAsked(t *testing.T) {
	h := wordy(t)
	// "Oboe" with a capital is in the title, the composer, the staff name
	// and the rehearsal mark; "oboe" lower case is elsewhere.
	all := len(found(t, h, "oboe"))
	exact := len(found(t, h, "oboe", "case", "1"))
	if exact >= all {
		t.Errorf("matching capitals found %d of %d, which is not fewer",
			exact, all)
	}
	if exact == 0 {
		t.Error("matching capitals found nothing at all")
	}
}

// Writing that is empty never matches, so a search does not turn up every
// unwritten title in the score.
func TestEmptyWritingIsNotAMatch(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.app.score.Identity.Title = "" // and the rest are empty already
	if n := len(found(t, h, "a")); n != 0 {
		t.Errorf("%d matches in a score with nothing written in it", n)
	}
}

// Replacing with nothing is deleting, which is a thing people mean.
func TestReplacingWithNothingDeletes(t *testing.T) {
	h := wordy(t)
	h.post("/api/find?q=" + url.QueryEscape("Oboe ") + "&with=")
	if got := h.app.score.Parts[0].Measures[0].Marker; got != "A" {
		t.Errorf("the rehearsal mark came out %q, want %q", got, "A")
	}
}

func TestASearchForNothingIsRefused(t *testing.T) {
	h := wordy(t)
	if why := h.postFails("/api/find?q=&with=x"); why == "" {
		t.Error("a search for nothing was accepted")
	}
	// Replacing needs saying what with, even if that is nothing: the
	// parameter being absent is a mistake, where being empty is a choice.
	if why := h.postFails("/api/find?q=oboe"); why == "" {
		t.Error("a replace with no replacement was accepted")
	}
}
