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

/*
 * Beams grouped into the unit the metre is read in.
 *
 * Breaking at every beat is readable and is not what printed music does.
 * Simple quadruple is two units of duple and "these two units must always be
 * clearly distinguished"; simple duple is one unit of the whole bar; simple
 * triple keeps its three beats and must not come out looking like compound
 * duple. A beat that is uneven is not joined to its neighbour, because a
 * unit of mixed values beamed as one says the bar is in some other metre.
 */
func TestBeamsFollowTheMetricalUnit(t *testing.T) {
	eighth := model.Duration{Num: 1, Den: 8}
	sixteenth := model.Duration{Num: 1, Den: 16}

	sizes := func(t model.TimeSignature, d model.Duration, n int) []int {
		var out []int
		for _, g := range beamGroups(placedRun(t, d, n), t) {
			out = append(out, len(g))
		}
		return out
	}
	same := func(got []int, want ...int) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	four := model.TimeSignature{Beats: 4, BeatValue: 4}
	if got := sizes(four, eighth, 8); !same(got, 4, 4) {
		t.Errorf("eight eighths in 4/4 came out as %v, want two units of four: "+
			"beats two and three must never be beamed together", got)
	}
	if got := sizes(four, sixteenth, 16); !same(got, 8, 8) {
		t.Errorf("sixteen sixteenths in 4/4 came out as %v, want 8 and 8", got)
	}

	two := model.TimeSignature{Beats: 2, BeatValue: 4}
	if got := sizes(two, eighth, 4); !same(got, 4) {
		t.Errorf("four eighths in 2/4 came out as %v, want the bar in one", got)
	}

	// Simple triple keeps its beats: beamed in threes it would read as 6/8,
	// which the handbooks call out by name.
	three := model.TimeSignature{Beats: 3, BeatValue: 4}
	if got := sizes(three, eighth, 6); !same(got, 2, 2, 2) {
		t.Errorf("six eighths in 3/4 came out as %v, want three beats of two: "+
			"anything else reads as compound duple", got)
	}

	// Compound is grouped by its pulse, which it already was.
	six := model.TimeSignature{Beats: 6, BeatValue: 8}
	if got := sizes(six, eighth, 6); !same(got, 3, 3) {
		t.Errorf("six eighths in 6/8 came out as %v, want two pulses of three", got)
	}
}

// An uneven beat is not joined to the one beside it.
func TestAnUnevenBeatKeepsItsOwnBeam(t *testing.T) {
	four := model.TimeSignature{Beats: 4, BeatValue: 4}
	// A dotted eighth and a sixteenth on beat one, two eighths on beat two:
	// one unit of the metre, but not one beam.
	var items []*placed
	at := model.Duration{Num: 0, Den: 1}
	add := func(num, den, dots int) {
		d := model.Duration{Num: num, Den: den}
		v, dd := written(d)
		items = append(items, &placed{
			value: v, dots: dd, start: at, note: &model.Note{},
		})
		at = at.Add(d)
	}
	add(3, 16, 1)
	add(1, 16, 0)
	add(1, 8, 0)
	add(1, 8, 0)

	got := beamGroups(items, four)
	if len(got) != 2 {
		var sizes []int
		for _, g := range got {
			sizes = append(sizes, len(g))
		}
		t.Errorf("a dotted beat and an even one came out as %v, want them "+
			"beamed apart", sizes)
	}
}

/*
 * The marks that hang off a note and are not articulations.
 */

// An arpeggio is a wavy line in front of the chord, reaching all of it.
// A chord whose line stopped short of its top note would be asking for half
// a roll.
func TestAnArpeggioReachesTheWholeChord(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	ch := &model.Chord{Dur: q, Notes: []*model.Note{
		{Pitch: model.Pitch{Step: model.StepC, Octave: 4}},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 4}},
		{Pitch: model.Pitch{Step: model.StepG, Octave: 4}},
	}}
	ch.Marks.Arpeggio = &model.Arpeggio{}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices:       []*model.Voice{{Elements: []model.Element{ch}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	var line []Glyph
	var heads []Glyph
	for _, g := range lay.Glyphs {
		switch {
		case g.Name == "arpeggiato":
			line = append(line, g)
		case strings.HasPrefix(g.Name, "notehead"):
			heads = append(heads, g)
		}
	}
	if len(line) < 2 {
		t.Fatalf("the line is %d segments, which is not a line", len(line))
	}
	if len(heads) != 3 {
		t.Fatalf("%d noteheads", len(heads))
	}
	// In front of the chord, not through it.
	for _, g := range line {
		for _, h := range heads {
			if g.X >= h.X {
				t.Errorf("a segment at x=%.2f is not in front of a notehead "+
					"at x=%.2f", g.X, h.X)
				break
			}
		}
	}
	// And reaching from the bottom note to the top.
	top, bottom := math.Inf(1), math.Inf(-1)
	for _, g := range line {
		top = math.Min(top, g.Y-arpeggioHeight)
		bottom = math.Max(bottom, g.Y)
	}
	hTop, hBottom := math.Inf(1), math.Inf(-1)
	for _, h := range heads {
		hTop = math.Min(hTop, h.Y)
		hBottom = math.Max(hBottom, h.Y)
	}
	if top > hTop {
		t.Errorf("the line stops at %.2f and the top note is at %.2f", top, hTop)
	}
	if bottom < hBottom {
		t.Errorf("the line starts at %.2f and the bottom note is at %.2f",
			bottom, hBottom)
	}
}

// Rolling downward is said with an arrowhead at the foot of the line, and
// rolling upward with no arrow at all — which is what makes the arrow mean
// something.
func TestADownwardRollSaysSoWithAnArrow(t *testing.T) {
	roll := func(down bool) bool {
		q := model.Duration{Num: 1, Den: 4}
		ch := &model.Chord{Dur: q, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepC, Octave: 4}},
			{Pitch: model.Pitch{Step: model.StepG, Octave: 4}},
		}}
		ch.Marks.Arpeggio = &model.Arpeggio{Down: down}
		s := model.NewScore()
		s.Parts[0].Measures = []*model.Measure{{
			Voices:       []*model.Voice{{Elements: []model.Element{ch}}},
			RightBarline: model.BarSingle,
		}}
		for _, g := range Engrave(s, DefaultOptions()).Glyphs {
			if g.Name == "arpeggiatoDown" {
				return true
			}
		}
		return false
	}
	if roll(false) {
		t.Error("an upward roll was given an arrowhead")
	}
	if !roll(true) {
		t.Error("a downward roll was drawn without one")
	}
}

// A breath goes over the staff after the note, and never past the barline:
// at the end of a measure it is set just before the line.
func TestABreathSitsOverTheStaffBeforeTheBarline(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	last := &model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: q}
	last.Marks.Break = model.BreathComma
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices: []*model.Voice{{Elements: []model.Element{
			&model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q},
			&model.Note{Pitch: model.Pitch{Step: model.StepD, Octave: 4}, Dur: q},
			&model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: q},
			last,
		}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	var mark *Glyph
	for i, g := range lay.Glyphs {
		if g.Name == "breathMarkComma" {
			mark = &lay.Glyphs[i]
		}
	}
	if mark == nil {
		t.Fatal("no breath mark was drawn")
	}
	// Over the staff: above the top line, whichever way the stems point.
	top := math.Inf(1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.Y1 < top {
			top = l.Y1
		}
	}
	if mark.Y >= top {
		t.Errorf("the breath is at %.2f and the top line at %.2f: it belongs "+
			"over the staff", mark.Y, top)
	}
	// And inside the bar. The barline is the rightmost vertical line.
	bar := 0.0
	for _, l := range lay.Lines {
		if l.X1 == l.X2 && l.X1 > bar {
			bar = l.X1
		}
	}
	if mark.X > bar {
		t.Errorf("the breath is at x=%.2f, past the barline at %.2f",
			mark.X, bar)
	}
}

// A chord's fingerings stand in a column beside it, one per notehead from
// the bottom up. Centred over the noteheads they would be printed on the
// notes above them, which on a chord a third apart is every figure but the
// top one.
func TestAChordsFingeringsStandBesideIt(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	ch := &model.Chord{Dur: q, Notes: []*model.Note{
		{Pitch: model.Pitch{Step: model.StepC, Octave: 4}},
		{Pitch: model.Pitch{Step: model.StepE, Octave: 4}},
		{Pitch: model.Pitch{Step: model.StepG, Octave: 4}},
	}}
	ch.Marks.Fingering = []string{"1", "3", "5"}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices:       []*model.Voice{{Elements: []model.Element{ch}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	var figs []Text
	for _, tx := range lay.Texts {
		if tx.S == "1" || tx.S == "3" || tx.S == "5" {
			figs = append(figs, tx)
		}
	}
	if len(figs) != 3 {
		t.Fatalf("%d figures were drawn, want 3", len(figs))
	}
	// One column, so every figure at the same x.
	for _, f := range figs[1:] {
		if math.Abs(f.X-figs[0].X) > 0.01 {
			t.Errorf("the figures are at x=%.2f and %.2f, which is not a column",
				figs[0].X, f.X)
		}
	}
	// Bottom note first: the lowest figure is the one for the lowest note.
	byY := map[string]float64{}
	for _, f := range figs {
		byY[f.S] = f.Y
	}
	if !(byY["1"] > byY["3"] && byY["3"] > byY["5"]) {
		t.Errorf("the figures read %v from the top down, which is not the "+
			"order a player reads a chord in", byY)
	}
}

// The bow and pick strokes go over the staff whichever way the stems point.
//
// A down-bow written under a stem-up note reads as belonging to a lower line
// of music, and the two strokes are the same symbols a guitarist reads as
// down-stroke and up-stroke, which the handbooks put above the staff without
// exception.
func TestBowAndPickStrokesGoOverTheStaff(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	// A low note, so the stem goes up and the old rule would have put the
	// mark underneath.
	n := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q}
	n.Marks.Articulations = []model.Articulation{model.DownBow, model.Staccato}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices:       []*model.Voice{{Elements: []model.Element{n}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	top := math.Inf(1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.Y1 < top {
			top = l.Y1
		}
	}
	var bow, dot *Glyph
	for i, g := range lay.Glyphs {
		switch {
		case strings.Contains(g.Name, "DownBow"):
			bow = &lay.Glyphs[i]
		case strings.Contains(g.Name, "staccato"):
			dot = &lay.Glyphs[i]
		}
	}
	if bow == nil {
		t.Fatal("no bow mark was drawn")
	}
	if bow.Y >= top {
		t.Errorf("the bow mark is at %.2f and the top line at %.2f", bow.Y, top)
	}
	// And the staccato dot stays where it belongs, on the notehead's own
	// side: it is a mark on the note and not a direction over the staff.
	if dot != nil && dot.Y < top {
		t.Error("the staccato dot was moved over the staff with the bow mark")
	}
}

/*
 * The two spanners that are lines rather than arcs.
 */

// A pedal mark is read under the staff, always: a line with a hook turned
// up at each end and a V wherever the player changes it.
func TestAPedalMarkRunsUnderTheStaff(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	a := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q}
	b := &model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: q}
	c := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: q}
	a.Marks.Pedal = &model.PedalMark{Start: true}
	b.Marks.Pedal = &model.PedalMark{Change: true}
	c.Marks.Pedal = &model.PedalMark{Stop: true}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices: []*model.Voice{{Elements: []model.Element{
			a, b, c, &model.Rest{Dur: q},
		}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	var span *SpanPos
	for i := range lay.Spans {
		if lay.Spans[i].Kind == "pedal" {
			span = &lay.Spans[i]
		}
	}
	if span == nil {
		t.Fatal("no pedal mark was recorded, so nothing could be clicked to " +
			"take it away")
	}
	// The staff's own bottom line, not merely the lowest horizontal line
	// drawn: the pedal mark is itself a horizontal line, and down there.
	// The staff lines are the ones that begin at the left margin.
	left := math.Inf(1)
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X1 < left {
			left = l.X1
		}
	}
	bottom := 0.0
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && math.Abs(l.X1-left) < 0.01 && l.Y1 > bottom {
			bottom = l.Y1
		}
	}
	if span.Y2 <= bottom {
		t.Errorf("the pedal line is at %.2f and the bottom staff line at %.2f: "+
			"it belongs under the staff", span.Y2, bottom)
	}
	// And it runs from the note it opened on to the one it closed on.
	if span.X2 <= span.X1 {
		t.Errorf("it runs from %.2f to %.2f", span.X1, span.X2)
	}
	// The V in the middle: two short strokes rising off the line between
	// its ends, which nothing else down there draws.
	notch := 0
	for _, l := range lay.Lines {
		if l.X1 == l.X2 || l.Y1 == l.Y2 {
			continue
		}
		if l.X1 > span.X1 && l.X2 < span.X2 && l.Y1 > bottom {
			notch++
		}
	}
	if notch != 2 {
		t.Errorf("the pedal change is drawn with %d strokes, want the two of "+
			"a V", notch)
	}
}

// The old form is "Ped." under the first note and a star under the last.
// A great deal of printed music has it, and a program that cannot show it
// cannot show that music.
func TestTheOldPedalFormIsDrawnWhenAskedFor(t *testing.T) {
	q := model.Duration{Num: 1, Den: 4}
	a := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q}
	b := &model.Note{Pitch: model.Pitch{Step: model.StepE, Octave: 4}, Dur: q}
	a.Marks.Pedal = &model.PedalMark{Start: true, Text: true, Half: true}
	b.Marks.Pedal = &model.PedalMark{Stop: true, Text: true}
	s := model.NewScore()
	s.Parts[0].Measures = []*model.Measure{{
		Voices:       []*model.Voice{{Elements: []model.Element{a, b}}},
		RightBarline: model.BarSingle,
	}}

	lay := Engrave(s, DefaultOptions())
	var ped, up bool
	for _, g := range lay.Glyphs {
		switch g.Name {
		case "keyboardPedalPed":
			ped = true
		case "keyboardPedalUp":
			up = true
		}
	}
	if !ped || !up {
		t.Errorf("Ped.=%v star=%v: the old form was asked for and not drawn",
			ped, up)
	}
	// And the half pedal says so.
	half := false
	for _, tx := range lay.Texts {
		if tx.S == "1/2" {
			half = true
		}
	}
	if !half {
		t.Error("a half pedal was not marked as one")
	}
}

// A slide runs between the two noteheads it joins, clear of both: a line
// that touched either would read as a tie gone wrong.
func TestASlideRunsBetweenItsTwoNotes(t *testing.T) {
	for _, wavy := range []bool{false, true} {
		q := model.Duration{Num: 1, Den: 4}
		a := &model.Note{Pitch: model.Pitch{Step: model.StepC, Octave: 4}, Dur: q}
		b := &model.Note{Pitch: model.Pitch{Step: model.StepG, Octave: 5}, Dur: q}
		a.Marks.Glissando = &model.Glissando{Start: true, Wavy: wavy, Text: "gliss."}
		b.Marks.Glissando = &model.Glissando{Stop: true, Wavy: wavy}
		s := model.NewScore()
		s.Parts[0].Measures = []*model.Measure{{
			Voices:       []*model.Voice{{Elements: []model.Element{a, b}}},
			RightBarline: model.BarSingle,
		}}

		lay := Engrave(s, DefaultOptions())
		var span *SpanPos
		for i := range lay.Spans {
			if lay.Spans[i].Kind == "glissando" {
				span = &lay.Spans[i]
			}
		}
		if span == nil {
			t.Fatalf("wavy=%v: no slide was recorded", wavy)
		}
		// Between the two noteheads, and not reaching either of them.
		var first, last Glyph
		for _, g := range lay.Glyphs {
			if !strings.HasPrefix(g.Name, "notehead") {
				continue
			}
			if first.Name == "" || g.X < first.X {
				first = g
			}
			if g.X > last.X {
				last = g
			}
		}
		// Clear of both, measured as distance from each notehead's middle:
		// on a steep slide most of that clearance is vertical, so a test
		// on x alone says nothing.
		mid := noteheadWidth(model.Quarter) / 2
		for _, h := range []Glyph{first, last} {
			d := math.Min(
				math.Hypot(span.X1-(h.X+mid), span.Y1-h.Y),
				math.Hypot(span.X2-(h.X+mid), span.Y2-h.Y))
			if d < glissGap*0.9 {
				t.Errorf("wavy=%v: the slide comes within %.2f of the notehead "+
					"at %.2f, and has to start and finish clear of it",
					wavy, d, h.X)
			}
		}
		if span.X1 >= span.X2 {
			t.Errorf("wavy=%v: the slide runs %.2f..%.2f", wavy, span.X1, span.X2)
		}
		// The word over it, on the upper side whichever way the slide runs.
		var label *Text
		for i := range lay.Texts {
			if lay.Texts[i].S == "gliss." {
				label = &lay.Texts[i]
			}
		}
		if label == nil {
			t.Fatalf("wavy=%v: the word over the slide was not drawn", wavy)
		}
		midY := (span.Y1 + span.Y2) / 2
		if label.Y >= midY {
			t.Errorf("wavy=%v: the word is at %.2f and the middle of the line "+
				"at %.2f, so it is printed under it", wavy, label.Y, midY)
		}
	}
}
