/*
 * wholepiece_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A whole piece, written out as GMN and read back.
 *
 * Twenty short examples written to cover one thing each will pass while
 * the format is still unusable, because what breaks a format is not one
 * feature but all of them at once: a slur that crosses a barline into a
 * tuplet, a tie whose other end is three bars away, a second voice that
 * is silent for a bar and comes back.
 *
 * samples/altes-10e-solo.abc is the 10me Solo — 131 bars, three staves,
 * two voices on two of them, and some two and a half thousand things
 * worth keeping. Everything the two directions still disagreed about was
 * found by this and by nothing else:
 *
 *   every explicit barline that fell where the meter already ended a bar,
 *   which is where a repeat is always written;
 *   the accidentals of every grace note;
 *   a hundred and twenty slurs, ties and hairpins attached to the wrong
 *   notes, for want of the number GUIDO pairs the two ends of a span by;
 *   every bar a second voice was silent for, which collapsed and left the
 *   rest of that voice sitting a bar to the left of the staff;
 *   forty bars tied into one note, from a tie whose end was never written.
 */

package guido

import (
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/musescore"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
)

// thePieces are the whole scores the repository carries: a flute solo
// with a piano part under it, and a rag in two hands with repeats and
// voltas in it.
var thePieces = []string{
	"../../../samples/altes-10e-solo.abc",
	"../../../samples/the-entertainer.mxl",
}

func TestTheWholePieceSurvivesBeingWrittenOut(t *testing.T) {
	for _, path := range thePieces {
		t.Run(path, func(t *testing.T) { wholePiece(t, path) })
	}
}

func wholePiece(t *testing.T, path string) {
	t.Helper()
	imp, err := format.ImporterFor(path)
	if err != nil {
		t.Skipf("nothing here reads %s: %v", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("the sample is not here: %v", err)
	}
	defer f.Close()

	orig, err := imp.Import(f)
	if orig == nil {
		t.Fatalf("the sample could not be read: %v", err)
	}

	text := write(t, orig)
	back, err := Import(strings.NewReader(text))
	if back == nil {
		t.Fatalf("what was written could not be read: %v", err)
	}
	if err != nil {
		// Not fatal: the complaints are about the piece and not about the
		// reading of it — a hairpin the ABC leaves open stays open here.
		t.Logf("read back with complaints: %v", err)
	}

	a, b := describe(orig), describe(back)
	if len(a) < 500 {
		t.Fatalf("the sample describes as only %d things; it should be "+
			"a whole piece", len(a))
	}
	diff(t, a, b, "")
}
