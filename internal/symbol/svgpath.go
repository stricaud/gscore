/*
 * svgpath.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package symbol

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Reading the outline out of an SVG path.
//
// A symbol drawn in a vector program arrives as path data: a string of
// letters and numbers in SVG's own shorthand, with ten commands, each in an
// absolute and a relative form, and three of them defined by what came
// before. Nothing downstream wants to know any of that.
//
// So it is read once, here, into straight lines and cubic curves — which
// are the only two things a page can draw. The SVG renderer writes them
// back out as path data and the PDF renderer writes them as its own
// operators, and neither has to parse anything or know that quadratics and
// arcs were ever involved.

// SegKind is what one piece of an outline is.
type SegKind int

const (
	// SegMove starts a new contour at P[0].
	SegMove SegKind = iota
	// SegLine is a straight piece to P[0].
	SegLine
	// SegCubic is a cubic Bézier with controls P[0] and P[1], ending at
	// P[2]. Everything curved becomes one of these.
	SegCubic
	// SegClose joins the contour back to where it started.
	SegClose
)

// Seg is one piece of an outline.
type Seg struct {
	Kind SegKind
	P    [3]Point
}

// Path is an outline: a list of pieces, in order.
type Path []Seg

// ParsePath reads SVG path data.
//
// Everything is kept in the units the file used; moving it onto a staff is
// a separate question and belongs to whoever knows how big the symbol is
// meant to be. See Path.Transform.
//
// An error names what it could not read rather than where in the string it
// gave up, because the person seeing it drew a picture and did not write
// the path by hand.
func ParsePath(d string) (Path, error) {
	p := &pathReader{s: d}
	var out Path
	// cur is where the pen is, start where the contour began, and prev the
	// control point the smooth commands reflect.
	var cur, start, prevC2, prevQ Point
	var lastCmd byte

	for {
		p.space()
		if p.done() {
			break
		}
		cmd, ok := p.command()
		if !ok {
			// A run of numbers with no letter repeats the last command,
			// which is how SVG writes a polyline.
			if lastCmd == 0 {
				return nil, fmt.Errorf("path begins with %q, not a command",
					p.peek())
			}
			cmd = lastCmd
			// M repeated means L, which is the one case where repeating
			// is not simply the same command again.
			if cmd == 'M' {
				cmd = 'L'
			} else if cmd == 'm' {
				cmd = 'l'
			}
		}
		// A path begins where the pen is put down. Starting anywhere
		// else is not a path the specification allows, and reading it as
		// though it began at the origin is a guess about somebody's
		// drawing rather than a reading of it.
		if lastCmd == 0 && upper(cmd) != 'M' {
			return nil, fmt.Errorf("a path begins with a moveto, not %q",
				string(cmd))
		}
		rel := cmd >= 'a' && cmd <= 'z'
		at := func(x, y float64) Point {
			if rel {
				return Point{X: cur.X + x, Y: cur.Y + y}
			}
			return Point{X: x, Y: y}
		}

		switch upper(cmd) {
		case 'M':
			x, y, err := p.two()
			if err != nil {
				return nil, err
			}
			cur = at(x, y)
			start = cur
			out = append(out, Seg{Kind: SegMove, P: [3]Point{cur}})
		case 'L':
			x, y, err := p.two()
			if err != nil {
				return nil, err
			}
			cur = at(x, y)
			out = append(out, Seg{Kind: SegLine, P: [3]Point{cur}})
		case 'H':
			x, err := p.one()
			if err != nil {
				return nil, err
			}
			if rel {
				cur.X += x
			} else {
				cur.X = x
			}
			out = append(out, Seg{Kind: SegLine, P: [3]Point{cur}})
		case 'V':
			y, err := p.one()
			if err != nil {
				return nil, err
			}
			if rel {
				cur.Y += y
			} else {
				cur.Y = y
			}
			out = append(out, Seg{Kind: SegLine, P: [3]Point{cur}})
		case 'C':
			n, err := p.n(6)
			if err != nil {
				return nil, err
			}
			c1, c2 := at(n[0], n[1]), at(n[2], n[3])
			cur = at(n[4], n[5])
			out = append(out, Seg{Kind: SegCubic, P: [3]Point{c1, c2, cur}})
			prevC2 = c2
		case 'S':
			n, err := p.n(4)
			if err != nil {
				return nil, err
			}
			c1 := cur
			if upper(lastCmd) == 'C' || upper(lastCmd) == 'S' {
				// Smooth: the first control is the last one reflected.
				c1 = Point{X: 2*cur.X - prevC2.X, Y: 2*cur.Y - prevC2.Y}
			}
			c2 := at(n[0], n[1])
			cur = at(n[2], n[3])
			out = append(out, Seg{Kind: SegCubic, P: [3]Point{c1, c2, cur}})
			prevC2 = c2
		case 'Q':
			n, err := p.n(4)
			if err != nil {
				return nil, err
			}
			q := at(n[0], n[1])
			end := at(n[2], n[3])
			out = append(out, quadratic(cur, q, end))
			cur, prevQ = end, q
		case 'T':
			x, y, err := p.two()
			if err != nil {
				return nil, err
			}
			q := cur
			if upper(lastCmd) == 'Q' || upper(lastCmd) == 'T' {
				q = Point{X: 2*cur.X - prevQ.X, Y: 2*cur.Y - prevQ.Y}
			}
			end := at(x, y)
			out = append(out, quadratic(cur, q, end))
			cur, prevQ = end, q
		case 'A':
			n, err := p.n(7)
			if err != nil {
				return nil, err
			}
			end := at(n[5], n[6])
			out = append(out, arc(cur, end, n[0], n[1], n[2], n[3] != 0, n[4] != 0)...)
			cur = end
		case 'Z':
			out = append(out, Seg{Kind: SegClose})
			cur = start
		default:
			return nil, fmt.Errorf("path command %q is not one SVG has", string(cmd))
		}
		lastCmd = cmd
	}
	return out, nil
}

// quadratic is the cubic that draws the same curve.
//
// Every quadratic is a cubic — its controls are two thirds of the way from
// each end towards the single control — so nothing downstream needs to know
// quadratics exist.
func quadratic(from, c, to Point) Seg {
	return Seg{Kind: SegCubic, P: [3]Point{
		{X: from.X + 2.0/3.0*(c.X-from.X), Y: from.Y + 2.0/3.0*(c.Y-from.Y)},
		{X: to.X + 2.0/3.0*(c.X-to.X), Y: to.Y + 2.0/3.0*(c.Y-to.Y)},
		to,
	}}
}

// arc is the cubics that draw an elliptical arc.
//
// SVG's arc is given as an end point and the ellipse to get there on, which
// is convenient to write and impossible to draw directly: no page format
// has an arc operator. The standard conversion recovers the centre and then
// cuts the sweep into pieces of no more than a quarter turn, each of which
// a cubic fits to within a thousandth of the radius.
//
// Degenerate cases — no distance to travel, or an axis of zero — are a
// straight line, which is what the specification says they are.
func arc(from, to Point, rx, ry, rotation float64, large, sweep bool) []Seg {
	if from == to {
		return nil
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return []Seg{{Kind: SegLine, P: [3]Point{to}}}
	}
	phi := rotation * math.Pi / 180
	cosP, sinP := math.Cos(phi), math.Sin(phi)

	// Into the ellipse's own frame, where it is a unit circle.
	dx2, dy2 := (from.X-to.X)/2, (from.Y-to.Y)/2
	x1 := cosP*dx2 + sinP*dy2
	y1 := -sinP*dx2 + cosP*dy2

	// An ellipse too small to reach is grown until it just does, which is
	// what the specification asks for rather than refusing the arc.
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}

	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	if den == 0 {
		return []Seg{{Kind: SegLine, P: [3]Point{to}}}
	}
	co := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		co = -co
	}
	cx1 := co * rx * y1 / ry
	cy1 := -co * ry * x1 / rx

	cx := cosP*cx1 - sinP*cy1 + (from.X+to.X)/2
	cy := sinP*cx1 + cosP*cy1 + (from.Y+to.Y)/2

	angle := func(ux, uy, vx, vy float64) float64 {
		dot := ux*vx + uy*vy
		len := math.Hypot(ux, uy) * math.Hypot(vx, vy)
		if len == 0 {
			return 0
		}
		a := math.Acos(math.Max(-1, math.Min(1, dot/len)))
		if ux*vy-uy*vx < 0 {
			return -a
		}
		return a
	}
	theta := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	delta := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}

	// A quarter turn at a time: a cubic fits that closely and does not
	// fit much more.
	pieces := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	if pieces == 0 {
		return nil
	}
	step := delta / float64(pieces)
	// The control-point distance that makes a cubic match a circular arc
	// of this angle.
	k := 4.0 / 3.0 * math.Tan(step/4)

	on := func(t float64) (Point, Point) {
		cosT, sinT := math.Cos(t), math.Sin(t)
		px := cx + rx*cosP*cosT - ry*sinP*sinT
		py := cy + rx*sinP*cosT + ry*cosP*sinT
		// The derivative, for the control points.
		dx := -rx*cosP*sinT - ry*sinP*cosT
		dy := -rx*sinP*sinT + ry*cosP*cosT
		return Point{X: px, Y: py}, Point{X: dx, Y: dy}
	}

	var out []Seg
	t := theta
	for i := 0; i < pieces; i++ {
		p0, d0 := on(t)
		p1, d1 := on(t + step)
		out = append(out, Seg{Kind: SegCubic, P: [3]Point{
			{X: p0.X + k*d0.X, Y: p0.Y + k*d0.Y},
			{X: p1.X - k*d1.X, Y: p1.Y - k*d1.Y},
			p1,
		}})
		t += step
	}
	return out
}

// Bounds is the box the outline fits in.
//
// Measured on the points, control points included. That is a little larger
// than the ink for a curve that bends away from its controls, and larger is
// the safe way to be wrong: a symbol with room to spare around it is tidy,
// and one measured too small is clipped.
func (p Path) Bounds() (minX, minY, maxX, maxY float64) {
	first := true
	for _, s := range p {
		n := 0
		switch s.Kind {
		case SegMove, SegLine:
			n = 1
		case SegCubic:
			n = 3
		}
		for i := 0; i < n; i++ {
			pt := s.P[i]
			if first {
				minX, minY, maxX, maxY = pt.X, pt.Y, pt.X, pt.Y
				first = false
				continue
			}
			minX, minY = math.Min(minX, pt.X), math.Min(minY, pt.Y)
			maxX, maxY = math.Max(maxX, pt.X), math.Max(maxY, pt.Y)
		}
	}
	return
}

// Transform scales and shifts every point: x' = x*sx + dx.
//
// Which is how a drawing in a file's own units becomes a symbol measured in
// staff spaces.
func (p Path) Transform(sx, sy, dx, dy float64) Path {
	out := make(Path, len(p))
	for i, s := range p {
		out[i] = Seg{Kind: s.Kind}
		for j := range s.P {
			out[i].P[j] = Point{X: s.P[j].X*sx + dx, Y: s.P[j].Y*sy + dy}
		}
	}
	return out
}

// String writes the outline back as SVG path data, for a renderer whose
// page is itself SVG.
func (p Path) String() string {
	var b strings.Builder
	num := func(v float64) string {
		return strconv.FormatFloat(v, 'f', 3, 64)
	}
	for _, s := range p {
		switch s.Kind {
		case SegMove:
			fmt.Fprintf(&b, "M%s %s", num(s.P[0].X), num(s.P[0].Y))
		case SegLine:
			fmt.Fprintf(&b, "L%s %s", num(s.P[0].X), num(s.P[0].Y))
		case SegCubic:
			fmt.Fprintf(&b, "C%s %s %s %s %s %s",
				num(s.P[0].X), num(s.P[0].Y),
				num(s.P[1].X), num(s.P[1].Y),
				num(s.P[2].X), num(s.P[2].Y))
		case SegClose:
			b.WriteString("Z")
		}
	}
	return b.String()
}

/*
 * Reading the numbers.
 */

type pathReader struct {
	s string
	i int
}

func (p *pathReader) done() bool { return p.i >= len(p.s) }

func (p *pathReader) peek() string {
	if p.done() {
		return ""
	}
	return string(p.s[p.i])
}

// space steps over anything between numbers: blanks, and the commas SVG
// allows instead of them.
func (p *pathReader) space() {
	for !p.done() {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r', ',':
			p.i++
		default:
			return
		}
	}
}

func (p *pathReader) command() (byte, bool) {
	if p.done() {
		return 0, false
	}
	c := p.s[p.i]
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		p.i++
		return c, true
	}
	return 0, false
}

// one reads a number. SVG allows a sign to act as a separator, so "1-2" is
// two numbers and not a subtraction.
func (p *pathReader) one() (float64, error) {
	p.space()
	start := p.i
	if !p.done() && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}
	seenDot, seenE := false, false
	for !p.done() {
		c := p.s[p.i]
		switch {
		case c >= '0' && c <= '9':
			p.i++
		case c == '.' && !seenDot && !seenE:
			seenDot = true
			p.i++
		case (c == 'e' || c == 'E') && !seenE && p.i > start:
			seenE = true
			p.i++
			if !p.done() && (p.s[p.i] == '+' || p.s[p.i] == '-') {
				p.i++
			}
		default:
			goto done
		}
	}
done:
	if p.i == start {
		return 0, fmt.Errorf("expected a number in the path, found %q", p.peek())
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", p.s[start:p.i])
	}
	return v, nil
}

func (p *pathReader) two() (float64, float64, error) {
	x, err := p.one()
	if err != nil {
		return 0, 0, err
	}
	y, err := p.one()
	return x, y, err
}

func (p *pathReader) n(count int) ([]float64, error) {
	out := make([]float64, count)
	for i := range out {
		v, err := p.one()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}
