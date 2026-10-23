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
	// Velocity is how loud, 0 to 127, from the dynamic in force.
	Velocity int `json:"velocity"`
	Part     int `json:"part"`
	// Program is the staff's General MIDI instrument, and Family is how it is
	// sounded. The browser needs the second to pick a voice; the first is
	// there so it can be more specific later.
	Program int    `json:"program"`
	Family  string `json:"family"`
	// Drum is the General MIDI percussion key when this event is a drum
	// rather than a pitch, and 0 otherwise. A drum staff's positions name
	// instruments, so what sounds cannot be worked out from the pitch.
	Drum int `json:"drum,omitempty"`
	// DrumName is what that instrument is called, for anything that wants to
	// say what is playing.
	DrumName string `json:"drumName,omitempty"`
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
		// A dynamic holds until another replaces it.
		loud := model.MF
		// An octave line shifts everything under it; it opens at one note and
		// closes at another, possibly bars later.
		shift := 0
		// A hairpin is a change of volume over a stretch of music, so what it
		// does cannot be known at the note it starts on. The notes it covers
		// are collected and scaled once it closes.
		var swell *swelling
		for mi, m := range p.Measures {
			// Voices within a measure sound together, so each starts at the
			// measure's time and the measure lasts as long as the longest.
			if len(m.Voices) == 0 {
				continue
			}
			// Only the first voice, matching the engraver, so that refs line
			// up. ABC import puts everything in one voice per measure.
			v := m.Voices[0]

			// A bar marked as a repeat sounds as the bar it stands for. The
			// sign is an instruction to the player, so it has to be one to
			// the playback too — a chart with eight repeat signs in it is
			// otherwise eight bars of silence.
			if m.Repeat > 0 {
				if src := repeatSource(p, mi, m.Repeat); src != nil {
					v = src
				}
			}
			vt := t
			for _, el := range v.Elements {
				d := el.Duration().Float() * secondsPerWhole
				mk := marksOf(el)
				if mk != nil && mk.Octave != nil && mk.Octave.Start {
					shift = mk.Octave.Amount
				}
				first := len(out.Events)
				switch n := el.(type) {
				case *model.Note:
					if n.Dynamic != model.DynamicNone {
						loud = n.Dynamic
						swell = nil // a written dynamic settles the question
					}
					addNote(&out, open, &n.Marks, sounding(p, n.Pitch, shift),
						vt, d, pi, ref, p.Instrument, loud, n.Percussion)
				case *model.Chord:
					if n.Dynamic != model.DynamicNone {
						loud = n.Dynamic
						swell = nil
					}
					for _, cn := range n.Notes {
						addNote(&out, open, &n.Marks, sounding(p, cn.Pitch, shift),
							vt, d, pi, ref, p.Instrument, loud, cn.Percussion)
					}
				}
				if mk != nil {
					if h := mk.Hairpin; h != nil {
						if h.Start && swell == nil {
							swell = &swelling{kind: h.Kind, from: first}
						}
						if h.Stop && swell != nil {
							swell.apply(&out, len(out.Events), loud)
							swell = nil
						}
					}
					if swell != nil {
						swell.to = len(out.Events)
					}
					if mk.Octave != nil && mk.Octave.Stop {
						shift = 0
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

func addNote(out *Playback, open map[int]int, m *model.Marks, p model.Pitch,
	start, dur float64, part, ref int, inst model.Instrument, loud model.Dynamic,
	drum int) {
	midi := p.MIDI()

	// A drum is a hit, not a note: it does not sustain, it is not tied, and
	// what sounds is named by the staff position rather than by the pitch.
	if drum > 0 {
		name := ""
		if pn, ok := model.PercussionByKey(drum); ok {
			name = pn.Name
		}
		out.Events = append(out.Events, Event{
			MIDI: drum, Start: start, Dur: dur, Part: part, Ref: ref,
			Family: "percussion", Drum: drum, DrumName: name,
			Velocity: loud.Velocity(),
		})
		return
	}

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
	out.Events = append(out.Events, Event{
		MIDI: midi, Start: start, Dur: dur, Part: part, Ref: ref,
		Program: int(inst), Family: inst.Family().String(),
		Velocity: loud.Velocity(),
	})
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

// sounding turns a written pitch into the one that is heard: the staff's
// transposition, and any octave line over it.
//
// An alto saxophone's part is written a major sixth above what comes out of
// the instrument, and an 8va line means "play this an octave higher than it is
// written" precisely so that the notes can stay on the staff. Both are
// differences between the page and the sound, and both belong here rather than
// in the model, which holds what is printed.
func sounding(p *model.Part, pitch model.Pitch, octaves int) model.Pitch {
	out := p.Transpose.Apply(pitch)
	out.Octave += octaves
	return out
}

// marksOf gets an element's marks, or nil for one that carries none.
func marksOf(el model.Element) *model.Marks {
	switch e := el.(type) {
	case *model.Note:
		return &e.Marks
	case *model.Chord:
		return &e.Marks
	case *model.Rest:
		return &e.Marks
	}
	return nil
}

// swelling is a hairpin being collected: the events it covers, and which way
// it goes.
type swelling struct {
	kind     model.HairpinKind
	from, to int
}

// apply ramps the velocity of everything under the hairpin.
//
// A crescendo is not a volume, it is a journey between two: it starts where
// the music already is and arrives one step louder, unless a written dynamic
// says otherwise, in which case the dynamic has already replaced this.
func (s *swelling) apply(out *Playback, end int, loud model.Dynamic) {
	if s.to > end {
		s.to = end
	}
	n := s.to - s.from
	if n <= 0 {
		return
	}
	base := out.Events[s.from].Velocity
	span := 28
	if s.kind == model.Diminuendo {
		span = -28
	}
	// Clamp so a crescendo from fff does not run off the top.
	if base+span > 124 {
		span = 124 - base
	}
	if base+span < 16 {
		span = 16 - base
	}
	for i := s.from; i < s.to; i++ {
		f := float64(i-s.from) / float64(n-1)
		if n == 1 {
			f = 1
		}
		out.Events[i].Velocity = base + int(float64(span)*f)
	}
	_ = loud
}

// repeatSource finds the bar a repeat sign stands for.
//
// One sign means the bar before. A two-bar sign stands for the two before it,
// so the first of the pair repeats the bar two back and the second repeats the
// one two back from it — which, walking forwards, is just "the bar `n` back"
// in both cases. A sign pointing at another sign is followed through rather
// than played as nothing.
func repeatSource(p *model.Part, at, bars int) *model.Voice {
	if bars < 1 {
		bars = 1
	}
	// A chain of signs is followed, but not forever: a file that points a
	// repeat at itself should come back silent rather than hang.
	for hops := 0; hops < 64; hops++ {
		at -= bars
		if at < 0 || at >= len(p.Measures) {
			return nil
		}
		m := p.Measures[at]
		if len(m.Voices) == 0 {
			return nil
		}
		if m.Repeat == 0 {
			return m.Voices[0]
		}
		bars = m.Repeat
	}
	return nil
}
