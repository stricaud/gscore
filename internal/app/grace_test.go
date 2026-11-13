/*
 * grace_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * Making a grace note.
 *
 * Everything else about them was here already — the model holds them, the
 * engraver sets them at cue size, playback steals their time, and both the
 * ABC and the Guitar Pro readers bring them in. There was no way to write
 * one. They could be imported and played and never made.
 *
 * The verb is "turn these notes into grace notes": the pitches are entered
 * with the note entry that exists, and then told to lean on the note after
 * them. A command carrying a pitch of its own would be a second way of
 * saying a pitch, with its own bugs about clefs and transposing staves.
 */

// graceOf is the group hanging on one element, or nil.
func graceOf(el model.Element) *model.GraceGroup {
	if mk := marksOfElement(el); mk != nil {
		return mk.Grace
	}
	return nil
}

// fails posts and expects a refusal, returning the reason given.
func (h *harness) fails(path string) string {
	h.t.Helper()
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	if rec.Code == http.StatusOK {
		h.t.Fatalf("POST %s was meant to be refused", path)
	}
	return strings.TrimSpace(rec.Body.String())
}

func TestGraceNoteLeansOnTheNoteAfterIt(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter") // the ornament
	h.note(0, 0, 4, "quarter") // the note it will lean on
	h.note(0, 0, 5, "quarter")

	h.post("/api/grace?part=0&measure=0&at=1")

	els := h.elements(0, 0)
	if len(els) != 2 {
		t.Fatalf("the bar holds %d elements, want 2: a grace note takes no "+
			"time and leaves the voice", len(els))
	}
	g := graceOf(els[0])
	if g == nil || len(g.Notes) != 1 {
		t.Fatalf("the first note carries %v, want one grace note", g)
	}
	if !g.Slashed {
		t.Error("the group is not slashed; a crush is the usual sense of the word")
	}
}

func TestSeveralNotesBecomeOneGroupInOrder(t *testing.T) {
	h := newHarness(t)
	for _, step := range []int{2, 3, 6} {
		h.note(0, 0, step, "quarter")
	}
	h.post("/api/grace?sel=0:0:0,0:0:1")

	els := h.elements(0, 0)
	if len(els) != 1 {
		t.Fatalf("the bar holds %d elements, want the one they lean on", len(els))
	}
	g := graceOf(els[0])
	if g == nil || len(g.Notes) != 2 {
		t.Fatalf("the note carries %v, want two grace notes", g)
	}
	// In the order they were written: an ornament played backwards is a
	// different ornament.
	if g.Notes[0].Pitch.Step >= g.Notes[1].Pitch.Step {
		t.Errorf("the group runs %v then %v, want the order they were written in",
			g.Notes[0].Pitch, g.Notes[1].Pitch)
	}
}

func TestAnAppoggiaturaIsNotSlashed(t *testing.T) {
	// The stroke is the difference between a crush before the beat and an
	// appoggiatura that takes its time from the note it leans on. A player
	// does two different things with them.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	h.post("/api/grace?part=0&measure=0&at=1&slashed=0&make=1")
	if g := graceOf(h.elements(0, 0)[0]); g == nil || g.Slashed {
		t.Errorf("the group is %v, want one that is not slashed", g)
	}
}

func TestTheSlashCanBeChangedAfterwards(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	h.post("/api/grace?part=0&measure=0&at=1")
	h.post("/api/grace?part=0&measure=0&at=1&slashed=0")
	if g := graceOf(h.elements(0, 0)[0]); g == nil || g.Slashed {
		t.Errorf("the group is %v, want the stroke taken off", g)
	}
}

func TestGraceNotesCanBeTakenOff(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	h.post("/api/grace?part=0&measure=0&at=1")
	h.post("/api/grace?part=0&measure=0&at=1&grace=none")
	if g := graceOf(h.elements(0, 0)[0]); g != nil {
		t.Errorf("the note still carries %v", g)
	}
}

func TestAGraceNoteNeedsSomethingToLeanOn(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	if msg := h.fails("/api/grace?part=0&measure=0&at=1"); !strings.Contains(msg, "lean") {
		t.Errorf("refused with %q, want a word about what it leans on", msg)
	}
}

func TestAGraceNoteCannotLeanOnARest(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/insert?part=0&measure=0&at=-1&dur=quarter&kind=rest")
	if msg := h.fails("/api/grace?part=0&measure=0&at=1"); !strings.Contains(msg, "rest") {
		t.Errorf("refused with %q, want a word about the rest", msg)
	}
}

func TestGraceNotesStayInOneBar(t *testing.T) {
	// "The note after it" stops meaning anything once the selection is
	// scattered over the score, so the command says so rather than taking
	// the first few and leaving the rest.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/addmeasure")
	h.note(0, 1, 4, "quarter")
	msg := h.fails("/api/grace?sel=0:0:0,0:1:0")
	if !strings.Contains(msg, "one bar") {
		t.Errorf("refused with %q, want a word about the one bar", msg)
	}
}

func TestGraceNotesAreEngravedSmallAndFirst(t *testing.T) {
	// End to end, because the point of the command is a page: the editor
	// asks, the model holds, and the engraver has to draw the result at cue
	// size in front of the note it leans on. Anything short of that is a
	// field set in memory.
	h := newHarness(t)
	h.note(0, 0, 2, "eighth")
	h.note(0, 0, 3, "eighth")
	h.note(0, 0, 4, "half")
	h.post("/api/grace?sel=0:0:0,0:0:1")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	var small, full []engrave.Glyph
	for _, g := range lay.Glyphs {
		if !strings.HasPrefix(g.Name, "notehead") {
			continue
		}
		// A cue note is drawn at three fifths of the size; a full-size one
		// is left at the default, which the renderer fills in as zero here.
		if g.Size > 0 && g.Size < 4 {
			small = append(small, g)
		} else {
			full = append(full, g)
		}
	}
	if len(small) != 2 {
		t.Fatalf("%d small noteheads engraved, want the 2 grace notes", len(small))
	}
	if len(full) != 1 {
		t.Fatalf("%d full-size noteheads, want the one they lean on", len(full))
	}
	// In front of it, which is what "leaning on" means on the page.
	for i, g := range small {
		if g.X >= full[0].X {
			t.Errorf("grace note %d is at x=%.2f, not before the note at %.2f",
				i+1, g.X, full[0].X)
		}
	}
	if small[0].X >= small[1].X {
		t.Error("the two grace notes are not in the order they were written")
	}
}
