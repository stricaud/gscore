package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Two things picked up on two staves are joined from the earlier to the
// later, whichever order they were picked up in, and the line can be taken
// away again.
func TestALineIsDrawnBetweenTwoStaves(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 2, "quarter")
	h.post("/api/addstaff?instrument=piano")
	ms := h.app.score.Parts
	lower := len(ms) - 1
	// Something to point at on the lower staff: a rest a quarter in.
	ms[lower].Measures[0].Voices[0].Elements = []model.Element{
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}
	// Picked up later first: the rest at the second beat, then the note at
	// the first.
	h.post("/api/staffline?sel=" + itoa(lower) + ":0:1,0:0:0")
	note := ms[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	rest := ms[lower].Measures[0].Voices[0].Elements[1].(*model.Rest)
	if note.StaffLine == nil || !note.StaffLine.Start {
		t.Errorf("the note, which comes first, does not start the line: %+v", note.StaffLine)
	}
	if rest.StaffLine == nil || !rest.StaffLine.Stop {
		t.Errorf("the rest does not end the line: %+v", rest.StaffLine)
	}

	h.post("/api/staffline?sel=0:0:0," + itoa(lower) + ":0:1&remove=1")
	if note.StaffLine != nil || rest.StaffLine != nil {
		t.Error("the line was not taken away")
	}

	if msg := h.postFails("/api/staffline?sel=0:0:0,0:0:1"); msg == "" {
		t.Error("a line between two notes of one staff was accepted")
	}
}
