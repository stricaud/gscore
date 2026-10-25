/*
 * ledgers_test.go
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
	"sort"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// The ledger lines of neighbouring notes have to be told apart.
//
// Each is drawn from a little before its notehead to a little after, and the
// closest two notes may be set is two spaces — which left two hundredths of
// a space between one line and the next. At reading size that is nothing, so
// a run climbing above the staff came out as an unbroken grid and read as
// staff lines that are not there.

// ledgerRows is every ledger line in a layout, gathered by the row it is on.
//
// A ledger line is told from a staff line by its length: a staff line runs
// the width of the system and a ledger line is about a notehead wide.
func ledgerRows(lay *Layout) map[int][][2]float64 {
	rows := map[int][][2]float64{}
	for _, l := range lay.Lines {
		if l.Y1 != l.Y2 || l.X2-l.X1 > 4 {
			continue
		}
		k := int(math.Round(l.Y1 * 100))
		rows[k] = append(rows[k], [2]float64{l.X1, l.X2})
	}
	for _, r := range rows {
		sort.Slice(r, func(i, j int) bool { return r[i][0] < r[j][0] })
	}
	return rows
}

// climb is a run of notes rising off the top of the staff, which is the
// shape that showed the problem.
func climb(n int, dur model.Duration) *model.Score {
	v := &model.Voice{}
	for i := 0; i < n; i++ {
		v.Elements = append(v.Elements, &model.Note{
			Pitch: model.PitchFromDiatonicIndex(40 + i),
			Dur:   dur,
		})
	}
	return &model.Score{Parts: []*model.Part{{
		Time:     model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{v}}},
	}}}
}

func TestLedgerLinesOfNeighbouringNotesDoNotJoin(t *testing.T) {
	lay := Engrave(climb(12, model.Duration{Num: 1, Den: 32}), DefaultOptions())

	rows := ledgerRows(lay)
	if len(rows) == 0 {
		t.Fatal("the run drew no ledger lines at all")
	}
	crowded := 0
	for y, r := range rows {
		for i := 1; i < len(r); i++ {
			gap := r[i][0] - r[i-1][1]
			if gap < ledgerGap-0.001 {
				t.Errorf("on the row at y=%.2f two ledger lines are %.3f "+
					"spaces apart, which reads as one line", float64(y)/100, gap)
			}
			crowded++
		}
	}
	if crowded < 10 {
		t.Fatalf("only %d pairs of ledger lines were close enough to be "+
			"worth checking; this test is not testing anything", crowded)
	}
}

// Pulling them apart must never pull one off the note it belongs to: a
// notehead sitting on a line it overhangs is worse than two lines joining.
func TestALedgerLineAlwaysSpansItsNotehead(t *testing.T) {
	lay := Engrave(climb(12, model.Duration{Num: 1, Den: 32}), DefaultOptions())

	head := smufl.Advance("noteheadBlack")
	for y, r := range ledgerRows(lay) {
		for _, l := range r {
			if w := l[1] - l[0]; w < head {
				t.Errorf("a ledger line at y=%.2f is %.3f wide and the "+
					"notehead on it is %.3f: the note hangs off its own line",
					float64(y)/100, w, head)
			}
		}
	}
}

// A note with room around it keeps the whole of its line. The separation is
// for crowding and nothing else, so ordinary music is left exactly as it was.
func TestAnIsolatedNoteKeepsItsWholeLedgerLine(t *testing.T) {
	o := DefaultOptions()
	// A quarter, so the notehead is the one the width below names.
	lay := Engrave(climb(1, model.Duration{Num: 1, Den: 4}), o)

	want := smufl.Advance("noteheadBlack") + 2*o.Defaults.LegerLineExtension
	for y, r := range ledgerRows(lay) {
		for _, l := range r {
			if w := l[1] - l[0]; math.Abs(w-want) > 0.001 {
				t.Errorf("a lone note's ledger line at y=%.2f is %.3f wide, "+
					"want %.3f: nothing was crowding it",
					float64(y)/100, w, want)
			}
		}
	}
}

// Two noteheads in the same column — a chord, or two voices meeting — share
// a ledger line rather than crowding one. Pulling those apart would be
// pulling one note's own line off it.
func TestAChordsNoteheadsShareTheirLedgerLines(t *testing.T) {
	o := DefaultOptions()
	ch := &model.Chord{Dur: model.Duration{Num: 1, Den: 4}}
	for _, i := range []int{40, 42, 44} {
		ch.Notes = append(ch.Notes, &model.Note{Pitch: model.PitchFromDiatonicIndex(i)})
	}
	sc := &model.Score{Parts: []*model.Part{{
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		Measures: []*model.Measure{{Voices: []*model.Voice{
			{Elements: []model.Element{ch}}}}},
	}}}
	lay := Engrave(sc, o)

	want := smufl.Advance("noteheadBlack") + 2*o.Defaults.LegerLineExtension
	for y, r := range ledgerRows(lay) {
		for _, l := range r {
			if w := l[1] - l[0]; w < want-0.001 {
				t.Errorf("a chord's ledger line at y=%.2f was shortened to "+
					"%.3f, want %.3f: its own noteheads crowded it",
					float64(y)/100, w, want)
			}
		}
	}
}

// Notes cannot be set closer together than a ledger line is wide.
//
// The separation is what makes a run readable; this is what keeps it from
// having to do all the work. Two notes closer than the line is wide would
// have their lines overlapping rather than merely touching, and the most the
// separation could then do is end both lines at their noteheads.
//
// This is arithmetic, not taste, so it is checked rather than remembered.
func TestNotesAreNeverCloserThanALedgerLineIsWide(t *testing.T) {
	o := DefaultOptions()
	head := smufl.Advance("noteheadBlack")
	ledger := head + 2*o.Defaults.LegerLineExtension

	if o.Spacings.MinNote < ledger {
		t.Errorf("the closest two notes may be set is %.3f spaces, and a "+
			"ledger line is %.3f wide: the ledger lines of neighbouring "+
			"notes would overlap",
			o.Spacings.MinNote, ledger)
	}
}
