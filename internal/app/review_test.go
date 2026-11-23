package app

import (
	"encoding/json"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// Marking the bars that were picked up, on two staves, and taking a mark
// off again; and the marks reach the editor with the bars' positions, which
// is what it tints them from.
func TestMarkingBarsWhileCheckingAScore(t *testing.T) {
	h := newHarness(t)
	h.bars(3)
	h.post("/api/addstaff?instrument=piano")
	ms := func(p int) []*model.Measure { return h.app.score.Parts[p].Measures }

	h.post("/api/review?review=good&bars=0:0,0:1,1:1")
	if ms(0)[0].Review != model.ReviewGood || ms(0)[1].Review != model.ReviewGood ||
		ms(1)[1].Review != model.ReviewGood {
		t.Fatal("the bars picked up are not all marked good")
	}
	if ms(1)[0].Review != "" || ms(0)[2].Review != "" {
		t.Error("a bar that was not picked up was marked")
	}

	h.post("/api/review?review=fix&bars=0:1")
	h.post("/api/review?review=none&bars=0:0")
	if ms(0)[0].Review != "" || ms(0)[1].Review != model.ReviewFix {
		t.Errorf("marks are %q and %q, want none and fix",
			ms(0)[0].Review, ms(0)[1].Review)
	}

	var d struct {
		Reviews []reviewMark `json:"reviews"`
	}
	if err := json.Unmarshal([]byte(h.get("/api/hits")), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Reviews) != 2 {
		t.Fatalf("%d marked bars reported, want 2: %+v", len(d.Reviews), d.Reviews)
	}

	if msg := h.postFails("/api/review?review=maybe&bars=0:0"); msg == "" {
		t.Error("a mark that is not good, check or fix was accepted")
	}
	if msg := h.postFails("/api/review?review=good"); msg == "" {
		t.Error("a mark with no bar was accepted")
	}
}

// Colours are checked before they are kept, and every mark always has one.
func TestReviewColoursAreCheckedAndComplete(t *testing.T) {
	got := cleanReviewColours(map[string]string{
		"good": "#00ff00", "check": "not a colour", "other": "#123456",
	})
	if len(got) != 1 || got["good"] != "#00ff00" {
		t.Errorf("cleaned to %v, want good alone", got)
	}
	all := ReviewColourMap()
	for _, k := range []string{model.ReviewGood, model.ReviewCheck, model.ReviewFix} {
		if all[k] == "" {
			t.Errorf("no colour for %q", k)
		}
	}
}

// When the checking is done, every mark comes off at once, and Undo puts
// them back.
func TestErasingEveryReviewMark(t *testing.T) {
	h := newHarness(t)
	h.bars(3)
	h.post("/api/review?review=good&bars=0:0,0:1")
	h.post("/api/review?review=fix&bars=0:2")
	h.post("/api/review?review=none&all=1")
	for i, m := range h.app.score.Parts[0].Measures {
		if m.Review != "" {
			t.Errorf("bar %d is still marked %q", i+1, m.Review)
		}
	}
	h.post("/api/undo")
	if r := h.app.score.Parts[0].Measures[2].Review; r != model.ReviewFix {
		t.Errorf("after Undo bar 3 is marked %q, want fix", r)
	}
	if msg := h.postFails("/api/review?review=good&all=1"); msg == "" {
		t.Error("marking every bar at once was accepted")
	}
}
