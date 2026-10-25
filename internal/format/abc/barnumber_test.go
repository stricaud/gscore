/*
 * barnumber_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package abc

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// ABC has one syntax for putting text over a staff and it is the chord
// symbol, so that is what people use when what they want is a bar number.
// Read literally the number is set in bold over the music, beside the bar
// number gscore works out for itself — two numbers over one bar, usually
// disagreeing, which is what somebody looking at the page sees.

func chordsOf(t *testing.T, src string) []string {
	t.Helper()
	s, err := Import(strings.NewReader(src))
	if s == nil {
		t.Fatalf("nothing was read: %v", err)
	}
	var out []string
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOfElement(el)
					if mk == nil || mk.Chord == nil {
						continue
					}
					out = append(out, mk.Chord.Root+mk.Chord.Quality+mk.Chord.Text)
				}
			}
		}
	}
	return out
}

func marksOfElement(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

func TestABarNumberWrittenAsAChordIsNotAChord(t *testing.T) {
	const src = "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n\"22\"c4 | \"103\"d4 |\n"
	if got := chordsOf(t, src); len(got) != 0 {
		t.Errorf("got chords %v, want none: those are bar numbers", got)
	}
}

// And it is reported rather than silently dropped. A file is read past its
// mistakes, and then the mistakes are listed.
func TestTheBarNumbersAreReported(t *testing.T) {
	const src = "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n\"22\"c4 |\n"
	_, err := Import(strings.NewReader(src))
	if err == nil {
		t.Fatal("nothing was reported")
	}
	if !strings.Contains(err.Error(), "bar number") {
		t.Errorf("the report does not say what happened: %v", err)
	}
}

// A real chord is still a real chord, numbers and all: C7 and Dm7 have
// digits in them and are not bar numbers.
func TestRealChordsStillRead(t *testing.T) {
	const src = "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n\"C7\"c4 | \"Dm7\"d4 | \"F#m\"e4 |\n"
	got := chordsOf(t, src)
	if len(got) != 3 {
		t.Fatalf("got %d chords %v, want 3", len(got), got)
	}
}

// ABC's own syntax for an annotation is a leading ^, and that still writes
// whatever it is given — including a number, where somebody really does
// want one over a bar.
func TestAnAnnotatedNumberIsStillWritten(t *testing.T) {
	const src = "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n\"^7\"c4 |\n"
	s, _ := Import(strings.NewReader(src))
	if s == nil {
		t.Fatal("nothing was read")
	}
	found := ""
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if mk := marksOfElement(el); mk != nil {
						for _, tx := range mk.Texts {
							found = tx.Text
						}
					}
				}
			}
		}
	}
	if found != "7" {
		t.Errorf("the annotation came out as %q, want %q", found, "7")
	}
}
