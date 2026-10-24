/*
 * flac_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package flac

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The point of a lossless format is that what comes back is what went in, so
// that is what these check — against the reference decoder, which is the only
// opinion that counts. Where it is not installed the tests say so and stop,
// rather than passing on the strength of nothing having been read back.

// signal is a piece of music-shaped audio: a tone that slides, a quiet
// passage, a loud one, some silence and some noise. Each of those is a
// different problem for an encoder, and the awkward ones — the noise, the
// jumps — are the ones that catch a bug.
func signal(frames int) []int16 {
	rng := rand.New(rand.NewSource(7))
	pcm := make([]int16, 2*frames)
	for i := 0; i < frames; i++ {
		t := float64(i) / 44100
		hz := 220 + 400*t
		v := math.Sin(2*math.Pi*hz*t) * 0.4
		switch {
		case i%9000 < 1200: // silence
			v = 0
		case i%9000 < 2400: // noise
			v = rng.Float64()*2 - 1
		case i%9000 < 3000: // full scale, where clipping would show
			v = math.Copysign(1, v)
		}
		pcm[2*i] = int16(v * 32767)
		// The right channel close to the left but not identical, which is
		// what makes coding them together worth doing.
		pcm[2*i+1] = int16(v*31000) + int16(rng.Intn(8))
	}
	return pcm
}

// decoded runs the reference decoder over our file and returns the samples it
// got back.
func decoded(t *testing.T, path string) []int16 {
	t.Helper()
	raw := filepath.Join(t.TempDir(), "out.raw")
	cmd := exec.Command("flac", "-d", "--force-raw-format",
		"--endian=little", "--sign=signed", "-f", "-o", raw, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the reference decoder refused the file: %v\n%s", err, out)
	}
	b, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

func writeFile(t *testing.T, pcm []int16, channels, rate int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, pcm, channels, rate); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.flac")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func needFLAC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("flac"); err != nil {
		t.Skip("the reference decoder is not installed, so this cannot be checked")
	}
}

func TestWhatComesBackIsWhatWentIn(t *testing.T) {
	needFLAC(t)
	// Not a whole number of blocks, so the short last frame is written too.
	pcm := signal(4096*3 + 517)
	got := decoded(t, writeFile(t, pcm, 2, 44100))

	if len(got) != len(pcm) {
		t.Fatalf("got %d samples back, put %d in", len(got), len(pcm))
	}
	for i := range pcm {
		if got[i] != pcm[i] {
			t.Fatalf("sample %d came back as %d, was %d", i, got[i], pcm[i])
		}
	}
}

// The checksum in the header is of the audio, and the decoder compares it
// with what it decoded. A file that passes -t is one no player will complain
// about.
func TestTheReferenceDecoderVerifiesTheFile(t *testing.T) {
	needFLAC(t)
	path := writeFile(t, signal(4096*2+1), 2, 44100)
	if out, err := exec.Command("flac", "-t", path).CombinedOutput(); err != nil {
		t.Fatalf("flac -t failed: %v\n%s", err, out)
	}
}

func TestMonoAndOtherRates(t *testing.T) {
	needFLAC(t)
	stereo := signal(5000)
	mono := make([]int16, 5000)
	for i := range mono {
		mono[i] = stereo[2*i]
	}
	path := writeFile(t, mono, 1, 48000)
	got := decoded(t, path)
	if len(got) != len(mono) {
		t.Fatalf("got %d samples back, put %d in", len(got), len(mono))
	}
	for i := range mono {
		if got[i] != mono[i] {
			t.Fatalf("sample %d came back as %d, was %d", i, got[i], mono[i])
		}
	}
}

// Silence is the case where the format should cost almost nothing, and where
// an encoder that quietly wrote the samples out as they are would not be
// noticed by any test that only checks what comes back.
func TestSilenceCostsAlmostNothing(t *testing.T) {
	frames := 44100 * 4
	var buf bytes.Buffer
	if err := Encode(&buf, make([]int16, 2*frames), 2, 44100); err != nil {
		t.Fatal(err)
	}
	raw := 4 * frames
	if buf.Len() > raw/50 {
		t.Errorf("four seconds of silence took %d bytes of a possible %d: "+
			"it is not being compressed at all", buf.Len(), raw)
	}
}

// And music should come out appreciably smaller than the samples it was made
// from, or there is no reason to be writing FLAC rather than WAV.
func TestMusicIsSmallerThanTheSamples(t *testing.T) {
	pcm := make([]int16, 2*44100)
	for i := 0; i < len(pcm)/2; i++ {
		t := float64(i) / 44100
		v := 0.5*math.Sin(2*math.Pi*440*t) + 0.3*math.Sin(2*math.Pi*660*t)
		pcm[2*i] = int16(v * 20000)
		pcm[2*i+1] = int16(v * 19000)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, pcm, 2, 44100); err != nil {
		t.Fatal(err)
	}
	if raw := 2 * len(pcm); buf.Len() > raw*3/4 {
		t.Errorf("a second of tone took %d bytes of a possible %d", buf.Len(), raw)
	}
}
