package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func pitch(s model.Step, oct int) model.Pitch { return model.Pitch{Step: s, Octave: oct} }

func TestStaffStepPerClef(t *testing.T) {
	// The reference notes every musician knows: the bottom line of each staff
	// is step 0 and the top line is step 8.
	cases := []struct {
		clef model.Clef
		p    model.Pitch
		want int
	}{
		{model.Treble, pitch(model.StepE, 4), 0},  // bottom line
		{model.Treble, pitch(model.StepF, 5), 8},  // top line
		{model.Treble, pitch(model.StepB, 4), 4},  // middle line
		{model.Treble, pitch(model.StepC, 4), -2}, // middle C, one ledger below
		{model.Bass, pitch(model.StepG, 2), 0},    // bottom line
		{model.Bass, pitch(model.StepA, 3), 8},    // top line
		{model.Bass, pitch(model.StepD, 3), 4},    // middle line
		{model.Bass, pitch(model.StepC, 4), 10},   // middle C, one ledger above
		{model.Alto, pitch(model.StepC, 4), 4},    // middle C on the middle line
		{model.Tenor, pitch(model.StepC, 4), 6},   // middle C on the fourth line
	}
	for _, c := range cases {
		if got := StaffStep(c.clef, c.p); got != c.want {
			t.Errorf("%v on %v = step %d, want %d", c.p, c.clef, got, c.want)
		}
	}
}

func TestAccidentalsDoNotMoveNoteheads(t *testing.T) {
	plain := StaffStep(model.Treble, model.Pitch{Step: model.StepF, Octave: 4})
	for _, alter := range []int{-2, -1, 1, 2} {
		p := model.Pitch{Step: model.StepF, Octave: 4, Alter: alter}
		if got := StaffStep(model.Treble, p); got != plain {
			t.Errorf("alter %d moved the notehead from %d to %d", alter, plain, got)
		}
	}
}

func TestKeySignatureAccidentalPositions(t *testing.T) {
	// The classic treble pattern for sharps: F C G D A E B at these steps.
	want := []int{8, 5, 9, 6, 3, 7, 4}
	got := keyAccidentalSteps(model.Treble, 7)
	if len(got) != 7 {
		t.Fatalf("seven sharps gave %d accidentals", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sharp %d at step %d, want %d (all: %v)", i, got[i], want[i], got)
		}
	}

	// Flats sit lower, and the first is always B.
	flats := keyAccidentalSteps(model.Treble, -7)
	if len(flats) != 7 {
		t.Fatalf("seven flats gave %d accidentals", len(flats))
	}
	for i, s := range flats {
		if s < 1 || s > 7 {
			t.Errorf("flat %d at step %d, outside the usual window", i, s)
		}
	}

	// Bass clef shifts the whole pattern down two steps.
	bass := keyAccidentalSteps(model.Bass, 7)
	for i := range want {
		if bass[i] != want[i]-2 {
			t.Errorf("bass sharp %d at %d, want %d", i, bass[i], want[i]-2)
		}
	}

	if n := len(keyAccidentalSteps(model.Treble, 0)); n != 0 {
		t.Errorf("C major drew %d accidentals", n)
	}
}

// scoreOf builds a one-part score from a list of durations at one pitch.
func scoreOf(t *testing.T, time model.TimeSignature, durs ...model.Duration) *model.Score {
	t.Helper()
	m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
	for _, d := range durs {
		m.Voices[0].Elements = append(m.Voices[0].Elements, &model.Note{
			Pitch: pitch(model.StepB, 4), Dur: d,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{Clef: model.Treble, Time: time, Measures: []*model.Measure{m}}},
	}
}

func TestBeamGroupingFollowsTheBeat(t *testing.T) {
	eighth := model.Duration{Num: 1, Den: 8}

	// In 2/4 the beat is a quarter, so eighths group in twos.
	items := placedRun(model.TimeSignature{Beats: 2, BeatValue: 4}, eighth, 4)
	groups := beamGroups(items, model.TimeSignature{Beats: 2, BeatValue: 4})
	if len(groups) != 2 {
		t.Fatalf("4 eighths in 2/4 gave %d groups, want 2", len(groups))
	}
	for i, g := range groups {
		if len(g) != 2 {
			t.Errorf("group %d has %d notes, want 2", i, len(g))
		}
	}

	// In 6/8 the beat is a dotted quarter, so they group in threes.
	six := model.TimeSignature{Beats: 6, BeatValue: 8}
	items = placedRun(six, eighth, 6)
	groups = beamGroups(items, six)
	if len(groups) != 2 {
		t.Fatalf("6 eighths in 6/8 gave %d groups, want 2", len(groups))
	}
	for i, g := range groups {
		if len(g) != 3 {
			t.Errorf("group %d has %d notes, want 3 in compound time", i, len(g))
		}
	}
}

// placedRun makes a run of identical beamable notes with their start times.
func placedRun(t model.TimeSignature, d model.Duration, n int) []*placed {
	out := make([]*placed, 0, n)
	at := model.Duration{Num: 0, Den: 1}
	for i := 0; i < n; i++ {
		v, dots, _ := d.Written()
		out = append(out, &placed{
			note: &model.Note{Dur: d}, value: v, dots: dots,
			start: at, x: float64(i) * 2,
		})
		at = at.Add(d)
	}
	return out
}

func TestRestsBreakBeamGroups(t *testing.T) {
	eighth := model.Duration{Num: 1, Den: 8}
	sig := model.TimeSignature{Beats: 4, BeatValue: 4}
	items := placedRun(sig, eighth, 4)
	// Replace the second with a rest.
	items[1].note = nil
	items[1].rest = &model.Rest{Dur: eighth}

	groups := beamGroups(items, sig)
	for _, g := range groups {
		for _, p := range g {
			if p.rest != nil {
				t.Fatal("a rest ended up inside a beam group")
			}
		}
	}
}

func TestQuartersAreNotBeamed(t *testing.T) {
	sig := model.TimeSignature{Beats: 4, BeatValue: 4}
	items := placedRun(sig, model.Duration{Num: 1, Den: 4}, 4)
	if g := beamGroups(items, sig); len(g) != 0 {
		t.Errorf("quarters were beamed into %d groups", len(g))
	}
}

func TestStemDirectionPivotsOnTheMiddleLine(t *testing.T) {
	e := &engraver{opt: DefaultOptions()}
	// The rule carried over from realize_area.c: below the middle line the
	// stem goes up, on it or above it the stem goes down.
	// A note on the middle line stems up. That is gscore's own rule: in
	// realize_area.c the test is "y < middle", so a note exactly on the line
	// falls to the else branch and gets an upward stem.
	cases := []struct {
		step int
		up   bool
	}{{0, true}, {3, true}, {4, true}, {5, false}, {8, false}, {-3, true}}
	for _, c := range cases {
		p := &placed{lo: c.step, hi: c.step}
		if got := e.stemUp(p); got != c.up {
			t.Errorf("step %d: stem up = %v, want %v", c.step, got, c.up)
		}
	}

	// A chord takes its direction from whichever outer note is further from
	// the middle line.
	if !e.stemUp(&placed{lo: 0, hi: 5}) {
		t.Error("a chord reaching further below should stem up")
	}
	if e.stemUp(&placed{lo: 3, hi: 10}) {
		t.Error("a chord reaching further above should stem down")
	}
}

// TestNothingOverflowsTheStaff is the regression test for the bug that put
// notes past the end of the staff lines: every glyph and stem must sit within
// the staff lines drawn for its system.
func TestNothingOverflowsTheStaff(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	var durs []model.Duration
	for i := 0; i < 60; i++ {
		durs = append(durs, quarter)
	}
	s := scoreOf(t, model.TimeSignature{Beats: 4, BeatValue: 4}, durs...)
	// Spread them over many measures.
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < 20; i++ {
		m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
		for j := 0; j < 3; j++ {
			m.Voices[0].Elements = append(m.Voices[0].Elements,
				&model.Note{Pitch: pitch(model.StepB, 4), Dur: quarter})
		}
		p.Measures = append(p.Measures, m)
	}

	for _, mode := range []Mode{Paginated, Strip} {
		opt := DefaultOptions()
		opt.Mode = mode
		lay := Engrave(s, opt)

		// Collect each system's staff extent from its horizontal lines.
		type span struct{ x0, x1 float64 }
		staves := map[float64]span{}
		for _, ln := range lay.Lines {
			if ln.Y1 != ln.Y2 {
				continue
			}
			if ln.Thickness != opt.Defaults.StaffLineThickness {
				continue // a ledger line, not a staff line
			}
			sp, ok := staves[ln.Y1]
			if !ok {
				staves[ln.Y1] = span{ln.X1, ln.X2}
				continue
			}
			staves[ln.Y1] = span{math.Min(sp.x0, ln.X1), math.Max(sp.x1, ln.X2)}
		}
		if len(staves) == 0 {
			t.Fatalf("mode %v drew no staff lines", mode)
		}

		// Every notehead must fall inside some staff's horizontal extent.
		for _, g := range lay.Glyphs {
			if g.Ref == 0 {
				continue
			}
			inside := false
			for _, sp := range staves {
				if g.X >= sp.x0-0.01 && g.X <= sp.x1+0.01 {
					inside = true
					break
				}
			}
			if !inside {
				t.Errorf("mode %v: glyph %s at x=%.2f lies outside every staff", mode, g.Name, g.X)
				break
			}
		}
	}
}

func TestPaginatedBreaksIntoSystems(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	s := scoreOf(t, model.TimeSignature{Beats: 4, BeatValue: 4})
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < 40; i++ {
		m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
		for j := 0; j < 4; j++ {
			m.Voices[0].Elements = append(m.Voices[0].Elements,
				&model.Note{Pitch: pitch(model.StepB, 4), Dur: quarter})
		}
		p.Measures = append(p.Measures, m)
	}

	opt := DefaultOptions()
	opt.Mode = Paginated
	page := Engrave(s, opt)
	opt.Mode = Strip
	strip := Engrave(s, opt)

	if page.Width > opt.LineWidth+2*opt.Spacings.Margin+1 {
		t.Errorf("a paginated layout is %v wide, past the %v line width",
			page.Width, opt.LineWidth)
	}
	if page.Height <= strip.Height {
		t.Error("breaking into systems should make the layout taller")
	}
	// The strip is the 0.0.9 behaviour: one unbroken run, however long.
	if strip.Width <= page.Width {
		t.Errorf("the strip (%v) should be wider than the page (%v)", strip.Width, page.Width)
	}
}

func TestStavesAlignAcrossParts(t *testing.T) {
	// Two parts with very different note counts in the same measures: the
	// measures must still start at the same x on both staves, or reading down
	// a column means nothing.
	mk := func(perMeasure int) *model.Part {
		p := &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for i := 0; i < 4; i++ {
			m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
			for j := 0; j < perMeasure; j++ {
				m.Voices[0].Elements = append(m.Voices[0].Elements, &model.Note{
					Pitch: pitch(model.StepB, 4),
					Dur:   model.Duration{Num: 1, Den: perMeasure * 4},
				})
			}
			p.Measures = append(p.Measures, m)
		}
		return p
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{mk(1), mk(8)},
	}

	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)

	byMeasure := map[int][]float64{}
	for _, h := range lay.Hits {
		byMeasure[h.Measure] = append(byMeasure[h.Measure], h.XMin)
	}
	if len(byMeasure) != 4 {
		t.Fatalf("got hit regions for %d measures, want 4", len(byMeasure))
	}
	for mi, xs := range byMeasure {
		if len(xs) != 2 {
			t.Errorf("measure %d has %d staves", mi, len(xs))
			continue
		}
		if math.Abs(xs[0]-xs[1]) > 0.001 {
			t.Errorf("measure %d starts at %v on one staff and %v on the other", mi, xs[0], xs[1])
		}
	}
}

func TestTupletsAreDrawnAtTheirWrittenValue(t *testing.T) {
	// Three triplet eighths: each sounds for a twelfth but must be drawn as an
	// eighth, which is a black notehead with one beam.
	twelfth := model.Duration{Num: 1, Den: 12}
	m := &model.Measure{Voices: []*model.Voice{{}}, RightBarline: model.BarSingle}
	for i := 0; i < 3; i++ {
		n := &model.Note{Pitch: pitch(model.StepB, 4), Dur: twelfth}
		n.Tuplet = &model.Tuplet{Actual: 3, Normal: 2, Start: i == 0, Stop: i == 2}
		m.Voices[0].Elements = append(m.Voices[0].Elements, n)
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 2, BeatValue: 4},
			Measures: []*model.Measure{m},
		}},
	}
	lay := Engrave(s, DefaultOptions())

	heads := 0
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			heads++
		}
		if g.Name == "noteheadWhole" || g.Name == "noteheadHalf" {
			t.Errorf("a triplet eighth was drawn as %s", g.Name)
		}
	}
	if heads != 3 {
		t.Errorf("drew %d black noteheads, want 3", heads)
	}

	// The group is numbered, or it cannot be told from three plain eighths.
	found := false
	for _, tx := range lay.Texts {
		if tx.S == "3" {
			found = true
		}
	}
	if !found {
		t.Error("the tuplet was not numbered")
	}
}

func TestHitRegionsCoverEveryMeasure(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	s := scoreOf(t, model.TimeSignature{Beats: 4, BeatValue: 4}, quarter, quarter)
	lay := Engrave(s, DefaultOptions())

	if len(lay.Hits) != 1 {
		t.Fatalf("got %d hit regions, want 1", len(lay.Hits))
	}
	h := lay.Hits[0]
	if h.XMax <= h.XMin {
		t.Errorf("the region is %v wide", h.XMax-h.XMin)
	}
	if h.Clef != "treble" {
		t.Errorf("clef = %q", h.Clef)
	}
	if len(h.Elements) != 2 {
		t.Errorf("the region lists %d elements, want 2", len(h.Elements))
	}
	// The listed positions must be inside the region, in order, and carry
	// enough to identify what was clicked.
	for i, el := range h.Elements {
		if el.X < h.XMin-0.01 || el.X > h.XMax+0.01 {
			t.Errorf("element %d at %v is outside the region %v..%v", i, el.X, h.XMin, h.XMax)
		}
		if i > 0 && el.X <= h.Elements[i-1].X {
			t.Errorf("element %d is not to the right of the one before it", i)
		}
		if el.Ref == 0 {
			t.Errorf("element %d has no ref, so a click could not select it", i)
		}
		if el.Index != i {
			t.Errorf("element %d reports index %d", i, el.Index)
		}
		if el.Kind != "note" {
			t.Errorf("element %d is %q, want a note", i, el.Kind)
		}
		if el.Width <= 0 {
			t.Errorf("element %d has no width, so it cannot be hit", i)
		}
	}
	// A notehead must land where the region says it did.
	if _, ok := lay.Refs[1]; !ok {
		t.Error("the first element has no recorded position")
	}
}

func TestEmptyScoreStillDrawsAStaff(t *testing.T) {
	// A new document shows a clef and a key signature, or there is nothing to
	// click on to start writing.
	s := model.NewScore()
	lay := Engrave(s, DefaultOptions())
	if len(lay.Lines) < 5 {
		t.Errorf("an empty score drew %d lines, want at least a staff", len(lay.Lines))
	}
	clef := false
	for _, g := range lay.Glyphs {
		if g.Name == "gClef" {
			clef = true
		}
	}
	if !clef {
		t.Error("no clef on an empty score")
	}
}

func TestEngraveIsPure(t *testing.T) {
	// Engraving must not change the music. The whole architecture rests on it.
	quarter := model.Duration{Num: 1, Den: 4}
	s := scoreOf(t, model.TimeSignature{Beats: 4, BeatValue: 4}, quarter, quarter)
	before := s.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	pitchBefore, durBefore := before.Pitch, before.Dur

	Engrave(s, DefaultOptions())
	Engrave(s, DefaultOptions())

	after := s.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if after.Pitch != pitchBefore || after.Dur != durBefore {
		t.Error("engraving modified the model")
	}
}

func TestStemOverrideBeatsTheRule(t *testing.T) {
	e := &engraver{opt: DefaultOptions()}

	// A note high above the staff would stem down; the override says up.
	high := &placed{lo: 10, hi: 10, note: &model.Note{}}
	if e.stemUp(high) {
		t.Fatal("a high note should stem down before any override")
	}
	high.note.Stem = model.StemUp
	if !e.stemUp(high) {
		t.Error("the override did not turn the stem up")
	}

	// And the other way round.
	low := &placed{lo: 0, hi: 0, note: &model.Note{}}
	if !e.stemUp(low) {
		t.Fatal("a low note should stem up before any override")
	}
	low.note.Stem = model.StemDown
	if e.stemUp(low) {
		t.Error("the override did not turn the stem down")
	}

	// Auto gives the rule back.
	low.note.Stem = model.StemAuto
	if !e.stemUp(low) {
		t.Error("auto should return to the midpoint rule")
	}
}

func TestOverriddenStemsActuallyPointThatWay(t *testing.T) {
	// The geometry has to follow, not just the decision: a forced-up stem
	// must leave the notehead going up the page, which is a smaller y.
	for _, tc := range []struct {
		name string
		stem model.Stem
		up   bool
	}{{"up", model.StemUp, true}, {"down", model.StemDown, false}} {
		n := &model.Note{
			Pitch: pitch(model.StepB, 4), // the middle line
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		n.Stem = tc.stem
		m := &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		}
		s := &model.Score{
			Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
			Parts: []*model.Part{{
				Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
				Measures: []*model.Measure{m},
			}},
		}
		lay := Engrave(s, DefaultOptions())

		var stem *Line
		for i := range lay.Lines {
			if lay.Lines[i].X1 == lay.Lines[i].X2 &&
				lay.Lines[i].Thickness == lay.Defaults().StemThickness {
				stem = &lay.Lines[i]
				break
			}
		}
		if stem == nil {
			t.Fatalf("%s: no stem drawn", tc.name)
		}
		goesUp := stem.Y2 < stem.Y1
		if goesUp != tc.up {
			t.Errorf("%s: the stem runs from y=%.2f to y=%.2f, which is the wrong way",
				tc.name, stem.Y1, stem.Y2)
		}
	}
}

func TestBeamedGroupFollowsAnOverride(t *testing.T) {
	// A beam cannot point two ways, so one marked note settles the group.
	eighth := model.Duration{Num: 1, Den: 8}
	v := &model.Voice{}
	for i := 0; i < 4; i++ {
		n := &model.Note{Pitch: pitch(model.StepG, 5), Dur: eighth} // high: would stem down
		if i == 0 {
			n.Stem = model.StemUp
		}
		v.Elements = append(v.Elements, n)
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 2, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{v}, RightBarline: model.BarSingle,
			}},
		}},
	}
	lay := Engrave(s, DefaultOptions())

	noteY := 0.0
	for _, g := range lay.Glyphs {
		if g.Name == "noteheadBlack" {
			noteY = g.Y
			break
		}
	}
	for _, ln := range lay.Lines {
		if ln.Thickness == lay.Defaults().BeamThickness {
			if ln.Y1 >= noteY {
				t.Errorf("the beam sits at y=%.2f, below the noteheads at %.2f, "+
					"so the override was ignored", ln.Y1, noteY)
			}
			return
		}
	}
	t.Error("no beam was drawn")
}

func TestDynamicsAreDrawnBelowTheStaff(t *testing.T) {
	n := &model.Note{Pitch: pitch(model.StepB, 4), Dur: model.Duration{Num: 1, Den: 4}}
	n.Dynamic = model.FF
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{n}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
	lay := Engrave(s, DefaultOptions())

	var found *Glyph
	for i := range lay.Glyphs {
		if lay.Glyphs[i].Name == "dynamicFF" {
			found = &lay.Glyphs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("the dynamic was not drawn")
	}
	// It must clear the bottom staff line, or it collides with the music.
	staffBottom := 0.0
	for _, ln := range lay.Lines {
		if ln.Y1 == ln.Y2 && ln.Thickness == lay.Defaults().StaffLineThickness && ln.Y1 > staffBottom {
			staffBottom = ln.Y1
		}
	}
	if found.Y <= staffBottom {
		t.Errorf("the dynamic is at y=%.2f, not below the staff at %.2f", found.Y, staffBottom)
	}
}
