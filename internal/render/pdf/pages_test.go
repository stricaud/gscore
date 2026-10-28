/*
 * pages_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package pdf

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"

	// So that a file named by GSCORE_SCORE can actually be read: the
	// importers register themselves, and nothing else in here imports them.
	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/gscore"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
	_ "github.com/stricaud/gscore/internal/format/nwc"
)

// How many sheets a song comes to.
//
// This is the thing somebody notices about a PDF before they notice anything
// else, and it was wrong: a pop song of a hundred-odd bars on one staff came
// out across five sheets of A4 with half of every sheet white. Three things
// did it, and each of them is kept honest here — the room over a system was
// counted rather than measured, the bar numbers somebody had written as
// chord symbols reserved a chord row on every line, and the type size was
// chosen by "will one system fit the page" when the question is how many
// pages there are.

// aSong is a hundred and nine bars of one staff, four to the bar and mostly
// eighths, with slurs over them and a dynamic here and there: a pop song as
// a lead sheet, which is the shape of music this got wrong.
//
// Built rather than kept as a file, because the song that showed the problem
// is somebody's copyright and this is the part of it that matters — how much
// music there is and how dense it is.
func aSong(bars int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	// A quarter, two eighths and two quarters: four beats in five notes,
	// which is about what a song of words comes to.
	pattern := []model.Duration{
		{Num: 1, Den: 4}, {Num: 1, Den: 8}, {Num: 1, Den: 8},
		{Num: 1, Den: 4}, {Num: 1, Den: 4},
	}
	for i := 0; i < bars; i++ {
		var els []model.Element
		for k, d := range pattern {
			n := &model.Note{
				Pitch: model.Pitch{Step: model.Step((i*3 + k) % 7), Octave: 4 + (i+k)%2},
				Dur:   d,
			}
			switch k {
			case 0:
				n.SlurStart = true
				if i%16 == 0 {
					n.Dynamic = model.Forte
				}
			case 3:
				n.SlurStop = true
			}
			els = append(els, n)
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Identity: model.Identity{Title: "A Song", Composer: "Nobody"},
		Tempo:    model.Tempo{BPM: 137, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
}

// sheets is how many pages a score comes to, decided exactly as printing
// decides it.
func sheets(t *testing.T, s *model.Score) (int, Options) {
	t.Helper()
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowMeasureNumbers = true
	o, eo := Fit(s, DefaultOptions(), eo)
	lay := engrave.Engrave(s, eo)
	return len((&renderer{lay: lay, opt: o}).pages()), o
}

func TestALongSongIsFittedToFewerSheets(t *testing.T) {
	s := aSong(109)
	n, o := sheets(t, s)

	// What it would have come to at the size that was asked for, which is
	// what it used to come to: the size was only ever reduced to make one
	// system fit the sheet, and a song's system always fits.
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowMeasureNumbers = true
	asked := pagesAt(s, DefaultOptions(), eo)

	t.Logf("109 bars on one staff: %d sheets at %.2fpt, %d at the %.2fpt asked for",
		n, o.StaffSpace, asked, DefaultOptions().StaffSpace)
	if n >= asked {
		t.Errorf("fitted to %d sheets at %.2fpt, which is no better than the "+
			"%d it comes to at %.2fpt", n, o.StaffSpace, asked,
			DefaultOptions().StaffSpace)
	}
	// And not by setting it at a size nobody can read.
	if o.StaffSpace < PartStaffSpace {
		t.Errorf("it was set at %.2fpt, smaller than a printed part (%.2fpt)",
			o.StaffSpace, PartStaffSpace)
	}
}

// Shorter music is not shrunk for nothing. The size only comes down to save
// a sheet, so a song that was always going to fit one page is set at the
// size it was asked for.
func TestAShortPieceKeepsItsSize(t *testing.T) {
	_, o := sheets(t, aSong(8))
	if want := DefaultOptions().StaffSpace; o.StaffSpace != want {
		t.Errorf("eight bars were set at %.2fpt rather than the %.2fpt asked "+
			"for: nothing was saved by shrinking them", o.StaffSpace, want)
	}
}

// The size chosen is the largest one that needs that many sheets, not the
// smallest one that happens to.
func TestItPicksTheLargestTypeThatSaves(t *testing.T) {
	s := aSong(109)
	n, o := sheets(t, s)

	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowMeasureNumbers = true
	bigger := o
	bigger.StaffSpace = o.StaffSpace + tightenStep
	if bigger.StaffSpace > DefaultOptions().StaffSpace {
		return // it never came down, so there is nothing to check
	}
	if got := pagesAt(s, bigger, eo); got <= n {
		t.Errorf("at %.2fpt it still comes to %d sheets, so %.2fpt was "+
			"smaller than it had to be", bigger.StaffSpace, got, o.StaffSpace)
	}
}

// And the whole thing really is written out.
func TestTheSongPrints(t *testing.T) {
	s := aSong(109)
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowMeasureNumbers = true
	o, eo := Fit(s, DefaultOptions(), eo)

	var buf bytes.Buffer
	if err := Render(&buf, engrave.Engrave(s, eo), o); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("that is not a PDF")
	}
	if buf.Len() < 10_000 {
		t.Errorf("the whole song came to %d bytes, which is too little to be "+
			"a hundred bars of music", buf.Len())
	}
}

// Run against a real score with GSCORE_SCORE=/path/to/let-it-go.abc, which
// is how this was found in the first place. A copyrighted song is not kept
// in the repository, so by default there is nothing to run against and this
// says so rather than passing quietly.
func TestARealScoreIfThereIsOne(t *testing.T) {
	path := os.Getenv("GSCORE_SCORE")
	if path == "" {
		t.Skip("set GSCORE_SCORE to a score file to check that one too")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	imp, err := format.ImporterFor(path)
	if err != nil {
		t.Fatal(err)
	}
	// A partial read is still a read: that file is full of bar numbers
	// written as chord symbols, and saying so is what the importer does.
	s, _ := imp.Import(f)
	if s == nil {
		t.Fatalf("%s could not be read at all", path)
	}

	bars := 0
	for _, p := range s.Parts {
		if len(p.Measures) > bars {
			bars = len(p.Measures)
		}
	}
	n, o := sheets(t, s)
	t.Logf("%s: %d bars, %d staves, %d sheets at %.2fpt",
		strings.TrimSpace(path), bars, len(s.Parts), n, o.StaffSpace)
	if len(s.Parts) == 1 && bars >= 100 && bars <= 130 && n > 2 {
		t.Errorf("%d bars on one staff came to %d sheets", bars, n)
	}
}

// Splitting a system that will not fit the sheet.
//
// Seventeen staves on A4 on its side cannot be set at any readable size: the
// system needs 604pt and the page gives 503. It used to be printed anyway,
// with the bottom staves sliced off by the paper's edge — the conductor's
// score came out missing its percussion, silently. Now the page is divided
// between two staves and the system goes on overleaf.

// bandsOf is how the renderer will cut this drawing into pages.
func bandsOf(t *testing.T, s *model.Score, o Options) ([]band, *engrave.Layout, Options) {
	t.Helper()
	eo := engrave.DefaultOptions()
	eo.Mode = engrave.Paginated
	eo.ShowPartNames = len(s.Parts) > 1
	o, eo = Fit(s, o, eo)
	lay := engrave.Engrave(s, eo)
	return (&renderer{lay: lay, opt: o}).pages(), lay, o
}

// staffSpans lists every staff drawn, by the range of the drawing it covers.
func staffSpans(lay *engrave.Layout) [][2]float64 {
	seen := map[float64]bool{}
	var out [][2]float64
	for _, h := range lay.Hits {
		if seen[h.StaffTop] {
			continue
		}
		seen[h.StaffTop] = true
		height := h.StaffHeight
		if height <= 0 {
			height = 4
		}
		out = append(out, [2]float64{h.StaffTop, h.StaffTop + height})
	}
	return out
}

func TestNoStaffIsCutInHalfByAPageBreak(t *testing.T) {
	s := ensemble(17, 8)
	bands, lay, _ := bandsOf(t, s, DefaultOptions().Landscape())
	if len(bands) < 2 {
		t.Fatalf("%d pages, so nothing was divided", len(bands))
	}
	for _, span := range staffSpans(lay) {
		for i, b := range bands[1:] {
			// A page starts at the previous band's end; a staff that
			// straddles that line is a staff printed in two halves.
			if b.top > span[0] && b.top < span[1] {
				t.Errorf("page %d begins at %.2f, inside a staff running "+
					"%.2f..%.2f", i+2, b.top, span[0], span[1])
			}
		}
	}
}

// And the staves are divided evenly. Ten that have to go on two pages read
// better as five and five than as eight and two: the page is being turned
// either way.
func TestADividedSystemIsDividedEvenly(t *testing.T) {
	s := ensemble(17, 4) // one system, too tall for the sheet on its side
	bands, lay, o := bandsOf(t, s, DefaultOptions().Landscape())
	if len(bands) < 2 {
		t.Skipf("it fitted at %.2fpt, so there was nothing to divide", o.StaffSpace)
	}

	// How many staves landed on each page.
	counts := make([]int, len(bands))
	for _, span := range staffSpans(lay) {
		for i, b := range bands {
			if span[0] >= b.top && span[0] < b.bottom+1 {
				counts[i]++
				break
			}
		}
	}
	lo, hi := counts[0], counts[0]
	for _, n := range counts {
		if n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	if total := 17; lo+hi != total && len(bands) == 2 {
		t.Errorf("%d staves landed on %d pages: %v", total, len(bands), counts)
	}
	// Even to within one staff, which is as even as an odd number gets.
	if hi-lo > 1 {
		t.Errorf("the staves came out %v, which is not an even division", counts)
	}
}

// A score that fits is not divided. Cutting a system in two is the answer to
// an impossible sheet and must not become the ordinary case.
func TestAScoreThatFitsIsNotDivided(t *testing.T) {
	for _, staves := range []int{1, 4, 10} {
		s := ensemble(staves, 24)
		bands, lay, o := bandsOf(t, s, DefaultOptions())
		for _, span := range staffSpans(lay) {
			for _, b := range bands[1:] {
				if b.top > span[0] && b.top < span[1] {
					t.Errorf("%d staves at %.2fpt: a page begins inside a staff",
						staves, o.StaffSpace)
				}
			}
		}
		// Every system whole, which is what "not divided" means.
		for _, sys := range lay.Systems {
			cut := false
			for _, b := range bands[1:] {
				if b.top > sys.Top+0.01 && b.top < sys.Bottom-0.01 {
					cut = true
				}
			}
			if cut {
				t.Errorf("%d staves: a system was divided although it fits", staves)
			}
		}
	}
}

// A part is set for a player on a stand, not for the conductor. Printed with
// the score's own settings it came out at the size seventeen staves need,
// which for one staff is unreadable.
func TestAPartIsSetLargerThanTheScore(t *testing.T) {
	full := ensemble(17, 16)
	_, _, scoreOpt := bandsOf(t, full, DefaultOptions())

	one := &model.Score{
		Identity: full.Identity,
		Tempo:    full.Tempo,
		Parts:    []*model.Part{full.Parts[0]},
	}
	_, _, partOpt := bandsOf(t, one, DefaultOptions())

	if partOpt.StaffSpace <= scoreOpt.StaffSpace {
		t.Errorf("the part is set at %.2fpt and the score at %.2fpt: a part "+
			"should be the larger of the two", partOpt.StaffSpace, scoreOpt.StaffSpace)
	}
	if partOpt.StaffSpace < PartStaffSpace {
		t.Errorf("a one-staff part came out at %.2fpt", partOpt.StaffSpace)
	}
}

// A movement begins a page, whether or not the page before it is full.
//
// It is the one cut the renderer does not get to choose. A section break is
// where the music starts afresh, and run on from the bar before it the
// Presto of the Moonlight sonata began halfway down a page of the
// Allegretto — in another key, another meter and another tempo.
func TestAMovementBeginsAPage(t *testing.T) {
	lay := &engrave.Layout{
		Width: 120, Height: 90,
		Systems: []engrave.SystemPos{
			{Top: 0, Staff: 2, Bottom: 20, First: 0, Last: 3},
			{Top: 20, Staff: 22, Bottom: 40, First: 4, Last: 7},
			// A new movement, a long way from the bottom of the page.
			{Top: 40, Staff: 42, Bottom: 60, First: 8, Last: 11, StartsSection: true},
			{Top: 60, Staff: 62, Bottom: 90, First: 12, Last: 15},
		},
	}
	o := DefaultOptions()
	// A page with room for four of these systems, so nothing but the break
	// can be what divides them.
	o.StaffSpace = 1
	o.MarginTop, o.MarginBottom = 0, 0
	o.PageHeight = 100

	got := (&renderer{lay: lay, opt: o}).pages()
	if len(got) != 2 {
		t.Fatalf("%d pages, want 2: the movement did not begin one", len(got))
	}
	if got[1].top != 40 {
		t.Errorf("the second page begins at %v, want 40 — where the movement does",
			got[1].top)
	}
	if got[0].top != 0 {
		t.Errorf("the first page begins at %v", got[0].top)
	}
}

func TestWithNoMovementsThePageIsFilled(t *testing.T) {
	// The same drawing with the break taken out: four systems that fit, so
	// one page.
	lay := &engrave.Layout{
		Width: 120, Height: 90,
		Systems: []engrave.SystemPos{
			{Top: 0, Staff: 2, Bottom: 20, First: 0, Last: 3},
			{Top: 20, Staff: 22, Bottom: 40, First: 4, Last: 7},
			{Top: 40, Staff: 42, Bottom: 60, First: 8, Last: 11},
			{Top: 60, Staff: 62, Bottom: 90, First: 12, Last: 15},
		},
	}
	o := DefaultOptions()
	o.StaffSpace = 1
	o.MarginTop, o.MarginBottom = 0, 0
	o.PageHeight = 100

	if got := (&renderer{lay: lay, opt: o}).pages(); len(got) != 1 {
		t.Errorf("%d pages for a drawing that fits on one", len(got))
	}
}
