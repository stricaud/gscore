/*
 * words_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The words over the notes, through MusicXML.
 *
 * They were read and never written. A Dorico score opened in gscore and
 * saved out as MusicXML came out with every "dolce", every "pizz." and
 * every "più" gone — most of what is printed over the staves of a score —
 * and nothing said so, because nothing was wrong: the words were in the
 * score, and the writer had no line about them.
 *
 * MusicXML has no names for the kinds of writing gscore keeps, so what
 * crosses is what the words look like: the side of the staff and the face.
 * That is what these tests hold it to — the word arrives, on the right note,
 * on the right side, looking the way it looked — and not to a kind that the
 * format cannot say.
 */

package musicxml

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// worded is a bar of quarter notes, the first of them carrying these marks.
func worded(texts []model.TextMark, dyn model.Dynamic) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{
		Name: "Flute", Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: 5},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		if i == 0 {
			n.Texts = texts
			n.Dynamic = dyn
		}
		els = append(els, n)
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: els}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)
	s.Normalise()
	return s
}

// firstTexts are the words that came back on the first note of the bar.
func firstTexts(t *testing.T, s *model.Score) []model.TextMark {
	t.Helper()
	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) == 0 {
		t.Fatal("the bar came back empty")
	}
	n, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the first element came back as %T", els[0])
	}
	return n.Texts
}

func TestAWordOnANoteIsWritten(t *testing.T) {
	out := exported(t, worded([]model.TextMark{
		{Text: "dolce", Style: model.TextExpression},
	}, model.DynamicNone))
	if !strings.Contains(out, ">dolce</words>") {
		t.Errorf("the word is not in the file:\n%s", out)
	}
	// Before the note it is written on, which is how a direction says which
	// note it is about.
	if at, note := strings.Index(out, "dolce"), strings.Index(out, "<note"); at > note {
		t.Errorf("the word is written after the note it belongs to")
	}
}

func TestAWordComesBackOnItsNote(t *testing.T) {
	got := firstTexts(t, reimported(t, exported(t, worded(
		[]model.TextMark{{Text: "dolce", Style: model.TextExpression}},
		model.DynamicNone))))
	if len(got) != 1 {
		t.Fatalf("%d words came back on the note: %+v", len(got), got)
	}
	if got[0].Text != "dolce" {
		t.Errorf("the word came back as %q", got[0].Text)
	}
}

func TestOnlyTheFirstNoteKeepsTheWord(t *testing.T) {
	// A word belongs to the note it was written on. One that spread to the
	// rest of the bar would be read as four of them.
	s := reimported(t, exported(t, worded(
		[]model.TextMark{{Text: "dolce", Style: model.TextExpression}},
		model.DynamicNone)))
	els := s.Parts[0].Measures[0].Voices[0].Elements
	for i, el := range els[1:] {
		n, ok := el.(*model.Note)
		if !ok {
			continue
		}
		if len(n.Texts) != 0 {
			t.Errorf("note %d also came back with %+v", i+2, n.Texts)
		}
	}
}

func TestAQualifierIsWrittenInFrontOfItsDynamic(t *testing.T) {
	// "più p" is a word in front of the letters. In a file the order the
	// directions are written in is the order they are printed in, so the
	// word has to come first — and both have to come back.
	s := worded([]model.TextMark{
		{Text: "più", Style: model.TextExpression},
	}, model.Piano)
	out := exported(t, s)
	word := strings.Index(out, "più")
	dyn := strings.Index(out, "<dynamics>")
	if word < 0 || dyn < 0 {
		t.Fatalf("the file has the word at %d and the dynamic at %d:\n%s",
			word, dyn, out)
	}
	if word > dyn {
		t.Error("the word is written after the dynamic, so it prints as \"p più\"")
	}

	back := reimported(t, out)
	got := firstTexts(t, back)
	if len(got) != 1 || got[0].Text != "più" {
		t.Errorf("the word came back as %+v", got)
	}
	n := back.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Dynamic != model.Piano {
		t.Errorf("the dynamic came back as %v, want p", n.Dynamic)
	}
}

func TestAWordKeepsItsSideAndItsFace(t *testing.T) {
	// The kind of writing has no name in MusicXML; what it looks like does.
	// An expression is italic under the staff and a technique upright over
	// it, and that much survives, whatever the mark is called at the far
	// end.
	for _, c := range []struct {
		what   string
		style  model.TextStyle
		side   model.Placement
		italic bool
	}{
		{"an expression", model.TextExpression, model.PlacementBelow, true},
		{"a technique", model.TextTechnique, model.PlacementAbove, false},
	} {
		out := exported(t, worded([]model.TextMark{
			{Text: "sempre", Style: c.style},
		}, model.DynamicNone))
		back := firstTexts(t, reimported(t, out))
		if len(back) != 1 {
			t.Fatalf("%s came back as %d words: %+v", c.what, len(back), back)
		}
		if back[0].Placement != c.side {
			t.Errorf("%s came back on side %v, want %v",
				c.what, back[0].Placement, c.side)
		}
		if back[0].Italic != c.italic {
			t.Errorf("%s came back italic=%v, want %v",
				c.what, back[0].Italic, c.italic)
		}
	}
}

func TestTheSideAskedForBeatsTheKinds(t *testing.T) {
	// A mark that says where it goes goes there. An expression printed over
	// the staff — because the dynamics are under it and would collide — has
	// to come back over the staff.
	out := exported(t, worded([]model.TextMark{
		{Text: "espr.", Style: model.TextExpression,
			Placement: model.PlacementAbove},
	}, model.DynamicNone))
	if !strings.Contains(out, `placement="above"`) {
		t.Errorf("the word was written under the staff anyway:\n%s", out)
	}
	back := firstTexts(t, reimported(t, out))
	if len(back) != 1 || back[0].Placement != model.PlacementAbove {
		t.Errorf("it came back as %+v", back)
	}
}

func TestSeveralWordsOnOneNoteAllArrive(t *testing.T) {
	out := exported(t, worded([]model.TextMark{
		{Text: "pizz.", Style: model.TextTechnique},
		{Text: "sul tasto", Style: model.TextTechnique},
	}, model.DynamicNone))
	back := firstTexts(t, reimported(t, out))
	if len(back) != 2 {
		t.Fatalf("%d of the two words came back: %+v", len(back), back)
	}
	if back[0].Text != "pizz." || back[1].Text != "sul tasto" {
		t.Errorf("the words came back as %q and %q, in that order",
			back[0].Text, back[1].Text)
	}
}

func TestAWordWithNothingInItIsNotWritten(t *testing.T) {
	// An empty <words/> is a mark with no mark in it, and every reader
	// draws it as a blank over the staff.
	out := exported(t, worded([]model.TextMark{
		{Text: "   ", Style: model.TextExpression},
	}, model.DynamicNone))
	if strings.Contains(out, "<words") {
		t.Errorf("a word with nothing in it was written:\n%s", out)
	}
	// And no direction was written to hold it. Counted against the same
	// score without the mark, because the bar has a direction of its own —
	// the tempo at the head of the piece.
	plain := exported(t, worded(nil, model.DynamicNone))
	if got, want := strings.Count(out, "<direction "),
		strings.Count(plain, "<direction "); got != want {
		t.Errorf("%d directions were written where the same score without "+
			"the empty mark has %d:\n%s", got, want, out)
	}
}
