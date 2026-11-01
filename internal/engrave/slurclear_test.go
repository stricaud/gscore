/*
 * slurclear_test.go
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	_ "github.com/stricaud/gscore/internal/format/abc"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * A slur has to clear the music it arcs over — all of it.
 *
 * Reported from a flute part: a run of thirty-seconds climbing two octaves
 * under one slur, and the slur drawn straight through the noteheads of the
 * first third of it. The arch was being lifted to clear the highest note
 * under the curve, but only at the curve's own middle, where a quadratic
 * is at its furthest from the straight line between its ends. Over a
 * rising run the notes near the start stand well above the curve there,
 * because the curve has barely left its first note yet, and they were cut
 * through.
 *
 * The line a slur draws is the phrase. A slur through the noteheads is the
 * one place it must never be.
 */

// runUp is the shape that was reported: a steep climb and then a stretch
// at the top of it, all under one slur.
//
// The climb is what breaks a curve fitted to its ends alone. The line
// between the first note and the last rises evenly across the whole run,
// while the music is already at the top of it a quarter of the way along,
// so every note of the plateau stands above that line — highest above it
// nearest the start, which is exactly where a curve has least room.
func runUp(climb, fall int, from model.Pitch, dur model.Duration) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	at := from.DiatonicIndex()
	var steps []int
	for i := 0; i < climb; i++ {
		steps = append(steps, at+i)
	}
	// And then falling away from the top of it, which is what makes the
	// line between the first note and the last shallower than the climb
	// and leaves the music of the climb standing above that line.
	top := at + climb - 1
	for i := 1; i <= fall; i++ {
		steps = append(steps, top-i)
	}

	var els []model.Element
	for i, st := range steps {
		n := &model.Note{Pitch: model.PitchFromDiatonicIndex(st), Dur: dur}
		n.SlurStart, n.SlurStop = i == 0, i == len(steps)-1
		els = append(els, n)
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices:       []*model.Voice{{Elements: els}},
		RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)
	return s
}

// noteheadMid is half a notehead, which is the difference between where a
// glyph is placed — its left edge — and where a curve meets it.
const noteheadMid = 0.6

// curveY is where a quadratic curve runs at one x. Its control point sits
// at the middle in x, which makes x move with t evenly and the parameter
// readable straight off the position.
func curveY(c Curve, x float64) (float64, bool) {
	if x <= c.X1 || x >= c.X2 {
		return 0, false
	}
	t := (x - c.X1) / (c.X2 - c.X1)
	u := 1 - t
	return u*u*c.Y1 + 2*t*u*c.CY + t*t*c.Y2, true
}

// cutting reports the worst place the longest curve runs through a
// notehead, in staff spaces, and where that was.
//
// Which side the curve is on is read off the curve itself rather than
// assumed: a slur goes away from the stems, so the same phrase written an
// octave lower is drawn underneath and intrudes downwards.
func cutting(l *Layout) (float64, float64) {
	var curve *Curve
	for i := range l.Curves {
		if curve == nil || l.Curves[i].X2-l.Curves[i].X1 > curve.X2-curve.X1 {
			curve = &l.Curves[i]
		}
	}
	if curve == nil {
		return 0, 0
	}
	// y grows downward, so a curve whose middle is above the line between
	// its ends has the smaller y there.
	above := curve.CY < (curve.Y1+curve.Y2)/2

	worst, where := 0.0, 0.0
	for _, g := range l.Glyphs {
		if !strings.HasPrefix(g.Name, "notehead") {
			continue
		}
		y, ok := curveY(*curve, g.X)
		if !ok {
			continue
		}
		// A notehead is a space tall, so its edge is half a space from
		// the middle it is drawn at.
		cut := y - (g.Y - 0.5)
		if !above {
			cut = (g.Y + 0.5) - y
		}
		if cut > worst {
			worst, where = cut, g.X
		}
	}
	return worst, where
}

func TestASlurClearsARisingRun(t *testing.T) {
	// Up two octaves in eight thirty-seconds and then twenty-four more
	// around the top of it, which is the shape that was reported.
	s := runUp(25, 7, model.Pitch{Step: model.StepC, Octave: 3},
		model.Duration{Num: 1, Den: 32})

	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	lay := Engrave(s, o)

	if len(lay.Curves) == 0 {
		t.Fatal("no slur was drawn at all")
	}
	if cut, where := cutting(lay); cut > 0 {
		t.Errorf("the slur runs %.2f spaces through a notehead at x=%.1f", cut, where)
	}
}

// The same the other way up: a run falling under a slur drawn beneath it.
func TestASlurClearsAFallingRun(t *testing.T) {
	s := runUp(25, 7, model.Pitch{Step: model.StepE, Octave: 6},
		model.Duration{Num: 1, Den: 32})
	// Turned round, so the run falls away from its first note. Low enough
	// that the stems go up and the slur is drawn underneath.
	els := s.Parts[0].Measures[0].Voices[0].Elements
	for i, j := 0, len(els)-1; i < j; i, j = i+1, j-1 {
		els[i], els[j] = els[j], els[i]
	}
	// The marks travel with the places, not with the notes, so they are
	// put back on the ends after the turn.
	for i, el := range els {
		n := el.(*model.Note)
		n.SlurStart, n.SlurStop = i == 0, i == len(els)-1
	}

	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	lay := Engrave(s, o)
	if cut, where := cutting(lay); cut > 0 {
		t.Errorf("the slur runs %.2f spaces through a notehead at x=%.1f", cut, where)
	}
}

// A phrase that is flat is where the old arch was right, and it should not
// have grown an arch it does not need.
func TestAFlatPhraseKeepsItsModestArch(t *testing.T) {
	s := model.NewScore()
	s.Parts = nil
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	var els []model.Element
	for i := 0; i < 8; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		n.SlurStart, n.SlurStop = i == 0, i == 7
		els = append(els, n)
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
	})
	s.Parts = append(s.Parts, p)

	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	lay := Engrave(s, o)
	if len(lay.Curves) == 0 {
		t.Fatal("no slur")
	}
	c := lay.Curves[0]
	// How far the curve's middle stands off the line between its ends, on
	// whichever side it was drawn: eight notes of one pitch put the stems
	// one way and the curve the other.
	rise := (c.Y1+c.Y2)/2 - (0.25*c.Y1 + 0.5*c.CY + 0.25*c.Y2)
	if rise < 0 {
		rise = -rise
	}
	if rise > 3.5 {
		t.Errorf("a slur over eight notes of one pitch rises %.2f spaces", rise)
	}
	if rise <= 0 {
		t.Errorf("it does not arch at all: %.2f", rise)
	}
	if cut, _ := cutting(lay); cut > 0 {
		t.Errorf("and it cuts the notes by %.2f", cut)
	}
}

// And the piece it was reported from.
//
// A property rather than a picture: no slur anywhere in a real score may
// run through a notehead. The Altes solo is the hardest case in the
// samples — a flute study of long slurred runs climbing two octaves at a
// time — and it is where this was first seen.
func TestNoSlurInTheSamplesCutsANote(t *testing.T) {
	files, err := filepath.Glob("../../samples/*.abc")
	if err != nil || len(files) == 0 {
		t.Skip("the samples are not here")
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		imp, err := format.ImporterFor(path)
		if err != nil {
			f.Close()
			continue
		}
		s, _ := imp.Import(f)
		f.Close()
		if s == nil {
			continue
		}

		// As one long strip, so that every bar stands at an x of its own:
		// on a page the systems sit under each other and the same x comes
		// round again on every one of them, which would set a curve on
		// one system against the noteheads of another.
		o := DefaultOptions()
		o.Mode = Strip
		o.WriteAhead = false
		lay := Engrave(s, o)

		// Every slur against every notehead under it, which is the whole
		// of what "a slur must not cut a note" means.
		//
		// Slurs, not every curve: a tie joins two noteheads of one pitch
		// and is drawn between them, so anything that happens to stand
		// under a long one is not something it was ever arcing over.
		worst, where, at := 0.0, 0.0, ""
		for _, a := range lay.Arcs {
			if a.Kind != "slur" {
				continue
			}
			c := Curve{X1: a.X1, Y1: a.Y1, CX: a.CX, CY: a.CY, X2: a.X2, Y2: a.Y2}
			above := c.CY < (c.Y1+c.Y2)/2
			for _, g := range lay.Glyphs {
				if !strings.HasPrefix(g.Name, "notehead") {
					continue
				}
				// A glyph stands at its left edge and a curve ends at a
				// notehead's middle, so the note a curve ends on is a
				// notehead's width inside its own span. Those two notes
				// are the curve's own and it is drawn from them.
				mid := g.X + noteheadMid
				if mid < c.X1+noteheadMid*2 || mid > c.X2-noteheadMid*2 {
					continue
				}
				y, ok := curveY(c, mid)
				if !ok {
					continue
				}
				// Only the staff the curve is on: staves sit about ten
				// spaces apart, and a curve says nothing about the notes
				// on the next one down.
				lo := math.Min(math.Min(c.Y1, c.Y2), c.CY) - 6
				hi := math.Max(math.Max(c.Y1, c.Y2), c.CY) + 6
				if g.Y < lo || g.Y > hi {
					continue
				}
				cut := y - (g.Y - 0.5)
				if !above {
					cut = (g.Y + 0.5) - y
				}
				if cut > worst {
					worst, where, at = cut, g.X, filepath.Base(path)
				}
			}
		}
		// A hair of overlap is the curve's own thickness meeting a
		// notehead's corner; a quarter of a space is the curve drawn
		// through the note.
		if worst > 0.25 {
			t.Errorf("%s: a slur runs %.2f spaces through a notehead at x=%.1f",
				at, worst, where)
		}
	}
}
