/*
 * tempo_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"math"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// A tempo change has to reach the notes as well as the barlines. Playback
// used to work out how long a whole note lasts once, for the whole score, so
// a piece that slowed down at the bridge went on sounding at the speed it
// started — and the cursor, which follows the barlines, drifted away from the
// notes it was supposed to be pointing at from that bar onwards.

func slowingScore() *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	for i := 0; i < 6; i++ {
		m := &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Note{
					Pitch: model.Pitch{Step: model.StepG, Octave: 4},
					Dur:   model.Duration{Num: 1, Den: 1},
				},
			}}},
			RightBarline: model.BarSingle,
		}
		if i == 3 {
			// Half the speed, counted in the same beat.
			m.Tempo = &model.Tempo{BPM: 60, Unit: model.Duration{Num: 1, Den: 4}}
		}
		p.Measures = append(p.Measures, m)
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func TestPlaybackFollowsATempoChange(t *testing.T) {
	pb := buildPlayback(slowingScore())
	if len(pb.Measures) != 6 {
		t.Fatalf("got %d bars", len(pb.Measures))
	}

	// 4/4 at 120 to the quarter is two seconds a bar; at 60 it is four.
	for i, want := range []float64{2, 2, 2, 4, 4, 4} {
		if got := pb.Measures[i].Dur; math.Abs(got-want) > 0.001 {
			t.Errorf("bar %d lasts %.3fs, want %.3f", i+1, got, want)
		}
	}

	// And the notes move with the bars rather than staying on the old clock.
	if len(pb.Events) != 6 {
		t.Fatalf("got %d notes", len(pb.Events))
	}
	for i, want := range []float64{2, 2, 2, 4, 4, 4} {
		if got := pb.Events[i].Dur; math.Abs(got-want) > 0.001 {
			t.Errorf("note %d lasts %.3fs, want %.3f", i+1, got, want)
		}
	}

	// Six bars: three at two seconds and three at four.
	if want := 18.0; math.Abs(pb.Duration-want) > 0.001 {
		t.Errorf("the piece lasts %.3fs, want %.3f", pb.Duration, want)
	}
}

// The beat the number counts matters as much as the number: 6/8 goes at so
// many dotted quarters a minute and not at so many quarters.
func TestADottedBeatIsHalfAsLongAgain(t *testing.T) {
	s := slowingScore()
	s.Parts[0].Measures[3].Tempo = &model.Tempo{
		BPM: 60, Unit: model.Duration{Num: 3, Den: 8},
	}
	pb := buildPlayback(s)

	// A 4/4 bar is four quarters, which is 8/3 dotted quarters; at 60 of
	// them a minute that is 2.667 seconds.
	if got, want := pb.Measures[3].Dur, 8.0/3.0; math.Abs(got-want) > 0.001 {
		t.Errorf("the bar lasts %.3fs, want %.3f", got, want)
	}
}

// beatName is what the dialog needs to open showing the beat that is really
// in force, rather than resetting it to a quarter every time.
func TestTheBeatIsNamedBackTheWayItWasGiven(t *testing.T) {
	cases := []struct {
		d      model.Duration
		name   string
		dotted bool
	}{
		{model.Duration{Num: 1, Den: 4}, "quarter", false},
		{model.Duration{Num: 1, Den: 2}, "half", false},
		{model.Duration{Num: 1, Den: 8}, "eighth", false},
		{model.Duration{Num: 3, Den: 8}, "quarter", true},
		{model.Duration{Num: 3, Den: 4}, "half", true},
		{model.Duration{Num: 3, Den: 16}, "eighth", true},
	}
	for _, c := range cases {
		name, dotted := beatName(c.d)
		if name != c.name || dotted != c.dotted {
			t.Errorf("%v reads back as %s dotted=%v, want %s dotted=%v",
				c.d, name, dotted, c.name, c.dotted)
		}
	}
}
