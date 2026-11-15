/*
 * secondvoice_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

/*
 * A command aimed at a place means that place, on the line of music it was
 * aimed at.
 *
 * A request names where: a staff, a bar, and how far into it. It may also
 * name which line of music, and where it does, that is not a hint. Every
 * resolver here read the first voice whatever the request said — so a
 * command aimed at the second voice landed on whatever happened to stand at
 * the same place in the first, which is somebody else's note and was
 * probably already right.
 *
 * It went unnoticed because the editor sends sel= for most things, and the
 * selection grammar has always carried the voice. These are the other
 * door: part, measure and at, which is what the symbols, the clef, the
 * chord symbols and the ties use.
 *
 * Found by sweeping for it after a score turned up that writes its music in
 * the second voice with the first left empty — four bugs in one afternoon,
 * all the same shape.
 */

// twoLineBar writes a bar with a different note in each of two lines of
// music, so that a command landing on the wrong one is visible.
//
// The first voice gets Gs, the second gets Cs.
func twoLineBar(h *harness, measure int) {
	h.t.Helper()
	for i := 0; i < 2; i++ {
		h.note(0, measure, 2, "half") // G4, the first line
	}
	for i := 0; i < 2; i++ {
		h.post(fmt.Sprintf(
			"/api/insert?part=0&measure=%d&at=-1&voice=1&step=-2&dur=half&kind=note",
			measure))
	}
}

// lineNotes is the pitches of one line of music of one bar.
func lineNotes(h *harness, measure, voice int) []string {
	h.t.Helper()
	m := h.app.score.Parts[0].Measures[measure]
	if voice >= len(m.Voices) {
		return nil
	}
	var out []string
	for _, el := range m.Voices[voice].Elements {
		if n, ok := el.(*model.Note); ok {
			out = append(out, n.Pitch.String())
		}
	}
	return out
}

// A symbol put on a note of the second voice belongs to that note.
func TestASymbolGoesOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)
	h.postBody("/api/symbolpicture?name=device", colourPNG(20, 20))

	h.post("/api/insertsymbol?part=0&measure=0&at=1&voice=1&name=device&attach=1")

	m := h.app.score.Parts[0].Measures[0]
	if n := m.Voices[0].Elements[0].(*model.Note); len(n.Symbols) > 0 {
		t.Errorf("the symbol went on the first voice's note")
	}
	if n := m.Voices[1].Elements[0].(*model.Note); len(n.Symbols) != 1 {
		t.Errorf("the second voice's note carries %d symbols, want 1",
			len(n.Symbols))
	}
}

// And a mark aimed by the cursor goes on the line the cursor is on.
func TestAMarkGoesOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)

	h.post("/api/dynamic?part=0&measure=0&at=1&voice=1&dynamic=f")

	m := h.app.score.Parts[0].Measures[0]
	if first := m.Voices[0].Elements[0].(*model.Note); first.Dynamic != model.DynamicNone {
		t.Errorf("the dynamic went on the first voice: %v", first.Dynamic)
	}
	if second := m.Voices[1].Elements[0].(*model.Note); second.Dynamic == model.DynamicNone {
		t.Error("the second voice's note has no dynamic")
	}
}

// Words written beside a note go beside that note.
//
// A staff whose music is written in the second voice with the first left
// empty refused every text outright — "the bar is empty" — because the
// words were aimed at the first line, which has nothing in it to write
// them beside. Typing CHORUS over a chorus did nothing at all.
func TestWordsGoBesideTheNoteTheyWereAimedAt(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 2; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=2&dur=half&kind=note")
	}
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = nil

	h.post("/api/text?part=0&measure=0&at=1&voice=1&text=CHORUS&style=expression")

	n := h.app.score.Parts[0].Measures[0].Voices[1].Elements[0].(*model.Note)
	if len(n.Texts) != 1 || n.Texts[0].Text != "CHORUS" {
		t.Errorf("the second voice's note carries %v, want CHORUS", n.Texts)
	}
}

// And the one that was picked up is the one that changes, on its own line.
func TestWordsOnTheSecondVoiceCanBeTakenAwayAgain(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)
	h.post("/api/text?part=0&measure=0&at=1&voice=1&text=CHORUS&style=expression")

	h.post("/api/text?part=0&measure=0&at=1&voice=1&text=&index=0")

	n := h.app.score.Parts[0].Measures[0].Voices[1].Elements[0].(*model.Note)
	if len(n.Texts) != 0 {
		t.Errorf("the words stayed: %v", n.Texts)
	}
}

// A symbol is made larger on the line it stands on.
func TestASymbolIsResizedOnTheLineItStandsOn(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)
	h.postBody("/api/symbolpicture?name=device", colourPNG(20, 20))
	h.post("/api/insertsymbol?part=0&measure=0&at=1&voice=1&name=device&attach=1")

	h.post("/api/symbolscale?part=0&measure=0&at=1&voice=1&name=device&scale=2")

	n := h.app.score.Parts[0].Measures[0].Voices[1].Elements[0].(*model.Note)
	if len(n.Symbols) != 1 || n.Symbols[0].Scale != 2 {
		t.Errorf("the second voice's symbol is at %v, want 2", n.Symbols)
	}
}

// The cursor remembers which line it was left on.
//
// Where it did not, a score whose music is in the second voice opened with
// the pen on the empty first line, and the first thing typed started a
// second line of music under the one already there.
func TestTheCursorRemembersItsLine(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)

	h.post("/api/cursor?part=0&measure=0&at=2&step=4&voice=1")

	if c := h.app.score.Cursor; c == nil || c.Voice != 1 {
		t.Errorf("the cursor came back on voice %v", c)
	}
	var info struct {
		Cursor struct{ Voice int } `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/info")), &info); err != nil {
		t.Fatal(err)
	}
	if info.Cursor.Voice != 1 {
		t.Errorf("the editor is told to open on voice %d", info.Cursor.Voice)
	}
}

// Transposing a selection moves the notes selected, not the ones at those
// places in the first voice.
func TestTransposingASelectionMovesTheLineItNames(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)
	was := lineNotes(h, 0, 0)

	h.post("/api/transpose?sel=0:0:1/0&semitones=2")

	if got := lineNotes(h, 0, 0); got[0] != was[0] {
		t.Errorf("the first voice moved from %s to %s and was not asked to",
			was[0], got[0])
	}
	if got := lineNotes(h, 0, 1); got[0] == "C4" {
		t.Error("the second voice's note did not move")
	}
}

// A tie runs to the next note of its own line. Looked for in the first
// voice, a tie from a second-voice note found the wrong note or none.
func TestATieRunsToTheNextNoteOfItsOwnLine(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)

	h.post("/api/tie?part=0&measure=0&at=1&voice=1")

	m := h.app.score.Parts[0].Measures[0]
	second := m.Voices[1].Elements
	if !second[0].(*model.Note).TieStart || !second[1].(*model.Note).TieStop {
		t.Error("the second voice's two notes are not tied to each other")
	}
	for i, el := range m.Voices[0].Elements {
		if n := el.(*model.Note); n.TieStart || n.TieStop {
			t.Errorf("the first voice's note %d was tied and was not asked to", i)
		}
	}
}

// A clef written mid-bar goes on the note the cursor is on.
func TestAClefGoesOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)

	h.post("/api/clef?part=0&measure=0&at=2&voice=1&clef=bass")

	m := h.app.score.Parts[0].Measures[0]
	if n := m.Voices[0].Elements[1].(*model.Note); n.Clef != nil {
		t.Error("the clef went on the first voice's note")
	}
	if n := m.Voices[1].Elements[1].(*model.Note); n.Clef == nil {
		t.Error("the second voice's note carries no clef")
	}
}

// Moving a rest by the arrows moves the one named, not its neighbour above.
func TestARestMovesOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "half")
	h.post("/api/insert?part=0&measure=0&at=-1&step=2&dur=half&kind=rest")
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=-2&dur=half&kind=rest")

	h.post("/api/nudge?part=0&measure=0&at=1&voice=1&steps=-2")

	m := h.app.score.Parts[0].Measures[0]
	if r, ok := m.Voices[1].Elements[0].(*model.Rest); !ok || r.Lift != -2 {
		t.Errorf("the second voice's rest was not moved")
	}
	if r, ok := m.Voices[0].Elements[1].(*model.Rest); ok && r.Lift != 0 {
		t.Errorf("the first voice's rest moved by %d", r.Lift)
	}
}

/*
 * And how long a bar lasts is how long its music lasts.
 *
 * It was read off the first voice of each staff, which says nothing at all
 * about a bar whose music is written in the second. How long a bar lasts is
 * what the cursor steps by and what the hit regions are measured against, so
 * a bar read as empty is a bar nothing can be aimed at.
 */

func TestABarLastsAsLongAsItsMusic(t *testing.T) {
	h := newHarness(t)
	// Two quarters in a bar of four: half a bar of music, so the answer
	// cannot be mistaken for the meter's own length.
	for i := 0; i < 2; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=2&dur=quarter&kind=note")
	}
	// The first voice is empty; the second holds the music.
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = nil

	want := model.Duration{Num: 1, Den: 2}
	if bar := h.app.score.MeasureDuration(0); bar.Cmp(want) != 0 {
		t.Errorf("the bar is %v long and its music is %v", bar, want)
	}
}

// The words of a verse come back with the bars counted the same way.
func TestLyricsStillComeBackFromTheSecondVoice(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 2; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=2&dur=half&kind=note")
	}
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = nil

	h.post("/api/lyrics?part=0&verse=1&text=" + url.QueryEscape("one two"))
	var back struct{ Text string }
	if err := json.Unmarshal([]byte(h.get("/api/getlyrics?part=0&verse=1")), &back); err != nil {
		t.Fatal(err)
	}
	if back.Text != "one two" {
		t.Errorf("the verse came back as %q", back.Text)
	}
}

// A tuplet is made of the notes named, on the line they are on.
func TestATupletIsMadeOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
		h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=-2&dur=quarter&kind=note")
	}

	h.post("/api/tuplet?part=0&measure=0&at=3&voice=1&n=3")

	m := h.app.score.Parts[0].Measures[0]
	for i, el := range m.Voices[0].Elements {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			t.Errorf("the first voice's note %d was put in a tuplet", i)
		}
	}
	grouped := 0
	for _, el := range m.Voices[1].Elements {
		if mk := marksOfElement(el); mk != nil && mk.Tuplet != nil {
			grouped++
		}
	}
	if grouped != 3 {
		t.Errorf("%d of the second voice's notes are in the tuplet, want 3", grouped)
	}
}

// And a grace note leans on the note it was written against.
func TestAGraceNoteLeansOnTheLineItWasAimedAt(t *testing.T) {
	h := newHarness(t)
	twoLineBar(h, 0)

	h.post("/api/grace?sel=0:0:1/0&step=0")

	m := h.app.score.Parts[0].Measures[0]
	if mk := marksOfElement(m.Voices[0].Elements[0]); mk != nil && mk.Grace != nil {
		t.Error("the grace note leans on the first voice's note")
	}
	if mk := marksOfElement(m.Voices[1].Elements[0]); mk == nil || mk.Grace == nil {
		t.Error("the second voice's note carries no grace note")
	}
}
