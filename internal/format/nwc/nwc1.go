/*
 * nwc1.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
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
	// version is the file's own, as major*100+minor, because the records
	// are not the same size in every one: a note is ten bytes of value in
	// 1.70 and eight in 1.75.
	version int
	// tempo is the first metronome mark met, which is the piece's.
	tempo int
	// flatChords counts the chords whose notehead count was not one this
	// reader could use, and which therefore came to the single note their
	// record carries.
	flatChords int
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
			// Not simply string(bytes): these files were written before
			// UTF-8, so a staff called "Flûte" holds one byte for the û.
			s := decodeText(w.b[w.i:j])
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
//
// The dot is 0x04. It was read as 0x01 and the double dot as 0x02, and both
// were wrong: 0x01 is not set on a single note anywhere in the corpus, so no
// dotted note in any file was ever lengthened, and 0x02 is something else
// again — read as a double dot it turned two hundred and twenty-one plain
// notes in Fur Elise into double-dotted ones.
//
// Found by asking the files. A bar is right when its notes add up to its
// meter, and that is a question a reader can ask itself without knowing the
// music: across seventeen published scores and eight thousand bars, 0x04 as
// the dot takes the bars that add up from 61.5% to 73.8%, and no other bit of
// the byte comes near it. 0x01 and 0x02 both make it worse.
//
// The tie at 0x10 is confirmed the same way, by what the music does rather
// than by what adds up: a tie joins two notes of the same pitch, and 94.1% of
// the notes carrying 0x10 are followed by a note of the same pitch where
// notes in general are followed by one 21.2% of the time. Fur Elise sets it
// on no note at all, which is why it comes out with no ties in it — the file
// has slurs, which are the byte spanners.go reads, and no ties.
//
// What 0x02 and 0x08 mean is still unknown, and so is the double dot: no bit
// of the record behaves like one. A double-dotted note therefore reads as a
// single-dotted one, which is a bar that does not add up rather than a note
// in the wrong place.
const (
	artDotted   = 0x04
	artTie      = 0x10
	artStaccato = 0x20
	artAccent   = 0x40
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
	// slurFrom and slurTo say this note is joined by a slur to the one
	// before it and to the one after it. A note in the middle of a phrase
	// has both; the note a slur begins on has only the second.
	slurFrom bool
	slurTo   bool
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

	// The slur is a span written on every note it covers, rather than a
	// mark on its two ends: bit one says the note is joined to the one
	// after it and bit two to the one before. So the opening of Für
	// Elise reads 01, 03, 03, 03, 03, 03, 03, 03, 02 — a slur beginning,
	// carrying over seven notes and ending on the ninth, which is exactly
	// the phrase mark printed over it.
	//
	// Nothing read this byte before, so no score opened in gscore has ever
	// had a slur in it.
	//
	// 1.70 only, because the byte is not the same thing everywhere. A 1.75
	// score leaves it zero on all but a handful of notes, and a 2.0 one
	// writes 01, 80 and 81 in it — read as a span that would be hundreds
	// of slurs that begin and never end, drawn across the page to the
	// margin of every system after them. What this byte means in those
	// versions is still to be found out.
	if w.version > 0 && w.version < 175 {
		nv.slurTo = raw[5]&0x01 != 0
		nv.slurFrom = raw[5]&0x02 != 0
	}

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

	if artic&artDotted != 0 {
		nv.dur, nv.dots = nv.dur.Scale(3, 2), 1
	}
	nv.tie = artic&artTie != 0
	nv.staccato = artic&artStaccato != 0
	nv.accent = artic&artAccent != 0

	// Byte two is the beam, the stem and the triplet, two bits each.
	//
	// Bits 0 and 1 are the note's place in its beam — 0 unbeamed, 1
	// beginning, 2 continuing, 3 ending, which is what the opening of Für
	// Elise writes over its two beamed groups. Bits 2 and 3 are its place
	// in its triplet, counted the same way and 0 when it is in none. The
	// rest of the byte is the stem.
	//
	// Bit 0x20 was read as the triplet before, and it is part of the stem:
	// half the notes in every file carry it, so half the notes in every
	// file were played at two thirds of their length. The treble of Für
	// Elise is 0x2n throughout and the bass 0x1n. It is what made the
	// opening sound wrong — the six sixteenths of bar one became six
	// twenty-fourths, so the bar held a quarter of the time it should and
	// the rest of it was silence.
	//
	// The flag is in bits 2 and 3, and Für Elise does have triplets:
	// forty-five notes carry them, in one unbroken run of 1, 2, 3 repeated
	// fifteen times, which is the five bars of triplet sixteenths before
	// the theme comes back. Nothing else in the piece is in a triplet and
	// nothing else in it carries the bits.
	if (stem>>2)&3 != 0 {
		nv.dur = nv.dur.Scale(2, 3)
		nv.triplet = true
	}

	// The position is signed and counted from the middle line, as in NWCTXT.
	nv.step = int(int8(position)) + 4
	if w.version > 0 && w.version < 175 {
		// 1.70 counts the other way: downwards from the middle line where
		// 1.75 counts upwards. Everything else about the byte is the same,
		// so the reading is the same with the sign turned round.
		//
		// Checked against the piece itself. Beethoven's Albumblatt opens
		// E, D sharp, E, D sharp, E, B, D natural, C — in the fifth octave
		// bar the B — and the eight bytes are -3, -2, -3, -2, -3, 0, -2,
		// -1. Position 0 lands on the middle line of the staff, which is
		// what says the reading is right rather than merely fitted: B
		// above middle C in the treble, D below it in the bass.
		nv.step = -int(int8(position)) + 4
	}
	nv.acc = nwc1Accidentals[accidental&0x07]
	return nv, nil
}

/*
 * The walk
 */

// parseNWC1 reads an inflated 1.7x score.
// parseNWC1 reads the object stream a score before 2.75 is stored as.
//
// version is the file's own, as major*100+minor: 175 for a 1.75 score and
// 200 for a 2.0 one. One number because the layout changed at 1.75 and again
// at 2.0, and comparing against the minor number alone put 2.0 before both.
func parseNWC1(raw []byte, version int) (*model.Score, error) {
	w := &walker{b: raw, version: version}

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

	// The page setup, the print settings and the fonts, which are all
	// discarded — and are the part of the header that moves.
	//
	// Between the title block and the fonts sit the print options, the
	// paper, the margins and a block of print settings, and every one of
	// those has changed size at some point in the format's life. A 1.75
	// score has thirty-eight bytes of print settings; a 2.0 score has
	// thirty-nine and an extra string before them. Counted with a constant,
	// a file from the other version lands in the middle of a font name and
	// every byte after it is nonsense — "expected the score marker 0xFF but
	// found 0x54", which is the T of Times New Roman.
	//
	// So the fonts are found rather than counted to. Twelve typefaces, four
	// bytes of style after each, and the score marker immediately after the
	// twelfth: that shape is specific enough to recognise, and recognising
	// it is the one way to be right about a version nobody wrote down.
	// findSetup is the same trick findNWCTXT uses on a newer file, for the
	// same reason.
	if err := w.findFonts(); err != nil {
		return nil, err
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
	// A second marker, in the versions that write one.
	//
	// 1.75 goes straight from the layering byte to the staff count; 2.0 puts
	// another 0xFF in between. Taken for the low byte of the count it made a
	// five-staff score claim to have one thousand five hundred and thirty-
	// five staves, which is at least a complaint you can read.
	//
	// Safe to test for: the count is a staff count, and 0xFF as its low byte
	// would be a score of at least 255 staves, which the check below refuses
	// anyway.
	if b, err := w.peek(); err == nil && b == 0xFF {
		if err := w.skip(1); err != nil {
			return nil, err
		}
	}
	staffCount, err := w.u16()
	if err != nil {
		return nil, err
	}
	if staffCount <= 0 || staffCount > 64 {
		return nil, fmt.Errorf("nwc: %d staves, which cannot be right", staffCount)
	}

	// Staves that could not be read, to tell the reader about afterwards.
	var lost []string

	for i := 0; i < staffCount; i++ {
		// Land on the staff before reading it.
		//
		// A staff ends where the next one begins, and nearly always the
		// walk arrives there exactly. Nearly: one staff in the Rossi left a
		// thirty-five byte tail behind — two records the object count did
		// not admit to — and the staff after it was then read from the
		// middle of that tail and came out as wreckage, along with every
		// staff after it.
		//
		// A staff header is four printable strings in a row, which is a
		// shape the middle of an object stream does not often make. So when
		// the walk is not standing on one, it looks for the next one within
		// a short distance rather than reading nonsense. The same search
		// picks the walk up again after a staff that failed halfway, which
		// is how a file with one bad staff now gives up only that staff
		// instead of everything after it.
		searched := false
		if i > 0 && !w.atStaffHeader() {
			if !w.findStaffHeader() {
				// Nothing left that looks like a staff. The ones the file
				// promised and did not deliver are named rather than
				// quietly forgotten: a score that comes back four staves
				// short while saying nothing is the failure this reader
				// exists to avoid.
				for n := i; n < staffCount; n++ {
					lost = append(lost, fmt.Sprintf(
						"staff %d: the file ends before it", n+1))
				}
				break
			}
			searched = true
		}
		at := w.i
		found := at
		part, err := w.staff(version)
		if err != nil {
			// Say what went wrong, keep whatever music the staff gave up
			// before it did, and look for the next staff past this one's
			// own header. Past the whole of it: a search beginning one byte
			// in finds "st violins" where the name was "1st violins", four
			// printable strings again, and reads the same broken staff for
			// ever.
			kept := ""
			if part != nil && countElements(part) > 0 {
				kept = fmt.Sprintf(", keeping the %d bars read before that",
					len(part.Measures))
			}
			// From where it went wrong, not from where it began: the walk
			// is already most of the way through this staff's music, and
			// starting again at the header searches that staff's own
			// settings and finds four printable strings in them.
			if !w.atStaffHeader() {
				w.pastStaffHeader()
			}
			if w.findStaffHeader() {
				lost = append(lost, fmt.Sprintf("staff %d: %v%s", i+1, err, kept))
				if part != nil && countElements(part) > 0 {
					score.Parts = append(score.Parts, part)
				}
				continue
			}
		}
		if err != nil {
			// nwc1.posa warns that the walk can resume a byte out of step
			// after a Text or Tempo object, which then loses the rest of the
			// staff. Keeping the staves already read is far better than
			// throwing the whole score away, as long as the caller is told.
			if len(score.Parts) > 0 {
				nameInstruments(score)
				return score, &PartialError{
					Staves: len(score.Parts), Total: staffCount,
					Cause:      fmt.Errorf("staff %d: %w", i+1, err),
					Lost:       lost,
					LostStaves: countStaffLines(lost),
				}
			}
			return nil, fmt.Errorf("nwc: staff %d: %w", i+1, err)
		}
		// A staff whose name is made of control bytes is not a staff: it is
		// what the walk produces once it has lost its place and is reading
		// note data as text. Everything after that point is wreckage, and
		// appending it puts staves on the page that are not in the file.
		if !looksLikeText(part.Name) {
			// Lost the place: look for the next staff rather than stopping
			// here, starting past this one's header.
			w.i = at
			w.pastStaffHeader()
			if w.findStaffHeader() {
				lost = append(lost, fmt.Sprintf("staff %d: its name is not text, so the walk had lost its place", i+1))
				continue
			}
			nameInstruments(score)
			if len(score.Parts) > 0 {
				return score, &PartialError{
					Staves: len(score.Parts), Total: staffCount,
					Cause:      fmt.Errorf("staff %d: the file stops making sense here", i+1),
					Lost:       lost,
					LostStaves: countStaffLines(lost),
				}
			}
			return nil, fmt.Errorf("nwc: staff %d: the file stops making sense here", i+1)
		}
		// A staff found by searching has to have music in it.
		//
		// Four printable strings is a weak shape, and an object stream
		// makes one here and there by accident — a staff called "d" with
		// nothing in it, which is not a staff at all. Where the walk
		// arrived exactly, an empty staff is believed, because an empty
		// staff is a thing somebody can write; where it had to go looking,
		// nothing found means nothing was there.
		// Once anything has gone wrong, a staff has to prove itself.
		//
		// Before the first failure the file is believed: an empty staff is
		// a thing somebody can write. After one, the walk's idea of where
		// it is has been wrong at least once, and a staff with no music in
		// it is far more likely to be four printable strings the object
		// stream made by accident — a staff called "d" — than a real one.
		if (searched || len(lost) > 0) && countElements(part) == 0 {
			// Four printable strings in a row is a weak shape and an
			// object stream makes one by accident here and there. Finding
			// one with no music behind it means the search stopped too
			// early, not that the staff is empty — so it goes on looking
			// from past this one rather than giving up the staff's turn.
			w.i = found
			w.pastStaffHeader()
			if w.findStaffHeader() {
				i--
				continue
			}
			lost = append(lost, fmt.Sprintf(
				"staff %d: nothing readable where the next staff should be", i+1))
			continue
		}
		score.Parts = append(score.Parts, part)
	}
	// Whatever the file did not say about what plays each staff, its own
	// names usually do.
	nameInstruments(score)
	fitBarRests(score)
	if w.tempo > 0 {
		score.Tempo = model.Tempo{BPM: w.tempo, Unit: model.Duration{Num: 1, Den: 4}}
	}
	// A 1.70 score is read in part, and says so.
	//
	// The notes themselves are right — Beethoven's Albumblatt opens E, D
	// sharp, E, D sharp, E, B, D natural, C, and that is what comes out,
	// natural sign and all. What is not right is how far it gets: an
	// object type this reader does not know turns up partway through a
	// staff, and the records before it are read with sizes worked out from
	// one file. A staff that stopped early may have gone astray a little
	// before it stopped.
	//
	// Said out loud rather than left to be discovered. Notes that are
	// quietly wrong are worse than notes that are missing, because nothing
	// about the page says anything happened.
	if w.flatChords > 0 {
		lost = append(lost, fmt.Sprintf(
			"%d chords did not say how many notes they have, and were read as "+
				"the one note their record carries", w.flatChords))
	}
	if version > 0 && version < 175 && len(score.Parts) > 0 {
		lost = append([]string{
			"this is a NoteWorthy 1.70 file, which gscore reads only in part: " +
				"the opening of each staff is right, but a staff that stopped " +
				"early may be wrong for some bars before it stopped",
		}, lost...)
	}

	// Every staff that was gone around is named, even though the score came
	// back whole: a staff missing from the page without a word about it is
	// the one thing this reader must never do.
	if len(lost) > 0 {
		return score, &PartialError{
			Staves: len(score.Parts), Total: staffCount,
			Lost: lost, LostStaves: countStaffLines(lost),
		}
	}
	return score, nil
}

// PartialError reports that only some of a score could be read. It carries the
// score, so a caller can show what was understood rather than nothing.
type PartialError struct {
	Staves int
	Total  int
	Cause  error
	// Lost names the staves the walk could not read in full, one line
	// each, and anything else the reader of this file should know. A staff
	// skipped without a word is the thing this whole reader is written not
	// to do, and finding the next staff made it newly possible to skip one
	// and carry on as if nothing had happened.
	Lost []string
	// LostStaves is how many of those lines are about a staff, since the
	// list also carries remarks about the file as a whole.
	LostStaves int
}

func (e *PartialError) Error() string {
	if e.Cause == nil && len(e.Lost) > 0 {
		switch n := e.LostStaves; {
		case n == 0:
			// Every staff came through; the list holds remarks about the
			// file rather than staves it lost. Saying "two staves could
			// not be read" of a score with both its staves on the page is
			// worse than saying nothing.
			return "nwc: some of this file was not read"
		case n == 1:
			return "nwc: one staff could not be read in full"
		default:
			return fmt.Sprintf("nwc: %d staves could not be read in full", n)
		}
	}
	return fmt.Sprintf("nwc: read %d of %d staves (%v)", e.Staves, e.Total, e.Cause)
}

func (e *PartialError) Unwrap() error { return e.Cause }

// Partial implements format.Partial: what was read is worth keeping.
func (e *PartialError) Partial() {}

// Details implements format.Partial.
func (e *PartialError) Details() []string {
	out := make([]string, 0, len(e.Lost)+1)
	if e.Cause != nil {
		out = append(out, e.Error())
	}
	return append(out, e.Lost...)
}

func (w *walker) staff(version int) (*model.Part, error) {
	name, err := w.cstring()
	if err != nil {
		return nil, err
	}
	if _, err := w.cstring(); err != nil { // group
		return nil, err
	}
	if version >= 200 {
		// 2.0 names the instrument and the sound it plays with, where 1.75
		// had only the staff's own name and its group.
		for i := 0; i < 2; i++ {
			if _, err := w.cstring(); err != nil {
				return nil, err
			}
		}
	}

	if err := w.skip(15 + 2 + 2); err != nil { // settings, upper, lower
		return nil, err
	}
	if version >= 175 {
		if err := w.skip(1); err != nil { // staff lines
			return nil, err
		}
	}
	if err := w.skip(4 + 1 + 1 + 1 + 4); err != nil {
		return nil, err
	}
	if version >= 200 {
		// Ten more bytes of staff settings than 1.75 keeps. 2.0 gave the
		// staff a few more things to remember about itself — what it plays
		// through, how it is laid out — and they sit here.
		if err := w.skip(10); err != nil {
			return nil, err
		}
	}

	lyricLines, err := w.u16()
	if err != nil {
		return nil, err
	}

	// The word after the line count belongs to the lyrics, not to the staff.
	//
	// 1.75 writes it either way. 2.0 writes it only when there are lyrics to
	// describe, so on a staff with none — the piano under a song, which is
	// most staves in most songs — reading it anyway took the object count
	// for it and then found a count of zero in the two bytes after. The
	// staff came out empty, and the one after it began in the middle of
	// this one's music.
	if lyricLines > 0 || version < 200 {
		if _, err := w.u16(); err != nil { // lyric options
			return nil, err
		}
	}

	if lyricLines > 0 {
		if version >= 200 {
			if err := w.lyrics2(); err != nil {
				return nil, err
			}
		} else if err := w.lyrics(lyricLines); err != nil {
			return nil, err
		}
	}

	objectCount, err := w.u16()
	if err != nil {
		return nil, err
	}
	// A 1.7x staff says it holds two more objects than it stores. Getting
	// this wrong runs the object loop past the end of the staff — and 2.0
	// counts them honestly, so taking two off there leaves the last two
	// unread and the staff after it starting in the middle of this one.
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
		if w.remaining() > 0 && w.b[w.i] == 0xFF && w.endOfObjects() {
			// A marker closes the list early; the count a staff reports is
			// not always the number of objects it really stores.
			break
		}
		if err := w.object(st); err != nil {
			// Keep what was read. A staff that goes wrong at object five
			// hundred has four hundred and ninety-nine objects of music in
			// it, and throwing those away to report a clean failure is the
			// opposite of what this reader is for: the file is somebody's
			// piece, and most of it is right there.
			st.closeMeasure(model.BarFinal)
			linkTies(part)
			return part, fmt.Errorf("object %d of %d: %w", i+1, objectCount, err)
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
	// clefSet says the staff's opening clef has been read, so that a clef
	// change partway through does not become the staff's own.
	clefSet bool
	meas    *model.Measure
	// pendingDynamic waits for the next note, since NoteWorthy writes the
	// marking as an object of its own just before what it applies to.
	pendingDynamic model.Dynamic
	// chord is the chord being gathered and chordLeft how many of its
	// noteheads are still to come. A 1.70 chord is a record that says how
	// many notes it has and then lets them follow as ordinary note
	// records, so the notes after one belong to it rather than to the bar.
	chord     *model.Chord
	chordLeft int
}

func (s *staffState) ensure() {
	if s.meas == nil {
		s.meas = &model.Measure{Voices: []*model.Voice{{}}}
	}
}

func (s *staffState) add(el model.Element) {
	s.ensure()
	// A note while a chord is open is one of its noteheads.
	if s.chordLeft > 0 {
		if n, ok := el.(*model.Note); ok {
			s.chord.Notes = append(s.chord.Notes, n)
			s.chordLeft--
			if s.chordLeft == 0 {
				s.chord = nil
			}
			return
		}
		// Anything else means the noteheads are not where the chord said
		// they would be, and the chord is as finished as it is going to be.
		s.endChord()
	}
	s.meas.Voices[0].Elements = append(s.meas.Voices[0].Elements, el)
}

// openChord starts a chord of n noteheads, which the next n note records
// fill. The chord takes its place in the bar now, so that the notes landing
// in it do not have to be put back in order afterwards.
func (s *staffState) openChord(ch *model.Chord, n int) {
	s.endChord()
	s.ensure()
	s.meas.Voices[0].Elements = append(s.meas.Voices[0].Elements, ch)
	s.chord, s.chordLeft = ch, n
}

// endChord stops waiting for noteheads. One that never got any is taken back
// out of the bar: an empty chord is a thing the engraver would have to draw
// nothing for, in a space nothing else may use.
func (s *staffState) endChord() {
	if s.chord == nil {
		return
	}
	if len(s.chord.Notes) == 0 && s.meas != nil {
		els := s.meas.Voices[0].Elements
		if len(els) > 0 && els[len(els)-1] == model.Element(s.chord) {
			s.meas.Voices[0].Elements = els[:len(els)-1]
		}
	}
	s.chord, s.chordLeft = nil, 0
}

func (s *staffState) closeMeasure(style model.BarlineStyle) {
	s.endChord()
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
	flags, ferr := w.u8()
	if ferr != nil {
		return ferr
	}
	if err := w.skip(1); err != nil { // the second flag byte
		return err
	}

	// A record marked 5 is four bytes long and holds nothing else.
	//
	// Whatever the type byte says. 1.70 writes these between the music —
	// as a 255, as a 254, and as a 1 that is not a key signature — and
	// reading the last of those as the thirteen-byte key signature its
	// type claims took the walk through a barline and into the middle of a
	// fingering mark. The mark the record carries is the length, not the
	// type.
	if flags == 5 && w.version > 0 && w.version < 175 {
		return w.skip(1)
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
		if clef, ok := nwc1Clefs[c]; ok && !st.clefSet {
			// The first one only. A staff that changes clef partway — the
			// left hand of a piano piece going up into the treble for a
			// while — writes another of these, and taking the last one
			// made the whole staff treble from its first bar. gscore keeps
			// one clef per staff, so the one it opens with is the one that
			// belongs to it.
			st.part.Clef = clef
			st.clefSet = true
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
		bpm, terr := w.u16()
		if terr != nil {
			return terr
		}
		// The first one is the piece's speed. The value was read and
		// thrown away before, so every score opened at a hundred and
		// twenty whatever it said: Für Elise is marked Poco Moto, sixty-
		// nine to the quarter, and arrived as a march.
		//
		// Counted in quarters, which is what gscore counts in. NoteWorthy
		// shows the same mark as 138 to the eighth, which is the same
		// speed said the other way round.
		if w.tempo == 0 && bpm >= 20 && bpm <= 400 {
			w.tempo = bpm
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
		// A 1.70 note carries two bytes more than a 1.75 one — and a 1.70
		// rest does not, which is the kind of thing only the file can tell
		// you. Read as eight, every note left two bytes of itself behind
		// and the walk was two further out of step with each one; read as
		// ten for rests as well, every rest ate the two bytes after it and
		// the time signature arrived as 8/0.
		if objType == 8 && w.version > 0 && w.version < 175 {
			if err := w.skip(2); err != nil {
				return err
			}
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
		// A slur belongs to its two ends. The file marks every note under
		// it, so the ends are the note that is joined onward without being
		// joined back, and the note that is joined back without going on.
		n.SlurStart = nv.slurTo && !nv.slurFrom
		n.SlurStop = nv.slurFrom && !nv.slurTo
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
		// A chord opens with the same block a note carries, and in 1.70
		// that block is ten bytes rather than eight — just as a note's is,
		// and so is each notehead after it. Changing one without the other
		// is worse than changing neither: with a ten-byte opening and
		// eight-byte noteheads every chord came out two bytes short for
		// each note in it, and four more bars were lost than gained.
		if w.version > 0 && w.version < 175 {
			// A 1.70 chord does not carry its noteheads: it says how many
			// it has and lets them follow as ordinary note records.
			//
			// The record is the size of a note — the same eight-byte value
			// block and two-byte tail — and then four bytes more, the
			// third of which is the count. Its own value block is not an
			// extra note. It repeats one of the noteheads that follow: in
			// all eighty-one chords of Für Elise the pitch it carries is
			// one of theirs and every notehead has its duration.
			//
			// Read as a note of its own with the noteheads behind it as
			// notes of their own, a three-note chord became three notes in
			// a row and took three times the time. That is what pulled the
			// hands of Für Elise apart: from the first chord in bar 25 the
			// left hand had bars holding two and three bars of music, so
			// it ran through them while the right hand was still in the
			// bar they share — the staves of a NoteWorthy score keep the
			// same barlines, and ours no longer did.
			nv, cerr := w.noteValue()
			if cerr != nil {
				return cerr
			}
			if err := w.skip(2); err != nil {
				return err
			}
			tail := make([]byte, 4)
			for i := range tail {
				c, terr := w.u8()
				if terr != nil {
					return terr
				}
				tail[i] = c
			}
			count := int(tail[2])
			if count < 1 || count > 8 {
				// Not a count this reader understands. The value block is
				// one real notehead, so the melody is kept and the harmony
				// above it is not — which the log says out loud.
				n := &model.Note{
					Pitch:      pitchFor(st.part, nv.step, nv.acc),
					Dur:        nv.dur,
					Accidental: nv.acc,
				}
				n.TieStart = nv.tie
				n.Tuplet = tupletMark(nv.triplet)
				st.add(n)
				w.flatChords++
				return nil
			}
			ch := &model.Chord{Dur: nv.dur}
			ch.Tuplet = tupletMark(nv.triplet)
			st.openChord(ch, count)
			return nil
		}
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

	case 254, 255:
		// A 1.70 score writes one of these here and there inside a staff.
		// What they are for is not known — nothing in the music changes
		// when they are stepped over — but they are counted among the
		// staff's objects and both are one byte long, and the reader used
		// to take the 0xFF for the end of the staff: a hundred-bar piano
		// piece stopped at bar twenty-one, where the first one happened to
		// be.
		return w.skip(1)

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

// fontBlockSearch is how far past the title block the fonts are looked for.
//
// They are about ninety bytes along in every file met so far; this is room
// for a version that puts a good deal more in between, and small enough that
// the search cannot wander into the music.
const fontBlockSearch = 512

// findFonts moves the walk to just past the twelve font records.
//
// It tries each offset in turn and keeps the first that fits the shape:
// twelve NUL-terminated typefaces, four bytes after each, and then the 0xFF
// that opens the score. Twelve strings in a row ending exactly on a marker is
// not a coincidence anything else in the header produces.
func (w *walker) findFonts() error {
	start := w.i
	limit := start + fontBlockSearch
	if limit > len(w.b) {
		limit = len(w.b)
	}
	for at := start; at < limit; at++ {
		probe := &walker{b: w.b, i: at}
		if !probe.looksLikeFonts() {
			continue
		}
		w.i = probe.i
		return nil
	}
	return fmt.Errorf("nwc: the twelve fonts are not where any version puts them, " +
		"so the rest of the header cannot be trusted")
}

// How many fonts a score carries, which is not the same in every version:
// 1.70 keeps ten and 1.75 and 2.0 keep twelve. So they are read until the
// score marker turns up rather than counted, with a floor and a ceiling to
// keep the search honest.
const (
	minFonts = 8
	maxFonts = 24
)

// looksLikeFonts reads font records until the score marker, and reports
// whether that is what it found.
func (w *walker) looksLikeFonts() bool {
	for n := 0; n <= maxFonts; n++ {
		if w.i >= len(w.b) {
			return false
		}
		if w.b[w.i] == 0xFF {
			// The marker that ends the list: this was a font block if
			// enough of one came before it.
			return n >= minFonts
		}
		name, err := w.cstring()
		if err != nil {
			return false
		}
		// A typeface is a short printable name. Checking it keeps the
		// search from accepting a run of empty strings in a pad field,
		// which is the one other place a dozen NULs sit together.
		if !looksLikeTypeface(name) {
			return false
		}
		if err := w.skip(4); err != nil {
			return false
		}
	}
	return false
}

// looksLikeTypeface says whether a string could be the name of a font.
func looksLikeTypeface(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			return false
		}
	}
	return true
}

// peek is the next byte without taking it.
func (w *walker) peek() (byte, error) {
	if w.i >= len(w.b) {
		return 0, fmt.Errorf("nwc: the file stops making sense here")
	}
	return w.b[w.i], nil
}

// lyrics2 walks the lyric block of a 2.0 score.
//
// 1.75 stores the words in whole 256-byte pages and says how many; 2.0 says
// how long the block is and how much of it is used, which is both tidier and
// a different shape entirely. Read as pages, the length came out as a page
// count of zero — "unknown lyric block size 0x00" — and the staff was lost
// there, with its music still unread behind it.
//
// All of the lines live in the one block, so there is one of these however
// many verses the staff carries.
func (w *walker) lyrics2() error {
	room, err := w.u16() // how long the block is
	if err != nil {
		return err
	}
	if _, err := w.u16(); err != nil { // how much of it is words
		return err
	}
	if err := w.skip(2); err != nil {
		return err
	}
	if room > 1<<20 {
		return fmt.Errorf("a lyric block of %d bytes, which cannot be right", room)
	}
	if err := w.skip(int(room)); err != nil {
		return err
	}
	// Two bytes close the block, as one closes a 1.75 one.
	return w.skip(2)
}

// staffSearch is how far past the end of one staff the next one is looked
// for.
//
// A staff's worth of music, and then some: when a staff stops early the next
// one is however far away the rest of its music is, and in Für Elise that is
// four and a half thousand bytes. Searching only as far as a tail would
// reach found nothing and the left hand of the piece was never read.
const staffSearch = 1 << 20

// atStaffHeader says whether the walk is standing on the start of a staff.
//
// Four printable strings in a row — the name, the group, the instrument and
// the sound — and then bytes that are not another string. It is a weak
// signature taken alone and a strong one taken here, where the alternative
// is reading an object stream as text.
func (w *walker) atStaffHeader() bool {
	probe := &walker{b: w.b, i: w.i}
	for n := 0; n < 4; n++ {
		s, err := probe.cstring()
		if err != nil {
			return false
		}
		if n == 0 && (s == "" || !looksLikeText(s)) {
			return false
		}
		if len(s) > 48 || (s != "" && !looksLikeText(s)) {
			return false
		}
	}
	// A staff has a good deal more in it than its four names.
	return probe.remaining() > 32
}

// findStaffHeader moves the walk to the next staff header, and says whether
// it found one.
func (w *walker) findStaffHeader() bool {
	limit := w.i + staffSearch
	if limit > len(w.b) {
		limit = len(w.b)
	}
	for at := w.i; at < limit; at++ {
		probe := &walker{b: w.b, i: at}
		if probe.atStaffHeader() {
			w.i = at
			return true
		}
	}
	return false
}

// pastStaffHeader steps over a staff's four names, so that a search for the
// next staff cannot find this one again a byte further on.
func (w *walker) pastStaffHeader() {
	for n := 0; n < 4; n++ {
		if _, err := w.cstring(); err != nil {
			return
		}
	}
}

// countElements is how much music a staff holds, across its voices.
func countElements(p *model.Part) int {
	n := 0
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			n += len(v.Elements)
		}
	}
	return n
}

// endOfObjects says whether the 0xFF the walk is standing on closes the
// staff's object list, rather than being a byte inside it.
//
// A staff's list can end before its count is used up, and 0xFF is what ends
// it — but 0xFF also occurs inside the list, and treating every one as the
// end stopped a hundred-bar piano piece after twenty-one bars. What tells
// them apart is what comes next: at the end of a staff there is another
// staff, or nothing at all.
func (w *walker) endOfObjects() bool {
	probe := &walker{b: w.b, i: w.i, version: w.version}
	// Past the marker and the byte that follows it, which is where the next
	// staff's name begins when one does.
	if err := probe.skip(2); err != nil {
		return true // nothing left: this is certainly the end
	}
	if probe.remaining() < 32 {
		return true
	}
	return probe.atStaffHeader()
}

// countStaffLines is how many of the lines are about one staff rather than
// about the file as a whole, so that "three staves could not be read" counts
// staves and not remarks.
func countStaffLines(lines []string) int {
	n := 0
	for _, l := range lines {
		if len(l) >= 6 && l[:6] == "staff " {
			n++
		}
	}
	return n
}

// fitBarRests gives a bar of silence the length of its bar.
//
// A staff that is silent for a bar is written as one whole rest, whatever the
// meter — which is how a bar rest is printed, and NoteWorthy stores what it
// prints. In three eight that rest says one whole note where the bar is three
// eighths of one, and since a bar lasts as long as the longest thing in it,
// the bar then lasted eight times too long: in Fur Elise the six notes of the
// opening were followed by two and a half seconds of nothing while the left
// hand's empty bar ran its course.
//
// The staves of a NoteWorthy score share their barlines, so a whole rest in a
// short bar cannot mean a whole note of silence — it would put every staff
// after it out of step with the one it is in. It means the staff has this bar
// off, and that is what it is read as.
//
// Only a bar with nothing else in it, and only a rest longer than the bar, so
// this can shorten a bar of silence and nothing else. Across the corpus it
// takes the bars that add up from 73.8% to 87.7%, and three movements of the
// Requiem from under two thirds to very nearly all.
func fitBarRests(score *model.Score) {
	for _, p := range score.Parts {
		for i, m := range p.Measures {
			if len(m.Voices) != 1 || len(m.Voices[0].Elements) != 1 {
				continue
			}
			r, ok := m.Voices[0].Elements[0].(*model.Rest)
			if !ok {
				continue
			}
			bar := p.TimeAt(i).BarDuration()
			if bar.IsZero() || r.Dur.Cmp(bar) <= 0 {
				continue
			}
			r.Dur = bar
		}
	}
}
