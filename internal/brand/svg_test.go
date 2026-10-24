/*
 * svg_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package brand

import (
	"math"
	"testing"
)

// The mascot is read as outlines so that he can be printed at any size.
//
// This reads one drawing — his — and the point of the tests is that it
// reads that one correctly and says so when it cannot, rather than that it
// is a general SVG renderer. It is not one and should not become one.

func TestTheMascotReads(t *testing.T) {
	d, err := GilbertDrawing()
	if err != nil {
		t.Fatalf("the drawing did not read: %v", err)
	}
	if d.Width != 321 || d.Height != 317 {
		t.Errorf("the viewBox came out %gx%g, want 321x317", d.Width, d.Height)
	}
	// Every shape the file draws: eleven paths, twelve circles, two
	// ellipses. A shape quietly dropped is a logo missing a sword.
	if len(d.Shapes) != 25 {
		t.Errorf("%d shapes were read, want 25", len(d.Shapes))
	}

	// Nothing is drawn in nothing, and nothing is drawn outside the box.
	for i, s := range d.Shapes {
		if s.Fill == nil && s.Stroke == nil {
			t.Errorf("shape %d is painted in neither a fill nor a stroke", i)
		}
		if len(s.Ops) == 0 {
			t.Errorf("shape %d has no outline", i)
		}
		for _, op := range s.Ops {
			for k := 0; k+1 < len(op.P); k += 2 {
				x, y := op.P[k], op.P[k+1]
				// A margin, because a stroke's width reaches past the
				// outline and a curve's control points may sit outside it.
				if x < -40 || x > d.Width+40 || y < -40 || y > d.Height+40 {
					t.Errorf("shape %d has a point at %.1f,%.1f, well outside "+
						"the %gx%g box", i, x, y, d.Width, d.Height)
				}
			}
		}
	}
}

// A white shape with a black outline is two colours, and the outline colour
// comes from a group three levels up: the whole drawing is stroked in one
// ink, said once, at the top.
func TestPaintIsInheritedFromTheGroupsAround(t *testing.T) {
	d, err := ParseSVG([]byte(`<svg viewBox="0 0 10 10">
		<g stroke="#000000" stroke-width="2.5">
			<g fill="#ffffff">
				<circle cx="5" cy="5" r="4"/>
				<circle cx="5" cy="5" r="1" fill="#000000" stroke="none"/>
			</g>
		</g>
	</svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Shapes) != 2 {
		t.Fatalf("%d shapes", len(d.Shapes))
	}
	white, dot := d.Shapes[0], d.Shapes[1]
	if white.Fill == nil || white.Fill.R != 1 {
		t.Errorf("the disc was not filled white: %v", white.Fill)
	}
	if white.Stroke == nil || white.Stroke.R != 0 {
		t.Error("the disc did not inherit the outline from two groups up")
	}
	if white.StrokeWidth != 2.5 {
		t.Errorf("the width came out %g, want the group's 2.5", white.StrokeWidth)
	}
	// And a shape's own attributes beat what it inherits.
	if dot.Fill == nil || dot.Fill.R != 0 {
		t.Errorf("the dot's own fill was not used: %v", dot.Fill)
	}
	if dot.Stroke != nil {
		t.Error("the dot said stroke=none and was outlined anyway")
	}
}

// Arcs are the hair tufts, and they are how a bulge is written: an arc has
// to come out as curves that start and finish where it said.
func TestAnArcRunsFromWhereItStartsToWhereItEnds(t *testing.T) {
	// A half circle of radius 10, from the left of it to the right.
	d, err := ParseSVG([]byte(
		`<svg viewBox="0 0 40 40"><path fill="#000000" ` +
			`d="M 10 20 A 10 10 0 0 1 30 20 Z"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Shapes) != 1 {
		t.Fatalf("%d shapes", len(d.Shapes))
	}
	ops := d.Shapes[0].Ops
	if len(ops) < 3 {
		t.Fatalf("the arc became %d operations", len(ops))
	}
	last := ops[len(ops)-2] // the one before the close
	if last.Kind != 'c' {
		t.Fatalf("the arc ends in a %q, want a curve", string(last.Kind))
	}
	x, y := last.P[4], last.P[5]
	if math.Abs(x-30) > 0.01 || math.Abs(y-20) > 0.01 {
		t.Errorf("the arc finishes at %.2f,%.2f, want 30,20", x, y)
	}
	// And it goes over the top, which is what the sweep flag asked for.
	top := 20.0
	for _, op := range ops {
		for k := 1; k < len(op.P); k += 2 {
			if op.P[k] < top {
				top = op.P[k]
			}
		}
	}
	if top > 12 {
		t.Errorf("the arc only reached y=%.1f; a half circle of radius 10 "+
			"from 10,20 to 30,20 should reach about 10", top)
	}
}

// A quadratic — the sword blades are drawn with them — becomes the cubic
// that follows the same path, because that is the only curve PDF has.
func TestAQuadraticBecomesTheSameCurve(t *testing.T) {
	d, err := ParseSVG([]byte(
		`<svg viewBox="0 0 10 10"><path fill="#000000" d="M 0 0 Q 5 10 10 0"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	ops := d.Shapes[0].Ops
	if len(ops) != 2 || ops[1].Kind != 'c' {
		t.Fatalf("the quadratic came out as %v", ops)
	}
	// The halfway point of a quadratic from 0,0 through 5,10 to 10,0 is
	// 5,5; the cubic must pass through the same place.
	p := ops[1].P
	mid := func(a, b, c, dd float64) float64 {
		return 0.125*a + 0.375*b + 0.375*c + 0.125*dd
	}
	x := mid(0, p[0], p[2], p[4])
	y := mid(0, p[1], p[3], p[5])
	if math.Abs(x-5) > 0.001 || math.Abs(y-5) > 0.001 {
		t.Errorf("the curve passes through %.3f,%.3f, want 5,5", x, y)
	}
}

// Anything it cannot read, it says so about. A drawing with a shape silently
// missing is worse than one that refuses: the first is noticed by whoever
// opens the PDF, the second by whoever builds it.
func TestWhatItCannotReadIsAnError(t *testing.T) {
	for _, bad := range []string{
		`<svg viewBox="0 0 10 10"><rect x="1" y="1" width="2" height="2"/></svg>`,
		`<svg viewBox="0 0 10 10"><path d="M 0 0 X 3 3"/></svg>`,
		`<svg viewBox="0 0 10 10"><path d="M 0 0 C 1 1 2"/></svg>`,
	} {
		if _, err := ParseSVG([]byte(bad)); err == nil {
			t.Errorf("this was read without complaint: %s", bad)
		}
	}
}
