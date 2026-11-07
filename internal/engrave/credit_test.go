/*
 * credit_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The title and the credits, which are set as well as typed.
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func credited(id model.Identity) *Layout {
	s := &model.Score{Identity: id}
	s.Parts = []*model.Part{{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{{
			Elements: []model.Element{&model.Rest{Dur: model.Duration{Num: 1, Den: 1}}},
		}}}},
	}}
	return Engrave(s, DefaultOptions())
}

func textSaying(lay *Layout, words string) (Text, bool) {
	for _, t := range lay.Texts {
		if t.S == words {
			return t, true
		}
	}
	return Text{}, false
}

// The arranger is a line of its own, under the composer.
func TestTheArrangerIsCreditedUnderTheComposer(t *testing.T) {
	lay := credited(model.Identity{
		Title:    "A piece",
		Composer: "J. S. Bach",
		Arranger: "A. Smith",
	})
	composer, ok := textSaying(lay, "J. S. Bach")
	if !ok {
		t.Fatal("the composer is not on the page")
	}
	arranger, ok := textSaying(lay, "A. Smith")
	if !ok {
		t.Fatal("the arranger is not on the page")
	}
	if arranger.Y <= composer.Y {
		t.Errorf("the arranger is at %.2f and the composer at %.2f, want it under",
			arranger.Y, composer.Y)
	}
	if arranger.Anchor != composer.Anchor {
		t.Errorf("the arranger is anchored %q and the composer %q",
			arranger.Anchor, composer.Anchor)
	}
}

// A credit set in two styles is one line of two pieces, and the line is
// anchored where it always was.
//
// One text per piece, each placed by a measurement made in the engraver,
// was the first way this was built and it was wrong: the widths here come
// from a table of Times metrics and the drawing is done in whatever face
// the renderer has, so the two halves of a title overlapped by about a
// space. The renderer knows the font; it lays the pieces out.
func TestACreditInTwoStylesIsOneLineOfTwoPieces(t *testing.T) {
	lay := credited(model.Identity{
		Title:    "A piece",
		Composer: "Music by J. S. Bach",
		ComposerRuns: []model.Span{
			{Text: "Music by "},
			{Text: "J. S. Bach", Bold: true, Colour: "#c01020"},
		},
	})
	line, ok := textSaying(lay, "Music by J. S. Bach")
	if !ok {
		var had []string
		for _, t := range lay.Texts {
			had = append(had, t.S)
		}
		t.Fatalf("the credit is not on the page; it holds %q", strings.Join(had, " | "))
	}
	if len(line.Runs) != 2 {
		t.Fatalf("the line is drawn in %d pieces, want 2", len(line.Runs))
	}
	if line.Anchor != "end" {
		t.Errorf("the line is anchored %q, want the right-hand edge", line.Anchor)
	}
	if line.Runs[0].Bold || !line.Runs[1].Bold {
		t.Errorf("the pieces are set bold %v and %v, want plain then bold",
			line.Runs[0].Bold, line.Runs[1].Bold)
	}
	if line.Runs[1].Colour != "#c01020" {
		t.Errorf("the name is printed in %q", line.Runs[1].Colour)
	}
	// The words, with the formatting dropped, are still the words: anything
	// that reads a score rather than drawing it needs to know none of this.
	if line.S != "Music by J. S. Bach" {
		t.Errorf("the line says %q", line.S)
	}
}

// Formatting that does not spell the words it formats is not used.
func TestRunsThatDoNotSpellTheFieldAreIgnored(t *testing.T) {
	lay := credited(model.Identity{
		Title:     "A piece",
		Composer:  "J. S. Bach",
		Arranger:  "A. Smith",
		TitleRuns: []model.Span{{Text: "Something else entirely", Bold: true}},
	})
	line, ok := textSaying(lay, "A piece")
	if !ok {
		t.Fatal("the title is not on the page")
	}
	if len(line.Runs) != 0 {
		t.Errorf("the title is drawn in %d pieces, want the words it holds",
			len(line.Runs))
	}
	if _, found := textSaying(lay, "Something else entirely"); found {
		t.Error("words the score does not say are on the page")
	}
}

// A line in one style stays one piece of text, anchored where it belongs,
// so that the renderer centres it by the font's own widths.
func TestAPlainTitleIsStillOnePieceOfText(t *testing.T) {
	lay := credited(model.Identity{Title: "A piece"})
	line, ok := textSaying(lay, "A piece")
	if !ok {
		t.Fatal("the title is not on the page")
	}
	if len(line.Runs) != 0 {
		t.Errorf("a plain title is drawn in %d pieces, want one text", len(line.Runs))
	}
	if line.Anchor != "middle" {
		t.Errorf("the title is anchored %q, want the middle", line.Anchor)
	}
}

// A line break in a field is a line on the page, whichever run it falls in.
func TestALineBreakInACreditStartsALine(t *testing.T) {
	lay := credited(model.Identity{
		Title:    "A piece",
		Composer: "Words by A\nMusic by B",
		ComposerRuns: []model.Span{
			{Text: "Words by A\nMusic by "},
			{Text: "B", Bold: true},
		},
	})
	first, ok := textSaying(lay, "Words by A")
	if !ok {
		t.Fatal("the first line of the credit is not on the page")
	}
	second, ok := textSaying(lay, "Music by B")
	if !ok {
		t.Fatal("the second line of the credit is not on the page")
	}
	if second.Y <= first.Y {
		t.Errorf("the second line is at %.2f and the first at %.2f", second.Y, first.Y)
	}
	if len(second.Runs) != 2 {
		t.Errorf("the second line is drawn in %d pieces, want 2", len(second.Runs))
	}
}
