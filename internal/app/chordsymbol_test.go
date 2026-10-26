/*
 * chordsymbol_test.go
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
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A chord symbol is written over the music, and what it is written over is
// a bar: naming the harmony is something you say about a bar, not about the
// third quaver of one. So the bar being selected is enough, and the symbol
// goes on the first thing in it.

func chordOn(h *harness, part, measure int) *model.ChordSymbol {
	h.t.Helper()
	els := h.elements(part, measure)
	if len(els) == 0 {
		return nil
	}
	m := marksOfElement(els[0])
	if m == nil {
		return nil
	}
	return m.Chord
}

func TestAChordGoesOnTheFirstThingInTheBar(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.note(0, 0, 8, "quarter")

	// at=1 is what the editor sends when a bar is selected and no note is.
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Am7")

	c := chordOn(h, 0, 0)
	if c == nil {
		t.Fatal("no chord was written")
	}
	if c.String() != "Am7" {
		t.Errorf("the chord came out %q", c.String())
	}
	// And on the first note only: a symbol on every note of the bar says
	// the harmony changes on every note.
	if m := marksOfElement(h.elements(0, 0)[1]); m != nil && m.Chord != nil {
		t.Error("the second note was given one as well")
	}
}

// The name and the diagram are separate choices: either, or both.
func TestTheNameAndTheDiagramAreSeparate(t *testing.T) {
	for _, c := range []struct {
		what     string
		query    string
		wantName string
		wantFret bool
	}{
		{"a name alone", "chord=Am7", "Am7", false},
		{"a name and a diagram", "chord=Am7&frame=x02210", "Am7", true},
		{"a diagram alone", "chord=G&frame=320003&name=0", "", true},
	} {
		h := newHarness(t)
		h.note(0, 0, 6, "quarter")
		h.post("/api/chordsymbol?part=0&measure=0&at=1&" + c.query)

		sym := chordOn(h, 0, 0)
		if sym == nil {
			t.Errorf("%s: nothing was written", c.what)
			continue
		}
		if got := sym.String(); got != c.wantName {
			t.Errorf("%s: the name came out %q, want %q", c.what, got, c.wantName)
		}
		if got := sym.Frame != nil; got != c.wantFret {
			t.Errorf("%s: a diagram: %v", c.what, got)
		}
	}
}

// Neither one is nothing, and nothing takes the symbol away.
func TestNeitherNameNorDiagramTakesItOff(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Am7&frame=x02210")
	if chordOn(h, 0, 0) == nil {
		t.Fatal("nothing was written to take off")
	}

	for _, q := range []string{"chord=none", "chord=", "chord=Am7&name=0"} {
		h2 := newHarness(t)
		h2.note(0, 0, 6, "quarter")
		h2.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Am7")
		h2.post("/api/chordsymbol?part=0&measure=0&at=1&" + q)
		if c := chordOn(h2, 0, 0); c != nil {
			t.Errorf("%q left %q behind", q, c.String())
		}
	}
}

// The fingering comes back out the way it was typed, so that changing a
// chord is changing it rather than typing the whole thing again.
func TestTheFingeringComesBackOut(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"x02210", "x02210"},
		{"320003", "320003"},
		{"x 12 14 14 13 x", "x 12 14 14 13 x"},
	} {
		h := newHarness(t)
		h.note(0, 0, 6, "quarter")
		h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=X&frame=" +
			strings.ReplaceAll(c.in, " ", "%20"))

		sym := chordOn(h, 0, 0)
		if sym == nil || sym.Frame == nil {
			t.Errorf("%q: no diagram", c.in)
			continue
		}
		if got := sym.Frame.Spell(); got != c.want {
			t.Errorf("%q came back as %q", c.in, got)
		}
	}
}

// The picker draws what it is about to write, with the same engraver that
// draws the page — so a symbol that looks right in the box looks right on
// the staff.
func TestThePickerDrawsTheChord(t *testing.T) {
	h := newHarness(t)
	svg := h.get("/api/chordpreview?chord=Am7&space=12")

	if !strings.Contains(svg, "<svg") {
		t.Fatal("the picker did not draw anything")
	}
	if !strings.Contains(svg, ">A<") {
		t.Error("the chord's root is not in the picture")
	}
	// No clef: the picture is of what goes above the staff, and a treble
	// clef would be most of its width.
	if strings.Contains(svg, "") {
		t.Error("a clef was drawn in the picture")
	}

	// And the diagram, when there is one.
	withFrame := h.get("/api/chordpreview?chord=G&frame=320003&name=0&space=12")
	if len(withFrame) <= len(svg) {
		t.Error("the diagram added nothing to the picture")
	}
}

// A chord symbol can be pointed at.
//
// It is written on a line of its own above the staff, as far up as the
// highest note of that stretch of music pushed it, and nothing but the
// engraver knows where that turned out to be. Without its box the editor
// could write a chord and then had no way to reach it: a click on "C7"
// found the empty air above the notes and, with a note tool in hand, wrote
// a note into it. The only way to change one was to select the bar under
// it and know that was how.
func TestAChordSymbolCanBePointedAt(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 6, "quarter")
	h.note(0, 0, 8, "quarter")
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=Am7")

	boxes := h.chordBoxes()
	if len(boxes) != 1 {
		t.Fatalf("%d chord symbols have a box, want one", len(boxes))
	}
	b := boxes[0]
	if b["name"] != "Am7" {
		t.Errorf("the box is for %q, want Am7", b["name"])
	}
	if w, hgt := b["width"].(float64), b["height"].(float64); w <= 0 || hgt <= 0 {
		t.Errorf("the box is %vx%v, which cannot be clicked", w, hgt)
	}
	// Above the staff, which is where the symbol is drawn.
	hits := h.hits()
	if len(hits) == 0 {
		t.Fatal("no measures")
	}
	if top := hits[0]["staffTop"].(float64); b["y"].(float64) >= top {
		t.Errorf("the box is at y=%v, below the staff's top at %v", b["y"], top)
	}
}

// Two chords in one bar are two symbols, each pointing at its own note.
//
// "F7 Bb7" over the two halves of a bar is the ordinary way a chart is
// written, and a box that named only the bar would change the first chord
// whichever of them was clicked.
func TestTwoChordsInABarAreTwoSymbols(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 4; i++ {
		h.note(0, 0, 4+i, "quarter")
	}
	h.post("/api/chordsymbol?part=0&measure=0&at=1&chord=F7")
	h.post("/api/chordsymbol?part=0&measure=0&at=3&chord=Bb7")

	boxes := h.chordBoxes()
	if len(boxes) != 2 {
		t.Fatalf("%d boxes, want one for each chord", len(boxes))
	}
	first, second := boxes[0], boxes[1]
	if first["name"] != "F7" || second["name"] != "Bb7" {
		t.Errorf("the boxes are for %q and %q, want F7 and Bb7",
			first["name"], second["name"])
	}
	// Each names the element it is written on, which is how the editor
	// changes one without touching the other.
	if first["ref"] == second["ref"] {
		t.Errorf("both symbols name element %v", first["ref"])
	}
	// And they do not sit on top of each other.
	if left, right := first["x"].(float64)+first["width"].(float64),
		second["x"].(float64); left > right {
		t.Errorf("F7 ends at %.2f and Bb7 starts at %.2f", left, right)
	}
}

// chordBoxes is what the editor is told about the chord symbols on the page.
func (h *harness) chordBoxes() []map[string]any {
	h.t.Helper()
	var out []map[string]any
	for _, c := range h.layoutField("chords") {
		out = append(out, c)
	}
	return out
}

// hits is the same for the measures.
func (h *harness) hits() []map[string]any { return h.layoutField("hits") }

// layoutField reads one list out of the drawing the editor asks for.
func (h *harness) layoutField(name string) []map[string]any {
	h.t.Helper()
	var d map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/hits?mode=strip")), &d); err != nil {
		h.t.Fatalf("hits: %v", err)
	}
	list, _ := d[name].([]any)
	var out []map[string]any
	for _, v := range list {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
