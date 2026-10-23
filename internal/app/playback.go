/*
 * playback.go
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

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/smufl"
)

// Event is one sounding note, with its timing already resolved to seconds.
//
// The score is flattened here rather than in the browser so that tempo, ties
// and voices are interpreted once, by the same code that engraves them. The
// browser only has to make a noise at the right moment.
type Event struct {
	MIDI  int     `json:"midi"`
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
	Part  int     `json:"part"`
	// Ref is the element this note came from, numbered exactly as the
	// engraver numbers its glyphs. That shared numbering is what lets the
	// editor light up the notehead that is sounding.
	Ref int `json:"ref"`
}

// MeasureTime is when a measure begins and how long it lasts.
//
// The playback cursor follows this rather than the notes. A score has one
// timeline but many staves, and each has its notes in different places, so a
// cursor built from note positions lurches between parts. Time is the thing
// every staff agrees on.
type MeasureTime struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
}

// Playback is a whole score's worth of events.
type Playback struct {
	Events   []Event       `json:"events"`
	Measures []MeasureTime `json:"measures"`
	Duration float64       `json:"duration"`
	Tempo    int           `json:"tempo"`
}

// buildPlayback walks the score and resolves every note to a time in seconds.
//
// gscore 0.0.9 declared score_midi_play() in src/include/score.h and a
// midi_instrument field on every staff, but never implemented either; this is
// that feature, twenty years later and without a MIDI device in the way.
func buildPlayback(s *model.Score) Playback {
	bpm := s.Tempo.BPM
	if bpm <= 0 {
		bpm = 120
	}
	unit := s.Tempo.Unit
	if unit.Den == 0 || unit.Num == 0 {
		unit = model.Duration{Num: 1, Den: 4}
	}
	// One unit lasts 60/bpm seconds, so a whole note lasts that scaled by how
	// many units fit in it.
	secondsPerUnit := 60.0 / float64(bpm)
	secondsPerWhole := secondsPerUnit / unit.Float()

	out := Playback{Tempo: bpm}
	// ref counts elements in the same order the engraver does: parts, then
	// measures, then the first voice's elements, rests included. Both sides
	// must walk identically or the highlight lands on the wrong note.
	ref := 1

	// Every part shares one timeline, so a measure's length is however long
	// the part that fills it most takes. A staff resting through a bar must
	// not shorten it.
	measureDur := map[int]float64{}

	for pi, p := range s.Parts {
		t := 0.0
		// A tie means the two notes sound as one, so a tied-to note extends
		// the event already running rather than starting another.
		open := map[int]int{} // midi -> index in out.Events
		for mi, m := range p.Measures {
			// Voices within a measure sound together, so each starts at the
			// measure's time and the measure lasts as long as the longest.
			if len(m.Voices) == 0 {
				continue
			}
			// Only the first voice, matching the engraver, so that refs line
			// up. ABC import puts everything in one voice per measure.
			v := m.Voices[0]
			vt := t
			for _, el := range v.Elements {
				d := el.Duration().Float() * secondsPerWhole
				switch n := el.(type) {
				case *model.Note:
					addNote(&out, open, &n.Marks, n.Pitch, vt, d, pi, ref)
				case *model.Chord:
					for _, cn := range n.Notes {
						addNote(&out, open, &n.Marks, cn.Pitch, vt, d, pi, ref)
					}
				}
				ref++
				vt += d
			}
			if d := vt - t; d > measureDur[mi] {
				measureDur[mi] = d
			}
			t = maxF(t, vt)
		}
		if t > out.Duration {
			out.Duration = t
		}
	}

	// The measures end to end, which is the cursor's path.
	count := 0
	for _, p := range s.Parts {
		if n := len(p.Measures); n > count {
			count = n
		}
	}
	at := 0.0
	for i := 0; i < count; i++ {
		d := measureDur[i]
		if d <= 0 {
			// An empty bar still takes its meter's worth of time, or the
			// cursor would skip straight over it.
			d = emptyMeasureDur(s, i, secondsPerWhole)
		}
		out.Measures = append(out.Measures, MeasureTime{Index: i, Start: at, Dur: d})
		at += d
	}
	if at > out.Duration {
		out.Duration = at
	}
	return out
}

// emptyMeasureDur is how long a bar lasts when nothing in it sounds.
func emptyMeasureDur(s *model.Score, i int, secondsPerWhole float64) float64 {
	for _, p := range s.Parts {
		if i < len(p.Measures) {
			if bar := p.Time.BarDuration(); !bar.IsZero() {
				return bar.Float() * secondsPerWhole
			}
		}
	}
	return 0
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func addNote(out *Playback, open map[int]int, m *model.Marks, p model.Pitch, start, dur float64, part, ref int) {
	midi := p.MIDI()
	if m.TieStop {
		if idx, ok := open[midi]; ok {
			// A tie is one sound, not two, so extend what is already running.
			out.Events[idx].Dur += dur
			if !m.TieStart {
				delete(open, midi)
			}
			return
		}
	}
	out.Events = append(out.Events, Event{MIDI: midi, Start: start, Dur: dur, Part: part, Ref: ref})
	if m.TieStart {
		open[midi] = len(out.Events) - 1
	}
}

func (a *App) handlePlayback(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	pb := buildPlayback(a.score)
	a.mu.RUnlock()
	writeJSON(w, pb)
}

/*
 * Small helpers the server needs.
 */

func fontData() []byte { return smufl.FontData() }

// pngEncode writes an image, used for the symbol palette.
func pngEncode(w io.Writer, img image.Image) error { return png.Encode(w, img) }

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// fs strips the embedded directory prefix so the UI is served from the root.
func fsSub(f fs.FS, dir string) (fs.FS, error) { return fs.Sub(f, dir) }
