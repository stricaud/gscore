/*
 * stream_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package synth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/midi"
)

// aFont finds a soundfont to test with. There is none in the repository —
// they are tens of megabytes of somebody else's recordings — so these run
// where one has been downloaded and say nothing where one has not.
func aFont(t *testing.T) *Font {
	t.Helper()
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skip("nowhere to look for a soundfont")
	}
	dir := filepath.Join(base, "Gscore", "soundfonts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("no soundfont downloaded; the recording tests say how")
	}
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".sf2") {
			f, err := LoadFont(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			return f
		}
	}
	t.Skip("no soundfont downloaded")
	return nil
}

func scales() midi.Song {
	var song midi.Song
	for i := 0; i < 16; i++ {
		song.Notes = append(song.Notes, midi.Note{
			Start: float64(i) * 0.25, Dur: 0.5, Gate: 0.9,
			MIDI: 60 + i, Velocity: 90, Part: 0, Program: 0,
		})
	}
	return song
}

// Played a piece at a time, the music is the same music.
//
// This is the whole claim of streaming it: the same synthesizer is carried
// from one piece to the next, so a note held across a join goes on being
// held and the reverb of one piece is still in the air during the next. An
// implementation that started fresh for each piece would pass every other
// test here and click once a second.
func TestPlayingItInPiecesIsTheSamePerformance(t *testing.T) {
	f := aFont(t)
	opt := DefaultOptions()
	opt.Normalize = false
	opt.Tail = 0.5

	whole, err := NewStream(scales(), nil, f, opt, 0)
	if err != nil {
		t.Fatal(err)
	}
	frames := int(whole.Left()*float64(whole.SampleRate())) + 1
	want := whole.Read(frames)

	pieces, err := NewStream(scales(), nil, f, opt, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []int16
	for !pieces.Done() {
		got = append(got, pieces.Read(pieces.SampleRate()/4)...) // quarter-second pieces
	}

	if len(want) == 0 {
		t.Fatal("the whole performance came out empty")
	}
	if len(got) != len(want) {
		t.Fatalf("in pieces it came to %d samples, whole it came to %d",
			len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d differs: %d in pieces, %d whole", i, got[i], want[i])
		}
	}
}

// Playing from the middle starts there, and what is left is what is left.
func TestPlayingFromTheMiddle(t *testing.T) {
	f := aFont(t)
	opt := DefaultOptions()
	opt.Tail = 0.5

	all, err := NewStream(scales(), nil, f, opt, 0)
	if err != nil {
		t.Fatal(err)
	}
	half, err := NewStream(scales(), nil, f, opt, 2)
	if err != nil {
		t.Fatal(err)
	}
	if left := all.Left() - half.Left(); left < 1.99 || left > 2.01 {
		t.Errorf("starting two seconds in left %.3f seconds less to play, want 2",
			left)
	}
	// And it is not silence: a note sounding at that moment is taken up
	// where it is, rather than everything before it simply being dropped.
	pcm := half.Read(half.SampleRate() / 2)
	loud := 0
	for _, s := range pcm {
		if s > 200 || s < -200 {
			loud++
		}
	}
	if loud == 0 {
		t.Error("the first half second from the middle of the piece is silent")
	}
}

// A performance played to its end stops rather than going on for ever.
func TestItEnds(t *testing.T) {
	f := aFont(t)
	opt := DefaultOptions()
	opt.Tail = 0.25
	st, err := NewStream(scales(), nil, f, opt, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000 && !st.Done(); i++ {
		st.Read(st.SampleRate())
	}
	if !st.Done() {
		t.Fatal("the performance never finished")
	}
	if pcm := st.Read(1024); pcm != nil {
		t.Errorf("it gave %d more samples after it was over", len(pcm))
	}
}
