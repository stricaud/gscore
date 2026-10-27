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
	"strings"

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
	// brace is the staff just read saying it is the upper or the lower half
	// of a grand staff. See dialect.braceByte.
	brace int
	// printed is the staff just read saying it is one of the staves on the
	// page, rather than one that is only played.
	printed bool
	// layerWithNext is the staff just read asking to be printed on the
	// same staff as the one after it. It lives here rather than on the
	// part because it is a thing about the file, not about the music: once
	// the two staves are one part it has nothing left to say.
	layerWithNext bool
	// group is the staff just read's group name, for the same reason.
	group string
}

// symbolFont says whether a text slot holds symbols rather than words.
//
// What is written in one is strokes: a staff user draws things NoteWorthy has
// no object for by typing in a music font, and the 8va over bars 24 to 29 of
// Danse Macabre is a run of 0xF2 in one, which is a dash, with a hook on the
// end. Printed as text it is a line of o-graves across the page.
//
// By the slot rather than by the typeface's name, which is not always one
// that says what it is — this file draws its 8va in "MusikDingsSans". Across
// the corpus the first eight slots carry the writing — "dolce", "poco rit.",
// "una corda", a fingering — and everything from the eighth up carries
// symbols: two thousand of them, and not one word.
func (w *walker) symbolFont(slot byte) bool {
	return slot >= nwcMusicFont
}

// dia is the layout this file's version is written in.
func (w *walker) dia() dialect { return dialectOf(w.version) }

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
// The staccato is 0x02 and was read as 0x20, which put the dot on twenty
// notes of Für Elise that have none and left the hundred and eighty-two that
// have one without it. NoteWorthy prints a staccato dot under every note of
// the left hand through the middle of the piece, and 0x02 is the bit those
// notes carry.
//
// 0x40 is not an accent, which is what it was read as. It says the note is a
// notehead of the chord before it: in Für Elise it is set on all one hundred
// and ninety-two chord noteheads and on none of the six hundred and forty-
// five other notes, with no exception either way. Read as an accent it put
// one on every note of every chord in the piece.
//
// The accent is 0x20, which the staccato had taken. NoteWorthy prints a ">"
// over the dotted note in bar 25 of Für Elise and over its twin in bar 26,
// and those two are the notes carrying 0x24 — the accent with the dot.
// Twenty notes in the piece have it.
//
// What 0x08 means is still unknown, and so is the double dot: no bit of the
// record behaves like one. A double-dotted note therefore reads as a single-
// dotted one, which is a bar that does not add up rather than a note in the
// wrong place.
const (
	artStaccato = 0x02
	artDotted   = 0x04
	artTie      = 0x10
	artAccent   = 0x20
	// artChordNote marks a notehead of the chord record before it, which is
	// how a chord keeps its notes together — see the chord in object().
	artChordNote = 0x40
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
	// grace says the note is written small before the note it ornaments
	// and takes none of the bar's time.
	grace bool
	// chordNote says this note is one of the noteheads of the chord record
	// before it, rather than a note of its own in the bar.
	chordNote bool
	triplet   int
	step      int
	acc       model.Accidental
	// slurFrom and slurTo say this note is joined by a slur to the one
	// before it and to the one after it. A note in the middle of a phrase
	// has both; the note a slur begins on has only the second.
	slurFrom bool
	slurTo   bool
	// beam is the note's place in the beam over it, straight off the file:
	// 0 none, 1 beginning, 2 continuing, 3 ending.
	beam byte
}

func (w *walker) noteValue(rest bool) (noteValue, error) {
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
	// From 1.75 only bit one is a slur and bit two is something else, so
	// the far end has to be worked out rather than read: the slur ends on
	// the note after the last one joined onward.
	//
	// Measured against the page. Bar 195 of Danse Macabre carries the bit
	// on six of its elements and bar 196 on two, and the phrase mark
	// printed over them runs from the second element of 195 to the third
	// of 196 — the same eight. The short mark at bar 198 is the same bit
	// on one chord, joining it to the next.
	//
	// The other bits are left alone. A 2.0 file writes 0x80 as well, and
	// read as part of a span that would be hundreds of slurs beginning and
	// never ending, drawn across the page to the margin of every system
	// after them.
	nv.slurTo = raw[5]&0x01 != 0
	if w.dia().slurHasBothEnds {
		nv.slurFrom = raw[5]&0x02 != 0
		// And 0x20 of the same byte is a grace note, which takes no time
		// in the bar it is written in.
		//
		// Three notes in Für Elise have it — the two the piece leans into
		// bar 25 with and the one before the run in bar 28 — and those two
		// bars were the only two left in the piece whose notes did not add
		// up to their meter, each by exactly the grace notes in it. Read as
		// notes of their own they were an eighth of music that is not
		// there, played on the beat and pushing everything after them late.
		nv.grace = raw[5]&0x20 != 0
	}

	// One extra byte follows when the accidental byte says so — on a note.
	//
	// A rest has no accidental, so the byte in that place means something
	// else and must not be read as one. In a 2.5 score it is often 0xF8,
	// and every rest in the piece therefore ate the byte after it: a bar of
	// Danse Macabre with two rests in it put the walk two bytes out, and
	// what followed was read as a run of clefs.
	if !rest && accidental&0x40 == 0x40 {
		if _, err := w.u8(); err != nil {
			return nv, err
		}
	}

	// From 2.0 the top half of the duration byte carries flags of its own,
	// so only the bottom four bits say how long the note is. Danse Macabre
	// writes 0x11 for a half note, and read whole that is no duration at
	// all: the walk gave up on it two thirds of the way down the first
	// staff.
	if w.dia().durationHasFlags {
		durByte &= 0x0F
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
	nv.chordNote = artic&artChordNote != 0

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
	nv.beam = stem & 3
	// The two bits say which of the three it is as well as that it is one,
	// and both halves are needed: the figure is written over the group, so
	// a group with no first and no last note is never written at all.
	nv.triplet = int(stem>>2) & 3
	if nv.triplet != tripletNone {
		nv.dur = nv.dur.Scale(2, 3)
	}

	// The position is signed and counted from the middle line, as in
	// NWCTXT — but which way it counts is the least consistent thing in
	// the format. 1.70 counts downwards, 1.75 and 2.0 upwards, and 2.5
	// downwards again.
	//
	// Every one of those was measured rather than assumed. Beethoven's
	// Albumblatt opens E, D sharp, E, D sharp, E, B, D natural, C and its
	// eight bytes are -3, -2, -3, -2, -3, 0, -2, -1: position 0 lands on
	// the middle line, which is what says the 1.70 reading is right rather
	// than merely fitted. The subject of Die Kunst der Fuge is in the
	// corpus in a 2.0 file and comes out as Bach wrote it — up a fifth,
	// down a third, down a third, down a step and back — which counted the
	// other way is that shape upside down. And the opening of Danse
	// Macabre is a half note under the treble staff with a right hand
	// alternating A4 and D5 above it; counted upwards those come out G5
	// and C5 and G4, the mirror of what NoteWorthy prints.
	nv.step = int(int8(position)) + 4
	if w.dia().positionsCountDown {
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

	// The byte that closes the font table, and the padding behind it.
	//
	// Most files write 0xFF there and refusing anything else used to be the
	// check that the header walk was still in step. It is not one: the
	// Op. 111 sonata is 1.75, writes twelve fonts exactly as Op. 27 does
	// and puts a zero in that slot, and the whole file — eight staves of
	// it — was thrown away over the byte. What follows is the same either
	// way, and the staff count below is the real check, since a count
	// outside one to sixty-four says the walk is lost.
	if err := w.skip(1 + 3); err != nil { // marker, 2 pad, layering
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
	// And then whatever padding this version puts before the count.
	//
	// 2.5 writes a zero byte there, and read as the count's low byte it
	// made a five-staff score claim to have one thousand two hundred and
	// eighty: the same failure as the marker above, one version later.
	//
	// Found rather than counted, for the reason the fonts are. A count's
	// low byte is never zero — a score has at least one staff — so a zero
	// here is padding whatever version wrote it, and a version that puts
	// none is unaffected.
	for {
		b, err := w.peek()
		if err != nil || b != 0x00 {
			break
		}
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
	// One entry per part kept, saying whether that staff is printed on the
	// same staff as the one after it, and which group it is filed under.
	var shares []bool
	var groups []string
	braces := map[*model.Part]int{}
	printed := map[*model.Part]bool{}

	// The count is a floor, not a ceiling.
	//
	// A 2.5 score writes staves the count does not admit to: Danse Macabre
	// says five and has six, the sixth being the left hand of the two
	// printed staves — the half of the piece a pianist actually reads. It
	// was simply dropped, and the file said nothing about losing it.
	//
	// So the staves are read until there are no more of them. The count
	// still decides how many are owed, and a file that ends before it has
	// delivered them says so; one that has more keeps going. The cap is
	// what stops a file whose stream happens to keep making the header
	// shape from being read for ever.
	const maxStaves = 64
	for i := 0; i < maxStaves; i++ {
		if i >= staffCount && !w.atStaffHeader() && !w.looksLikeAnotherStaff() {
			break
		}
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
		// Land on the staff, the first one included.
		//
		// The first was trusted to be exactly where the header walk left
		// off, which held until a version put something between the staff
		// count and the staff: 2.5 writes twelve bytes there, and read as
		// a staff they are a name of control characters and a score of
		// nothing. The search that picks the walk up after every other
		// staff does for the first one too.
		searched := false
		if !w.atStaffHeader() {
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
			if i >= staffCount {
				// Past what the file promised, so this is one of the extra
				// staves rather than a missing one: it is read, but the
				// search is not allowed to wander for it.
				_ = searched
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
					shares = append(shares, false)
					groups = append(groups, w.group)
					// What the header said about the staff still holds: it
					// is read before the music, and it is the music that
					// went wrong. Left out here, Op. 111's "Lower Right
					// Hand" — which breaks halfway down — lost its claim to
					// be a printed staff and was taken off the page.
					braces[part] = w.brace
					if w.printed {
						printed[part] = true
					}
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
		shares = append(shares, w.layerWithNext)
		groups = append(groups, w.group)
		// Held against the part rather than its index, because merging the
		// layers below drops staves and moves the rest up.
		braces[part] = w.brace
		if w.printed {
			printed[part] = true
		}
	}
	hidePlayOnly(score.Parts, printed)
	score.Parts, groups = mergeLayers(score.Parts, shares, groups)
	// Again, now the layers are one staff. A spanner is held open per line
	// while drawing, and merging moves lines about: one closed on its own
	// staff can come back open on the staff it is merged into.
	for _, p := range score.Parts {
		closeDanglingSpans(p)
	}
	score.Groups = staffGroups(score.Parts, groups, braces)
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
	d := dialectOf(version)

	// The names a staff header opens with, which is where the versions
	// differ most: the staff's own and the group it is filed under in
	// 1.75; the instrument and the sound it plays with as well from 2.0;
	// and from 2.5 the layer it belongs to, "Hidden" or "Display".
	//
	// Counted one short, the walk ended a string early and read the last
	// of them as the staff's own: a score of staves called "Hidden" with
	// no music in them.
	names := make([]string, d.headerNames)
	for i := range names {
		s, err := w.cstring()
		if err != nil {
			return nil, err
		}
		names[i] = s
	}
	name := names[0]
	// The group NoteWorthy prints down the left of the staves that share
	// it, with a brace or a bracket round them.
	w.group = names[1]
	layer := ""
	if d.headerNames >= 5 {
		// How one instrument is written twice — a staff that sounds and a
		// staff that is printed. The names in Danse Macabre say so: "Piano
		// RH1 Sounding" is Hidden and "Piano LH2" is Display.
		layer = names[4]
	}

	// Where the staff's settings begin, which is what the one flag read out
	// of them below is measured from.
	settings := w.i
	if err := w.skip(15 + 2 + 2); err != nil { // settings, upper, lower
		return nil, err
	}
	if d.staffLinesByte {
		if err := w.skip(1); err != nil { // staff lines
			return nil, err
		}
	}
	if err := w.skip(4 + 1 + 1 + 1 + 4); err != nil {
		return nil, err
	}
	// More staff settings than 1.75 keeps. 2.0 gave the staff a few extra
	// things to remember about itself — what it plays through, how it is
	// laid out — and they sit here.
	if err := w.skip(d.extraSettings); err != nil {
		return nil, err
	}

	// "Layer with next staff": the two staves are printed as one.
	//
	// It is how a pianist's right hand is written as two independent lines
	// on a single staff — one part of the texture per layer, each with its
	// own stems and rests — and it is why Danse Macabre has eight staves in
	// the file and two on the page. Without it the score came out as four
	// grand staves of the same music stacked up the page.
	//
	// Read at a measured offset rather than a derived one, and the offset
	// moves with the version: see dialect.layerByte. Moonlight Sonata is
	// 1.75 and four staves in the file, "Right Hand" layered with "RH2"
	// and "Left Hand" with "LH2"; read at 2.0's offset they would all be
	// unlayered, which is the four staves gscore drew where NoteWorthy
	// draws a pianist's two.
	w.layerWithNext = false
	if d.layerByte > 0 && settings+d.layerByte < len(w.b) {
		w.layerWithNext = w.b[settings+d.layerByte] != 0
	}

	// Whether the staff is printed at all.
	//
	// NoteWorthy writes an instrument twice over where the sound wanted and
	// the notation wanted are not the same thing: a staff to be played and
	// a staff to be read, with only the second on the page. 2.5 names the
	// two layers "Hidden" and "Display" in the staff header and that is how
	// Danse Macabre was sorted out, but no earlier version writes the
	// names — the Op. 111 sonata is 1.75 and calls its four playing staves
	// "RH Interpretation", "LH Interpretation" and so on, which says it to
	// a reader and nothing to a program.
	//
	// The second byte of the settings says it, and says the same thing in
	// every version: 1 on the staves Danse Macabre calls Display and 0 on
	// the ones it calls Sounding, 1 on Op. 111's four hands and 0 on its
	// four interpretations, 1 on the two staves of the Op. 27 Presto that
	// are not in the group called "Hidden".
	//
	// Only where some staff claims it, though. Most files leave it zero
	// throughout — everything in them is printed — and a file that never
	// says which staves are printed is not saying that none of them are.
	w.printed = d.printedFlag && settings+1 < len(w.b) && w.b[settings+1] == 1

	// And whether the staff is half of a grand staff, which is the one
	// thing that says to brace it when the file files every staff under
	// NoteWorthy's default group and so names no group at all.
	w.brace = braceNone
	if d.braceByte > 0 && settings+d.braceByte < len(w.b) {
		w.brace = int(w.b[settings+d.braceByte])
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
	if lyricLines > 0 || d.lyricOptionsAlways {
		if _, err := w.u16(); err != nil { // lyric options
			return nil, err
		}
	}

	if lyricLines > 0 {
		if d.pagedLyrics {
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
		// A staff in the hidden layer is the one that sounds, not the one
		// that is printed: NoteWorthy writes an instrument twice over, a
		// staff to be played and a staff to be read, and prints only the
		// second. Danse Macabre is eight staves that way and two on the
		// page, and drawn all together they were four grand staves of the
		// same music stacked up the page.
		//
		// Hidden in gscore takes a staff off the page and out of what is
		// played, and the printed staves carry the same notes, so the
		// piece still sounds.
		// A staff filed under a group called "Hidden" says the same thing
		// in words, which is how the Op. 27 Presto says it: "_Treble Clef"
		// and "_Bass Clef", in the group "Hidden", under the two staves
		// that are printed. 1.70 has no flag this reader trusts, so the
		// name is all there is.
		Hidden: layer == "Hidden" || w.group == "Hidden",
		Clef:   model.Treble,
		Key:    model.KeySignature{Fifths: 0, Mode: model.Major},
		Time:   model.TimeSignature{Beats: 4, BeatValue: 4, Symbol: model.TimeNumeric},
	}
	st := &staffState{part: part}
	// The count is a floor here too.
	//
	// A staff goes on until the next one begins, and the number it gives
	// is not always the number it holds: the Rossi leaves two records
	// behind, and three of Danse Macabre's printed staves stop five, seven
	// and thirteen bars before the end of the piece — the last chords of a
	// Liszt transcription, missing with nothing said about them.
	//
	// So past the count the walk carries on until it reaches the next
	// staff, runs out of file, or stops making sense. Stops quietly: the
	// staff has already given everything the file promised, and a complaint
	// about the tail would be a complaint about music that is on the page.
	const tail = 1 << 16
	for i := 0; i < objectCount+tail; i++ {
		past := i >= objectCount
		if past && (w.remaining() == 0 || w.atStaffHeader()) {
			break
		}
		if w.remaining() > 0 && w.b[w.i] == 0xFF && w.endOfObjects() {
			// A marker closes the list early; the count a staff reports is
			// not always the number of objects it really stores.
			break
		}
		at := w.i
		if err := w.object(st); err != nil {
			// A staff that walks into the next staff's name has simply
			// run out of music, whatever its object count claims. Danse
			// Macabre's left hand says it has sixty-three thousand
			// objects and has about two thousand, and reading on turned
			// the P of "Piano LH1 Sounding" into an unknown object type
			// 80 — and left the next staff to be found by searching,
			// three bytes late, under the name "no LH1 Sounding".
			//
			// Tested here rather than before each object because the
			// signature is a weak one: four strings in a row, which a run
			// of zeroes in the middle of the music also satisfies.
			w.i = at
			if past || w.atStaffHeader() {
				break
			}
			// Keep what was read. A staff that goes wrong at object five
			// hundred has four hundred and ninety-nine objects of music in
			// it, and throwing those away to report a clean failure is the
			// opposite of what this reader is for: the file is somebody's
			// piece, and most of it is right there.
			st.closeMeasure(model.BarFinal)
			linkTies(part)
			closeDanglingSpans(part)
			return part, fmt.Errorf("object %d of %d at byte %d: %w", i+1, objectCount, w.i, err)
		}
	}
	st.closeMeasure(model.BarFinal)
	linkTies(part)
	closeDanglingSpans(part)
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
	// change partway through does not become the staff's own. timeSet is
	// the same for the meter.
	clefSet bool
	timeSet bool
	keySet  bool
	// time is the meter in force and clef the sign in force, so that a
	// change back to what the staff opened with is still written down.
	time model.TimeSignature
	clef model.Clef
	key  model.KeySignature
	meas *model.Measure
	// pendingDynamic waits for the next note, since NoteWorthy writes the
	// marking as an object of its own just before what it applies to.
	pendingDynamic model.Dynamic
	// pendingClef is a clef change written partway through a bar, waiting
	// for the note it is printed in front of.
	pendingClef *model.Clef
	// pendingFermata and pendingText are the pause and the words written
	// just before what they belong to, as NoteWorthy writes both.
	pendingFermata bool
	pendingText    []string
	// An octave bracket being waited on. NoteWorthy writes the word and
	// then draws the bracket after it with dashes, and only the hook on
	// the end says how far it reaches. Until that turns up there is
	// nothing to draw but the word, so the word is written as words and
	// taken back if the bracket turns out to be real.
	// slurOpen says the element before this one was joined onward, so a
	// note that is not closes the slur.
	slurOpen bool

	// A wedge being drawn, and the last note it has reached.
	pendingHairpin *model.Hairpin
	hairpin        *model.Hairpin
	hairpinLast    model.Element

	octaveWord   string
	octaveAmount int
	octaveOn     model.Element
	octaveLast   model.Element
	// chord is the chord being gathered and chordLeft how many of its
	// noteheads are still to come. A 1.70 chord is a record that says how
	// many notes it has and then lets them follow as ordinary note
	// records, so the notes after one belong to it rather than to the bar.
	chord     *model.Chord
	chordLeft int
	// grace holds the small notes read so far, which belong to the next
	// note rather than to the bar. They are kept across a barline: a piece
	// that leans into a downbeat writes them at the end of the bar before.
	grace []*model.Note
	// ending is the volta bracket of the bar before, and endingBar which
	// bar that was, so that a bar carrying the same number goes on with it
	// instead of opening another.
	ending    *model.Ending
	endingBar int
	// accs holds the accidentals written so far in the bar now being read,
	// by the note they were written on. They are cleared at the barline.
	accs map[[2]int]int
}

// nowClef and nowKey are the sign and the key in force, which is what a note
// is read against. Before the staff has said otherwise they are its own.
func (s *staffState) nowClef() model.Clef {
	if s.clefSet {
		return s.clef
	}
	return s.part.Clef
}

func (s *staffState) nowKey() model.KeySignature {
	if s.keySet {
		return s.key
	}
	return s.part.Key
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
	s.attachClef(el)
	s.meas.Voices[0].Elements = append(s.meas.Voices[0].Elements, el)
}

// attachClef puts a clef change waiting to be written in front of the element
// it belongs to.
func (s *staffState) attachClef(el model.Element) {
	m := marksOf(el)
	if m == nil {
		return
	}
	if s.pendingClef != nil {
		m.Clef = s.pendingClef
		s.pendingClef = nil
	}
	if s.pendingFermata {
		m.AddArticulation(model.Fermata)
		s.pendingFermata = false
	}
	for _, t := range s.pendingText {
		m.Texts = append(m.Texts, model.TextMark{Text: t, Italic: true})
	}
	s.pendingText = s.pendingText[:0]
	if s.pendingHairpin != nil {
		m.Hairpin = s.pendingHairpin
		s.pendingHairpin = nil
	}
	if s.hairpin != nil {
		s.hairpinLast = el
	}
	if s.octaveWord != "" && s.octaveOn == nil {
		// The word has found the note it is written over. If the bracket
		// after it is ever finished, this is where it begins.
		s.octaveOn = el
	}
	if s.octaveOn != nil {
		s.octaveLast = el
	}
}

// slurRun works out where a slur begins and ends from the one bit that says
// a note is joined to the next.
//
// From 1.75 the file marks only that, so the end of a phrase mark is not
// written anywhere: it is the first element after the run of joined ones.
func (s *staffState) slurRun(m *model.Marks, joined bool) {
	switch {
	case joined && !s.slurOpen:
		m.SlurStart = true
	case !joined && s.slurOpen:
		m.SlurStop = true
	}
	s.slurOpen = joined
}

// openHairpin starts a wedge, drawn from the next note on.
func (s *staffState) openHairpin(kind model.HairpinKind) {
	s.closeHairpin()
	s.pendingHairpin = &model.Hairpin{Kind: kind, Start: true}
	s.hairpin = s.pendingHairpin
}

// closeHairpin ends the wedge on the last note it reached.
func (s *staffState) closeHairpin() {
	if s.hairpin == nil {
		return
	}
	if m := marksOf(s.hairpinLast); m != nil {
		if m.Hairpin == s.hairpin {
			// A wedge over a single note opens and closes on it.
			m.Hairpin.Stop = true
		} else {
			m.Hairpin = &model.Hairpin{Kind: s.hairpin.Kind, Stop: true}
		}
	}
	s.hairpin, s.pendingHairpin, s.hairpinLast = nil, nil, nil
}

// openOctave remembers an octave word, in case a bracket is drawn after it.
func (s *staffState) openOctave(word string, amount int) {
	s.forgetOctave()
	s.octaveWord, s.octaveAmount = word, amount
	// Written as words for now. A word with no bracket after it is a
	// marking like any other, and most of them are: of the sixty-six in
	// Danse Macabre, twenty-four are followed by a bracket that finishes.
	s.pendingText = append(s.pendingText, word)
}

// closeOctave turns the word and the bracket drawn after it into one line
// over the notes it covers.
func (s *staffState) closeOctave() {
	on, last := marksOf(s.octaveOn), marksOf(s.octaveLast)
	if s.octaveWord == "" || on == nil || last == nil {
		s.forgetOctave()
		return
	}
	// The line draws its own label, so the word is taken back off the note
	// it was written on.
	for i, t := range on.Texts {
		if t.Text == s.octaveWord {
			on.Texts = append(on.Texts[:i], on.Texts[i+1:]...)
			break
		}
	}
	line := &model.OctaveShift{
		Amount: s.octaveAmount, Start: true, SoundsAsWritten: true,
	}
	on.Octave = line
	if s.octaveLast == s.octaveOn {
		line.Stop = true
	} else {
		last.Octave = &model.OctaveShift{
			Amount: s.octaveAmount, Stop: true, SoundsAsWritten: true,
		}
	}
	s.forgetOctave()
}

// forgetOctave gives up on a word whose bracket never arrived. The word
// stays where it was written.
func (s *staffState) forgetOctave() {
	s.octaveWord, s.octaveAmount = "", 0
	s.octaveOn, s.octaveLast = nil, nil
}

// marksOf is the marks an element carries, if it is the kind that carries
// any.
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

// openChord starts a chord of n noteheads, which the next n note records
// fill. The chord takes its place in the bar now, so that the notes landing
// in it do not have to be put back in order afterwards.
func (s *staffState) openChord(ch *model.Chord, n int) {
	s.endChord()
	s.ensure()
	s.attachClef(ch)
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

// openEnding marks the bar now being read as part of a volta bracket.
//
// NoteWorthy writes the ending on every bar the bracket covers rather than
// once at its head, so each bar closes its own bracket and a bar that
// carries the same number as the one before it goes on with it. Left to
// close at the next structural barline instead, the "2." of Für Elise's
// second repeat ran from bar 24 to the last bar of the piece — seventy bars
// under one bracket.
func (s *staffState) openEnding(n int) {
	s.ensure()
	here := len(s.part.Measures)
	e := &model.Ending{Numbers: []int{n}, Start: true, Stop: true}
	if s.ending != nil && s.endingBar == here-1 &&
		len(s.ending.Numbers) == 1 && s.ending.Numbers[0] == n {
		s.ending.Stop, s.ending.Open = false, false
		e.Start = false
	}
	s.meas.Ending = e
	s.ending, s.endingBar = e, here
}

func (s *staffState) closeMeasure(style model.BarlineStyle) {
	s.endChord()
	if s.meas == nil {
		return
	}
	// Nothing to go back for: a bracket that ends where the music carries
	// on is written without its closing hook.
	if e := s.meas.Ending; e != nil && e.Stop &&
		style != model.BarRepeatEnd && style != model.BarRepeatBoth {
		e.Open = true
	}
	s.meas.RightBarline = style
	s.part.Measures = append(s.part.Measures, s.meas)
	s.meas = nil
	// A written accidental lasts to the barline and no further.
	s.accs = nil
	// So does a wedge, give or take. NoteWorthy draws one from where it is
	// marked to the dynamic it arrives at — but a crescendo that is never
	// answered then runs for ever: measured across this piece the spans
	// come out a median of eleven bars and a longest of thirty-nine, which
	// is not a crescendo anybody wrote. Its real length is not in the file
	// in any form found so far, so a wedge is drawn for the bar it is
	// marked in. NoteWorthy's own at bar 244 carries a little past the
	// barline, so this is short rather than wrong.
	s.closeHairpin()
}

// pitch turns a staff position into a sounding pitch, under the clef and key
// in force and whatever accidentals this bar has already had.
//
// An accidental is written once and holds for the rest of the bar, which is
// a rule of notation rather than of this file format: NoteWorthy draws the
// sharp on the first F of the bar and on none of the others, and means all
// of them. Read note by note, the left hand of Danse Macabre played F sharp,
// F, F, F where the page says F sharp four times over.
func (s *staffState) pitch(step int, acc model.Accidental) model.Pitch {
	p := pitchFor(s.nowClef(), s.nowKey(), step, acc)
	key := [2]int{int(p.Step), p.Octave}
	if acc != model.NoAccidental {
		if s.accs == nil {
			s.accs = map[[2]int]int{}
		}
		s.accs[key] = p.Alter
		return p
	}
	if alter, ok := s.accs[key]; ok {
		p.Alter = alter
	}
	return p
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
// wideObject is the bit in an object's third byte that makes its record
// three bytes longer than the plain one of the same type.
//
// Found by walking the first staff of Danse Macabre — eighty-four thousand
// bytes and twenty-two text objects, each of which the walk has to land on
// exactly — and asking which records were the wrong length. Barlines and
// volta brackets came in two sizes: 02 00 08 00 00 and 02 00 20 00 00 01 FE
// 05, the same type, five bytes and eight. The bit is set on both of them
// and on nothing in the stretch that already read correctly, and honouring
// it took the walk from a third of the way down the staff to the end of it.
//
// What the three bytes hold is not known. They are not needed for anything
// gscore draws, so they are stepped over rather than read.
// nwcFermata is the Tempo Variance style that draws a pause, and
// nwcMusicFont the font index of NoteWorthy's own symbol font.
const (
	nwcFermata   = 2
	nwcMusicFont = 8
)

const wideObject = 0x20

func (w *walker) object(st *staffState) error {
	if w.i >= len(w.b) {
		return w.skip(1)
	}
	wide := w.dia().wideRecords && w.i+2 < len(w.b) && w.b[w.i+2]&wideObject != 0
	if err := w.objectBody(st); err != nil {
		return err
	}
	if wide {
		return w.skip(3)
	}
	return nil
}

func (w *walker) objectBody(st *staffState) error {
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
	if flags == 5 && w.dia().shortFlagRecord {
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
		if clef, ok := nwc1Clefs[c]; ok {
			// The first one is the staff's own; the rest are changes
			// written where they happen.
			//
			// A staff is not one clef. The left hand of Danse Macabre goes
			// up into the treble at bar 5 and stays there for pages, and
			// NoteWorthy draws the sign where it changes. Taking the last
			// clef in the piece for the staff's own made the whole staff
			// treble from its first bar; taking only the first threw every
			// change away, and the passages that change came out drawn an
			// octave and a half from where they belong.
			if !st.clefSet {
				st.part.Clef = clef
				st.clefSet = true
			} else if st.clef != clef {
				st.ensure()
				if len(st.meas.Voices[0].Elements) == 0 {
					// Nothing written in this bar yet, so the sign belongs
					// at its head.
					st.meas.Clef = &clef
				} else {
					// Partway through the bar, which is where NoteWorthy
					// writes it and where it has to be drawn: a change
					// hoisted to the head of the bar is read by the notes
					// in front of it as well, and they are then drawn
					// against a clef they were never written under. Danse
					// Macabre's left hand dives into the bass and back
					// inside a bar at 118 and 119, and the grace note
					// before each sign came out six ledger lines below the
					// staff.
					st.pendingClef = &clef
				}
			}
			st.clef = clef
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
		// 2.5 writes two bytes more here, and nothing in the first staff
		// of Danse Macabre says what is in them. Read short, the walk came
		// off the rails at the first key change and spent the rest of the
		// staff reading music as a run of clefs.
		if err := w.skip(w.dia().keyPad); err != nil {
			return err
		}
		// Each bit set is one altered note.
		sig := model.KeySignature{Fifths: bits(sharps) - bits(flats)}
		// The first is the staff's key and the rest are changes, as with
		// the meter and the clef. Written onto the part every time, the
		// last key in the piece became the key of its first bar — and
		// since an unmarked note takes its accidental from the key, every
		// note in the staff was read against whatever key it ended in.
		if !st.keySet {
			st.part.Key = sig
			st.keySet = true
		} else if st.key != sig {
			st.ensure()
			st.meas.Key = &sig
		}
		st.key = sig

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
		// A volta bracket — the "1." and "2." over the bars either side of
		// a repeat. The record is five bytes and the fourth is the number;
		// Für Elise writes it at the head of bars 8 and 9 and again at 23
		// and 24, in both staves, which is where NoteWorthy prints them.
		//
		// Read as two bytes of nothing before, the brackets were simply
		// missing from the page, and the repeat played the first ending
		// twice over instead of taking the second.
		n, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(1); err != nil {
			return err
		}
		if n >= 1 && n <= 8 {
			st.openEnding(int(n))
		}

	case 4: // instrument patch
		if w.dia().patchHasWords {
			// 2.5 writes a string in the middle of this one, so its length
			// is not fixed: eight bytes, the words, one byte, and then the
			// ladder of velocities a dynamic plays at.
			//
			// The words are what the mark draws. Danse Macabre's are "8 -
			// - - - - - ", which is the ottava line over bar 76, and a
			// plain "8" where the line is short. Read as a fixed eight
			// bytes the walk came out short by the whole string and read
			// the next object from the middle of this one — "unknown
			// object type 56", which is no type at all but the 0x38 that
			// begins "8".
			if err := w.skip(8); err != nil {
				return err
			}
			if _, err := w.cstring(); err != nil {
				return err
			}
			return w.skip(1 + 8)
		}
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
			sig := model.TimeSignature{
				Beats: top, BeatValue: 1 << bottom, Symbol: model.TimeNumeric,
			}
			// The first one is the staff's meter; the rest are changes
			// written where they happen.
			//
			// Every one of them used to be written onto the part, so the
			// last meter in the piece became the meter of its first bar.
			// Danse Macabre opens in two-four and turns into a waltz
			// partway down, and it opened as a waltz: three-four over four
			// bars holding a half note each, which is not a bar of music.
			if !st.timeSet {
				st.part.Time = sig
				st.timeSet = true
			} else if st.time != sig {
				st.ensure()
				st.meas.Time = &sig
			}
			st.time = sig
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
		if !w.dia().tempoHasWords {
			// 2.5 writes no words with the speed: the object ends at the
			// two bytes after the number.
			//
			// Read as a 2.0 tempo, the string it does not have was taken
			// out of whatever followed — a dynamic, a rest — and the walk
			// went on from the middle of it. Both tempos in the first
			// staff of Danse Macabre are nine bytes, the second of them
			// "a Tempo" at bar 79, where the words are a text object of
			// their own standing beside it.
			return w.skip(2)
		}
		// One byte between the speed and the words, not two.
		//
		// The byte is the note the speed is counted in. Taking two ate the
		// first letter of the words with it — "Adagio sostenuto" came back
		// as "dagio sostenuto" — and where a tempo has no words at all the
		// record is then read one byte long, which is enough to lose the
		// staff: Beethoven's Op. 27 has one at bar 68 and the walk went
		// into the middle of the next object and never recovered.
		if err := w.skip(1); err != nil { // the beat it is counted in
			return err
		}
		text, err := w.cstring()
		if text != "" {
			st.pendingText = append(st.pendingText, text)
		}
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
		//
		// It also ends a wedge: a crescendo is drawn from where it starts
		// to the dynamic it arrives at, which is the only thing in the
		// file that says how far it reaches.
		st.closeHairpin()
		st.pendingDynamic = nwc1Dynamics[level]

	case 8, 9: // note, rest
		nv, err := w.noteValue(objType == 9)
		if err != nil {
			return err
		}
		// A 1.70 note carries two bytes more than a 1.75 one — and a 1.70
		// rest does not, which is the kind of thing only the file can tell
		// you. Read as eight, every note left two bytes of itself behind
		// and the walk was two further out of step with each one; read as
		// ten for rests as well, every rest ate the two bytes after it and
		// the time signature arrived as 8/0.
		if pad := w.dia().notePad; objType == 8 && pad > 0 {
			if err := w.skip(pad); err != nil {
				return err
			}
		}
		if objType == 9 {
			r := &model.Rest{Dur: nv.dur}
			r.Tuplet = tupletMark(nv.triplet)
			r.Beam = beamMark(nv.beam)
			// A rest reads the slur bit like anything else, so a phrase
			// mark that does not carry over it ends there. Skipped, the
			// run stayed open across whatever silence followed: a layer of
			// Danse Macabre that plays one chord at bar 494 and then rests
			// held its slur to bar 666.
			if !w.dia().slurHasBothEnds {
				st.slurRun(&r.Marks, nv.slurTo)
			}
			st.add(r)
			return nil
		}
		n := &model.Note{
			Pitch:      st.pitch(nv.step, nv.acc),
			Dur:        nv.dur,
			Accidental: nv.acc,
		}
		n.TieStart = nv.tie
		n.Tuplet = tupletMark(nv.triplet)
		n.Beam = beamMark(nv.beam)
		// A slur belongs to its two ends. The file marks every note under
		// it, so the ends are the note that is joined onward without being
		// joined back, and the note that is joined back without going on.
		if w.dia().slurHasBothEnds {
			n.SlurStart = nv.slurTo && !nv.slurFrom
			n.SlurStop = nv.slurFrom && !nv.slurTo
		} else {
			st.slurRun(&n.Marks, nv.slurTo)
		}
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
		if nv.grace {
			st.grace = append(st.grace, n)
			return nil
		}
		if len(st.grace) > 0 {
			n.Grace = &model.GraceGroup{Notes: st.grace}
			// A slur that begins on a small note belongs to the phrase the
			// note it ornaments is in. Left where it was it would be lost,
			// since the engraver draws spans along the bar and the small
			// notes are not in it — and the note that closed the slur
			// would be closing one that never opened.
			for _, g := range st.grace {
				if g.SlurStart && !n.SlurStop {
					n.SlurStart = true
				}
				g.SlurStart, g.SlurStop = false, false
			}
			st.grace = nil
		}
		// A chord that is still waiting for noteheads gets this one only if
		// the note says it is one. A count this reader read wrongly would
		// otherwise swallow the melody behind the chord, which is the kind
		// of mistake that is invisible on the page until the bar is played.
		if st.chordLeft > 0 && !nv.chordNote {
			st.endChord()
		}
		st.add(n)

	case 10: // chord
		// A chord opens with the same block a note carries, and in 1.70
		// that block is ten bytes rather than eight — just as a note's is,
		// and so is each notehead after it. Changing one without the other
		// is worse than changing neither: with a ten-byte opening and
		// eight-byte noteheads every chord came out two bytes short for
		// each note in it, and four more bars were lost than gained.
		if w.dia().chordNotesFollow {
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
			nv, cerr := w.noteValue(false)
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
					Pitch:      st.pitch(nv.step, nv.acc),
					Dur:        nv.dur,
					Accidental: nv.acc,
				}
				n.TieStart = nv.tie
				n.Tuplet = tupletMark(nv.triplet)
				n.Beam = beamMark(nv.beam)
				st.add(n)
				w.flatChords++
				return nil
			}
			ch := &model.Chord{Dur: nv.dur}
			ch.Tuplet = tupletMark(nv.triplet)
			ch.Beam = beamMark(nv.beam)
			if !w.dia().slurHasBothEnds {
				st.slurRun(&ch.Marks, nv.slurTo)
			}
			// The marks are written on the chord's own record, not on the
			// noteheads that follow it, and belong to the chord as a whole:
			// NoteWorthy prints one staccato dot under a chord, not one
			// under each of its notes.
			if nv.staccato {
				ch.AddArticulation(model.Staccato)
			}
			if nv.accent {
				ch.AddArticulation(model.Accent)
			}
			st.openChord(ch, count)
			return nil
		}
		// The same block a note carries, read the same way — not skipped
		// over as eight bytes.
		//
		// It is eight bytes until the accidental byte says it is nine, and
		// a chord whose own block said so left the walk one byte short:
		// the notehead count was read from the middle of the block, came
		// back as ten, and ten noteheads were taken out of the music after
		// it. In Danse Macabre that is bar 180 or so, where the walk
		// stopped with "unrecognised duration byte 0x08" — a type byte
		// read as a duration.
		head, err := w.noteValue(false)
		if err != nil {
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
		// The chord's own record carries the beam, the triplet and the
		// marks; its noteheads only carry pitches.
		ch.Beam = beamMark(head.beam)
		ch.Tuplet = tupletMark(head.triplet)
		st.slurRun(&ch.Marks, head.slurTo)
		if head.staccato {
			ch.AddArticulation(model.Staccato)
		}
		if head.accent {
			ch.AddArticulation(model.Accent)
		}
		for i := 0; i < int(count); i++ {
			if err := w.skip(3); err != nil {
				return err
			}
			nv, err := w.noteValue(false)
			if err != nil {
				return err
			}
			if i == 0 {
				ch.Dur = nv.dur
			}
			n := &model.Note{
				Pitch:      st.pitch(nv.step, nv.acc),
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
		// A fermata is one of these.
		//
		// NoteWorthy has no fermata of its own: the pause over the last
		// chord of a phrase is a Tempo Variance whose style says Fermata,
		// written as an object just before the note it sits over. Bar 30
		// of Danse Macabre is a whole rest under one, and the rest came
		// out bare.
		// Where it sits and how it is placed come first; the style is the
		// third byte of the four.
		if err := w.skip(2); err != nil {
			return err
		}
		style, err := w.u8()
		if err != nil {
			return err
		}
		if err := w.skip(1); err != nil {
			return err
		}
		if style == nwcFermata {
			st.pendingFermata = true
		}
		return nil
	case 15: // dynamic variance
		// A crescendo or a diminuendo: the wedge under the staff that
		// says the music grows or fades. It was stepped over, so no
		// NoteWorthy score gscore opened has ever had one in it.
		//
		// Three bytes: where it sits, how it is placed, and which way it
		// goes. Which way is the third, and the corpus says so without
		// having to be told: a crescendo runs from a softer dynamic to a
		// louder one, and of the sixty-two marked 0 that are followed by
		// another dynamic, fifty-three get louder. Of the sixteen marked
		// 1, every single one gets softer.
		if err := w.skip(2); err != nil {
			return err
		}
		style, err := w.u8()
		if err != nil {
			return err
		}
		kind, ok := nwc1Hairpins[style]
		if !ok {
			// Styles beyond the wedges — the ones NoteWorthy writes as
			// words — have nothing to draw a wedge for.
			st.closeHairpin()
			return nil
		}
		st.openHairpin(kind)
		return nil
	case 16: // performance style
		return w.skip(3)
	case 17: // text
		// The words written beside the music: "Mouvement modéré de
		// Valse", "marcato", "una corda". They were read and thrown away,
		// so every direction a composer wrote on the page was missing from
		// it.
		//
		// The font the words are drawn in is the byte before them. The
		// first eight slots hold type and the rest hold symbols, in which
		// a staff author draws what NoteWorthy has no object for — an
		// octave line is the word "8va" in type and then a row of 0xF2 in
		// a music font, which is a dash. Printed as text that is a line of
		// o-graves across the page, so the strokes are read as the bracket
		// they are and nothing is printed for them.
		if err := w.skip(2); err != nil {
			return err
		}
		font, err := w.u8()
		if err != nil {
			return err
		}
		text, err := w.cstring()
		if err != nil {
			return err
		}
		// An octave bracket is the word "8va" in type and then runs of
		// dashes in a music font, finishing with a hook that turns down
		// towards the staff. The dashes themselves are strokes and are
		// not printed as text — a line of o-graves across the page is
		// what that gives. What they are read for is the hook: it is the
		// only thing in the file that says how far the bracket reaches,
		// and Danse Macabre's first one reaches bar 29.
		if w.symbolFont(font) {
			if dashes, ends := octaveDashes(text); dashes && ends {
				st.closeOctave()
			}
			return nil
		}
		if !looksLikeText(text) {
			return nil
		}
		if n, ok := octaveAmount(text); ok {
			st.openOctave(text, n)
			return nil
		}
		st.pendingText = append(st.pendingText, text)
		return nil
	case 18: // rest chord
		// A rest that NoteWorthy keeps as a chord, because the layer it
		// is in has notes elsewhere in the bar. It holds time like any
		// other rest, and stepped over its bar came out short: the left
		// hand of Danse Macabre at bar 254 is one of these and five
		// quavers, and without it the bar is an eighth light.
		//
		// 2.0 and later, where the block has been read and checked. In
		// 1.75 the same change costs a bar in two files of the corpus, so
		// what the record holds there is still to be found out.
		if !w.dia().restChordHoldsTime {
			return w.skip(8 + 2)
		}
		nv, err := w.noteValue(true)
		if err != nil {
			return err
		}
		r := &model.Rest{Dur: nv.dur}
		r.Tuplet = tupletMark(nv.triplet)
		st.add(r)
		return w.skip(2)

	case 20:
		// A type 2.5 writes that the older versions do not, and that
		// nothing in nwc1.posa describes. The second staff of Danse
		// Macabre has one between two bars of silence, holding 0x03E8 —
		// a round thousand — and whatever that is, nothing on the page
		// depends on it. Four bytes after the header, which is what makes
		// the rest of the staff read.
		return w.skip(4)

	case 22:
		// Another of 2.5's own, two bytes after the header. It stands just
		// before the words "Mouvement modéré de Valse" on the staff that
		// is printed, and like the type 20 beside it nothing gscore draws
		// needs what it holds.
		return w.skip(2)

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

// pitchFor turns a staff position into a pitch, against the clef and key in
// force where it is written.
//
// Not against the staff's own: NoteWorthy stores where on the staff a note
// sits, and what note that is depends on the sign at the head of the line.
// Read against the clef the staff opened with, the left hand of Danse Macabre
// came out an octave and a half below where it is written for the fifty bars
// it spends up in the treble — and the page showed it there, piled under the
// staff on ledger lines, with a treble clef in front of it saying otherwise.
func pitchFor(clef model.Clef, key model.KeySignature, step int, acc model.Accidental) model.Pitch {
	pitch := model.PitchFromDiatonicIndex(step - clefOffset(clef))
	if acc != model.NoAccidental {
		pitch.Alter = acc.Semitones()
	} else {
		pitch.Alter = key.AlterFor(pitch.Step)
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
		// A typeface is a short printable name. Checking it keeps the
		// search from accepting a run of empty strings in a pad field,
		// which is the one other place a dozen NULs sit together.
		//
		// It is also what ends the list in a file that writes no marker,
		// which Op. 111 does: twelve good records and then something that
		// is not one. The walk is put back to where that record began, so
		// the caller starts on the byte after the last font either way.
		at := w.i
		name, err := w.cstring()
		if err != nil || !looksLikeTypeface(name) || w.skip(4) != nil {
			w.i = at
			return n >= minFonts
		}
	}
	return false
}

// hasLetterOrDigit says whether a staff name has anything in it a name is
// made of.
//
// A staff is named for somebody to read, so there is a letter or a number in
// it somewhere — "S" and "1st violins" both qualify. An object stream makes
// the four-strings-in-a-row shape by accident here and there, and the names
// those accidents carry are single punctuation marks: Beethoven's Op. 27
// came back with staves called \", !, * and %, each holding a few bars of
// the staff it was found inside.
func hasLetterOrDigit(s string) bool {
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			return true
		case c >= 0xC0:
			// An accented letter, which Latin-1 puts up here.
			return true
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

// looksLikeAnotherStaff says whether a staff header is still to come,
// without moving the walk.
func (w *walker) looksLikeAnotherStaff() bool {
	probe := &walker{b: w.b, i: w.i, version: w.version}
	return probe.findStaffHeader()
}

// atStaffHeader says whether the walk is standing on the start of a staff.
//
// Four printable strings in a row — the name, the group, the instrument and
// the sound — and then bytes that are not another string. It is a weak
// signature taken alone and a strong one taken here, where the alternative
// is reading an object stream as text.
func (w *walker) atStaffHeader() bool {
	// As many names as this version writes, and no more.
	//
	// Four was right for 2.0 and wrong for 1.75, which writes two: the
	// staff's own name and its group, and then its settings. Asked for
	// four, a real 1.75 header fails the test as soon as the settings
	// happen not to read as text — Beethoven's Op. 27 has "RH2" and
	// "Standard" followed by a 0x04, and the walk went straight past the
	// staff and spent the next ten thousand bytes looking for one.
	probe := &walker{b: w.b, i: w.i}
	for n := 0; n < w.dia().headerNames; n++ {
		s, err := probe.cstring()
		if err != nil {
			return false
		}
		if n == 0 && (s == "" || !looksLikeText(s) || !hasLetterOrDigit(s)) {
			// A staff is named for somebody to read, so its name starts
			// with something printable. Danse Macabre's object stream
			// makes the four-strings-in-a-row shape by accident here and
			// there, and every one of those accidents is called "\t": read
			// as staves they added three to a six-staff score, one of them
			// with three hundred and ninety-three bars of someone else's
			// music in it.
			//
			// Printable rather than a letter, because a name that opens
			// with a digit is ordinary — "1st violins".
			return false
		}
		if len(s) > 48 || (s != "" && !looksLikeText(s)) {
			return false
		}
	}
	// A staff has a good deal more in it than its names.
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
			w.i = nameStart(w.b, at)
			return true
		}
	}
	return false
}

// nameStart trims the odd bytes a search can pick up in front of a staff's
// name.
//
// The search stops at the first place that makes the header shape, and a
// stray printable byte or two before the name makes that shape just as well.
// Danse Macabre's fifth staff was found two bytes early and came back called
// "0øPiano RH1 Display" — the name right but everything measured from it two
// bytes out, which lost the staff after it as well.
//
// A staff name is plain ASCII in every file in the corpus — "Piano RH1
// Display", "1st violins" — so the walk steps forward to the first start
// that gives one of those and is still a staff header. "0øPiano RH1 Display"
// is not one; the name two bytes later is.
//
// Measured from the name, not trimmed afterwards: everything in the staff is
// laid out relative to where its header begins, so two bytes of rubbish in
// front of the name costs the whole staff and the one after it, not just the
// look of a label.
func nameStart(b []byte, at int) int {
	for i := at; i < at+8 && i < len(b); i++ {
		probe := &walker{b: b, i: i}
		if !probe.atStaffHeader() {
			break
		}
		name := &walker{b: b, i: i}
		s, err := name.cstring()
		if err != nil {
			break
		}
		if plainASCII(s) {
			return i
		}
	}
	return at
}

// plainASCII says whether every character of a string is an ordinary
// printable one.
func plainASCII(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
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

// mergeLayers puts the staves that asked to share one staff onto one part.
//
// NoteWorthy writes a pianist's two hands as four staves when each hand has
// two independent lines, and marks the first of each pair "layer with next
// staff". The page shows two staves; the file holds four. Read one part per
// staff, Danse Macabre came out as four grand staves of the same music
// stacked up the page, where NoteWorthy prints one.
//
// A layer becomes a voice, which is what a voice is for: its own stems, its
// own rests, the same bar. Everything a measure carries besides its notes —
// the barline that closes it, a change of meter or key or clef — belongs to
// the bar rather than to the line, so the first staff of the pair keeps its
// own and the rest only add their voices.
func mergeLayers(parts []*model.Part, shares []bool, groups []string) ([]*model.Part, []string) {
	if len(shares) != len(parts) || len(groups) != len(parts) {
		return parts, groups
	}
	out := make([]*model.Part, 0, len(parts))
	kept := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		p, g := parts[i], groups[i]
		for i+1 < len(parts) && shares[i] && parts[i+1].Hidden == p.Hidden {
			layerOnto(p, parts[i+1])
			i++
			if g == "" {
				g = groups[i]
			}
		}
		out = append(out, p)
		kept = append(kept, g)
	}
	return out, kept
}

// hidePlayOnly takes the staves that are only played off the page.
//
// See the note in staff() on the byte this reads. The rule is the whole of
// it: where no staff says it is printed, every staff is, because a file that
// has never been told which staves to print is not a file with nothing on
// its page.
func hidePlayOnly(parts []*model.Part, printed map[*model.Part]bool) {
	any := false
	for _, p := range parts {
		if printed[p] {
			any = true
			break
		}
	}
	if !any {
		return
	}
	for _, p := range parts {
		if !printed[p] {
			// Hidden takes the staff off the page and out of what is
			// played, and the printed staves carry the same music, so the
			// piece still sounds.
			p.Hidden = true
		}
	}
}

// nwcDefaultGroup is the name NoteWorthy files carry when nobody has filed
// the staff under anything. It is not an instrument group and nothing is
// drawn round it.
const nwcDefaultGroup = "Standard"

// staffGroups turns the staves' group names into what is drawn down the left
// of them.
//
// A staff with no name of its own belongs to the group above it, which is how
// NoteWorthy writes a grand staff: "Piano" on the right hand and nothing on
// the left, because the word labels the pair rather than the staff. Read as
// one group per staff, Danse Macabre had no brace at all and its two staves
// stood apart on the page as if they were two instruments.
//
// A brace for two staves and a bracket for more: two staves under one name
// are one player's hands, and more than two are a section.
func staffGroups(parts []*model.Part, groups []string, braces map[*model.Part]int) []model.StaffGroup {
	if len(groups) != len(parts) {
		return nil
	}
	var out []model.StaffGroup
	name, first := "", -1
	flush := func(last int) {
		if first >= 0 && last > first && name != "" && name != nwcDefaultGroup {
			g := model.StaffGroup{
				First: first, Last: last, Name: name,
				Symbol: model.GroupBracket,
			}
			if last-first == 1 {
				g.Symbol, g.Barlines = model.GroupBrace, true
			}
			out = append(out, g)
		}
		name, first = "", -1
	}
	for i, p := range parts {
		if p.Hidden {
			// Not on the page, so nothing is drawn beside it, and it does
			// not hold a group open across the staves that are.
			flush(i - 1)
			continue
		}
		if g := groups[i]; g != "" {
			flush(i - 1)
			name, first = g, i
			continue
		}
		if first < 0 {
			continue
		}
	}
	flush(len(parts) - 1)
	return append(out, grandStaves(parts, braces, out)...)
}

// grandStaves braces the pairs that said so in their own settings and that no
// group name has already covered.
//
// Most piano files name the group — "Piano" on the right hand, nothing on the
// left — and the pass above is enough. Moonlight Sonata names nothing: all
// four of its staves are filed under NoteWorthy's default group, and the two
// that are left after the layers are merged stood apart on the page with no
// brace and no barline between them, which is two instruments rather than one
// pianist. What the file does say is in the settings of each staff: the right
// hand calls itself the upper half of a grand staff and the left hand the
// lower.
func grandStaves(parts []*model.Part, braces map[*model.Part]int, have []model.StaffGroup) []model.StaffGroup {
	covered := func(i int) bool {
		for _, g := range have {
			if i >= g.First && i <= g.Last {
				return true
			}
		}
		return false
	}
	var out []model.StaffGroup
	for i := 0; i+1 < len(parts); i++ {
		if braces[parts[i]] != braceUpper || braces[parts[i+1]] != braceLower {
			continue
		}
		if parts[i].Hidden || parts[i+1].Hidden || covered(i) || covered(i+1) {
			continue
		}
		out = append(out, model.StaffGroup{
			First: i, Last: i + 1,
			Symbol: model.GroupBrace, Barlines: true,
		})
		i++
	}
	return out
}

// layerOnto adds every voice of one staff to the bars of another.
//
// A bar one layer writes exactly as another already has is not added twice.
// The sustained note under a run is written in both layers of Danse
// Macabre's right hand, and NoteWorthy prints it once: two layers in unison
// are one line on the page. Added twice it came out as two half notes side
// by side in every one of the first four bars, which is not what is written
// and not what is played.
func layerOnto(p, q *model.Part) {
	for len(p.Measures) < len(q.Measures) {
		p.Measures = append(p.Measures, &model.Measure{Voices: []*model.Voice{{}}})
	}
	for i, m := range q.Measures {
		to := p.Measures[i]
		// A change written at the head of the bar belongs to the bar, not
		// to the line: whichever layer carries it, the merged staff needs
		// it. The first layer's own wins where both have one.
		if to.Clef == nil {
			to.Clef = m.Clef
		}
		if to.Key == nil {
			to.Key = m.Key
		}
		if to.Time == nil {
			to.Time = m.Time
		}
		for _, v := range m.Voices {
			// A line that only repeats one already here adds no notehead
			// to the page — but if it is carrying a slur, a wedge or a
			// change of clef, dropping it drops those too, and a slur
			// whose end goes missing is drawn on to the end of the piece.
			// Kept, its notes become hidden rests below and the marks stay
			// where they were written.
			if !hasVoice(to, v) || voiceHoldsMarks(v) {
				to.Voices = append(to.Voices, v)
			}
		}
		hideUnisons(to)
		emptySilentVoices(to)
		hideCoveredRests(to)
	}
}

// hideCoveredRests takes the rests off the page where another line is already
// playing through them.
//
// Two layers on one staff share its space, and NoteWorthy prints the notes
// and leaves out the rest that sits under them: a layer that holds its place
// for half a bar while the other plays does it with a rest nobody sees. Bar
// 254 of Danse Macabre is one — a minim rest under two crotchets — and
// printed it sat in the middle of the music where the page has nothing.
//
// Hidden rather than thrown away. The rest is what holds the rest of the
// layer in time, and a line that loses it plays everything after it early.
func hideCoveredRests(m *model.Measure) {
	type span struct{ from, to model.Duration }
	var plays []span
	for _, v := range m.Voices {
		at := model.Duration{Num: 0, Den: 1}
		for _, e := range v.Elements {
			d := e.Duration()
			if _, ok := e.(*model.Rest); !ok {
				plays = append(plays, span{at, at.Add(d)})
			}
			at = at.Add(d)
		}
	}
	for _, v := range m.Voices {
		at := model.Duration{Num: 0, Den: 1}
		for _, e := range v.Elements {
			d := e.Duration()
			if r, ok := e.(*model.Rest); ok {
				end := at.Add(d)
				for _, s := range plays {
					if s.from.Cmp(end) < 0 && at.Cmp(s.to) < 0 {
						r.Hidden = true
						break
					}
				}
			}
			at = at.Add(d)
		}
	}
}

// emptySilentVoices clears the lines that only rest, in a bar where another
// line plays, leaving the line itself where it was.
//
// Cleared rather than taken out. Taking it out moves every line below it up
// one, and a line that is voice one in the bar where a slur opens and voice
// nought in the bar where it closes is two different lines as far as the
// drawing is concerned: the slur opens and is never closed. In Danse Macabre
// the left hand's second layer is slurred from bar 494 to 497 and the first
// layer rests through 495, and that one bar of silence turned a four-bar
// phrase mark into one that ran to the end of the piece.
//
// Cleared rather than left with its rests hidden, because a bar of hidden
// rests still asks for the room to put them in: left in, they spread this
// piece over eight pages more than it needs.
//
// A bar in which every line rests keeps its rest, because a bar has to hold
// something. So does a line carrying a slur, a wedge or a change of clef for
// the music around it.
func emptySilentVoices(m *model.Measure) {
	plays := false
	for _, v := range m.Voices {
		if voicePlays(v) {
			plays = true
			break
		}
	}
	if !plays {
		return
	}
	for _, v := range m.Voices {
		if !voicePlays(v) && !voiceHoldsMarks(v) {
			v.Elements = nil
		}
	}
}

// closeDanglingSpans ends the slurs and wedges a staff ran out before
// closing.
//
// The file marks where a thing begins and leaves the end to be worked out,
// so a staff that stops mid-phrase leaves one open — and an open spanner is
// drawn to the right margin of its system and of every system after it, to
// the end of the piece. Danse Macabre had a slur like that from bar 168 to
// 317 and three wedges running hundreds of bars.
//
// Ended on the last note the line has, which is what the page shows anyway.
// Per voice, because that is how a spanner is held open while drawing: a
// mark left in one line is never closed by another.
func closeDanglingSpans(p *model.Part) {
	most := 0
	for _, m := range p.Measures {
		if len(m.Voices) > most {
			most = len(m.Voices)
		}
	}
	for vi := 0; vi < most; vi++ {
		var slurAt, wedgeAt *model.Marks
		var wedge model.HairpinKind
		var last *model.Marks
		// close ends whatever is open on the note before a new one starts,
		// and at the end of the line.
		closeWedge := func(on *model.Marks) {
			if wedgeAt == nil || on == nil {
				return
			}
			// A wedge that opens on the last note there is covers that note
			// and no more, which the model says with both ends on it.
			if on == wedgeAt {
				wedgeAt.Hairpin.Stop = true
				wedgeAt = nil
				return
			}
			if on.Hairpin == nil {
				on.Hairpin = &model.Hairpin{Kind: wedge, Stop: true}
			} else {
				on.Hairpin.Stop = true
			}
			wedgeAt = nil
		}
		for _, m := range p.Measures {
			if vi >= len(m.Voices) {
				continue
			}
			for _, e := range m.Voices[vi].Elements {
				mk := marksOf(e)
				if mk == nil {
					continue
				}
				if mk.SlurStop {
					slurAt = nil
				}
				if mk.SlurStart {
					// A second slur where the first never ended: they meet
					// on this note, which ends the one before it and begins
					// the one after — which is how a run of slurs over one
					// phrase is written, and what the engraver expects.
					if slurAt != nil && mk != slurAt {
						mk.SlurStop = true
					}
					slurAt = mk
				}
				if h := mk.Hairpin; h != nil {
					if h.Stop {
						wedgeAt = nil
					}
					if h.Start {
						// A second wedge where the first never ended: the
						// first ends on the note before this one.
						closeWedge(last)
						wedgeAt, wedge = mk, h.Kind
					}
				}
				last = mk
			}
		}
		if slurAt != nil && last != nil {
			// Likewise a slur: over the one note if that is all it reached.
			last.SlurStop = true
		}
		closeWedge(last)
	}
}

// holdsASpan says whether an element is one end of something that reaches
// across the music, and so cannot be moved to another line on its own.
func holdsASpan(el model.Element) bool {
	m := marksOf(el)
	if m == nil {
		return false
	}
	return m.SlurStart || m.SlurStop || m.TieStart ||
		m.Hairpin != nil || m.Octave != nil || m.Trill != nil
}

// mergeMarks moves onto a notehead what the copy of it was carrying.
//
// The words beside the music, a change of clef and a dynamic belong to the
// line rather than to the note; the note is only where they are written
// down. Two layers playing the same thing are one notehead, so whichever of
// them wrote the mark, it belongs to that one.
//
// Not the ends of a slur or a wedge, which cannot be moved on their own —
// an element carrying one of those is not hidden in the first place. Nor the
// articulations or the beam, which are already on the notehead that stays.
func mergeMarks(to, from *model.Marks) {
	if to == nil || from == nil {
		return
	}
	if to.Clef == nil {
		to.Clef = from.Clef
	}
	if to.Dynamic == model.DynamicNone {
		to.Dynamic = from.Dynamic
	}
	to.Texts = append(to.Texts, from.Texts...)
}

// holdsAMark says whether a rest is carrying something for the line it is
// in, so that taking it off the page would take that with it.
func holdsAMark(el model.Element) bool {
	m := marksOf(el)
	if m == nil {
		return false
	}
	return m.SlurStart || m.SlurStop || m.TieStart || m.Hairpin != nil ||
		m.Octave != nil || m.Trill != nil || m.Clef != nil ||
		m.Dynamic != model.DynamicNone || len(m.Texts) > 0
}

// voiceHoldsMarks says whether a line of rests is carrying anything for the
// music around it, which a line of plain silence is not.
func voiceHoldsMarks(v *model.Voice) bool {
	for _, e := range v.Elements {
		if holdsAMark(e) {
			return true
		}
	}
	return false
}

// voicePlays says whether a line has anything in it but rests.
func voicePlays(v *model.Voice) bool {
	for _, e := range v.Elements {
		switch e.(type) {
		case *model.Rest:
		default:
			return true
		}
	}
	return false
}

// hasVoice says whether a bar already holds this line of music.
func hasVoice(m *model.Measure, v *model.Voice) bool {
	want := voiceKey(v)
	for _, got := range m.Voices {
		if voiceKey(got) == want {
			return true
		}
	}
	return false
}

// voiceKey is what a voice puts on the page: the notes, their pitches and
// how long they last, in order.
func voiceKey(v *model.Voice) string {
	var b strings.Builder
	for _, e := range v.Elements {
		b.WriteString(elementKey(e))
	}
	return b.String()
}

// elementKey is what one element puts on the page.
func elementKey(e model.Element) string {
	var b strings.Builder
	{
		switch x := e.(type) {
		case *model.Note:
			fmt.Fprintf(&b, "n%s%d%+d/%v;", x.Pitch.Step, x.Pitch.Octave, x.Pitch.Alter, x.Dur)
		case *model.Rest:
			fmt.Fprintf(&b, "r%v;", x.Dur)
		case *model.Chord:
			b.WriteString("c")
			for _, n := range x.Notes {
				fmt.Fprintf(&b, "%s%d%+d,", n.Pitch.Step, n.Pitch.Octave, n.Pitch.Alter)
			}
			fmt.Fprintf(&b, "/%v;", x.Dur)
		default:
			fmt.Fprintf(&b, "%T;", e)
		}
	}
	return b.String()
}

// hideUnisons takes the second copy of a note off the page where two layers
// play the same thing at the same moment.
//
// Layered staves share the staff, so a note both of them hold is one
// notehead: NoteWorthy prints it once. Bar 149 of Danse Macabre is a layer
// whose first chord is its own and whose other four are the layer below it
// note for note, and printed as written that was every chord drawn twice
// with two stems through it.
//
// A hidden rest takes the copy's place, because the time still has to be
// held: the line it belongs to goes on after it.
func hideUnisons(m *model.Measure) {
	if len(m.Voices) < 2 {
		return
	}
	// The notehead kept for each thing played, by the moment it is played.
	kept := map[string]*model.Marks{}
	mark := func(v *model.Voice, record bool) {
		now := model.Duration{Num: 0, Den: 1}
		for i, e := range v.Elements {
			d := e.Duration()
			if _, isRest := e.(*model.Rest); !isRest {
				id := fmt.Sprintf("%v|%s", now, elementKey(e))
				switch {
				case record:
					if _, seen := kept[id]; !seen {
						kept[id] = marksOf(e)
					}
				case kept[id] != nil:
					// A notehead holding one end of a slur or a wedge is
					// printed, copy or not. Both ends are written in the
					// same line, and hiding one of them moves it to the
					// line that keeps the notehead while the other stays
					// where it was: the slur then has an end in each and
					// closes in neither. One in Danse Macabre opened at
					// bar 168 and ran to 317 that way.
					//
					// An extra notehead where two layers meet under a
					// phrase mark is a smaller thing than a slur drawn
					// across half the piece.
					if holdsASpan(e) {
						break
					}
					// The copy goes, and what it was carrying — the words,
					// a change of clef, a dynamic — goes to the notehead
					// that stays.
					mergeMarks(kept[id], marksOf(e))
					v.Elements[i] = &model.Rest{Dur: d, Hidden: true}
				}
			}
			now = now.Add(d)
		}
	}
	mark(m.Voices[0], true)
	for _, v := range m.Voices[1:] {
		mark(v, false)
		mark(v, true)
	}
}
