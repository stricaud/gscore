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

// Lookup finds the sound a part will actually be played with, and says
// whether it is the one that was asked for.
//
// A font is not obliged to carry all 128 programs and most are nowhere near
// it: half the soundfonts in the world are one instrument, recorded by one
// person, filed at whichever program number they felt like. A violin
// soundfont whose single preset sits at program 0 is not a soundfont without
// a violin in it — but asking it for program 40, which is what General MIDI
// calls a violin, finds nothing.
//
// So an exact match wins, and failing that the font's first preset is used:
// if somebody points a staff at "Dark Violins.sf2" they mean to hear dark
// violins, whatever number is written inside the file. What they do not mean
// is silence, which is what a strict reading gives them. The second return
// says which of the two happened, so that the dialog can show a substitution
// as a substitution rather than quietly making one.
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
	if presets := f.Presets(); len(presets) > 0 {
		return presets[0], false
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
	if len(song.Notes) == 0 {
		return nil, errors.New("synth: there is nothing to play")
	}
	st, err := NewStream(song, fonts, fallback, opt, 0)
	if err != nil {
		return nil, err
	}
	// Rendered whole, because a recording is weighed before it is written:
	// how loud the loudest moment turned out is not known until the last
	// one has been played.
	frames := int(st.Left()*float64(st.SampleRate())) + 1
	left := make([]float64, frames)
	right := make([]float64, frames)
	at := 0
	for at < frames {
		n := st.read(left[at:], right[at:])
		if n == 0 {
			break
		}
		at += n
	}
	return mixdown(left[:at], right[:at], opt.Gain, opt.Normalize), nil
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
