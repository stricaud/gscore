/*
 * svg.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package svg draws an engraved layout as SVG.
//
// SVG is the right target for notation: it is resolution independent, so one
// renderer serves the screen at any zoom and the printed page, and it is what
// the editor's canvas shows. The C program drew with gdk_draw_point and
// gdk_draw_line straight onto a fixed-size pixmap, which is why it could
// never zoom or print.
package svg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/png"
	"io"
	"math"
	"strings"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/smufl"
	"github.com/stricaud/gscore/internal/symbol"
)

func pngEncode(w io.Writer, img image.Image) error { return png.Encode(w, img) }

// Options control one rendering.
type Options struct {
	// Symbols resolves names the music font does not carry: the user's own
	// drawings. Without it a custom symbol would simply not appear.
	Symbols *symbol.Registry

	// StaffSpace is how many SVG units one staff space becomes. Everything
	// scales from it, because every coordinate the engraver produces is in
	// staff spaces.
	StaffSpace float64
	// EmbedFont writes the music font into the file as a data URI, so the SVG
	// renders correctly anywhere on its own. It costs about 1.2 MB.
	EmbedFont bool
	// Interactive adds the class and data attributes the editor uses for hit
	// testing.
	Interactive bool

	// Fragment writes the shapes alone, with no <svg> around them and no
	// stylesheet: a piece to be put inside a drawing that is already there.
	//
	// It is how a long score is drawn a piece at a time without rebuilding
	// what is already on the page. The editor keeps one drawing the size of
	// the whole score and fills it with these, adding one at the edge it is
	// moving towards and dropping the one it has left behind, so scrolling
	// costs one piece rather than everything in view.
	Fragment bool

	// Window draws only the part of the score that reaches into a box,
	// leaving the rest out. Nil draws the whole of it.
	//
	// It is for a score too big to put on a page all at once. Beethoven's
	// ninth is fifty-three staves of two and a half thousand bars, which
	// comes to nine hundred thousand shapes: a browser handed all of them
	// spends seconds laying them out and then stutters at every scroll,
	// because every repaint walks the lot to work out what is on screen.
	// Drawn a screenful at a time it is a few thousand.
	//
	// The drawing keeps its full width and height either way, so a
	// coordinate means the same thing whether the whole score was written
	// or a window of it. Nothing that reads the drawing has to know.
	Window *Box
}

// Box is a rectangle in staff spaces.
type Box struct{ X0, Y0, X1, Y1 float64 }

// touches says whether a shape's own box reaches into the window.
//
// Generously: a box that only grazes the edge is drawn. The cost of drawing
// one shape too many is one shape, and the cost of leaving one out is a gap
// in the music.
func (b *Box) touches(x0, y0, x1, y1 float64) bool {
	if b == nil {
		return true
	}
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return x1 >= b.X0 && x0 <= b.X1 && y1 >= b.Y0 && y0 <= b.Y1
}

// glyphSlack is how far a glyph may reach beyond the point it is drawn at,
// in staff spaces, for deciding whether it is in the window.
//
// The tallest in the font is a clef at about two and a half spaces either
// way; six is comfortably past anything, and being generous here costs a
// handful of shapes at the edge of a window that is a screenful wider than
// the screen anyway.
const glyphSlack = 6

// DefaultOptions returns a sensible rendering setup.
func DefaultOptions() Options {
	return Options{StaffSpace: 8, EmbedFont: true}
}

// Render writes the layout as a standalone SVG document.
func Render(w io.Writer, lay *engrave.Layout, opt Options) error {
	var b strings.Builder
	s := opt.StaffSpace

	if !opt.Fragment {
		width, height := lay.Width*s, lay.Height*s
		fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.1f" height="%.1f" `+
			`viewBox="0 0 %.1f %.1f" class="gscore-score">`+"\n", width, height, width, height)
		writeStyle(&b, opt, s)
	}
	win := opt.Window

	for _, ln := range lay.Lines {
		if !win.touches(ln.X1, ln.Y1, ln.X2, ln.Y2) {
			continue
		}
		dash := ""
		if ln.Dash > 0 {
			dash = fmt.Sprintf(` stroke-dasharray="%.2f %.2f"`, ln.Dash*s, ln.Dash*s*0.8)
		}
		fmt.Fprintf(&b, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke-width="%.2f"%s/>`+"\n",
			ln.X1*s, ln.Y1*s, ln.X2*s, ln.Y2*s, ln.Thickness*s, dash)
	}

	for _, q := range lay.Quads {
		if !win.touches(spread(q.X[:], q.Y[:])) {
			continue
		}
		b.WriteString(`<polygon fill="#000" points="`)
		for i := range q.X {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%.2f,%.2f", q.X[i]*s, q.Y[i]*s)
		}
		b.WriteString("\"/>\n")
	}

	for _, c := range lay.Circles {
		if !win.touches(c.X-c.Radius, c.Y-c.Radius, c.X+c.Radius, c.Y+c.Radius) {
			continue
		}
		if c.Filled {
			fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="#000"/>`+"\n",
				c.X*s, c.Y*s, c.Radius*s)
			continue
		}
		fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="none" `+
			`stroke="#000" stroke-width="%.2f"/>`+"\n",
			c.X*s, c.Y*s, c.Radius*s, c.Thickness*s)
	}

	for _, c := range lay.Curves {
		// A quadratic stays inside the box of its ends and its control
		// point, so that box is a safe outline and a cheap one.
		if !win.touches(
			math.Min(c.X1, math.Min(c.X2, c.CX)), math.Min(c.Y1, math.Min(c.Y2, c.CY)),
			math.Max(c.X1, math.Max(c.X2, c.CX)), math.Max(c.Y1, math.Max(c.Y2, c.CY))) {
			continue
		}
		fmt.Fprintf(&b, `<path class="curve" d="M %.2f %.2f Q %.2f %.2f %.2f %.2f" stroke-width="%.2f"/>`+"\n",
			c.X1*s, c.Y1*s, c.CX*s, c.CY*s, c.X2*s, c.Y2*s, c.Thickness*s)
	}

	for _, g := range lay.Glyphs {
		if !win.touches(g.X-glyphSlack, g.Y-glyphSlack, g.X+glyphSlack, g.Y+glyphSlack) {
			continue
		}
		r, ok := smufl.Glyph(g.Name)
		if !ok {
			// Not in the font, so it is one of the user's own symbols.
			if err := drawSymbol(&b, opt, g, opt.Interactive); err != nil {
				return err
			}
			continue
		}
		attr := ""
		if opt.Interactive && g.Ref != 0 {
			attr = fmt.Sprintf(` data-ref="%d"`, g.Ref)
		}
		if g.Size > 0 {
			// An inline style, not a font-size attribute. A presentation
			// attribute is author-level styling of the lowest priority, so
			// the .g rule in the stylesheet above beat it and every glyph
			// asking to be drawn smaller — a grace note, the accidental in a
			// chord symbol — came out full size.
			attr += fmt.Sprintf(` style="font-size:%.2fpx"`, g.Size*s)
		}
		fmt.Fprintf(&b, `<text class="g" x="%.2f" y="%.2f"%s>%s</text>`+"\n",
			g.X*s, g.Y*s, attr, html.EscapeString(string(r)))
	}

	for _, t := range lay.Texts {
		// Written from its baseline, and from either end of it or its
		// middle depending on the anchor, so the box is the widest the
		// word could be in any of those.
		if w := float64(len(t.S)) * t.Size; !win.touches(
			t.X-w, t.Y-t.Size*2, t.X+w, t.Y+t.Size) {
			continue
		}
		anchor := t.Anchor
		if anchor == "" {
			anchor = "start"
		}
		style := ""
		if t.Italic {
			style += "font-style:italic;"
		}
		if t.Bold {
			style += "font-weight:bold;"
		}
		fmt.Fprintf(&b, `<text class="t" x="%.2f" y="%.2f" font-size="%.2f" text-anchor="%s" style="%s">%s</text>`+"\n",
			t.X*s, t.Y*s, t.Size*s, anchor, style, html.EscapeString(t.S))
	}

	if !opt.Fragment {
		b.WriteString("</svg>\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeStyle writes the rules the shapes are drawn under.
func writeStyle(b *strings.Builder, opt Options, s float64) {
	b.WriteString("<style>\n")
	if opt.EmbedFont {
		// The font travels with the file so the score renders the same
		// wherever it is opened, with nothing to install.
		fmt.Fprintf(b, `@font-face{font-family:"%s";src:url("data:font/otf;base64,%s") format("opentype");}`+"\n",
			smufl.FontFamily, base64.StdEncoding.EncodeToString(smufl.FontData()))
	}
	fmt.Fprintf(b, `.gscore-score{background:#fff;}`+"\n")
	fmt.Fprintf(b, `.g{font-family:"%s";font-size:%.3fpx;fill:#000;}`+"\n", smufl.FontFamily, 4*s)
	fmt.Fprintf(b, `.t{font-family:Georgia,"Times New Roman",serif;fill:#000;}`+"\n")
	fmt.Fprintf(b, `line{stroke:#000;stroke-linecap:butt;}`+"\n")
	fmt.Fprintf(b, `path.curve{fill:none;stroke:#000;}`+"\n")
	b.WriteString("</style>\n")
}

// spread is the box a run of points covers.
func spread(xs, ys []float64) (x0, y0, x1, y1 float64) {
	if len(xs) == 0 || len(ys) == 0 {
		return 0, 0, 0, 0
	}
	x0, x1 = xs[0], xs[0]
	y0, y1 = ys[0], ys[0]
	for i := range xs {
		x0, x1 = math.Min(x0, xs[i]), math.Max(x1, xs[i])
	}
	for i := range ys {
		y0, y1 = math.Min(y0, ys[i]), math.Max(y1, ys[i])
	}
	return x0, y0, x1, y1
}

// drawSymbol places one hand-drawn symbol.
//
// A pixel grid is written into the file as an image, scaled by the staff size
// so that it keeps its musical proportions however far the score is zoomed,
// and rendered without smoothing so the pixels stay the pixels that were
// drawn.
func drawSymbol(b *strings.Builder, opt Options, g engrave.Glyph, interactive bool) error {
	if opt.Symbols == nil {
		return nil
	}
	sym, ok := opt.Symbols.Lookup(g.Name)
	if !ok || sym.Kind != symbol.FromPixels || sym.Pixels == nil {
		return nil
	}
	px := sym.Pixels
	s := opt.StaffSpace

	var png bytes.Buffer
	if err := pngEncode(&png, px.Image()); err != nil {
		return err
	}
	w := px.WidthInSpaces() * s
	h := px.HeightInSpaces() * s
	x := g.X*s - px.OriginX*px.PixelSize*s
	y := g.Y*s - px.OriginY*px.PixelSize*s

	// The editor needs to be able to point at it: which element it belongs to,
	// and which symbol it is, so that dragging it knows what to move.
	attr := ""
	if interactive {
		attr = fmt.Sprintf(` data-symbol="%s"`, html.EscapeString(g.Name))
		if g.Ref != 0 {
			attr += fmt.Sprintf(` data-ref="%d"`, g.Ref)
		}
	}
	fmt.Fprintf(b, `<image x="%.2f" y="%.2f" width="%.2f" height="%.2f" `+
		`image-rendering="pixelated"%s href="data:image/png;base64,%s"/>`+"\n",
		x, y, w, h, attr, base64.StdEncoding.EncodeToString(png.Bytes()))
	return nil
}
