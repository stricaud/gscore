/*
 * musescore_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package musescore

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/format"
	"github.com/stricaud/gscore/internal/model"
)

// A score with one of everything this reader claims to understand, written
// the way MuseScore 4 writes it: the title in the frame rather than in the
// metadata, a part whose staves carry no id, a transposing instrument, and
// spanners that say where they go at both ends.
const sample = `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="4.50">
  <Score>
    <metaTag name="workTitle">Untitled score</metaTag>
    <metaTag name="composer">Composer / arranger</metaTag>
    <metaTag name="copyright">Public domain</metaTag>
    <Part id="1">
      <Staff>
        <StaffType group="pitched"><name>stdNormal</name></StaffType>
      </Staff>
      <trackName>Alto Saxophone</trackName>
      <Instrument id="alto-saxophone">
        <longName>Alto Saxophone</longName>
        <transposeDiatonic>-5</transposeDiatonic>
        <transposeChromatic>-9</transposeChromatic>
        <Channel><program value="65"/></Channel>
      </Instrument>
    </Part>
    <Part id="2">
      <Staff>
        <bracket type="1" span="2" col="0" visible="1"/>
        <barLineSpan>1</barLineSpan>
        <StaffType group="pitched"><name>stdNormal</name></StaffType>
      </Staff>
      <Staff>
        <defaultClef>F</defaultClef>
        <StaffType group="pitched"><name>stdNormal</name></StaffType>
      </Staff>
      <trackName>Piano</trackName>
      <Instrument id="piano">
        <longName>Piano</longName>
        <clef staff="2">F</clef>
        <Channel><program value="0"/></Channel>
      </Instrument>
    </Part>
    <Staff id="1">
      <VBox>
        <Text><style>title</style><text>A Little Tune</text></Text>
        <Text><style>composer</style><text>Nobody</text></Text>
      </VBox>
      <Measure>
        <voice>
          <KeySig><concertKey>-2</concertKey></KeySig>
          <TimeSig><sigN>3</sigN><sigD>4</sigD></TimeSig>
          <Tempo>
            <tempo>1.6</tempo>
            <text><sym>metNoteQuarterUp</sym> = 96</text>
          </Tempo>
          <Dynamic><subtype>mf</subtype></Dynamic>
          <Chord>
            <durationType>quarter</durationType>
            <Articulation><subtype>articStaccatoAbove</subtype></Articulation>
            <Spanner type="Slur">
              <Slur><eid>x</eid></Slur>
              <next><location><fractions>1/2</fractions></location></next>
            </Spanner>
            <Lyrics><no>0</no><syllabic>begin</syllabic><text>Hel</text></Lyrics>
            <Note><pitch>60</pitch><tpc>14</tpc><tpc2>17</tpc2></Note>
          </Chord>
          <Chord>
            <durationType>quarter</durationType>
            <dots>1</dots>
            <Note>
              <pitch>63</pitch><tpc>11</tpc><tpc2>14</tpc2>
              <Spanner type="Tie">
                <Tie><eid>y</eid></Tie>
                <next><location><fractions>1/4</fractions></location></next>
              </Spanner>
            </Note>
          </Chord>
          <Rest><durationType>eighth</durationType></Rest>
        </voice>
      </Measure>
      <Measure>
        <startRepeat/>
        <voice>
          <Spanner type="Volta">
            <Volta><beginText>1.</beginText><endings>1</endings></Volta>
            <next><location><measures>1</measures></location></next>
          </Spanner>
          <Tuplet><normalNotes>2</normalNotes><actualNotes>3</actualNotes>
            <baseNote>eighth</baseNote></Tuplet>
          <Chord>
            <durationType>eighth</durationType>
            <Note><pitch>65</pitch><tpc>13</tpc><tpc2>16</tpc2></Note>
          </Chord>
          <Chord>
            <durationType>eighth</durationType>
            <Note><pitch>67</pitch><tpc>15</tpc><tpc2>18</tpc2></Note>
          </Chord>
          <Chord>
            <durationType>eighth</durationType>
            <Note><pitch>69</pitch><tpc>17</tpc><tpc2>20</tpc2></Note>
          </Chord>
          <endTuplet/>
          <Chord>
            <acciaccatura/>
            <durationType>eighth</durationType>
            <Note><pitch>71</pitch><tpc>19</tpc><tpc2>22</tpc2></Note>
          </Chord>
          <Chord>
            <durationType>half</durationType>
            <Spanner type="Slur"><prev><location><fractions>-1/2</fractions></location></prev></Spanner>
            <Note><pitch>72</pitch><tpc>14</tpc><tpc2>17</tpc2></Note>
            <Note><pitch>76</pitch><tpc>18</tpc><tpc2>21</tpc2></Note>
          </Chord>
        </voice>
        <endRepeat>2</endRepeat>
      </Measure>
    </Staff>
    <Staff id="2">
      <Measure>
        <voice><Rest><durationType>measure</durationType><duration>3/4</duration></Rest></voice>
      </Measure>
      <Measure>
        <voice><Rest><durationType>measure</durationType><duration>3/4</duration></Rest></voice>
      </Measure>
    </Staff>
    <Staff id="3">
      <Measure>
        <voice><Rest><durationType>measure</durationType><duration>3/4</duration></Rest></voice>
      </Measure>
      <Measure>
        <voice><Rest><durationType>measure</durationType><duration>3/4</duration></Rest></voice>
      </Measure>
    </Staff>
  </Score>
</museScore>`

func read(t *testing.T, text string) *model.Score {
	t.Helper()
	s, err := Import(strings.NewReader(text))
	if err != nil {
		if _, ok := err.(format.Partial); !ok {
			t.Fatalf("Import: %v", err)
		}
		t.Logf("partial read: %v", err)
	}
	if s == nil {
		t.Fatal("no score")
	}
	return s
}

func TestTheScoreItselfIsRead(t *testing.T) {
	s := read(t, sample)

	// The title is in the frame at the head of the page. The metadata still
	// says "Untitled score", which is MuseScore's placeholder and not a
	// title; printing it over a piece that has a name would be worse than
	// printing nothing.
	if s.Identity.Title != "A Little Tune" {
		t.Errorf("title is %q", s.Identity.Title)
	}
	if s.Identity.Composer != "Nobody" {
		t.Errorf("composer is %q", s.Identity.Composer)
	}
	if s.Identity.Copyright != "Public domain" {
		t.Errorf("copyright is %q", s.Identity.Copyright)
	}

	// MuseScore keeps the tempo as quarter notes a second.
	if s.Tempo.BPM != 96 {
		t.Errorf("tempo is %d, want 96 from 1.6 a second", s.Tempo.BPM)
	}
	// The metronome mark is drawn from the numbers, so the symbol that
	// spells it in the file is not words and must not be kept as any.
	if s.Tempo.Text != "" {
		t.Errorf("tempo text is %q, want nothing", s.Tempo.Text)
	}
}

func TestEachStaffBecomesAPart(t *testing.T) {
	s := read(t, sample)

	if len(s.Parts) != 3 {
		t.Fatalf("got %d parts, want the saxophone and the piano's two staves", len(s.Parts))
	}
	if s.Parts[0].Name != "Alto Saxophone" {
		t.Errorf("the first part is %q", s.Parts[0].Name)
	}
	if s.Parts[1].Name != "Piano" {
		t.Errorf("the piano's right hand is %q", s.Parts[1].Name)
	}
	// A piano is one instrument on two staves: naming both says it is two.
	if s.Parts[2].Name != "" {
		t.Errorf("the piano's left hand is named %q", s.Parts[2].Name)
	}

	// The clef a staff starts in is usually not written into its bars at
	// all: it comes from the instrument, and reading the bars alone put a
	// piano's left hand in the treble.
	if s.Parts[1].Clef != model.Treble || s.Parts[2].Clef != model.Bass {
		t.Errorf("the piano is in %v and %v", s.Parts[1].Clef, s.Parts[2].Clef)
	}

	if len(s.Groups) != 1 {
		t.Fatalf("got %d groups, want the piano braced", len(s.Groups))
	}
	g := s.Groups[0]
	if g.First != 1 || g.Last != 2 || g.Symbol != model.GroupBrace || !g.Barlines {
		t.Errorf("the brace is %+v", g)
	}

	if s.Parts[0].Instrument != model.AltoSax {
		t.Errorf("the saxophone sounds as program %d", s.Parts[0].Instrument)
	}
	if k := s.Parts[0].Key; k.Fifths != -2 {
		t.Errorf("the key is %d flats, want two", -k.Fifths)
	}
	if ts := s.Parts[0].Time; ts.Beats != 3 || ts.BeatValue != 4 {
		t.Errorf("the meter is %v", ts)
	}
}

// MuseScore records the pitch that sounds. gscore records the pitch that is
// printed, and on a transposing staff those are different notes: an alto
// saxophone's written C sounds the E flat a major sixth below it. Reading the
// sounding pitch onto the page puts every note of the part in the wrong
// place.
func TestATransposingPartIsReadAsItIsWritten(t *testing.T) {
	s := read(t, sample)
	sax := s.Parts[0]

	if tr := sax.Transpose; tr.Diatonic != -5 || tr.Chromatic != -9 {
		t.Errorf("the transposition is %+v", tr)
	}

	first := firstNote(t, sax)
	// <pitch>60</pitch> sounds middle C; written on an alto it is the A above.
	want := model.Pitch{Step: model.StepA, Octave: 4}
	if first.Pitch != want {
		t.Errorf("the first note is written %v, want %v", first.Pitch, want)
	}
	// And it sounds what the file said it sounds.
	if got := sax.Transpose.Apply(first.Pitch).MIDI(); got != 60 {
		t.Errorf("it sounds as MIDI %d, want 60", got)
	}
}

func TestNotesRestsAndTheirMarks(t *testing.T) {
	s := read(t, sample)
	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 3 {
		t.Fatalf("got %d elements in the first bar, want two notes and a rest", len(els))
	}

	n0, ok := els[0].(*model.Note)
	if !ok {
		t.Fatalf("the first element is %T", els[0])
	}
	if n0.Dur != (model.Duration{Num: 1, Den: 4}) {
		t.Errorf("the first note lasts %v", n0.Dur)
	}
	if !n0.HasArticulation(model.Staccato) {
		t.Error("the staccato was not read")
	}
	if n0.Dynamic != model.MF {
		t.Errorf("the dynamic is %v", n0.Dynamic)
	}
	if !n0.SlurStart {
		t.Error("the slur does not start on the first note")
	}
	if len(n0.Lyrics) != 1 || n0.Lyrics[0].Text != "Hel" ||
		n0.Lyrics[0].Verse != 1 || n0.Lyrics[0].Syllabic != model.SyllableBegin {
		t.Errorf("the lyric is %+v", n0.Lyrics)
	}

	n1 := els[1].(*model.Note)
	// A dotted quarter is a quarter and an eighth.
	if n1.Dur != (model.Duration{Num: 3, Den: 8}) {
		t.Errorf("the dotted quarter lasts %v", n1.Dur)
	}
	if !n1.TieStart {
		t.Error("the tie does not start")
	}
	// It sounds E flat; on an alto that is written as the C a major sixth
	// above, which is what tpc2 spells and tpc does not.
	if want := (model.Pitch{Step: model.StepC, Octave: 5}); n1.Pitch != want {
		t.Errorf("the second note is written %v, want %v", n1.Pitch, want)
	}

	if _, ok := els[2].(*model.Rest); !ok {
		t.Errorf("the third element is %T, want a rest", els[2])
	}
}

func TestTupletsChordsAndGraceNotes(t *testing.T) {
	s := read(t, sample)
	els := s.Parts[0].Measures[1].Voices[0].Elements
	if len(els) != 4 {
		t.Fatalf("got %d elements, want three triplet eighths and a chord", len(els))
	}

	for i := 0; i < 3; i++ {
		n := els[i].(*model.Note)
		// A triplet eighth sounds a twelfth of a whole note while the
		// notehead says an eighth, which is the whole reason the ratio has
		// to be stored beside the duration.
		if n.Dur != (model.Duration{Num: 1, Den: 12}) {
			t.Errorf("triplet note %d lasts %v, want 1/12", i, n.Dur)
		}
		if n.Tuplet == nil || n.Tuplet.Actual != 3 || n.Tuplet.Normal != 2 {
			t.Errorf("triplet note %d carries %+v", i, n.Tuplet)
			continue
		}
		if (i == 0) != n.Tuplet.Start {
			t.Errorf("triplet note %d has Start=%v", i, n.Tuplet.Start)
		}
		if (i == 2) != n.Tuplet.Stop {
			t.Errorf("triplet note %d has Stop=%v", i, n.Tuplet.Stop)
		}
		if written := n.WrittenDuration(n.Dur); written != (model.Duration{Num: 1, Den: 8}) {
			t.Errorf("triplet note %d is written as %v, want an eighth", i, written)
		}
	}

	c, ok := els[3].(*model.Chord)
	if !ok {
		t.Fatalf("the last element is %T, want a chord", els[3])
	}
	if len(c.Notes) != 2 {
		t.Errorf("the chord has %d notes", len(c.Notes))
	}
	if !c.SlurStop {
		t.Error("the slur does not close on the chord")
	}
	// A grace note takes no time out of the bar, so it hangs off the note it
	// leans on rather than standing in the voice beside it.
	if c.Grace == nil || len(c.Grace.Notes) != 1 {
		t.Fatalf("the grace note is %+v", c.Grace)
	}
	if !c.Grace.Slashed {
		t.Error("an acciaccatura is slashed")
	}
}

func TestRepeatsAndVoltas(t *testing.T) {
	s := read(t, sample)
	ms := s.Parts[0].Measures

	// A repeat opens on the barline between the first bar and the second,
	// and gscore keeps a barline on the bar it closes.
	if ms[0].RightBarline != model.BarRepeatStart {
		t.Errorf("the first bar closes with %v", ms[0].RightBarline)
	}
	if ms[1].RightBarline != model.BarRepeatEnd {
		t.Errorf("the second bar closes with %v", ms[1].RightBarline)
	}
	e := ms[1].Ending
	if e == nil {
		t.Fatal("the volta bracket was not read")
	}
	if !e.Start || len(e.Numbers) != 1 || e.Numbers[0] != 1 {
		t.Errorf("the bracket is %+v", e)
	}
}

// A .mscz is a zip holding the .mscx this reader actually wants, beside a
// style sheet, a thumbnail and the playback settings. Opening one is the
// whole point: nobody keeps their work as loose XML.
func TestAZippedScoreReadsTheSame(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"META-INF/container.xml":   `<container><rootfiles></rootfiles></container>`,
		"score_style.mss":          "<museScore/>",
		"Thumbnails/thumbnail.png": "not really a png",
		"A Little Tune.mscx":       sample,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		if _, ok := err.(format.Partial); !ok {
			t.Fatalf("Import: %v", err)
		}
	}
	if s == nil || s.Identity.Title != "A Little Tune" || len(s.Parts) != 3 {
		t.Fatalf("the zipped score came back as %+v", s)
	}
}

// What the reader cannot use it says, rather than refusing the file or
// dropping it in silence. One tremolo is not a reason to lose a symphony.
func TestWhatCannotBeReadIsReported(t *testing.T) {
	doc := strings.Replace(sample,
		`<Rest><durationType>eighth</durationType></Rest>`,
		`<Rest><durationType>eighth</durationType></Rest>
		 <Spanner type="Vibrato"><Vibrato><subtype>guitarVibrato</subtype></Vibrato>
		   <next><location><measures>1</measures></location></next></Spanner>`, 1)

	s, err := Import(strings.NewReader(doc))
	if s == nil {
		t.Fatal("the score was refused over something it could do without")
	}
	p, ok := err.(format.Partial)
	if !ok {
		t.Fatalf("err is %v, want a partial read", err)
	}
	found := false
	for _, d := range p.Details() {
		if strings.Contains(d, "Vibrato") {
			found = true
		}
	}
	if !found {
		t.Errorf("the vibrato was dropped without a word: %v", p.Details())
	}
}

// A file that is not a MuseScore file at all is a different thing from one
// that is and has a tremolo in it, and only the first is worth refusing.
func TestSomethingElseEntirelyIsRefused(t *testing.T) {
	if _, err := Import(strings.NewReader("X:1\nT:not this\n")); err == nil {
		t.Error("an ABC file was read as a MuseScore one")
	}
	if _, err := Import(strings.NewReader("")); err == nil {
		t.Error("an empty file was read as a score")
	}
}

// The tonal pitch class is how MuseScore says which letter a note is written
// as, and it has to be read with the MIDI number rather than instead of it:
// B sharp and C flat are written in one octave and sound in the next.
func TestTonalPitchClassesSpellTheNote(t *testing.T) {
	cases := []struct {
		tpc, midi int
		want      model.Pitch
	}{
		{14, 60, model.Pitch{Step: model.StepC, Octave: 4}},
		{13, 65, model.Pitch{Step: model.StepF, Octave: 4}},
		{20, 66, model.Pitch{Step: model.StepF, Octave: 4, Alter: 1}},
		{11, 63, model.Pitch{Step: model.StepE, Octave: 4, Alter: -1}},
		// B sharp sounds as the C above it and is written in the octave below.
		{19 + 7, 72, model.Pitch{Step: model.StepB, Octave: 4, Alter: 1}},
		// C flat, the other way round: it sounds as the B below it.
		{14 - 7, 59, model.Pitch{Step: model.StepC, Octave: 4, Alter: -1}},
	}
	for _, c := range cases {
		got, ok := tpcPitch(c.tpc, c.midi)
		if !ok {
			t.Errorf("tpc %d was refused", c.tpc)
			continue
		}
		if got != c.want {
			t.Errorf("tpc %d at MIDI %d spelled %v, want %v", c.tpc, c.midi, got, c.want)
		}
		if got.MIDI() != c.midi {
			t.Errorf("tpc %d spelled %v, which sounds %d and not %d",
				c.tpc, got, got.MIDI(), c.midi)
		}
	}
	if _, ok := tpcPitch(-99, 60); ok {
		t.Error("a nonsense pitch class was accepted")
	}
}

func TestTheImporterIsRegistered(t *testing.T) {
	imp, err := format.ImporterFor("something.mscz")
	if err != nil {
		t.Fatalf("no importer claims .mscz: %v", err)
	}
	if imp.Name() != "MuseScore" {
		t.Errorf(".mscz is claimed by %s", imp.Name())
	}
	if _, err := format.ImporterFor("something.mscx"); err != nil {
		t.Errorf("no importer claims .mscx: %v", err)
	}
}

func firstNote(t *testing.T, p *model.Part) *model.Note {
	t.Helper()
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if n, ok := el.(*model.Note); ok {
					return n
				}
			}
		}
	}
	t.Fatal("the part has no notes")
	return nil
}

// MuseScore records a barline only where it is not the one that would have
// been drawn anyway, and the last bar of a score is drawn with a final
// barline. So the end of the piece is in the file by being absent from it,
// and reading the file literally ended every score on a plain barline —
// which on a page says "turn over" rather than "that is the end".
func TestTheScoreEndsOnAFinalBarline(t *testing.T) {
	// The sample's last bar closes a repeat, so that is how it ends: what
	// the file does say outranks what it leaves out.
	s := read(t, sample)
	s.Normalise()
	for i, p := range s.Parts {
		if got := p.Measures[len(p.Measures)-1].RightBarline; got != model.BarRepeatEnd {
			t.Errorf("part %d ends on %v, want the repeat the file wrote", i, got)
		}
	}

	// Take the repeat away and there is nothing left to say how it ends, so
	// it ends the way MuseScore draws it.
	plain := strings.Replace(sample, "<endRepeat>2</endRepeat>", "", 1)
	s = read(t, plain)
	s.Normalise()
	for i, p := range s.Parts {
		if got := p.Measures[len(p.Measures)-1].RightBarline; got != model.BarFinal {
			t.Errorf("part %d ends on %v, want a final barline", i, got)
		}
	}
}

// A repeat is written on the first staff and meant for the score. The model
// spreads it, so this is really a check that nothing here gets in the way of
// that — the importer must leave the repeat where the file put it rather than
// inventing one somewhere else.
func TestARepeatReadFromOneStaffReachesThemAll(t *testing.T) {
	s := read(t, sample)
	s.Normalise()

	for i, p := range s.Parts {
		if len(p.Measures) < 2 {
			t.Fatalf("part %d has %d bars", i, len(p.Measures))
		}
		if got := p.Measures[0].RightBarline; got != model.BarRepeatStart {
			t.Errorf("part %d opens the repeat as %v", i, got)
		}
		if got := p.Measures[1].RightBarline; got != model.BarRepeatEnd {
			t.Errorf("part %d closes the repeat as %v", i, got)
		}
	}
}
