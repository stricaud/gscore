/*
 * breath_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the player breathes.
 *
 * The mark had been in gscore all along without a way to ask for one: the
 * model carried it, the engraver drew all four of them, the file format
 * saved them and a MusicXML file brought them in — and a wind part typed
 * into gscore could not say where to take a breath, which is the one mark a
 * wind player cannot do without.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// breathOf is the mark on the first note of the first bar.
func breathOf(h *harness) model.Breath {
	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if len(els) == 0 {
		return model.NoBreath
	}
	n, ok := els[0].(*model.Note)
	if !ok {
		return model.NoBreath
	}
	return n.Break
}

func TestABreathMarkIsPutOnTheNote(t *testing.T) {
	for _, c := range []struct {
		asked string
		want  model.Breath
	}{
		{"comma", model.BreathComma},
		{"tick", model.BreathTick},
		{"caesura", model.Caesura},
		{"curved caesura", model.CaesuraCurved},
	} {
		h := newHarness(t)
		h.note(0, 0, 2, "quarter")
		h.post("/api/breath?part=0&measure=0&at=1&breath=" +
			strings.ReplaceAll(c.asked, " ", "%20"))
		if got := breathOf(h); got != c.want {
			t.Errorf("asked for %q and the note carries %v", c.asked, got)
		}
	}
}

func TestABreathMarkIsDrawnAfterTheNote(t *testing.T) {
	// The whole of the point: it is a direction about the gap between two
	// notes, so it goes past the notehead and over the staff.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/breath?part=0&measure=0&at=1&breath=comma")

	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	var mark, head *engrave.Glyph
	for i, g := range lay.Glyphs {
		switch g.Name {
		case "breathMarkComma":
			mark = &lay.Glyphs[i]
		case "noteheadBlack":
			head = &lay.Glyphs[i]
		}
	}
	if mark == nil {
		t.Fatal("the breath mark was not drawn")
	}
	if head == nil {
		t.Fatal("the note was not drawn")
	}
	if mark.X <= head.X {
		t.Errorf("the mark is at x=%.2f and the note at x=%.2f: it belongs "+
			"after the note", mark.X, head.X)
	}
	if mark.Y >= head.Y {
		t.Errorf("the mark is at y=%.2f and the note at y=%.2f: it belongs "+
			"over the staff", mark.Y, head.Y)
	}
}

func TestNoneTakesTheBreathMarkOff(t *testing.T) {
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/breath?part=0&measure=0&at=1&breath=comma")
	h.post("/api/breath?part=0&measure=0&at=1&breath=none")
	if got := breathOf(h); got != model.NoBreath {
		t.Errorf("the note still carries %v", got)
	}
}

func TestASignNobodyKnowsIsRefused(t *testing.T) {
	// Rather than quietly writing a comma: a mark that is not the one that
	// was asked for is worse than an error, because nothing says so.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	if msg := h.postFails("/api/breath?part=0&measure=0&at=1&breath=sniff"); msg == "" {
		t.Fatal("a breath mark gscore does not have was accepted")
	}
	if got := breathOf(h); got != model.NoBreath {
		t.Errorf("the note carries %v after a request that failed", got)
	}
}
