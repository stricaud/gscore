/*
 * graceslur_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The end of a slur that begins on a grace note.
 *
 * A slur falls on the side away from the stems of the notes it covers, and
 * where that is the side the stems are on anyway it hangs off the stem's end
 * rather than off the notehead. The note it began on is long gone by then,
 * so the stem is recorded when the slur opens — and the grace note's was
 * not. A stem left unsaid reads as one pointing down and ending at y zero,
 * which is the top of the whole drawing, so an appoggiatura whose phrase
 * stems upward was drawn from the title block down to the staff.
 *
 * On one staff that is a wrong curve. On a score of many it is a wrong
 * score: the room between two staves is measured from what is drawn around
 * them, so the oboes of the 1812 Overture were set a hundred spaces below
 * the flutes, and every staff under them was pushed down the page after.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// at is a step and an octave, so a test can say where in the staff a note
// sits — which is what decides the way its stem points and so the side the
// slur falls on.
type at struct {
	step model.Step
	oct  int
}

// appoggiatura is a bar whose second note is leant into by grace notes,
// slurred from the first of them, and whose phrase runs on to the note after.
//
// The slur covers the note after as well as the one leant into, because the
// side it falls on is decided by the stems of the notes it covers and a slur
// that ends on the note it leans into covers none of them. It is also what
// the file that showed this up contains: an appoggiatura inside a phrase,
// which is where one usually is.
func appoggiatura(t *testing.T, grace []at, main, next at) *Layout {
	t.Helper()
	note := func(p at) *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: p.step, Octave: p.oct},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
	}
	var small []*model.Note
	for _, p := range grace {
		small = append(small, note(p))
	}
	small[0].SlurStart = 1

	lean := note(main)
	lean.Grace = &model.GraceGroup{Notes: small}
	end := note(next)
	end.SlurStop = 1

	v := &model.Voice{Elements: []model.Element{lean, end}}
	score := &model.Score{Parts: []*model.Part{{
		Clef:     model.Treble,
		Time:     model.TimeSignature{Beats: 2, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
	}}}
	lay := Engrave(score, DefaultOptions())
	if len(lay.Curves) == 0 {
		t.Fatal("the slur was not drawn at all")
	}
	return lay
}

func TestASlurFromAGraceNoteStartsOnIt(t *testing.T) {
	for _, c := range []struct {
		name       string
		grace      []at
		main, next at
	}{
		// Low notes, so every stem points up and the slur falls below
		// them: the side that reads the stem of the note it began on.
		{"one grace note", []at{{model.StepG, 4}},
			at{model.StepE, 4}, at{model.StepD, 4}},
		{"a beamed pair", []at{{model.StepF, 4}, {model.StepG, 4}},
			at{model.StepE, 4}, at{model.StepD, 4}},
		{"a group of three",
			[]at{{model.StepE, 4}, {model.StepF, 4}, {model.StepG, 4}},
			at{model.StepD, 4}, at{model.StepC, 4}},
		// And the ordinary way up, where the stems point down and the slur
		// arches over them instead.
		{"a phrase that stems downward", []at{{model.StepG, 5}},
			at{model.StepF, 5}, at{model.StepE, 5}},
	} {
		t.Run(c.name, func(t *testing.T) {
			sl := appoggiatura(t, c.grace, c.main, c.next).Curves[0]
			// The two ends are a grace note and the note it leans into, a
			// few spaces apart at most. The fault drew one end at the top
			// of the page, which shows as ends far apart.
			if d := math.Abs(sl.Y1 - sl.Y2); d > 6 {
				t.Errorf("the slur runs from y %.2f to y %.2f, %.1f spaces "+
					"apart: one of its ends is not on a note", sl.Y1, sl.Y2, d)
			}
		})
	}
}

// The curve still runs forwards: from the small note to the one it leans
// into, and not from the main note back to itself.
func TestAGraceSlurRunsFromTheSmallNote(t *testing.T) {
	sl := appoggiatura(t, []at{{model.StepG, 4}},
		at{model.StepE, 4}, at{model.StepD, 4}).Curves[0]
	if sl.X2 <= sl.X1 {
		t.Errorf("the slur runs from x %.2f to x %.2f, which is backwards",
			sl.X1, sl.X2)
	}
	// A grace note stands in front of the note it decorates, within a space
	// or two of it: a slur reaching back further than that has found the
	// wrong note.
	if w := sl.X2 - sl.X1; w > 6 {
		t.Errorf("the slur is %.1f spaces wide, which is wider than a grace "+
			"note stands from the note it leans into", w)
	}
}

// The end is on the small notehead, within the reach of the note's own stem
// — which is as far as a slur ever leaves the note it belongs to.
func TestTheGraceSlurEndsNearTheSmallNotehead(t *testing.T) {
	lay := appoggiatura(t, []at{{model.StepG, 4}},
		at{model.StepE, 4}, at{model.StepD, 4})
	head := math.Inf(1)
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" && g.Size > 0 && g.Size < 4 {
			head = math.Min(head, g.Y)
		}
	}
	if math.IsInf(head, 1) {
		t.Fatal("no grace notehead was drawn")
	}
	if d := math.Abs(lay.Curves[0].Y1 - head); d > 5 {
		t.Errorf("the grace notehead is at y %.2f and the slur begins at "+
			"y %.2f, %.1f spaces from it", head, lay.Curves[0].Y1, d)
	}
}

// What the wrong curve cost the page: the room between two staves is
// measured from what is drawn around them, so one slur reaching out of the
// title block set every staff under it adrift.
func TestAGraceSlurDoesNotPushTheStavesApart(t *testing.T) {
	note := func(s model.Step, oct int) *model.Note {
		return &model.Note{
			Pitch: model.Pitch{Step: s, Octave: oct},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
	}
	part := func(withGrace bool) *model.Part {
		lean := note(model.StepE, 4)
		if withGrace {
			small := note(model.StepG, 4)
			small.SlurStart = 1
			lean.Grace = &model.GraceGroup{Notes: []*model.Note{small}}
		}
		end := note(model.StepD, 4)
		end.SlurStop = model.SlurEnd(withGrace)
		v := &model.Voice{Elements: []model.Element{lean, end}}
		return &model.Part{
			Clef:     model.Treble,
			Time:     model.TimeSignature{Beats: 2, BeatValue: 4},
			Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
		}
	}
	// Four staves, with the appoggiatura well down the page. The fault
	// pinned the curve to the top of the drawing, so what it cost grew with
	// the distance from there — which is why a band score showed it and a
	// song did not.
	gap := func(withGrace bool) float64 {
		var parts []*model.Part
		for i := 0; i < 4; i++ {
			parts = append(parts, part(withGrace && i == 2))
		}
		opt := DefaultOptions()
		opt.ShowPartNames = false
		lay := Engrave(&model.Score{Parts: parts}, opt)
		var lines []float64
		for _, l := range lay.Lines {
			if l.Y1 == l.Y2 && l.X2-l.X1 > 10 {
				lines = append(lines, l.Y1)
			}
		}
		if len(lines) < 20 {
			t.Fatalf("four staves of five lines were not drawn: %d lines",
				len(lines))
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, y := range lines {
			lo, hi = math.Min(lo, y), math.Max(hi, y)
		}
		return hi - lo
	}
	with, without := gap(true), gap(false)
	// An appoggiatura asks for a little room under its staff, and no more
	// than a little.
	if with > without+4 {
		t.Errorf("four staves stand %.1f spaces deep with an appoggiatura on "+
			"the third and %.1f without: the slur is reserving room it does "+
			"not use", with, without)
	}
}
