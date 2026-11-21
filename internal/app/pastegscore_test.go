/*
 * pastegscore_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Copying in one gscore and pasting in another running beside it.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// systemClipboard is the one clipboard two gscores on a machine share.
type systemClipboard struct {
	kind   string
	data   []byte
	change int
}

func (c *systemClipboard) read(kind string) ([]byte, int) {
	if kind != c.kind {
		return nil, c.change
	}
	return c.data, c.change
}

func (c *systemClipboard) write(kind string, data []byte) int {
	c.kind, c.data = kind, append([]byte(nil), data...)
	c.change++
	return c.change
}

// gscoreOn is a gscore of its own — its own memory, its own copy — on the
// machine whose clipboard that is.
func gscoreOn(t *testing.T, cb *systemClipboard) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	h.app.server = nil // a program of its own, not a window of the other
	h.app.local = true
	h.app.pasteboard = cb.read
	h.app.setPasteboard = cb.write
	return h
}

func TestMusicCopiedInAnotherGscoreIsPasted(t *testing.T) {
	cb := &systemClipboard{change: 3}
	one, two := gscoreOn(t, cb), gscoreOn(t, cb)

	// The second has a copy of its own, made first.
	two.note(0, 0, 0, "half")
	two.post("/api/copy?sel=0:0:0")

	// Then two notes are copied in the first, the first of them staccato.
	one.note(0, 0, 4, "quarter")
	one.note(0, 0, 5, "quarter")
	g := one.app.score.Parts[0].Measures[0].Voices[0].Elements[0].(*model.Note)
	g.Marks.Articulations = append(g.Marks.Articulations, model.Staccato)
	one.post("/api/copy?sel=0:0:0,0:0:1")

	d := two.post("/api/paste?part=0&measure=0&at=0")
	if d["from"] != "another gscore" || d["pasted"] != float64(2) {
		t.Fatalf("the paste is %v, want the first gscore's two notes", d)
	}
	els := two.app.score.Parts[0].Measures[0].Voices[0].Elements
	first, ok := els[0].(*model.Note)
	if !ok || first.Pitch.String() != g.Pitch.String() || !first.Marks.HasArticulation(model.Staccato) {
		t.Errorf("bar 1 opens with %#v, want the staccato %s", els[0], g.Pitch)
	}

	// And the first, pasting, gets its own copy and is not told it came
	// from elsewhere.
	d = one.post("/api/paste?part=0&measure=1&at=0")
	if _, outside := d["from"]; outside || d["pasted"] != float64(2) {
		t.Errorf("the first gscore's own paste is %v", d)
	}
}
