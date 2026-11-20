/*
 * coda_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A coda begins a line of its own, set in from the margin. Asked for as
 * "when we have a coda, I want to make sure we have a clean break from the
 * previous bar to a new line and have some left margin".
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// sixBars is six bars of whole notes, which fit on one line with room to
// spare, with the coda sign on the bar asked for (or none for -1).
func sixBars(coda int) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 6; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 1}},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	if coda >= 0 {
		p.Measures[coda].Sign = model.Coda
	}
	return &model.Score{Parts: []*model.Part{p}}
}

// staffStart is where the staff lines of the system holding a y begin.
func staffStart(lay *Layout, top, bottom float64) float64 {
	left := math.Inf(1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X2-l.X1 > 8 && l.Y1 >= top && l.Y1 <= bottom && l.X1 < left {
			left = l.X1
		}
	}
	return left
}

func TestACodaBeginsALineOfItsOwnSetIn(t *testing.T) {
	plain := Engrave(sixBars(-1), DefaultOptions())
	if len(plain.Systems) != 1 {
		t.Fatalf("six whole notes take %d lines without a coda, want 1", len(plain.Systems))
	}
	lay := Engrave(sixBars(3), DefaultOptions())
	if len(lay.Systems) != 2 {
		t.Fatalf("with the coda at bar 4 the music takes %d lines, want 2", len(lay.Systems))
	}
	if lay.Systems[1].First != 3 {
		t.Errorf("the second line begins at bar %d, want the coda's bar 4", lay.Systems[1].First+1)
	}
	first := staffStart(lay, lay.Systems[0].Top, lay.Systems[0].Bottom)
	coda := staffStart(lay, lay.Systems[1].Top, lay.Systems[1].Bottom)
	if math.Abs((coda-first)-codaIndent) > 0.01 {
		t.Errorf("the coda's line begins at %.2f and the first at %.2f: set in by %.2f, want %d",
			coda, first, coda-first, codaIndent)
	}
	// And the coda sign is on the second line, over its bar.
	found := false
	for _, s := range lay.Signs {
		if s.Text == "coda" && s.Measure == 3 && s.Y >= lay.Systems[1].Top-6 && s.X >= coda {
			found = true
		}
	}
	if !found {
		t.Errorf("no coda sign over bar 4 on its own line: %+v", lay.Signs)
	}
}

// The bars before the coda still break into lines as they fit: the coda
// ends whichever line it falls in, it does not pull every bar before it
// onto one line. Twenty bars of quarters with the coda on the last ran as
// one system of nineteen bars, which is what "the paginated view looks
// continuous" was.
func TestACodaDoesNotDragTheBarsBeforeItOntoOneLine(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 20; i++ {
		var els []model.Element
		for k := 0; k < 4; k++ {
			els = append(els, &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	p.Measures[19].Sign = model.Coda
	opt := DefaultOptions()
	opt.Mode = Paginated
	lay := Engrave(&model.Score{Parts: []*model.Part{p}}, opt)
	if len(lay.Systems) < 3 {
		t.Fatalf("twenty bars and a coda take %d lines, want the bars broken as they fit and the coda on its own", len(lay.Systems))
	}
	last := lay.Systems[len(lay.Systems)-1]
	if last.First != 19 {
		t.Errorf("the last line begins at bar %d, want the coda's bar 20", last.First+1)
	}
	// The first line holds what it would hold without the coda.
	p.Measures[19].Sign = model.NoSign
	plain := Engrave(&model.Score{Parts: []*model.Part{p}}, opt)
	if got, want := lay.Systems[0].Last, plain.Systems[0].Last; got != want {
		t.Errorf("the first line ends at bar %d with the coda and %d without", got+1, want+1)
	}
	if lay.Width > opt.LineWidth+2*opt.Spacings.Margin+0.01 {
		t.Errorf("the page is %.1f wide, wider than its %.1f", lay.Width, opt.LineWidth+2*opt.Spacings.Margin)
	}
}

// The strip is one unbroken line: a coda neither breaks it nor sets
// anything in.
func TestACodaLeavesTheStripUnbroken(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(sixBars(3), opt)
	plain := Engrave(sixBars(-1), opt)
	if len(lay.Systems) != len(plain.Systems) {
		t.Errorf("the strip has %d lines with a coda and %d without", len(lay.Systems), len(plain.Systems))
	}
	if a, b := staffStart(lay, lay.Systems[0].Top, lay.Systems[0].Bottom),
		staffStart(plain, plain.Systems[0].Top, plain.Systems[0].Bottom); math.Abs(a-b) > 0.01 {
		t.Errorf("the strip begins at %.2f with a coda and %.2f without", a, b)
	}
}

// A coda in the first bar is nothing to set apart from: the piece begins
// there, and the line begins at the margin like any first line.
func TestACodaInBarOneIsNotSetIn(t *testing.T) {
	lay := Engrave(sixBars(0), DefaultOptions())
	plain := Engrave(sixBars(-1), DefaultOptions())
	a := staffStart(lay, lay.Systems[0].Top, lay.Systems[0].Bottom)
	b := staffStart(plain, plain.Systems[0].Top, plain.Systems[0].Bottom)
	if math.Abs(a-b) > 0.01 {
		t.Errorf("the first line begins at %.2f with a coda and %.2f without", a, b)
	}
}
