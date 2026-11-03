/*
 * rehearsal_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Rehearsal marks through MusicXML.
 *
 * Every other format gscore reads carries them — MuseScore, NoteWorthy,
 * LilyPond, Finale — and this one did not, in either direction. A score
 * read from a Finale file with its letters intact lost every one of them
 * on the way out, which is the kind of loss nobody finds until a rehearsal.
 */

package musicxml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// marked is a score of n staves whose bars carry the given marks, by bar.
func marked(staves int, marks map[int]string, bars int) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Name: "Staff", Clef: model.Treble,
			Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for b := 0; b < bars; b++ {
			m := &model.Measure{
				Voices: []*model.Voice{{Elements: []model.Element{
					&model.Rest{Dur: model.Duration{Num: 1, Den: 1}},
				}}},
				RightBarline: model.BarSingle,
			}
			m.Marker = marks[b]
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	s.Normalise()
	return s
}

func exported(t *testing.T, s *model.Score) string {
	t.Helper()
	var b bytes.Buffer
	if err := Export(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func reimported(t *testing.T, xml string) *model.Score {
	t.Helper()
	s, err := Import(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestARehearsalMarkIsWritten(t *testing.T) {
	out := exported(t, marked(1, map[int]string{1: "A"}, 3))
	if !strings.Contains(out, "<rehearsal>A</rehearsal>") {
		t.Errorf("no rehearsal mark in:\n%s", out)
	}
	// Its own element and not a word over the music. A reader that saw
	// words would put "poco rit." in the list of places to start from.
	if strings.Contains(out, "<words>A</words>") {
		t.Error("the mark was written as a word")
	}
}

func TestARehearsalMarkComesBack(t *testing.T) {
	s := reimported(t, exported(t, marked(1, map[int]string{1: "B"}, 3)))
	if got := s.Parts[0].Measures[1].Marker; got != "B" {
		t.Errorf("bar 2 came back marked %q", got)
	}
	if got := s.Parts[0].Measures[0].Marker; got != "" {
		t.Errorf("bar 1 came back marked %q", got)
	}
}

// Written once however many staves there are. Twenty staves would
// otherwise give every reader twenty letters to print over one bar.
func TestAMarkIsWrittenOnceForTheWholeScore(t *testing.T) {
	out := exported(t, marked(4, map[int]string{0: "A"}, 2))
	if n := strings.Count(out, "<rehearsal>"); n != 1 {
		t.Errorf("the mark was written %d times", n)
	}
}

// And comes back on every staff, because that is what it means: "from
// letter B" has to find the same bar for everybody in the room.
func TestAMarkComesBackOnEveryStaff(t *testing.T) {
	s := reimported(t, exported(t, marked(4, map[int]string{1: "C"}, 3)))
	if len(s.Parts) != 4 {
		t.Fatalf("%d staves came back", len(s.Parts))
	}
	for i, p := range s.Parts {
		if got := p.Measures[1].Marker; got != "C" {
			t.Errorf("staff %d has %q at bar 2", i+1, got)
		}
	}
}

func TestSeveralMarksKeepTheirBars(t *testing.T) {
	want := map[int]string{0: "A", 2: "B", 5: "C"}
	s := reimported(t, exported(t, marked(2, want, 7)))
	for at := 0; at < 7; at++ {
		if got := s.Parts[0].Measures[at].Marker; got != want[at] {
			t.Errorf("bar %d is marked %q, want %q", at+1, got, want[at])
		}
	}
}

// Marks are not only letters: a bar number, a word, "1a" for an insert.
func TestAMarkIsWhateverItSays(t *testing.T) {
	for _, text := range []string{"A", "AA", "12", "1a", "Verse", "B♭"} {
		s := reimported(t, exported(t, marked(1, map[int]string{0: text}, 2)))
		if got := s.Parts[0].Measures[0].Marker; got != text {
			t.Errorf("%q came back as %q", text, got)
		}
	}
}

// A mark somebody else wrote, in a file gscore did not write. The one in
// the official test suite sits on the fifth bar.
func TestAMarkIsReadFromAFileGscoreDidNotWrite(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list>
  <part id="P1">
    <measure number="1">
      <attributes><divisions>1</divisions>
        <key><fifths>0</fifths></key>
        <time><beats>4</beats><beat-type>4</beat-type></time>
        <clef><sign>G</sign><line>2</line></clef>
      </attributes>
      <note><rest/><duration>4</duration><type>whole</type></note>
    </measure>
    <measure number="2">
      <direction placement="above">
        <direction-type><rehearsal enclosure="square">A</rehearsal></direction-type>
      </direction>
      <note><rest/><duration>4</duration><type>whole</type></note>
    </measure>
  </part>
</score-partwise>`
	s := reimported(t, doc)
	if got := s.Parts[0].Measures[1].Marker; got != "A" {
		t.Errorf("the mark came back as %q", got)
	}
}

// A word over the music is not a rehearsal mark and must not become one.
func TestAWordIsNotARehearsalMark(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list>
  <part id="P1">
    <measure number="1">
      <attributes><divisions>1</divisions>
        <key><fifths>0</fifths></key>
        <time><beats>4</beats><beat-type>4</beat-type></time>
        <clef><sign>G</sign><line>2</line></clef>
      </attributes>
      <direction placement="above">
        <direction-type><words>poco rit.</words></direction-type>
      </direction>
      <note><rest/><duration>4</duration><type>whole</type></note>
    </measure>
  </part>
</score-partwise>`
	s := reimported(t, doc)
	if got := s.Parts[0].Measures[0].Marker; got != "" {
		t.Errorf("a word became the mark %q", got)
	}
}

// A score with no marks writes none, so nothing appears in files that
// never had any.
func TestAScoreWithNoMarksWritesNone(t *testing.T) {
	out := exported(t, marked(2, nil, 3))
	if strings.Contains(out, "<rehearsal>") {
		t.Error("a mark was written into a score that has none")
	}
}
