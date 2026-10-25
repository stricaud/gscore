/*
 * xpm.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package symbol

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"strconv"
	"strings"
)

// DecodeXPM reads an X PixMap into a bitmap.
//
// XPM is C source holding an array of strings, which makes it the rare image
// format a person can draw by typing. That is why gscore used it for all 287
// of its symbols, and why it stays supported: it is the simplest way to draw a
// symbol that no font has.
//
// The decoder returns a grid at DefaultPixelSize with its origin at the left
// edge, vertically centred. XPM itself records neither, so a caller placing
// something other than a staff-centred symbol must set them.
func DecodeXPM(r io.Reader) (*Bitmap, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("xpm: %w", err)
	}
	strs := cStrings(string(data))
	if len(strs) == 0 {
		return nil, fmt.Errorf("xpm: no strings found; is this an XPM file?")
	}

	w, h, ncolors, cpp, err := xpmHeader(strs[0])
	if err != nil {
		return nil, err
	}
	if len(strs) < 1+ncolors+h {
		return nil, fmt.Errorf("xpm: file declares %d colours and %d rows but holds only %d strings",
			ncolors, h, len(strs))
	}

	palette := make(map[string]color.NRGBA, ncolors)
	for i := 0; i < ncolors; i++ {
		key, c, err := xpmColor(strs[1+i], cpp)
		if err != nil {
			return nil, fmt.Errorf("xpm: colour %d: %w", i, err)
		}
		palette[key] = c
	}

	b := NewBitmap(w, h)
	for y := 0; y < h; y++ {
		row := strs[1+ncolors+y]
		if len(row) < w*cpp {
			return nil, fmt.Errorf("xpm: row %d is %d chars, want %d", y, len(row), w*cpp)
		}
		for x := 0; x < w; x++ {
			key := row[x*cpp : (x+1)*cpp]
			c, ok := palette[key]
			if !ok {
				return nil, fmt.Errorf("xpm: row %d, column %d: undefined colour key %q", y, x, key)
			}
			b.Pix[y*w+x] = c
		}
	}
	return b, nil
}

// cStrings pulls the double-quoted literals out of C source, in order,
// skipping whatever surrounds them.
func cStrings(src string) []string {
	var out []string
	for i := 0; i < len(src); i++ {
		if src[i] != '"' {
			continue
		}
		var b strings.Builder
		i++
		for i < len(src) && src[i] != '"' {
			if src[i] == '\\' && i+1 < len(src) {
				i++
				switch src[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(src[i])
				}
				i++
				continue
			}
			b.WriteByte(src[i])
			i++
		}
		out = append(out, b.String())
	}
	return out
}

func xpmHeader(s string) (w, h, ncolors, cpp int, err error) {
	f := strings.Fields(s)
	if len(f) < 4 {
		return 0, 0, 0, 0, fmt.Errorf("xpm: bad header %q", s)
	}
	vals := make([]int, 4)
	for i := 0; i < 4; i++ {
		v, convErr := strconv.Atoi(f[i])
		if convErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("xpm: bad header field %q in %q", f[i], s)
		}
		vals[i] = v
	}
	w, h, ncolors, cpp = vals[0], vals[1], vals[2], vals[3]
	if w <= 0 || h <= 0 || cpp <= 0 || ncolors < 0 {
		return 0, 0, 0, 0, fmt.Errorf("xpm: nonsensical header %q", s)
	}
	return w, h, ncolors, cpp, nil
}

// colourKeys are XPM's visual classes. Only c, the colour display, and m, the
// monochrome fallback, carry anything useful here.
var colourKeys = map[string]bool{"c": true, "m": true, "s": true, "g": true, "g4": true, "b": true}

func xpmColor(s string, cpp int) (string, color.NRGBA, error) {
	if len(s) < cpp {
		return "", color.NRGBA{}, fmt.Errorf("entry %q is shorter than %d characters", s, cpp)
	}
	key := s[:cpp]

	// The remainder is a sequence of "<class> <colour>" pairs, and a colour
	// name may itself be several words, as in "navy blue".
	fields := strings.Fields(s[cpp:])
	values := map[string]string{}
	for i := 0; i < len(fields); {
		class := fields[i]
		if !colourKeys[class] {
			return "", color.NRGBA{}, fmt.Errorf("entry %q: unknown visual class %q", s, class)
		}
		i++
		var parts []string
		for i < len(fields) && !colourKeys[fields[i]] {
			parts = append(parts, fields[i])
			i++
		}
		values[class] = strings.Join(parts, " ")
	}

	spec, ok := values["c"]
	if !ok {
		spec, ok = values["m"]
	}
	if !ok || spec == "" {
		return "", color.NRGBA{}, fmt.Errorf("entry %q defines no colour", s)
	}
	c, err := parseXPMColor(spec)
	if err != nil {
		return "", color.NRGBA{}, fmt.Errorf("entry %q: %w", s, err)
	}
	return key, c, nil
}

// namedColors covers the X11 names that turn up in practice. An unknown name
// is an error rather than a guess, so that a file is never silently misread.
var namedColors = map[string]color.NRGBA{
	"black":   {0, 0, 0, 255},
	"white":   {255, 255, 255, 255},
	"red":     {255, 0, 0, 255},
	"green":   {0, 255, 0, 255},
	"blue":    {0, 0, 255, 255},
	"yellow":  {255, 255, 0, 255},
	"cyan":    {0, 255, 255, 255},
	"magenta": {255, 0, 255, 255},
	"gray":    {190, 190, 190, 255},
	"grey":    {190, 190, 190, 255},
	"navy":    {0, 0, 128, 255},
	"orange":  {255, 165, 0, 255},
	"pink":    {255, 192, 203, 255},
	"brown":   {165, 42, 42, 255},
	"purple":  {160, 32, 240, 255},
}

func parseXPMColor(spec string) (color.NRGBA, error) {
	s := strings.TrimSpace(spec)
	if strings.EqualFold(s, "none") {
		return color.NRGBA{}, nil
	}
	if strings.HasPrefix(s, "#") {
		return parseHexColor(s[1:])
	}
	lower := strings.ToLower(s)
	if c, ok := namedColors[lower]; ok {
		return c, nil
	}
	if c, ok := parseNumberedGray(lower); ok {
		return c, nil
	}
	return color.NRGBA{}, fmt.Errorf("unsupported colour %q", spec)
}

// parseNumberedGray handles X11's gray0 to gray100, a percentage scale rather
// than an intensity. Seventeen of gscore's toolbar pixmaps use them.
func parseNumberedGray(s string) (color.NRGBA, bool) {
	var digits string
	switch {
	case strings.HasPrefix(s, "gray"):
		digits = s[len("gray"):]
	case strings.HasPrefix(s, "grey"):
		digits = s[len("grey"):]
	default:
		return color.NRGBA{}, false
	}
	if digits == "" {
		return color.NRGBA{}, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 || n > 100 {
		return color.NRGBA{}, false
	}
	v := uint8((n*255 + 50) / 100)
	return color.NRGBA{R: v, G: v, B: v, A: 255}, true
}

// parseHexColor handles XPM's #RGB, #RRGGBB and #RRRRGGGGBBBB forms, which
// differ only in how many digits each channel gets.
func parseHexColor(hex string) (color.NRGBA, error) {
	if len(hex)%3 != 0 || len(hex) == 0 {
		return color.NRGBA{}, fmt.Errorf("bad hex colour %q", hex)
	}
	n := len(hex) / 3
	if n > 4 {
		return color.NRGBA{}, fmt.Errorf("bad hex colour %q", hex)
	}
	channel := func(i int) (uint8, error) {
		v, err := strconv.ParseUint(hex[i*n:(i+1)*n], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("bad hex colour %q", hex)
		}
		// Scale whatever precision the file used down to eight bits.
		max := uint64(1)<<(4*n) - 1
		return uint8(v * 255 / max), nil
	}
	r, err := channel(0)
	if err != nil {
		return color.NRGBA{}, err
	}
	g, err := channel(1)
	if err != nil {
		return color.NRGBA{}, err
	}
	b, err := channel(2)
	if err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: r, G: g, B: b, A: 255}, nil
}

// EncodeXPM writes a bitmap back out as XPM.
//
// Round-tripping matters because XPM is an authoring format here, not just an
// import path: a symbol saved into a .gscore file can be pulled out, edited by
// hand in a text editor, and put back.
func EncodeXPM(b *Bitmap, name string) []byte {
	// Collect the distinct colours, in a stable order.
	index := map[color.NRGBA]string{}
	var order []color.NRGBA
	for _, c := range b.Pix {
		key := c
		if key.A == 0 {
			key = color.NRGBA{} // every transparent pixel is the same colour
		}
		if _, seen := index[key]; !seen {
			index[key] = ""
			order = append(order, key)
		}
	}

	// Enough printable characters for one per colour; two per pixel when the
	// palette outgrows the alphabet.
	const alphabet = " .XoO+@#$%&*=-;:>,<1234567890qwertyuipasdfghjklzxcvbnmMNBVCZASDFGHJKLPIUYTREWQ"
	cpp := 1
	for pow := len(alphabet); pow < len(order); pow *= len(alphabet) {
		cpp++
	}
	for i, c := range order {
		index[c] = keyFor(i, cpp, alphabet)
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "/* XPM */\n/* %s */\nstatic char *%s[] = {\n", name, safeIdent(name))
	fmt.Fprintf(&out, "\"%d %d %d %d\",\n", b.W, b.H, len(order), cpp)
	for _, c := range order {
		if c.A == 0 {
			fmt.Fprintf(&out, "\"%s\tc None\",\n", index[c])
			continue
		}
		fmt.Fprintf(&out, "\"%s\tc #%02X%02X%02X\",\n", index[c], c.R, c.G, c.B)
	}
	for y := 0; y < b.H; y++ {
		out.WriteByte('"')
		for x := 0; x < b.W; x++ {
			c := b.At(x, y)
			if c.A == 0 {
				c = color.NRGBA{}
			}
			out.WriteString(index[c])
		}
		out.WriteByte('"')
		if y < b.H-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("};\n")
	return out.Bytes()
}

func keyFor(i, cpp int, alphabet string) string {
	buf := make([]byte, cpp)
	for p := cpp - 1; p >= 0; p-- {
		buf[p] = alphabet[i%len(alphabet)]
		i /= len(alphabet)
	}
	return string(buf)
}

func safeIdent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
			b.WriteByte(c)
		case c >= '0' && c <= '9' && b.Len() > 0:
			b.WriteByte(c)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "symbol"
	}
	return b.String() + "_xpm"
}
