/*
 * font.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package pdf

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stricaud/gscore/internal/sfnt"
)

// Fonts in a PDF, of which there are two kinds here.
//
// The fourteen standard fonts — Times, Helvetica, Courier and their variants —
// need no embedding at all: every PDF reader has them, and a title set in
// Times is a dozen bytes of dictionary. That is what the words on a score use.
//
// The music font cannot work that way. Nobody's reader has Bravura, so it is
// embedded whole, which means describing it as a CID font: a two-byte
// encoding, a table of glyph widths, and the font file itself as a stream.
// The reason for the two-byte encoding is that SMuFL lives at U+E000 and
// above, well past anything a single-byte encoding can reach.

// Font is a font a page can draw with.
type Font struct {
	key string
	obj int

	// For a standard font: the widths of the Latin-1 range.
	std *stdFont

	// For an embedded font: what the glyphs are and how wide.
	f        *sfnt.Font
	glyphs   map[rune]uint16
	embedded bool
}

// StandardFont names one of the fourteen faces every reader has.
type StandardFont string

const (
	Times            StandardFont = "Times-Roman"
	TimesBold        StandardFont = "Times-Bold"
	TimesItalic      StandardFont = "Times-Italic"
	TimesBoldItalic  StandardFont = "Times-BoldItalic"
	Helvetica        StandardFont = "Helvetica"
	HelveticaBold    StandardFont = "Helvetica-Bold"
	HelveticaOblique StandardFont = "Helvetica-Oblique"
	Courier          StandardFont = "Courier"
)

// Standard adds one of the built-in fonts, or returns the one already added.
func (d *Doc) Standard(name StandardFont) *Font {
	key := "F" + sanitise(string(name))
	if f, ok := d.fonts[key]; ok {
		return f
	}
	// WinAnsiEncoding so that the accented letters in a composer's name come
	// out right without a per-document encoding table.
	obj := d.add("<< /Type /Font /Subtype /Type1 /BaseFont /%s "+
		"/Encoding /WinAnsiEncoding >>", name)
	f := &Font{key: key, obj: obj, std: stdMetrics(name)}
	d.fonts[key] = f
	d.fontKeys = append(d.fontKeys, key)
	return f
}

// Embed adds a font from its file, so a reader that does not have it can
// still draw with it. The whole file goes in: subsetting a CFF font means
// rewriting its charstring index, and a music font that prints correctly
// everywhere is worth half a megabyte.
func (d *Doc) Embed(name string, data []byte) (*Font, error) {
	key := "F" + sanitise(name)
	if f, ok := d.fonts[key]; ok {
		return f, nil
	}
	parsed, err := sfnt.Parse(data)
	if err != nil {
		return nil, err
	}

	f := &Font{key: key, f: parsed, glyphs: map[rune]uint16{}, embedded: true}
	d.fonts[key] = f
	d.fontKeys = append(d.fontKeys, key)

	// The objects are made now and filled in at the end, because the widths
	// cannot be known until every page has been drawn: only then is it
	// settled which glyphs the document uses.
	f.obj = d.add("<< >>")
	d.pending = append(d.pending, f)
	return f, nil
}

// finish writes out an embedded font once the document knows which glyphs it
// needs. A /W array listing three and a half thousand widths would be most of
// the file; listing the hundred or so that are actually drawn is nothing.
func (d *Doc) finish(f *Font) {
	ps := f.f.PostScriptName
	file := d.addStream(fmt.Sprintf("/Subtype /OpenType"), f.f.Data)

	flags := 4 // symbolic: the font does not use the standard Latin set
	descriptor := d.add("<< /Type /FontDescriptor /FontName /%s /Flags %d "+
		"/FontBBox [%d %d %d %d] /ItalicAngle %.1f /Ascent %d /Descent %d "+
		"/CapHeight %d /StemV 80 /FontFile3 %d 0 R >>",
		ps, flags,
		f.f.Scale(f.f.XMin), f.f.Scale(f.f.YMin),
		f.f.Scale(f.f.XMax), f.f.Scale(f.f.YMax),
		f.f.ItalicAngle, f.f.Scale(f.f.Ascent), f.f.Scale(f.f.Descent),
		f.f.Scale(f.f.CapHeight), file)

	// The widths, by glyph id, for the glyphs this document draws.
	ids := make([]int, 0, len(f.glyphs))
	seen := map[uint16]bool{}
	for _, g := range f.glyphs {
		if !seen[g] {
			seen[g] = true
			ids = append(ids, int(g))
		}
	}
	sort.Ints(ids)
	var widths strings.Builder
	widths.WriteByte('[')
	for i := 0; i < len(ids); {
		// Runs of consecutive ids share one entry, which is most of them in
		// a music font: the glyphs of one range are numbered together.
		j := i
		for j+1 < len(ids) && ids[j+1] == ids[j]+1 {
			j++
		}
		fmt.Fprintf(&widths, " %d [", ids[i])
		for k := i; k <= j; k++ {
			fmt.Fprintf(&widths, " %d", f.f.Advance(uint16(ids[k])))
		}
		widths.WriteString(" ]")
		i = j + 1
	}
	widths.WriteString(" ]")

	// CIDToGIDMap is /Identity for a CIDFontType0 by definition: the CID is
	// the glyph id, which is why the text is encoded as glyph ids.
	cid := d.add("<< /Type /Font /Subtype /CIDFontType0 /BaseFont /%s "+
		"/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> "+
		"/FontDescriptor %d 0 R /DW 0 /W %s >>",
		ps, descriptor, widths.String())

	toUni := d.addStream("", []byte(f.toUnicode()))

	d.objects[f.obj].data = []byte(fmt.Sprintf(
		"<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H "+
			"/DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>", ps, cid, toUni))
}

// toUnicode builds the map that lets text be copied out of the PDF.
//
// The text in the stream is glyph ids, which mean nothing outside this font.
// Without this, selecting a bar of music and copying it gives gibberish; with
// it, it gives the SMuFL characters, which is at least honest.
func (f *Font) toUnicode() string {
	type pair struct {
		gid uint16
		r   rune
	}
	pairs := make([]pair, 0, len(f.glyphs))
	for r, g := range f.glyphs {
		pairs = append(pairs, pair{g, r})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].gid < pairs[j].gid })

	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	b.WriteString("/CMapName /gscore-identity def\n/CMapType 2 def\n")
	b.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	b.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")

	// A hundred at a time is the limit the format sets.
	for i := 0; i < len(pairs); i += 100 {
		end := i + 100
		if end > len(pairs) {
			end = len(pairs)
		}
		fmt.Fprintf(&b, "%d beginbfchar\n", end-i)
		for _, p := range pairs[i:end] {
			if p.r > 0xFFFF {
				// Outside the basic plane, written as a surrogate pair.
				v := p.r - 0x10000
				fmt.Fprintf(&b, "<%04X> <%04X%04X>\n", p.gid,
					0xD800+(v>>10), 0xDC00+(v&0x3FF))
				continue
			}
			fmt.Fprintf(&b, "<%04X> <%04X>\n", p.gid, p.r)
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.String()
}

// encode turns a string into the bytes a Tj operator wants.
func (f *Font) encode(s string) string {
	if !f.embedded {
		var b strings.Builder
		b.WriteByte('(')
		for _, r := range s {
			c := byte('?')
			if r < 256 {
				c = byte(r)
			}
			switch c {
			case '(', ')', '\\':
				b.WriteByte('\\')
			}
			b.WriteByte(c)
		}
		b.WriteByte(')')
		return b.String()
	}

	// Embedded: two bytes per glyph, hexadecimal, which avoids having to
	// escape anything at all.
	var b strings.Builder
	b.WriteByte('<')
	for _, r := range s {
		g := f.f.GlyphIndex(r)
		if g != 0 {
			f.glyphs[r] = g
		}
		fmt.Fprintf(&b, "%04X", g)
	}
	b.WriteByte('>')
	return b.String()
}

// width is how wide a string is, in thousandths of the type size.
func (f *Font) width(s string) float64 {
	total := 0
	for _, r := range s {
		if f.embedded {
			total += f.f.Advance(f.f.GlyphIndex(r))
			continue
		}
		total += f.std.width(r)
	}
	return float64(total)
}

func sanitise(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "X"
	}
	return b.String()
}
