/*
 * multirest_test.go
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

/*
 * Bars of rest, counted instead of printed one by one.
 *
 * What makes this worth testing rather than looking at is everything it must
 * not swallow. A rehearsal mark, a change of key, a double barline, a volta,
 * a dynamic left on a rest: each of them is printed at a bar, and a bar that
 * has been counted instead of drawn is a bar with nowhere to print it. The
 * sign is easy; knowing where it has to stop is the whole of the work.
 */

// restBars is a score of one or more staves, silent except where notes are
// asked for. notes[staff] lists the bars that staff plays in.
func restBars(bars int, staves int, notes map[int][]int) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for st := 0; st < staves; st++ {
		p := &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		plays := map[int]bool{}
		for _, at := range notes[st] {
			plays[at] = true
		}
		for i := 0; i < bars; i++ {
			var el model.Element = &model.Rest{Dur: model.Duration{Num: 1, Den: 1}}
			if plays[i] {
				el = &model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				}
			}
			p.Measures = append(p.Measures, &model.Measure{
				Voices:       []*model.Voice{{Elements: []model.Element{el}}},
				RightBarline: model.BarSingle,
			})
		}
		s.Parts = append(s.Parts, p)
	}
	return s
}

func multiOpts(on bool) Options {
	o := DefaultOptions()
	o.Mode = Strip
	o.WriteAhead = false
	o.MultiRests = on
	return o
}

// hBars counts the thick horizontal strokes of a multi-measure rest, which
// is one per staff that is resting.
func hBars(l *Layout) int {
	n := 0
	for _, ln := range l.Lines {
		if ln.Y1 == ln.Y2 && ln.X2 > ln.X1 &&
			math.Abs(ln.Thickness-multiRestThickness) < 1e-9 {
			n++
		}
	}
	return n
}

// counted is the numbers written over them, as the glyphs they are set in.
//
// The count is set in the same figures as a time signature, so a glyph is
// only one of these when it is standing over one of the strokes: otherwise
// the staff's own 4/4 is read as a count of four.
func counted(l *Layout) []string {
	var out []string
	for _, g := range l.Glyphs {
		if len(g.Name) <= 7 || g.Name[:7] != "timeSig" {
			continue
		}
		for _, ln := range l.Lines {
			if ln.Y1 != ln.Y2 || math.Abs(ln.Thickness-multiRestThickness) > 1e-9 {
				continue
			}
			if g.X >= ln.X1-1 && g.X <= ln.X2+1 && g.Y < ln.Y1 {
				out = append(out, g.Name)
				break
			}
		}
	}
	return out
}

func barlineCount(l *Layout) int { return len(l.Barlines) }

// The plain case: eight bars, four of them silent on every staff.
func TestSilentBarsAreCountedNotDrawn(t *testing.T) {
	s := restBars(8, 2, map[int][]int{0: {0, 1, 6, 7}, 1: {0, 1, 6, 7}})

	plain := Engrave(s, multiOpts(false))
	if got := hBars(plain); got != 0 {
		t.Fatalf("the option is off and %d of them were drawn anyway", got)
	}

	got := Engrave(s, multiOpts(true))
	if n := hBars(got); n != 2 {
		t.Errorf("%d multi-measure rests drawn, want one on each of the two staves", n)
	}
	// Bars 3, 4, 5 and 6 are the silent ones, so the sign says four.
	want := []string{"timeSig4", "timeSig4"}
	if names := counted(got); len(names) != len(want) {
		t.Errorf("the count over it is %v, want a 4 on each staff", names)
	}
	// Three barlines went with the three bars that are no longer drawn, on
	// each of the two staves.
	if a, b := barlineCount(plain), barlineCount(got); a-b != 6 {
		t.Errorf("%d barlines became %d; want six fewer", a, b)
	}
}

// A run is as long as the silence, and no longer.
func TestARunStopsWhereAStaffPlays(t *testing.T) {
	// Only the second staff plays in bar 5, which is enough to stop it:
	// there is nowhere on the page for its notes if the bar is not drawn.
	s := restBars(8, 2, map[int][]int{0: {0}, 1: {0, 4}})
	got := Engrave(s, multiOpts(true))

	// Bars 2–4 rest, then bar 5 plays, then bars 6–8 rest: two runs, each
	// drawn on both staves.
	if n := hBars(got); n != 4 {
		t.Fatalf("%d signs drawn, want two runs on two staves", n)
	}
	want := map[string]int{"timeSig3": 4}
	have := map[string]int{}
	for _, n := range counted(got) {
		have[n]++
	}
	if len(have) != len(want) || have["timeSig3"] != want["timeSig3"] {
		t.Errorf("the counts are %v, want two runs of three on each staff", have)
	}
}

// Two bars is the shortest thing worth counting; one is just a bar of rest.
func TestASingleSilentBarIsLeftAlone(t *testing.T) {
	s := restBars(4, 1, map[int][]int{0: {0, 2, 3}})
	if n := hBars(Engrave(s, multiOpts(true))); n != 0 {
		t.Errorf("one bar of rest was drawn as a multi-measure rest")
	}
}

// Everything that has to be printed at a bar stops the run there, because a
// bar that is counted instead of drawn cannot carry it.
func TestWhatBreaksARun(t *testing.T) {
	key := model.KeySignature{Fifths: 3}
	for _, c := range []struct {
		what string
		mark func(m *model.Measure)
	}{
		{"a rehearsal mark", func(m *model.Measure) { m.Marker = "B" }},
		{"a key change", func(m *model.Measure) { m.Key = &key }},
		{"a meter change", func(m *model.Measure) {
			m.Time = &model.TimeSignature{Beats: 3, BeatValue: 4}
		}},
		{"a clef change", func(m *model.Measure) {
			c := model.Bass
			m.Clef = &c
		}},
		{"a tempo mark", func(m *model.Measure) {
			m.Tempo = &model.Tempo{BPM: 92}
		}},
		{"the start of a movement", func(m *model.Measure) { m.SectionTitle = "Trio" }},
	} {
		// Bars 2–7 rest; the mark goes on bar 5, in the middle of the run.
		s := restBars(8, 1, map[int][]int{0: {0, 7}})
		c.mark(s.Parts[0].Measures[4])
		got := Engrave(s, multiOpts(true))

		// Three bars, then the marked bar opening a run of three.
		if n := hBars(got); n != 2 {
			t.Errorf("%s: %d signs, want the run broken into two", c.what, n)
		}
		for _, name := range counted(got) {
			if name != "timeSig3" {
				t.Errorf("%s: a run of %s, want two runs of three", c.what, name)
			}
		}
	}
}

// A mark on a rest is something somebody wrote and expects to see.
func TestARestThatCarriesAMarkIsNotSilence(t *testing.T) {
	// Bars 2 to 5 rest. Left alone they are one run of four.
	s := restBars(6, 1, map[int][]int{0: {0, 5}})
	if names := counted(Engrave(s, multiOpts(true))); len(names) != 1 ||
		names[0] != "timeSig4" {
		t.Fatalf("the four silent bars came out as %v, want one run of four", names)
	}

	// With a p on the third bar's rest, that bar has to be drawn: the run
	// is what is left after it, and the bar before it stands alone.
	r := s.Parts[0].Measures[2].Voices[0].Elements[0].(*model.Rest)
	r.Dynamic = model.Piano

	got := Engrave(s, multiOpts(true))
	if n := hBars(got); n != 1 {
		t.Fatalf("%d signs, want the one run that is left after the p", n)
	}
	if names := counted(got); len(names) != 1 || names[0] != "timeSig2" {
		t.Errorf("the count is %v, want two: the bar with the p on it was counted away", names)
	}
}

// A volta is drawn over the bars it covers, so its bars are never counted.
func TestAVoltaIsNeverInsideARun(t *testing.T) {
	s := restBars(6, 1, map[int][]int{0: {0}})
	for i := 2; i <= 4; i++ {
		s.Parts[0].Measures[i].Ending = &model.Ending{Numbers: []int{1}}
	}
	got := Engrave(s, multiOpts(true))
	if n := hBars(got); n != 0 {
		t.Errorf("%d signs drawn over bars under a volta bracket", n)
	}
}

// A run may finish at a double bar or a repeat — that is where a stretch of
// rest usually does finish — and the line has to be printed even though the
// bar carrying it is not drawn.
func TestTheRunCarriesTheBarlineItEndsWith(t *testing.T) {
	s := restBars(6, 1, map[int][]int{0: {0, 5}})
	s.Parts[0].Measures[3].RightBarline = model.BarDouble
	got := Engrave(s, multiOpts(true))

	// Bars 2–4 close with the double bar, so that is one run; bar 5 is left
	// on its own, which is a bar of rest and not a sign.
	if n := hBars(got); n != 1 {
		t.Fatalf("%d signs, want the run to end at the double bar", n)
	}
	if names := counted(got); len(names) != 1 || names[0] != "timeSig3" {
		t.Errorf("the count is %v, want three", names)
	}
	found := false
	for _, b := range got.Barlines {
		if b.Style == model.BarDouble.String() {
			found = true
		}
	}
	if !found {
		t.Error("the double bar went with the bars that were not drawn")
	}
}

// The bars that are not drawn are not numbered either: their columns are the
// run's own, so every number would be printed on top of the next one.
func TestTheSwallowedBarsAreNotNumbered(t *testing.T) {
	s := restBars(8, 1, map[int][]int{0: {0, 7}})
	o := multiOpts(true)
	o.ShowMeasureNumbers = true
	got := Engrave(s, o)

	at := map[float64]int{}
	for _, tx := range got.Texts {
		at[tx.X]++
	}
	for x, n := range at {
		if n > 1 {
			t.Errorf("%d numbers printed at x=%v, one on top of another", n, x)
		}
	}
}

/*
 * And the bars it swallows do not each get a system of their own.
 *
 * A swallowed bar's column collapses onto the end of the run, so it takes
 * no room and draws nothing. The line breaker scored it exactly as it
 * scored the line ending at the sign, and an equal score does not beat the
 * best — so the system ended at the sign and every bar after it was handed
 * a line to itself with nothing on it. The alto flute of one big band score
 * came out as a page of music followed by four pages of empty staves:
 * fifty-three systems holding one invisible bar each.
 */

// blankSystems names the systems that hold nothing but bars a multi-rest
// swallowed — a staff ruled across the page with a clef on it and no music.
//
// By the bars rather than by the ink: a system like that is still given the
// clef and the key signature at its head, so counting glyphs says it has
// something on it when what it has is a label for nothing.
func blankSystems(s *model.Score, lay *Layout) []SystemPos {
	bars := 0
	for _, p := range s.Parts {
		if n := len(p.Measures); n > bars {
			bars = n
		}
	}
	_, hidden := planMultiRests(s.Parts, bars)
	var out []SystemPos
	for _, sy := range lay.Systems {
		blank := true
		for b := sy.First; b <= sy.Last; b++ {
			if !hidden[b] {
				blank = false
				break
			}
		}
		if blank {
			out = append(out, sy)
		}
	}
	return out
}

func TestABarSwallowedByAMultiRestDoesNotStartASystem(t *testing.T) {
	// Five bars of music and then sixty of silence, which is the shape of
	// a wind part that has finished and is counting to the end.
	s := restBars(65, 1, map[int][]int{0: {0, 1, 2, 3, 4}})
	o := multiOpts(true)
	o.Mode = Paginated
	lay := Engrave(s, o)

	if len(lay.Systems) == 0 {
		t.Fatal("nothing was laid out")
	}
	for _, sy := range blankSystems(s, lay) {
		t.Errorf("a system holds bars %d-%d, every one of them swallowed "+
			"by the multi-rest: a staff ruled for nothing", sy.First, sy.Last)
	}
	if len(lay.Systems) > 4 {
		t.Errorf("%d systems for five bars of music and one multi-rest",
			len(lay.Systems))
	}
	// The last system reaches the end of the piece: the swallowed bars
	// belong to the run's own line.
	if last := lay.Systems[len(lay.Systems)-1]; last.Last != 64 {
		t.Errorf("the last system ends at bar %d, and the piece has 65", last.Last+1)
	}
}

// The line is usually full by the time the sign is placed, and that is the
// case that got onto paper: `ideal >= avail` ends the search at the sign,
// so the bars after it are never even looked at. Narrow lines make every
// width do it.
func TestSwallowedBarsJoinTheLineEvenWhenItIsAlreadyFull(t *testing.T) {
	for _, line := range []float64{30, 40, 50, 60, 80, 100} {
		for _, music := range []int{6, 7, 9, 10, 12} {
			var notes []int
			for i := 0; i < music; i++ {
				notes = append(notes, i)
			}
			s := restBars(60, 1, map[int][]int{0: notes})
			o := multiOpts(true)
			o.Mode = Paginated
			o.LineWidth = line
			lay := Engrave(s, o)
			for _, sy := range blankSystems(s, lay) {
				t.Errorf("line %.0f, %d bars of music: a system holds bars "+
					"%d-%d and every one of them is swallowed",
					line, music, sy.First, sy.Last)
			}
		}
	}
}
