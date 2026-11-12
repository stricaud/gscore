/*
 * contents_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What a script can read.
 *
 * It was a pitch and a length and nothing else, which made the console a
 * thing that writes music and cannot read it: a script could generate a
 * row but could not find the bars with no dynamic in them, or the lyric
 * with no hyphen. Most of what anybody wants a script for is looking
 * rather than writing.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/analysis"
	"github.com/stricaud/gscore/internal/model"
)

// A length is written as a figure and some dots, and the arithmetic that
// turns a duration back into them had no answer for two dots.
func TestALengthIsReadBackAsItIsWritten(t *testing.T) {
	for _, c := range []struct {
		num, den int
		length   int
		dots     int
		what     string
	}{
		{1, 1, 1, 0, "a whole note"},
		{1, 4, 4, 0, "a quarter"},
		{3, 8, 4, 1, "a dotted quarter"},
		{7, 16, 4, 2, "a double-dotted quarter"},
		// Seven thirty-seconds, which is not divisible by three — so
		// undotting one dot at a time cannot reach it, and every
		// double-dotted note came back with no length at all.
		{7, 32, 8, 2, "a double-dotted eighth"},
		{15, 32, 4, 3, "a triple-dotted quarter"},
		{1, 32, 32, 0, "a thirty-second"},
		// A tuplet note is not a length any notehead spells, and saying
		// so is better than guessing.
		{1, 12, 0, 0, "a third of a quarter"},
	} {
		got, dots := plainLength(model.Duration{Num: c.num, Den: c.den})
		if got != c.length || dots != c.dots {
			t.Errorf("%s (%d/%d) read back as length %d with %d dots, want %d and %d",
				c.what, c.num, c.den, got, dots, c.length, c.dots)
		}
	}
}

// contentsOf reads the whole score the way a script does.
func contentsOf(t *testing.T, h *harness) []staffContents {
	t.Helper()
	var out struct {
		Staves []staffContents `json:"staves"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/contents")), &out); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	return out.Staves
}

// Where a note falls in its bar, which a script asking "what is on beat
// three" should not have to add up itself.
func TestEachNoteSaysWhereInTheBarItFalls(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	notes := contentsOf(t, h)[0].Measures[0].Notes
	if len(notes) < 4 {
		t.Fatalf("%d notes came back", len(notes))
	}
	// Nought over one and not the zero value: adding to a duration of
	// nought over nought gives nought over nought, and every note came
	// back at the start of the bar.
	for i, want := range []string{"0/1", "1/4", "1/2", "3/4"} {
		if notes[i].At != want {
			t.Errorf("note %d falls at %q, want %q", i, notes[i].At, want)
		}
	}
}

// Everything written on a note reaches a script.
func TestWhatIsWrittenOnANoteIsRead(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")

	el := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0]
	m := marksOn(el)
	m.Articulations = []model.Articulation{model.Staccato}
	m.Dynamic = model.Forte
	m.Lyrics = []model.Lyric{{Text: "glo", Verse: 1}}
	m.Texts = []model.TextMark{{Text: "dolce"}}
	m.Chord = &model.ChordSymbol{Root: "C", Quality: "m7"}
	m.SlurStart = 1
	m.TieStart = true

	n := contentsOf(t, h)[0].Measures[0].Notes[0]
	for _, c := range []struct{ got, want, what string }{
		{n.Dynamic, "f", "the dynamic"},
		{n.ChordSymbol, "Cm7", "the chord symbol"},
	} {
		if c.got != c.want {
			t.Errorf("%s came back as %q, want %q", c.what, c.got, c.want)
		}
	}
	if len(n.Articulations) != 1 || n.Articulations[0] == "" {
		t.Errorf("the articulation came back as %v", n.Articulations)
	}
	if len(n.Lyrics) != 1 || n.Lyrics[0] != "glo" {
		t.Errorf("the lyric came back as %v", n.Lyrics)
	}
	if len(n.Text) != 1 || n.Text[0] != "dolce" {
		t.Errorf("the text came back as %v", n.Text)
	}
	if !n.SlurStart || !n.TieStart {
		t.Error("the slur and tie ends did not come back")
	}
}

// And everything written on the bar around it.
func TestWhatIsWrittenOnABarIsRead(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	m := h.app.score.Parts[0].Measures[0]
	m.Marker = "A"
	m.SectionTitle = "Presto"
	m.Tempo = &model.Tempo{BPM: 92, Unit: model.Duration{Num: 1, Den: 4}, Text: "Allegro"}

	bar := contentsOf(t, h)[0].Measures[0]
	if bar.Marker != "A" || bar.Section != "Presto" {
		t.Errorf("the bar came back as %+v", bar)
	}
	if bar.Tempo == "" {
		t.Error("the tempo was not read")
	}
	if bar.Number != 1 {
		t.Errorf("the bar is numbered %d, want 1", bar.Number)
	}
}

// The staff says what it is, which a script checking a player's range has
// to know before it can say anything useful.
func TestAStaffSaysWhatItIs(t *testing.T) {
	h := newHarness(t)
	h.post("/api/addstaff?instrument=clarinet&clef=treble")
	h.note(1, 0, 2, "quarter")

	staves := contentsOf(t, h)
	if len(staves) < 2 {
		t.Fatalf("%d staves came back", len(staves))
	}
	if staves[1].Clef == "" || staves[1].Time == "" {
		t.Errorf("the staff came back as %+v", staves[1])
	}
}

// The old shape still works: a script written when this said only pitch
// and length goes on reading bars the way it always did.
func TestTheOlderShapeStillReads(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	st := contentsOf(t, h)[0]
	if len(st.Bars) == 0 || len(st.Bars[0]) == 0 {
		t.Fatal("bars came back empty")
	}
	if st.Bars[0][0].Pitch == "" {
		t.Error("the note in bars has no pitch")
	}
}

// A note in a tuplet is read as it is written, not as it sounds.
//
// Eleven in the time of six are written as eighths and sound as elevenths
// of six of them. A script reporting "an eighth" is reading the page;
// beats is the time it takes.
func TestATupletNoteIsReadAsItIsWritten(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2+i, "eighth")
	}
	// Three in the time of two, which is how a triplet is stored.
	for i, el := range h.app.score.Parts[0].Measures[0].Voices[0].Elements {
		if i >= 3 {
			break
		}
		m := marksOn(el)
		m.Tuplet = &model.Tuplet{Actual: 3, Normal: 2,
			Start: i == 0, Stop: i == 2}
		// And the sounding time they really take.
		if n, ok := el.(*model.Note); ok {
			n.Dur = model.Duration{Num: 1, Den: 12}
		}
	}

	n := contentsOf(t, h)[0].Measures[0].Notes[0]
	if n.Length != 8 {
		t.Errorf("a triplet eighth is read as length %d, want 8 — what is "+
			"written on the page is an eighth", n.Length)
	}
	if n.Beats != "1/12" {
		t.Errorf("it sounds for %q, want 1/12", n.Beats)
	}
	if n.Tuplet != "3:2" {
		t.Errorf("the tuplet came back as %q, want 3:2", n.Tuplet)
	}
}

/*
 * What a chord symbol is made of.
 *
 * The console cannot ask: its verbs are synchronous so a script reads as
 * music rather than as plumbing, so the whole table is handed over before
 * the script runs. What is checked here is the shape the editor reads it
 * in, because a renamed field is a chordNotes() that quietly returns
 * nothing for every chord and nothing anywhere that says why.
 */
func TestTheChordTableIsServedAsTheEditorReadsIt(t *testing.T) {
	h := newHarness(t)
	var got struct {
		Qualities map[string][]struct {
			Degree    int `json:"degree"`
			Semitones int `json:"semitones"`
		} `json:"qualities"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/chordqualities")), &got); err != nil {
		t.Fatalf("the endpoint did not answer: %v", err)
	}
	if len(got.Qualities) < 40 {
		t.Fatalf("%d qualities came back", len(got.Qualities))
	}
	// The chords the editor's own tests spell, by the names they are typed
	// under. A quality missing from here is a chord symbol somebody writes
	// and a script cannot read.
	for _, q := range []string{"", "m", "7", "maj7", "M7", "m7", "dim7", "9"} {
		tones, ok := got.Qualities[q]
		if !ok {
			t.Errorf("%q is not in the table", q)
			continue
		}
		if len(tones) < 2 || tones[0].Degree != 0 || tones[0].Semitones != 0 {
			t.Errorf("%q came back as %+v", q, tones)
		}
	}
}

// Every note the table can be spelled into is a note gscore can read back.
//
// The editor spells a chord by walking up the letters and sharpening or
// flattening each one to land where the table says it sounds. That gives
// names like "bbb4" — B double flat — and a name the pitch parser will not
// take is a chord a script writes and the score refuses, one note at a
// time, after the batch has already begun.
func TestEveryChordToneCanBeWrittenAsAPitch(t *testing.T) {
	// How far above C each letter sounds, and the marks the editor writes
	// for each amount of alteration. The same two facts the editor's
	// chordNotesOf works from.
	letters := "cdefgab"
	semis := map[byte]int{'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11}
	marks := map[int]string{2: "x", 1: "#", 0: "n", -1: "b", -2: "bb"}

	for quality, tones := range analysis.ChordQualities() {
		for ri := 0; ri < 7; ri++ {
			for _, alter := range []int{-1, 0, 1} {
				root := letters[ri]
				rootSemi := semis[root] + alter
				for _, tone := range tones {
					deg := ri + tone.Degree
					up := letters[deg%7]
					need := (rootSemi + tone.Semitones) -
						(semis[up] + 12*(deg/7))
					mark, ok := marks[need]
					if !ok {
						// Not every chord can be written from every root:
						// the sharpened ninth of B sharp wants a triple
						// sharp. The editor gives back nothing there, which
						// is right, so there is nothing to check.
						continue
					}
					name := string(up) + mark + "4"
					if _, _, ok := model.ParsePitch(name); !ok {
						t.Fatalf("%c%s%s spells %q, which is not a pitch",
							root, marks[alter], quality, name)
					}
				}
			}
		}
	}
}
