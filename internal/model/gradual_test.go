/*
 * gradual_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * A speed on its way somewhere.
 *
 * "rit." over a bar is an instruction to a player and a slowing to anything
 * that plays the score. Carried as words alone, the music runs on at the old
 * speed straight through the end of a phrase, which is the one place a
 * listener is certain to notice.
 */

package model

import "testing"

// slowing is a score of n bars whose tempo starts at 120 and, from the bar
// given, falls to three quarters of that over so many bars.
func slowing(bars, from, over, to int) *Score {
	s := NewScore()
	s.Tempo = Tempo{BPM: 120, Unit: Duration{Num: 1, Den: 4}}
	p := s.Parts[0]
	p.Measures = nil
	for i := 0; i < bars; i++ {
		m := &Measure{Voices: []*Voice{{}}, RightBarline: BarSingle}
		if i == from {
			m.Tempo = &Tempo{
				BPM: 120, Unit: Duration{Num: 1, Den: 4},
				ToBPM: to, OverBars: over, Silent: true,
			}
		}
		p.Measures = append(p.Measures, m)
	}
	return s
}

func TestARitardandoIsPlayedAsOne(t *testing.T) {
	// Four bars at 120, then a slowing to 90 over two bars, then on at 90.
	got := slowing(8, 4, 2, 90).Tempos()
	want := []int{120, 120, 120, 120, 120, 105, 90, 90}
	for i := range want {
		if got[i].BPM != want[i] {
			t.Errorf("bar %d is played at %d, want %d", i+1, got[i].BPM, want[i])
		}
	}
}

func TestTheSlowingArrivesAndStaysThere(t *testing.T) {
	// A ritardando with nothing after it does not spring back: the music
	// carries on at the speed it reached, which is what a player does
	// until something says otherwise.
	got := slowing(6, 2, 1, 60).Tempos()
	if got[2].BPM != 120 {
		t.Errorf("the bar it starts on is played at %d, and a rit. starts "+
			"from the speed in force", got[2].BPM)
	}
	for i := 3; i < 6; i++ {
		if got[i].BPM != 60 {
			t.Errorf("bar %d is played at %d, want 60", i+1, got[i].BPM)
		}
	}
}

func TestAnAccelerandoGoesTheOtherWay(t *testing.T) {
	got := slowing(6, 1, 2, 160).Tempos()
	if got[2].BPM <= got[1].BPM || got[3].BPM != 160 {
		t.Errorf("the speeds came out %d, %d, %d", got[1].BPM, got[2].BPM,
			got[3].BPM)
	}
}

func TestAnOrdinaryTempoIsNotARamp(t *testing.T) {
	s := slowing(4, 1, 2, 90)
	s.Parts[0].Measures[1].Tempo.ToBPM = 0
	s.Parts[0].Measures[1].Tempo.OverBars = 0
	for i, got := range s.Tempos() {
		if got.BPM != 120 {
			t.Errorf("bar %d is played at %d, want 120", i+1, got.BPM)
		}
	}
}

func TestASlowingMakesTheMusicTakeLonger(t *testing.T) {
	// The whole point, said in seconds: the same bars, played slower,
	// take more time.
	plain := slowing(8, 4, 2, 90)
	plain.Parts[0].Measures[4].Tempo = nil
	fast, slow := 0.0, 0.0
	for _, t := range plain.Tempos() {
		fast += 60.0 / float64(t.BPM)
	}
	for _, t := range slowing(8, 4, 2, 90).Tempos() {
		slow += 60.0 / float64(t.BPM)
	}
	if !(slow > fast) {
		t.Errorf("with the ritardando the music takes %.3f and without it "+
			"%.3f", slow, fast)
	}
}
