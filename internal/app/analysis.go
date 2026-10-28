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
	"github.com/stricaud/gscore/internal/model"
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

// handleReach says how far each staff reaches, and whether the instrument it
// is written for can go there.
//
// The one check a composer cannot do for themselves. Everything else about a
// part can be heard by reading it; whether a horn can play a passage depends
// on the horn and not on the music, and somebody who does not play one has
// no way of finding out except by asking a player.
func (a *App) handleReach(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	notes := soundedNotes(a.score)
	insts := make([]analysis.Instrument, len(a.score.Parts))
	for i, p := range a.score.Parts {
		in := analysis.Instrument{
			Staff: p.Name, Name: p.Instrument.String(),
			// A staff of drums is read by position on a kit, not by
			// pitch, so there is no range to ask about. Asked anyway,
			// the bass drum of Beethoven's ninth was taken for a string
			// instrument written six hundred bars below its range.
			Percussion: p.Clef == model.Percussion,
		}
		if rr, ok := p.Instrument.Reach(); ok && !in.Percussion {
			in.Known = true
			in.Low, in.High = rr.Low, rr.High
			in.EasyLow, in.EasyHigh = rr.EasyLow, rr.EasyHigh
		}
		insts[i] = in
	}
	a.mu.RUnlock()

	writeJSON(w, map[string]any{
		"parts": analysis.Reaches(notes, insts, pitchName),
	})
}

// soundedNotes is every notehead in the score, as it sounds.
//
// Sounding rather than written, because a range is a physical fact about an
// instrument and does not move because the part is spelled in another key: a
// clarinet's bottom note is the same note either way. So the part's
// transposition is applied, and so is any octave line over it.
//
// The numbering is the engraver's, by way of baseRefs, so a finding can
// point at the notehead it is about and the editor can light it up. Taken
// from the performance instead it would have counted a repeated bar twice
// and a tremolo thirty times.
func soundedNotes(s *model.Score) []analysis.Sounded {
	base := baseRefs(s)
	var out []analysis.Sounded
	for pi, p := range s.Parts {
		// An octave line shifts everything under it until it closes, the
		// same way playback reads it. A bracket over notes that are at
		// pitch already says where the music is, not where to move it.
		shift := 0
		for mi := range p.Measures {
			for k, v := range p.Measures[mi].Ordered() {
				if mk := marksOf(v.El); mk != nil && mk.Octave != nil {
					if mk.Octave.Start {
						shift = mk.Octave.Amount
						if mk.Octave.SoundsAsWritten {
							shift = 0
						}
					}
				}
				ref := base[pi][mi] + k
				d := v.El.Duration()
				held := 0.0
				if d.Den != 0 {
					held = float64(d.Num) / float64(d.Den)
				}
				add := func(pitch model.Pitch) {
					out = append(out, analysis.Sounded{
						Part: pi, MIDI: sounding(p, pitch, shift).MIDI(),
						Bar: mi, Ref: ref, Held: held,
					})
				}
				switch el := v.El.(type) {
				case *model.Note:
					add(el.Pitch)
				case *model.Chord:
					for _, n := range el.Notes {
						add(n.Pitch)
					}
				}
				if mk := marksOf(v.El); mk != nil && mk.Octave != nil && mk.Octave.Stop {
					shift = 0
				}
			}
		}
	}
	return out
}
