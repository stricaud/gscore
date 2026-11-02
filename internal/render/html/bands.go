/*
 * bands.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package html

import (
	"math"

	"github.com/stricaud/gscore/internal/engrave"
)

// A drawing too large for a page, cut into pieces the page fetches.
//
// The editor has always done this against its own server: it keeps one
// drawing the size of the whole score and asks for the stretch it is showing
// plus a screenful either way, because a browser handed Beethoven's ninth —
// nine hundred thousand shapes, two and a third million units tall — spends
// seconds laying it out and then stutters at every scroll.
//
// A page on a plain file host has no server to ask. So the pieces are written
// out beside it instead, once, and the page fetches them as files. Same
// mechanism; the answers are just already on disk.

// Band is one piece of a drawing, written on its own.
type Band struct {
	Index int `json:"i"`
	// Y0 and Y1 are where the piece sits in the drawing, in the units the
	// page uses everywhere else — the same ones Box is in, so the player can
	// compare a band against the bar it is playing without converting.
	Y0 float64 `json:"y0"`
	Y1 float64 `json:"y1"`
	// First and Last are the bars it holds, so a player that knows which bar
	// it is in knows which piece it wants without measuring anything.
	First int `json:"first"`
	Last  int `json:"last"`
	// URL is where to fetch it.
	URL string `json:"u"`
}

// Bands cuts a layout into pieces of perBand systems each.
//
// Between systems and never through one. A piece cut through a system would
// show half an orchestra, and the staves of a system belong to each other:
// they are one moment of the music read downwards.
//
// A system that reaches a little into the next piece is in both of them.
// Drawing a shape twice costs one shape; leaving it out is a gap in the
// music, which is the trade svg.Options.Window already makes at its edges.
//
// url says where piece i will be served from. It is the caller's business
// because only the caller knows what it is going to call the files.
func Bands(lay *engrave.Layout, scale float64, perBand int, url func(i int) string) []Band {
	if lay == nil || len(lay.Systems) == 0 || perBand < 1 {
		return nil
	}
	var out []Band
	// Where the last piece ended, which is where the next one starts.
	from := 0.0
	for start := 0; start < len(lay.Systems); start += perBand {
		end := start + perBand
		if end > len(lay.Systems) {
			end = len(lay.Systems)
		}
		group := lay.Systems[start:end]

		y0, y1 := math.Inf(1), math.Inf(-1)
		first, last := math.MaxInt32, -1
		for _, sys := range group {
			y0 = math.Min(y0, sys.Top)
			y1 = math.Max(y1, sys.Bottom)
			if sys.First < first {
				first = sys.First
			}
			if sys.Last > last {
				last = sys.Last
			}
		}
		// The pieces divide the drawing between them with nothing left over:
		// each begins where the one before ended, the first at the very top
		// and the last at the very bottom.
		//
		// Cut to the systems alone they do not, and what falls between them
		// is drawn in none of them. Two things live there. Above the first
		// system are the title, the composer and the tempo — eight pieces of
		// text, which left the score headed by nothing. And between any two
		// systems is whatever is written in the gap: a line across the full
		// width of the page went missing from the middle of a flute solo.
		y0 = from
		if end == len(lay.Systems) {
			y1 = math.Max(y1, lay.Height)
		}
		from = y1

		i := len(out)
		out = append(out, Band{
			Index: i,
			Y0:    round(y0 * scale),
			Y1:    round(y1 * scale),
			First: first,
			Last:  last,
			URL:   url(i),
		})
	}
	return out
}

// Window is the piece of the drawing a band covers, in the engraver's own
// units, ready for svg.Options.Window.
//
// Scale is undone here rather than at the call: a band is written down in the
// page's units because that is what the page compares it against, and the
// renderer works in staff spaces.
func (b Band) Window(width, scale float64) (x0, y0, x1, y1 float64) {
	if scale == 0 {
		scale = 1
	}
	return 0, b.Y0 / scale, width, b.Y1 / scale
}
