package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// An opening repeat draws a thick line, a thin line and two dots. The bar it
// opens must start clear of all of it.
func TestRepeatBarlineLeavesRoom(t *testing.T) {
	for _, style := range []model.BarlineStyle{
		model.BarSingle, model.BarDouble, model.BarFinal,
		model.BarRepeatStart, model.BarRepeatEnd, model.BarRepeatBoth,
	} {
		s := model.NewScore()
		p := s.Parts[0]
		p.Measures = nil
		for i := 0; i < 3; i++ {
			m := &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4},
					Dur: model.Duration{Num: 1, Den: 1}},
			}}}}
			if i == 0 {
				m.RightBarline = style
			}
			p.Measures = append(p.Measures, m)
		}

		opt := DefaultOptions()
		opt.Mode = Strip
		opt.WriteAhead = false
		lay := Engrave(s, opt)

		// Where the barline's ink actually reaches.
		rightmost := 0.0
		for _, l := range lay.Lines {
			if l.X1 == l.X2 && l.Y2-l.Y1 > 3 && l.X1 > rightmost {
				rightmost = l.X1 + l.Thickness/2
			}
		}
		for _, g := range lay.Glyphs {
			if g.Name == "augmentationDot" {
				if end := g.X + smufl.Advance(g.Name); end > rightmost {
					rightmost = end
				}
			}
		}
		// Only the first barline matters here; take the first bar's extent.
		var firstBarEnd, secondNote float64
		for _, h := range lay.Hits {
			if h.Measure == 0 {
				firstBarEnd = h.XMax
			}
			if h.Measure == 1 && len(h.Elements) > 0 {
				secondNote = h.Elements[0].X
			}
		}
		// The ink of the barline that closes bar one.
		ink := 0.0
		for _, l := range lay.Lines {
			if l.X1 == l.X2 && l.Y2-l.Y1 > 3 && l.X1 >= firstBarEnd && l.X1 < secondNote {
				if end := l.X1 + l.Thickness/2; end > ink {
					ink = end
				}
			}
		}
		for _, g := range lay.Glyphs {
			if g.Name == "augmentationDot" && g.X >= firstBarEnd && g.X < secondNote+2 {
				if end := g.X + smufl.Advance(g.Name); end > ink {
					ink = end
				}
			}
		}
		gap := secondNote - ink
		t.Logf("%-12v barline ink ends %6.2f, next note at %6.2f (gap %5.2f)",
			style, ink, secondNote, gap)
		if gap < 0 {
			t.Errorf("%v: the barline overlaps the next bar's first note by %.2f spaces",
				style, -gap)
		}
	}
}

// A repeat is dots and a heavy line, and which side the dots fall on is the
// whole of what the sign says.
//
// TestRepeatBarlineLeavesRoom above asks whether room was left for the marks.
// Nothing asked whether anything was put in it, and for one style nothing
// was: repeat-both reserved its room in barlineRoom and then fell through to
// a plain thin line in the drawing. The Badinerie turns at bar 16, and the
// page drew one line there, like every other bar in the piece.
func TestEveryRepeatDrawsItsDots(t *testing.T) {
	for _, c := range []struct {
		style       model.BarlineStyle
		left, right int
	}{
		{model.BarSingle, 0, 0},
		{model.BarRepeatEnd, 2, 0},
		{model.BarRepeatStart, 0, 2},
		{model.BarRepeatBoth, 2, 2},
	} {
		s := model.NewScore()
		p := s.Parts[0]
		p.Measures = nil
		for i := 0; i < 3; i++ {
			m := &model.Measure{Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4},
					Dur: model.Duration{Num: 1, Den: 1}},
			}}}}
			if i == 0 {
				m.RightBarline = c.style
			}
			p.Measures = append(p.Measures, m)
		}

		opt := DefaultOptions()
		opt.Mode = Strip
		opt.WriteAhead = false
		lay := Engrave(s, opt)

		var dots []float64
		for _, g := range lay.Glyphs {
			if g.Name == "augmentationDot" {
				dots = append(dots, g.X)
			}
		}
		if c.left+c.right == 0 {
			if len(dots) != 0 {
				t.Errorf("%v: %d repeat dots on a barline that has none",
					c.style, len(dots))
			}
			continue
		}
		if len(dots) != c.left+c.right {
			t.Errorf("%v: %d repeat dots drawn, expected %d",
				c.style, len(dots), c.left+c.right)
			continue
		}

		// The heavy stroke the dots are placed around.
		heavy := 0.0
		found := false
		for _, l := range lay.Lines {
			if l.X1 != l.X2 || l.Y2-l.Y1 <= 3 {
				continue
			}
			if l.Thickness >= opt.Defaults.ThickBarlineThickness &&
				(!found || l.X1 < heavy) {
				heavy, found = l.X1, true
			}
		}
		if !found {
			t.Errorf("%v: dots but no heavy line for them to belong to", c.style)
			continue
		}

		before, after := 0, 0
		for _, x := range dots {
			if x < heavy {
				before++
			} else {
				after++
			}
		}
		if before != c.left || after != c.right {
			t.Errorf("%v: %d dots before the heavy line and %d after, "+
				"expected %d and %d", c.style, before, after, c.left, c.right)
		}
	}
}
