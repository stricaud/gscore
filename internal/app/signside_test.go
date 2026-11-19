/*
 * signside_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Moving a sign to the other end of its bar, as the double click does.
 */

package app

import (
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestASignCanBeMovedToTheEndOfItsBarAndBack(t *testing.T) {
	h := newHarness(t)
	h.post("/api/new")
	h.bars(3)
	h.post("/api/jump?measure=1&sign=tocoda")
	m := h.app.score.Parts[0].Measures[1]
	if m.Sign != model.ToCoda || m.SignAtEnd {
		t.Fatalf("a new sign is %v at the end %v, want To Coda at the head", m.Sign, m.SignAtEnd)
	}
	// The side on its own, which is what the dialog sends.
	h.post("/api/jump?measure=1&side=right")
	if !m.SignAtEnd {
		t.Error("asked for the right, the sign stayed at the head")
	}
	h.post("/api/jump?measure=1&side=left")
	if m.SignAtEnd {
		t.Error("asked for the left, the sign stayed at the end")
	}
	// Or with the sign, in one step.
	h.post("/api/jump?measure=2&sign=fine&side=right")
	if m2 := h.app.score.Parts[0].Measures[2]; m2.Sign != model.Fine || !m2.SignAtEnd {
		t.Errorf("Fine at the end came out as %v, at the end %v", m2.Sign, m2.SignAtEnd)
	}
	if msg := h.postFails("/api/jump?measure=1&side=middle"); !strings.Contains(msg, "left or right") {
		t.Errorf("a side that is neither was answered %q", msg)
	}
}
