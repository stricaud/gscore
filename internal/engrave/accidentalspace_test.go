/*
 * accidentalspace_test.go
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
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

/*
 * An accidental belongs to its note.
 *
 * It is read as part of the note it is written on, so it has to fit inside
 * the room that note already has rather than be given room of its own. Given
 * its own, it pushes its note to the right, and a bar of equal notes says on
 * the page that one of them is longer than the rest.
 *
 * The room is there to be used. A quarter note is given 2.8 staff spaces and
 * its notehead is 1.18 of them; the rest is air, and a sharp is 1.0 wide. So
 * the sharp costs almost nothing, and only where the air runs out — the short
 * values, where the duration's own space is down to the floor — does anything
 * have to be added at all.
 */

// barOfQuarters is a bar of n equal notes, with a sharp on one of them.
// Pass -1 for no accidental.
func barOfEqual(n, sharpOn int) *model.Score {
	s := model.NewScore()
	p := s.Parts[0]
	m := model.NewMeasure()
	var els []model.Element
	for i := 0; i < n; i++ {
		note := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: n},
		}
		if i == sharpOn {
			note.Pitch.Alter = 1
			note.Accidental = model.Sharp
			// Forced, so that SpellAccidentals is not what this is about.
			note.Forced = true
		}
		els = append(els, note)
	}
	m.Voices = []*model.Voice{{Elements: els}}
	p.Measures = []*model.Measure{m}
	return s
}

// noteheadXs is where the noteheads are, whatever shape they are drawn in: a
// half note is not a noteheadBlack, and a test that looked only for those
// found no notes at all in a bar of halves.
func noteheadXs(lay *Layout) []float64 {
	var xs []float64
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "notehead") {
			xs = append(xs, g.X)
		}
	}
	return xs
}

// headInk is the ink of the notehead a duration is drawn with.
func headInk(t *testing.T, d model.Duration) float64 {
	t.Helper()
	v, _, ok := d.Written()
	if !ok {
		t.Fatalf("%v has no written form", d)
	}
	b, ok := smufl.BBox(smufl.NoteheadGlyph(v))
	if !ok {
		t.Fatalf("no box for the notehead of %v", d)
	}
	return b.Right
}

// A note carrying an accidental does not read as late.
//
// Four quarter notes with a sharp on the third used to be spaced 2.800,
// 4.446, 2.800 — the gap in front of the sharpened note 59% wider than the
// others, in a bar where every note is the same length. The rhythm is what
// notation exists to show, and that page showed the wrong one.
func TestAnAccidentalDoesNotMakeItsNoteLate(t *testing.T) {
	plain := noteheadXs(Engrave(barOfEqual(4, -1), DefaultOptions()))
	sharp := noteheadXs(Engrave(barOfEqual(4, 2), DefaultOptions()))
	if len(plain) != 4 || len(sharp) != 4 {
		t.Fatalf("got %d and %d noteheads, want 4 of each", len(plain), len(sharp))
	}

	gaps := func(xs []float64) []float64 {
		var out []float64
		for i := 1; i < len(xs); i++ {
			out = append(out, xs[i]-xs[i-1])
		}
		return out
	}
	was, now := gaps(plain), gaps(sharp)

	// Every gap the same, to within a fortieth of a space. The sharp may
	// cost a little where the air runs short; it may not cost a reader the
	// rhythm.
	for i := range now {
		if math.Abs(now[i]-was[i]) > 0.05 {
			t.Errorf("gap %d is %.3f with a sharp in the bar and %.3f without; "+
				"the bar is four equal notes and should look like it",
				i+1, now[i], was[i])
		}
	}
}

// And it still clears what is written before it.
//
// The sharp is drawn backwards from its own notehead, so room that is not
// reserved is a sharp printed through the notehead in front. Asked at every
// duration, because the air in the gap is the duration's own space and runs
// out at the short values, and at several system widths, because that is
// what stretches the gaps.
func TestAnAccidentalClearsWhatIsBeforeIt(t *testing.T) {
	for _, n := range []int{2, 4, 8, 16, 32} {
		// On the last note, so there is always one before it to clear.
		on := n - 1
		for _, width := range []float64{0, 60, 40, 24} {
			opt := DefaultOptions()
			if width > 0 {
				opt.LineWidth = width
			}
			lay := Engrave(barOfEqual(n, on), opt)

			heads := noteheadXs(lay)
			var sharpX float64
			found := false
			for _, g := range lay.Glyphs {
				if g.Name == "accidentalSharp" {
					sharpX, found = g.X, true
					break
				}
			}
			if !found || len(heads) <= on {
				t.Errorf("1/%d at width %v: no sharp drawn", n, width)
				continue
			}

			// The sharp starts clear of the ink of the note before it, by
			// the air the spacings ask for.
			want := opt.Spacings.BeforeAccidental
			got := sharpX - (heads[on-1] + headInk(t, model.Duration{Num: 1, Den: n}))
			if got < want-0.001 {
				t.Errorf("1/%d at width %v: the sharp starts %.3f after the "+
					"notehead before it, and the spacings ask for %.3f",
					n, width, got, want)
			}
		}
	}
}

// columnInk is where the ink ends, which is not where the advance says the
// next thing would start. The difference is the air an accidental is tucked
// into, so reading the wrong one would hand out room that is not there.
func TestColumnInkIsInkAndNotAdvance(t *testing.T) {
	e := &engraver{opt: DefaultOptions()}

	whole := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 1},
	}
	got := e.columnInk(slot{elems: []voiced{{el: whole}}})
	box, ok := smufl.BBox("noteheadWhole")
	if !ok {
		t.Fatal("no box for a whole notehead")
	}
	if math.Abs(got-box.Right) > 0.001 {
		t.Errorf("a whole notehead's ink is %.3f, want the font's %.3f", got, box.Right)
	}

	// A dot hangs off the right of the notehead and is ink too, so a dotted
	// note leaves less air behind it than a plain one.
	dotted := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 3, Den: 8},
	}
	withDot := e.columnInk(slot{elems: []voiced{{el: dotted}}})
	if withDot <= got {
		t.Errorf("a dotted note's ink is %.3f and a plain one's %.3f; the dot "+
			"takes room", withDot, got)
	}
}

// An accidental in the tightest bar there is still has to be paid for: the
// air runs out, and then the gap opens.
func TestWhereTheAirRunsOutTheGapOpens(t *testing.T) {
	short := noteheadXs(Engrave(barOfEqual(16, -1), DefaultOptions()))
	sharp := noteheadXs(Engrave(barOfEqual(16, 2), DefaultOptions()))
	if len(short) < 4 || len(sharp) < 4 {
		t.Fatal("not enough noteheads")
	}
	plainGap := short[2] - short[1]
	sharpGap := sharp[2] - sharp[1]
	if sharpGap <= plainGap {
		t.Errorf("at a sixteenth the gap is %.3f with a sharp and %.3f "+
			"without; there is no air left to tuck it into, so it has to "+
			"open", sharpGap, plainGap)
	}
}
