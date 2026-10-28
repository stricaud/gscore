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

	// What the chord is called, and what it is doing in the key the music
	// is in where it falls.
	//
	// The name is a fact about the notes; the numeral is a fact about the
	// notes and the key together, and it is the one that says what the
	// chord is *for*. Without it the panel could tell somebody that C♯ E
	// G♯ is rough to two decimal places and not that it is the tonic.
	midis := make([]int, 0, len(sounding))
	for _, n := range sounding {
		midis = append(midis, n.MIDI)
	}
	chord := analysis.NameChord(midis)
	a.mu.RLock()
	bar := barAt(pb, at)
	key, fit := keyAround(a.score, bar)
	// Spelled the way the music around it is spelled: the dominant of a
	// piece in four sharps is G♯ major, and calling it A♭ major is the
	// same twelve notes written in a language the page is not in.
	fifths := 0
	for _, p := range a.score.Parts {
		if !p.Hidden && !p.Unpitched() {
			fifths = p.KeyAt(bar).Fifths
			break
		}
	}
	a.mu.RUnlock()

	writeJSON(w, map[string]any{
		"chord":      chord,
		"key":        key,
		"keyName":    key.Name(fifths),
		"keyFit":     fit,
		"numeral":    analysis.Numeral(chord, key),
		"function":   analysis.Function(chord, key),
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
			Staff: p.Name, Name: p.SoundName(),
			// A staff of drums is read by position on a kit, not by
			// pitch, so there is no range to ask about. Asked anyway,
			// the bass drum of Beethoven's ninth was taken for a string
			// instrument written six hundred bars below its range.
			Percussion: p.Unpitched(),
		}
		if rr, ok := p.Reach(); ok && !in.Percussion {
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

// barAt is which bar of the score a moment falls in.
func barAt(pb Playback, at float64) int {
	for _, m := range pb.Measures {
		if at >= m.Start && at < m.Start+m.Dur {
			return m.Index
		}
	}
	if n := len(pb.Measures); n > 0 {
		return pb.Measures[n-1].Index
	}
	return 0
}

// keyAround is the key the music is in at a bar, read from the phrase around
// it rather than from the key signature.
//
// A signature is written once and stands until somebody writes another one;
// the music moves through keys inside it all the time, and a numeral worked
// out from the signature would call the whole development of a sonata by the
// numerals of its exposition.
func keyAround(s *model.Score, bar int) (analysis.Key, analysis.Fit) {
	lo := bar - analysis.KeyWindow/2
	if lo < 0 {
		lo = 0
	}
	hi := lo + analysis.KeyWindow
	var w analysis.Weights
	for _, p := range s.Parts {
		if p.Hidden || p.Unpitched() {
			continue
		}
		for mi := lo; mi < hi && mi < len(p.Measures); mi++ {
			for _, v := range p.Measures[mi].Voices {
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						pc := sounding(p, pitch, 0).MIDI() % 12
						w[((pc%12)+12)%12] += held
					}
				}
			}
		}
	}
	return analysis.KeyOf(w)
}

// pitchesIn is the written pitches an element carries, and none for a rest.
func pitchesIn(el model.Element) []model.Pitch {
	switch e := el.(type) {
	case *model.Note:
		return []model.Pitch{e.Pitch}
	case *model.Chord:
		out := make([]model.Pitch, 0, len(e.Notes))
		for _, n := range e.Notes {
			out = append(out, n.Pitch)
		}
		return out
	}
	return nil
}

// handleKeys says what key the music is in, bar by bar, gathered into the
// stretches where it stays the same.
//
// Not what the key signature says. A signature is written once and stands
// until somebody writes another one; the music moves through keys inside it
// all the time, and where it moves is most of what there is to understand
// about a piece. The development of a sonata is the signature of the
// exposition and almost none of its keys.
func (a *App) handleKeys(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	bars := barWeights(a.score)
	fifths := make([]int, len(bars))
	for i := range bars {
		for _, p := range a.score.Parts {
			if !p.Hidden && !p.Unpitched() {
				fifths[i] = p.KeyAt(i).Fifths
				break
			}
		}
	}
	a.mu.RUnlock()

	spans := analysis.Keys(bars)
	out := make([]map[string]any, 0, len(spans))
	for _, s := range spans {
		at := 0
		if s.FromBar < len(fifths) {
			at = fifths[s.FromBar]
		}
		out = append(out, map[string]any{
			"fromBar": s.FromBar, "toBar": s.ToBar,
			"key": s.Key, "name": s.Key.Name(at), "fit": s.Fit,
			"clear": s.Fit.Clear(),
		})
	}
	// The movements are not sent: the editor already has them from
	// /api/info, and that is where the bar numbers printed on the page are
	// worked out from.
	writeJSON(w, map[string]any{"bars": len(bars), "keys": out})
}

// barWeights is how long each of the twelve pitch classes sounds in each
// bar, which is what a key is read from.
//
// How long and not how many: a semibreve on the tonic says more about the
// key than six passing sixteenths, and counting notes says the opposite.
// Every pitched staff counts, because the key is the piece's and not any one
// staff's — a bass line alone is in a dozen keys at once.
func barWeights(s *model.Score) []analysis.Weights {
	bars := 0
	for _, p := range s.Parts {
		if len(p.Measures) > bars {
			bars = len(p.Measures)
		}
	}
	out := make([]analysis.Weights, bars)
	for _, p := range s.Parts {
		if p.Hidden || p.Unpitched() {
			continue
		}
		for mi, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						pc := ((sounding(p, pitch, 0).MIDI() % 12) + 12) % 12
						out[mi][pc] += held
					}
				}
			}
		}
	}
	return out
}

// handleMap draws the shape of a piece: who is playing when, and what
// repeats what.
//
// Both are questions about the whole of it, which gscore could answer
// nothing of. A symphony is fifty-three staves and two and a half thousand
// bars and there was no way to see any of it at once — not which
// instruments carry a passage, not where the material comes back, not where
// the piece is dense and where it thins out.
func (a *App) handleMap(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	busy, shapes := scoreShape(a.score)
	a.mu.RUnlock()

	least := analysis.LeastEcho
	if v, err := strconv.Atoi(r.URL.Query().Get("least")); err == nil && v >= 1 && v <= 64 {
		least = v
	}
	echoes := analysis.Echoes(shapes, least)
	if echoes == nil {
		echoes = []analysis.Echo{}
	}
	writeJSON(w, map[string]any{
		"bars":    len(shapes),
		"staves":  busy,
		"busiest": analysis.Busiest(busy),
		"echoes":  echoes,
	})
}

// scoreShape reads the whole score once: how much each staff plays in each
// bar, and what each bar holds in a form two bars can be compared in.
//
// Once, because a symphony is a large walk and there is no reason to take it
// twice. The two answers come off the same pass.
func scoreShape(s *model.Score) ([]analysis.Busy, []analysis.BarShape) {
	bars := 0
	for _, p := range s.Parts {
		if len(p.Measures) > bars {
			bars = len(p.Measures)
		}
	}
	busy := make([]analysis.Busy, 0, len(s.Parts))
	inBar := make([][]analysis.BarNote, bars)
	for pi, p := range s.Parts {
		b := analysis.Busy{Part: pi, Name: p.Name, Bars: make([]int, bars)}
		if b.Name == "" {
			b.Name = p.SoundName()
		}
		for mi, m := range p.Measures {
			at := 0.0
			for _, v := range m.Voices {
				at = 0
				for _, el := range v.Elements {
					d := el.Duration()
					held := 0.0
					if d.Den != 0 {
						held = float64(d.Num) / float64(d.Den)
					}
					for _, pitch := range pitchesIn(el) {
						b.Bars[mi]++
						b.Notes++
						// A staff of drums is counted as playing — it is
						// — but its pitches are places on a kit and would
						// make two bars of the same rhythm look like two
						// different bars.
						if !p.Unpitched() {
							inBar[mi] = append(inBar[mi], analysis.BarNote{
								Part: pi, MIDI: sounding(p, pitch, 0).MIDI(),
								At: at, Held: held,
							})
						}
					}
					at += held
				}
			}
			if b.Bars[mi] > 0 {
				b.Playing++
			}
		}
		busy = append(busy, b)
	}
	shapes := make([]analysis.BarShape, bars)
	for i := range inBar {
		shapes[i] = analysis.Shaped(inBar[i])
	}
	return busy, shapes
}
