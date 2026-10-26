/*
 * analysis.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"net/http"
	"strconv"

	"github.com/stricaud/gscore/internal/analysis"
)

// handleHarmonics says what the harmony is made of at one moment.
//
// The moment is given in seconds along the performance, which is what the
// playback cursor already works in — so the panel can follow the music as it
// plays, or sit on the bar the cursor is in while it is being written.
//
// Everything is worked out from the written notes. A spectrum taken off the
// audio would show the soundfont: its samples, its filters, its reverb, and
// a different answer for the same chord on another instrument. What is asked
// here is a question about the notes.
func (a *App) handleHarmonics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	at, err := strconv.ParseFloat(q.Get("at"), 64)
	if err != nil {
		at = -1
	}

	opt := analysis.DefaultOptions()
	if v, err := strconv.Atoi(q.Get("partials")); err == nil && v > 0 && v <= 32 {
		opt.Partials = v
	}
	if v, err := strconv.ParseFloat(q.Get("rolloff"), 64); err == nil && v > 0 {
		opt.Rolloff = v
	}

	a.mu.RLock()
	pb := buildPlayback(a.score)
	names := make([]string, len(a.score.Parts))
	for i, p := range a.score.Parts {
		names[i] = p.Name
		if names[i] == "" {
			names[i] = p.Instrument.String()
		}
	}
	a.mu.RUnlock()

	// Where to look, when the question was asked about a note rather than
	// about a moment.
	//
	// The editor knows which notehead is picked up and nothing about the
	// clock; the timeline is here. So a note can be named instead of a
	// time, and "what is sounding when this note sounds" — which is the
	// question somebody writing a chord is actually asking — needs nothing
	// in the browser to work it out.
	if v, err := strconv.Atoi(q.Get("ref")); err == nil && v > 0 {
		for _, e := range pb.Events {
			if e.Ref == v {
				at = e.Start
				break
			}
		}
	} else if v, err := strconv.Atoi(q.Get("measure")); err == nil && v >= 0 {
		for _, m := range pb.Measures {
			if m.Index == v {
				at = m.Start
				break
			}
		}
	}
	if at < 0 {
		at = 0
	}

	// Everything sounding at that moment. A note counts from the instant it
	// starts until it stops: a chord is what is held together, which is what
	// the question is about, and a note that has just been released is no
	// longer part of it.
	var sounding []analysis.Note
	for _, e := range pb.Events {
		if e.Drum > 0 {
			continue // a drum has no pitch to put a series on
		}
		if at < e.Start || at >= e.Start+e.Dur {
			continue
		}
		sounding = append(sounding, analysis.Note{
			MIDI:     e.MIDI,
			Name:     pitchName(e.MIDI),
			Part:     e.Part,
			Velocity: e.Velocity,
			Ref:      e.Ref,
		})
	}

	out := analysis.Of(sounding, opt)
	out.At = at

	writeJSON(w, map[string]any{
		"at":         out.At,
		"duration":   pb.Duration,
		"notes":      out.Notes,
		"partials":   out.Partials,
		"coincident": out.Coincident,
		"intervals":  out.Intervals,
		"roughness":  out.Roughness,
		"verdict":    out.Verdict,
		"staves":     names,
	})
}

// pitchName writes a MIDI note the way a musician says it.
//
// Sharps throughout, because a sounding pitch has no key signature to say
// whether it is an F sharp or a G flat: this is what is heard, not what is
// written, and the two part company on every transposing instrument.
func pitchName(midi int) string {
	names := [...]string{"C", "C♯", "D", "D♯", "E", "F", "F♯", "G", "G♯", "A", "A♯", "B"}
	if midi < 0 || midi > 127 {
		return ""
	}
	return names[midi%12] + strconv.Itoa(midi/12-1)
}
