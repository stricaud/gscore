/*
 * record_take.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"

	takes "github.com/stricaud/gscore/internal/recording"
)

// Taking a note in through the microphone.
//
// The page does the listening, because that is where the microphone is: a
// browser hands a page the sound and hands a program nothing. What it sends
// here is the take as it was heard — mono, 16-bit, at whatever rate the
// machine was running at — and what goes back is what the take turns out to
// be: which note, how far off it, where the sound starts and stops, and
// where in it a loop can be hidden.
//
// The reading is package recording's and not written twice. The page could
// have done its own, in JavaScript, and then a note recorded in the browser
// and the same note recorded in the application would disagree about what
// it was — which is the kind of difference nobody tracks down.

// takeInfo is what a recorded note turns out to be.
type takeInfo struct {
	// Hz is the pitch that was heard, zero where there is none to find —
	// which is a legitimate answer: a cymbal is a sample too.
	Hz float64 `json:"hz"`
	// Key is the MIDI key nearest that pitch and Cents how far off it is.
	Key   int `json:"key"`
	Cents int `json:"cents"`
	// Name is what to call that key — "A4" — for the page to show.
	Name string `json:"name"`
	// Frames is how long the take is once the silence at either end is
	// off, and Seconds the same in time.
	Frames  int     `json:"frames"`
	Seconds float64 `json:"seconds"`
	// LoopStart and LoopEnd are the stretch that can be repeated while a
	// key is held, both zero where none was found.
	LoopStart int `json:"loopStart"`
	LoopEnd   int `json:"loopEnd"`
	// Peak is the loudest sample in it, from 0 to 1, which is what says
	// whether the recording was made at a usable level.
	Peak float64 `json:"peak"`
	// Quiet and Loud warn about the level: too near the noise to be worth
	// keeping, or so near the top that it may already be clipped.
	Quiet bool `json:"quiet,omitempty"`
	Loud  bool `json:"loud,omitempty"`
}

// handleTake reads one recorded note and says what it is.
//
// The body is the samples themselves — 16-bit, little-endian, mono — and
// the rate comes in the query, because a browser records at whatever its
// audio clock is running at and that is not always 44100.
func (a *App) handleTake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	rate, err := strconv.Atoi(r.URL.Query().Get("rate"))
	if err != nil || rate < 8000 || rate > 192000 {
		http.Error(w, "a take has to say what rate it was recorded at, "+
			"between 8000 and 192000", http.StatusBadRequest)
		return
	}
	// A minute of mono at 48k is six megabytes; a note is two seconds.
	// Anything past that is not a note and is refused rather than read.
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		http.Error(w, "the take could not be read: "+err.Error(),
			http.StatusBadRequest)
		return
	}
	if len(raw) < 4 {
		http.Error(w, "there is nothing in this take", http.StatusBadRequest)
		return
	}
	data := make([]int16, len(raw)/2)
	for i := range data {
		data[i] = int16(binary.LittleEndian.Uint16(raw[2*i:]))
	}

	take, err := takes.Read(data, rate)
	if err != nil {
		// Not an error in the program: a take with nothing in it is
		// something that happens, and the words say what to do about it.
		writeJSON(w, map[string]any{"error": err.Error()})
		return
	}

	out := takeInfo{
		Hz: take.Hz, Key: take.Key, Cents: take.Cents,
		Frames: len(take.Data), Seconds: float64(len(take.Data)) / float64(rate),
		LoopStart: take.LoopStart, LoopEnd: take.LoopEnd,
		Peak: take.Peak,
		// A take under a twentieth of full scale is mostly room, and one
		// that reaches the top may already have been clipped on the way
		// in — which no amount of care afterwards can undo.
		Quiet: take.Peak < 0.05,
		Loud:  take.Peak > 0.98,
	}
	if take.Hz > 0 {
		out.Name = keyName(take.Key)
	}
	writeJSON(w, out)
}

// keyName is what a MIDI key is called: 69 is A4.
//
// Sharps and no flats, because this is a label on a recording rather than
// notation: the sample of the black key between A and B is called A#4 in
// every soundfont editor there has ever been.
func keyName(key int) string {
	names := [...]string{"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B"}
	if key < 0 || key > 127 {
		return ""
	}
	return fmt.Sprintf("%s%d", names[key%12], key/12-1)
}
