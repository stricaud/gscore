/*
 * ties_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tie joins a note to the one that follows it.
 *
 * MuseScore writes the tie on the note it starts from and writes nothing at
 * all on the note it reaches, so the end has to be found — and "the next
 * element of this voice" is not the same as "the next note". A voice that
 * stops and starts again leaves a gap, and the next element after the gap
 * may be anywhere.
 *
 * The left hand of the Moonlight sonata has such a voice: it holds a note
 * in bar 131 and does not appear again until bar 142, in the next movement.
 * The tie was drawn the whole way, a line across two systems and through
 * two staves of the Presto.
 *
 * And a tie the engraver cannot close is worse than one that is wrong: it
 * is carried to the right margin of its system and begun again on the next,
 * and the next, to the end of the piece. So a tie has both ends or neither.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// run builds one voice spread over bars, as closeTies sees it: each entry is
// one bar's worth of elements.
func run(bars ...[]model.Element) []*model.Measure {
	var out []*model.Measure
	for _, els := range bars {
		out = append(out, &model.Measure{Voices: []*model.Voice{{Elements: els}}})
	}
	return out
}

func noteOf(step model.Step, oct int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: oct},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
}

func TestATieReachesTheNoteAfterIt(t *testing.T) {
	a, b := noteOf(model.StepA, 2), noteOf(model.StepA, 2)
	a.TieStart = true
	closeTies(run([]model.Element{a, b}))

	if !a.TieStart || !b.TieStop {
		t.Errorf("the tie came out start=%v stop=%v, want both", a.TieStart, b.TieStop)
	}
}

func TestATieReachesOverOneBarline(t *testing.T) {
	a, b := noteOf(model.StepA, 2), noteOf(model.StepA, 2)
	a.TieStart = true
	closeTies(run([]model.Element{a}, []model.Element{b}))

	if !a.TieStart || !b.TieStop {
		t.Errorf("a tie across the barline came out start=%v stop=%v",
			a.TieStart, b.TieStop)
	}
}

func TestATieDoesNotReachAcrossAGapInItsVoice(t *testing.T) {
	// Eleven bars of nothing, then the voice comes back. That is not the
	// note after it, whatever the pitch says.
	a, b := noteOf(model.StepA, 2), noteOf(model.StepA, 2)
	a.TieStart = true
	bars := [][]model.Element{{a}}
	for i := 0; i < 10; i++ {
		bars = append(bars, nil)
	}
	bars = append(bars, []model.Element{b})
	closeTies(run(bars...))

	if b.TieStop {
		t.Error("a note eleven bars later was tied to")
	}
	if a.TieStart {
		t.Error("the tie was kept with nothing to tie to, so it is drawn to " +
			"the margin of this system and of every system after it")
	}
}

func TestATieToADifferentPitchIsNotATie(t *testing.T) {
	a, b := noteOf(model.StepA, 2), noteOf(model.StepB, 2)
	a.TieStart = true
	closeTies(run([]model.Element{a, b}))

	if b.TieStop {
		t.Error("a note of another pitch was tied to")
	}
	if a.TieStart {
		t.Error("the tie was kept with nothing to tie to")
	}
}

func TestATieIntoSilenceIsNotATie(t *testing.T) {
	a := noteOf(model.StepA, 2)
	a.TieStart = true
	rest := &model.Rest{Dur: model.Duration{Num: 1, Den: 4}}
	closeTies(run([]model.Element{a, rest}))

	if a.TieStart {
		t.Error("a tie into a rest was kept")
	}
}

func TestAChordTiedAsAWholeKeepsOnlyWhatItCanTie(t *testing.T) {
	// A chord may be tied as a whole. Where one of its noteheads has
	// somewhere to go and the others have not, the chord still carries a
	// tie — and where none of them has, it carries none.
	c := &model.Chord{Dur: model.Duration{Num: 1, Den: 4}, TieStart: true,
		Notes: []*model.Note{noteOf(model.StepA, 2), noteOf(model.StepC, 3)}}
	next := &model.Chord{Dur: model.Duration{Num: 1, Den: 4},
		Notes: []*model.Note{noteOf(model.StepA, 2)}}
	closeTies(run([]model.Element{c, next}))

	if !c.TieStart {
		t.Error("the chord lost a tie one of its notes could make")
	}
	// The chord carries the tie, not its noteheads: that is how MuseScore
	// writes one tied as a whole, and nothing here changes it.
	if next.Notes[0].TieStop != true {
		t.Error("the note under it was not tied to")
	}

	// And with nothing under it at all.
	alone := &model.Chord{Dur: model.Duration{Num: 1, Den: 4}, TieStart: true,
		Notes: []*model.Note{noteOf(model.StepA, 2)}}
	closeTies(run([]model.Element{alone}))
	if alone.TieStart || alone.Notes[0].TieStart {
		t.Error("a chord tied to nothing kept its tie")
	}
}

func TestAHeldNoteDoesNotSurviveTheEndOfItsMovement(t *testing.T) {
	// The Allegretto ends and the Presto begins, in another key and another
	// meter. Nothing is still ringing from before.
	s := model.NewScore()
	p := s.Parts[0]
	a, b := noteOf(model.StepA, 2), noteOf(model.StepA, 2)
	a.TieStart = true
	p.Measures = run([]model.Element{a}, []model.Element{b})
	p.Measures[0].EndsSection = true

	breakTiesAtMovements(s)

	if a.TieStart {
		t.Error("a note was held across the end of its movement")
	}
}

func TestAHeldNoteInsideAMovementIsLeftAlone(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	a, b := noteOf(model.StepA, 2), noteOf(model.StepA, 2)
	a.TieStart = true
	b.TieStop = true
	p.Measures = run([]model.Element{a}, []model.Element{b})

	breakTiesAtMovements(s)

	if !a.TieStart || !b.TieStop {
		t.Error("an ordinary tie was broken")
	}
}
