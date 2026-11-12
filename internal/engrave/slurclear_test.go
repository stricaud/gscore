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
		n.SlurStart, n.SlurStop = model.SlurEnd(i == 0), model.SlurEnd(i == len(steps)-1)
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
		n.SlurStart, n.SlurStop = model.SlurEnd(i == 0), model.SlurEnd(i == len(els)-1)
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
		n.SlurStart, n.SlurStop = model.SlurEnd(i == 0), model.SlurEnd(i == 7)
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
// nearestStaff says which staff a y belongs to: the one whose lines it is
// nearest, measuring from the band itself so that anything between two
// staves goes to the closer.
func nearestStaff(lay *Layout, y float64) int {
	best, at := math.Inf(1), -1
	seen := map[float64]int{}
	for _, h := range lay.Hits {
		if _, ok := seen[h.StaffTop]; ok {
			continue
		}
		seen[h.StaffTop] = len(seen)
		d := 0.0
		switch {
		case y < h.StaffTop:
			d = h.StaffTop - y
		case y > h.StaffTop+h.StaffHeight:
			d = y - (h.StaffTop + h.StaffHeight)
		}
		if d < best {
			best, at = d, seen[h.StaffTop]
		}
	}
	return at
}

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
				// Only the staff the curve is on. A curve says nothing
				// about the notes on the next staff down, and on a score
				// of three staves they are close enough to be caught by
				// any rule of thumb about distance: this one used to
				// allow six spaces either side of the curve, and a
				// notehead on the staff above came inside that by a
				// tenth of a space the moment the staves were packed
				// slightly closer. Each is assigned to the staff it is
				// nearest, which is what "the staff it is on" means.
				if nearestStaff(lay, (c.Y1+c.Y2)/2) != nearestStaff(lay, g.Y) {
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

// quadBand is how far a four-sided shape reaches up and down at one x,
// read by walking its edges.
//
// Beams are what these are, and a beam is a sloping parallelogram: its top
// and bottom at any x are both somewhere between where they are at its two
// ends, so neither can be had from the shape's bounding box without
// claiming the whole corner of the page a steep beam cuts across.
func quadBand(q Quad, x float64) (top, bottom float64, ok bool) {
	top, bottom = math.Inf(1), math.Inf(-1)
	n := len(q.X)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		x1, y1, x2, y2 := q.X[i], q.Y[i], q.X[j], q.Y[j]
		if x1 == x2 {
			if x != x1 {
				continue
			}
		} else if x < math.Min(x1, x2) || x > math.Max(x1, x2) {
			continue
		}
		y := y1
		if x1 != x2 {
			y = y1 + (y2-y1)*(x-x1)/(x2-x1)
		}
		top, bottom = math.Min(top, y), math.Max(bottom, y)
		ok = true
	}
	return top, bottom, ok
}

// worstBeamCut is the deepest a slur runs into a beam anywhere in a
// drawing, and where.
func worstBeamCut(lay *Layout) (float64, float64) {
	worst, where := 0.0, 0.0
	for _, a := range lay.Arcs {
		if a.Kind != "slur" {
			continue
		}
		c := Curve{X1: a.X1, Y1: a.Y1, CX: a.CX, CY: a.CY, X2: a.X2, Y2: a.Y2}
		for _, q := range lay.Quads {
			if len(q.X) == 0 {
				continue
			}
			lo, hi := q.X[0], q.X[0]
			for _, x := range q.X {
				lo, hi = math.Min(lo, x), math.Max(hi, x)
			}
			const steps = 12
			for i := 0; i <= steps; i++ {
				x := lo + (hi-lo)*float64(i)/steps
				// The beam over the slur's own last note is drawn from the
				// same stem the curve ends at, so they meet there by
				// construction and that is not the curve cutting it.
				if x < c.X1+noteheadMid*2 || x > c.X2-noteheadMid*2 {
					continue
				}
				y, ok := curveY(c, x)
				if !ok {
					continue
				}
				top, bottom, ok := quadBand(q, x)
				if !ok || y < top || y > bottom {
					continue
				}
				if cut := math.Min(y-top, bottom-y); cut > worst {
					worst, where = cut, x
				}
			}
		}
	}
	return worst, where
}

// A slur must clear the beams as well as the noteheads.
//
// Bar 58 of the eighth Altes solo: a run of beamed eighths climbing well
// above the staff, and the slur ending on the note after them, which is
// low. The arch was fitted to the middle half of the span only, so the
// beam standing in the last quarter of it was not there to be cleared, and
// the curve came down over the final note straight through the beam.
//
// What sits that near an end is the end's business and not the arch's: a
// control point has next to no purchase a tenth of the way from a corner,
// and asking it to lift the curve there throws the middle of the slur off
// the top of the page. So the end rises over it instead — see slurShoulder.
func TestASlurClearsTheBeamBesideItsLastNote(t *testing.T) {
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	// Four eighths climbing from low to high: most of them are under the
	// middle line, so the group's stems point up and its beam has to be
	// carried above the top of the climb. Then one more eighth, just over
	// the middle line, whose own stem points down — so the stems disagree,
	// the slur goes above, and it has to come down from the beam to a note
	// five spaces below it, in the width of one eighth note.
	//
	// A rest after that one, to keep it out of the beam: a note the beam
	// reaches is a note the curve may meet the beam at, and then there is
	// nothing to see.
	type at struct {
		step   model.Step
		octave int
		den    int
	}
	for _, n := range []at{
		{model.StepC, 4, 8}, {model.StepD, 4, 8},
		{model.StepE, 4, 8}, {model.StepG, 5, 8},
		{model.StepC, 5, 8},
	} {
		m.Voices[0].Elements = append(m.Voices[0].Elements, &model.Note{
			Pitch: model.Pitch{Step: n.step, Octave: n.octave},
			Dur:   model.Duration{Num: 1, Den: n.den},
		})
	}
	m.Voices[0].Elements = append(m.Voices[0].Elements,
		&model.Rest{Dur: model.Duration{Num: 1, Den: 8}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}})
	els := m.Voices[0].Elements
	els[0].(*model.Note).Marks.SlurStart = 1
	els[4].(*model.Note).Marks.SlurStop = 1

	lay := Engrave(s, DefaultOptions())
	if len(lay.Quads) == 0 {
		t.Fatal("nothing beamed, so there is no beam to clear")
	}
	if worst, where := worstBeamCut(lay); worst > 0.25 {
		t.Errorf("the slur runs %.2f spaces through a beam at x=%.1f",
			worst, where)
	}

	// And it clears the beam by rising over it, not by arching over it.
	// Arching is what it used to do, and the beam being two thirds of the
	// way along is what made that so expensive: the curve stood eight
	// spaces off the line between its own ends, a spike over the staff
	// rather than a slur. Clearing the beam and looking like that is not
	// passing this test.
	sl := theSlur(t, lay)
	apex := 0.25*sl.Y1 + 0.5*sl.CY + 0.25*sl.Y2
	if rise := (sl.Y1+sl.Y2)/2 - apex; rise > 5 {
		t.Errorf("the slur stands %.2f spaces off the line between its ends, "+
			"which is a spike and not a slur", rise)
	}
}

// And the same over every sample, as the noteheads are checked above.
func TestNoSlurInTheSamplesCutsABeam(t *testing.T) {
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

		o := DefaultOptions()
		o.Mode = Strip
		o.WriteAhead = false
		lay := Engrave(s, o)

		worst, where := worstBeamCut(lay)
		at := filepath.Base(path)

		// A hair is the curve's thickness meeting a beam's corner; a
		// quarter of a space is the curve drawn through it.
		if worst > 0.25 {
			t.Errorf("%s: a slur runs %.2f spaces through a beam at x=%.1f",
				at, worst, where)
		}
	}
}

// A slur clears the other voice's stems at its own two notes.
//
// Reported from bar 48 of the eighth Altes solo, in the piano: two voices
// on one staff, both slurred between the same two chords, and the two
// curves drawn as a lens with the lower voice's stems run straight through
// the upper one.
//
// A slur goes on the side away from the stems of the notes it covers, and
// with two voices there is no such side — one points up and the other
// down. The curve of the voice that stems down is drawn above, which is
// where the other voice's stems are, and the ends of that curve sit in the
// same two columns of the staff as those stems. What is between the ends
// is lifted over by the arch; the ends are the ones that have to be told.
func TestASlurClearsTheOtherVoicesStems(t *testing.T) {
	s := model.NewScore()
	m := s.Parts[0].Measures[0]
	half := model.Duration{Num: 1, Den: 2}
	// The voice that stems up, written above.
	up := &model.Voice{}
	for i := 0; i < 2; i++ {
		n := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 5}, Dur: half}
		n.Marks.Stem = model.StemUp
		up.Elements = append(up.Elements, n)
	}
	// And the one that stems down, slurred, whose curve therefore goes
	// above and into the other voice's stems.
	down := &model.Voice{}
	for i := 0; i < 2; i++ {
		n := &model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: half}
		n.Marks.Stem = model.StemDown
		down.Elements = append(down.Elements, n)
	}
	down.Elements[0].(*model.Note).Marks.SlurStart = 1
	down.Elements[1].(*model.Note).Marks.SlurStop = 1
	m.Voices = []*model.Voice{up, down}

	lay := Engrave(s, DefaultOptions())
	sl := theSlur(t, lay)
	if sl.CY >= (sl.Y1+sl.Y2)/2 {
		t.Fatalf("the slur is not above: ends %.2f..%.2f, arch %.2f",
			sl.Y1, sl.Y2, sl.CY)
	}

	// The tops of the other voice's stems, in the two columns the slur
	// ends in. A stem is a vertical line; the ledger lines a low note
	// carries are horizontal and are not.
	top := func(x float64) float64 {
		out := math.Inf(1)
		for _, l := range lay.Lines {
			if l.X1 != l.X2 || math.Abs(l.X1-x) > 1 {
				continue
			}
			out = math.Min(out, math.Min(l.Y1, l.Y2))
		}
		return out
	}
	for _, end := range []struct {
		what string
		x, y float64
	}{{"begins", sl.X1, sl.Y1}, {"ends", sl.X2, sl.Y2}} {
		stem := top(end.x)
		if math.IsInf(stem, 0) {
			t.Fatalf("no stem found in the column the slur %s in", end.what)
		}
		// y grows downward, so clear of the stem means above its top.
		if end.y > stem {
			t.Errorf("the slur %s at %.2f and the other voice's stem there "+
				"reaches %.2f, so the stem is drawn through the curve",
				end.what, end.y, stem)
		}
	}
}

// A slur is placed clear of the music, not of other slurs.
//
// It was placed clear of everything drawn, and a slur already drawn was
// part of that. In a bar of four voices each carrying a slur of its own,
// every slur cleared the one before it: the fourth hung thirty-three
// spaces below the staff and dragged the two hands of the piano apart for
// the whole system with it. Bar 75 of the Rhapsody, where the gap between
// the hands came to forty-six spaces with nothing in it.
//
// So the question this asks is not how deep a slur goes — that is the
// chord under it, and the chord has every right to push it — but whether
// the slurs of the other voices add to it. They must not.
func TestSlursDoNotQueueUnderEachOther(t *testing.T) {
	// A right hand of four voices, each a pair of notes. slurAll gives
	// every voice a slur; otherwise only the lowest has one.
	depth := func(slurAll bool) []float64 {
		score := model.NewScore()
		m := score.Parts[0].Measures[0]
		steps := []int{7, 4, 2, 0}
		for v, step := range steps {
			if v > 0 {
				m.Voices = append(m.Voices, &model.Voice{})
			}
			for i := 0; i < 2; i++ {
				n := &model.Note{
					Pitch: model.PitchFromDiatonicIndex(step + i),
					Dur:   model.Duration{Num: 1, Den: 4},
				}
				if slurAll || v == len(steps)-1 {
					n.Marks.SlurStart = model.SlurEnd(i == 0)
					n.Marks.SlurStop = model.SlurEnd(i == 1)
				}
				m.Voices[v].Elements = append(m.Voices[v].Elements, n)
			}
		}
		lay := Engrave(score, DefaultOptions())
		out := []float64{}
		for _, c := range lay.Curves {
			out = append(out, c.CY)
		}
		return out
	}

	alone, crowded := depth(false), depth(true)
	if len(alone) != 1 {
		t.Fatalf("%d slurs drawn for the one voice that has one", len(alone))
	}
	if len(crowded) != 4 {
		t.Fatalf("%d slurs drawn for four voices", len(crowded))
	}
	// The same slur, found among the four by being in the same place: if
	// the others had pushed it, none of them would be.
	for _, c := range crowded {
		if math.Abs(c-alone[0]) < 0.01 {
			return
		}
	}
	t.Errorf("the lowest voice's slur arches at %.2f on its own, and the "+
		"four in the bar arch at %v: it has been placed clear of the "+
		"others", alone[0], crowded)
}
