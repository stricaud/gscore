/*
 * repeatbar_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The bar written as 𝄎 — play the one before again.
 *
 * MuseScore writes the sign and nothing else, so the music has to be
 * fetched from the bar before. Left unread the bar came out silent: fifty
 * of the ninety bars of one big band's drum part.
 *
 * Worse than silent. The reader counts MuseScore's clock itself, because
 * <tick> is an absolute position and the bar's own start has to be taken
 * off it — and a bar that read as nothing advanced that clock by nothing.
 * Six of these in a row left every tick after them reading six bars early,
 * so a rewind that should have opened the bar's second voice looked like a
 * gap far ahead in the first: the bass drum was laid end to end after the
 * cymbals behind five and a quarter whole notes of hidden rest, and the bar
 * came out holding seven whole notes where the meter allows one. Playback
 * takes a bar's length from the longest part, so the playhead sat in that
 * one bar for seventeen seconds while the band played on and whatever was
 * heard no longer matched where it stood.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// anEighth is one eighth note at the given pitch.
func anEighth(pitch int) string {
	return `<Chord><durationType>eighth</durationType>` +
		`<Note><pitch>` + itoa(pitch) + `</pitch><tpc>14</tpc></Note></Chord>`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestARepeatedBarIsTheBarBefore(t *testing.T) {
	s := read(t, oldScore(
		`<Measure>`+aQuarter(60)+aQuarter(62)+aQuarter(64)+aQuarter(65)+`</Measure>`+
			`<Measure><RepeatMeasure/></Measure>`))

	bars := s.Parts[0].Measures
	if len(bars) != 2 {
		t.Fatalf("%d bars, want 2", len(bars))
	}
	first := bars[0].Voices[0].Elements
	second := bars[1].Voices[0].Elements
	if len(second) != len(first) {
		t.Fatalf("the repeated bar holds %d things and the bar it repeats holds %d",
			len(second), len(first))
	}
	for i := range first {
		a, _ := first[i].(*model.Note)
		b, _ := second[i].(*model.Note)
		if a == nil || b == nil {
			t.Fatalf("element %d is a %T and a %T", i, first[i], second[i])
		}
		if a.Pitch != b.Pitch {
			t.Errorf("note %d is %v in the bar and %v in its repeat", i+1, a.Pitch, b.Pitch)
		}
		if a == b {
			t.Errorf("note %d is the same note in both bars, not a copy", i+1)
		}
	}
}

// The clock is the half of it that does the damage, and it has to hold even
// for a bar this reader cannot make sense of at all.
func TestAnUnreadableBarStillTakesUpItsBarOfTheClock(t *testing.T) {
	// Two bars the reader understands with one it does not between them,
	// and then a second voice opened by rewinding to the third bar's start.
	// Tick 3840 is two whole notes in: the start of bar three.
	s := read(t, oldScore(
		`<Measure>`+aQuarter(60)+aQuarter(60)+aQuarter(60)+aQuarter(60)+`</Measure>`+
			`<Measure><SomethingThisReaderHasNeverHeardOf/></Measure>`+
			`<Measure>`+aQuarter(72)+aQuarter(72)+aQuarter(72)+aQuarter(72)+
			`<tick>3840</tick>`+aQuarter(48)+`</Measure>`))

	bars := s.Parts[0].Measures
	if len(bars) != 3 {
		t.Fatalf("%d bars, want 3", len(bars))
	}
	m := bars[2]
	if len(m.Voices) != 2 {
		t.Fatalf("bar three has %d voice(s), want 2 — the rewind did not open "+
			"one, so the clock had fallen behind", len(m.Voices))
	}
	whole := model.Duration{Num: 1, Den: 1}
	for vi, v := range m.Voices {
		if got := v.Duration(); got.Cmp(whole) > 0 {
			t.Errorf("voice %d of bar three holds %v, and the meter allows %v",
				vi+1, got, whole)
		}
	}
}

// Both together, which is the shape the drum part is actually in.
func TestVoicesStillLineUpAfterARunOfRepeatedBars(t *testing.T) {
	bar := `<Measure>` + anEighth(60) + anEighth(60) + aQuarter(60) + aQuarter(60) + aQuarter(60) +
		`<tick>960</tick>` + aQuarter(36) + `</Measure>`
	repeats := ""
	for i := 0; i < 6; i++ {
		repeats += `<Measure><RepeatMeasure/></Measure>`
	}
	// Bar eight: seven whole notes have gone by, so its own start is 13440
	// and a rewind to its second beat is 13920.
	last := `<Measure>` + aQuarter(60) + aQuarter(60) + aQuarter(60) + aQuarter(60) +
		`<tick>13920</tick>` + aQuarter(36) + `</Measure>`
	s := read(t, oldScore(bar+repeats+last))

	bars := s.Parts[0].Measures
	if len(bars) != 8 {
		t.Fatalf("%d bars, want 8", len(bars))
	}
	// Every bar of the run carries the music, and none of them is overlong.
	whole := model.Duration{Num: 1, Den: 1}
	for i, m := range bars {
		held := 0
		for vi, v := range m.Voices {
			if v == nil {
				continue
			}
			held += len(v.Elements)
			if got := v.Duration(); got.Cmp(whole) > 0 {
				t.Errorf("bar %d voice %d holds %v, and the meter allows %v",
					i+1, vi+1, got, whole)
			}
		}
		if held == 0 {
			t.Errorf("bar %d came out empty", i+1)
		}
	}
	if n := len(bars[7].Voices); n != 2 {
		t.Errorf("the bar after the run has %d voice(s), want 2 — the clock "+
			"lost the repeated bars", n)
	}
}
