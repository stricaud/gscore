/*
 * bands_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package html

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// A long score, so that there is more than one system to cut between.
func longTune(bars int) *model.Score {
	p := &model.Part{
		Name: "Flute", Clef: model.Treble,
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	for i := 0; i < bars; i++ {
		var els []model.Element
		for k := 0; k < 4; k++ {
			els = append(els, &model.Note{
				Pitch: model.Pitch{Step: model.Step((i + k) % 7), Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 4},
			})
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		})
	}
	s := &model.Score{
		Identity: model.Identity{Title: "A Long Tune", Composer: "Somebody"},
		Tempo:    model.Tempo{BPM: 96, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
	s.Normalise()
	return s
}

func names(i int) string { return fmt.Sprintf("piece.%03d.svg", i) }

// The pieces divide the drawing between them and leave nothing out.
//
// Anything in no piece is drawn in no piece: it is simply not on the page.
// Cut to the systems alone, two things fell through — the title block above
// the first system, and whatever is written in the gap between two systems.
func TestBandsDivideTheWholeDrawing(t *testing.T) {
	s := longTune(60)
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	if len(lay.Systems) < 3 {
		t.Fatalf("a test of cutting between systems needs several; got %d",
			len(lay.Systems))
	}
	const scale = 8

	for _, perBand := range []int{1, 2, 5} {
		got := Bands(lay, scale, perBand, names)
		if len(got) == 0 {
			t.Fatalf("%d systems a piece: no pieces at all", perBand)
		}
		if got[0].Y0 != 0 {
			t.Errorf("%d systems a piece: the first starts at %g, not the top "+
				"of the drawing — the title is written above the first system",
				perBand, got[0].Y0)
		}
		want := round(lay.Height * scale)
		if last := got[len(got)-1].Y1; last < want {
			t.Errorf("%d systems a piece: the last ends at %g, short of the "+
				"drawing's %g", perBand, last, want)
		}
		for i := 1; i < len(got); i++ {
			if gap := got[i].Y0 - got[i-1].Y1; math.Abs(gap) > 0.01 {
				t.Errorf("%d systems a piece: a gap of %g between pieces %d "+
					"and %d, where anything drawn is drawn in neither",
					perBand, gap, i-1, i)
			}
		}
		// And the bars they name run through without a hole.
		if got[0].First != 0 {
			t.Errorf("%d systems a piece: the first piece starts at bar %d",
				perBand, got[0].First)
		}
		for i := 1; i < len(got); i++ {
			if got[i].First > got[i-1].Last+1 {
				t.Errorf("%d systems a piece: bars %d..%d are in no piece",
					perBand, got[i-1].Last+1, got[i].First-1)
			}
		}
		if last := got[len(got)-1].Last; last != s.MeasureCount()-1 {
			t.Errorf("%d systems a piece: the last bar named is %d of %d",
				perBand, last, s.MeasureCount()-1)
		}
		if got[0].URL != "piece.000.svg" {
			t.Errorf("the caller's naming was not used: %q", got[0].URL)
		}
	}
}

// Asked for nothing, it does nothing, rather than dividing by zero.
func TestBandsOfNothing(t *testing.T) {
	lay := engrave.Engrave(longTune(4), engrave.DefaultOptions())
	if got := Bands(lay, 8, 0, names); got != nil {
		t.Errorf("no systems to a piece gave %d pieces", len(got))
	}
	if got := Bands(nil, 8, 2, names); got != nil {
		t.Errorf("no layout gave %d pieces", len(got))
	}
}

// A window asked of a band is the same stretch, in the engraver's units.
func TestABandsWindowIsItsOwnStretch(t *testing.T) {
	b := Band{Y0: 800, Y1: 1600}
	x0, y0, x1, y1 := b.Window(140, 8)
	if x0 != 0 || x1 != 140 {
		t.Errorf("the window is %g..%g across, not the drawing's full width", x0, x1)
	}
	if y0 != 100 || y1 != 200 {
		t.Errorf("the window is %g..%g down; the band is 800..1600 at 8 to the "+
			"space, so 100..200", y0, y1)
	}
}

var glyph = regexp.MustCompile(`<text class="g"`)

// A banded page carries the drawing the music goes into, and says where the
// music is — but none of the music.
func TestABandedPageCarriesTheShellAndNotTheMusic(t *testing.T) {
	s := longTune(60)
	lay := engrave.Engrave(s, engrave.DefaultOptions())
	opt := DefaultOptions()
	opt.FontURL = "font.otf"
	list := Bands(lay, opt.SVG.StaffSpace, 2, names)

	// First the same page with the music in it, to compare against.
	var whole bytes.Buffer
	if err := Render(&whole, lay, s, nil, opt); err != nil {
		t.Fatal(err)
	}
	opt.Bands = list
	var banded bytes.Buffer
	if err := Render(&banded, lay, s, nil, opt); err != nil {
		t.Fatal(err)
	}

	if n := len(glyph.FindAllString(whole.String(), -1)); n == 0 {
		t.Fatal("the ordinary page has no music in it, so this proves nothing")
	}
	if n := len(glyph.FindAllString(banded.String(), -1)); n != 0 {
		t.Errorf("the banded page carries %d glyphs; it should carry none", n)
	}
	if banded.Len() >= whole.Len() {
		t.Errorf("the banded page is %d bytes against the whole one's %d",
			banded.Len(), whole.Len())
	}

	// The drawing is still the size of the score, so the scrollbar is honest
	// and a coordinate means what the bar boxes say it means.
	size := regexp.MustCompile(`<svg[^>]*width="([\d.]+)" height="([\d.]+)"`)
	a := size.FindStringSubmatch(whole.String())
	b := size.FindStringSubmatch(banded.String())
	if a == nil || b == nil {
		t.Fatal("no <svg> to measure")
	}
	if a[1] != b[1] || a[2] != b[2] {
		t.Errorf("the banded drawing is %s x %s, the whole one %s x %s",
			b[1], b[2], a[1], a[2])
	}

	// And it says where the pieces are.
	text := banded.String()
	if !strings.Contains(text, `id="gscore-bands"`) {
		t.Fatal("the page does not say where the drawing is")
	}
	raw := text[strings.Index(text, `id="gscore-bands"`):]
	raw = raw[strings.Index(raw, ">")+1 : strings.Index(raw, "</script>")]
	var back []Band
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &back); err != nil {
		t.Fatalf("the list of pieces is not readable: %v", err)
	}
	if len(back) != len(list) {
		t.Errorf("%d pieces listed, %d written", len(back), len(list))
	}
	if len(back) > 0 && back[0].URL != list[0].URL {
		t.Errorf("the first piece is listed as %q, not %q", back[0].URL, list[0].URL)
	}
}
