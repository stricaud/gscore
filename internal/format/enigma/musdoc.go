/*
 * musdoc.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package enigma

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

/*
 * From pools to a document.
 *
 * The two eras frame their records differently and name their record types
 * differently, and that is the whole of the difference: the fields inside a
 * record sit at the same offsets in both, because the binary layout of 2011
 * is the layout the ETF specification describes and the rows of 2001 are the
 * same fields in the same order. So the framing is read twice, once per era,
 * and the fields are read once.
 *
 *	2001-2005   fixed 16-byte rows carrying two-character tags, and a
 *	            record too big for one row runs on into more rows under the
 *	            same tag and key
 *	2011-2012   variable-length records that carry their own tag, key and
 *	            length, laid end to end with two-byte filler between
 *	            sections
 */

// musRecord is one logical record, however it was framed.
type musRecord struct {
	// what this record is, in the names the XML uses, so that nothing
	// downstream has to know which era it came from. A record whose type
	// this reader does not know has an empty name and is counted rather
	// than read.
	what string
	num  int // the cmper; for a details record, the first of two
	num2 int // the second, for a details record
	part int // 0 is the score itself; above that, an extracted part

	// Each incidence's bytes, in file order. A record that fits in one
	// incidence — which is most of them, and all of them in the newer era
	// bar a handful of frames — has one.
	parts [][]byte
}

// all is every incidence's bytes run together, which is how a field offset
// into a multi-row record is meant to be read.
func (r musRecord) all() []byte {
	if len(r.parts) == 1 {
		return r.parts[0]
	}
	return bytes.Join(r.parts, nil)
}

// last is the final incidence, which is where a frame keeps its entries.
func (r musRecord) last() []byte {
	if len(r.parts) == 0 {
		return nil
	}
	return r.parts[len(r.parts)-1]
}

// What this reader knows how to read. Everything else in the two pools is
// counted and reported, which for this format is most of what is in them.
//
// The older era names a record type with two characters, held as a number so
// that they come out back to front on a file written by a PC. The newer era
// numbers them. Only the types gscore builds a score out of are here.
var (
	musRowTags = map[string]string{
		"MS": "measSpec",
		"IS": "staffSpec",
		"FR": "frameSpec",
		"IU": "instUsed",
		"GF": "gfhold",
		"TX": "textBlock",
		"NG": "staffGroup",
		"DT": "textExprDef",
		"DY": "measExprAssign",
		"De": "exprEnclosure",
	}
	musNumTags = map[int]string{
		176:  "measSpec",
		231:  "staffSpec",
		146:  "frameSpec",
		1044: "gfhold",
	}
)

const (
	// A row of the older pools, and how much of it is data.
	musRow           = 16
	musRowOthersData = 4 // after cmper and tag
	// What one row of the older `others` pool carries, which is also the
	// size of one slot of a record written as an array of them.
	musRowOthersSlot  = musRow - musRowOthersData
	musRowDetailsData = 6 // after both cmpers and the tag
	// Headers in the newer pools: tag, key or keys, and a length.
	musRecOthersHead  = 10
	musRecDetailsHead = 12
	// What follows a record's payload is the length of a further block,
	// which is zero on almost every record and was taken for a fixed
	// trailer for a long time because of it.
	musRecExtra = 4

	// Caps, to bound a walk over somebody else's file.
	musMaxRows       = 1 << 20
	musMaxIncidences = 4096
	musMaxRecords    = 1 << 21
	musMaxPayload1   = 1 << 21 // one record's payload

	// Where a frame keeps the number of the entry chain for each layer.
	// It moved between 2001 and 2005 and stayed put after.
	musFrameBase2001 = 4
	musFrameBase2005 = 6
	musFrameLayers   = 4

	// Field offsets, shared by both eras.
	musMeasKey     = 2
	musMeasBeats   = 4
	musMeasDivBeat = 6
	musMeasFlags   = 10
	musStaffTrans  = 20
	musStaffName   = 30
	musStaffAbbrv  = 32
	musGroupStart  = 0
	musGroupEnd    = 2
	musGroupShape  = 10
	musHoldClefID  = 0

	// An expression: what it is called, what it does to playback, and the
	// words the library describes it by.
	musExprPlay  = 11
	musExprWords = 36
	// Its enclosure, where it has one: the box drawn round a rehearsal
	// mark. The top bit of the flags byte is a box that is not drawn.
	musEnclFlags  = 10
	musEnclHidden = 0x80
	// One place an expression is written: which definition, and on which
	// staff of the bar the record is keyed to.
	musPlaceExpr  = 0
	musPlaceStaff = 11

	// The category a .mus reader invents for its rehearsal marks, these
	// files having no categories of their own. Any number does; it is only
	// ever compared with itself.
	musRehearsalCategory = 1
)

// What an expression does when the score is played, which is the only thing
// a .mus says about what kind of mark it is.
const (
	musPlayTempo     = 1
	musPlayAmplitude = 2
)

// musBarlineStyles is the top half of a measure's flags byte.
//
// A final barline would be 3, and it does not occur once in the files anybody
// has counted, so it is left out rather than guessed at.
var musBarlineStyles = map[int]string{1: "normal", 2: "double"}

// The bottom half of the same byte: a repeat barline one way or the other,
// and the start of a numbered ending.
//
// The two repeat signs are read. The ending is not: gscore draws a numbered
// ending as a bracket over a run of bars, and the bit says only that one
// begins here — not which times through it is played, nor where it stops,
// and a first-time bar played every time is a piece of the wrong length.
// So it is counted and named instead, because a piece whose repeats have
// gone quiet is the sort of thing somebody finds out in a rehearsal.
const (
	musRepeatEnding  = 0x02
	musRepeatBack    = 0x04
	musRepeatForward = 0x08
)

// musDocument turns the pools into the document the score is built from.
func musDocument(pools []musPool, order binary.ByteOrder, era musEra) (document, map[string]int, error) {
	var doc document
	census := map[string]int{}

	named, err := musIdentify(pools, order)
	if err != nil {
		return doc, nil, err
	}

	entries, err := musEntries(named[poolEntries], order)
	if err != nil {
		return doc, nil, fmt.Errorf("mus: %w", err)
	}
	if n := musGraceNotes(named[poolEntries], order); n > 0 {
		census["grace notes, which come through at their written length"] = n
	}

	others, details, unread := musRecords(named, order, era)
	if unread > 0 {
		census["other kinds of record, which hold the marks on the music"] = unread
	}

	// The frames, and which of them name entries the pool does not hold. A
	// frame that names a missing entry is dropped rather than kept, because
	// the score builder refuses a chain it cannot walk — so keeping one
	// would cost the whole document rather than one frame of music.
	frames, dropped := musFrames(others, entries, order)
	holds := musHolds(details, others, dropped, order, era)

	// A .mus is written in place, so an entry deleted in Finale keeps its
	// slot in the pool until the file is next compacted. The pool therefore
	// holds both the music as it stands and whatever earlier music has not
	// been written over yet, and the frames are the only thing that says
	// which is which. Keeping the rest would put bars of deleted music in
	// the score.
	live := musLive(entries, frames, holds)

	// A document with no music in it is a document, not a failure.
	//
	// Blank is the ordinary state of a great many .mus files: a template is
	// staves and bars and nothing else, and so is the starting file of every
	// exercise in a tutorial. Of the hundred and two files in the one
	// collection large enough to argue from, seventy-one are blank, and
	// every one of them has its staves, its bars, its names and its key —
	// which is the whole of what a template is for. Refusing them, as this
	// did until it was first run against real files, loses a usable document
	// and tells somebody theirs is broken when it is not.
	//
	// So nothing here refuses. What is worth saying is the one case where
	// the file does hold music that this cannot reach: a frame naming a
	// chain of entries the pool does not contain. Those are dropped above,
	// because the score builder refuses a chain it cannot walk, and a frame
	// dropped silently is a bar of music gone with nothing said. Entries no
	// frame names are the opposite case and are not counted: that is the
	// space where deleted music used to be, and counting it would say a
	// score arrived incomplete when it arrived whole.
	if len(dropped) > 0 {
		census["bars of music whose notes are not in the file"] = len(dropped)
	}

	doc.Entries.Entries = live
	doc.Others.Bars = musBars(others, order, census)
	doc.Others.Staves = musStaves(others, order)
	doc.Others.Frames = musFrameSpecs(frames)
	doc.Others.Used = musUsed(others, doc.Others.Staves, order, census)
	doc.Details.Holds = holds
	doc.Details.Groups = musGroups(details, order)

	// The marks written against a bar. Only the rehearsal marks are made
	// sense of, so the one category this document needs is invented here
	// rather than read: a file of this era has none to read.
	defs, places, unknown := musRehearsals(others, order)
	for i := range defs {
		defs[i].Category = musRehearsalCategory
	}
	doc.Others.Exprs = defs
	doc.Others.ExprPlaces = places
	doc.Others.Categories = []markingsCategory{
		{Cmper: musRehearsalCategory, Type: "rehearsalMarks"},
	}
	if unknown > 0 {
		census["marks written against a bar, other than rehearsal marks"] = unknown
	}
	// The records that name the words come first, and the blocks pointing
	// at themselves after them, because the lookup takes the first record
	// of a number: a name a record speaks for is read through the record,
	// and a name no record covers still finds its block.
	selfs, texts := musTexts(named[poolText])
	doc.Others.Blocks = append(musTextBlocks(others, order), selfs...)
	doc.Texts.Blocks = texts

	return doc, census, nil
}

// musIdentify works out which pool is which.
//
// The older container names them and is taken at its word, including when it
// says a pool is empty: an empty entry pool is a real answer that three files
// in the hundred and forty anybody has looked at give, and sniffing on past
// it would either find nothing or find another pool and lie about it.
//
// The newer container names nothing. There it writes four pools in a fixed
// order — the records, the details, the music, the words — which is what is
// tried first, and then checked: the pool said to be the music has to read
// as the music. Only if that fails is each pool identified by what is in it,
// the entry pool being recognisable outright, the two record pools told
// apart by which framing rule reads them to the end, and the text pool being
// the one with Finale's own markup in it.
//
// Order first and structure second rather than the other way round, because
// structure alone cannot always tell the two record pools apart: a pool of
// nothing but empty records satisfies both rules at once, each reading its
// length field out of the other's zeroes.
func musIdentify(pools []musPool, order binary.ByteOrder) (map[int][]byte, error) {
	named := map[int][]byte{}
	labelled := false
	for _, p := range pools {
		if p.kind == 0 {
			continue
		}
		labelled = true
		if _, seen := named[p.kind]; !seen {
			named[p.kind] = p.data
		}
	}
	if labelled {
		if _, ok := named[poolEntries]; !ok {
			return nil, fmt.Errorf(
				"mus: this file's pools are named and none of them is the music")
		}
		return named, nil
	}

	// The order this era writes them in, checked rather than trusted.
	if written := []int{poolOthers, poolDetails, poolEntries, poolText}; len(pools) == len(written) {
		inOrder := map[int][]byte{}
		for i, kind := range written {
			inOrder[kind] = pools[i].data
		}
		if musLooksLikeEntries(inOrder[poolEntries], order) {
			return inOrder, nil
		}
	}

	for _, p := range pools {
		var kind int
		switch {
		case musLooksLikeEntries(p.data, order):
			kind = poolEntries
		case bytes.Contains(p.data, []byte("^block(")):
			kind = poolText
		case musTiles(p.data, order, musRecOthersHead):
			kind = poolOthers
		case musTiles(p.data, order, musRecDetailsHead):
			kind = poolDetails
		default:
			continue
		}
		if _, seen := named[kind]; !seen {
			named[kind] = p.data
		}
	}
	if _, ok := named[poolEntries]; !ok {
		return nil, fmt.Errorf(
			"mus: none of this file's %d pools holds music that can be read as "+
				"Finale's entries — if this came from Finale 2011 or 2012, it is "+
				"a kind of that format gscore has not met", len(pools))
	}
	return named, nil
}

// musRecords reads the two record pools, by whichever framing the era uses.
//
// It returns the records this reader knows, keyed so that the incidences of
// one record arrive together, and a count of everything else — which is the
// honest measure of how much of a Finale document this does not read.
func musRecords(named map[int][]byte, order binary.ByteOrder, era musEra) (
	others, details map[[3]int]*musRecord, unread int) {

	if era == eraDCL {
		others, unread = musWalkRows(named[poolOthers], order, musRowOthersData)
		d, n := musWalkRows(named[poolDetails], order, musRowDetailsData)
		return others, d, unread + n
	}
	others, unread = musWalkRecords(named[poolOthers], order, musRecOthersHead)
	d, n := musWalkRecords(named[poolDetails], order, musRecDetailsHead)
	return others, d, unread + n
}

// musWalkRows reads one pool of the older era: a flat table of fixed rows.
//
// The tag is a number holding two characters, so on a file written by a PC
// they come out back to front — ^MS is stored as "SM". Reading the pair as it
// lies finds no tag anybody knows in two files out of three, which is how
// that was noticed.
func musWalkRows(pool []byte, order binary.ByteOrder, dataAt int) (map[[3]int]*musRecord, int) {
	out := map[[3]int]*musRecord{}
	unread := 0
	if len(pool)%musRow != 0 || len(pool)/musRow > musMaxRows {
		return out, 0
	}

	for at := 0; at+musRow <= len(pool); at += musRow {
		row := pool[at : at+musRow]
		num := int(order.Uint16(row))
		num2 := 0
		if dataAt == musRowDetailsData {
			num2 = int(order.Uint16(row[2:]))
		}
		pair := row[dataAt-2 : dataAt]
		if order == binary.LittleEndian {
			pair = []byte{pair[1], pair[0]}
		}
		tag := string(pair)

		what, known := musRowTags[tag]
		if !known {
			unread++
			continue
		}
		key := [3]int{musTagKey(what), num, num2}
		rec := out[key]
		if rec == nil {
			rec = &musRecord{what: what, num: num, num2: num2}
			out[key] = rec
		}
		if len(rec.parts) < musMaxIncidences {
			rec.parts = append(rec.parts, row[dataAt:])
		}
	}
	return out, unread
}

// musWalkRecords reads one pool of the newer era: records that carry their
// own tag, key and length.
//
// Sections of one tag sit together and are separated by a two-byte filler
// word, which is all zeroes or all ones. Both have to be skipped: a run of
// all ones reads as a record of no length at all, and a walk that consumes
// its header and no more falls four bytes short and carries on, wrong, for a
// long way before it fails somewhere unrelated.
func musWalkRecords(pool []byte, order binary.ByteOrder, head int) (map[[3]int]*musRecord, int) {
	out := map[[3]int]*musRecord{}
	unread, seen := 0, 0

	for at := 0; at+head <= len(pool) && seen < musMaxRecords; seen++ {
		if w := order.Uint16(pool[at:]); w == 0x0000 || w == 0xffff {
			at += 2
			seen--
			continue
		}

		tag := int(order.Uint16(pool[at:]))
		num := int(order.Uint16(pool[at+2:]))
		num2, part := 0, 0
		if head == musRecDetailsHead {
			num2 = int(order.Uint16(pool[at+4:]))
		} else {
			part = int(order.Uint16(pool[at+4:]))
		}
		length := int(order.Uint32(pool[at+head-4:]))
		if length > musMaxPayload1 || at+head+length+musRecExtra > len(pool) {
			break
		}
		payload := pool[at+head : at+head+length]
		extra := int(order.Uint32(pool[at+head+length:]))
		step := head + length + musRecExtra + extra
		if step <= 0 || at+step > len(pool) {
			break
		}
		at += step

		what, known := musNumTags[tag]
		if !known {
			unread++
			continue
		}
		key := [3]int{musTagKey(what), num, num2}
		rec := out[key]
		if rec == nil {
			rec = &musRecord{what: what, num: num, num2: num2, part: part}
			out[key] = rec
		}
		if len(rec.parts) < musMaxIncidences {
			rec.parts = append(rec.parts, payload)
		}
	}
	return out, unread
}

// musTiles says whether a pool reads to its end under one of the two framing
// rules, which is what tells the newer era's two record pools apart.
//
// It is a property of real pools rather than a guarantee: a pool of nothing
// but empty records would satisfy both, because each rule would read its
// length out of the other's zeroes. Real pools are not like that.
func musTiles(pool []byte, order binary.ByteOrder, head int) bool {
	if len(pool) == 0 {
		return false
	}
	for at, seen := 0, 0; seen < musMaxRecords; seen++ {
		if at == len(pool) {
			return true
		}
		if at+head > len(pool) {
			return false
		}
		if w := order.Uint16(pool[at:]); w == 0x0000 || w == 0xffff {
			at += 2
			seen--
			continue
		}
		length := int(order.Uint32(pool[at+head-4:]))
		if length > musMaxPayload1 || at+head+length+musRecExtra > len(pool) {
			return false
		}
		step := head + length + musRecExtra + int(order.Uint32(pool[at+head+length:]))
		if step <= 0 || at+step > len(pool) {
			return false
		}
		at += step
	}
	return false
}

// musTagKey keeps records of different types from colliding on one key.
func musTagKey(what string) int {
	for i, name := range []string{
		"measSpec", "staffSpec", "frameSpec", "instUsed", "gfhold", "textBlock",
		"staffGroup", "textExprDef", "measExprAssign", "exprEnclosure",
	} {
		if name == what {
			return i + 1
		}
	}
	return 0
}

// musSorted is every record of one type, in key order, so that a document
// comes out the same way every time it is read.
func musSorted(recs map[[3]int]*musRecord, what string) []*musRecord {
	var out []*musRecord
	for _, r := range recs {
		if r.what == what && r.part == 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].num != out[b].num {
			return out[a].num < out[b].num
		}
		return out[a].num2 < out[b].num2
	})
	return out
}

// musBars reads the measures.
//
// A stored key of nothing is left out rather than written as zero, which is
// what the XML does too and what the score builder expects: an absent key
// means the measure keeps the one before it, where a key of zero would mean
// C major and would wipe the signature off every measure that inherits one.
func musBars(others map[[3]int]*musRecord, order binary.ByteOrder,
	census map[string]int) []measSpec {

	var out []measSpec
	repeats := 0
	for _, r := range musSorted(others, "measSpec") {
		p := r.all()
		if len(p) < musMeasDivBeat+2 {
			continue
		}
		bar := measSpec{
			Cmper:   r.num,
			Beats:   int(order.Uint16(p[musMeasBeats:])),
			DivBeat: int(order.Uint16(p[musMeasDivBeat:])),
		}
		if word := int(order.Uint16(p[musMeasKey:])); word != 0 {
			// Two fields in one word: the low byte counts the accidentals,
			// sharps up and flats down as a signed byte, and the high byte
			// is the mode.
			//
			// Read whole, the word is right for sharps by accident and
			// wrong for every flat: two flats is 0xFE, which as sixteen
			// bits is 254, and 254 is not a key signature anybody has, so
			// every flat key in the file was being thrown away as
			// nonstandard. A Bach fugue in G minor came through in no key
			// at all, with the two flats it is written in missing from
			// every staff — the one musical error in that whole file.
			//
			// The split is what the hundred and two files say: of the six
			// distinct words among them, five have a zero high byte and one
			// is 0x01FE, which is the G minor file, and the low bytes are
			// 0, ±1, 2 and 4. gscore's modes are numbered the way Finale
			// numbers them, major first and minor second.
			fifths := int(int8(uint8(word)))
			bar.Key = &fifths
			bar.Mode = word >> 8
		}
		if len(p) > musMeasFlags+1 {
			// The low byte of the number at +10, not the byte at +10. The
			// pools hold two-byte values, so on a file written by a Mac
			// the flags are the second of the pair and on a PC the first;
			// reading the byte itself takes the high half of every Mac
			// file and finds barline styles that do not exist.
			flags := int(order.Uint16(p[musMeasFlags:]) & 0xff)
			if style, ok := musBarlineStyles[flags>>4]; ok {
				bar.Barline = style
			}
			bar.RepeatForward = flags&musRepeatForward != 0
			bar.RepeatBack = flags&musRepeatBack != 0
			if flags&musRepeatEnding != 0 {
				repeats++
			}
		}
		out = append(out, bar)
	}
	if repeats > 0 {
		census["first and second endings, where a repeat is played differently"] = repeats
	}
	return out
}

// musStaves reads the staves.
//
// Of the transposition a .mus holds only the part that says how the key
// signature changes — so many sharps added — and not the interval the staff
// is written at. The interval within the octave does not have to be stored,
// because that number is the same answer said twice: a staff whose key gains
// three sharps is written three fifths up, which is a sixth, and a sixth is
// five steps and nine semitones however it is written down. So the steps are
// worked out here from the sharps, and the two halves of the interval agree
// by construction.
//
// They did not before, and that is worth saying because of what it looked
// like. The steps were left at nothing while the semitones were read, so
// every note of a transposing staff was written where it sounds and then
// given nine semitones of accidental to make up the difference: a page of
// double sharps, on notes two octaves from where a saxophone plays. The
// music was not wrong — it was a sixth, spelled as a unison and a monstrous
// accidental, which is the same sound and no part anybody could read.
//
// What is genuinely absent is how many octaves the interval spans: Finale
// folds it into a residue within an octave and keeps the octaves in the
// instrument definition, which is not in the file, so a tenor saxophone and
// a soprano store the same bytes. That is recovered from the staff's name
// where the name is one gscore knows — see withOctavesOf — and a .musx has
// it outright, because Finale wrote it out when it saved one.
func musStaves(others map[[3]int]*musRecord, order binary.ByteOrder) []staffSpec {
	var out []staffSpec
	for _, r := range musSorted(others, "staffSpec") {
		p := r.all()
		s := staffSpec{Cmper: r.num, StaffLines: 5}
		if len(p) >= musStaffTrans+2 {
			raw := int(order.Uint16(p[musStaffTrans:]))
			adjust := raw & musTCDAltSize
			if raw&musTCDAltBelow != 0 {
				adjust = -adjust
			}
			s.Transposition.Adjust = adjust
			// Four steps to the fifth, folded into the octave. The octave
			// itself is not in the file, so the staff is marked as having
			// lost it rather than being given one that was guessed.
			s.Transposition.Interval = mod(adjust*4, 7)
			s.OctaveLost = adjust != 0
		}
		if len(p) >= musStaffName+2 {
			s.FullName = int(order.Uint16(p[musStaffName:]))
		}
		if len(p) >= musStaffAbbrv+2 {
			s.AbbrvName = int(order.Uint16(p[musStaffAbbrv:]))
		}
		out = append(out, s)
	}
	return out
}

// musFrame is one frame's span of the entry chain.
type musFrame struct {
	num        int
	start, end int
}

// musFrames reads the frames, and says which ones could not be used.
//
// A frame's pair of entries is in its last incidence. Where there is more
// than one, the first holds the time within the bar that the frame's music
// starts at, which this reader does not use — but reading the pair from the
// first incidence instead would find two numbers that are not entries at all.
func musFrames(others map[[3]int]*musRecord, entries []entry, order binary.ByteOrder) (
	[]musFrame, map[int]bool) {

	held := map[int]bool{}
	for _, e := range entries {
		held[e.Num] = true
	}

	var out []musFrame
	dropped := map[int]bool{}
	for _, r := range musSorted(others, "frameSpec") {
		p := r.last()
		if len(p) < 8 {
			dropped[r.num] = true
			continue
		}
		f := musFrame{
			num:   r.num,
			start: int(order.Uint32(p)),
			end:   int(order.Uint32(p[4:])),
		}
		if !held[f.start] || !held[f.end] {
			dropped[r.num] = true
			continue
		}
		out = append(out, f)
	}
	return out, dropped
}

func musFrameSpecs(frames []musFrame) []frameSpec {
	var out []frameSpec
	for _, f := range frames {
		out = append(out, frameSpec{
			Cmper:      f.num,
			StartEntry: f.start,
			EndEntry:   f.end,
		})
	}
	return out
}

// musHolds reads what puts music on a staff: for each staff and measure, the
// clef and the frame of each layer.
//
// Where the layers start inside the record moved between 2001 and 2005, and
// what says which is at hand already: a staff record is three rows in a 2001
// file and six in a 2005 one. Guessing the later offset for both finds about
// a third of the frames.
func musHolds(details, others map[[3]int]*musRecord, dropped map[int]bool,
	order binary.ByteOrder, era musEra) []gfhold {

	base := musFrameBase2005
	if era == eraDCL {
		if specs := musSorted(others, "staffSpec"); len(specs) > 0 && len(specs[0].parts) == 3 {
			base = musFrameBase2001
		}
	}

	var out []gfhold
	for _, r := range musSorted(details, "gfhold") {
		p := r.all()
		h := gfhold{Staff: r.num, Meas: r.num2}
		if len(p) >= musHoldClefID+2 {
			// A clef of nothing means the staff's own default, which is
			// what an absent field means to the score builder. Writing
			// zero would mean the first clef in the table.
			if id := int(order.Uint16(p[musHoldClefID:])); id != 0 {
				h.ClefID = &id
			}
		}
		into := []*int{&h.Frame1, &h.Frame2, &h.Frame3, &h.Frame4}
		for layer := 0; layer < musFrameLayers; layer++ {
			at := base + 2*layer
			if at+2 > len(p) {
				break
			}
			// A frame that was dropped takes its references with it: a
			// hold naming a frame that is not there is refused the same
			// way a broken chain is, and that would cost the document.
			if f := int(order.Uint16(p[at:])); f != 0 && !dropped[f] {
				*into[layer] = f
			}
		}
		out = append(out, h)
	}
	return out
}

// musLive is the entries some frame reaches, in pool order.
//
// Everything else is the space where deleted music used to be. That this is
// what it is rather than music the reader failed to place is not a guess: of
// the entries no frame reaches, the overwhelming majority are exact copies of
// passages the frames do reach, matched as whole chains rather than note by
// note.
func musLive(entries []entry, frames []musFrame, holds []gfhold) []entry {
	byNum := make(map[int]entry, len(entries))
	for _, e := range entries {
		byNum[e.Num] = e
	}
	span := make(map[int]musFrame, len(frames))
	for _, f := range frames {
		span[f.num] = f
	}

	reached := map[int]bool{}
	for _, h := range holds {
		for _, which := range []int{h.Frame1, h.Frame2, h.Frame3, h.Frame4} {
			f, ok := span[which]
			if !ok {
				continue
			}
			// Bounded by the pool: every step takes an entry not yet
			// visited, so a chain that loops or never meets its end still
			// stops.
			at := f.start
			for range entries {
				e, ok := byNum[at]
				if !ok || reached[at] {
					break
				}
				reached[at] = true
				if at == f.end {
					break
				}
				at = e.Next
			}
		}
	}

	var out []entry
	for _, e := range entries {
		if reached[e.Num] {
			out = append(out, e)
		}
	}
	return out
}

// musUsed is the order the staves are read in, top to bottom.
//
// Not the order the staff records are in: that is the order somebody built
// the score in. The older pools hold the reading order outright; the newer
// ones hold it under a name nobody has identified, so a file from 2011 or
// 2012 falls back to the staff records and says so, since a score whose
// staves are in the wrong order is wrong in a way that is easy to miss and
// easy to fix by hand once somebody knows.
func musUsed(others map[[3]int]*musRecord, staves []staffSpec,
	order binary.ByteOrder, census map[string]int) []instUsed {

	rec := others[[3]int{musTagKey("instUsed"), 0, 0}]
	if rec == nil {
		if len(staves) > 1 {
			census["the order the staves are read in, top to bottom"] = 1
		}
		return nil
	}

	exists := map[int]bool{}
	for _, s := range staves {
		exists[s.Cmper] = true
	}

	var out []instUsed
	for i, p := range rec.parts {
		if len(p) < 2 {
			return nil
		}
		inst := int(order.Uint16(p))
		if !exists[inst] {
			// Not the layout order after all. Better to fall back to the
			// staff records, which are at least every staff once, than to
			// put the music on staves that are not there.
			census["the order the staves are read in, top to bottom"] = 1
			return nil
		}
		out = append(out, instUsed{Inci: i, Inst: inst})
	}
	if len(out) != len(staves) {
		census["the order the staves are read in, top to bottom"] = 1
		return nil
	}
	return out
}

// musTextBlock matches one block of Finale's tagged text.
var musTextBlock = regexp.MustCompile(`(?s)\^block\((\d+)\)(.*?)\^end`)

// musRehearsals reads the marks a conductor calls a bar by.
//
// Finale says an expression in two records: a definition — what the mark is
// and what it does when the score is played — and an assignment, one place
// it is written. What a .mus does not say is what *kind* of mark it is.
// Categories came to Finale long after these files were written, so there is
// no record to ask and no field in the definition that names one: before
// categories, an expression was a line in one flat list.
//
// So the kind has to be worked out from what the definition carries, and
// only one kind can be worked out with any confidence:
//
//   - a definition that sets the speed or the loudness says so, in a byte
//     that says what it does to playback. Those are the tempo marks and the
//     dynamics, and neither is read here — a dynamic belongs to the note it
//     stands over and this reader has no way to find that note, and a tempo
//     mark needs the beat it counts in, which is somewhere this has not
//     found.
//   - of what is left, a mark written against a bar and drawn inside a box
//     is a rehearsal mark. That is what the box is for: it is the one
//     marking on a page that has to be found from across a hall.
//
// A boxed direction written on a bar would be read as a rehearsal mark too.
// That is the cost of having no categories, and it is the right way round:
// the mark lands on the bar it was written on either way, and a conductor
// who calls a bar by the wrong name is better off than one with no name
// for it at all.
func musRehearsals(others map[[3]int]*musRecord, order binary.ByteOrder) (
	[]textExprDef, []measExprAssign, int) {

	// Which definitions are drawn in a box. An enclosure record exists for
	// marks that are not boxed as well, with the top bit of its flags set
	// to say the box is there and not drawn.
	boxed := map[int]bool{}
	for _, r := range musSorted(others, "exprEnclosure") {
		p := r.all()
		if len(p) > musEnclFlags && p[musEnclFlags]&musEnclHidden == 0 {
			boxed[r.num] = true
		}
	}

	defs := map[int]textExprDef{}
	for _, r := range musSorted(others, "textExprDef") {
		p := r.all()
		if len(p) <= musExprPlay {
			continue
		}
		switch p[musExprPlay] {
		case musPlayTempo, musPlayAmplitude:
			continue
		}
		if !boxed[r.num] {
			continue
		}
		words := musExprDesc(p)
		defs[r.num] = textExprDef{
			Cmper: r.num, Words: words, Numbered: words == "",
		}
	}

	var places []measExprAssign
	var used []textExprDef
	seen, other := map[int]bool{}, 0
	for _, r := range musSorted(others, "measExprAssign") {
		p := r.all()
		for at := 0; at+musRowOthersSlot <= len(p); at += musRowOthersSlot {
			slot := p[at : at+musRowOthersSlot]
			id := int(order.Uint16(slot[musPlaceExpr:]))
			def, ok := defs[id]
			if !ok {
				other++
				continue
			}
			places = append(places, measExprAssign{
				Meas:  r.num,
				Expr:  id,
				Staff: int(slot[musPlaceStaff]),
				Inci:  len(places),
			})
			if !seen[id] {
				seen[id] = true
				used = append(used, def)
			}
		}
	}
	return used, places, other
}

// musExprDesc is how the library describes a definition, which is the words
// it prints only when it has fixed words to print.
//
// A rehearsal mark Finale numbers or letters by itself has none — the mark
// is worked out when the page is drawn — and what is stored in their place
// is a phrase describing the mark: "Boxed Rehearsal Number". A mark is a
// token and a description is a sentence, so a space tells them apart, and
// the marks that have none are named after the bar they stand on, which is
// what Finale prints for the numbered ones.
func musExprDesc(p []byte) string {
	if len(p) <= musExprWords {
		return ""
	}
	raw := p[musExprWords:]
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	words := strings.TrimSpace(string(raw))
	if strings.ContainsAny(words, " \t") {
		return ""
	}
	return words
}

// musGroups reads the brackets and braces down the left of a system.
//
// A group is two staff numbers and a shape, and the shape is the whole of
// what it says on the page: a brace for the two hands of one player, a
// bracket for a family of them, a plain line for staves that merely belong
// near each other. Without it a saxophone quartet comes out as four
// unconnected staves — which is four soloists who happen to share a page,
// and not what the score says.
//
// One field of the record is deliberately not read. The catalogued layout
// puts a flag at +21 saying whether the barlines run down through the gaps
// between the staves, and on the quartet this was read against that byte is
// zero while the printed page runs them through plainly — so the offset is
// not the flag, or not only the flag, and reading it would draw a quartet
// wrong on the strength of a number that has already been seen to be wrong.
// The shape decides instead: a bracket or a brace is what joins players into
// one thing, and a line is what stops short of saying so. That is Finale's
// own default for a new group, and the case it does not cover is a group
// whose barlines have been turned off by hand.
func musGroups(details map[[3]int]*musRecord, order binary.ByteOrder) []staffGroup {
	var out []staffGroup
	for _, r := range musSorted(details, "staffGroup") {
		p := r.all()
		if len(p) < musGroupShape+2 {
			continue
		}
		g := staffGroup{
			StartInst: int(order.Uint16(p[musGroupStart:])),
			EndInst:   int(order.Uint16(p[musGroupEnd:])),
		}
		g.Bracket.ID = int(order.Uint16(p[musGroupShape:]))
		if g.Bracket.ID != 0 {
			// Finale's own word for barlines drawn through the gap.
			g.BarlineStyle = "group"
		}
		out = append(out, g)
	}
	return out
}

// musTextBlocks reads the records that say which words a name stands for.
//
// A staff does not name its words directly. It names a text-block record,
// and that record names the block of words — and the two numbers are not the
// same one: in the quartet this was found on, the staves ask for blocks 50
// to 57 and the words are in blocks 18 to 25. Reading the first number as if
// it were the second looks for words that are not there, which is a staff
// with no name, which is a staff played as a piano. Four saxophones came
// through as four pianos that way, and nothing said so, because a nameless
// staff is not an error.
//
// The record is written over three rows and only the first holds the number.
// The rest is which font the words are set in, which gscore does not keep.
func musTextBlocks(others map[[3]int]*musRecord, order binary.ByteOrder) []textBlock {
	var out []textBlock
	for _, r := range musSorted(others, "textBlock") {
		p := r.all()
		if len(p) < 2 {
			continue
		}
		id := int(order.Uint16(p))
		if id == 0 {
			continue
		}
		// A .mus keeps one pool of words, so there is no second pool for a
		// block to be in and nothing to choose between.
		out = append(out, textBlock{Cmper: r.num, TextID: id, Tag: "block"})
	}
	return out
}

// musTexts reads the words out of the text pool.
//
// The pool is Finale's own markup and is the same in both eras and in a
// .musx: a run of blocks, each numbered, with the font and the size written
// inline and the words among them.
//
// Each block is given here as a record pointing at itself as well as as the
// words, which is what a name is read by where no text-block record speaks
// for it — in a newer .mus, whose record types this does not yet know by
// number, that is every name. Where musTextBlocks finds one, it wins.
func musTexts(pool []byte) ([]textBlock, []blockText) {
	var blocks []textBlock
	var texts []blockText
	for _, m := range musTextBlock.FindAllSubmatch(pool, -1) {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		if n == 0 {
			continue
		}
		blocks = append(blocks, textBlock{Cmper: n, TextID: n, Tag: "block"})
		texts = append(texts, blockText{Number: n, Text: musText(m[2])})
	}
	return blocks, texts
}

// musText is one block's words with the markup taken out.
func musText(raw []byte) string {
	return strings.TrimSpace(enigmaTags.ReplaceAllString(string(raw), ""))
}
