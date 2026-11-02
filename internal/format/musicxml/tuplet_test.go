/*
 * tuplet_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Tuplets, and the bar numbers a file gives itself.
 *
 * Neither was read at all. A Chopin nocturne came in with its elevens and
 * twenties as plain runs of notes — no figure over them, and no flags
 * either, because the written value of an eleventh of a bar is not a
 * length any notehead spells — and every bar of it was numbered one low.
 */

package musicxml

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// score reads a scrap of MusicXML, with the boilerplate written in.
func score(t *testing.T, measures string) *model.Score {
	t.Helper()
	doc := `<?xml version="1.0"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>
  <part id="P1">` + measures + `</part>
</score-partwise>`
	s, err := importer{}.Import(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	return s
}

// tupletsIn is every tuplet in a bar, with where each group begins and
// ends, as a line that is easy to read in a failure.
func tupletsIn(m *model.Measure) string {
	var out []string
	for _, v := range m.Voices {
		for _, el := range v.Elements {
			mk := marksOfElement(el)
			if mk == nil || mk.Tuplet == nil {
				out = append(out, "-")
				continue
			}
			s := ""
			if mk.Tuplet.Start {
				s += "["
			}
			s += itoa(mk.Tuplet.Actual) + ":" + itoa(mk.Tuplet.Normal)
			if mk.Tuplet.Hidden {
				s += "!"
			}
			if mk.Tuplet.Stop {
				s += "]"
			}
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A tuplet the file brackets itself.
func TestATupletTheFileBracketsIsRead(t *testing.T) {
	s := score(t, `<measure number="1">
    <attributes><divisions>12</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification>
      <notations><tuplet type="start" number="1"/></notations></note>
    <note><pitch><step>D</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification></note>
    <note><pitch><step>E</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification>
      <notations><tuplet type="stop" number="1"/></notations></note>
  </measure>`)
	if got, want := tupletsIn(s.Parts[0].Measures[0]), "[3:2 3:2 3:2]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A file may give the arithmetic and no brackets at all, which is legal
// and common. Without inferring the ends, a group of eleven came out as
// eleven groups of one, each with its own 11 over it.
func TestATupletWithNoBracketsStillGroups(t *testing.T) {
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification></note>`
	plain := `<note><pitch><step>G</step><octave>4</octave></pitch><duration>12</duration>
      <type>quarter</type></note>`
	s := score(t, `<measure number="1">
    <attributes><divisions>12</divisions>
      <time><beats>2</beats><beat-type>4</beat-type></time></attributes>
    `+note+note+note+plain+`</measure>`)
	if got, want := tupletsIn(s.Parts[0].Measures[0]), "[3:2 3:2 3:2] -"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Two triplets in a row are two groups, not one of six. Nothing in the
// numbers says where one ends, so it is the run in the voice that does.
func TestTwoTripletsInARowAreTwoGroups(t *testing.T) {
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification>
      <notations><tuplet type="%s" number="1"/></notations></note>`
	mid := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification></note>`
	one := strings.Replace(note, "%s", "start", 1) + mid + strings.Replace(note, "%s", "stop", 1)
	s := score(t, `<measure number="1">
    <attributes><divisions>12</divisions>
      <time><beats>2</beats><beat-type>4</beat-type></time></attributes>
    `+one+one+`</measure>`)
	want := "[3:2 3:2 3:2] [3:2 3:2 3:2]"
	if got := tupletsIn(s.Parts[0].Measures[0]); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The written value comes from <type>, not from the sounding duration.
//
// Eleven notes in the time of six eighths do not divide the divisions
// evenly, so a file rounds each one — and scaling that back up by eleven
// sixths gives a length no notehead spells. The notes then had no flags,
// so no beam, so the figure over them was given a bracket to hang from.
func TestAnAwkwardTupletKeepsItsWrittenValue(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<measure number="1">
    <attributes><divisions>480</divisions>
      <time><beats>6</beats><beat-type>4</beat-type></time></attributes>`)
	// 1440 divisions to share between eleven notes: 130 each, as a real
	// file writes it, which is ten divisions short of the whole.
	for i := 0; i < 11; i++ {
		b.WriteString(`<note><pitch><step>C</step><octave>5</octave></pitch>
      <duration>130</duration><type>eighth</type>
      <time-modification><actual-notes>11</actual-notes><normal-notes>6</normal-notes></time-modification></note>`)
	}
	b.WriteString(`</measure>`)
	s := score(t, b.String())

	el := s.Parts[0].Measures[0].Voices[0].Elements
	if len(el) != 11 {
		t.Fatalf("%d notes came in, want 11", len(el))
	}
	for i, e := range el {
		mk := marksOfElement(e)
		w := mk.WrittenDuration(e.Duration())
		if _, _, ok := w.Written(); !ok {
			t.Fatalf("note %d is written as %v, which is not a length any "+
				"notehead spells — so it gets no flag and no beam", i, w)
		}
		if w.Cmp(model.Eighth.Duration()) != 0 {
			t.Errorf("note %d is written as %v, want an eighth", i, w)
		}
	}
}

// A tuplet written down for the arithmetic and not to be read: a
// measured tremolo is six sixteenths in the time of four, and the
// strokes through the stem are how that is written, not a 6.
func TestATupletTheFileHidesIsHidden(t *testing.T) {
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification>
      <notations><tuplet type="%s" number="1" print-object="no"/></notations></note>`
	mid := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration>
      <type>eighth</type>
      <time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification></note>`
	s := score(t, `<measure number="1">
    <attributes><divisions>12</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time></attributes>
    `+strings.Replace(note, "%s", "start", 1)+mid+
		strings.Replace(note, "%s", "stop", 1)+`</measure>`)
	if got := tupletsIn(s.Parts[0].Measures[0]); !strings.Contains(got, "!") {
		t.Errorf("got %q, want the group marked as not drawn", got)
	}
}

/*
 * The bar numbers a file gives itself.
 */

// A short first bar that the file does not call implicit is bar one all
// the same, and the engraver's guess at an upbeat must not override it.
func TestAShortFirstBarThatIsNotImplicitIsStillBarOne(t *testing.T) {
	s := score(t, `<measure number="1">
    <attributes><divisions>4</divisions>
      <time><beats>4</beats><beat-type>4</beat-type></time></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><type>quarter</type></note>
  </measure>
  <measure number="2">
    <note><pitch><step>D</step><octave>4</octave></pitch><duration>16</duration><type>whole</type></note>
  </measure>`)
	from := s.Parts[0].Measures[0].NumberFrom
	if from == nil || *from != 1 {
		t.Fatalf("the first bar says %v, want it told to start at 1", from)
	}
}

// A file numbered plainly 1, 2, 3 says nothing the engraver does not work
// out for itself, so nothing is written down.
func TestAPlainlyNumberedFileCarriesNoRestarts(t *testing.T) {
	bar := `<measure number="%d">
    <note><pitch><step>C</step><octave>4</octave></pitch><duration>16</duration><type>whole</type></note>
  </measure>`
	body := `<measure number="1">
    <attributes><divisions>4</divisions>
      <time><beats>4</beats><beat-type>4</beat-type></time></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch><duration>16</duration><type>whole</type></note>
  </measure>` + strings.Replace(bar, "%d", "2", 1) + strings.Replace(bar, "%d", "3", 1)
	s := score(t, body)
	// The first bar is told where to start, because nothing else says so;
	// the ones after it simply count on.
	for i, m := range s.Parts[0].Measures {
		if i > 0 && m.NumberFrom != nil {
			t.Errorf("bar %d carries a restart at %d, and should not", i, *m.NumberFrom)
		}
	}
}

// A pickup the file does mark is left to the engraver, whose rule for one
// is the same and already right.
func TestAnImplicitFirstBarIsLeftAlone(t *testing.T) {
	s := score(t, `<measure number="0" implicit="yes">
    <attributes><divisions>4</divisions>
      <time><beats>4</beats><beat-type>4</beat-type></time></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><type>quarter</type></note>
  </measure>
  <measure number="1">
    <note><pitch><step>D</step><octave>4</octave></pitch><duration>16</duration><type>whole</type></note>
  </measure>`)
	if from := s.Parts[0].Measures[0].NumberFrom; from != nil {
		t.Errorf("a pickup the file marked was given a number of %d", *from)
	}
}
