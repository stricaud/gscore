/*
 * record.go
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

// Making a recording of the score.
//
// The same performance the editor plays and the MIDI file writes — there is
// one answer to what the music sounds like and everything here reads it —
// played through recorded instruments and written out losslessly.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/flac"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/synth"
)

// recording is what was asked for: which soundfonts, and how to record them.
type recording struct {
	// Font is the soundfont everything plays through unless told otherwise,
	// and Assign names the parts that play through another one.
	Font   string
	Assign map[int]string
	synth.Options
}

// readRecording reads the request's query into the choices made in the
// dialog, falling back on the sensible answer wherever nothing was said.
func readRecording(q map[string][]string) recording {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	rec := recording{
		Font:    get("font"),
		Assign:  parseAssignment(get("assign")),
		Options: synth.DefaultOptions(),
	}
	if n, err := strconv.Atoi(get("rate")); err == nil && n > 0 {
		rec.SampleRate = n
	}
	if v, err := strconv.ParseFloat(get("gain"), 64); err == nil && v > 0 {
		rec.Gain = v
	}
	if v, err := strconv.ParseFloat(get("tail"), 64); err == nil && v >= 0 {
		rec.Tail = v
	}
	if get("reverb") == "0" {
		rec.Reverb = false
	}
	if get("normalize") == "0" {
		rec.Normalize = false
	}
	return rec
}

// RecordFile records a score through one soundfont, which is what the
// command line asks for.
//
// With no soundfont named, the first one on this machine is used: somebody
// who has exactly one has already answered the question, and somebody who
// has none is told so rather than handed silence.
func RecordFile(w io.Writer, s *model.Score, font string) error {
	if font == "" {
		if installed := listSoundfonts(); len(installed) > 0 {
			font = installed[0].Path
		}
	}
	return recordFLAC(w, s, recording{Font: font, Options: synth.DefaultOptions()})
}

// recordFLAC plays the score through the chosen soundfonts and writes it as
// a FLAC file.
func recordFLAC(w io.Writer, s *model.Score, rec recording) error {
	if rec.Font == "" {
		return errNoSoundfont
	}
	fallback, err := loadFont(rec.Font)
	if err != nil {
		return fmt.Errorf("that soundfont could not be read: %w", err)
	}
	fonts := map[int]*synth.Font{}
	for part, path := range rec.Assign {
		if path == "" || path == rec.Font {
			continue
		}
		f, err := loadFont(path)
		if err != nil {
			return fmt.Errorf("the soundfont for staff %d could not be read: %w",
				part+1, err)
		}
		fonts[part] = f
	}

	pcm, err := synth.Render(Song(s), fonts, fallback, rec.Options)
	if err != nil {
		return err
	}
	return flac.Encode(w, pcm, 2, rec.SampleRate)
}

// handleRecord hands back a recording as a download.
//
// The counterpart of handleMIDI: in a browser there is nowhere to save to,
// so the file comes back. It is made in memory first — a few minutes of
// music is tens of megabytes — rather than streamed, because a render that
// fails halfway through would otherwise have already sent a successful
// header and half a file.
func (a *App) handleRecord(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	score := a.score
	name := a.suggestedBase()
	a.mu.RUnlock()

	var buf bytes.Buffer
	if err := recordFLAC(&buf, score, readRecording(r.URL.Query())); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "audio/flac")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", name+".flac"))
	w.Write(buf.Bytes())
}
