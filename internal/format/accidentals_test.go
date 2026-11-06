/*
 * accidentals_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package format_test

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"

	_ "github.com/stricaud/gscore/internal/format/abc"
	_ "github.com/stricaud/gscore/internal/format/musicxml"
)

/*
 * A score that comes in from a file is spelled before anybody sees it.
 *
 * Which notes print an accidental is not something any file states — a file
 * says how each note is altered, which is as true of the fourth F sharp in a
 * bar as of the first — so it is worked out on the way in, once, for every
 * format. These are the cases that said so: each one was measured on the
 * real thing before the pass existed, and the number beside it is what the
 * page actually showed.
 */

// signs counts the accidentals that would be printed.
func signs(s *model.Score) int {
	n := 0
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					switch e := el.(type) {
					case *model.Note:
						if e.Accidental != model.NoAccidental {
							n++
						}
					case *model.Chord:
						for _, cn := range e.Notes {
							if cn.Accidental != model.NoAccidental {
								n++
							}
						}
					}
				}
			}
		}
	}
	return n
}

func read(t *testing.T, name, text string) *model.Score {
	t.Helper()
	imp, err := format.ImporterFor(name)
	if err != nil {
		t.Fatalf("nothing reads %s: %v", name, err)
	}
	scores, err := format.ImportAll(imp, strings.NewReader(text))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if len(scores) == 0 {
		t.Fatalf("%s gave no score", name)
	}
	return scores[0]
}

// An accidental written twice in one bar is printed once.
//
// ABC carries an accidental through the bar, as notation does, so the second
// ^F is redundant — and somebody typing one out writes what is clear to type
// rather than what is correct to print. This read three sharps where two
// belong: one per bar.
func TestARedundantAccidentalIsNotPrintedTwice(t *testing.T) {
	s := read(t, "tune.abc", "X:1\nT:t\nM:4/4\nL:1/4\nK:C\n^F ^F F F | ^F2 F2 |\n")

	if got, want := signs(s), 2; got != want {
		t.Errorf("the page shows %d accidentals, want %d — one in each bar, "+
			"and the notes after them are already sharp", got, want)
	}
}

// An alteration a file states and never says to print is printed.
//
// This is the other way the two fields come apart, and the worse one: a note
// sounding a semitone off with nothing on the page to say so. MusicXML keeps
// <alter> and <accidental> separate, and a file may give the first without
// the second — the four altered notes here showed no signs at all.
func TestAnAlterationWithNoSignGetsOne(t *testing.T) {
	s := read(t, "t.musicxml", `<?xml version="1.0"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list>
  <part id="P1"><measure number="1">
    <attributes><divisions>1</divisions>
      <key><fifths>0</fifths></key>
      <time><beats>4</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <note><pitch><step>F</step><alter>1</alter><octave>4</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
    <note><pitch><step>C</step><alter>1</alter><octave>5</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
    <note><pitch><step>B</step><alter>-1</alter><octave>4</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
    <note><pitch><step>E</step><alter>-1</alter><octave>4</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
  </measure></part>
</score-partwise>`)

	if got, want := signs(s), 4; got != want {
		t.Errorf("the page shows %d accidentals, want %d — four altered notes, "+
			"and nothing else in the bar says so", got, want)
	}
}

// And where the file does say what was printed, that stands.
//
// MusicXML's <accidental> is a statement about a page somebody engraved. A
// sharp in a key that already has one is redundant and was printed anyway,
// which is the other program's decision to have made.
func TestAPrintedAccidentalAFileStatesIsKept(t *testing.T) {
	s := read(t, "t.musicxml", `<?xml version="1.0"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list>
  <part id="P1"><measure number="1">
    <attributes><divisions>1</divisions>
      <key><fifths>1</fifths></key>
      <time><beats>4</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <note><pitch><step>F</step><alter>1</alter><octave>4</octave></pitch>
      <duration>2</duration><type>half</type><accidental>sharp</accidental></note>
    <note><pitch><step>F</step><alter>1</alter><octave>4</octave></pitch>
      <duration>2</duration><type>half</type></note>
  </measure></part>
</score-partwise>`)

	if got, want := signs(s), 1; got != want {
		t.Errorf("the page shows %d accidentals, want %d — the file asked for "+
			"the first and said nothing about the second", got, want)
	}
}

// A file that states neither the alteration nor the sign is left alone.
//
// Strictly, no <alter> is an alteration of nothing, so a C under a key of
// five flats would be a C natural and would need a natural printed on it.
// Files do not mean that: plenty set a key signature and then write bare
// pitches under it, and taken at their word every one of those notes grows a
// sign that no printed copy of the music has.
func TestBarePitchesUnderAKeySignatureAreLeftAlone(t *testing.T) {
	s := read(t, "t.musicxml", `<?xml version="1.0"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list>
  <part id="P1"><measure number="1">
    <attributes><divisions>1</divisions>
      <key><fifths>-5</fifths></key>
      <time><beats>4</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>2</duration><type>half</type></note>
    <note><pitch><step>G</step><octave>4</octave></pitch>
      <duration>2</duration><type>half</type></note>
  </measure></part>
</score-partwise>`)

	if got, want := signs(s), 0; got != want {
		t.Errorf("the page shows %d accidentals, want %d", got, want)
	}
}
