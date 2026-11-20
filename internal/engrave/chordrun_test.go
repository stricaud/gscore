/*
 * chordrun_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Several chords written into one symbol are set as several chords.
 *
 * "G#7#5(b9) G#7b5(b9) C#7" over a bar of rests was one big G and a line of
 * small type, with the other two roots lost in it. Reported as "with gscore
 * this is confusing", against a printed copy where the three read alike.
 */

package engrave

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAChordRunSetsEveryRootAtFullSize(t *testing.T) {
	c := &model.ChordSymbol{Root: "G#", Quality: "7#5(b9) G#7b5(b9) C#7"}
	pieces, width := chordPieces(c)

	var roots []chordPiece
	for _, p := range pieces {
		if p.glyph == "" && p.size == chordSize {
			roots = append(roots, p)
		}
	}
	var names []string
	for _, r := range roots {
		names = append(names, r.text)
	}
	if got := strings.Join(names, " "); got != "G G C" {
		t.Fatalf("the full-size letters are %q, want G G C", got)
	}
	// In order along the row, with air between one chord and the next.
	one, _ := chordPieces(&model.ChordSymbol{Root: "G#", Quality: "7#5(b9)"})
	end := 0.0
	for _, p := range one {
		if p.glyph == "" {
			end = p.x + chordTextWidth(p.text, p.size)
		} else {
			end = p.x + glyphAdvance(p.glyph, p.size)
		}
	}
	if roots[1].x < end+chordRunGap-0.01 {
		t.Errorf("the second chord starts at %.2f, inside the first which ends at %.2f", roots[1].x, end)
	}
	if roots[2].x <= roots[1].x {
		t.Errorf("the third chord starts at %.2f, before the second at %.2f", roots[2].x, roots[1].x)
	}
	if width <= roots[2].x {
		t.Errorf("the run is %.2f wide and its last chord starts at %.2f", width, roots[2].x)
	}
}

// One chord with a space in its quality is still one chord.
func TestAQualityWithASpaceIsNotARun(t *testing.T) {
	if run := chordRun(&model.ChordSymbol{Root: "C", Quality: "7 alt"}); run != nil {
		t.Errorf("C7 alt was split into %d chords", len(run))
	}
	if run := chordRun(&model.ChordSymbol{Root: "C", Quality: "7", Bass: "E"}); run != nil {
		t.Errorf("C7/E was split into %d chords", len(run))
	}
}

// A slash chord in the run keeps its bass, and a word after a chord stays
// with that chord.
func TestARunKeepsBassesAndWords(t *testing.T) {
	run := chordRun(&model.ChordSymbol{Root: "D", Quality: "m7", Bass: "G C7 alt F#m"})
	if len(run) != 3 {
		t.Fatalf("split into %d chords, want 3", len(run))
	}
	if run[0].Root != "D" || run[0].Quality != "m7" || run[0].Bass != "G" {
		t.Errorf("the first came out as %+v", *run[0])
	}
	if run[1].Root != "C" || run[1].Quality != "7 alt" {
		t.Errorf("the second came out as %+v", *run[1])
	}
	if run[2].Root != "F#" || run[2].Quality != "m" {
		t.Errorf("the third came out as %+v", *run[2])
	}
}
