/*
 * bracket_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The bracket that gathers a family of staves.
 *
 * It was three straight lines: a vertical and a diagonal serif at each end.
 * A serif of even width cannot come to a point and the corner where it met
 * the vertical showed as a notch, so it read as an arrow rather than as a
 * bracket. The ends are now the font's own bracketTop and bracketBottom,
 * which are drawn shapes with a crest and a point.
 *
 * And a bracket is not only for a pianist's two staves. It covers whatever
 * run of staves it is given — the strings of an orchestra, a rhythm
 * section — which is most of what it is for.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// bracketScore is staves staves, with a group over first..last.
func bracketScore(staves, first, last int, symbol model.GroupSymbol) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for i := 0; i < staves; i++ {
		s.Parts = append(s.Parts, &model.Part{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
				}}},
				RightBarline: model.BarSingle,
			}},
		})
	}
	s.Groups = []model.StaffGroup{{First: first, Last: last, Symbol: symbol}}
	s.Normalise()
	return s
}

func glyphsNamed(lay *Layout, name string) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if g.Name == name {
			out = append(out, g)
		}
	}
	return out
}

func TestABracketIsCappedWithTheFontsOwnEnds(t *testing.T) {
	lay := Engrave(bracketScore(3, 0, 2, model.GroupBracket), DefaultOptions())

	top := glyphsNamed(lay, "bracketTop")
	bottom := glyphsNamed(lay, "bracketBottom")
	if len(top) != 1 || len(bottom) != 1 {
		t.Fatalf("drew %d tops and %d bottoms, want one of each", len(top), len(bottom))
	}
	// The top cap is above the bottom one, and they share the vertical
	// they cap: a claw off to one side would be a claw on nothing.
	if top[0].Y >= bottom[0].Y {
		t.Errorf("the top cap is at y=%.2f, below the bottom one at %.2f",
			top[0].Y, bottom[0].Y)
	}
	if top[0].X != bottom[0].X {
		t.Errorf("the caps are at x=%.2f and x=%.2f; they cap the same line",
			top[0].X, bottom[0].X)
	}
}

// The vertical reaches both caps, so there is no gap between the line and
// the shape on the end of it.
func TestTheBracketsVerticalReachesItsEnds(t *testing.T) {
	lay := Engrave(bracketScore(3, 0, 2, model.GroupBracket), DefaultOptions())
	top := glyphsNamed(lay, "bracketTop")[0]
	bottom := glyphsNamed(lay, "bracketBottom")[0]

	found := false
	for _, l := range lay.Lines {
		if l.X1 != l.X2 || l.Thickness < bracketThickness {
			continue
		}
		if l.Y1 <= top.Y+0.01 && l.Y2 >= bottom.Y-0.01 {
			found = true
		}
	}
	if !found {
		t.Errorf("no vertical runs from %.2f to %.2f", top.Y, bottom.Y)
	}
}

// A brace is not a bracket and must not have grown claws.
func TestABraceHasNoBracketEnds(t *testing.T) {
	lay := Engrave(bracketScore(2, 0, 1, model.GroupBrace), DefaultOptions())
	if n := len(glyphsNamed(lay, "bracketTop")); n != 0 {
		t.Errorf("a brace drew %d bracket ends", n)
	}
}

// Whatever run of staves it is given, which is the point: the strings of an
// orchestra are bracketed together and they are not staves one and two.
func TestABracketCoversWhateverStavesItIsGiven(t *testing.T) {
	for _, tc := range []struct{ first, last int }{
		{0, 1}, {1, 3}, {2, 5}, {0, 5},
	} {
		s := bracketScore(6, tc.first, tc.last, model.GroupBracket)
		lay := Engrave(s, DefaultOptions())
		top := glyphsNamed(lay, "bracketTop")
		bottom := glyphsNamed(lay, "bracketBottom")
		if len(top) != 1 || len(bottom) != 1 {
			t.Errorf("staves %d..%d: %d tops, %d bottoms",
				tc.first, tc.last, len(top), len(bottom))
			continue
		}
		// It gets taller as it covers more staves, and that is the whole
		// check that it is following the group rather than guessing.
		if got := bottom[0].Y - top[0].Y; got <= 0 {
			t.Errorf("staves %d..%d: the bracket is %.2f tall",
				tc.first, tc.last, got)
		}
	}
}

func TestABracketOverMoreStavesIsTaller(t *testing.T) {
	height := func(first, last int) float64 {
		lay := Engrave(bracketScore(6, first, last, model.GroupBracket), DefaultOptions())
		return glyphsNamed(lay, "bracketBottom")[0].Y - glyphsNamed(lay, "bracketTop")[0].Y
	}
	two, four := height(0, 1), height(0, 3)
	if four <= two {
		t.Errorf("four staves bracket %.2f tall, two staves %.2f", four, two)
	}
}

// Two groups on one score, which is every orchestral page ever printed: a
// brace on the piano and a bracket on the strings.
func TestAScoreCanCarryABraceAndABracketAtOnce(t *testing.T) {
	s := bracketScore(5, 0, 1, model.GroupBrace)
	s.Groups = append(s.Groups, model.StaffGroup{
		First: 2, Last: 4, Symbol: model.GroupBracket,
	})
	lay := Engrave(s, DefaultOptions())

	if n := len(glyphsNamed(lay, "bracketTop")); n != 1 {
		t.Errorf("drew %d bracket tops, want the one over the strings", n)
	}
	// And the brace is still there, as the font's own glyph.
	if _, ok := glyphNamed(lay, "brace"); !ok {
		t.Error("the brace was not drawn")
	}
}

/*
 * A brace inside a bracket.
 *
 * A bracket round a family that has a piano in it is two symbols on one
 * system. Drawn in the same column they sit on top of each other, and the
 * brace comes off worst: its arms swell out to the left, so it crosses the
 * bracket rather than standing inside it.
 */

// nested is a flute and a piano, bracketed together, which is the score
// that showed this.
func nested() *model.Score {
	s := bracketScore(3, 0, 2, model.GroupBracket)
	s.Parts[2].Clef = model.Bass
	s.Groups = append(s.Groups, model.StaffGroup{
		First: 1, Last: 2, Symbol: model.GroupBrace, Barlines: true,
	})
	return s
}

// braceLeft is the span the brace occupies across the page.
//
// The glyph is set from its left edge and is as wide as scaling it to its
// own height makes it, which is what braceWidth says.
func braceLeft(lay *Layout) (lo, hi float64, ok bool) {
	for _, g := range lay.Glyphs {
		if g.Name != "brace" {
			continue
		}
		// Size is the em the glyph is set at, four spaces to the em.
		h := g.Size * braceNaturalH / 4
		return g.X, g.X + braceWidth(h), true
	}
	return 0, 0, false
}

// bracketAt is the x of the bracket's heavy vertical.
func bracketAt(lay *Layout) (float64, bool) {
	for _, l := range lay.Lines {
		if l.X1 == l.X2 && l.Y2-l.Y1 > 4 && l.Thickness >= bracketThickness {
			return l.X1, true
		}
	}
	return 0, false
}

func TestABraceInsideABracketStandsInsideIt(t *testing.T) {
	lay := Engrave(nested(), DefaultOptions())

	bx, ok := bracketAt(lay)
	if !ok {
		t.Fatal("no bracket was drawn")
	}
	lo, _, ok := braceLeft(lay)
	if !ok {
		t.Fatal("no brace was drawn")
	}
	// The brace begins to the right of the bracket, which is what
	// "inside" means on a page: the outermost symbol is furthest from
	// the music.
	if lo <= bx+bracketThickness/2 {
		t.Errorf("the brace starts at %.2f and the bracket is at %.2f — "+
			"the brace is drawn across it", lo, bx)
	}
}

// And neither of them is drawn over the clef.
func TestTheSymbolsClearTheClefs(t *testing.T) {
	lay := Engrave(nested(), DefaultOptions())

	_, braceRight, ok := braceLeft(lay)
	if !ok {
		t.Fatal("no brace was drawn")
	}
	clef := 1e9
	for _, g := range lay.Glyphs {
		// has is a prefix check, and the clefs are gClef and fClef.
		if (has(g.Name, "gClef") || has(g.Name, "fClef")) && g.X < clef {
			clef = g.X
		}
	}
	if clef == 1e9 {
		t.Fatal("no clef was drawn")
	}
	if braceRight >= clef {
		t.Errorf("the brace reaches %.2f and the first clef is at %.2f",
			braceRight, clef)
	}
}

// Nesting makes room by moving the music right, not by moving the
// outermost symbol off the edge of the page: the left margin is where the
// page begins and the bracket stands on it either way.
func TestNestingMakesRoomByMovingTheMusic(t *testing.T) {
	staffStart := func(lay *Layout) float64 {
		out := 1e9
		for _, l := range lay.Lines {
			if l.Y1 == l.Y2 && l.X2-l.X1 > 4 && l.X1 < out {
				out = l.X1
			}
		}
		return out
	}
	plain := staffStart(Engrave(bracketScore(3, 0, 2, model.GroupBracket), DefaultOptions()))
	with := staffStart(Engrave(nested(), DefaultOptions()))
	if with <= plain {
		t.Errorf("the staves start at %.2f nested and %.2f plain; "+
			"the brace has nowhere to go", with, plain)
	}

	// And the bracket itself has not moved: it is on the margin in both,
	// because room is made by moving the music and not by pushing the
	// outermost symbol off the edge of the page.
	a, _ := bracketAt(Engrave(bracketScore(3, 0, 2, model.GroupBracket), DefaultOptions()))
	b, _ := bracketAt(Engrave(nested(), DefaultOptions()))
	if math.Abs(a-b) > 0.001 {
		t.Errorf("the bracket moved from %.4f to %.4f", a, b)
	}
}

// Two brackets side by side are not nested and must not be stepped apart:
// that would say there is a structure on the page that is not there.
func TestSiblingBracketsShareAColumn(t *testing.T) {
	s := bracketScore(6, 0, 2, model.GroupBracket)
	s.Groups = append(s.Groups, model.StaffGroup{
		First: 3, Last: 5, Symbol: model.GroupBracket,
	})
	lay := Engrave(s, DefaultOptions())

	var xs []float64
	for _, l := range lay.Lines {
		if l.X1 == l.X2 && l.Y2-l.Y1 > 4 && l.Thickness >= bracketThickness {
			xs = append(xs, l.X1)
		}
	}
	if len(xs) != 2 {
		t.Fatalf("%d brackets were drawn", len(xs))
	}
	if xs[0] != xs[1] {
		t.Errorf("the two brackets are at %.2f and %.2f", xs[0], xs[1])
	}
}
