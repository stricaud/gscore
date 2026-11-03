/*
 * svgpath_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading the outline out of an SVG path.
 *
 * Ten commands, each in an absolute and a relative form, three of them
 * defined by what came before, and numbers that run together without
 * separators. All of it becomes lines and cubics, which is the only thing
 * a page can draw.
 */

package symbol

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.001 }

func pointsNear(a, b Point) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func parse(t *testing.T, d string) Path {
	t.Helper()
	p, err := ParsePath(d)
	if err != nil {
		t.Fatalf("%q: %v", d, err)
	}
	return p
}

func TestAStraightOutlineIsRead(t *testing.T) {
	p := parse(t, "M0 0 L10 0 L10 10 Z")
	if len(p) != 4 {
		t.Fatalf("%d pieces", len(p))
	}
	if p[0].Kind != SegMove || p[1].Kind != SegLine || p[3].Kind != SegClose {
		t.Errorf("the pieces are %v", p)
	}
	if !pointsNear(p[2].P[0], Point{X: 10, Y: 10}) {
		t.Errorf("the corner is %v", p[2].P[0])
	}
}

// Lower case is relative, and the pen carries on from where it is.
func TestRelativeCommandsFollowThePen(t *testing.T) {
	p := parse(t, "M10 10 l5 0 l0 5")
	if !pointsNear(p[1].P[0], Point{X: 15, Y: 10}) {
		t.Errorf("after l5 0 the pen is at %v", p[1].P[0])
	}
	if !pointsNear(p[2].P[0], Point{X: 15, Y: 15}) {
		t.Errorf("after l0 5 the pen is at %v", p[2].P[0])
	}
}

func TestHorizontalAndVerticalKeepTheOtherAxis(t *testing.T) {
	p := parse(t, "M3 4 H10 V20 h-5 v-5")
	want := []Point{{X: 3, Y: 4}, {X: 10, Y: 4}, {X: 10, Y: 20}, {X: 5, Y: 20}, {X: 5, Y: 15}}
	for i, w := range want {
		if !pointsNear(p[i].P[0], w) {
			t.Errorf("piece %d is at %v, want %v", i, p[i].P[0], w)
		}
	}
}

// Numbers with no letter in front repeat the last command, which is how a
// polyline is written — and after an M they are L, not another M.
func TestRepeatedNumbersRepeatTheCommand(t *testing.T) {
	p := parse(t, "M0 0 1 1 2 2")
	if len(p) != 3 {
		t.Fatalf("%d pieces", len(p))
	}
	if p[1].Kind != SegLine || p[2].Kind != SegLine {
		t.Errorf("a repeated M should be an L: %v", p)
	}
}

// A sign acts as a separator, so "1-2" is two numbers.
func TestASignSeparatesNumbers(t *testing.T) {
	p := parse(t, "M1-2L3-4")
	if !pointsNear(p[0].P[0], Point{X: 1, Y: -2}) {
		t.Errorf("the first point is %v", p[0].P[0])
	}
	if !pointsNear(p[1].P[0], Point{X: 3, Y: -4}) {
		t.Errorf("the second point is %v", p[1].P[0])
	}
}

func TestExponentsAreRead(t *testing.T) {
	p := parse(t, "M1e2 2.5e-1")
	if !pointsNear(p[0].P[0], Point{X: 100, Y: 0.25}) {
		t.Errorf("read %v", p[0].P[0])
	}
}

/*
 * Curves.
 */

// Every quadratic is a cubic, so nothing downstream has to know they exist.
func TestAQuadraticBecomesACubic(t *testing.T) {
	p := parse(t, "M0 0 Q6 0 6 6")
	if len(p) != 2 || p[1].Kind != SegCubic {
		t.Fatalf("the pieces are %v", p)
	}
	// Two thirds of the way from each end towards the single control.
	if !pointsNear(p[1].P[0], Point{X: 4, Y: 0}) {
		t.Errorf("the first control is %v, want {4 0}", p[1].P[0])
	}
	if !pointsNear(p[1].P[1], Point{X: 6, Y: 2}) {
		t.Errorf("the second control is %v, want {6 2}", p[1].P[1])
	}
}

// S reflects the control of the curve before it.
func TestASmoothCurveReflectsTheOneBefore(t *testing.T) {
	p := parse(t, "M0 0 C1 1 2 2 3 3 S5 5 6 6")
	if len(p) != 3 {
		t.Fatalf("%d pieces", len(p))
	}
	// The last control was {2 2} and the pen is at {3 3}, so the
	// reflection is {4 4}.
	if !pointsNear(p[2].P[0], Point{X: 4, Y: 4}) {
		t.Errorf("the reflected control is %v, want {4 4}", p[2].P[0])
	}
}

// And an S that does not follow a curve starts from the pen itself.
func TestASmoothCurveAfterALineStartsWhereItIs(t *testing.T) {
	p := parse(t, "M0 0 L2 0 S4 2 6 0")
	if !pointsNear(p[2].P[0], Point{X: 2, Y: 0}) {
		t.Errorf("the first control is %v, want the pen at {2 0}", p[2].P[0])
	}
}

/*
 * Arcs, which no page format can draw.
 */

func TestAnArcBecomesCurves(t *testing.T) {
	p := parse(t, "M0 0 A5 5 0 0 1 10 0")
	if len(p) < 2 {
		t.Fatalf("the arc gave %d pieces", len(p))
	}
	for _, s := range p[1:] {
		if s.Kind != SegCubic {
			t.Errorf("an arc gave a %v", s.Kind)
		}
	}
	// It ends where it was told to, whatever it did in between.
	last := p[len(p)-1]
	if !pointsNear(last.P[2], Point{X: 10, Y: 0}) {
		t.Errorf("the arc ends at %v, want {10 0}", last.P[2])
	}
}

// A half circle has to bulge: the curve is nowhere near the straight line
// between its ends.
func TestAnArcIsNotAStraightLine(t *testing.T) {
	p := parse(t, "M0 0 A5 5 0 0 1 10 0")
	_, minY, _, maxY := p.Bounds()
	if maxY-minY < 2 {
		t.Errorf("the arc is %.2f tall; it should bulge", maxY-minY)
	}
}

// The degenerate ones the specification calls straight lines.
func TestADegenerateArcIsALine(t *testing.T) {
	p := parse(t, "M0 0 A0 5 0 0 1 10 0")
	if len(p) != 2 || p[1].Kind != SegLine {
		t.Errorf("an arc with no radius gave %v", p)
	}
	// And one that goes nowhere is nothing at all.
	p = parse(t, "M3 3 A5 5 0 0 1 3 3")
	if len(p) != 1 {
		t.Errorf("an arc to where it started gave %v", p)
	}
}

/*
 * What it will not read.
 */

func TestNonsenseIsRefusedWithAReason(t *testing.T) {
	for _, d := range []string{"L1 1", "M0 0 X5 5", "M0", "M0 0 C1 2 3"} {
		if _, err := ParsePath(d); err == nil {
			t.Errorf("%q was read without complaint", d)
		}
	}
}

func TestAnEmptyPathIsNothingRatherThanAnError(t *testing.T) {
	p, err := ParsePath("   ")
	if err != nil || len(p) != 0 {
		t.Errorf("an empty path gave %v, %v", p, err)
	}
}

/*
 * Measuring and moving.
 */

func TestTheBoxFitsTheOutline(t *testing.T) {
	p := parse(t, "M0 0 L10 0 L10 4 Z")
	x0, y0, x1, y1 := p.Bounds()
	if !near(x0, 0) || !near(y0, 0) || !near(x1, 10) || !near(y1, 4) {
		t.Errorf("the box is %.1f,%.1f..%.1f,%.1f", x0, y0, x1, y1)
	}
}

func TestTransformMovesEveryPoint(t *testing.T) {
	p := parse(t, "M0 0 C1 1 2 2 3 3").Transform(2, 3, 10, 20)
	if !pointsNear(p[0].P[0], Point{X: 10, Y: 20}) {
		t.Errorf("the start moved to %v", p[0].P[0])
	}
	if !pointsNear(p[1].P[2], Point{X: 16, Y: 29}) {
		t.Errorf("the end moved to %v", p[1].P[2])
	}
}

// Written back out, it reads the same way again: the renderer whose page
// is SVG gets path data and nothing has been lost in between.
func TestAnOutlineSurvivesBeingWrittenOut(t *testing.T) {
	want := parse(t, "M0 0 L10 0 Q15 0 15 5 Z")
	again := parse(t, want.String())
	if len(again) != len(want) {
		t.Fatalf("%d pieces became %d", len(want), len(again))
	}
	for i := range want {
		if want[i].Kind != again[i].Kind {
			t.Errorf("piece %d changed kind", i)
		}
		for j := range want[i].P {
			if !pointsNear(want[i].P[j], again[i].P[j]) {
				t.Errorf("piece %d point %d moved from %v to %v",
					i, j, want[i].P[j], again[i].P[j])
			}
		}
	}
}
