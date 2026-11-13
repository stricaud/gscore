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

	// Crop is the part of the drawing this file holds, in staff spaces: a
	// page of a paginated score, so that one drawing becomes one printed
	// page and a page break falls where the engraver put it.
	//
	// It leaves out what does not reach into the box, exactly as Window
	// does, and differs in what the drawing then is: a cropped file is the
	// size of its box and its contents sit at the top of it, where a
	// windowed one keeps the whole score's size and leaves a hole. The one
	// is a page; the other is a screenful of a longer roll.
	//
	// Printing a paginated score as a single tall drawing was what made
	// this necessary: the browser cut it into pages wherever the paper ran
	// out, which was through the middle of a staff.
	Crop *Box

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

	// Scope confines the stylesheet to this drawing, under a class of this
	// name, for an SVG that will be put inline in a page beside others.
	//
	// An inline <svg> hands its <style> to the page it is in: the rules go
	// into the document's own stylesheet and are not confined to the
	// drawing they were written for, however they look. So the fifteen
	// little staves of the key signature picker, drawn at eleven units to
	// the space and dropped into the page, set the size of every glyph in
	// the score behind them — and the music stayed that way after the
	// picker was closed, because the picture is still in the page.
	//
	// Empty leaves the rules bare, which is right for a drawing that is a
	// document on its own.
	Scope string
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

// writeOpen writes the <svg> the score is drawn in, and the rules it is drawn
// under. The size is the whole score's whether a window of it is being drawn
// or all of it, so a coordinate means the same thing either way.
func writeOpen(b *strings.Builder, lay *engrave.Layout, opt Options) {
	s := opt.StaffSpace
	width, height := lay.Width*s, lay.Height*s
	x0, y0 := 0.0, 0.0
	if c := opt.Crop; c != nil {
		x0, y0 = c.X0*s, c.Y0*s
		width, height = (c.X1-c.X0)*s, (c.Y1-c.Y0)*s
	}
	class := "gscore-score"
	if opt.Scope != "" {
		class += " " + scopeClass(opt.Scope)
	}
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.1f" height="%.1f" `+
		`viewBox="%.1f %.1f %.1f %.1f" class="%s">`+"\n",
		width, height, x0, y0, width, height, class)
	writeStyle(b, opt, s)
}

// Shell writes the drawing with no music in it: the <svg> of the score's full
// size, and the rules the music will be drawn under when it arrives.
//
// It is the other half of Fragment. A page too large to carry a long score
// whole carries this instead, the right size and so with the right scrollbar,
// and fills it with fragments as they are wanted. Beethoven's ninth is nine
// hundred thousand shapes and two and a third million units tall; the shell
// of it is a few hundred bytes.
func Shell(w io.Writer, lay *engrave.Layout, opt Options) error {
	var b strings.Builder
	writeOpen(&b, lay, opt)
	b.WriteString("</svg>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// Render writes the layout as a standalone SVG document.
func Render(w io.Writer, lay *engrave.Layout, opt Options) error {
	var b strings.Builder
	s := opt.StaffSpace

	if !opt.Fragment {
		writeOpen(&b, lay, opt)
	}
	win := opt.Window
	// A cropped drawing holds what reaches into its own box and nothing
	// else, which is the same question a window asks.
	if win == nil && opt.Crop != nil {
		win = opt.Crop
	}

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
		// Both go in one inline style, not in presentation attributes. A
		// presentation attribute is author-level styling of the lowest
		// priority, so the .g rule in the stylesheet above beat it and every
		// glyph asking to be drawn smaller — a grace note, the accidental in
		// a chord symbol — came out full size.
		style := ""
		if g.Size > 0 {
			style += fmt.Sprintf("font-size:%.2fpx;", g.Size*s)
		}
		// A glyph that is a reading aid rather than ink: the beat marks. The
		// .g rule fills them with the score's colour, so saying it here is
		// the only way it holds.
		if g.Colour != "" {
			style += "fill:" + html.EscapeString(g.Colour) + ";"
		}
		if style != "" {
			attr += fmt.Sprintf(` style="%s"`, style)
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
		if t.Colour != "" {
			style += "fill:" + html.EscapeString(t.Colour) + ";"
		}
		// A line set in more than one style is one text of several
		// pieces, so that the browser lays them out and anchors the line
		// by what it really drew. Spelling it as several texts, each
		// placed by a measurement made here, is how a title in two faces
		// came out with its two halves overlapping: the widths were read
		// from a table and the type was set by the browser, and a table
		// is near enough to reserve room by and not near enough to set
		// type with.
		//
		// The spaces are kept, because the space between two such pieces
		// is at the end of the first of them and SVG drops it otherwise.
		inner := html.EscapeString(t.S)
		if len(t.Runs) > 0 {
			var runs strings.Builder
			for _, r := range t.Runs {
				rs := ""
				if r.Italic != t.Italic {
					rs += "font-style:" + ifElse(r.Italic, "italic", "normal") + ";"
				}
				if r.Bold != t.Bold {
					rs += "font-weight:" + ifElse(r.Bold, "bold", "normal") + ";"
				}
				if r.Colour != "" {
					rs += "fill:" + html.EscapeString(r.Colour) + ";"
				}
				if r.Size > 0 && r.Size != t.Size {
					rs += fmt.Sprintf("font-size:%.2fpx;", r.Size*s)
				}
				fmt.Fprintf(&runs, `<tspan style="%s">%s</tspan>`, rs,
					html.EscapeString(r.S))
			}
			inner = runs.String()
		}
		fmt.Fprintf(&b, `<text class="t" xml:space="preserve" x="%.2f" y="%.2f" font-size="%.2f" text-anchor="%s" style="%s">%s</text>`+"\n",
			t.X*s, t.Y*s, t.Size*s, anchor, style, inner)
	}

	if !opt.Fragment {
		b.WriteString("</svg>\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// ifElse is the conditional expression Go does not have, for the one place
// here that wants a word chosen by a flag.
func ifElse(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// writeStyle writes the rules the shapes are drawn under.
// scopeClass is the class a scoped drawing's rules hang from. A class
// rather than an id, because a picker draws the same picture several times
// over and there would be as many of them in the page at once.
func scopeClass(scope string) string { return "gscore-" + scope }

func writeStyle(b *strings.Builder, opt Options, s float64) {
	b.WriteString("<style>\n")
	if opt.EmbedFont {
		// The font travels with the file so the score renders the same
		// wherever it is opened, with nothing to install.
		fmt.Fprintf(b, `@font-face{font-family:"%s";src:url("data:font/otf;base64,%s") format("opentype");}`+"\n",
			smufl.FontFamily, base64.StdEncoding.EncodeToString(smufl.FontData()))
	}
	// What the rules hang from. Unscoped they are the whole page's, which
	// is what a drawing that is a file on its own wants and is exactly
	// wrong for one that is going inside a page that has a score in it.
	self, under := ".gscore-score", ""
	if opt.Scope != "" {
		self = "." + scopeClass(opt.Scope)
		under = self + " "
	}
	fmt.Fprintf(b, `%s{background:#fff;}`+"\n", self)
	fmt.Fprintf(b, `%s.g{font-family:"%s";font-size:%.3fpx;fill:#000;}`+"\n",
		under, smufl.FontFamily, 4*s)
	// white-space, because a line of a credit set in two faces is drawn as
	// two pieces and the space between them is at the end of the first.
	// Left to itself SVG throws that space away — xml:space says to keep
	// it, and is honoured when the drawing is parsed as XML but not always
	// when it is put into a page as markup, so the rule says it too.
	// The reading face is the one the engraver measured and the one a PDF
	// prints in, and those have to be the same face.
	//
	// The words on a score are not laid out by the browser alone: the
	// engraver reserves the room they need and then puts the next thing
	// after them, and it does that from a table of Times widths. Drawn in
	// Georgia, which is a fifth wider in the lower case, the next thing
	// landed inside the words — a metronome mark came out with its note
	// written over the last letter of "Molto moderato". Times New Roman is
	// the same widths as the table, on every machine that has it, and
	// Liberation Serif is drawn to the same widths for a machine that has
	// not; the score on the screen is then the score that prints.
	fmt.Fprintf(b, `%s.t{font-family:"Times New Roman",Times,"Liberation Serif",serif;fill:#000;white-space:pre;}`+"\n", under)
	fmt.Fprintf(b, `%sline{stroke:#000;stroke-linecap:butt;}`+"\n", under)
	fmt.Fprintf(b, `%spath.curve{fill:none;stroke:#000;}`+"\n", under)
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
// drawVectorSymbol draws a symbol that is an outline rather than a grid.
//
// Its path is already in staff spaces, measured from the symbol's own
// origin, so all that is left is the staff size and where on the page it
// goes. Written as a path and not as a picture: it is the one kind of
// symbol that does not have to be resampled to be made larger, and
// turning it into pixels to draw it would throw that away.
func drawVectorSymbol(b *strings.Builder, opt Options, g engrave.Glyph,
	sym *symbol.Symbol, interactive bool) error {
	if sym.Path == "" {
		return nil
	}
	s := opt.StaffSpace
	k := g.ScaleOr1()
	attr := ""
	if interactive {
		attr = fmt.Sprintf(` data-symbol="%s"`, html.EscapeString(g.Name))
		if g.Ref != 0 {
			attr += fmt.Sprintf(` data-ref="%d"`, g.Ref)
		}
		attr += fmt.Sprintf(` data-scale="%.3f"`, k)
	}
	// One transform rather than a rewritten path: the outline is the
	// same shape wherever it is put, and scaling it here keeps the file
	// small when a symbol is used many times. How large this placement is
	// drawn goes into the same transform, after the move, so the drawing
	// grows about the point it is pinned to.
	fmt.Fprintf(b, `<path d="%s" fill="currentColor" transform="translate(%.2f,%.2f) scale(%.4f)"%s/>`+"\n",
		html.EscapeString(sym.Path), g.X*s, g.Y*s, s*k, attr)
	return nil
}

func drawSymbol(b *strings.Builder, opt Options, g engrave.Glyph, interactive bool) error {
	if opt.Symbols == nil {
		return nil
	}
	sym, ok := opt.Symbols.Lookup(g.Name)
	if !ok {
		return nil
	}
	if sym.Kind == symbol.FromVector {
		return drawVectorSymbol(b, opt, g, sym, interactive)
	}
	if sym.Kind != symbol.FromPixels || sym.Pixels == nil {
		return nil
	}
	px := sym.Pixels
	s := opt.StaffSpace

	var png bytes.Buffer
	if err := pngEncode(&png, px.Image()); err != nil {
		return err
	}
	// How large this placement is drawn: the scale multiplies everything
	// measured from the grid, the size and the origin alike, so a symbol
	// made larger grows about the point it is pinned to rather than away
	// from it.
	k := g.ScaleOr1()
	w := px.WidthInSpaces() * k * s
	h := px.HeightInSpaces() * k * s
	x := g.X*s - px.OriginX*px.PixelSize*k*s
	y := g.Y*s - px.OriginY*px.PixelSize*k*s

	// The editor needs to be able to point at it: which element it belongs to,
	// and which symbol it is, so that dragging it knows what to move.
	attr := ""
	if interactive {
		attr = fmt.Sprintf(` data-symbol="%s"`, html.EscapeString(g.Name))
		if g.Ref != 0 {
			attr += fmt.Sprintf(` data-ref="%d"`, g.Ref)
		}
		// What it is drawn at, so that a handle dragged on the page knows
		// what it is changing from.
		attr += fmt.Sprintf(` data-scale="%.3f"`, k)
	}
	fmt.Fprintf(b, `<image x="%.2f" y="%.2f" width="%.2f" height="%.2f" `+
		`image-rendering="pixelated"%s href="data:image/png;base64,%s"/>`+"\n",
		x, y, w, h, attr, base64.StdEncoding.EncodeToString(png.Bytes()))
	return nil
}
