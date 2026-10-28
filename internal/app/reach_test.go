/*
 * reach_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How far each staff reaches, through the door the browser uses.
 *
 * The arithmetic is tested in internal/analysis. What is tested here is what
 * is handed to it: that the notes are counted as they sound rather than as
 * they are spelled, that a notehead can be pointed at afterwards, and that a
 * staff of drums is not asked a question about pitch.
 */

package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/analysis"
	"github.com/stricaud/gscore/internal/model"
)

func reachOf(t *testing.T, h *harness) []analysis.PartReach {
	t.Helper()
	var out struct {
		Parts []analysis.PartReach `json:"parts"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/reach")), &out); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	return out.Parts
}

func TestAStaffsTwoEndsAreReportedWithTheirBars(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")  // middle C
	h.note(0, 1, 13, "whole") // an octave up
	h.note(0, 2, 0, "whole")

	got := reachOf(t, h)
	if len(got) != 1 {
		t.Fatalf("%d staves, want 1", len(got))
	}
	r := got[0]
	if r.Highest.Bar != 1 {
		t.Errorf("the highest note is said to be in bar %d, and it is in bar 1",
			r.Highest.Bar)
	}
	if r.Lowest.Bar != 2 {
		t.Errorf("the lowest note is said to be in bar %d, and it is in bar 2",
			r.Lowest.Bar)
	}
	if r.Lowest.Ref == 0 || r.Highest.Ref == 0 {
		t.Error("neither end can be pointed at on the page")
	}
	if r.Lowest.Name == "" || r.Highest.Name == "" {
		t.Error("the notes came back unnamed")
	}
}

// A range is a physical fact about an instrument and does not move because
// the part is spelled in another key. A clarinet's written E sounds a D, and
// it is the D that has to be inside the clarinet.
func TestNotesAreCountedAsTheySoundNotAsTheyAreSpelled(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	written := reachOf(t, h)[0].Lowest.MIDI

	h.app.lock()
	h.app.score.Parts[0].Transpose = model.Transposition{Chromatic: -2, Diatonic: -1}
	h.app.mu.Unlock()

	if n := reachOf(t, h)[0].Lowest.MIDI; n != written-2 {
		t.Errorf("the note sounds %d on a B flat instrument and %d on a C one: "+
			"it was read as it is spelled rather than as it sounds", n, written)
	}
}

// An octave line over the notes moves them too, the same way playback reads
// it: a bracket says where the music is.
func TestAnOctaveLineMovesWhatIsUnderIt(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	plain := reachOf(t, h)[0].Lowest.MIDI

	h.app.lock()
	el := h.app.score.Parts[0].Measures[0].Voices[0].Elements[0]
	marksOfElement(el).Octave = &model.OctaveShift{Amount: 1, Start: true, Stop: true}
	h.app.mu.Unlock()

	if n := reachOf(t, h)[0].Lowest.MIDI; n != plain+12 {
		t.Errorf("the note under the bracket sounds %d and sounded %d without it: "+
			"the octave line was not read", n, plain)
	}
}

// A staff of drums is read by position on a kit, not by pitch. Asked anyway,
// the bass drum of Beethoven's ninth was taken for a string instrument
// written six hundred bars below its range.
func TestADrumStaffIsNotAskedAboutItsRange(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	h.post("/api/staff?part=0&clef=percussion")

	r := reachOf(t, h)[0]
	if !r.Percussion {
		t.Fatal("a staff of drums was not recognised as one")
	}
	if len(r.Findings) != 0 {
		t.Errorf("it was judged anyway: %+v", r.Findings)
	}
}

// A chord is several notes, and the one at the top of it is as much the
// staff's highest note as a single note would be.
func TestEveryNoteOfAChordCounts(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	h.post("/api/addpitch?part=0&measure=0&at=1&step=13")

	r := reachOf(t, h)[0]
	if r.Notes != 2 {
		t.Errorf("%d notes counted in a chord of two", r.Notes)
	}
	if r.Highest.MIDI <= r.Lowest.MIDI {
		t.Errorf("the chord came out as one note: %d to %d",
			r.Lowest.MIDI, r.Highest.MIDI)
	}
}

// An instrument gscore has a range for says so; one it does not stays quiet
// rather than guessing.
func TestTheInstrumentsOwnRangeComesBackWhenItIsKnown(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "whole")
	h.post("/api/staff?part=0&instrument=flute")

	r := reachOf(t, h)[0]
	if r.Playable == nil || r.Easy == nil {
		t.Fatal("the flute's range did not come back")
	}
	if r.Playable.Low != 60 {
		t.Errorf("the flute is said to start at %d, and it starts at middle C",
			r.Playable.Low)
	}
	if r.Easy.High >= r.Playable.High {
		t.Error("the comfortable stretch is not inside the range")
	}
}
