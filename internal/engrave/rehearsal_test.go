/*
 * rehearsal_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The rehearsal letter, said again further down a large score.
 *
 * A cellist at the foot of a twenty-stave page cannot read a letter
 * eighteen staves up, and a conductor calling "from letter M" is talking to
 * the whole room. Printed scores repeat the mark above each family for that
 * reason. A song and a piano piece do not need it and must not get it.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// markedScore is staves staves of one marked bar, grouped as given.
func markedScore(staves int, mark string, groups []model.StaffGroup) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
				}}},
				RightBarline: model.BarSingle,
				Marker:       mark,
			}},
		}
		s.Parts = append(s.Parts, p)
	}
	s.Groups = groups
	s.Normalise()
	return s
}

// lettersDrawn counts how many times a mark's text was written.
func lettersDrawn(lay *Layout, text string) int {
	n := 0
	for _, t := range lay.Texts {
		if t.S == text {
			n++
		}
	}
	return n
}

func TestASmallScoreSaysTheLetterOnce(t *testing.T) {
	// No groups at all: a song, a lead sheet.
	lay := Engrave(markedScore(1, "A", nil), DefaultOptions())
	if got := lettersDrawn(lay, "A"); got != 1 {
		t.Errorf("the letter was written %d times", got)
	}
}

// A pianist reads two staves and does not need the letter twice. A brace
// is not a bracket.
func TestABracedPairSaysItOnce(t *testing.T) {
	lay := Engrave(markedScore(2, "A", []model.StaffGroup{
		{First: 0, Last: 1, Symbol: model.GroupBrace, Barlines: true},
	}), DefaultOptions())
	if got := lettersDrawn(lay, "A"); got != 1 {
		t.Errorf("the letter was written %d times over a grand staff", got)
	}
}

// A bracket round the whole score repeats nothing: the mark is already at
// the top of it.
func TestABracketOverEverythingSaysItOnce(t *testing.T) {
	lay := Engrave(markedScore(4, "A", []model.StaffGroup{
		{First: 0, Last: 3, Symbol: model.GroupBracket},
	}), DefaultOptions())
	if got := lettersDrawn(lay, "A"); got != 1 {
		t.Errorf("the letter was written %d times", got)
	}
}

// Woodwind, brass and strings: the top staff of each family says it.
func TestEachFamilySaysTheLetter(t *testing.T) {
	lay := Engrave(markedScore(9, "M", []model.StaffGroup{
		{First: 0, Last: 2, Symbol: model.GroupBracket}, // woodwind
		{First: 3, Last: 5, Symbol: model.GroupBracket}, // brass
		{First: 6, Last: 8, Symbol: model.GroupBracket}, // strings
	}), DefaultOptions())

	// Once at the top of the page, and once more for each family below it.
	if got := lettersDrawn(lay, "M"); got != 3 {
		t.Errorf("the letter was written %d times, want 3", got)
	}
}

// Each repeat is boxed like the first: it is the same mark, not a label.
func TestARepeatedLetterIsBoxedToo(t *testing.T) {
	lay := Engrave(markedScore(6, "B", []model.StaffGroup{
		{First: 0, Last: 2, Symbol: model.GroupBracket},
		{First: 3, Last: 5, Symbol: model.GroupBracket},
	}), DefaultOptions())

	var ys []float64
	for _, tx := range lay.Texts {
		if tx.S == "B" {
			ys = append(ys, tx.Y)
		}
	}
	if len(ys) != 2 {
		t.Fatalf("the letter was written %d times", len(ys))
	}
	// The second is further down the page than the first.
	if ys[1] <= ys[0] {
		t.Errorf("the repeats are at y=%.2f and y=%.2f", ys[0], ys[1])
	}
	// And they stand in one column: a row of letters down the page that
	// does not line up reads as a mistake.
	var xs []float64
	for _, tx := range lay.Texts {
		if tx.S == "B" {
			xs = append(xs, tx.X)
		}
	}
	if len(xs) == 2 && xs[0] != xs[1] {
		t.Errorf("the repeats are at x=%.2f and x=%.2f", xs[0], xs[1])
	}
	// A box round each: its top and bottom are short horizontals either
	// side of the letter. The text sits on its baseline, which is inside
	// the box rather than at the foot of it, so the window reaches a
	// little past the letter's own height in both directions.
	for _, y := range ys {
		boxed := 0
		for _, l := range lay.Lines {
			if l.Y1 == l.Y2 && l.Y1 > y-markBoxHeight-0.01 && l.Y1 < y+1.0 &&
				l.X2-l.X1 < 6 {
				boxed++
			}
		}
		if boxed < 2 {
			t.Errorf("the letter at y=%.2f has %d sides of a box", y, boxed)
		}
	}
}

// Only the letter is repeated. A jump direction is an instruction and is
// written once; four copies of "D.S. al Coda" is four instructions.
func TestOnlyTheLetterIsRepeated(t *testing.T) {
	s := markedScore(6, "A", []model.StaffGroup{
		{First: 0, Last: 2, Symbol: model.GroupBracket},
		{First: 3, Last: 5, Symbol: model.GroupBracket},
	})
	for _, p := range s.Parts {
		p.Measures[0].Sign = model.Segno
	}
	lay := Engrave(s, DefaultOptions())

	if got := lettersDrawn(lay, "A"); got != 2 {
		t.Errorf("the letter was written %d times", got)
	}
	segni := 0
	for _, g := range lay.Glyphs {
		if g.Name == "segno" {
			segni++
		}
	}
	if segni != 1 {
		t.Errorf("the segno was drawn %d times", segni)
	}
}

// And it can be turned off, for anybody who wants the older page.
func TestTheRepeatCanBeTurnedOff(t *testing.T) {
	opt := DefaultOptions()
	opt.RepeatMarkers = false
	lay := Engrave(markedScore(6, "A", []model.StaffGroup{
		{First: 0, Last: 2, Symbol: model.GroupBracket},
		{First: 3, Last: 5, Symbol: model.GroupBracket},
	}), opt)
	if got := lettersDrawn(lay, "A"); got != 1 {
		t.Errorf("the letter was written %d times with the repeat off", got)
	}
}

// A bar with no mark gets nothing, however many families there are.
func TestNoMarkMeansNoLetters(t *testing.T) {
	lay := Engrave(markedScore(6, "", []model.StaffGroup{
		{First: 0, Last: 2, Symbol: model.GroupBracket},
		{First: 3, Last: 5, Symbol: model.GroupBracket},
	}), DefaultOptions())
	for _, tx := range lay.Texts {
		if tx.S == "A" || tx.S == "B" {
			t.Errorf("a letter %q appeared from nowhere", tx.S)
		}
	}
}
