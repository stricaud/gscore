/*
 * lyrics_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

import (
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The words, laid under the notes.
 *
 * w: was listed among the fields that "do not bear on the notes" and
 * dropped, which is a strange thing to say about the words. The model has
 * carried Lyric since it was written, the engraver draws the syllables and
 * even widens a bar to fit them, and every other reader fills them in. In
 * the ABC on this machine alone, eight hundred and eleven lines of words
 * went nowhere.
 *
 * What these ask is the counting: ABC says which syllable belongs to which
 * note by running them along in order, with a few signs for the places
 * where words and notes do not march in step.
 */

// sungOn is the syllables of one verse, note by note, with "" for a note
// nobody sings on.
func sungOn(s *model.Score, verse int) []string {
	var out []string
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					var mk *model.Marks
					switch e := el.(type) {
					case *model.Note:
						mk = &e.Marks
					case *model.Chord:
						mk = &e.Marks
					default:
						continue // a rest is not sung on
					}
					word := ""
					for _, ly := range mk.Lyrics {
						if ly.Verse == verse {
							word = ly.Text
							if ly.NeedsHyphen() {
								word += "-"
							}
							if ly.Extend {
								word += "_"
							}
						}
					}
					out = append(out, word)
				}
			}
		}
	}
	return out
}

func same(t *testing.T, got, want []string, what string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s:\n got %v\nwant %v", what, got, want)
	}
}

func TestWordsRunAlongTheNotes(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|\nw: one two three four\n")
	same(t, sungOn(s, 1), []string{"one", "two", "three", "four"}, "one word a note")
}

func TestAHyphenCarriesAWordOn(t *testing.T) {
	// "Twin-kle" is one word on two notes, and the hyphen between them is
	// drawn. The middle of a three-syllable word is neither its start nor
	// its end, and the engraver needs to know which.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|\nw: Twin-kle lit-tle\n")
	same(t, sungOn(s, 1), []string{"Twin-", "kle", "lit-", "tle"}, "hyphenated words")

	notes := notesOf(s)
	if got := notes[0].Lyrics[0].Syllabic; got != model.SyllableBegin {
		t.Errorf("Twin is %v, want the start of a word", got)
	}
	if got := notes[1].Lyrics[0].Syllabic; got != model.SyllableEnd {
		t.Errorf("kle is %v, want the end of a word", got)
	}

	three := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDE F|\nw: won-der-ful day\n")
	mid := notesOf(three)[1].Lyrics[0].Syllabic
	if mid != model.SyllableMiddle {
		t.Errorf("der is %v, want the middle of a word", mid)
	}
}

func TestAnUnderscoreHoldsTheSyllable(t *testing.T) {
	// The syllable before is sung on this note too, with the long line
	// under the notes it covers.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|\nw: one two_ _ four\n")
	same(t, sungOn(s, 1), []string{"one", "two_", "", ""}, "a held syllable")
	if !notesOf(s)[1].Lyrics[0].Extend {
		t.Error("the held syllable does not draw its line")
	}
}

func TestAStarSkipsANote(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|\nw: one * three four\n")
	same(t, sungOn(s, 1), []string{"one", "", "three", "four"}, "a skipped note")
}

func TestABarMovesTheWordsOn(t *testing.T) {
	// The sign that rescues a line whose words have slipped: "|" says the
	// next syllable belongs to the first note of the next bar.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|GABc|\nw: one two | five six\n")
	same(t, sungOn(s, 1),
		[]string{"one", "two", "", "", "five", "six", "", ""}, "words moved to the bar")
}

func TestATildeIsOneSyllableOfTwoWords(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCD|\nw: a~new day\n")
	same(t, sungOn(s, 1), []string{"a new", "day"}, "two words on one note")
}

func TestAnEscapedHyphenStaysInTheWord(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCD|\nw: well\\-known word\n")
	same(t, sungOn(s, 1), []string{"well-known", "word"}, "a hyphen in the word")
}

func TestSeveralLinesOfWordsAreVerses(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCDEF|\n"+
		"w: one two three four\nw: un deux trois quatre\n")
	same(t, sungOn(s, 1), []string{"one", "two", "three", "four"}, "verse 1")
	same(t, sungOn(s, 2), []string{"un", "deux", "trois", "quatre"}, "verse 2")
}

func TestWordsBelongToTheLineAboveThem(t *testing.T) {
	// Each line of music starts the count again. Carried on from the line
	// before, the second verse would be laid under the first line's notes.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCD|\nw: one two\nEF|\nw: three four\n")
	same(t, sungOn(s, 1), []string{"one", "two", "three", "four"}, "two lines of words")
}

func TestWordsCarryThroughAContinuedLine(t *testing.T) {
	// A line broken with "\" is one line of music, so the words under it
	// are laid against all of its notes.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCD\\\nEF|\nw: one two three four\n")
	same(t, sungOn(s, 1), []string{"one", "two", "three", "four"}, "a continued line")
}

func TestRestsAreNotSungOn(t *testing.T) {
	// A syllable goes to a note. Counting the rest would put every word
	// after it on the wrong note.
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCz DE|\nw: one two three\n")
	same(t, sungOn(s, 1), []string{"one", "two", "three"}, "a rest in the middle")
}

func TestMoreWordsThanNotesDoesNotPileUp(t *testing.T) {
	s := importOne(t, "X:1\nT:t\nM:4/4\nL:1/4\nK:C\nCD|\nw: one two three four\n")
	same(t, sungOn(s, 1), []string{"one", "two"}, "words past the end of the line")
}
