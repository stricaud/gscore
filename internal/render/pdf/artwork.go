/*
 * artwork.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"

	"github.com/stricaud/gscore/internal/brand"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/pdf"
)

/*
 * The pictures printed around the music: a cover before it and a stamp
 * after it.
 *
 * Neither is music and neither is drawn while the music is being written.
 * A cover is the sheet a publisher wraps the score in; the block at the
 * foot of the last page is an advertisement — "POPULAR COMMUNITY SONGS,
 * PRICE 25¢ A COPY" across the bottom of the Ellington. They belong to the
 * printed document, which is why they live here, in the only part of the
 * program that knows what a sheet of paper is, and why the editor never
 * shows them: a cover standing between you and bar one is in the way.
 *
 * Once per document rather than once per piece. A score and its parts are
 * one PDF of many sections, and a cover on each of them would be a cover
 * between every part.
 */

// coverPage puts the cover in as the first page of the document.
//
// The whole sheet, with the picture's own proportions kept and the spare
// edge split evenly: a cover is a picture of a sheet of paper and stretching
// it to the paper would be the one way to make it obviously wrong. It is
// printed to the edges and not inside the margins, because the margins are
// the music's and a cover is not music.
func coverPage(doc *pdf.Doc, o Options) {
	if o.Cover == nil || len(o.Cover.Data) == 0 {
		return
	}
	page := doc.NewPage(pdf.Point(o.PageWidth), pdf.Point(o.PageHeight))
	defer page.Close()
	drawArtwork(doc, page, o.Cover, "cover",
		0, 0, o.PageWidth, o.PageHeight, "center")
}

// stampGapFoot is the air between the stamp and the edge of the paper.
const stampGapFoot = 8

// footStamp prints the stamp along the bottom of the page.
//
// In the bottom margin, under the music rather than among it. The margin is
// the one band of a page that is known to be empty, so a stamp there can
// never be printed over a system however long the piece runs — and a block
// at the foot of the last page is where a printer has always put one.
//
// It is fitted into the width the music has and the height the margin has,
// whichever runs out first, so a wide advertisement uses the width and a
// tall device uses the margin. A page with no bottom margin to speak of
// gets no stamp rather than a stamp over its last bar.
func (r *renderer) footStamp() {
	art := r.opt.Stamp
	if art == nil || len(art.Data) == 0 {
		return
	}
	boxH := r.opt.MarginBottom - 2*stampGapFoot
	boxW := r.opt.PageWidth - r.opt.MarginLeft - r.opt.MarginRight
	if boxH <= 0 || boxW <= 0 {
		return
	}
	drawArtwork(r.doc, r.page, art, "stamp",
		r.opt.MarginLeft, stampGapFoot, boxW, boxH, art.Align)
}

// drawArtwork fits a picture into a box and draws it, keeping its own
// proportions and placing it by the alignment asked for.
//
// x and y are the box's bottom left corner in PDF coordinates, which count
// up the page.
func drawArtwork(doc *pdf.Doc, page *pdf.Page, art *model.Artwork, name string,
	x, y, boxW, boxH float64, align string) {
	w, h, draw, ok := measureArtwork(doc, art, name)
	if !ok || w <= 0 || h <= 0 {
		return // a picture that will not read is not worth failing a score for
	}
	s := math.Min(boxW/w, boxH/h)
	if s <= 0 {
		return
	}
	w, h = w*s, h*s
	switch align {
	case "left":
	case "right":
		x += boxW - w
	default: // centred, which is what an empty alignment means
		x += (boxW - w) / 2
	}
	// Vertically centred in the box whatever the alignment says: left,
	// centre and right are the only question anybody has about a stamp,
	// and a second one about the height would be a setting nobody wants.
	y += (boxH - h) / 2
	draw(page, x, y, w, h)
}

// measureArtwork works out how large a picture is in its own units and hands
// back the means of drawing it.
//
// Two kinds, and they are drawn differently. A photograph or a scan is a
// grid of pixels and goes in as an image. A drawing is a set of outlines and
// goes in as outlines, so that it prints at the printer's resolution rather
// than at whatever grid it was drawn on — which for a publisher's device,
// all fine rules and small lettering, is the difference between a mark and a
// smudge.
func measureArtwork(doc *pdf.Doc, art *model.Artwork, name string) (
	w, h float64, draw func(*pdf.Page, float64, float64, float64, float64), ok bool) {

	if isVector(art) {
		d, err := brand.ParseSVG(art.Data)
		if err != nil || d == nil || len(d.Shapes) == 0 ||
			d.Width <= 0 || d.Height <= 0 {
			return 0, 0, nil, false
		}
		return d.Width, d.Height, func(p *pdf.Page, x, y, w, h float64) {
			drawDrawing(p, d, x, y, w, h)
		}, true
	}

	img, _, err := image.Decode(bytes.NewReader(art.Data))
	if err != nil || img == nil {
		return 0, 0, nil, false
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return 0, 0, nil, false
	}
	ref := doc.Image(name, img)
	if ref == nil {
		return 0, 0, nil, false
	}
	return float64(b.Dx()), float64(b.Dy()),
		func(p *pdf.Page, x, y, w, h float64) {
			p.Image(ref, pdf.Point(x), pdf.Point(y), pdf.Point(w), pdf.Point(h))
		}, true
}

// isVector says the file is a drawing rather than a grid of pixels.
//
// By what is in it and not only by what it was called: a file uploaded
// through a browser arrives with whatever name the browser gave it, and a
// drawing saved as "cover.png" is still a drawing.
func isVector(art *model.Artwork) bool {
	if art.Format == "svg" {
		return true
	}
	head := art.Data
	if len(head) > 512 {
		head = head[:512]
	}
	return bytes.Contains(head, []byte("<svg")) ||
		bytes.Contains(head, []byte("<?xml"))
}

// drawDrawing paints a parsed drawing into a box, the way the maker's mark
// is painted: SVG measures down the page and PDF measures up it, so y is
// turned over here, once, in the one place that knows both.
func drawDrawing(page *pdf.Page, d *brand.Drawing, bx, by, bw, bh float64) {
	if d.Width <= 0 || d.Height <= 0 {
		return
	}
	s := math.Min(bw/d.Width, bh/d.Height)
	top := by + bh - (bh-d.Height*s)/2
	left := bx + (bw-d.Width*s)/2
	x := func(v float64) pdf.Point { return pdf.Point(left + v*s) }
	y := func(v float64) pdf.Point { return pdf.Point(top - v*s) }

	page.Save()
	for _, sh := range d.Shapes {
		if sh.Fill != nil {
			page.SetFillRGB(sh.Fill.R, sh.Fill.G, sh.Fill.B)
		}
		if sh.Stroke != nil {
			page.SetStrokeRGB(sh.Stroke.R, sh.Stroke.G, sh.Stroke.B)
			page.SetLineWidth(sh.StrokeWidth * s)
		}
		for _, op := range sh.Ops {
			switch op.Kind {
			case 'm':
				page.Begin(x(op.P[0]), y(op.P[1]))
			case 'l':
				page.LineTo(x(op.P[0]), y(op.P[1]))
			case 'c':
				page.CurveTo(x(op.P[0]), y(op.P[1]), x(op.P[2]), y(op.P[3]),
					x(op.P[4]), y(op.P[5]))
			case 'z':
				page.ClosePath()
			}
		}
		page.Paint(sh.Fill != nil, sh.Stroke != nil)
	}
	page.Restore()
}
