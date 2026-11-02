/*
 * barline_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A score of three bars with one barline style on the second.
//
// The second rather than the first, because a repeat that opens is written on
// the bar after the one it is a property of, and a style on the last bar has
// no bar after it to be written on.
func threeBars(style model.BarlineStyle) *model.Score {
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < 3; i++ {
		m := &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4},
				Dur: model.Duration{Num: 1, Den: 1}},
		}}}}
		if i == 1 {
			m.RightBarline = style
		}
		p.Measures = append(p.Measures, m)
	}
	return s
}

// Every barline a score can hold survives being written out and read back.
//
// Two did not. repeat-both was never written at all, so a bar where one
// repeat closes and the next opens came out as the closing alone — the music
// went back and then had nothing to say it went on. And an invisible barline
// was written as nothing, which a reader takes for an ordinary one, so the
// score came back with a line nobody had put there.
func TestEveryBarlineSurvivesTheRoundTrip(t *testing.T) {
	for _, style := range []model.BarlineStyle{
		model.BarSingle, model.BarDouble, model.BarFinal,
		model.BarRepeatStart, model.BarRepeatEnd, model.BarRepeatBoth,
		model.BarNone,
	} {
		var out bytes.Buffer
		if err := Export(&out, threeBars(style)); err != nil {
			t.Fatalf("%v: writing it out: %v", style, err)
		}
		back, err := Import(bytes.NewReader(out.Bytes()))
		if err != nil {
			t.Fatalf("%v: reading it back: %v", style, err)
		}
		if len(back.Parts) == 0 || len(back.Parts[0].Measures) < 2 {
			t.Fatalf("%v: came back with no second bar", style)
		}
		if got := back.Parts[0].Measures[1].RightBarline; got != style {
			t.Errorf("%v came back as %v", style, got)
		}
	}
}

// A repeat that opens is written on the left of the bar it opens, and a
// left-hand barline has to be the first thing in its measure.
//
// It used to be written on the bar it closes — the bar before — and appended
// after that bar's notes, which is both the wrong measure and a place a
// left-hand barline cannot be.
func TestAnOpeningRepeatIsWrittenOnTheBarItOpens(t *testing.T) {
	var out bytes.Buffer
	if err := Export(&out, threeBars(model.BarRepeatBoth)); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Parts []struct {
			Measures []struct {
				Number  string `xml:"number,attr"`
				Content []struct {
					XMLName  xml.Name
					Location string `xml:"location,attr"`
					Repeat   *struct {
						Direction string `xml:"direction,attr"`
					} `xml:"repeat"`
				} `xml:",any"`
			} `xml:"measure"`
		} `xml:"part"`
	}
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("reading what was written: %v", err)
	}
	if len(doc.Parts) != 1 || len(doc.Parts[0].Measures) != 3 {
		t.Fatalf("expected one part of three bars, got %d parts", len(doc.Parts))
	}

	// The second bar closes the repeat, on its right.
	closing := 0
	for _, it := range doc.Parts[0].Measures[1].Content {
		if it.XMLName.Local == "barline" {
			closing++
			if it.Location != "right" {
				t.Errorf("the closing repeat is on the %q of bar 2", it.Location)
			}
			if it.Repeat == nil || it.Repeat.Direction != "backward" {
				t.Errorf("bar 2's barline does not close a repeat")
			}
		}
	}
	if closing != 1 {
		t.Errorf("bar 2 carries %d barlines, expected 1", closing)
	}

	// The third opens one, on its left, before anything else in the bar.
	third := doc.Parts[0].Measures[2].Content
	if len(third) == 0 {
		t.Fatal("bar 3 is empty")
	}
	if third[0].XMLName.Local != "barline" {
		t.Errorf("bar 3 begins with <%s>, not the barline that opens it",
			third[0].XMLName.Local)
	}
	if third[0].Location != "left" {
		t.Errorf("the opening repeat is on the %q of bar 3", third[0].Location)
	}
	if third[0].Repeat == nil || third[0].Repeat.Direction != "forward" {
		t.Errorf("bar 3's first barline does not open a repeat")
	}
	if n := strings.Count(string(out.Bytes()), "direction=\"forward\""); n != 1 {
		t.Errorf("%d opening repeats written, expected 1", n)
	}
}

// What closes a bar and what opens the next are one mark on the page.
func TestAClosingAndAnOpeningRepeatBecomeOneSign(t *testing.T) {
	for _, c := range []struct {
		closing, opening, want model.BarlineStyle
	}{
		{model.BarRepeatEnd, model.BarRepeatStart, model.BarRepeatBoth},
		{model.BarSingle, model.BarRepeatStart, model.BarRepeatStart},
		{model.BarNone, model.BarRepeatStart, model.BarRepeatStart},
		{model.BarRepeatBoth, model.BarRepeatStart, model.BarRepeatBoth},
		// Nothing opening leaves what closes alone.
		{model.BarFinal, model.BarSingle, model.BarFinal},
		// And a repeat opening after a final bar is not one mark; the bar
		// already written is the one that stays.
		{model.BarFinal, model.BarRepeatStart, model.BarFinal},
	} {
		if got := joinBarlines(c.closing, c.opening); got != c.want {
			t.Errorf("%v closing and %v opening joined to %v, expected %v",
				c.closing, c.opening, got, c.want)
		}
	}
}
