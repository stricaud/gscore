/*
 * pastemusescore_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Pasting what was copied in MuseScore: the system clipboard, read on the
 * machine the editor runs on, lands as gscore's own copy would.
 */

package app

import (
	"os"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// museScoreCopy is a range copied in MuseScore 4.7: three staves of three
// bars in 6/4 (the middle one twice that long, under fermatas), the top a
// held F sharp and then rests.
func museScoreCopy(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../format/musescore/testdata/clipboard-stafflist.xml")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// pianoAndVoice is a score of three staves in 6/4, on the machine the
// editor runs on, whose clipboard holds data at change count n.
func pianoAndVoice(t *testing.T, data []byte, n int) *harness {
	t.Helper()
	h := newHarness(t)
	h.post("/api/new")
	h.post("/api/addstaff")
	h.post("/api/addstaff")
	for _, p := range h.app.score.Parts {
		p.Time = model.TimeSignature{Beats: 6, BeatValue: 4}
	}
	h.app.local = true
	h.app.pasteboard = func(kind string) ([]byte, int) {
		if kind != "application/musescore/stafflist" {
			t.Errorf("the clipboard was asked for %q", kind)
		}
		return data, n
	}
	return h
}

func TestMusicCopiedInMuseScoreIsPasted(t *testing.T) {
	h := pianoAndVoice(t, museScoreCopy(t), 7)
	d := h.post("/api/paste?part=0&measure=0&at=0")
	if d["from"] != "MuseScore" {
		t.Errorf("the paste came from %v, want MuseScore", d["from"])
	}
	if n, _ := d["pasted"].(float64); n < 20 {
		t.Errorf("%v elements pasted, want the three staves' worth", d["pasted"])
	}

	// The top staff, cut into the 6/4 bars of this score: the held note,
	// the note it is held to, under its fermata, and the rests after.
	top := h.app.score.Parts[0].Measures
	if len(top) < 4 {
		t.Fatalf("the top staff has %d bars after the paste", len(top))
	}
	first, ok := top[0].Voices[0].Elements[0].(*model.Note)
	if !ok || first.Pitch.String() != "F#4" || !first.TieStart {
		t.Errorf("bar 1 opens with %#v, want a tied F sharp", top[0].Voices[0].Elements[0])
	}
	second, ok := top[1].Voices[0].Elements[0].(*model.Note)
	if !ok || !second.TieStop || !second.Marks.HasArticulation(model.Fermata) {
		t.Errorf("bar 2 opens with %#v, want the F sharp held under a fermata", top[1].Voices[0].Elements[0])
	}
	if r, ok := top[2].Voices[0].Elements[0].(*model.Rest); !ok || !r.Hidden {
		t.Errorf("bar 3 opens with %#v, want the hidden rest", top[2].Voices[0].Elements[0])
	}
	if n := len(top[3].Voices[0].Elements); n != 2 {
		t.Errorf("bar 4 holds %d elements, want the whole rest and the half", n)
	}

	// The staves under it, each to its own: the right hand's chord under
	// its ff, and the left hand's bass.
	rh := h.app.score.Parts[1].Measures[0].Voices[0].Elements
	if len(rh) < 2 {
		t.Fatalf("the right hand's first bar holds %d elements", len(rh))
	}
	if c, ok := rh[1].(*model.Chord); !ok || c.Marks.Dynamic != model.FF || len(c.Notes) != 4 {
		t.Errorf("the right hand's first chord is %#v", rh[1])
	}
	if lh := h.app.score.Parts[2].Measures[0].Voices[0].Elements; len(lh) == 0 {
		t.Error("nothing landed on the left hand")
	}
}

// What was copied last wins: a copy made in gscore after MuseScore's is the
// one meant, and one made before it is not.
func TestTheLaterCopyIsTheOnePasted(t *testing.T) {
	h := pianoAndVoice(t, museScoreCopy(t), 7)
	h.note(0, 0, 2, "quarter")
	h.note(0, 0, 4, "quarter")
	h.post("/api/copy?sel=0:0:0,0:0:1") // gscore's copy, at change count 7

	d := h.post("/api/paste?part=0&measure=1&at=0")
	if _, outside := d["from"]; outside || d["pasted"] != float64(2) {
		t.Errorf("after copying in gscore the paste is %v, want gscore's two notes", d)
	}

	// MuseScore copies again: the count moves on.
	h.app.pasteboard = func(string) ([]byte, int) { return museScoreCopy(t), 8 }
	d = h.post("/api/paste?part=0&measure=1&at=0")
	if d["from"] != "MuseScore" {
		t.Errorf("after copying in MuseScore the paste is %v, want MuseScore's", d)
	}
}

// A server reached from another machine does not show that machine this
// one's clipboard.
func TestTheClipboardIsNotReadForAServerReachedFromElsewhere(t *testing.T) {
	h := pianoAndVoice(t, museScoreCopy(t), 7)
	h.app.local = false
	if msg := h.postFails("/api/paste?part=0&measure=0&at=0"); !strings.Contains(msg, "nothing has been copied") {
		t.Errorf("a remote paste with nothing copied was answered %q", msg)
	}
}

// Something else on the clipboard, or nothing, is not a paste.
func TestOtherThingsOnTheClipboardAreNotPasted(t *testing.T) {
	h := pianoAndVoice(t, nil, 7)
	if msg := h.postFails("/api/paste?part=0&measure=0&at=0"); !strings.Contains(msg, "nothing has been copied") {
		t.Errorf("an empty clipboard was answered %q", msg)
	}
	h = pianoAndVoice(t, []byte("<html>not music</html>"), 7)
	if msg := h.postFails("/api/paste?part=0&measure=0&at=0"); !strings.Contains(msg, "MuseScore") {
		t.Errorf("a clipboard that is not music was answered %q", msg)
	}
}

func TestWhichBarAPointFallsIn(t *testing.T) {
	bar := model.TimeSignature{Beats: 6, BeatValue: 4}.BarDuration()
	cases := []struct {
		num, den, want int
	}{
		{0, 1, 0}, {3, 2, 1}, {3, 1, 2}, {9, 2, 3}, {11, 2, 3}, {-1, 2, 0},
	}
	for _, c := range cases {
		if got := barIndexOf(model.Duration{Num: c.num, Den: c.den}, bar); got != c.want {
			t.Errorf("%d/%d into bars of 6/4 is bar %d, want %d", c.num, c.den, got, c.want)
		}
	}
	if got := barIndexOf(model.Duration{Num: 5, Den: 1}, model.Duration{}); got != 0 {
		t.Errorf("with no bar length the bar is %d, want 0", got)
	}
}
