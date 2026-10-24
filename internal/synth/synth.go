/*
 * synth.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

// Package synth turns a performance into sound, using recordings of real
// instruments.
//
// The editor's own player is built from oscillators, because it has to start
// making a noise the instant somebody presses play and it has nothing to load
// first. That is the right trade for playing along while you write and the
// wrong one for a recording somebody else will listen to, where what is
// wanted is a flute that sounds like a flute.
//
// A SoundFont is how that is done: a file of recorded notes with the rules
// for stretching them across the keyboard, which is what every hardware
// sampler has done since the 1990s and what the General MIDI programs the
// score already carries were written to index.
//
// Playing one is not this program's expertise, so it is not this program's
// code: MeltySynth does it, and does it the same way FluidSynth does. What
// is here is everything either side of that — which part plays through which
// soundfont, when every note starts and stops, and how the result is mixed.
package synth

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/sinshu/go-meltysynth/meltysynth"

	"github.com/stricaud/gscore/internal/midi"
)

// drumBank is the bank number a soundfont files its drum kits under. A kit
// is not a program among the others: the same patch number means a snare in
// one bank and a harpsichord in the other.
const drumBank = 128

// Font is a soundfont, read once and played by as many parts as point at it.
type Font struct {
	Path string
	Name string
	sf   *meltysynth.SoundFont
}

// Preset is one sound a font can make.
type Preset struct {
	Name    string `json:"name"`
	Bank    int    `json:"bank"`
	Program int    `json:"program"`
}

// LoadFont reads a soundfont from disk.
func LoadFont(path string) (*Font, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sf, err := meltysynth.NewSoundFont(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	name := sf.Info.BankName
	if name == "" {
		name = path
	}
	return &Font{Path: path, Name: name, sf: sf}, nil
}

// Presets lists what the font can play, in bank and program order.
func (f *Font) Presets() []Preset {
	out := make([]Preset, 0, len(f.sf.Presets))
	for _, p := range f.sf.Presets {
		out = append(out, Preset{
			Name: p.Name, Bank: int(p.BankNumber), Program: int(p.PatchNumber),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bank != out[j].Bank {
			return out[i].Bank < out[j].Bank
		}
		return out[i].Program < out[j].Program
	})
	return out
}

// Lookup finds the sound a part will actually be played with.
//
// A font is not obliged to carry all 128 programs, and most of the small ones
// do not. What happens then is decided here rather than discovered when the
// recording comes out silent: the answer is the name of what will sound, or
// nothing, which is what the interface reports as missing.
func (f *Font) Lookup(program int, drum bool) (Preset, bool) {
	bank := 0
	if drum {
		bank, program = drumBank, 0
	}
	for _, p := range f.sf.Presets {
		if int(p.BankNumber) == bank && int(p.PatchNumber) == program {
			return Preset{Name: p.Name, Bank: bank, Program: program}, true
		}
	}
	return Preset{}, false
}

/*
 * Rendering
 */

// Options are the knobs on the recording itself.
type Options struct {
	// SampleRate in hertz. 44100 unless there is a reason.
	SampleRate int
	// Gain multiplies the mix before it is written, 1 for as it came out.
	Gain float64
	// Tail is how many seconds of room to leave after the last note ends,
	// so that a hall's reverb and a piano's decay are recorded rather than
	// cut off mid-ring.
	Tail float64
	// Reverb turns on the reverb and chorus the soundfont asks for. It is
	// what a soundfont is voiced to be heard through, so it is on unless
	// somebody wants the dry samples.
	Reverb bool
	// Normalize brings the loudest moment of the recording up to just under
	// full scale.
	//
	// A score for one instrument played at an ordinary dynamic comes out
	// around a tenth of full scale, which is correct and sounds like a
	// mistake: beside anything else on a phone or in a playlist it is
	// inaudible, and turning it up is the listener's problem rather than
	// ours. One number multiplies the whole recording, so every dynamic in
	// the piece stays exactly as far from every other.
	Normalize bool
}

// DefaultOptions is a CD-quality recording with the room left on the end.
func DefaultOptions() Options {
	return Options{SampleRate: 44100, Gain: 1, Tail: 2.5, Reverb: true, Normalize: true}
}

// Render plays a performance and returns it as interleaved stereo samples.
//
// fonts says which part is played through which soundfont, by part index;
// anything not named there is played through fallback. Parts assigned to
// different fonts are rendered separately and mixed, which is the only way
// to have a piano from one library and strings from another in one recording.
func Render(song midi.Song, fonts map[int]*Font, fallback *Font, opt Options) ([]int16, error) {
	if opt.SampleRate <= 0 {
		opt.SampleRate = 44100
	}
	if opt.Gain == 0 {
		opt.Gain = 1
	}
	if fallback == nil {
		return nil, errors.New("synth: no soundfont to play this with")
	}
	if len(song.Notes) == 0 {
		return nil, errors.New("synth: there is nothing to play")
	}

	end := 0.0
	for _, n := range song.Notes {
		if stop := n.Start + n.Dur; stop > end {
			end = stop
		}
	}
	frames := int((end+opt.Tail)*float64(opt.SampleRate)) + 1

	// One group per font, because one synthesizer plays one soundfont.
	groups := map[*Font][]midi.Note{}
	for _, n := range song.Notes {
		f := fallback
		if assigned, ok := fonts[n.Part]; ok && assigned != nil {
			f = assigned
		}
		groups[f] = append(groups[f], n)
	}

	left := make([]float64, frames)
	right := make([]float64, frames)
	for f, notes := range groups {
		if err := play(f, notes, left, right, opt); err != nil {
			return nil, err
		}
	}
	return mixdown(left, right, opt.Gain, opt.Normalize), nil
}

// play renders one font's share of the music into the mix.
func play(f *Font, notes []midi.Note, left, right []float64, opt Options) error {
	settings := meltysynth.NewSynthesizerSettings(int32(opt.SampleRate))
	settings.EnableReverbAndChorus = opt.Reverb
	// A score can have more going on at once than a synthesizer's default
	// allowance: a held pedal under a chord in every part, plus the tails of
	// what came before. Notes over the limit are stolen, and a stolen note
	// stops in the middle, which is heard.
	settings.MaximumPolyphony = 256
	sy, err := meltysynth.NewSynthesizer(f.sf, settings)
	if err != nil {
		return err
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
		if drum {
			// The percussion channel is already a drum channel in the
			// synthesizer; everywhere else the bank has to be asked for.
			if ch != 9 {
				sy.ProcessMidiMessage(int32(ch), 0xB0, 0, drumBank)
			}
		}
		sy.ProcessMidiMessage(int32(ch), 0xC0, int32(program), 0)
	}

	// Everything that happens, in the order it happens. Stopping a note is
	// an event like starting one, and the two interleave: a long note is
	// still sounding while several short ones come and go.
	type event struct {
		at  int
		on  bool
		ch  int
		key int
		vel int
		seq int
	}
	events := make([]event, 0, 2*len(notes))
	for i, n := range notes {
		ch := channels[n.Part]
		gate := n.Gate
		if gate <= 0 {
			gate = 1
		}
		start := int(n.Start * float64(opt.SampleRate))
		stop := int((n.Start + n.Dur*gate) * float64(opt.SampleRate))
		if stop <= start {
			stop = start + 1
		}
		// A drum's key is which drum it is, not a pitch.
		key := n.MIDI
		if n.Drum > 0 {
			key = n.Drum
		}
		events = append(events,
			event{at: start, on: true, ch: ch, key: key, vel: n.Velocity, seq: i},
			event{at: stop, ch: ch, key: key, seq: i})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].at != events[j].at {
			return events[i].at < events[j].at
		}
		// At the same instant, release before striking: a repeated note has
		// to be let go before it is played again, or the second one is not
		// heard at all.
		return !events[i].on && events[j].on
	})

	buf := struct{ l, r []float32 }{
		make([]float32, len(left)), make([]float32, len(right)),
	}
	at := 0
	render := func(to int) {
		if to > len(buf.l) {
			to = len(buf.l)
		}
		if to > at {
			sy.Render(buf.l[at:to], buf.r[at:to])
			at = to
		}
	}
	for _, e := range events {
		render(e.at)
		if e.at >= len(buf.l) {
			break
		}
		if e.on {
			vel := e.vel
			if vel <= 0 {
				vel = 80
			}
			sy.NoteOn(int32(e.ch), int32(e.key), int32(vel))
		} else {
			sy.NoteOff(int32(e.ch), int32(e.key))
		}
	}
	render(len(buf.l))

	for i := range left {
		left[i] += float64(buf.l[i])
		right[i] += float64(buf.r[i])
	}
	return nil
}

// channelsFor gives every part a MIDI channel of its own.
//
// Sixteen is all there are, and the tenth is the drum channel whether anybody
// wants it or not, so a score with more parts than that has some of them
// sharing. Sharing is survivable — the parts have the same program far more
// often than not — and running out is not.
func channelsFor(notes []midi.Note) map[int]int {
	parts := map[int]bool{}
	var order []int
	drums := map[int]bool{}
	for _, n := range notes {
		if !parts[n.Part] {
			parts[n.Part] = true
			order = append(order, n.Part)
		}
		if n.Drum > 0 {
			drums[n.Part] = true
		}
	}
	sort.Ints(order)

	out := make(map[int]int, len(order))
	next := 0
	for _, p := range order {
		if drums[p] {
			out[p] = 9
			continue
		}
		if next == 9 {
			next++
		}
		out[p] = next % 16
		next++
	}
	return out
}

// fullScale is where the loudest moment of a recording is put: a hair under
// the top, which is where anything else you would play after it sits.
const fullScale = 0.89

// mixdown turns the mix into samples, at a sensible level and without
// clipping.
//
// Several parts summed can pass full scale even where none of them did, and
// a sample that passes full scale wraps round to the opposite extreme and
// cracks. Rather than clamp — which distorts the loudest moment, the one
// most likely to be listened to — the whole recording is scaled, which is
// what a mastering engineer would do with the fader.
func mixdown(left, right []float64, gain float64, normalize bool) []int16 {
	peak := 0.0
	for i := range left {
		peak = math.Max(peak, math.Abs(left[i]*gain))
		peak = math.Max(peak, math.Abs(right[i]*gain))
	}
	scale := gain
	switch {
	case peak == 0:
	case normalize:
		scale = gain * fullScale / peak
	case peak > 0.99:
		scale = gain * 0.99 / peak
	}

	out := make([]int16, 2*len(left))
	for i := range left {
		out[2*i] = sample(left[i] * scale)
		out[2*i+1] = sample(right[i] * scale)
	}
	return out
}

func sample(v float64) int16 {
	n := math.Round(v * 32767)
	if n > 32767 {
		n = 32767
	}
	if n < -32768 {
		n = -32768
	}
	return int16(n)
}
