/*
 * textalign_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Words can be asked for at the left, the middle or the right of the bar,
// and changed from one to another, as the window does.
func TestWordsCanBeAlignedAlongTheBar(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(1)
	h.note(0, 0, 2, "quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=Fine&align=right")
	mk := marksOfElement(h.app.score.Parts[0].Measures[0].Voices[0].Elements[0])
	if len(mk.Texts) != 1 || mk.Texts[0].Align != model.AlignRight {
		t.Fatalf("the note carries %+v, want Fine at the right", mk.Texts)
	}
	h.post("/api/text?part=0&measure=0&at=1&text=Fine&align=center&index=0")
	if mk.Texts[0].Align != model.AlignCentre || len(mk.Texts) != 1 {
		t.Errorf("changed to the middle, the note carries %+v", mk.Texts)
	}
	h.post("/api/text?part=0&measure=0&at=1&text=Fine&index=0")
	if mk.Texts[0].Align != model.AlignNote {
		t.Errorf("asked for nothing, the words are still aligned %v", mk.Texts[0].Align)
	}
	if msg := h.postFails("/api/text?part=0&measure=0&at=1&text=Fine&align=sideways"); !strings.Contains(msg, "left, centre or right") {
		t.Errorf("an alignment that is not one was answered %q", msg)
	}
}
