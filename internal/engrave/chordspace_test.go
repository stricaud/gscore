/*
 * chordspace_test.go
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
	"github.com/stricaud/gscore/internal/smufl"
)

/*
 * The pieces of a chord symbol do not sit on each other.
 *
 * A symbol is not one piece of text: a bold capital for the root, a small
 * raised sign for its flat, smaller type again for the quality. Each was
 * advanced by textWidth, which averages over ordinary words — mostly
 * lowercase, about half the type size a letter. A bold capital is half as
 * wide again and a flat sign is not a letter at all, so both were advanced
 * too little: "G7" printed its seven on the G's shoulder and "E♭6" put the
 * flat through the E. Reported as "sometimes readable, sometimes not".
 */

// right is where a piece of a symbol ends.
func right(p chordPiece) float64 {
	if p.glyph != "" {
		return p.x + accidentalAdvance(p.glyph)
	}
	return p.x + chordTextWidth(p.text, p.size)
}

func TestThePiecesOfAChordDoNotOverlap(t *testing.T) {
	for _, c := range []*model.ChordSymbol{
		{Root: "G", Quality: "7"},
		{Root: "C", Quality: "7b9"},
		{Root: "Eb", Quality: "6"},
		{Root: "Ab", Quality: "-6"},
		{Root: "F", Quality: "-7"},
		{Root: "Bb", Quality: "7"},
		{Root: "C", Quality: "maj7", Bass: "G"},
		{Root: "F#", Quality: "m7b5", Bass: "Eb"},
	} {
		pieces, width := chordPieces(c)
		if len(pieces) < 2 {
			t.Errorf("%s came out as %d pieces", c.String(), len(pieces))
			continue
		}
		// Not merely clear of each other: a flat set exactly where the
		// letter stops touches it, which reads as one smudged shape. Every
		// join wants air in it.
		want := chordPieceGap * chordSize
		for i := 1; i < len(pieces); i++ {
			gap := pieces[i].x - right(pieces[i-1])
			if gap < 0 {
				t.Errorf("%s: %q runs %.2f spaces into what follows it",
					c.String(), piece(pieces[i-1]), -gap)
			} else if gap < want*0.9 {
				t.Errorf("%s: %.2f spaces between %q and %q, want about %.2f",
					c.String(), gap, piece(pieces[i-1]), piece(pieces[i]), want)
			}
		}
		// And the room kept for it is the room it takes, or the symbol
		// after it is allowed to sit on top.
		if last := right(pieces[len(pieces)-1]); width < last {
			t.Errorf("%s: measured %.2f wide, drawn %.2f", c.String(), width, last)
		}
	}
}

// piece names a piece for a failure message.
func piece(p chordPiece) string {
	if p.glyph != "" {
		return p.glyph
	}
	return p.text
}

// A bold capital is wider than the average letter, which is what the old
// reckoning used for everything.
func TestAChordRootIsMeasuredAsACapital(t *testing.T) {
	for _, root := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		got := chordTextWidth(root, chordSize)
		if avg := textWidth(root, chordSize); got <= avg {
			t.Errorf("%s is measured %.2f, no wider than the %.2f an average "+
				"letter gets", root, got, avg)
		}
	}
	// Digits are narrower than capitals, and the quality is mostly digits.
	if chordTextWidth("7", chordSize) >= chordTextWidth("G", chordSize) {
		t.Error("a seven is measured as wide as a G")
	}
}

// The flat in a chord name is a glyph, and the font says how wide it is.
func TestAChordAccidentalIsMeasuredByTheFont(t *testing.T) {
	flat := accidentalAdvance("accidentalFlat")
	if want := smufl.Advance("accidentalFlat") * accidentalSize("accidentalFlat") / 4; flat != want {
		t.Errorf("the flat advances %.3f, want the font's %.3f", flat, want)
	}
	if flat <= 0 {
		t.Error("the flat takes no room at all")
	}
}

// Nothing in a chord symbol rises above its root.
//
// An accidental is a staff glyph drawn about the note it alters, not type: a
// flat hangs below that point and throws its ascender above, a sharp is
// symmetric about it. Given one size and one height they stand to different
// heights and land in different places, and a line of chords set that way
// has a ragged top edge — the capital you actually read being the shortest
// thing in it.
func TestEveryPieceOfAChordFinishesLevelWithTheRoot(t *testing.T) {
	top := chordCapHeight * chordSize

	for _, glyph := range []string{
		"accidentalFlat", "accidentalSharp", "accidentalNatural",
		"accidentalDoubleFlat", "accidentalDoubleSharp",
	} {
		ink := accidentalInk[glyph]
		reach := accidentalRise(glyph) + ink[0]*accidentalSize(glyph)/4
		if math.Abs(reach-top) > 0.001 {
			t.Errorf("%s reaches %.3f above the baseline, want the root's %.3f",
				glyph, reach, top)
		}
		// And they all stand the same height as each other.
		height := (ink[0] - ink[1]) * accidentalSize(glyph) / 4
		if want := chordAccidentalHeight * top; math.Abs(height-want) > 0.001 {
			t.Errorf("%s stands %.3f tall, want %.3f", glyph, height, want)
		}
	}

	// The quality too: smaller type, raised so that its top is the root's.
	pieces, _ := chordPieces(&model.ChordSymbol{Root: "B", Quality: "7"})
	q := pieces[len(pieces)-1]
	if reach := q.dy + chordCapHeight*q.size; math.Abs(reach-top) > 0.001 {
		t.Errorf("the quality reaches %.3f, want the root's %.3f", reach, top)
	}
}
