/*
 * pdf.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package pdf renders an engraved layout to a PDF file.
//
// This is the counterpart of render/svg and works the same way: the layout
// holds lines, curves, glyphs and text in staff spaces, and this turns them
// into a file. Nothing is decided here that the engraver has not already
// decided — a renderer that made spacing decisions would be a second engraver
// with different opinions, and a score would print differently from how it
// looks on screen.
//
// Two things the SVG renderer does not have to think about:
//
// PDF's origin is the bottom left corner and y grows upwards; a score is laid
// out from the top down. Every y is flipped once, here, rather than the
// layout being expressed twice.
//
// A PDF has pages. The layout is one tall drawing, so it is cut into pages
// between systems — never through one, which is what Layout.Systems is for.
package pdf

import (
	"io"
	"strings"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/pdf"
	"github.com/stricaud/gscore/internal/smufl"
	"github.com/stricaud/gscore/internal/symbol"
)

// Options control one rendering.
type Options struct {
	// StaffSpace is how big one staff space is, in points. Engravers measure
	// type size by the distance between two staff lines; 7 points is about
	// what a printed part uses and 5 is a pocket score.
	StaffSpace float64

	// Page size and margins, in points.
	PageWidth, PageHeight          float64
	MarginLeft, MarginRight        float64
	MarginTop, MarginBottom        float64

	// Symbols resolves the names the music font does not carry.
	Symbols *symbol.Registry

	// Title, Author and Subject go in the document's own properties, which is
	// what a library or a file manager shows.
	Title, Author, Subject string
}

// DefaultOptions is A4 with the margins a part is usually printed with.
func DefaultOptions() Options {
	return Options{
		StaffSpace:   6.2,
		PageWidth:    float64(pdf.A4Width),
		PageHeight:   float64(pdf.A4Height),
		MarginLeft:   48,
		MarginRight:  40,
		MarginTop:    46,
		MarginBottom: 46,
		Title:        "",
	}
}

// Letter returns the same thing on US paper.
func Letter() Options {
	o := DefaultOptions()
	o.PageWidth = float64(pdf.LetterWidth)
	o.PageHeight = float64(pdf.LetterHeight)
	return o
}

// Render writes one layout as a PDF.
func Render(w io.Writer, lay *engrave.Layout, opt Options) error {
	doc := pdf.New()
	doc.Title = opt.Title
	doc.Author = opt.Author
	doc.Subject = opt.Subject
	doc.Creator = "gscore"

	r := &renderer{doc: doc, lay: lay, opt: opt, s: pdf.Point(opt.StaffSpace)}

	music, err := doc.Embed("Bravura", smufl.FontData())
	if err != nil {
		return err
	}
	r.music = music
	r.text = doc.Standard(pdf.Times)
	r.textBold = doc.Standard(pdf.TimesBold)
	r.textItalic = doc.Standard(pdf.TimesItalic)
	r.textBoldItalic = doc.Standard(pdf.TimesBoldItalic)

	r.run()
	return doc.Write(w)
}

// RenderAll writes several layouts into one PDF, each starting a new page.
//
// This is what a tune book is: forty exercises are forty layouts and one
// document, not forty files. It is also how a score and its parts are printed
// together — the conductor's copy, then one page per player.
func RenderAll(w io.Writer, layouts []*engrave.Layout, opt Options) error {
	doc := pdf.New()
	doc.Title = opt.Title
	doc.Author = opt.Author
	doc.Subject = opt.Subject
	doc.Creator = "gscore"

	music, err := doc.Embed("Bravura", smufl.FontData())
	if err != nil {
		return err
	}
	for _, lay := range layouts {
		r := &renderer{doc: doc, lay: lay, opt: opt, s: pdf.Point(opt.StaffSpace)}
		r.music = music
		r.text = doc.Standard(pdf.Times)
		r.textBold = doc.Standard(pdf.TimesBold)
		r.textItalic = doc.Standard(pdf.TimesItalic)
		r.textBoldItalic = doc.Standard(pdf.TimesBoldItalic)
		r.run()
	}
	return doc.Write(w)
}

type renderer struct {
	doc *pdf.Doc
	lay *engrave.Layout
	opt Options
	s   pdf.Point // points per staff space

	music          *pdf.Font
	text           *pdf.Font
	textBold       *pdf.Font
	textItalic     *pdf.Font
	textBoldItalic *pdf.Font

	page *pdf.Page
	// offset is how far down the layout this page starts, in staff spaces.
	offset float64
}

// run cuts the layout into pages and draws each one.
func (r *renderer) run() {
	for _, band := range r.pages() {
		r.offset = band.top
		r.page = r.doc.NewPage(pdf.Point(r.opt.PageWidth), pdf.Point(r.opt.PageHeight))
		r.page.SetGray(0)

		// Nothing from the next page's systems may bleed into this one. The
		// clip is the band, not the page: a stem or a slur is allowed to
		// reach a little past what it belongs to, and without this the
		// system that starts the next page is drawn across the bottom of
		// this one.
		r.page.Save()
		top := r.y(band.top) + r.s*0.5
		bottom := r.y(band.bottom) - r.s*2
		r.page.Clip(0, bottom, pdf.Point(r.opt.PageWidth), top-bottom)
		r.draw(band.top, band.bottom)
		r.page.Restore()
		r.page.Close()
	}
}

// band is one page's worth of the layout, in staff spaces.
type band struct{ top, bottom float64 }

// pages cuts the layout between systems so that none is split in two.
//
// A system that will not fit a page on its own goes on a page of its own and
// overflows, which is better than dropping it: a score with one enormous
// system is a score somebody meant to print on a larger sheet, and saying so
// by printing it is clearer than silently losing a line of music.
func (r *renderer) pages() []band {
	usable := (r.opt.PageHeight - r.opt.MarginTop - r.opt.MarginBottom) / r.opt.StaffSpace
	if usable <= 0 {
		usable = 1
	}

	systems := r.lay.Systems
	if len(systems) == 0 {
		return []band{{0, r.lay.Height}}
	}

	var out []band
	// The first page starts at the very top of the drawing, because the
	// title block sits above the first system and belongs with it.
	top := 0.0
	for i, sys := range systems {
		if sys.Bottom-top <= usable {
			continue // it still fits
		}
		if i == 0 {
			// One system too tall for a page on its own. Printing it and
			// letting it overflow says so; dropping it would not.
			continue
		}
		// The page ends where the next system's own territory begins, not
		// where the last one's staff lines stopped: the chord symbols and
		// diagrams of the system overleaf sit above its staff and would
		// otherwise be printed along the bottom of this page.
		out = append(out, band{top, sys.Top - 0.5})
		top = sys.Top
	}
	out = append(out, band{top, r.lay.Height})
	return out
}

// x and y convert a point in the layout to one on the page. The flip happens
// here and nowhere else.
func (r *renderer) x(v float64) pdf.Point {
	return pdf.Point(r.opt.MarginLeft) + pdf.Point(v)*r.s
}

func (r *renderer) y(v float64) pdf.Point {
	return pdf.Point(r.opt.PageHeight-r.opt.MarginTop) - pdf.Point(v-r.offset)*r.s
}

// visible says whether something at this height belongs on this page.
func visible(v, top, bottom float64) bool {
	// Generous at both ends: a stem or a slur reaches past the note it
	// belongs to, and clipping decides the rest.
	return v >= top-12 && v <= bottom+12
}

func (r *renderer) draw(top, bottom float64) {
	r.drawLines(top, bottom)
	r.drawCircles(top, bottom)
	r.drawCurves(top, bottom)
	r.drawGlyphs(top, bottom)
	r.drawTexts(top, bottom)
}

func (r *renderer) drawLines(top, bottom float64) {
	for _, ln := range r.lay.Lines {
		if !visible(ln.Y1, top, bottom) && !visible(ln.Y2, top, bottom) {
			continue
		}
		if ln.Dash > 0 {
			r.page.Dash(pdf.Point(ln.Dash)*r.s, pdf.Point(ln.Dash)*r.s*0.8)
			r.page.SetLineWidth(float64(pdf.Point(ln.Thickness) * r.s))
			r.page.Line(r.x(ln.X1), r.y(ln.Y1), r.x(ln.X2), r.y(ln.Y2))
			r.page.Dash(0, 0)
			continue
		}
		// A horizontal rule — a staff line, a beam, a bracket of a repeat —
		// is drawn as a filled rectangle rather than a stroke. A stroke is
		// centred on its path and rounds to the device grid from the middle;
		// a rectangle lands exactly where it was put, which is what keeps
		// five staff lines evenly spaced at small sizes.
		if ln.Y1 == ln.Y2 {
			h := pdf.Point(ln.Thickness) * r.s
			x1, x2 := r.x(ln.X1), r.x(ln.X2)
			if x2 < x1 {
				x1, x2 = x2, x1
			}
			r.page.Rect(x1, r.y(ln.Y1)-h/2, x2-x1, h)
			continue
		}
		if ln.X1 == ln.X2 {
			w := pdf.Point(ln.Thickness) * r.s
			y1, y2 := r.y(ln.Y1), r.y(ln.Y2)
			if y2 < y1 {
				y1, y2 = y2, y1
			}
			r.page.Rect(r.x(ln.X1)-w/2, y1, w, y2-y1)
			continue
		}
		r.page.SetLineWidth(float64(pdf.Point(ln.Thickness) * r.s))
		r.page.Line(r.x(ln.X1), r.y(ln.Y1), r.x(ln.X2), r.y(ln.Y2))
	}
}

func (r *renderer) drawCircles(top, bottom float64) {
	for _, c := range r.lay.Circles {
		if !visible(c.Y, top, bottom) {
			continue
		}
		r.page.Circle(r.x(c.X), r.y(c.Y), pdf.Point(c.Radius)*r.s,
			c.Filled, pdf.Point(c.Thickness)*r.s)
	}
}

func (r *renderer) drawCurves(top, bottom float64) {
	for _, c := range r.lay.Curves {
		if !visible(c.Y1, top, bottom) && !visible(c.Y2, top, bottom) {
			continue
		}
		r.page.Quad(r.x(c.X1), r.y(c.Y1), r.x(c.CX), r.y(c.CY),
			r.x(c.X2), r.y(c.Y2), pdf.Point(c.Thickness)*r.s)
	}
}

func (r *renderer) drawGlyphs(top, bottom float64) {
	for _, g := range r.lay.Glyphs {
		if !visible(g.Y, top, bottom) {
			continue
		}
		run, ok := smufl.Glyph(g.Name)
		if !ok {
			r.drawSymbol(g)
			continue
		}
		// A SMuFL glyph is drawn at a type size of four staff spaces: that is
		// the convention the whole standard rests on, and why the same
		// numbers work for every font that follows it.
		size := r.s * 4
		if g.Size > 0 {
			size = pdf.Point(g.Size) * r.s
		}
		r.page.Text(r.music, size, r.x(g.X), r.y(g.Y), string(run))
	}
}

// drawSymbol places one hand-drawn symbol, which is a grid of pixels rather
// than a glyph.
func (r *renderer) drawSymbol(g engrave.Glyph) {
	if r.opt.Symbols == nil {
		return
	}
	sym, ok := r.opt.Symbols.Lookup(g.Name)
	if !ok || sym.Kind != symbol.FromPixels || sym.Pixels == nil {
		return
	}
	px := sym.Pixels
	img := r.doc.Image(g.Name, px.Image())
	if img == nil {
		return
	}
	w := pdf.Point(px.WidthInSpaces()) * r.s
	h := pdf.Point(px.HeightInSpaces()) * r.s
	// The origin is the point in the grid that lands where the symbol was
	// placed. Across, that is a simple shift left; down, the layout measures
	// from the top of the picture and a PDF places images by their bottom
	// corner, so the rest of the height has to be taken off as well.
	x := r.x(g.X) - pdf.Point(px.OriginX*px.PixelSize)*r.s
	y := r.y(g.Y) + pdf.Point(px.OriginY*px.PixelSize)*r.s - h
	r.page.Image(img, x, y, w, h)
}

func (r *renderer) drawTexts(top, bottom float64) {
	for _, t := range r.lay.Texts {
		if !visible(t.Y, top, bottom) {
			continue
		}
		f := r.text
		switch {
		case t.Bold && t.Italic:
			f = r.textBoldItalic
		case t.Bold:
			f = r.textBold
		case t.Italic:
			f = r.textItalic
		}
		size := pdf.Point(t.Size) * r.s
		x := r.x(t.X)
		// PDF has no text-anchor: a string is drawn from where it starts, so
		// centring and right-aligning are done by measuring it first.
		switch t.Anchor {
		case "middle":
			x -= r.page.TextWidth(f, size, t.S) / 2
		case "end":
			x -= r.page.TextWidth(f, size, t.S)
		}
		// A line break in a text block — a composer credit over three lines —
		// is drawn as the lines it is.
		lines := strings.Split(t.S, "\n")
		for i, line := range lines {
			lx := r.x(t.X)
			switch t.Anchor {
			case "middle":
				lx -= r.page.TextWidth(f, size, line) / 2
			case "end":
				lx -= r.page.TextWidth(f, size, line)
			}
			r.page.Text(f, size, lx, r.y(t.Y)-pdf.Point(i)*size*1.2, line)
		}
	}
}
