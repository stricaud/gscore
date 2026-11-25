/*
 * transposingclef_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The clef a transposing instrument is read in.
 *
 * MuseScore gives a tenor saxophone a concert clef, the treble with an 8
 * under it, and a transposing clef, the plain treble. gscore keeps written
 * pitches, so the second is the one; taking the first drew the tenors of a
 * Sophisticated Lady big band chart an octave and more above the staff, and
 * the baritone in the bass clef.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func saxScore(inst, music string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="2.06">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Tenor Sax</trackName>
      <Instrument>` + inst + `<transposeDiatonic>-8</transposeDiatonic>
        <transposeChromatic>-14</transposeChromatic></Instrument></Part>
    <Staff id="1"><Measure><voice>` + music + `
      <Chord><durationType>whole</durationType>
        <Note><pitch>58</pitch><tpc>12</tpc><tpc2>14</tpc2></Note></Chord>
    </voice></Measure></Staff>
  </Score>
</museScore>`
}

func TestATransposingInstrumentIsReadInTheClefThePlayerReads(t *testing.T) {
	for name, src := range map[string]string{
		"instrument": saxScore(`<concertClef>G8vb</concertClef><transposingClef>G</transposingClef>`, ""),
		"reversed":   saxScore(`<transposingClef>G</transposingClef><concertClef>G8vb</concertClef>`, ""),
		"in the music": saxScore(`<concertClef>G8vb</concertClef><transposingClef>G</transposingClef>`,
			`<Clef><concertClefType>G8vb</concertClefType><transposingClefType>G</transposingClefType></Clef>`),
	} {
		if got := read(t, src).Parts[0].Clef; got != model.Treble {
			t.Errorf("%s: clef %v, want the treble", name, got)
		}
	}
	bari := saxScore(`<concertClef>F</concertClef><transposingClef>G</transposingClef>`, "")
	if got := read(t, bari).Parts[0].Clef; got != model.Treble {
		t.Errorf("baritone: clef %v, want the treble, not its concert bass clef", got)
	}
}
