/*
 * colour.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import "strings"

// Colour on the page.
//
// A layout carries colours as the strings a stylesheet would: "#6f9fd8". The
// editor draws into a browser, which speaks that natively, and the other
// renderers do not — a PDF wants three numbers between nought and one. So the
// spelling is settled in one place rather than in each of them, because a
// colour that comes out one shade on the screen and another on the printed
// page is a bug nobody can see until the paper is in their hand.

// RGB reads a CSS hex colour into the three components a renderer wants.
//
// "#rgb" and "#rrggbb", with or without the hash, which is every form the
// editor's own colour picker produces. Anything else is not a colour this
// knows how to print, and says so rather than guessing: a glyph drawn in a
// colour nobody asked for is worse than one drawn in black.
func RGB(colour string) (r, g, b float64, ok bool) {
	s := strings.TrimSpace(colour)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		// The short form doubles each digit, so "#f00" and "#ff0000" are the
		// same red — which is what every browser does with it.
		var v [3]float64
		for i := 0; i < 3; i++ {
			n, good := hexDigit(s[i])
			if !good {
				return 0, 0, 0, false
			}
			v[i] = float64(n*16+n) / 255
		}
		return v[0], v[1], v[2], true
	case 6:
		var v [3]float64
		for i := 0; i < 3; i++ {
			hi, good := hexDigit(s[i*2])
			if !good {
				return 0, 0, 0, false
			}
			lo, good := hexDigit(s[i*2+1])
			if !good {
				return 0, 0, 0, false
			}
			v[i] = float64(hi*16+lo) / 255
		}
		return v[0], v[1], v[2], true
	}
	return 0, 0, 0, false
}

func hexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}
