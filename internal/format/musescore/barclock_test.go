/*
 * barclock_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Where the reader thinks it has got to.
 *
 * MuseScore writes <tick> as an absolute position from the start of the
 * score, and the reader subtracts its own running count to find where in the
 * bar that is. A rewind to somewhere earlier opens the bar's next voice. So
 * the reader's count has to agree with the file's: if it falls behind, a
 * rewind reads as a step forward and the voice that should stand under the
 * one already there is laid end to end after it instead.
 *
 * The Moonlight sonata came out with three whole notes too many in its left
 * hand for exactly that reason, and the Presto's sustained bass was played
 * on its own after the running eighths rather than underneath them.
 */

package musescore

import (
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// MuseScore 4 writes a whole-bar rest's real length as text and MuseScore 2
// wrote it as a pair of attributes. Read only as text, the bar of a quarter
// that carries the Trio's upbeat came out four quarters long, and from there
// the reader was a bar and a half ahead of the file.
func TestAWholeBarRestIsAsLongAsTheFileSays(t *testing.T) {
	s := read(t, oldScore(`<Measure len="1/4"><Rest>`+
		`<durationType>measure</durationType>`+
		`<duration z="1" n="4"></duration></Rest></Measure>`+
		`<Measure>`+aQuarter(60)+`</Measure>`))

	els := s.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("%d things in the bar, want the one rest", len(els))
	}
	want := model.Duration{Num: 1, Den: 4}
	if got := els[0].Duration(); got.Cmp(want) != 0 {
		t.Errorf("the bar's rest is %v long, and the bar is %v", got, want)
	}
}

// The newer spelling still works.
func TestAWholeBarRestWrittenAsTextIsReadToo(t *testing.T) {
	s := read(t, oldScore(`<Measure><Rest>`+
		`<durationType>measure</durationType>`+
		`<duration>3/4</duration></Rest></Measure>`))

	want := model.Duration{Num: 3, Den: 4}
	got := s.Parts[0].Measures[0].Voices[0].Elements[0].Duration()
	if got.Cmp(want) != 0 {
		t.Errorf("the bar's rest is %v long, want %v", got, want)
	}
}

// And a rewind is a rewind however far back it reaches.
//
// The reader's count can still run ahead of the file's — a two-note tremolo
// is written as two notes and played in the time of one, and the reader
// counts both. A tick landing before the bar even began is such a rewind,
// and reading it as "no move at all" is what loses the voice.
func TestARewindFromBeforeTheBarStillOpensAVoice(t *testing.T) {
	s := read(t, oldScore(
		`<Measure>`+aQuarter(60)+aQuarter(62)+aQuarter(64)+aQuarter(65)+`</Measure>`+
			`<Measure>`+aQuarter(48)+aQuarter(50)+aQuarter(52)+aQuarter(53)+
			`<tick>0</tick>`+aQuarter(36)+`</Measure>`))

	m := s.Parts[0].Measures[1]
	if len(m.Voices) != 2 {
		t.Fatalf("the bar holds %d voices, want the second one the rewind opens",
			len(m.Voices))
	}
	if n := len(m.Voices[0].Elements); n != 4 {
		t.Errorf("the first voice holds %d notes, want the four written in it", n)
	}
	if n := len(m.Voices[1].Elements); n != 1 {
		t.Errorf("the second voice holds %d notes, want the one after the rewind", n)
	}
}
