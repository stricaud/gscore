/*
 * chordrow_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * A row of chord symbols does not sit on itself.
 *
 * A symbol is written from its own note's x and runs to the right of it, so
 * two symbols on neighbouring beats stand only as far apart as the beats do.
 * Nothing booked that room — chordWidth was computed for the outline above
 * the staff and for nothing else — and the names ran through each other
 * wherever the chords changed faster than about one a quarter note.
 *
 * Reported from a Guitar Pro file of Stairway: bar 4 has G/B on one beat and
 * Am on the next, and printed the two one through the other, which reads as
 * "GABm". The same file had A7sus4 running into the D after it over one
 * guitar while the identical pair over the other guitar came out clear — the
 * beats there happened to fall further apart. That is the tell: a collision
 * that depends on luck rather than on the chords is a collision nobody has
 * booked room against.
 *
 * Lyrics have claimed their width since they were written. These tests say
 * the chords do too, in the three places it can go wrong: along one bar,
 * across a barline, and under a squeeze.
 */

// chordAt hangs a symbol on an element.
func chordAt(el model.Element, root, quality string) model.Element {
	marksOf(el).Chord = &model.ChordSymbol{Root: root, Quality: quality}
	return el
}

// eighths builds one bar of eight eighth notes, so that chords can be put on
// whichever of them a test wants.
func eighths(names ...string) *model.Score {
	e := model.Duration{Num: 1, Den: 8}
	var els []model.Element
	for i := 0; i < 8; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: e,
		}
		var el model.Element = n
		if i < len(names) && names[i] != "" {
			root, quality := names[i][:1], names[i][1:]
			el = chordAt(el, root, quality)
		}
		els = append(els, el)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices:       []*model.Voice{{Elements: els}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

// tightestChordGap is the least air between one symbol and the next along a
// row, over every row of the layout. Negative means two of them overlap.
func tightestChordGap(lay *Layout) (float64, string, string) {
	rows := map[float64][]ChordPos{}
	for _, c := range lay.Chords {
		key := float64(int(c.Y*4+0.5)) / 4
		rows[key] = append(rows[key], c)
	}
	least, a, b := 1e9, "", ""
	for _, r := range rows {
		sort.Slice(r, func(i, j int) bool { return r[i].X < r[j].X })
		for i := 1; i < len(r); i++ {
			if gap := r[i].X - (r[i-1].X + r[i-1].Width); gap < least {
				least, a, b = gap, r[i-1].Name, r[i].Name
			}
		}
	}
	return least, a, b
}

func TestChordsOnNeighbouringBeatsDoNotOverlap(t *testing.T) {
	// Every eighth of the bar named, which is faster than any chart really
	// changes and so is the hardest case the spacing has to answer.
	sc := eighths("Gm7", "Am7", "Bb", "Cmaj7", "Dm7", "Eb", "F7", "Gm7")
	lay := Engrave(sc, DefaultOptions())
	if len(lay.Chords) != 8 {
		t.Fatalf("%d chord symbols, wanted 8", len(lay.Chords))
	}
	gap, a, b := tightestChordGap(lay)
	if gap < chordSymbolGap-0.01 {
		t.Errorf("%q and %q are %.2f apart, wanted at least %.2f",
			a, b, gap, chordSymbolGap)
	}
}

func TestAChordAtTheEndOfABarClearsTheNextBar(t *testing.T) {
	// The case neither bar can see on its own: a wide name on the last
	// eighth of one bar and a name on the downbeat of the next.
	sc := eighths("", "", "", "", "", "", "", "Cmaj7")
	m := sc.Parts[0].Measures[0]
	second := &model.Measure{
		Voices:       []*model.Voice{{}},
		RightBarline: model.BarSingle,
	}
	for i := 0; i < 8; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.StepB, Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 8},
		}
		var el model.Element = n
		if i == 0 {
			el = chordAt(el, "F", "m7b5")
		}
		second.Voices[0].Elements = append(second.Voices[0].Elements, el)
	}
	sc.Parts[0].Measures = []*model.Measure{m, second}

	lay := Engrave(sc, DefaultOptions())
	gap, a, b := tightestChordGap(lay)
	if gap < chordSymbolGap-0.01 {
		t.Errorf("across the barline %q and %q are %.2f apart, wanted at least %.2f",
			a, b, gap, chordSymbolGap)
	}
}

func TestChordsStayApartWhenTheSystemIsSqueezed(t *testing.T) {
	// The room has to be in the minimum and not only in the ideal. A system
	// squeezed to fit a line gives up its air first, and air is exactly what
	// keeps two names apart: if the gap lived only in the ideal, the first
	// crowded line would print them through each other again.
	sc := eighths("Gm7", "Am7", "Bb", "Cmaj7", "Dm7", "Eb", "F7", "Gm7")
	opt := DefaultOptions()
	opt.LineWidth = 30 // narrow enough that the bar cannot have its ideal
	lay := Engrave(sc, opt)
	gap, a, b := tightestChordGap(lay)
	if gap < 0 {
		t.Errorf("squeezed, %q and %q overlap by %.2f", a, b, -gap)
	}
}

func TestAPairWritesItsChordsOnce(t *testing.T) {
	// A guitar printed as notation above and tablature below is one player
	// reading one line written down twice, and the chords belong to the
	// line. Drawn from both staves they came out twice: two rows of names
	// and two rows of fingering diagrams, the second row between the staves.
	sc := eighths("Gm7", "", "", "", "Cmaj7", "", "", "")
	sc.Parts[0].Tab, _ = model.TuningNamed("guitar")
	if n := sc.PairWithNotation(); n != 1 {
		t.Fatalf("pairing added %d staves, wanted 1", n)
	}
	lay := Engrave(sc, DefaultOptions())
	if len(lay.Chords) != 2 {
		names := make([]string, len(lay.Chords))
		for i, c := range lay.Chords {
			names[i] = c.Name
		}
		t.Errorf("%d chord symbols drawn (%v), wanted the 2 that are written",
			len(lay.Chords), names)
	}
}
