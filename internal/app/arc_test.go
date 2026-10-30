package app

import (
	"strings"
	"testing"

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
		if m.SlurStop {
			s += "<s"
		}
		if m.SlurStart {
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
	marksOfElement(els[0]).SlurStart = true
	marksOfElement(els[1]).SlurStop = true
	marksOfElement(els[1]).SlurStart = true
	marksOfElement(els[2]).SlurStop = true

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
