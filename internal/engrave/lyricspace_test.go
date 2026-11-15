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
	"math"
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
			kind, held := model.SyllableSingle, false
			if strings.HasSuffix(w, "_") {
				w, held = strings.TrimSuffix(w, "_"), true
			}
			if strings.HasSuffix(w, "-") {
				w, kind = strings.TrimSuffix(w, "-"), model.SyllableBegin
			}
			n.Lyrics = []model.Lyric{{
				Text: w, Verse: 1, Syllabic: kind, Extend: held,
			}}
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

/*
 * The line that carries a held syllable.
 *
 * "days; ______" in the Altes runs from the word, over the barline, across
 * the whole of the note it is tied into, and stops where the sound stops.
 * It used to be drawn with the word, inside that word's own column and no
 * further, so the line was as long as a notehead's share of a bar however
 * long the syllable was actually held. Nothing about a column knows how long
 * a note is held, which is why this is a spanner. See extendAt.
 */

// extenders are the lines ruled under the words: horizontal, and sitting a
// hair under a verse's own baseline.
func extenders(lay *Layout) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if l.Y1 != l.Y2 {
			continue
		}
		for _, t := range syllables(lay) {
			if math.Abs(l.Y1-(t.Y+extendDrop)) < 0.01 {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

// heldOverABarline is one syllable sung on a note that is tied into the next
// bar, and a bar of plain notes after it to stop against.
func heldOverABarline() *model.Score {
	bar := func(els ...model.Element) *model.Measure {
		return &model.Measure{Voices: []*model.Voice{{Elements: els}}}
	}
	whole := func() *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}
	}
	sings, held, after := whole(), whole(), whole()
	sings.Lyrics = []model.Lyric{{
		Text: "days;", Verse: 1, Syllabic: model.SyllableSingle, Extend: true,
	}}
	sings.TieStart, held.TieStop = true, true
	after.Lyrics = []model.Lyric{{
		Text: "Here's", Verse: 1, Syllabic: model.SyllableSingle,
	}}

	last := bar(after)
	last.RightBarline = model.BarFinal
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef:     model.Treble,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{bar(sings), bar(held), last},
		}},
	}
}

func TestAHeldSyllableRunsAcrossTheNoteItIsHeldOn(t *testing.T) {
	lay := Engrave(heldOverABarline(), DefaultOptions())

	lines := extenders(lay)
	if len(lines) != 1 {
		t.Fatalf("%d lines ruled under the words, want 1", len(lines))
	}
	line := lines[0]

	var word, next Text
	for _, s := range syllables(lay) {
		switch s.S {
		case "days;":
			word = s
		case "Here's":
			next = s
		}
	}
	if word.S == "" || next.S == "" {
		t.Fatal("the words are not on the page")
	}

	// Clear of the word it belongs to, at both ends.
	if left := line.X1 - (word.X + textWidth(word.S, lyricSize)/2); left <= 0 {
		t.Errorf("the line starts %.2f spaces inside %q", -left, word.S)
	}
	if right := (next.X - textWidth(next.S, lyricSize)/2) - line.X2; right <= 0 {
		t.Errorf("the line runs %.2f spaces into %q", -right, next.S)
	}

	// And over the barline: the note is tied into the bar after it, so that
	// is how long the singer is on this syllable.
	if len(lay.Barlines) == 0 {
		t.Fatal("no barlines in the layout")
	}
	first := lay.Barlines[0].X
	if line.X2 <= first {
		t.Errorf("the line stops at %.2f and the barline is at %.2f: it "+
			"ends where the bar does rather than where the sound does",
			line.X2, first)
	}
}

// And stops where the sound stops, not where the next word starts. The
// syllable is held for as long as it is sung and no longer.
func TestAHeldSyllableStopsWhereTheSoundStops(t *testing.T) {
	lay := Engrave(heldOverABarline(), DefaultOptions())
	lines := extenders(lay)
	if len(lines) != 1 {
		t.Fatalf("%d lines ruled under the words, want 1", len(lines))
	}
	if len(lay.Barlines) < 2 {
		t.Fatalf("%d barlines, want the two between three bars", len(lay.Barlines))
	}
	// The tie ends with the second bar, so the line does too.
	if second := lay.Barlines[1].X; lines[0].X2 > second {
		t.Errorf("the line runs to %.2f, past the end of the sound at %.2f",
			lines[0].X2, second)
	}
}

// A syllable held on a note of its own has only that note's column to run
// in, so the column has to be given room for it. Without it the line came
// out shorter than it was worth drawing and the hold vanished off the page
// — which is what happened to every extender in the MusicXML suite.
func TestAHeldSyllableIsGivenRoomToRunIn(t *testing.T) {
	plain := Engrave(sung("dusk", "y"), DefaultOptions())
	held := Engrave(sung("dusk_", "y"), DefaultOptions())
	if held.Width <= plain.Width {
		t.Errorf("a held syllable is %.2f wide and a plain one %.2f; "+
			"the line asked for no room", held.Width, plain.Width)
	}
	if len(extenders(held)) != 1 {
		t.Errorf("%d lines ruled under a held syllable, want 1",
			len(extenders(held)))
	}
}

/*
 * A held syllable follows its sound into another voice.
 *
 * The line was kept per line of music, and the sound it follows is not: a
 * note held across a barline can arrive written in a different voice from
 * the one it left, which is how some files are written and how Bumpty Bump
 * is written at bar 25. Kept per voice, the line found nothing in its own
 * voice to close it, so it was drawn to the margin of that system and
 * picked up again on the next — and on every system after that. Reported
 * as "why the pdf export makes the lyrics on bars 27 to 30 underlined?",
 * which is what a line running under a whole system of words looks like.
 */

// heldIntoAnotherVoice is three bars: a syllable held on a whole note in
// the first voice, tied into a bar whose music is in the second, and then a
// bar of words to stop against.
func heldIntoAnotherVoice() *model.Score {
	whole := func() *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}
	}
	sings, held := whole(), whole()
	sings.Lyrics = []model.Lyric{{
		Text: "craze:", Verse: 1, Syllabic: model.SyllableSingle, Extend: true,
	}}
	sings.TieStart, held.TieStop = true, true

	after := whole()
	after.Lyrics = []model.Lyric{{
		Text: "Grab", Verse: 1, Syllabic: model.SyllableSingle,
	}}
	last := &model.Measure{
		Voices:       []*model.Voice{{}, {Elements: []model.Element{after}}},
		RightBarline: model.BarFinal,
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{
				// Sung in the first voice.
				{Voices: []*model.Voice{{Elements: []model.Element{sings}}, {}}},
				// Held into the second.
				{Voices: []*model.Voice{{}, {Elements: []model.Element{held}}}},
				last,
			},
		}},
	}
}

func TestAHeldSyllableFollowsItsSoundIntoAnotherVoice(t *testing.T) {
	lay := Engrave(heldIntoAnotherVoice(), DefaultOptions())

	lines := extenders(lay)
	if len(lines) != 1 {
		t.Fatalf("%d lines ruled under the words, want the one", len(lines))
	}
	var next Text
	for _, s := range syllables(lay) {
		if s.S == "Grab" {
			next = s
		}
	}
	if next.S == "" {
		t.Fatal("the word it runs to is not on the page")
	}
	if right := (next.X - textWidth(next.S, lyricSize)/2) - lines[0].X2; right <= 0 {
		t.Errorf("the line runs %.2f spaces into %q: it was never stopped",
			-right, next.S)
	}
	// And stops at the end of the sound, which is the bar the tie ends in.
	if len(lay.Barlines) < 2 {
		t.Fatalf("%d barlines, want the two between three bars", len(lay.Barlines))
	}
	if lines[0].X2 > lay.Barlines[1].X {
		t.Errorf("the line runs to %.2f, past the end of the held note at %.2f",
			lines[0].X2, lay.Barlines[1].X)
	}
}
