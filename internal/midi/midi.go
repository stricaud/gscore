/*
 * midi.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

// Package midi writes a performance as a standard MIDI file.
//
// It is handed notes that have already been resolved — when each sounds, how
// long it is held, how hard it is struck, which instrument plays it — and
// turns them into the bytes a sequencer or a soundfont renderer reads.
// Working out what a score sounds like is the program's job and happens in
// one place; this is one of the things that reads the answer, the browser's
// player being another.
//
// What it is for: handing the music to something that makes a better noise
// than a web page can. A soundfont renderer takes a MIDI file and gives back
// audio, so this is the first step to an exported recording.
package midi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

// ticksPerQuarter is the time grid the file is written on.
//
// 480 divides by 2, 3, 4, 5, 6 and 8, so triplets, quintuplets and
// sixty-fourth notes all land on whole numbers. It is what every sequencer
// writes and nothing has trouble reading.
const ticksPerQuarter = 480

// Note is one sound to be written.
type Note struct {
	// Start and Dur are in seconds, as the player's own clock measures
	// them: this package turns them into ticks using the bars below.
	Start, Dur float64
	// Gate is how much of Dur the note is actually held for. Below one it
	// is detached from what follows; above one it runs into it, which is
	// what a slur asks for.
	Gate     float64
	MIDI     int
	Velocity int
	// Part is which staff played it, and Program its General MIDI
	// instrument. Drum, when set, is a percussion key and the note goes to
	// the drum channel instead.
	Part    int
	Program int
	Drum    int
}

// Bar is a measure of the performance: when it begins, how long it lasts,
// and how much written music is in it.
//
// Both are needed. The seconds say where the notes are; the beats say what
// the tempo is, and a MIDI file has to carry a tempo or everything that
// reads it will invent one.
type Bar struct {
	Start, Dur float64 // seconds
	Quarters   float64 // how many quarter notes the bar holds
}

// Song is everything the file needs.
type Song struct {
	Name  string
	Notes []Note
	Bars  []Bar
}

// Write writes the song as a format 1 standard MIDI file.
//
// Format 1 is several tracks played together: one per staff, which is what
// lets a sequencer show the score as it was written rather than as one
// undifferentiated stream. The first track carries the tempo and nothing
// else, as the format intends.
func Write(w io.Writer, s Song) error {
	clock := newClock(s.Bars)

	var tracks [][]byte
	head, err := tempoTrack(s, clock)
	if err != nil {
		return err
	}
	tracks = append(tracks, head)

	for _, part := range partsOf(s.Notes) {
		t, err := partTrack(s, part, clock)
		if err != nil {
			return err
		}
		tracks = append(tracks, t)
	}

	var b bytes.Buffer
	b.WriteString("MThd")
	_ = binary.Write(&b, binary.BigEndian, uint32(6))
	_ = binary.Write(&b, binary.BigEndian, uint16(1)) // format 1
	_ = binary.Write(&b, binary.BigEndian, uint16(len(tracks)))
	_ = binary.Write(&b, binary.BigEndian, uint16(ticksPerQuarter))
	for _, t := range tracks {
		b.WriteString("MTrk")
		_ = binary.Write(&b, binary.BigEndian, uint32(len(t)))
		b.Write(t)
	}
	_, err = w.Write(b.Bytes())
	return err
}

/*
 * Time
 */

// clock turns seconds into ticks.
//
// A performance is held in seconds because that is what playing it means.
// A MIDI file is in ticks, and ticks are musical: how long one lasts is
// whatever the tempo in force says. So the two are reconciled bar by bar —
// each bar is so many quarter notes and took so many seconds, which gives
// both the tempo to write and the mapping for the notes inside it.
//
// Within a bar it is linear, which is exactly how time passes there.
type clock struct {
	bars  []Bar
	ticks []int // the tick each bar begins on
}

func newClock(bars []Bar) *clock {
	c := &clock{bars: bars, ticks: make([]int, len(bars)+1)}
	at := 0
	for i, b := range bars {
		c.ticks[i] = at
		at += int(math.Round(b.Quarters * ticksPerQuarter))
	}
	if len(bars) > 0 {
		c.ticks[len(bars)] = at
	}
	return c
}

// at is the tick a moment falls on.
func (c *clock) at(seconds float64) int {
	if len(c.bars) == 0 {
		// Nothing to measure against: a quarter note a second, which is
		// something rather than a division by zero.
		return int(math.Round(seconds * ticksPerQuarter))
	}
	for i, b := range c.bars {
		if seconds >= b.Start+b.Dur && i < len(c.bars)-1 {
			continue
		}
		part := 0.0
		if b.Dur > 0 {
			part = (seconds - b.Start) / b.Dur
		}
		span := float64(c.ticks[i+1] - c.ticks[i])
		return c.ticks[i] + int(math.Round(part*span))
	}
	return c.ticks[len(c.bars)]
}

// tempoTrack is the first track: the tempo in force at each bar, and
// nothing else.
func tempoTrack(s Song, c *clock) ([]byte, error) {
	var ev []event
	if s.Name != "" {
		ev = append(ev, event{tick: 0, data: meta(0x03, []byte(s.Name))})
	}
	last := -1
	for i, b := range s.Bars {
		if b.Dur <= 0 || b.Quarters <= 0 {
			continue
		}
		// Microseconds to the quarter note, which is how MIDI says tempo.
		us := int(math.Round(b.Dur / b.Quarters * 1e6))
		if us == last {
			continue // the speed has not changed, so nothing is written
		}
		last = us
		ev = append(ev, event{
			tick: c.ticks[i],
			data: meta(0x51, []byte{byte(us >> 16), byte(us >> 8), byte(us)}),
		})
	}
	if len(ev) == 0 {
		ev = append(ev, event{tick: 0, data: meta(0x51, []byte{0x07, 0xA1, 0x20})})
	}
	return track(ev)
}

/*
 * Notes
 */

// partsOf lists the staves that play anything, in order.
func partsOf(notes []Note) []int {
	seen := map[int]bool{}
	var out []int
	for _, n := range notes {
		if !seen[n.Part] {
			seen[n.Part] = true
			out = append(out, n.Part)
		}
	}
	sort.Ints(out)
	return out
}

// drumChannel is channel 10, counted from zero, which is where every
// General MIDI device keeps its percussion.
const drumChannel = 9

// channelFor hands out a channel to each staff.
//
// One each while there are any to give, because two staves on one channel
// cannot hold the same note at the same time — the second note-off stops
// both. Past fifteen staves they are shared, which is the best a format
// with sixteen channels can do.
func channelFor(parts []int, part int) int {
	ch := 0
	for _, p := range parts {
		if ch == drumChannel {
			ch++ // kept for the drums
		}
		if p == part {
			break
		}
		ch++
	}
	if ch > 15 {
		ch = ch%15 + 1 // wrap, skipping nothing: better shared than silent
	}
	return ch
}

func partTrack(s Song, part int, c *clock) ([]byte, error) {
	parts := partsOf(s.Notes)
	ch := channelFor(parts, part)

	var ev []event
	program := -1
	for _, n := range s.Notes {
		if n.Part != part {
			continue
		}
		on, off := ch, n.MIDI
		if n.Drum > 0 {
			on, off = drumChannel, n.Drum
		} else if n.Program != program {
			program = n.Program
			ev = append(ev, event{
				tick: c.at(n.Start),
				data: []byte{byte(0xC0 | ch), byte(n.Program & 0x7F)},
			})
		}

		gate := n.Gate
		if gate <= 0 {
			gate = 1
		}
		start := c.at(n.Start)
		end := c.at(n.Start + n.Dur*gate)
		if end <= start {
			end = start + 1 // nothing lasts no time at all
		}
		vel := n.Velocity
		if vel <= 0 {
			vel = 80
		}
		if vel > 127 {
			vel = 127
		}
		ev = append(ev,
			event{tick: start, data: []byte{byte(0x90 | on), byte(off & 0x7F), byte(vel)}},
			// A note-off as a note-on of no force: what every sequencer
			// writes, because it runs in the same status byte as the note
			// before it.
			event{tick: end, order: 1, data: []byte{byte(0x90 | on), byte(off & 0x7F), 0}})
	}
	return track(ev)
}

/*
 * The bytes
 */

type event struct {
	tick int
	// order separates what falls on the same tick: a note ending there is
	// written before one beginning, so that a repeated note is stopped
	// before it is struck again rather than after.
	order int
	data  []byte
}

func track(ev []event) ([]byte, error) {
	sort.SliceStable(ev, func(i, j int) bool {
		if ev[i].tick != ev[j].tick {
			return ev[i].tick < ev[j].tick
		}
		return ev[i].order > ev[j].order
	})

	var b bytes.Buffer
	at := 0
	for _, e := range ev {
		if e.tick < at {
			return nil, fmt.Errorf("midi: an event at %d follows one at %d", e.tick, at)
		}
		writeVar(&b, e.tick-at)
		b.Write(e.data)
		at = e.tick
	}
	writeVar(&b, 0)
	b.Write(meta(0x2F, nil)) // end of track
	return b.Bytes(), nil
}

func meta(kind byte, data []byte) []byte {
	var b bytes.Buffer
	b.WriteByte(0xFF)
	b.WriteByte(kind)
	writeVar(&b, len(data))
	b.Write(data)
	return b.Bytes()
}

// writeVar writes MIDI's variable-length number: seven bits at a time, the
// top bit set on every byte but the last.
func writeVar(b *bytes.Buffer, n int) {
	if n < 0 {
		n = 0
	}
	var out [5]byte
	i := len(out) - 1
	out[i] = byte(n & 0x7F)
	for n >>= 7; n > 0; n >>= 7 {
		i--
		out[i] = byte(n&0x7F) | 0x80
	}
	b.Write(out[i:])
}
