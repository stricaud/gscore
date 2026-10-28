/*
 * sectionbreak_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where one movement ends and another begins.
 *
 * Run on from the bar before it, the Presto of the Moonlight sonata began
 * halfway along a system that ends the Allegretto — in another key, another
 * meter and another tempo, with nothing but a double barline to say that
 * anything had happened. A movement begins a system of its own, and a page
 * of its own.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// movements builds one staff of n bars with a section break after each of
// the given bars, counted from zero.
func movements(n int, breaks ...int) *model.Score {
	s := model.NewScore()
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < n; i++ {
		m := &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
			&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
		}}}}
		p.Measures = append(p.Measures, m)
	}
	for _, at := range breaks {
		p.Measures[at].EndsSection = true
	}
	return s
}

func TestNoSystemRunsAcrossASectionBreak(t *testing.T) {
	// Twelve bars of whole rests: several would fit on a line, so the break
	// has to be what stops them.
	s := movements(12, 3, 7)
	lay := Engrave(s, DefaultOptions())
	if len(lay.Systems) < 3 {
		t.Fatalf("%d systems, want at least one per movement", len(lay.Systems))
	}
	for _, sys := range lay.Systems {
		for _, at := range []int{3, 7} {
			if sys.First <= at && sys.Last > at {
				t.Errorf("a system runs from bar %d to bar %d, across the "+
					"movement that ends at bar %d",
					sys.First+1, sys.Last+1, at+1)
			}
		}
	}
}

func TestTheFirstSystemOfAMovementSaysSo(t *testing.T) {
	s := movements(12, 3, 7)
	lay := Engrave(s, DefaultOptions())

	want := map[int]bool{4: true, 8: true}
	for _, sys := range lay.Systems {
		if got := sys.StartsSection; got != want[sys.First] {
			t.Errorf("the system starting at bar %d says it begins a movement: %v, want %v",
				sys.First+1, got, want[sys.First])
		}
	}
	// The first system of all is the first page already, and saying it
	// begins a movement would ask for a page break before the title.
	if lay.Systems[0].StartsSection {
		t.Error("the first system of the score asks for a page break before it")
	}
}

func TestAScoreWithNoBreaksIsLaidOutAsBefore(t *testing.T) {
	plain := Engrave(movements(12), DefaultOptions())
	for _, sys := range plain.Systems {
		if sys.StartsSection {
			t.Errorf("a score with no movements in it asked for a page break at bar %d",
				sys.First+1)
		}
	}
	// And the bars still go as many to a system as they fit.
	if len(plain.Systems) >= len(Engrave(movements(12, 3, 7), DefaultOptions()).Systems) {
		t.Error("breaking a score into movements did not cost it any systems, " +
			"so the break is doing nothing")
	}
}

// A movement is headed by its name, and the heading belongs to it.
//
// Where the page is cut matters as much as that the heading is drawn: a
// heading printed at the foot of the page before its own movement is worse
// than no heading. The system records its territory as beginning at the
// heading, which is where the renderer cuts.
func TestAMovementIsHeadedByItsName(t *testing.T) {
	s := movements(12, 3, 7)
	s.Parts[0].Measures[0].SectionTitle = "Movement I"
	s.Parts[0].Measures[4].SectionTitle = "Movement II"
	s.Parts[0].Measures[8].SectionTitle = "Movement III"

	lay := Engrave(s, DefaultOptions())

	for _, want := range []string{"Movement I", "Movement II", "Movement III"} {
		found := false
		for _, tx := range lay.Texts {
			if tx.S == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is nowhere on the page", want)
		}
	}

	// The heading sits inside its own system's territory, not in the one
	// above it.
	for _, name := range []string{"Movement II", "Movement III"} {
		var at float64
		for _, tx := range lay.Texts {
			if tx.S == name {
				at = tx.Y
			}
		}
		var owner *SystemPos
		for i := range lay.Systems {
			sys := &lay.Systems[i]
			if at >= sys.Top && at <= sys.Bottom {
				owner = sys
			}
		}
		if owner == nil {
			t.Errorf("%q is in no system's territory, so the page may be cut "+
				"between it and the music it heads", name)
			continue
		}
		if !owner.StartsSection {
			t.Errorf("%q sits in the system starting at bar %d, which is not "+
				"the movement it names", name, owner.First+1)
		}
	}
}

func TestAMovementWithNoNameIsStillSeparated(t *testing.T) {
	// The break is the music's; the name is the file's, and plenty of files
	// do not write one.
	s := movements(12, 3, 7)
	lay := Engrave(s, DefaultOptions())
	if len(lay.Systems) < 3 {
		t.Fatalf("%d systems", len(lay.Systems))
	}
	started := 0
	for _, sys := range lay.Systems {
		if sys.StartsSection {
			started++
		}
	}
	if started != 2 {
		t.Errorf("%d systems begin a movement, want 2", started)
	}
}

// Bars are numbered from one in every movement.
//
// That is how a movement is read and how it is cited: the Presto's fourth
// bar is its own fourth, not the sonata's hundred and thirty-seventh. Every
// printed edition numbers them that way and so does MuseScore, which is
// where these scores are written.
func TestBarsAreNumberedFromOneInEveryMovement(t *testing.T) {
	s := movements(12, 3, 7)
	e := &engraver{score: s, opt: DefaultOptions()}

	// Bars 1..4, then 1..4 again, then 1..4 again.
	want := []int{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4}
	for i, w := range want {
		if got := e.barNumber(i); got != w {
			t.Errorf("bar %d of the score is numbered %d, want %d", i+1, got, w)
		}
	}
}

func TestAMovementThatOpensWithAnUpbeatCountsFromItsFirstFullBar(t *testing.T) {
	// The Allegretto of the Moonlight sonata opens on a quarter-note
	// upbeat. An anacrusis is not counted, in a movement exactly as in a
	// piece: its first full bar is its bar one.
	s := movements(8, 3)
	// The fifth bar — the first of the second movement — holds a quarter
	// where the meter asks for a whole.
	s.Parts[0].Measures[4].Voices[0].Elements = []model.Element{
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}
	e := &engraver{score: s, opt: DefaultOptions()}

	if got := e.barNumber(4); got != 0 {
		t.Errorf("the upbeat is numbered %d, want 0 — it is not counted", got)
	}
	if got := e.barNumber(5); got != 1 {
		t.Errorf("the first full bar of the movement is numbered %d, want 1", got)
	}
}

func TestAScoreWithOneMovementNumbersStraightThrough(t *testing.T) {
	s := movements(6)
	e := &engraver{score: s, opt: DefaultOptions()}
	for i := 0; i < 6; i++ {
		if got := e.barNumber(i); got != i+1 {
			t.Errorf("bar %d is numbered %d", i+1, got)
		}
	}
}
