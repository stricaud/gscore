/*
 * metronome_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// What the metronome ticks through each bar.
//
// The interface divides a bar's played length by this number, so the count
// is the whole of what it needs to be right: a bar counted in the wrong
// number of beats is a click that drifts off the music and stays off it.

// metroBars builds one staff in a meter, each bar holding the notes given.
func metroBars(t model.TimeSignature, fill ...[]model.Element) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: t}
	for _, els := range fill {
		p.Measures = append(p.Measures, &model.Measure{
			Voices:       []*model.Voice{{Elements: els}},
			RightBarline: model.BarSingle,
		})
	}
	return &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
}

func metroNotes(n int, d model.Duration) []model.Element {
	var out []model.Element
	for i := 0; i < n; i++ {
		out = append(out, &model.Note{
			Pitch: model.Pitch{Step: model.StepG, Octave: 4}, Dur: d,
		})
	}
	return out
}

func TestTheMetronomeCountsTheBeatsOfTheMeter(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	eighth := model.Duration{Num: 1, Den: 8}

	for _, tc := range []struct {
		name string
		time model.TimeSignature
		bar  []model.Element
		want int
	}{
		{"4/4 is four", model.TimeSignature{Beats: 4, BeatValue: 4},
			metroNotes(4, quarter), 4},
		{"3/4 is three", model.TimeSignature{Beats: 3, BeatValue: 4},
			metroNotes(3, quarter), 3},
		// Compound time is counted in threes: a jig is two beats in the bar
		// and not six, which is also how its notes are beamed and how its
		// beats are marked on the page.
		{"6/8 is two", model.TimeSignature{Beats: 6, BeatValue: 8},
			metroNotes(6, eighth), 2},
		{"9/8 is three", model.TimeSignature{Beats: 9, BeatValue: 8},
			metroNotes(9, eighth), 3},
		// 3/8 is the exception: one bar of three quavers at speed, beaten
		// in three rather than as one beat of a dotted crotchet.
		{"3/8 is three", model.TimeSignature{Beats: 3, BeatValue: 8},
			metroNotes(3, eighth), 3},
		{"7/8 is seven", model.TimeSignature{Beats: 7, BeatValue: 8},
			metroNotes(7, eighth), 7},
		{"2/2 is two", model.TimeSignature{Beats: 2, BeatValue: 2},
			metroNotes(4, quarter), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := buildPlayback(metroBars(tc.time, tc.bar))
			if len(pb.Measures) != 1 {
				t.Fatalf("%d bars in the performance, want 1", len(pb.Measures))
			}
			if got := pb.Measures[0].Beats; got != tc.want {
				t.Errorf("%s counted in %d beats, want %d", tc.time, got, tc.want)
			}
		})
	}
}

// A pickup is a bar of as many beats as it holds. Ticking a full bar through
// a one-beat upbeat is three clicks of silence before the music starts, which
// is exactly when somebody following the click loses the entry.
func TestAPickupIsCountedByWhatIsInIt(t *testing.T) {
	quarter := model.Duration{Num: 1, Den: 4}
	s := metroBars(model.TimeSignature{Beats: 4, BeatValue: 4},
		metroNotes(1, quarter), metroNotes(4, quarter))

	pb := buildPlayback(s)
	if len(pb.Measures) != 2 {
		t.Fatalf("%d bars in the performance, want 2", len(pb.Measures))
	}
	if got := pb.Measures[0].Beats; got != 1 {
		t.Errorf("the pickup was counted in %d beats, want 1", got)
	}
	if got := pb.Measures[1].Beats; got != 4 {
		t.Errorf("the full bar was counted in %d beats, want 4", got)
	}
}

// The beat the metronome ticks and the beat the page marks are the same
// beat, because both are the meter's own. They are read from one place, so
// what is heard cannot fall somewhere the page shows nothing.
func TestTheHeardBeatIsTheDrawnBeat(t *testing.T) {
	for _, time := range []model.TimeSignature{
		{Beats: 4, BeatValue: 4},
		{Beats: 3, BeatValue: 4},
		{Beats: 6, BeatValue: 8},
		{Beats: 9, BeatValue: 8},
		{Beats: 3, BeatValue: 8},
		{Beats: 7, BeatValue: 8},
		{Beats: 2, BeatValue: 2},
	} {
		unit := time.BeatUnit()
		bar := time.BarDuration()
		if unit.IsZero() || bar.IsZero() {
			t.Fatalf("%s has no beat", time)
		}
		// The count the metronome uses, against the count the beat marks
		// draw: both are the bar divided by the meter's beat.
		want := int(bar.Float()/unit.Float() + 0.5)
		if got := time.BeatsPerBar(); got != want {
			t.Errorf("%s: BeatsPerBar is %d, but the bar holds %d beats",
				time, got, want)
		}
	}
}
