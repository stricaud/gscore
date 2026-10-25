/*
 * grace_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// graceScore is one bar with two grace notes in front of its second note.
func graceScore(slashed bool) *model.Score {
	note := func(step model.Step, oct int) *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: step, Octave: oct},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
	}
	main := note(model.StepE, 4)
	main.Grace = &model.GraceGroup{
		Notes:   []*model.Note{note(model.StepG, 4), note(model.StepA, 4)},
		Slashed: slashed,
	}
	v := &model.Voice{Elements: []model.Element{note(model.StepC, 4), main}}
	return &model.Score{Parts: []*model.Part{{
		Clef:     model.Treble,
		Time:     model.TimeSignature{Beats: 2, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
	}}}
}

// notesOfScore lists the notes of a one-voice score, in order.
func notesOfScore(s *model.Score) []*model.Note {
	var out []*model.Note
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok {
						out = append(out, n)
					}
				}
			}
		}
	}
	return out
}

// fullGlyphSize is the type size a SMuFL glyph is drawn at by default: four
// staff spaces, which is the convention the standard rests on.
const fullGlyphSize = 4.0

// isCueSize reports whether a glyph was asked for smaller than normal.
//
// Size is a type size in staff spaces, not a fraction of normal. Reading it
// as a fraction is how the grace notes came to be asked for at a sixth of
// the size meant — so this says what the test actually means and is the one
// place that knows the unit.
func isCueSize(size float64) bool {
	return size > 0 && size < fullGlyphSize
}

// graceNoteheads counts the small noteheads a layout holds.
func graceNoteheads(lay *Layout) int {
	n := 0
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) {
			n++
		}
	}
	return n
}

// graceFlags counts the flags. A group of two or more is beamed instead, so
// there should be none at all except on a group of one.
func graceFlags(lay *Layout) int {
	n := 0
	for _, g := range lay.Glyphs {
		if g.Name == "flag8thUp" {
			n++
		}
	}
	return n
}

func TestGraceNotesAreDrawn(t *testing.T) {
	// Parsed into the model but never drawn is the same loss as not read at
	// all, with the added insult of the file looking as though it opened.
	lay := Engrave(graceScore(false), DefaultOptions())
	if got := graceNoteheads(lay); got != 2 {
		t.Errorf("got %d small noteheads, want 2", got)
	}
}

func TestAGroupOfGraceNotesIsBeamedNotFlagged(t *testing.T) {
	// A flag hangs off the right of its stem far enough to print on the next
	// small notehead, which is what a group of grace notes looked like: two
	// heads 1.1 spaces apart with a flag half a space wide between them.
	// Beaming them is both what fixes that and what an engraver does.
	lay := Engrave(graceScore(false), DefaultOptions())
	if got := graceFlags(lay); got != 0 {
		t.Errorf("got %d flags on a group of two, want none: it should be beamed", got)
	}
}

func TestALoneGraceNoteIsFlagged(t *testing.T) {
	// There is nothing to beam it to.
	s := graceScore(false)
	g := notesOfScore(s)[1].Grace
	g.Notes = g.Notes[:1]

	lay := Engrave(s, DefaultOptions())
	if got := graceFlags(lay); got != 1 {
		t.Errorf("got %d flags on a lone grace note, want 1", got)
	}
}

func TestGraceNotesDoNotCollideWithEachOther(t *testing.T) {
	// Every small notehead has to clear the one before it, flag or no flag.
	lay := Engrave(graceScore(false), DefaultOptions())
	head := 1.18 * graceScale // a notehead is about 1.18 spaces wide

	var xs []float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) {
			xs = append(xs, g.X)
		}
	}
	for i := 1; i < len(xs); i++ {
		if xs[i] < xs[i-1]+head {
			t.Errorf("grace note %d at x=%.2f overlaps the one at x=%.2f, which is %.2f wide",
				i, xs[i], xs[i-1], head)
		}
	}
}

func TestGraceNotesClearTheMainAccidental(t *testing.T) {
	// A sharp in front of the note and a grace group in front of that are two
	// things in the same place unless the group is told about the sharp. It
	// was not, and the group printed over it.
	s := graceScore(false)
	main := notesOfScore(s)[1]
	main.Accidental = model.Sharp
	main.Pitch.Alter = 1

	lay := Engrave(s, DefaultOptions())

	var accX float64
	for _, g := range lay.Glyphs {
		if g.Name == "accidentalSharp" && g.Size == 0 {
			accX = g.X
		}
	}
	if accX == 0 {
		t.Fatal("the main note's sharp was not drawn")
	}
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) && g.X+1.18*graceScale > accX {
			t.Errorf("a grace note at x=%.2f runs into the sharp at x=%.2f", g.X, accX)
		}
	}
}

func TestGraceNotesSitBeforeTheirNote(t *testing.T) {
	lay := Engrave(graceScore(false), DefaultOptions())

	// The note they lean on is the second full-size notehead in the bar.
	var mainX float64
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Size == 0 {
			mainX = g.X // the last full-size head is the one with the graces
		}
	}
	if mainX == 0 {
		t.Fatal("no full-size notehead was drawn")
	}
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) && g.X >= mainX {
			t.Errorf("a grace note at x=%.2f is not before its note at x=%.2f", g.X, mainX)
		}
	}
}

func TestSlashIsDrawnOnlyForAnAcciaccatura(t *testing.T) {
	// The stroke through the stem is the only thing that tells a player to
	// crush the notes in before the beat rather than take their written value
	// out of the note they precede. Drawing it either way round is wrong.
	//
	// It is told from the beam above it by its slope: a beam's is capped at
	// graceBeamSlope, and the stroke's is far steeper than that.
	count := func(s *model.Score) int {
		lay := Engrave(s, DefaultOptions())
		n := 0
		for _, l := range lay.Lines {
			dx, dy := l.X2-l.X1, l.Y2-l.Y1
			if dx == 0 || dy == 0 {
				continue
			}
			if slope := dy / dx; slope < -graceBeamSlope*1.5 {
				n++
			}
		}
		return n
	}
	if got := count(graceScore(true)); got != 1 {
		t.Errorf("{/ge}: got %d strokes, want 1", got)
	}
	if got := count(graceScore(false)); got != 0 {
		t.Errorf("{ge}: got %d strokes, want none", got)
	}
}

func TestAGraceBeamIsNotSteep(t *testing.T) {
	// A beam that follows the pitches exactly reads as an error. Engravers
	// cap the slope; so does this.
	notes := []*model.Note{
		{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: model.Duration{Num: 1, Den: 8}},
		{Pitch: model.Pitch{Step: model.StepB, Octave: 5}, Dur: model.Duration{Num: 1, Den: 8}},
	}
	s := graceScore(false)
	notesOfScore(s)[1].Grace = &model.GraceGroup{Notes: notes}

	lay := Engrave(s, DefaultOptions())
	for _, l := range lay.Lines {
		dx := l.X2 - l.X1
		if dx <= 0 {
			continue
		}
		if slope := (l.Y2 - l.Y1) / dx; slope > graceBeamSlope+0.01 || slope < -graceBeamSlope-0.01 {
			t.Errorf("a line slopes at %.2f, steeper than the cap of %.2f", slope, graceBeamSlope)
		}
	}
}

func TestEveryGraceStemReachesItsBeam(t *testing.T) {
	// A stem that stops short of the beam, or runs through it, is the most
	// obvious kind of wrong there is.
	notes := []*model.Note{
		{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 8}},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 5}, Dur: model.Duration{Num: 1, Den: 8}},
		{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: model.Duration{Num: 1, Den: 8}},
	}
	s := graceScore(false)
	notesOfScore(s)[1].Grace = &model.GraceGroup{Notes: notes}
	lay := Engrave(s, DefaultOptions())

	// The grace notes are the small noteheads; their stems stand just to the
	// right of them. Finding the beam by where it runs rather than by how
	// thick it is keeps this from turning on a number that may change.
	var lo, hi float64 = 1e9, -1e9
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) {
			lo = math.Min(lo, g.X)
			hi = math.Max(hi, g.X)
		}
	}
	if lo > hi {
		t.Fatal("no grace noteheads were drawn")
	}

	var stems []Line
	for _, l := range lay.Lines {
		if l.X1 == l.X2 && lo <= l.X1 && l.X1 <= hi+2 {
			stems = append(stems, l)
		}
	}
	var beam *Quad
	for i, b := range lay.Beams() {
		if x, _ := b.Left(); x >= lo-1 {
			beam = &lay.Quads[i]
			break
		}
	}
	if beam == nil {
		t.Fatal("no beam was drawn over the group")
	}
	if len(stems) != 3 {
		t.Fatalf("got %d stems over %d grace notes", len(stems), len(notes))
	}
	bx0, by0 := beam.Left()
	bx1, by1 := beam.Right()
	for _, st := range stems {
		f := (st.X1 - bx0) / (bx1 - bx0)
		at := by0 + (by1-by0)*f
		top := math.Min(st.Y1, st.Y2)
		if math.Abs(top-at) > beam.Thickness() {
			t.Errorf("a stem tops out at y=%.2f and the beam is at y=%.2f there", top, at)
		}
	}
}

func TestGraceNotesDoNotOverlapWhatComesBefore(t *testing.T) {
	// The room has to be budgeted in the layout, not taken out of the note
	// before: two grace notes drawn into the gap would print over it.
	lay := Engrave(graceScore(false), DefaultOptions())

	var firstRight, graceLeft float64
	graceLeft = 1e9
	for _, g := range lay.Glyphs {
		switch {
		case g.Name == "noteheadBlack" && g.Size == 0 && firstRight == 0:
			firstRight = g.X + 1.0 // a notehead is about one space wide
		case g.Name == "noteheadBlack" && isCueSize(g.Size) && g.X < graceLeft:
			graceLeft = g.X
		}
	}
	if graceLeft < firstRight {
		t.Errorf("the first grace note is at x=%.2f, inside the note before it, which ends at x=%.2f",
			graceLeft, firstRight)
	}
}

func TestBeamEndsAreVertical(t *testing.T) {
	// A beam is a parallelogram whose ends are vertical, so that they finish
	// flush with the stems they join. Drawn as a thick stroked line it gets
	// caps square to its own direction instead, which on any beam that is not
	// level shows as a point sticking out past the outer stem.
	s := graceScore(false)
	lay := Engrave(s, DefaultOptions())

	beams := lay.Beams()
	if len(beams) == 0 {
		t.Fatal("no beams were drawn")
	}
	for _, b := range beams {
		if b.X[0] != b.X[3] {
			t.Errorf("the left end runs from x=%.3f to x=%.3f: it is not vertical", b.X[0], b.X[3])
		}
		if b.X[1] != b.X[2] {
			t.Errorf("the right end runs from x=%.3f to x=%.3f: it is not vertical", b.X[1], b.X[2])
		}
		if b.Thickness() <= 0 {
			t.Errorf("a beam has no thickness")
		}
	}
}

func TestABeamDoesNotOverhangItsStems(t *testing.T) {
	// The outer stems are where the beam stops. A beam reaching past them is
	// the visible half of the same bug as a cap square to the line.
	notes := []*model.Note{
		{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: model.Duration{Num: 1, Den: 8}},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 5}, Dur: model.Duration{Num: 1, Den: 8}},
	}
	s := graceScore(false)
	notesOfScore(s)[1].Grace = &model.GraceGroup{Notes: notes}
	lay := Engrave(s, DefaultOptions())

	var lo, hi float64 = 1e9, -1e9
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && isCueSize(g.Size) {
			lo = math.Min(lo, g.X)
			hi = math.Max(hi, g.X)
		}
	}
	for _, b := range lay.Beams() {
		x0, _ := b.Left()
		x1, _ := b.Right()
		if x0 < lo-1 {
			continue // a beam belonging to the full-size notes
		}
		// A notehead is about 1.18 spaces wide and the stem stands at its
		// right-hand edge, so the beam may not start before the first head
		// nor end more than a head's width past the last.
		if x0 < lo || x1 > hi+1.18 {
			t.Errorf("a beam runs from x=%.2f to x=%.2f over heads at x=%.2f to %.2f",
				x0, x1, lo, hi)
		}
	}
}

// TestEveryRendererDrawsBeams guards against a new drawing primitive reaching
// one renderer and not the others.
//
// Beams moved out of Lines and into Quads to get their ends cut vertically. A
// renderer that was not told would simply stop drawing beams — silently, and
// only in its own output format, which is exactly the kind of thing nobody
// notices until a printed part comes back wrong.
func TestEveryRendererDrawsBeams(t *testing.T) {
	lay := Engrave(graceScore(false), DefaultOptions())
	if len(lay.Beams()) == 0 {
		t.Fatal("the test score has no beams in it")
	}
	// Quads are beams and beams are quads: if that ever stops being true,
	// Beams() has to start filtering and every caller has to be revisited.
	if len(lay.Beams()) != len(lay.Quads) {
		t.Error("not every quad is a beam any more; Beams() needs to say which are")
	}
}

func TestGraceGlyphsAreAskedForAtCueSize(t *testing.T) {
	// Size is a type size in staff spaces and a SMuFL glyph's default is
	// four of them, so a cue glyph is four times the cue scale. Passing the
	// scale alone asked for a glyph a sixth of the size meant: the PDF drew
	// specks, and the SVG — through a bug of its own in which a stylesheet
	// rule beat the size attribute — drew them full size. Neither renderer
	// drew what was asked for, and they disagreed with each other.
	lay := Engrave(graceScore(false), DefaultOptions())

	n := 0
	for _, g := range lay.Glyphs {
		if !isCueSize(g.Size) {
			continue
		}
		n++
		want := fullGlyphSize * graceScale
		if math.Abs(g.Size-want) > 0.001 {
			t.Errorf("%s is drawn at size %.3f, want %.3f", g.Name, g.Size, want)
		}
	}
	if n == 0 {
		t.Fatal("nothing was drawn at cue size")
	}
}
