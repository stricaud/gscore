/*
 * voice_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * Writing a second line on one staff.
 *
 * A measure holds voices and a voice holds elements; two independent lines
 * on one staff — a held note under a moving part — are two voices, written
 * across each other rather than after each other. The model has always said
 * so and the engraver has always drawn it. The editor could only ever reach
 * the first, so a second voice could be imported, drawn, played and printed,
 * and never touched.
 *
 * And worse than untouchable. The drawing reported an element's place in the
 * bar's drawing order — every voice flattened into one list — and the editor
 * sent that number back as an index into the first voice. With one voice the
 * two are the same number; with two they come apart at the first note, and
 * every click in the bar reached something other than what was clicked.
 */

// quarters writes n quarter notes into the voice given, at the end of bar 0.
func (h *harness) quarters(voice, n int) {
	for i := 0; i < n; i++ {
		h.post("/api/insert?part=0&measure=0&at=-1&step=4&dur=quarter&kind=note" +
			voiceQ(voice))
	}
}

func voiceQ(v int) string {
	if v == 0 {
		return ""
	}
	return "&voice=" + string(rune('0'+v))
}

// firstBar is bar one of the only staff.
func firstBar(t *testing.T, h *harness) *model.Measure {
	t.Helper()
	return h.app.score.Parts[0].Measures[0]
}

func TestWritingIntoASecondVoiceMakesIt(t *testing.T) {
	// There is no other moment a second voice could begin. Asking somebody
	// to create one first would be asking them to say twice what they have
	// already said by choosing it.
	h := newHarness(t)
	h.quarters(0, 4)
	if n := len(firstBar(t, h).Voices); n != 1 {
		t.Fatalf("the bar has %d voices before anything asked for a second", n)
	}

	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	m := firstBar(t, h)
	if len(m.Voices) != 2 {
		t.Fatalf("the bar has %d voices, wanted 2", len(m.Voices))
	}
	if n := len(m.Voices[0].Elements); n != 4 {
		t.Errorf("the first voice has %d notes, wanted the 4 it had", n)
	}
	if n := len(m.Voices[1].Elements); n != 1 {
		t.Fatalf("the second voice has %d notes, wanted 1", n)
	}
	whole := model.Duration{Num: 1, Den: 1}
	if got := m.Voices[1].Elements[0].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("the held note is %v, wanted a semibreve", got)
	}
	// Both lines fill the bar, which is what makes them two voices rather
	// than one line with a hole in it.
	if got := m.Voices[0].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("the first voice lasts %v", got)
	}
}

func TestTheDrawingNamesEachNoteByItsOwnVoice(t *testing.T) {
	// The index the editor sends back is an index into one line of the
	// staff, because that is the only index the model has. Reported as the
	// place in the bar's drawing order, the second quarter of a bar of four
	// over a semibreve came back as 2 — and Voices[0][2] is the third.
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	var upper, lower []engrave.ElementPos
	for _, hit := range lay.Hits {
		if hit.Part != 0 || hit.Measure != 0 {
			continue
		}
		for _, el := range hit.Elements {
			if el.Voice == 0 {
				upper = append(upper, el)
			} else {
				lower = append(lower, el)
			}
		}
	}
	if len(upper) != 4 || len(lower) != 1 {
		t.Fatalf("%d drawn in voice 1 and %d in voice 2, wanted 4 and 1",
			len(upper), len(lower))
	}
	for i, el := range upper {
		if el.Index != i {
			t.Errorf("the %d%s note of voice 1 is drawn as index %d",
				i+1, "th", el.Index)
		}
	}
	if lower[0].Index != 0 {
		t.Errorf("the held note is index %d; it is the first of its own line",
			lower[0].Index)
	}
}

func TestASelectionCanNameTheSecondVoice(t *testing.T) {
	// "0:0:1/0" is the first element of the second line of bar 0. A bare
	// index goes on meaning the first line, which is what every selection
	// written before voices could be reached already meant.
	got, err := parseTargets("0:0:1/2")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].voice != 1 || got[0].index != 2 {
		t.Errorf("read as %+v, wanted voice 1 index 2", got[0])
	}
	plain, err := parseTargets("0:3:2")
	if err != nil {
		t.Fatal(err)
	}
	if plain[0].voice != 0 || plain[0].index != 2 {
		t.Errorf("a bare index read as %+v, wanted voice 0 index 2", plain[0])
	}
	// With a grace note after it, which is the fourth field.
	g, err := parseTargets("0:3:1/2:1")
	if err != nil {
		t.Fatal(err)
	}
	if g[0].voice != 1 || g[0].index != 2 || g[0].graceOrd() != 1 {
		t.Errorf("read as %+v", g[0])
	}
	if _, err := parseTargets("0:3:9/2"); err == nil {
		t.Error("a voice past the end was accepted")
	}
}

func TestAMarkGoesOnTheVoiceThatWasPickedUp(t *testing.T) {
	// The test the whole coordinate exists for. Both lines have an element
	// 0, so a command that names only the index reaches whichever line the
	// server looks in first — which was always the upper one.
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	h.post("/api/articulation?articulation=staccato&sel=0:0:1/0")

	m := firstBar(t, h)
	if mk := marksOfElement(m.Voices[1].Elements[0]); mk == nil ||
		len(mk.Articulations) == 0 {
		t.Error("the held note of the second voice has no staccato on it")
	}
	if mk := marksOfElement(m.Voices[0].Elements[0]); mk != nil &&
		len(mk.Articulations) > 0 {
		t.Error("the first voice's note was marked instead")
	}
}

func TestEmptyingASecondVoiceTakesItAway(t *testing.T) {
	// A bar of one voice and an empty line nobody can see is not a bar of
	// one voice: the empty line is counted in the numbering, written out as
	// a voice, and makes the staff answer "two" to everything that asks.
	h := newHarness(t)
	h.quarters(0, 2)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=half&kind=note")
	if n := len(firstBar(t, h).Voices); n != 2 {
		t.Fatalf("the setup made %d voices", n)
	}

	h.post("/api/delete?part=0&measure=0&voice=1&at=0")

	if n := len(firstBar(t, h).Voices); n != 1 {
		t.Errorf("the bar still has %d voices after the second was emptied", n)
	}
	if n := len(firstBar(t, h).Voices[0].Elements); n != 2 {
		t.Errorf("the first voice lost notes: %d left", n)
	}
}

func TestDeletingReachesTheVoiceItNames(t *testing.T) {
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	// Element 0 of the second line, not element 0 of the first.
	h.post("/api/delete?part=0&measure=0&voice=1&at=0")

	m := firstBar(t, h)
	if n := len(m.Voices[0].Elements); n != 4 {
		t.Errorf("the first voice has %d notes; none of them was asked for", n)
	}
}

func TestADurationChangeReachesTheVoiceItNames(t *testing.T) {
	// It used to take the first voice holding anything at that index and
	// stop there, so picking up the second voice's note and asking for a
	// shorter one changed the first voice's note and left the selected one
	// exactly as it was.
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	h.post("/api/duration?value=half&sel=0:0:1/0")

	m := firstBar(t, h)
	half := model.Duration{Num: 1, Den: 2}
	quarter := model.Duration{Num: 1, Den: 4}
	if got := m.Voices[1].Elements[0].Duration(); got.Cmp(half) != 0 {
		t.Errorf("the held note is %v, wanted a minim", got)
	}
	if got := m.Voices[0].Elements[0].Duration(); got.Cmp(quarter) != 0 {
		t.Errorf("the first voice's note became %v; it was not selected", got)
	}
}

func TestBuildingAChordReachesTheVoiceItNames(t *testing.T) {
	// A chord is built from the bottom up: write the lowest note, move the
	// cursor, add the next. Adding read the first voice whatever line was
	// being written in, so somebody writing a second voice asked for another
	// chord note and watched a notehead appear in the voice above — a second
	// away from a note already there, and unreadable.
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	// Element 0 of the second line, two steps above the note already there.
	h.post("/api/addpitch?part=0&measure=0&voice=1&index=0&step=2")

	m := firstBar(t, h)
	ch, ok := m.Voices[1].Elements[0].(*model.Chord)
	if !ok {
		t.Fatalf("the second voice holds a %T, wanted a chord", m.Voices[1].Elements[0])
	}
	if n := len(ch.Notes); n != 2 {
		t.Errorf("the chord has %d noteheads, wanted 2", n)
	}
	for i, el := range m.Voices[0].Elements {
		if _, isChord := el.(*model.Chord); isChord {
			t.Fatalf("note %d of the first voice became a chord; "+
				"the notehead went into the wrong line", i+1)
		}
	}
}

func TestAChordCannotBeBuiltOnAVoiceThatIsNotThere(t *testing.T) {
	// Adding a notehead builds on a note that is already written, so an
	// empty line means the selection is stale rather than that a line
	// should be made.
	h := newHarness(t)
	h.quarters(0, 4)
	if msg := h.postFails(
		"/api/addpitch?part=0&measure=0&voice=1&index=0&step=2",
	); msg == "" {
		t.Error("a chord was built on a line with nothing in it")
	}
	if n := len(firstBar(t, h).Voices); n != 1 {
		t.Errorf("the bar grew to %d voices for a chord nobody could build", n)
	}
}

func TestThereIsALimitToTheVoices(t *testing.T) {
	h := newHarness(t)
	if msg := h.postFails(
		"/api/insert?part=0&measure=0&at=-1&voice=9&step=4&dur=quarter&kind=note",
	); !strings.Contains(msg, "voice") {
		t.Errorf("a ninth voice was refused with %q", msg)
	}
}

/*
 * And it survives the file.
 *
 * A second voice written in the editor is only worth writing if it is still
 * there when the score is opened again, and still there when it is handed to
 * another program. The gscore format keeps the voices because the measure it
 * serialises holds them; this says so out loud, because a format is exactly
 * the thing nobody notices has lost something until months later.
 */

func TestASecondVoiceSurvivesSavingAndOpening(t *testing.T) {
	h := newHarness(t)
	h.quarters(0, 4)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=whole&kind=note")

	path := filepath.Join(t.TempDir(), "two-voices.gscore")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gscorefmt.Write(f, h.app.score, h.app.symbols); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// A fresh editor, as though the program had been closed and started
	// again.
	open := newHarness(t)
	if err := open.app.openPath(path); err != nil {
		t.Fatal(err)
	}
	m := firstBar(t, open)
	if len(m.Voices) != 2 {
		t.Fatalf("the bar opened with %d voices, wanted 2", len(m.Voices))
	}
	if n := len(m.Voices[0].Elements); n != 4 {
		t.Errorf("the first voice opened with %d notes, wanted 4", n)
	}
	if n := len(m.Voices[1].Elements); n != 1 {
		t.Fatalf("the second voice opened with %d notes, wanted 1", n)
	}
	whole := model.Duration{Num: 1, Den: 1}
	if got := m.Voices[1].Elements[0].Duration(); got.Cmp(whole) != 0 {
		t.Errorf("the held note opened as %v, wanted a semibreve", got)
	}
}

func TestASecondVoiceCanBeEditedAfterOpening(t *testing.T) {
	// The point of keeping it: a voice that comes back read-only is a
	// voice that has to be written somewhere else every time.
	h := newHarness(t)
	h.quarters(0, 2)
	h.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=0&dur=half&kind=note")

	path := filepath.Join(t.TempDir(), "two-voices.gscore")
	f, _ := os.Create(path)
	if err := gscorefmt.Write(f, h.app.score, h.app.symbols); err != nil {
		t.Fatal(err)
	}
	f.Close()

	open := newHarness(t)
	if err := open.app.openPath(path); err != nil {
		t.Fatal(err)
	}
	// A second note into the line that came out of the file, and then a
	// mark on the first of them, named by the selection grammar.
	open.post("/api/insert?part=0&measure=0&at=-1&voice=1&step=2&dur=half&kind=note")
	open.post("/api/articulation?articulation=staccato&sel=0:0:1/0")

	m := firstBar(t, open)
	if n := len(m.Voices[1].Elements); n != 2 {
		t.Fatalf("the second voice has %d notes after writing into it", n)
	}
	if mk := marksOfElement(m.Voices[1].Elements[0]); mk == nil ||
		len(mk.Articulations) == 0 {
		t.Error("the mark did not land on the reopened second voice")
	}
}
