/*
 * live.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package app

// Playing the score through a soundfont, while it is being listened to.
//
// The editor's own player is built from oscillators and starts the instant
// the key is pressed, which is what playing along while you write needs. It
// does not sound like a flute. This is the other answer: the real instruments
// from the recording, rendered just ahead of the listener instead of all at
// once, so that pressing play still answers immediately.
//
// The page asks for a second of music at a time and schedules each piece
// against the one before it. Nothing here knows about that scheduling — it
// is handed frames when it is asked for frames, and the browser's clock is
// what the music is actually kept in time by.

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/stricaud/gscore/internal/synth"
)

// chunkFrames is how much is rendered for one request, a second at 44.1kHz.
//
// Short enough that the first one is ready before anybody notices, long
// enough that a page asking for the next one is not a page asking constantly.
const chunkFrames = 44100

// liveIdle is how long a performance nobody is listening to is kept.
//
// A page that is closed or reloaded mid-piece never says it stopped, and
// each performance holds a synthesizer and its voices. They are not large,
// but they are not nothing and they would otherwise accumulate for as long
// as gscore is open.
const liveIdle = 2 * time.Minute

// live is one performance being rendered as it is played.
type live struct {
	stream *synth.Stream
	mu     sync.Mutex
	used   time.Time
}

var performances struct {
	sync.Mutex
	byID map[string]*live
	next int
}

// handleLiveStart begins a performance and says where to listen for it.
func (a *App) handleLiveStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	// Same rule as saving and recording: the soundfonts are files on this
	// machine, and rendering an orchestra is work somebody else should not
	// be able to ask for. Served over a network the editor plays through its
	// own oscillators, which is what the page falls back to.
	if !a.canSound() {
		http.Error(w, "playing through a soundfont only works on this machine",
			http.StatusForbidden)
		return
	}
	q := r.URL.Query()
	rec := readRecording(q)
	if rec.Font == "" {
		rec.Font = a.playbackFont()
	}
	if rec.Font == "" {
		http.Error(w, errNoSoundfont.Error(), http.StatusBadRequest)
		return
	}
	// Played, not recorded: no normalising, because the level cannot be
	// decided from music that has not been rendered yet, and a level that
	// changed as it went would be worse than one that is simply what it is.
	rec.Normalize = false
	// And no tail: the reverb of the last chord is worth recording and is
	// just a wait before the cursor stops.
	rec.Tail = 0.3

	from, _ := strconv.ParseFloat(q.Get("from"), 64)

	fallback, err := loadFont(rec.Font)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fonts := map[int]*synth.Font{}
	for part, path := range rec.Assign {
		if path == "" || path == rec.Font {
			continue
		}
		f, err := loadFont(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fonts[part] = f
	}

	a.mu.RLock()
	song := Song(a.score)
	a.mu.RUnlock()
	if len(song.Notes) == 0 {
		http.Error(w, "there is nothing to play", http.StatusBadRequest)
		return
	}

	stream, err := synth.NewStream(song, fonts, fallback, rec.Options, from)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	performances.Lock()
	if performances.byID == nil {
		performances.byID = map[string]*live{}
	}
	// Whatever nobody came back for.
	for id, p := range performances.byID {
		if time.Since(p.used) > liveIdle {
			delete(performances.byID, id)
		}
	}
	performances.next++
	id := fmt.Sprintf("p%d-%d", performances.next, time.Now().UnixNano())
	performances.byID[id] = &live{stream: stream, used: time.Now()}
	performances.Unlock()

	writeJSON(w, map[string]any{
		"id":       id,
		"rate":     stream.SampleRate(),
		"chunk":    chunkFrames,
		"duration": stream.Left(),
		"font":     fallback.Name,
	})
}

// handleLiveChunk renders the next piece of a performance.
//
// Raw interleaved 16-bit samples, which is what the page turns straight into
// something it can play. Nothing is encoded on the way: this is a localhost
// request for a second of audio and any format cleverer than "the samples"
// would cost more to pack and unpack than it saves.
func (a *App) handleLiveChunk(w http.ResponseWriter, r *http.Request) {
	performances.Lock()
	p := performances.byID[r.URL.Query().Get("id")]
	performances.Unlock()
	if p == nil {
		http.Error(w, "that performance is over", http.StatusNotFound)
		return
	}

	frames := chunkFrames
	if n, err := strconv.Atoi(r.URL.Query().Get("frames")); err == nil && n > 0 && n <= 8*chunkFrames {
		frames = n
	}

	p.mu.Lock()
	p.used = time.Now()
	pcm := p.stream.Read(frames)
	done := p.stream.Done()
	p.mu.Unlock()

	out := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(s))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if done {
		// So the page knows to stop asking rather than discovering it from
		// an empty answer it cannot tell from a slow one.
		w.Header().Set("X-Gscore-End", "1")
	}
	w.Write(out)
}

// handleLiveStop ends a performance.
func (a *App) handleLiveStop(w http.ResponseWriter, r *http.Request) {
	performances.Lock()
	delete(performances.byID, r.URL.Query().Get("id"))
	performances.Unlock()
	writeJSON(w, map[string]string{"status": "ok"})
}
