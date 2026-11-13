/*
 * wholebarrest_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os"
	"path/filepath"
	"testing"

	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * A bar of silence is as long as its bar.
 *
 * A whole-bar rest means "this bar is silent", so its length is whatever
 * the bar is. It is written down as a duration all the same — it has to
 * be, so that it plays and can be picked up like anything else — and that
 * written duration is what goes stale when the meter changes under it.
 *
 * Left stale, a six-four dotted whole rest sat in a bar of four-four: the
 * bar was half a bar overfull, its barline was pushed out, and the rest
 * printed with an augmentation dot nobody had asked for. Reported as "why
 * do I have a dot on my rest and a line in the first measure".
 */

func barsOfSilence(t *testing.T, beats, value, n int) *harness {
	t.Helper()
	h := newHarness(t)
	p := h.app.score.Parts[0]
	p.Time = model.TimeSignature{Beats: beats, BeatValue: value}
	bar := p.TimeAt(0).BarDuration()
	p.Measures = nil
	for i := 0; i < n; i++ {
		p.Measures = append(p.Measures, &model.Measure{
			Voices: []*model.Voice{{Elements: []model.Element{
				&model.Rest{Dur: bar},
			}}},
			RightBarline: model.BarSingle,
		})
	}
	return h
}

func restOf(t *testing.T, h *harness, at int) *model.Rest {
	t.Helper()
	els := h.app.score.Parts[0].Measures[at].Voices[0].Elements
	if len(els) != 1 {
		t.Fatalf("bar %d holds %d elements", at, len(els))
	}
	r, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("bar %d holds a %T", at, els[0])
	}
	return r
}

func TestChangingTheMeterReMeasuresTheBarsOfSilence(t *testing.T) {
	h := barsOfSilence(t, 6, 4, 4) // a dotted whole rest a bar
	if got := restOf(t, h, 0).Dur; got != (model.Duration{Num: 3, Den: 2}) {
		t.Fatalf("the setup wrote %v, not a six-four bar", got)
	}
	h.post("/api/staff?part=0&time=4/4")
	for i := 0; i < 4; i++ {
		if got := restOf(t, h, i).Dur; got != (model.Duration{Num: 1, Den: 1}) {
			t.Errorf("bar %d is silent for %v, want a whole bar of four-four",
				i+1, got)
		}
	}
}

func TestAGoingToALongerMeterLengthensThemToo(t *testing.T) {
	h := barsOfSilence(t, 2, 4, 2)
	h.post("/api/staff?part=0&time=6/8")
	if got := restOf(t, h, 0).Dur; got != (model.Duration{Num: 3, Den: 4}) {
		t.Errorf("the bar is silent for %v, want three quarters", got)
	}
}

func TestABarWithMusicInItIsLeftAlone(t *testing.T) {
	// Shortening a meter under real notes is a question about the music
	// that no rule here can answer, so nothing here answers it.
	h := barsOfSilence(t, 6, 4, 2)
	q := model.Duration{Num: 1, Den: 4}
	h.app.score.Parts[0].Measures[1].Voices[0].Elements = []model.Element{
		&model.Note{Pitch: model.Pitch{Step: model.StepB, Octave: 4}, Dur: q},
		&model.Rest{Dur: q},
	}
	h.post("/api/staff?part=0&time=4/4")

	if got := restOf(t, h, 0).Dur; got != (model.Duration{Num: 1, Den: 1}) {
		t.Errorf("the silent bar is %v", got)
	}
	els := h.app.score.Parts[0].Measures[1].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("the bar with music in it now holds %d elements", len(els))
	}
	if r, ok := els[1].(*model.Rest); !ok || r.Dur != q {
		t.Errorf("the rest among the notes became %v", els[1].Duration())
	}
}

func TestARestThatAlreadyFitsIsNotTouched(t *testing.T) {
	h := barsOfSilence(t, 4, 4, 2)
	before := restOf(t, h, 0)
	h.post("/api/staff?part=0&time=4/4")
	if restOf(t, h, 0) != before {
		t.Error("the rest was replaced rather than left alone")
	}
	if got := restOf(t, h, 0).Dur; got != (model.Duration{Num: 1, Den: 1}) {
		t.Errorf("it is now %v", got)
	}
}

func TestASilentBarWithAMarkInItIsStillASilentBar(t *testing.T) {
	// A symbol standing in the bar is an element of the voice and takes no
	// time at all. Counted as music, the one bar of a score that carried a
	// symbol was the one bar this would not mend — which is exactly the
	// bar the symbol had been put in to begin with.
	h := barsOfSilence(t, 6, 4, 2)
	v := h.app.score.Parts[0].Measures[0].Voices[0]
	v.Elements = append(v.Elements, &model.SymbolElement{
		Symbol: model.SymbolMark{Name: "mark"},
	})
	h.post("/api/staff?part=0&time=4/4")

	els := h.app.score.Parts[0].Measures[0].Voices[0].Elements
	if len(els) != 2 {
		t.Fatalf("the bar holds %d elements", len(els))
	}
	r, ok := els[0].(*model.Rest)
	if !ok {
		t.Fatalf("the first element is a %T", els[0])
	}
	if r.Dur != (model.Duration{Num: 1, Den: 1}) {
		t.Errorf("the bar is silent for %v, want a whole bar of four-four", r.Dur)
	}
	if _, ok := els[1].(*model.SymbolElement); !ok {
		t.Error("the mark was lost")
	}
}

/*
 * And a file that is already wrong is mended on the way in.
 *
 * Setting the meter mends the bars, but a score saved before that was
 * written is saved wrong, and nobody is going to set a meter that is
 * already right in order to repair it. "I just opened the file and did a
 * pdf export. The first measure is still with stuff it should not have."
 */

// writeAndOpen saves a score and opens it in a fresh editor, as though the
// program had been closed and started again.
func writeAndOpen(t *testing.T, h *harness) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), "silence.gscore")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gscorefmt.Write(f, h.app.score, h.app.symbols); err != nil {
		t.Fatal(err)
	}
	f.Close()

	open := newHarness(t)
	if err := open.app.openPath(path); err != nil {
		t.Fatal(err)
	}
	return open
}

func TestOpeningAFileMendsARestLongerThanItsBar(t *testing.T) {
	h := barsOfSilence(t, 6, 4, 3)
	// The meter is changed under the rests without going through the
	// endpoint that re-measures them, which is how a file saved by an
	// older version came to hold this.
	h.app.score.Parts[0].Time = model.TimeSignature{Beats: 4, BeatValue: 4}

	open := writeAndOpen(t, h)
	for i := 0; i < 3; i++ {
		if got := restOf(t, open, i).Dur; got != (model.Duration{Num: 1, Den: 1}) {
			t.Errorf("bar %d opened at %v, want a whole bar of four-four",
				i+1, got)
		}
	}
}

func TestOpeningAFileLeavesAShortBarAlone(t *testing.T) {
	// The other half of the repair, which opening does not make. Without a
	// mark for a pickup, an incomplete bar is written by having less in it,
	// so a bar holding one short rest may be exactly what somebody meant —
	// and lengthening it on open would quietly add a beat to their music.
	h := barsOfSilence(t, 4, 4, 2)
	h.app.score.Parts[0].Measures[0].Voices[0].Elements = []model.Element{
		&model.Rest{Dur: model.Duration{Num: 1, Den: 4}},
	}

	open := writeAndOpen(t, h)
	if got := restOf(t, open, 0).Dur; got != (model.Duration{Num: 1, Den: 4}) {
		t.Errorf("the short bar opened at %v, want the quarter it was saved as",
			got)
	}
}
