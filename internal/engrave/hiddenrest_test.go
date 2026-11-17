/*
 * hiddenrest_test.go
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

// A hidden element keeps its room and prints nothing. The editor asks to
// see it all the same, in grey, so it can be found and picked up. See
// model.Marks.Hidden and Options.ShowHidden.

// restAndNotes is a bar of a rest and two notes, the first two hidden or
// not as asked: the rest by its own flag, the note by its marks.
func restAndNotes(hidden bool) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	first := &model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: q,
		Accidental: model.Sharp}
	first.Marks.Hidden = hidden
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Rest{Dur: q, Hidden: hidden},
					first,
					&model.Note{Pitch: model.Pitch{Step: model.StepD, Octave: 5}, Dur: q},
				}}},
				RightBarline: model.BarSingle,
			}},
		}},
	}
}

func restGlyphs(lay *Layout) []Glyph {
	var out []Glyph
	for _, g := range lay.Glyphs {
		if has(g.Name, "rest") {
			out = append(out, g)
		}
	}
	return out
}

func TestAHiddenElementKeepsItsRoomAndPrintsNothing(t *testing.T) {
	opt := DefaultOptions()
	shown := Engrave(restAndNotes(false), opt)
	printed := Engrave(restAndNotes(true), opt)

	if n := len(restGlyphs(shown)); n != 1 {
		t.Fatalf("%d rests drawn for a bar with one, want 1", n)
	}
	if n := len(restGlyphs(printed)); n != 0 {
		t.Errorf("%d rests printed for a hidden rest, want none", n)
	}
	// The hidden note goes too, with its stem and its sharp; the note
	// after both stands where it stood, so their room is kept.
	a, b := heads(shown), heads(printed)
	if len(a) != 2 || len(b) != 1 {
		t.Fatalf("%d and %d noteheads, want 2 and then 1", len(a), len(b))
	}
	if math.Abs(a[1].X-b[0].X) > 0.01 {
		t.Errorf("the last note moved from %.2f to %.2f when the others were hidden",
			a[1].X, b[0].X)
	}
	if len(printed.Lines) >= len(shown.Lines) {
		t.Errorf("the hidden note's stem was still drawn: %d lines, was %d",
			len(printed.Lines), len(shown.Lines))
	}
	for _, g := range printed.Glyphs {
		if has(g.Name, "accidental") {
			t.Errorf("the hidden note's sharp was still drawn")
		}
	}

	// The editor sees it all, in grey, stems included.
	opt.ShowHidden = true
	edited := Engrave(restAndNotes(true), opt)
	if n := len(restGlyphs(edited)); n != 1 {
		t.Fatalf("%d rests drawn for the editor, want the hidden one", n)
	}
	if got := restGlyphs(edited)[0].Colour; got != hiddenColour {
		t.Errorf("the hidden rest is drawn in %q, want the grey %q", got, hiddenColour)
	}
	hs := heads(edited)
	if len(hs) != 2 {
		t.Fatalf("%d noteheads drawn for the editor, want both", len(hs))
	}
	if hs[0].Colour != hiddenColour || hs[1].Colour != "" {
		t.Errorf("the heads are drawn in %q and %q, want the grey and then black",
			hs[0].Colour, hs[1].Colour)
	}
	grey, black := 0, 0
	for _, ln := range edited.Lines {
		if ln.Colour == hiddenColour {
			grey++
		} else {
			black++
		}
	}
	if grey == 0 {
		t.Error("no grey line: the hidden note's stem is drawn as ink")
	}
	if black == 0 {
		t.Error("no black line: the other note's stem went grey as well")
	}
}
