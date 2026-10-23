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

	out := Playback{Tempo: bpm}
	// The order the bars are actually played in: a repeat sends you back, a
	// first-time bar is skipped the second time through. Both the notes and
	// the cursor walk this, so the line down the page goes round when the
	// music does.
	order := s.PlayOrder()

	// Which number the engraver gave the first element of each bar of each
	// staff. A bar played twice sounds the same notes both times, so the
	// numbering is worked out once from the written score and reused —
	// counting along as we play would give a bar's notes new numbers on the
	// second time through and the highlight would stop finding them.
	refBase := baseRefs(s)

	// When each bar begins, worked out before any note is placed.
	//
	// There is one timeline in a score and every part is on it. Letting each
	// part keep its own clock — advancing it by whatever that part happens to
	// have in the bar — means a part with a short bar falls behind the
	// barline and stays behind: in Mozart's Offertorium the soloists' clock
	// ran forty-nine seconds behind the measure timeline by the last bar, so
	// the cursor and the note it was supposed to be pointing at were most of
	// a page apart. Deciding the bar lines first and hanging every part off
	// them makes that unrepresentable.
	starts, total := measureTimeline(s, order)
	for k, start := range starts {
		d := total - start
		if k+1 < len(starts) {
			d = starts[k+1] - start
		}
		out.Measures = append(out.Measures, MeasureTime{
			Index: order[k], Start: start, Dur: d,
		})
	}
	out.Duration = total

	for pi, p := range s.Parts {
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
		for step, mi := range order {
			if mi >= len(p.Measures) {
				continue // this staff is shorter than the score
			}
			m := p.Measures[mi]
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
				if src := p.RepeatSource(mi, m.Repeat); src != nil {
					v = src
				}
			}
			// Every part starts this bar where the bar starts, not where its
			// own last bar happened to end.
			vt := 0.0
			if step < len(starts) {
				vt = starts[step]
			}
			// How long a whole note lasts in this bar. A piece is not one
			// speed, so this is asked bar by bar rather than once for the
			// score: a tempo change partway through has to reach the notes
			// as well as the barlines, or the two drift apart from the bar
			// it is written at onwards.
			secondsPerWhole := wholeNoteSeconds(s, mi)

			ref := refBase[pi][mi]
			if m.Repeat > 0 {
				// A bar showing a repeat sign has no noteheads of its own —
				// the sign is all that is drawn. Its notes are borrowed from
				// the bar it stands for, so numbering them after that bar's
				// noteheads would light up a bar somewhere else on the page.
				// Zero means "nothing to highlight", which is the truth.
				ref = 0
			}
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
					gd := addGrace(&out, n.Grace, p, shift, vt, d, secondsPerWhole, pi, loud)
					addNote(&out, open, &n.Marks, sounding(p, n.Pitch, shift),
						vt+gd, d-gd, pi, ref, p.Instrument, loud, n.Percussion)
				case *model.Chord:
					if n.Dynamic != model.DynamicNone {
						loud = n.Dynamic
						swell = nil
					}
					gd := addGrace(&out, n.Grace, p, shift, vt, d, secondsPerWhole, pi, loud)
					for _, cn := range n.Notes {
						addNote(&out, open, &n.Marks, sounding(p, cn.Pitch, shift),
							vt+gd, d-gd, pi, ref, p.Instrument, loud, cn.Percussion)
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
				if ref != 0 {
					ref++
				}
				vt += d
			}
			// A note may ring past the last barline; the piece is not over
			// until it has stopped.
			if vt > out.Duration {
				out.Duration = vt
			}
		}
	}
	return out
}

// measureTimeline decides when each bar begins.
//
// How long a bar lasts is model.Score.MeasureDuration, and it is asked there
// rather than worked out again here: the engraver asks the same question to
// place a note within its bar, and two answers to it is exactly how the
// cursor and the note it pointed at came to be a bar apart.
//
// Returns the start of every bar and the time the last one ends.
func measureTimeline(s *model.Score, order []int) ([]float64, float64) {
	starts := make([]float64, len(order))
	at := 0.0
	for k, i := range order {
		starts[k] = at
		at += s.MeasureDuration(i).Float() * wholeNoteSeconds(s, i)
	}
	return starts, at
}

// wholeNoteSeconds is how long a whole note lasts in the bar at this index.
//
// One beat lasts 60/bpm seconds, and a whole note is however many beats fit
// in it — which is why the beat the number counts has to be carried beside
// the number. A tempo of 120 means nothing on its own.
func wholeNoteSeconds(s *model.Score, bar int) float64 {
	t := s.TempoAt(bar)
	bpm := t.BPM
	if bpm <= 0 {
		bpm = 120
	}
	unit := t.Unit
	if unit.Den == 0 || unit.Num == 0 {
		unit = model.Duration{Num: 1, Den: 4}
	}
	return (60.0 / float64(bpm)) / unit.Float()
}

// baseRefs gives the number the engraver assigned to the first element of
// each bar of each staff.
//
// The engraver numbers elements walking staves, then bars, then the first
// voice. Playback has to agree exactly, because that number is the only thing
// tying a sounding note to the notehead drawn for it — and once a bar can be
// played more than once, counting along as we go no longer agrees.
func baseRefs(s *model.Score) [][]int {
	out := make([][]int, len(s.Parts))
	ref := 1
	for pi, p := range s.Parts {
		out[pi] = make([]int, len(p.Measures))
		for mi, m := range p.Measures {
			out[pi][mi] = ref
			if len(m.Voices) > 0 {
				ref += len(m.Voices[0].Elements)
			}
		}
	}
	return out
}

// addGrace sounds the small notes in front of a note and returns how much of
// that note's time they took.
//
// They take it from the note they lean on rather than from the note before,
// so that the bar still lasts exactly as long as it says it does. Stealing
// from the note before would be truer to how an acciaccatura is actually
// played and would put every bar after it out of step with the cursor, which
// follows the written durations — a trade the other way round is not worth
// making for an ornament a few milliseconds long.
func addGrace(out *Playback, g *model.GraceGroup, part *model.Part, shift int,
	start, dur, secondsPerWhole float64, pi int, loud model.Dynamic) float64 {

	if g == nil || len(g.Notes) == 0 {
		return 0
	}
	n := float64(len(g.Notes))
	// A sixteenth each, or a third of the note they precede shared between
	// them, whichever is less: grace notes in front of a short note have to
	// be shorter still, or they swallow it.
	each := secondsPerWhole / 16
	if each*n > dur/3 {
		each = dur / 3 / n
	}
	at := start
	for _, gn := range g.Notes {
		// Ref 0: a grace note is drawn but not numbered, so there is no
		// notehead for the editor to light up. Zero says so.
		addNote(out, map[int]int{}, &model.Marks{}, sounding(part, gn.Pitch, shift),
			at, each, pi, 0, part.Instrument, loud, gn.Percussion)
		at += each
	}
	return each * n
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
