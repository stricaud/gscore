/*
 * pdf.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package pdf writes PDF files.
//
// It is small on purpose. A score is lines, curves, text and the odd picture,
// and PDF draws all four in about a dozen operators; everything else in the
// specification is for things gscore does not do. What is here is a document
// of numbered objects, a page tree, flate-compressed content streams, the
// fonts those streams name, and the cross-reference table that ties them
// together.
//
// The one part that is not trivial is embedding the music font. A PDF cannot
// be handed a .otf and left to work it out: it needs the glyph widths, the
// font's own name and its bounding box written out as PDF objects beside the
// file. internal/sfnt reads those out of the font; the rest is bookkeeping.
//
// Why write this rather than print from a browser. Printing works and will go
// on working, but it needs a browser, which the command line does not have and
// a build server does not either. A score should be turnable into a PDF by a
// script, in a Makefile, on a machine with no screen.
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
)

// Point is the PDF unit: one seventy-second of an inch.
type Point float64

// Common paper, in points.
const (
	A4Width      Point = 595.28
	A4Height     Point = 841.89
	LetterWidth  Point = 612
	LetterHeight Point = 792
)

// Doc is a document being built.
type Doc struct {
	objects []object
	pages   []int // object numbers of the pages, in order

	fonts    map[string]*Font
	fontKeys []string

	images    map[string]*Image
	imageKeys []string

	// pending are embedded fonts, finished at the end once the document
	// knows which glyphs it actually draws.
	pending []*Font

	Title, Author, Subject, Creator string
}

type object struct {
	data []byte
	// stream is written after the dictionary, when there is one.
	stream []byte
}

// New starts an empty document.
func New() *Doc {
	return &Doc{
		fonts:  map[string]*Font{},
		images: map[string]*Image{},
		// Object 0 is the free-list head and is never written as an object,
		// but it takes a slot so that object numbers and indices agree.
		objects: []object{{}},
	}
}

// add appends an object and returns its number.
func (d *Doc) add(format string, args ...any) int {
	d.objects = append(d.objects, object{data: []byte(fmt.Sprintf(format, args...))})
	return len(d.objects) - 1
}

// addStream appends an object with a stream attached.
func (d *Doc) addStream(dict string, data []byte) int {
	// Flate is the only filter worth having here: a page of music is mostly
	// repeated operators and compresses to a fraction of its size.
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	compressed := buf.Bytes()

	full := fmt.Sprintf("<< %s /Filter /FlateDecode /Length %d >>", dict, len(compressed))
	d.objects = append(d.objects, object{data: []byte(full), stream: compressed})
	return len(d.objects) - 1
}

/*
 * Pages
 */

// Page is one page being drawn on.
type Page struct {
	doc           *Doc
	Width, Height Point
	buf           bytes.Buffer
	// used names the resources this page actually refers to, so a page does
	// not declare a font it never uses.
	usedFonts  map[string]bool
	usedImages map[string]bool
	// state
	gray     float64
	lineWide float64
}

// NewPage starts a page. PDF's origin is the bottom left corner and y grows
// upwards, which is the opposite of how a score is laid out — the caller deals
// with that, because only the caller knows what it is drawing.
func (d *Doc) NewPage(w, h Point) *Page {
	return &Page{
		doc: d, Width: w, Height: h,
		usedFonts: map[string]bool{}, usedImages: map[string]bool{},
		gray: 0, lineWide: 1,
	}
}

// Close finishes a page and adds it to the document.
func (p *Page) Close() {
	content := p.doc.addStream("", p.buf.Bytes())

	var res strings.Builder
	res.WriteString("<< /ProcSet [/PDF /Text /ImageC /ImageB]")
	if len(p.usedFonts) > 0 {
		res.WriteString(" /Font <<")
		for _, key := range p.doc.fontKeys {
			if p.usedFonts[key] {
				fmt.Fprintf(&res, " /%s %d 0 R", key, p.doc.fonts[key].obj)
			}
		}
		res.WriteString(" >>")
	}
	if len(p.usedImages) > 0 {
		res.WriteString(" /XObject <<")
		for _, key := range p.doc.imageKeys {
			if p.usedImages[key] {
				fmt.Fprintf(&res, " /%s %d 0 R", key, p.doc.images[key].obj)
			}
		}
		res.WriteString(" >>")
	}
	res.WriteString(" >>")

	// The parent is patched once the page tree exists, which it cannot until
	// every page is known. A placeholder of the right shape keeps the object
	// a fixed size so it can be rewritten in place.
	num := p.doc.add("<< /Type /Page /Parent %s /MediaBox [0 0 %.2f %.2f] "+
		"/Resources %s /Contents %d 0 R >>",
		pagesPlaceholder, float64(p.Width), float64(p.Height), res.String(), content)
	p.doc.pages = append(p.doc.pages, num)
}

// pagesPlaceholder is replaced with the page tree's object reference once it
// is known. It is deliberately long enough to hold any number that could
// plausibly appear, so the replacement never changes an object's length in a
// way that matters — objects are measured after the substitution.
const pagesPlaceholder = "@@PAGES@@"

/*
 * Drawing
 */

// SetGray sets the drawing colour, 0 black to 1 white.
func (p *Page) SetGray(v float64) {
	if v == p.gray {
		return
	}
	p.gray = v
	fmt.Fprintf(&p.buf, "%.3f g %.3f G\n", v, v)
}

// SetRGB sets the drawing colour.
func (p *Page) SetRGB(r, g, b float64) {
	p.gray = -1
	fmt.Fprintf(&p.buf, "%.3f %.3f %.3f rg %.3f %.3f %.3f RG\n", r, g, b, r, g, b)
}

// SetFillRGB and SetStrokeRGB set the two colours separately.
//
// SetRGB sets both to the same thing, which is all music needs: a notehead
// is one colour. A drawing is not — a white shape with a black outline is
// two — so these exist for it.
func (p *Page) SetFillRGB(r, g, b float64) {
	p.gray = -1
	fmt.Fprintf(&p.buf, "%.3f %.3f %.3f rg\n", r, g, b)
}

func (p *Page) SetStrokeRGB(r, g, b float64) {
	p.gray = -1
	fmt.Fprintf(&p.buf, "%.3f %.3f %.3f RG\n", r, g, b)
}

// SetLineWidth sets the stroke width in points.
func (p *Page) SetLineWidth(w float64) {
	if w == p.lineWide {
		return
	}
	p.lineWide = w
	fmt.Fprintf(&p.buf, "%.3f w\n", w)
}

// Line strokes a straight line.
func (p *Page) Line(x1, y1, x2, y2 Point) {
	fmt.Fprintf(&p.buf, "%.2f %.2f m %.2f %.2f l S\n", x1, y1, x2, y2)
}

// Dash turns the dashed pattern on, or off when on is 0.
func (p *Page) Dash(on, off Point) {
	if on <= 0 {
		p.buf.WriteString("[] 0 d\n")
		return
	}
	fmt.Fprintf(&p.buf, "[%.2f %.2f] 0 d\n", on, off)
}

// Rect fills a rectangle, which is how a thick horizontal — a beam, a staff
// line at a heavy weight — is best drawn: a stroked line is centred on its
// path and a filled rectangle is where you put it.
func (p *Page) Rect(x, y, w, h Point) {
	fmt.Fprintf(&p.buf, "%.2f %.2f %.2f %.2f re f\n", x, y, w, h)
}

// Quad draws a quadratic Bézier, which PDF has no operator for: it only knows
// cubics, so the control point is converted. A slur is a quadratic in the
// layout and this is where it becomes drawable.
func (p *Page) Quad(x1, y1, cx, cy, x2, y2 Point, thickness Point) {
	c1x := x1 + 2.0/3.0*(cx-x1)
	c1y := y1 + 2.0/3.0*(cy-y1)
	c2x := x2 + 2.0/3.0*(cx-x2)
	c2y := y2 + 2.0/3.0*(cy-y2)
	p.SetLineWidth(float64(thickness))
	fmt.Fprintf(&p.buf, "%.2f %.2f m %.2f %.2f %.2f %.2f %.2f %.2f c S\n",
		x1, y1, c1x, c1y, c2x, c2y, x2, y2)
}

// Circle draws a round mark, filled or as a ring.
//
// PDF has no circle operator, so it is four Bézier curves. The magic number
// is the one that makes a cubic curve pass closest to a quarter circle, which
// everything that draws circles in PostScript has used since 1988.
func (p *Page) Circle(cx, cy, radius Point, filled bool, thickness Point) {
	const k = 0.5522847498
	o := radius * k
	// Before the path, not after it. Between a path's first operator and the
	// one that paints it, nothing may change the graphics state — a width set
	// in the middle leaves the path unfinished and the page fills with the
	// wreckage, which is exactly what it did.
	if !filled {
		p.SetLineWidth(float64(thickness))
	}
	fmt.Fprintf(&p.buf, "%.2f %.2f m", cx+radius, cy)
	fmt.Fprintf(&p.buf, " %.2f %.2f %.2f %.2f %.2f %.2f c",
		cx+radius, cy+o, cx+o, cy+radius, cx, cy+radius)
	fmt.Fprintf(&p.buf, " %.2f %.2f %.2f %.2f %.2f %.2f c",
		cx-o, cy+radius, cx-radius, cy+o, cx-radius, cy)
	fmt.Fprintf(&p.buf, " %.2f %.2f %.2f %.2f %.2f %.2f c",
		cx-radius, cy-o, cx-o, cy-radius, cx, cy-radius)
	fmt.Fprintf(&p.buf, " %.2f %.2f %.2f %.2f %.2f %.2f c",
		cx+o, cy-radius, cx+radius, cy-o, cx+radius, cy)
	if filled {
		p.buf.WriteString(" h f\n")
		return
	}
	p.buf.WriteString(" h S\n")
}

// Polygon fills a closed shape, for the wedges and hooks that are neither
// lines nor glyphs.
func (p *Page) Polygon(pts ...Point) {
	if len(pts) < 4 {
		return
	}
	fmt.Fprintf(&p.buf, "%.2f %.2f m", pts[0], pts[1])
	for i := 2; i+1 < len(pts); i += 2 {
		fmt.Fprintf(&p.buf, " %.2f %.2f l", pts[i], pts[i+1])
	}
	p.buf.WriteString(" h f\n")
}

// An outline built up and then painted.
//
// Rect, Circle and Polygon each draw one shape and paint it in the same
// breath, which is all the engraver ever needs: music is made of lines,
// curves and glyphs. A drawing is not — it is a path of many pieces, filled
// and stroked as one — so this is the general form, and it is the only thing
// in here with a state of its own.
//
// Between the first operator of a path and the one that paints it, nothing
// may change the graphics state: a width or a colour set in the middle
// leaves the path unfinished and the rest of the page is drawn into the
// wreckage. So the colour and the width go on before Begin.

// Begin starts an outline at a point.
func (p *Page) Begin(x, y Point) {
	fmt.Fprintf(&p.buf, "%.3f %.3f m", x, y)
}

// LineTo adds a straight piece.
func (p *Page) LineTo(x, y Point) {
	fmt.Fprintf(&p.buf, " %.3f %.3f l", x, y)
}

// CurveTo adds a cubic Bézier, which is the only curve PDF knows.
func (p *Page) CurveTo(c1x, c1y, c2x, c2y, x, y Point) {
	fmt.Fprintf(&p.buf, " %.3f %.3f %.3f %.3f %.3f %.3f c",
		c1x, c1y, c2x, c2y, x, y)
}

// ClosePath joins the end of the outline back to where it started.
func (p *Page) ClosePath() { p.buf.WriteString(" h") }

// Paint finishes the outline: filled, stroked, or both.
//
// Both is one operator rather than two paths, because a shape filled and
// then stroked separately is drawn twice and the seam between the two shows
// at the edges.
func (p *Page) Paint(fill, stroke bool) {
	switch {
	case fill && stroke:
		p.buf.WriteString(" B\n")
	case fill:
		p.buf.WriteString(" f\n")
	case stroke:
		p.buf.WriteString(" S\n")
	default:
		p.buf.WriteString(" n\n") // nothing to paint, but the path must end
	}
}

// Text draws a string in a font at a size, with the given left baseline.
func (p *Page) Text(f *Font, size Point, x, y Point, s string) {
	if s == "" || f == nil {
		return
	}
	p.usedFonts[f.key] = true
	fmt.Fprintf(&p.buf, "BT /%s %.2f Tf %.2f %.2f Td %s Tj ET\n",
		f.key, size, x, y, f.encode(s))
}

// TextWidth is how wide a string is at a size, which the caller needs to
// centre or right-align it.
func (p *Page) TextWidth(f *Font, size Point, s string) Point {
	if f == nil {
		return 0
	}
	return Point(float64(size) * f.width(s) / 1000)
}

// Image draws a picture in the rectangle given.
func (p *Page) Image(img *Image, x, y, w, h Point) {
	if img == nil {
		return
	}
	p.usedImages[img.key] = true
	// An image is drawn into the unit square, so the matrix is the rectangle.
	fmt.Fprintf(&p.buf, "q %.2f 0 0 %.2f %.2f %.2f cm /%s Do Q\n",
		w, h, x, y, img.key)
}

// Save and Restore bracket a change to the drawing state.
func (p *Page) Save()    { p.buf.WriteString("q\n") }
func (p *Page) Restore() { p.buf.WriteString("Q\n"); p.gray = -1; p.lineWide = -1 }

// Clip limits what is drawn to a rectangle until the next Restore.
func (p *Page) Clip(x, y, w, h Point) {
	fmt.Fprintf(&p.buf, "%.2f %.2f %.2f %.2f re W n\n", x, y, w, h)
}

/*
 * Writing it out
 */

// Write finishes the document.
func (d *Doc) Write(w io.Writer) error {
	if len(d.pages) == 0 {
		// A PDF with no pages is not a PDF, so an empty one gets a blank.
		d.NewPage(A4Width, A4Height).Close()
	}

	// The embedded fonts can only be written once every page is drawn.
	for _, f := range d.pending {
		d.finish(f)
	}
	d.pending = nil

	kids := make([]string, len(d.pages))
	for i, n := range d.pages {
		kids[i] = fmt.Sprintf("%d 0 R", n)
	}
	pagesObj := d.add("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(d.pages))

	// Every page points at the page tree, whose number was not known while
	// the pages were being made.
	ref := []byte(fmt.Sprintf("%d 0 R", pagesObj))
	for i := range d.objects {
		if bytes.Contains(d.objects[i].data, []byte(pagesPlaceholder)) {
			d.objects[i].data = bytes.ReplaceAll(
				d.objects[i].data, []byte(pagesPlaceholder), ref)
		}
	}

	info := 0
	if d.Title != "" || d.Author != "" || d.Creator != "" {
		var b strings.Builder
		b.WriteString("<<")
		if d.Title != "" {
			fmt.Fprintf(&b, " /Title %s", pdfString(d.Title))
		}
		if d.Author != "" {
			fmt.Fprintf(&b, " /Author %s", pdfString(d.Author))
		}
		if d.Subject != "" {
			fmt.Fprintf(&b, " /Subject %s", pdfString(d.Subject))
		}
		if d.Creator != "" {
			fmt.Fprintf(&b, " /Creator %s /Producer %s",
				pdfString(d.Creator), pdfString(d.Creator))
		}
		b.WriteString(" >>")
		info = d.add("%s", b.String())
	}
	root := d.add("<< /Type /Catalog /Pages %d 0 R >>", pagesObj)

	var out bytes.Buffer
	// 1.6 is what embedding an OpenType font with CFF outlines needs.
	out.WriteString("%PDF-1.6\n")
	// A comment of high bytes, which tells anything transferring the file
	// that it is binary and must not have its line endings meddled with.
	out.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'})

	offsets := make([]int, len(d.objects))
	for i := 1; i < len(d.objects); i++ {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i)
		out.Write(d.objects[i].data)
		if d.objects[i].stream != nil {
			out.WriteString("\nstream\n")
			out.Write(d.objects[i].stream)
			out.WriteString("\nendstream")
		}
		out.WriteString("\nendobj\n")
	}

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(d.objects))
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i < len(d.objects); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R", len(d.objects), root)
	if info != 0 {
		fmt.Fprintf(&out, " /Info %d 0 R", info)
	}
	fmt.Fprintf(&out, " >>\nstartxref\n%d\n%%%%EOF\n", xref)

	_, err := w.Write(out.Bytes())
	return err
}

// pdfString escapes a string for a PDF dictionary.
func pdfString(s string) string {
	var b strings.Builder
	b.WriteByte('(')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(')')
	return b.String()
}
