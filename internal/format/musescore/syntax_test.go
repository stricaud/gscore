/*
 * syntax_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musescore

import (
	"sort"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * The rest of what MuseScore writes.
 *
 * Each of these is something counted in a corpus of 838 MuseScore files —
 * the program's own visual tests, three versions of musescore.com
 * downloads and a working musician's own drawer — that the reader used to
 * pass over. The fixtures are written the way the files are: MuseScore 4
 * unless the syntax is older than that.
 */

// ms4 is one staff of music in a MuseScore 4 file, the bars given.
func ms4(bars string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="4.50">
  <Score>
    <Part id="1">
      <Staff><StaffType group="pitched"><name>stdNormal</name></StaffType></Staff>
      <trackName>Flute</trackName>
      <Instrument id="flute"><longName>Flute</longName><Channel><program value="73"/></Channel></Instrument>
    </Part>
    <Staff id="1">` + bars + `</Staff>
  </Score>
</museScore>`
}

// bar wraps the elements of one bar in one voice.
func bar(els string) string { return `<Measure><voice>` + els + `</voice></Measure>` }

// crotchet is a quarter note on middle C with whatever is written inside it.
func crotchet(inside string) string {
	return `<Chord><durationType>quarter</durationType>` + inside +
		`<Note><pitch>60</pitch><tpc>14</tpc></Note></Chord>`
}

// elements lists the elements of the first voice of a bar.
func elements(t *testing.T, s *model.Score, bar int) []model.Element {
	t.Helper()
	m := s.Parts[0].Measures
	if bar >= len(m) {
		t.Fatalf("bar %d is not there: %d bars read", bar, len(m))
	}
	return m[bar].Voices[0].Elements
}

func TestAMelismaIsReadFromItsLength(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(`<Lyrics><ticks>480</ticks><ticks_f>1/4</ticks_f><text>plui</text></Lyrics>`)+
			crotchet(``)+
			crotchet(`<Lyrics><ticks_f>0/1</ticks_f><text>e</text></Lyrics>`))))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Lyrics) != 1 || !ms[0].Lyrics[0].Extend {
		t.Errorf("the held syllable is not marked as held: %+v", ms[0].Lyrics)
	}
	if len(ms[2].Lyrics) != 1 || ms[2].Lyrics[0].Extend {
		t.Errorf("a syllable held for no time is marked as held: %+v", ms[2].Lyrics)
	}
}

func TestAPortatoIsATenutoAndAStaccato(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(`<Articulation><subtype>articTenutoStaccatoBelow</subtype></Articulation>`)+
			crotchet(`<Articulation><subtype>portato</subtype></Articulation>`))))
	for i, m := range marksOfBar(s, 0, 0) {
		if len(m.Articulations) != 2 || m.Articulations[0] != model.Tenuto || m.Articulations[1] != model.Staccato {
			t.Errorf("note %d carries %v, want a tenuto and a staccato", i, m.Articulations)
		}
	}
	// The first says which side it is written on; the second leaves it
	// to the engraver.
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].ArticulationPlace) != 2 || ms[0].ArticulationPlace[1] != model.PlacementBelow {
		t.Errorf("the mark named Below is placed %v", ms[0].ArticulationPlace)
	}
	if len(ms[1].ArticulationPlace) != 0 {
		t.Errorf("an old-style mark was given a side: %v", ms[1].ArticulationPlace)
	}
}

func TestAMarkWithoutANameOfItsOwnIsDrawnAsItsGlyph(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(`<Articulation><subtype>brassMuteClosed</subtype></Articulation>`)+
			crotchet(`<Articulation><subtype>prallprall</subtype></Articulation>`)+
			crotchet(`<Articulation><subtype>articAccentStaccatoAbove</subtype></Articulation>`))))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Symbols) != 1 || ms[0].Symbols[0].Name != "brassMuteClosed" {
		t.Errorf("the stopped-horn sign came out as %+v", ms[0].Symbols)
	}
	if len(ms[1].Symbols) != 1 || ms[1].Symbols[0].Name != "ornamentTremblement" {
		t.Errorf("MuseScore 2's prallprall came out as %+v, want the glyph MuseScore maps it to", ms[1].Symbols)
	}
	if len(ms[2].Articulations) != 2 || len(ms[2].Symbols) != 0 {
		t.Errorf("an accent-staccato came out as %v and %v", ms[2].Articulations, ms[2].Symbols)
	}
}

func TestADynamicWrittenInWordsKeepsItsWords(t *testing.T) {
	s := read(t, ms4(bar(
		`<Dynamic><subtype>other-dynamics</subtype><text>più <sym>dynamicForte</sym></text></Dynamic>`+
			crotchet(``)+
			`<Dynamic><subtype>other-dynamics</subtype><velocity>33</velocity><text>sotto voce</text></Dynamic>`+
			crotchet(``)+
			`<Dynamic><subtype>sff</subtype></Dynamic>`+
			crotchet(``))))
	ms := marksOfBar(s, 0, 0)
	if ms[0].Dynamic != model.Forte || ms[0].DynamicText != "più f" {
		t.Errorf("più f came out as %v %q", ms[0].Dynamic, ms[0].DynamicText)
	}
	// No standard mark among the words: what it sounds as comes from the
	// velocity, which 33 puts at pp.
	if ms[1].Dynamic != model.PP || ms[1].DynamicText != "sotto voce" {
		t.Errorf("sotto voce came out as %v %q", ms[1].Dynamic, ms[1].DynamicText)
	}
	if ms[2].Dynamic == model.DynamicNone {
		t.Errorf("sff was not read")
	}
}

func TestABreathGoesOnTheNoteBeforeIt(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(``)+`<Breath><symbol>breathMarkComma</symbol></Breath>`+
			crotchet(``)+`<Breath><symbol>caesura</symbol></Breath>`+
			crotchet(``))))
	ms := marksOfBar(s, 0, 0)
	if ms[0].Break != model.BreathComma {
		t.Errorf("the first note's breath is %v", ms[0].Break)
	}
	if ms[1].Break != model.Caesura {
		t.Errorf("the second note's caesura is %v", ms[1].Break)
	}
	if ms[2].Break != model.NoBreath {
		t.Errorf("the third note has a breath it was not given: %v", ms[2].Break)
	}
	// And the MuseScore 2 numbering.
	s = read(t, oldScore(`<Measure>`+aQuarter(60)+`<Breath><subtype>3</subtype></Breath></Measure>`))
	if m := marksOfBar(s, 0, 0); m[0].Break != model.Caesura {
		t.Errorf("MuseScore 2's breath 3 is %v, want a caesura", m[0].Break)
	}
}

func TestAnArpeggioAndItsDirection(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(`<Arpeggio><subtype>0</subtype></Arpeggio>`)+
			crotchet(`<Arpeggio><subtype>2</subtype></Arpeggio>`)+
			crotchet(`<Arpeggio><subtype>3</subtype></Arpeggio>`))))
	ms := marksOfBar(s, 0, 0)
	if ms[0].Arpeggio == nil || ms[0].Arpeggio.Down {
		t.Errorf("a plain arpeggio came out as %+v", ms[0].Arpeggio)
	}
	if ms[1].Arpeggio == nil || !ms[1].Arpeggio.Down {
		t.Errorf("a downward arpeggio came out as %+v", ms[1].Arpeggio)
	}
	if ms[2].Arpeggio != nil {
		t.Errorf("a bracket, which says the chord is not rolled, came out as an arpeggio")
	}
}

func TestAFallIsDrawnBesideTheNote(t *testing.T) {
	s := read(t, ms4(bar(crotchet(`<ChordLine><subtype>1</subtype></ChordLine>`))))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Symbols) != 1 || ms[0].Symbols[0].Name != "brassFallLipShort" || ms[0].Symbols[0].OffsetX <= 0 {
		t.Errorf("the fall came out as %+v", ms[0].Symbols)
	}
}

func TestAChordsFingeringIsReadFromTheBottomUp(t *testing.T) {
	s := read(t, ms4(bar(
		`<Chord><durationType>quarter</durationType>`+
			`<Note><pitch>60</pitch><tpc>14</tpc><Fingering><text>1</text></Fingering></Note>`+
			`<Note><pitch>64</pitch><tpc>18</tpc><Fingering><text>3</text></Fingering></Note>`+
			`<Note><pitch>67</pitch><tpc>15</tpc><Fingering><text>5</text></Fingering></Note>`+
			`</Chord>`+
			`<Chord><durationType>quarter</durationType>`+
			`<Note><pitch>62</pitch><tpc>16</tpc><Fingering><text>2</text></Fingering></Note></Chord>`)))
	ms := marksOfBar(s, 0, 0)
	if got := strings.Join(ms[0].Fingering, ""); got != "135" {
		t.Errorf("the chord's fingering is %q, want 135", got)
	}
	if got := strings.Join(ms[1].Fingering, ""); got != "2" {
		t.Errorf("the note's fingering is %q, want 2", got)
	}
	if len(ms[0].Texts) != 0 {
		t.Errorf("the fingering was also written as text: %+v", ms[0].Texts)
	}
}

func TestAGlissandoIsReadAtBothEnds(t *testing.T) {
	s := read(t, ms4(bar(
		`<Chord><durationType>quarter</durationType><Note><pitch>60</pitch><tpc>14</tpc>`+
			`<Spanner type="Glissando"><Glissando><subtype>1</subtype><text>gliss.</text></Glissando>`+
			`<next><location><fractions>1/4</fractions></location></next></Spanner></Note></Chord>`+
			`<Chord><durationType>quarter</durationType><Note><pitch>72</pitch><tpc>14</tpc>`+
			`<Spanner type="Glissando"><prev><location><fractions>-1/4</fractions></location></prev></Spanner>`+
			`</Note></Chord>`)))
	ms := marksOfBar(s, 0, 0)
	g := ms[0].Glissando
	if g == nil || !g.Start || !g.Wavy || g.Text != "gliss." {
		t.Errorf("the slide begins as %+v", g)
	}
	if g := ms[1].Glissando; g == nil || !g.Stop {
		t.Errorf("the slide ends as %+v", g)
	}
}

func TestANoteOnTheOtherStaffAndACueNote(t *testing.T) {
	s := read(t, ms4(bar(
		crotchet(`<staffMove>1</staffMove>`)+
			crotchet(`<small>1</small>`)+
			`<Rest><durationType>quarter</durationType><staffMove>-1</staffMove><small>1</small></Rest>`)))
	ms := marksOfBar(s, 0, 0)
	if ms[0].Cross != model.OnStaffBelow {
		t.Errorf("a chord moved down a staff is drawn %v", ms[0].Cross)
	}
	if !ms[1].Small {
		t.Errorf("a cue-size chord is not small")
	}
	if ms[2].Cross != model.OnStaffAbove || !ms[2].Small {
		t.Errorf("a rest moved up a staff, cue-size, came out %v small=%v", ms[2].Cross, ms[2].Small)
	}
}

func TestAnInvisibleRestIsHiddenAndAMovedOneIsLifted(t *testing.T) {
	s := read(t, ms4(bar(
		`<Rest><durationType>quarter</durationType><visible>0</visible></Rest>`+
			`<Rest><durationType>quarter</durationType><offset x="0" y="-1"/></Rest>`)))
	els := elements(t, s, 0)
	if r, ok := els[0].(*model.Rest); !ok || !r.Hidden {
		t.Errorf("an invisible rest is %+v", els[0])
	}
	// A space upward is two steps up.
	if r, ok := els[1].(*model.Rest); !ok || r.Lift != 2 {
		t.Errorf("a rest moved up a space is lifted %+v", els[1])
	}
}

func TestAFermataOnARestOfMuseScore2(t *testing.T) {
	s := read(t, oldScore(`<Measure><Rest><durationType>whole</durationType>`+
		`<Articulation><subtype>fermata</subtype></Articulation></Rest></Measure>`))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Articulations) != 1 || ms[0].Articulations[0] != model.Fermata {
		t.Errorf("the rest carries %v, want a fermata", ms[0].Articulations)
	}
}

func TestAPedalLineAndAChangeOfPedal(t *testing.T) {
	s := read(t, ms4(bar(
		`<Spanner type="Pedal"><Pedal><endHookType>1</endHookType></Pedal>`+
			`<next><location><fractions>2/4</fractions></location></next></Spanner>`+
			crotchet(``)+crotchet(``)+
			// One ends and another begins on the third note: a change.
			`<Spanner type="Pedal"><prev><location><fractions>-2/4</fractions></location></prev></Spanner>`+
			`<Spanner type="Pedal"><Pedal><lineVisible>0</lineVisible><beginText><text>Ped.</text></beginText></Pedal>`+
			`<next><location><fractions>1/4</fractions></location></next></Spanner>`+
			crotchet(``)+
			`<Spanner type="Pedal"><prev><location><fractions>-1/4</fractions></location></prev></Spanner>`+
			crotchet(``))))
	ms := marksOfBar(s, 0, 0)
	if p := ms[0].Pedal; p == nil || !p.Start || p.Stop || p.Text {
		t.Errorf("the pedal begins as %+v", p)
	}
	if p := ms[1].Pedal; p != nil {
		t.Errorf("the second note carries a pedal mark it was not given: %+v", p)
	}
	if p := ms[2].Pedal; p == nil || !p.Start || !p.Stop || !p.Change {
		t.Errorf("the change of pedal came out as %+v", p)
	}
	if p := ms[2].Pedal; p == nil || !p.Text {
		t.Errorf("a pedal with no line is not in the Ped. and star form: %+v", p)
	}
	if p := ms[3].Pedal; p == nil || !p.Stop || p.Start {
		t.Errorf("the pedal ends as %+v", p)
	}
}

func TestATextLineKeepsItsWordsAndAPalmMuteItsTechnique(t *testing.T) {
	s := read(t, ms4(bar(
		`<Spanner type="TextLine"><TextLine><beginText>rit.</beginText></TextLine>`+
			`<next><location><fractions>1/4</fractions></location></next></Spanner>`+
			`<Spanner type="PalmMute"><PalmMute/><next><location><fractions>1/4</fractions></location></next></Spanner>`+
			`<Spanner type="Vibrato"><Vibrato><subtype>guitarVibratoWide</subtype></Vibrato>`+
			`<next><location><fractions>1/4</fractions></location></next></Spanner>`+
			crotchet(``))))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Texts) != 1 || ms[0].Texts[0].Text != "rit." {
		t.Errorf("the text line's words came out as %+v", ms[0].Texts)
	}
	want := []model.Technique{model.PalmMute, model.WideVibrato}
	if len(ms[0].Techniques) != 2 || ms[0].Techniques[0] != want[0] || ms[0].Techniques[1] != want[1] {
		t.Errorf("the techniques came out as %v, want %v", ms[0].Techniques, want)
	}
}

func TestAKeySignatureNamesItsMode(t *testing.T) {
	s := read(t, ms4(bar(`<KeySig><concertKey>-3</concertKey><actualKey>-3</actualKey><mode>minor</mode></KeySig>`+crotchet(``))))
	if k := s.Parts[0].Key; k.Fifths != -3 || k.Mode != model.Minor {
		t.Errorf("the key is %+v, want C minor", k)
	}
}

func TestMuseScore1NumbersItsClefs(t *testing.T) {
	doc := strings.Replace(oldScore(`<Measure><Clef><subtype>4</subtype></Clef>`+aQuarter(48)+`</Measure>`),
		`<defaultClef>F</defaultClef>`, ``, 1)
	s := read(t, doc)
	if s.Parts[0].Clef != model.Bass {
		t.Errorf("clef 4 is %v, want the bass clef", s.Parts[0].Clef)
	}
}

func TestALineBreakStartsANewSystem(t *testing.T) {
	s := read(t, ms4(
		`<Measure><LayoutBreak><subtype>line</subtype></LayoutBreak><voice>`+crotchet(``)+`</voice></Measure>`+
			`<Measure><LayoutBreak><subtype>page</subtype></LayoutBreak><voice>`+crotchet(``)+`</voice></Measure>`+
			bar(crotchet(``))))
	m := s.Parts[0].Measures
	if !m[0].SystemBreak || !m[1].SystemBreak || m[2].SystemBreak {
		t.Errorf("the breaks are %v %v %v, want the first two", m[0].SystemBreak, m[1].SystemBreak, m[2].SystemBreak)
	}
}

func TestAStaffSaysHowManyLinesItHasAndWhetherItIsShown(t *testing.T) {
	doc := strings.Replace(ms4(bar(crotchet(``))),
		`<Staff><StaffType group="pitched"><name>stdNormal</name></StaffType></Staff>`,
		`<Staff><StaffType group="percussion"><name>perc1Line</name><lines>1</lines></StaffType></Staff><show>0</show>`, 1)
	s := read(t, doc)
	p := s.Parts[0]
	if p.StaffLines != 1 {
		t.Errorf("the staff is ruled with %d lines, want 1", p.StaffLines)
	}
	if !p.Hidden {
		t.Errorf("a part the file does not show is shown")
	}
}

func TestTheLyricistIsReadFromTheTitlePage(t *testing.T) {
	s := read(t, ms4(`<VBox><Text><style>lyricist</style><text>Irving Mills</text></Text></VBox>`+bar(crotchet(``))))
	if s.Identity.Lyricist != "Irving Mills" {
		t.Errorf("the lyricist is %q", s.Identity.Lyricist)
	}
}

func TestAnOldChordSymbolIsNamedByTheChordList(t *testing.T) {
	doc := strings.Replace(oldScore(`<Measure><Harmony><root>14</root><extension>64</extension></Harmony>`+aQuarter(60)+`</Measure>`),
		`<Part>`, `<ChordList><chord id="64"><name>7</name></chord></ChordList><Part>`, 1)
	s := read(t, doc)
	ms := marksOfBar(s, 0, 0)
	if ms[0].Chord == nil || ms[0].Chord.Root != "C" || ms[0].Chord.Quality != "7" {
		t.Errorf("the chord symbol came out as %+v, want C7", ms[0].Chord)
	}
}

func TestATieSaysWhichWayItArcsAndAnAccidentalKeepsItsBrackets(t *testing.T) {
	s := read(t, ms4(bar(
		`<Chord><durationType>quarter</durationType><Note><pitch>61</pitch><tpc>21</tpc>`+
			`<Accidental><subtype>accidentalSharp</subtype><bracket>1</bracket><role>1</role></Accidental>`+
			`<Spanner type="Tie"><Tie><up>down</up></Tie><next><location><fractions>1/4</fractions></location></next></Spanner>`+
			`</Note></Chord>`+
			`<Chord><durationType>quarter</durationType><Note><pitch>61</pitch><tpc>21</tpc>`+
			`<Spanner type="Tie"><prev><location><fractions>-1/4</fractions></location></prev></Spanner></Note></Chord>`)))
	n := firstNote(t, s.Parts[0])
	if n.Accidental != model.Sharp || !n.Forced || !n.Parens {
		t.Errorf("the courtesy sharp came out as %v forced=%v parens=%v", n.Accidental, n.Forced, n.Parens)
	}
	if n.TieDir != model.StemDown {
		t.Errorf("the tie arcs %v, want down", n.TieDir)
	}
}

func TestWordsInTheBarAreKept(t *testing.T) {
	s := read(t, ms4(bar(
		`<InstrumentChange><text>To Piccolo</text></InstrumentChange>`+
			`<PlayTechAnnotation><text>pizz.</text></PlayTechAnnotation>`+
			`<Symbol><name>keyboardPedalHeel1</name></Symbol>`+
			crotchet(`<Note><pitch>60</pitch><tpc>14</tpc><Symbol><name>accidentalSharp</name></Symbol></Note>`))))
	ms := marksOfBar(s, 0, 0)
	var words []string
	for _, tm := range ms[0].Texts {
		words = append(words, tm.Text)
	}
	if got := strings.Join(words, "|"); got != "To Piccolo|pizz." {
		t.Errorf("the words came out as %q", got)
	}
	var names []string
	for _, sm := range ms[0].Symbols {
		names = append(names, sm.Name)
	}
	sort.Strings(names)
	if got := strings.Join(names, "|"); got != "accidentalSharp|keyboardPedalHeel1" {
		t.Errorf("the symbols came out as %q", got)
	}
}

func TestARepeatWrittenAsABarline(t *testing.T) {
	s := read(t, ms4(
		bar(crotchet(``))+
			bar(`<BarLine><subtype>start-repeat</subtype></BarLine>`+crotchet(``)+
				`<BarLine><subtype>end-repeat</subtype></BarLine>`)+
			bar(crotchet(``)+`<BarLine><subtype>end-start-repeat</subtype></BarLine>`)+
			bar(crotchet(``))))
	m := s.Parts[0].Measures
	want := []model.BarlineStyle{model.BarRepeatStart, model.BarRepeatEnd, model.BarRepeatBoth, model.BarFinal}
	for i, w := range want {
		if m[i].RightBarline != w {
			t.Errorf("bar %d ends with %v, want %v", i+1, m[i].RightBarline, w)
		}
	}
}

func TestARitardandoLineKeepsItsWords(t *testing.T) {
	s := read(t, ms4(bar(
		`<Spanner type="GradualTempoChange"><GradualTempoChange><tempoChangeType>1</tempoChangeType>`+
			`<beginText>rit.</beginText></GradualTempoChange>`+
			`<next><location><fractions>1/4</fractions></location></next></Spanner>`+
			crotchet(``))))
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Texts) != 1 || ms[0].Texts[0].Text != "rit." || ms[0].Texts[0].Style != model.TextTempo {
		t.Errorf("the ritardando came out as %+v", ms[0].Texts)
	}
}

func TestMuseScore2WroteTheRepeatSignInsideTheVoice(t *testing.T) {
	s := read(t, oldScore(`<Measure>`+aQuarter(60)+`</Measure><Measure><voice><RepeatMeasure><durationType>measure</durationType></RepeatMeasure></voice></Measure>`))
	if s.Parts[0].Measures[1].Repeat != 1 {
		t.Errorf("the second bar is not marked as a repeat of the first")
	}
}

func TestMuseScore1KeptItsPartsUnderTheRoot(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<museScore version="1.14">
  <Part><Staff id="1"><cleflist><clef tick="0" idx="4"/></cleflist></Staff><trackName>Piano</trackName></Part>
  <Staff id="1">
    <Measure number="1"><Text><style>Staff</style><text>dolce</text></Text>` + aQuarter(48) + `
      <BarLine><subtype>5</subtype></BarLine></Measure>
  </Staff>
</museScore>`
	s := read(t, doc)
	p := s.Parts[0]
	if len(p.Measures) != 1 || len(p.Measures[0].Voices[0].Elements) != 1 {
		t.Fatalf("the music was not found under the root: %d bars", len(p.Measures))
	}
	if p.Clef != model.Bass {
		t.Errorf("the clef is %v, want the bass clef from the clef list", p.Clef)
	}
	ms := marksOfBar(s, 0, 0)
	if len(ms[0].Texts) != 1 || ms[0].Texts[0].Text != "dolce" {
		t.Errorf("the staff text came out as %+v", ms[0].Texts)
	}
	if p.Measures[0].RightBarline != model.BarFinal {
		t.Errorf("barline 5 is %v, want the final barline", p.Measures[0].RightBarline)
	}
}
