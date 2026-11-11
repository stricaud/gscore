/*
 * recording.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package recording turns a take into a sample.
//
// Somebody plays one note into a microphone. What comes back is a few
// seconds of room with a note somewhere in the middle of it: a moment of
// breath before the sound starts, the note, and a tail of nothing after it.
// What a soundfont wants is the note — knowing which note it is, where the
// steady part of it is, and where in that steady part it can be made to
// repeat so that a two-second recording holds a long note.
//
// Three questions, and this package answers them:
//
//   - which note is it. By the pitch, not by what somebody said they were
//     going to play, because a flute is not always where its player thinks
//     it is and a sample a quarter-tone out is worse than no sample.
//   - where does it start and stop. The silence at either end is thrown
//     away and the attack is kept: the first tenth of a second of a flute
//     note is the breath taking hold, and it is most of what makes a
//     listener hear a flute rather than a sine wave. A sample trimmed to
//     its steady part alone fades in from nowhere.
//   - and where can it loop. In the steady part, over a whole number of
//     periods, matched at the same point in the waveform.
//
// None of this writes a file; package sf2 does that. A Take is what goes
// into an sf2.Sample.
package recording

import (
	"errors"
	"math"
	"sort"
)

// Take is one recorded note, read.
type Take struct {
	// Data is the recording with the silence at either end taken off. The
	// attack is still at the front of it.
	Data []int16

	// Rate is what it was recorded at, unchanged.
	Rate int

	// Hz is the pitch that was heard, and Key is the MIDI key nearest to
	// it, with Cents saying how far off that key it is: -50 to 50. A
	// soundfont plays the sample shifted by the difference between the key
	// being played and Key, and corrects the rest with Cents, so a take
	// that is fifteen cents sharp plays in tune.
	Hz    float64
	Key   int
	Cents int

	// LoopStart and LoopEnd are the stretch of Data to repeat while a key
	// is held, LoopEnd being the frame after the last of it. Both zero
	// where no loop was found, and then a note is as long as the
	// recording.
	LoopStart, LoopEnd int

	// Peak is the loudest sample in the take, from 0 to 1, which is what
	// says whether the recording was made at a usable level.
	Peak float64
}

// Read works out everything that can be worked out about one recorded note.
//
// It never refuses a recording that has a sound in it. A take with no
// findable pitch — a breath, a key click, a cymbal — still comes back
// trimmed, with Hz zero and no loop, because somebody recording a
// percussion bank wants exactly that and a program that would not let them
// have it is a program that is wrong about music.
func Read(data []int16, rate int) (Take, error) {
	if rate <= 0 {
		return Take{}, errors.New("recording: a take has to say what rate " +
			"it was recorded at")
	}
	if len(data) == 0 {
		return Take{}, errors.New("recording: the take is empty")
	}

	peak := peakOf(data)
	if peak < silence {
		return Take{}, errors.New("recording: there is no sound in this " +
			"take — nothing was recorded, or the microphone was not listening")
	}

	from, to := Trim(data, rate)
	cut := data[from:to]
	out := Take{Data: cut, Rate: rate, Peak: peak}

	hz, ok := Pitch(cut, rate)
	if !ok {
		// Trimmed, and nothing else claimed about it.
		return out, nil
	}
	out.Hz = hz
	out.Key, out.Cents = Key(hz)

	if s, e, ok := Loop(cut, rate, hz); ok {
		out.LoopStart, out.LoopEnd = s, e
	}
	return out, nil
}

/* ---------- how loud ---------- */

// silence is the level below which a take is held to be empty, as a
// fraction of full scale.
//
// About -60 dB. A room with nobody in it records at around -70; a flute
// three feet from a microphone records at -10. Anything under this is not
// a quiet recording, it is a recording that did not happen.
const silence = 0.001

// trimFloor2 is how far below the loudest moment the sound is taken to
// have started and stopped, as a fraction of it.
//
// Measured on blocks of a few milliseconds rather than on single samples,
// so it is a fraction of the loudest block and not of the loudest sample:
// about -34 dB. It is only half the answer — the other half is the room,
// which is measured from the take itself. See Trim.
const trimFloor2 = 0.02

func peakOf(data []int16) float64 {
	m := 0
	for _, v := range data {
		a := int(v)
		if a < 0 {
			a = -a
		}
		if a > m {
			m = a
		}
	}
	return float64(m) / 32768
}

// Trim finds where the sound starts and stops, and hands back the stretch
// between them.
//
// The attack is kept. What is found is where the sound crosses a fortieth
// of its own loudest moment, and then the start is moved back by a few
// milliseconds and to the nearest crossing of zero: a sample that begins
// part way up a waveform begins with a click, and a sample that begins
// after the attack begins with no instrument on it.
// It is measured in blocks of a few milliseconds rather than sample by
// sample, and against the room rather than against nothing. A room is
// never silent: a quiet one records forty-five decibels below the note
// being played into it, which is exactly where a fixed threshold would
// sit, and then a take has silence at neither end and nothing is trimmed
// at all. So the quietest tenth of the take is taken to be the room, and
// the sound is what stands above it.
func Trim(data []int16, rate int) (from, to int) {
	if len(data) == 0 {
		return 0, 0
	}
	peak := peakOf(data)
	if peak < silence {
		return 0, len(data)
	}

	block := rate / 200 // five milliseconds
	if block < 1 {
		block = 1
	}
	n := len(data) / block
	if n < 4 {
		return 0, len(data)
	}
	level := make([]float64, n)
	for i := range level {
		var sum float64
		for _, v := range data[i*block : (i+1)*block] {
			sum += float64(v) * float64(v)
		}
		level[i] = math.Sqrt(sum / float64(block))
	}

	quiet := append([]float64(nil), level...)
	sort.Float64s(quiet)
	room := quiet[len(quiet)/10]
	loud := quiet[len(quiet)-1]

	// A take whose loudest moment is not much above its quietest has no
	// note in it to find the edges of — a cymbal, a rustle, a room.
	if loud < room*2 {
		return 0, len(data)
	}
	floor := math.Max(loud*trimFloor2, room*3)

	first, last := -1, -1
	for i, v := range level {
		if v >= floor {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return 0, len(data)
	}
	// And then back out over whatever is still above the room, so that a
	// slow attack is kept whole rather than cut at the threshold. Capped,
	// because past a certain point what is being kept is the room.
	const reach = 30 // a hundred and fifty milliseconds
	for back := 0; back < reach && first > 0 && level[first-1] > room*1.5; back++ {
		first--
	}
	for on := 0; on < reach && last < len(level)-1 && level[last+1] > room*1.5; on++ {
		last++
	}

	from, to = first*block, (last+1)*block
	if to > len(data) {
		to = len(data)
	}
	if from >= to {
		return 0, len(data)
	}

	// Back off, so that nothing of the attack is lost to the threshold
	// having been crossed a little late.
	from -= rate / 200 // five milliseconds
	if from < 0 {
		from = 0
	}
	from = zeroBefore(data, from)

	// And let the tail finish rather than cutting it off at the floor.
	to += rate / 100
	if to > len(data) {
		to = len(data)
	}
	return from, to
}

// zeroBefore walks back to the nearest place the waveform crosses zero, so
// that a sample does not begin part way up a wave.
func zeroBefore(data []int16, at int) int {
	for i := at; i > 0; i-- {
		if data[i-1] == 0 || (data[i-1] < 0) != (data[i] < 0) {
			return i
		}
		if at-i > 200 {
			break // not worth hunting further; the click is inaudible
		}
	}
	return at
}

/* ---------- which note ---------- */

// Pitch is what note the recording is of, in hertz.
//
// By the YIN difference function, which is autocorrelation turned upside
// down: instead of looking for the lag at which a sound most resembles
// itself, it looks for the lag at which it least differs from itself, and
// then divides by the average difference so far. That last step is the
// whole of why it is used here rather than plain autocorrelation — it is
// what stops a note being heard an octave too low, which plain
// autocorrelation does on anything with a strong second harmonic, and
// which on a flute would be most of them.
//
// Several windows are read across the sound and the middle answer is
// taken, so that one unlucky window — a breath, a finger, the end of the
// note — does not decide it.
func Pitch(data []int16, rate int) (float64, bool) {
	const window = 2048
	if len(data) < window*2 || rate <= 0 {
		return 0, false
	}
	// Not from the very start: the attack is where the pitch is least
	// settled, and it is the part this is worst at.
	begin := len(data) / 8
	span := len(data) - begin - window*2
	if span <= 0 {
		begin, span = 0, len(data)-window*2
	}
	if span <= 0 {
		return 0, false
	}

	var found []float64
	const tries = 8
	for i := 0; i < tries; i++ {
		at := begin + span*i/tries
		if hz, ok := yin(data[at:at+window*2], rate, window); ok {
			found = append(found, hz)
		}
	}
	if len(found) == 0 {
		return 0, false
	}
	sort.Float64s(found)
	hz := found[len(found)/2]

	// A pitch outside what any instrument plays is not a pitch: it is a
	// rumble in the room or a hiss in the microphone read as one.
	if hz < 20 || hz > 5000 {
		return 0, false
	}
	return hz, true
}

// yin reads one window. maxTau is how long a period it will consider,
// which sets the lowest note it can hear.
func yin(x []int16, rate, maxTau int) (float64, bool) {
	// How far from periodic a lag may be and still be taken as the answer.
	// A pure tone comes in under 0.01 and a breathy flute around 0.1;
	// above 0.2 there is no pitch there to find.
	const threshold = 0.15

	n := len(x) - maxTau
	if n <= 0 {
		return 0, false
	}
	d := make([]float64, maxTau)
	for tau := 1; tau < maxTau; tau++ {
		var sum float64
		for j := 0; j < n; j++ {
			diff := float64(x[j]) - float64(x[j+tau])
			sum += diff * diff
		}
		d[tau] = sum
	}

	// The cumulative mean normalised difference: each lag against the
	// average of every shorter one.
	norm := make([]float64, maxTau)
	norm[0] = 1
	var running float64
	for tau := 1; tau < maxTau; tau++ {
		running += d[tau]
		if running == 0 {
			norm[tau] = 1
			continue
		}
		norm[tau] = d[tau] * float64(tau) / running
	}

	// The first lag that dips under the threshold and is the bottom of its
	// own dip — the first, not the smallest, because the smallest is as
	// often as not the octave below it.
	best := -1
	for tau := 2; tau < maxTau-1; tau++ {
		if norm[tau] < threshold && norm[tau] <= norm[tau+1] {
			best = tau
			break
		}
	}
	if best < 0 {
		return 0, false
	}

	// And where between two samples the bottom of the dip actually lies,
	// by fitting a parabola to it: without this the answer is quantised to
	// whole samples, which at the top of a flute's range is a third of a
	// semitone.
	tau := float64(best)
	if best > 1 && best < maxTau-1 {
		a, b, c := norm[best-1], norm[best], norm[best+1]
		if den := 2 * (2*b - a - c); den != 0 {
			tau += (c - a) / den
		}
	}
	if tau <= 0 {
		return 0, false
	}
	return float64(rate) / tau, true
}

// Key is the MIDI key a pitch is nearest to, and how far off it is in
// cents.
//
// A4 is key 69 and 440 Hz, which is the tuning everything else is measured
// from. A take that comes back at 442 is not retuned — it is written down
// as key 69, eight cents sharp, and the synthesiser corrects it when it
// plays. Retuning the recording would mean resampling it, which is a
// quality lost for nothing.
func Key(hz float64) (key, cents int) {
	if hz <= 0 {
		return 0, 0
	}
	exact := 69 + 12*math.Log2(hz/440)
	key = int(math.Round(exact))
	if key < 0 {
		key = 0
	}
	if key > 127 {
		key = 127
	}
	cents = int(math.Round((exact - float64(key)) * 100))
	// Rounding to the nearest key means never more than half a semitone
	// out, which is the whole range the field has.
	if cents > 50 {
		cents = 50
	}
	if cents < -50 {
		cents = -50
	}
	return key, cents
}

/* ---------- where it loops ---------- */

// loopSeconds is how long a loop to look for.
//
// Long enough that the ear does not hear the repeat — a quarter of a second
// of flute is several cycles of whatever the breath is doing, and a loop
// much shorter than that buzzes. Short enough to fit inside the steady part
// of a two-second take with room either side.
const loopSeconds = 0.25

// Loop finds a stretch of the steady part of a recording that can be
// repeated without a click or a jump.
//
// Three things make a loop inaudible, and all three are here. It covers a
// whole number of periods, so the waveform arrives back where it started.
// It begins and ends at the same point in the cycle — the same rising
// crossing of zero — so there is no step. And of the lengths that satisfy
// both, it takes the one whose end least differs from its start over a
// couple of periods, which is what settles the rest: the vibrato, the
// breath noise, the slow swell of a held note.
//
// ok is false where there is nothing to loop: too short a recording, no
// pitch, or a sound that never settles.
func Loop(data []int16, rate int, hz float64) (start, end int, ok bool) {
	if hz <= 0 || rate <= 0 {
		return 0, 0, false
	}
	period := float64(rate) / hz
	if period < 2 {
		return 0, 0, false
	}

	// The steady part: past the attack, and stopping before the end so
	// that the decay is not looped.
	from := int(math.Max(float64(len(data))*0.25, 0.1*float64(rate)))
	until := len(data) - int(0.05*float64(rate))
	if until-from < int(4*period) {
		// A short recording: use what there is, from just past the attack.
		from = int(math.Min(float64(len(data))/4, 0.05*float64(rate)))
		until = len(data)
	}
	if until-from < int(4*period) {
		return 0, 0, false
	}

	// A whole number of periods, as near a quarter of a second as that
	// allows, and at least four.
	cycles := int(math.Round(loopSeconds * float64(rate) / period))
	if cycles < 4 {
		cycles = 4
	}
	for cycles > 4 && int(float64(cycles)*period) > until-from {
		cycles--
	}
	length := int(math.Round(float64(cycles) * period))
	if length < 2 || from+length >= until {
		return 0, 0, false
	}

	start = risingAfter(data, from, int(period))
	if start < 0 || start+length >= until {
		return 0, 0, false
	}

	// And then the end, hunted over a period either side of where a whole
	// number of cycles lands: the pitch is never exactly what was
	// measured, so the arithmetic puts it close and the waveform itself
	// says where exactly.
	window := int(2 * period)
	if start+length+window > len(data) {
		window = len(data) - start - length
	}
	if window < int(period) {
		return 0, 0, false
	}

	bestAt, bestCost := -1, math.MaxFloat64
	lo := start + length - int(period/2)
	hi := start + length + int(period/2)
	if lo < start+2 {
		lo = start + 2
	}
	for e := lo; e <= hi && e+window <= len(data); e++ {
		cost := difference(data, start, e, window)
		if cost < bestCost {
			bestAt, bestCost = e, cost
		}
	}
	if bestAt < 0 {
		return 0, 0, false
	}

	// A loop whose ends do not actually match is worse than none: it
	// repeats a click for as long as the note is held, and a note held
	// without a loop merely stops.
	if bestCost > loopTolerance {
		return 0, 0, false
	}
	return start, bestAt, true
}

// loopTolerance is how different the two ends of a loop may be, as a
// fraction of the loudness there.
//
// A tenth. Below that the join is inaudible under the sound itself; above
// it there is a bump on every repeat, and the honest answer is that this
// recording has no loop in it.
const loopTolerance = 0.1

// difference is how unlike the sound at b is to the sound at a, over n
// frames, as a fraction of how loud it is there.
//
// Relative, because what matters is whether the step at the join can be
// heard against the sound it is in, and the same absolute step is nothing
// in a loud note and a click in a quiet one.
func difference(data []int16, a, b, n int) float64 {
	var diff, level float64
	for i := 0; i < n; i++ {
		x := float64(data[a+i])
		y := float64(data[b+i])
		diff += (x - y) * (x - y)
		level += x * x
	}
	if level == 0 {
		return math.MaxFloat64
	}
	return math.Sqrt(diff / level)
}

// risingAfter finds the next place the waveform crosses zero going upwards,
// which is one definite point in a cycle and so a place two ends of a loop
// can be made to agree about.
func risingAfter(data []int16, at, within int) int {
	if at < 1 {
		at = 1
	}
	for i := at; i < len(data) && i < at+within*2; i++ {
		if data[i-1] <= 0 && data[i] > 0 {
			return i
		}
	}
	return -1
}
