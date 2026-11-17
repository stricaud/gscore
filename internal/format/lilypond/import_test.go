/*
 * import_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// one reads a stretch of LilyPond as a file with no includes.
func one(t *testing.T, text string) *model.Score {
	t.Helper()
	scores, err := ImportText(text, "test.ly", "")
	if len(scores) == 0 {
		t.Fatalf("nothing was read: %v", err)
	}
	if err != nil {
		t.Logf("read with reservations: %v", err)
	}
	return scores[0]
}

// clean reads a stretch and insists nothing was left out.
func clean(t *testing.T, text string) *model.Score {
	t.Helper()
	scores, err := ImportText(text, "test.ly", "")
	if len(scores) == 0 {
		t.Fatalf("nothing was read: %v", err)
	}
	if err != nil {
		t.Fatalf("something was left out: %v", err)
	}
	return scores[0]
}

// spelled writes a voice's elements the way a test can compare them:
// "C4/1/4 D4/1/8 r/1/8 <C4 E4>/1/4".
func spelled(v *model.Voice) string {
	var out []string
	for _, el := range v.Elements {
		switch e := el.(type) {
		case *model.Note:
			out = append(out, fmt.Sprintf("%s/%s", e.Pitch, e.Dur))
		case *model.Rest:
			if e.Hidden {
				out = append(out, "s/"+e.Dur.String())
			} else {
				out = append(out, "r/"+e.Dur.String())
			}
		case *model.Chord:
			var ps []string
			for _, n := range e.Notes {
				ps = append(ps, n.Pitch.String())
			}
			out = append(out, fmt.Sprintf("<%s>/%s", strings.Join(ps, " "), e.Dur))
		}
	}
	return strings.Join(out, " ")
}

func voiceOf(t *testing.T, s *model.Score, part, measure, voice int) *model.Voice {
	t.Helper()
	if part >= len(s.Parts) || measure >= len(s.Parts[part].Measures) ||
		voice >= len(s.Parts[part].Measures[measure].Voices) {
		t.Fatalf("no voice %d in bar %d of part %d", voice+1, measure+1, part+1)
	}
	return s.Parts[part].Measures[measure].Voices[voice]
}

func TestNoteNamesInEveryLanguage(t *testing.T) {
	for _, c := range []struct{ lang, text, want string }{
		{"nederlands", "cis'4 es' as' bes' c''", "C#4/1/4 Eb4/1/4 Ab4/1/4 Bb4/1/4 C5/1/4"},
		{"english", "cs'4 ef' a-flat' bf' c''", "C#4/1/4 Eb4/1/4 Ab4/1/4 Bb4/1/4 C5/1/4"},
		{"italiano", "dod'4 mib' lab' sib' do''", "C#4/1/4 Eb4/1/4 Ab4/1/4 Bb4/1/4 C5/1/4"},
		{"deutsch", "cis'4 es' as' b' h'", "C#4/1/4 Eb4/1/4 Ab4/1/4 Bb4/1/4 B4/1/4"},
		{"français", "ré'4 mib' fad' sib' do''", "D4/1/4 Eb4/1/4 F#4/1/4 Bb4/1/4 C5/1/4"},
	} {
		text := c.text
		if c.lang == "english" {
			text = strings.ReplaceAll(text, "a-flat'", "af'")
		}
		s := clean(t, fmt.Sprintf(`\language "%s" { %s }`, c.lang, text))
		got := ""
		for _, m := range s.Parts[0].Measures {
			got += spelled(m.Voices[0]) + " "
		}
		if strings.TrimSpace(got) != c.want {
			t.Errorf("%s: got %q, want %q", c.lang, strings.TrimSpace(got), c.want)
		}
	}
}

func TestLengthsAndDots(t *testing.T) {
	s := clean(t, `{ c'4 c'8. c'16 c'2 | c'2.. c'8 | c'1*2 | c'\breve }`)
	var got []string
	for _, m := range s.Parts[0].Measures {
		got = append(got, spelled(m.Voices[0]))
	}
	want := []string{"C4/1/4 C4/3/16 C4/1/16 C4/1/2", "C4/7/8 C4/1/8", "C4/1/1", "C4/1/1", "C4/1/1", "C4/1/1"}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("got %q", strings.Join(got, " | "))
	}
}

func TestRelativePitches(t *testing.T) {
	s := clean(t, `\relative c' { c4 d e f g a b c | d' c, <c e g> q }`)
	var all []string
	for _, m := range s.Parts[0].Measures {
		all = append(all, spelled(m.Voices[0]))
	}
	if got := strings.Join(all, " "); got !=
		"C4/1/4 D4/1/4 E4/1/4 F4/1/4 G4/1/4 A4/1/4 B4/1/4 C5/1/4 D6/1/4 C5/1/4 <C5 E5 G5>/1/4 <C5 E5 G5>/1/4" {
		t.Errorf("got %q", got)
	}
	// A \relative inside a \relative is sealed: the chord after it goes on
	// from c', not from the b'' of the rest.
	s = clean(t, `\language "english" \relative c' { \relative c'' { b8\rest } <ef a c ef>32 c8 }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "r/1/8 <Eb4 A4 C5 Eb5>/1/32 C4/1/8" {
		t.Errorf("nested relative: got %q", got)
	}
	// \relative with no pitch: written as if absolute.
	s = clean(t, `\relative { c'4 e g c }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "C4/1/4 E4/1/4 G4/1/4 C5/1/4" {
		t.Errorf("relative with no pitch: got %q", got)
	}
}

func TestARepeatedChordKeepsItsPitches(t *testing.T) {
	s := clean(t, `\language "english" \relative c' { <df g df'>16 4 4 c8 8 }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "<Db4 G4 Db5>/1/16 <Db4 G4 Db5>/1/4 <Db4 G4 Db5>/1/4 C4/1/8 C4/1/8" {
		t.Errorf("got %q", got)
	}
}

func TestBarsFromTheMeterAndAnUpbeat(t *testing.T) {
	s := clean(t, `{ \time 3/4 \partial 4 c'4 | d'2 e'4 | f'2. | \time 2/4 g'2 }`)
	p := s.Parts[0]
	if len(p.Measures) != 4 {
		t.Fatalf("%d bars, want 4", len(p.Measures))
	}
	if p.Time.Beats != 3 || p.Measures[3].Time == nil || p.Measures[3].Time.Beats != 2 {
		t.Errorf("the meters did not arrive: %+v, %+v", p.Time, p.Measures[3].Time)
	}
	if got := spelled(p.Measures[0].Voices[0]); got != "C4/1/4" {
		t.Errorf("the upbeat is %q", got)
	}
	if p.Measures[1].Time != nil {
		t.Errorf("the second bar restates the meter")
	}
}

func TestANoteOverABarlineIsSplitAndTied(t *testing.T) {
	s := clean(t, `{ c'2. d'2 ~ d'4 }`)
	p := s.Parts[0]
	if len(p.Measures) != 2 {
		t.Fatalf("%d bars", len(p.Measures))
	}
	if got := spelled(p.Measures[0].Voices[0]); got != "C4/3/4 D4/1/4" {
		t.Errorf("bar 1 is %q", got)
	}
	if got := spelled(p.Measures[1].Voices[0]); got != "D4/1/4 D4/1/4" {
		t.Errorf("bar 2 is %q", got)
	}
	first := p.Measures[0].Voices[0].Elements[1].(*model.Note)
	second := p.Measures[1].Voices[0].Elements[0].(*model.Note)
	third := p.Measures[1].Voices[0].Elements[1].(*model.Note)
	if !first.TieStart || !second.TieStop || !second.TieStart || !third.TieStop {
		t.Errorf("the ties are wrong: %v %v / %v %v / %v", first.TieStart, second.TieStop, second.TieStart, third.TieStop, third.TieStart)
	}
}

func TestSlursBeamsAndArticulations(t *testing.T) {
	s := clean(t, `{ c'8( d'-. e'-> f') g'8-[ a' b'] r8 | c''4\( d''\) }`)
	v := voiceOf(t, s, 0, 0, 0)
	n := func(i int) *model.Note { return v.Elements[i].(*model.Note) }
	if n(0).SlurStart != 1 || n(3).SlurStop != 1 {
		t.Errorf("the slur is wrong")
	}
	if !n(1).HasArticulation(model.Staccato) || !n(2).HasArticulation(model.Accent) {
		t.Errorf("the articulations are wrong")
	}
	if n(4).Beam != model.BeamStart || n(5).Beam != model.BeamContinue || n(6).Beam != model.BeamStop {
		t.Errorf("the beam is wrong: %v %v %v", n(4).Beam, n(5).Beam, n(6).Beam)
	}
	v2 := voiceOf(t, s, 0, 1, 0)
	if v2.Elements[0].(*model.Note).SlurStart != 1 || v2.Elements[1].(*model.Note).SlurStop != 1 {
		t.Errorf("the phrasing slur is wrong")
	}
}

func TestTuplets(t *testing.T) {
	s := clean(t, `{ \tuplet 3/2 { c'8 d' e' } \times 2/3 { f'4 g' a' } b'4 }`)
	v := voiceOf(t, s, 0, 0, 0)
	if got := spelled(v); got != "C4/1/12 D4/1/12 E4/1/12 F4/1/6 G4/1/6 A4/1/6 B4/1/4" {
		t.Errorf("got %q", got)
	}
	first := v.Elements[0].(*model.Note)
	last := v.Elements[2].(*model.Note)
	if first.Tuplet == nil || !first.Tuplet.Start || first.Tuplet.Actual != 3 || first.Tuplet.Normal != 2 {
		t.Errorf("the tuplet does not begin: %+v", first.Tuplet)
	}
	if last.Tuplet == nil || !last.Tuplet.Stop {
		t.Errorf("the tuplet does not end: %+v", last.Tuplet)
	}
	if v.Elements[3].(*model.Note).Tuplet == nil || !v.Elements[3].(*model.Note).Tuplet.Start {
		t.Errorf("the second tuplet does not begin")
	}
	if v.Elements[6].(*model.Note).Tuplet != nil {
		t.Errorf("the plain note is in a tuplet")
	}
}

func TestGraceNotesLeanOnTheNoteAfter(t *testing.T) {
	s := clean(t, `{ \grace { b16 } c'4 \acciaccatura d'8 e'4 \afterGrace f'2 { e'16 f' } g'4 }`)
	v := voiceOf(t, s, 0, 0, 0)
	if got := spelled(v); got != "C4/1/4 E4/1/4 F4/1/2" {
		t.Errorf("got %q", got)
	}
	c := v.Elements[0].(*model.Note)
	if c.Grace == nil || len(c.Grace.Notes) != 1 || c.Grace.Notes[0].Pitch.String() != "B3" {
		t.Errorf("the grace is wrong: %+v", c.Grace)
	}
	e := v.Elements[1].(*model.Note)
	if e.Grace == nil || !e.Grace.Slashed {
		t.Errorf("the acciaccatura is not slashed")
	}
	g := voiceOf(t, s, 0, 1, 0).Elements[0].(*model.Note)
	if g.Grace == nil || len(g.Grace.Notes) != 2 {
		t.Errorf("the after-grace did not reach the next note: %+v", g.Grace)
	}
}

func TestDynamicsHairpinsWordsAndFingering(t *testing.T) {
	s := clean(t, `{ c'4\p\< d'-3 e'\! f'\sfz^"dolce" | g'2\> a'\! _\markup { \italic "cresc." } }`)
	v := voiceOf(t, s, 0, 0, 0)
	n := func(i int) *model.Note { return v.Elements[i].(*model.Note) }
	if n(0).Dynamic != model.Piano || n(0).Hairpin == nil || !n(0).Hairpin.Start || n(0).Hairpin.Kind != model.Crescendo {
		t.Errorf("the p and the crescendo are wrong: %v %+v", n(0).Dynamic, n(0).Hairpin)
	}
	if len(n(1).Fingering) != 1 || n(1).Fingering[0] != "3" {
		t.Errorf("the fingering is wrong: %v", n(1).Fingering)
	}
	if n(2).Hairpin == nil || !n(2).Hairpin.Stop {
		t.Errorf("the crescendo does not end")
	}
	if n(3).Dynamic != model.SForzando || len(n(3).Texts) != 1 || n(3).Texts[0].Text != "dolce" ||
		n(3).Texts[0].Placement != model.PlacementAbove {
		t.Errorf("the sfz and the word are wrong: %v %+v", n(3).Dynamic, n(3).Texts)
	}
	v2 := voiceOf(t, s, 0, 1, 0)
	a := v2.Elements[1].(*model.Note)
	if len(a.Texts) != 1 || a.Texts[0].Text != "cresc." || a.Texts[0].Placement != model.PlacementBelow {
		t.Errorf("the markup is wrong: %+v", a.Texts)
	}
}

func TestTwoVoicesOnOneStaff(t *testing.T) {
	s := clean(t, `{ << { c''2 d'' } \\ { e'4 f' g' a' } >> | b'1 }`)
	m := s.Parts[0].Measures[0]
	if len(m.Voices) != 2 {
		t.Fatalf("%d voices", len(m.Voices))
	}
	if got := spelled(m.Voices[0]); got != "C5/1/2 D5/1/2" {
		t.Errorf("voice 1 is %q", got)
	}
	if got := spelled(m.Voices[1]); got != "E4/1/4 F4/1/4 G4/1/4 A4/1/4" {
		t.Errorf("voice 2 is %q", got)
	}
	if len(s.Parts[0].Measures[1].Voices) != 1 {
		t.Errorf("the bar after has %d voices", len(s.Parts[0].Measures[1].Voices))
	}
}

func TestARepeatWithTwoEndings(t *testing.T) {
	s := clean(t, `{ c'1 \repeat volta 2 { d'1 e'1 } \alternative { { f'1 } { g'1 } } a'1 }`)
	p := s.Parts[0]
	if len(p.Measures) != 6 {
		t.Fatalf("%d bars", len(p.Measures))
	}
	if p.Measures[0].RightBarline != model.BarRepeatStart {
		t.Errorf("the repeat does not open after bar 1: %v", p.Measures[0].RightBarline)
	}
	if p.Measures[3].RightBarline != model.BarRepeatEnd {
		t.Errorf("the first ending does not close with a repeat: %v", p.Measures[3].RightBarline)
	}
	if e := p.Measures[3].Ending; e == nil || !e.Start || !e.Stop || e.Label() != "1." {
		t.Errorf("the first ending is wrong: %+v", e)
	}
	if e := p.Measures[4].Ending; e == nil || !e.Open || e.Label() != "2." {
		t.Errorf("the second ending is wrong: %+v", e)
	}
	s = clean(t, `{ \repeat unfold 3 { c'4 } d'4 }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "C4/1/4 C4/1/4 C4/1/4 D4/1/4" {
		t.Errorf("unfold: got %q", got)
	}
}

func TestClefsKeysAndMetersChange(t *testing.T) {
	s := clean(t, `{ \clef bass \key d \major c2 d | \clef treble \key f \minor e'4 \clef bass f g2 }`)
	p := s.Parts[0]
	if p.Clef != model.Bass || p.Key.Fifths != 2 {
		t.Errorf("the part starts in %v, %+v", p.Clef, p.Key)
	}
	m := p.Measures[1]
	if m.Clef == nil || *m.Clef != model.Treble || m.Key == nil || m.Key.Fifths != -4 || m.Key.Mode != model.Minor {
		t.Errorf("the second bar does not change: %v %+v", m.Clef, m.Key)
	}
	f := m.Voices[0].Elements[1].(*model.Note)
	if f.Clef == nil || *f.Clef != model.Bass {
		t.Errorf("the clef written between the notes is lost")
	}
}

func TestAPianoHasTwoStavesAndAVoiceMayMoveBetweenThem(t *testing.T) {
	s := clean(t, `\score { \new PianoStaff << \new Staff = "right" { c''4 \change Staff = "left" c'4 \change Staff = "right" c''2 } \new Staff = "left" { \clef bass c2 c } >> }`)
	if len(s.Parts) != 2 || len(s.Groups) != 1 || s.Groups[0].Symbol != model.GroupBrace {
		t.Fatalf("%d parts, groups %+v", len(s.Parts), s.Groups)
	}
	if s.Parts[0].Name != "right" || s.Parts[1].Clef != model.Bass {
		t.Errorf("the staves are %q (%v) and %q (%v)", s.Parts[0].Name, s.Parts[0].Clef, s.Parts[1].Name, s.Parts[1].Clef)
	}
	// The note that went to the left hand is a voice of the left hand's
	// bar, and the right hand's bar goes on without it.
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "C5/1/4 C5/1/2" {
		t.Errorf("the right hand is %q", got)
	}
	m := s.Parts[1].Measures[0]
	if len(m.Voices) != 2 || spelled(m.Voices[1]) != "C4/1/4" {
		t.Errorf("the left hand's bar is wrong: %d voices, %q", len(m.Voices), spelled(m.Voices[len(m.Voices)-1]))
	}
}

func TestWordsUnderTheNotes(t *testing.T) {
	s := clean(t, `<< \new Voice = "m" { c'4 d' e' f' | g'2 g' } \new Lyrics \lyricsto "m" { Twin -- kle star __ _ high } >>`)
	v := voiceOf(t, s, 0, 0, 0)
	n := func(i int) *model.Note { return v.Elements[i].(*model.Note) }
	if len(n(0).Lyrics) != 1 || n(0).Lyrics[0].Text != "Twin" || n(0).Lyrics[0].Syllabic != model.SyllableBegin {
		t.Errorf("the first syllable is wrong: %+v", n(0).Lyrics)
	}
	if len(n(1).Lyrics) != 1 || n(1).Lyrics[0].Syllabic != model.SyllableEnd {
		t.Errorf("the second syllable is wrong: %+v", n(1).Lyrics)
	}
	if len(n(2).Lyrics) != 1 || !n(2).Lyrics[0].Extend {
		t.Errorf("the held syllable is wrong: %+v", n(2).Lyrics)
	}
	if len(n(3).Lyrics) != 0 {
		t.Errorf("the skipped note has a word")
	}
	g := voiceOf(t, s, 0, 1, 0).Elements[0].(*model.Note)
	if len(g.Lyrics) != 1 || g.Lyrics[0].Text != "high" {
		t.Errorf("the last word is wrong: %+v", g.Lyrics)
	}
	s = clean(t, `\relative c' { c4 d e f } \addlyrics { la la li lo }`)
	if n := voiceOf(t, s, 0, 0, 0).Elements[3].(*model.Note); len(n.Lyrics) != 1 || n.Lyrics[0].Text != "lo" {
		t.Errorf("addlyrics: %+v", n.Lyrics)
	}
}

func TestADrumStaff(t *testing.T) {
	s := clean(t, `\new DrumStaff \drummode { bd4 sn hh8 hh <bd sn>4 }`)
	p := s.Parts[0]
	if p.Clef != model.Percussion {
		t.Errorf("the staff is not percussion")
	}
	v := p.Measures[0].Voices[0]
	if len(v.Elements) != 5 {
		t.Fatalf("%d elements", len(v.Elements))
	}
	if n := v.Elements[0].(*model.Note); n.Percussion != 36 {
		t.Errorf("the bass drum is key %d", n.Percussion)
	}
	if n := v.Elements[2].(*model.Note); n.Percussion != 42 || n.Head != model.HeadX {
		t.Errorf("the hi-hat is %d with head %v", n.Percussion, n.Head)
	}
}

func TestTheHeaderAndTheTempo(t *testing.T) {
	s := clean(t, `\header { title = "A Song" composer = "Somebody" subtitle = \markup { \italic "Op. 3" } }
\score { \new Staff { \tempo "Allegro" 4 = 132 c'1 \tempo "Adagio" d'1 \mark \default e'1 \mark \default f'1 } }`)
	if s.Identity.Title != "A Song" || s.Identity.Composer != "Somebody" || s.Identity.Subtitle != "Op. 3" {
		t.Errorf("the header is wrong: %+v", s.Identity)
	}
	if s.Tempo.BPM != 132 || s.Tempo.Text != "Allegro" || s.Tempo.Silent {
		t.Errorf("the tempo is wrong: %+v", s.Tempo)
	}
	if plain := clean(t, `{ c'1 }`); plain.Tempo.BPM != 60 {
		t.Errorf("a score with no tempo plays at %d, not 60", plain.Tempo.BPM)
	}
	if words := clean(t, `{ \tempo "Grave" c'1 }`); words.Tempo.BPM != 60 || !words.Tempo.Silent || words.Tempo.Text != "Grave" {
		t.Errorf("a score with words alone: %+v", words.Tempo)
	}
	m := s.Parts[0].Measures[1]
	if m.Tempo == nil || m.Tempo.Text != "Adagio" || !m.Tempo.Silent {
		t.Errorf("the second tempo is wrong: %+v", m.Tempo)
	}
	if s.Parts[0].Measures[2].Marker != "A" || s.Parts[0].Measures[3].Marker != "B" {
		t.Errorf("the marks are %q and %q", s.Parts[0].Measures[2].Marker, s.Parts[0].Measures[3].Marker)
	}
}

func TestLayoutCommandsAreSteppedOver(t *testing.T) {
	s := clean(t, `{ \once\override Staff.TextScript.extra-offset = #'(-1 . -1.5)
\override Slur.height-limit = #4 \omit Beam \undo \omit Beam \single \hide Stem
c'4-\tweak Y-offset #3.6 ^4 d'4 \shape #'((0 . 0) (1 . 1)) Slur e'4( f') \repeat unfold 9 { g'4 } }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "C4/1/4 D4/1/4 E4/1/4 F4/1/4" {
		t.Errorf("got %q", got)
	}
	if n := voiceOf(t, s, 0, 0, 0).Elements[0].(*model.Note); len(n.Fingering) != 1 || n.Fingering[0] != "4" {
		t.Errorf("the fingering after the tweak is lost: %v", n.Fingering)
	}
	if n := len(s.Parts[0].Measures); n != 4 {
		t.Errorf("the unfold gave %d bars, want 4", n)
	}
}

func TestAnEmptyChordCarriesItsMarksToTheNextNote(t *testing.T) {
	s := clean(t, `{ c'4 <>\pp ^\markup pizz. d'4 e'2 }`)
	v := voiceOf(t, s, 0, 0, 0)
	if got := spelled(v); got != "C4/1/4 D4/1/4 E4/1/2" {
		t.Errorf("got %q", got)
	}
	d := v.Elements[1].(*model.Note)
	if d.Dynamic != model.PP || len(d.Texts) != 1 || d.Texts[0].Text != "pizz." {
		t.Errorf("the marks did not reach the note: %v %+v", d.Dynamic, d.Texts)
	}
}

func TestAMarkupVariableIsAWord(t *testing.T) {
	s := clean(t, `nonLigato = \markup { \italic "non ligato" }
{ c'4-\nonLigato d'4 \nonLigato e'2 }`)
	v := voiceOf(t, s, 0, 0, 0)
	if n := v.Elements[0].(*model.Note); len(n.Texts) != 1 || n.Texts[0].Text != "non ligato" {
		t.Errorf("the word on the note is wrong: %+v", n.Texts)
	}
	if n := v.Elements[2].(*model.Note); len(n.Texts) != 1 || n.Texts[0].Text != "non ligato" {
		t.Errorf("the word before the note is wrong: %+v", n.Texts)
	}
}

func TestTrillsOctaveLinesAndPedals(t *testing.T) {
	s := clean(t, `{ \ottava #1 c'''4\startTrillSpan d'''\sustainOn e'''\stopTrillSpan \ottava #0 f'''\sustainOff }`)
	v := voiceOf(t, s, 0, 0, 0)
	n := func(i int) *model.Note { return v.Elements[i].(*model.Note) }
	if n(0).Octave == nil || !n(0).Octave.Start || n(0).Octave.Amount != 1 || n(3).Octave == nil || !n(3).Octave.Stop {
		t.Errorf("the octave line is wrong: %+v %+v", n(0).Octave, n(3).Octave)
	}
	if n(0).Trill == nil || !n(0).Trill.Start || n(2).Trill == nil || !n(2).Trill.Stop {
		t.Errorf("the trill is wrong")
	}
	if n(1).Pedal == nil || !n(1).Pedal.Start || n(3).Pedal == nil || !n(3).Pedal.Stop {
		t.Errorf("the pedal is wrong")
	}
}

func TestSeveralScoresMakeABook(t *testing.T) {
	scores, err := ImportText(`\header { title = "Two" }
\score { \new Staff { c'1 } \header { piece = "I" } }
\score { \new Staff { d'1 e'1 } \header { piece = "II" } }`, "two.ly", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 2 {
		t.Fatalf("%d scores", len(scores))
	}
	if len(scores[1].Parts[0].Measures) != 2 {
		t.Errorf("the second movement has %d bars", len(scores[1].Parts[0].Measures))
	}
}

func TestATransposingInstrument(t *testing.T) {
	s := clean(t, `\new Staff \with { instrumentName = "Clarinet in B flat" } { \transposition bes c'4 d' e' f' }`)
	p := s.Parts[0]
	if p.Name != "Clarinet in B flat" || p.Transpose.Chromatic != -2 || p.Transpose.Diatonic != -1 {
		t.Errorf("got %q %+v", p.Name, p.Transpose)
	}
	s = clean(t, `{ \transpose c d { c'4 e' } }`)
	if got := spelled(voiceOf(t, s, 0, 0, 0)); got != "D4/1/4 F#4/1/4" {
		t.Errorf("transpose: got %q", got)
	}
}

func TestAnAfterGraceCarriesTheTrillsEnd(t *testing.T) {
	s := clean(t, `{ a2 \afterGrace fs'2\startTrillSpan { e'16\stopTrillSpan fs' } | g'4 d' e' f' }`)
	v := voiceOf(t, s, 0, 0, 0)
	fs := v.Elements[1].(*model.Note)
	if fs.Trill == nil || !fs.Trill.Start || fs.Trill.Stop {
		t.Errorf("the trill does not open on the half note: %+v", fs.Trill)
	}
	g := voiceOf(t, s, 0, 1, 0).Elements[0].(*model.Note)
	if g.Trill == nil || !g.Trill.Stop || g.Trill.Start {
		t.Errorf("the trill does not end on the note after the grace: %+v", g.Trill)
	}
	if g.Grace == nil || len(g.Grace.Notes) != 2 {
		t.Errorf("the grace notes are not on the note after: %+v", g.Grace)
	}
}

func TestAnOctaveLineEndingAfterAVoicesLastNote(t *testing.T) {
	s := clean(t, `{ << \relative c''' { \ottava #1 c4 d e f \ottava #0 } \\ \relative c' { e4 f g a } >> | g'1 }`)
	m := s.Parts[0].Measures[0]
	if len(m.Voices) != 2 {
		t.Fatalf("%d voices", len(m.Voices))
	}
	last := m.Voices[0].Elements[3].(*model.Note)
	if last.Octave == nil || !last.Octave.Stop {
		t.Errorf("the line does not end on the voice's last note: %+v", last.Octave)
	}
	if first := m.Voices[1].Elements[0].(*model.Note); first.Octave != nil {
		t.Errorf("the end landed on the other voice: %+v", first.Octave)
	}
}
