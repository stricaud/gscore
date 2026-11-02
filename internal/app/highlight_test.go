/*
 * highlight_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Lighting up the note that is sounding.
 *
 * The editor finds the notehead for a sounding note by the number the
 * engraver gave it: playback numbers its events the same way the engraver
 * numbers its glyphs, and that number is the only thing tying the two
 * together. There is nothing to notice if it drifts — the editor simply
 * lights the wrong note, or none — so it is checked here, on a score of
 * several staves, which is where the two walks could disagree.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
)

// chorale is a score of three staves all playing at once, which is what a
// score is and what a single-staff test cannot catch.
func chorale(bars, staves int) *model.Score {
	s := model.NewScore()
	s.Parts = nil
	for i := 0; i < staves; i++ {
		p := &model.Part{
			Name: "Staff", Clef: model.Treble,
			Time: model.TimeSignature{Beats: 4, BeatValue: 4},
		}
		for b := 0; b < bars; b++ {
			m := &model.Measure{
				Voices:       []*model.Voice{{}},
				RightBarline: model.BarSingle,
			}
			for k := 0; k < 4; k++ {
				m.Voices[0].Elements = append(m.Voices[0].Elements,
					&model.Note{
						Pitch: model.Pitch{Step: model.Step((k + i) % 7), Octave: 4},
						Dur:   model.Duration{Num: 1, Den: 4},
					})
			}
			p.Measures = append(p.Measures, m)
		}
		s.Parts = append(s.Parts, p)
	}
	return s
}

// Every note the player sounds has a notehead drawn for it.
//
// An event whose number names no glyph is a note that plays and lights
// nothing, which is how a score of five voices came to look like a solo.
func TestEverySoundingNoteHasANoteheadDrawnForIt(t *testing.T) {
	for _, bars := range []int{4, 40} {
		s := chorale(bars, 3)
		pb := buildPlayback(s)
		opt := engrave.DefaultOptions()
		opt.Mode = engrave.Paginated
		lay := engrave.Engrave(s, opt)

		drawn := map[int]bool{}
		for ref := range lay.Refs {
			drawn[ref] = true
		}
		missing := map[int]int{}
		for _, ev := range pb.Events {
			// Grace notes are drawn without a number of their own, and say
			// so by carrying none.
			if ev.Ref == 0 {
				continue
			}
			if !drawn[ev.Ref] {
				missing[ev.Part]++
			}
		}
		if len(missing) > 0 {
			t.Errorf("%d bars: notes that sound and light nothing, by staff: %v",
				bars, missing)
		}
	}
}

// And no two staves share one.
//
// Two staves pointing at the same notehead is worse than pointing at none:
// one note lights for the whole chord, and the score looks as though only
// the top line is playing — which is exactly what it looks like when the
// two walks have gone out of step.
func TestTwoStavesNeverShareANotehead(t *testing.T) {
	s := chorale(40, 3)
	pb := buildPlayback(s)

	on := map[int]int{}
	for _, ev := range pb.Events {
		if ev.Ref == 0 {
			continue
		}
		if was, seen := on[ev.Ref]; seen && was != ev.Part {
			t.Fatalf("notehead %d is claimed by staff %d and staff %d",
				ev.Ref, was, ev.Part)
		}
		on[ev.Ref] = ev.Part
	}
}

// Every staff is heard. A score whose lower staves produced no events at
// all would light one note for the same reason, and the cause would look
// identical from the outside.
func TestEveryStaffIsPlayed(t *testing.T) {
	s := chorale(8, 3)
	pb := buildPlayback(s)

	byPart := map[int]int{}
	for _, ev := range pb.Events {
		byPart[ev.Part]++
	}
	for i := range s.Parts {
		if byPart[i] == 0 {
			t.Errorf("staff %d sounds nothing, so nothing on it can light up", i)
		}
	}
}

// The staves sound together, which is the thing being lit: at a moment in
// the middle of the piece, one note from each staff is ringing.
func TestTheStavesSoundTogether(t *testing.T) {
	s := chorale(8, 3)
	pb := buildPlayback(s)

	// Four seconds in, which at the default tempo is somewhere in the
	// middle of the second bar on every staff.
	at := 4.0
	ringing := map[int][]int{}
	for _, ev := range pb.Events {
		if at >= ev.Start && at < ev.Start+ev.Dur {
			ringing[ev.Part] = append(ringing[ev.Part], ev.Ref)
		}
	}
	if len(ringing) != len(s.Parts) {
		t.Fatalf("at %.1fs the staves ringing are %v, want all %d of them",
			at, ringing, len(s.Parts))
	}
	// And each on its own notehead, so each lights its own.
	seen := map[int]bool{}
	for part, refs := range ringing {
		for _, ref := range refs {
			if seen[ref] {
				t.Errorf("staff %d rings on notehead %d, which another staff "+
					"is already lighting", part, ref)
			}
			seen[ref] = true
		}
	}
}
