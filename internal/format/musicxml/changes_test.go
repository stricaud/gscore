/*
 * changes_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A piece is not one key, one meter, one clef or one speed.
 *
 * All four used to be read onto the part, where each change overwrote the
 * one before it, so a file arrived in whatever it happened to say last.
 * Altès's 10me Solo is three movements in one file — E flat in 3/4, then A
 * flat in 6/8, then C in cut time — and gscore drew the whole of it, from
 * the first bar, in C and cut time. The notes were all correct and every
 * accidental in the piece was wrong.
 */

package musicxml

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// movements is the shape of that file: three sections, each stating its
// own key and meter, and the numbering starting again at each one.
const movements = `
  <measure number="1">
    <attributes><divisions>1</divisions>
      <key><fifths>-3</fifths></key>
      <time><beats>3</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <direction><direction-type><words>Allegro Moderato</words></direction-type>
      <sound tempo="114"/></direction>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>3</duration><type>half</type><dot/></note>
  </measure>
  <measure number="2">
    <note><pitch><step>D</step><octave>4</octave></pitch>
      <duration>3</duration><type>half</type><dot/></note>
  </measure>
  <measure number="1">
    <attributes>
      <key><fifths>-4</fifths></key>
      <time><beats>6</beats><beat-type>8</beat-type></time>
    </attributes>
    <direction><direction-type><words>Andante</words></direction-type>
      <sound tempo="92"/></direction>
    <note><pitch><step>E</step><octave>4</octave></pitch>
      <duration>3</duration><type>half</type><dot/></note>
  </measure>
  <measure number="1">
    <attributes>
      <key><fifths>0</fifths></key>
      <time symbol="cut"><beats>2</beats><beat-type>2</beat-type></time>
      <clef><sign>F</sign><line>4</line></clef>
    </attributes>
    <note><pitch><step>F</step><octave>4</octave></pitch>
      <duration>4</duration><type>whole</type></note>
  </measure>`

func TestThePieceStartsInWhatItStartsIn(t *testing.T) {
	s := score(t, movements)
	p := s.Parts[0]
	if p.Key.Fifths != -3 {
		t.Errorf("the piece opens in %d flats; the file says three",
			-p.Key.Fifths)
	}
	if p.Time.Beats != 3 || p.Time.BeatValue != 4 {
		t.Errorf("the piece opens in %d/%d; the file says 3/4",
			p.Time.Beats, p.Time.BeatValue)
	}
	if p.Clef != model.Treble {
		t.Errorf("the piece opens in clef %v; the file says treble", p.Clef)
	}
}

func TestEveryChangeIsWrittenOnTheBarItHappensAt(t *testing.T) {
	s := score(t, movements)
	p := s.Parts[0]
	if len(p.Measures) != 4 {
		t.Fatalf("%d bars, not 4", len(p.Measures))
	}

	// Bar 2 says nothing, so nothing changes there.
	if p.Measures[1].Key != nil || p.Measures[1].Time != nil {
		t.Error("bar 2 changes a signature it does not state")
	}

	// The second movement.
	if k := p.Measures[2].Key; k == nil || k.Fifths != -4 {
		t.Errorf("bar 3 should change to four flats, got %v", k)
	}
	if m := p.Measures[2].Time; m == nil || m.Beats != 6 || m.BeatValue != 8 {
		t.Errorf("bar 3 should change to 6/8, got %v", m)
	}
	if p.Measures[2].Clef != nil {
		t.Error("bar 3 changes a clef it does not state")
	}

	// The third, which also changes clef.
	if k := p.Measures[3].Key; k == nil || k.Fifths != 0 {
		t.Errorf("bar 4 should change to no key, got %v", k)
	}
	if m := p.Measures[3].Time; m == nil || m.Symbol != model.TimeCut {
		t.Errorf("bar 4 should change to cut time, got %v", m)
	}
	if c := p.Measures[3].Clef; c == nil || *c != model.Bass {
		t.Errorf("bar 4 should change to the bass clef, got %v", c)
	}
}

// A signature stated again without changing is not a change: a file writes
// the key out at the head of every system, and a sign drawn in the middle
// of the music saying what it already said is wrong.
func TestASignatureSaidTwiceIsNotAChange(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>1</divisions>
      <key><fifths>2</fifths></key>
      <time><beats>4</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>4</duration><type>whole</type></note>
  </measure>
  <measure number="2">
    <attributes>
      <key><fifths>2</fifths></key>
      <time><beats>4</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef>
    </attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>4</duration><type>whole</type></note>
  </measure>`)
	m := s.Parts[0].Measures[1]
	if m.Key != nil || m.Time != nil || m.Clef != nil {
		t.Errorf("bar 2 restates what it was already in and was read as a "+
			"change: key %v time %v clef %v", m.Key, m.Time, m.Clef)
	}
}

// A word with a speed behind it and no metronome mark: what MuseScore
// writes for every tempo nobody asked to see printed. Read as a metronome
// mark alone, this set no speed at all and threw the word away with it —
// and then took the piece's tempo from a mark twenty bars later.
func TestAWordCanSetTheSpeed(t *testing.T) {
	s := score(t, movements)
	if s.Tempo.BPM != 114 {
		t.Errorf("the piece plays at %d, and the file says 114", s.Tempo.BPM)
	}
	if s.Tempo.Text != "Allegro Moderato" {
		t.Errorf("the mark over bar 1 is %q, not %q",
			s.Tempo.Text, "Allegro Moderato")
	}
	if !s.Tempo.Silent {
		t.Error("a word with no metronome mark behind it should print as " +
			"the word alone")
	}
	at := s.Parts[0].Measures[2].Tempo
	if at == nil || at.BPM != 92 || at.Text != "Andante" {
		t.Errorf("the second movement's speed is %v, and the file says "+
			"Andante at 92", at)
	}
}

// Marks written on a chord.
//
// A chord is several <note> elements and one event, and a mark that
// belongs to all of it may be written on any one of them. gscore keeps a
// chord's marks on the chord and reads only those, so everything written
// on the second notehead onward was read and thrown away.
func TestAMarkOnAnyNoteheadIsAMarkOnTheChord(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>2</divisions>
      <time><beats>2</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>2</duration><type>quarter</type></note>
    <note><chord/><pitch><step>E</step><octave>4</octave></pitch>
      <duration>2</duration><type>quarter</type>
      <notations><arpeggiate/></notations>
      <lyric><text>oh</text></lyric></note>
    <note><chord/><pitch><step>G</step><octave>4</octave></pitch>
      <duration>2</duration><type>quarter</type>
      <notations><articulations><staccato/></articulations>
        <technical><fingering>3</fingering></technical></notations></note>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>2</duration><type>quarter</type></note>
  </measure>`)
	el := s.Parts[0].Measures[0].Voices[0].Elements[0]
	ch, ok := el.(*model.Chord)
	if !ok {
		t.Fatalf("the first element is %T, not a chord", el)
	}
	if ch.Arpeggio == nil {
		t.Error("the chord is spread and came back unspread")
	}
	if len(ch.Lyrics) != 1 || ch.Lyrics[0].Text != "oh" {
		t.Errorf("the syllable under the chord is %v", ch.Lyrics)
	}
	if !ch.HasArticulation(model.Staccato) {
		t.Error("the chord is staccato and came back plain")
	}
	if len(ch.Fingering) != 1 || ch.Fingering[0] != "3" {
		t.Errorf("the fingering on the chord is %v", ch.Fingering)
	}
}

// A triplet of chords.
//
// The chord takes the first notehead's marks by value, and a tuplet is not
// settled until the whole voice has been read — so the figure was written
// onto a note that was no longer in the music. Every triplet in the piano
// part of the 10me Solo came out as three chords of the right length with
// nothing over them.
func TestAChordCanBeInATuplet(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<measure number="1">
    <attributes><divisions>6</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>`)
	for i := 0; i < 3; i++ {
		for _, step := range []string{"C", "E", "G"} {
			b.WriteString(`<note>`)
			if step != "C" {
				b.WriteString(`<chord/>`)
			}
			b.WriteString(`<pitch><step>` + step + `</step><octave>4</octave></pitch>
        <duration>2</duration><type>eighth</type>
        <time-modification><actual-notes>3</actual-notes>
          <normal-notes>2</normal-notes></time-modification></note>`)
		}
	}
	b.WriteString(`</measure>`)

	s := score(t, b.String())
	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 3 {
		t.Fatalf("%d elements, not three chords", len(els))
	}
	for i, el := range els {
		ch, ok := el.(*model.Chord)
		if !ok {
			t.Fatalf("element %d is %T, not a chord", i, el)
		}
		if ch.Tuplet == nil {
			t.Fatalf("chord %d is in a triplet and came back without one", i)
		}
		if ch.Tuplet.Actual != 3 || ch.Tuplet.Normal != 2 {
			t.Errorf("chord %d is a %d:%d, not a 3:2",
				i, ch.Tuplet.Actual, ch.Tuplet.Normal)
		}
		if want := i == 0; ch.Tuplet.Start != want {
			t.Errorf("chord %d: start %v, wanted %v", i, ch.Tuplet.Start, want)
		}
		if want := i == 2; ch.Tuplet.Stop != want {
			t.Errorf("chord %d: stop %v, wanted %v", i, ch.Tuplet.Stop, want)
		}
	}
}

// A fermata over the pause at the end, which is how a good half of the
// repertoire finishes. A rest carried no marks at all.
func TestARestIsMarkedToo(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>1</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>
    <note><rest/><duration>1</duration><type>quarter</type>
      <notations><fermata/></notations>
      <lyric><text>ah</text></lyric></note>
  </measure>`)
	el := s.Parts[0].Measures[0].Voices[0].Elements[0]
	r, ok := el.(*model.Rest)
	if !ok {
		t.Fatalf("the element is %T, not a rest", el)
	}
	if !r.HasArticulation(model.Fermata) {
		t.Error("the pause has a fermata over it and came back bare")
	}
	if len(r.Lyrics) != 1 {
		t.Errorf("the syllable over the rest is %v", r.Lyrics)
	}
}

// A hairpin that opens and closes on the same note. The model holds a span
// as one mark with an end at each side, and writing the second end over
// the first left a crescendo that only ever stopped.
func TestASpanCanBeginAndEndOnOneNote(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>1</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>
    <direction><direction-type><wedge type="diminuendo"/></direction-type></direction>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
    <direction><direction-type><wedge type="stop"/></direction-type></direction>
  </measure>`)
	el := s.Parts[0].Measures[0].Voices[0].Elements[0]
	h := marksOfElement(el).Hairpin
	if h == nil {
		t.Fatal("the hairpin did not arrive at all")
	}
	if !h.Start || !h.Stop {
		t.Errorf("the hairpin starts %v and stops %v; it does both here",
			h.Start, h.Stop)
	}
	if h.Kind != model.Diminuendo {
		t.Errorf("the hairpin is a %v, and the file says diminuendo", h.Kind)
	}
}

// One <direction> holding several <direction-type>: "cresc." and the
// dashed line that carries it, a dynamic and the wedge that follows it.
// Read as a single element, which is what Go's decoder does with a
// repeated one, only the last of them survived.
func TestOneDirectionCanSayMoreThanOneThing(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>1</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>
    <direction placement="below">
      <direction-type><dynamics><p/></dynamics></direction-type>
      <direction-type><wedge type="crescendo"/></direction-type>
      <direction-type><words>poco a poco</words></direction-type>
    </direction>
    <note><pitch><step>C</step><octave>4</octave></pitch>
      <duration>1</duration><type>quarter</type></note>
  </measure>`)
	m := marksOfElement(s.Parts[0].Measures[0].Voices[0].Elements[0])
	if m.Dynamic != model.Piano {
		t.Errorf("the dynamic is %v, and the file says p", m.Dynamic)
	}
	if m.Hairpin == nil || !m.Hairpin.Start {
		t.Error("the crescendo did not arrive")
	}
	if len(m.Texts) != 1 || m.Texts[0].Text != "poco a poco" {
		t.Errorf("the words are %v", m.Texts)
	}
}

// A part on two staves may carry a different key on each.
func TestEachStaffKeepsItsOwnSignature(t *testing.T) {
	s := score(t, `
  <measure number="1">
    <attributes><divisions>1</divisions><staves>2</staves>
      <key number="1"><fifths>3</fifths></key>
      <key number="2"><fifths>-2</fifths></key>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef number="1"><sign>G</sign><line>2</line></clef>
      <clef number="2"><sign>F</sign><line>4</line></clef>
    </attributes>
    <note><pitch><step>C</step><octave>5</octave></pitch>
      <duration>1</duration><type>quarter</type><staff>1</staff></note>
    <backup><duration>1</duration></backup>
    <note><pitch><step>C</step><octave>3</octave></pitch>
      <duration>1</duration><type>quarter</type><staff>2</staff></note>
  </measure>`)
	if len(s.Parts) != 2 {
		t.Fatalf("%d staves, not 2", len(s.Parts))
	}
	if s.Parts[0].Key.Fifths != 3 {
		t.Errorf("the upper staff is in %d sharps, and the file says 3",
			s.Parts[0].Key.Fifths)
	}
	if s.Parts[1].Key.Fifths != -2 {
		t.Errorf("the lower staff is in %d, and the file says two flats",
			s.Parts[1].Key.Fifths)
	}
}

// What is printed on the first page and what the catalogue says are two
// different things, and the subtitle is only ever in the first. "subtitle"
// is not a kind of creator and never matched anything, so every file's
// subtitle was lost and no file's arranger was ever read.
func TestTheHeaderIsWhatIsPrintedOnThePage(t *testing.T) {
	doc := `<?xml version="1.0"?>
<score-partwise version="4.0">
  <work><work-title>10me Solo pour la Flute</work-title></work>
  <identification>
    <creator type="arranger">Rainer Marwan</creator>
    <creator type="composer">Joseph Henry Altès</creator>
    <miscellaneous>
      <miscellaneous-field name="subtitle">Op. 36</miscellaneous-field>
    </miscellaneous>
  </identification>
  <credit><credit-type>composer</credit-type>
    <credit-words>Joseph Henry Altès
Bearbeitet von: Rainer Marwan (2025)</credit-words></credit>
  <part-list><score-part id="P1"><part-name>Flute</part-name></score-part></part-list>
  <part id="P1"><measure number="1">
    <attributes><divisions>1</divisions>
      <time><beats>1</beats><beat-type>4</beat-type></time>
      <clef><sign>G</sign><line>2</line></clef></attributes>
    <note><rest/><duration>1</duration><type>quarter</type></note>
  </measure></part>
</score-partwise>`
	s, err := importer{}.Import(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if s.Identity.Title != "10me Solo pour la Flute" {
		t.Errorf("title %q", s.Identity.Title)
	}
	if s.Identity.Subtitle != "Op. 36" {
		t.Errorf("subtitle %q, and the file says Op. 36", s.Identity.Subtitle)
	}
	want := "Joseph Henry Altès\nBearbeitet von: Rainer Marwan (2025)"
	if s.Identity.Composer != want {
		t.Errorf("the credit block is\n%q\nand the page says\n%q",
			s.Identity.Composer, want)
	}
}
