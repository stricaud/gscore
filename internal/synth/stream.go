/*
 * stream.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package synth

// Playing a score as it is heard rather than all at once.
//
// A recording can take its time: it renders the whole piece, looks at how
// loud the loudest moment turned out, and writes the file. Playing cannot.
// Somebody presses play and expects sound, and a few seconds of silence while
// an orchestra is computed is not an editor.
//
// So the same machinery is run forward a block at a time, just ahead of what
// is being listened to. It is the same synthesizer carried along from block
// to block, which is what matters and what a chunk-at-a-time renderer that
// started fresh each time would lose: a note held across a boundary goes on
// being held, a cymbal goes on decaying, and the reverb of one block is still
// in the air during the next.

import (
	"errors"
	"sort"

	"github.com/sinshu/go-meltysynth/meltysynth"

	"github.com/stricaud/gscore/internal/midi"
)

// Stream renders a performance a block at a time.
//
// Not safe for use from two goroutines at once: it is one performance moving
// forward, and there is only one place it can be up to.
type Stream struct {
	rate   int
	frames int // how many are left to render in total
	gain   float64
	groups []*voiceGroup
}

// voiceGroup is everything played through one soundfont.
type voiceGroup struct {
	sy     *meltysynth.Synthesizer
	events []streamEvent
	next   int
	l, r   []float32
}

// streamEvent is a note starting or stopping, at a frame.
type streamEvent struct {
	at       int
	on       bool
	ch       int
	key, vel int
}

// NewStream prepares a performance to be played from a point in it.
//
// from is in seconds. A note already sounding there is taken up part way
// through rather than struck again, which is what the editor's own player
// does and what somebody carrying on from a pause is listening for.
func NewStream(song midi.Song, fonts map[int]*Font, fallback *Font, opt Options, from float64) (*Stream, error) {
	if opt.SampleRate <= 0 {
		opt.SampleRate = 44100
	}
	if opt.Gain == 0 {
		opt.Gain = 1
	}
	if fallback == nil {
		return nil, errors.New("synth: no soundfont to play this with")
	}
	if from < 0 {
		from = 0
	}

	end := 0.0
	for _, n := range song.Notes {
		if stop := n.Start + n.Dur; stop > end {
			end = stop
		}
	}
	st := &Stream{rate: opt.SampleRate, gain: opt.Gain}
	if left := end + opt.Tail - from; left > 0 {
		st.frames = int(left*float64(opt.SampleRate)) + 1
	}

	byFont := map[*Font][]midi.Note{}
	for _, n := range song.Notes {
		f := fallback
		if assigned, ok := fonts[n.Part]; ok && assigned != nil {
			f = assigned
		}
		byFont[f] = append(byFont[f], n)
	}
	for f, notes := range byFont {
		g, err := newGroup(f, notes, opt, from)
		if err != nil {
			return nil, err
		}
		st.groups = append(st.groups, g)
	}
	return st, nil
}

func newGroup(f *Font, notes []midi.Note, opt Options, from float64) (*voiceGroup, error) {
	settings := meltysynth.NewSynthesizerSettings(int32(opt.SampleRate))
	settings.EnableReverbAndChorus = opt.Reverb
	// A score can have more going on at once than a synthesizer's default
	// allowance: a held pedal under a chord in every part, plus the tails of
	// what came before. Notes over the limit are stolen, and a stolen note
	// stops in the middle, which is heard.
	settings.MaximumPolyphony = 256
	sy, err := meltysynth.NewSynthesizer(f.sf, settings)
	if err != nil {
		return nil, err
	}

	channels := channelsFor(notes)
	for part, ch := range channels {
		program, drum := 0, false
		for _, n := range notes {
			if n.Part == part {
				program, drum = n.Program, n.Drum > 0
				break
			}
		}
		// Through the font's own answer, not the General MIDI number: a
		// one-instrument soundfont has its instrument somewhere else, and
		// sending it the number the score carries selects nothing.
		bank := 0
		if drum {
			bank = drumBank
		}
		if p, _ := f.Lookup(program, drum); p.Name != "" {
			bank, program = p.Bank, p.Program
		}
		if ch != 9 {
			sy.ProcessMidiMessage(int32(ch), 0xB0, 0, int32(bank))
		}
		sy.ProcessMidiMessage(int32(ch), 0xC0, int32(program), 0)
	}

	g := &voiceGroup{sy: sy}
	rate := float64(opt.SampleRate)
	for _, n := range notes {
		gate := n.Gate
		if gate <= 0 {
			gate = 1
		}
		stop := n.Start + n.Dur*gate
		if stop <= from {
			continue // over before we came in
		}
		// A drum's key is which drum it is, not a pitch.
		key := n.MIDI
		if n.Drum > 0 {
			key = n.Drum
		}
		ch := channels[n.Part]
		vel := n.Velocity
		if vel <= 0 {
			vel = 80
		}
		start := int((n.Start - from) * rate)
		if start < 0 {
			start = 0
		}
		off := int((stop - from) * rate)
		if off <= start {
			off = start + 1
		}
		g.events = append(g.events,
			streamEvent{at: start, on: true, ch: ch, key: key, vel: vel},
			streamEvent{at: off, ch: ch, key: key})
	}
	sort.Slice(g.events, func(i, j int) bool {
		if g.events[i].at != g.events[j].at {
			return g.events[i].at < g.events[j].at
		}
		// At the same instant, release before striking: a repeated note has
		// to be let go before it is played again, or the second one is not
		// heard at all.
		return !g.events[i].on && g.events[j].on
	})
	return g, nil
}

// SampleRate is how many frames a second this renders.
func (s *Stream) SampleRate() int { return s.rate }

// Left is how much music there is still to come, in seconds.
func (s *Stream) Left() float64 { return float64(s.frames) / float64(s.rate) }

// Done says the performance has been rendered to its end.
func (s *Stream) Done() bool { return s.frames <= 0 }

// read renders the next frames into left and right, summing every soundfont
// in use, and returns how many it managed — fewer than asked for at the end
// of the piece, and none at all once it is over.
func (s *Stream) read(left, right []float64) int {
	n := len(left)
	if n > len(right) {
		n = len(right)
	}
	if n > s.frames {
		n = s.frames
	}
	if n <= 0 {
		return 0
	}
	for i := 0; i < n; i++ {
		left[i], right[i] = 0, 0
	}
	for _, g := range s.groups {
		g.render(left[:n], right[:n])
	}
	s.frames -= n
	for _, g := range s.groups {
		g.advance(n)
	}
	return n
}

// render plays this group's share of the next frames and adds it in.
//
// The block is cut at every note, so that a note starts on the frame it is
// written to start on and not at the next block boundary. At a hundred notes
// a second that is a hundred short renders, which is nothing beside the
// hundred thousand frames of arithmetic underneath them.
func (g *voiceGroup) render(left, right []float64) {
	if cap(g.l) < len(left) {
		g.l = make([]float32, len(left))
		g.r = make([]float32, len(left))
	}
	l, r := g.l[:len(left)], g.r[:len(left)]

	at := 0
	i := g.next
	for at < len(left) {
		// Everything that happens on this frame.
		for i < len(g.events) && g.events[i].at <= at {
			e := g.events[i]
			if e.on {
				g.sy.NoteOn(int32(e.ch), int32(e.key), int32(e.vel))
			} else {
				g.sy.NoteOff(int32(e.ch), int32(e.key))
			}
			i++
		}
		// Up to the next one, or to the end of the block.
		to := len(left)
		if i < len(g.events) && g.events[i].at < to {
			to = g.events[i].at
		}
		if to <= at {
			to = at + 1
			if to > len(left) {
				break
			}
		}
		g.sy.Render(l[at:to], r[at:to])
		at = to
	}
	for k := range left {
		left[k] += float64(l[k])
		right[k] += float64(r[k])
	}
}

// advance moves this group's events along by a block, so that the next one
// counts from its own beginning.
func (g *voiceGroup) advance(n int) {
	for g.next < len(g.events) && g.events[g.next].at < n {
		g.next++
	}
	for i := g.next; i < len(g.events); i++ {
		g.events[i].at -= n
	}
}

// Read renders the next frames as interleaved stereo samples, ready to be
// played.
//
// No normalising, unlike a recording: the level has to be the same from one
// block to the next or the music would swell and shrink as it played,
// following nothing but what happened to be loud a moment ago.
func (s *Stream) Read(frames int) []int16 {
	left := make([]float64, frames)
	right := make([]float64, frames)
	n := s.read(left, right)
	if n == 0 {
		return nil
	}
	out := make([]int16, 2*n)
	for i := 0; i < n; i++ {
		out[2*i] = sample(left[i] * s.gain)
		out[2*i+1] = sample(right[i] * s.gain)
	}
	return out
}
