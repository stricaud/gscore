package engrave

import (
	"math"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// An articulation has to clear both the note it belongs to and the staff.
//
// Clearing one is not enough, and the two pull in opposite directions: a note
// inside the staff needs its mark pushed out past the lines, while a note
// outside them needs it pushed further out still. Satisfying only the staff
// dragged the marks of high and low notes back onto the noteheads.
func TestArticulationClearsNoteAndStaff(t *testing.T) {
	// Steps from well below the staff to well above it. The bottom line is
	// 0 and the top is 8, so these run from four spaces under to four over.
	for step := -8; step <= 16; step++ {
		s := model.NewScore()
		p := s.Parts[0]
		n := &model.Note{
			Pitch: PitchForStep(model.Treble, step),
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		n.AddArticulation(model.Marcato)
		n.AddArticulation(model.Staccato)
		p.Measures[0].Voices[0].Elements = []model.Element{n}

		opt := DefaultOptions()
		opt.Mode = Strip
		opt.WriteAhead = false
		lay := Engrave(s, opt)

		staffTop := lay.Hits[0].StaffTop
		noteY := 0.0
		var marks []Glyph
		for _, g := range lay.Glyphs {
			switch g.Name {
			case "noteheadBlack":
				noteY = g.Y
			case "articMarcatoAbove", "articMarcatoBelow",
				"articStaccatoAbove", "articStaccatoBelow":
				marks = append(marks, g)
			}
		}
		if len(marks) != 2 {
			t.Fatalf("step %d: got %d articulations, want 2", step, len(marks))
		}

		for _, g := range marks {
			above := g.Y < noteY
			// Clear of the notehead by at least a space.
			if gap := noteY - g.Y; above && gap < 1 {
				t.Errorf("step %d: a mark above sits %.2f from the notehead", step, gap)
			}
			if gap := g.Y - noteY; !above && gap < 1 {
				t.Errorf("step %d: a mark below sits %.2f from the notehead", step, gap)
			}
			// And outside the staff, never on the lines between them.
			inStaff := g.Y > staffTop-0.01 && g.Y < staffTop+staffHeight+0.01
			if inStaff {
				t.Errorf("step %d: a mark at y=%.2f is inside the staff (%.2f..%.2f)",
					step, g.Y, staffTop, staffTop+staffHeight)
			}
		}

		// The two marks must not sit on each other either.
		if d := marks[0].Y - marks[1].Y; d < 1 && d > -1 {
			t.Errorf("step %d: two articulations %.2f apart", step, d)
		}
	}
}

/*
 * A mark that belongs to a notehead is centred on it.
 */

// Every articulation sits over the middle of its note.
//
// A glyph is drawn from its origin, which is its left edge and not its
// middle, so a mark put at the note's centre lands half its own width to the
// right of it. A tenuto came out 0.674 staff spaces off — more than half a
// notehead — and on the page that is a row of dashes that do not line up
// with the notes under them.
//
// Asked of each kind, because they are different widths and the error is
// half the width: a staccato dot is 0.34 wide and a tenuto 1.35, so the dot
// looks nearly right while the dash plainly is not.
func TestArticulationsAreCentredOnTheirNotes(t *testing.T) {
	for _, a := range []struct {
		what model.Articulation
		name string
	}{
		{model.Staccato, "articStaccatoAbove"},
		{model.Tenuto, "articTenutoAbove"},
		{model.Accent, "articAccentAbove"},
		{model.Marcato, "articMarcatoAbove"},
		{model.Staccatissimo, "articStaccatissimoAbove"},
	} {
		t.Run(a.name, func(t *testing.T) {
			// Both ways round. A high note's stem points down and its mark
			// goes above; a low note's points up and its mark goes below,
			// and the two take different turnings through the code. Whole
			// notes would have dodged the question altogether, having no
			// stems at all.
			for _, c := range []struct {
				where  string
				octave int
				suffix string
			}{
				{"above a stem-down note", 5, "Above"},
				{"below a stem-up note", 4, "Below"},
			} {
				s := model.NewScore()
				p := s.Parts[0]
				m := model.NewMeasure()
				n := &model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: c.octave},
					Dur:   model.Duration{Num: 1, Den: 4},
				}
				n.AddArticulation(a.what)
				m.Voices = []*model.Voice{{Elements: []model.Element{n}}}
				p.Measures = []*model.Measure{m}

				lay := Engrave(s, DefaultOptions())

				centre := func(name string) (float64, bool) {
					for _, g := range lay.Glyphs {
						if g.Name != name {
							continue
						}
						b, ok := smufl.BBox(name)
						if !ok {
							t.Fatalf("no box for %s", name)
						}
						return g.X + (b.Left+b.Right)/2, true
					}
					return 0, false
				}

				head, ok := centre("noteheadBlack")
				if !ok {
					t.Fatalf("%s: no notehead drawn", c.where)
				}
				want := strings.TrimSuffix(a.name, "Above") + c.suffix
				mark, ok := centre(want)
				if !ok {
					t.Fatalf("%s: no %s drawn", c.where, want)
				}
				if off := mark - head; math.Abs(off) > 0.001 {
					t.Errorf("%s: the %s is %+.3f staff spaces from the middle "+
						"of its note", c.where, want, off)
				}
			}
		})
	}
}

// And so do the bow strokes, which go above the staff whichever way the
// stems point and so take a path of their own through the code.
func TestBowStrokesAreCentredOnTheirNotes(t *testing.T) {
	s := model.NewScore()
	p := s.Parts[0]
	m := model.NewMeasure()
	n := &model.Note{
		Pitch: model.Pitch{Step: model.StepB, Octave: 4},
		Dur:   model.Duration{Num: 1, Den: 1},
	}
	n.AddArticulation(model.DownBow)
	m.Voices = []*model.Voice{{Elements: []model.Element{n}}}
	p.Measures = []*model.Measure{m}

	lay := Engrave(s, DefaultOptions())

	var head, mark float64
	var gotHead, gotMark bool
	for _, g := range lay.Glyphs {
		b, ok := smufl.BBox(g.Name)
		if !ok {
			continue
		}
		switch g.Name {
		case "noteheadWhole":
			head, gotHead = g.X+(b.Left+b.Right)/2, true
		case "stringsDownBow":
			mark, gotMark = g.X+(b.Left+b.Right)/2, true
		}
	}
	if !gotHead || !gotMark {
		t.Fatalf("notehead drawn: %v, down bow drawn: %v", gotHead, gotMark)
	}
	if off := mark - head; math.Abs(off) > 0.001 {
		t.Errorf("the down bow is %+.3f staff spaces from the middle of its note", off)
	}
}

// centred puts the ink in the middle, which is not the same as putting the
// advance width in the middle for a glyph whose ink sits off to one side.
func TestCentredUsesTheInkAndNotTheAdvance(t *testing.T) {
	const name = "articTenutoAbove"
	b, ok := smufl.BBox(name)
	if !ok {
		t.Fatalf("no box for %s", name)
	}
	origin, left, right := centred(name, 10)

	if mid := (left + right) / 2; math.Abs(mid-10) > 0.001 {
		t.Errorf("the ink runs %.3f to %.3f, centred on %.3f, want 10", left, right, mid)
	}
	if want := 10 - (b.Left+b.Right)/2; math.Abs(origin-want) > 0.001 {
		t.Errorf("origin %.3f, want %.3f", origin, want)
	}
	// An unknown glyph has no box, and the advance is then the best there
	// is rather than an error.
	if o, _, _ := centred("noSuchGlyphAnywhere", 10); o != 10-smufl.Advance("noSuchGlyphAnywhere")/2 {
		t.Error("an unmeasured glyph is not centred on its advance width")
	}
}
