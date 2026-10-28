/*
 * oldspanner_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Long marks, the way MuseScore 1 and 2 wrote them.
 *
 * A hairpin, a volta, a trill and an octave line all say "from here to
 * there", and those versions write them as a pair: the mark itself where it
 * begins, carrying an id, and an <endSpanner> naming that id at the tick
 * where it stops. MuseScore 3 replaced the pair with one <Spanner> element
 * written out at both ends, and that was the only form this reader knew.
 *
 * So the Moonlight sonata opened with all ninety-eight of its hairpins,
 * both of its voltas and every one of its trills gone, and four lines of
 * warning that named no bar to look in.
 */

package musescore

import (
	"strconv"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// oldScore wraps a single bass-staff bar written in the MuseScore 2 form,
// where the voices of a bar are not wrapped in <voice> at all.
func oldScore(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="2.06">
  <Score>
    <Part><Staff id="1"><defaultClef>F</defaultClef></Staff><trackName>Piano</trackName></Part>
    <Staff id="1">` + body + `</Staff>
  </Score>
</museScore>`
}

// aQuarter is one quarter note at the given pitch.
func aQuarter(pitch int) string {
	return `<Chord><durationType>quarter</durationType>` +
		`<Note><pitch>` + strconv.Itoa(pitch) + `</pitch><tpc>14</tpc></Note></Chord>`
}

// marksOfBar lists the marks of every element of a bar, in order.
func marksOfBar(s *model.Score, part, bar int) []*model.Marks {
	var out []*model.Marks
	for _, v := range s.Parts[part].Measures[bar].Voices {
		for _, el := range v.Elements {
			out = append(out, marksOf(el))
		}
	}
	return out
}

func TestAHairpinWrittenTheOldWayIsRead(t *testing.T) {
	s := read(t, oldScore(`<Measure>
		<HairPin id="8"><subtype>0</subtype>
			<beginText><text>cresc.</text></beginText></HairPin>`+
		aQuarter(60)+aQuarter(62)+aQuarter(64)+
		`<endSpanner id="8"></endSpanner>`+aQuarter(65)+`</Measure>`))

	got := marksOfBar(s, 0, 0)
	if len(got) != 4 {
		t.Fatalf("%d notes in the bar, want 4", len(got))
	}
	if h := got[0].Hairpin; h == nil || !h.Start || h.Kind != model.Crescendo {
		t.Errorf("the first note carries %+v, want a crescendo opening", h)
	}
	if h := got[2].Hairpin; h == nil || !h.Stop {
		t.Errorf("the third note carries %+v, want the wedge closing on it", h)
	}
	if got[3].Hairpin != nil {
		t.Error("the note after the <endSpanner> is inside the hairpin")
	}
}

func TestAHairpinsDirectionIsItsSubtype(t *testing.T) {
	s := read(t, oldScore(`<Measure>
		<HairPin id="3"><subtype>1</subtype></HairPin>`+
		aQuarter(60)+`<endSpanner id="3"></endSpanner>`+aQuarter(62)+`</Measure>`))
	h := marksOfBar(s, 0, 0)[0].Hairpin
	if h == nil || h.Kind != model.Diminuendo {
		t.Errorf("subtype 1 came out as %+v, want a diminuendo", h)
	}
}

// A volta's <endSpanner> is written at the tick the bracket stops, which is
// the start of the bar after the one it covers — so the bracket is over the
// bar before, not the bar the element is written in.
func TestAVoltaCoversTheBarBeforeItsEnd(t *testing.T) {
	s := read(t, oldScore(
		`<Measure>`+aQuarter(60)+`</Measure>`+
			`<Measure><Volta id="245"><endHook>1</endHook>`+
			`<beginText><text>1.</text></beginText><endings>1</endings></Volta>`+
			aQuarter(62)+`</Measure>`+
			`<Measure><endSpanner id="245"></endSpanner>`+aQuarter(64)+`</Measure>`))

	bars := s.Parts[0].Measures
	if e := bars[0].Ending; e != nil {
		t.Errorf("bar 1 carries %+v, which is outside the bracket", e)
	}
	e := bars[1].Ending
	if e == nil {
		t.Fatal("the bar under the bracket has no ending")
	}
	if !e.Start || !e.Stop {
		t.Errorf("the bracket is %+v, want it opening and closing on this bar", e)
	}
	if len(e.Numbers) != 1 || e.Numbers[0] != 1 {
		t.Errorf("the bracket reads %v, want the first time through", e.Numbers)
	}
	if bars[2].Ending != nil {
		t.Errorf("bar 3 carries %+v, and the bracket stops before it", bars[2].Ending)
	}
}

func TestATrillWrittenTheOldWayIsRead(t *testing.T) {
	s := read(t, oldScore(`<Measure>
		<Trill id="197"><subtype>trill</subtype></Trill>`+
		aQuarter(60)+aQuarter(62)+`<endSpanner id="197"></endSpanner>`+
		aQuarter(64)+`</Measure>`))

	got := marksOfBar(s, 0, 0)
	if tr := got[0].Trill; tr == nil || !tr.Start {
		t.Errorf("the first note carries %+v, want the trill beginning", tr)
	}
	if tr := got[1].Trill; tr == nil || !tr.Stop {
		t.Errorf("the second note carries %+v, want the wavy line ending", tr)
	}
	if got[2].Trill != nil {
		t.Error("the note after the <endSpanner> is still trilling")
	}
}

// The slur's two ends are on the chords; what stands in the bar is only its
// shape, and reporting it buried the marks that really were unread.
func TestTheBarsSlurElementIsNotComplainedAbout(t *testing.T) {
	s := read(t, oldScore(`<Measure><Slur id="2"><track>0</track></Slur>`+
		`<Chord><durationType>quarter</durationType>`+
		`<Slur type="start" id="2"></Slur>`+
		`<Note><pitch>60</pitch><tpc>14</tpc></Note></Chord>`+
		`<Chord><durationType>quarter</durationType>`+
		`<Slur type="stop" id="2"></Slur>`+
		`<Note><pitch>62</pitch><tpc>16</tpc></Note></Chord></Measure>`))

	got := marksOfBar(s, 0, 0)
	if !got[0].SlurStart || !got[1].SlurStop {
		t.Errorf("the slur came out %v..%v, want it held across the pair",
			got[0].SlurStart, got[1].SlurStop)
	}
}

// Bar 54 of the Moonlight sonata's Presto: the left hand's crescendo is
// written at the first beat, behind a right hand that already fills the
// bar. Given to the voice standing there it was thrown away when that voice
// was closed, which is one of the ninety-eight.
func TestAMarkAtARewindBelongsToTheVoiceTheRewindOpens(t *testing.T) {
	s := read(t, oldScore(`<Measure>`+
		aQuarter(72)+aQuarter(74)+aQuarter(76)+aQuarter(77)+
		`<tick>0</tick>`+
		`<HairPin id="46"><subtype>0</subtype></HairPin>`+
		aQuarter(48)+aQuarter(50)+aQuarter(52)+aQuarter(53)+
		`<endSpanner id="46"></endSpanner></Measure>`))

	m := s.Parts[0].Measures[0]
	if len(m.Voices) != 2 {
		t.Fatalf("the bar holds %d voices, want 2", len(m.Voices))
	}
	first := marksOf(m.Voices[1].Elements[0])
	if first == nil || first.Hairpin == nil || !first.Hairpin.Start {
		t.Fatalf("the second voice opens with %+v, want the crescendo", first)
	}
	last := marksOf(m.Voices[1].Elements[len(m.Voices[1].Elements)-1])
	if last == nil || last.Hairpin == nil || !last.Hairpin.Stop {
		t.Errorf("the second voice ends with %+v, want the wedge closing", last)
	}
	for i, el := range m.Voices[0].Elements {
		if mk := marksOf(el); mk != nil && mk.Hairpin != nil {
			t.Errorf("element %d of the first voice carries the other hand's hairpin", i)
		}
	}
}

// A <tick> that only places a long mark is not a rewind. Reading every one
// of them as the start of the next voice cut bars into pieces.
func TestATickThatOnlyPlacesAMarkDoesNotOpenAVoice(t *testing.T) {
	s := read(t, oldScore(`<Measure>`+
		aQuarter(60)+aQuarter(62)+
		`<tick>240</tick><endSpanner id="99"></endSpanner>`+
		`<tick>960</tick>`+aQuarter(64)+aQuarter(65)+`</Measure>`))

	m := s.Parts[0].Measures[0]
	if len(m.Voices) != 1 {
		t.Fatalf("the bar holds %d voices, want 1", len(m.Voices))
	}
	if n := len(m.Voices[0].Elements); n != 4 {
		t.Errorf("the voice holds %d notes, want the four that were written", n)
	}
}
