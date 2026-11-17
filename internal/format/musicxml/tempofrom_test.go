/*
 * tempofrom_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A metronome mark with two beat units and no number is one beat equalling
// another: "♩ = ♩." where the meter changes. It used to be dropped, since
// there was no number to read. The speed it sets is the one in force,
// counted in the new beat.
func TestABeatEqualToAnotherIsRead(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>2</divisions>
      <time><beats>4</beats><beat-type>4</beat-type></time>
    </attributes>
    <direction placement="above"><direction-type>
      <metronome><beat-unit>quarter</beat-unit><per-minute>96</per-minute></metronome>
    </direction-type></direction>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>8</duration><type>whole</type></note>
  </measure>
  <measure number="2">
    <attributes><time><beats>12</beats><beat-type>8</beat-type></time></attributes>
    <direction placement="above"><direction-type>
      <metronome parentheses="yes">
        <beat-unit>quarter</beat-unit>
        <beat-unit>quarter</beat-unit><beat-unit-dot/>
      </metronome>
    </direction-type></direction>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>12</duration><type>whole</type><dot/></note>
  </measure>`)
	at := s.Parts[0].Measures[1].Tempo
	if at == nil {
		t.Fatal("the mark over bar 2 was dropped")
	}
	if want := (model.Duration{Num: 1, Den: 4}); at.From != want {
		t.Errorf("the beat before is %s, want %s", at.From, want)
	}
	if want := (model.Duration{Num: 3, Den: 8}); at.Unit != want {
		t.Errorf("the new beat is %s, want %s", at.Unit, want)
	}
	if at.BPM != 96 {
		t.Errorf("the new beat goes at %d, want the 96 the old one had", at.BPM)
	}
	if !at.Parens {
		t.Error("the brackets round the mark were dropped")
	}

	// And out again, the same way.
	out := exported(t, s)
	if !strings.Contains(out, "<beat-unit>quarter</beat-unit><beat-unit>quarter</beat-unit><beat-unit-dot></beat-unit-dot>") &&
		!strings.Contains(strings.Join(strings.Fields(out), ""),
			"<beat-unit>quarter</beat-unit><beat-unit>quarter</beat-unit><beat-unit-dot></beat-unit-dot>") {
		t.Errorf("the mark was not written as two beats:\n%s", out)
	}
	back := reimported(t, out).Parts[0].Measures[1].Tempo
	if back == nil || back.From != at.From || back.Unit != at.Unit || back.BPM != 96 {
		t.Errorf("the mark came back as %+v", back)
	}
}

// Two dots are seven quarters of the note, not nine: a double-dotted half
// is a value a notehead can write, and nine eighths is not.
func TestTwoDotsOnABeatAreReadRight(t *testing.T) {
	m := &metronome{Items: []metroItem{
		{XMLName: xml.Name{Local: "beat-unit"}, Text: "half"},
		{XMLName: xml.Name{Local: "beat-unit-dot"}},
		{XMLName: xml.Name{Local: "beat-unit-dot"}},
	}}
	units := m.units()
	if len(units) != 1 {
		t.Fatalf("%d beats read, want 1", len(units))
	}
	if want := (model.Duration{Num: 7, Den: 8}); units[0] != want {
		t.Errorf("a double-dotted half read as %s, want %s", units[0], want)
	}
}
