/*
 * harmony_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A chord symbol is read wherever MuseScore has put its fields.
//
// MuseScore 4.4 moved the root and the name a level deeper, into a wrapper
// of their own. Read as furniture and skipped, every chord symbol in a file
// written by that version came back empty: a lead sheet with no chords over
// it and nothing to say they had been there. Reported as "no chord names
// written" of a file holding forty-two of them.
func TestAChordSymbolIsReadInEitherShape(t *testing.T) {
	// The same chord — E flat six — in the old shape and the new one.
	const old = `<museScore version="3.01"><Score><Staff id="1"><Measure>
      <voice>
        <Harmony><root>11</root><name>6</name></Harmony>
        <Chord><durationType>whole</durationType><Note><pitch>63</pitch><tpc>11</tpc></Note></Chord>
      </voice>
    </Measure></Staff></Score></museScore>`
	const wrapped = `<museScore version="4.40"><Score><Staff id="1"><Measure>
      <voice>
        <Harmony><harmonyInfo><name>6</name><root>11</root></harmonyInfo><eid>x</eid></Harmony>
        <Chord><durationType>whole</durationType><Note><pitch>63</pitch><tpc>11</tpc></Note></Chord>
      </voice>
    </Measure></Staff></Score></museScore>`

	for _, tc := range []struct{ what, text string }{
		{"the old shape", old},
		{"MuseScore 4.4's wrapper", wrapped},
	} {
		s := read(t, tc.text)
		sym := firstChordSymbol(s)
		if sym == nil {
			t.Errorf("%s: no chord symbol was read", tc.what)
			continue
		}
		if got := sym.String(); got != "Eb6" {
			t.Errorf("%s: the chord is %q, want %q", tc.what, got, "Eb6")
		}
	}
}

// firstChordSymbol is the first chord written over the music.
func firstChordSymbol(s *model.Score) *model.ChordSymbol {
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					var mk *model.Marks
					switch e := el.(type) {
					case *model.Note:
						mk = &e.Marks
					case *model.Chord:
						mk = &e.Marks
					case *model.Rest:
						mk = &e.Marks
					}
					if mk != nil && mk.Chord != nil {
						return mk.Chord
					}
				}
			}
		}
	}
	return nil
}
