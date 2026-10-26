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

// sung builds one bar of eighth notes with a syllable under each.
func sung(words ...string) *model.Score {
	var els []model.Element
	for _, w := range words {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		if w != "" {
			n.Lyrics = []model.Lyric{{Text: w, Verse: 1}}
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
