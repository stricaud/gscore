/*
 * balanceslurs_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A slur with only one of its two ends.
 *
 * MuseScore names both, so one without the other is one this reader failed
 * to follow: a slur whose far end is on the other staff of a piano score,
 * or on a chord it could not read, or in a voice that was not there. The
 * Moonlight sonata is a MuseScore 2 file and has a few of each.
 *
 * It matters because of what the drawing does with one. A slur still open
 * when a system ends is carried to the right margin and started again on
 * the system after it, and the one after that, for the rest of the piece —
 * so a single missing end is a line drawn across every remaining page.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func quarter() model.Duration { return model.Duration{Num: 1, Den: 4} }

// staff builds one voice of notes, so the marks can be described alone.
func staff(n int) (*model.Part, []*model.Note) {
	notes := make([]*model.Note, n)
	var els []model.Element
	for i := range notes {
		notes[i] = &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: quarter(),
		}
		els = append(els, notes[i])
	}
	p := &model.Part{Measures: []*model.Measure{{
		Voices: []*model.Voice{{Elements: els}},
	}}}
	return p, notes
}

func TestASlurWithNoEndIsNotDrawn(t *testing.T) {
	p, n := staff(4)
	n[1].Marks.SlurStart = true // and nothing ever stops it

	balanceSlurs(p)

	if n[1].Marks.SlurStart {
		t.Error("a slur with no end was kept, and it is drawn to the margin of " +
			"this system and of every system after it")
	}
}

func TestASlurWithNoBeginningIsNotDrawn(t *testing.T) {
	p, n := staff(4)
	n[2].Marks.SlurStop = true // with nothing open

	balanceSlurs(p)

	if n[2].Marks.SlurStop {
		t.Error("an end with no beginning was kept")
	}
}

func TestASlurWithBothEndsIsLeftAlone(t *testing.T) {
	p, n := staff(5)
	n[1].Marks.SlurStart = true
	n[3].Marks.SlurStop = true

	balanceSlurs(p)

	if !n[1].Marks.SlurStart || !n[3].Marks.SlurStop {
		t.Error("a slur with both ends was thrown away")
	}
	// And nothing else grew one.
	for i, x := range n {
		if i == 1 && x.Marks.SlurStart {
			continue
		}
		if i == 3 && x.Marks.SlurStop {
			continue
		}
		if x.Marks.SlurStart || x.Marks.SlurStop {
			t.Errorf("note %d gained a slur mark", i)
		}
	}
}

func TestOneSlurEndsBeforeTheNextBegins(t *testing.T) {
	// The drawing holds one open at a time per line of music, so a second
	// beginning while one is open is no new slur. What is left has to be
	// what will be drawn, or this has not answered the question.
	p, n := staff(6)
	n[0].Marks.SlurStart = true
	n[1].Marks.SlurStart = true // ignored by the engraver
	n[3].Marks.SlurStop = true  // ends the first
	n[4].Marks.SlurStart = true
	n[5].Marks.SlurStop = true

	balanceSlurs(p)

	if !n[0].Marks.SlurStart || !n[3].Marks.SlurStop {
		t.Error("the first slur was lost")
	}
	if n[1].Marks.SlurStart {
		t.Error("a second beginning was kept while one was already open; the " +
			"engraver ignores it, so keeping it says the score holds a slur " +
			"that is never drawn")
	}
	if !n[4].Marks.SlurStart || !n[5].Marks.SlurStop {
		t.Error("the slur after it was lost")
	}
}

func TestAGraceNoteSlursIntoTheNoteItLeansOn(t *testing.T) {
	// The commonest slur in this music: an appoggiatura leaning into its
	// note. The beginning is on the grace note, inside the note it
	// decorates, so a walk that only looks at the voice's own elements
	// never sees it — and then the end looks like an end with no
	// beginning, and is thrown away.
	p, n := staff(3)
	g := &model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: quarter()}
	g.Marks.SlurStart = true
	n[1].Marks.Grace = &model.GraceGroup{Notes: []*model.Note{g}}
	n[2].Marks.SlurStop = true

	balanceSlurs(p)

	if !g.Marks.SlurStart {
		t.Error("the grace note's slur was thrown away")
	}
	if !n[2].Marks.SlurStop {
		t.Error("the end of the grace note's slur was thrown away as an " +
			"end with no beginning")
	}
}

func TestAGraceNoteSlurWithNoEndIsNotDrawnEither(t *testing.T) {
	p, n := staff(3)
	g := &model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: quarter()}
	g.Marks.SlurStart = true
	n[1].Marks.Grace = &model.GraceGroup{Notes: []*model.Note{g}}

	balanceSlurs(p)

	if g.Marks.SlurStart {
		t.Error("a grace note's slur with no end was kept")
	}
}

func TestEachLineOfMusicIsBalancedOnItsOwn(t *testing.T) {
	// A slur is held open per voice while drawing, so one left open in the
	// upper line is never closed by the lower one.
	a := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: quarter()}
	b := &model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: quarter()}
	a.Marks.SlurStart = true
	b.Marks.SlurStop = true
	p := &model.Part{Measures: []*model.Measure{{Voices: []*model.Voice{
		{Elements: []model.Element{a}},
		{Elements: []model.Element{b}},
	}}}}

	balanceSlurs(p)

	if a.Marks.SlurStart {
		t.Error("a slur in the upper line was closed by the lower one")
	}
	if b.Marks.SlurStop {
		t.Error("an end in the lower line was kept for a beginning in the upper")
	}
}
