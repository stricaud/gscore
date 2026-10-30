/*
 * room_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	// Nothing over the staff means nothing, bar numbers included. They are
	// on by default and they are written above the first measure of every
	// system after the first, so leaving them on would be asking a system
	// with a number over it to reserve no room for the number.
	opt.ShowMeasureNumbers = false
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

// stacked is two staves written the way bar 55 of the Rossini is: marks
// stacked under the upper one and notes climbing above the lower one, so
// that the gap between them has to hold both.
//
// The upper staff's notes are high, so their stems point down and the slur
// and the marcato marks are drawn below the staff, under which go a dynamic
// and a hairpin. The lower staff's notes are above its top line, with
// marcato marks above them again. Nothing in the gap is where the count
// expects it: the count knows about the dynamic and the hairpin and has
// never heard of the other four.
func stacked(bars int) *model.Score {
	upper := &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	lower := &model.Part{Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	q := model.Duration{Num: 1, Den: 4}
	for i := 0; i < bars; i++ {
		var up, down []model.Element
		for k := 0; k < 4; k++ {
			hi := &model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 5}, Dur: q,
			}
			hi.Articulations = []model.Articulation{model.Marcato}
			if k == 0 {
				hi.Dynamic = model.Forte
				hi.SlurStart = true
				hi.Hairpin = &model.Hairpin{Kind: model.Diminuendo, Start: true}
			}
			if k == 3 {
				hi.SlurStop = true
				hi.Hairpin = &model.Hairpin{Kind: model.Diminuendo, Stop: true}
			}
			up = append(up, hi)

			// Above the staff, on ledger lines, with a mark over it and
			// a slur arching over that — which is the other half of bar
			// 55, and is no more counted than the marks opposite it.
			lo := &model.Note{
				Pitch: model.Pitch{Step: model.StepA, Octave: 5}, Dur: q,
			}
			lo.Articulations = []model.Articulation{model.Marcato}
			if k == 0 {
				lo.SlurStart = true
			}
			if k == 3 {
				lo.SlurStop = true
			}
			down = append(down, lo)
		}
		upper.Measures = append(upper.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: up}}, RightBarline: model.BarSingle,
		})
		lower.Measures = append(lower.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: down}}, RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{upper, lower},
	}
}

// What is written under one staff clears the notes of the next.
//
// The gap between two staves is reserved before either is drawn, from a
// count of what goes into it, and the count knows the marks it has names
// for and nothing else. A hairpin that had to clear a row of marcato marks
// and a slur, neither of them counted, went on down past the gap and came
// to rest across the first note of the staff below — which is bar 55 of the
// Rossini, a diminuendo lying over a saxophone's top line.
func TestAHairpinClearsTheStaffUnderIt(t *testing.T) {
	lay := Engrave(stacked(8), DefaultOptions())
	if len(lay.Hairpins) == 0 {
		t.Fatal("no hairpins were drawn, so nothing was tested")
	}
	// Where the lower staff's noteheads are, which is above its own lines.
	heads := map[int]float64{}
	for _, h := range lay.Hits {
		if h.Part != 1 {
			continue
		}
		for _, el := range h.Elements {
			if y, ok := heads[h.Measure]; !ok || el.Y < y {
				heads[h.Measure] = el.Y
			}
		}
	}
	for _, h := range lay.Hairpins {
		if h.Part != 0 {
			continue
		}
		head, ok := heads[h.Measure]
		if !ok {
			continue
		}
		// The top of the notehead: it is drawn centred on its step and is
		// one space tall. What is asked for is not that the two miss each
		// other by a hair but that they are a staff's clearance apart —
		// the air this engraver leaves between one staff's music and the
		// next one's, which is what makes two staves read as two. Before
		// the gaps were measured this came to a fifth of that, and there
		// was nothing between the hairpin and the marks over the notes but
		// the marks' own luck.
		if clear := (head - 0.5) - (h.Y + h.Height); clear < staffClearance-0.05 {
			t.Errorf("the hairpin in bar %d comes within %.2f spaces of the "+
				"note under it, and a staff's clearance is %.2f",
				h.Measure+1, clear, staffClearance)
		}
	}
}

// The second pass settles: a third one would change nothing.
//
// The gaps between a system's staves are measured on one pass and used on
// the next, and that is only sound if the measurement does not move when
// the gaps do. It does not, because a staff's outline is built from its own
// ink and placed against its own lines — but "because" is an argument and
// this is the check. If it ever stopped being true the drawing would walk:
// each pass would find the gaps it had just widened and widen them again.
func TestTheMeasuredGapsSettle(t *testing.T) {
	opt := DefaultOptions()
	opt.Mode = Paginated
	opt.LineWidth = 70
	s := stacked(24)

	first := engraveWith(s, opt, nil)
	second := engraveWith(s, opt, roomAbove(first))
	third := engraveWith(s, opt, roomAbove(second))

	if len(second.gaps) == 0 {
		t.Fatal("no gaps were measured, so nothing was tested")
	}
	// The gaps, and only the gaps. How far a system reaches above and below
	// itself does go on moving from pass to pass — the drawing is a little
	// shorter each time and the engraver stops at two, which it did before
	// any of this — but that is a height settling downwards, not a gap
	// feeding on itself.
	for bar, want := range second.gaps {
		got := third.gaps[bar]
		if len(got) != len(want) {
			t.Errorf("the system at bar %d has %d gaps measured again and "+
				"%d before", bar+1, len(got), len(want))
			continue
		}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 0.001 {
				t.Errorf("the gap under staff %d of the system at bar %d "+
					"measured %.3f and then %.3f", i+1, bar+1, want[i], got[i])
			}
		}
	}
	// And the first pass, which had only the count to go on, wanted less
	// than the measurement did — otherwise this score is not the
	// stacked one it is supposed to be and proves nothing.
	if first.Height >= second.Height {
		t.Errorf("the counted pass is %.2f spaces tall and the measured one "+
			"%.2f: the count was not short", first.Height, second.Height)
	}
}
