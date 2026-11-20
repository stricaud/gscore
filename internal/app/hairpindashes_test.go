/*
 * hairpindashes_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Changing the line after a "cresc." already written, as the dialog does.
 */

package app

import (
	"strings"
	"testing"
)

func TestTheLineAfterTheWordCanBeChanged(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(1)
	for i := 0; i < 3; i++ {
		h.note(0, 0, 2, "quarter")
	}
	h.post("/api/hairpin?hairpin=cresc.&sel=0:0:0,0:0:3")
	first := marksOfElement(h.app.score.Parts[0].Measures[0].Voices[0].Elements[0])
	if first.Hairpin == nil || first.Hairpin.Dashes != 0 {
		t.Fatalf("a new cresc. carries %+v, want its line to the end", first.Hairpin)
	}
	h.post("/api/hairpin?dashes=none&sel=0:0:0")
	if first.Hairpin.Dashes != -1 || !first.Hairpin.Words || !first.Hairpin.Start {
		t.Errorf("asked for no line, the hairpin is %+v", first.Hairpin)
	}
	h.post("/api/hairpin?dashes=3&sel=0:0:0")
	if first.Hairpin.Dashes != 3 {
		t.Errorf("asked for three dashes, the hairpin is %+v", first.Hairpin)
	}
	h.post("/api/hairpin?dashes=end&sel=0:0:0")
	if first.Hairpin.Dashes != 0 {
		t.Errorf("asked for the line to the end, the hairpin is %+v", first.Hairpin)
	}
	if msg := h.postFails("/api/hairpin?dashes=many&sel=0:0:0"); !strings.Contains(msg, "end, none or a number") {
		t.Errorf("a count that is not one was answered %q", msg)
	}
	// A wedge has no line to change.
	h.post("/api/hairpin?hairpin=crescendo&sel=0:0:0,0:0:3")
	if msg := h.postFails("/api/hairpin?dashes=none&sel=0:0:0"); !strings.Contains(msg, "written as words") {
		t.Errorf("changing the line of a wedge was answered %q", msg)
	}
}
