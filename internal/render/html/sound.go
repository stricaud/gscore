/*
 * sound.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package html

import (
	"encoding/json"
	"math"

	"github.com/stricaud/gscore/internal/engrave"
)

// What a shared page needs to play itself.
//
// The editor builds this from the score; the page is handed the result
// rather than the score, because working out when every note sounds is the
// program's job and not a web page's. These types are the shape the player
// in the page expects, and they are declared here so that what crosses that
// line is written down rather than being a blob of JSON nobody owns.

// Note is one sound: when it starts, how long it lasts, and what it is.
type Note struct {
	Start float64 `json:"t"`
	Dur   float64 `json:"d"`
	MIDI  int     `json:"m"`
	// Velocity is how loud, 0 to 127, from the dynamic in force.
	Velocity int `json:"v,omitempty"`
	// Gate is how much of Dur is sounded, as a fraction: short under a
	// staccato dot, the whole length under a tenuto line, over 1 where a
	// slur carries the note into the next one.
	Gate float64 `json:"g,omitempty"`
	// Family is how it is sounded — sustained, plucked, struck — which is
	// all the player needs to choose a voice.
	Family string `json:"f,omitempty"`
	// Ref is the element it came from, numbered as the engraver numbers its
	// glyphs. That shared numbering is what lights up the notehead that is
	// sounding.
	Ref int `json:"r,omitempty"`
}

// Bar is when a measure begins and how long it lasts.
//
// The cursor follows this rather than the notes: a score has one timeline
// but many staves, and a cursor built from note positions lurches between
// them.
type Bar struct {
	Index int     `json:"i"`
	Start float64 `json:"t"`
	Dur   float64 `json:"d"`
}

// Box is where a measure was drawn, in the page's own units.
type Box struct {
	Index  int     `json:"i"`
	X0, X1 float64 `json:"-"`
	Top    float64 `json:"-"`
	Bottom float64 `json:"-"`
	// The JSON is terse because there is one of these per bar and a long
	// score has hundreds.
	A float64 `json:"a"` // left
	B float64 `json:"b"` // right
	C float64 `json:"c"` // top
	// Named for its field, like the other three. It said "e" — a slip of one
	// key along the row — and the player reads a bar's bottom as "d", so
	// every bar in every shared page ever written said its bottom was
	// nothing at all. The cursor's height is the bottom less the top, so the
	// cursor was NaN tall: there has never been a visible playback cursor on
	// a shared page, on any score. The page and the player are written and
	// served together, so the two agreeing is the whole of the fix.
	D float64 `json:"d"` // bottom
}

// Sound is everything the page plays and follows.
type Sound struct {
	Notes    []Note  `json:"notes"`
	Bars     []Bar   `json:"bars"`
	Boxes    []Box   `json:"boxes"`
	Duration float64 `json:"duration"`
	Tempo    int     `json:"tempo"`
}

// Boxes is where each measure was drawn, in the units the page's SVG uses.
//
// One box per bar, taken across every staff: the cursor is a line through
// the whole system, as a conductor's eye is, and not a mark on one staff.
func Boxes(lay *engrave.Layout, scale float64) []Box {
	byBar := map[int]*Box{}
	for _, h := range lay.Hits {
		b, ok := byBar[h.Measure]
		if !ok {
			b = &Box{Index: h.Measure, X0: math.Inf(1), X1: math.Inf(-1),
				Top: math.Inf(1), Bottom: math.Inf(-1)}
			byBar[h.Measure] = b
		}
		b.X0 = math.Min(b.X0, h.XMin)
		b.X1 = math.Max(b.X1, h.XMax)
		b.Top = math.Min(b.Top, h.StaffTop)
		height := h.StaffHeight
		if height <= 0 {
			height = 4
		}
		b.Bottom = math.Max(b.Bottom, h.StaffTop+height)
	}
	out := make([]Box, 0, len(byBar))
	for i := 0; i < len(byBar)+len(lay.Hits); i++ {
		b, ok := byBar[i]
		if !ok {
			continue
		}
		b.A, b.B = round(b.X0*scale), round(b.X1*scale)
		b.C, b.D = round(b.Top*scale), round(b.Bottom*scale)
		out = append(out, *b)
	}
	return out
}

func round(v float64) float64 { return math.Round(v*10) / 10 }

// soundJSON is the whole thing as the player reads it.
func soundJSON(s *Sound) ([]byte, error) { return json.Marshal(s) }

// cue is a note as a page with a recording of itself needs one: when to light
// a notehead up, and for how long.
//
// The names are the ones a note already uses, so the player reads a cue and a
// note the same way and never has to know which it was handed.
type cue struct {
	Start float64 `json:"t"`
	Dur   float64 `json:"d"`
	Ref   int     `json:"r,omitempty"`
}

// playing is a Sound for a page that does not make its own.
type playing struct {
	Notes    []cue   `json:"notes"`
	Bars     []Bar   `json:"bars"`
	Boxes    []Box   `json:"boxes"`
	Duration float64 `json:"duration"`
	Tempo    int     `json:"tempo"`
}

// playingJSON is the same thing for a page that plays a recording.
//
// Everything a synthesiser would have needed is dropped. The pitch, the
// loudness, how much of the written length sounds and what family of
// instrument it is all answer "how should this be made to sound", and the
// recording has settled that already. What is left is the one thing the page
// still does with a note: show which head is sounding. A note that lights
// nothing up — it carries no number — goes too.
//
// On the ninth symphony this is the difference between ten megabytes of notes
// and about three.
func playingJSON(s *Sound) ([]byte, error) {
	p := playing{
		Notes:    make([]cue, 0, len(s.Notes)),
		Bars:     s.Bars,
		Boxes:    s.Boxes,
		Duration: s.Duration,
		Tempo:    s.Tempo,
	}
	for _, n := range s.Notes {
		if n.Ref == 0 {
			continue
		}
		p.Notes = append(p.Notes, cue{Start: n.Start, Dur: n.Dur, Ref: n.Ref})
	}
	return json.Marshal(p)
}
