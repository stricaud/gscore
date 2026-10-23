/*
 * nwc1.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 */

package nwc

import (
	"encoding/binary"
	"fmt"

	"github.com/stricaud/gscore/internal/model"
)

// Reading the NoteWorthy 1.7x binary score.
//
// The walk follows fileformat.posa/nwc1.posa exactly: the layout there was
// derived from 26 files and checked by consuming a 1.75 file to its last byte.
// Everything structural — the header, the font table, the staff records, the
// object stream, the lyric pages and the "a staff claims two more objects than
// it stores" quirk — comes from that description.
//
// What the description marks as `hex` without naming the values is the handful
// of fields that carry musical meaning: the duration byte, the accidental
// byte, and how the position is signed. Those are decoded here from the
// mappings that NWCTXT implies, and an unrecognised value is an error naming
// the byte rather than a guess, because a wrong duration silently rewrites the
// music. One real file is enough to settle any that are wrong.

type walker struct {
	b []byte
	i int
}

func (w *walker) remaining() int { return len(w.b) - w.i }

func (w *walker) need(n int) error {
	if w.i+n > len(w.b) {
		return fmt.Errorf("nwc: the file ends early, %d bytes short", w.i+n-len(w.b))
	}
	return nil
}

func (w *walker) u8() (byte, error) {
	if err := w.need(1); err != nil {
		return 0, err
	}
	c := w.b[w.i]
	w.i++
	return c, nil
}

func (w *walker) u16() (int, error) {
	if err := w.need(2); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint16(w.b[w.i:])
	w.i += 2
	return int(v), nil
}

func (w *walker) skip(n int) error {
	if err := w.need(n); err != nil {
		return err
	}
	w.i += n
	return nil
}

func (w *walker) cstring() (string, error) {
	for j := w.i; j < len(w.b); j++ {
		if w.b[j] == 0 {
			s := string(w.b[w.i:j])
			w.i = j + 1
			return s, nil
		}
	}
	return "", fmt.Errorf("nwc: a string runs off the end of the file")
}

/*
 * Musical meaning of the note bytes.
 */

// nwc1Durations maps the duration byte onto a note value.
//
// NoteWorthy's own text format names these Whole, Half, 4th, 8th, 16th, 32nd
// and 64th, and the binary byte counts the same ladder. An unknown value is
// refused rather than approximated.
var nwc1Durations = map[byte]model.Duration{
	0: {Num: 1, Den: 1},  // whole
	1: {Num: 1, Den: 2},  // half
	2: {Num: 1, Den: 4},  // quarter
	3: {Num: 1, Den: 8},  // eighth
	4: {Num: 1, Den: 16}, // sixteenth
	5: {Num: 1, Den: 32},
	6: {Num: 1, Den: 64},
}

// Bits in the articulation byte.
const (
	artDotted    = 0x01
	artDblDotted = 0x02
	artTie       = 0x10
	artStaccato  = 0x20
	artAccent    = 0x40
)

// nwc1Accidentals maps the low bits of the accidental byte.
//
// The order is NoteWorthy's own, which puts "no accidental" last rather than
// first: the five accidentals are numbered in the order they appear in the
// program's own palette and the absence of one comes after them. Reading 0 as
// "none" is the obvious guess and it is wrong — it turns the Requiem into 653
// double flats, which is how this was found.
var nwc1Accidentals = map[byte]model.Accidental{
	0x00: model.Sharp,
	0x01: model.Flat,
	0x02: model.Natural,
	0x03: model.DoubleSharp,
	0x04: model.DoubleFlat,
	0x05: model.NoAccidental,
}

// noteValue is the eight-byte record a note or rest carries.
type noteValue struct {
	dur      model.Duration
	dots     int
	tie      bool
	staccato bool
	accent   bool
	triplet  bool
	step     int
	acc      model.Accidental
}

func (w *walker) noteValue() (noteValue, error) {
	var nv noteValue
	raw := make([]byte, 8)
	for i := range raw {
		c, err := w.u8()
		if err != nil {
			return nv, err
		}
		raw[i] = c
	}
	durByte, stem := raw[0], raw[2]
	artic, position, accidental := raw[4], raw[6], raw[7]

	// One extra byte follows when the accidental byte says so.
	if accidental&0x40 == 0x40 {
		if _, err := w.u8(); err != nil {
			return nv, err
		}
	}

	d, ok := nwc1Durations[durByte]
	if !ok {
		return nv, fmt.Errorf(
			"nwc: unrecognised duration byte 0x%02X; the 1.7x duration table "+
				"in this reader is incomplete", durByte)
	}
	nv.dur = d

	switch {
	case artic&artDblDotted != 0:
		nv.dur, nv.dots = nv.dur.Scale(7, 4), 2
	case artic&artDotted != 0:
		nv.dur, nv.dots = nv.dur.Scale(3, 2), 1
	}
	nv.tie = artic&artTie != 0
	nv.staccato = artic&artStaccato != 0
	nv.accent = artic&artAccent != 0

	// The stem byte carries the triplet flag.
	if stem&0x20 != 0 {
		nv.dur = nv.dur.Scale(2, 3)
		nv.triplet = true
	}

	// The position is signed and counted from the middle line, as in NWCTXT.
	nv.step = int(int8(position)) + 4
	nv.acc = nwc1Accidentals[accidental&0x07]
	return nv, nil
}

/*
 * The walk
 */

// parseNWC1 reads an inflated 1.7x score.
func parseNWC1(raw []byte, minor int) (*model.Score, error) {
	w := &walker{b: raw}

	// Header, as nwc.posa describes it.
	if _, err := w.cstring(); err != nil { // vendor
		return nil, err
	}
	if err := w.skip(2); err != nil {
		return nil, err
	}
	if _, err := w.cstring(); err != nil { // product
		return nil, err
	}
	if err := w.skip(6); err != nil { // minor, major, marker, 3 pad
		return nil, err
	}
	for i := 0; i < 2; i++ { // registered to, registration code
		if _, err := w.cstring(); err != nil {
			return nil, err
		}
	}
	if err := w.skip(10); err != nil { // 8 pad, flags, gap
		return nil, err
	}

	score := &model.Score{
		Tempo: model.Tempo{BPM: 120, Unit: model.Duration{Num: 1, Den: 4}},
	}
	title, err := w.cstring()
	if err != nil {
		return nil, err
	}
	author, err := w.cstring()
	if err != nil {
		return nil, err
	}
	copyright1, err := w.cstring()
	if err != nil {
		return nil, err
	}
	for i := 0; i < 2; i++ { // copyright2, comments
		if _, err := w.cstring(); err != nil {
			return nil, err
		}
	}
	score.Identity = model.Identity{Title: title, Composer: author, Copyright: copyright1}

	// Page setup.
	for i := 0; i < 2; i++ { // print options, paper
		if _, err := w.cstring(); err != nil {
			return nil, err
		}
	}
	if err := w.skip(4); err != nil {
		return nil, err
	}
	if _, err := w.cstring(); err != nil { // margins
		return nil, err
	}

	// Print settings, staff size, then the twelve fonts a 1.7x score carries.
	if err := w.skip(38); err != nil {
		return nil, err
	}
	for i := 0; i < 12; i++ {
		if _, err := w.cstring(); err != nil { // typeface
			return nil, err
		}
		if err := w.skip(4); err != nil { // style, size, pad, charset
			return nil, err
		}
	}

	marker, err := w.u8()
	if err != nil {
		return nil, err
	}
	if marker != 0xFF {
		return nil, fmt.Errorf(
			"nwc: expected the score marker 0xFF but found 0x%02X; the header "+
				"walk has gone out of step", marker)
	}
	if err := w.skip(3); err != nil { // 2 pad, layering
		return nil, err
	}
	staffCount, err := w.u16()
	if err != nil {
		return nil, err
	}
	if staffCount <= 0 || staffCount > 64 {
		return nil, fmt.Errorf("nwc: %d staves, which cannot be right", staffCount)
	}

	for i := 0; i < staffCount; i++ {
		part, err := w.staff(minor)
		if err != nil {
			// nwc1.posa warns that the walk can resume a byte out of step
			// after a Text or Tempo object, which then loses the rest of the
			// staff. Keeping the staves already read is far better than
			// throwing the whole score away, as long as the caller is told.
			if len(score.Parts) > 0 {
				return score, &PartialError{
					Staves: len(score.Parts), Total: staffCount,
					Cause: fmt.Errorf("staff %d: %w", i+1, err),
				}
			}
			return nil, fmt.Errorf("nwc: staff %d: %w", i+1, err)
		}
		score.Parts = append(score.Parts, part)
	}
	return score, nil
}

// PartialError reports that only some of a score could be read. It carries the
// score, so a caller can show what was understood rather than nothing.
type PartialError struct {
	Staves int
	Total  int
	Cause  error
}

func (e *PartialError) Error() string {
	return fmt.Sprintf("nwc: read %d of %d staves (%v)", e.Staves, e.Total, e.Cause)
}

func (e *PartialError) Unwrap() error { return e.Cause }

func (w *walker) staff(minor int) (*model.Part, error) {
	name, err := w.cstring()
	if err != nil {
		return nil, err
	}
	if _, err := w.cstring(); err != nil { // group
		return nil, err
	}

	if err := w.skip(15 + 2 + 2); err != nil { // settings, upper, lower
		return nil, err
	}
	if minor >= 75 {
		if err := w.skip(1); err != nil { // staff lines
			return nil, err
		}
	}
	if err := w.skip(4 + 1 + 1 + 1 + 4); err != nil {
		return nil, err
	}

	lyricLines, err := w.u16()
	if err != nil {
		return nil, err
	}
	if _, err := w.u16(); err != nil { // lyric flags
		return nil, err
	}

	if lyricLines > 0 {
		if err := w.lyrics(lyricLines); err != nil {
			return nil, err
		}
	}

	objectCount, err := w.u16()
	if err != nil {
		return nil, err
	}
	// A staff says it holds two more objects than it stores. Getting this
	// wrong runs the object loop past the end of the staff.
	objectCount -= 2
	if objectCount < 0 {
		objectCount = 0
	}

	part := &model.Part{
		Name: name,
		Clef: model.Treble,
		Key:  model.KeySignature{Fifths: 0, Mode: model.Major},
		Time: model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
	}
	st := &staffState{part: part}
	for i := 0; i < objectCount; i++ {
		if w.remaining() > 0 && w.b[w.i] == 0xFF {
			// A marker closes the list early; the count a staff reports is
			// not always the number of objects it really stores.
			break
		}
		if err := w.object(st); err != nil {
			return nil, fmt.Errorf("object %d of %d: %w", i+1, objectCount, err)
		}
	}
	st.closeMeasure(model.BarFinal)
	linkTies(part)
	return part, nil
}

// lyrics walks the fixed-size pages a staff with words carries before its
// objects. Only the page size matters to the walk.
func (w *walker) lyrics(lines int) error {
	if _, err := w.u16(); err != nil { // options
		return err
	}
	if err := w.skip(1); err != nil {
		return err
	}
	for i := 0; i < lines; i++ {
		block, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(2 + 1 + 1); err != nil { // length, two pads
			return err
		}
		if block == 0 || block > 16 {
			return fmt.Errorf("unknown lyric block size 0x%02X", block)
		}
		// The byte counts 256-byte pages: 4 is the 1 KiB page and 8 the 2 KiB
		// one that nwc1.posa names, and smaller values occur too.
		if err := w.skip(256 * int(block)); err != nil {
			return err
		}
		if err := w.skip(1); err != nil {
			return err
		}
	}
	// One more byte closes the lyric section.
	return w.skip(1)
}

type staffState struct {
	part *model.Part
	meas *model.Measure
	// pendingDynamic waits for the next note, since NoteWorthy writes the
	// marking as an object of its own just before what it applies to.
	pendingDynamic model.Dynamic
}

func (s *staffState) ensure() {
	if s.meas == nil {
		s.meas = &model.Measure{Voices: []*model.Voice{{}}}
	}
}

func (s *staffState) add(el model.Element) {
	s.ensure()
	s.meas.Voices[0].Elements = append(s.meas.Voices[0].Elements, el)
}

func (s *staffState) closeMeasure(style model.BarlineStyle) {
	if s.meas == nil {
		return
	}
	s.meas.RightBarline = style
	s.part.Measures = append(s.part.Measures, s.meas)
	s.meas = nil
}

var nwc1Clefs = map[int]model.Clef{
	0: model.Treble, 1: model.Bass, 2: model.Alto, 3: model.Tenor,
}

// nwc1Dynamics follows the ladder nwc1.posa enumerates for object type 7.
var nwc1Dynamics = map[byte]model.Dynamic{
	0: model.PPP, 1: model.PP, 2: model.Piano, 3: model.MP,
	4: model.MF, 5: model.Forte, 6: model.FF, 7: model.FFF,
}

var nwc1Barlines = map[byte]model.BarlineStyle{
	0: model.BarSingle, 1: model.BarDouble,
	4: model.BarDouble, 5: model.BarFinal,
	6: model.BarRepeatStart, 7: model.BarRepeatEnd,
	8: model.BarRepeatStart, 9: model.BarRepeatEnd,
}

// object reads one entry of the music stream.
func (w *walker) object(st *staffState) error {
	objType, err := w.u8()
	if err != nil {
		return err
	}
	if err := w.skip(2); err != nil { // object flags
		return err
	}

	switch objType {
	case 0: // clef
		c, err := w.u16()
		if err != nil {
			return err
		}
		if _, err := w.u16(); err != nil { // octave shift
			return err
		}
		if clef, ok := nwc1Clefs[c]; ok {
			st.part.Clef = clef
		}

	case 1: // key signature
		flats, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(1); err != nil {
			return err
		}
		sharps, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(7); err != nil {
			return err
		}
		// Each bit set is one altered note.
		st.part.Key = model.KeySignature{Fifths: bits(sharps) - bits(flats)}

	case 2: // barline
		style, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(1); err != nil { // repeat count
			return err
		}
		s, ok := nwc1Barlines[style]
		if !ok {
			s = model.BarSingle
		}
		st.closeMeasure(s)

	case 3: // ending
		return w.skip(2)

	case 4: // instrument patch
		// nwc1.posa leaves the eight bytes undescribed; the General MIDI
		// program is the second of them.
		raw := make([]byte, 8)
		for i := range raw {
			c, err := w.u8()
			if err != nil {
				return err
			}
			raw[i] = c
		}
		if p := raw[1]; p < 128 {
			st.part.Instrument = model.Instrument(p)
		}

	case 5: // time signature
		top, err := w.u16()
		if err != nil {
			return err
		}
		bottom, err := w.u16()
		if err != nil {
			return err
		}
		if _, err := w.u16(); err != nil {
			return err
		}
		if top > 0 && bottom > 0 {
			st.part.Time = model.TimeSignature{
				Beats: top, BeatValue: 1 << bottom, Symbol: model.TimeNumeric,
			}
		}

	case 6: // tempo
		if err := w.skip(2); err != nil {
			return err
		}
		if _, err := w.u16(); err != nil { // tempo value
			return err
		}
		if err := w.skip(2); err != nil { // base, pad
			return err
		}
		_, err := w.cstring() // tempo text
		return err

	case 7: // dynamic
		if err := w.skip(2); err != nil { // position, placement
			return err
		}
		level, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(4); err != nil { // velocity, volume
			return err
		}
		// nwc1.posa names the levels ppp through fff in order; the marking
		// applies to whatever sounds next.
		st.pendingDynamic = nwc1Dynamics[level]

	case 8, 9: // note, rest
		nv, err := w.noteValue()
		if err != nil {
			return err
		}
		if objType == 9 {
			r := &model.Rest{Dur: nv.dur}
			r.Tuplet = tupletMark(nv.triplet)
			st.add(r)
			return nil
		}
		n := &model.Note{
			Pitch:      pitchFor(st.part, nv.step, nv.acc),
			Dur:        nv.dur,
			Accidental: nv.acc,
		}
		n.TieStart = nv.tie
		n.Tuplet = tupletMark(nv.triplet)
		if st.pendingDynamic != model.DynamicNone {
			n.Dynamic = st.pendingDynamic
			st.pendingDynamic = model.DynamicNone
		}
		if nv.staccato {
			n.AddArticulation(model.Staccato)
		}
		if nv.accent {
			n.AddArticulation(model.Accent)
		}
		st.add(n)

	case 10: // chord
		if err := w.skip(8); err != nil { // the same shape a note carries
			return err
		}
		count, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(1); err != nil {
			return err
		}
		ch := &model.Chord{}
		for i := 0; i < int(count); i++ {
			if err := w.skip(3); err != nil {
				return err
			}
			nv, err := w.noteValue()
			if err != nil {
				return err
			}
			if i == 0 {
				ch.Dur = nv.dur
				ch.Tuplet = tupletMark(nv.triplet)
			}
			n := &model.Note{
				Pitch:      pitchFor(st.part, nv.step, nv.acc),
				Dur:        nv.dur,
				Accidental: nv.acc,
			}
			n.TieStart = nv.tie
			ch.Notes = append(ch.Notes, n)
		}
		if len(ch.Notes) > 0 {
			st.add(ch)
		}

	case 11: // sustain pedal
		return w.skip(3)
	case 12: // flow direction
		return w.skip(2 + 2)
	case 13: // MIDI instruction
		return w.skip(2 + 32)
	case 14: // tempo variance
		return w.skip(4)
	case 15: // dynamic variance
		return w.skip(3)
	case 16: // performance style
		return w.skip(3)
	case 17: // text
		if err := w.skip(3); err != nil {
			return err
		}
		_, err := w.cstring()
		return err
	case 18: // rest chord
		return w.skip(8 + 2)

	default:
		return fmt.Errorf("unknown object type %d", objType)
	}
	return nil
}

func bits(b byte) int {
	n := 0
	for ; b != 0; b >>= 1 {
		n += int(b & 1)
	}
	return n
}

func pitchFor(p *model.Part, step int, acc model.Accidental) model.Pitch {
	pitch := model.PitchFromDiatonicIndex(step - clefOffset(p.Clef))
	if acc != model.NoAccidental {
		pitch.Alter = acc.Semitones()
	} else {
		pitch.Alter = p.Key.AlterFor(pitch.Step)
	}
	return pitch
}
