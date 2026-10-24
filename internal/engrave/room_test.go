/*
 * room_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// The room over a system is counted before it is drawn and measured after.
// These check the measurement is used, and that it is honest: a system that
// claims less room than its ink takes is a system whose slurs are printed
// over the one above it, or cut off by a page break.

// plainSong is a long single-staff tune with nothing written over it: no
// chord symbols, no voltas, no octave lines. The room above it should come
// to almost nothing, and before the measuring pass it came to a great deal.
func plainSong(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		var els []model.Element
		for k := 0; k < 8; k++ {
			els = append(els, &model.Note{
				// Inside the staff, so nothing reaches above it at all.
				Pitch: model.Pitch{Step: model.Step((i + k) % 5), Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 8},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func TestASystemWithNothingOverItReservesNothing(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 70
	lay := Engrave(plainSong(24), opt)

	if len(lay.Systems) < 3 {
		t.Fatalf("%d systems, which proves nothing", len(lay.Systems))
	}
	for i, sys := range lay.Systems[1:] {
		if room := sys.Staff - sys.Top; room > 2.5 {
			t.Errorf("system %d keeps %.2f spaces over a staff with nothing "+
				"written above it", i+1, room)
		}
	}
}

// And measuring it makes the drawing shorter, which is the whole point: it
// is pages.
func TestMeasuringTheRoomShortensTheDrawing(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 70
	s := plainSong(24)

	counted := engraveWith(s, opt, nil)
	measured := engraveWith(s, opt, roomAbove(counted))
	if measured.Height >= counted.Height {
		t.Errorf("measured %.1f spaces tall, counted %.1f: no shorter",
			measured.Height, counted.Height)
	}
	if len(measured.Systems) != len(counted.Systems) {
		t.Fatalf("the second pass drew %d systems and the first %d: the two "+
			"must agree or the measurements belong to other systems",
			len(measured.Systems), len(counted.Systems))
	}
	for i := range counted.Systems {
		if measured.Systems[i].First != counted.Systems[i].First {
			t.Fatalf("system %d starts at bar %d measured and %d counted",
				i, measured.Systems[i].First, counted.Systems[i].First)
		}
	}
}

// A page is cut between systems, and nothing may be lying across the cut.
//
// This is the property that matters rather than "everything is inside its
// system": where exactly a slur under one staff stops belonging to it and
// starts belonging to the gap is a matter of opinion, but a stroke of ink
// with the scissors through it is not. The renderer breaks a page at the
// top of the system that starts the next one, so that line has to fall in
// clear air all the way across.
func TestNothingLiesAcrossAPageBreak(t *testing.T) {
	for _, s := range []struct {
		name  string
		score *model.Score
	}{
		{"a plain tune", plainSong(24)},
		{"one with marks under it", underlined(24)},
	} {
		opt := DefaultOptions()
		opt.Mode = Paginated
		opt.LineWidth = 60
		lay := Engrave(s.score, opt)
		if len(lay.Systems) < 3 {
			t.Fatalf("%s: %d systems, which proves nothing", s.name, len(lay.Systems))
		}

		for i := 1; i < len(lay.Systems); i++ {
			cut := lay.Systems[i].Top
			across := func(top, bottom float64, what string) {
				// Touching the line is not crossing it: a system's own top
				// is where its highest ink begins.
				if top < cut-0.01 && bottom > cut+0.01 {
					t.Errorf("%s: a page break before system %d cuts at %.2f, "+
						"through %s running %.2f..%.2f",
						s.name, i, cut, what, top, bottom)
				}
			}
			for _, c := range lay.Curves {
				lo, hi := quadExtent(c.Y1, c.CY, c.Y2)
				across(lo, hi, "a slur")
			}
			for _, l := range lay.Lines {
				// Barlines and staff lines belong to one system by
				// construction; it is the sloping and spanning ones that
				// could reach.
				across(math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2), "a line")
			}
			for _, g := range lay.Glyphs {
				up, down := glyphReach(g.Name)
				across(g.Y-up, g.Y+down, g.Name)
			}
		}
	}
}
