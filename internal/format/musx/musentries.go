/*
 * musentries.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musx

import (
	"encoding/binary"
	"fmt"
)

/*
 * The entry pool: the notes themselves.
 *
 * This is the one part of the older format that was ever documented by the
 * people who wrote it — Coda published a preliminary note on it in 1996,
 * which is where the flag names below come from — and the one part that did
 * not change between the two eras. The 2001 files and the 2012 files lay the
 * music out identically and differ only in which way round they write their
 * numbers, so there is one reading of it here rather than two.
 *
 * An "entry", in Finale's words, is a note, a chord or a rest. The pool is a
 * flat array of fixed 38-byte slots, each tagged with the entry it belongs
 * to, and an entry with more than two notes in it runs on into further slots:
 *
 *	0-3    entnum        which entry this slot belongs to
 *	4-5    slot index    0 for an entry's first slot, then 1, 2, ...
 *
 * and then, in a first slot:
 *
 *	6-9    prev          the entry before, 0 if none
 *	10-13  next          the entry after, 0 if none
 *	14-15  dura          the written length, in EDUs; 1024 is a quarter
 *	16-17  pos           moved by hand, in EVPUs; not read
 *	18-21  entry flag
 *	22-23  extended flag; not read
 *	24-25  how many notes follow
 *	26-37  the first two notes
 *
 * while a later slot carries five notes from offset 6 and nothing else. A
 * note is six bytes: two of pitch and four of flags.
 */

const (
	musSlot      = 38
	musSlotHead  = 6 // entnum and slot index, on every slot
	musSlotNotes = 2 // notes in a first slot, after its fixed fields
	// Notes in each later slot: the whole of it but the header.
	musMoreNotes = (musSlot - musSlotHead) / musNote
	musFirstNote = 26
	musNote      = 6
	// An entry can carry twelve notes, which is what the 1996 note says
	// and what Finale's own chord editing allows.
	musMaxNotes = 12
)

// The entry flag. Only the bits this reader acts on are named; the rest say
// where supplementary records are, and there are a great many of them.
const (
	// musSetBit is on every legal entry. Nothing else in the pool has it,
	// which is most of what identifies the pool.
	musSetBit = 0x80000000
	// musNoteBit is on for a note and off for a rest — but see musEntries:
	// it is not what decides whether an entry has pitch in it.
	musNoteBit = 0x40000000
	// musFloatRest is a rest that sits at the middle of the staff. Having
	// no pitch, it carries no note record.
	musFloatRest = 0x01000000
	// musGraceNote is a note written small and played out of time.
	musGraceNote = 0x00800000
)

// The note flag.
const (
	musTieStart = 0x40000000
	musTieEnd   = 0x20000000
)

// The pitch and the alteration share two bytes, and the alteration is the low
// four bits of them.
const (
	musTCDAltBits  = 4
	musTCDAltSize  = 0x07 // how far from the key, in semitones
	musTCDAltBelow = 0x08 // and whether that is down rather than up
)

// musEntries reads the pool into the same entries the XML reader builds.
func musEntries(data []byte, order binary.ByteOrder) ([]entry, error) {
	var out []entry
	for at := 0; at+musSlot <= len(data); {
		count := int(order.Uint16(data[at+24:]))
		if count > musMaxNotes {
			return nil, fmt.Errorf(
				"the entry at %d says it holds %d notes, and the most one can "+
					"hold is %d", at, count, musMaxNotes)
		}

		flag := order.Uint32(data[at+18:])

		// There are three cases here and all three are real:
		//
		//   a note           the note bit is on, and the notes are there
		//   a floating rest  the floating-rest bit is on, and a note
		//                    record is stored anyway as a placeholder,
		//                    with nothing in it
		//   a moved rest     neither bit is on: dragging a rest off the
		//                    middle line turns the floating-rest bit off
		//                    and stores a real note record — not a pitch,
		//                    but how far down the staff to draw it
		//
		// So the note bit says whether this is a note, and the
		// floating-rest bit says whether there is a note record to read.
		// Deciding either question with the other bit turns every rest
		// somebody nudged into a note, which is a wrong note in a bar that
		// still adds up — the kind nobody finds by looking at the bar.
		notes := count
		if flag&musFloatRest != 0 {
			notes = 0
		}

		e := entry{
			Num:  int(order.Uint32(data[at:])),
			Next: int(order.Uint32(data[at+10:])),
			Dura: int(order.Uint16(data[at+14:])),
		}
		if flag&musNoteBit != 0 {
			// The XML writes this as an element that is either there or
			// not, and the reader downstream asks only whether it is.
			e.IsNote = &struct{}{}
		}
		for _, o := range musNoteOffsets(at, notes) {
			if o+musNote > len(data) {
				return nil, fmt.Errorf(
					"the notes of the entry at %d run past the end of the pool", at)
			}
			e.Notes = append(e.Notes, musNote6(data[o:], order))
		}
		out = append(out, e)

		at += musSlot * musSlotSpan(count)
	}
	return out, nil
}

// musNote6 reads one six-byte note record.
//
// The pitch is the top twelve bits of the first two, counted in steps of the
// scale from the tonic of the key, and the alteration is the bottom four.
//
// Those four bits are stored as a sign and a size, with bit three being the
// sign: 9 is a flat, not a double-flat-and-a-half. The 1996 note calls it "a
// signed quantity ... -8 to +7", which reads as the usual two's complement
// and is wrong — real files disagree with the document, and real files win.
// The two readings agree on everything up to a sharp, which is why this is
// the sort of mistake that survives a long time before anybody meets a flat.
func musNote6(rec []byte, order binary.ByteOrder) enigNote {
	tcd := order.Uint16(rec)
	flag := order.Uint32(rec[2:])

	alt := int(tcd & musTCDAltSize)
	if tcd&musTCDAltBelow != 0 {
		alt = -alt
	}
	return enigNote{
		HarmLev:  musSigned(int(tcd)>>musTCDAltBits, 16-musTCDAltBits),
		HarmAlt:  alt,
		TieStart: musFlag(flag&musTieStart != 0),
		TieEnd:   musFlag(flag&musTieEnd != 0),
	}
}

func musFlag(on bool) *struct{} {
	if on {
		return &struct{}{}
	}
	return nil
}

func musSigned(value, bits int) int {
	if value&(1<<(bits-1)) != 0 {
		return value - 1<<bits
	}
	return value
}

// musSlotSpan is how many slots one entry takes up.
//
// At least one even for a rest, which has no notes at all — otherwise a walk
// over the pool would never move.
func musSlotSpan(notes int) int {
	over := notes - musSlotNotes
	if over <= 0 {
		return 1
	}
	return 1 + (over+musMoreNotes-1)/musMoreNotes
}

// musNoteOffsets is where an entry's note records are, across its slots.
func musNoteOffsets(at, notes int) []int {
	var out []int
	for i := 0; i < notes && i < musSlotNotes; i++ {
		out = append(out, at+musFirstNote+musNote*i)
	}
	for left, slot := notes-musSlotNotes, 1; left > 0; left, slot = left-musMoreNotes, slot+1 {
		start := at + musSlot*slot + musSlotHead
		for i := 0; i < left && i < musMoreNotes; i++ {
			out = append(out, start+musNote*i)
		}
	}
	return out
}

// musLooksLikeEntries says whether a pool is the entry pool, read this way
// round.
//
// By what is in it rather than where it is, because the newer container does
// not label its pools. Every legal entry carries the set bit and a believable
// note count, and the slots have to tile the pool exactly with nothing over —
// which no other pool manages.
//
// It is strict enough to tell the two byte orders apart as well, which is
// what makes it safe to use on a pool whose container never said what it was.
func musLooksLikeEntries(data []byte, order binary.ByteOrder) bool {
	if len(data) == 0 || len(data)%musSlot != 0 {
		return false
	}
	at := 0
	for at+musSlot <= len(data) {
		if order.Uint16(data[at+4:]) != 0 {
			return false // a first slot is slot number zero
		}
		if order.Uint32(data[at+18:])&musSetBit == 0 {
			return false
		}
		count := int(order.Uint16(data[at+24:]))
		if count > musMaxNotes {
			return false
		}
		at += musSlot * musSlotSpan(count)
	}
	return at == len(data)
}

// musGraceNotes counts the entries written as grace notes.
//
// They are read as ordinary notes of their written length, which is not what
// they are, so they are worth saying out loud rather than leaving somebody to
// find an extra beat in a bar.
func musGraceNotes(data []byte, order binary.ByteOrder) int {
	n := 0
	for at := 0; at+musSlot <= len(data); {
		flag := order.Uint32(data[at+18:])
		if flag&musGraceNote != 0 {
			n++
		}
		at += musSlot * musSlotSpan(int(order.Uint16(data[at+24:])))
	}
	return n
}
