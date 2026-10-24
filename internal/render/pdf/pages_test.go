/*
 * pages_test.go
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
