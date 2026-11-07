/*
 * textstyle_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The kinds of writing on a page.
 *
 * Words on a score are not one thing with a font chosen for each. The kind
 * decides the type, the slant, which side of the staff it goes on and
 * whether it is for one player or for everybody — and those are
 * conventions a reader relies on without noticing.
 */

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// textsOf is every text a part's first written-on note carries.
func textsOf(t *testing.T, h *harness, part int) []model.TextMark {
	t.Helper()
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if mk := marksOfElement(el); mk != nil && len(mk.Texts) > 0 {
					return mk.Texts
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return nil
}

func firstText(t *testing.T, h *harness, part int) model.TextMark {
	t.Helper()
	for _, m := range h.app.score.Parts[part].Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if mk := marksOfElement(el); mk != nil && len(mk.Texts) > 0 {
					return mk.Texts[0]
				}
			}
		}
	}
	t.Fatal("nothing was written")
	return model.TextMark{}
}

func TestTextIsWrittenInTheKindItWasAskedFor(t *testing.T) {
	for _, name := range []string{
		"expression", "technique", "tempo", "direction", "plain",
	} {
		h := newHarness(t)
		h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
		h.post("/api/text?part=0&measure=0&at=1&text=hello&style=" + name)

		want, _ := model.TextStyleFromName(name)
		if got := firstText(t, h, 0).Style; got != want {
			t.Errorf("%q came back as %v", name, got)
		}
	}
}

// The kind decides which side of the staff it goes on, and the mark says
// nothing unless somebody overrode it. A placement written into every
// mark is a placement that cannot follow a change of style.
func TestTheKindDecidesWhereItGoesUnlessToldOtherwise(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=dolce&style=expression")

	mark := firstText(t, h, 0)
	if mark.Placement != model.PlacementAuto {
		t.Errorf("it was pinned to %v rather than left to its kind", mark.Placement)
	}
	// And an expression marking goes under the staff, where the dynamics
	// are, because it is the same kind of instruction as a dynamic.
	if got := mark.Look().Place; got != model.PlacementBelow {
		t.Errorf("an expression marking goes %v", got)
	}

	// Said outright, it is honoured.
	h2 := newHarness(t)
	h2.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h2.post("/api/text?part=0&measure=0&at=1&text=dolce&style=expression&placement=above")
	if got := firstText(t, h2, 0).Look().Place; got != model.PlacementAbove {
		t.Errorf("asked for above, it goes %v", got)
	}
}

// Italic and bold add to the style and cannot take away from it: the
// alternative needs a third state for "not set", and a mark that can
// countermand its own style in both directions is worth less than one
// that says plainly what it wants.
func TestBoldAndItalicAddToTheKind(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=molto&style=technique&italic=1")

	look := firstText(t, h, 0).Look()
	if !look.Italic {
		t.Error("the slant asked for was not applied")
	}
	if look.Bold {
		t.Error("it came out bold as well")
	}
	// A tempo marking is bold already, and asking for bold changes nothing.
	h2 := newHarness(t)
	h2.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h2.post("/api/text?part=0&measure=0&at=1&text=Allegro&style=tempo")
	if !firstText(t, h2, 0).Look().Bold {
		t.Error("a tempo marking is not bold")
	}
}

// Words for everybody are not the same as words for one player, and the
// difference is part of the kind rather than a switch beside it.
func TestTheKindSaysWhoTheWordsAreFor(t *testing.T) {
	for _, tc := range []struct {
		style  string
		system bool
	}{
		{"tempo", true}, {"direction", true},
		{"expression", false}, {"technique", false}, {"plain", false},
	} {
		s, _ := model.TextStyleFromName(tc.style)
		if got := s.Look().System; got != tc.system {
			t.Errorf("%s is for everybody: %v, want %v", tc.style, got, tc.system)
		}
	}
}

func TestAKindNobodyHasIsRefused(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	got := h.postFails("/api/text?part=0&measure=0&at=1&text=hi&style=shouty")
	if !strings.Contains(got, "no text style") {
		t.Errorf("it said %q", got)
	}
}

// The list is served so that whatever is choosing from it does not have to
// write the same answers down again.
func TestTheKindsAreListedWithHowEachIsSet(t *testing.T) {
	h := newHarness(t)
	var out []struct {
		Name   string  `json:"name"`
		Label  string  `json:"label"`
		Size   float64 `json:"size"`
		Italic bool    `json:"italic"`
		Bold   bool    `json:"bold"`
		Place  string  `json:"place"`
		System bool    `json:"system"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/textstyles")), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != len(model.TextStyles()) {
		t.Fatalf("%d kinds were listed, want %d", len(out), len(model.TextStyles()))
	}
	for _, s := range out {
		if s.Label == "" {
			t.Errorf("%q has nothing to call it in a menu", s.Name)
		}
		if s.Size <= 0 {
			t.Errorf("%q is set at %v spaces", s.Name, s.Size)
		}
		if _, ok := model.TextStyleFromName(s.Name); !ok {
			t.Errorf("%q is listed and is not a kind", s.Name)
		}
	}
	// And the one that is bold is the tempo, which is how a reader tells
	// a change of speed from everything else over the system.
	for _, s := range out {
		if s.Name == "tempo" && !s.Bold {
			t.Error("the tempo marking is not bold")
		}
		if s.Name == "expression" && !s.Italic {
			t.Error("an expression marking is not italic")
		}
	}
}

/*
 * The metronome marking.
 */

func TestTheMetronomeMarkingCanBeHidden(t *testing.T) {
	h := newHarness(t)
	if !h.app.engraveOpt.ShowTempoMark {
		t.Fatal("it does not start out shown")
	}
	out := h.post("/api/view?tempomark=0")
	if on, _ := out["tempomark"].(bool); on {
		t.Error("it says the mark is still shown")
	}
	if h.app.engraveOpt.ShowTempoMark {
		t.Error("the drawing still shows it")
	}
	h.post("/api/view?tempomark=1")
	if !h.app.engraveOpt.ShowTempoMark {
		t.Error("it would not come back")
	}
}

// Hiding the mark is a decision about the drawing. The piece still plays
// at that speed, which is the thing that would be alarming if it were not
// true.
func TestHidingTheMarkingDoesNotChangeTheSpeed(t *testing.T) {
	h := newHarness(t)
	h.post("/api/tempo?bpm=92")
	was := h.app.score.Tempo.BPM
	h.post("/api/view?tempomark=0")
	if got := h.app.score.Tempo.BPM; got != was {
		t.Errorf("the speed went from %d to %d", was, got)
	}
}

// Anything not named is left as it was, which is what lets the menu send
// one switch at a time.
func TestTheViewLeavesAloneWhatItIsNotAsked(t *testing.T) {
	h := newHarness(t)
	h.post("/api/view?names=1")
	h.post("/api/view?tempomark=0")
	if !h.app.engraveOpt.ShowPartNames {
		t.Error("asking about the metronome mark turned the names off")
	}
}

// Words that were picked up by clicking them are changed, not added to.
//
// A note may carry two — "pizz." over the staff and "dolce" under it — and
// changing the one somebody pointed at must leave the other alone. Without
// a way to say which, writing again appended a third, and the only way back
// was to take all of them off.
func TestChangingOneOfANotesTextsLeavesTheOther(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=pizz.&style=technique")
	h.post("/api/text?part=0&measure=0&at=1&text=dolce&style=expression")

	texts := textsOf(t, h, 0)
	if len(texts) != 2 {
		t.Fatalf("the note carries %d texts, want 2", len(texts))
	}

	h.post("/api/text?part=0&measure=0&at=1&text=arco&style=technique&index=0")
	texts = textsOf(t, h, 0)
	if len(texts) != 2 {
		t.Fatalf("after changing one the note carries %d texts, want 2", len(texts))
	}
	if texts[0].Text != "arco" {
		t.Errorf("the first says %q, want arco", texts[0].Text)
	}
	if texts[1].Text != "dolce" {
		t.Errorf("the second says %q, want dolce", texts[1].Text)
	}
}

// And removing the one that was pointed at removes that one.
func TestRemovingOneOfANotesTextsLeavesTheOther(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=pizz.&style=technique")
	h.post("/api/text?part=0&measure=0&at=1&text=dolce&style=expression")

	h.post("/api/text?part=0&measure=0&at=1&text=&index=0")
	texts := textsOf(t, h, 0)
	if len(texts) != 1 {
		t.Fatalf("the note carries %d texts, want 1", len(texts))
	}
	if texts[0].Text != "dolce" {
		t.Errorf("what is left says %q, want dolce", texts[0].Text)
	}
}

// Changing which side of the staff it goes on keeps the words and the kind.
func TestWordsCanBeMovedToTheOtherSideOfTheStaff(t *testing.T) {
	h := newHarness(t)
	h.post("/api/insert?part=0&measure=0&at=0&step=4&dur=quarter")
	h.post("/api/text?part=0&measure=0&at=1&text=pizz.&style=technique")
	if got := firstText(t, h, 0).Look().Place; got != model.PlacementAbove {
		t.Fatalf("a technique starts out %v", got)
	}

	h.post("/api/text?part=0&measure=0&at=1&text=pizz.&style=technique" +
		"&placement=below&index=0")
	mark := firstText(t, h, 0)
	if got := mark.Look().Place; got != model.PlacementBelow {
		t.Errorf("after asking for below it goes %v", got)
	}
	if mark.Text != "pizz." || mark.Style != model.TextTechnique {
		t.Errorf("the words came back as %q in %v", mark.Text, mark.Style)
	}
}
