/*
 * svgfile.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package symbol

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Reading a symbol out of an SVG file.
//
// What is taken is the outlines and nothing else. An SVG from a drawing
// program carries layers, colours, strokes, gradients, text, clipping and a
// great deal of the program's own bookkeeping; a symbol on a stave is a
// black shape, and the shape is the part that means anything here.
//
// So: every <path>, and the rectangles, circles, ellipses, lines and
// polygons, each turned into the same lines and cubics. What is left out is
// left out quietly, because an SVG that says it is magenta is not an SVG
// that failed to load — it is a drawing whose colour this program has no
// use for.
//
// The size comes from the file and the scale from the caller. A drawing
// is in whatever units its author worked in, and how large it should be on
// a staff is not something the file can know.

// DefaultSVGHeight is how tall a symbol read from an SVG is made, in staff
// spaces, when nothing says otherwise.
//
// Two spaces: the height of a mark that sits over a note without reaching
// the staff above. It is a starting point and not a rule — the symbol can
// be scaled afterwards — but a default that needs changing every time is
// no default at all, and this is the size most marks are.
const DefaultSVGHeight = 2.0

// svgDoc is as much of an SVG as the outlines need.
type svgDoc struct {
	ViewBox string     `xml:"viewBox,attr"`
	Width   string     `xml:"width,attr"`
	Height  string     `xml:"height,attr"`
	Shapes  []svgShape `xml:",any"`
}

// svgShape is one element, whatever it turns out to be. Read with ",any"
// so that a group's children come through the same door as the top level.
type svgShape struct {
	XMLName  xml.Name
	D        string     `xml:"d,attr"`
	X        string     `xml:"x,attr"`
	Y        string     `xml:"y,attr"`
	W        string     `xml:"width,attr"`
	H        string     `xml:"height,attr"`
	CX       string     `xml:"cx,attr"`
	CY       string     `xml:"cy,attr"`
	R        string     `xml:"r,attr"`
	RX       string     `xml:"rx,attr"`
	RY       string     `xml:"ry,attr"`
	X1       string     `xml:"x1,attr"`
	Y1       string     `xml:"y1,attr"`
	X2       string     `xml:"x2,attr"`
	Y2       string     `xml:"y2,attr"`
	Points   string     `xml:"points,attr"`
	Children []svgShape `xml:",any"`
}

// ReadSVG turns an SVG file into a symbol named name.
//
// height is how tall to make it in staff spaces; zero asks for
// DefaultSVGHeight. The shape keeps its proportions, is centred on the
// staff position vertically and starts at it horizontally — which is where
// a mark attached to a note belongs.
func ReadSVG(r io.Reader, name string, height float64) (*Symbol, error) {
	var doc svgDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var all Path
	var errs []string
	collect(doc.Shapes, &all, &errs)
	if len(all) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("%s: %s", name, errs[0])
		}
		return nil, fmt.Errorf("%s: no outlines in it", name)
	}

	// The box the drawing actually fills, rather than the one it declares.
	// A drawing program writes a page and puts a small shape in the corner
	// of it, and scaling the page would give a symbol mostly made of air.
	minX, minY, maxX, maxY := all.Bounds()
	w, h := maxX-minX, maxY-minY
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%s: the drawing has no size", name)
	}
	if height <= 0 {
		height = DefaultSVGHeight
	}
	scale := height / h

	// Moved so that the left edge is at zero and the middle is on the
	// staff position, which is how a bitmap symbol is registered too.
	out := all.Transform(scale, scale, -minX*scale, -minY*scale-height/2)

	return &Symbol{
		Name:    name,
		Kind:    FromVector,
		Path:    out.String(),
		Advance: w * scale,
	}, nil
}

// collect walks the elements, turning each into outlines.
func collect(shapes []svgShape, into *Path, errs *[]string) {
	for _, s := range shapes {
		switch strings.ToLower(s.XMLName.Local) {
		case "path":
			if s.D == "" {
				break
			}
			p, err := ParsePath(s.D)
			if err != nil {
				*errs = append(*errs, err.Error())
				break
			}
			*into = append(*into, p...)
		case "rect":
			x, y := num(s.X), num(s.Y)
			w, h := num(s.W), num(s.H)
			if w > 0 && h > 0 {
				*into = append(*into,
					Seg{Kind: SegMove, P: [3]Point{{X: x, Y: y}}},
					Seg{Kind: SegLine, P: [3]Point{{X: x + w, Y: y}}},
					Seg{Kind: SegLine, P: [3]Point{{X: x + w, Y: y + h}}},
					Seg{Kind: SegLine, P: [3]Point{{X: x, Y: y + h}}},
					Seg{Kind: SegClose})
			}
		case "circle":
			*into = append(*into, ellipse(num(s.CX), num(s.CY), num(s.R), num(s.R))...)
		case "ellipse":
			*into = append(*into, ellipse(num(s.CX), num(s.CY), num(s.RX), num(s.RY))...)
		case "line":
			*into = append(*into,
				Seg{Kind: SegMove, P: [3]Point{{X: num(s.X1), Y: num(s.Y1)}}},
				Seg{Kind: SegLine, P: [3]Point{{X: num(s.X2), Y: num(s.Y2)}}})
		case "polygon", "polyline":
			pts := points(s.Points)
			if len(pts) < 2 {
				break
			}
			*into = append(*into, Seg{Kind: SegMove, P: [3]Point{pts[0]}})
			for _, pt := range pts[1:] {
				*into = append(*into, Seg{Kind: SegLine, P: [3]Point{pt}})
			}
			if strings.ToLower(s.XMLName.Local) == "polygon" {
				*into = append(*into, Seg{Kind: SegClose})
			}
		}
		// A group, a layer, a <defs>: whatever it is, what is inside it
		// may be a shape.
		collect(s.Children, into, errs)
	}
}

// ellipse is the four cubics that draw one.
//
// The magic number is the one every drawing program uses: a quarter of an
// ellipse is matched to within a thousandth of its radius by a cubic whose
// controls stand this far along the tangents.
const kappa = 0.5522847498

func ellipse(cx, cy, rx, ry float64) Path {
	if rx <= 0 || ry <= 0 {
		return nil
	}
	ox, oy := rx*kappa, ry*kappa
	return Path{
		{Kind: SegMove, P: [3]Point{{X: cx - rx, Y: cy}}},
		{Kind: SegCubic, P: [3]Point{
			{X: cx - rx, Y: cy - oy}, {X: cx - ox, Y: cy - ry}, {X: cx, Y: cy - ry}}},
		{Kind: SegCubic, P: [3]Point{
			{X: cx + ox, Y: cy - ry}, {X: cx + rx, Y: cy - oy}, {X: cx + rx, Y: cy}}},
		{Kind: SegCubic, P: [3]Point{
			{X: cx + rx, Y: cy + oy}, {X: cx + ox, Y: cy + ry}, {X: cx, Y: cy + ry}}},
		{Kind: SegCubic, P: [3]Point{
			{X: cx - ox, Y: cy + ry}, {X: cx - rx, Y: cy + oy}, {X: cx - rx, Y: cy}}},
		{Kind: SegClose},
	}
}

// num reads an SVG length, ignoring any unit after it. A symbol is scaled
// to the size it is asked for, so what the file called its units does not
// survive the journey anyway.
func num(s string) float64 {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) {
		c := s[end]
		if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' ||
			c == 'e' || c == 'E' {
			end++
			continue
		}
		break
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// points reads the "x,y x,y" of a polygon.
func points(s string) []Point {
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\n' || r == '\t' || r == '\r'
	})
	var out []Point
	for i := 0; i+1 < len(f); i += 2 {
		out = append(out, Point{X: num(f[i]), Y: num(f[i+1])})
	}
	return out
}
