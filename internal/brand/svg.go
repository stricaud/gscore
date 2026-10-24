/*
 * svg.go
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
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Reading the mascot as shapes rather than as pixels.
//
// Gilbert is drawn once and printed at whatever size the page wants him. A
// bitmap is one size, and a logo stamped on a corner from a bitmap is either
// soft or enormous; the drawing is outlines, and outlines print at the
// resolution of whatever prints them.
//
// This reads the subset of SVG the drawing is written in and nothing else:
// groups, paths, circles and ellipses, with a flat colour and a line width.
// It is not an SVG renderer and should not grow into one. If the drawing
// ever needs something this does not have, it says so rather than leaving
// the shape out — a logo silently missing a sword is worse than one that
// refuses to print.

// RGB is a flat colour, each channel from zero to one.
type RGB struct{ R, G, B float64 }

// Op is one piece of an outline: a move, a line, a cubic curve, or the close
// that joins the end back to the start.
type Op struct {
	Kind byte      // 'm', 'l', 'c' or 'z'
	P    []float64 // two numbers for a move or a line, six for a curve
}

// Shape is one outline and how it is painted.
type Shape struct {
	Ops         []Op
	Fill        *RGB
	Stroke      *RGB
	StrokeWidth float64
}

// Drawing is a picture in its own coordinates, which is what the viewBox
// says. Whoever draws it decides what those are worth.
type Drawing struct {
	Width, Height float64
	Shapes        []Shape
}

/*
 * The document
 */

type svgNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []svgNode  `xml:",any"`
}

// paint is the state a shape inherits from the groups it is inside.
type paint struct {
	fill, stroke *RGB
	width        float64
}

// ParseSVG reads a drawing from the subset of SVG described above.
func ParseSVG(data []byte) (*Drawing, error) {
	var root svgNode
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root.XMLName.Local != "svg" {
		return nil, fmt.Errorf("brand: the file is a <%s>, not an <svg>",
			root.XMLName.Local)
	}
	d := &Drawing{Width: 1, Height: 1}
	if box := attr(root.Attrs, "viewBox"); box != "" {
		n := numbers(box)
		if len(n) == 4 && n[2] > 0 && n[3] > 0 {
			d.Width, d.Height = n[2], n[3]
		}
	}
	// Black, opaque, hairline: what SVG paints with when nothing says
	// otherwise.
	start := paint{fill: &RGB{}, width: 1}
	if err := walk(d, root, start); err != nil {
		return nil, err
	}
	return d, nil
}

// walk draws a node and everything inside it.
func walk(d *Drawing, n svgNode, in paint) error {
	p := inherit(in, n.Attrs)

	switch n.XMLName.Local {
	case "svg", "g":
		for _, c := range n.Children {
			if err := walk(d, c, p); err != nil {
				return err
			}
		}
		return nil
	case "title", "desc", "metadata", "defs":
		return nil // not drawn
	case "path":
		ops, err := parsePath(attr(n.Attrs, "d"))
		if err != nil {
			return err
		}
		add(d, ops, p)
		return nil
	case "circle":
		cx, cy := number(attr(n.Attrs, "cx")), number(attr(n.Attrs, "cy"))
		r := number(attr(n.Attrs, "r"))
		add(d, oval(cx, cy, r, r), p)
		return nil
	case "ellipse":
		cx, cy := number(attr(n.Attrs, "cx")), number(attr(n.Attrs, "cy"))
		add(d, oval(cx, cy, number(attr(n.Attrs, "rx")),
			number(attr(n.Attrs, "ry"))), p)
		return nil
	}
	return fmt.Errorf("brand: <%s> is not one of the shapes this reads",
		n.XMLName.Local)
}

func add(d *Drawing, ops []Op, p paint) {
	if len(ops) == 0 || (p.fill == nil && p.stroke == nil) {
		return // nothing drawn, or drawn in nothing
	}
	d.Shapes = append(d.Shapes, Shape{
		Ops: ops, Fill: p.fill, Stroke: p.stroke, StrokeWidth: p.width,
	})
}

// inherit applies a node's own painting attributes over the ones it is
// inside. Anything not mentioned stays as the group left it, which is what
// makes one stroke colour on the outermost group serve the whole drawing.
func inherit(in paint, attrs []xml.Attr) paint {
	out := in
	for _, a := range attrs {
		switch a.Name.Local {
		case "fill":
			out.fill = colour(a.Value)
		case "stroke":
			out.stroke = colour(a.Value)
		case "stroke-width":
			out.width = number(a.Value)
		}
	}
	return out
}

// colour reads a flat colour, or nil for "none".
func colour(s string) *RGB {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" || s == "transparent" {
		return nil
	}
	if s == "black" {
		return &RGB{}
	}
	if s == "white" {
		return &RGB{1, 1, 1}
	}
	if !strings.HasPrefix(s, "#") {
		return &RGB{} // a name this does not know is drawn, in black
	}
	h := s[1:]
	if len(h) == 3 { // #abc is #aabbcc
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return &RGB{}
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return &RGB{}
	}
	return &RGB{
		R: float64(v>>16&0xFF) / 255,
		G: float64(v>>8&0xFF) / 255,
		B: float64(v&0xFF) / 255,
	}
}

func attr(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

/*
 * Outlines
 */

// kappa is the distance along the tangents that makes a cubic curve pass
// closest to a quarter circle: the number everything that draws circles in
// PostScript has used since 1988.
const kappa = 0.5522847498

// oval is a circle or an ellipse as four cubic curves.
func oval(cx, cy, rx, ry float64) []Op {
	if rx <= 0 || ry <= 0 {
		return nil
	}
	ox, oy := rx*kappa, ry*kappa
	return []Op{
		{Kind: 'm', P: []float64{cx + rx, cy}},
		{Kind: 'c', P: []float64{cx + rx, cy + oy, cx + ox, cy + ry, cx, cy + ry}},
		{Kind: 'c', P: []float64{cx - ox, cy + ry, cx - rx, cy + oy, cx - rx, cy}},
		{Kind: 'c', P: []float64{cx - rx, cy - oy, cx - ox, cy - ry, cx, cy - ry}},
		{Kind: 'c', P: []float64{cx + ox, cy - ry, cx + rx, cy - oy, cx + rx, cy}},
		{Kind: 'z'},
	}
}

// parsePath reads a d attribute.
//
// Absolute and relative forms of the commands the drawing uses: move, line,
// cubic, quadratic, arc and close. The horizontal and vertical shorthands
// come free with the line. A command this does not know is an error rather
// than a silently dropped piece of the picture.
func parsePath(d string) ([]Op, error) {
	toks := tokens(d)
	var ops []Op
	var cur, start [2]float64
	i := 0

	// take reads n numbers, failing if the path runs out.
	take := func(n int) ([]float64, error) {
		if i+n > len(toks.nums) {
			return nil, fmt.Errorf("brand: the path ends in the middle of a command")
		}
		out := toks.nums[i : i+n]
		i += n
		return out, nil
	}

	for _, c := range toks.cmds {
		rel := c.cmd >= 'a' && c.cmd <= 'z'
		at := func(v []float64, k int) (x, y float64) {
			x, y = v[k], v[k+1]
			if rel {
				x, y = cur[0]+x, cur[1]+y
			}
			return
		}
		// Each command may be followed by several sets of numbers, which is
		// an implicit repeat of it — a run of line segments after one L.
		for n := 0; n < c.runs; n++ {
			switch strings.ToUpper(string(c.cmd))[0] {
			case 'M':
				v, err := take(2)
				if err != nil {
					return nil, err
				}
				x, y := at(v, 0)
				cur, start = [2]float64{x, y}, [2]float64{x, y}
				ops = append(ops, Op{Kind: 'm', P: []float64{x, y}})
			case 'L':
				v, err := take(2)
				if err != nil {
					return nil, err
				}
				x, y := at(v, 0)
				cur = [2]float64{x, y}
				ops = append(ops, Op{Kind: 'l', P: []float64{x, y}})
			case 'H':
				v, err := take(1)
				if err != nil {
					return nil, err
				}
				x := v[0]
				if rel {
					x += cur[0]
				}
				cur[0] = x
				ops = append(ops, Op{Kind: 'l', P: []float64{x, cur[1]}})
			case 'V':
				v, err := take(1)
				if err != nil {
					return nil, err
				}
				y := v[0]
				if rel {
					y += cur[1]
				}
				cur[1] = y
				ops = append(ops, Op{Kind: 'l', P: []float64{cur[0], y}})
			case 'C':
				v, err := take(6)
				if err != nil {
					return nil, err
				}
				x1, y1 := at(v, 0)
				x2, y2 := at(v, 2)
				x, y := at(v, 4)
				cur = [2]float64{x, y}
				ops = append(ops, Op{Kind: 'c', P: []float64{x1, y1, x2, y2, x, y}})
			case 'Q':
				v, err := take(4)
				if err != nil {
					return nil, err
				}
				qx, qy := at(v, 0)
				x, y := at(v, 2)
				// A quadratic raised to a cubic: PDF knows only cubics, and
				// the two control points sit two thirds of the way along
				// each tangent.
				ops = append(ops, Op{Kind: 'c', P: []float64{
					cur[0] + 2.0/3.0*(qx-cur[0]), cur[1] + 2.0/3.0*(qy-cur[1]),
					x + 2.0/3.0*(qx-x), y + 2.0/3.0*(qy-y),
					x, y,
				}})
				cur = [2]float64{x, y}
			case 'A':
				v, err := take(7)
				if err != nil {
					return nil, err
				}
				x, y := v[5], v[6]
				if rel {
					x, y = cur[0]+x, cur[1]+y
				}
				ops = append(ops, arcOps(cur[0], cur[1], v[0], v[1], v[2],
					v[3] != 0, v[4] != 0, x, y)...)
				cur = [2]float64{x, y}
			case 'Z':
				ops = append(ops, Op{Kind: 'z'})
				cur = start
			default:
				return nil, fmt.Errorf("brand: %q is not a path command this reads",
					string(c.cmd))
			}
		}
	}
	return ops, nil
}

// command is one letter and how many sets of numbers followed it.
type command struct {
	cmd  byte
	runs int
}

type pathTokens struct {
	cmds []command
	nums []float64
}

// tokens splits a path into its commands and its numbers.
func tokens(d string) pathTokens {
	var out pathTokens
	n := []rune(d)
	num := strings.Builder{}
	flush := func() {
		if num.Len() == 0 {
			return
		}
		v, err := strconv.ParseFloat(num.String(), 64)
		if err == nil {
			out.nums = append(out.nums, v)
			if len(out.cmds) > 0 {
				out.cmds[len(out.cmds)-1].runs++
			}
		}
		num.Reset()
	}
	for i := 0; i < len(n); i++ {
		c := n[i]
		switch {
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
			flush()
			out.cmds = append(out.cmds, command{cmd: byte(c)})
		case c == ',' || c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		case c == '-' || c == '+':
			// A sign starts a new number unless it is an exponent's.
			if num.Len() > 0 {
				last := num.String()[num.Len()-1]
				if last != 'e' && last != 'E' {
					flush()
				}
			}
			num.WriteRune(c)
		default:
			num.WriteRune(c)
		}
	}
	flush()

	// Each command's run count is the number of numbers that followed it;
	// turn that into how many times the command repeats.
	per := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4,
		'Q': 4, 'T': 2, 'A': 7, 'Z': 0}
	for i := range out.cmds {
		k := out.cmds[i].cmd
		if k >= 'a' && k <= 'z' {
			k -= 32
		}
		want := per[k]
		if want == 0 {
			out.cmds[i].runs = 1 // Z takes none and happens once
			continue
		}
		if out.cmds[i].runs /= want; out.cmds[i].runs < 1 {
			out.cmds[i].runs = 1
		}
	}
	return out
}

// arcOps turns an SVG elliptical arc into cubic curves.
//
// The arc is given by where it ends rather than by where its centre is, so
// the centre has to be found first — this is the conversion set out in the
// SVG specification's implementation notes, and then the sweep is cut into
// pieces of no more than a quarter turn, each of which a cubic can follow
// closely enough that no eye will tell.
func arcOps(x1, y1, rx, ry, rotation float64, large, sweep bool, x2, y2 float64) []Op {
	if rx == 0 || ry == 0 {
		return []Op{{Kind: 'l', P: []float64{x2, y2}}}
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	phi := rotation * math.Pi / 180
	cosp, sinp := math.Cos(phi), math.Sin(phi)

	// Into the ellipse's own frame, with the chord centred.
	dx2, dy2 := (x1-x2)/2, (y1-y2)/2
	x1p := cosp*dx2 + sinp*dy2
	y1p := -sinp*dx2 + cosp*dy2

	// Radii too small to reach are grown until they just do.
	if l := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}

	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		return []Op{{Kind: 'l', P: []float64{x2, y2}}}
	}
	co := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		co = -co
	}
	cxp := co * rx * y1p / ry
	cyp := -co * ry * x1p / rx
	cx := cosp*cxp - sinp*cyp + (x1+x2)/2
	cy := sinp*cxp + cosp*cyp + (y1+y2)/2

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
	theta := angle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	delta := angle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}

	// A quarter turn at a time, which is as far as a cubic follows a circle
	// without the error showing.
	pieces := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	if pieces < 1 {
		pieces = 1
	}
	step := delta / float64(pieces)
	k := 4.0 / 3.0 * math.Tan(step/4)

	at := func(t float64) (x, y, dx, dy float64) {
		cost, sint := math.Cos(t), math.Sin(t)
		x = cx + rx*cost*cosp - ry*sint*sinp
		y = cy + rx*cost*sinp + ry*sint*cosp
		dx = -rx*sint*cosp - ry*cost*sinp
		dy = -rx*sint*sinp + ry*cost*cosp
		return
	}

	var ops []Op
	t := theta
	for i := 0; i < pieces; i++ {
		x0, y0, dx0, dy0 := at(t)
		t += step
		x3, y3, dx3, dy3 := at(t)
		ops = append(ops, Op{Kind: 'c', P: []float64{
			x0 + k*dx0, y0 + k*dy0,
			x3 - k*dx3, y3 - k*dy3,
			x3, y3,
		}})
	}
	return ops
}

// numbers reads a list of them, for the viewBox.
func numbers(s string) []float64 {
	var out []float64
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n'
	}) {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func number(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
