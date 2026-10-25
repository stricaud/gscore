/*
 * opening_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"

	_ "github.com/stricaud/gscore/internal/format/musescore"
)

/*
 * samples/altes-opening.abc is the first six bars of the 10me Solo, typed out
 * by hand from the original edition, and samples/altes-10e-solo.mscz is the
 * whole piece. They are the same music, so they had better say so.
 *
 * Written by hand, an example drifts: a tie is dropped, an accidental is
 * forgotten, and nobody notices because it still looks like music. This
 * compares it against the score it was copied from, note for note, which is
 * the only reading that settles it. It found one already — the B in bar 6,
 * which is natural there because it is tied over from bar 4, and which had
 * quietly taken the B flat of the key signature instead.
 */
func TestOpeningMatchesTheScore(t *testing.T) {
	// The whole piece is not kept in the repository. Point at wherever it
	// lives to run this:
	//
	//     ALTES=~/scores/altes-10e-solo.mscz go test ./internal/engrave
	//
	// Without it there is nothing to compare against, which is not the same
	// as the example being wrong, so this skips rather than fails.
	source := os.Getenv("ALTES")
	if source == "" {
		source = "../../samples/altes-10e-solo.mscz"
	}
	if _, err := os.Stat(source); err != nil {
		t.Skipf("the whole score is not at %s; set ALTES to where it is", source)
	}
	orig, err := load(source)
	if err != nil && orig == nil {
		t.Fatalf("the score the example was copied from: %v", err)
	}
	mine, err := load("../../samples/altes-opening.abc")
	if err != nil && mine == nil {
		t.Fatalf("the example: %v", err)
	}
	if len(mine.Parts) != 3 {
		t.Fatalf("the example has %d staves; it should have the flute and "+
			"the two hands of the piano", len(mine.Parts))
	}
	for i := range mine.Parts {
		a, b := firstBars(orig.Parts[i], 6), firstBars(mine.Parts[i], 6)
		if a != b {
			t.Errorf("staff %d is not what the score has:\n--- the score\n%s--- the example\n%s",
				i, a, b)
		}
	}
}

// firstBars writes out the pitches and durations of a part's opening bars,
// voice by voice, in a form two readings can be compared as text.
func firstBars(p *model.Part, n int) string {
	var b strings.Builder
	for mi, m := range p.Measures {
		if mi >= n {
			break
		}
		fmt.Fprintf(&b, "bar %d:", mi+1)
		for vi, v := range m.Voices {
			if vi > 0 {
				b.WriteString("  /")
			}
			for _, el := range v.Elements {
				switch e := el.(type) {
				case *model.Note:
					fmt.Fprintf(&b, " %v%s", e.Pitch, e.Dur)
				case *model.Chord:
					b.WriteString(" [")
					for _, n := range e.Notes {
						fmt.Fprintf(&b, "%v,", n.Pitch)
					}
					fmt.Fprintf(&b, "]%s", e.Dur)
				case *model.Rest:
					fmt.Fprintf(&b, " rest%s", e.Dur)
				}
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
