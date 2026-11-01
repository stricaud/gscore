/*
 * multirest_test.go
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

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * Counting bars of rest is asked for in two places, and they are not the
 * same place.
 *
 * The View menu changes what is on the screen, which is a way of looking at
 * what will be printed without printing it. The print and export dialogs ask
 * again, because the page a player reads from and the page being written are
 * different pages: the editor draws every bar so that every bar can be
 * clicked in, and the part a player holds counts them so they can be
 * counted.
 */

// restingScore is eight bars with notes at either end and silence between.
func restingScore(h *harness) {
	p := h.app.score.Parts[0]
	p.Measures = nil
	for i := 0; i < 8; i++ {
		var el model.Element = &model.Rest{Dur: model.Duration{Num: 1, Den: 1}}
		if i == 0 || i == 7 {
			el = &model.Note{
				Pitch: model.Pitch{Step: model.StepG, Octave: 4},
				Dur:   model.Duration{Num: 1, Den: 1},
			}
		}
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: []model.Element{el}}},
			RightBarline: model.BarSingle,
		})
	}
}

// barlinesDrawn is how many barlines a drawing of the score has in it, which
// is the plainest measure of how many bars were drawn.
func barlinesDrawn(h *harness) int {
	h.app.mu.RLock()
	defer h.app.mu.RUnlock()
	return len(engrave.Engrave(h.app.score, h.app.engraveOpt).Barlines)
}

func TestTheViewSwitchReachesTheDrawing(t *testing.T) {
	h := newHarness(t)
	restingScore(h)

	every := barlinesDrawn(h)
	out := h.post("/api/view?multirest=1")
	if out["multirest"] != true {
		t.Fatalf("the switch answered %v", out["multirest"])
	}
	counted := barlinesDrawn(h)
	if counted >= every {
		t.Errorf("%d barlines became %d: the six silent bars were drawn anyway",
			every, counted)
	}

	// And back again, because a view switch that only goes one way is a
	// trap: the bars it hides are the bars you then cannot write in.
	h.post("/api/view?multirest=0")
	if back := barlinesDrawn(h); back != every {
		t.Errorf("%d barlines after turning it off, want the %d there were", back, every)
	}
}

// What is printed is asked for in the print dialog, not inherited from the
// screen: the two pages are for two different people.
func TestPrintingAsksForItselfAndIsNotToldByTheEditor(t *testing.T) {
	h := newHarness(t)
	restingScore(h)
	h.post("/api/view?multirest=1")

	// The editor is counting its bars of rest; the printed page is not,
	// because the dialog did not ask for it.
	plain := h.get("/print?what=tune")
	counted := h.get("/print?what=tune&multirest=1")

	if plain == counted {
		t.Fatal("asking for it on the printed page changed nothing")
	}
	if !strings.Contains(plain, "<svg") || !strings.Contains(counted, "<svg") {
		t.Fatal("the printed page has no music on it")
	}
	// Six bars of the eight are no longer drawn, so there is less ink.
	if len(counted) >= len(plain) {
		t.Errorf("the counted page is %d bytes against the drawn page's %d, "+
			"so nothing was left out", len(counted), len(plain))
	}
}
