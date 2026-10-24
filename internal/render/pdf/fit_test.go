/*
 * fit_test.go
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
	"bytes"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// A system has to fit the page it is printed on.
//
// At a fixed type size a conductor's score does not: seventeen staves at the
// size a part is printed at make a system twice the height of A4, and the
// renderer prints what will not fit overflowing the sheet. La Maritza came
// out as fifty pages of a sixty-three bar song, each carrying four staves of
// the seventeen and dropping the rest.

// ensemble is a score of n staves and bars, with a note in every bar.
func ensemble(staves, bars int) *model.Score {
	s := &model.Score{
		Identity: model.Identity{Title: "Ensemble"},
		Tempo:    model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Name: "Instrument", Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for j := 0; j < bars; j++ {
			var els []model.Element
			for k := 0; k < 4; k++ {
				els = append(els, &model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 4},
				})
			}
			p.Measures = append(p.Measures, &model.Measure{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			})
		}
		s.Parts = append(s.Parts, p)
	}
	s.Normalise()
	return s
}

func paginated(staves int) engrave.Options {
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowPartNames = staves > 1
	eo.ShowMeasureNumbers = true
	return eo
}

// tallest returns the height of the system that needs the most room, as a
// fraction of the page it has to go on.
func pageFill(t *testing.T, s *model.Score, o Options, eo engrave.Options) float64 {
	t.Helper()
	lay := engrave.Engrave(s, eo)
	if len(lay.Systems) == 0 {
		t.Fatal("no systems")
	}
	return tallestSystem(lay) * o.StaffSpace / (o.PageHeight - o.MarginTop - o.MarginBottom)
}

func TestABigScoreIsSetSmallEnoughToFitThePage(t *testing.T) {
	s := ensemble(17, 40)

	// What it was: the default size, and a system that does not fit.
	o, eo := DefaultOptions(), paginated(17)
	eo.LineWidth = o.LineWidth(eo.Spacings)
	if fill := pageFill(t, s, o, eo); fill <= 1 {
		t.Fatalf("a system of 17 staves fills %.2f of the page at the default "+
			"size, so this proves nothing", fill)
	}

	o, eo = Fit(s, DefaultOptions(), paginated(17))
	fill := pageFill(t, s, o, eo)
	if fill > 1 {
		t.Errorf("after fitting, a system still fills %.2f of the page at "+
			"%.2fpt", fill, o.StaffSpace)
	}
	if o.StaffSpace < MinStaffSpace {
		t.Errorf("set at %.2fpt, below the smallest readable %.2fpt",
			o.StaffSpace, MinStaffSpace)
	}
	// And the line it is given has to follow the size it is set at, or the
	// music is engraved wider than the sheet it is printed on.
	if want := o.LineWidth(eo.Spacings); eo.LineWidth != want {
		t.Errorf("line width %.1f does not match the type size (%.1f)",
			eo.LineWidth, want)
	}
}

// Fitting only ever goes smaller. A page with room to spare is not a reason
// to blow a song up, and a size somebody asked for is one not to exceed.
func TestASmallScoreIsLeftAtTheSizeItWasAskedFor(t *testing.T) {
	for _, staves := range []int{1, 2, 4} {
		o, _ := Fit(ensemble(staves, 24), DefaultOptions(), paginated(staves))
		if o.StaffSpace != DefaultOptions().StaffSpace {
			t.Errorf("%d staves: set at %.2fpt, want the default %.2fpt",
				staves, o.StaffSpace, DefaultOptions().StaffSpace)
		}
	}
}

// Turning the paper over is the other way of making a big score fit, and the
// two have to agree: on its side the page is shorter, so the music is set
// smaller still, and wider, so more of it goes on a line.
func TestLandscapeIsFittedToTheShorterPage(t *testing.T) {
	// Ten staves, which both ways up the page can hold. Seventeen on A4 on
	// its side cannot be done at any readable size — the sheet is 210mm tall
	// and the system wants more — and that is the next test's business.
	const staves = 10
	s := ensemble(staves, 40)
	up, eoUp := Fit(s, DefaultOptions(), paginated(staves))
	side, eoSide := Fit(s, DefaultOptions().Landscape(), paginated(staves))

	if side.StaffSpace >= up.StaffSpace {
		t.Errorf("on its side the music is set at %.2fpt and upright at %.2fpt: "+
			"the shorter page needs the smaller type", side.StaffSpace, up.StaffSpace)
	}
	if eoSide.LineWidth <= eoUp.LineWidth {
		t.Errorf("on its side a line holds %.1f spaces and upright %.1f: "+
			"the wider page should hold more", eoSide.LineWidth, eoUp.LineWidth)
	}
	if fill := pageFill(t, s, side, eoSide); fill > 1 {
		t.Errorf("on its side a system still fills %.2f of the page", fill)
	}
}

// A score that will not fit even at the smallest readable type is printed too
// big rather than shrunk until nobody can read it. Saying so by printing it
// is clearer than silently setting a score in type the size of a full stop.
func TestAScoreTooBigForAnyTypeStopsAtTheSmallest(t *testing.T) {
	o, _ := Fit(ensemble(60, 8), DefaultOptions(), paginated(60))
	if o.StaffSpace != MinStaffSpace {
		t.Errorf("set at %.2fpt, want to stop at %.2fpt",
			o.StaffSpace, MinStaffSpace)
	}
}

// Every page carries the mascot, because the pages of a score get pulled
// apart and handed round a band: a mark on the first sheet alone says
// nothing about the one in front of you.
func TestEveryPageIsStamped(t *testing.T) {
	s := ensemble(4, 60)
	o, eo := Fit(s, DefaultOptions(), paginated(4))
	lay := engrave.Engrave(s, eo)

	var buf bytes.Buffer
	if err := Render(&buf, lay, o); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()

	// A page's resources name the image it draws, and that dictionary is not
	// compressed: one mention per page that uses it.
	pages := bytes.Count(out, []byte("/Type /Page")) - bytes.Count(out, []byte("/Type /Pages"))
	if pages < 2 {
		t.Fatalf("%d pages, so this proves nothing about every page", pages)
	}
	if n := bytes.Count(out, []byte("Imgilbert")); n != pages {
		t.Errorf("the mascot is named on %d of %d pages", n, pages)
	}
}
