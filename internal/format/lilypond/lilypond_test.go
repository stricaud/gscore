/*
 * lilypond_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package lilypond

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// What comes out has to be LilyPond, not something that looks like it. The
// test that matters is at the bottom: the real program is run over the real
// output and told to complain about everything, because "it compiles" is the
// only claim worth making about source.

func render(t *testing.T, s *model.Score) string {
	t.Helper()
	var b bytes.Buffer
	if err := Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// tune builds a one-staff score from notes a test hands it.
func tune(time model.TimeSignature, bars ...[]model.Element) *model.Score {
	p := &model.Part{Name: "Flute", Clef: model.Treble, Time: time}
	for _, els := range bars {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Identity: model.Identity{Title: "A Tune", Composer: "Nobody"},
		Tempo:    model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts:    []*model.Part{p},
	}
}

func note(step model.Step, octave int, d model.Duration) *model.Note {
	return &model.Note{Pitch: model.Pitch{Step: step, Octave: octave}, Dur: d}
}

var (
	whole    = model.Duration{Num: 1, Den: 1}
	half     = model.Duration{Num: 1, Den: 2}
	quarter  = model.Duration{Num: 1, Den: 4}
	eighth   = model.Duration{Num: 1, Den: 8}
	fourFour = model.TimeSignature{Beats: 4, BeatValue: 4}
)

func TestThePitchesAreSpelledTheWayLilyPondSpellsThem(t *testing.T) {
	for _, c := range []struct {
		p    model.Pitch
		want string
	}{
		{model.Pitch{Step: model.StepC, Octave: 4}, "c'"},
		{model.Pitch{Step: model.StepC, Octave: 3}, "c"},
		{model.Pitch{Step: model.StepC, Octave: 2}, "c,"},
		{model.Pitch{Step: model.StepC, Octave: 5}, "c''"},
		{model.Pitch{Step: model.StepF, Octave: 4, Alter: 1}, "fis'"},
		{model.Pitch{Step: model.StepB, Octave: 3, Alter: -1}, "bes"},
		{model.Pitch{Step: model.StepG, Octave: 4, Alter: 2}, "gisis'"},
		{model.Pitch{Step: model.StepA, Octave: 4, Alter: -2}, "aeses'"},
	} {
		if got := pitchName(c.p); got != c.want {
			t.Errorf("%v came out as %q, want %q", c.p, got, c.want)
		}
	}
}

func TestTheLengthsAreSpelledTheWayLilyPondSpellsThem(t *testing.T) {
	for _, c := range []struct {
		d    model.Duration
		want string
	}{
		{whole, "1"},
		{half, "2"},
		{quarter, "4"},
		{eighth, "8"},
		{model.Duration{Num: 3, Den: 8}, "4."},
		{model.Duration{Num: 7, Den: 16}, "4.."},
		{model.Duration{Num: 1, Den: 32}, "32"},
	} {
		if got := lilyDuration(c.d); got != c.want {
			t.Errorf("%v came out as %q, want %q", c.d, got, c.want)
		}
	}
}

func TestWhatIsWrittenOnANote(t *testing.T) {
	n := note(model.StepC, 4, quarter)
	n.TieStart = true
	n.SlurStart = true
	n.AddArticulation(model.Staccato)
	n.AddArticulation(model.Mordent)
	n.Dynamic = model.Forte
	n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}

	out := render(t, tune(fourFour, []model.Element{n,
		note(model.StepD, 4, quarter), note(model.StepE, 4, half)}))

	for _, want := range []string{"c'4", "~", "-.", "\\mordent", "\\f", "\\<", "("} {
		if !strings.Contains(out, want) {
			t.Errorf("the output does not carry %q:\n%s", want, out)
		}
	}
}

// A note that ends one slur and begins another is ")(", never "()" — which
// is a slur with nothing in it and will not compile.
func TestASlurClosesBeforeTheNextOpens(t *testing.T) {
	a := note(model.StepC, 4, quarter)
	a.SlurStart = true
	b := note(model.StepD, 4, quarter)
	b.SlurStop, b.SlurStart = true, true
	c := note(model.StepE, 4, half)
	c.SlurStop = true

	out := render(t, tune(fourFour, []model.Element{a, b, c}))
	if strings.Contains(out, "()") {
		t.Errorf("an empty slur was written:\n%s", out)
	}
	if !strings.Contains(out, ")(") {
		t.Errorf("the note that does both did not come out as \")(\":\n%s", out)
	}
}

// A slur the score never closed is closed by the writer: the file has to be
// valid whatever the model holds.
func TestAnUnclosedSlurIsClosed(t *testing.T) {
	a := note(model.StepC, 4, whole)
	a.SlurStart = true
	out := render(t, tune(fourFour, []model.Element{a}))
	if !strings.Contains(out, "left open") {
		t.Errorf("nothing was said about the open slur:\n%s", out)
	}
	if strings.Count(out, "(") != strings.Count(out, ")") {
		t.Errorf("the slurs do not balance:\n%s", out)
	}
}

// A stray close is dropped rather than written, for the same reason.
func TestAStraySlurCloseIsDropped(t *testing.T) {
	a := note(model.StepC, 4, whole)
	a.SlurStop = true
	out := render(t, tune(fourFour, []model.Element{a}))
	if strings.Contains(out, "c'1)") {
		t.Errorf("a slur was closed that was never opened:\n%s", out)
	}
}

// Inside a tuplet a note is written as the value it stands in for.
func TestATupletIsOneGroupOfWrittenValues(t *testing.T) {
	var els []model.Element
	for i := 0; i < 3; i++ {
		n := note(model.Step(i), 4, model.Duration{Num: 1, Den: 6})
		n.Tuplet = &model.Tuplet{Actual: 3, Normal: 2, Start: i == 0, Stop: i == 2}
		els = append(els, n)
	}
	els = append(els, note(model.StepG, 4, half))

	out := render(t, tune(fourFour, els))
	if n := strings.Count(out, "\\tuplet 3/2 {"); n != 1 {
		t.Errorf("%d tuplet groups, want one around all three:\n%s", n, out)
	}
	// Three quarters in the time of two: written as quarters.
	if !strings.Contains(out, "\\tuplet 3/2 { c'4 d'4 e'4 }") {
		t.Errorf("the notes were not written as the values they stand in for:\n%s", out)
	}
}

func TestBothKindsOfTremolo(t *testing.T) {
	one := note(model.StepC, 4, whole)
	one.Tremolo = &model.Tremolo{Strokes: 3}
	if out := render(t, tune(fourFour, []model.Element{one})); !strings.Contains(out, "c'1:32") {
		t.Errorf("a one-note tremolo did not come out as a colon:\n%s", out)
	}

	a := note(model.StepC, 4, half)
	b := note(model.StepG, 4, half)
	a.Tremolo = &model.Tremolo{Strokes: 3, Start: true}
	b.Tremolo = &model.Tremolo{Strokes: 3, Stop: true}
	out := render(t, tune(fourFour, []model.Element{a, b}))
	if !strings.Contains(out, "\\repeat tremolo") {
		t.Errorf("a two-note tremolo did not come out as a repeat:\n%s", out)
	}
	// A bar of four beats is thirty-two thirty-seconds, so sixteen pairs.
	if !strings.Contains(out, "\\repeat tremolo 16 { c'32 g'32 }") {
		t.Errorf("the repeat does not add up:\n%s", out)
	}
}

// A transposing instrument is declared, so that LilyPond knows what the
// written pitches sound as.
func TestATransposingInstrumentSaysSo(t *testing.T) {
	s := tune(fourFour, []model.Element{note(model.StepC, 4, whole)})
	s.Parts[0].Name = "Clarinet in B flat"
	s.Parts[0].Transpose = model.Transposition{Diatonic: -1, Chromatic: -2}
	out := render(t, s)
	if !strings.Contains(out, "\\transposition bes") {
		t.Errorf("the transposition is missing or misspelled:\n%s", out)
	}
}

// A staff with no chord symbols gets no chord context. An empty one prints
// "N.C." over every bar of it.
func TestAStaffWithNoChordsGetsNoChordNames(t *testing.T) {
	out := render(t, tune(fourFour, []model.Element{note(model.StepC, 4, whole)}))
	if strings.Contains(out, "ChordNames") {
		t.Errorf("a chord context was made for a staff with no chords:\n%s", out)
	}

	n := note(model.StepC, 4, whole)
	n.Chord = &model.ChordSymbol{Root: "C", Quality: "m7"}
	out = render(t, tune(fourFour, []model.Element{n}))
	if !strings.Contains(out, "ChordNames") || !strings.Contains(out, "c':m7") {
		t.Errorf("the chord symbol did not come out:\n%s", out)
	}
}

// A piece that begins mid-bar says so, or every bar line after it is wrong.
func TestAPickupIsDeclared(t *testing.T) {
	s := tune(fourFour,
		[]model.Element{note(model.StepC, 4, quarter)},
		[]model.Element{note(model.StepD, 4, whole)})
	if out := render(t, s); !strings.Contains(out, "\\partial 4") {
		t.Errorf("the pickup was not declared:\n%s", out)
	}
}

// And a short bar in the middle — the half bar either side of a repeat —
// says how long it is, or the bar line lands by arithmetic somewhere else.
func TestAnIrregularBarSaysHowLongItIs(t *testing.T) {
	s := tune(fourFour,
		[]model.Element{note(model.StepC, 4, whole)},
		[]model.Element{note(model.StepD, 4, half)},
		[]model.Element{note(model.StepE, 4, whole)})
	out := render(t, s)
	if !strings.Contains(out, "measureLength = #1/2") {
		t.Errorf("the short bar did not say so:\n%s", out)
	}
	if !strings.Contains(out, "measureLength = #1/1") {
		t.Errorf("the bar after it did not say it was back to normal:\n%s", out)
	}
}

/*
 * The one that counts
 */

// lilypondAt is the program itself, where it is installed.
func lilypondAt(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("lilypond")
	if err != nil {
		t.Skip("LilyPond is not installed, so this cannot be checked")
	}
	return path
}

// compiles runs LilyPond over the source and fails on anything it says.
func compiles(t *testing.T, src string) {
	t.Helper()
	bin := lilypondAt(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "score.ly")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-dno-point-and-click", "-o",
		filepath.Join(dir, "out"), file)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("LilyPond would not read it: %v\n%s\n--- source ---\n%s",
			err, out, src)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(strings.ToLower(line), "warning") {
			continue
		}
		// More than fifteen staves is more than there are MIDI channels,
		// which is a fact about MIDI and not about the file.
		if strings.Contains(line, "MIDI channel") ||
			strings.Contains(line, "remapping modulo") {
			continue
		}
		t.Errorf("LilyPond complained: %s\n--- source ---\n%s", line, src)
	}
}

// TestLilyPondReadsWhatWeWrite puts a score carrying one of everything
// through the real program.
func TestLilyPondReadsWhatWeWrite(t *testing.T) {
	compiles(t, render(t, everything()))
}

// everything is a score using as much of the model as LilyPond can say, so
// that the compile is a real test rather than a test of four notes.
func everything() *model.Score {
	p := &model.Part{Name: "Clarinet in B flat", Clef: model.Treble,
		Time: fourFour, Key: model.KeySignature{Fifths: -3},
		Transpose: model.Transposition{Diatonic: -1, Chromatic: -2}}

	bar := func(els ...model.Element) *model.Measure {
		return &model.Measure{Voices: []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle}
	}

	// Ties, slurs, articulations, ornaments, a dynamic and a hairpin.
	a := note(model.StepC, 4, quarter)
	a.TieStart, a.SlurStart, a.Dynamic = true, true, model.Piano
	a.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
	a.AddArticulation(model.Staccato)
	b := note(model.StepC, 4, quarter)
	b.TieStop = true
	b.AddArticulation(model.Mordent)
	c := note(model.StepE, 4, quarter)
	c.AddArticulation(model.Accent)
	c.Trill = &model.Trill{Start: true, Stop: true}
	d := note(model.StepG, 4, quarter)
	d.SlurStop = true
	d.Hairpin = &model.Hairpin{Stop: true}
	d.Chord = &model.ChordSymbol{Root: "G", Quality: "7"}
	d.Lyrics = []model.Lyric{{Text: "sing", Verse: 1}}

	// A tuplet.
	var trip []model.Element
	for i := 0; i < 3; i++ {
		n := note(model.Step(i), 4, model.Duration{Num: 1, Den: 6})
		n.Tuplet = &model.Tuplet{Actual: 3, Normal: 2, Start: i == 0, Stop: i == 2}
		trip = append(trip, n)
	}
	trip = append(trip, note(model.StepA, 4, half))

	// Both tremolos, a chord, a rest and some grace notes.
	one := note(model.StepD, 4, half)
	one.Tremolo = &model.Tremolo{Strokes: 2}
	ta := note(model.StepF, 4, quarter)
	tb := note(model.StepA, 4, quarter)
	ta.Tremolo = &model.Tremolo{Strokes: 3, Start: true}
	tb.Tremolo = &model.Tremolo{Strokes: 3, Stop: true}

	ch := &model.Chord{Dur: half, Notes: []*model.Note{
		note(model.StepC, 4, half),
		note(model.StepE, 4, half),
		note(model.StepG, 4, half),
	}}
	gr := note(model.StepB, 3, half)
	gr.Grace = &model.GraceGroup{Slashed: true, Notes: []*model.Note{
		note(model.StepA, 3, eighth),
	}}
	rest := &model.Rest{Dur: half}
	oct := note(model.StepC, 5, half)
	oct.Octave = &model.OctaveShift{Amount: 1, Start: true}
	oct2 := note(model.StepD, 5, half)
	oct2.Octave = &model.OctaveShift{Amount: 1, Stop: true}
	oct2.Texts = []model.TextMark{{Text: "rit.", Placement: model.PlacementAbove}}

	p.Measures = append(p.Measures,
		bar(a, b, c, d),
		bar(trip...),
		bar(one, ta, tb),
		bar(ch, gr),
		bar(rest, oct),
		bar(oct2, note(model.StepE, 5, half)),
	)
	p.Measures[1].Marker = "A"
	p.Measures[2].RightBarline = model.BarRepeatEnd
	p.Measures[3].Ending = &model.Ending{Numbers: []int{1}, Start: true}
	p.Measures[3].RightBarline = model.BarRepeatEnd
	p.Measures[4].Ending = &model.Ending{Numbers: []int{2}, Start: true, Stop: true}
	p.Measures[5].Key = &model.KeySignature{Fifths: 2}
	p.Measures[5].Time = &model.TimeSignature{Beats: 4, BeatValue: 4}

	// A second staff, so the score block is the parallel kind.
	q := &model.Part{Name: "Cello", Clef: model.Bass, Time: fourFour}
	for range p.Measures {
		q.Measures = append(q.Measures, bar(note(model.StepC, 3, whole)))
	}

	s := &model.Score{
		Identity: model.Identity{Title: "One of Everything",
			Subtitle: "a test", Composer: "gscore"},
		Tempo: model.Tempo{BPM: 96, Unit: quarter},
		Parts: []*model.Part{p, q},
	}
	s.Normalise()
	return s
}

// And an empty score is still a file LilyPond will read.
func TestAnEmptyScoreStillCompiles(t *testing.T) {
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: quarter},
		Parts: []*model.Part{{Name: "Staff", Clef: model.Treble, Time: fourFour,
			Measures: []*model.Measure{{Voices: []*model.Voice{{}}}}}},
	}
	compiles(t, render(t, s))
}
