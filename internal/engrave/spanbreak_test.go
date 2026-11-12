/*
 * spanbreak_test.go
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
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tie or a slur that runs over a line break is cut at the right margin and
// picked up at the left of the next system. What it remembered of its start
// was the height on the page the note it left was at — and the next system's
// staff is a long way further down the drawing, so the continuation was drawn
// from the old staff to the new one: one near-vertical stroke the height of
// the page, through every system in between.

// tiedChain is a run of whole notes, each tied to the next, long enough to
// need several systems.
func tiedChain(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		n := &model.Note{
			Pitch:    model.Pitch{Step: model.StepB, Octave: 4},
			Dur:      model.Duration{Num: 1, Den: 1},
			TieStart: i < bars-1,
			TieStop:  i > 0,
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func TestATieBrokenOverASystemStaysOnItsOwnSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40 // narrow enough to break the chain over several lines
	lay := Engrave(tiedChain(16), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the chain spread over at least three", len(lay.Systems))
	}

	for i, c := range lay.Curves {
		if math.Abs(c.Y2-c.Y1) > staffHeight {
			t.Errorf("curve %d runs from y=%.2f to y=%.2f, %.2f spaces: "+
				"a tie reaching from one system to another",
				i, c.Y1, c.Y2, math.Abs(c.Y2-c.Y1))
		}
	}
}

func TestASlurBrokenOverASystemStaysOnItsOwnSystem(t *testing.T) {
	s := tiedChain(16)
	ms := s.Parts[0].Measures
	for i, m := range ms {
		n := m.Voices[0].Elements[0].(*model.Note)
		n.TieStart, n.TieStop = false, false
		n.SlurStart = model.SlurEnd(i == 0)
		n.SlurStop = model.SlurEnd(i == len(ms)-1)
	}

	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(s, opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the slur spread over at least three", len(lay.Systems))
	}

	for i, c := range lay.Curves {
		// A slur arches, so it is allowed more room than a tie — but never
		// the distance from one staff to the next.
		if math.Abs(c.Y2-c.Y1) > staffHeight {
			t.Errorf("curve %d runs from y=%.2f to y=%.2f: "+
				"a slur reaching from one system to another", i, c.Y1, c.Y2)
		}
	}
}

// A phrase whose notes have stems, and stems pointing both ways, so that the
// slur is drawn on the side its stems are on. The chain above is whole notes,
// which have no stems at all: the stem is what the second half of this bug
// hangs off, and a score of whole notes never reaches it.
func stemmedPhrase(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		var els []model.Element
		for b := 0; b < 4; b++ {
			// The note the phrase begins on stems up, and the ones after it
			// stem down, so the slur is drawn above and its first end hangs
			// off that first stem. That is the case the bug lives in: an end
			// hung off a stem is placed from the stem's own height on the
			// page, and the stem is on the system the phrase began on.
			step, octave := model.StepB, 5
			if i == 0 && b == 0 {
				step, octave = model.StepC, 4
			}
			els = append(els, &model.Note{
				Pitch: model.Pitch{Step: step, Octave: octave},
				Dur:   model.Duration{Num: 1, Den: 4},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		})
	}
	first := p.Measures[0].Voices[0].Elements[0].(*model.Note)
	last := p.Measures[bars-1].Voices[0].Elements[3].(*model.Note)
	first.SlurStart, last.SlurStop = 1, 1
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

// Where a slur is cut at a line break, the half drawn on the next system has
// no note at its left end: that end is the margin the system began at. What
// closed it worked the end out from the note the phrase began on all the same
// — its pitch, its stem, its staff — and that note is on the system above, or
// on the page above. The curve came out as a straight line drawn diagonally
// across the page, from where the phrase began to where it ends, and the
// system it crossed was pushed open to make room for it.
//
// Quilter's Interlude, Op.8 no.7 has one: a slur the engraver of the file left
// open in bar 2 of the left hand, which runs on to bar 6 and so over the page
// turn. The Altes tenth solo has another.
func TestASlurResumedAfterABreakStartsOnItsOwnSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(stemmedPhrase(16), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the phrase spread over at least three",
			len(lay.Systems))
	}

	// A curve belongs to one system: the one its left end is on. Its right
	// end has to be on that system too.
	for i, c := range lay.Curves {
		for _, sys := range lay.Systems {
			if c.Y1 < sys.Top || c.Y1 > sys.Bottom {
				continue
			}
			if c.Y2 < sys.Top || c.Y2 > sys.Bottom {
				t.Errorf("curve %d begins at y=%.2f on a system running "+
					"%.2f to %.2f and ends at y=%.2f, below it: the slur was "+
					"drawn from one system to another",
					i, c.Y1, sys.Top, sys.Bottom, c.Y2)
			}
			break
		}
	}
}

// The stub a tie leaves where a system ends, and the one that picks it up on
// the next, are barely wider than a notehead. Drawn at a tie's full arch they
// rose further than they reached across, and what printed over the note was a
// caret — which on a score is an accent, and means something else entirely.
func TestAShortTieIsAnArcAndNotASpike(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	for i, c := range lay.Curves {
		width := c.X2 - c.X1
		// How far the control point stands off the chord of the curve. It is
		// set at twice the rise, so this is generous either way.
		rise := math.Abs(c.CY - (c.Y1+c.Y2)/2)
		if rise > width {
			t.Errorf("curve %d is %.2f spaces wide and reaches %.2f out: "+
				"it is drawn as a spike, not an arc", i, width, rise)
		}
	}
}

// Where a curve is picked up at the head of a system there has to be
// somewhere to pick it up from. There was not: it began in the column the
// music begins in, which is where the note it joins already sits, so the
// continuation was a notehead wide and arrived from nowhere.
func TestAResumedCurveHasRoomToComeInFrom(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("got %d systems, want the chain broken at least twice", len(lay.Systems))
	}
	for i, c := range lay.Curves {
		if w := c.X2 - c.X1; w < 1 {
			t.Errorf("curve %d is %.2f spaces wide: there was no room to draw it", i, w)
		}
	}
}

// And the stub left behind runs to the end of the line, not to wherever the
// note that starts it happens to be. It used to be handed the width of the
// staff where it wanted the end of it, and stopped four spaces short — in the
// middle of the last bar, which reads as a tie to nothing.
func TestABrokenCurveRunsToTheEndOfItsSystem(t *testing.T) {
	opt := DefaultOptions()
	opt.LineWidth = 40
	lay := Engrave(tiedChain(16), opt)

	// Where the music ends on a full system: the line width plus the blank
	// border the engraver keeps at the left of it.
	end := opt.LineWidth + opt.Spacings.Margin

	// Every system but the last leaves a tie hanging, so every one of them
	// should have a curve reaching the margin.
	for i, sys := range lay.Systems[:len(lay.Systems)-1] {
		found := false
		for _, c := range lay.Curves {
			if c.Y1 < sys.Top || c.Y1 > sys.Bottom {
				continue
			}
			if math.Abs(c.X2-end) < 0.01 {
				found = true
			}
		}
		if !found {
			t.Errorf("system %d leaves a tie open but nothing reaches x=%.2f", i, end)
		}
	}
}

/*
 * The brackets of a repeat.
 *
 * Three things were wrong with them at once in the Tico-Tico, and each
 * was a different mistake: the height came from the whole system rather
 * than the bars under the bracket, the ends were put at the next bar's
 * first note rather than at the barline, and the number was written on
 * the line instead of inside the bracket.
 */

// voltaScore is two endings over two bars, with the repeat between them.
func voltaScore() *model.Score {
	score := model.NewScore()
	p := score.Parts[0]
	for len(p.Measures) < 4 {
		p.AppendMeasure()
	}
	for i, m := range p.Measures {
		m.Voices[0].Elements = []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur: model.Duration{Num: 1, Den: 1}},
		}
		_ = i
	}
	p.Measures[1].Ending = &model.Ending{Numbers: []int{1}, Start: true, Stop: true}
	p.Measures[1].RightBarline = model.BarRepeatEnd
	p.Measures[2].Ending = &model.Ending{Numbers: []int{2}, Start: true, Stop: true}
	return score
}

// bracketsOf finds the horizontal lines of the volta brackets: the long
// level lines above the staff.
func bracketsOf(lay *Layout, top float64) []Line {
	var out []Line
	for _, l := range lay.Lines {
		if math.Abs(l.Y1-l.Y2) < 0.01 && l.Y1 < top-0.5 && l.X2-l.X1 > 2 {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].X1 < out[j].X1 })
	return out
}

func TestAVoltaClosesAtItsBarline(t *testing.T) {
	lay := Engrave(voltaScore(), DefaultOptions())
	top := lay.Hits[0].StaffTop
	br := bracketsOf(lay, top)
	if len(br) < 2 {
		t.Fatalf("%d brackets were drawn, want two", len(br))
	}
	// The barline the first ending closes at: a repeat, drawn inside the
	// bar rather than at the column where the next one starts.
	var repeat float64
	for _, b := range lay.Barlines {
		if b.Measure == 1 {
			repeat = b.X
		}
	}
	if repeat == 0 {
		t.Fatal("no barline was recorded for the first ending")
	}
	if math.Abs(br[0].X2-repeat) > 0.3 {
		t.Errorf("the first bracket ends at %.2f and its barline is at %.2f",
			br[0].X2, repeat)
	}
	// And the second begins after it, not where the first stopped.
	if br[1].X1 <= br[0].X2 {
		t.Errorf("the second bracket begins at %.2f, at or before the end "+
			"of the first at %.2f", br[1].X1, br[0].X2)
	}
}

// The number goes inside the bracket, under the line. Written on the
// line, the bracket runs through the top of the digits.
func TestTheVoltaNumberIsInsideTheBracket(t *testing.T) {
	lay := Engrave(voltaScore(), DefaultOptions())
	top := lay.Hits[0].StaffTop
	br := bracketsOf(lay, top)
	if len(br) == 0 {
		t.Fatal("no bracket was drawn")
	}
	line := br[0].Y1
	var number *Text
	for i := range lay.Texts {
		if lay.Texts[i].S == "1." {
			number = &lay.Texts[i]
		}
	}
	if number == nil {
		t.Fatal("the first ending has no number")
	}
	// A text's baseline is under its letters, so the top of the digits is
	// a cap height above it. All of it must be below the line.
	capHeight := number.Size * 0.72
	if number.Y-capHeight < line {
		t.Errorf("the number's top is at %.2f and the bracket line at %.2f: "+
			"the line goes through the digits", number.Y-capHeight, line)
	}
	// And inside the hook, not hanging below the bracket.
	hook := DefaultOptions().Spacings.VoltaHook
	if number.Y > line+hook+0.1 {
		t.Errorf("the number's baseline is %.2f below the line, past the "+
			"%.2f hook", number.Y-line, hook)
	}
}

// A slur goes into the outline as the arc it is. As the box round its
// ends and its control point it stood at its full height across its whole
// width, and everything above it — bar numbers, and the volta brackets
// above those — was lifted clear of a shape that is not there.
func TestASlurIsMeasuredAsAnArcAndNotAsABox(t *testing.T) {
	s := newSkyline(0, 20, 0, 4)
	addCurve(s, Curve{X1: 0, Y1: 0, CX: 10, CY: -8, X2: 20, Y2: 0})
	// At the ends the arc is at the staff; the box would be eight spaces
	// tall there too.
	if at := s.reach(0, 1, false); at < -2 {
		t.Errorf("the arc is %.1f spaces tall at its left end", -at)
	}
	// And in the middle it really is high.
	if at := s.reach(9, 11, false); at > -3 {
		t.Errorf("the arc is only %.1f spaces tall in the middle", -at)
	}
}
