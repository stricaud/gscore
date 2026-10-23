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
}

// DefaultOptions returns a sensible rendering setup.
func DefaultOptions() Options {
	return Options{StaffSpace: 8, EmbedFont: true}
}

// Render writes the layout as a standalone SVG document.
func Render(w io.Writer, lay *engrave.Layout, opt Options) error {
	var b strings.Builder
	s := opt.StaffSpace

	width, height := lay.Width*s, lay.Height*s
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.1f" height="%.1f" `+
		`viewBox="0 0 %.1f %.1f" class="gscore-score">`+"\n", width, height, width, height)

	b.WriteString("<style>\n")
	if opt.EmbedFont {
		// The font travels with the file so the score renders the same
		// wherever it is opened, with nothing to install.
		fmt.Fprintf(&b, `@font-face{font-family:"%s";src:url("data:font/otf;base64,%s") format("opentype");}`+"\n",
			smufl.FontFamily, base64.StdEncoding.EncodeToString(smufl.FontData()))
	}
	fmt.Fprintf(&b, `.gscore-score{background:#fff;}`+"\n")
	fmt.Fprintf(&b, `.g{font-family:"%s";font-size:%.3fpx;fill:#000;}`+"\n", smufl.FontFamily, 4*s)
	fmt.Fprintf(&b, `.t{font-family:Georgia,"Times New Roman",serif;fill:#000;}`+"\n")
	fmt.Fprintf(&b, `line{stroke:#000;stroke-linecap:butt;}`+"\n")
	fmt.Fprintf(&b, `path.curve{fill:none;stroke:#000;}`+"\n")
	b.WriteString("</style>\n")

	for _, ln := range lay.Lines {
		dash := ""
		if ln.Dash > 0 {
			dash = fmt.Sprintf(` stroke-dasharray="%.2f %.2f"`, ln.Dash*s, ln.Dash*s*0.8)
		}
		fmt.Fprintf(&b, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke-width="%.2f"%s/>`+"\n",
			ln.X1*s, ln.Y1*s, ln.X2*s, ln.Y2*s, ln.Thickness*s, dash)
	}

	for _, c := range lay.Curves {
		fmt.Fprintf(&b, `<path class="curve" d="M %.2f %.2f Q %.2f %.2f %.2f %.2f" stroke-width="%.2f"/>`+"\n",
			c.X1*s, c.Y1*s, c.CX*s, c.CY*s, c.X2*s, c.Y2*s, c.Thickness*s)
	}

	for _, g := range lay.Glyphs {
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
		fmt.Fprintf(&b, `<text class="g" x="%.2f" y="%.2f"%s>%s</text>`+"\n",
			g.X*s, g.Y*s, attr, html.EscapeString(string(r)))
	}

	for _, t := range lay.Texts {
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

	b.WriteString("</svg>\n")
	_, err := io.WriteString(w, b.String())
	return err
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
