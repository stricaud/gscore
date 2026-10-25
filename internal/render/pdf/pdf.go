/*
 * pdf.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/stricaud/gscore/internal/brand"
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
	PageWidth, PageHeight   float64
	MarginLeft, MarginRight float64
	MarginTop, MarginBottom float64

	// Symbols resolves the names the music font does not carry.
	Symbols *symbol.Registry

	// Title, Author and Subject go in the document's own properties, which is
	// what a library or a file manager shows.
	Title, Author, Subject string

	// Label names this piece in the top margin of its first page, beside
	// the maker's mark.
	//
	// For a part. A score is read whole and says whose it is on its title
	// page, but a part is pulled out of the document and handed to one
	// player — printed on its own, photographed on its own, mailed on its
	// own — and a sheet of music with no instrument on it is a sheet
	// somebody has to play a bar of to identify. Setting it where the eye
	// lands first, rather than only under the title, is what a publisher
	// does for the same reason.
	//
	// It also says this piece is shared by itself, so its first page
	// carries the maker's mark whether or not it is the first page of the
	// document.
	Label string

	// PageNumbers writes "2/5" along the foot of each page.
	//
	// Counted within this piece and not across the document: a part is
	// four pages, and the player holding it needs to know they have all
	// four of theirs, not that they are holding sheets 23 to 26 of
	// something. A single-page piece is not numbered at all — "1/1" tells
	// nobody anything.
	PageNumbers bool
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

// Landscape turns the paper on its side.
//
// A conductor's score is what it is for. Twelve staves to a system want a
// page that is wide rather than tall: upright, a full score either runs at a
// type size nobody can read across a pit or spills two systems onto three
// pages. The same is true of a single long line of music, which on an upright
// page spends half the sheet on margins.
//
// Turning the paper is all this does. How wide a system may be then follows
// from the paper, because LineWidth is asked rather than assumed.
func (o Options) Landscape() Options {
	if o.PageWidth < o.PageHeight {
		o.PageWidth, o.PageHeight = o.PageHeight, o.PageWidth
	}
	return o
}

// LineWidth is how much room a system has on this paper, in staff spaces,
// which is what the engraver needs telling before it lays anything out.
//
// Only the page setup knows it. The same score is engraved differently on A4
// upright, on A4 turned on its side and at a pocket score's type size, and a
// constant could only ever be right for one of the three — asked for a staff
// space of twelve, the engraver drew a system wider than the sheet and the
// music ran off the edge.
//
// The engraver keeps its own blank border at each side of the music, inside
// the page margins, so that is taken off here: what is left reaches the
// margin and no further.
func (o Options) LineWidth(sp engrave.Spacings) float64 {
	space := o.StaffSpace
	if space <= 0 {
		space = DefaultOptions().StaffSpace
	}
	w := (o.PageWidth-o.MarginLeft-o.MarginRight)/space - 2*sp.Margin
	// A system narrower than this is not a system. Rather than engrave
	// something impossible, print it too wide and let the page say so.
	if w < 12 {
		w = 12
	}
	return w
}

// Render writes one layout as a PDF.
func Render(w io.Writer, lay *engrave.Layout, opt Options) error {
	doc := pdf.New()
	doc.Title = opt.Title
	doc.Author = opt.Author
	doc.Subject = opt.Subject
	doc.Creator = brand.Name

	r := &renderer{doc: doc, lay: lay, opt: opt, s: pdf.Point(opt.StaffSpace),
		first: true}

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

// Section is one piece of a document: a drawing and the paper it goes on.
//
// The paper belongs to the piece and not to the document, because the pieces
// are not the same kind of thing. A conductor's score of seventeen staves is
// set small, and on its side if that is what holds it; the flute part behind
// it is one staff and belongs upright at a size somebody can read from a
// stand. Printing both at the score's settings is how a part comes out at
// three and a half points, which is what used to happen.
type Section struct {
	Layout *engrave.Layout
	Opt    Options
}

// RenderAll writes several layouts into one PDF, each starting a new page,
// all on the same paper.
//
// This is what a tune book is: forty exercises are forty layouts and one
// document, not forty files.
func RenderAll(w io.Writer, layouts []*engrave.Layout, opt Options) error {
	secs := make([]Section, 0, len(layouts))
	for _, lay := range layouts {
		secs = append(secs, Section{Layout: lay, Opt: opt})
	}
	return RenderSections(w, secs)
}

// RenderSections writes several pieces into one PDF, each on its own paper.
//
// This is how a score and its parts are printed together: the conductor's
// copy, then one part per player, each set the way that piece wants to be.
func RenderSections(w io.Writer, secs []Section) error {
	if len(secs) == 0 {
		return nil
	}
	doc := pdf.New()
	doc.Title = secs[0].Opt.Title
	doc.Author = secs[0].Opt.Author
	doc.Subject = secs[0].Opt.Subject
	doc.Creator = brand.Name

	music, err := doc.Embed("Bravura", smufl.FontData())
	if err != nil {
		return err
	}
	for i, sec := range secs {
		r := &renderer{doc: doc, lay: sec.Layout, opt: sec.Opt,
			s: pdf.Point(sec.Opt.StaffSpace), first: i == 0}
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
	// first says this renderer draws the first page of the document, which
	// is the one that carries the maker's mark. A book of tunes is many
	// layouts and one document, so it is the document's first page that
	// counts, not each layout's.
	first bool
}

// run cuts the layout into pages and draws each one.
func (r *renderer) run() {
	// The mark goes on the first page of the document, and on the first
	// page of anything that will be shared on its own.
	first := r.first || r.opt.Label != ""
	bands := r.pages()
	for n, band := range bands {
		r.offset = band.top
		r.page = r.doc.NewPage(pdf.Point(r.opt.PageWidth), pdf.Point(r.opt.PageHeight))
		r.page.SetGray(0)
		if first {
			r.stamp()
			r.label()
			first = false
		}

		// Nothing from the next page's systems may bleed into this one. The
		// clip is the band, not the page: a stem or a slur is allowed to
		// reach a little past what it belongs to, and without this the
		// system that starts the next page is drawn across the bottom of
		// this one.
		r.page.Save()
		// Tight at the bottom. The band already ends where the next system's
		// own territory begins, so any slack there is room for that system's
		// rehearsal marks to print along the foot of this page.
		top := r.y(band.top) + r.s*0.5
		bottom := r.y(band.bottom) - r.s*0.25
		r.page.Clip(0, bottom, pdf.Point(r.opt.PageWidth), top-bottom)
		r.draw(band.top, band.bottom)
		r.page.Restore()
		r.pageNumber(n+1, len(bands))
		r.page.Close()
	}
}

// How the mascot is stamped on a page.
const (
	// stampSize is the box he is drawn in, in points: a square, with the
	// drawing fitted inside it and its proportions kept.
	//
	// Eleven millimetres. Big enough to be recognised across a desk, small
	// enough to stay a maker's mark in the margin rather than something
	// printed on somebody else's music — and small enough that the music
	// keeps nearly all the page it had.
	stampSize = 32
	// stampGap is the air around him: between the stamp and the paper's
	// edge above, and between the stamp and the music below. Twice this and
	// the stamp itself have to fit the top margin, or he is drawn smaller.
	stampGap = 6
)

// numberSize is the type a page number is set in, in points.
const numberSize = 10

// pageNumber writes which sheet this is, along the foot of the page.
//
// Centred, because that is where a reader's eye goes for it and because a
// part is as often photocopied two-up as printed: a number in a corner ends
// up in the gutter or trimmed off, and one in the middle does not.
func (r *renderer) pageNumber(n, total int) {
	if !r.opt.PageNumbers || total < 2 {
		return
	}
	s := strconv.Itoa(n) + "/" + strconv.Itoa(total)
	w := float64(r.page.TextWidth(r.text, pdf.Point(numberSize), s))
	// Halfway down the bottom margin, under the music and clear of it.
	baseline := r.opt.MarginBottom/2 - numberSize*0.36
	r.page.Text(r.text, pdf.Point(numberSize),
		pdf.Point((r.opt.PageWidth-w)/2), pdf.Point(baseline), s)
}

// labelSize is the type an instrument's name is set in, in points.
//
// Large enough to read across a music stand without looking for it, small
// enough to stay a label in the margin: it is not a title, and the title is
// already on the page under it.
const labelSize = 13

// label writes whose part this is, in the top margin to the right of the
// mark.
func (r *renderer) label() {
	if r.opt.Label == "" {
		return
	}
	left, top, box := r.stampRect()
	// To the right of the mark, on its own middle line, so the two read as
	// one line rather than as a drawing with something written over it.
	x := left
	middle := r.opt.PageHeight - r.opt.MarginTop/2
	if box > 0 {
		x += box + stampGap*2
		middle = top - box/2
	}
	// Text is drawn from its baseline, which sits about a third of the type
	// size below the middle of a capital.
	r.page.Text(r.textBold, pdf.Point(labelSize),
		pdf.Point(x), pdf.Point(middle-labelSize*0.36), r.opt.Label)
}

// stampRect is where the mark is drawn and how big: its left edge, its top
// edge, and the side of the square it is fitted into. A box of zero means
// the margin has no room for it.
//
// Shared with label, which has to stand to the right of it and on its line.
func (r *renderer) stampRect() (left, top, box float64) {
	box = float64(stampSize)
	if room := r.opt.MarginTop - 2*stampGap; box > room {
		box = room
	}
	if box < 6 {
		return r.opt.MarginLeft, 0, 0
	}
	return r.opt.MarginLeft, r.opt.PageHeight - (r.opt.MarginTop-box)/2 - box, box
}

// stamp prints Gilbert in the top left of the first page.
//
// The first page only. It says who made the thing, which is said once: on
// every sheet it stops being a maker's mark and becomes something printed
// over somebody else's music, and a score is mostly not its first page.
//
// He is drawn, not placed: the outlines go onto the paper at whatever
// resolution prints them, where a bitmap would be fixed at its own and go
// soft the moment it is printed larger than it was drawn.
//
// He goes in the margin, and never into the music: on paper with a margin
// too narrow to hold him he is drawn smaller rather than over the top
// staff.
func (r *renderer) stamp() {
	d, err := brand.GilbertDrawing()
	if err != nil || d == nil || len(d.Shapes) == 0 {
		return // a logo that will not read is not worth failing a score for
	}
	side := math.Max(d.Width, d.Height)
	if side <= 0 {
		return
	}
	left, top, box := r.stampRect()
	if box == 0 {
		return // a margin with no room in it gets no mark
	}
	s := box / side

	// SVG measures down the page and PDF measures up it, so y is turned
	// over here — once, in the one place that knows both.
	x := func(v float64) pdf.Point { return pdf.Point(left + v*s) }
	y := func(v float64) pdf.Point { return pdf.Point(top - v*s) }

	r.page.Save()
	for _, sh := range d.Shapes {
		if sh.Fill != nil {
			r.page.SetFillRGB(sh.Fill.R, sh.Fill.G, sh.Fill.B)
		}
		if sh.Stroke != nil {
			r.page.SetStrokeRGB(sh.Stroke.R, sh.Stroke.G, sh.Stroke.B)
			r.page.SetLineWidth(sh.StrokeWidth * s)
		}
		for _, op := range sh.Ops {
			switch op.Kind {
			case 'm':
				r.page.Begin(x(op.P[0]), y(op.P[1]))
			case 'l':
				r.page.LineTo(x(op.P[0]), y(op.P[1]))
			case 'c':
				r.page.CurveTo(x(op.P[0]), y(op.P[1]), x(op.P[2]), y(op.P[3]),
					x(op.P[4]), y(op.P[5]))
			case 'z':
				r.page.ClosePath()
			}
		}
		r.page.Paint(sh.Fill != nil, sh.Stroke != nil)
	}
	r.page.Restore()
	// The music is drawn in black, and the drawing was not.
	r.page.SetGray(0)
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

	// Where the page may be cut, in order down the drawing.
	//
	// Between systems, always. And inside a system that will not fit a page
	// at any readable size, between its staves: seventeen staves on A4 on
	// its side is not a thing that can be set, and the choice is between
	// printing part of the band and printing a band with its bottom sliced
	// off by the paper's edge.
	breaks := []float64{}
	for i, sys := range systems {
		if i > 0 {
			breaks = append(breaks, sys.Top)
		}
		if sys.Bottom-sys.Top > usable {
			breaks = append(breaks, r.staffCuts(sys, usable)...)
		}
	}
	breaks = append(breaks, r.lay.Height)
	sort.Float64s(breaks)

	var out []band
	// The first page starts at the very top of the drawing, because the
	// title block sits above the first system and belongs with it.
	top, prev := 0.0, 0.0
	for _, at := range breaks {
		if at-top > usable {
			if prev <= top {
				// One piece too tall for a page even on its own, and
				// nowhere to cut it. Print it and let it say so.
				prev = at
			}
			// The page ends where the next piece's own territory begins,
			// not where the last one's staff lines stopped: the chord
			// symbols of the system overleaf sit above its staff and would
			// otherwise print along the bottom of this page.
			out = append(out, band{top, prev - 0.5})
			top = prev
		}
		if at-top <= usable {
			prev = at
		}
	}
	if top < r.lay.Height {
		out = append(out, band{top, r.lay.Height})
	}
	if len(out) == 0 {
		out = append(out, band{0, r.lay.Height})
	}
	return out
}

// staffCuts picks where to divide a system that will not fit one page,
// splitting it between staves and as evenly as it can.
//
// Evenly, because the pages are being printed either way. Ten staves that
// have to go on two pages read better as five and five than as eight and
// two: the reader is turning the page in the middle of a bar whatever
// happens, and two half-empty pages are easier to follow than one crammed
// one and one nearly blank.
//
// Between staves and never through one. A cut through a staff is what the
// paper's edge does on its own, and it is the thing being fixed here.
func (r *renderer) staffCuts(sys engrave.SystemPos, usable float64) []float64 {
	type staff struct{ top, bottom float64 }
	var staves []staff
	seen := map[float64]bool{}
	for _, h := range r.lay.Hits {
		if h.Measure < sys.First || h.Measure > sys.Last || seen[h.StaffTop] {
			continue
		}
		seen[h.StaffTop] = true
		height := h.StaffHeight
		if height <= 0 {
			height = 4
		}
		staves = append(staves, staff{h.StaffTop, h.StaffTop + height})
	}
	if len(staves) < 2 {
		return nil // one staff cannot be divided between two pages
	}
	sort.Slice(staves, func(i, j int) bool { return staves[i].top < staves[j].top })

	// The gaps, which are the only places a cut may fall: midway between
	// what one staff reaches down to and what the next starts at.
	gaps := make([]float64, 0, len(staves)-1)
	for i := 0; i+1 < len(staves); i++ {
		gaps = append(gaps, (staves[i].bottom+staves[i+1].top)/2)
	}

	height := sys.Bottom - sys.Top
	// As many pages as it takes, and then more only if the staves do not
	// divide evenly enough to keep every piece on its page.
	for n := int(math.Ceil(height / usable)); n <= len(gaps)+1; n++ {
		if n < 2 {
			return nil
		}
		cuts := evenCuts(gaps, sys.Top, height, n)
		if fits(cuts, sys.Top, sys.Bottom, usable) {
			return cuts
		}
	}
	// No division of this system fits. Cut at every gap, which is the most
	// that can be done, and the clipping says the rest.
	return gaps
}

// evenCuts chooses n-1 of the available gaps, as near as they come to
// dividing the system into n equal pieces.
func evenCuts(gaps []float64, top, height float64, n int) []float64 {
	used := make(map[int]bool, n)
	cuts := make([]float64, 0, n-1)
	for k := 1; k < n; k++ {
		want := top + height*float64(k)/float64(n)
		best := -1
		for i, at := range gaps {
			if used[i] {
				continue
			}
			if best < 0 || math.Abs(at-want) < math.Abs(gaps[best]-want) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		used[best] = true
		cuts = append(cuts, gaps[best])
	}
	sort.Float64s(cuts)
	return cuts
}

// fits says every piece a set of cuts makes is short enough for a page.
func fits(cuts []float64, top, bottom, usable float64) bool {
	at := top
	for _, c := range cuts {
		if c-at > usable {
			return false
		}
		at = c
	}
	return bottom-at <= usable
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
	r.drawQuads(top, bottom)
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

// drawQuads fills the beams.
//
// A beam is a parallelogram with vertical ends, so that it finishes flush
// with the stems it joins. It used to be a thick stroked line, whose caps are
// square to the line rather than vertical — which left a point sticking out
// past the outer stem of every beam that was not level.
func (r *renderer) drawQuads(top, bottom float64) {
	for _, q := range r.lay.Quads {
		shown := false
		for _, y := range q.Y {
			shown = shown || visible(y, top, bottom)
		}
		if !shown {
			continue
		}
		pts := make([]pdf.Point, 0, 8)
		for i := range q.X {
			pts = append(pts, r.x(q.X[i]), r.y(q.Y[i]))
		}
		r.page.Polygon(pts...)
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
		// A line break in a text block — a composer credit over three lines —
		// is drawn as the lines it is.
		for i, line := range strings.Split(t.S, "\n") {
			// Set in runs, so that a flat sign in a staff name is printed as
			// one rather than as the question mark WinAnsi has instead.
			rs := r.split(line, f, size)
			lx := r.x(t.X)
			// PDF has no text-anchor: a string is drawn from where it
			// starts, so centring and right-aligning are done by measuring
			// it first.
			switch t.Anchor {
			case "middle":
				lx -= r.width(rs) / 2
			case "end":
				lx -= r.width(rs)
			}
			r.drawRuns(rs, lx, r.y(t.Y)-pdf.Point(i)*size*1.2)
		}
	}
}
