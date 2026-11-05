/*
 * marks.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"math"
	"strconv"
)

// Glyph is one character drawn on the page, with the name its font gives it.
//
// The name is what makes this worth doing. A PDF from LilyPond carries an
// encoding table that says, in the file, that code 0 of this font is
// "noteheads.s0" and code 3 is "clefs.G"; a SMuFL font gives names like
// "noteheadBlack". Either way the file tells us what the mark means, and we
// are spared having to recognise it.
type Glyph struct {
	Name string  // from the font's /Differences, empty if it gives none
	Code uint32  // the code in the string: one byte, or two for a CID font
	Rune rune    // from the font's /ToUnicode, zero if it gives none
	Font string  // the resource name, e.g. "R8"
	Base string  // the font's own name, e.g. "Leland"
	Size float64 // in points, after the transform
	X, Y float64 // the glyph's origin, in page points from the bottom left
}

// Named reports a name for the glyph if the file gave one, either way round.
//
// Two producers, two conventions. LilyPond writes a simple font with a
// /Differences table naming every code — "noteheads.s0" — and no ToUnicode.
// MuseScore writes a two-byte CID font with no names at all, but a ToUnicode
// map, and because Leland is a SMuFL font that map lands in the range the
// standard reserves, where the codepoint *is* the identity of the glyph.
//
// This package stops at reporting both; turning U+E0A2 into "noteheadWhole"
// is the music layer's business, and internal/smufl already does it.
func (g Glyph) Named() bool { return g.Name != "" || g.Rune != 0 }

// Segment is a straight line that was stroked.
//
// Staff lines, stems, barlines and beams are all this. Curves are not
// recorded: a slur carries no pitch, and leaving it out is cheaper than
// carrying something we would not use.
type Segment struct {
	X1, Y1, X2, Y2 float64
	Width          float64
}

// Horizontal reports whether the segment lies flat, within a hair.
//
// Staff lines in a tilted scan are not flat, but in an engraved PDF they are
// exactly flat, and the tolerance is only here for files written with
// rounding errors.
func (s Segment) Horizontal() bool {
	return math.Abs(s.Y2-s.Y1) <= 0.05*math.Max(1, math.Abs(s.X2-s.X1))
}

// Vertical reports whether the segment stands upright.
func (s Segment) Vertical() bool {
	return math.Abs(s.X2-s.X1) <= 0.05*math.Max(1, math.Abs(s.Y2-s.Y1))
}

// Content is everything read off one page.
type Content struct {
	Width, Height float64
	Glyphs        []Glyph
	Segments      []Segment
}

// matrix is a PDF transform, [a b c d e f].
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func (m matrix) apply(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// scale is the transform's magnification, used to turn a font size in text
// space into one in page points.
func (m matrix) scale() float64 {
	return math.Hypot(m[1], m[3])
}

// Page reads one page, numbered from zero.
func (d *Doc_) Page(n int) (*Content, error) {
	pages := d.Pages()
	if n < 0 || n >= len(pages) {
		return nil, errPage{want: n, have: len(pages)}
	}
	page := pages[n]
	c := &Content{Width: 612, Height: 792}
	if box, ok := d.Resolve(page["MediaBox"]).(Array); ok && len(box) == 4 {
		x0, y0 := num(d.Resolve(box[0])), num(d.Resolve(box[1]))
		x1, y1 := num(d.Resolve(box[2])), num(d.Resolve(box[3]))
		c.Width, c.Height = x1-x0, y1-y0
	}
	d.run(d.content(page), d.dict(page["Resources"]), state{ctm: identity, width: 1}, c, 0)
	return c, nil
}

type errPage struct {
	want, have int
}

func (e errPage) Error() string {
	return "pdf: page " + strconv.Itoa(e.want) + " of " + strconv.Itoa(e.have)
}

func num(o Obj) float64 {
	v, _ := o.(float64)
	return v
}

// fontInfo is what we need of a font: its real name, and what its codes mean.
type fontInfo struct {
	base    string
	names   map[byte]string
	uni     map[uint32]rune
	twoByte bool
}

// fonts reads a resource dictionary's fonts, keeping each one's encoding.
//
// The /Differences array is the prize. It is a flat list mixing numbers and
// names — "0 /noteheads.s0 /scripts.rcomma" means code 0 is the first name,
// code 1 the next, and so on until another number resets the counter.
func (d *Doc_) fonts(res Dict) map[string]fontInfo {
	out := map[string]fontInfo{}
	if res == nil {
		return out
	}
	fd := d.dict(res["Font"])
	for key, ref := range fd {
		f := d.dict(ref)
		if f == nil {
			continue
		}
		info := fontInfo{names: map[byte]string{}, uni: map[uint32]rune{}}
		if b, ok := d.Resolve(f["BaseFont"]).(Name); ok {
			info.base = string(b)
			// producers prefix a subset tag, "OPMUBF+Emmentaler-18"
			if len(info.base) > 7 && info.base[6] == '+' {
				info.base = info.base[7:]
			}
		}
		// A Type0 font addresses its glyphs with two bytes, not one. Reading
		// it a byte at a time yields twice as many glyphs as the page has,
		// every one of them wrong, so this test has to come before anything
		// else is believed about the string.
		if sub, _ := d.Resolve(f["Subtype"]).(Name); sub == "Type0" {
			info.twoByte = true
		}
		if e, ok := d.Resolve(f["Encoding"]).(Name); ok &&
			(e == "Identity-H" || e == "Identity-V") {
			info.twoByte = true
		}
		if s, ok := d.Resolve(f["ToUnicode"]).(*Stream); ok {
			if body, err := d.decode(s); err == nil {
				parseCMap(body, info.uni)
			}
		}
		enc := d.dict(f["Encoding"])
		if enc != nil {
			if diff, ok := d.Resolve(enc["Differences"]).(Array); ok {
				code := 0
				for _, o := range diff {
					switch v := d.Resolve(o).(type) {
					case float64:
						code = int(v)
					case Name:
						if code >= 0 && code < 256 {
							info.names[byte(code)] = string(v)
						}
						code++
					}
				}
			}
		}
		out[key] = info
	}
	return out
}

// state is the part of the graphics state that affects where things land.
type state struct {
	ctm   matrix
	width float64
}

// run walks the content stream and records what it draws.
//
// Only the operators that place something are handled. Colour, shading,
// transparency and clipping are read past: a score is black on white, and a
// reader that tried to honour them would be a renderer.
func (d *Doc_) run(content []byte, res Dict, gs state, c *Content, depth int) {
	if depth > 8 {
		return
	}
	fonts := d.fonts(res)
	l := &lexer{data: content}
	var stack []Obj
	var saved []state

	var tm, tlm matrix // text matrix, and the line matrix it restarts from
	var font fontInfo
	var fontKey string
	var size, leading float64

	var cur struct{ x, y float64 }
	var start struct{ x, y float64 }
	var pending []Segment // the path being built, not yet painted

	pop := func(n int) []float64 {
		v := make([]float64, n)
		for i := n - 1; i >= 0; i-- {
			if len(stack) == 0 {
				continue
			}
			v[i] = num(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
		}
		return v
	}

	show := func(s String) {
		step := 1
		if font.twoByte {
			step = 2
		}
		for i := 0; i+step <= len(s); i += step {
			var code uint32
			if step == 2 {
				code = uint32(s[i])<<8 | uint32(s[i+1])
			} else {
				code = uint32(s[i])
			}
			trm := matrix{size, 0, 0, size, 0, 0}.mul(tm).mul(gs.ctm)
			x, y := trm.apply(0, 0)
			var nm string
			if code < 256 {
				nm = font.names[byte(code)]
			}
			c.Glyphs = append(c.Glyphs, Glyph{
				Name: nm,
				Code: code,
				Rune: font.uni[code],
				Font: fontKey,
				Base: font.base,
				Size: trm.scale(),
				X:    x, Y: y,
			})
			// We do not advance by the glyph's width: the files this reads
			// position every glyph with its own Tm or Td, so the advance is
			// never used, and carrying a width table to compute it would be
			// a lot of machinery for a number nobody reads.
		}
	}

	for {
		l.ws()
		if l.pos >= len(l.data) {
			return
		}
		o, isObj := l.object()
		if isObj {
			stack = append(stack, o)
			continue
		}
		op, _ := o.(Name)
		switch string(op) {
		case "q":
			saved = append(saved, gs)
		case "Q":
			if n := len(saved); n > 0 {
				gs = saved[n-1]
				saved = saved[:n-1]
			}
		case "cm":
			v := pop(6)
			gs.ctm = matrix{v[0], v[1], v[2], v[3], v[4], v[5]}.mul(gs.ctm)
		case "w":
			gs.width = pop(1)[0]
		case "m":
			v := pop(2)
			cur.x, cur.y = v[0], v[1]
			start = cur
		case "l":
			v := pop(2)
			x1, y1 := gs.ctm.apply(cur.x, cur.y)
			x2, y2 := gs.ctm.apply(v[0], v[1])
			pending = append(pending, Segment{x1, y1, x2, y2,
				gs.width * gs.ctm.scale()})
			cur.x, cur.y = v[0], v[1]
		case "h":
			x1, y1 := gs.ctm.apply(cur.x, cur.y)
			x2, y2 := gs.ctm.apply(start.x, start.y)
			pending = append(pending, Segment{x1, y1, x2, y2,
				gs.width * gs.ctm.scale()})
			cur = start
		case "re":
			v := pop(4)
			x, y, w, h := v[0], v[1], v[2], v[3]
			pts := [5][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}, {x, y}}
			for i := 0; i < 4; i++ {
				x1, y1 := gs.ctm.apply(pts[i][0], pts[i][1])
				x2, y2 := gs.ctm.apply(pts[i+1][0], pts[i+1][1])
				pending = append(pending, Segment{x1, y1, x2, y2,
					gs.width * gs.ctm.scale()})
			}
		case "c", "v", "y":
			// a curve: its endpoint becomes current, the shape is dropped
			var v []float64
			if string(op) == "c" {
				v = pop(6)
				cur.x, cur.y = v[4], v[5]
			} else {
				v = pop(4)
				cur.x, cur.y = v[2], v[3]
			}
		case "S", "s", "f", "F", "f*", "B", "B*", "b", "b*":
			c.Segments = append(c.Segments, pending...)
			pending = nil
			stack = nil
		case "n":
			pending = nil
			stack = nil
		case "BT":
			tm, tlm = identity, identity
		case "ET":
		case "Tf":
			if len(stack) >= 2 {
				size = num(stack[len(stack)-1])
				if n, ok := stack[len(stack)-2].(Name); ok {
					fontKey = string(n)
					font = fonts[fontKey]
				}
			}
			stack = nil
		case "Tm":
			v := pop(6)
			tm = matrix{v[0], v[1], v[2], v[3], v[4], v[5]}
			tlm = tm
		case "Td":
			v := pop(2)
			tlm = matrix{1, 0, 0, 1, v[0], v[1]}.mul(tlm)
			tm = tlm
		case "TD":
			v := pop(2)
			leading = -v[1]
			tlm = matrix{1, 0, 0, 1, v[0], v[1]}.mul(tlm)
			tm = tlm
		case "TL":
			leading = pop(1)[0]
		case "T*":
			tlm = matrix{1, 0, 0, 1, 0, -leading}.mul(tlm)
			tm = tlm
		case "Tj":
			if len(stack) > 0 {
				if s, ok := stack[len(stack)-1].(String); ok {
					show(s)
				}
			}
			stack = nil
		case "'":
			tlm = matrix{1, 0, 0, 1, 0, -leading}.mul(tlm)
			tm = tlm
			if len(stack) > 0 {
				if s, ok := stack[len(stack)-1].(String); ok {
					show(s)
				}
			}
			stack = nil
		case "\"":
			if len(stack) >= 3 {
				leading = num(stack[len(stack)-3])
			}
			tlm = matrix{1, 0, 0, 1, 0, -leading}.mul(tlm)
			tm = tlm
			if len(stack) > 0 {
				if s, ok := stack[len(stack)-1].(String); ok {
					show(s)
				}
			}
			stack = nil
		case "TJ":
			if len(stack) > 0 {
				if a, ok := stack[len(stack)-1].(Array); ok {
					for _, e := range a {
						switch v := e.(type) {
						case String:
							show(v)
						case float64:
							// a kern, in thousandths of the font size, moving
							// the text position back
							tm = matrix{1, 0, 0, 1, -v / 1000 * size, 0}.mul(tm)
						}
					}
				}
			}
			stack = nil
		case "Do":
			// A form XObject is a page within the page: its own content
			// stream, its own resources, and a matrix saying where it goes.
			// Ignoring it is not an option — pdfcrop, and every tool that
			// imposes or reuses a page, wraps the whole content in one, so
			// a reader that skips Do reads those files as blank.
			if len(stack) > 0 && res != nil {
				if n, ok := stack[len(stack)-1].(Name); ok {
					d.form(string(n), res, gs, c, depth)
				}
			}
			stack = nil
		default:
			stack = nil
		}
	}
}

// form draws an XObject, if it is a form; images are not our business.
func (d *Doc_) form(name string, res Dict, gs state, c *Content, depth int) {
	xo := d.dict(res["XObject"])
	if xo == nil {
		return
	}
	s, ok := d.Resolve(xo[name]).(*Stream)
	if !ok {
		return
	}
	if sub, _ := d.Resolve(s.Dict["Subtype"]).(Name); sub != "Form" {
		return
	}
	if m, ok := d.Resolve(s.Dict["Matrix"]).(Array); ok && len(m) == 6 {
		var mm matrix
		for i := range mm {
			mm[i] = num(d.Resolve(m[i]))
		}
		gs.ctm = mm.mul(gs.ctm)
	}
	inner := d.dict(s.Dict["Resources"])
	if inner == nil {
		inner = res
	}
	body, err := d.decode(s)
	if err != nil {
		return
	}
	d.run(body, inner, gs, c, depth+1)
}

// parseCMap reads the code-to-Unicode map a font may carry.
//
// Only the two forms that matter are handled, and they are both plain: a
// bfchar block pairing one code with one value, and a bfrange block giving a
// first code, a last code, and either the value the first maps to or a list
// of values. Everything else a CMap may contain describes how to render
// text, which is not why we are here.
func parseCMap(body []byte, out map[uint32]rune) {
	l := &lexer{data: body}
	var pending []Obj
	for {
		l.ws()
		if l.pos >= len(l.data) {
			return
		}
		o, isObj := l.object()
		if isObj {
			pending = append(pending, o)
			if len(pending) > 64 {
				pending = pending[len(pending)-64:]
			}
			continue
		}
		switch op, _ := o.(Name); string(op) {
		case "beginbfchar":
			for {
				src, ok := l.object()
				if !ok {
					break
				}
				dst, ok := l.object()
				if !ok {
					break
				}
				a, okA := src.(String)
				b, okB := dst.(String)
				if okA && okB {
					out[beCode(a)] = beRune(b)
				}
			}
		case "beginbfrange":
			for {
				lo, ok := l.object()
				if !ok {
					break
				}
				hi, ok := l.object()
				if !ok {
					break
				}
				dst, ok := l.object()
				if !ok {
					break
				}
				a, okA := lo.(String)
				b, okB := hi.(String)
				if !okA || !okB {
					break
				}
				first, last := beCode(a), beCode(b)
				if last < first || last-first > 0xFFFF {
					continue
				}
				switch v := dst.(type) {
				case String:
					base := beRune(v)
					for k := first; k <= last; k++ {
						out[k] = base + rune(k-first)
					}
				case Array:
					for k := first; k <= last && int(k-first) < len(v); k++ {
						if s, ok := v[k-first].(String); ok {
							out[k] = beRune(s)
						}
					}
				}
			}
		}
		pending = nil
	}
}

// beCode reads a big-endian code of one or two bytes.
func beCode(s String) uint32 {
	var v uint32
	for i := 0; i < len(s) && i < 4; i++ {
		v = v<<8 | uint32(s[i])
	}
	return v
}

// beRune reads a UTF-16BE value, taking the first unit; the surrogate pairs
// that would need more live outside the range music fonts use.
func beRune(s String) rune {
	if len(s) >= 2 {
		return rune(uint32(s[0])<<8 | uint32(s[1]))
	}
	if len(s) == 1 {
		return rune(s[0])
	}
	return 0
}
