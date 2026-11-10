/*
 * accidentalnames_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What MuseScore calls an accidental, over three of its versions.
 *
 * MuseScore 4 writes the SMuFL glyph name, MuseScore 3 the enumerator with
 * the letter doubled — "sharp sharp" — and MuseScore 2 the plain English,
 * "double sharp", which is the same words gscore uses itself. Only that
 * last pair was missing from the table, so a double sharp in a file written
 * by MuseScore 2 was read as no accidental at all and logged as a sign
 * gscore cannot draw. It draws it: the fault was in the reading.
 *
 * Reported against the 1812 Overture, a MuseScore 2.1 file, where the cor
 * anglais at bar 61 and a grace note in the first violins at bar 53 each
 * lost their sign.
 */

package musescore

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// oneNote is a bar of one note carrying this accidental element.
func oneNote(sub string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="2.06">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Oboe</trackName></Part>
    <Staff id="1"><Measure><voice>
      <Chord><durationType>quarter</durationType>
        <Note><Accidental><subtype>` + sub + `</subtype></Accidental>
          <pitch>62</pitch><tpc>28</tpc></Note></Chord>
    </voice></Measure></Staff>
  </Score>
</museScore>`
}

// signOn is the accidental written on the one note of such a bar.
func signOn(t *testing.T, sub string) model.Accidental {
	t.Helper()
	s := read(t, oneNote(sub))
	if len(s.Parts) == 0 {
		t.Fatal("the file came back with no staves")
	}
	return firstNote(t, s.Parts[0]).Accidental
}

func TestEverySpellingMuseScoreHasUsedIsRead(t *testing.T) {
	for _, c := range []struct {
		sub  string
		want model.Accidental
	}{
		// MuseScore 2's names, which are the ones that were missing.
		{"double sharp", model.DoubleSharp},
		{"double flat", model.DoubleFlat},
		{"sharp", model.Sharp},
		{"flat", model.Flat},
		{"natural", model.Natural},
		// MuseScore 3's.
		{"sharp sharp", model.DoubleSharp},
		{"flat flat", model.DoubleFlat},
		// MuseScore 4's.
		{"accidentalDoubleSharp", model.DoubleSharp},
		{"accidentalDoubleFlat", model.DoubleFlat},
		{"accidentalSharp", model.Sharp},
		{"accidentalFlat", model.Flat},
		{"accidentalNatural", model.Natural},
	} {
		t.Run(c.sub, func(t *testing.T) {
			if got := signOn(t, c.sub); got != c.want {
				t.Errorf("%q was read as %v, want %v", c.sub, got, c.want)
			}
		})
	}
}

// And it is not reported as a sign gscore cannot draw, because it draws it.
func TestADoubleSharpIsNotReportedAsUndrawable(t *testing.T) {
	_, err := Import(strings.NewReader(oneNote("double sharp")))
	if err != nil && strings.Contains(err.Error(), "accidental") {
		t.Errorf("reading a double sharp said: %v", err)
	}
	if _, ok := smufl.Glyph(smufl.AccidentalGlyph(model.DoubleSharp)); !ok {
		t.Error("the music font has no double sharp, which is the one thing " +
			"that would make the old message true")
	}
}
