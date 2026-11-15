/*
 * tieclose_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tie that is never closed is closed on the note after it.
//
// Nothing stopped one before: an open tie is drawn to the margin at every
// line break and picked up again on the next system, so a tie whose other
// half is missing was drawn across the rest of the piece. Any file can say
// a note is held and not say into what — Finale leaves the mark on a note
// whose neighbour has changed pitch, and the MusicXML suite has a case of it
// on purpose — and the page then showed a curve sweeping over whole systems
// with the music underneath it.

// oneBar puts a line of notes in a single bar, so every curve in the layout
// belongs to them.
func tieTestBar(notes ...*model.Note) *model.Score {
	els := make([]model.Element, 0, len(notes))
	for _, n := range notes {
		els = append(els, n)
	}
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: len(notes), BeatValue: 4},
		Measures: []*model.Measure{{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarFinal,
		}},
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func tiedTestNote(step model.Step, octave int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: octave},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
}

func TestATieWithNoEndStopsAtTheNextNote(t *testing.T) {
	held := tiedTestNote(model.StepC, 4)
	held.TieStart = true
	score := tieTestBar(held, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepE, 4), tiedTestNote(model.StepF, 4))

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}

	// It has to end on the next note and not run on past the ones after it.
	hit := lay.Hits[0]
	if len(hit.Elements) < 2 {
		t.Fatal("the bar did not come out with its notes in it")
	}
	second, third := hit.Elements[1].X, hit.Elements[2].X
	if end := lay.Curves[0].X2; end > third {
		t.Errorf("the tie ends at %.2f, past the third note at %.2f", end, third)
	}
	// Up to the note it is tied to. Not onto it: a tie stops a hair short of
	// the notehead it arrives at rather than running under it. See tieGap.
	if end := lay.Curves[0].X2; end < second-tieGap {
		t.Errorf("the tie ends at %.2f, short of the note it is tied to at %.2f",
			end, second)
	}
}

// And it stays a tie: one curve between two notes, not a line across the
// system.
func TestAnUnendedTieIsNotDrawnAcrossTheSystem(t *testing.T) {
	held := tiedTestNote(model.StepG, 4)
	held.TieStart = true
	var notes []*model.Note
	notes = append(notes, held)
	for i := 0; i < 15; i++ {
		notes = append(notes, tiedTestNote(model.StepA, 4))
	}
	score := tieTestBar(notes...)

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want one", len(lay.Curves))
	}
	width := lay.Curves[0].X2 - lay.Curves[0].X1
	if width > 4 {
		t.Errorf("the tie is %.2f spaces wide, which is a line across the music", width)
	}
}

// A tie that is closed is left exactly as it was.
func TestARealTieIsUntouched(t *testing.T) {
	first := tiedTestNote(model.StepC, 4)
	first.TieStart = true
	second := tiedTestNote(model.StepC, 4)
	second.TieStop = true
	score := tieTestBar(first, second, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepE, 4))

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}
	hit := lay.Hits[0]
	// From the first note to the second, which is what a tie is.
	if end, want := lay.Curves[0].X2, hit.Elements[2].X; end > want {
		t.Errorf("the tie ends at %.2f, past the note after the one it joins at %.2f", end, want)
	}
}

// A note held through several bars keeps its chain: each note stops the tie
// before it and starts the next, and none of them is cut short.
func TestATieChainIsUnbroken(t *testing.T) {
	var measures []*model.Measure
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 1},
		}
		n.TieStart = i < 3
		n.TieStop = i > 0
		measures = append(measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		})
	}
	score := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: measures,
		}},
	}

	lay := Engrave(score, DefaultOptions())
	// Three ties join four notes.
	if len(lay.Curves) != 3 {
		t.Errorf("%d curves for a note held through four bars, want three", len(lay.Curves))
	}
}

// Every notehead of a chord is tied on its own account, and an unended one
// among them does not drag the others with it.
func TestAnUnendedTieInAChord(t *testing.T) {
	lower := tiedTestNote(model.StepC, 4)
	upper := tiedTestNote(model.StepE, 4)
	upper.TieStart = true
	chord := &model.Chord{
		Notes: []*model.Note{lower, upper},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{
			Voices: []*model.Voice{{Elements: []model.Element{
				chord, tiedTestNote(model.StepD, 4), tiedTestNote(model.StepF, 4), tiedTestNote(model.StepG, 4),
			}}},
			RightBarline: model.BarFinal,
		}},
	}
	score := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves, want the one tie", len(lay.Curves))
	}
	if width := lay.Curves[0].X2 - lay.Curves[0].X1; width > 4 {
		t.Errorf("the chord's tie is %.2f spaces wide", width)
	}
}

/*
 * A tie stands off both of the noteheads it joins.
 *
 * Both ends used to be put at the middle of their noteheads, which is where
 * a tie is measured from and not where it is drawn from. A notehead is about
 * half a space tall, so at tieLift below the middle of one the curve came out
 * of the bottom of the head it was leaving and went into the bottom of the
 * head it was arriving at. Reported from the page: "for ties I find the curve
 * too close to the note. It is touching it and it should not."
 */

func TestATieDoesNotTouchTheNotesItJoins(t *testing.T) {
	held := tiedTestNote(model.StepC, 4)
	held.TieStart = true
	stops := tiedTestNote(model.StepC, 4)
	stops.TieStop = true
	score := tieTestBar(held, stops,
		tiedTestNote(model.StepE, 4), tiedTestNote(model.StepF, 4))

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}
	tie := lay.Curves[0]

	hit := lay.Hits[0]
	if len(hit.Elements) < 2 {
		t.Fatal("the bar did not come out with its notes in it")
	}
	// X is a notehead's left edge, and a quarter note's head is one
	// noteheadWidth across.
	head := noteheadWidth(model.Quarter)
	from, to := hit.Elements[0].X, hit.Elements[1].X

	if air := tie.X1 - (from + head); air <= 0 {
		t.Errorf("the tie starts %.2f spaces inside the note it leaves", -air)
	}
	if air := to - tie.X2; air <= 0 {
		t.Errorf("the tie ends %.2f spaces inside the note it arrives at", -air)
	}
	// And still joins them: a curve that has backed away from both notes
	// until there is nothing left of it joins nothing.
	if tie.X2-tie.X1 < tieLeast {
		t.Errorf("the tie is %.2f spaces wide, which is no curve at all",
			tie.X2-tie.X1)
	}
}

// Two notes with no room between them get a short tie rather than none.
func TestATieBetweenNotesStandingCloseIsStillDrawn(t *testing.T) {
	held := tiedTestNote(model.StepC, 4)
	held.TieStart = true
	stops := tiedTestNote(model.StepC, 4)
	stops.TieStop = true
	var notes []*model.Note
	notes = append(notes, held, stops)
	for i := 0; i < 14; i++ {
		notes = append(notes, tiedTestNote(model.StepA, 4))
	}
	score := tieTestBar(notes...)

	opt := DefaultOptions()
	opt.LineWidth = 60 // squeezed to its minimum
	lay := Engrave(score, opt)

	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}
	if w := lay.Curves[0].X2 - lay.Curves[0].X1; w < tieLeast-0.001 {
		t.Errorf("squeezed, the tie came out %.2f spaces wide", w)
	}
}

/*
 * Which way a tie arcs, and saying so by hand.
 *
 * Away from the stems where the staff carries one line of music. The
 * question is whether it carries one, and that is not the same question as
 * whether anything is numbered as a second voice: a bar whose music is all
 * in the second voice with nothing in the first is one line of music however
 * it is numbered. Read as two, every stem in it turned down by number and
 * the ties went to the stem side with them — reported from bar 27 of Bumpty
 * Bump, which is written that way from the repeat onward: "in this situation
 * I want to tie above, not below".
 */

// inSecondVoice is one bar whose music is all in the second voice, with an
// empty first voice in front of it, which is how some files are written.
func inSecondVoice(notes ...*model.Note) *model.Score {
	els := make([]model.Element, 0, len(notes))
	for _, n := range notes {
		els = append(els, n)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{}, {Elements: els}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
}

func TestMusicAloneInTheSecondVoiceIsOneLineOfMusic(t *testing.T) {
	// High notes, which on a staff of their own stem down and so tie over.
	held := tiedTestNote(model.StepD, 5)
	held.TieStart = true
	stops := tiedTestNote(model.StepD, 5)
	stops.TieStop = true
	score := inSecondVoice(tiedTestNote(model.StepD, 5),
		tiedTestNote(model.StepE, 5), held, stops)

	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) != 1 {
		t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
	}
	tie := lay.Curves[0]
	if tie.CY >= (tie.Y1+tie.Y2)/2 {
		t.Errorf("the tie arcs downward, onto the stems: ends %.2f..%.2f, "+
			"arch %.2f", tie.Y1, tie.Y2, tie.CY)
	}
}

// And whichever way the engraver would have put it, it can be said by hand.
func TestATieGoesWhereItIsTold(t *testing.T) {
	arcs := func(dir model.Stem) Curve {
		t.Helper()
		held := tiedTestNote(model.StepD, 5)
		held.TieStart, held.TieDir = true, dir
		stops := tiedTestNote(model.StepD, 5)
		stops.TieStop = true
		lay := Engrave(tieTestBar(held, stops,
			tiedTestNote(model.StepE, 5), tiedTestNote(model.StepF, 5)),
			DefaultOptions())
		if len(lay.Curves) != 1 {
			t.Fatalf("%d curves drawn, want the one tie", len(lay.Curves))
		}
		return lay.Curves[0]
	}

	up := arcs(model.StemUp)
	if up.CY >= (up.Y1+up.Y2)/2 {
		t.Errorf("told up, the tie arcs down: ends %.2f..%.2f, arch %.2f",
			up.Y1, up.Y2, up.CY)
	}
	down := arcs(model.StemDown)
	if down.CY <= (down.Y1+down.Y2)/2 {
		t.Errorf("told down, the tie arcs up: ends %.2f..%.2f, arch %.2f",
			down.Y1, down.Y2, down.CY)
	}
	// And left alone, it is wherever the engraver puts it — which is one of
	// the two, and the point is that saying so changes it.
	auto := arcs(model.StemAuto)
	if (auto.CY < (auto.Y1+auto.Y2)/2) != (up.CY < (up.Y1+up.Y2)/2) &&
		(auto.CY < (auto.Y1+auto.Y2)/2) != (down.CY < (down.Y1+down.Y2)/2) {
		t.Errorf("left alone the tie went a third way: arch %.2f", auto.CY)
	}
}

// One notehead of a chord at a time, because a chord is tied notehead by
// notehead and the others keep the way they were drawn.
func TestOneOfAChordsTiesCanBeTurnedOver(t *testing.T) {
	lower := tiedTestNote(model.StepC, 4)
	upper := tiedTestNote(model.StepE, 4)
	lower.TieStart, upper.TieStart = true, true
	// The lower notehead's tie fans downward on its own; told up, it goes up.
	lower.TieDir = model.StemUp

	chord := &model.Chord{
		Notes: []*model.Note{lower, upper},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	after := &model.Chord{
		Notes: []*model.Note{tiedTestNote(model.StepC, 4), tiedTestNote(model.StepE, 4)},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
	after.Notes[0].TieStop, after.Notes[1].TieStop = true, true

	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{
			Voices: []*model.Voice{{Elements: []model.Element{
				chord, after, tiedTestNote(model.StepG, 4), tiedTestNote(model.StepG, 4),
			}}},
			RightBarline: model.BarFinal,
		}},
	}
	lay := Engrave(&model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}, DefaultOptions())

	if len(lay.Curves) != 2 {
		t.Fatalf("%d curves drawn, want one for each notehead", len(lay.Curves))
	}
	// The upper notehead's tie fans up on its own and the lower one's fans
	// down; the lower one has been told up, so both arc up now.
	for _, c := range lay.Curves {
		if c.CY >= (c.Y1+c.Y2)/2 {
			t.Errorf("a tie still arcs downward: ends %.2f..%.2f, arch %.2f",
				c.Y1, c.Y2, c.CY)
		}
	}
}

// And saying it of one notehead says nothing about the others.
func TestTurningOneOfAChordsTiesLeavesTheRest(t *testing.T) {
	make2 := func(dir model.Stem) []Curve {
		lower := tiedTestNote(model.StepC, 4)
		upper := tiedTestNote(model.StepE, 4)
		lower.TieStart, upper.TieStart = true, true
		lower.TieDir = dir
		chord := &model.Chord{
			Notes: []*model.Note{lower, upper},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		after := &model.Chord{
			Notes: []*model.Note{tiedTestNote(model.StepC, 4), tiedTestNote(model.StepE, 4)},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		after.Notes[0].TieStop, after.Notes[1].TieStop = true, true
		p := &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					chord, after, tiedTestNote(model.StepG, 4), tiedTestNote(model.StepG, 4),
				}}},
				RightBarline: model.BarFinal,
			}},
		}
		lay := Engrave(&model.Score{
			Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
			Parts: []*model.Part{p},
		}, DefaultOptions())
		return lay.Curves
	}

	plain, turned := make2(model.StemAuto), make2(model.StemUp)
	if len(plain) != 2 || len(turned) != 2 {
		t.Fatalf("%d curves plain and %d turned, want two of each",
			len(plain), len(turned))
	}
	up := func(c Curve) bool { return c.CY < (c.Y1+c.Y2)/2 }
	// Left alone, the two fan apart: one up and one down.
	if up(plain[0]) == up(plain[1]) {
		t.Fatalf("the chord's two ties do not fan apart on their own")
	}
	// The one that was already up is where it was.
	for i := range plain {
		if up(plain[i]) && !up(turned[i]) {
			t.Error("turning the other notehead's tie took this one with it")
		}
	}
}
