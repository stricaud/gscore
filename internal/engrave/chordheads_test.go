/*
 * chordheads_test.go
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
	"github.com/stricaud/gscore/internal/smufl"
)

// Two noteheads a second apart cannot share a column: drawn there they are
// one through the other. One of each pair moves to the other side of the
// stem, and which one is not a free choice — the chord is read from the stem
// outwards, the note nearest it stays, and from there they alternate.
//
// The rule used to be "move me if anything is a second from me", which in a
// cluster of three moved the middle note and the top one both and left those
// two in the same column a second apart: the one thing the rule exists to
// prevent.

func chordOf(steps ...model.Step) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	c := &model.Chord{Dur: q}
	for i, st := range steps {
		c.Notes = append(c.Notes, &model.Note{
			Pitch: model.Pitch{Step: st, Octave: 4 + i/7}, Dur: q,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: []model.Element{c}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func heads(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "notehead") {
			out = append(out, g)
		}
	}
	return out
}

// noTwoSecondsInAColumn is the rule itself, asked of whatever was drawn.
func noTwoSecondsInAColumn(t *testing.T, lay *Layout, what string) {
	t.Helper()
	hs := heads(lay)
	for i := range hs {
		for j := i + 1; j < len(hs); j++ {
			a, b := hs[i], hs[j]
			if math.Abs(a.X-b.X) > 0.05 {
				continue // different columns, nothing to answer for
			}
			// Half a staff space apart in y is one step: a second.
			if math.Abs(math.Abs(a.Y-b.Y)-0.5) < 0.05 {
				t.Errorf("%s: two noteheads a second apart share the column at "+
					"x=%.3f, y=%.3f and %.3f", what, a.X, a.Y, b.Y)
			}
		}
	}
}

func TestChordHeadsASecondApartAreDisplaced(t *testing.T) {
	cases := []struct {
		what  string
		steps []model.Step
	}{
		{"a second", []model.Step{model.StepG, model.StepA}},
		{"a third", []model.Step{model.StepG, model.StepB}},
		{"a cluster of three", []model.Step{model.StepG, model.StepA, model.StepB}},
		{"a cluster of four", []model.Step{
			model.StepF, model.StepG, model.StepA, model.StepB}},
		{"a second under a third", []model.Step{
			model.StepG, model.StepA, model.StepC}},
	}
	for _, c := range cases {
		lay := Engrave(chordOf(c.steps...), DefaultOptions())
		if n := len(heads(lay)); n != len(c.steps) {
			t.Errorf("%s: %d noteheads drawn, want %d", c.what, n, len(c.steps))
		}
		noTwoSecondsInAColumn(t, lay, c.what)
	}
}

// A chord with no seconds in it is one column: nothing is moved that does
// not have to be, or a plain triad comes out looking like a cluster.
func TestAChordWithNoSecondsStaysInOneColumn(t *testing.T) {
	lay := Engrave(chordOf(model.StepC, model.StepE, model.StepG), DefaultOptions())
	hs := heads(lay)
	if len(hs) != 3 {
		t.Fatalf("%d noteheads", len(hs))
	}
	for _, h := range hs {
		if math.Abs(h.X-hs[0].X) > 0.05 {
			t.Errorf("a triad was spread over two columns: x=%.3f and %.3f",
				hs[0].X, h.X)
		}
	}
}

// The note nearest the stem is the one that stays put, because that is the
// column the stem is drawn against.
func TestTheNoteNearestTheStemIsNotMoved(t *testing.T) {
	// Low in the staff, so the stem points up and the lowest note anchors.
	lay := Engrave(chordOf(model.StepD, model.StepE), DefaultOptions())
	hs := heads(lay)
	if len(hs) != 2 {
		t.Fatalf("%d noteheads", len(hs))
	}
	// y grows downward: the lower-sounding note has the larger y.
	low, high := hs[0], hs[1]
	if low.Y < high.Y {
		low, high = high, low
	}
	if low.X > high.X {
		t.Errorf("the upper note stayed and the lower one moved: "+
			"low x=%.3f, high x=%.3f", low.X, high.X)
	}
}

// A chord symbol goes above the music, not through it.
//
// It used to be written at a fixed height over the top staff line, which is
// right for a staff whose notes stay between the lines and wrong for one
// with a run three ledger lines up: "Am7" came out printed across its own
// notehead. A chart is mostly chord symbols, so this is most of what a chart
// looks like.
func TestAChordSymbolClearsTheNotesUnderIt(t *testing.T) {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	// High, low, high: the row has to clear the highest of them and stay a
	// row, rather than following the tune up and down.
	for i, octave := range []int{6, 4, 6} {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepC, Octave: octave},
			Dur:   model.Duration{Num: 1, Den: 1},
		}
		n.Chord = &model.ChordSymbol{Root: "C", Quality: "m7"}
		_ = i
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{n}}},
			RightBarline: model.BarSingle,
		})
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	lay := Engrave(s, opt)

	var roots []Text
	for _, t := range lay.Texts {
		if t.S == "C" {
			roots = append(roots, t)
		}
	}
	if len(roots) != 3 {
		t.Fatalf("%d chord symbols drawn, want 3", len(roots))
	}

	// One row: every symbol at the same height.
	for i, r := range roots[1:] {
		if math.Abs(r.Y-roots[0].Y) > 0.01 {
			t.Errorf("chord %d is at y=%.2f and the first at y=%.2f: they are "+
				"not on one line", i+1, r.Y, roots[0].Y)
		}
	}

	// And clear of every notehead and ledger line under them.
	bottom := roots[0].Y + chordSize*0.25
	for _, g := range lay.Glyphs {
		if !has(g.Name, "notehead") {
			continue
		}
		if g.Y-0.55 < bottom {
			t.Errorf("a notehead at y=%.2f is under a chord symbol whose own "+
				"ink reaches %.2f", g.Y, bottom)
		}
	}
	for _, l := range lay.Lines {
		// Ledger lines are short and horizontal, above the staff.
		if math.Abs(l.Y1-l.Y2) > 0.01 || l.X2-l.X1 > 3 {
			continue
		}
		if l.Y1 < bottom && l.Y1 > roots[0].Y-6 {
			t.Errorf("a ledger line at y=%.2f runs through the chord row at %.2f",
				l.Y1, bottom)
		}
	}
}

// qn is one quarter note of a chord.
func qn(step model.Step, octave int) *model.Note {
	return &model.Note{
		Pitch: model.Pitch{Step: step, Octave: octave},
		Dur:   model.Duration{Num: 1, Den: 4},
	}
}

// stemmedChord is one chord at the head of a bar, told which way to point,
// with the rest of the bar left as a rest.
func stemmedChord(stem model.Stem, notes ...*model.Note) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	c := &model.Chord{Dur: q, Notes: notes}
	c.Stem = stem
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					c, &model.Rest{Dur: model.Duration{Num: 3, Den: 4}},
				}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// The note that steps out of the column steps across the stem.
//
// Which note gives way was already right — the chord is read from the stem
// outwards and the second of a pair moves. Which way it went was not: it was
// moved right whether the stem pointed up or down, and a stem-down chord's
// lower note then hung off the far side with the stem running down between
// the two. Reported as "the A should be on the left, otherwise the chord
// looks weird".
func TestADisplacedNoteheadCrossesTheStem(t *testing.T) {
	cases := []struct {
		what string
		stem model.Stem
		// Where the head that gave way ends up, against the one that stayed.
		left bool
	}{
		{"stem down", model.StemDown, true},
		{"stem up", model.StemUp, false},
	}
	for _, c := range cases {
		lay := Engrave(stemmedChord(c.stem,
			qn(model.StepA, 4), qn(model.StepB, 4)), DefaultOptions())

		hs := heads(lay)
		if len(hs) != 2 {
			t.Fatalf("%s: %d noteheads, want 2", c.what, len(hs))
		}
		// y grows downward, so the lower-sounding note has the larger y. It
		// is the one that gives way with the stem down; with the stem up it
		// is the upper one.
		low, high := hs[0], hs[1]
		if low.Y < high.Y {
			low, high = high, low
		}
		moved, stayed := high, low
		if c.left {
			moved, stayed = low, high
		}
		if c.left && moved.X >= stayed.X {
			t.Errorf("%s: the note that gave way is at x=%.3f, the one that "+
				"stayed at %.3f — it should be to its left",
				c.what, moved.X, stayed.X)
		}
		if !c.left && moved.X <= stayed.X {
			t.Errorf("%s: the note that gave way is at x=%.3f, the one that "+
				"stayed at %.3f — it should be to its right",
				c.what, moved.X, stayed.X)
		}

		// And it really is across the stem, not merely beside the chord.
		stem, ok := stemOf(lay, stayed)
		if !ok {
			t.Fatalf("%s: no stem found on the note that stayed", c.what)
		}
		w := noteheadWidth(model.Quarter)
		if c.left && moved.X+w > stem.X1+0.01 {
			t.Errorf("%s: the moved head reaches x=%.3f, past the stem at %.3f",
				c.what, moved.X+w, stem.X1)
		}
		if !c.left && moved.X < stem.X1-0.01 {
			t.Errorf("%s: the moved head starts at x=%.3f, before the stem at %.3f",
				c.what, moved.X, stem.X1)
		}
	}
}

// A ledger line belongs under the notehead it carries.
//
// Drawn at the column instead, a head that stepped out of the column sat
// beside its own ledger line, with the line running out into empty staff and
// nothing under the note.
func TestALedgerLineFollowsTheNoteheadThatSteppedOut(t *testing.T) {
	// A second, well below the staff, so both heads need ledger lines.
	lay := Engrave(stemmedChord(model.StemUp,
		qn(model.StepB, 2), qn(model.StepC, 3)), DefaultOptions())

	hs := heads(lay)
	if len(hs) != 2 {
		t.Fatalf("%d noteheads, want 2", len(hs))
	}
	if math.Abs(hs[0].X-hs[1].X) < 0.05 {
		t.Fatal("neither head stepped out, so there is nothing to answer for")
	}
	// Not at the head's own height: a note sitting in the space between two
	// ledger lines has none of its own. What every head must have is lines
	// drawn under it rather than under the column it stepped out of.
	w := noteheadWidth(model.Quarter)
	for _, g := range hs {
		under := false
		for _, l := range lay.Lines {
			if math.Abs(l.Y1-l.Y2) > 0.01 || l.X2-l.X1 > 3 {
				continue // a staff line or a beam, not a ledger line
			}
			if l.X1 <= g.X+0.01 && l.X2 >= g.X+w-0.01 {
				under = true
			}
		}
		if !under {
			t.Errorf("the notehead at x=%.3f y=%.3f has no ledger line "+
				"reaching under it: they were drawn at the column instead",
				g.X, g.Y)
		}
	}
}

// The accidentals of a chord stand clear of the whole of it.
//
// They are written in a column of their own to the left of the noteheads.
// With a head stepped left into that column, an accidental placed off the
// column alone is printed on the notehead.
func TestAccidentalsClearANoteheadThatSteppedLeft(t *testing.T) {
	flat := func(step model.Step, octave int) *model.Note {
		n := qn(step, octave)
		n.Pitch.Alter = -1
		n.Accidental = model.Flat
		return n
	}
	lay := Engrave(stemmedChord(model.StemDown,
		flat(model.StepA, 4), flat(model.StepB, 4)), DefaultOptions())

	leftmost := math.Inf(1)
	for _, g := range heads(lay) {
		if g.X < leftmost {
			leftmost = g.X
		}
	}
	flats := 0
	for _, g := range lay.Glyphs {
		if g.Name != "accidentalFlat" {
			continue
		}
		flats++
		if right := g.X + smufl.Advance(g.Name); right > leftmost+0.01 {
			t.Errorf("a flat reaches x=%.3f, past the leftmost notehead at %.3f",
				right, leftmost)
		}
	}
	if flats != 2 {
		t.Errorf("%d flats drawn, want 2", flats)
	}
}

// Room for the step is booked before the column as well as after it.
//
// Which side the head goes depends on the stem, and the stem is not settled
// until the music is drawn — a beam decides it for every note under it. So
// both sides are kept free. Without the room in front, a stem-down chord at
// the head of a bar was written on top of the barline that opens it.
func TestAChordAtTheHeadOfABarClearsItsBarline(t *testing.T) {
	d := model.Duration{Num: 1, Den: 2}
	chord := func() *model.Chord {
		c := &model.Chord{Dur: d, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepC, Octave: 5}, Dur: d},
			{Pitch: model.Pitch{Step: model.StepD, Octave: 5}, Dur: d},
		}}
		c.Stem = model.StemDown
		return c
	}
	bar := func() *model.Measure {
		return &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{chord(), chord()}}},
			RightBarline: model.BarSingle,
		}
	}
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef:     model.Treble,
			Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{bar(), bar()},
		}},
	}
	opt := DefaultOptions()
	opt.Mode = Strip
	opt.WriteAhead = false
	lay := Engrave(s, opt)

	line := 0.0
	for _, b := range lay.Barlines {
		if b.Measure == 0 {
			line = b.X
		}
	}
	if line == 0 {
		t.Fatal("no barline drawn between the two bars")
	}
	w := noteheadWidth(model.Half)
	for _, g := range heads(lay) {
		if g.X < line-0.01 && g.X+w > line+0.01 {
			t.Errorf("a notehead spanning %.3f–%.3f is written across the "+
				"barline at %.3f", g.X, g.X+w, line)
		}
	}
}
