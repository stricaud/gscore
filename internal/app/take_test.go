//go:build !js

/*
 * take_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading a note played into the microphone.
 *
 * The page listens, because that is where the microphone is, and sends the
 * samples here. What they turn out to be is worked out in Go so that a note
 * recorded in the browser and the same note recorded in the application
 * cannot disagree about what it is.
 */

package app

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
)

// blown makes a take as a microphone would hand one over: silence, a note
// with a soft attack, and a tail of room.
func blown(hz float64, rate int, seconds float64) []byte {
	lead := int(0.2 * float64(rate))
	note := int(seconds * float64(rate))
	tail := int(0.2 * float64(rate))
	out := make([]byte, 0, (lead+note+tail)*2)
	put := func(v float64) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(int16(v*28000)))
		out = append(out, b[0], b[1])
	}
	for i := 0; i < lead; i++ {
		put(0.002 * math.Sin(float64(i)))
	}
	for i := 0; i < note; i++ {
		t := float64(i) / float64(rate)
		env := math.Min(1, t/0.05)
		if left := seconds - t; left < 0.1 {
			env = left / 0.1
		}
		put(0.5 * env * math.Sin(2*math.Pi*hz*t))
	}
	for i := 0; i < tail; i++ {
		put(0.002 * math.Sin(float64(i)))
	}
	return out
}

// send posts a take and reads the answer.
func send(t *testing.T, h *harness, body []byte, query string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/take?"+query,
		bytes.NewReader(body))
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/take?%s answered %d: %s",
			query, rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("the answer was %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestATakeSaysWhichNoteItIs(t *testing.T) {
	h := newHarness(t)
	got := send(t, h, blown(440, 44100, 1.5), "rate=44100")

	if got["name"] != "A4" {
		t.Errorf("the take came back as %v, want A4", got["name"])
	}
	if key, _ := got["key"].(float64); key != 69 {
		t.Errorf("the take is key %v, want 69", got["key"])
	}
	if cents, _ := got["cents"].(float64); math.Abs(cents) > 5 {
		t.Errorf("the take is %v cents off, want near nought", got["cents"])
	}
	if hz, _ := got["hz"].(float64); math.Abs(hz-440) > 3 {
		t.Errorf("the take was heard at %v Hz, want 440", got["hz"])
	}
}

func TestATakeIsTrimmedAndLooped(t *testing.T) {
	h := newHarness(t)
	got := send(t, h, blown(392, 44100, 2), "rate=44100")

	frames, _ := got["frames"].(float64)
	if frames <= 0 || frames >= float64(44100*2.4) {
		t.Errorf("the take came back %v frames long, want the note alone "+
			"out of two and a half seconds", got["frames"])
	}
	start, _ := got["loopStart"].(float64)
	end, _ := got["loopEnd"].(float64)
	if end <= start {
		t.Errorf("no loop was found in two seconds of steady tone: %v to %v",
			got["loopStart"], got["loopEnd"])
	}
	if end > frames {
		t.Errorf("the loop ends at %v, past the end of a take of %v frames",
			got["loopEnd"], got["frames"])
	}
}

func TestATakeSaysWhenTheLevelIsWrong(t *testing.T) {
	// Somebody holding a flute three feet from a laptop gets a recording
	// that is mostly room, and nothing afterwards can mend it. Better to
	// say so while the instrument is still out of its case.
	h := newHarness(t)
	quiet := blown(440, 44100, 1)
	// A tenth of the level.
	for i := 0; i+1 < len(quiet); i += 2 {
		v := int16(binary.LittleEndian.Uint16(quiet[i:]))
		binary.LittleEndian.PutUint16(quiet[i:], uint16(v/10))
	}
	got := send(t, h, quiet, "rate=44100")
	if got["quiet"] != true {
		t.Errorf("a take at a tenth of the level did not say so: %v", got)
	}
	// And a good one does not complain.
	fine := send(t, h, blown(440, 44100, 1), "rate=44100")
	if fine["quiet"] == true || fine["loud"] == true {
		t.Errorf("a take at a good level was complained about: %v", fine)
	}
}

func TestSomethingWithNoPitchStillComesBack(t *testing.T) {
	// A cymbal is a sample too, and a reader that refused it would be
	// refusing the instrument.
	h := newHarness(t)
	// Real noise and not a sawtooth: anything with a period in it has a
	// pitch, and the reader is right to find one.
	rng := rand.New(rand.NewSource(3))
	noise := make([]byte, 44100*2)
	for i := 0; i+1 < len(noise); i += 2 {
		binary.LittleEndian.PutUint16(noise[i:],
			uint16(int16(rng.Intn(40000)-20000)))
	}
	got := send(t, h, noise, "rate=44100")
	if got["error"] != nil {
		t.Fatalf("a take with no pitch was refused: %v", got["error"])
	}
	if hz, _ := got["hz"].(float64); hz != 0 {
		t.Errorf("a pitch of %v Hz was invented for noise", got["hz"])
	}
	if frames, _ := got["frames"].(float64); frames <= 0 {
		t.Error("nothing came back")
	}
}

func TestATakeThatIsNotOneIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, c := range []struct {
		why   string
		body  []byte
		query string
	}{
		{"no rate", blown(440, 44100, 1), ""},
		{"a rate nothing records at", blown(440, 44100, 1), "rate=3"},
		{"nothing in it", nil, "rate=44100"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/take?"+c.query,
			bytes.NewReader(c.body))
		h.mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("a take with %s was accepted", c.why)
		}
	}
	// Silence is not a failure of the program, so it comes back as words
	// rather than as an error code: it is something that happens, and the
	// words say what to do about it.
	silent := make([]byte, 44100*2)
	got := send(t, h, silent, "rate=44100")
	if got["error"] == nil {
		t.Error("a take of pure silence was read as a recording")
	}
}
