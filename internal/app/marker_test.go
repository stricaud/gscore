/*
 * marker_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// "From letter B" has to mean the same bar to everybody in the room.
//
// A rehearsal mark is written on the bar, not on a staff: it goes on every
// staff of the score so that it is in the conductor's copy and in every
// player's part. A mark one player cannot find is worse than no mark.

// band is a score of several staves, each with the same number of bars.
func markerBand(t *testing.T, staves, bars int) *App {
	t.Helper()
	s := model.NewScore()
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Name: "Part", Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for j := 0; j < bars; j++ {
			p.Measures = append(p.Measures, &model.Measure{
				Voices: []*model.Voice{{Elements: []model.Element{&model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				}}}},
				RightBarline: model.BarSingle,
			})
		}
		if i == 0 {
			s.Parts[0] = p
		} else {
			s.Parts = append(s.Parts, p)
		}
	}
	s.Normalise()
	a := New(s, "")
	return a
}

// mounted is the harness wrapped round an app that already has music in it.
func mounted(t *testing.T, a *App) *harness {
	t.Helper()
	mux := http.NewServeMux()
	a.routes(mux)
	return &harness{t: t, app: a, mux: mux}
}

func TestARehearsalMarkGoesOnEveryStaff(t *testing.T) {
	a := markerBand(t, 5, 12)
	h := mounted(t, a)
	h.post("/api/marker?measure=8&text=B")

	for i, p := range a.score.Parts {
		if got := p.Measures[8].Marker; got != "B" {
			t.Errorf("staff %d has %q at bar 9, want B", i, got)
		}
	}
	// And nowhere else.
	for i, p := range a.score.Parts {
		for j, m := range p.Measures {
			if j != 8 && m.Marker != "" {
				t.Errorf("staff %d bar %d also says %q", i, j+1, m.Marker)
			}
		}
	}
}

// A part on its own carries it too, which is what makes the part worth
// printing: the player has to find letter B as fast as the conductor.
func TestAPartOnItsOwnKeepsTheMark(t *testing.T) {
	a := markerBand(t, 5, 12)
	h := mounted(t, a)
	h.post("/api/marker?measure=8&text=B")

	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Paginated
	opt.LineWidth = 60

	for i, p := range a.score.Parts {
		lay := engrave.Engrave(onePart(a.score, p), opt)
		found := false
		for _, tx := range lay.Texts {
			if tx.S == "B" {
				found = true
			}
		}
		if !found {
			t.Errorf("the part for staff %d does not say letter B anywhere", i)
		}
	}

	// And the conductor's score says it once, not once a staff: it is a
	// mark on the system, not on each line of it.
	lay := engrave.Engrave(a.score, opt)
	n := 0
	for _, tx := range lay.Texts {
		if tx.S == "B" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the score says B %d times, want once", n)
	}
}

// Asking for a mark that says the number of its bar sets both at once: the
// mark, and the way the score names the marks in its sequence.
//
// It is offered where the mark is put in and not only in the window that
// sets the style, because that is where somebody is standing when they want
// it — and the alternative they were reaching for, typing the number in by
// hand, writes a mark that stops being true the moment a bar is put in
// before it.
func TestAMarkCanBeAskedForByItsBarNumber(t *testing.T) {
	a := markerBand(t, 2, 12)
	h := mounted(t, a)
	h.post("/api/marker?measure=8&auto=1&style=bars")

	if got := a.score.Markers.String(); got != "bars" {
		t.Errorf("the marks are named by %q, want bars", got)
	}
	// The bar is in the sequence, which is what makes the name follow it
	// rather than being written down once.
	if !a.score.Parts[0].Measures[8].MarkerAuto {
		t.Fatal("the bar was not put in the sequence")
	}
	named := a.score.RehearsalMarks(engrave.BarNumbers(a.score))
	if got := named[8]; got != "9" {
		t.Errorf("the mark at bar 9 is called %q, want 9", got)
	}

	// And it goes on being the number of its bar. A mark somebody typed
	// out as "9" would still say 9 after a bar was put in front of it,
	// which is the mistake this exists to stop.
	h.post("/api/insertmeasure?measure=0")
	named = a.score.RehearsalMarks(engrave.BarNumbers(a.score))
	if got := named[9]; got != "10" {
		t.Errorf("after a bar was put in, the mark is called %q, want 10", got)
	}
}
