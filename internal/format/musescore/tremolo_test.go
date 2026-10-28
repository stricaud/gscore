/*
 * tremolo_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The two-note tremolo.
 *
 * Beams drawn between a pair of chords, meaning alternate between them for
 * as long as the two of them last. MuseScore names it c8, c16 and so on,
 * counting by the note the alternation is written in, and writes it on the
 * first chord of the pair with nothing at all on the second.
 *
 * It was reported as unread, because the mark belongs to the pair rather
 * than to either chord — so bar 23 of the Presto came out as four half
 * notes in the left hand, and a pianist reading it played four half notes.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// twoChords is a bar holding two half notes, the first carrying whatever
// tremolo element is given.
func twoChords(trem string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1"><Measure><voice>
      <Chord><durationType>half</durationType>` + trem +
		`<Note><pitch>57</pitch><tpc>17</tpc></Note></Chord>
      <Chord><durationType>half</durationType>
        <Note><pitch>61</pitch><tpc>21</tpc></Note></Chord>
    </voice></Measure></Staff>
  </Score>
</museScore>`
}

func tremolosIn(t *testing.T, s *model.Score) []*model.Tremolo {
	t.Helper()
	var out []*model.Tremolo
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					mk := marksOf(el)
					if mk == nil {
						continue
					}
					out = append(out, mk.Tremolo)
				}
			}
		}
	}
	return out
}

func TestATwoNoteTremoloMarksBothOfItsChords(t *testing.T) {
	got := tremolosIn(t, read(t, twoChords(
		`<Tremolo><subtype>c16</subtype></Tremolo>`)))
	if len(got) != 2 {
		t.Fatalf("%d elements in the bar, want 2", len(got))
	}
	if got[0] == nil || !got[0].Start || got[0].Stop {
		t.Errorf("the first chord came out %+v, want the beginning of a pair", got[0])
	}
	if got[1] == nil || !got[1].Stop || got[1].Start {
		t.Errorf("the second chord came out %+v, want the end of a pair — "+
			"MuseScore writes nothing on it, so it has to be marked here", got[1])
	}
	// c16 is an alternation written in sixteenths, which is two beams.
	for i, tr := range got {
		if tr != nil && tr.Strokes != 2 {
			t.Errorf("chord %d has %d strokes, want the 2 of a sixteenth", i, tr.Strokes)
		}
	}
}

func TestEachTwoNoteTremoloSpeedIsItsOwnNumberOfBeams(t *testing.T) {
	for _, tc := range []struct {
		sub   string
		beams int
	}{{"c8", 1}, {"c16", 2}, {"c32", 3}, {"c64", 4}} {
		got := tremolosIn(t, read(t, twoChords(
			`<Tremolo><subtype>`+tc.sub+`</subtype></Tremolo>`)))
		if len(got) != 2 || got[0] == nil {
			t.Fatalf("%s: nothing was read", tc.sub)
		}
		if got[0].Strokes != tc.beams {
			t.Errorf("%s has %d beams, want %d", tc.sub, got[0].Strokes, tc.beams)
		}
		if !got[0].Paired() {
			t.Errorf("%s came out as the slashes on one note", tc.sub)
		}
	}
}

func TestTheSlashesOnOneNoteAreStillTheSlashesOnOneNote(t *testing.T) {
	// r16 is a note repeated in sixteenths: two slashes through its own
	// stem, belonging to that note and to nothing else. Nothing here
	// changes it.
	got := tremolosIn(t, read(t, twoChords(
		`<Tremolo><subtype>r16</subtype></Tremolo>`)))
	if len(got) != 2 || got[0] == nil {
		t.Fatal("nothing was read")
	}
	if got[0].Paired() {
		t.Error("a repeated note came out as one end of a pair")
	}
	if got[1] != nil {
		t.Errorf("the chord after it was marked %+v; a repeated note has no partner", got[1])
	}
}

func TestMuseScoreFourNamesTheTwoNoteKindInTheElement(t *testing.T) {
	// MuseScore 4 writes <TremoloTwoChord> with no subtype where 3 wrote
	// <Tremolo><subtype>c8</subtype>.
	got := tremolosIn(t, read(t, twoChords(`<TremoloTwoChord></TremoloTwoChord>`)))
	if len(got) != 2 || got[0] == nil {
		t.Fatal("nothing was read")
	}
	if !got[0].Start || got[1] == nil || !got[1].Stop {
		t.Errorf("came out as %+v and %+v, want a pair", got[0], got[1])
	}
}

func TestTheNotesOfATwoNoteTremoloKeepTheirOwnLengths(t *testing.T) {
	// Two half notes of a two-note tremolo fill four beats, exactly as two
	// half notes do. What changes is only what is heard in that time.
	s := read(t, twoChords(`<Tremolo><subtype>c16</subtype></Tremolo>`))
	half := model.Duration{Num: 1, Den: 2}
	for i, el := range s.Parts[0].Measures[0].Voices[0].Elements {
		if got := el.Duration(); got.Cmp(half) != 0 {
			t.Errorf("note %d lasts %v, want %v", i, got, half)
		}
	}
}
