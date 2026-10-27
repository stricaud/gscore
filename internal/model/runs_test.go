/*
 * runs_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * What is in force bar by bar, asked all at once.
 *
 * ClefAt and TempoAt each answer for one bar and walk the score from its
 * first bar to do it. Asked about every bar in turn — which is what laying
 * a score out and what performing it both do — that walk is taken again for
 * every bar, and a long score pays for it twice over. Beethoven's ninth is
 * fifty-three staves of two and a half thousand bars: more than half of its
 * fifty-five seconds of layout went into ClefAt, and twenty-six of the
 * thirty-one seconds it took to work out how to play it went into TempoAt.
 *
 * Clefs and Tempos answer for every bar in one pass. They are only worth
 * having if they agree with the ones they replace, in every bar, including
 * the awkward ones: a change written in the middle of a bar belongs to the
 * bars after it, and a change written at the head of a bar belongs to that
 * bar.
 */

package model

import "testing"

func TestTheClefRunAgreesWithTheClefInEveryBar(t *testing.T) {
	treble, bass, alto := Treble, Bass, Alto
	p := &Part{Clef: Treble}
	for i := 0; i < 6; i++ {
		p.Measures = append(p.Measures, &Measure{Voices: []*Voice{{}}})
	}
	// A change at the head of bar 2: that bar is already in bass.
	p.Measures[2].Clef = &bass
	// And one written inside bar 4, after a quarter note, which belongs to
	// bar 5 and not to bar 4.
	n := &Note{Dur: Duration{Num: 1, Den: 4}}
	mid := &Note{Dur: Duration{Num: 1, Den: 4}}
	mid.Marks.Clef = &alto
	p.Measures[4].Voices[0].Elements = []Element{n, mid}
	_ = treble

	run := p.Clefs()
	if len(run) != len(p.Measures) {
		t.Fatalf("the run is %d bars long, want %d", len(run), len(p.Measures))
	}
	for i := range p.Measures {
		if got, want := run[i], p.ClefAt(i); got != want {
			t.Errorf("bar %d: the run says %v and the bar says %v", i, got, want)
		}
	}
	// And the cases the walk is there for, named rather than left to the
	// comparison above to catch by luck.
	if run[1] != Treble {
		t.Errorf("bar 1 is %v, want the treble it started in", run[1])
	}
	if run[2] != Bass {
		t.Errorf("bar 2 is %v, want the bass written at its head", run[2])
	}
	if run[4] != Bass {
		t.Errorf("bar 4 is %v: a clef written inside a bar belongs to the bars after it", run[4])
	}
	if run[5] != Alto {
		t.Errorf("bar 5 is %v, want the alto that bar 4 changed to", run[5])
	}
}

func TestTheTempoRunAgreesWithTheTempoInEveryBar(t *testing.T) {
	s := &Score{Tempo: Tempo{BPM: 60, Unit: Duration{Num: 1, Den: 4}}}
	// Two staves of unequal length, because the tempo may be written on
	// either and the shorter one runs out first.
	a := &Part{}
	b := &Part{}
	for i := 0; i < 5; i++ {
		a.Measures = append(a.Measures, &Measure{})
	}
	for i := 0; i < 3; i++ {
		b.Measures = append(b.Measures, &Measure{})
	}
	fast := Tempo{BPM: 144, Unit: Duration{Num: 1, Den: 4}}
	slow := Tempo{BPM: 40, Unit: Duration{Num: 1, Den: 2}}
	// Written on the second staff at bar 1 and on the first at bar 3: a
	// tempo belongs to the score, wherever in it somebody wrote it down.
	b.Measures[1].Tempo = &fast
	a.Measures[3].Tempo = &slow
	s.Parts = []*Part{a, b}

	run := s.Tempos()
	if len(run) != 5 {
		t.Fatalf("the run is %d bars long, want the 5 of the longest staff", len(run))
	}
	for i := 0; i < 5; i++ {
		if got, want := run[i], s.TempoAt(i); got != want {
			t.Errorf("bar %d: the run says %v and the score says %v", i, got, want)
		}
	}
	if run[0].BPM != 60 {
		t.Errorf("bar 0 is %v, want the score's own speed", run[0])
	}
	if run[1] != fast {
		t.Errorf("bar 1 is %v: a tempo written at a bar governs that bar", run[1])
	}
	if run[2] != fast {
		t.Errorf("bar 2 is %v, want the speed still in force", run[2])
	}
	if run[4] != slow {
		t.Errorf("bar 4 is %v, want the speed set at bar 3 — past the end of "+
			"the staff it was not written on", run[4])
	}
}
