/*
 * symbolscale_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * A symbol is resized where it stands.
 *
 * Per placement and not per symbol, which is the whole point: one drawing
 * is used in many places, and the same drawing wants to be small over a
 * note and large at the head of a movement. Kept on the symbol it would be
 * one size everywhere, and a second copy of the drawing at another size is
 * a second thing to keep in step.
 */

func symbolOn(h *harness, at int) *model.SymbolMark {
	h.t.Helper()
	mk := marksOfElement(h.elements(0, 0)[at])
	if mk == nil || len(mk.Symbols) == 0 {
		h.t.Fatalf("there is no symbol on element %d", at)
	}
	return &mk.Symbols[0]
}

func withSymbol(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/insertsymbol?part=0&measure=0&at=1&name=mine&attach=1")
	return h
}

func TestASymbolIsResizedWhereItStands(t *testing.T) {
	h := withSymbol(t)
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=mine&scale=2")
	if got := symbolOn(h, 0).ScaleOr1(); got != 2 {
		t.Errorf("the symbol is at %v, want twice its size", got)
	}
}

func TestAStepIsAStepFromWhereItIs(t *testing.T) {
	// By rather than to, so the same press does the same thing at any
	// size: a tenth larger is a tenth larger whatever it is already at.
	h := withSymbol(t)
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=mine&scale=2")
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=mine&by=1.5")
	if got := symbolOn(h, 0).ScaleOr1(); got != 3 {
		t.Errorf("the symbol is at %v, want three times its size", got)
	}
}

func TestASymbolNobodyResizedSaysNothingAboutItsSize(t *testing.T) {
	// Zero reads as one, so every score written before symbols could be
	// resized comes out exactly as it did.
	h := withSymbol(t)
	s := symbolOn(h, 0)
	if s.Scale != 0 {
		t.Errorf("a fresh symbol carries a scale of %v", s.Scale)
	}
	if s.ScaleOr1() != 1 {
		t.Errorf("it is drawn at %v", s.ScaleOr1())
	}
}

func TestASizeNobodyMeantIsRefused(t *testing.T) {
	h := withSymbol(t)
	for _, q := range []string{"scale=0.001", "scale=100"} {
		msg := h.fails("/api/symbolscale?part=0&measure=0&at=1&name=mine&" + q)
		if !strings.Contains(msg, "tenth") {
			t.Errorf("%s refused with %q", q, msg)
		}
	}
	if got := symbolOn(h, 0).ScaleOr1(); got != 1 {
		t.Errorf("the symbol was resized anyway, to %v", got)
	}
}

func TestResizingSaysWhatToDoWhenNothingIsAsked(t *testing.T) {
	h := withSymbol(t)
	msg := h.fails("/api/symbolscale?part=0&measure=0&at=1&name=mine")
	if !strings.Contains(msg, "scale it") {
		t.Errorf("refused with %q", msg)
	}
}

func TestTheScaleReachesTheDrawing(t *testing.T) {
	// The end of it: a number in the model that no renderer reads is a
	// setting that does nothing.
	h := withSymbol(t)
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=mine&scale=2.5")
	lay := engrave.Engrave(h.app.score, engrave.DefaultOptions())
	found := false
	for _, g := range lay.Glyphs {
		if g.Name != "mine" {
			continue
		}
		found = true
		if g.ScaleOr1() != 2.5 {
			t.Errorf("the glyph is drawn at %v, want 2.5", g.ScaleOr1())
		}
	}
	if !found {
		t.Error("the symbol was not drawn at all")
	}
}

func TestOneSymbolResizedLeavesTheOthersAlone(t *testing.T) {
	// The whole reason the size is on the placement: the same drawing
	// standing somewhere else must not change.
	h := withSymbol(t)
	h.note(0, 0, 4, "quarter")
	h.post("/api/insertsymbol?part=0&measure=0&at=2&name=mine&attach=1")
	h.post("/api/symbolscale?part=0&measure=0&at=1&name=mine&scale=3")
	if got := symbolOn(h, 0).ScaleOr1(); got != 3 {
		t.Errorf("the first is at %v", got)
	}
	if got := symbolOn(h, 1).ScaleOr1(); got != 1 {
		t.Errorf("the second changed too, to %v", got)
	}
}

func TestASymbolStandingOnItsOwnIsResizedToo(t *testing.T) {
	// The other kind: a sign put into the bar rather than onto a note. It
	// carries no Marks for markAt to find, which is why the handler looks
	// for it first.
	h := newHarness(t)
	h.note(0, 0, 2, "quarter")
	h.post("/api/insertsymbol?part=0&measure=0&at=1&name=mine")

	var at int
	var lone *model.SymbolElement
	for i, el := range h.elements(0, 0) {
		if se, ok := el.(*model.SymbolElement); ok {
			at, lone = i, se
		}
	}
	if lone == nil {
		t.Fatal("no symbol was put into the bar")
	}
	h.post(fmt.Sprintf(
		"/api/symbolscale?part=0&measure=0&at=%d&name=mine&scale=1.75", at+1))
	if got := lone.Symbol.ScaleOr1(); got != 1.75 {
		t.Errorf("the lone symbol is at %v, want 1.75", got)
	}
}
