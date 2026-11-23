package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// Words picked up and moved with the arrows go where they were sent, stay
// there when the words are changed, and are put back by an absolute 0,0.
// The "l.h." of Black and Tan Fantasy, set by rule against the noteheads it
// was about, was the case: it could be written, but not moved.
func TestWordsOnANoteCanBeMoved(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=l.h.&placement=below")
	textX := func() (float64, float64) {
		lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
		for _, x := range lay.Texts {
			if x.S == "l.h." {
				return x.X, x.Y
			}
		}
		t.Fatal("the words were not drawn")
		return 0, 0
	}
	x0, y0 := textX()

	h.post("/api/textmove?part=0&measure=0&at=1&index=0&dx=-1&dy=0.5")
	h.post("/api/textmove?part=0&measure=0&at=1&index=0&dx=-0.25&dy=0")
	x1, y1 := textX()
	if d := x1 - x0; d > -1.2 || d < -1.3 {
		t.Errorf("moved %.2f across, want -1.25", d)
	}
	if d := y1 - y0; d < 0.45 {
		t.Errorf("moved %.2f down, want at least 0.5", d)
	}

	// Changing the words keeps where they were put.
	h.post("/api/text?part=0&measure=0&at=1&index=0&text=l.h.&placement=below")
	n := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	if n.Texts[0].OffsetX != -1.25 {
		t.Errorf("after changing the words the offset is %.2f", n.Texts[0].OffsetX)
	}

	h.post("/api/textmove?part=0&measure=0&at=1&index=0&dx=0&dy=0&absolute=1")
	if n.Texts[0].OffsetX != 0 || n.Texts[0].OffsetY != 0 {
		t.Error("an absolute 0,0 did not put the words back")
	}

	if msg := h.postFails("/api/textmove?part=0&measure=0&at=1&index=3&dx=1&dy=0"); msg == "" {
		t.Error("a text the note does not have was moved")
	}
}
