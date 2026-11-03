/*
 * pagesetup_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The paper a score is printed on.
 *
 * Not only how it comes out of the printer. The paper decides how wide a
 * system may be, and the engraver is told that before it lays anything
 * out — so this is part of the document and is kept with it.
 */

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestAScoreStartsOnNoParticularPaper(t *testing.T) {
	h := newHarness(t)
	var out map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/pagesetup")), &out); err != nil {
		t.Fatal(err)
	}

	// Nothing chosen, so the score follows the program as its defaults
	// change rather than having today's frozen into it.
	if chosen, _ := out["chosen"].(bool); chosen {
		t.Error("a new score claims to have chosen its paper")
	}
	// And it still answers with a real sheet, which is what the dialog
	// has to show.
	if w, _ := out["width"].(float64); w < 100 {
		t.Errorf("it says the paper is %v mm wide", out["width"])
	}
	if got, _ := out["paper"].(string); got != model.DefaultPaper {
		t.Errorf("the default paper is %q, want %q", got, model.DefaultPaper)
	}
}

func TestThePaperCanBeChosen(t *testing.T) {
	h := newHarness(t)
	h.post("/api/pagesetup?paper=a3&landscape=1")

	p := h.app.score.Page
	if p.Paper != "a3" || !p.Landscape {
		t.Fatalf("the page is %+v", p)
	}
	// A3 on its side is 420 by 297, not 297 by 420.
	w, hh := p.Size()
	if w <= hh {
		t.Errorf("on its side it is %v by %v", w, hh)
	}
}

// Each margin falls back on its own default. Somebody who widened the left
// margin and said nothing about the others meant the others to stay.
func TestOneMarginChangedLeavesTheOthersAlone(t *testing.T) {
	h := newHarness(t)
	h.post("/api/pagesetup?marginLeft=30")

	left, right, top, bottom := h.app.score.Page.Margins()
	if left != 30 {
		t.Errorf("the left margin is %v, want 30", left)
	}
	for name, got := range map[string]float64{
		"right": right, "top": top, "bottom": bottom,
	} {
		if got <= 0 {
			t.Errorf("the %s margin came out as %v", name, got)
		}
	}
	if right != model.DefaultMarginRight {
		t.Errorf("the right margin moved to %v", right)
	}
}

// Going back to the default is not the same as writing today's defaults
// into the file: one follows the program and the other freezes it.
func TestGoingBackToTheDefaultChoosesNothing(t *testing.T) {
	h := newHarness(t)
	h.post("/api/pagesetup?paper=letter&marginLeft=30")
	if h.app.score.Page.IsZero() {
		t.Fatal("nothing was set")
	}
	h.post("/api/pagesetup?reset=1")
	if !h.app.score.Page.IsZero() {
		t.Errorf("after a reset the page is %+v", h.app.score.Page)
	}
}

func TestAPaperNobodyMakesIsRefused(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/pagesetup?paper=foolscap"); !strings.Contains(got, "no paper") {
		t.Errorf("it said %q", got)
	}
	if !h.app.score.Page.IsZero() {
		t.Error("it changed the page anyway")
	}
}

func TestMarginsThatLeaveNoRoomAreRefused(t *testing.T) {
	h := newHarness(t)
	got := h.postFails("/api/pagesetup?paper=a4&marginLeft=100&marginRight=100")
	if !strings.Contains(got, "no room") {
		t.Errorf("it said %q", got)
	}
	if !h.app.score.Page.IsZero() {
		t.Error("it set them anyway")
	}
}

func TestACustomPageHasToSayWhatSizeItIs(t *testing.T) {
	h := newHarness(t)
	if got := h.postFails("/api/pagesetup?paper=custom"); !strings.Contains(got, "width") {
		t.Errorf("it said %q", got)
	}
	h.post("/api/pagesetup?paper=custom&width=300&height=400")
	if w, hh := h.app.score.Page.Size(); w != 300 || hh != 400 {
		t.Errorf("the custom page is %v by %v", w, hh)
	}
}

func TestANonsenseMeasurementIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{
		"marginTop=-5", "marginTop=400", "marginTop=wide",
		"space=1", "space=99", "width=10&height=10&paper=custom",
	} {
		if got := h.postFails("/api/pagesetup?" + q); got == "" {
			t.Errorf("%q was allowed", q)
		}
	}
}

// The staff size is in points, because that is how an engraving
// measurement is quoted everywhere else here, and zero means fit.
func TestTheStaffSizeCanBeSetOrLeftToFit(t *testing.T) {
	h := newHarness(t)
	h.post("/api/pagesetup?space=5.5")
	if got := h.app.score.Page.StaffSpace; got != 5.5 {
		t.Errorf("the staff size is %v, want 5.5", got)
	}
	h.post("/api/pagesetup?space=0")
	if got := h.app.score.Page.StaffSpace; got != 0 {
		t.Errorf("asked to fit, the staff size is %v, want 0", got)
	}
}

// The paper is what a system has to fit into, so the engraver has to be
// told. A wider sheet is a wider line, and that is the whole reason this
// belongs to the score rather than to the export.
func TestWiderPaperGivesAWiderSystem(t *testing.T) {
	narrow := paperFor(model.Page{Paper: "a5"})
	wide := paperFor(model.Page{Paper: "a3"})
	sideways := paperFor(model.Page{Paper: "a4", Landscape: true})
	upright := paperFor(model.Page{Paper: "a4"})

	if !(narrow.PageWidth < upright.PageWidth && upright.PageWidth < wide.PageWidth) {
		t.Errorf("A5 %v, A4 %v, A3 %v are not in order",
			narrow.PageWidth, upright.PageWidth, wide.PageWidth)
	}
	if sideways.PageWidth <= upright.PageWidth {
		t.Errorf("A4 on its side is %v wide, upright is %v",
			sideways.PageWidth, upright.PageWidth)
	}
}

// Millimetres in the model, points at the renderer. One conversion, at the
// edge, or the two have to know about each other's units.
func TestTheMarginsReachTheRendererInPoints(t *testing.T) {
	o := paperFor(model.Page{Paper: "a4", MarginLeft: 25.4})
	if got := o.MarginLeft; got < 71.9 || got > 72.1 {
		t.Errorf("an inch of margin came to %v points, want 72", got)
	}
	// A4 is 210 mm, which is 595.28 points.
	if got := o.PageWidth; got < 595 || got > 596 {
		t.Errorf("A4 is %v points wide, want about 595.28", got)
	}
}

// A score that has chosen nothing gets the renderer's own defaults
// untouched, so nothing moves on a file written before any of this.
func TestAScoreThatChoseNothingIsPrintedAsItAlwaysWas(t *testing.T) {
	if paperFor(model.Page{}) != defaultPaperOptions() {
		t.Error("the default page is no longer the renderer's default")
	}
}
