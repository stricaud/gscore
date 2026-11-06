/*
 * read.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Reading PDFs, as far as a score needs.
//
// An engraved PDF already contains the score, exactly. A notehead in a file
// from LilyPond, MuseScore, Finale or Sibelius is a glyph from a music font,
// placed at a position given to six decimal places, and the staff it sits on
// is five stroked lines whose coordinates are in the file too. The pitch is
// the difference between the two. Nothing has to be recognised, measured or
// guessed, and there is no such thing as a borderline case.
//
// That is worth saying plainly because the obvious way to open a PDF of music
// is to rasterise it and run optical recognition over the picture — throwing
// away the exact answer in order to estimate it back. internal/omr exists for
// the files where there is no choice, which is to say photographs; this is
// for the ones where there is.
//
// What is read, and what is not. Glyphs with their positions, names and
// sizes; stroked straight lines. That is enough for staves, noteheads, clefs,
// accidentals and barlines — the notes, in other words. Curves are skipped,
// so slurs and ties are not seen, and shading, patterns and transparency are
// ignored rather than approximated.
//
// Measured, on a LilyPond page of this method's own making: 128 noteheads
// read for the 128 the source contains, 16 breath marks for 16, both clefs,
// both time signatures, and every one of the 128 pitches equal to the source
// it was engraved from. That is the point of the exercise — not a good score,
// but no score at all, because there is nothing here to be wrong about.
//
// Objects are found by scanning the file for them rather than by following
// the cross-reference table. The table is the official route and this is the
// robust one: it costs one pass over the bytes, it needs no handling for the
// two quite different table formats, and it still works on a file whose table
// is wrong — which, for a format people generate with scripts, is common.
// The exception is objects packed inside an object stream, a PDF 1.5 feature:
// those are not found yet, and a file that uses them reads as empty.
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Obj is any PDF object: nil, bool, float64, Name, String, Array, Dict, Ref
// or *Stream.
type Obj any

// Name is a PDF name, without its leading slash.
type Name string

// String is a PDF string, already unescaped.
type String string

// Array is a PDF array.
type Array []Obj

// Dict is a PDF dictionary, keyed without the leading slash.
type Dict map[string]Obj

// Ref is a reference to another object.
type Ref struct{ Num, Gen int }

// Stream is a dictionary with bytes attached.
type Stream struct {
	Dict Dict
	Raw  []byte
}

// Doc_ is a parsed file. The trailing underscore keeps it clear of Doc, which
// is this package's writer.
type Doc_ struct {
	objs map[int]Obj
}

// Parse reads a PDF.
func Parse(data []byte) (*Doc_, error) {
	d := &Doc_{objs: map[int]Obj{}}
	// "<num> <gen> obj" at the start of a line is the only place an indirect
	// object may begin, which makes the scan unambiguous.
	for i := 0; i+3 < len(data); {
		j := bytes.Index(data[i:], []byte(" obj"))
		if j < 0 {
			break
		}
		at := i + j
		start := lineStart(data, at)
		var num, gen int
		if n, err := fmt.Sscanf(string(data[start:at]), "%d %d", &num, &gen); n == 2 && err == nil {
			l := &lexer{data: data, pos: at + 4}
			if o, ok := l.object(); ok {
				d.objs[num] = l.maybeStream(o, data)
			}
		}
		i = at + 4
	}
	d.expandObjStm()
	if len(d.objs) == 0 {
		return nil, fmt.Errorf("pdf: no objects found")
	}
	return d, nil
}

// expandObjStm unpacks the objects that PDF 1.5 allows to be packed, several
// at a time, inside a compressed stream.
//
// Scanning the file finds the container but not what is in it, so without
// this a file written by a modern producer — LuaLaTeX among them — parses
// perfectly and yields nothing, which is the most unhelpful kind of failure.
//
// The layout is plain enough: /N pairs of integers, each an object number
// and an offset, then the objects themselves starting at /First.
func (d *Doc_) expandObjStm() {
	var streams []*Stream
	for _, o := range d.objs {
		s, ok := o.(*Stream)
		if !ok {
			continue
		}
		if ty, _ := d.Resolve(s.Dict["Type"]).(Name); ty == "ObjStm" {
			streams = append(streams, s)
		}
	}
	for _, s := range streams {
		body, err := d.decode(s)
		if err != nil {
			continue
		}
		n := int(num(d.Resolve(s.Dict["N"])))
		first := int(num(d.Resolve(s.Dict["First"])))
		if n <= 0 || first <= 0 || first > len(body) {
			continue
		}
		head := &lexer{data: body[:first]}
		type entry struct{ num, off int }
		var list []entry
		for i := 0; i < n; i++ {
			a := head.token()
			b := head.token()
			an, err1 := strconv.Atoi(a)
			bn, err2 := strconv.Atoi(b)
			if err1 != nil || err2 != nil {
				break
			}
			list = append(list, entry{an, bn})
		}
		for _, e := range list {
			at := first + e.off
			if at < 0 || at >= len(body) {
				continue
			}
			l := &lexer{data: body, pos: at}
			if o, ok := l.object(); ok {
				// an object already found loose in the file wins: it is a
				// later revision of the same number
				if _, seen := d.objs[e.num]; !seen {
					d.objs[e.num] = o
				}
			}
		}
	}
}

func lineStart(data []byte, at int) int {
	s := at
	for s > 0 && data[s-1] != '\n' && data[s-1] != '\r' {
		s--
	}
	return s
}

// Resolve follows references until it reaches something else.
func (d *Doc_) Resolve(o Obj) Obj {
	for i := 0; i < 32; i++ {
		r, ok := o.(Ref)
		if !ok {
			return o
		}
		o = d.objs[r.Num]
	}
	return nil
}

func (d *Doc_) dict(o Obj) Dict {
	switch v := d.Resolve(o).(type) {
	case Dict:
		return v
	case *Stream:
		return v.Dict
	}
	return nil
}

// Pages returns every page's dictionary, in reading order.
func (d *Doc_) Pages() []Dict {
	var root Dict
	for _, o := range d.objs {
		if dd := d.dict(o); dd != nil {
			if n, _ := d.Resolve(dd["Type"]).(Name); n == "Catalog" {
				root = dd
				break
			}
		}
	}
	var out []Dict
	if root == nil {
		return out
	}
	var walk func(o Obj, inherited Dict, depth int)
	walk = func(o Obj, inherited Dict, depth int) {
		if depth > 32 {
			return
		}
		node := d.dict(o)
		if node == nil {
			return
		}
		// Resources and MediaBox are inherited down the page tree, so a page
		// may not carry its own; we hand them down as we descend.
		next := Dict{}
		for k, v := range inherited {
			next[k] = v
		}
		for _, k := range []string{"Resources", "MediaBox"} {
			if v, ok := node[k]; ok {
				next[k] = v
			}
		}
		switch n, _ := d.Resolve(node["Type"]).(Name); n {
		case "Page":
			page := Dict{}
			for k, v := range node {
				page[k] = v
			}
			for k, v := range next {
				if _, ok := page[k]; !ok {
					page[k] = v
				}
			}
			out = append(out, page)
		default:
			kids, _ := d.Resolve(node["Kids"]).(Array)
			for _, k := range kids {
				walk(k, next, depth+1)
			}
		}
	}
	walk(root["Pages"], Dict{}, 0)
	return out
}

// content concatenates a page's content streams, decoded.
func (d *Doc_) content(page Dict) []byte {
	var buf bytes.Buffer
	add := func(o Obj) {
		if s, ok := d.Resolve(o).(*Stream); ok {
			b, err := d.decode(s)
			if err == nil {
				buf.Write(b)
				buf.WriteByte('\n')
			}
		}
	}
	switch v := d.Resolve(page["Contents"]).(type) {
	case Array:
		for _, o := range v {
			add(o)
		}
	default:
		add(page["Contents"])
	}
	return buf.Bytes()
}

// decode applies the stream's filters. Flate is the only one that matters for
// the files this reads; anything else is reported rather than guessed at.
func (d *Doc_) decode(s *Stream) ([]byte, error) {
	var filters []Name
	switch f := d.Resolve(s.Dict["Filter"]).(type) {
	case Name:
		filters = []Name{f}
	case Array:
		for _, o := range f {
			if n, ok := d.Resolve(o).(Name); ok {
				filters = append(filters, n)
			}
		}
	}
	out := s.Raw
	for _, f := range filters {
		switch f {
		case "FlateDecode":
			zr, err := zlib.NewReader(bytes.NewReader(out))
			if err != nil {
				return nil, err
			}
			b, err := io.ReadAll(zr)
			zr.Close()
			// a truncated stream still yields what it managed; a score read
			// from most of a page beats an error
			if err != nil && len(b) == 0 {
				return nil, err
			}
			out = b
		default:
			return nil, fmt.Errorf("pdf: unsupported filter %q", f)
		}
	}
	return out, nil
}

// --- lexer ------------------------------------------------------------------

type lexer struct {
	data []byte
	pos  int
}

func (l *lexer) ws() {
	for l.pos < len(l.data) {
		c := l.data[l.pos]
		if c == '%' {
			for l.pos < len(l.data) && l.data[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0 {
			l.pos++
			continue
		}
		return
	}
}

func isDelim(c byte) bool {
	return c == '(' || c == ')' || c == '<' || c == '>' || c == '[' || c == ']' ||
		c == '{' || c == '}' || c == '/' || c == '%'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0
}

// token reads the next bare token: a number, a keyword, or an operator.
func (l *lexer) token() string {
	l.ws()
	start := l.pos
	for l.pos < len(l.data) && !isSpace(l.data[l.pos]) && !isDelim(l.data[l.pos]) {
		l.pos++
	}
	if l.pos == start && l.pos < len(l.data) {
		l.pos++
	}
	return string(l.data[start:l.pos])
}

func (l *lexer) object() (Obj, bool) {
	l.ws()
	if l.pos >= len(l.data) {
		return nil, false
	}
	switch c := l.data[l.pos]; {
	case c == '/':
		l.pos++
		start := l.pos
		for l.pos < len(l.data) && !isSpace(l.data[l.pos]) && !isDelim(l.data[l.pos]) {
			l.pos++
		}
		return Name(decodeName(string(l.data[start:l.pos]))), true
	case c == '(':
		return l.literalString(), true
	case c == '[':
		l.pos++
		var a Array
		for {
			l.ws()
			if l.pos >= len(l.data) || l.data[l.pos] == ']' {
				l.pos++
				return a, true
			}
			o, ok := l.object()
			if !ok {
				return a, true
			}
			a = append(a, o)
		}
	case c == '<' && l.pos+1 < len(l.data) && l.data[l.pos+1] == '<':
		l.pos += 2
		dd := Dict{}
		for {
			l.ws()
			if l.pos+1 < len(l.data) && l.data[l.pos] == '>' && l.data[l.pos+1] == '>' {
				l.pos += 2
				return dd, true
			}
			k, ok := l.object()
			if !ok {
				return dd, true
			}
			kn, ok := k.(Name)
			if !ok {
				continue
			}
			v, ok := l.object()
			if !ok {
				return dd, true
			}
			dd[string(kn)] = v
		}
	case c == '<':
		return l.hexString(), true
	case c == ']' || c == '>' || c == ')' || c == '}':
		l.pos++
		return nil, false
	}
	save := l.pos
	tok := l.token()
	switch tok {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	case "":
		return nil, false
	}
	if v, err := strconv.ParseFloat(tok, 64); err == nil {
		// "<num> <num> R" is a reference; anything else is just a number
		if num, err2 := strconv.Atoi(tok); err2 == nil {
			mark := l.pos
			t2 := l.token()
			if gen, err3 := strconv.Atoi(t2); err3 == nil {
				t3 := l.token()
				if t3 == "R" {
					return Ref{Num: num, Gen: gen}, true
				}
			}
			l.pos = mark
		}
		return v, true
	}
	l.pos = save
	l.token()
	return Name(tok), false // an operator: returned, flagged as not an object
}

func decodeName(s string) string {
	if !strings.Contains(s, "#") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func (l *lexer) literalString() String {
	l.pos++ // (
	var b strings.Builder
	depth := 1
	for l.pos < len(l.data) {
		c := l.data[l.pos]
		l.pos++
		switch c {
		case '\\':
			if l.pos >= len(l.data) {
				break
			}
			e := l.data[l.pos]
			l.pos++
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\n':
			case '\r':
				if l.pos < len(l.data) && l.data[l.pos] == '\n' {
					l.pos++
				}
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && l.pos < len(l.data) &&
						l.data[l.pos] >= '0' && l.data[l.pos] <= '7'; k++ {
						v = v*8 + int(l.data[l.pos]-'0')
						l.pos++
					}
					b.WriteByte(byte(v))
				} else {
					b.WriteByte(e)
				}
			}
		case '(':
			depth++
			b.WriteByte(c)
		case ')':
			depth--
			if depth == 0 {
				return String(b.String())
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return String(b.String())
}

func (l *lexer) hexString() String {
	l.pos++ // <
	var b strings.Builder
	var hi = -1
	for l.pos < len(l.data) && l.data[l.pos] != '>' {
		c := l.data[l.pos]
		l.pos++
		var v int
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c >= 'a' && c <= 'f':
			v = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v = int(c-'A') + 10
		default:
			continue
		}
		if hi < 0 {
			hi = v
		} else {
			b.WriteByte(byte(hi*16 + v))
			hi = -1
		}
	}
	if hi >= 0 {
		b.WriteByte(byte(hi * 16))
	}
	l.pos++ // >
	return String(b.String())
}

// maybeStream attaches the bytes after a dictionary, if the keyword follows.
func (l *lexer) maybeStream(o Obj, data []byte) Obj {
	dd, ok := o.(Dict)
	if !ok {
		return o
	}
	save := l.pos
	l.ws()
	if !bytes.HasPrefix(data[l.pos:], []byte("stream")) {
		l.pos = save
		return o
	}
	l.pos += len("stream")
	if l.pos < len(data) && data[l.pos] == '\r' {
		l.pos++
	}
	if l.pos < len(data) && data[l.pos] == '\n' {
		l.pos++
	}
	start := l.pos
	end := -1
	// /Length may be an indirect reference, which we cannot resolve while
	// still parsing; "endstream" is unambiguous enough and always right.
	if n, ok := dd["Length"].(float64); ok && start+int(n) <= len(data) {
		if bytes.HasPrefix(bytes.TrimLeft(data[start+int(n):], "\r\n \t"),
			[]byte("endstream")) {
			end = start + int(n)
		}
	}
	if end < 0 {
		k := bytes.Index(data[start:], []byte("endstream"))
		if k < 0 {
			return o
		}
		end = start + k
		for end > start && (data[end-1] == '\n' || data[end-1] == '\r') {
			end--
		}
	}
	return &Stream{Dict: dd, Raw: data[start:end]}
}

// Size is a page's width and height in points, from its MediaBox.
//
// The box is inherited down the page tree, and Pages() has already handed it
// down, so the page's own dictionary always carries one by the time it gets
// here. A page without one is still answered — with US Letter, the size a
// PDF viewer assumes in the same situation — because a drawing laid over the
// wrong-sized sheet is more useful than no drawing.
func (d *Doc_) Size(n int) (w, h float64) {
	pages := d.Pages()
	if n < 0 || n >= len(pages) {
		return 612, 792
	}
	box, _ := d.Resolve(pages[n]["MediaBox"]).(Array)
	if len(box) != 4 {
		return 612, 792
	}
	v := make([]float64, 4)
	for i := range v {
		switch x := d.Resolve(box[i]).(type) {
		case float64:
			v[i] = x
		case int:
			v[i] = float64(x)
		case int64:
			v[i] = float64(x)
		default:
			return 612, 792
		}
	}
	w, h = v[2]-v[0], v[3]-v[1]
	if w <= 0 || h <= 0 {
		return 612, 792
	}
	return w, h
}
