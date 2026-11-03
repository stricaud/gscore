/*
 * crossstaff_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Notes written on the staff next door.
 *
 * The thing that has to be exactly right is the height. A note written
 * across is read against the OTHER staff's clef and drawn at the OTHER
 * staff's position, and either of those got wrong puts the notehead on a
 * line that means a different pitch — which is not a misprint, it is a
 * wrong note, and nothing on the page says so.
 *
 * So these do not check that something was drawn somewhere plausible.
 * They check that a note written across lands on exactly the spot the
 * same pitch occupies when it is written on that staff to begin with.
 */

package engrave

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// pianoWith builds a grand staff: a treble staff holding upper, a bass
// staff holding lower, joined so they are drawn as one instrument.
func pianoWith(upper, lower []model.Element) *model.Score {
	bar := func(els []model.Element) []*model.Measure {
		return []*model.Measure{{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		}}
	}
	time := model.TimeSignature{Beats: 4, BeatValue: 4}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{
			{Name: "Piano", Clef: model.Treble, Time: time, Measures: bar(upper)},
			{Name: "Piano", Clef: model.Bass, Time: time, Measures: bar(lower)},
		},
		Groups: []model.StaffGroup{{First: 0, Last: 1,
			Symbol: model.GroupBrace, Barlines: true, Name: "Piano"}},
	}
}

func eighth(step model.Step, oct int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: oct},
		Dur:   model.Duration{Num: 1, Den: 8},
	}
}

// headYOf is where the one notehead of a drawing sits.
func headYOf(t *testing.T, lay *Layout, which int) float64 {
	t.Helper()
	var heads []Glyph
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") {
			heads = append(heads, g)
		}
	}
	if which >= len(heads) {
		t.Fatalf("only %d noteheads were drawn, wanted number %d",
			len(heads), which+1)
	}
	return heads[which].Y
}

// staffTopsOf is where each staff of a two-staff system begins, read off
// the ruled lines themselves.
//
// Measured inside the one drawing rather than by engraving the music twice
// and comparing. Two layouts do not have their staves in the same places:
// the gap between them is worked out from what is written around them, so
// a score with a note on the upper staff and one with the same note on the
// lower are spaced differently, and comparing the two measures that
// difference rather than anything about the note.
func staffTopsOf(t *testing.T, lay *Layout) (upper, lower float64) {
	t.Helper()
	// The long ones. A ledger line is level and runs left to right too,
	// and a note high above the lower staff draws several — taken for
	// staff lines they shuffle every index along, which is a test that
	// fails on exactly the notes it is there to check.
	widest := 0.0
	for _, l := range lay.Lines {
		if l.Y1 == l.Y2 && l.X2-l.X1 > widest {
			widest = l.X2 - l.X1
		}
	}
	seen := map[float64]bool{}
	var ys []float64
	for _, l := range lay.Lines {
		if l.Y1 != l.Y2 || l.X2-l.X1 < widest*0.5 {
			continue
		}
		y := math.Round(l.Y1*100) / 100
		if !seen[y] {
			seen[y] = true
			ys = append(ys, y)
		}
	}
	sort.Float64s(ys)
	if len(ys) != 10 {
		t.Fatalf("%d staff lines were ruled, want ten: %v", len(ys), ys)
	}
	return ys[0], ys[5]
}

// wantY is where a pitch sits on a staff whose top line is at top, read in
// a given clef.
func wantY(top float64, clef model.Clef, p model.Pitch) float64 {
	return top + stepY(float64(StaffStep(clef, p))) - stepY(float64(staffHeight*2))
}

// The whole of it: a note written across lands on exactly the line that
// pitch occupies on the staff it was written onto, read in that staff's
// clef.
func TestANoteWrittenAcrossLandsWhereThatStaffPutsIt(t *testing.T) {
	for _, tc := range []struct {
		step model.Step
		oct  int
	}{
		{model.StepC, 4}, {model.StepG, 3}, {model.StepE, 3},
		{model.StepA, 4}, {model.StepD, 3},
	} {
		n := eighth(tc.step, tc.oct)
		n.Cross = model.OnStaffBelow
		lay := Engrave(pianoWith([]model.Element{n}, nil), DefaultOptions())

		_, lower := staffTopsOf(t, lay)
		got := headYOf(t, lay, 0)
		want := wantY(lower, model.Bass, n.Pitch)
		if math.Abs(got-want) > 0.01 {
			t.Errorf("%v%d written across sits at %.3f; the bass staff "+
				"writes that pitch at %.3f — %.1f lines out",
				tc.step, tc.oct, got, want, (got-want)/0.5)
		}
	}
}

// Read against the other staff's clef, which is the half of this that
// looks right and is wrong: a C on a ledger line under the treble staff is
// the third space of the bass staff, and a note drawn at the right height
// against the wrong clef is a sixth out with nothing to say so.
func TestANoteWrittenAcrossIsReadAgainstThatStaffsClef(t *testing.T) {
	n := eighth(model.StepC, 4)
	n.Cross = model.OnStaffBelow
	lay := Engrave(pianoWith([]model.Element{n}, nil), DefaultOptions())
	_, lower := staffTopsOf(t, lay)
	got := headYOf(t, lay, 0)

	// Middle C is one ledger line below the treble staff and one ledger
	// line above the bass staff — the same pitch, five lines apart on the
	// page. Reading it in the treble clef and drawing it on the bass
	// staff would put it there.
	wrong := wantY(lower, model.Treble, n.Pitch)
	if math.Abs(got-wrong) < 0.01 {
		t.Errorf("it was drawn on the bass staff but read in the treble "+
			"clef: %.3f", got)
	}
	if want := wantY(lower, model.Bass, n.Pitch); math.Abs(got-want) > 0.01 {
		t.Errorf("it sits at %.3f, want %.3f", got, want)
	}
}

// And the other way: a left-hand note written up on the treble staff.
func TestANoteCanBeWrittenOnTheStaffAbove(t *testing.T) {
	n := eighth(model.StepE, 4)
	n.Cross = model.OnStaffAbove
	lay := Engrave(pianoWith(nil, []model.Element{n}), DefaultOptions())

	upper, _ := staffTopsOf(t, lay)
	got := headYOf(t, lay, 0)
	if want := wantY(upper, model.Treble, n.Pitch); math.Abs(got-want) > 0.01 {
		t.Errorf("written up it sits at %.3f, want %.3f", got, want)
	}
}

// A note with no staff to go to is written where it is, rather than off
// the top of the page.
func TestANoteWithNowhereToGoStaysPut(t *testing.T) {
	up := eighth(model.StepE, 4)
	up.Cross = model.OnStaffAbove // the top staff has nothing above it
	lay := Engrave(pianoWith([]model.Element{up}, nil), DefaultOptions())

	upper, _ := staffTopsOf(t, lay)
	got := headYOf(t, lay, 0)
	if want := wantY(upper, model.Treble, up.Pitch); math.Abs(got-want) > 0.01 {
		t.Errorf("it moved to %.3f from %.3f", got, want)
	}
}

// The beam of a group that crosses goes between the staves, and every stem
// reaches it. A beam left over the upper staff would have its stems running
// down through everything written in the gap.
func TestACrossingBeamGoesBetweenTheStaves(t *testing.T) {
	var els []model.Element
	for i, n := range []*model.Note{
		eighth(model.StepG, 4), eighth(model.StepE, 4),
		eighth(model.StepC, 4), eighth(model.StepG, 3),
	} {
		if i >= 2 {
			n.Cross = model.OnStaffBelow
		}
		els = append(els, n)
	}
	lay := Engrave(pianoWith(els, nil), DefaultOptions())

	upperTop, lowerTop := staffTopsOf(t, lay)
	upperBottom := upperTop + staffHeight

	// The beam is a quad. The one belonging to this group sits in the gap.
	if len(lay.Quads) == 0 {
		t.Fatal("no beam was drawn")
	}
	found := false
	for _, q := range lay.Quads {
		y := q.Y[0]
		if y > upperBottom && y < lowerTop {
			found = true
		}
	}
	if !found {
		var ys []float64
		for _, q := range lay.Quads {
			ys = append(ys, q.Y[0])
		}
		t.Errorf("no beam between %.2f and %.2f; the beams are at %v",
			upperBottom, lowerTop, ys)
	}
}

// Nothing about the music changes, which is the whole point of its being a
// mark rather than a move: the note is still in the voice and bar it was
// in, so what sounds is untouched.
func TestWritingANoteAcrossChangesNothingAboutTheMusic(t *testing.T) {
	n := eighth(model.StepC, 4)
	n.Cross = model.OnStaffBelow
	s := pianoWith([]model.Element{n}, nil)

	if got := len(s.Parts[0].Measures[0].Voices[0].Elements); got != 1 {
		t.Errorf("the upper staff holds %d things", got)
	}
	if len(s.Parts[1].Measures) > 0 &&
		len(s.Parts[1].Measures[0].Voices[0].Elements) != 0 {
		t.Error("something was moved onto the lower staff")
	}
	if n.Pitch.MIDI() != 60 {
		t.Errorf("the note now sounds %d", n.Pitch.MIDI())
	}
}

/*
 * Words for the whole system.
 *
 * A tempo marking is read by everybody, so it goes over the top staff of
 * the system whichever staff it was typed on.
 */

func withText(el model.Element, t model.TextMark) model.Element {
	switch e := el.(type) {
	case *model.Note:
		e.Texts = append(e.Texts, t)
	}
	return el
}

func TestWordsForEverybodyGoOverTheTopStaff(t *testing.T) {
	lower := eighth(model.StepG, 3)
	withText(lower, model.TextMark{Text: "Allegro", Style: model.TextTempo})
	lay := Engrave(pianoWith(
		[]model.Element{eighth(model.StepG, 4)},
		[]model.Element{lower}), DefaultOptions())

	upper, lowerTop := staffTopsOf(t, lay)
	var at float64
	found := false
	for _, tx := range lay.Texts {
		if tx.S == "Allegro" {
			at, found = tx.Y, true
		}
	}
	if !found {
		t.Fatal("the marking was not drawn")
	}
	if at >= upper {
		t.Errorf("it sits at %.2f, which is not above the top staff at %.2f",
			at, upper)
	}
	if at > lowerTop {
		t.Errorf("it sits at %.2f, below the staff it was typed on", at)
	}
}

// And words for one player stay with that player, however far up the page
// the system marking went.
//
// This is what went wrong first: the marking was drawn above the top staff
// during the lower staff's pass, and a staff's outline is whatever went on
// the page during its pass — so the lower staff looked as tall as the
// system, and everything else it had to place was pushed clear of that. A
// "pizz." two bars later came out above the top staff.
func TestWordsForOnePlayerStayWithThatPlayer(t *testing.T) {
	a := eighth(model.StepG, 3)
	withText(a, model.TextMark{Text: "Allegro", Style: model.TextTempo})
	b := eighth(model.StepE, 3)
	withText(b, model.TextMark{Text: "pizz.", Style: model.TextTechnique})
	lay := Engrave(pianoWith(
		[]model.Element{eighth(model.StepG, 4), eighth(model.StepG, 4)},
		[]model.Element{a, b}), DefaultOptions())

	upper, lowerTop := staffTopsOf(t, lay)
	for _, tx := range lay.Texts {
		if tx.S != "pizz." {
			continue
		}
		if tx.Y < upper {
			t.Errorf("pizz. was drawn at %.2f, above the top staff at %.2f — "+
				"it belongs to the lower staff at %.2f", tx.Y, upper, lowerTop)
		}
		// Just above its own staff, not far off somewhere.
		if tx.Y < lowerTop-6 {
			t.Errorf("pizz. is %.2f spaces above its own staff", lowerTop-tx.Y)
		}
		return
	}
	t.Fatal("pizz. was not drawn")
}

// Two of them over the same place are stacked rather than printed on top
// of each other.
func TestTwoSystemMarkingsDoNotCollide(t *testing.T) {
	a := eighth(model.StepG, 3)
	withText(a, model.TextMark{Text: "Allegro", Style: model.TextTempo})
	withText(a, model.TextMark{Text: "To Coda", Style: model.TextDirection})
	lay := Engrave(pianoWith(
		[]model.Element{eighth(model.StepG, 4)},
		[]model.Element{a}), DefaultOptions())

	ys := map[string]float64{}
	for _, tx := range lay.Texts {
		if tx.S == "Allegro" || tx.S == "To Coda" {
			ys[tx.S] = tx.Y
		}
	}
	if len(ys) != 2 {
		t.Fatalf("only %v were drawn", ys)
	}
	if math.Abs(ys["Allegro"]-ys["To Coda"]) < 1 {
		t.Errorf("they were written on top of each other, both at about %.2f",
			ys["Allegro"])
	}
}
