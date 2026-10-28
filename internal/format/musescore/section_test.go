/*
 * section_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where one movement ends and another begins.
 *
 * MuseScore writes it as a layout break of subtype "section", among a
 * handful of others that are only about where the page turns. It was
 * skipped with them, and it is the one of them that is about the music: a
 * "D.C. al Fine" at the end of a minuet's trio means the beginning of the
 * minuet, and the Fine it stops at ends the movement rather than the piece.
 *
 * Without it, the Moonlight sonata played its first movement, its second,
 * went back to bar one for the D.C., stopped at the Fine halfway through
 * the second — and never played the third at all.
 */

package musescore

import "testing"

func TestASectionBreakIsReadAndThePageBreaksAreNot(t *testing.T) {
	bar := func(breakOf string) string {
		b := ""
		if breakOf != "" {
			b = `<LayoutBreak><subtype>` + breakOf + `</subtype></LayoutBreak>`
		}
		return `<Measure>` + b + `<voice><Rest><durationType>measure</durationType>` +
			`<duration>4/4</duration></Rest></voice></Measure>`
	}
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1">` +
		bar("") + bar("line") + bar("section") + bar("page") + bar("") +
		`</Staff>
  </Score>
</museScore>`

	s := read(t, doc)
	if len(s.Parts) != 1 {
		t.Fatalf("%d staves", len(s.Parts))
	}
	got := []bool{}
	for _, m := range s.Parts[0].Measures {
		got = append(got, m.EndsSection)
	}
	want := []bool{false, false, true, false, false}
	if len(got) != len(want) {
		t.Fatalf("%d bars, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bar %d ends a section: %v, want %v — a page break is "+
				"about where the paper turns and says nothing about the music",
				i+1, got[i], want[i])
		}
	}
}

func TestTheRestOfABarIsStillReadAroundASectionBreak(t *testing.T) {
	// The break is read by descending into it, so the walk has to come back
	// out and carry on with the bar.
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1"><Measure>
      <LayoutBreak><subtype>section</subtype></LayoutBreak>
      <voice>
        <Chord><durationType>quarter</durationType>
          <Note><pitch>60</pitch><tpc>14</tpc></Note></Chord>
        <Chord><durationType>quarter</durationType>
          <Note><pitch>62</pitch><tpc>16</tpc></Note></Chord>
      </voice>
    </Measure></Staff>
  </Score>
</museScore>`

	s := read(t, doc)
	m := s.Parts[0].Measures[0]
	if !m.EndsSection {
		t.Error("the break was not read")
	}
	n := 0
	for _, v := range m.Voices {
		n += len(v.Elements)
	}
	if n != 2 {
		t.Errorf("%d notes came out of the bar, want the 2 written after the break", n)
	}
}

// A movement's name is in a text frame standing between two bars.
//
// MuseScore writes the work's title in a frame at the head of the score and
// each movement's own in a frame of its own further down — "Movement III"
// over the Presto. A later frame was thrown away as a second title, so the
// movement had no name anywhere on the page and the music simply began.
func TestAMovementIsNamedByTheFrameBeforeIt(t *testing.T) {
	bar := func(before string) string {
		return before + `<Measure><voice><Rest><durationType>measure</durationType>` +
			`<duration>4/4</duration></Rest></voice></Measure>`
	}
	frame := func(style, text string) string {
		return `<VBox><Text><style>` + style + `</style><text>` + text +
			`</text></Text></VBox>`
	}
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1">` +
		bar(frame("Work Title", "A Sonata")+frame("Movement Title", "Movement I")) +
		bar("") +
		bar(frame("Movement Title", "Movement II")) +
		`</Staff>
  </Score>
</museScore>`

	s := read(t, doc)
	got := []string{}
	for _, m := range s.Parts[0].Measures {
		got = append(got, m.SectionTitle)
	}
	want := []string{"Movement I", "", "Movement II"}
	if len(got) != len(want) {
		t.Fatalf("%d bars, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bar %d is headed %q, want %q", i+1, got[i], want[i])
		}
	}
}

func TestATitleAtTheHeadIsThePiecesOwn(t *testing.T) {
	// A frame before any bar heads the whole score; one further down heads
	// what comes after it. Where the file says "title" for both, it is
	// where the frame stands that tells them apart.
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="3.01">
  <Score>
    <Part><Staff id="1"></Staff><trackName>Piano</trackName></Part>
    <Staff id="1">
      <VBox><Text><style>Title</style><text>A Sonata</text></Text></VBox>
      <Measure><voice><Rest><durationType>measure</durationType>
        <duration>4/4</duration></Rest></voice></Measure>
      <VBox><Text><style>Title</style><text>Second Movement</text></Text></VBox>
      <Measure><voice><Rest><durationType>measure</durationType>
        <duration>4/4</duration></Rest></voice></Measure>
    </Staff>
  </Score>
</museScore>`

	s := read(t, doc)
	if s.Identity.Title != "A Sonata" {
		t.Errorf("the score is called %q, want the title at the head of it", s.Identity.Title)
	}
	if got := s.Parts[0].Measures[0].SectionTitle; got != "" {
		t.Errorf("bar 1 is headed %q; the frame before it is the score's own title", got)
	}
	if got := s.Parts[0].Measures[1].SectionTitle; got != "Second Movement" {
		t.Errorf("bar 2 is headed %q, want the title written before it", got)
	}
}
