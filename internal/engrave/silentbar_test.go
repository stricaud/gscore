/*
 * silentbar_test.go
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
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

/*
 * A sign standing in a bar is not music, and the bar has to go on knowing it.
 *
 * A symbol put into a voice on its own takes no time: it is a mark somebody
 * drew and placed, like the ukulele tuning chart over bar one of a song
 * sheet. It is positioned the way a note is, so that it has somewhere to
 * stand against the staff — and being positioned like a note was enough for
 * two other parts of the engraver to treat it as one.
 *
 * It was given a quarter note's stem, drawn from the middle line down past
 * the bottom of the staff with no notehead on the end of it: a stroke through
 * the bar growing out of nothing. And it was counted as the bar's content, so
 * the rest beside it was no longer the whole of the bar — which cost that
 * rest both halves of what a whole-bar rest gets. It was not centred, and it
 * was not written as a semibreve but at whatever length it literally held,
 * dot and all.
 *
 * One bar of a real score showed both at once, and the bars either side of it
 * — the same silence, no symbol — came out right. Reported as "why do I have
 * a dot on my rest and a line in the first measure".
 */

// silentBars is a part of bars holding nothing but a rest apiece, in the
// meter given, with each rest as long as whatever `rest` says.
func silentBars(beats, value int, rest model.Duration, n int) *model.Score {
	p := &model.Part{
		Clef: model.Treble,
		Time: model.TimeSignature{Beats: beats, BeatValue: value},
	}
	for i := 0; i < n; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Rest{Dur: rest},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

// markIn puts a symbol into a bar's voice, as inserting one unattached does.
func markIn(sc *model.Score, bar int) {
	v := sc.Parts[0].Measures[bar].Voices[0]
	v.Elements = append(v.Elements, &model.SymbolElement{
		Symbol: model.SymbolMark{Name: "device"},
	})
}

// restsDrawn is every rest glyph in a layout.
func restsDrawn(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if strings.HasPrefix(g.Name, "rest") {
			out = append(out, g)
		}
	}
	return out
}

// dotsDrawn is every augmentation dot.
func dotsDrawn(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if strings.Contains(strings.ToLower(g.Name), "dot") {
			out = append(out, g)
		}
	}
	return out
}

func TestASilentBarWithAMarkInItIsDrawnAsASilentBar(t *testing.T) {
	// The rest is longer than the bar, which is how the reported file
	// stood: bars of six-four silence left behind by a change to four-four.
	// Whatever is in the model, the sign for a silent bar is a semibreve
	// rest with no dot on it, and that is what every bar here must print.
	sc := silentBars(4, 4, model.Duration{Num: 3, Den: 2}, 3)
	markIn(sc, 0)
	lay := Engrave(sc, DefaultOptions())

	rests := restsDrawn(lay)
	if len(rests) != 3 {
		t.Fatalf("%d rests drawn, wanted 3", len(rests))
	}
	for i, r := range rests {
		if r.Name != "restWhole" {
			t.Errorf("bar %d prints %q, wanted a semibreve rest", i+1, r.Name)
		}
	}
	if d := dotsDrawn(lay); len(d) != 0 {
		t.Errorf("%d augmentation dots drawn, wanted none", len(d))
	}
}

func TestTheMarkDoesNotMoveTheRestOffCentre(t *testing.T) {
	// The other half of the same test: a bar of silence puts its rest in
	// the middle of the bar, which is where the eye looks for it and where
	// a click has to find it. Counted as content, the mark left the rest
	// hard against the opening barline with the whole bar empty to the
	// right of it.
	//
	// Centred in its own bar, which is the claim — not standing at the
	// same x as the rest of a bar with no mark in it. A mark takes a column
	// like anything else placed along a bar, so the bar it is in is a
	// little wider, and its middle is a little further along.
	sc := silentBars(4, 4, model.Duration{Num: 1, Den: 1}, 3)
	markIn(sc, 1) // the second bar, which has no clef in front of it
	lay := Engrave(sc, DefaultOptions())

	rests := restsDrawn(lay)
	if len(rests) != 3 {
		t.Fatalf("%d rests drawn, wanted 3", len(rests))
	}
	var bar *Hit
	for i := range lay.Hits {
		if lay.Hits[i].Measure == 1 {
			bar = &lay.Hits[i]
		}
	}
	if bar == nil {
		t.Fatal("the bar is not in the layout")
	}
	w := smufl.Advance(smufl.RestGlyph(model.Whole))
	got := rests[1].X + w/2
	want := (bar.XMin + bar.XMax) / 2
	if math.Abs(got-want) > 0.5 {
		t.Errorf("the rest is centred on x=%.2f in a bar running %.2f..%.2f, "+
			"whose middle is %.2f", got, bar.XMin, bar.XMax, want)
	}
}

func TestAMarkStandingInABarGrowsNoStem(t *testing.T) {
	// A stem is drawn from a notehead, and a sign has none. Drawn anyway,
	// it was a vertical line from the middle of the staff to a space and a
	// half below it, attached to nothing.
	sc := silentBars(4, 4, model.Duration{Num: 1, Den: 1}, 1)
	markIn(sc, 0)
	lay := Engrave(sc, DefaultOptions())

	for _, l := range lay.Lines {
		if l.X1 != l.X2 {
			continue // staff lines and the like run across, not down
		}
		known := false
		for _, b := range lay.Barlines {
			if math.Abs(b.X-l.X1) < 0.01 {
				known = true
			}
		}
		if !known {
			t.Errorf("a vertical line at x=%.2f runs y=%.2f..%.2f and is "+
				"neither a barline nor anybody's stem", l.X1, l.Y1, l.Y2)
		}
	}
}

func TestTwoRestsInABarAreStillTwoRests(t *testing.T) {
	// The guard the whole thing turns on. Ignoring what takes no time must
	// not become ignoring a second thing that does: a bar of two half rests
	// is two rests of their own length, not one silent bar, and neither of
	// them is centred or redrawn as a semibreve.
	h := model.Duration{Num: 1, Den: 2}
	sc := silentBars(4, 4, h, 1)
	v := sc.Parts[0].Measures[0].Voices[0]
	v.Elements = append(v.Elements, &model.Rest{Dur: h})

	rests := restsDrawn(Engrave(sc, DefaultOptions()))
	if len(rests) != 2 {
		t.Fatalf("%d rests drawn, wanted 2", len(rests))
	}
	for _, r := range rests {
		if r.Name != "restHalf" {
			t.Errorf("a half rest printed as %q", r.Name)
		}
	}
}
