/*
 * chordrules_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Two rules of chord engraving, checked against the page.
 *
 * The accidentals of a chord, where they cannot stand in one column: the
 * highest takes the place next to the chord, each one below it that would
 * collide steps out to the left, and as soon as one can stand in line with
 * the top one again it does. And the stem of a chord: it points away from
 * the note farthest from the middle line, and down when the outer notes
 * are equally far. Both are in the MPA guide and in Gould, and both were
 * believed to hold here; this is where they are made to.
 */

package engrave

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

type chordNote struct {
	step   model.Step
	octave int
	acc    model.Accidental
}

func ruleChord(notes ...chordNote) *model.Score {
	q := model.Duration{Num: 1, Den: 4}
	ch := &model.Chord{Dur: q}
	for _, n := range notes {
		ch.Notes = append(ch.Notes, &model.Note{
			Pitch: pitch(n.step, n.octave), Dur: q, Accidental: n.acc,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: q},
		Parts: []*model.Part{{
			Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
			Measures: []*model.Measure{{
				Voices: []*model.Voice{{Elements: []model.Element{
					ch, &model.Rest{Dur: model.Duration{Num: 3, Den: 4}},
				}}},
				RightBarline: model.BarFinal,
			}},
		}},
	}
}

func engraveStrip(s *model.Score) *Layout {
	opt := DefaultOptions()
	opt.Mode = Strip
	return Engrave(s, opt)
}

// accidentalsDrawn is every accidental on the page, with its reach up and
// down from where it is drawn, so that two can be asked whether they would
// collide in one column.
type drawnSign struct {
	name        string
	x, y        float64
	top, bottom float64
}

func accidentalsDrawn(lay *Layout) []drawnSign {
	var out []drawnSign
	for _, g := range lay.Glyphs {
		if !has(g.Name, "accidental") {
			continue
		}
		up, down := glyphReach(g.Name)
		out = append(out, drawnSign{
			name: g.Name, x: g.X, y: g.Y,
			top: g.Y - up - accidentalAir, bottom: g.Y + down + accidentalAir,
		})
	}
	return out
}

func (a drawnSign) collides(b drawnSign) bool {
	return a.top < b.bottom && b.top < a.bottom
}

// Rule (g). Four chords whose accidentals cannot all stand in one column,
// including the guide's own example of a stack of sharps.
func TestChordAccidentalsFileFromTheTopAndStepOutOnlyWhenTheyMust(t *testing.T) {
	for _, c := range []struct {
		name  string
		notes []chordNote
	}{
		{"two thirds of sharps", []chordNote{
			{model.StepG, 4, model.Sharp}, {model.StepB, 4, model.Sharp},
			{model.StepD, 5, model.Sharp}, {model.StepF, 5, model.Sharp}}},
		{"a seventh chord of flats", []chordNote{
			{model.StepB, 3, model.Flat}, {model.StepD, 4, model.Flat},
			{model.StepF, 4, model.Flat}, {model.StepA, 4, model.Flat}}},
		{"a cluster", []chordNote{
			{model.StepC, 5, model.Sharp}, {model.StepD, 5, model.Flat},
			{model.StepE, 5, model.Natural}}},
		{"wide enough to align", []chordNote{
			{model.StepC, 4, model.Sharp}, {model.StepE, 5, model.Sharp}}},
	} {
		lay := engraveStrip(ruleChord(c.notes...))
		signs := accidentalsDrawn(lay)
		if len(signs) != len(c.notes) {
			t.Fatalf("%s: %d accidentals drawn, want %d", c.name, len(signs), len(c.notes))
		}
		// The files, by x: the column nearest the chord is the furthest
		// right. Signs at one x are one file.
		var files []float64
		for _, s := range signs {
			found := false
			for _, f := range files {
				if math.Abs(f-s.x) < 0.01 {
					found = true
				}
			}
			if !found {
				files = append(files, s.x)
			}
		}
		right := math.Inf(-1)
		for _, f := range files {
			right = math.Max(right, f)
		}
		fileOf := func(s drawnSign) int {
			// Counted from the chord outwards: 0 is nearest.
			n := 0
			for _, f := range files {
				if f > s.x+0.01 {
					n++
				}
			}
			return n
		}

		// The highest accidental stands next to the chord.
		top := signs[0]
		for _, s := range signs {
			if s.y < top.y {
				top = s
			}
		}
		if math.Abs(top.x-right) > 0.01 {
			t.Errorf("%s: the top accidental stands at %.2f, not in the file next to the chord at %.2f",
				c.name, top.x, right)
		}
		for i, a := range signs {
			// Nothing in one file collides.
			for j, b := range signs {
				if j != i && math.Abs(a.x-b.x) < 0.01 && a.collides(b) {
					t.Errorf("%s: %s at %.1f and %s at %.1f share a file and collide",
						c.name, a.name, a.y, b.name, b.y)
				}
			}
			// And a sign stepped out to file k had no room in any file
			// nearer the chord: something there collides with it.
			for k := 0; k < fileOf(a); k++ {
				blocked := false
				for _, b := range signs {
					if fileOf(b) == k && a.collides(b) {
						blocked = true
					}
				}
				if !blocked {
					t.Errorf("%s: %s at %.1f stepped out to file %d though file %d had room for it",
						c.name, a.name, a.y, fileOf(a), k)
				}
			}
		}
	}
}

// chordStemGoesUp reads the chord's stem off the page: up if it rises
// above the highest head, down if it falls below the lowest.
func chordStemGoesUp(t *testing.T, lay *Layout) bool {
	t.Helper()
	th := DefaultOptions().Defaults.StemThickness
	hi, lo := math.Inf(1), math.Inf(-1)
	for _, g := range lay.Glyphs {
		if has(g.Name, "notehead") {
			hi, lo = math.Min(hi, g.Y), math.Max(lo, g.Y)
		}
	}
	for _, l := range lay.Lines {
		if l.X1 != l.X2 || math.Abs(l.Thickness-th) > 1e-6 {
			continue
		}
		top, bottom := math.Min(l.Y1, l.Y2), math.Max(l.Y1, l.Y2)
		switch {
		case top < hi-1 && bottom <= lo+0.01:
			return true
		case bottom > lo+1 && top >= hi-0.01:
			return false
		}
	}
	t.Fatal("no stem was found on the chord")
	return false
}

// The stem of a chord points away from the note farthest from the middle
// line, and down when the outer notes are equally far from it.
func TestAChordsStemFollowsTheNoteFarthestFromTheMiddle(t *testing.T) {
	for _, c := range []struct {
		name  string
		notes []chordNote
		up    bool
	}{
		{"low triad", []chordNote{{model.StepE, 4, 0}, {model.StepG, 4, 0}, {model.StepB, 4, 0}}, true},
		{"high triad", []chordNote{{model.StepD, 5, 0}, {model.StepF, 5, 0}, {model.StepA, 5, 0}}, false},
		{"far below, a little above", []chordNote{{model.StepC, 4, 0}, {model.StepC, 5, 0}}, true},
		{"a little below, far above", []chordNote{{model.StepA, 4, 0}, {model.StepA, 5, 0}}, false},
		{"equidistant, a fifth", []chordNote{{model.StepG, 4, 0}, {model.StepD, 5, 0}}, false},
		{"equidistant, a third", []chordNote{{model.StepA, 4, 0}, {model.StepC, 5, 0}}, false},
		{"equidistant, an octave", []chordNote{{model.StepE, 4, 0}, {model.StepF, 5, 0}}, false},
	} {
		lay := engraveStrip(ruleChord(c.notes...))
		if got := chordStemGoesUp(t, lay); got != c.up {
			want := "down"
			if c.up {
				want = "up"
			}
			t.Errorf("%s: the stem goes the wrong way, want %s", c.name, want)
		}
	}
}
