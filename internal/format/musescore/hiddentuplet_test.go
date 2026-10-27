/*
 * hiddentuplet_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A tuplet that is counted and not written.
 *
 * A measured tremolo — a dotted quarter with strokes through its stem,
 * meaning "repeat this note at sixteenths" — is six sixteenths in the time
 * of four. The tuplet is in the file because the arithmetic is; the strokes
 * are how it is written down, and the figure and the bracket are turned
 * off. MuseScore says so in the tuplet's numberType.
 *
 * Drawn anyway, every tremolo in the cellos of Beethoven's ninth carried a
 * bracketed 6 over it that is in no printed score of the piece — two to a
 * bar, for most of the first movement.
 */

package musescore

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// one staff, one bar, with whatever is written between the voice tags.
func oneBar(inner string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Cello</trackName></Part>
    <Staff id="1"><Measure><voice>` + inner + `</voice></Measure></Staff>
  </Score>
</museScore>`
}

const sixteenth = `<Chord><durationType>16th</durationType>` +
	`<Note><pitch>45</pitch><tpc>17</tpc></Note></Chord>`

func tupletsIn(t *testing.T, s *model.Score) []*model.Tuplet {
	t.Helper()
	var out []*model.Tuplet
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok && n.Tuplet != nil {
						out = append(out, n.Tuplet)
					}
					if c, ok := el.(*model.Chord); ok && c.Tuplet != nil {
						out = append(out, c.Tuplet)
					}
				}
			}
		}
	}
	return out
}

func TestATupletMuseScoreDoesNotWriteIsNotWritten(t *testing.T) {
	// The second of the two that open the cellos: a dotted quarter standing
	// for six sixteenths, with the figure and the bracket turned off.
	hidden := oneBar(
		`<Tuplet><numberType>2</numberType><bracketType>2</bracketType>` +
			`<normalNotes>4</normalNotes><actualNotes>6</actualNotes>` +
			`<baseNote>16th</baseNote></Tuplet>` +
			`<Chord><dots>1</dots><durationType>quarter</durationType>` +
			`<Note><pitch>45</pitch><tpc>17</tpc></Note>` +
			`<Tremolo><subtype>r16</subtype></Tremolo></Chord>` +
			`<endTuplet></endTuplet>`)

	got := tupletsIn(t, read(t, hidden))
	if len(got) != 1 {
		t.Fatalf("%d tuplets came out, want the one in the file", len(got))
	}
	if !got[0].Hidden {
		t.Error("the tuplet is drawn, so a bracketed 6 goes over a tremolo that " +
			"is already written as strokes through its stem")
	}
	if got[0].Actual != 6 || got[0].Normal != 4 {
		t.Errorf("the tuplet is %d in the time of %d, want 6 in the time of 4",
			got[0].Actual, got[0].Normal)
	}
}

func TestATupletMuseScoreDoesWriteIsStillWritten(t *testing.T) {
	// The first of the two: an ordinary sextuplet of sixteenths, with its
	// figure shown. Nothing about this changes.
	shown := oneBar(
		`<Tuplet><normalNotes>4</normalNotes><actualNotes>6</actualNotes>` +
			`<baseNote>16th</baseNote><Number><style>Tuplet</style>` +
			`<text>6</text></Number></Tuplet>` +
			strings.Repeat(sixteenth, 6) +
			`<endTuplet></endTuplet>`)

	got := tupletsIn(t, read(t, shown))
	if len(got) != 6 {
		t.Fatalf("%d notes are in a tuplet, want the six", len(got))
	}
	for i, tp := range got {
		if tp.Hidden {
			t.Errorf("note %d: a sextuplet that says 6 over itself came out hidden", i)
		}
	}
	if !got[0].Start {
		t.Error("the group does not begin at its first note")
	}
	if !got[5].Stop {
		t.Error("the group does not end at its last note, so nothing is drawn at all")
	}
}
