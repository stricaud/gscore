/*
 * stripheader_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a score is called, in a view that has no pages.
 *
 * A title is centred on the page and a credit is set against its right-hand
 * edge, which is what every printed score does and what gscore does too. A
 * continuous strip has no page: the music runs as far as it runs, the
 * drawing is trimmed to it, and the middle of that is wherever the piece
 * happens to end — half a mile to the right on a symphony.
 *
 * So the title went there, placed by a pass that cannot run until the last
 * bar is laid out. On the screen that is a score whose name appears only
 * when the engraving has finished, in a place nobody is looking at. In a
 * strip the header belongs at the left margin, where the music starts.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// named is a long enough score to have a middle worth missing.
func named(bars int) *model.Score {
	s := model.NewScore()
	s.Identity = model.Identity{
		Title:    "Boléro",
		Subtitle: "pour orchestre",
		Composer: "Maurice Ravel",
		Arranger: "arr. nobody",
	}
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < bars; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepC, Octave: 5},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	return s
}

// headerTexts are the lines of the title block: everything drawn above the
// first staff.
func headerTexts(lay *Layout) []Text {
	var out []Text
	for _, t := range lay.Texts {
		switch t.S {
		case "Boléro", "pour orchestre", "Maurice Ravel", "arr. nobody":
			out = append(out, t)
		}
	}
	return out
}

func stripOptions() Options {
	o := DefaultOptions()
	o.Mode = Strip
	return o
}

func TestTheHeaderOfAStripIsAtTheLeft(t *testing.T) {
	lay := Engrave(named(60), stripOptions())
	got := headerTexts(lay)
	if len(got) != 4 {
		t.Fatalf("%d lines of header were drawn, want the four", len(got))
	}
	// Against the left margin, which is where the music starts. The margin
	// itself is the only x small enough to be it.
	margin := DefaultOptions().Spacings.Margin
	for _, x := range got {
		if x.X > margin+0.01 {
			t.Errorf("%q is drawn at x=%.1f, and the left margin is %.1f",
				x.S, x.X, margin)
		}
		if x.Anchor != "start" {
			t.Errorf("%q is anchored by its %q", x.S, x.Anchor)
		}
	}
}

func TestTheHeaderDoesNotMoveAsTheMusicGrows(t *testing.T) {
	// The point of the whole arrangement: where the title goes is known
	// before a note is laid out, so it does not wait for the engraving to
	// finish and does not move when more music arrives.
	short := headerTexts(Engrave(named(4), stripOptions()))
	long := headerTexts(Engrave(named(400), stripOptions()))
	if len(short) != len(long) {
		t.Fatalf("%d lines and %d", len(short), len(long))
	}
	for i := range short {
		if short[i].X != long[i].X {
			t.Errorf("%q is at x=%.1f in a short score and x=%.1f in a long "+
				"one", short[i].S, short[i].X, long[i].X)
		}
	}
}

func TestAPaginatedScoreKeepsItsCentredTitle(t *testing.T) {
	// And the page is untouched: a title centred over the music and a
	// credit against the right-hand edge is what a printed score does.
	lay := Engrave(named(60), DefaultOptions())
	var title, composer *Text
	for i, x := range lay.Texts {
		switch x.S {
		case "Boléro":
			title = &lay.Texts[i]
		case "Maurice Ravel":
			composer = &lay.Texts[i]
		}
	}
	if title == nil || composer == nil {
		t.Fatal("the title block was not drawn")
	}
	if title.Anchor != "middle" {
		t.Errorf("the title is anchored by its %q", title.Anchor)
	}
	if composer.Anchor != "end" {
		t.Errorf("the composer is anchored by its %q", composer.Anchor)
	}
	if title.X <= DefaultOptions().Spacings.Margin {
		t.Errorf("the title is at x=%.1f, which is not centred on anything",
			title.X)
	}
}
