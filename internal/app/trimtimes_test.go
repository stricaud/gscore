/*
 * trimtimes_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How precisely a performance says when a note happens.
 *
 * Times worked out by adding thirds of a beat together come out as
 * 2645.209921115705 and 0.9090909090909092, and every one of those digits
 * is a byte written and a byte read back. A symphony is a hundred and
 * thirty-two thousand notes, so the times alone were a sixth of the
 * eighteen megabytes it took to say how to play it — and in the page, where
 * the core writing them out is WebAssembly, that is part of the wait
 * between pressing Play and hearing anything.
 *
 * A microsecond is a thousandth of the shortest gap anybody hears as two
 * sounds rather than one, so rounding there costs nothing anybody can hear.
 */

package app

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestTimesAreRoundedToSomethingWorthWriting(t *testing.T) {
	events := []Event{{
		Start: 2645.209921115705,
		Dur:   0.9090909090909092,
		Gate:  0.9339999999999999,
	}}
	trimTimes(events)

	if got := events[0].Start; math.Abs(got-2645.209921115705) > 1e-6 {
		t.Errorf("start became %v, which is more than a microsecond from where it was", got)
	}
	if got := events[0].Dur; math.Abs(got-0.9090909090909092) > 1e-6 {
		t.Errorf("duration became %v", got)
	}

	b, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, long := range []string{"2645.209921115705", "0.9090909090909092"} {
		if strings.Contains(string(b), long) {
			t.Errorf("the event still writes %s out in full: %s", long, b)
		}
	}
	// And the point of it: short enough to be worth sending a hundred and
	// thirty thousand times.
	if len(b) > 140 {
		t.Errorf("an event is %d bytes: %s", len(b), b)
	}
}

func TestRoundingDoesNotMoveTheMusic(t *testing.T) {
	// A sixteenth at 120 is 125 milliseconds; the rounding has to be far
	// below anything that is music rather than arithmetic.
	events := []Event{
		{Start: 0, Dur: 0.125},
		{Start: 1.0 / 3.0, Dur: 1.0 / 3.0},
		{Start: 2.0 / 3.0, Dur: 1.0 / 3.0},
	}
	want := make([]Event, len(events))
	copy(want, events)
	trimTimes(events)

	for i := range events {
		if d := math.Abs(events[i].Start - want[i].Start); d > 1e-6 {
			t.Errorf("note %d starts %v seconds from where it did", i, d)
		}
		if d := math.Abs(events[i].Dur - want[i].Dur); d > 1e-6 {
			t.Errorf("note %d lasts %v seconds longer or shorter", i, d)
		}
	}
	// The triplet still adds up to the beat it is three of.
	total := events[1].Dur + events[2].Dur + events[0].Dur
	if math.Abs(total-(1.0/3.0+1.0/3.0+0.125)) > 1e-6 {
		t.Errorf("the bar adds up to %v", total)
	}
}
