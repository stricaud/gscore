package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// marksAt reads the tie and slur marks of a bar's elements, in order.
func marksAt(t *testing.T, h *harness) []string {
	t.Helper()
	var out []string
	for _, el := range h.app.score.Parts[0].Measures[0].Voices[0].Elements {
		m := marksOfElement(el)
		if m == nil {
			out = append(out, "-")
			continue
		}
		s := ""
		if m.TieStop {
			s += "<t"
		}
		if m.TieStart {
			s += "t>"
		}
		if m.SlurStop > 0 {
			s += "<s"
		}
		if m.SlurStart > 0 {
			s += "s>"
		}
		if s == "" {
			s = "."
		}
		out = append(out, s)
	}
	return out
}

func sameMarks(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d elements, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("element %d is %q, want %q (all of them: %v)",
				i, got[i], want[i], got)
		}
	}
}

// Taking away one tie of a chain leaves the others.
//
// A note held through four bars stops one tie and starts the next on the
// same notehead. Clearing both marks on both ends of the one that was
// clicked took away its neighbours with it, so pointing at the second of
// three ties and pressing Backspace left none at all.
func TestTakingOneTieAwayLeavesTheOnesEitherSideOfIt(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2, "quarter")
	}
	h.post("/api/tie?tie=on&sel=0:0:0,0:0:1,0:0:2,0:0:3")
	sameMarks(t, marksAt(t, h), "t>", "<tt>", "<tt>", "<t")

	// The middle one: from the second note to the third.
	h.post("/api/tie?tie=none&ends=1&step=2&sel=0:0:1,0:0:2")
	sameMarks(t, marksAt(t, h), "t>", "<t", "t>", "<t")
}

// Without ends, it is still "untie these notes", which is what selecting a
// passage and asking for no tie means.
func TestUntyingASelectionStillClearsAllOfIt(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
	}
	h.post("/api/tie?tie=on&sel=0:0:0,0:0:1,0:0:2")
	h.post("/api/tie?tie=none&sel=0:0:0,0:0:1,0:0:2")
	sameMarks(t, marksAt(t, h), ".", ".", ".")
}

// The same for a phrase: a note that closes one slur and opens the next
// keeps the mark that is not being taken away.
func TestTakingOneSlurAwayLeavesTheNoteItMeetsStillSlurred(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
	}
	// Written by hand, because asking twice through the API cannot say it:
	// see the note on handleSlur.
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	marksOfElement(els[0]).SlurStart = 1
	marksOfElement(els[1]).SlurStop = 1
	marksOfElement(els[1]).SlurStart = 1
	marksOfElement(els[2]).SlurStop = 1

	h.post("/api/slur?slur=none&ends=1&sel=0:0:0,0:0:1")
	sameMarks(t, marksAt(t, h), ".", "s>", "<s")
}

// Untying one notehead of a chord that is tied as a whole.
//
// A tie written on the chord covers every notehead in it. Clearing the one
// notehead left the chord's own mark saying "all of them", so the curve was
// drawn again and the click appeared to do nothing at all.
func TestUntyingOneNoteheadOfAChordTiedWhole(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 0, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=0&step=2")
	h.note(0, 0, 0, "quarter")
	h.post("/api/addpitch?part=0&measure=0&index=1&step=2")
	h.post("/api/tie?tie=on&sel=0:0:0,0:0:1")

	h.post("/api/tie?tie=none&ends=1&step=2&sel=0:0:0,0:0:1")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	first := els[0].(*model.Chord)
	second := els[1].(*model.Chord)
	if first.TieStart || second.TieStop {
		t.Error("the chord still carries the tie as a whole, so every " +
			"notehead is still tied")
	}
	// The bottom notehead keeps its tie; the top one has lost it.
	if !first.Notes[0].TieStart || !second.Notes[0].TieStop {
		t.Error("the notehead that was not asked about lost its tie")
	}
	if first.Notes[1].TieStart || second.Notes[1].TieStop {
		t.Error("the notehead that was asked about is still tied")
	}
}

// The editor is told where every curve went, so one can be clicked.
func TestTheHitsCarryTheCurves(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 2, "quarter")
	h.post("/api/tie?tie=on&sel=0:0:0,0:0:1")

	d := h.get("/api/hits?mode=strip")
	if !strings.Contains(d, `"arcs"`) {
		t.Fatal("the hits say nothing about the curves on the page")
	}
	if !strings.Contains(d, `"kind":"tie"`) {
		t.Errorf("no tie among the curves reported:\n%s", d)
	}
}

// A slur that opens on a grace note goes when it is clicked and deleted.
//
// The engraver reports the curve against the note the grace leans on, so
// that is what the editor names as its first end — and that note's own
// mark was cleared, which was already nothing. The curve stayed, and with
// the stop at its far end gone it ran on to the next one.
func TestTakingAwayASlurThatOpensOnAGraceNote(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
	}
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	grace := &model.Note{Pitch: els[0].(*model.Note).Pitch, Dur: model.Duration{Num: 1, Den: 8}}
	grace.Marks.SlurStart = 1
	marksOfElement(els[0]).Grace = &model.GraceGroup{Notes: []*model.Note{grace}}
	marksOfElement(els[2]).SlurStop = 1

	h.post("/api/slur?slur=none&ends=1&sel=0:0:0,0:0:2")
	if grace.Marks.SlurStart != 0 {
		t.Error("the grace note still opens the slur, so it is still drawn")
	}
	sameMarks(t, marksAt(t, h), ".", ".", ".")
}

// heldIntoTheOtherVoice is two bars of a piano's right hand: a chord of F and
// the F an octave up, then the upper F held on in the tune while the lower
// one is held as the bottom of the accompaniment's chord, in the other voice.
// Bars 24 and 25 of Black and Tan Fantasy.
func heldIntoTheOtherVoice(t *testing.T) (*harness, *model.Chord, *model.Chord, *model.Chord) {
	t.Helper()
	half := model.Duration{Num: 1, Den: 2}
	n := func(step model.Step, oct int) *model.Note {
		return &model.Note{Pitch: model.Pitch{Step: step, Octave: oct}, Dur: half}
	}
	held := &model.Chord{Dur: half, Notes: []*model.Note{n(model.StepF, 4), n(model.StepF, 5)}}
	tune := &model.Chord{Dur: half, Notes: []*model.Note{n(model.StepA, 4), n(model.StepF, 5)}}
	under := &model.Chord{Dur: half, Notes: []*model.Note{n(model.StepF, 4), n(model.StepB, 4)}}
	s := model.NewScore()
	p := s.Parts[0]
	p.Time = model.TimeSignature{Beats: 2, BeatValue: 4}
	p.Measures = []*model.Measure{
		{Voices: []*model.Voice{{Elements: []model.Element{held}}}},
		{Voices: []*model.Voice{
			{Elements: []model.Element{tune}},
			{Elements: []model.Element{under}},
		}},
	}
	a := New(s, "")
	a.setPasteboard = nil
	mux := http.NewServeMux()
	a.routes(mux)
	return &harness{t: t, app: a, mux: mux}, held, tune, under
}

// Tying the lower notehead holds it into the other voice's F, which is where
// that note goes on. It was refused: the next note of its own voice has no F
// at that height.
func TestTyingANoteheadIntoTheOtherVoice(t *testing.T) {
	h, held, tune, under := heldIntoTheOtherVoice(t)
	f4 := engrave.StaffStep(model.Treble, model.Pitch{Step: model.StepF, Octave: 4})
	h.post(fmt.Sprintf("/api/tie?tie=on&step=%d&sel=0:0:0", f4))
	if !held.Notes[0].TieStart {
		t.Error("the held F does not start a tie")
	}
	if !under.Notes[0].TieStop {
		t.Error("the tie does not end on the other voice's F")
	}
	if tune.TieStop || tune.Notes[0].TieStop || tune.Notes[1].TieStop {
		t.Error("the tune's chord was marked, which has no F at that height")
	}
}

// And the whole chord: the upper F into the tune, the lower into the other
// voice.
func TestTyingAChordWhoseNotesPartIntoTwoVoices(t *testing.T) {
	h, held, tune, under := heldIntoTheOtherVoice(t)
	h.post("/api/tie?tie=on&sel=0:0:0")
	if !held.TieStart {
		t.Error("the chord does not start its ties")
	}
	if !tune.TieStop {
		t.Error("the upper F is not held into the tune")
	}
	if !under.Notes[0].TieStop {
		t.Error("the lower F is not held into the other voice")
	}
}
