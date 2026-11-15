/*
 * lyricspace_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A sung bar is spaced by its words where they are wider than its notes.
//
// Eight eighth notes take as much room as eight eighth notes, and "Our
// lit-tle dream cas-tle with ev-'ry dream gone" takes three times that. Set
// by the notes alone the syllables were printed on top of one another — one
// black word across the system, which is what "the lyrics are overlapping"
// looks like.

// sung builds one bar of eighth notes with a syllable under each. A word
// written with a trailing "-" is half of one, and carries a hyphen to the
// note after it.
func sung(words ...string) *model.Score {
	var els []model.Element
	for _, w := range words {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		if w != "" {
			kind := model.SyllableSingle
			if strings.HasSuffix(w, "-") {
				w, kind = strings.TrimSuffix(w, "-"), model.SyllableBegin
			}
			n.Lyrics = []model.Lyric{{Text: w, Verse: 1, Syllabic: kind}}
		}
		els = append(els, n)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: len(words), BeatValue: 8},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
}

// syllables are the words drawn, with where each one sits.
func syllables(lay *Layout) []Text {
	var out []Text
	for _, t := range lay.Texts {
		if t.Anchor == "middle" && t.Size == lyricSize {
			out = append(out, t)
		}
	}
	return out
}

func TestWordsDoNotOverlapEachOther(t *testing.T) {
	// The line that was reported, syllable for syllable.
	score := sung("Our", "lit", "tle", "dream", "cas", "tle", "with", "ev")
	lay := Engrave(score, DefaultOptions())

	words := syllables(lay)
	if len(words) != 8 {
		t.Fatalf("%d syllables drawn, want 8", len(words))
	}
	for i := 1; i < len(words); i++ {
		prev, here := words[i-1], words[i]
		// Each is centred on its notehead, so they clash when the halves
		// of two neighbours reach into each other.
		gap := (here.X - textWidth(here.S, lyricSize)/2) -
			(prev.X + textWidth(prev.S, lyricSize)/2)
		if gap < 0 {
			t.Errorf("%q and %q overlap by %.2f spaces", prev.S, here.S, -gap)
		}
	}
}

// And a bar with no words in it is spaced exactly as it was.
func TestWordsOnlyWidenTheBarsThatHaveThem(t *testing.T) {
	plain := sung("", "", "", "")
	words := sung("extraordinarily", "", "", "")

	a := Engrave(plain, DefaultOptions())
	b := Engrave(words, DefaultOptions())
	if b.Width <= a.Width {
		t.Errorf("a bar of long words is %.2f wide and a bar of none %.2f; "+
			"the words asked for no room", b.Width, a.Width)
	}
}

// A squeezed system is a tighter setting of the music, not an unreadable
// one: the words are in the minimum width as well as the ideal.
func TestWordsAreInTheMinimumWidthToo(t *testing.T) {
	score := sung("Our", "lit", "tle", "dream", "cas", "tle", "with", "ev")
	opt := DefaultOptions()
	// A page too narrow for the bar, so the system is squeezed to its
	// minimum and the words have to survive it.
	opt.LineWidth = 60
	lay := Engrave(score, opt)

	words := syllables(lay)
	for i := 1; i < len(words); i++ {
		prev, here := words[i-1], words[i]
		if here.X < prev.X {
			continue // a new system: nothing to compare
		}
		gap := (here.X - textWidth(here.S, lyricSize)/2) -
			(prev.X + textWidth(prev.S, lyricSize)/2)
		if gap < 0 {
			t.Errorf("squeezed, %q and %q overlap by %.2f spaces", prev.S, here.S, -gap)
		}
	}
}

/*
 * The hyphen between the halves of a word.
 *
 * It belongs in the gap between the two syllables, with air on both sides,
 * so that it reads as joining them rather than as part of either. It used
 * to be put at a fixed fraction of the column's width, which knows how far
 * away the next note is and nothing about the words: a syllable is centred
 * on its notehead, so a long one reaches further from the same place, and
 * "dusk-" had its hyphen printed over the k. Reported, from the engraved
 * page, as "the - is overlapping the char before".
 */

// hyphens are the hyphens drawn between syllables, left to right.
func hyphens(lay *Layout) []Text {
	var out []Text
	for _, t := range lay.Texts {
		if t.S == "-" && t.Size == lyricSize {
			out = append(out, t)
		}
	}
	return out
}

// The words of the snippet this was reported from, with the long syllable
// that showed it up.
func TestAHyphenStandsBetweenTheHalvesOfAWord(t *testing.T) {
	score := sung("Folks", "in", "dusk-", "y", "Hah-", "lum", "sol-", "emn")
	lay := Engrave(score, DefaultOptions())

	// The words, which are everything drawn down there that is not one of
	// the hyphens being measured.
	var words []Text
	for _, t := range syllables(lay) {
		if t.S != "-" {
			words = append(words, t)
		}
	}
	marks := hyphens(lay)
	if len(marks) != 3 {
		t.Fatalf("%d hyphens drawn, want 3", len(marks))
	}
	// Each hyphen against the words either side of it.
	for _, h := range marks {
		var before, after *Text
		for i := range words {
			if words[i].X < h.X {
				before = &words[i]
			} else if after == nil {
				after = &words[i]
			}
		}
		if before == nil || after == nil {
			t.Fatalf("a hyphen at %.2f with no word on both sides", h.X)
		}
		half := textWidth("-", lyricSize) / 2
		left := (h.X - half) -
			(before.X + textWidth(before.S, lyricSize)/2)
		right := (after.X - textWidth(after.S, lyricSize)/2) - (h.X + half)
		if left < 0 {
			t.Errorf("the hyphen after %q is printed over it by %.2f spaces",
				before.S, -left)
		}
		if right < 0 {
			t.Errorf("the hyphen before %q is printed over it by %.2f spaces",
				after.S, -right)
		}
		// And between them rather than hard against one of them: the air
		// on the two sides is within a hair of being the same.
		if d := left - right; d > 0.2 || d < -0.2 {
			t.Errorf("the hyphen between %q and %q has %.2f of air on one "+
				"side and %.2f on the other", before.S, after.S, left, right)
		}
	}
}

// The room for it is booked when the bar is spaced, or there is nowhere to
// put it that is not on top of a word.
func TestAHyphenIsGivenRoomToStandIn(t *testing.T) {
	plain := Engrave(sung("dusk", "y"), DefaultOptions())
	split := Engrave(sung("dusk-", "y"), DefaultOptions())
	if split.Width <= plain.Width {
		t.Errorf("a hyphenated pair is %.2f wide and a plain one %.2f; "+
			"the hyphen asked for no room", split.Width, plain.Width)
	}
}
