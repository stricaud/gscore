/*
 * tremolopair_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Writing and unwriting a two-note tremolo.
 *
 * It is one mark written on two notes, and it was only ever asked for one
 * way round: pick the first note and the mark pairs it with whatever is
 * written next. That is a rule about how the file is stored rather than
 * about what the mark means, and what anybody does with the mouse is pick
 * the two notes it goes between.
 *
 * Taking it off had the other half of the same problem. Cleared from one
 * note, the other was left holding half a pair: nothing is drawn, the note
 * plays once as though it had never been marked, and the mark is still
 * there for the next thing written beside it to pair with.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// tremolosOf reads the tremolo mark of every element of the first bar.
func tremolosOf(t *testing.T, a *App) []*model.Tremolo {
	t.Helper()
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []*model.Tremolo
	for _, el := range a.score.Parts[0].Measures[0].Voices[0].Elements {
		mk := marksOfElement(el)
		if mk == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, mk.Tremolo)
	}
	return out
}

func twoHalves(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.note(0, 0, 6, "half")
	h.note(0, 0, 8, "half")
	return h
}

func TestTwoChosenNotesTakeTheTremoloBetweenThem(t *testing.T) {
	h := twoHalves(t)
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:1")

	got := tremolosOf(t, h.app)
	if len(got) != 2 {
		t.Fatalf("%d notes in the bar, want 2", len(got))
	}
	if got[0] == nil || !got[0].Start || got[0].Strokes != 3 {
		t.Errorf("the first note carries %+v, want the pair opening with three beams", got[0])
	}
	if got[1] == nil || !got[1].Stop || got[1].Strokes != 3 {
		t.Errorf("the second note carries %+v, want the pair closing", got[1])
	}
}

// The order they were picked in is not the order they are played in.
func TestTheTwoNotesCanBeChosenBackwards(t *testing.T) {
	h := twoHalves(t)
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=2&pair=1&sel=0:0:1,0:0:0")

	got := tremolosOf(t, h.app)
	if got[0] == nil || !got[0].Start {
		t.Errorf("the first note carries %+v, want the pair opening on it", got[0])
	}
	if got[1] == nil || !got[1].Stop {
		t.Errorf("the second note carries %+v, want the pair closing on it", got[1])
	}
}

// Two notes with something written between them are not an alternation
// between neighbours, and there is nothing the mark could mean over the
// note in the middle.
func TestTwoNotesWithANoteBetweenThemAreRefused(t *testing.T) {
	h := twoHalves(t)
	h.note(0, 0, 9, "half")
	why := h.postFails("/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:2")
	if why == "" {
		t.Fatal("refused without saying why")
	}
	for i, tr := range tremolosOf(t, h.app) {
		if tr != nil {
			t.Errorf("note %d was marked anyway", i)
		}
	}
}

func TestMoreThanTwoNotesAreRefused(t *testing.T) {
	h := twoHalves(t)
	h.note(0, 0, 9, "half")
	if why := h.postFails(
		"/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:1,0:0:2"); why == "" {
		t.Fatal("refused without saying why")
	}
}

// Removing it from either note removes the whole of it.
func TestTakingATremoloOffOneNoteTakesItOffItsPartner(t *testing.T) {
	for _, which := range []int{0, 1} {
		h := twoHalves(t)
		h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:1")
		h.post("/api/tremolo?part=0&measure=0&at=" + itoa(which+1) +
			"&strokes=0&sel=0:0:" + itoa(which))

		for i, tr := range tremolosOf(t, h.app) {
			if tr != nil {
				t.Errorf("clearing note %d left note %d holding %+v", which, i, tr)
			}
		}
	}
}

// And the note that is left plays once, as it is written.
func TestWhatIsLeftOfAClearedPairSoundsOnce(t *testing.T) {
	h := twoHalves(t)
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:1")
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=0&sel=0:0:0")

	h.app.mu.RLock()
	pb := buildPlayback(h.app.score)
	h.app.mu.RUnlock()
	if len(pb.Events) != 2 {
		t.Errorf("the pair sounds %d notes after it was removed, want the two written",
			len(pb.Events))
	}
}

// And the editor is told where the beams are, because they are the only
// thing on the page that stands for the mark: without a region of their own
// a click on them finds whatever is underneath.
func TestTheTremolosBeamsAreReportedAsAPlaceToClick(t *testing.T) {
	h := twoHalves(t)
	h.post("/api/tremolo?part=0&measure=0&at=1&strokes=3&pair=1&sel=0:0:0,0:0:1")

	var got struct {
		Tremolos []struct {
			Part, Measure, From, To, Beams int
			Width, Height                  float64
		} `json:"tremolos"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/hits?mode=strip")), &got); err != nil {
		t.Fatalf("reading the regions: %v", err)
	}
	if len(got.Tremolos) != 1 {
		t.Fatalf("%d regions reported for one tremolo, want 1", len(got.Tremolos))
	}
	tr := got.Tremolos[0]
	if tr.Beams != 3 {
		t.Errorf("the region says %d beams, and three were asked for", tr.Beams)
	}
	if tr.From == tr.To {
		t.Errorf("the region names note %d at both ends", tr.From)
	}
	if tr.Width <= 0 || tr.Height <= 0 {
		t.Errorf("the region is %.2f by %.2f, which cannot be clicked", tr.Width, tr.Height)
	}
}

// Each note of a two-note tremolo is written with the length of the whole
// tremolo and takes half of it.
//
// Reported from bar 31 of "I put a spell on you": a 12/8 bar shaken between
// a chord and a note, printed as two dotted whole notes with the strokes
// between them. Marked as they were drawn, the two dotted wholes were a bar
// twice as long as its meter, and it played for twice as long.
func TestATremoloPairLastsWhatItsBarHolds(t *testing.T) {
	h := newHarness(t)
	h.app.score.Parts[0].Time = model.TimeSignature{Beats: 12, BeatValue: 8}
	dottedWhole := model.Duration{Num: 3, Den: 2}
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = []model.Element{
		&model.Chord{Dur: dottedWhole, Notes: []*model.Note{
			{Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: dottedWhole},
			{Pitch: model.Pitch{Step: model.StepD, Octave: 5}, Dur: dottedWhole},
		}},
		&model.Note{Pitch: model.Pitch{Step: model.StepA, Octave: 4}, Dur: dottedWhole},
	}
	bar := func() (written, lasts []model.Duration, total model.Duration) {
		h.app.mu.RLock()
		defer h.app.mu.RUnlock()
		total = model.Duration{Num: 0, Den: 1}
		for _, el := range h.app.score.Parts[0].Measures[0].Voices[0].Elements {
			written = append(written, marksOfElement(el).WrittenDuration(el.Duration()))
			lasts = append(lasts, el.Duration())
			total = total.Add(el.Duration())
		}
		return
	}

	h.post("/api/tremolo?strokes=3&pair=1&sel=0:0:0,0:0:1")
	written, lasts, total := bar()
	for i := range written {
		if written[i] != dottedWhole {
			t.Errorf("note %d is drawn as %v, want the dotted whole it was written as", i, written[i])
		}
		if lasts[i] != (model.Duration{Num: 3, Den: 4}) {
			t.Errorf("note %d lasts %v, want half the bar", i, lasts[i])
		}
	}
	if total != dottedWhole {
		t.Errorf("the bar lasts %v, want the 12/8 it is in", total)
	}

	// Taken off, the two are the notes that were written: whole length each.
	h.post("/api/tremolo?strokes=0&pair=1&sel=0:0:0,0:0:1")
	written, lasts, _ = bar()
	for i := range written {
		if written[i] != dottedWhole || lasts[i] != dottedWhole {
			t.Errorf("after taking the tremolo off, note %d is drawn %v and lasts %v, want %v both",
				i, written[i], lasts[i], dottedWhole)
		}
	}

	// And made again, it is halved once, not twice.
	h.post("/api/tremolo?strokes=3&pair=1&sel=0:0:0,0:0:1")
	h.post("/api/tremolo?strokes=2&pair=1&sel=0:0:0,0:0:1")
	if _, _, total := bar(); total != dottedWhole {
		t.Errorf("a pair made twice lasts %v, want %v", total, dottedWhole)
	}
}
