/*
 * spans_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The marks a score has and the editor could not write.
 *
 * Each of these was already read from every format gscore imports, drawn by
 * the engraver, played back and written out again — and there was no way to
 * put one into a score by hand. A trill, the pedal, the stroke up a chord,
 * the slide from one note to another, and a clef written partway down a
 * staff: all of them arrive in a file and none of them could be typed.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// marksAt is what is written on one element of the first staff.
func markedAt(h *harness, measure, index int) *model.Marks {
	els := h.app.score.Parts[0].Measures[measure].Voices[0].Elements
	if index >= len(els) {
		h.t.Fatalf("bar %d holds %d things", measure+1, len(els))
	}
	return marksOfElement(els[index])
}

// four notes in the first bar, for a mark to be hung on.
func fourNotes(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 2+i, "quarter")
	}
	return h
}

func TestATrillOnOneNoteCarriesBothEnds(t *testing.T) {
	// "tr" over a single notehead, which is how most trills are written.
	h := fourNotes(t)
	h.post("/api/trill?part=0&measure=0&at=1&sel=0:0:0&trill=on")
	tr := markedAt(h, 0, 0).Trill
	if tr == nil || !tr.Start || !tr.Stop {
		t.Fatalf("the note carries %+v", tr)
	}
}

func TestATrillOverAPassageIsALine(t *testing.T) {
	h := fourNotes(t)
	h.post("/api/trill?part=0&measure=0&at=1&sel=0:0:0,0:0:1,0:0:2&trill=on")
	if tr := markedAt(h, 0, 0).Trill; tr == nil || !tr.Start || tr.Stop {
		t.Errorf("the first note carries %+v", tr)
	}
	if tr := markedAt(h, 0, 2).Trill; tr == nil || tr.Start || !tr.Stop {
		t.Errorf("the last note carries %+v", tr)
	}
	h.post("/api/trill?part=0&measure=0&at=1&sel=0:0:0,0:0:1,0:0:2&trill=none")
	if tr := markedAt(h, 0, 0).Trill; tr != nil {
		t.Errorf("the trill was not taken away: %+v", tr)
	}
}

func TestThePedalGoesDownOnTheFirstNoteAndUpOnTheLast(t *testing.T) {
	h := fourNotes(t)
	h.post("/api/pedal?part=0&measure=0&at=1&sel=0:0:0,0:0:1,0:0:3&pedal=on")
	if p := markedAt(h, 0, 0).Pedal; p == nil || !p.Start || p.Stop {
		t.Errorf("the pedal does not go down on the first note: %+v", p)
	}
	if p := markedAt(h, 0, 3).Pedal; p == nil || !p.Stop || p.Start {
		t.Errorf("the pedal does not come up on the last: %+v", p)
	}
	// The notch in the middle of a line: released and pressed again,
	// without lifting the phrase.
	h.post("/api/pedal?part=0&measure=0&at=1&sel=0:0:2&pedal=change")
	if p := markedAt(h, 0, 2).Pedal; p == nil || !p.Change {
		t.Errorf("the pedal does not change there: %+v", p)
	}
	// And the old way of writing it, "Ped." and a star.
	h.post("/api/pedal?part=0&measure=0&at=1&sel=0:0:0&pedal=on&style=text")
	if p := markedAt(h, 0, 0).Pedal; p == nil || !p.Text {
		t.Errorf("the pedal was not asked for in words: %+v", p)
	}
}

func TestAChordIsRolled(t *testing.T) {
	h := fourNotes(t)
	h.post("/api/arpeggio?part=0&measure=0&at=1&sel=0:0:0&arpeggio=on")
	if a := markedAt(h, 0, 0).Arpeggio; a == nil || a.Down {
		t.Errorf("the chord is rolled %+v", a)
	}
	h.post("/api/arpeggio?part=0&measure=0&at=1&sel=0:0:1&arpeggio=down")
	if a := markedAt(h, 0, 1).Arpeggio; a == nil || !a.Down {
		t.Errorf("the chord is rolled %+v", a)
	}
	h.post("/api/arpeggio?part=0&measure=0&at=1&sel=0:0:0&arpeggio=none")
	if a := markedAt(h, 0, 0).Arpeggio; a != nil {
		t.Errorf("the roll was not taken away: %+v", a)
	}
}

func TestAGlissandoRunsFromOneNoteToAnother(t *testing.T) {
	h := fourNotes(t)
	h.post("/api/glissando?part=0&measure=0&at=1&sel=0:0:0,0:0:1&glissando=on")
	if g := markedAt(h, 0, 0).Glissando; g == nil || !g.Start {
		t.Errorf("the slide does not leave the first note: %+v", g)
	}
	if g := markedAt(h, 0, 1).Glissando; g == nil || !g.Stop {
		t.Errorf("the slide does not arrive: %+v", g)
	}
	// A portamento is the same thing drawn wavy, with its own word.
	h.post("/api/glissando?part=0&measure=0&at=1&sel=0:0:2,0:0:3" +
		"&glissando=on&line=wavy&text=port.")
	if g := markedAt(h, 0, 2).Glissando; g == nil || !g.Wavy || g.Text != "port." {
		t.Errorf("the portamento came out as %+v", g)
	}
}

func TestAClefCanChangePartwayDownAStaff(t *testing.T) {
	// A cellist going up into the tenor clef for a passage and dropping
	// back. The model has carried these since the importers started
	// reading them; nothing could write one.
	h := newHarness(t)
	h.post("/api/addmeasure")
	h.post("/api/addmeasure")
	h.post("/api/staff?part=0&clef=tenor&at=1")
	p := h.app.score.Parts[0]
	if p.Clef != model.Treble {
		t.Errorf("the staff's own clef became %v", p.Clef)
	}
	if c := p.Measures[1].Clef; c == nil || *c != model.Tenor {
		t.Errorf("bar 2 carries %v", c)
	}
	// And back again, which is a second change and not the staff's clef.
	h.post("/api/staff?part=0&clef=bass&at=2")
	if c := p.Measures[2].Clef; c == nil || *c != model.Bass {
		t.Errorf("bar 3 carries %v", c)
	}
	// Taking one away.
	h.post("/api/staff?part=0&clef=none&at=1")
	if c := p.Measures[1].Clef; c != nil {
		t.Errorf("the change at bar 2 is still there: %v", c)
	}
	// Asking for the clef the staff already reads in writes no sign: a
	// clef that changes nothing is a sign nobody can act on.
	h.post("/api/staff?part=0&clef=treble&at=1")
	if c := p.Measures[1].Clef; c != nil {
		t.Errorf("a clef that changes nothing was written: %v", c)
	}
	// With no bar named it is still the staff's own clef, which is what
	// the palette has always done.
	h.post("/api/staff?part=0&clef=alto")
	if p.Clef != model.Alto {
		t.Errorf("the staff's clef is %v", p.Clef)
	}
}
