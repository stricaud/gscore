/*
 * handbook_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Rules every notation handbook states, checked against what is drawn.
 *
 * These came out of reading one of them against gscore. Each was a defect
 * nothing else caught: three of them because the page looked plausible and
 * one because the code that was meant to fix it had its sign the wrong way
 * round and so did nothing at all. They are written down here because none
 * of them is the sort of thing anybody notices twice.
 */

package engrave

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

// slurShoulder lifts an end over what stands just inside it — in both
// directions.
//
// It did neither. The lift was computed as (y - want) rather than
// (want - y), which comes out negative exactly when there is something to
// clear, so the function returned the end untouched every time. Nothing
// showed it because the arch clears most of what the shoulder is for, and
// the one case it cannot — a note just inside the end — is the case the
// shoulder exists for and the one nobody had a picture of.
func TestTheSlurShoulderActuallyLifts(t *testing.T) {
	// A phrase that leaps up out of its first note and then climbs: the
	// second note stands just inside the slur's own end, where the arch
	// cannot reach it and only the shoulder can.
	s := model.NewScore()
	p := s.Parts[0]
	m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
	steps := []int{0, 5, 6, 7, 8, 9, 10, 11}
	for i, st := range steps {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.Step(st % 7), Octave: 4 + st/7},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		n.Marks.SlurStart = i == 0
		n.Marks.SlurStop = i == len(steps)-1
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	p.Measures = []*model.Measure{m}

	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	lay := Engrave(s, o)

	var slur *ArcPos
	for i := range lay.Arcs {
		if lay.Arcs[i].Kind == "slur" {
			slur = &lay.Arcs[i]
		}
	}
	if slur == nil {
		t.Fatal("no slur was drawn")
	}
	c := Curve{X1: slur.X1, Y1: slur.Y1, CX: slur.CX, CY: slur.CY,
		X2: slur.X2, Y2: slur.Y2}
	above := c.CY < (c.Y1+c.Y2)/2

	// Every notehead the curve passes over has to stay clear of it. The
	// second note of this phrase is the one that was cut: the shoulder
	// computed its lift with the subtraction the wrong way round, so it
	// came out negative exactly when there was something to clear and the
	// end was never raised at all.
	worst := 0.0
	for _, g := range lay.Glyphs {
		if !strings.HasPrefix(g.Name, "notehead") {
			continue
		}
		mid := g.X + noteheadMid
		if mid <= c.X1 || mid >= c.X2 {
			continue
		}
		y, ok := curveY(c, mid)
		if !ok {
			continue
		}
		cut := y - (g.Y - 0.5)
		if !above {
			cut = (g.Y + 0.5) - y
		}
		if cut > worst {
			worst = cut
		}
	}
	if worst > 0.25 {
		t.Errorf("the slur runs %.2f spaces through a notehead just inside "+
			"its own end, which is what the shoulder is for", worst)
	}
}

// A beam falls by what the interval between its ends allows, and no more.
//
// Capping the slope bounds how steep a beam is, not how far it falls, which
// on a group spread across a bar is no bound at all: a run in the Altes came
// out slanting 3.8 staff spaces where the handbooks say half a space to two
// and very seldom more.
func TestNoBeamInTheSamplesSlantsTooFar(t *testing.T) {
	for _, path := range handbookScores(t) {
		lay := handbookLayout(t, path)
		for _, q := range lay.Beams() {
			if d := math.Abs(q.Y[0] - q.Y[1]); d > 2.01 {
				t.Errorf("%s: a beam falls %.2f staff spaces",
					filepath.Base(path), d)
				break
			}
		}
	}
}

// And the table itself, which is what that bound is made of.
func TestABeamsSlantFollowsTheInterval(t *testing.T) {
	e := &engraver{opt: DefaultOptions(), scale: 1}
	at := func(step int) *placed {
		return &placed{lo: step, hi: step, value: model.Eighth}
	}
	for _, c := range []struct {
		name     string
		from, to int
		want     float64
	}{
		{"the same pitch is horizontal", 4, 4, 0},
		{"a second", 4, 5, 0.5},
		{"a third", 4, 6, 1},
		{"a fourth", 4, 7, 1.5},
		{"a seventh", 1, 7, 1.5},
		{"an octave", 1, 8, 2},
	} {
		g := []*placed{at(c.from), at(c.to)}
		if got := e.beamSlantFor(g, true); got != c.want {
			t.Errorf("%s: %v spaces, want %v", c.name, got, c.want)
		}
		// The same interval the other way round falls by the same amount.
		g = []*placed{at(c.to), at(c.from)}
		if got := e.beamSlantFor(g, true); got != c.want {
			t.Errorf("%s, descending: %v spaces, want %v", c.name, got, c.want)
		}
	}

	// Out on ledger lines it is half a space whatever the interval: the
	// stems there are long and a steep beam over them reads as a wedge.
	far := []*placed{{lo: -4, hi: -4, value: model.Eighth},
		{lo: 4, hi: 4, value: model.Eighth}}
	if got := e.beamSlantFor(far, true); got != 0.5 {
		t.Errorf("over ledger lines: %v spaces, want 0.5", got)
	}
}

// A third and a fourth beam are added towards the noteheads, so the stems
// have to grow to keep the notes clear of the lowest one.
//
// Without it a run of 32nds has its lowest beam sitting on its own
// noteheads, which is the one thing a beam must never touch.
func TestDeepBeamsLengthenTheStems(t *testing.T) {
	reach := func(den int) float64 {
		s := model.NewScore()
		p := s.Parts[0]
		m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
		for k := 0; k < 4; k++ {
			m.Voices[0].Elements = append(m.Voices[0].Elements,
				&model.Note{
					Pitch: model.Pitch{Step: 2, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: den},
				})
		}
		p.Measures = []*model.Measure{m}

		lay := Engrave(s, DefaultOptions())
		// How far the stems reach from the noteheads: the notes are all one
		// pitch, so every stem is the same and the beam is horizontal.
		head, top := 0.0, math.Inf(1)
		for _, g := range lay.Glyphs {
			if strings.HasPrefix(g.Name, "notehead") {
				head = g.Y
			}
		}
		for _, l := range lay.Lines {
			if l.X1 == l.X2 && l.Y2 < top {
				top = l.Y2
			}
		}
		return head - top
	}

	// An eighth and a sixteenth carry one and two beams and need no extra
	// room; a 32nd carries three and a 64th four, and each of those two
	// pushes the stems out by the height of the beam it added.
	eighth, sixteenth := reach(8), reach(16)
	if math.Abs(sixteenth-eighth) > 0.01 {
		t.Errorf("a sixteenth's stem reaches %.2f and an eighth's %.2f: a "+
			"second beam needs no more room", sixteenth, eighth)
	}
	thirtySecond, sixtyFourth := reach(32), reach(64)
	if thirtySecond <= sixteenth {
		t.Errorf("a 32nd's stem reaches %.2f spaces and a 16th's %.2f: the "+
			"third beam was added without room for it",
			thirtySecond, sixteenth)
	}
	if sixtyFourth <= thirtySecond {
		t.Errorf("a 64th's stem reaches %.2f and a 32nd's %.2f: the fourth "+
			"beam was added without room for it", sixtyFourth, thirtySecond)
	}
	// Each by about the beam it added, which is what the handbooks call
	// approximately a staff space.
	for _, grew := range []float64{thirtySecond - sixteenth, sixtyFourth - thirtySecond} {
		if grew < 0.5 || grew > 1.2 {
			t.Errorf("the stems grew by %.2f spaces for one more beam", grew)
		}
	}
}

// A fractional beam is as long as a notehead is wide.
func TestAFractionalBeamIsANoteheadWide(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
	// A dotted eighth and a sixteenth: the sixteenth carries the stub.
	m.Voices[0].Elements = append(m.Voices[0].Elements,
		&model.Note{Pitch: model.Pitch{Step: 2, Octave: 4},
			Dur: model.Duration{Num: 3, Den: 16}},
		&model.Note{Pitch: model.Pitch{Step: 2, Octave: 4},
			Dur: model.Duration{Num: 1, Den: 16}})
	p.Measures = []*model.Measure{m}

	lay := Engrave(s, DefaultOptions())
	// The shortest beam in it is the stub; the full one runs the group.
	shortest := math.Inf(1)
	for _, q := range lay.Beams() {
		if w := math.Abs(q.X[1] - q.X[0]); w < shortest {
			shortest = w
		}
	}
	want := noteheadWidth(model.Sixteenth)
	if math.Abs(shortest-want) > 0.05 {
		t.Errorf("the fractional beam is %.2f spaces and a notehead is %.2f",
			shortest, want)
	}
}

func handbookScores(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("../../samples/*.abc")
	more, _ := filepath.Glob("testdata/suite/*.musicxml")
	files = append(files, more...)
	if len(files) == 0 {
		t.Skip("the samples are not here")
	}
	return files
}

func handbookLayout(t *testing.T, path string) *Layout {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	imp, err := format.ImporterFor(path)
	if err != nil {
		t.Skip(err)
	}
	s, _ := imp.Import(f)
	if s == nil {
		t.Skip("nothing could be read from " + path)
	}
	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	return Engrave(s, o)
}
