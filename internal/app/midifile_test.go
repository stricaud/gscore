/*
 * midifile_test.go
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
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

// What comes out has to be the performance, not the notation.
//
// A MIDI file is read by things that make a noise, so the questions it has
// to answer are a player's: how long is this note actually held, how hard is
// it struck, and when. Slurs and dynamics are the two that say most, and
// they are what the file is checked for here — by reading the bytes back,
// because a writer that is checked against its own idea of itself is not
// checked at all.

// played is one note as the file says to play it.
type played struct {
	ch, key, vel int
	on, off      int // ticks
}

type midiFile struct {
	division int
	tempos   []struct{ tick, usPerQuarter int }
	notes    []played
}

// readMIDI takes the file apart far enough to ask what it says.
func readMIDI(t *testing.T, b []byte) midiFile {
	t.Helper()
	if string(b[:4]) != "MThd" {
		t.Fatalf("not a MIDI file: %q", b[:4])
	}
	format := binary.BigEndian.Uint16(b[8:10])
	tracks := int(binary.BigEndian.Uint16(b[10:12]))
	out := midiFile{division: int(binary.BigEndian.Uint16(b[12:14]))}
	if format != 1 {
		t.Errorf("format %d, want 1 — one track a staff", format)
	}

	at := 14
	for n := 0; n < tracks; n++ {
		if string(b[at:at+4]) != "MTrk" {
			t.Fatalf("track %d does not start with MTrk", n)
		}
		length := int(binary.BigEndian.Uint32(b[at+4 : at+8]))
		body := b[at+8 : at+8+length]
		at += 8 + length

		tick, i := 0, 0
		open := map[int]int{} // channel<<8|key -> on tick
		readVar := func() int {
			v := 0
			for {
				c := body[i]
				i++
				v = v<<7 | int(c&0x7F)
				if c&0x80 == 0 {
					return v
				}
			}
		}
		for i < len(body) {
			tick += readVar()
			status := body[i]
			switch {
			case status == 0xFF:
				kind := body[i+1]
				i += 2
				size := readVar()
				if kind == 0x51 {
					us := int(body[i])<<16 | int(body[i+1])<<8 | int(body[i+2])
					out.tempos = append(out.tempos,
						struct{ tick, usPerQuarter int }{tick, us})
				}
				i += size
			case status&0xF0 == 0x90:
				ch, key, vel := int(status&0x0F), int(body[i+1]), int(body[i+2])
				i += 3
				id := ch<<8 | key
				if vel > 0 {
					open[id] = tick
					out.notes = append(out.notes,
						played{ch: ch, key: key, vel: vel, on: tick, off: -1})
					continue
				}
				// A note off, written as a strike of no force.
				for k := len(out.notes) - 1; k >= 0; k-- {
					if out.notes[k].ch == ch && out.notes[k].key == key &&
						out.notes[k].off < 0 {
						out.notes[k].off = tick
						break
					}
				}
			case status&0xF0 == 0xC0:
				i += 2
			case status&0xF0 == 0xB0, status&0xF0 == 0xE0, status&0xF0 == 0x80,
				status&0xF0 == 0xA0:
				i += 3
			default:
				t.Fatalf("unexpected status %#x in track %d", status, n)
			}
		}
	}
	for _, n := range out.notes {
		if n.off < 0 {
			t.Errorf("the note %d on channel %d is never stopped", n.key, n.ch)
		}
	}
	return out
}

func midiOf(t *testing.T, s *model.Score) midiFile {
	t.Helper()
	var b bytes.Buffer
	if err := MIDIFile(&b, s); err != nil {
		t.Fatal(err)
	}
	return readMIDI(t, b.Bytes())
}

// phrase is a bar of four quarter notes, with marks put on them by hand.
func phrase(mark func(i int, n *model.Note)) *model.Score {
	p := &model.Part{Clef: model.Treble, Time: model.TimeSignature{Beats: 4, BeatValue: 4}}
	var els []model.Element
	for i := 0; i < 4; i++ {
		n := &model.Note{
			Pitch: model.Pitch{Step: model.Step(i), Octave: 4},
			Dur:   model.Duration{Num: 1, Den: 4},
		}
		if mark != nil {
			mark(i, n)
		}
		els = append(els, n)
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
	})
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
	s.Normalise()
	return s
}

// A slurred note runs into the next; an ordinary one stops before it. That
// difference is the whole of what a slur asks a player for, and a file that
// does not carry it is a file of notes with the phrasing thrown away.
func TestASlurredNoteRunsIntoTheNext(t *testing.T) {
	plain := midiOf(t, phrase(nil))
	slurred := midiOf(t, phrase(func(i int, n *model.Note) {
		n.SlurStart = i == 0
		n.SlurStop = i == 3
	}))

	if len(plain.notes) != 4 || len(slurred.notes) != 4 {
		t.Fatalf("%d and %d notes", len(plain.notes), len(slurred.notes))
	}
	for i := 0; i < 3; i++ {
		gap := plain.notes[i].off - plain.notes[i+1].on
		if gap >= 0 {
			t.Errorf("ordinary note %d is not detached: it stops %d ticks "+
				"after the next begins", i, gap)
		}
		over := slurred.notes[i].off - slurred.notes[i+1].on
		if over <= 0 {
			t.Errorf("slurred note %d stops %d ticks before the next begins; "+
				"it should run into it", i, -over)
		}
	}
	// The last note of a slur is not slurred to anything, so it is let go.
	last := slurred.notes[3]
	if last.off-last.on >= plain.division*2 {
		t.Error("the note the slur ends on is held past its length")
	}
}

// A dot over a note is a short note and the silence after it. It has to be
// shorter than an ordinary one, and an ordinary one shorter than a slurred.
func TestStaccatoIsShortAndTenutoIsNot(t *testing.T) {
	held := func(f *model.Note) {}
	_ = held

	length := func(mark func(*model.Note)) int {
		m := midiOf(t, phrase(func(i int, n *model.Note) {
			if i == 1 && mark != nil {
				mark(n)
			}
		}))
		return m.notes[1].off - m.notes[1].on
	}

	staccato := length(func(n *model.Note) {
		n.Articulations = []model.Articulation{model.Staccato}
	})
	plain := length(nil)
	tenuto := length(func(n *model.Note) {
		n.Articulations = []model.Articulation{model.Tenuto}
	})

	if !(staccato < plain && plain < tenuto) {
		t.Errorf("staccato %d, plain %d, tenuto %d ticks: they should run "+
			"short, middling and full", staccato, plain, tenuto)
	}
	// And a staccato quarter still occupies its beat: the next note is not
	// brought forward.
	m := midiOf(t, phrase(func(i int, n *model.Note) {
		if i == 1 {
			n.Articulations = []model.Articulation{model.Staccato}
		}
	}))
	if got, want := m.notes[2].on, 2*m.division; got != want {
		t.Errorf("the note after a staccato starts at tick %d, want %d",
			got, want)
	}
}

// How hard each note is struck is the dynamic in force, and an accent is
// louder than its neighbours.
func TestDynamicsReachTheFile(t *testing.T) {
	m := midiOf(t, phrase(func(i int, n *model.Note) {
		switch i {
		case 0:
			n.Dynamic = model.PP
		case 2:
			n.Dynamic = model.Forte
		case 3:
			n.Articulations = []model.Articulation{model.Accent}
		}
	}))
	if len(m.notes) != 4 {
		t.Fatalf("%d notes", len(m.notes))
	}
	if m.notes[0].vel >= m.notes[2].vel {
		t.Errorf("pianissimo came out at %d and forte at %d",
			m.notes[0].vel, m.notes[2].vel)
	}
	if m.notes[1].vel != m.notes[0].vel {
		t.Errorf("the dynamic did not hold across notes: %d then %d",
			m.notes[0].vel, m.notes[1].vel)
	}
	if m.notes[3].vel <= m.notes[2].vel {
		t.Errorf("the accented note is %d against %d: it should be louder",
			m.notes[3].vel, m.notes[2].vel)
	}
	for _, n := range m.notes {
		if n.vel < 1 || n.vel > 127 {
			t.Errorf("a velocity of %d is not a velocity", n.vel)
		}
	}
}

// A hairpin is a change of volume over a stretch of music, so the notes
// under it come out rising.
func TestAHairpinRises(t *testing.T) {
	m := midiOf(t, phrase(func(i int, n *model.Note) {
		if i == 0 {
			n.Dynamic = model.Piano
			n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Start: true}
		}
		if i == 3 {
			n.Hairpin = &model.Hairpin{Kind: model.Crescendo, Stop: true}
		}
	}))
	if m.notes[0].vel >= m.notes[3].vel {
		t.Errorf("the crescendo runs %d → %d", m.notes[0].vel, m.notes[3].vel)
	}
}

// The tempo is written down, and written again wherever it changes — or
// everything that reads the file invents one.
func TestTheTempoIsWrittenAndFollowed(t *testing.T) {
	s := phrase(nil)
	for i := 0; i < 3; i++ {
		s.Parts[0].Measures = append(s.Parts[0].Measures,
			cloneMeasure(s.Parts[0].Measures[0]))
	}
	s.Parts[0].Measures[2].Tempo = &model.Tempo{
		BPM: 60, Unit: model.Duration{Num: 1, Den: 4},
	}
	m := midiOf(t, s)

	if len(m.tempos) != 2 {
		t.Fatalf("%d tempo events, want the one at the head and the change",
			len(m.tempos))
	}
	bpm := func(us int) float64 { return 60000000 / float64(us) }
	if got := bpm(m.tempos[0].usPerQuarter); got < 119 || got > 121 {
		t.Errorf("the score starts at %.1f bpm, want 120", got)
	}
	if got := bpm(m.tempos[1].usPerQuarter); got < 59 || got > 61 {
		t.Errorf("it changes to %.1f bpm, want 60", got)
	}
	// And the change is written where it happens: the third bar.
	if want := 2 * 4 * m.division; m.tempos[1].tick != want {
		t.Errorf("the change is at tick %d, want %d", m.tempos[1].tick, want)
	}
}

// Drums go to channel ten, where every General MIDI device keeps them, and
// what sounds is the drum rather than the staff position.
func TestDrumsGoToTheDrumChannel(t *testing.T) {
	p := &model.Part{
		Clef: model.Percussion, Instrument: model.Instrument(0),
		Time: model.TimeSignature{Beats: 4, BeatValue: 4},
	}
	var els []model.Element
	for i := 0; i < 4; i++ {
		els = append(els, &model.Note{
			Pitch:      model.Pitch{Step: model.StepC, Octave: 5},
			Dur:        model.Duration{Num: 1, Den: 4},
			Percussion: 38, // the snare
		})
	}
	p.Measures = append(p.Measures, &model.Measure{
		Voices: []*model.Voice{{Elements: els}}, RightBarline: model.BarSingle,
	})
	s := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
		Parts: []*model.Part{p},
	}
	s.Normalise()

	m := midiOf(t, s)
	for i, n := range m.notes {
		if n.ch != 9 {
			t.Errorf("drum %d is on channel %d, want 10", i, n.ch+1)
		}
		if n.key != 38 {
			t.Errorf("drum %d sounds key %d, want the snare at 38", i, n.key)
		}
	}
}

// A tie is one sound, not two struck notes — which is what it means.
func TestATieIsOneNote(t *testing.T) {
	s := phrase(func(i int, n *model.Note) {
		n.Pitch = model.Pitch{Step: model.StepG, Octave: 4}
		n.TieStart = i == 0
		n.TieStop = i == 1
	})
	m := midiOf(t, s)
	if len(m.notes) != 3 {
		t.Fatalf("%d notes sounded, want 3: the tied pair is one", len(m.notes))
	}
	// One note of about two beats, not one of about one: what matters is
	// that the second quarter was never struck again. It is let go a
	// breath before the next note, as any note is.
	held := m.notes[0].off - m.notes[0].on
	if held < m.division*2-m.division/4 || held > m.division*2 {
		t.Errorf("the tied note is held %d ticks; two quarters is %d",
			held, m.division*2)
	}
}

// Every staff gets its own channel while there are channels, because two
// staves sharing one cannot hold the same note at the same time.
func TestEachStaffGetsItsOwnChannel(t *testing.T) {
	s := phrase(nil)
	for i := 0; i < 3; i++ {
		s.Parts = append(s.Parts, clonePart(s.Parts[0]))
	}
	s.Normalise()
	m := midiOf(t, s)

	seen := map[int]bool{}
	for _, n := range m.notes {
		seen[n.ch] = true
	}
	if len(seen) != 4 {
		t.Errorf("four staves came out on %d channels: %v", len(seen), seen)
	}
	if seen[9] {
		t.Error("a staff was given the drum channel")
	}
}

func cloneMeasure(m *model.Measure) *model.Measure {
	out := &model.Measure{RightBarline: m.RightBarline}
	for _, v := range m.Voices {
		nv := &model.Voice{}
		for _, el := range v.Elements {
			nv.Elements = append(nv.Elements, cloneElement(el))
		}
		out.Voices = append(out.Voices, nv)
	}
	return out
}

func clonePart(p *model.Part) *model.Part {
	out := &model.Part{Clef: p.Clef, Time: p.Time, Instrument: p.Instrument}
	for _, m := range p.Measures {
		out.Measures = append(out.Measures, cloneMeasure(m))
	}
	return out
}

var _ = fmt.Sprintf
